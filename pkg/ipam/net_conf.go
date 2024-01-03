package ipam

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/everoute/ipam/api/ipam/v1alpha1"
	"github.com/everoute/ipam/pkg/constants"
	"github.com/everoute/ipam/pkg/utils"
)

type NetConf struct {
	Pool string
	IP   string
	// default is containerID, type=cniused, defined by the cni
	AllocateIdentify string
	K8sPodName       string
	K8sPodNs         string
	Owner            string
	// default is allocate IP to Pod
	Type v1alpha1.AllocateType
}

// Complete add ippool and static ip info to NetConf, param k8sClient must add corev1 scheme and appsv1 scheme
func (c *NetConf) Complete(ctx context.Context, k8sClient client.Client, poolNs string) error {
	// only complete netconf by k8s annotation
	if c.Type != v1alpha1.AllocateTypePod {
		return nil
	}
	// has been specify pool, return
	if c.Pool != "" {
		return nil
	}

	if c.K8sPodNs == "" || c.K8sPodName == "" {
		klog.Errorf("netconf %v must set K8sPodNs and K8sPodName for type %s", *c, v1alpha1.AllocateTypePod)
		return fmt.Errorf("must set K8sPodNs and K8sPodName for type %s", v1alpha1.AllocateTypePod)
	}

	// complete by pod
	pod := corev1.Pod{}
	podNsName := types.NamespacedName{Namespace: c.K8sPodNs, Name: c.K8sPodName}
	err := k8sClient.Get(ctx, podNsName, &pod)
	if err != nil {
		klog.Errorf("Failed to get pod %v, err: %v", podNsName, err)
		return err
	}
	if pool, ok := pod.Annotations[constants.IpamAnnotationPool]; ok {
		c.Pool = pool
	}
	if ip, ok := pod.Annotations[constants.IpamAnnotationStaticIP]; ok {
		if c.Pool == "" {
			klog.Errorf("Pod %v can't only specify static IP but no pool", pod)
			return fmt.Errorf("can't only specify static IP but no pool")
		}
		c.IP = ip
	}
	if c.Pool != "" {
		return nil
	}

	// complete by statefulset
	if len(pod.OwnerReferences) > 0 {
		for i := range pod.OwnerReferences {
			if pod.OwnerReferences[i].Kind == constants.KindStatefulSet {
				if err := c.completeByStatefulSet(ctx, k8sClient, pod.OwnerReferences[i].Name, poolNs); err != nil {
					klog.Errorf("Failed to get pod %v specified ip or pool from statefulset, err: %v", podNsName, err)
					return err
				}
				return nil
			}
		}
	}

	return nil
}

func (c *NetConf) Valid() error {
	if c.Type == "" {
		return fmt.Errorf("must set Type")
	}

	if c.Type == v1alpha1.AllocateTypeCNIUsed || c.Type == v1alpha1.AllocateTypePod {
		if c.AllocateIdentify == "" {
			return fmt.Errorf("type %s must set AllocatedIdentify", c.Type)
		}
	}

	if c.Type == v1alpha1.AllocateTypePod || c.Type == v1alpha1.AllocateTypeStatefulSet {
		if c.K8sPodName == "" || c.K8sPodNs == "" {
			return fmt.Errorf("type %s must set K8sPodNs and K8sPodName", c.Type)
		}
	}

	if c.Type == v1alpha1.AllocateTypeStatefulSet {
		if c.Owner == "" {
			return fmt.Errorf("type %s must set Owner", c.Type)
		}
		if c.Pool == "" || c.IP == "" {
			return fmt.Errorf("type %s must set Pool and IP", c.Type)
		}
	}

	return nil
}

func (c *NetConf) getAllocateID() string {
	allocatedID := c.AllocateIdentify
	if c.Type == v1alpha1.AllocateTypePod || c.Type == v1alpha1.AllocateTypeStatefulSet {
		allocatedID = utils.GenAllocateIDFromPod(c.K8sPodNs, c.K8sPodName)
	}
	return allocatedID
}

func (c *NetConf) genAllocateInfo() v1alpha1.AllocateInfo {
	a := v1alpha1.AllocateInfo{
		Type:  c.Type,
		ID:    c.getAllocateID(),
		Owner: c.Owner,
	}
	if a.Type == v1alpha1.AllocateTypePod {
		a.CID = c.AllocateIdentify
	}
	return a
}

func (c *NetConf) podStr() string {
	return c.K8sPodNs + "/" + c.K8sPodName
}

func (c *NetConf) completeByStatefulSet(ctx context.Context, k8sClient client.Client, stsName string, poolNs string) error {
	sts := appsv1.StatefulSet{}
	stsNsName := types.NamespacedName{Namespace: c.K8sPodNs, Name: stsName}
	err := k8sClient.Get(ctx, stsNsName, &sts)
	if err != nil {
		klog.Errorf("Failed to get statefulset %v, err: %v", stsNsName, err)
		return err
	}
	if pool, ok := sts.Annotations[constants.IpamAnnotationPool]; ok {
		c.Pool = pool
	}

	var ipList []string
	if ips, ok := sts.Annotations[constants.IpamAnnotationIPList]; ok {
		if c.Pool == "" {
			klog.Errorf("statefulset %v can't only specify IP list but no pool", sts)
			return fmt.Errorf("can't only specify IP list but no pool")
		}
		ipList = strings.Split(ips, ",")
	} else {
		return nil
	}

	c.Type = v1alpha1.AllocateTypeStatefulSet
	c.Owner = utils.GenOwner(sts.GetNamespace(), sts.GetName())

	pool := v1alpha1.IPPool{}
	poolNsName := types.NamespacedName{Namespace: poolNs, Name: c.Pool}
	if err := k8sClient.Get(ctx, poolNsName, &pool); err != nil {
		klog.Errorf("Failed to get specified ippool %v by pod %s owner statefulset %v, err: %v", poolNsName, c.podStr(), stsNsName, err)
		return err
	}
	_, ipNet, _ := net.ParseCIDR(pool.Spec.CIDR)
	unUsedIPs := []string{}
	for _, ipStr := range ipList {
		ip := net.ParseIP(ipStr)
		// check if valid
		if ip == nil {
			klog.Errorf("Invalid ip %s", ipStr)
			continue
		}
		if !ipNet.Contains(ip) {
			klog.Errorf("IP %s doesn't in specified pool %v", ipStr, pool)
			continue
		}
		if _, exist := pool.Status.UsedIps[ipStr]; exist {
			continue
		}
		if allocateInfo, exist := pool.Status.AllocatedIPs[ipStr]; exist {
			if allocateInfo.Type == v1alpha1.AllocateTypeStatefulSet && allocateInfo.ID == c.getAllocateID() && allocateInfo.Owner == c.Owner {
				c.IP = ipStr
				return nil
			}
			continue
		}

		unUsedIPs = append(unUsedIPs, ipStr)
	}

	if len(unUsedIPs) > 0 {
		//nolint:gosec
		index := rand.Intn(len(unUsedIPs))
		c.IP = unUsedIPs[index]
		return nil
	}

	klog.Errorf("For statefulset %v ipList %v, no valid or unallocate ip in pool %v to allocate to pod %s", stsNsName, ipList, poolNsName, c.podStr())
	return fmt.Errorf("no valid or unallocate ip in statefulset %v ip list %v", stsNsName, ipList)
}
