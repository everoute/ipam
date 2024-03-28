package v1alpha1

import (
	"net"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/everoute/ipam/pkg/utils"
)

// +genclient
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

type IPPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec contains description of the IPPool
	Spec IPPoolSpec `json:"spec"`

	// Status is the current state of the IPPool
	Status IPPoolStatus `json:"status,omitempty"`
}

// IPPoolSpec provides the specification of an IPPool
type IPPoolSpec struct {
	// CIDR is an IP net string, e.g. 192.168.1.0/24
	// IP will allocated from CIDR
	//nolint: lll
	// +kubebuilder:validation:Pattern="^(?:(?:[0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}(?:[0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\/([1-9]|[1-2]\\d|3[0-2])$"
	// +optional
	CIDR string `json:"cidr,omitempty"`
	// Except is IP net string array, e.g. [192.168.1.0/24, 192.168.2.1/32], when allocate ip to Pod, ip in Except won't be allocated
	// +optional
	Except []string `json:"except,omitempty"`

	// Start is the start ip of an ip range, required End
	// +kubebuilder:validation:Pattern="^(?:(?:[0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}(?:[0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$"
	// +optional
	Start string `json:"start,omitempty"`

	// End is the end ip of an ip range, required Start
	// +kubebuilder:validation:Pattern="^(?:(?:[0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}(?:[0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$"
	// +optional
	End string `json:"end,omitempty"`

	// Subnet is the total L2 network,
	//nolint: lll
	// +kubebuilder:validation:Pattern="^(?:(?:[0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}(?:[0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\/([1-9]|[1-2]\\d|3[0-2])$"
	Subnet string `json:"subnet"`
	// Gateway must a valid IP in Subnet
	//nolint: lll
	// +kubebuilder:validation:Pattern="^(((([1]?\\d)?\\d|2[0-4]\\d|25[0-5])\\.){3}(([1]?\\d)?\\d|2[0-4]\\d|25[0-5]))|([\\da-fA-F]{1,4}(\\:[\\da-fA-F]{1,4}){7})|(([\\da-fA-F]{1,4}:){0,5}::([\\da-fA-F]{1,4}:){0,5}[\\da-fA-F]{1,4})$"
	Gateway string `json:"gateway"`
	Private bool   `json:"private,omitempty"`
}

// IPPoolStatus describe the current state of the IPPool
type IPPoolStatus struct {
	// UsedIps can't delete to compatible with upgrade scenarios
	UsedIps map[string]string `json:"usedips,omitempty"`
	// AllocatedIPs is ip and allocated infos
	AllocatedIPs map[string]AllocateInfo `json:"allocatedips,omitempty"`
	// Offset stores the current read pointer
	// -1 means this pool is full
	Offset int64 `json:"offset,omitempty"`
}

type AllocateInfo struct {
	// Type=pod, ID=podns/name
	ID string `json:"id"`
	// Type=pod, CID=containerID
	CID  string       `json:"cid,omitempty"`
	Type AllocateType `json:"type"`
	// Type=statefulset, owner=statefulsetns/name
	Owner string `json:"owner,omitempty"`
}

type AllocateType string

const (
	AllocateTypeCNIUsed     AllocateType = "cniused"
	AllocateTypePod         AllocateType = "pod"
	AllocateTypeStatefulSet AllocateType = "statefulset"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// IPPoolList contains a list of IPPool
type IPPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IPPool `json:"items"`
}

func (r *IPPool) StartIP() net.IP {
	if r.Spec.Start != "" {
		return net.ParseIP(r.Spec.Start)
	}

	_, ipNet, _ := net.ParseCIDR(r.Spec.CIDR)
	return utils.FirstIP(ipNet)
}

func (r *IPPool) EndIP() net.IP {
	if r.Spec.End != "" {
		return net.ParseIP(r.Spec.End)
	}

	_, ipNet, _ := net.ParseCIDR(r.Spec.CIDR)
	return utils.LastIP(ipNet)
}

func (r *IPPool) Length() int64 {
	if r.Spec.CIDR != "" {
		_, ipNet, _ := net.ParseCIDR(r.Spec.CIDR)
		ones, bits := ipNet.Mask.Size()
		hostBits := bits - ones
		return 1 << hostBits
	}

	return int64(utils.Ipv4ToUint32(net.ParseIP(r.Spec.End)) - utils.Ipv4ToUint32(net.ParseIP(r.Spec.Start)) + 1)
}

func (r *IPPool) Contains(ip net.IP) bool {
	startIPN := utils.Ipv4ToUint32(r.StartIP())
	endIPN := utils.Ipv4ToUint32(r.EndIP())
	ipN := utils.Ipv4ToUint32(ip)
	if ipN < startIPN {
		return false
	}
	if ipN > endIPN {
		return false
	}

	for i := range r.Spec.Except {
		_, ipNet, _ := net.ParseCIDR(r.Spec.Except[i])
		if ipNet.Contains(ip) {
			return false
		}
	}
	return true
}
