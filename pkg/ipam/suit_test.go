package ipam

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	cniv1 "github.com/containernetworking/cni/pkg/types/100"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/everoute/ipam/api/ipam/v1alpha1"
)

var (
	k8sClient client.Client
	testEnv   *envtest.Environment
	ipam      *Ipam
	ns        = "ipam"
	ctx       = context.Background()
	interval  = time.Second
	timeout   = 2 * time.Minute
)

var _ = BeforeSuite(func() {
	By("setup test enviroment")
	notExistCluster := false
	testEnv = &envtest.Environment{
		UseExistingCluster:    &notExistCluster,
		BinaryAssetsDirectory: "/usr/local/bin",
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths:              []string{filepath.Join("..", "..", "deploy", "crds")},
			CleanUpAfterUse:    true,
			ErrorIfPathMissing: true,
		},
	}
	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())

	By("get k8sClient")
	scheme := scheme.Scheme
	Expect(corev1.AddToScheme(scheme)).Should(Succeed())
	Expect(v1alpha1.AddToScheme(scheme)).Should(Succeed())
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	By("create namespace")
	nsObj := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}
	Expect(k8sClient.Create(ctx, &nsObj)).Should(Succeed())

	By("init ipam")
	ipam = InitIpam(k8sClient, ns)
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Expect(testEnv.Stop()).Should(Succeed())
})

func TestIPAM(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ipam Suite")
}

func makeAllocateStatus(input ...string) map[string]v1alpha1.AllocateInfo {
	res := make(map[string]v1alpha1.AllocateInfo)
	i := 0
	for i < len(input)-2 {
		res[input[i]] = v1alpha1.AllocateInfo{
			ID:   input[i+1],
			Type: v1alpha1.AllocateType(input[i+2]),
		}
		i += 3
	}
	return res
}

func makeUsedIPStatus(input ...string) map[string]string {
	res := make(map[string]string)
	i := 0
	for i < len(input)-1 {
		res[input[i]] = input[i+1]
		i += 2
	}
	return res
}

func makeCNIIPconfig(ip, mask, gateway string) *cniv1.IPConfig {
	maskByte := []byte(net.ParseIP(mask))
	return &cniv1.IPConfig{
		Address: net.IPNet{
			IP:   net.ParseIP(ip),
			Mask: net.IPv4Mask(maskByte[12], maskByte[13], maskByte[14], maskByte[15]),
		},
		Gateway: net.ParseIP(gateway),
	}
}
