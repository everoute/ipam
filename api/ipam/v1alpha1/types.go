package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	CIDR string `json:"cidr"`
	// Subnet is the total L2 network,
	//nolint: lll
	// +kubebuilder:validation:Pattern="^(?:(?:[0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}(?:[0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\/([1-9]|[1-2]\\d|3[0-2])$"
	Subnet string `json:"subnet"`
	// Gateway must a valid IP in Subnet
	//nolint: lll
	// +kubebuilder:validation:Pattern="^(((([1]?\\d)?\\d|2[0-4]\\d|25[0-5])\\.){3}(([1]?\\d)?\\d|2[0-4]\\d|25[0-5]))|([\\da-fA-F]{1,4}(\\:[\\da-fA-F]{1,4}){7})|(([\\da-fA-F]{1,4}:){0,5}::([\\da-fA-F]{1,4}:){0,5}[\\da-fA-F]{1,4})$"
	Gateway string `json:"gateway"`
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
	ID string `json:"id"`
	// Type=pod, ID=podns/name
	Type AllocateType `json:"type,omitempty"`
}

type AllocateType string

const (
	AllocatedTypeCNIUsed AllocateType = "cniused"
	AllocatedTypePod     AllocateType = "pod"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// IPPoolList contains a list of IPPool
type IPPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IPPool `json:"items"`
}
