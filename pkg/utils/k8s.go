package utils

func GetAllocateIDFromPod(podNs, podName string) string {
	return podNs + "/" + podName
}
