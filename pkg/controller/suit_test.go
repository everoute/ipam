package controller

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
	testEnv     *envtest.Environment
	k8sClient   client.Client
	ns          = "test-controller"
	ctx, cancel = context.WithCancel(ctrl.SetupSignalHandler())
	interval    = 1 * time.Second
	timeout     = 1 * time.Minute
)

var _ = BeforeSuite(func() {
	By("setup test enviroment")
	notExistCluster := false
	testEnv = &envtest.Environment{
		ControlPlaneStopTimeout: 60 * time.Second,
		UseExistingCluster:      &notExistCluster,
		BinaryAssetsDirectory:   "/usr/local/bin",
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths:              []string{filepath.Join("..", "..", "deploy", "crds")},
			CleanUpAfterUse:    true,
			ErrorIfPathMissing: true,
		},
	}
	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ShouldNot(BeNil())

	By("init k8s manager")
	scheme := scheme.Scheme
	Expect(corev1.AddToScheme(scheme)).Should(Succeed())
	Expect(v1alpha1.AddToScheme(scheme)).Should(Succeed())
	Expect(appsv1.AddToScheme(scheme)).Should(Succeed())
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(mgr).ToNot(BeNil())

	By("setup statefulset controller")
	Expect((&STSReconciler{Client: mgr.GetClient()}).SetUpWithManager(mgr)).Should(Succeed())

	By("get k8sClient")
	k8sClient = mgr.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	By("create namespace")
	nsObj := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}
	Expect(k8sClient.Create(ctx, &nsObj)).Should(Succeed())

	By("start mgr")
	go func() {
		Expect(mgr.Start(ctx)).Should(Succeed())
	}()
})

var _ = AfterSuite(func() {
	By("stop controller manager")
	cancel()

	By("tearing down the test environment")
	Expect(testEnv.Stop()).Should(Succeed())
})

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller suit")
}
