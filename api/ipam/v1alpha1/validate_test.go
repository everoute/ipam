package v1alpha1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//nolint:funlen
func TestValidatePool(t *testing.T) {
	poollist := IPPoolList{Items: []IPPool{
		{
			Spec: IPPoolSpec{
				CIDR: "10.20.0.0/16",
			},
		},
		{
			Spec: IPPoolSpec{
				CIDR: "10.30.0.0/16",
			},
		},
		{
			Spec: IPPoolSpec{
				CIDR: "10.40.0.0/16",
			},
		},
	}}

	err := ValidatePool(poollist, IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test1",
			Namespace: "default",
		},
		Spec: IPPoolSpec{
			CIDR: "10.50.0.0/16",
		},
	}, "")
	if err != nil {
		t.Fatal("no conflict")
	}

	err = ValidatePool(poollist, IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test2",
			Namespace: "default",
		},
		Spec: IPPoolSpec{
			CIDR: "10.50.10.0/24",
		},
	}, "")
	if err.Error() != "default/test2 (want add) conflict with default/test1 (exist). "+
		"And(cidr) want is 10.50.10.0/24 exist is 10.50.0.0/16" {
		t.Fatal("big exist")
	}

	err = ValidatePool(poollist, IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test1",
			Namespace: "default",
		},
		Spec: IPPoolSpec{
			CIDR: "10.0.0.0/8",
		},
	}, "default/test1")
	if err.Error() != "default/test1 (want add) conflict with / (exist). And(cidr) want is 10.0.0.0/8 exist is 10.20.0.0/16" {
		t.Fatal("small exist")
	}

	err = ValidatePool(poollist, IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test3",
			Namespace: "default",
		},
		Spec: IPPoolSpec{
			CIDR: "10.30.0.0/16",
		},
	}, "")
	if err.Error() != "default/test3 (want add) conflict with / (exist). And(cidr) want is 10.30.0.0/16 exist is 10.30.0.0/16" {
		t.Fatal("same exist")
	}

	err = ValidatePool(poollist, IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test1",
			Namespace: "default",
		},
		Spec: IPPoolSpec{
			CIDR: "10.50.0.0/16",
		},
	}, "")
	if err != nil {
		t.Fatal("no conflict")
	}

	err = ValidatePool(poollist, IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test1",
			Namespace: "default",
		},
		Spec: IPPoolSpec{
			CIDR: "10.50.20.0/24",
		},
	}, "default/test1")
	if err != nil {
		t.Fatal("update should right")
	}

	// add one ippool
	_ = ValidatePool(poollist, IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test2",
			Namespace: "default",
		},
		Spec: IPPoolSpec{
			CIDR: "10.100.0.0/16",
		},
	}, "")

	if len(mycache.newAddPools) != 2 {
		t.Fatal("local cache only two")
	}
}
