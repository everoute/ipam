package cron

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/everoute/ipam/api/ipam/v1alpha1"
)

var (
	k8sClient    client.Client
	testEnv      *envtest.Environment
	cleanStaleIP *CleanStaleIP
	period       = 5 * time.Second
	ns           = "cron-test"
	ctx          = context.Background()
	interval     = 1 * time.Second
	timeout      = 10 * time.Second
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
	Expect(appsv1.AddToScheme(scheme)).Should(Succeed())
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

	By("init CleanStaleIP")
	cleanStaleIP = NewCleanStaleIP(period, k8sClient, k8sClient)

	By("cleanStaleIP.Run")
	cleanStaleIP.Run(ctrl.SetupSignalHandler())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Expect(testEnv.Stop()).Should(Succeed())
})

func TestCron(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "cron Suite")
}
