package cron

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/everoute/ipam/api/ipam/v1alpha1"
	"github.com/everoute/ipam/pkg/constants"
)

var _ = Describe("clean_stale_ip", func() {
	pod1Name := "pod1"
	pod1 := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod1Name,
			Namespace: ns,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test",
					Image: "network",
				},
			},
		},
	}
	pod2Name := "pod2"
	pod2 := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod2Name,
			Namespace: ns,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test",
					Image: "network",
				},
			},
		},
	}
	sts1Name := "sts1"
	podLabel := make(map[string]string, 1)
	podLabel["Owner"] = sts1Name
	sts1 := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sts1Name,
			Namespace: ns,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: podLabel,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabel,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "network",
						},
					},
				},
			},
		},
	}
	pool1 := v1alpha1.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pool1",
			Namespace: ns,
		},
		Spec: v1alpha1.IPPoolSpec{
			CIDR:    "10.10.65.0/28",
			Subnet:  "10.10.64.0/20",
			Gateway: "10.10.65.1",
		},
	}
	pool2 := v1alpha1.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pool2",
			Namespace: ns,
		},
		Spec: v1alpha1.IPPoolSpec{
			CIDR:    "12.10.64.0/29",
			Subnet:  "12.10.64.0/29",
			Gateway: "12.10.64.1",
		},
	}
	BeforeEach(func() {
		By("setup resources")
		pod1Copy := pod1.DeepCopy()
		Expect(k8sClient.Create(ctx, pod1Copy)).Should(Succeed())
		pod1Copy.Status.PodIP = "10.10.65.1"
		Expect(k8sClient.Status().Update(ctx, pod1Copy)).Should(Succeed())
		pod2Copy := pod2.DeepCopy()
		Expect(k8sClient.Create(ctx, pod2Copy)).Should(Succeed())
		pool1Copy := pool1.DeepCopy()
		Expect(k8sClient.Create(ctx, pool1Copy)).Should(Succeed())
		pool2Copy := pool2.DeepCopy()
		Expect(k8sClient.Create(ctx, pool2Copy)).Should(Succeed())
		sts1Copy := sts1.DeepCopy()
		Expect(k8sClient.Create(ctx, sts1Copy)).Should(Succeed())
	})
	AfterEach(func() {
		By("clean resources")
		Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.IPPool{}, client.InNamespace(ns))).Should(Succeed())
		Expect(k8sClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(ns))).Should(Succeed())
		Expect(k8sClient.DeleteAllOf(ctx, &appsv1.StatefulSet{}, client.InNamespace(ns))).Should(Succeed())
	})

	Context("single pool has stale IP for Pod in pool", func() {
		When("pool has full", func() {
			BeforeEach(func() {
				ippool := v1alpha1.IPPool{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
				ippool.Status.Offset = constants.IPPoolOffsetFull
				ippool.Status.AllocatedIPs = make(map[string]v1alpha1.AllocateInfo)
				ippool.Status.AllocatedIPs["10.10.65.1"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypePod, ID: ns + "/" + pod1Name}
				ippool.Status.AllocatedIPs["10.10.65.2"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypePod, ID: ns + "/" + pod1Name}
				ippool.Status.AllocatedIPs["10.10.65.3"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypePod, ID: ns + "/" + "pod-unexist"}
				ippool.Status.AllocatedIPs["10.10.65.4"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeStatefulSet, ID: "ownerstsPod", Owner: ns + "/" + sts1Name}
				Expect(k8sClient.Status().Update(ctx, &ippool)).Should(Succeed())
			})
			It("clean stale IP", func() {
				time.Sleep(period)
				Eventually(func(g Gomega) {
					ippool := v1alpha1.IPPool{}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
					By("should reset offset")
					g.Expect(ippool.Status.Offset).Should(Equal(int64(constants.IPPoolOffsetReset)))
					By("should cleanup stale IP for Pod")
					g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(2))
					g.Expect(ippool.Status.AllocatedIPs).Should(HaveKey("10.10.65.1"))
					g.Expect(ippool.Status.AllocatedIPs).ShouldNot(HaveKey("10.10.65.2"))
					g.Expect(ippool.Status.AllocatedIPs).Should(HaveKey("10.10.65.4"))
					g.Expect(ippool.Status.AllocatedIPs).ShouldNot(HaveKey("10.10.65.3"))
					By("another pool doesn't change")
					ippool2 := v1alpha1.IPPool{}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool2"}, &ippool2)).Should(Succeed())
					g.Expect(ippool2.Status.AllocatedIPs).Should(BeNil())
				}, timeout, interval).Should(Succeed())
			})
		})
		When("pool doesn't full", func() {
			BeforeEach(func() {
				ippool := v1alpha1.IPPool{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
				ippool.Status.Offset = 1
				ippool.Status.AllocatedIPs = make(map[string]v1alpha1.AllocateInfo)
				ippool.Status.AllocatedIPs["10.10.65.1"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypePod, ID: ns + "/" + pod1Name}
				ippool.Status.AllocatedIPs["10.10.65.2"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeCNIUsed, ID: "dfgggg"}
				ippool.Status.AllocatedIPs["10.10.65.3"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypePod, ID: ns + "/" + "pod-unexist"}
				Expect(k8sClient.Status().Update(ctx, &ippool)).Should(Succeed())
			})
			It("clean stale IP", func() {
				time.Sleep(period)
				Eventually(func(g Gomega) {
					ippool := v1alpha1.IPPool{}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
					By("shouldn't change offset")
					g.Expect(ippool.Status.Offset).Should(Equal(int64(1)))
					By("should cleanup stale IP")
					g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(2))
					g.Expect(ippool.Status.AllocatedIPs).Should(HaveKey("10.10.65.1"))
					g.Expect(ippool.Status.AllocatedIPs).Should(HaveKey("10.10.65.2"))
					g.Expect(ippool.Status.AllocatedIPs).ShouldNot(HaveKey("10.10.65.3"))
					By("another pool doesn't change")
					ippool2 := v1alpha1.IPPool{}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool2"}, &ippool2)).Should(Succeed())
					g.Expect(ippool2.Status.AllocatedIPs).Should(BeNil())
				}, timeout, interval).Should(Succeed())
			})
		})
	})

	Context("single pool has stale IP for StatefulSet in pool", func() {
		When("pool has full", func() {
			BeforeEach(func() {
				ippool := v1alpha1.IPPool{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
				ippool.Status.Offset = constants.IPPoolOffsetFull
				ippool.Status.AllocatedIPs = make(map[string]v1alpha1.AllocateInfo)
				ippool.Status.AllocatedIPs["10.10.65.1"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypePod, ID: ns + "/" + pod1Name}
				ippool.Status.AllocatedIPs["10.10.65.2"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeCNIUsed, ID: "dfgggg"}
				ippool.Status.AllocatedIPs["10.10.65.4"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeStatefulSet, ID: "ownerstsPod", Owner: ns + "/" + sts1Name}
				ippool.Status.AllocatedIPs["10.10.65.5"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeStatefulSet, ID: "ownerstsPod", Owner: ns + "/sts-unexist"}
				ippool.Status.AllocatedIPs["10.10.65.7"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeStatefulSet, ID: "ownerstsPod", Owner: ns + "/sts-unexist"}
				Expect(k8sClient.Status().Update(ctx, &ippool)).Should(Succeed())
			})
			It("clean stale IP", func() {
				time.Sleep(period)
				Eventually(func(g Gomega) {
					ippool := v1alpha1.IPPool{}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
					By("should reset offset")
					g.Expect(ippool.Status.Offset).Should(Equal(int64(constants.IPPoolOffsetReset)))
					By("should cleanup stale IP for StatefulSet")
					g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(3))
					g.Expect(ippool.Status.AllocatedIPs).Should(HaveKey("10.10.65.1"))
					g.Expect(ippool.Status.AllocatedIPs).Should(HaveKey("10.10.65.2"))
					g.Expect(ippool.Status.AllocatedIPs).Should(HaveKey("10.10.65.4"))
					g.Expect(ippool.Status.AllocatedIPs).ShouldNot(HaveKey("10.10.65.5"))
					g.Expect(ippool.Status.AllocatedIPs).ShouldNot(HaveKey("10.10.65.7"))
					By("another pool doesn't change")
					ippool2 := v1alpha1.IPPool{}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool2"}, &ippool2)).Should(Succeed())
					g.Expect(ippool2.Status.AllocatedIPs).Should(BeNil())
				}, timeout, interval).Should(Succeed())
			})
		})
		When("pool doesn't full", func() {
			BeforeEach(func() {
				ippool := v1alpha1.IPPool{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
				ippool.Status.Offset = 1
				ippool.Status.AllocatedIPs = make(map[string]v1alpha1.AllocateInfo)
				ippool.Status.AllocatedIPs["10.10.65.1"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypePod, ID: ns + "/" + pod1Name}
				ippool.Status.AllocatedIPs["10.10.65.2"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeCNIUsed, ID: "dfgggg"}
				ippool.Status.AllocatedIPs["10.10.65.4"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeStatefulSet, ID: "ownerstsPod", Owner: ns + "/" + sts1Name}
				ippool.Status.AllocatedIPs["10.10.65.5"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeStatefulSet, ID: "ownerstsPod", Owner: ns + "/sts-unexist"}
				ippool.Status.AllocatedIPs["10.10.65.7"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeStatefulSet, ID: "ownerstsPod", Owner: ns + "/sts-unexist"}
				Expect(k8sClient.Status().Update(ctx, &ippool)).Should(Succeed())
			})
			It("clean stale IP", func() {
				time.Sleep(period)
				Eventually(func(g Gomega) {
					ippool := v1alpha1.IPPool{}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
					By("shouldn't change offset")
					g.Expect(ippool.Status.Offset).Should(Equal(int64(1)))
					By("should cleanup stale IP for StatefulSet")
					g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(3))
					g.Expect(ippool.Status.AllocatedIPs).Should(HaveKey("10.10.65.1"))
					g.Expect(ippool.Status.AllocatedIPs).Should(HaveKey("10.10.65.2"))
					g.Expect(ippool.Status.AllocatedIPs).Should(HaveKey("10.10.65.4"))
					g.Expect(ippool.Status.AllocatedIPs).ShouldNot(HaveKey("10.10.65.5"))
					g.Expect(ippool.Status.AllocatedIPs).ShouldNot(HaveKey("10.10.65.7"))
					By("another pool doesn't change")
					ippool2 := v1alpha1.IPPool{}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool2"}, &ippool2)).Should(Succeed())
					g.Expect(ippool2.Status.AllocatedIPs).Should(BeNil())
				}, timeout, interval).Should(Succeed())
			})
		})
	})

	Context("multi pool has stale IP in pool", func() {
		BeforeEach(func() {
			ippool := v1alpha1.IPPool{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
			ippool.Status.Offset = 1
			ippool.Status.AllocatedIPs = make(map[string]v1alpha1.AllocateInfo)
			ippool.Status.AllocatedIPs["10.10.65.1"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypePod, ID: ns + "/" + pod1Name}
			ippool.Status.AllocatedIPs["10.10.65.2"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeCNIUsed, ID: "dfgggg"}
			ippool.Status.AllocatedIPs["10.10.65.3"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypePod, ID: ns + "/" + "pod-unexist"}
			ippool.Status.AllocatedIPs["10.10.65.4"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeStatefulSet, ID: "ownerstsPod", Owner: ns + "/" + sts1Name}
			ippool.Status.AllocatedIPs["10.10.65.5"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeStatefulSet, ID: "ownerstsPod", Owner: ns + "/sts-unexist"}
			Expect(k8sClient.Status().Update(ctx, &ippool)).Should(Succeed())
			ippool2 := v1alpha1.IPPool{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool2"}, &ippool2)).Should(Succeed())
			ippool2.Status.Offset = constants.IPPoolOffsetFull
			ippool2.Status.AllocatedIPs = make(map[string]v1alpha1.AllocateInfo)
			ippool2.Status.AllocatedIPs["12.10.65.1"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypePod, ID: ns + "/" + pod2Name}
			ippool2.Status.AllocatedIPs["12.10.65.2"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeCNIUsed, ID: "dfgggg"}
			ippool2.Status.AllocatedIPs["12.10.65.3"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypePod, ID: ns + "/" + "pod-unexist"}
			ippool2.Status.AllocatedIPs["12.10.65.4"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeStatefulSet, ID: "ownerstsPod", Owner: ns + "/" + sts1Name}
			ippool2.Status.AllocatedIPs["12.10.65.5"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeStatefulSet, ID: "ownerstsPod", Owner: ns + "/sts-unexist"}
			ippool2.Status.AllocatedIPs["12.10.65.7"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeStatefulSet, ID: "ownerstsPod", Owner: ns + "/sts-unexist"}

			Expect(k8sClient.Status().Update(ctx, &ippool2)).Should(Succeed())
		})
		It("clean stale ip", func() {
			time.Sleep(period)
			Eventually(func(g Gomega) {
				ippool := v1alpha1.IPPool{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
				By("shouldn't change offset")
				g.Expect(ippool.Status.Offset).Should(Equal(int64(1)))
				By("should cleanup stale IP")
				g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(3))
				g.Expect(ippool.Status.AllocatedIPs).Should(HaveKey("10.10.65.1"))
				g.Expect(ippool.Status.AllocatedIPs).Should(HaveKey("10.10.65.2"))
				g.Expect(ippool.Status.AllocatedIPs).Should(HaveKey("10.10.65.4"))
				g.Expect(ippool.Status.AllocatedIPs).ShouldNot(HaveKey("10.10.65.3"))
				g.Expect(ippool.Status.AllocatedIPs).ShouldNot(HaveKey("10.10.65.5"))

				ippool2 := v1alpha1.IPPool{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool2"}, &ippool2)).Should(Succeed())
				By("should reset offset")
				g.Expect(ippool2.Status.Offset).Should(Equal(int64(constants.IPPoolOffsetReset)))
				By("should cleanup stale IP")
				g.Expect(ippool2.Status.AllocatedIPs).Should(HaveKey("12.10.65.1"))
				g.Expect(ippool2.Status.AllocatedIPs).Should(HaveKey("12.10.65.2"))
				g.Expect(ippool2.Status.AllocatedIPs).Should(HaveKey("12.10.65.4"))
				g.Expect(ippool2.Status.AllocatedIPs).ShouldNot(HaveKey("12.10.65.3"))
				g.Expect(ippool2.Status.AllocatedIPs).ShouldNot(HaveKey("12.10.65.5"))
				g.Expect(ippool2.Status.AllocatedIPs).ShouldNot(HaveKey("12.10.65.7"))
			}, timeout, interval).Should(Succeed())
		})
	})
})
