package controller

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/everoute/ipam/api/ipam/v1alpha1"
	"github.com/everoute/ipam/pkg/constants"
)

var _ = Describe("ippool controller test", func() {
	name := "pool-offset"
	pool := v1alpha1.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: v1alpha1.IPPoolSpec{
			CIDR:    "192.18.1.1/24",
			Gateway: "192.18.1.1",
			Subnet:  "192.18.0.0/23",
		},
	}
	BeforeEach(func() {
		p := pool.DeepCopy()
		Expect(k8sClient.Create(ctx, p)).Should(Succeed())
	})
	AfterEach(func() {
		Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.IPPool{}, client.InNamespace(ns))).Should(Succeed())
	})

	Context("update ippool spec", func() {
		When("offset doesn't full", func() {
			BeforeEach(func() {
				By("set offset")
				p := v1alpha1.IPPool{}
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, &p)).Should(Succeed())
				}, timeout, interval).Should(Succeed())
				p.Status.Offset = 10
				Expect(k8sClient.Status().Update(ctx, &p)).Should(Succeed())

				By("update spec cidr")
				p.Spec.CIDR = "192.18.2.1/24"
				Expect(k8sClient.Update(ctx, &p)).Should(Succeed())
			})
			It("should reset offset", func() {
				Eventually(func(g Gomega) {
					p := v1alpha1.IPPool{}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, &p)).Should(Succeed())
					g.Expect(p.Status.Offset).Should(Equal(constants.IPPoolOffsetReset))
				}, timeout, interval).Should(Succeed())
			})
		})

		When("offset full", func() {
			BeforeEach(func() {
				By("set offset")
				p := v1alpha1.IPPool{}
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, &p)).Should(Succeed())
				}, timeout, interval).Should(Succeed())

				p.Status.Offset = constants.IPPoolOffsetFull
				Expect(k8sClient.Status().Update(ctx, &p)).Should(Succeed())

				By("update spec except")
				p.Spec.Except = append(p.Spec.Except, "192.18.1.17/32")
				Expect(k8sClient.Update(ctx, &p)).Should(Succeed())
			})
			It("should reset offset", func() {
				Eventually(func(g Gomega) {
					p := v1alpha1.IPPool{}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, &p)).Should(Succeed())
					g.Expect(p.Status.Offset).Should(Equal(constants.IPPoolOffsetReset))
				}, timeout, interval).Should(Succeed())
			})
		})
	})
})

func TestPredicateUpdate(t *testing.T) {
	tests := []struct {
		name string
		old  *v1alpha1.IPPool
		new  *v1alpha1.IPPool
		exp  bool
	}{
		{
			name: "no update",
			old:  newIPPool("10.10.1.1/23", "10.1.1.1", "10.10.1.34", "10.10.1.56", ""),
			new:  newIPPool("10.10.1.1/23", "10.1.1.1", "10.10.1.34", "10.10.1.56", ""),
			exp:  false,
		},
		{
			name: "update status",
			old:  newIPPoolWithStatus(newIPPool("10.10.1.1/23", "10.1.1.1", "10.10.1.34", "10.10.1.56", ""), 10, nil, nil),
			new:  newIPPoolWithStatus(newIPPool("10.10.1.1/23", "10.1.1.1", "10.10.1.34", "10.10.1.56", ""), 12, []string{"10.1.1.1"}, nil),
			exp:  false,
		},
		{
			name: "update start-end but offset is 0",
			old:  newIPPool("10.10.1.1/23", "10.1.1.1", "10.10.1.34", "10.10.1.56", ""),
			new:  newIPPool("10.10.1.1/23", "10.1.1.1", "10.10.1.35", "10.10.1.57", ""),
			exp:  false,
		},
		{
			name: "update cidr",
			old:  newIPPoolWithStatus(newIPPool("10.10.1.1/23", "10.10.1.1", "", "", "10.10.1.1/24"), 10, nil, nil),
			new:  newIPPoolWithStatus(newIPPool("10.10.1.1/23", "10.10.1.1", "", "", "10.10.2.1/24"), 10, nil, nil),
			exp:  true,
		},
		{
			name: "update start",
			old:  newIPPoolWithStatus(newIPPool("10.10.1.1/23", "10.1.1.1", "10.10.1.34", "10.10.1.56", ""), 10, nil, nil),
			new:  newIPPoolWithStatus(newIPPool("10.10.1.1/23", "10.1.1.1", "10.10.1.36", "10.10.1.56", ""), 10, nil, nil),
			exp:  true,
		},
		{
			name: "update end",
			old:  newIPPoolWithStatus(newIPPool("10.10.1.1/23", "10.1.1.1", "10.10.1.34", "10.10.1.56", ""), 10, nil, nil),
			new:  newIPPoolWithStatus(newIPPool("10.10.1.1/23", "10.1.1.1", "10.10.1.34", "10.10.1.59", ""), 10, nil, nil),
			exp:  true,
		},
		{
			name: "add except",
			old:  newIPPoolWithStatus(newIPPool("10.10.1.1/23", "10.10.1.1", "", "", "10.10.1.1/24"), 10, nil, nil),
			new:  newIPPoolWithStatus(newIPPool("10.10.1.1/23", "10.10.1.1", "", "", "10.10.1.1/24", "10.10.1.6", "10.10.1.7"), 10, nil, nil),
			exp:  true,
		},
		{
			name: "del except",
			old:  newIPPoolWithStatus(newIPPool("10.10.1.1/23", "10.10.1.1", "", "", "10.10.1.1/24", "10.10.1.6", "10.10.1.7"), 10, nil, nil),
			new:  newIPPoolWithStatus(newIPPool("10.10.1.1/23", "10.10.1.1", "", "", "10.10.1.1/24", "10.10.1.6"), 10, nil, nil),
			exp:  true,
		},
		{
			name: "update except to duplicate one",
			old:  newIPPoolWithStatus(newIPPool("10.10.1.1/23", "10.10.1.1", "", "", "10.10.1.1/24", "10.10.1.6", "10.10.1.7"), 10, nil, nil),
			new:  newIPPoolWithStatus(newIPPool("10.10.1.1/23", "10.10.1.1", "", "", "10.10.1.1/24", "10.10.1.6", "10.10.1.6"), 10, nil, nil),
			exp:  true,
		},
		{
			name: "take place except",
			old:  newIPPoolWithStatus(newIPPool("10.10.1.1/23", "10.10.1.1", "", "", "10.10.1.1/24", "10.10.1.8", "10.10.1.6"), 10, nil, nil),
			new:  newIPPoolWithStatus(newIPPool("10.10.1.1/23", "10.10.1.1", "", "", "10.10.1.1/24", "10.10.1.6", "10.10.1.8"), 10, nil, nil),
			exp:  false,
		},
		{
			name: "update except",
			old:  newIPPoolWithStatus(newIPPool("10.10.1.1/23", "10.10.1.1", "", "", "10.10.1.1/24", "10.10.1.8", "10.10.1.6"), 10, nil, nil),
			new:  newIPPoolWithStatus(newIPPool("10.10.1.1/23", "10.10.1.1", "", "", "10.10.1.1/24", "10.10.1.7", "10.10.1.8"), 10, nil, nil),
			exp:  true,
		},
	}

	p := &PoolController{}
	for i := range tests {
		e := event.UpdateEvent{
			ObjectNew: tests[i].new,
			ObjectOld: tests[i].old,
		}
		res := p.predicateUpdate(e)
		if res != tests[i].exp {
			t.Errorf("test %s failed, expect is %v, real is %v", tests[i].name, tests[i].exp, res)
		}
	}
}
