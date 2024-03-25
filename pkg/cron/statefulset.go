package cron

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/everoute/ipam/api/ipam/v1alpha1"
	"github.com/everoute/ipam/pkg/constants"
	"github.com/everoute/ipam/pkg/utils"
)

var _ ProcessFun = cleanStaleIPForStatefulSet

func cleanStaleIPForStatefulSet(ctx context.Context, k8sClient client.Client, k8sReader client.Reader) {
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
			if allo.Type != v1alpha1.AllocateTypeStatefulSet {
				continue
			}
			stsNsName := utils.GetNsNameByAllocateOwner(allo.Owner)
			if stsNsName.Name == "" || stsNsName.Namespace == "" {
				klog.Errorf("Can't get StatefulSet namespace and name for allocate info %v and ip %s in ippool %v", allo, ip, poolNsName)
				continue
			}
			err := k8sReader.Get(ctx, stsNsName, &appsv1.StatefulSet{})
			if err == nil {
				continue
			}
			if !errors.IsNotFound(err) {
				klog.Errorf("Failed to get StatefulSet %v for clean stale ip in ippool %v, err: %v", stsNsName, poolNsName, err)
				continue
			}
			klog.Infof("IP %s for pod %v owned by StatefulSet %v is stale, will cleanup from ippool %v", ip, allo.ID, stsNsName, poolNsName)
			delIPs = append(delIPs, ip)
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
			klog.Errorf("Failed to update ippool %s status, err: %s", poolNsName, err)
		}
	}
}
