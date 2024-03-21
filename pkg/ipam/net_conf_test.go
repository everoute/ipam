package ipam

import (
	"fmt"
	"testing"

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

var _ = Describe("net_conf_complete", func() {
	podname := "pod1"
	podLabel := make(map[string]string)
	podLabel["owner"] = "sts-sts1"
	pod1 := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podname,
			Namespace: ns,
			Labels:    podLabel,
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
	pool1 := v1alpha1.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pool1",
			Namespace: ns,
		},
		Spec: v1alpha1.IPPoolSpec{
			CIDR:    "10.10.65.0/29",
			Except:  []string{"10.10.65.7/32"},
			Subnet:  "10.10.64.0/20",
			Gateway: "10.10.64.1",
		},
	}
	AfterEach(func() {
		Expect(k8sClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(ns))).Should(Succeed())
		Expect(k8sClient.DeleteAllOf(ctx, &appsv1.StatefulSet{}, client.InNamespace(ns))).Should(Succeed())
		Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.IPPool{}, client.InNamespace(ns))).Should(Succeed())
	})

	Context("type cniused", func() {
		When("netconf has been set k8sinfo", func() {
			When("k8s pod annotation set static ip", func() {
				BeforeEach(func() {
					pod := pod1.DeepCopy()
					pod.Annotations = make(map[string]string, 2)
					pod.Annotations[constants.IpamAnnotationPool] = "pool1"
					pod.Annotations[constants.IpamAnnotationStaticIP] = "10.10.10.1"
					Expect(k8sClient.Create(ctx, pod)).Should(Succeed())
				})
				When("netconf has been set pool info", func() {
					var c *NetConf
					BeforeEach(func() {
						c = &NetConf{
							Type:       v1alpha1.AllocateTypeCNIUsed,
							K8sPodName: podname,
							K8sPodNs:   ns,
							Pool:       "pool2",
						}
					})
					It("ip and pool doesn't changed by k8sinfo", func() {
						Expect(c.Complete(ctx, k8sClient, ns)).Should(Succeed())
						Expect(c.Pool).Should(Equal("pool2"))
						Expect(c.IP).Should(Equal(""))
					})
				})
				When("netconf doesn't set pool info", func() {
					var c *NetConf
					BeforeEach(func() {
						c = &NetConf{
							Type:       v1alpha1.AllocateTypeCNIUsed,
							K8sPodName: podname,
							K8sPodNs:   ns,
						}
					})
					It("shouldn't set ip and pool by k8sinfo", func() {
						Expect(c.Complete(ctx, k8sClient, ns)).Should(Succeed())
						Expect(c.Pool).Should(Equal(""))
						Expect(c.IP).Should(Equal(""))
					})
				})
			})
		})
	})

	Context("type pod", func() {
		When("k8s pod annotation set pool", func() {
			BeforeEach(func() {
				pod := pod1.DeepCopy()
				pod.Annotations = make(map[string]string, 2)
				pod.Annotations[constants.IpamAnnotationPool] = "pool1"
				Expect(k8sClient.Create(ctx, pod)).Should(Succeed())
			})
			When("netconf set ippool", func() {
				var c *NetConf
				BeforeEach(func() {
					c = &NetConf{
						Type:       v1alpha1.AllocateTypePod,
						K8sPodName: podname,
						K8sPodNs:   ns,
						Pool:       "pool2",
					}
				})
				It("netconf pool doesn't changed by k8sinfo", func() {
					Expect(c.Complete(ctx, k8sClient, ns)).Should(Succeed())
					Expect(c.Pool).Should(Equal("pool2"))
					Expect(c.IP).Should(Equal(""))
				})
			})
			When("netconf set static ip", func() {
				var c *NetConf
				BeforeEach(func() {
					c = &NetConf{
						Type:       v1alpha1.AllocateTypePod,
						K8sPodName: podname,
						K8sPodNs:   ns,
						Pool:       "pool2",
						IP:         "10.10.10.2",
					}
				})
				It("netconf pool and ip doesn't changed by k8sinfo", func() {
					Expect(c.Complete(ctx, k8sClient, ns)).Should(Succeed())
					Expect(c.Pool).Should(Equal("pool2"))
					Expect(c.IP).Should(Equal("10.10.10.2"))
				})
			})
			When("netconf doesn't set pool and ip", func() {
				When("netconf has been set k8sinfo", func() {
					var c *NetConf
					BeforeEach(func() {
						c = &NetConf{
							Type:       v1alpha1.AllocateTypePod,
							K8sPodName: podname,
							K8sPodNs:   ns,
						}
					})
					It("netconf will set pool by k8sinfo", func() {
						Expect(c.Complete(ctx, k8sClient, ns)).Should(Succeed())
						Expect(c.Pool).Should(Equal("pool1"))
						Expect(c.IP).Should(Equal(""))
					})
				})
				When("netconf doesn't set k8sinfo", func() {
					var c *NetConf
					BeforeEach(func() {
						c = &NetConf{
							Type: v1alpha1.AllocateTypePod,
						}
					})
					It("error for netconf is invalid", func() {
						err := c.Complete(ctx, k8sClient, ns)
						Expect(err).Should(MatchError(fmt.Sprintf("must set K8sPodNs and K8sPodName for type %s", v1alpha1.AllocateTypePod)))
						Expect(c.Pool).Should(Equal(""))
						Expect(c.IP).Should(Equal(""))
					})
				})
			})
		})
		When("k8s pod annotation set static ip", func() {
			Context("k8s pod annotation set ip and pool", func() {
				BeforeEach(func() {
					pod := pod1.DeepCopy()
					pod.Annotations = make(map[string]string, 2)
					pod.Annotations[constants.IpamAnnotationPool] = "pool1"
					pod.Annotations[constants.IpamAnnotationStaticIP] = "10.10.10.1"
					Expect(k8sClient.Create(ctx, pod)).Should(Succeed())
				})
				When("netconf set ippool", func() {
					When("netconf ippool is different to pool setted by pod annotation", func() {
						var c *NetConf
						BeforeEach(func() {
							c = &NetConf{
								Type:       v1alpha1.AllocateTypePod,
								K8sPodName: podname,
								K8sPodNs:   ns,
								Pool:       "pool2",
							}
						})
						It("netconf pool and ip doesn't changed by k8sinfo", func() {
							Expect(c.Complete(ctx, k8sClient, ns)).Should(Succeed())
							Expect(c.Pool).Should(Equal("pool2"))
							Expect(c.IP).Should(Equal(""))
						})
					})
					When("netconf ippool is same as pool setted by pod annotation", func() {
						var c *NetConf
						BeforeEach(func() {
							c = &NetConf{
								Type:       v1alpha1.AllocateTypePod,
								K8sPodName: podname,
								K8sPodNs:   ns,
								Pool:       "pool1",
							}
						})
						It("netconf doesn't set ip by k8sinfo", func() {
							Expect(c.Complete(ctx, k8sClient, ns)).Should(Succeed())
							Expect(c.Pool).Should(Equal("pool1"))
							Expect(c.IP).Should(Equal(""))
						})
					})
				})
				When("netconf set static ip", func() {
					When("netconf ippool is different to pool setted by pod annotation", func() {
						var c *NetConf
						BeforeEach(func() {
							c = &NetConf{
								Type:       v1alpha1.AllocateTypePod,
								K8sPodName: podname,
								K8sPodNs:   ns,
								Pool:       "pool2",
								IP:         "10.10.10.2",
							}
						})
						It("netconf pool and ip doesn't changed by k8sinfo", func() {
							Expect(c.Complete(ctx, k8sClient, ns)).Should(Succeed())
							Expect(c.Pool).Should(Equal("pool2"))
							Expect(c.IP).Should(Equal("10.10.10.2"))
						})
					})
					When("netconf ippool is same as pool setted by pod annotation", func() {
						var c *NetConf
						BeforeEach(func() {
							c = &NetConf{
								Type:       v1alpha1.AllocateTypePod,
								K8sPodName: podname,
								K8sPodNs:   ns,
								Pool:       "pool1",
								IP:         "10.10.10.2",
							}
						})
						It("netconf ip doesn't changed by k8sinfo", func() {
							Expect(c.Complete(ctx, k8sClient, ns)).Should(Succeed())
							Expect(c.Pool).Should(Equal("pool1"))
							Expect(c.IP).Should(Equal("10.10.10.2"))
						})
					})
				})
				When("netconf doesn't set pool and ip", func() {
					When("netconf doesn't set k8sinfo", func() {
						var c *NetConf
						BeforeEach(func() {
							c = &NetConf{
								Type: v1alpha1.AllocateTypePod,
							}
						})
						It("error for netconf is invalid", func() {
							err := c.Complete(ctx, k8sClient, ns)
							Expect(err).Should(MatchError(fmt.Sprintf("must set K8sPodNs and K8sPodName for type %s", v1alpha1.AllocateTypePod)))
							Expect(c.Pool).Should(Equal(""))
							Expect(c.IP).Should(Equal(""))
						})
					})
					When("netconf set exist k8s info", func() {
						var c *NetConf
						BeforeEach(func() {
							c = &NetConf{
								Type:       v1alpha1.AllocateTypePod,
								K8sPodName: podname,
								K8sPodNs:   ns,
							}
						})
						It("netconf will set pool and ip by k8sinfo", func() {
							Expect(c.Complete(ctx, k8sClient, ns)).Should(Succeed())
							Expect(c.Pool).Should(Equal("pool1"))
							Expect(c.IP).Should(Equal("10.10.10.1"))
						})
					})
				})
			})
			Context("k8s pod annotation only set ip", func() {
				BeforeEach(func() {
					pod := pod1.DeepCopy()
					pod.Annotations = make(map[string]string, 1)
					pod.Annotations[constants.IpamAnnotationStaticIP] = "10.10.10.1"
					Expect(k8sClient.Create(ctx, pod)).Should(Succeed())
				})
				It("error for pod annotation only set ip", func() {
					c := &NetConf{
						Type:       v1alpha1.AllocateTypePod,
						K8sPodName: podname,
						K8sPodNs:   ns,
					}
					err := c.Complete(ctx, k8sClient, ns)
					Expect(err).Should(MatchError("can't only specify static IP but no pool"))
				})
			})
		})
		When("k8s pod doesn't set annotation", func() {
			BeforeEach(func() {
				pod := pod1.DeepCopy()
				Expect(k8sClient.Create(ctx, pod)).Should(Succeed())
			})
			When("netconf set ippool", func() {
				var c *NetConf
				BeforeEach(func() {
					c = &NetConf{
						Type:       v1alpha1.AllocateTypePod,
						K8sPodName: podname,
						K8sPodNs:   ns,
						Pool:       "pool2",
					}
				})
				It("netconf doesn't changed", func() {
					Expect(c.Complete(ctx, k8sClient, ns)).Should(Succeed())
					Expect(c.Pool).Should(Equal("pool2"))
					Expect(c.IP).Should(Equal(""))
				})
			})
			When("netconf doesn't set ippool", func() {
				var c *NetConf
				BeforeEach(func() {
					c = &NetConf{
						Type:       v1alpha1.AllocateTypePod,
						K8sPodName: podname,
						K8sPodNs:   ns,
					}
				})
				It("netconf doesn't changed", func() {
					Expect(c.Complete(ctx, k8sClient, ns)).Should(Succeed())
					Expect(c.Pool).Should(Equal(""))
					Expect(c.IP).Should(Equal(""))
				})
			})
		})
		When("k8s pod doesn't exist", func() {
			When("netconf has been set ippool", func() {
				var c *NetConf
				BeforeEach(func() {
					c = &NetConf{
						Type:       v1alpha1.AllocateTypePod,
						K8sPodName: podname,
						K8sPodNs:   ns,
						Pool:       "pool2",
					}
				})
				It("netconf doesn't changed", func() {
					Expect(c.Complete(ctx, k8sClient, ns)).Should(Succeed())
					Expect(c.Pool).Should(Equal("pool2"))
					Expect(c.IP).Should(Equal(""))
				})
			})
			When("netconf doesn't set ippool", func() {
				var c *NetConf
				BeforeEach(func() {
					c = &NetConf{
						Type:       v1alpha1.AllocateTypePod,
						K8sPodName: podname,
						K8sPodNs:   ns,
					}
				})
				It("error for can't find pod", func() {
					err := c.Complete(ctx, k8sClient, ns)
					Expect(err).ShouldNot(BeNil())
					Expect(c.Pool).Should(Equal(""))
					Expect(c.IP).Should(Equal(""))
				})
			})
		})
	})

	Context("type statefulset", func() {
		stsName := "sts1"
		sts1 := appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      stsName,
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
		Context("statefulset specify ippool", func() {
			BeforeEach(func() {
				sts := sts1.DeepCopy()
				sts.Annotations = make(map[string]string, 1)
				sts.Annotations[constants.IpamAnnotationPool] = "pool1"
				Expect(k8sClient.Create(ctx, sts)).Should(Succeed())
				pod := pod1.DeepCopy()
				pod.OwnerReferences = make([]metav1.OwnerReference, 1)
				pod.OwnerReferences[0] = metav1.OwnerReference{Kind: constants.KindStatefulSet, Name: stsName, UID: sts.GetUID(), APIVersion: "apps/v1"}
				Expect(k8sClient.Create(ctx, pod)).Should(Succeed())
			})
			When("pod specify another ippool", func() {
				BeforeEach(func() {
					pod := corev1.Pod{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: podname}, &pod)).Should(Succeed())
					pod.Annotations = make(map[string]string, 1)
					pod.Annotations[constants.IpamAnnotationPool] = "pool2"
					Expect(k8sClient.Update(ctx, &pod)).Should(Succeed())
				})
				It("netconf set ippool specified by pod", func() {
					c := NetConf{
						K8sPodName: podname,
						K8sPodNs:   ns,
						Type:       v1alpha1.AllocateTypePod,
					}
					Expect(c.Complete(ctx, k8sClient, ns)).Should(Succeed())
					Expect(c.Type).Should(Equal(v1alpha1.AllocateTypePod))
					Expect(c.Pool).Should(Equal("pool2"))
				})
			})
			It("netconf set ipool specified by statefulset", func() {
				c := NetConf{
					K8sPodName: podname,
					K8sPodNs:   ns,
					Type:       v1alpha1.AllocateTypePod,
				}
				Expect(c.Complete(ctx, k8sClient, ns)).Should(Succeed())
				Expect(c.Type).Should(Equal(v1alpha1.AllocateTypePod))
				Expect(c.Pool).Should(Equal("pool1"))
				Expect(c.Owner).Should(Equal(""))
			})
		})
		Context("statefulset specify ip-list", func() {
			BeforeEach(func() {
				pool := pool1.DeepCopy()
				Expect(k8sClient.Create(ctx, pool)).Should(Succeed())
				sts := sts1.DeepCopy()
				sts.Annotations = make(map[string]string, 2)
				sts.Annotations[constants.IpamAnnotationPool] = "pool1"
				sts.Annotations[constants.IpamAnnotationIPList] = "10.10.65.1,10.10.65.2"
				Expect(k8sClient.Create(ctx, sts)).Should(Succeed())
				pod := pod1.DeepCopy()
				pod.OwnerReferences = make([]metav1.OwnerReference, 1)
				pod.OwnerReferences[0] = metav1.OwnerReference{Kind: constants.KindStatefulSet, Name: stsName, UID: sts.GetUID(), APIVersion: "apps/v1"}
				Expect(k8sClient.Create(ctx, pod)).Should(Succeed())
			})
			When("pod specify static IP", func() {
				BeforeEach(func() {
					pod := corev1.Pod{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: podname}, &pod)).Should(Succeed())
					pod.Annotations = make(map[string]string, 2)
					pod.Annotations[constants.IpamAnnotationPool] = "pool1"
					pod.Annotations[constants.IpamAnnotationStaticIP] = "10.10.65.3"
					Expect(k8sClient.Update(ctx, &pod)).Should(Succeed())
				})
				It("netconf set ip specified by pod static IP", func() {
					c := NetConf{
						K8sPodName: podname,
						K8sPodNs:   ns,
						Type:       v1alpha1.AllocateTypePod,
					}
					Expect(c.Complete(ctx, k8sClient, ns)).Should(Succeed())
					Expect(c.Type).Should(Equal(v1alpha1.AllocateTypePod))
					Expect(c.Pool).Should(Equal("pool1"))
					Expect(c.IP).Should(Equal("10.10.65.3"))
					Expect(c.Owner).Should(Equal(""))
				})
			})
			When("ip-list is not in ippool", func() {
				BeforeEach(func() {
					sts := appsv1.StatefulSet{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: stsName}, &sts)).Should(Succeed())
					sts.Annotations[constants.IpamAnnotationIPList] = "12.10.65.1"
					Expect(k8sClient.Update(ctx, &sts)).Should(Succeed())
				})
				It("netconf set ip failed", func() {
					c := NetConf{
						K8sPodName: podname,
						K8sPodNs:   ns,
						Type:       v1alpha1.AllocateTypePod,
					}
					err := c.Complete(ctx, k8sClient, ns)
					stsNsName := types.NamespacedName{Namespace: ns, Name: stsName}
					ipList := []string{"12.10.65.1"}
					Expect(err).Should(MatchError(fmt.Sprintf("no valid or unallocate ip in statefulset %v ip list %v", stsNsName, ipList)))
				})
			})
			When("ip-list is in ippool except", func() {
				BeforeEach(func() {
					sts := appsv1.StatefulSet{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: stsName}, &sts)).Should(Succeed())
					sts.Annotations[constants.IpamAnnotationIPList] = "12.10.65.7"
					Expect(k8sClient.Update(ctx, &sts)).Should(Succeed())
				})
				It("netconf set ip failed", func() {
					c := NetConf{
						K8sPodName: podname,
						K8sPodNs:   ns,
						Type:       v1alpha1.AllocateTypePod,
					}
					err := c.Complete(ctx, k8sClient, ns)
					stsNsName := types.NamespacedName{Namespace: ns, Name: stsName}
					ipList := []string{"12.10.65.7"}
					Expect(err).Should(MatchError(fmt.Sprintf("no valid or unallocate ip in statefulset %v ip list %v", stsNsName, ipList)))
				})
			})
			When("ip-list is not in ippool start-end", func() {
				BeforeEach(func() {
					p := v1alpha1.IPPool{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: pool1.Name}, &p)).Should(Succeed())
					p.Spec.CIDR = ""
					p.Spec.Except = nil
					p.Spec.Start = "10.10.65.0"
					p.Spec.End = "10.10.65.7"
					Expect(k8sClient.Update(ctx, &p)).Should(Succeed())
					sts := appsv1.StatefulSet{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: stsName}, &sts)).Should(Succeed())
					sts.Annotations[constants.IpamAnnotationIPList] = "10.10.65.9"
					Expect(k8sClient.Update(ctx, &sts)).Should(Succeed())
				})
				It("netconf set ip failed", func() {
					c := NetConf{
						K8sPodName: podname,
						K8sPodNs:   ns,
						Type:       v1alpha1.AllocateTypePod,
					}
					err := c.Complete(ctx, k8sClient, ns)
					stsNsName := types.NamespacedName{Namespace: ns, Name: stsName}
					ipList := []string{"10.10.65.9"}
					Expect(err).Should(MatchError(fmt.Sprintf("no valid or unallocate ip in statefulset %v ip list %v", stsNsName, ipList)))
				})
			})
			When("ip-list is empty string", func() {
				BeforeEach(func() {
					sts := appsv1.StatefulSet{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: stsName}, &sts)).Should(Succeed())
					sts.Annotations[constants.IpamAnnotationIPList] = ""
					Expect(k8sClient.Update(ctx, &sts)).Should(Succeed())
				})

				It("netconf set ip failed", func() {
					c := NetConf{
						K8sPodName: podname,
						K8sPodNs:   ns,
						Type:       v1alpha1.AllocateTypePod,
					}
					err := c.Complete(ctx, k8sClient, ns)
					stsNsName := types.NamespacedName{Namespace: ns, Name: stsName}
					ipList := []string{""}
					Expect(err).Should(MatchError(fmt.Sprintf("no valid or unallocate ip in statefulset %v ip list %v", stsNsName, ipList)))
				})
			})
			When("ip-list is invalid ip", func() {
				BeforeEach(func() {
					sts := appsv1.StatefulSet{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: stsName}, &sts)).Should(Succeed())
					sts.Annotations[constants.IpamAnnotationIPList] = "10.1.2,10.3.4"
					Expect(k8sClient.Update(ctx, &sts)).Should(Succeed())
				})

				It("netconf set ip failed", func() {
					c := NetConf{
						K8sPodName: podname,
						K8sPodNs:   ns,
						Type:       v1alpha1.AllocateTypePod,
					}
					err := c.Complete(ctx, k8sClient, ns)
					stsNsName := types.NamespacedName{Namespace: ns, Name: stsName}
					ipList := []string{"10.1.2", "10.3.4"}
					Expect(err).Should(MatchError(fmt.Sprintf("no valid or unallocate ip in statefulset %v ip list %v", stsNsName, ipList)))
				})
			})
			When("ip-list has been allocated", func() {
				BeforeEach(func() {
					pool := v1alpha1.IPPool{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &pool)).Should(Succeed())
					pool.Status.AllocatedIPs = make(map[string]v1alpha1.AllocateInfo)
					pool.Status.AllocatedIPs["10.10.65.1"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeCNIUsed, ID: "cniusedid"}
					pool.Status.AllocatedIPs["10.10.65.2"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypePod, ID: "ns-unexist/pod-unexist"}
					pool.Status.AllocatedIPs["10.10.65.4"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypePod, ID: ns + "/" + podname}
					pool.Status.UsedIps = make(map[string]string)
					pool.Status.UsedIps["10.10.65.3"] = "containerid"
					Expect(k8sClient.Status().Update(ctx, &pool)).Should(Succeed())

					sts := appsv1.StatefulSet{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: stsName}, &sts)).Should(Succeed())
					sts.Annotations[constants.IpamAnnotationIPList] = "10.10.65.1,10.10.65.2,10.10.65.3,10.10.65.4"
					Expect(k8sClient.Update(ctx, &sts)).Should(Succeed())
				})
				It("netconf set ip failed", func() {
					c := NetConf{
						K8sPodName: podname,
						K8sPodNs:   ns,
						Type:       v1alpha1.AllocateTypePod,
					}
					err := c.Complete(ctx, k8sClient, ns)
					stsNsName := types.NamespacedName{Namespace: ns, Name: stsName}
					ipList := []string{"10.10.65.1", "10.10.65.2", "10.10.65.3", "10.10.65.4"}
					Expect(err).Should(MatchError(fmt.Sprintf("no valid or unallocate ip in statefulset %v ip list %v", stsNsName, ipList)))
				})
			})
			When("first allocate ip from ip-list", func() {
				BeforeEach(func() {
					sts := appsv1.StatefulSet{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: stsName}, &sts)).Should(Succeed())
					sts.Annotations[constants.IpamAnnotationIPList] = "10.10.65.1,10.10.65.2,10.10.65.3,10.10.65.4"
					Expect(k8sClient.Update(ctx, &sts)).Should(Succeed())
				})
				It("netconf set ip from ip-list", func() {
					c := NetConf{
						K8sPodName: podname,
						K8sPodNs:   ns,
						Type:       v1alpha1.AllocateTypePod,
					}
					Expect(c.Complete(ctx, k8sClient, ns)).Should(Succeed())
					Expect(c.Type).Should(Equal(v1alpha1.AllocateTypeStatefulSet))
					Expect(c.Pool).Should(Equal("pool1"))
					Expect(c.IP).Should(BeElementOf("10.10.65.1", "10.10.65.2", "10.10.65.3", "10.10.65.4"))
					Expect(c.Owner).Should(Equal(ns + "/" + stsName))
				})
			})
			When("allocate ip from ip-list", func() {
				BeforeEach(func() {
					pool := v1alpha1.IPPool{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &pool)).Should(Succeed())
					pool.Status.AllocatedIPs = make(map[string]v1alpha1.AllocateInfo)
					pool.Status.AllocatedIPs["10.10.65.1"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeCNIUsed, ID: "cniusedid"}
					pool.Status.UsedIps = make(map[string]string)
					pool.Status.UsedIps["10.10.65.3"] = "containerid"
					Expect(k8sClient.Status().Update(ctx, &pool)).Should(Succeed())

					sts := appsv1.StatefulSet{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: stsName}, &sts)).Should(Succeed())
					sts.Annotations[constants.IpamAnnotationIPList] = "10.10.65.1,10.10.65.2,10.10.65.3,10.10.65.4"
					Expect(k8sClient.Update(ctx, &sts)).Should(Succeed())
				})
				It("netconf set ip from ip-list", func() {
					c := NetConf{
						K8sPodName: podname,
						K8sPodNs:   ns,
						Type:       v1alpha1.AllocateTypePod,
					}
					Expect(c.Complete(ctx, k8sClient, ns)).Should(Succeed())
					Expect(c.Type).Should(Equal(v1alpha1.AllocateTypeStatefulSet))
					Expect(c.Pool).Should(Equal("pool1"))
					Expect(c.IP).Should(BeElementOf("10.10.65.2", "10.10.65.4"))
					Expect(c.Owner).Should(Equal(ns + "/" + stsName))
				})
			})
			When("reallocate ip from ip-list", func() {
				BeforeEach(func() {
					pool := v1alpha1.IPPool{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "pool1"}, &pool)).Should(Succeed())
					pool.Status.AllocatedIPs = make(map[string]v1alpha1.AllocateInfo)
					pool.Status.AllocatedIPs["10.10.65.1"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeCNIUsed, ID: "cniusedid"}
					pool.Status.AllocatedIPs["10.10.65.4"] = v1alpha1.AllocateInfo{Type: v1alpha1.AllocateTypeStatefulSet, ID: ns + "/" + podname, Owner: ns + "/" + stsName}
					pool.Status.UsedIps = make(map[string]string)
					pool.Status.UsedIps["10.10.65.3"] = "containerid"
					Expect(k8sClient.Status().Update(ctx, &pool)).Should(Succeed())

					sts := appsv1.StatefulSet{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: stsName}, &sts)).Should(Succeed())
					sts.Annotations[constants.IpamAnnotationIPList] = "10.10.65.1,10.10.65.2,10.10.65.3,10.10.65.4"
					Expect(k8sClient.Update(ctx, &sts)).Should(Succeed())
				})
				It("netconf set same ip for pod reconfigure", func() {
					c := NetConf{
						K8sPodName: podname,
						K8sPodNs:   ns,
						Type:       v1alpha1.AllocateTypePod,
					}
					Expect(c.Complete(ctx, k8sClient, ns)).Should(Succeed())
					Expect(c.Type).Should(Equal(v1alpha1.AllocateTypeStatefulSet))
					Expect(c.Pool).Should(Equal("pool1"))
					Expect(c.IP).Should(Equal("10.10.65.4"))
					Expect(c.Owner).Should(Equal(ns + "/" + stsName))
				})
			})
		})
		Context("pod owner statefulset doesn't exist", func() {
			BeforeEach(func() {
				pod := pod1.DeepCopy()
				pod.OwnerReferences = make([]metav1.OwnerReference, 1)
				pod.OwnerReferences[0] = metav1.OwnerReference{Kind: constants.KindStatefulSet, Name: stsName, UID: "123456", APIVersion: "apps/v1"}
				Expect(k8sClient.Create(ctx, pod)).Should(Succeed())
			})
			It("netconf failed set ip and ippool", func() {
				c := NetConf{
					K8sPodName: podname,
					K8sPodNs:   ns,
					Type:       v1alpha1.AllocateTypePod,
				}
				Expect(c.Complete(ctx, k8sClient, ns)).ShouldNot(BeNil())
			})
		})
	})
})

func TestValid(t *testing.T) {
	tests := []struct {
		name    string
		c       NetConf
		isValid bool
	}{
		{
			name: "valid netconf for type cniused",
			c: NetConf{
				Type:             v1alpha1.AllocateTypeCNIUsed,
				AllocateIdentify: "cniusedid",
			},
			isValid: true,
		},
		{
			name: "valid for type pod with containerid",
			c: NetConf{
				Type:             v1alpha1.AllocateTypePod,
				AllocateIdentify: "containerid",
				K8sPodName:       "pod",
				K8sPodNs:         "ns",
			},
			isValid: true,
		},
		{
			name: "valid with pool",
			c: NetConf{
				Type:             v1alpha1.AllocateTypePod,
				AllocateIdentify: "containerid",
				K8sPodName:       "pod",
				K8sPodNs:         "ns",
				Pool:             "pool",
			},
			isValid: true,
		},
		{
			name: "valid with ip",
			c: NetConf{
				Type:             v1alpha1.AllocateTypePod,
				K8sPodName:       "pod",
				K8sPodNs:         "ns",
				Pool:             "pool",
				IP:               "10.10.2.3",
				AllocateIdentify: "cid",
			},
			isValid: true,
		},
		{
			name: "no k8sPodName for type pod",
			c: NetConf{
				Type:             v1alpha1.AllocateTypePod,
				AllocateIdentify: "containerd",
				K8sPodNs:         "ns",
			},
			isValid: false,
		},
		{
			name: "no k8sPodNs for type pod",
			c: NetConf{
				Type:             v1alpha1.AllocateTypePod,
				K8sPodName:       "pod",
				AllocateIdentify: "cid",
			},
			isValid: false,
		},
		{
			name: "no AllocateIdentify for type pod",
			c: NetConf{
				Type:       v1alpha1.AllocateTypePod,
				K8sPodName: "pod",
				K8sPodNs:   "ns",
			},
			isValid: false,
		},
		{
			name: "no AllocateIdentify for type cniused",
			c: NetConf{
				Type:       v1alpha1.AllocateTypeCNIUsed,
				K8sPodName: "pod",
				K8sPodNs:   "ns",
			},
			isValid: false,
		},
		{
			name: "valid for type statefulset",
			c: NetConf{
				Type:       v1alpha1.AllocateTypeStatefulSet,
				K8sPodName: "pod",
				K8sPodNs:   "ns",
				Owner:      "ns/ss",
				Pool:       "pool1",
				IP:         "10.10.1.1",
			},
			isValid: true,
		},
		{
			name: "no owner for type statefulset",
			c: NetConf{
				Type:             v1alpha1.AllocateTypeStatefulSet,
				K8sPodName:       "pod",
				K8sPodNs:         "ns",
				Pool:             "pool1",
				IP:               "10.10.1.1",
				AllocateIdentify: "cid",
			},
			isValid: false,
		},
		{
			name: "no ippool for type statefulset",
			c: NetConf{
				Type:             v1alpha1.AllocateTypeStatefulSet,
				K8sPodName:       "pod",
				K8sPodNs:         "ns",
				Owner:            "ns/ss",
				IP:               "10.10.1.1",
				AllocateIdentify: "cid",
			},
			isValid: false,
		},
		{
			name: "no ip for type statefulset",
			c: NetConf{
				Type:             v1alpha1.AllocateTypeStatefulSet,
				K8sPodName:       "pod",
				K8sPodNs:         "ns",
				Owner:            "ns/ss",
				Pool:             "pool1",
				AllocateIdentify: "cid",
			},
			isValid: false,
		},
	}

	for _, item := range tests {
		err := item.c.Valid()
		if (err == nil) != item.isValid {
			t.Errorf("test %s failed, expect err %s == nil is %v", item.name, err, item.isValid)
		}
	}
}
