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
	name2 := "pool-2"
	name3 := "pool-3"
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
	pool2 := v1alpha1.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name2,
			Namespace: ns,
		},
		Spec: v1alpha1.IPPoolSpec{
			CIDR:    "192.168.0.0/24",
			Gateway: "192.168.0.1",
			Subnet:  "192.168.0.0/16",
			Except:  []string{"192.168.0.0/28"},
		},
	}

	pool3 := v1alpha1.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name3,
			Namespace: ns,
		},
		Spec: v1alpha1.IPPoolSpec{
			Start:   "192.168.0.0",
			End:     "192.168.2.16",
			Gateway: "192.168.0.1",
			Subnet:  "192.168.0.0/16",
			Except:  []string{"192.168.0.0/28"},
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
					p.Status.Offset = 10
					g.Expect(k8sClient.Status().Update(ctx, &p)).Should(Succeed())
				}, timeout, interval).Should(Succeed())

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
			It("should update counter", func() {
				Eventually(func(g Gomega) {
					p := v1alpha1.IPPool{}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, &p)).Should(Succeed())
					g.Expect(p.Status.TotalCount).Should(Equal(int64(256)))
					g.Expect(p.Status.AvailableCount).Should(Equal(int64(256)))
				}, timeout, interval).Should(Succeed())
			})
		})

		When("offset full", func() {
			BeforeEach(func() {
				By("set offset")
				p := v1alpha1.IPPool{}
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, &p)).Should(Succeed())
					p.Status.Offset = constants.IPPoolOffsetFull
					g.Expect(k8sClient.Status().Update(ctx, &p)).Should(Succeed())
				}, timeout, interval).Should(Succeed())

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
			It("should update counter", func() {
				Eventually(func(g Gomega) {
					p := v1alpha1.IPPool{}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, &p)).Should(Succeed())
					g.Expect(p.Status.TotalCount).Should(Equal(int64(253)))
					g.Expect(p.Status.AvailableCount).Should(Equal(int64(253)))
				}, timeout, interval).Should(Succeed())
			})
		})
	})
	Context("ip counter", func() {
		When("pool with cidr", func() {
			var p *v1alpha1.IPPool
			BeforeEach(func() {
				p = pool2.DeepCopy()
				Expect(k8sClient.Create(ctx, p)).Should(Succeed())
			})
			It("should set right counters", func() {
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name2, Namespace: ns}, p)).Should(Succeed())
					g.Expect(p.Status.TotalCount).Should(Equal(int64(240)))
					g.Expect(p.Status.AvailableCount).Should(Equal(int64(240)))
				}, timeout, interval).Should(Succeed())
			})

			When("update pool", func() {
				BeforeEach(func() {
					By("update spec cidr")
					Eventually(func(g Gomega) {
						g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name2, Namespace: ns}, p)).Should(Succeed())
						p.Spec.Except = []string{"192.168.128.0/28"}
						g.Expect(k8sClient.Update(ctx, p)).Should(Succeed())
					}, timeout, interval).Should(Succeed())
				})
				It("should update counters", func() {
					Eventually(func(g Gomega) {
						g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name2, Namespace: ns}, p)).Should(Succeed())
						g.Expect(p.Status.TotalCount).Should(Equal(int64(254)))
						g.Expect(p.Status.AvailableCount).Should(Equal(int64(254)))
					}, timeout, interval).Should(Succeed())
				})
			})
		})
		When("pool with start end", func() {
			var p *v1alpha1.IPPool
			BeforeEach(func() {
				p = pool3.DeepCopy()
				Expect(k8sClient.Create(ctx, p)).Should(Succeed())
			})
			It("should set right counters", func() {
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name3, Namespace: ns}, p)).Should(Succeed())
					g.Expect(p.Status.TotalCount).Should(Equal(int64(513)))
					g.Expect(p.Status.AvailableCount).Should(Equal(int64(513)))
				}, timeout, interval).Should(Succeed())
			})

			When("update pool", func() {
				BeforeEach(func() {
					By("update spec cidr")
					Eventually(func(g Gomega) {
						g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name3, Namespace: ns}, p)).Should(Succeed())
						p.Spec.Except = []string{"192.168.128.0/28"}
						g.Expect(k8sClient.Update(ctx, p)).Should(Succeed())
					}, timeout, interval).Should(Succeed())
				})
				It("should update counters", func() {
					Eventually(func(g Gomega) {
						g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name3, Namespace: ns}, p)).Should(Succeed())
						g.Expect(p.Status.TotalCount).Should(Equal(int64(527)))
						g.Expect(p.Status.AvailableCount).Should(Equal(int64(527)))
					}, timeout, interval).Should(Succeed())
				})
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
			exp:  true,
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
