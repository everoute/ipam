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

func TestLength(t *testing.T) {
	tests := []struct{
		name string
		pool *IPPool
		exp int64
	}{
		{
			name: "start end",
			pool: newIPPool("10.10.1.0/16", "10.10.1.1", "10.10.1.34", "10.10.2.78", ""),
			exp: 301,
		},
		{
			name: "cidr",
			pool: newIPPool("10.10.1.0/16", "10.10.1.1", "", "", "10.10.2.0/25"),
			exp: 128,
		},
		{
			name: "cidr with except",
			pool: newIPPool("10.10.1.0/16", "10.10.1.1", "", "", "10.10.2.0/25", "10.10.2.45/32"),
			exp: 128,
		},
	}

	for i := range tests {
		res := tests[i].pool.Length()
		if res != tests[i].exp {
			t.Errorf("test %s failed, expect is %d, real is %d", tests[i].name, tests[i].exp, res)
		}
	}
}

func TestContains(t *testing.T) {
	tests := []struct{
		name string
		pool *IPPool
		ip net.IP
		exp bool
	}{
		{
			name: "ip in start-end ippool",
			pool: newIPPool("10.10.0.1/16", "10.10.1.1", "10.10.1.2", "10.10.1.45", ""),
			ip: net.ParseIP("10.10.1.6"),
			exp: true,
		},
		{
			name: "ip not in start-end ippool",
			pool: newIPPool("10.10.1.1/16", "10.10.1.1", "10.10.1.2", "10.10.23.34", ""),
			ip: net.ParseIP("10.10.0.78"),
			exp: false,
		},
		{
			name: "ip in cidr ippool",
			pool: newIPPool("10.10.1.1/16", "10.10.1.1", "", "", "10.10.2.0/25", "10.10.2.34/32", "10.10.1.34/26"),
			ip: net.ParseIP("10.10.2.64"),
			exp: true,
		},
		{
			name: "ip not in cidr ipool",
			pool: newIPPool("10.10.1.1/16", "10.10.1.1", "", "", "10.10.2.0/25", "10.10.2.65/31", "10.10.1.34/26"),
			ip: net.ParseIP("10.10.3.61"),
			exp: false,
		},
		{
			name: "ip in except", 
			pool: newIPPool("10.10.1.1/16", "10.10.1.1", "", "", "10.10.2.0/25", "10.10.2.65/31", "10.10.1.36/26"),
			ip: net.ParseIP("10.10.2.64"),
			exp: false,
		},
	}
	for i := range tests {
		res := tests[i].pool.Contains(tests[i].ip)
		if res != tests[i].exp {
			t.Errorf("test %s failed, real is %v, except is %v", tests[i].name, res, tests[i].exp)
		}
	}
}
