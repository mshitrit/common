package etcd

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// var (
// 	cfg *rest.Config
// 	testEnv *envtest.Environment
// 	k8sClient client.Client
// 	k8sManager   manager.Manager
// 	ctx          context.Context
// 	cancel       context.CancelFunc
// )

func TestEtcd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Etcd Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	// call start or refactor when moving to "normal" testEnv test
})

var _ = AfterSuite(func() {
	// call stop or refactor when moving to "normal" testEnv test
})

// var _ = BeforeSuite(func() {
// 	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

// 	testScheme := runtime.NewScheme()

// 	By("bootstrapping test environment")
// 	testEnv = &envtest.Environment{
// 		CRDInstallOptions: envtest.CRDInstallOptions{
// 			Scheme: testScheme,
// 			Paths: []string{
// 				filepath.Join("..", "vendor", "github.com", "openshift", "api", "machine", "v1beta1"),
// 				filepath.Join("..", "config", "crd", "bases"),
// 			},
// 			ErrorIfPathMissing: true,
// 		},
// 	}

// 	var err error
// 	cfg, err = testEnv.Start()
// 	Expect(err).NotTo(HaveOccurred())
// 	Expect(cfg).NotTo(BeNil())

// 	//+kubebuilder:scaffold:scheme

// 	k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{Scheme: testScheme})
// 	Expect(err).NotTo(HaveOccurred())

// 	k8sClient, err = client.New(cfg, client.Options{Scheme: testScheme})
// 	Expect(err).NotTo(HaveOccurred())
// 	Expect(k8sClient).NotTo(BeNil())
// 	// err = (&etcdReconcile{
// 	// 	client:   k8sClient,
// 	// 	scheme:   k8sManager.GetScheme(),
// 	// }).SetupWithManager(k8sManager)
// 	// Expect(err).NotTo(HaveOccurred())

// 	go func() {
// 		// https://github.com/kubernetes-sigs/controller-runtime/issues/1571
// 		ctx, cancel = context.WithCancel(ctrl.SetupSignalHandler())
// 		err := k8sManager.Start(ctx)
// 		Expect(err).NotTo(HaveOccurred())
// 	}()
// })

// var _ = AfterSuite(func() {
// 	By("tearing down the test environment")
// 	cancel()
// 	err := testEnv.Stop()
// 	Expect(err).NotTo(HaveOccurred())
// })
