package ipam

import (
	"context"
	"fmt"
	"net"

	cniv1 "github.com/containernetworking/cni/pkg/types/100"
	"k8s.io/apimachinery/pkg/types"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/everoute/ipam/api/ipam/v1alpha1"
	"github.com/everoute/ipam/pkg/constants"
	"github.com/everoute/ipam/pkg/utils"
)

const (
	UpdateRetryCount = 5
	FindRetryCount   = 5
)

type OP int

const (
	IPAdd OP = iota
	IPDel
)

type Ipam struct {
	k8sClient client.Client
	namespace string
}

// InitIpam returns a Ipam, param k8sClient that must add ippool scheme
func InitIpam(k8sClient client.Client, namespace string) *Ipam {
	ipam := &Ipam{
		k8sClient: k8sClient,
		namespace: namespace,
	}

	return ipam
}

func (i *Ipam) getTargetIPPool(ctx context.Context, conf *NetConf) (*v1alpha1.IPPool, string, error) {
	ipPool := &v1alpha1.IPPool{}

	if conf.Pool != "" {
		// get user-specified ip pool
		req := k8stypes.NamespacedName{
			Name:      conf.Pool,
			Namespace: i.namespace,
		}
		if err := i.k8sClient.Get(ctx, req, ipPool); err != nil {
			return nil, "", fmt.Errorf("get ip pool %s error, err: %s", req, err)
		}
		if ip := reallocateIP(conf, ipPool); ip != "" {
			err := i.updateRelocateIPStatus(ctx, conf, ip, ipPool)
			return ipPool, ip, err
		}
		return ipPool, "", nil
	}

	// get target ip pool
	ipPools := v1alpha1.IPPoolList{}
	if err := i.k8sClient.List(ctx, &ipPools); err != nil {
		klog.Errorf("list ipPool error, err:%s", err)
		return nil, "", err
	}
	for index, item := range ipPools.Items {
		if ip := reallocateIP(conf, &ipPools.Items[index]); ip != "" {
			err := i.updateRelocateIPStatus(ctx, conf, ip, &ipPools.Items[index])
			return &ipPools.Items[index], ip, err
		}
		// get the first no-full ip pool
		if ipPool.Name == "" {
			if item.Status.Offset != constants.IPPoolOffsetFull && item.Name != "" {
				ipPool = &ipPools.Items[index]
			}
		}
	}
	if ipPool.Name == "" {
		return nil, "", fmt.Errorf("no IP address allocated in all pools")
	}
	return ipPool, "", nil
}

func (i *Ipam) updateRelocateIPStatus(ctx context.Context, conf *NetConf, ip string, ippool *v1alpha1.IPPool) error {
	if conf.Type != v1alpha1.AllocateTypePod {
		return nil
	}
	a := ippool.Status.AllocatedIPs[ip]
	a.Extra = conf.AllocateIdentify
	if err := i.k8sClient.Status().Update(ctx, ippool); err != nil {
		klog.Errorf("Failed to update ippool %s status for pod %v, err: %v", ippool.GetName(), *conf, err)
		return err
	}
	return nil
}

//nolint:gocognit
func (i *Ipam) ExecAdd(ctx context.Context, conf *NetConf) (*cniv1.Result, error) {
	if err := conf.Valid(); err != nil {
		klog.Errorf("Invalid param %v, err: %v", *conf, err)
		return nil, err
	}

	ipPool, ip, err := i.getTargetIPPool(ctx, conf)
	if err != nil {
		klog.Errorf("Get target IPPool failed: %v", err)
		return nil, err
	}
	if ip != "" {
		klog.Infof("Reallocate ip %s to the same request %v", ip, *conf)
		return i.ParseResult(ipPool, ip), nil
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
		if _, exist := ipPool.Status.UsedIps[conf.IP]; exist {
			return nil, fmt.Errorf("static ip %s is already in use", conf.IP)
		}
		if allocateInfo, exist := ipPool.Status.AllocatedIPs[conf.IP]; exist {
			return nil, fmt.Errorf("static ip %s is already in use by %v", conf.IP, allocateInfo)
		}

		// update ip address into pool
		if err := i.UpdatePool(ctx, conf, constants.IPPoolOffsetIgnore, IPAdd); err != nil {
			return nil, err
		}

		return i.ParseResult(ipPool, conf.IP), nil
	}

	for retry := 0; retry < FindRetryCount; retry++ {
		newIP, newOffset := i.FindNext(ipPool)
		klog.Info(newIP, newOffset)
		conf.IP = newIP.String()
		if err := i.UpdatePool(ctx, conf, newOffset, IPAdd); err != nil {
			klog.Error(err)
			req := types.NamespacedName{
				Namespace: i.namespace,
				Name:      ipPool.GetName(),
			}
			if err := i.k8sClient.Get(ctx, req, ipPool); err != nil {
				klog.Errorf("Failed to get ippool %s, err: %v, continue", req, err)
			}
			continue
		}
		if newOffset == constants.IPPoolOffsetFull {
			break
		}
		return i.ParseResult(ipPool, newIP.String()), nil
	}

	return nil, fmt.Errorf("find valid ip error in pool %s", conf.Pool)
}

func (i *Ipam) ExecCheck(*NetConf) error {
	return nil
}

func (i *Ipam) ExecDel(ctx context.Context, conf *NetConf) error {
	if err := conf.Valid(); err != nil {
		klog.Errorf("Invalid param %v, err: %v", *conf, err)
		return nil
	}

	// for statefulset specify ip list, doesn't release ip when pod delete
	if conf.Type == v1alpha1.AllocateTypeStatefulSet {
		return nil
	}

	ipPools := v1alpha1.IPPoolList{}
	if err := i.k8sClient.List(ctx, &ipPools); err != nil {
		klog.Errorf("list ipPool error, err:%s", err)
	}
	for _, item := range ipPools.Items {
		conf.Pool = item.Name
		_ = i.UpdatePool(ctx, conf, constants.IPPoolOffsetReset, IPDel)
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
			_, usedIPExist := ipPool.Status.UsedIps[newIP.String()]
			_, allocateExist := ipPool.Status.AllocatedIPs[newIP.String()]
			if !usedIPExist && !allocateExist {
				// get valid IP and set offset to next pos
				return newIP, (offset + 1) % int64(length)
			}
		}

		offset = (offset + 1) % int64(length)
		if offset == oldOffset {
			break
		}
	}

	return nil, constants.IPPoolOffsetFull
}

//nolint:gocognit
func (i *Ipam) UpdatePool(ctx context.Context, conf *NetConf, offset int64, op OP) error {
	for retry := 0; retry < UpdateRetryCount; retry++ {
		// get up-to-date pool
		pool := &v1alpha1.IPPool{}
		req := k8stypes.NamespacedName{
			Name:      conf.Pool,
			Namespace: i.namespace,
		}
		if err := i.k8sClient.Get(ctx, req, pool); err != nil {
			klog.Errorf("get ip pool error,err %s", err)
			continue
		}

		// init UsedIps
		if pool.Status.UsedIps == nil {
			pool.Status.UsedIps = make(map[string]string)
		}
		if pool.Status.AllocatedIPs == nil {
			pool.Status.AllocatedIPs = make(map[string]v1alpha1.AllocateInfo)
		}

		statusUpdate := false
		switch op {
		case IPAdd:
			if _, exist := pool.Status.UsedIps[conf.IP]; exist {
				return fmt.Errorf("ip address exist")
			}
			if _, exist := pool.Status.AllocatedIPs[conf.IP]; exist {
				return fmt.Errorf("ip address exist")
			}
			if offset != constants.IPPoolOffsetFull {
				pool.Status.AllocatedIPs[conf.IP] = conf.genAllocateInfo()
			}
			if offset != constants.IPPoolOffsetIgnore {
				pool.Status.Offset = offset
			}
			statusUpdate = true
		case IPDel:
			for k, v := range pool.Status.UsedIps {
				if v == conf.AllocateIdentify {
					delete(pool.Status.UsedIps, k)
					if pool.Status.Offset == constants.IPPoolOffsetFull {
						pool.Status.Offset = offset
					}
					statusUpdate = true
					break
				}
			}
			for k, v := range pool.Status.AllocatedIPs {
				// for statefulset specify ip list, doesn't release ip when pod delete
				if v.Type == v1alpha1.AllocateTypeStatefulSet {
					continue
				}
				if isSameAllocateInfoForDel(v, conf) {
					delete(pool.Status.AllocatedIPs, k)
					if pool.Status.Offset == constants.IPPoolOffsetFull {
						pool.Status.Offset = offset
					}
					statusUpdate = true
					break
				}
			}
		}

		if !statusUpdate {
			return nil
		}
		// update status
		err := i.k8sClient.Status().Update(ctx, pool)
		if err == nil {
			return nil
		}
		klog.Errorf("update ipPool error: %v", err)
	}

	return fmt.Errorf("update ipPool failed")
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

func (i *Ipam) FetchGwbyIP(ctx context.Context, ip net.IP) net.IP {
	ipPools := v1alpha1.IPPoolList{}
	if err := i.k8sClient.List(ctx, &ipPools); err != nil {
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

func (i *Ipam) GetNamespace() string {
	return i.namespace
}

func reallocateIP(conf *NetConf, ipPool *v1alpha1.IPPool) (ip string) {
	if conf.Pool != "" && conf.Pool != ipPool.Name {
		return ""
	}
	if ipPool.Status.AllocatedIPs == nil {
		return ""
	}

	for ip := range ipPool.Status.AllocatedIPs {
		if isSameAllocateInfo(ipPool.Status.AllocatedIPs[ip], conf) {
			if conf.IP == "" || conf.IP == ip {
				return ip
			}
			klog.Errorf("Request ip %s is different from allocated ip %s for the same request %v", conf.IP, ip, *conf)
			return ""
		}
	}

	return ""
}

func isSameAllocateInfo(allocateInfo v1alpha1.AllocateInfo, conf *NetConf) bool {
	allocateID := conf.getAllocateID()
	return allocateInfo.Type == conf.Type && allocateInfo.ID == allocateID && allocateInfo.Owner == conf.Owner
}

func isSameAllocateInfoForDel(allocateInfo v1alpha1.AllocateInfo, conf *NetConf) bool {
	if !isSameAllocateInfo(allocateInfo, conf) {
		return false
	}
	if allocateInfo.Type == v1alpha1.AllocateTypePod {
		return allocateInfo.Extra == conf.AllocateIdentify
	}
	return true
}
