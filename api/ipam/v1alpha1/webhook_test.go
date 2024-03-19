package v1alpha1

import (
	"fmt"
	"net"
	"testing"
)

func TestValidSpec(t *testing.T) {
	tests := []struct {
		name string
		pool *IPPool
		exp  error
	}{
		// normal
		{
			name: "valid for cidr",
			pool: newIPPool("10.10.1.0/24", "10.10.1.5", "", "", "10.10.1.128/25"),
			exp:  nil,
		},
		{
			name: "valid for cidr with except",
			pool: newIPPool("10.10.1.0/24", "10.10.1.5", "", "", "10.10.1.128/25", "10.10.1.129/30", "192.133.11.1/14", "10.10.1.192/32"),
			exp:  nil,
		},
		{
			name: "valid for start litter than end",
			pool: newIPPool("10.10.1.0/24", "10.10.1.5", "10.10.1.125", "10.10.1.134", ""),
			exp:  nil,
		},
		{
			name: "valid for start equal end",
			pool: newIPPool("10.10.1.0/24", "10.10.1.5", "10.10.1.125", "10.10.1.125", ""),
			exp:  nil,
		},

		// format err
		{
			name: "subnet is not valid ipnet",
			pool: newIPPool("10.10.1/24", "10.10.1.5", "", "", "10.10.1.128/25", "10.10.1.129/30", "192.133.11.1/14", "10.10.1.192/32"),
			exp:  fmt.Errorf("failed to parse subnet %s, err: %s", "10.10.1/24", &net.ParseError{Type: "CIDR address", Text: "10.10.1/24"}),
		},
		{
			name: "gateway is not a valid ipv4",
			pool: newIPPool("10.10.1.0/24", "10.fe.1.1", "", "", "10.10.1.128/25"),
			exp:  fmt.Errorf("invalid ippool gateway %s", "10.fe.1.1"),
		},
		{
			name: "cidr is not valid ipnet",
			pool: newIPPool("10.10.1.0/24", "10.10.1.5", "", "", "10.10.1.1334/25"),
			exp:  fmt.Errorf("parse ippool cidr %s failed, err: %s", "10.10.1.1334/25", &net.ParseError{Type: "CIDR address", Text: "10.10.1.1334/25"}),
		},
		{
			name: "except is not valid ipnet",
			pool: newIPPool("10.10.1.0/24", "10.10.1.5", "", "", "10.10.1.128/25", "10.10.1.129/32", "192.133.11.1/14", "10.10.1.192/300"),
			exp:  fmt.Errorf("parse spec.except %s, err: %s", "10.10.1.192/300", &net.ParseError{Type: "CIDR address", Text: "10.10.1.192/300"}),
		},
		{
			name: "start is not a valid ipv4",
			pool: newIPPool("10.10.1.0/24", "10.10.1.5", "10.10.1.", "10.10.1.125", ""),
			exp:  fmt.Errorf("invalid start ipv4 %s", "10.10.1."),
		},
		{
			name: "end is not a valid ipv4",
			pool: newIPPool("10.10.1.0/24", "10.10.1.5", "10.10.1.123", "10.10.1::125", ""),
			exp:  fmt.Errorf("invalid end ipv4 %s", "10.10.1::125"),
		},

		// optional err
		{
			name: "start and cidr can't set in the same time",
			pool: newIPPool("10.10.1.0/24", "10.10.1.5", "10.10.1.123", "", "10.10.1.128/25", "10.10.1.129/30", "192.133.11.1/14", "10.10.1.192/32"),
			exp:  fmt.Errorf("can't set spec.cidr and spec.start at the same time"),
		},
		{
			name: "end and cidr can't set in the same time",
			pool: newIPPool("10.10.1.0/24", "10.10.1.5", "", "10.10.1.123", "10.10.1.128/25", "10.10.1.129/30", "192.133.11.1/14", "10.10.1.192/32"),
			exp:  fmt.Errorf("can't set spec.cidr and spec.end at the same time"),
		},
		{
			name: "can't set except and start in the same time",
			pool: newIPPool("10.10.1.0/24", "10.10.1.5", "10.10.1.125", "", "", "10.10.1.127/32"),
			exp:  fmt.Errorf("can't set spec.except without spec.cidr"),
		},
		{
			name: "can't set except and end in the same time",
			pool: newIPPool("10.10.1.0/24", "10.10.1.5", "", "10.10.1.125", "", "10.10.1.127/32"),
			exp:  fmt.Errorf("can't set spec.except without spec.cidr"),
		},
		{
			name: "can't set except without cidr",
			pool: newIPPool("10.10.1.0/24", "10.10.1.5", "", "", "", "10.10.1.127/32"),
			exp:  fmt.Errorf("can't set spec.except without spec.cidr"),
		},
		{
			name: "can't set start without end",
			pool: newIPPool("10.10.1.0/24", "10.10.1.5", "10.10.1.23", "", ""),
			exp:  fmt.Errorf("must set spec.start and spec.end when doesn't set sepc.cidr"),
		},
		{
			name: "can't set end without start",
			pool: newIPPool("10.10.1.0/24", "10.10.1.5", "", "10.10.1.23", ""),
			exp:  fmt.Errorf("must set spec.start and spec.end when doesn't set sepc.cidr"),
		},
		{
			name: "must set cidr or start end",
			pool: newIPPool("10.10.1.0/24", "10.10.1.5", "", "", ""),
			exp:  fmt.Errorf("must set spec.start and spec.end when doesn't set sepc.cidr"),
		},

		// subnet
		{
			name: "gateway is not in subnet",
			pool: newIPPool("10.10.1.0/24", "10.10.2.1", "", "", "10.10.1.128/25"),
			exp:  fmt.Errorf("gateway %s doesn't in subnet %s", "10.10.2.1", "10.10.1.0/24"),
		},
		{
			name: "gateway can't be subnet network number",
			pool: newIPPool("10.10.1.0/24", "10.10.1.0", "", "", "10.10.1.128/25"),
			exp:  fmt.Errorf("gateway %s can't be subnet %s network number", "10.10.1.0", "10.10.1.0/24"),
		},
		{
			name: "start is not in subnet",
			pool: newIPPool("10.10.1.0/24", "10.10.1.5", "10.10.0.23", "10.10.1.128", ""),
			exp:  fmt.Errorf("ippool's ip must all in subnet %s", "10.10.1.0/24"),
		},
		{
			name: "end is not in subnet",
			pool: newIPPool("10.10.1.0/24", "10.10.1.5", "10.10.1.23", "10.10.2.128", ""),
			exp:  fmt.Errorf("ippool's ip must all in subnet %s", "10.10.1.0/24"),
		},
		{
			name: "cidr is not in subnet",
			pool: newIPPool("10.10.1.0/24", "10.10.1.5", "", "", "10.10.1.0/23"),
			exp:  fmt.Errorf("ippool's ip must all in subnet %s", "10.10.1.0/24"),
		},

		// start end
		{
			name: "start ip must litter or equal with end ip",
			pool: newIPPool("10.10.1.0/24", "10.10.1.5", "10.10.1.23", "10.10.1.8", ""),
			exp:  fmt.Errorf("start ip %s must smaller or equal to end ip %s", "10.10.1.23", "10.10.1.8"),
		},
	}

	for i := range tests {
		res := tests[i].pool.validateSpec()
		if res == nil && tests[i].exp == nil {
			continue
		}
		if res == nil || tests[i].exp == nil {
			t.Errorf("test %s failed, expect is %s, real is %s", tests[i].name, tests[i].exp, res)
		}
		if res.Error() != tests[i].exp.Error() {
			t.Errorf("test %s failed, expect is %s, real is %s", tests[i].name, tests[i].exp.Error(), res.Error())
		}
	}
}

func TestValidateAllocateIPs(t *testing.T) {
	tests := []struct {
		name string
		pool *IPPool
		exp  error
	}{
		{
			name: "no allocate or used ip for start-end",
			pool: newIPPoolWithStatus(newIPPool("10.10.1.0/24", "10.10.1.1", "10.10.1.123", "10.10.1.192", ""), nil, nil),
			exp:  nil,
		},
		{
			name: "only allocate ip and all in update ippool for start-end",
			pool: newIPPoolWithStatus(newIPPool("10.10.1.0/24", "10.10.1.1", "10.10.1.123", "10.10.1.192", ""), nil, []string{"10.10.1.129", "10.10.1.156"}),
			exp:  nil,
		},
		{
			name: "only used ip and all in update for cidr",
			pool: newIPPoolWithStatus(newIPPool("10.10.1.0/24", "10.10.1.1", "", "", "10.10.1.128/25"), []string{"10.10.1.129", "10.10.1.156"}, nil),
			exp:  nil,
		},
		{
			name: "allocate ip and used ip all in update ippool for cidr with except",
			pool: newIPPoolWithStatus(newIPPool("10.10.1.0/24", "10.10.1.1", "", "", "10.10.1.128/25", "10.10.1.129/32", "10.10.1.192/30"), []string{"10.10.1.128", "10.10.1.156"}, []string{"10.10.1.134"}),
			exp:  nil,
		},
		{
			name: "used ip in excepts",
			pool: newIPPoolWithStatus(newIPPool("10.10.1.0/24", "10.10.1.1", "", "", "10.10.1.128/25", "10.10.1.129/32", "10.10.1.192/30"), []string{"10.10.1.193", "10.10.1.128"}, nil),
			exp:  fmt.Errorf("ippool must contain has been allocated ip %s", "10.10.1.193"),
		},
		{
			name: "allocate ip in except",
			pool: newIPPoolWithStatus(newIPPool("10.10.1.0/24", "10.10.1.1", "", "", "10.10.1.128/25", "10.10.1.129/32", "10.10.1.192/30"), nil, []string{"10.10.1.193", "10.10.1.128"}),
			exp:  fmt.Errorf("ippool must contain has been allocated ip %s", "10.10.1.193"),
		},
		{
			name: "allocate ip not in cidr for changed net",
			pool: newIPPoolWithStatus(newIPPool("10.10.1.0/24", "10.10.1.1", "", "", "10.10.1.0/25", "10.10.1.129/32", "10.10.1.192/30"), nil, []string{"10.10.1.193", "10.10.1.28"}),
			exp:  fmt.Errorf("ippool must contain has been allocated ip %s", "10.10.1.193"),
		},
		{
			name: "allocate ip not in cidr for changed prefixlen",
			pool: newIPPoolWithStatus(newIPPool("10.10.1.0/24", "10.10.1.1", "", "", "10.10.1.128/26"), nil, []string{"10.10.1.254", "10.10.1.133"}),
			exp:  fmt.Errorf("ippool must contain has been allocated ip %s", "10.10.1.254"),
		},
		{
			name: "used ip not in cidr for changed net",
			pool: newIPPoolWithStatus(newIPPool("10.10.1.0/24", "10.10.1.1", "", "", "10.10.1.0/25", "10.10.1.129/32", "10.10.1.192/30"), []string{"10.10.1.193", "10.10.1.128"}, nil),
			exp:  fmt.Errorf("ippool must contain has been allocated ip %s", "10.10.1.193"),
		},
		{
			name: "used ip not in cidr for changed prefixlen",
			pool: newIPPoolWithStatus(newIPPool("10.10.1.0/24", "10.10.1.1", "", "", "10.10.1.128/26"), []string{"10.10.1.254", "10.10.1.133"}, nil),
			exp:  fmt.Errorf("ippool must contain has been allocated ip %s", "10.10.1.254"),
		},
		{
			name: "allocate ip not in start-end for increase start",
			pool: newIPPoolWithStatus(newIPPool("10.10.1.0/24", "10.10.1.1", "10.10.1.123", "10.10.1.192", ""), nil, []string{"10.10.1.121", "10.10.1.156"}),
			exp:  fmt.Errorf("ippool must contain has been allocated ip %s", "10.10.1.121"),
		},
		{
			name: "allocate ip not in start-end for decrease end",
			pool: newIPPoolWithStatus(newIPPool("10.10.1.0/24", "10.10.1.1", "10.10.1.121", "10.10.1.192", ""), nil, []string{"10.10.1.121", "10.10.1.199"}),
			exp:  fmt.Errorf("ippool must contain has been allocated ip %s", "10.10.1.199"),
		},
		{
			name: "allocate ip not in start-end for change start-end range",
			pool: newIPPoolWithStatus(newIPPool("10.10.1.0/24", "10.10.1.1", "10.10.1.121", "10.10.1.192", ""), nil, []string{"10.10.1.50"}),
			exp:  fmt.Errorf("ippool must contain has been allocated ip %s", "10.10.1.50"),
		},
		{
			name: "used ip not in start-end for increase start",
			pool: newIPPoolWithStatus(newIPPool("10.10.1.0/24", "10.10.1.1", "10.10.1.123", "10.10.1.192", ""), []string{"10.10.1.121", "10.10.1.156"}, nil),
			exp:  fmt.Errorf("ippool must contain has been allocated ip %s", "10.10.1.121"),
		},
		{
			name: "used ip not in start-end for decrease end",
			pool: newIPPoolWithStatus(newIPPool("10.10.1.0/24", "10.10.1.1", "10.10.1.121", "10.10.1.192", ""), []string{"10.10.1.121", "10.10.1.199"}, nil),
			exp:  fmt.Errorf("ippool must contain has been allocated ip %s", "10.10.1.199"),
		},
		{
			name: "used ip not in start-end for change start-end range",
			pool: newIPPoolWithStatus(newIPPool("10.10.1.0/24", "10.10.1.1", "10.10.1.121", "10.10.1.192", ""), []string{"10.10.1.50"}, nil),
			exp:  fmt.Errorf("ippool must contain has been allocated ip %s", "10.10.1.50"),
		},
	}
	for i := range tests {
		res := tests[i].pool.validateAllocateIPs()
		if res == nil && tests[i].exp == nil {
			continue
		}
		if res == nil || tests[i].exp == nil {
			t.Errorf("test %s failed, expect is %s, real is %s", tests[i].name, tests[i].exp, res)
		}
		if res.Error() != tests[i].exp.Error() {
			t.Errorf("test %s failed, expect is %s, real is %s", tests[i].name, tests[i].exp.Error(), res.Error())
		}
	}
}

func newIPPoolWithStatus(p *IPPool, usedIPs []string, allocateIPs []string) *IPPool {
	if len(usedIPs) > 0 {
		p.Status.UsedIps = make(map[string]string)
	}
	if len(allocateIPs) > 0 {
		p.Status.AllocatedIPs = make(map[string]AllocateInfo)
	}
	for _, ip := range usedIPs {
		p.Status.UsedIps[ip] = "xxxx"
	}
	for _, ip := range allocateIPs {
		p.Status.AllocatedIPs[ip] = AllocateInfo{
			ID:   "xxxx",
			Type: AllocateTypePod,
		}
	}

	return p
}
