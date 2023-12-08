package ipam

import (
	"context"
	"fmt"
	"net"

	cniv1 "github.com/containernetworking/cni/pkg/types/100"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/everoute/ipam/api/ipam/v1alpha1"
	"github.com/everoute/ipam/pkg/utils"
)

const (
	IpamAnnotationPool     = "ipam.everoute.io/pool"
	IpamAnnotationStaticIP = "ipam.everoute.io/static-ip"

	UpdateRetryCount = 5
	FindRetryCount   = 5
)

type OP int

const (
	IPAdd OP = iota
	IPDel
)

const (
	IPPoolOffsetFull   = -1
	IPPoolOffsetIgnore = -2
)

type Ipam struct {
	k8sClient client.Client
	namespace string
}

type NetConf struct {
	Pool        string
	IP          string
	ContainerID string
}

func InitIpam(k8sClient client.Client, namespace string) *Ipam {
	ipam := &Ipam{
		k8sClient: k8sClient,
		namespace: namespace,
	}

	return ipam
}

func (i *Ipam) ExecAdd(conf *NetConf) (*cniv1.Result, error) {
	ipPool := &v1alpha1.IPPool{}

	// get target ip pool
	if conf.Pool == "" {
		ipPools := v1alpha1.IPPoolList{}
		if err := i.k8sClient.List(context.Background(), &ipPools); err != nil {
			klog.Errorf("list ipPool error, err:%s", err)
		}
		// get the first no-full ip pool
		for index, item := range ipPools.Items {
			if item.Status.Offset != IPPoolOffsetFull && item.Name != "" {
				ipPool = &ipPools.Items[index]
				break
			}
		}
		if ipPool.Name == "" {
			return nil, fmt.Errorf("no IP address allocated in all pools")
		}
	} else {
		// get user-specified ip pool
		req := k8stypes.NamespacedName{
			Name:      conf.Pool,
			Namespace: i.namespace,
		}
		if err := i.k8sClient.Get(context.Background(), req, ipPool); err != nil {
			return nil, fmt.Errorf("get ip pool %s error, err: %s", req, err)
		}
	}

	conf.Pool = ipPool.Name
	klog.Infof("use ippool %s\n", ipPool.Name)
	_, ipNet, _ := net.ParseCIDR(ipPool.Spec.CIDR)

	// handle static ip
	if conf.IP != "" {
		ip := net.ParseIP(conf.IP)
		klog.Infof("use static ip %s\n", conf.IP)
		// check if valid
		if ip == nil {
			return nil, fmt.Errorf("invalid static ip %s", conf.IP)
		}
		if !ipNet.Contains(ip) {
			return nil, fmt.Errorf("static ip %s is not in target pool", conf.IP)
		}
		_, exist := ipPool.Status.UsedIps[conf.IP]
		if exist {
			return nil, fmt.Errorf("static ip %s is already in use", conf.IP)
		}

		// update ip address into pool
		if err := i.UpdatePool(conf, IPPoolOffsetIgnore, IPAdd); err != nil {
			return nil, err
		}

		return i.ParseResult(ipPool, conf.IP), nil
	}

	for retry := 0; retry < FindRetryCount; retry++ {
		newIP, newOffset := i.FindNext(ipPool)
		klog.Info(newIP, newOffset)
		conf.IP = newIP.String()
		if err := i.UpdatePool(conf, newOffset, IPAdd); err != nil {
			klog.Error(err)
			continue
		}
		if newOffset == IPPoolOffsetFull {
			break
		}
		return i.ParseResult(ipPool, newIP.String()), nil
	}

	return nil, fmt.Errorf("find valid ip error in pool %s", conf.Pool)
}

func (i *Ipam) ExecCheck(*NetConf) error {
	return nil
}

func (i *Ipam) ExecDel(containerID string) error {
	ipPools := v1alpha1.IPPoolList{}
	if err := i.k8sClient.List(context.Background(), &ipPools); err != nil {
		klog.Errorf("list ipPool error, err:%s", err)
	}
	for _, item := range ipPools.Items {
		_ = i.UpdatePool(&NetConf{Pool: item.Name, ContainerID: containerID}, 0, IPDel)
	}

	return nil
}

func (i *Ipam) FindNext(ipPool *v1alpha1.IPPool) (net.IP, int64) {
	_, ipNet, _ := net.ParseCIDR(ipPool.Spec.CIDR)
	_, subnet, _ := net.ParseCIDR(ipPool.Spec.Subnet)

	start := utils.Ipv4ToUint32(ipNet.IP)
	ones, bits := ipNet.Mask.Size()
	hostBits := bits - ones
	length := 1 << hostBits

	oldOffset := ipPool.Status.Offset
	offset := ipPool.Status.Offset

	for {
		ipNum := start + uint32(offset)
		newIP := utils.Uint32ToIpv4(ipNum)
		if !newIP.Equal(utils.FirstIP(subnet)) &&
			!newIP.Equal(utils.LastIP(subnet)) &&
			newIP.String() != ipPool.Spec.Gateway {
			if _, exist := ipPool.Status.UsedIps[newIP.String()]; !exist {
				// get valid IP and set offset to next pos
				return newIP, (offset + 1) % int64(length)
			}
		}

		offset = (offset + 1) % int64(length)
		if offset == oldOffset {
			break
		}
	}

	return nil, IPPoolOffsetFull
}

func (i *Ipam) UpdatePool(conf *NetConf, offset int64, op OP) error {
	for retry := 0; retry < UpdateRetryCount; retry++ {
		// get up-to-date pool
		pool := &v1alpha1.IPPool{}
		req := k8stypes.NamespacedName{
			Name:      conf.Pool,
			Namespace: i.namespace,
		}
		if err := i.k8sClient.Get(context.Background(), req, pool); err != nil {
			return fmt.Errorf("get ip pool error,err %s", err)
		}

		// init UsedIps
		if pool.Status.UsedIps == nil {
			pool.Status.UsedIps = make(map[string]string)
		}

		switch op {
		case IPAdd:
			if _, exist := pool.Status.UsedIps[conf.IP]; exist {
				return fmt.Errorf("ip address exist")
			}
			if offset != IPPoolOffsetFull {
				pool.Status.UsedIps[conf.IP] = conf.ContainerID
			}
			if offset != IPPoolOffsetIgnore {
				pool.Status.Offset = offset
			}
		case IPDel:
			for k, v := range pool.Status.UsedIps {
				if v == conf.ContainerID {
					delete(pool.Status.UsedIps, k)
					if pool.Status.Offset == IPPoolOffsetFull {
						pool.Status.Offset = offset
					}
					break
				}
			}
		}

		// update status
		err := i.k8sClient.Status().Update(context.Background(), pool)
		if err == nil {
			return nil
		}
		klog.Error(err)
	}

	return fmt.Errorf("update ipPool error")
}

func (i *Ipam) ParseResult(ipPool *v1alpha1.IPPool, ip string) *cniv1.Result {
	var ipNet *net.IPNet
	if ipPool.Spec.Subnet != "" {
		_, ipNet, _ = net.ParseCIDR(ipPool.Spec.Subnet)
	} else {
		_, ipNet, _ = net.ParseCIDR(ipPool.Spec.CIDR)
	}
	return &cniv1.Result{
		IPs: []*cniv1.IPConfig{
			{
				Address: net.IPNet{
					IP:   net.ParseIP(ip),
					Mask: ipNet.Mask,
				},
				Gateway: net.ParseIP(ipPool.Spec.Gateway),
			},
		},
	}
}

func (i *Ipam) FetchGwbyIP(ip net.IP) net.IP {
	ipPools := v1alpha1.IPPoolList{}
	if err := i.k8sClient.List(context.Background(), &ipPools); err != nil {
		klog.Errorf("list ipPool error, err:%s", err)
		return nil
	}
	for _, item := range ipPools.Items {
		_, ipNet, _ := net.ParseCIDR(item.Spec.CIDR)
		if ipNet.Contains(ip) {
			return net.ParseIP(item.Spec.Gateway)
		}
	}
	return nil
}
