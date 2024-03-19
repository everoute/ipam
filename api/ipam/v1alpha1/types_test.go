package v1alpha1

import (
	"net"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStartIP(t *testing.T) {
	tests := []struct {
		name string
		pool *IPPool
		exp  net.IP
	}{
		{
			name: "start end",
			pool: newIPPool("10.10.10.1/16", "10.10.10.1", "10.10.11.3", "10.10.34.125", ""),
			exp:  net.ParseIP("10.10.11.3"),
		},
		{
			name: "cidr",
			pool: newIPPool("", "10.10.10.1", "", "", "10.10.11.129/26"),
			exp:  net.ParseIP("10.10.11.128"),
		},
	}
	for i := range tests {
		res := tests[i].pool.StartIP()
		if !res.Equal(tests[i].exp) {
			t.Errorf("test %s failed, expect is %s, real is %s", tests[i].name, tests[i].exp, res)
		}
	}
}

func TestEndIP(t *testing.T) {
	tests := []struct {
		name string
		pool *IPPool
		exp  net.IP
	}{
		{
			name: "start end",
			pool: newIPPool("10.10.10.0/16", "10.10.10.1", "10.10.129.1", "10.10.129.127", ""),
			exp:  net.ParseIP("10.10.129.127"),
		},
		{
			name: "cidr",
			pool: newIPPool("10.10.10.0/16", "10.10.10.1", "", "", "10.10.10.0/24"),
			exp:  net.ParseIP("10.10.10.255"),
		},
	}
	for i := range tests {
		res := tests[i].pool.EndIP()
		if !res.Equal(tests[i].exp) {
			t.Errorf("test %s failed, expect is %s, real is %s", tests[i].name, tests[i].exp, res)
		}
	}
}

func newIPPool(subnet, gw, start, end ,cidr string, excepts ...string) *IPPool {
	return &IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pool",
			Namespace: "ns",
		},
		Spec: IPPoolSpec{
			Subnet:  subnet,
			Gateway: gw,
			CIDR:    cidr,
			Start:   start,
			End:     end,
			Except:  excepts,
		},
	}
}
