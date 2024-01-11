package etcd

import (
	"context"
	"fmt"

	"github.com/go-logr/zapr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	commonLabels "github.com/medik8s/common/pkg/labels"
)

var guardLabel = map[string]string{"app": "guard"}

var _ = Describe("Check etcd disruptions", func() {
	var (
		cl client.WithWatch
	)
	zapLogger, _ := zap.NewProduction()
	log := zapr.NewLogger(zapLogger)

	When("etcd PDB doesn't exist", func() {
		It("should fail", func() {
			cl = fake.NewClientBuilder().WithRuntimeObjects().Build()
			valid, _ := IsEtcdDisruptionAllowed(context.Background(), cl, log, nil)
			Expect(valid).To(BeFalse())
		})
	})
	When("etcd PDB exists", func() {
		var (
			pdb               *policyv1.PodDisruptionBudget
			controlPlaneNodes []*corev1.Node
			nodeGuardDown     string
		)

		BeforeEach(func() {
			cl = fake.NewClientBuilder().WithRuntimeObjects().Build()
			pdb = getEtcdPDB()
			Expect(cl.Create(context.Background(), pdb)).To(Succeed())
			pdb.Status.DisruptionsAllowed = int32(1)
			Expect(cl.Status().Update(context.Background(), pdb)).To(Succeed())
			DeferCleanup(cl.Delete, context.Background(), pdb)
		})
		JustBeforeEach(func() {
			controlPlaneNodes = newControlPlaneNodes(3)
			for _, node := range controlPlaneNodes {
				Expect(cl.Create(context.Background(), node)).To(Succeed())
				podGuard := getPodGuard(node.Name)
				Expect(cl.Create(context.Background(), podGuard)).To(Succeed())
			}
			DeferCleanup(func() {
				for _, node := range controlPlaneNodes {
					podGuard := getPodGuard(node.Name)
					Expect(cl.Delete(context.Background(), podGuard)).To(Succeed())
					Expect(cl.Delete(context.Background(), node)).To(Succeed())
				}
			})
		})

		It("should allow disruption for any control plane node in healhy cluster", func() {
			pdb.Status.DisruptionsAllowed = int32(1)
			Expect(cl.Status().Update(context.Background(), pdb)).To(Succeed())
			for _, node := range controlPlaneNodes {
				Expect(IsEtcdDisruptionAllowed(context.Background(), cl, log, node)).To(BeTrue())
			}
		})

		It("should allow disruption for a control plane node without guard pod", func() {
			By("Allowed Disruption is one")
			// create new node without guard pod
			unguardedNode := newControlPlaneNodes(1)[0]
			unguardedNode.Name = "no-guard-node"
			Expect(cl.Create(context.Background(), unguardedNode)).To(Succeed())
			DeferCleanup(cl.Delete, context.Background(), unguardedNode)
			Expect(IsEtcdDisruptionAllowed(context.Background(), cl, log, unguardedNode)).To(BeTrue())

			By("Allowed Disruption is zero")
			pdb.Status.DisruptionsAllowed = int32(0)
			Expect(cl.Status().Update(context.Background(), pdb)).To(Succeed())
			Expect(IsEtcdDisruptionAllowed(context.Background(), cl, log, unguardedNode)).To(BeTrue())
		})

		It("should allow disruption for one unhealthy control plane that is not violating quorum and reject others", func() {
			nodeGuardDown = controlPlaneNodes[2].Name
			pdb.Status.DisruptionsAllowed = int32(0)
			Expect(cl.Status().Update(context.Background(), pdb)).To(Succeed())
			dummyPodGuard := getPodGuard(nodeGuardDown)
			podGuard := &corev1.Pod{}
			Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(dummyPodGuard), podGuard)).To(Succeed())
			podGuard.Status.Conditions[0].Status = corev1.ConditionFalse
			Expect(cl.Status().Update(context.Background(), podGuard)).To(Succeed())
			for _, node := range controlPlaneNodes {
				if node.Name == nodeGuardDown {
					Expect(IsEtcdDisruptionAllowed(context.Background(), cl, log, node)).To(BeTrue())
				} else {
					Expect(IsEtcdDisruptionAllowed(context.Background(), cl, log, node)).To(BeFalse())
				}
			}
		})
	})
})

// getPodGuard returns gurad pod with expected label and Ready condition is True for a given nodeName
func getPodGuard(nodeName string) *corev1.Pod {
	dummyContainer := corev1.Container{
		Name:  "container- name",
		Image: "foo",
	}
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "guard-" + nodeName,
			Namespace: etcdNamespace,
			Labels:    guardLabel,
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{
				dummyContainer,
			},
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}

// getEtcdPDB returns PodDisruptionBudget object at etcd namespace with a matching guard pod selector
func getEtcdPDB() *policyv1.PodDisruptionBudget {
	return &policyv1.PodDisruptionBudget{
		TypeMeta: metav1.TypeMeta{Kind: "PodDisruptionBudget"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "etcd-guard-pdb",
			Namespace: etcdNamespace,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: guardLabel,
			},
		},
	}
}

func newControlPlaneNodes(nodesCount int) []*corev1.Node {
	nodes := make([]*corev1.Node, 0, nodesCount)
	for i := 0; i < nodesCount; i++ {
		node := newNode(fmt.Sprintf("control-plane-node-%d", i))
		nodes = append(nodes, node)
	}
	return nodes
}

func newNode(name string) *corev1.Node {
	labels := make(map[string]string, 1)
	labels[commonLabels.ControlPlaneRole] = ""
	return &corev1.Node{
		TypeMeta: metav1.TypeMeta{Kind: "Node"},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}
