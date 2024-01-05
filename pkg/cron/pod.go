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
		delIPs := make([]string, 0)
		for ip, allo := range ippool.Status.AllocatedIPs {
			if allo.Type != v1alpha1.AllocateTypePod {
				continue
			}
			podNsName := utils.GetPodNsNameByAllocateID(allo.ID)
			if podNsName.Name == "" || podNsName.Namespace == "" {
				klog.Errorf("Can't get pod namespace and name for allocate info %v and ip %s in ippool %v", allo, ip, poolNsName)
				continue
			}
			exists, err := isPodExist(ctx, podNsName, k8sClient, k8sReader)
			if err != nil {
				klog.Errorf("Failed to get pod %v for clean stale ip in ippool %v, err: %v", podNsName, poolNsName, err)
				continue
			}
			if !exists {
				klog.Infof("IP %s for pod %v is stale, will cleanup from ippool %v", ip, podNsName, poolNsName)
				delIPs = append(delIPs, ip)
			}
		}
		if len(delIPs) == 0 {
			continue
		}
		for _, ip := range delIPs {
			delete(ippool.Status.AllocatedIPs, ip)
		}
		if ippool.Status.Offset == constants.IPPoolOffsetFull {
			ippool.Status.Offset = constants.IPPoolOffsetReset
		}
		err := k8sClient.Status().Update(ctx, &ippool)
		if err != nil {
			klog.Errorf("Failed to update ippool %v status, err: %v", ippool, err)
		}
	}
}

func isPodExist(ctx context.Context, podNsName types.NamespacedName, k8sClient client.Client, k8sReader client.Reader) (bool, error) {
	err := k8sClient.Get(ctx, podNsName, &corev1.Pod{})
	if err == nil {
		return true, nil
	}
	if !errors.IsNotFound(err) {
		klog.Errorf("Failed to get pod %v, err: %v", podNsName, err)
		return true, err
	}

	err2 := k8sReader.Get(ctx, podNsName, &corev1.Pod{})
	if err2 == nil {
		return true, nil
	}
	if !errors.IsNotFound(err2) {
		klog.Errorf("Failed to get pod %v, err: %v", podNsName, err)
		return true, err
	}
	return false, nil
}
