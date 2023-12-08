package v1alpha1

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
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

	// set ippool spec immutable
	if !reflect.DeepEqual(r.Spec, old.(*IPPool).Spec) {
		return nil, fmt.Errorf("IPPool spec is immutable")
	}

	poollist := IPPoolList{}
	err := poolsReader.List(context.Background(), &poollist)
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
