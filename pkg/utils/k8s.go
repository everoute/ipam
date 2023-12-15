package utils

import (
	"strings"

	"k8s.io/apimachinery/pkg/types"
)

func GetAllocateIDFromPod(podNs, podName string) string {
	return podNs + "/" + podName
}

func GetPodNsNameByAllocateID(id string) types.NamespacedName {
	strs := strings.Split(id, "/")
	if len(strs) == 2 {
		return types.NamespacedName{
			Namespace: strs[0],
			Name:      strs[1],
		}
	}

	return types.NamespacedName{}
}
