package ipam

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/everoute/ipam/api/ipam/v1alpha1"
)

func TestGenAllocateInfo(t *testing.T) {
	tests := []struct {
		name string
		conf NetConf
		exp  v1alpha1.AllocateInfo
	}{
		{
			name: "type pod",
			conf: NetConf{
				Type:             v1alpha1.AllocatedTypePod,
				AllocateIdentify: "containerid",
				K8sPodName:       "podname",
				K8sPodNs:         "podns",
			},
			exp: v1alpha1.AllocateInfo{
				Type: v1alpha1.AllocatedTypePod,
				ID:   "podns/podname",
			},
		},
		{
			name: "type cniused",
			conf: NetConf{
				Type:             v1alpha1.AllocatedTypeCNIUsed,
				AllocateIdentify: "identify",
			},
			exp: v1alpha1.AllocateInfo{
				Type: v1alpha1.AllocatedTypeCNIUsed,
				ID:   "identify",
			},
		},
	}

	for _, item := range tests {
		res := item.conf.genAllocateInfo()
		if res != item.exp {
			t.Errorf("test %s failed, expect is %v, real is %v", item.name, item.exp, res)
		}
	}
}

func makeAllocateStatus(input ...string) map[string]v1alpha1.AllocateInfo {
	res := make(map[string]v1alpha1.AllocateInfo)
	i := 0
	for i < len(input)-2 {
		res[input[i]] = v1alpha1.AllocateInfo{
			ID:   input[i+1],
			Type: v1alpha1.AllocateType(input[i+2]),
		}
		i += 3
	}
	return res
}

func TestReallocateIP(t *testing.T) {
	tests := []struct {
		name   string
		conf   NetConf
		ippool v1alpha1.IPPool
		exp    string
	}{
		{
			name: "no reallocate for pod",
			conf: NetConf{
				Type:       v1alpha1.AllocatedTypePod,
				K8sPodName: "podName",
				K8sPodNs:   "podNs",
				AllocateIdentify: "123556",
			},
			ippool: v1alpha1.IPPool{
				Status: v1alpha1.IPPoolStatus{
					AllocatedIPs: makeAllocateStatus("10.1.1.1", "123556", "pod"),
				},
			},
			exp: "",
		},
		{
			name: "no reallocate for pool",
			conf: NetConf{
				Pool:       "pool1",
				Type:       v1alpha1.AllocatedTypePod,
				K8sPodName: "podName",
				K8sPodNs:   "podNs",
			},
			ippool: v1alpha1.IPPool{

				ObjectMeta: metav1.ObjectMeta{
					Name: "pool2",
				},
				Status: v1alpha1.IPPoolStatus{
					AllocatedIPs: makeAllocateStatus("10.1.1.1", "podNs/podName", "pod"),
				},
			},
			exp: "",
		},
		{
			name: "no reallocate for static IP",
			conf: NetConf{
				Pool:       "pool1",
				IP:         "10.1.1.2",
				Type:       v1alpha1.AllocatedTypePod,
				K8sPodName: "podName",
				K8sPodNs:   "podNs",
			},
			ippool: v1alpha1.IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pool1",
				},
				Status: v1alpha1.IPPoolStatus{
					AllocatedIPs: makeAllocateStatus("10.1.1.1", "podNs/podName", "pod"),
				},
			},
			exp: "",
		},
		{
			name: "no reallocate for cniused",
			conf: NetConf{
				Type:       v1alpha1.AllocatedTypeCNIUsed,
				AllocateIdentify: "123456",
				K8sPodName: "podName",
				K8sPodNs:   "podNs",
			},
			ippool: v1alpha1.IPPool{
				Status: v1alpha1.IPPoolStatus{
					AllocatedIPs: makeAllocateStatus("10.1.1.1", "podNs/podName", "cniused"),
				},
			},
			exp: "",
		},
		{
			name: "reallocate IP for pod",
			conf: NetConf{
				Type:       v1alpha1.AllocatedTypePod,
				K8sPodName: "podName",
				K8sPodNs:   "podNs",
			},
			ippool: v1alpha1.IPPool{
				Status: v1alpha1.IPPoolStatus{
					AllocatedIPs: makeAllocateStatus("10.1.1.1", "podNs/podName",  "pod", "10.1.1.2", "podNs/podName", "cniused"),
				},
			},
			exp: "10.1.1.1",
		},
		{
			name: "reallocate IP with pool",
			conf: NetConf{
				Pool: "pool1",
				Type:       v1alpha1.AllocatedTypeCNIUsed,
				AllocateIdentify: "123455",
				K8sPodName: "podName",
				K8sPodNs:   "podNs",
			},
			ippool: v1alpha1.IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pool1",
				},
				Status: v1alpha1.IPPoolStatus{
					AllocatedIPs: makeAllocateStatus("10.1.1.1", "podNs/podName",  "pod", "10.1.1.2", "123455", "cniused"),
				},
			},
			exp: "10.1.1.2",
		},
		{
			name: "rellocate IP with static IP",
			conf: NetConf{
				Pool: "pool1",
				IP: "10.1.1.3",
				Type:       v1alpha1.AllocatedTypeCNIUsed,
				AllocateIdentify: "123455",
				K8sPodName: "podName",
				K8sPodNs:   "podNs",
			},
			ippool: v1alpha1.IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pool1",
				},
				Status: v1alpha1.IPPoolStatus{
					AllocatedIPs: makeAllocateStatus("10.1.1.1", "podNs/podName",  "pod", "10.1.1.3", "123455", "cniused"),
				},
			},
			exp: "10.1.1.3",
		},
	}

	for _, item := range tests {
		res := reallocateIP(&item.conf, &item.ippool)
		if res != item.exp {
			t.Errorf("test %s failed, exp is %s, real is %s", item.name, item.exp, res)
		}
	}
}
