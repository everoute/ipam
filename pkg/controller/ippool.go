package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
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
)

type PoolController struct {
	client.Client
}

func (p *PoolController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.Infof("IPPool controller receive ippool %s", req.NamespacedName)
	pool := v1alpha1.IPPool{}
	err := p.Get(ctx, req.NamespacedName, &pool)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		klog.Errorf("Failed to get ippool %s, err: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	if pool.Status.Offset == constants.IPPoolOffsetReset {
		return ctrl.Result{}, nil
	}

	pool.Status.Offset = constants.IPPoolOffsetReset
	if err := p.Status().Update(ctx, &pool); err != nil {
		klog.Errorf("Failed to update ippool %s status, err: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}
	klog.Infof("Success update ippool %s offset to %d", req.NamespacedName, constants.IPPoolOffsetReset)
	return ctrl.Result{}, nil
}

func (p *PoolController) SetupWithManager(mgr ctrl.Manager) error {
	if mgr == nil {
		return fmt.Errorf("can't setup with nil mgr")
	}

	c, err := controller.New("ippool controller", mgr, controller.Options{
		Reconciler: p,
	})
	if err != nil {
		return err
	}

	return c.Watch(source.Kind(mgr.GetCache(), &v1alpha1.IPPool{}), &handler.EnqueueRequestForObject{}, predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool { return false },
		UpdateFunc: p.predicateUpdate,
		DeleteFunc: func(event.DeleteEvent) bool { return false },
	})
}

func (p *PoolController) predicateUpdate(e event.UpdateEvent) bool {
	newObj, newOk := e.ObjectNew.(*v1alpha1.IPPool)
	oldObj, oldOk := e.ObjectOld.(*v1alpha1.IPPool)
	if !newOk || !oldOk {
		klog.Errorf("Can't transform object to ippool")
		return false
	}

	if newObj.Status.Offset == 0 {
		return false
	}

	if newObj.Spec.CIDR != oldObj.Spec.CIDR {
		return true
	}

	if newObj.Spec.End != oldObj.Spec.End {
		return true
	}

	if newObj.Spec.Start != oldObj.Spec.Start {
		return true
	}

	if len(newObj.Spec.Except) != len(oldObj.Spec.Except) {
		return true
	}

	if len(newObj.Spec.Except) == 0 {
		return false
	}

	newExpect := sets.New[string]()
	newExpect.Insert(newObj.Spec.Except...)
	oldExpect := sets.New[string]()
	oldExpect.Insert(oldObj.Spec.Except...)
	return !newExpect.Equal(oldExpect)
}
