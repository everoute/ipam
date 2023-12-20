package v1alpha1

import (
	"context"
	"fmt"
	"net"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/everoute/ipam/pkg/utils"
)

var poolsReader client.Client

func (r *IPPool) SetupWebhookWithManager(mgr ctrl.Manager) error {
	poolsReader = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

var _ admission.Validator = &IPPool{}

func (r *IPPool) ValidateCreate() (admission.Warnings, error) {
	klog.Infoln("validate create ippool name is ", r.Namespace+`/`+r.Name)
	poollist := IPPoolList{}
	err := poolsReader.List(context.Background(), &poollist)
	if err != nil {
		return nil, fmt.Errorf("err in list ippools: %s", err.Error())
	}
	return nil, ValidatePool(poollist, *r, "")
}

func (r *IPPool) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	klog.Infoln("validate update ippool name is ", r.Namespace+`/`+r.Name)

	if reflect.DeepEqual(r.Spec, old.(*IPPool).Spec) {
		return nil, nil
	}

	oldSpec := old.(*IPPool).Spec
	if r.Spec.Gateway != oldSpec.Gateway {
		return nil, fmt.Errorf("can't modify IPPool gateway, try to update gateway from %s to %s", oldSpec.Gateway, r.Spec.Gateway)
	}
	if r.Spec.Subnet != oldSpec.Subnet {
		return nil, fmt.Errorf("can't modify IPPool subnet, try to update subnet from %s to %s", oldSpec.Subnet, r.Spec.Subnet)
	}

	_, oldNet, err := net.ParseCIDR(oldSpec.CIDR)
	if err != nil {
		return nil, fmt.Errorf("failed to parse old CIDR %s", oldSpec.CIDR)
	}
	_, newNet, err := net.ParseCIDR(r.Spec.CIDR)
	if err != nil {
		return nil, fmt.Errorf("failed to parse new CIDR %s", r.Spec.CIDR)
	}

	if !newNet.Contains(utils.FirstIP(oldNet)) || !newNet.Contains(utils.LastIP(oldNet)) {
		return nil, fmt.Errorf("the new CIDR %s must contain the old CIDR %s", r.Spec.CIDR, oldSpec.CIDR)
	}

	poollist := IPPoolList{}
	err = poolsReader.List(context.Background(), &poollist)
	if err != nil {
		return nil, fmt.Errorf("err in list ippools: %s", err.Error())
	}
	return nil, ValidatePool(poollist, *r, r.Namespace+`/`+r.Name)
}

func (r *IPPool) ValidateDelete() (admission.Warnings, error) {
	klog.Infoln("validate delete ippool name is ", r.Namespace+`/`+r.Name)
	_ = ValidatePool(IPPoolList{Items: []IPPool{}}, IPPool{}, r.Namespace+`/`+r.Name)
	return nil, nil
}
