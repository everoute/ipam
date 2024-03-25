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
	pool := r.Namespace + "/" + r.Name
	klog.Infof("validate create ippool name is %s", pool)
	if err := r.validateSpec(); err != nil {
		klog.Errorf("invalid ippool %s for create, err: %s", pool, err)
		return nil, err
	}

	poollist := IPPoolList{}
	err := poolsReader.List(context.Background(), &poollist)
	if err != nil {
		return nil, fmt.Errorf("err in list ippools: %s", err.Error())
	}
	return nil, ValidatePool(poollist, *r, "")
}

func (r *IPPool) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	pool := r.Namespace + "/" + r.Name
	klog.Infof("validate update ippool name is %s", pool)
	if reflect.DeepEqual(r.Spec, old.(*IPPool).Spec) {
		return nil, nil
	}

	if err := r.validateSpec(); err != nil {
		klog.Errorf("Invalid ippool %s for update, err: %s", pool, err)
		return nil, err
	}

	oldSpec := old.(*IPPool).Spec
	if r.Spec.Gateway != oldSpec.Gateway {
		return nil, fmt.Errorf("can't modify IPPool %s gateway, try to update gateway from %s to %s", pool, oldSpec.Gateway, r.Spec.Gateway)
	}
	if r.Spec.Subnet != oldSpec.Subnet {
		return nil, fmt.Errorf("can't modify IPPool %s subnet, try to update subnet from %s to %s", pool, oldSpec.Subnet, r.Spec.Subnet)
	}

	if err := r.validateAllocateIPs(); err != nil {
		klog.Errorf("IPPool %s must contains all allocate ip when update, err: %s", pool, err)
		return nil, err
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
	if len(r.Status.AllocatedIPs) != 0 || len(r.Status.UsedIps) != 0 {
		return nil, fmt.Errorf("IPPool has allocated IP, can't delete")
	}
	_ = ValidatePool(IPPoolList{Items: []IPPool{}}, IPPool{}, r.Namespace+`/`+r.Name)
	return nil, nil
}

//nolint:gocognit
func (r *IPPool) validateSpec() error {
	_, subnet, err := net.ParseCIDR(r.Spec.Subnet)
	if err != nil {
		return fmt.Errorf("failed to parse subnet %s, err: %s", r.Spec.Subnet, err)
	}
	gateway := net.ParseIP(r.Spec.Gateway)
	if gateway == nil {
		return fmt.Errorf("invalid ippool gateway %s", r.Spec.Gateway)
	}
	if !subnet.Contains(gateway) {
		return fmt.Errorf("gateway %s doesn't in subnet %s", r.Spec.Gateway, r.Spec.Subnet)
	}
	if gateway.Equal(subnet.IP) {
		return fmt.Errorf("gateway %s can't be subnet %s network number", r.Spec.Gateway, r.Spec.Subnet)
	}

	if r.Spec.CIDR != "" {
		if r.Spec.Start != "" {
			return fmt.Errorf("can't set spec.cidr and spec.start at the same time")
		}

		if r.Spec.End != "" {
			return fmt.Errorf("can't set spec.cidr and spec.end at the same time")
		}

		_, _, err := net.ParseCIDR(r.Spec.CIDR)
		if err != nil {
			return fmt.Errorf("parse ippool cidr %s failed, err: %s", r.Spec.CIDR, err)
		}

		for i := range r.Spec.Except {
			if _, _, err := net.ParseCIDR(r.Spec.Except[i]); err != nil {
				return fmt.Errorf("parse spec.except %s, err: %s", r.Spec.Except[i], err)
			}
		}
	} else {
		if len(r.Spec.Except) != 0 {
			return fmt.Errorf("can't set spec.except without spec.cidr")
		}

		if r.Spec.Start != "" && r.Spec.End != "" {
			startIP := net.ParseIP(r.Spec.Start)
			if startIP == nil || startIP.To4() == nil {
				return fmt.Errorf("invalid start ipv4 %s", r.Spec.Start)
			}
			endIP := net.ParseIP(r.Spec.End)
			if endIP == nil || endIP.To4() == nil {
				return fmt.Errorf("invalid end ipv4 %s", r.Spec.End)
			}

			if utils.IPBiggerThan(startIP, endIP) {
				return fmt.Errorf("start ip %s must smaller or equal to end ip %s", r.Spec.Start, r.Spec.End)
			}
		} else {
			return fmt.Errorf("must set spec.start and spec.end when doesn't set sepc.cidr")
		}
	}

	if !subnet.Contains(r.StartIP()) || !subnet.Contains(r.EndIP()) {
		return fmt.Errorf("ippool's ip must all in subnet %s", r.Spec.Subnet)
	}

	return nil
}

func (r *IPPool) validateAllocateIPs() error {
	if len(r.Status.AllocatedIPs) == 0 && len(r.Status.UsedIps) == 0 {
		return nil
	}

	if err := r.validateAllocateIPsForIPBlock(); err != nil {
		return err
	}

	return r.validateAllocateIPsForStartEnd()
}

func (r *IPPool) validateAllocateIPsForIPBlock() error {
	if r.Spec.CIDR == "" {
		return nil
	}
	_, ipNet, err := net.ParseCIDR(r.Spec.CIDR)
	if err != nil {
		return fmt.Errorf("parse ippool cidr %s failed, err: %s", r.Spec.CIDR, err)
	}
	exceptNets := []*net.IPNet{}
	for i := range r.Spec.Except {
		_, ipNet, err := net.ParseCIDR(r.Spec.Except[i])
		if err != nil {
			return fmt.Errorf("failed to parse ippool except cidr %s, err: %s", r.Spec.Except[i], err)
		}
		exceptNets = append(exceptNets, ipNet)
	}

	for k := range r.Status.AllocatedIPs {
		ip := net.ParseIP(k)
		if ip == nil {
			return fmt.Errorf("invalid allocate ip %s", k)
		}
		if !ipNet.Contains(ip) {
			return fmt.Errorf("ippool must contain has been allocated ip %s", k)
		}
		for i := range exceptNets {
			if exceptNets[i].Contains(ip) {
				return fmt.Errorf("ippool must contain has been allocated ip %s", k)
			}
		}
	}

	for k := range r.Status.UsedIps {
		ip := net.ParseIP(k)
		if ip == nil {
			return fmt.Errorf("invalid allocate ip %s", k)
		}
		if !ipNet.Contains(ip) {
			return fmt.Errorf("ippool must contain has been allocated ip %s", k)
		}
		for i := range exceptNets {
			if exceptNets[i].Contains(ip) {
				return fmt.Errorf("ippool must contain has been allocated ip %s", k)
			}
		}
	}

	return nil
}

//nolint:gocognit
func (r *IPPool) validateAllocateIPsForStartEnd() error {
	if r.Spec.Start == "" {
		return nil
	}

	startIP := net.ParseIP(r.Spec.Start)
	if startIP == nil || startIP.To4() == nil {
		return fmt.Errorf("invalid ippool start ipv4 %s", r.Spec.Start)
	}
	startIPn := utils.Ipv4ToUint32(startIP)

	endIP := net.ParseIP(r.Spec.End)
	if endIP == nil || endIP.To4() == nil {
		return fmt.Errorf("invalid ippool end ipv4 %s", r.Spec.End)
	}
	endIPn := utils.Ipv4ToUint32(endIP)

	for k := range r.Status.AllocatedIPs {
		ip := net.ParseIP(k)
		if ip == nil || ip.To4() == nil {
			return fmt.Errorf("invalid allocate ipv4 %s", k)
		}
		ipn := utils.Ipv4ToUint32(ip)
		if ipn < startIPn || ipn > endIPn {
			return fmt.Errorf("ippool must contain has been allocated ip %s", k)
		}
	}

	for k := range r.Status.UsedIps {
		ip := net.ParseIP(k)
		if ip == nil || ip.To4() == nil {
			return fmt.Errorf("invalid allocate ipv4 %s", k)
		}
		ipn := utils.Ipv4ToUint32(ip)
		if ipn < startIPn || ipn > endIPn {
			return fmt.Errorf("ippool must contain has been allocated ip %s", k)
		}
	}

	return nil
}
