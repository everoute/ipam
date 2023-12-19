package ipam

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
				Type:             v1alpha1.AllocateTypePod,
				AllocateIdentify: "containerid",
				K8sPodName:       "podname",
				K8sPodNs:         "podns",
			},
			exp: v1alpha1.AllocateInfo{
				Type: v1alpha1.AllocateTypePod,
				ID:   "podns/podname",
			},
		},
		{
			name: "type cniused",
			conf: NetConf{
				Type:             v1alpha1.AllocateTypeCNIUsed,
				AllocateIdentify: "identify",
			},
			exp: v1alpha1.AllocateInfo{
				Type: v1alpha1.AllocateTypeCNIUsed,
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
				Type:             v1alpha1.AllocateTypePod,
				K8sPodName:       "podName",
				K8sPodNs:         "podNs",
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
				Type:       v1alpha1.AllocateTypePod,
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
				Type:       v1alpha1.AllocateTypePod,
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
				Type:             v1alpha1.AllocateTypeCNIUsed,
				AllocateIdentify: "123456",
				K8sPodName:       "podName",
				K8sPodNs:         "podNs",
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
				Type:       v1alpha1.AllocateTypePod,
				K8sPodName: "podName",
				K8sPodNs:   "podNs",
			},
			ippool: v1alpha1.IPPool{
				Status: v1alpha1.IPPoolStatus{
					AllocatedIPs: makeAllocateStatus("10.1.1.1", "podNs/podName", "pod", "10.1.1.2", "podNs/podName", "cniused"),
				},
			},
			exp: "10.1.1.1",
		},
		{
			name: "reallocate IP with pool",
			conf: NetConf{
				Pool:             "pool1",
				Type:             v1alpha1.AllocateTypeCNIUsed,
				AllocateIdentify: "123455",
				K8sPodName:       "podName",
				K8sPodNs:         "podNs",
			},
			ippool: v1alpha1.IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pool1",
				},
				Status: v1alpha1.IPPoolStatus{
					AllocatedIPs: makeAllocateStatus("10.1.1.1", "podNs/podName", "pod", "10.1.1.2", "123455", "cniused"),
				},
			},
			exp: "10.1.1.2",
		},
		{
			name: "rellocate IP with static IP",
			conf: NetConf{
				Pool:             "pool1",
				IP:               "10.1.1.3",
				Type:             v1alpha1.AllocateTypeCNIUsed,
				AllocateIdentify: "123455",
				K8sPodName:       "podName",
				K8sPodNs:         "podNs",
			},
			ippool: v1alpha1.IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pool1",
				},
				Status: v1alpha1.IPPoolStatus{
					AllocatedIPs: makeAllocateStatus("10.1.1.1", "podNs/podName", "pod", "10.1.1.3", "123455", "cniused"),
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

var _ = Describe("ipam", func() {
	pool1mask := "255.255.240.0"
	pool1GW := "10.10.64.1"
	pool2mask := "255.255.255.248"
	pool2GW := "12.10.64.2"
	pool1 := v1alpha1.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pool1",
			Namespace: ns,
		},
		Spec: v1alpha1.IPPoolSpec{
			CIDR:    "10.10.65.0/30",
			Subnet:  "10.10.64.0/20",
			Gateway: pool1GW,
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
			Gateway: pool2GW,
		},
	}
	AfterEach(func() {
		Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.IPPool{}, client.InNamespace(ns))).Should(Succeed())
	})

	Context("allocate IP", func() {
		When("pool cidr is part of subnet", func() {
			BeforeEach(func() {
				pool1Copy := pool1.DeepCopy()
				Expect(k8sClient.Create(ctx, pool1Copy)).Should(Succeed())
			})

			Context("ippool has enough IP", func() {
				It("first allocate IP in ippool for type pod", func() {
					c := NetConf{
						Type:       v1alpha1.AllocateTypePod,
						K8sPodName: "pod1",
						K8sPodNs:   "ns1",
					}
					res, err := ipam.ExecAdd(&c)
					Expect(err).ToNot(HaveOccurred())
					exp := makeCNIIPconfig("10.10.65.0", pool1mask, pool1GW)
					Expect(*res.IPs[0]).To(Equal(*exp))
					Eventually(func(g Gomega) {
						ippool := v1alpha1.IPPool{}
						g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
						g.Expect(ippool.Status).ShouldNot(BeNil())
						g.Expect(ippool.Status.Offset).Should(Equal(int64(1)))
						g.Expect(ippool.Status.UsedIps).Should(BeNil())
						g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(1))
						g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("10.10.65.0", v1alpha1.AllocateInfo{ID: "ns1/pod1", Type: v1alpha1.AllocateTypePod}))
					}, timeout, interval).Should(Succeed())
				})
				When("ippool has allocated some IP", func() {
					BeforeEach(func() {
						ippool := v1alpha1.IPPool{}
						Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
						ippool.Status = v1alpha1.IPPoolStatus{
							Offset:       1,
							AllocatedIPs: makeAllocateStatus("10.10.65.0", "ns-exist/pod-exist", "pod"),
						}
						Expect(k8sClient.Status().Update(ctx, &ippool)).Should(Succeed())
					})
					It("allocate next IP for type cniused", func() {
						c := NetConf{
							Type:             v1alpha1.AllocateTypeCNIUsed,
							K8sPodName:       "pod1",
							K8sPodNs:         "ns1",
							AllocateIdentify: "identity",
						}
						res, err := ipam.ExecAdd(&c)
						Expect(err).ToNot(HaveOccurred())
						exp := makeCNIIPconfig("10.10.65.1", pool1mask, pool1GW)
						Expect(*res.IPs[0]).To(Equal(*exp))
						Eventually(func(g Gomega) {
							ippool := v1alpha1.IPPool{}
							g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
							g.Expect(ippool.Status).ShouldNot(BeNil())
							g.Expect(ippool.Status.Offset).Should(Equal(int64(2)))
							g.Expect(ippool.Status.UsedIps).Should(BeNil())
							g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(2))
							g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("10.10.65.0", v1alpha1.AllocateInfo{ID: "ns-exist/pod-exist", Type: v1alpha1.AllocateTypePod}))
							g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("10.10.65.1", v1alpha1.AllocateInfo{ID: "identity", Type: v1alpha1.AllocateTypeCNIUsed}))
						}, timeout, interval).Should(Succeed())
					})
				})
				When("ippool has allocated hole", func() {
					BeforeEach(func() {
						ippool := v1alpha1.IPPool{}
						Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
						ippool.Status = v1alpha1.IPPoolStatus{
							Offset:       1,
							UsedIps:      makeUsedIPStatus("10.10.65.0", "containerID"),
							AllocatedIPs: makeAllocateStatus("10.10.65.1", "ns-exist/pod-exist", "pod"),
						}
						Expect(k8sClient.Status().Update(ctx, &ippool)).Should(Succeed())
					})

					It("allocate valid IP", func() {
						c := NetConf{
							Type:       v1alpha1.AllocateTypePod,
							K8sPodName: "pod1",
							K8sPodNs:   "ns1",
						}
						res, err := ipam.ExecAdd(&c)
						Expect(err).ToNot(HaveOccurred())
						exp := makeCNIIPconfig("10.10.65.2", pool1mask, pool1GW)
						Expect(*res.IPs[0]).To(Equal(*exp))
						Eventually(func(g Gomega) {
							ippool := v1alpha1.IPPool{}
							g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
							g.Expect(ippool.Status).ShouldNot(BeNil())
							g.Expect(ippool.Status.Offset).Should(Equal(int64(3)))
							g.Expect(len(ippool.Status.UsedIps)).Should(Equal(1))
							g.Expect(ippool.Status.UsedIps).Should(HaveKeyWithValue("10.10.65.0", "containerID"))
							g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(2))
							g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("10.10.65.2", v1alpha1.AllocateInfo{ID: "ns1/pod1", Type: v1alpha1.AllocateTypePod}))
							g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("10.10.65.1", v1alpha1.AllocateInfo{ID: "ns-exist/pod-exist", Type: v1alpha1.AllocateTypePod}))
						}, timeout, interval).Should(Succeed())
					})
				})
			})

			When("all pool fulled", func() {
				BeforeEach(func() {
					ippool := v1alpha1.IPPool{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
					ippool.Status = v1alpha1.IPPoolStatus{
						Offset: -1,
					}
					Expect(k8sClient.Status().Update(ctx, &ippool)).Should(Succeed())
				})
				It("can't allocate IP", func() {
					c := NetConf{
						Type:       v1alpha1.AllocateTypePod,
						K8sPodName: "pod1",
						K8sPodNs:   "ns1",
					}
					res, err := ipam.ExecAdd(&c)
					Expect(res).Should(BeNil())
					Expect(err).Should(MatchError("no IP address allocated in all pools"))
				})
			})

			When("a pod request IP in a second time", func() {
				BeforeEach(func() {
					ippool := v1alpha1.IPPool{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
					ippool.Status = v1alpha1.IPPoolStatus{
						Offset:       2,
						UsedIps:      makeUsedIPStatus("10.10.65.0", "containerID"),
						AllocatedIPs: makeAllocateStatus("10.10.65.1", "ns-exist/pod-exist", "pod"),
					}
					Expect(k8sClient.Status().Update(ctx, &ippool)).Should(Succeed())
				})
				It("reallocate the same IP", func() {
					c := NetConf{
						Type:       v1alpha1.AllocateTypePod,
						K8sPodName: "pod-exist",
						K8sPodNs:   "ns-exist",
					}
					res, err := ipam.ExecAdd(&c)
					Expect(err).ToNot(HaveOccurred())
					exp := makeCNIIPconfig("10.10.65.1", pool1mask, pool1GW)
					Expect(*res.IPs[0]).To(Equal(*exp))
					Eventually(func(g Gomega) {
						ippool := v1alpha1.IPPool{}
						g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
						g.Expect(ippool.Status).ShouldNot(BeNil())
						g.Expect(ippool.Status.Offset).Should(Equal(int64(2)))
						g.Expect(len(ippool.Status.UsedIps)).Should(Equal(1))
						g.Expect(ippool.Status.UsedIps).Should(HaveKeyWithValue("10.10.65.0", "containerID"))
						g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(1))
						g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("10.10.65.1", v1alpha1.AllocateInfo{ID: "ns-exist/pod-exist", Type: v1alpha1.AllocateTypePod}))
					}, timeout, interval).Should(Succeed())
				})
			})
		})
		When("pool cidr is equal subnet", func() {
			BeforeEach(func() {
				pool2Copy := pool2.DeepCopy()
				Expect(k8sClient.Create(ctx, pool2Copy)).Should(Succeed())
			})
			It("skip subnet first IP", func() {
				c := NetConf{
					Type:       v1alpha1.AllocateTypePod,
					K8sPodName: "pod1",
					K8sPodNs:   "ns1",
				}
				res, err := ipam.ExecAdd(&c)
				Expect(err).ToNot(HaveOccurred())
				exp := makeCNIIPconfig("12.10.64.1", pool2mask, pool2GW)
				Expect(*res.IPs[0]).To(Equal(*exp))
				Eventually(func(g Gomega) {
					ippool := v1alpha1.IPPool{}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool2"}, &ippool)).Should(Succeed())
					g.Expect(ippool.Status).ShouldNot(BeNil())
					g.Expect(ippool.Status.Offset).Should(Equal(int64(2)))
					g.Expect(ippool.Status.UsedIps).Should(BeNil())
					g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(1))
					g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("12.10.64.1", v1alpha1.AllocateInfo{ID: "ns1/pod1", Type: v1alpha1.AllocateTypePod}))
				}, timeout, interval).Should(Succeed())
			})
			When("next IP is gateway", func() {
				BeforeEach(func() {
					ippool := v1alpha1.IPPool{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool2"}, &ippool)).Should(Succeed())
					ippool.Status = v1alpha1.IPPoolStatus{
						Offset:  2,
						UsedIps: makeUsedIPStatus("12.10.64.1", "containerID"),
					}
					Expect(k8sClient.Status().Update(ctx, &ippool)).Should(Succeed())
				})
				It("skip gateway", func() {
					c := NetConf{
						Type:       v1alpha1.AllocateTypePod,
						K8sPodName: "pod1",
						K8sPodNs:   "ns1",
					}
					res, err := ipam.ExecAdd(&c)
					Expect(err).ToNot(HaveOccurred())
					exp := makeCNIIPconfig("12.10.64.3", pool2mask, pool2GW)
					Expect(*res.IPs[0]).To(Equal(*exp))
					Eventually(func(g Gomega) {
						ippool := v1alpha1.IPPool{}
						g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool2"}, &ippool)).Should(Succeed())
						g.Expect(ippool.Status).ShouldNot(BeNil())
						g.Expect(ippool.Status.Offset).Should(Equal(int64(4)))
						g.Expect(len(ippool.Status.UsedIps)).Should(Equal(1))
						g.Expect(ippool.Status.UsedIps).Should(HaveKeyWithValue("12.10.64.1", "containerID"))
						g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(1))
						g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("12.10.64.3", v1alpha1.AllocateInfo{ID: "ns1/pod1", Type: v1alpha1.AllocateTypePod}))
					}, timeout, interval).Should(Succeed())
				})
			})
			When("next IP is subnet last IP", func() {
				BeforeEach(func() {
					ippool := v1alpha1.IPPool{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool2"}, &ippool)).Should(Succeed())
					ippool.Status = v1alpha1.IPPoolStatus{
						Offset:       7,
						UsedIps:      makeUsedIPStatus("12.10.64.1", "containerID"),
						AllocatedIPs: makeAllocateStatus("12.10.64.3", "n1/p1", "pod", "12.10.64.4", "n2/p2", "pod", "12.10.64.5", "n3/p3", "pod", "12.10.64.6", "n4/p4", "pod"),
					}
					Expect(k8sClient.Status().Update(ctx, &ippool)).Should(Succeed())
				})
				It("can't allocate IP", func() {
					c := NetConf{
						Type:       v1alpha1.AllocateTypePod,
						K8sPodName: "pod1",
						K8sPodNs:   "ns1",
					}
					res, err := ipam.ExecAdd(&c)
					Expect(res).Should(BeNil())
					Expect(err).Should(MatchError("find valid ip error in pool pool2"))
				})
			})
		})
		When("multi pool", func() {
			BeforeEach(func() {
				pool1Copy := pool1.DeepCopy()
				Expect(k8sClient.Create(ctx, pool1Copy)).Should(Succeed())
				pool2Copy := pool2.DeepCopy()
				Expect(k8sClient.Create(ctx, pool2Copy)).Should(Succeed())
			})
			When("first pool fulled", func() {
				BeforeEach(func() {
					ippool := v1alpha1.IPPool{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
					ippool.Status = v1alpha1.IPPoolStatus{
						Offset: -1,
					}
					Expect(k8sClient.Status().Update(ctx, &ippool)).Should(Succeed())
				})
				It("allocate from secondary pool", func() {
					c := NetConf{
						Type:       v1alpha1.AllocateTypePod,
						K8sPodName: "pod1",
						K8sPodNs:   "ns1",
					}
					res, err := ipam.ExecAdd(&c)
					Expect(err).ToNot(HaveOccurred())
					exp := makeCNIIPconfig("12.10.64.1", pool2mask, pool2GW)
					Expect(*res.IPs[0]).To(Equal(*exp))
					Eventually(func(g Gomega) {
						ippool := v1alpha1.IPPool{}
						g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool2"}, &ippool)).Should(Succeed())
						g.Expect(ippool.Status).ShouldNot(BeNil())
						g.Expect(ippool.Status.Offset).Should(Equal(int64(2)))
						g.Expect(ippool.Status.UsedIps).Should(BeNil())
						g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(1))
						g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("12.10.64.1", v1alpha1.AllocateInfo{ID: "ns1/pod1", Type: v1alpha1.AllocateTypePod}))
					}, timeout, interval).Should(Succeed())
				})
			})
			When("specify ippool", func() {
				It("allocate IP from specified pool", func() {
					c := NetConf{
						Pool:       "pool2",
						Type:       v1alpha1.AllocateTypePod,
						K8sPodName: "pod1",
						K8sPodNs:   "ns1",
					}
					res, err := ipam.ExecAdd(&c)
					Expect(err).ToNot(HaveOccurred())
					exp := makeCNIIPconfig("12.10.64.1", pool2mask, pool2GW)
					Expect(*res.IPs[0]).To(Equal(*exp))
					Eventually(func(g Gomega) {
						ippool := v1alpha1.IPPool{}
						g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool2"}, &ippool)).Should(Succeed())
						g.Expect(ippool.Status).ShouldNot(BeNil())
						g.Expect(ippool.Status.Offset).Should(Equal(int64(2)))
						g.Expect(ippool.Status.UsedIps).Should(BeNil())
						g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(1))
						g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("12.10.64.1", v1alpha1.AllocateInfo{ID: "ns1/pod1", Type: v1alpha1.AllocateTypePod}))
					}, timeout, interval).Should(Succeed())
				})
				When("a pod request IP in a second time", func() {
					BeforeEach(func() {
						ippool := v1alpha1.IPPool{}
						Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool2"}, &ippool)).Should(Succeed())
						ippool.Status = v1alpha1.IPPoolStatus{
							Offset:       4,
							AllocatedIPs: makeAllocateStatus("12.10.64.3", "ns1/pod1", "pod"),
						}
						Expect(k8sClient.Status().Update(ctx, &ippool)).Should(Succeed())
					})
					When("current IP in another pool", func() {
						It("allocate new IP from specified pool", func() {
							c := NetConf{
								Pool:       "pool1",
								Type:       v1alpha1.AllocateTypePod,
								K8sPodName: "pod1",
								K8sPodNs:   "ns1",
							}
							res, err := ipam.ExecAdd(&c)
							Expect(err).ToNot(HaveOccurred())
							exp := makeCNIIPconfig("10.10.65.0", pool1mask, pool1GW)
							Expect(*res.IPs[0]).To(Equal(*exp))
							Eventually(func(g Gomega) {
								ippool := v1alpha1.IPPool{}
								g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
								g.Expect(ippool.Status).ShouldNot(BeNil())
								g.Expect(ippool.Status.Offset).Should(Equal(int64(1)))
								g.Expect(ippool.Status.UsedIps).Should(BeNil())
								g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(1))
								g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("10.10.65.0", v1alpha1.AllocateInfo{ID: "ns1/pod1", Type: v1alpha1.AllocateTypePod}))
							}, timeout, interval).Should(Succeed())
						})
					})
					When("current IP in specified pool", func() {
						It("reallocate the same IP", func() {
							c := NetConf{
								Pool:       "pool2",
								Type:       v1alpha1.AllocateTypePod,
								K8sPodName: "pod1",
								K8sPodNs:   "ns1",
							}
							res, err := ipam.ExecAdd(&c)
							Expect(err).ToNot(HaveOccurred())
							exp := makeCNIIPconfig("12.10.64.3", pool2mask, pool2GW)
							Expect(*res.IPs[0]).To(Equal(*exp))
							Eventually(func(g Gomega) {
								ippool := v1alpha1.IPPool{}
								g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool2"}, &ippool)).Should(Succeed())
								g.Expect(ippool.Status).ShouldNot(BeNil())
								g.Expect(ippool.Status.Offset).Should(Equal(int64(4)))
								g.Expect(ippool.Status.UsedIps).Should(BeNil())
								g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(1))
								g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("12.10.64.3", v1alpha1.AllocateInfo{ID: "ns1/pod1", Type: v1alpha1.AllocateTypePod}))
							}, timeout, interval).Should(Succeed())
						})
					})
				})
				When("specified ippool doesn't exist", func() {
					It("can't allocate IP", func() {
						c := NetConf{
							Pool:       "pool-unexist",
							Type:       v1alpha1.AllocateTypePod,
							K8sPodName: "pod1",
							K8sPodNs:   "ns1",
						}
						res, err := ipam.ExecAdd(&c)
						Expect(res).Should(BeNil())
						Expect(err).ShouldNot(BeNil())
					})
				})
			})
			When("specify IP", func() {
				It("allocate specified IP", func() {
					c := NetConf{
						Pool:       "pool1",
						IP:         "10.10.65.3",
						Type:       v1alpha1.AllocateTypePod,
						K8sPodName: "pod1",
						K8sPodNs:   "ns1",
					}
					res, err := ipam.ExecAdd(&c)
					Expect(err).ToNot(HaveOccurred())
					exp := makeCNIIPconfig("10.10.65.3", pool1mask, pool1GW)
					Expect(*res.IPs[0]).To(Equal(*exp))
					Eventually(func(g Gomega) {
						ippool := v1alpha1.IPPool{}
						g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
						g.Expect(ippool.Status).ShouldNot(BeNil())
						g.Expect(ippool.Status.Offset).Should(Equal(int64(0)))
						g.Expect(ippool.Status.UsedIps).Should(BeNil())
						g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(1))
						g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("10.10.65.3", v1alpha1.AllocateInfo{ID: "ns1/pod1", Type: v1alpha1.AllocateTypePod}))
					}, timeout, interval).Should(Succeed())
				})

				It("allocate specified IP for type statefulset", func() {
					c := NetConf{
						Pool:       "pool1",
						IP:         "10.10.65.3",
						Type:       v1alpha1.AllocateTypeStatefulSet,
						K8sPodName: "pod1",
						K8sPodNs:   "ns1",
						Owner:      "ns1/sts1",
					}
					res, err := ipam.ExecAdd(&c)
					Expect(err).ToNot(HaveOccurred())
					exp := makeCNIIPconfig("10.10.65.3", pool1mask, pool1GW)
					Expect(*res.IPs[0]).To(Equal(*exp))
					Eventually(func(g Gomega) {
						ippool := v1alpha1.IPPool{}
						g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
						g.Expect(ippool.Status).ShouldNot(BeNil())
						g.Expect(ippool.Status.Offset).Should(Equal(int64(0)))
						g.Expect(ippool.Status.UsedIps).Should(BeNil())
						g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(1))
						g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("10.10.65.3", v1alpha1.AllocateInfo{ID: "ns1/pod1", Type: v1alpha1.AllocateTypeStatefulSet, Owner: "ns1/sts1"}))
					}, timeout, interval).Should(Succeed())
				})
				When("a pod request IP in a second time for type pod", func() {
					BeforeEach(func() {
						ippool := v1alpha1.IPPool{}
						Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool2"}, &ippool)).Should(Succeed())
						ippool.Status = v1alpha1.IPPoolStatus{
							Offset:       6,
							AllocatedIPs: makeAllocateStatus("12.10.64.5", "ns1/pod1", "pod"),
						}
						Expect(k8sClient.Status().Update(ctx, &ippool)).Should(Succeed())
					})
					When("current IP is different from specified IP", func() {
						It("allocate new IP", func() {
							c := NetConf{
								Pool:       "pool2",
								IP:         "12.10.64.6",
								Type:       v1alpha1.AllocateTypePod,
								K8sPodName: "pod1",
								K8sPodNs:   "ns1",
							}
							res, err := ipam.ExecAdd(&c)
							Expect(err).ToNot(HaveOccurred())
							exp := makeCNIIPconfig("12.10.64.6", pool2mask, pool2GW)
							Expect(*res.IPs[0]).To(Equal(*exp))
							Eventually(func(g Gomega) {
								ippool := v1alpha1.IPPool{}
								g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool2"}, &ippool)).Should(Succeed())
								g.Expect(ippool.Status).ShouldNot(BeNil())
								g.Expect(ippool.Status.Offset).Should(Equal(int64(6)))
								g.Expect(ippool.Status.UsedIps).Should(BeNil())
								g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(2))
								g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("12.10.64.5", v1alpha1.AllocateInfo{ID: "ns1/pod1", Type: v1alpha1.AllocateTypePod}))
								g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("12.10.64.6", v1alpha1.AllocateInfo{ID: "ns1/pod1", Type: v1alpha1.AllocateTypePod}))
							}, timeout, interval).Should(Succeed())
						})
					})
					When("current IP is same as specified IP", func() {
						It("reallocate the same IP", func() {
							c := NetConf{
								Pool:       "pool2",
								IP:         "12.10.64.5",
								Type:       v1alpha1.AllocateTypePod,
								K8sPodName: "pod1",
								K8sPodNs:   "ns1",
							}
							res, err := ipam.ExecAdd(&c)
							Expect(err).ToNot(HaveOccurred())
							exp := makeCNIIPconfig("12.10.64.5", pool2mask, pool2GW)
							Expect(*res.IPs[0]).To(Equal(*exp))
							Eventually(func(g Gomega) {
								ippool := v1alpha1.IPPool{}
								g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool2"}, &ippool)).Should(Succeed())
								g.Expect(ippool.Status).ShouldNot(BeNil())
								g.Expect(ippool.Status.Offset).Should(Equal(int64(6)))
								g.Expect(ippool.Status.UsedIps).Should(BeNil())
								g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(1))
								g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("12.10.64.5", v1alpha1.AllocateInfo{ID: "ns1/pod1", Type: v1alpha1.AllocateTypePod}))
							}, timeout, interval).Should(Succeed())
						})
					})
				})
				When("a pod request IP in a second time for type statefulset", func() {
					BeforeEach(func() {
						ippool := v1alpha1.IPPool{}
						Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool2"}, &ippool)).Should(Succeed())
						ippool.Status = v1alpha1.IPPoolStatus{
							Offset: 6,
						}
						ippool.Status.AllocatedIPs = make(map[string]v1alpha1.AllocateInfo)
						ippool.Status.AllocatedIPs["12.10.64.5"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeStatefulSet, Owner: "ns1/sts1", ID: "ns1/pod1"}
						Expect(k8sClient.Status().Update(ctx, &ippool)).Should(Succeed())
					})
					It("reallocate the same IP", func() {
						c := NetConf{
							Pool:       "pool2",
							IP:         "12.10.64.5",
							Type:       v1alpha1.AllocateTypeStatefulSet,
							K8sPodName: "pod1",
							K8sPodNs:   "ns1",
							Owner:      "ns1/sts1",
						}
						res, err := ipam.ExecAdd(&c)
						Expect(err).ToNot(HaveOccurred())
						exp := makeCNIIPconfig("12.10.64.5", pool2mask, pool2GW)
						Expect(*res.IPs[0]).To(Equal(*exp))
						Eventually(func(g Gomega) {
							ippool := v1alpha1.IPPool{}
							g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool2"}, &ippool)).Should(Succeed())
							g.Expect(ippool.Status).ShouldNot(BeNil())
							g.Expect(ippool.Status.Offset).Should(Equal(int64(6)))
							g.Expect(ippool.Status.UsedIps).Should(BeNil())
							g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(1))
							g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("12.10.64.5", v1alpha1.AllocateInfo{ID: "ns1/pod1", Type: v1alpha1.AllocateTypeStatefulSet, Owner: "ns1/sts1"}))
						}, timeout, interval).Should(Succeed())
					})
					It("can't reallocate IP for type different", func() {
						c := NetConf{
							Pool:       "pool2",
							IP:         "12.10.64.5",
							Type:       v1alpha1.AllocateTypePod,
							K8sPodName: "pod1",
							K8sPodNs:   "ns1",
						}
						_, err := ipam.ExecAdd(&c)
						allocateInfo := v1alpha1.AllocateInfo{ID: "ns1/pod1", Type: v1alpha1.AllocateTypeStatefulSet, Owner: "ns1/sts1"}
						Expect(err).Should(MatchError(fmt.Sprintf("static ip %s is already in use by %v", c.IP, allocateInfo)))
					})
					It("can't reallocate IP for owner different", func() {
						c := NetConf{
							Pool:       "pool2",
							IP:         "12.10.64.5",
							Type:       v1alpha1.AllocateTypeStatefulSet,
							K8sPodName: "pod1",
							K8sPodNs:   "ns1",
							Owner:      "ns1/sts2",
						}
						_, err := ipam.ExecAdd(&c)
						allocateInfo := v1alpha1.AllocateInfo{ID: "ns1/pod1", Type: v1alpha1.AllocateTypeStatefulSet, Owner: "ns1/sts1"}
						Expect(err).Should(MatchError(fmt.Sprintf("static ip %s is already in use by %v", c.IP, allocateInfo)))
					})
				})
				When("specified static IP has been used", func() {
					BeforeEach(func() {
						ippool := v1alpha1.IPPool{}
						Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool2"}, &ippool)).Should(Succeed())
						ippool.Status = v1alpha1.IPPoolStatus{
							Offset:  6,
							UsedIps: makeUsedIPStatus("12.10.64.5", "containerID"),
						}
						Expect(k8sClient.Status().Update(ctx, &ippool)).Should(Succeed())
					})
					It("can't allocate IP", func() {
						c := NetConf{
							Pool:       "pool2",
							IP:         "12.10.64.5",
							Type:       v1alpha1.AllocateTypePod,
							K8sPodName: "pod1",
							K8sPodNs:   "ns1",
						}
						res, err := ipam.ExecAdd(&c)
						Expect(res).Should(BeNil())
						Expect(err).Should(MatchError("static ip 12.10.64.5 is already in use"))
					})
				})
				When("specified static IP has been allocated", func() {
					BeforeEach(func() {
						ippool := v1alpha1.IPPool{}
						Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool2"}, &ippool)).Should(Succeed())
						ippool.Status = v1alpha1.IPPoolStatus{
							Offset:       6,
							AllocatedIPs: makeAllocateStatus("12.10.64.5", "ns-exist/pod-exist", "pod"),
						}
						Expect(k8sClient.Status().Update(ctx, &ippool)).Should(Succeed())
					})
					It("can't allocate IP", func() {
						c := NetConf{
							Pool:       "pool2",
							IP:         "12.10.64.5",
							Type:       v1alpha1.AllocateTypePod,
							K8sPodName: "pod1",
							K8sPodNs:   "ns1",
						}
						res, err := ipam.ExecAdd(&c)
						Expect(res).Should(BeNil())
						Expect(err).Should(MatchError(fmt.Sprintf("static ip 12.10.64.5 is already in use by %v",
							v1alpha1.AllocateInfo{ID: "ns-exist/pod-exist", Type: v1alpha1.AllocateTypePod})))
					})
				})
				When("specified static IP doesn't in pool", func() {
					It("can't allocate IP", func() {
						c := NetConf{
							Pool:       "pool2",
							IP:         "13.10.64.5",
							Type:       v1alpha1.AllocateTypePod,
							K8sPodName: "pod1",
							K8sPodNs:   "ns1",
						}
						res, err := ipam.ExecAdd(&c)
						Expect(res).Should(BeNil())
						Expect(err).Should(MatchError("static ip 13.10.64.5 is not in target pool"))
					})
				})
				When("specified invalid IP", func() {
					It("can't allocate IP", func() {
						c := NetConf{
							Pool:       "pool2",
							IP:         "13.10.64",
							Type:       v1alpha1.AllocateTypePod,
							K8sPodName: "pod1",
							K8sPodNs:   "ns1",
						}
						res, err := ipam.ExecAdd(&c)
						Expect(res).Should(BeNil())
						Expect(err).Should(MatchError("invalid static ip 13.10.64"))
					})
				})
				When("specified ippool doesn't exist", func() {
					It("can't allocate IP", func() {
						c := NetConf{
							Pool:       "pool-unexist",
							IP:         "12.10.64.5",
							Type:       v1alpha1.AllocateTypePod,
							K8sPodName: "pod1",
							K8sPodNs:   "ns1",
						}
						res, err := ipam.ExecAdd(&c)
						Expect(res).Should(BeNil())
						Expect(err).ShouldNot(BeNil())
					})
				})
			})
		})
	})

	Context("release IP", func() {
		BeforeEach(func() {
			pool1Copy := pool1.DeepCopy()
			Expect(k8sClient.Create(ctx, pool1Copy)).Should(Succeed())
			ippool := v1alpha1.IPPool{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
			ippool.Status = v1alpha1.IPPoolStatus{
				Offset:       3,
				UsedIps:      makeUsedIPStatus("10.10.64.2", "containerID"),
				AllocatedIPs: makeAllocateStatus("10.10.65.1", "ns1/pod1", "pod", "10.10.65.3", "cniusedID", "cniused"),
			}
			Expect(k8sClient.Status().Update(ctx, &ippool)).Should(Succeed())
		})
		It("release by usedIP", func() {
			c := NetConf{
				Type:             v1alpha1.AllocateTypePod,
				AllocateIdentify: "containerID",
				K8sPodName:       "pod-unexist",
				K8sPodNs:         "ns-unexist",
			}
			_ = ipam.ExecDel(&c)
			Eventually(func(g Gomega) {
				ippool := v1alpha1.IPPool{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
				g.Expect(ippool.Status).ShouldNot(BeNil())
				g.Expect(ippool.Status.Offset).Should(Equal(int64(3)))
				g.Expect(len(ippool.Status.UsedIps)).Should(Equal(0))
				g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(2))
			}, timeout, interval).Should(Succeed())
		})
		When("release by allocate info", func() {
			It("for type cniused", func() {
				c := NetConf{
					Type:             v1alpha1.AllocateTypeCNIUsed,
					AllocateIdentify: "cniusedID",
				}
				_ = ipam.ExecDel(&c)
				Eventually(func(g Gomega) {
					ippool := v1alpha1.IPPool{}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
					g.Expect(ippool.Status).ShouldNot(BeNil())
					g.Expect(ippool.Status.Offset).Should(Equal(int64(3)))
					g.Expect(len(ippool.Status.UsedIps)).Should(Equal(1))
					g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(1))
					g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("10.10.65.1", v1alpha1.AllocateInfo{ID: "ns1/pod1", Type: v1alpha1.AllocateTypePod}))
				}, timeout, interval).Should(Succeed())
			})
			When("for type pod", func() {
				It("release for allocate type pod", func() {
					c := NetConf{
						Type:             v1alpha1.AllocateTypePod,
						AllocateIdentify: "cniusedID",
						K8sPodName:       "pod1",
						K8sPodNs:         "ns1",
					}
					_ = ipam.ExecDel(&c)
					Eventually(func(g Gomega) {
						ippool := v1alpha1.IPPool{}
						g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
						g.Expect(ippool.Status).ShouldNot(BeNil())
						g.Expect(ippool.Status.Offset).Should(Equal(int64(3)))
						g.Expect(len(ippool.Status.UsedIps)).Should(Equal(1))
						g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(1))
						g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("10.10.65.3", v1alpha1.AllocateInfo{ID: "cniusedID", Type: v1alpha1.AllocateTypeCNIUsed}))
					}, timeout, interval).Should(Succeed())
				})

				When("for allocate type statefulset", func() {
					BeforeEach(func() {
						ippool := v1alpha1.IPPool{}
						Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
						ippool.Status.AllocatedIPs["10.10.65.1"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeStatefulSet, ID: "ns1/pod1", Owner: "ns1/sts1"}
						Expect(k8sClient.Status().Update(ctx, &ippool)).Should(Succeed())
					})
					It("doesn't release", func() {
						c := NetConf{
							Type:             v1alpha1.AllocateTypePod,
							AllocateIdentify: "cniusedID",
							K8sPodName:       "pod1",
							K8sPodNs:         "ns1",
						}
						_ = ipam.ExecDel(&c)
						Eventually(func(g Gomega) {
							ippool := v1alpha1.IPPool{}
							g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
							g.Expect(ippool.Status).ShouldNot(BeNil())
							g.Expect(ippool.Status.Offset).Should(Equal(int64(3)))
							g.Expect(len(ippool.Status.UsedIps)).Should(Equal(1))
							g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(2))
							g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("10.10.65.1", v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeStatefulSet, ID: "ns1/pod1", Owner: "ns1/sts1"}))
							g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("10.10.65.3", v1alpha1.AllocateInfo{ID: "cniusedID", Type: v1alpha1.AllocateTypeCNIUsed}))
						}, timeout, interval).Should(Succeed())
					})
				})
			})
		})
		It("release unexist", func() {
			c := NetConf{
				Type:       v1alpha1.AllocateTypePod,
				K8sPodName: "pod-unexist",
				K8sPodNs:   "ns-unexist",
			}
			_ = ipam.ExecDel(&c)
			Eventually(func(g Gomega) {
				ippool := v1alpha1.IPPool{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
				g.Expect(ippool.Status).ShouldNot(BeNil())
				g.Expect(ippool.Status.Offset).Should(Equal(int64(3)))
				g.Expect(len(ippool.Status.UsedIps)).Should(Equal(1))
				g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(2))
			})
		})
		When("multi pool", func() {
			BeforeEach(func() {
				pool2Copy := pool2.DeepCopy()
				Expect(k8sClient.Create(ctx, pool2Copy)).Should(Succeed())
				ippool := v1alpha1.IPPool{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool2"}, &ippool)).Should(Succeed())
				ippool.Status = v1alpha1.IPPoolStatus{
					Offset:       3,
					AllocatedIPs: makeAllocateStatus("12.10.64.1", "ns1/pod1", "pod"),
				}
				Expect(k8sClient.Status().Update(ctx, &ippool)).Should(Succeed())
			})
			It("release by usedIP and allocateinfo", func() {
				c := NetConf{
					Type:             v1alpha1.AllocateTypePod,
					AllocateIdentify: "containerID",
					K8sPodName:       "pod1",
					K8sPodNs:         "ns1",
				}
				_ = ipam.ExecDel(&c)
				Eventually(func(g Gomega) {
					ippool := v1alpha1.IPPool{}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &ippool)).Should(Succeed())
					g.Expect(ippool.Status).ShouldNot(BeNil())
					g.Expect(ippool.Status.Offset).Should(Equal(int64(3)))
					g.Expect(len(ippool.Status.UsedIps)).Should(Equal(0))
					g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(1))
					g.Expect(ippool.Status.AllocatedIPs).Should(HaveKeyWithValue("10.10.65.3", v1alpha1.AllocateInfo{ID: "cniusedID", Type: v1alpha1.AllocateTypeCNIUsed}))

					ippool = v1alpha1.IPPool{}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool2"}, &ippool)).Should(Succeed())
					g.Expect(ippool.Status).ShouldNot(BeNil())
					g.Expect(ippool.Status.Offset).Should(Equal(int64(3)))
					g.Expect(ippool.Status.UsedIps).Should(BeNil())
					g.Expect(len(ippool.Status.AllocatedIPs)).Should(Equal(0))
				}, timeout, interval).Should(Succeed())
			})
		})
	})
})
