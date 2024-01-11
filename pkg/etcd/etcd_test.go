package etcd

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	commonLabels "github.com/medik8s/common/pkg/labels"
)

var _ = Describe("Check ETCD disruptions", func() {
	var (
		cl client.WithWatch
	)

	When("ETCD PDB was not set", func() {
		It("should fail", func() {
			cl = fake.NewClientBuilder().WithRuntimeObjects().Build()
			valid, _ := IsControlPlaneNodeReady(context.Background(), cl, nil, "remediation")
			Expect(valid).To(BeFalse())
		})
	})
	When("ETCD PDB was set", func() {
		var (
			pdb               *policyv1.PodDisruptionBudget
			controlPlaneNodes []*corev1.Node
			nodeGuardDown     string
			// controlPlaneNodes, workerNodes [] corev1.Node
		)

		BeforeEach(func() {
			cl = fake.NewClientBuilder().WithRuntimeObjects().Build()
			pdb = getPDBEtcd()
			Expect(cl.Create(context.Background(), pdb)).To(Succeed())
			pdb.Status.DisruptionsAllowed = int32(1)
			Expect(cl.Status().Update(context.Background(), pdb)).To(Succeed())
			DeferCleanup(cl.Delete, context.Background(), pdb)
		})
		JustBeforeEach(func() {
			controlPlaneNodes = newControlPlaneNodes(3)
			for _, node := range controlPlaneNodes {
				Expect(cl.Create(context.Background(), node)).To(Succeed())
				podGuard := getPogGuard(node.Name)
				Expect(cl.Create(context.Background(), podGuard)).To(Succeed())
			}
			DeferCleanup(func() {
				for _, node := range controlPlaneNodes {
					podGuard := getPogGuard(node.Name)
					Expect(cl.Delete(context.Background(), podGuard)).To(Succeed())
					Expect(cl.Delete(context.Background(), node)).To(Succeed())
				}
			})
		})
		It("should allow remediation for healhy cluster", func() {
			pdb.Status.DisruptionsAllowed = int32(1)
			Expect(cl.Status().Update(context.Background(), pdb)).To(Succeed())
			for _, node := range controlPlaneNodes {
				Expect(IsControlPlaneNodeReady(context.Background(), cl, node, "remediation")).To(BeTrue())
			}

			// Expect(IsEtcdDisruptionAllowed(context.Background(), cl, &workerNodes[1], "remediation")).To(BeTrue())
		})
		It("should allow remediation for one unhealthy control plane that is not violating quorum and reject others", func() {
			nodeGuardDown = controlPlaneNodes[2].Name
			pdb.Status.DisruptionsAllowed = int32(0)
			Expect(cl.Status().Update(context.Background(), pdb)).To(Succeed())
			dummyPodGuard := getPogGuard(nodeGuardDown)
			podGuard := &corev1.Pod{}
			Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(dummyPodGuard), podGuard)).To(Succeed())
			podGuard.Status.Conditions[0].Status = corev1.ConditionFalse
			Expect(cl.Status().Update(context.Background(), podGuard)).To(Succeed())
			for _, node := range controlPlaneNodes {
				if node.Name == nodeGuardDown {
					Expect(IsControlPlaneNodeReady(context.Background(), cl, node, "remediation")).To(BeTrue())
				} else {
					Expect(IsControlPlaneNodeReady(context.Background(), cl, node, "remediation")).To(BeFalse())
				}
			}
		})
	})
})

func getPogGuard(nodeName string) *corev1.Pod {
	dummyContainer := corev1.Container{
		Name:  "container- name",
		Image: "foo",
	}
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "guard-" + nodeName,
			Namespace: etcdNamespace,
			Labels: map[string]string{
				"app": "guard",
			},
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

func getPDBEtcd() *policyv1.PodDisruptionBudget {
	return &policyv1.PodDisruptionBudget{
		TypeMeta: metav1.TypeMeta{Kind: "PodDisruptionBudget"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "etcd-guard-pdb",
			Namespace: etcdNamespace,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "guard",
				},
			},
		},
	}
}

func newControlPlaneNodes(nodesCount int) []*corev1.Node {
	nodes := make([]*corev1.Node, 0, nodesCount)
	for i := nodesCount; i > 0; i-- {
		node := newNode(fmt.Sprintf("control-plane-node-%d", i), corev1.NodeReady, corev1.ConditionUnknown)
		nodes = append(nodes, node)
	}
	return nodes
}

func newNode(name string, t corev1.NodeConditionType, s corev1.ConditionStatus) *corev1.Node {
	labels := make(map[string]string, 1)
	labels[commonLabels.ControlPlaneRole] = ""
	return &corev1.Node{
		TypeMeta: metav1.TypeMeta{Kind: "Node"},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:   t,
					Status: s,
				},
			},
		},
	}
}
