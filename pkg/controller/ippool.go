package controller

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/mikioh/ipaddr"
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

	pool.Status.Offset = constants.IPPoolOffsetReset

	// re-calculate ip counters
	pool.Status.TotalCount = p.calAvailableIPs(pool.Spec)
	pool.UpdateIPUsageCounter()
	if err := p.Status().Update(ctx, &pool); err != nil {
		klog.Errorf("Failed to update ippool %s status, err: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}
	pool.Status.UsedIps = nil
	pool.Status.AllocatedIPs = nil
	klog.Infof("Success update ippool %s to %+v %+v", req.NamespacedName, pool.Spec, pool.Status)
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
		CreateFunc: func(event.CreateEvent) bool { return true },
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

	if newObj.Spec.CIDR != oldObj.Spec.CIDR {
		return true
	}

	if newObj.Spec.End != oldObj.Spec.End {
		return true
	}

	if newObj.Spec.Start != oldObj.Spec.Start {
		return true
	}

	newExpect := sets.New(newObj.Spec.Except...)
	oldExpect := sets.New(oldObj.Spec.Except...)
	return !newExpect.Equal(oldExpect)
}

func (p *PoolController) calAvailableIPs(spec v1alpha1.IPPoolSpec) int64 {
	allPrefix := []ipaddr.Prefix{}
	exceptPrefix := []ipaddr.Prefix{}
	if spec.CIDR != "" {
		allPrefix = append(allPrefix, ip2Prefix(spec.CIDR))
	} else {
		allPrefix = append(allPrefix,
			ipaddr.Summarize(net.ParseIP(spec.Start), net.ParseIP(spec.End))...)
	}

	for _, item := range spec.Except {
		exceptPrefix = append(exceptPrefix, ip2Prefix(item))
	}
	_, subnetCIDR, _ := net.ParseCIDR(spec.Subnet)
	subnetPrefix := ipaddr.NewPrefix(subnetCIDR)
	// except first ip of subnet
	exceptPrefix = append(exceptPrefix, ip2Prefix(subnetCIDR.IP.String()))
	// except last ip of subnet
	exceptPrefix = append(exceptPrefix, ip2Prefix(subnetPrefix.Last().String()))
	// except gateway ip
	exceptPrefix = append(exceptPrefix, ip2Prefix(spec.Gateway))

	validPrefix := ipListDifference(allPrefix, exceptPrefix)
	var cnt int64
	for _, item := range validPrefix {
		cnt += item.NumNodes().Int64()
	}
	return cnt
}

func ip2Prefix(str string) ipaddr.Prefix {
	if !strings.Contains(str, "/") {
		str += "/32"
	}
	_, cidr, _ := net.ParseCIDR(str)
	return *ipaddr.NewPrefix(cidr)
}

func ipListDifference(newIPs, oldIPs []ipaddr.Prefix) []ipaddr.Prefix {
	var prefixTarget []ipaddr.Prefix
	for _, newIP := range newIPs {
		prefixCur := []ipaddr.Prefix{newIP}
		prefixNext := []ipaddr.Prefix{}
		for index := range oldIPs {
			for _, tmp := range prefixCur {
				if tmp.Equal(&oldIPs[index]) {
					continue
				}
				if tmp.Contains(&oldIPs[index]) {
					prefixNext = append(prefixNext, tmp.Exclude(&oldIPs[index])...)
				} else {
					prefixNext = append(prefixNext, tmp)
				}
			}
			prefixCur = append([]ipaddr.Prefix{}, prefixNext...)
			prefixNext = []ipaddr.Prefix{}
		}
		prefixTarget = append(prefixTarget, prefixCur...)
	}
	return ipaddr.Aggregate(prefixTarget)
}
