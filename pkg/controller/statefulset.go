package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/everoute/ipam/api/ipam/v1alpha1"
	"github.com/everoute/ipam/pkg/constants"
	"github.com/everoute/ipam/pkg/utils"
)

var stsPredicate predicate.Predicate = predicate.Funcs{
	CreateFunc: func(event.CreateEvent) bool {
		return false
	},
	UpdateFunc: func(event.UpdateEvent) bool {
		return false
	},
	DeleteFunc: func(event.DeleteEvent) bool {
		return true
	},
	GenericFunc: func(event.GenericEvent) bool {
		return false
	},
}

type STSReconciler struct {
	client.Client
}

func (s *STSReconciler) SetUpWithManager(mgr ctrl.Manager) error {
	if mgr == nil {
		return fmt.Errorf("can't setup with nil manager")
	}

	c, err := controller.New("statefulset-controller", mgr, controller.Options{
		Reconciler: s,
	})
	if err != nil {
		return err
	}

	err = c.Watch(source.Kind(mgr.GetCache(), &appsv1.StatefulSet{}), &handler.EnqueueRequestForObject{}, stsPredicate)
	return err
}

func (s *STSReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.Infof("Received StatefulSet %v reconcile", req.NamespacedName)
	err := s.Client.Get(ctx, req.NamespacedName, &appsv1.StatefulSet{})
	if err == nil {
		klog.Infof("Success get StatefulSet %v, return", req.NamespacedName)
		return ctrl.Result{}, nil
	}
	if !errors.IsNotFound(err) {
		klog.Errorf("Failed to get StatefulSet %v, err: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	pools := v1alpha1.IPPoolList{}
	if err := s.Client.List(ctx, &pools); err != nil {
		klog.Errorf("Failed to list IPPools, err: %v", err)
		return ctrl.Result{}, err
	}

	failed := false
	for i := range pools.Items {
		pool := pools.Items[i]
		if pool.Status.AllocatedIPs == nil {
			continue
		}
		releaseIPs := []string{}
		for ip, a := range pool.Status.AllocatedIPs {
			if a.Type != v1alpha1.AllocateTypeStatefulSet {
				continue
			}
			if a.Owner != utils.GenOwner(req.Namespace, req.Name) {
				continue
			}
			releaseIPs = append(releaseIPs, ip)
		}
		if len(releaseIPs) > 0 {
			for _, ip := range releaseIPs {
				delete(pool.Status.AllocatedIPs, ip)
			}
			if pool.Status.Offset == constants.IPPoolOffsetFull {
				pool.Status.Offset = constants.IPPoolOffsetReset
			}
			poolNsName := pool.GetNamespace() + "/" + pool.GetName()
			if err := s.Client.Status().Update(ctx, &pool); err != nil {
				failed = true
				klog.Errorf("Failed to release ip-list %v of deleted StatefulSet %v in ippool %s, err: %v", releaseIPs, req.NamespacedName, poolNsName, err)
			}
			klog.Infof("Success release ip-list %v of deleted StatefulSet %v in ippool %s", releaseIPs, req.NamespacedName, poolNsName)
		}
	}

	if failed {
		return ctrl.Result{}, fmt.Errorf("failed to release ip-list of deleted StatefulSet %v in all ippool", req.NamespacedName)
	}

	return ctrl.Result{}, nil
}
