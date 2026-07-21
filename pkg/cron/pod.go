package cron

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"

	"github.com/everoute/ipam/api/ipam/v1alpha1"
	"github.com/everoute/ipam/pkg/constants"
	"github.com/everoute/ipam/pkg/utils"
)

type podAllocationState int

const (
	podAllocationUsed podAllocationState = iota
	podAllocationPending
	podAllocationStale
)

func (c *CleanStaleIP) cleanStaleIPForPod(ctx context.Context) (hasPending bool, err error) {
	nextPending := make(map[podPendingKey]podPendingInfo)
	defer func() {
		if err != nil {
			c.podPending = make(map[podPendingKey]podPendingInfo)
			klog.Infof("Found errors during pod stale scan, clear pending items and will fast retry")
			hasPending = false
			return
		}
		c.podPending = nextPending
		klog.Infof("Update pod stale pending items to %v", c.podPending)
		hasPending = len(c.podPending) > 0
	}()

	ippools := v1alpha1.IPPoolList{}
	err = c.k8sClient.List(ctx, &ippools)
	if err != nil {
		klog.Errorf("Failed to list ippools, err: %v", err)
		return false, err
	}

	for i := range ippools.Items {
		ippool := ippools.Items[i]
		if ippool.Status.AllocatedIPs == nil {
			continue
		}
		poolNsName := types.NamespacedName{
			Namespace: ippool.GetNamespace(),
			Name:      ippool.GetName(),
		}
		for ip, allo := range ippool.Status.AllocatedIPs {
			if allo.Type != v1alpha1.AllocateTypePod {
				continue
			}
			if processErr := c.processPodAllocation(
				ctx, ip, allo, poolNsName, &ippools, nextPending,
			); processErr != nil {
				err = processErr
			}
		}
	}
	return false, err
}

func (c *CleanStaleIP) processPodAllocation(
	ctx context.Context, ip string, allo v1alpha1.AllocateInfo,
	poolNsName types.NamespacedName, ippools *v1alpha1.IPPoolList,
	nextPending map[podPendingKey]podPendingInfo,
) error {
	podNsName := utils.GetPodNsNameByAllocateID(allo.ID)
	if podNsName.Name == "" || podNsName.Namespace == "" {
		klog.Errorf(
			"Can't get pod namespace and name for allocate info %v and ip %s in ippool %v",
			allo, ip, poolNsName,
		)
		return nil
	}

	pendingKey := podPendingKey{Pool: poolNsName, IP: ip, Pod: podNsName}
	state, err := c.isIPUsedByPod(ctx, ip, podNsName, ippools)
	if err != nil {
		klog.Errorf(
			"Failed to get pod %v for clean stale ip in ippool %v, err: %v",
			podNsName, poolNsName, err,
		)
		return err
	}
	if state == podAllocationUsed {
		return nil
	}
	if state == podAllocationPending {
		if pending, ok := c.podPending[pendingKey]; ok && pending.AllocateInfo == allo {
			return c.cleanupStalePodAllocation(ctx, poolNsName, ip, allo, podNsName)
		}
		nextPending[pendingKey] = podPendingInfo{AllocateInfo: allo}
		klog.Infof(
			"IP %s for pod %s in ippool %s has a newer ippool allocation for pod status ip, "+
				"keep current allocation for a second confirmation",
			ip, podNsName, poolNsName,
		)
		return nil
	}
	return c.cleanupStalePodAllocation(ctx, poolNsName, ip, allo, podNsName)
}

func (c *CleanStaleIP) cleanupStalePodAllocation(
	ctx context.Context, poolNsName types.NamespacedName, ip string,
	allo v1alpha1.AllocateInfo, podNsName types.NamespacedName,
) error {
	klog.Infof(
		"IP %s with allocation %v is stale according to current pod state for pod %s, "+
			"begin to cleanup stale allocation from ippool %s",
		ip, allo, podNsName, poolNsName,
	)
	poolNow := v1alpha1.IPPool{}
	if err := c.k8sClient.Get(ctx, poolNsName, &poolNow); err != nil {
		klog.Errorf("Failed to get the latest ippool %s status, err: %s", poolNsName, err)
		return err
	}
	alloNew, ok := poolNow.Status.AllocatedIPs[ip]
	if !ok {
		klog.Infof("Stale ip %s doesn't in latest ippool %s, skip update ippool status", ip, poolNsName)
		return nil
	}
	if alloNew != allo {
		klog.Infof(
			"Allocate info of stale ip %s in ippool %s has updated, old is %v, new is %v, "+
				"skip update ippool status",
			ip, poolNsName, allo, alloNew,
		)
		return nil
	}
	delete(poolNow.Status.AllocatedIPs, ip)
	if poolNow.Status.Offset == constants.IPPoolOffsetFull {
		poolNow.Status.Offset = constants.IPPoolOffsetReset
	}
	poolNow.UpdateIPUsageCounter()
	if err := c.k8sClient.Status().Update(ctx, &poolNow); err != nil {
		klog.Errorf("Failed to cleanup ippool %s stale ip %s, update ippool status err: %s", poolNsName, ip, err)
		return err
	}
	klog.Infof(
		"Success to cleanup ippool %s stale ip %s with allocation %v for pod %s",
		poolNsName, ip, allo, podNsName,
	)
	return nil
}

func (c *CleanStaleIP) isIPUsedByPod(
	ctx context.Context, ip string, podNsName types.NamespacedName,
	ippools *v1alpha1.IPPoolList,
) (podAllocationState, error) {
	p := corev1.Pod{}
	err := c.k8sClient.Get(ctx, podNsName, &p)
	if err == nil {
		if podHasIP(&p, ip) {
			return podAllocationUsed, nil
		}
	} else if !errors.IsNotFound(err) {
		klog.Errorf("Failed to get pod %s, err: %s", podNsName, err)
		return podAllocationUsed, err
	}

	p = corev1.Pod{}
	err = c.k8sReader.Get(ctx, podNsName, &p)
	if err == nil {
		if podHasIP(&p, ip) {
			return podAllocationUsed, nil
		}

		if p.Status.PodIP == "" && len(p.Status.PodIPs) == 0 {
			klog.Infof("Can't get pod %s ip, keep ip %s allocate info in ippool", podNsName, ip)
			return podAllocationUsed, nil
		}
		currentPodID := utils.GenAllocateIDFromPod(podNsName.Namespace, podNsName.Name)
		allocations := getAllocatedPodByIPs(ippools, getPodIPs(&p))
		for podIP, allo := range allocations {
			if allo.ID == currentPodID {
				klog.Infof(
					"Pod %s still exists in phase %s and pod status ip %s has another ippool allocation "+
						"for the same pod, mark allocated ip %s as pending stale",
					podNsName, p.Status.Phase, podIP, ip,
				)
				return podAllocationPending, nil
			}
		}
		if len(allocations) > 0 {
			klog.Infof(
				"Pod %s still exists in phase %s, but none of status ip allocations %v belongs to "+
					"current pod, keep allocated ip %s",
				podNsName, p.Status.Phase, allocations, ip,
			)
			return podAllocationUsed, nil
		}
		klog.Infof(
			"Pod %s still exists in phase %s but current pod ip %s and pod ips %v don't match "+
				"allocated ip %s, and pod status ips aren't allocated in ippool, keep allocation",
			podNsName, p.Status.Phase, p.Status.PodIP, p.Status.PodIPs, ip,
		)
		return podAllocationUsed, nil
	}
	if !errors.IsNotFound(err) {
		klog.Errorf("Failed to get pod %s, err: %s", podNsName, err)
		return podAllocationUsed, err
	}
	klog.Infof("Pod %s is not found from uncached reader, mark allocated ip %s as stale", podNsName, ip)
	return podAllocationStale, nil
}

func podHasIP(pod *corev1.Pod, ip string) bool {
	if pod.Status.PodIP == ip {
		return true
	}
	for _, podIP := range pod.Status.PodIPs {
		if podIP.IP == ip {
			return true
		}
	}
	return false
}

func getAllocatedPodByIPs(ippools *v1alpha1.IPPoolList, ips []string) map[string]v1alpha1.AllocateInfo {
	res := make(map[string]v1alpha1.AllocateInfo)
	for _, ip := range ips {
		for i := range ippools.Items {
			allo, ok := ippools.Items[i].Status.AllocatedIPs[ip]
			if !ok {
				continue
			}
			res[ip] = allo
			break
		}
	}
	return res
}

func getPodIPs(pod *corev1.Pod) []string {
	res := make([]string, 0, len(pod.Status.PodIPs)+1)
	if pod.Status.PodIP != "" {
		res = append(res, pod.Status.PodIP)
	}
	for _, podIP := range pod.Status.PodIPs {
		if podIP.IP == "" || podIP.IP == pod.Status.PodIP {
			continue
		}
		res = append(res, podIP.IP)
	}
	return res
}
