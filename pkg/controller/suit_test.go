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
	"k8s.io/klog/v2"
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
	timeout     = 10 * time.Second
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
	ctrl.SetLogger(klog.Background())

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

	By("setup ippool controller")
	Expect((&PoolController{Client: mgr.GetClient()}).SetupWithManager(mgr)).Should(Succeed())

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

func newIPPool(subnet, gw, start, end, cidr string, excepts ...string) *v1alpha1.IPPool {
	return &v1alpha1.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pool",
			Namespace: "ns",
		},
		Spec: v1alpha1.IPPoolSpec{
			Subnet:  subnet,
			Gateway: gw,
			CIDR:    cidr,
			Start:   start,
			End:     end,
			Except:  excepts,
		},
	}
}

func newIPPoolWithStatus(p *v1alpha1.IPPool, offset int64, usedIPs []string, allocateIPs []string) *v1alpha1.IPPool {
	p.Status.Offset = offset
	if len(usedIPs) > 0 {
		p.Status.UsedIps = make(map[string]string)
	}
	if len(allocateIPs) > 0 {
		p.Status.AllocatedIPs = make(map[string]v1alpha1.AllocateInfo)
	}
	for _, ip := range usedIPs {
		p.Status.UsedIps[ip] = "xxxx"
	}
	for _, ip := range allocateIPs {
		p.Status.AllocatedIPs[ip] = v1alpha1.AllocateInfo{
			ID:   "xxxx",
			Type: v1alpha1.AllocateTypePod,
		}
	}

	return p
}
