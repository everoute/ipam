package cron

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/everoute/ipam/api/ipam/v1alpha1"
	"github.com/everoute/ipam/pkg/constants"
	"github.com/everoute/ipam/pkg/utils"
)

var _ ProcessFun = cleanStaleIPForPod

func cleanStaleIPForPod(ctx context.Context, k8sClient client.Client, k8sReader client.Reader) {
	ippools := v1alpha1.IPPoolList{}
	err := k8sClient.List(ctx, &ippools)
	if err != nil {
		klog.Errorf("Failed to list ippools, err: %v", err)
		return
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
			podNsName := utils.GetPodNsNameByAllocateID(allo.ID)
			if podNsName.Name == "" || podNsName.Namespace == "" {
				klog.Errorf("Can't get pod namespace and name for allocate info %v and ip %s in ippool %v", allo, ip, poolNsName)
				continue
			}
			used, err := isIPUsedByPod(ctx, ip, podNsName, k8sClient, k8sReader)
			if err != nil {
				klog.Errorf("Failed to get pod %v for clean stale ip in ippool %v, err: %v", podNsName, poolNsName, err)
				continue
			}
			if used {
				continue
			}
			poolNow := v1alpha1.IPPool{}
			if err := k8sClient.Get(ctx, poolNsName, &poolNow); err != nil {
				klog.Errorf("Failed to get the latest ippool %s status, err: %s", poolNsName, err)
				continue
			}
			alloNew, ok := poolNow.Status.AllocatedIPs[ip]
			if !ok {
				klog.Infof("Stale ip %s doesn't in latest ippool %s, skip update ippool status", ip, poolNsName)
				continue
			}
			if alloNew != allo {
				klog.Infof("Allocate info of stale ip %s in ippool %s has updated, old is %v, new is %v, skip update ippool status", ip, poolNsName, allo, alloNew)
				continue
			}
			klog.Infof("IP %s for pod %s is stale, begin to cleanup from ippool %s", ip, podNsName, poolNsName)
			delete(poolNow.Status.AllocatedIPs, ip)
			if poolNow.Status.Offset == constants.IPPoolOffsetFull {
				poolNow.Status.Offset = constants.IPPoolOffsetReset
			}
			poolNow.UpdateIPUsageCounter()
			err = k8sClient.Status().Update(ctx, &poolNow)
			if err != nil {
				klog.Errorf("Failed to cleanup ippool %s stale ip %s, update ippool status err: %s", poolNsName, ip, err)
			}
			klog.Infof("Success to cleanup ippool %s stale ip %s for pod %s", poolNsName, ip, podNsName)
		}
	}
}

func isIPUsedByPod(ctx context.Context, ip string, podNsName types.NamespacedName, k8sClient client.Client, k8sReader client.Reader) (bool, error) {
	p := corev1.Pod{}
	err := k8sClient.Get(ctx, podNsName, &p)
	if err == nil {
		if p.Status.PodIP == ip {
			return true, nil
		}
	} else {
		if !errors.IsNotFound(err) {
			klog.Errorf("Failed to get pod %s, err: %s", podNsName, err)
			return true, err
		}
	}

	err2 := k8sReader.Get(ctx, podNsName, &corev1.Pod{})
	if err2 == nil {
		if p.Status.PodIP == ip {
			return true, nil
		}
		if p.Status.PodIP == "" {
			klog.Infof("Can't get pod %s ip, keep ip %s allocate info in ippool", podNsName, ip)
			return true, nil
		}
		return false, nil
	}
	if !errors.IsNotFound(err2) {
		klog.Errorf("Failed to get pod %v, err: %v", podNsName, err)
		return true, err
	}
	return false, nil
}
