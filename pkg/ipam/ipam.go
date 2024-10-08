package ipam

import (
	"context"
	"fmt"
	"net"

	cniv1 "github.com/containernetworking/cni/pkg/types/100"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/errors"
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

//nolint:gocognit
func (i *Ipam) ExecAdd(ctx context.Context, conf *NetConf) (*cniv1.Result, error) {
	if err := conf.Valid(); err != nil {
		klog.Errorf("Invalid param %v, err: %v", *conf, err)
		return nil, err
	}

	ipPool, reallocIP, err := i.getTargetIPPool(ctx, conf)
	if err != nil {
		klog.Errorf("Get target IPPool failed: %v", err)
		return nil, err
	}
	if reallocIP != "" {
		klog.Infof("Reallocate ip %s to the same request %v", reallocIP, *conf)
		return i.ParseResult(ipPool, reallocIP), nil
	}

	conf.Pool = ipPool.Name
	klog.Infof("use ippool %s for request %v", ipPool.Name, *conf)

	// handle static ip
	if conf.IP != "" {
		ip := net.ParseIP(conf.IP)
		klog.Infof("use static ip %s\n", conf.IP)
		// check if valid
		if ip == nil {
			return nil, fmt.Errorf("invalid static ip %s", conf.IP)
		}
		if !ipPool.Contains(ip) {
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
		if retry > 0 {
			req := k8stypes.NamespacedName{
				Namespace: i.namespace,
				Name:      ipPool.GetName(),
			}
			if err := i.k8sClient.Get(ctx, req, ipPool); err != nil {
				klog.Errorf("Failed to get ippool %s, err: %v, continue", req, err)
				continue
			}
			if ipPool.Status.Offset == constants.IPPoolOffsetFull {
				break
			}
		}
		newIP, newOffset := i.FindNext(ipPool)
		klog.Info(newIP, newOffset)
		if newOffset == constants.IPPoolOffsetErr {
			klog.Errorf("can't find next IP for offset err")
			break
		}
		conf.IP = newIP.String()
		if err := i.UpdatePool(ctx, conf, newOffset, IPAdd); err != nil {
			klog.Error(err)
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

	if conf.Pool != "" {
		req := k8stypes.NamespacedName{
			Name:      conf.Pool,
			Namespace: i.namespace,
		}
		if err := i.k8sClient.Get(ctx, req, &v1alpha1.IPPool{}); err != nil {
			if apierrors.IsNotFound(err) {
				klog.Warningf("Can't release ip to ippool %v for the ippool doesn't exists, param: %v", req, *conf)
				return nil
			}
		}
		return i.UpdatePool(ctx, conf, constants.IPPoolOffsetReset, IPDel)
	}

	ipPools := v1alpha1.IPPoolList{}
	if err := i.k8sClient.List(ctx, &ipPools, client.InNamespace(i.namespace)); err != nil {
		klog.Errorf("list ipPool error, err:%s", err)
		return err
	}

	var errs []error
	for _, item := range ipPools.Items {
		conf.Pool = item.Name
		err := i.UpdatePool(ctx, conf, constants.IPPoolOffsetReset, IPDel)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.NewAggregate(errs)
	}

	return nil
}

func (i *Ipam) FindNext(ipPool *v1alpha1.IPPool) (net.IP, int64) {
	_, subnet, _ := net.ParseCIDR(ipPool.Spec.Subnet)

	start := utils.Ipv4ToUint32(ipPool.StartIP())
	length := ipPool.Length()

	oldOffset := ipPool.Status.Offset
	if oldOffset >= length {
		return nil, constants.IPPoolOffsetErr
	}
	offset := ipPool.Status.Offset

	firstIP := utils.FirstIP(subnet)
	lastIP := utils.LastIP(subnet)

	exceptNets := make([]*net.IPNet, 0, len(ipPool.Spec.Except))
	for i := range ipPool.Spec.Except {
		_, ipNet, _ := net.ParseCIDR(ipPool.Spec.Except[i])
		exceptNets = append(exceptNets, ipNet)
	}

	validIP := func(ip net.IP) bool {
		if ip.Equal(firstIP) || ip.Equal(lastIP) {
			return false
		}
		if ip.String() == ipPool.Spec.Gateway {
			return false
		}
		_, usedIPExist := ipPool.Status.UsedIps[ip.String()]
		_, allocateExist := ipPool.Status.AllocatedIPs[ip.String()]
		if usedIPExist || allocateExist {
			return false
		}
		for i := range exceptNets {
			if exceptNets[i].Contains(ip) {
				return false
			}
		}
		return true
	}

	for {
		ipNum := start + uint32(offset)
		newIP := utils.Uint32ToIpv4(ipNum)
		if validIP(newIP) {
			// get valid IP and set offset to next pos
			return newIP, (offset + 1) % length
		}

		offset = (offset + 1) % length
		if offset == oldOffset {
			break
		}
	}

	return nil, constants.IPPoolOffsetFull
}

//nolint:gocognit
func (i *Ipam) UpdatePool(ctx context.Context, conf *NetConf, offset int64, op OP) error {
	req := k8stypes.NamespacedName{
		Name:      conf.Pool,
		Namespace: i.namespace,
	}
	for retry := 0; retry < UpdateRetryCount; retry++ {
		// get up-to-date pool
		pool := &v1alpha1.IPPool{}
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
			if a, exist := pool.Status.AllocatedIPs[conf.IP]; exist {
				if isSameAllocateInfo(a, conf) {
					return nil
				}
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
				if isSameAllocateInfo(v, conf) {
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

		pool.UpdateIPUsageCounter()

		// update status
		err := i.k8sClient.Status().Update(ctx, pool)
		if err == nil {
			return nil
		}
		klog.Errorf("update ipPool %v error: %v", req, err)
	}

	return fmt.Errorf("update ipPool %v failed", req)
}

func (i *Ipam) ParseResult(ipPool *v1alpha1.IPPool, ip string) *cniv1.Result {
	var ipNet *net.IPNet
	_, ipNet, _ = net.ParseCIDR(ipPool.Spec.Subnet)
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
	if err := i.k8sClient.List(ctx, &ipPools, client.InNamespace(i.namespace)); err != nil {
		klog.Errorf("list ipPool error, err:%s", err)
		return nil
	}
	for _, item := range ipPools.Items {
		if item.Contains(ip) {
			return net.ParseIP(item.Spec.Gateway)
		}
	}
	return nil
}

func (i *Ipam) GetNamespace() string {
	return i.namespace
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
		if conf.IP == "" {
			if ipPool.Status.Offset == constants.IPPoolOffsetFull {
				return nil, "", fmt.Errorf("the specified ippool %s has no IP to allocate", req)
			}
			return ipPool, "", nil
		}

		if ip := reallocateIP(conf, ipPool); ip != "" {
			err := i.updateRelocateIPStatus(ctx, conf, ip, ipPool)
			return ipPool, ip, err
		}
		return ipPool, "", nil
	}

	// get target ip pool
	ipPools := v1alpha1.IPPoolList{}
	if err := i.k8sClient.List(ctx, &ipPools, client.InNamespace(i.namespace)); err != nil {
		klog.Errorf("list ipPool error, err:%s", err)
		return nil, "", err
	}
	for index, item := range ipPools.Items {
		if ipPools.Items[index].Spec.Private {
			continue
		}
		// get the first no-full ip pool
		if item.Status.Offset != constants.IPPoolOffsetFull && item.Name != "" {
			ipPool = &ipPools.Items[index]
			break
		}
	}
	if ipPool.Name == "" {
		return nil, "", fmt.Errorf("no IP address allocated in all public pools")
	}
	return ipPool, "", nil
}

func (i *Ipam) updateRelocateIPStatus(ctx context.Context, conf *NetConf, ip string, ippool *v1alpha1.IPPool) error {
	if conf.Type != v1alpha1.AllocateTypePod {
		return nil
	}
	if conf.AllocateIdentify == ippool.Status.AllocatedIPs[ip].CID {
		return nil
	}
	// update cid
	newAllo := ippool.Status.AllocatedIPs[ip]
	newAllo.CID = conf.AllocateIdentify
	ippool.Status.AllocatedIPs[ip] = newAllo
	if err := i.k8sClient.Status().Update(ctx, ippool); err != nil {
		klog.Errorf("Failed to update ippool %s status for pod %v, err: %v", ippool.GetName(), *conf, err)
		return err
	}
	return nil
}

func reallocateIP(conf *NetConf, ipPool *v1alpha1.IPPool) (ip string) {
	if ipPool.Status.AllocatedIPs == nil {
		return ""
	}
	if conf.IP == "" {
		return ""
	}

	a, exists := ipPool.Status.AllocatedIPs[conf.IP]
	if !exists {
		return ""
	}
	if isSameAllocateInfoForReallocate(a, conf) {
		return conf.IP
	}

	return ""
}

func isSameAllocateInfoForReallocate(allocateInfo v1alpha1.AllocateInfo, conf *NetConf) bool {
	allocateID := conf.getAllocateID()
	return allocateInfo.Type == conf.Type && allocateInfo.ID == allocateID && allocateInfo.Owner == conf.Owner
}

func isSameAllocateInfo(allocateInfo v1alpha1.AllocateInfo, conf *NetConf) bool {
	if !isSameAllocateInfoForReallocate(allocateInfo, conf) {
		return false
	}
	if allocateInfo.Type == v1alpha1.AllocateTypePod {
		return allocateInfo.CID == conf.AllocateIdentify
	}
	return true
}
