package ipam

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/everoute/ipam/api/ipam/v1alpha1"
	"github.com/everoute/ipam/pkg/constants"
)

var _ = Describe("net_conf_complete", func() {
	podname := "pod1"
	pod1 := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podname,
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
	AfterEach(func() {
		Expect(k8sClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(ns))).Should(Succeed())
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
							Type:       v1alpha1.AllocatedTypeCNIUsed,
							K8sPodName: podname,
							K8sPodNs:   ns,
							Pool:       "pool2",
						}
					})
					It("ip and pool doesn't changed by k8sinfo", func() {
						Expect(c.Complete(ctx, k8sClient)).Should(Succeed())
						Expect(c.Pool).Should(Equal("pool2"))
						Expect(c.IP).Should(Equal(""))
					})
				})
				When("netconf doesn't set pool info", func() {
					var c *NetConf
					BeforeEach(func() {
						c = &NetConf{
							Type:       v1alpha1.AllocatedTypeCNIUsed,
							K8sPodName: podname,
							K8sPodNs:   ns,
						}
					})
					It("shouldn't set ip and pool by k8sinfo", func() {
						Expect(c.Complete(ctx, k8sClient)).Should(Succeed())
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
						Type:       v1alpha1.AllocatedTypePod,
						K8sPodName: podname,
						K8sPodNs:   ns,
						Pool:       "pool2",
					}
				})
				It("netconf pool doesn't changed by k8sinfo", func() {
					Expect(c.Complete(ctx, k8sClient)).Should(Succeed())
					Expect(c.Pool).Should(Equal("pool2"))
					Expect(c.IP).Should(Equal(""))
				})
			})
			When("netconf set static ip", func() {
				var c *NetConf
				BeforeEach(func() {
					c = &NetConf{
						Type:       v1alpha1.AllocatedTypePod,
						K8sPodName: podname,
						K8sPodNs:   ns,
						Pool:       "pool2",
						IP:         "10.10.10.2",
					}
				})
				It("netconf pool and ip doesn't changed by k8sinfo", func() {
					Expect(c.Complete(ctx, k8sClient)).Should(Succeed())
					Expect(c.Pool).Should(Equal("pool2"))
					Expect(c.IP).Should(Equal("10.10.10.2"))
				})
			})
			When("netconf doesn't set pool and ip", func() {
				When("netconf has been set k8sinfo", func() {
					var c *NetConf
					BeforeEach(func() {
						c = &NetConf{
							Type:       v1alpha1.AllocatedTypePod,
							K8sPodName: podname,
							K8sPodNs:   ns,
						}
					})
					It("netconf will set pool by k8sinfo", func() {
						Expect(c.Complete(ctx, k8sClient)).Should(Succeed())
						Expect(c.Pool).Should(Equal("pool1"))
						Expect(c.IP).Should(Equal(""))
					})
				})
				When("netconf doesn't set k8sinfo", func() {
					var c *NetConf
					BeforeEach(func() {
						c = &NetConf{
							Type: v1alpha1.AllocatedTypePod,
						}
					})
					It("error for netconf is invalid", func() {
						err := c.Complete(ctx, k8sClient)
						Expect(err).Should(MatchError(fmt.Sprintf("must set K8sPodNs and K8sPodName for type %s", v1alpha1.AllocatedTypePod)))
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
								Type:       v1alpha1.AllocatedTypePod,
								K8sPodName: podname,
								K8sPodNs:   ns,
								Pool:       "pool2",
							}
						})
						It("netconf pool and ip doesn't changed by k8sinfo", func() {
							Expect(c.Complete(ctx, k8sClient)).Should(Succeed())
							Expect(c.Pool).Should(Equal("pool2"))
							Expect(c.IP).Should(Equal(""))
						})
					})
					When("netconf ippool is same as pool setted by pod annotation", func() {
						var c *NetConf
						BeforeEach(func() {
							c = &NetConf{
								Type:       v1alpha1.AllocatedTypePod,
								K8sPodName: podname,
								K8sPodNs:   ns,
								Pool:       "pool1",
							}
						})
						It("netconf doesn't set ip by k8sinfo", func() {
							Expect(c.Complete(ctx, k8sClient)).Should(Succeed())
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
								Type:       v1alpha1.AllocatedTypePod,
								K8sPodName: podname,
								K8sPodNs:   ns,
								Pool:       "pool2",
								IP:         "10.10.10.2",
							}
						})
						It("netconf pool and ip doesn't changed by k8sinfo", func() {
							Expect(c.Complete(ctx, k8sClient)).Should(Succeed())
							Expect(c.Pool).Should(Equal("pool2"))
							Expect(c.IP).Should(Equal("10.10.10.2"))
						})
					})
					When("netconf ippool is same as pool setted by pod annotation", func() {
						var c *NetConf
						BeforeEach(func() {
							c = &NetConf{
								Type:       v1alpha1.AllocatedTypePod,
								K8sPodName: podname,
								K8sPodNs:   ns,
								Pool:       "pool1",
								IP:         "10.10.10.2",
							}
						})
						It("netconf ip doesn't changed by k8sinfo", func() {
							Expect(c.Complete(ctx, k8sClient)).Should(Succeed())
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
								Type: v1alpha1.AllocatedTypePod,
							}
						})
						It("error for netconf is invalid", func() {
							err := c.Complete(ctx, k8sClient)
							Expect(err).Should(MatchError(fmt.Sprintf("must set K8sPodNs and K8sPodName for type %s", v1alpha1.AllocatedTypePod)))
							Expect(c.Pool).Should(Equal(""))
							Expect(c.IP).Should(Equal(""))
						})
					})
					When("netconf set exist k8s info", func() {
						var c *NetConf
						BeforeEach(func() {
							c = &NetConf{
								Type:       v1alpha1.AllocatedTypePod,
								K8sPodName: podname,
								K8sPodNs:   ns,
							}
						})
						It("netconf will set pool and ip by k8sinfo", func() {
							Expect(c.Complete(ctx, k8sClient)).Should(Succeed())
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
						Type:       v1alpha1.AllocatedTypePod,
						K8sPodName: podname,
						K8sPodNs:   ns,
					}
					err := c.Complete(ctx, k8sClient)
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
						Type:       v1alpha1.AllocatedTypePod,
						K8sPodName: podname,
						K8sPodNs:   ns,
						Pool:       "pool2",
					}
				})
				It("netconf doesn't changed", func() {
					Expect(c.Complete(ctx, k8sClient)).Should(Succeed())
					Expect(c.Pool).Should(Equal("pool2"))
					Expect(c.IP).Should(Equal(""))
				})
			})
			When("netconf doesn't set ippool", func() {
				var c *NetConf
				BeforeEach(func() {
					c = &NetConf{
						Type:       v1alpha1.AllocatedTypePod,
						K8sPodName: podname,
						K8sPodNs:   ns,
					}
				})
				It("netconf doesn't changed", func() {
					Expect(c.Complete(ctx, k8sClient)).Should(Succeed())
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
						Type:       v1alpha1.AllocatedTypePod,
						K8sPodName: podname,
						K8sPodNs:   ns,
						Pool:       "pool2",
					}
				})
				It("netconf doesn't changed", func() {
					Expect(c.Complete(ctx, k8sClient)).Should(Succeed())
					Expect(c.Pool).Should(Equal("pool2"))
					Expect(c.IP).Should(Equal(""))
				})
			})
			When("netconf doesn't set ippool", func() {
				var c *NetConf
				BeforeEach(func() {
					c = &NetConf{
						Type:       v1alpha1.AllocatedTypePod,
						K8sPodName: podname,
						K8sPodNs:   ns,
					}
				})
				It("error for can't find pod", func() {
					err := c.Complete(ctx, k8sClient)
					Expect(err).ShouldNot(BeNil())
					Expect(c.Pool).Should(Equal(""))
					Expect(c.IP).Should(Equal(""))
				})
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
				Type:             v1alpha1.AllocatedTypeCNIUsed,
				AllocateIdentify: "cniusedid",
			},
			isValid: true,
		},
		{
			name: "valid for type pod with containerid",
			c: NetConf{
				Type:             v1alpha1.AllocatedTypePod,
				AllocateIdentify: "containerid",
				K8sPodName:       "pod",
				K8sPodNs:         "ns",
			},
			isValid: true,
		},
		{
			name: "valid with pool",
			c: NetConf{
				Type:             v1alpha1.AllocatedTypePod,
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
				Type:       v1alpha1.AllocatedTypePod,
				K8sPodName: "pod",
				K8sPodNs:   "ns",
				Pool:       "pool",
				IP:         "10.10.2.3",
			},
			isValid: true,
		},
		{
			name: "no k8sPodName for type pod",
			c: NetConf{
				Type:             v1alpha1.AllocatedTypePod,
				AllocateIdentify: "containerd",
				K8sPodNs:         "ns",
			},
			isValid: false,
		},
		{
			name: "no k8sPodNs for type pod",
			c: NetConf{
				Type:       v1alpha1.AllocatedTypePod,
				K8sPodName: "pod",
			},
			isValid: false,
		},
		{
			name: "no AllocateIdentify for type cniused",
			c: NetConf{
				Type:       v1alpha1.AllocatedTypeCNIUsed,
				K8sPodName: "pod",
				K8sPodNs:   "ns",
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
