package ipam

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/everoute/ipam/api/ipam/v1alpha1"
	"github.com/everoute/ipam/pkg/constants"
	"github.com/everoute/ipam/pkg/utils"
)

type NetConf struct {
	Pool string
	IP   string
	// default is containerID, type=cniused, defined by the cni
	AllocateIdentify string
	K8sPodName       string
	K8sPodNs         string
	// default is allocate IP to Pod
	Type v1alpha1.AllocateType
}

// Complete add ippool and static ip info to NetConf, param k8sClient must add corev1 scheme
func (c *NetConf) Complete(ctx context.Context, k8sClient client.Client) error {
	// only complete netconf by k8s annotation
	if c.Type != v1alpha1.AllocatedTypePod {
		return nil
	}
	// has been specify pool, return
	if c.Pool != "" {
		return nil
	}

	if c.K8sPodNs == "" || c.K8sPodName == "" {
		klog.Errorf("netconf %v must set K8sPodNs and K8sPodName for type %s", *c, v1alpha1.AllocatedTypePod)
		return fmt.Errorf("must set K8sPodNs and K8sPodName for type %s", v1alpha1.AllocatedTypePod)
	}

	pod := corev1.Pod{}
	podNsName := types.NamespacedName{Namespace: c.K8sPodNs, Name: c.K8sPodName}
	err := k8sClient.Get(ctx, podNsName, &pod)
	if err != nil {
		klog.Errorf("Failed to get pod %v, err: %v", podNsName, err)
		return err
	}
	if pool, ok := pod.Annotations[constants.IpamAnnotationPool]; ok {
		c.Pool = pool
	}
	if ip, ok := pod.Annotations[constants.IpamAnnotationStaticIP]; ok {
		if c.Pool == "" {
			klog.Errorf("Pod %v can't only specify static IP but no pool", pod)
			return fmt.Errorf("can't only specify static IP but no pool")
		}
		c.IP = ip
	}
	return nil
}

func (c *NetConf) Valid() error {
	if c.Type == v1alpha1.AllocatedTypeCNIUsed {
		if c.AllocateIdentify == "" {
			return fmt.Errorf("type %s must set AllocatedIdentify", v1alpha1.AllocatedTypeCNIUsed)
		}
	}

	if c.Type == v1alpha1.AllocatedTypePod {
		if c.K8sPodName == "" || c.K8sPodNs == "" {
			return fmt.Errorf("type %s must set K8sPodNs and K8sPodName", v1alpha1.AllocatedTypePod)
		}
	}

	return nil
}

func (c *NetConf) getAllocateID() string {
	allocatedID := c.AllocateIdentify
	if c.Type == v1alpha1.AllocatedTypePod {
		allocatedID = utils.GetAllocateIDFromPod(c.K8sPodNs, c.K8sPodName)
	}
	return allocatedID
}

func (c *NetConf) genAllocateInfo() v1alpha1.AllocateInfo {
	return v1alpha1.AllocateInfo{
		Type: c.Type,
		ID:   c.getAllocateID(),
	}
}
