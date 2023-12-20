package controller

import (
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

var _ = Describe("statefulset_controller", func() {
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
	BeforeEach(func() {
		By("setup resources")
		pool := pool1.DeepCopy()
		Expect(k8sClient.Create(ctx, pool)).Should(Succeed())
		sts := sts1.DeepCopy()
		Expect(k8sClient.Create(ctx, sts)).Should(Succeed())
	})
	AfterEach(func() {
		By("clean resources")
		Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.IPPool{}, client.InNamespace(ns))).Should(Succeed())
		Expect(k8sClient.DeleteAllOf(ctx, &appsv1.StatefulSet{}, client.InNamespace(ns))).Should(Succeed())
	})
	Context("ippool full", func() {
		BeforeEach(func() {
			ippool := v1alpha1.IPPool{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
			ippool.Status.Offset = constants.IPPoolOffsetFull
			ippool.Status.AllocatedIPs = make(map[string]v1alpha1.AllocateInfo)
			ippool.Status.AllocatedIPs["10.10.65.1"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypePod, ID: ns + "/pod1"}
			ippool.Status.AllocatedIPs["10.10.65.6"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeCNIUsed, ID: "dfgggg"}
			ippool.Status.AllocatedIPs["10.10.65.2"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeStatefulSet, ID: "ownerstsPod", Owner: ns + "/" + sts1Name}
			ippool.Status.AllocatedIPs["10.10.65.4"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeStatefulSet, ID: "ownerstsPod", Owner: ns + "/" + sts1Name}
			ippool.Status.AllocatedIPs["10.10.65.5"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeStatefulSet, ID: "ownerstsPod", Owner: ns + "/stsname2"}
			Expect(k8sClient.Status().Update(ctx, &ippool)).Should(Succeed())
		})
		When("delete statefulset", func() {
			BeforeEach(func() {
				sts := appsv1.StatefulSet{}
				stsNsName := types.NamespacedName{Namespace: ns, Name: sts1Name}
				Expect(k8sClient.Get(ctx, stsNsName, &sts)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, &sts)).Should(Succeed())
			})
			It("release ip and reset ippool offset", func() {
				Eventually(func(g Gomega) {
					ippool := v1alpha1.IPPool{}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
					By("check reset offset")
					g.Expect(ippool.Status.Offset).Should(Equal(int64(constants.IPPoolOffsetReset)))
					By("check release ip")
					g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(3))
					g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("10.10.65.1", v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypePod, ID: ns + "/pod1"}))
					g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("10.10.65.6", v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeCNIUsed, ID: "dfgggg"}))
					g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("10.10.65.5", v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeStatefulSet, ID: "ownerstsPod", Owner: ns + "/stsname2"}))
					g.Expect(ippool.Status.AllocatedIPs).ShouldNot(HaveKey("10.10.65.2"))
					g.Expect(ippool.Status.AllocatedIPs).ShouldNot(HaveKey("10.10.65.4"))
				}, timeout, interval).Should(Succeed())
			})
		})
	})

	Context("ippool doesn't full", func() {
		BeforeEach(func() {
			ippool := v1alpha1.IPPool{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
			ippool.Status.Offset = 3
			ippool.Status.AllocatedIPs = make(map[string]v1alpha1.AllocateInfo)
			ippool.Status.AllocatedIPs["10.10.65.1"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypePod, ID: ns + "/pod1"}
			ippool.Status.AllocatedIPs["10.10.65.6"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeCNIUsed, ID: "dfgggg"}
			ippool.Status.AllocatedIPs["10.10.65.2"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeStatefulSet, ID: "ownerstsPod", Owner: ns + "/" + sts1Name}
			ippool.Status.AllocatedIPs["10.10.65.4"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeStatefulSet, ID: "ownerstsPod", Owner: ns + "/" + sts1Name}
			ippool.Status.AllocatedIPs["10.10.65.5"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeStatefulSet, ID: "ownerstsPod", Owner: ns + "/stsname2"}
			Expect(k8sClient.Status().Update(ctx, &ippool)).Should(Succeed())
		})
		When("delete statefulset", func() {
			BeforeEach(func() {
				sts := appsv1.StatefulSet{}
				stsNsName := types.NamespacedName{Namespace: ns, Name: sts1Name}
				Expect(k8sClient.Get(ctx, stsNsName, &sts)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, &sts)).Should(Succeed())
			})
			It("only release ip", func() {
				Eventually(func(g Gomega) {
					ippool := v1alpha1.IPPool{}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
					By("check doesn't change offset")
					g.Expect(ippool.Status.Offset).Should(Equal(int64(3)))
					By("check release ip")
					g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(3))
					g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("10.10.65.1", v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypePod, ID: ns + "/pod1"}))
					g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("10.10.65.6", v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeCNIUsed, ID: "dfgggg"}))
					g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("10.10.65.5", v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeStatefulSet, ID: "ownerstsPod", Owner: ns + "/stsname2"}))
					g.Expect(ippool.Status.AllocatedIPs).ShouldNot(HaveKey("10.10.65.2"))
					g.Expect(ippool.Status.AllocatedIPs).ShouldNot(HaveKey("10.10.65.4"))
				}, timeout, interval).Should(Succeed())
			})
		})
	})
})
