package etcd

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	commonLabels "github.com/medik8s/common/pkg/labels"
)

const (
	etcdQuorumPDBName = "etcd-guard-pdb" // The new name of the PDB - From OCP 4.11
)

var guardLabel = map[string]string{"app": "guard"}

var _ = Describe("Check if etcd disruption is allowed", func() {
	log := ctrl.Log.WithName("etcd-unit-test")
	fakeClient := fake.NewClientBuilder().WithRuntimeObjects().Build()

	BeforeEach(func() {
		// create etcd ns on 1st run
		etcdNs := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: etcdNamespace,
			},
		}

		if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(etcdNs), &corev1.Namespace{}); err != nil {
			Expect(fakeClient.Create(context.Background(), etcdNs)).To(Succeed())
		}
	})

	Context("without etcd quorum guard PDB", func() {
		When("etcd quorum cannot be verified", func() {
			It("should fail", func() {
				fakeClient = fake.NewClientBuilder().WithRuntimeObjects().Build()
				valid, _ := IsEtcdDisruptionAllowed(context.Background(), fakeClient, log, nil)
				Expect(valid).To(BeFalse())
			})
		})
	})
	Context("with etcd quorum guard PDB", func() {
		var (
			pdb                          *policyv1.PodDisruptionBudget
			unguardedNode, nodeGuardDown *corev1.Node
			controlPlaneNodes            []*corev1.Node
		)

		BeforeEach(func() {
			fakeClient = fake.NewClientBuilder().WithRuntimeObjects().Build()
			pdb = getEtcdPDB()
			Expect(fakeClient.Create(context.Background(), pdb)).To(Succeed())
			pdb.Status.DisruptionsAllowed = int32(1)
			Expect(fakeClient.Status().Update(context.Background(), pdb)).To(Succeed())
			DeferCleanup(fakeClient.Delete, context.Background(), pdb)
			unguardedNode = newControlPlaneNodes(1)[0]
			unguardedNode.Name = "no-guard-node"
		})
		JustBeforeEach(func() {
			controlPlaneNodes = newControlPlaneNodes(3)
			for _, node := range controlPlaneNodes {
				Expect(fakeClient.Create(context.Background(), node)).To(Succeed())
				podGuard := getPodGuard(node.Name)
				Expect(fakeClient.Create(context.Background(), podGuard)).To(Succeed())
			}
			nodeGuardDown = controlPlaneNodes[1]
			DeferCleanup(func() {
				for _, node := range controlPlaneNodes {
					podGuard := getPodGuard(node.Name)
					Expect(fakeClient.Delete(context.Background(), podGuard)).To(Succeed())
					Expect(fakeClient.Delete(context.Background(), node)).To(Succeed())
				}
			})
		})

		When("etcd PDB allowed disruptions is one", func() {
			BeforeEach(func() {
				pdb.Status.DisruptionsAllowed = int32(1)
				Expect(fakeClient.Status().Update(context.Background(), pdb)).To(Succeed())
			})
			It("should allow disruption for any control plane node with a guard pod", func() {
				for _, node := range controlPlaneNodes {
					Expect(IsEtcdDisruptionAllowed(context.Background(), fakeClient, log, node)).To(BeTrue())
				}
			})
			It("should allow disruption for a control plane node without a guard pod", func() {
				Expect(IsEtcdDisruptionAllowed(context.Background(), fakeClient, log, unguardedNode)).To(BeTrue())
			})
		})
		When("etcd PDB allowed disruptions is zero", func() {
			BeforeEach(func() {
				pdb.Status.DisruptionsAllowed = int32(0)
				Expect(fakeClient.Status().Update(context.Background(), pdb)).To(Succeed())
			})
			It("should allow disruption for a control plane node without a guard pod", func() {
				Expect(fakeClient.Create(context.Background(), unguardedNode)).To(Succeed())
				DeferCleanup(fakeClient.Delete, context.Background(), unguardedNode)
				Expect(IsEtcdDisruptionAllowed(context.Background(), fakeClient, log, unguardedNode)).To(BeTrue())
			})
			It("should allow disruption for already disrupted control plane node (guard pod ready condition = false)", func() {
				podGuard := &corev1.Pod{}
				Expect(fakeClient.Get(context.Background(), client.ObjectKeyFromObject(getPodGuard(nodeGuardDown.Name)), podGuard)).To(Succeed())
				podGuard.Status.Conditions[0].Status = corev1.ConditionFalse
				Expect(fakeClient.Status().Update(context.Background(), podGuard)).To(Succeed())
				Expect(IsEtcdDisruptionAllowed(context.Background(), fakeClient, log, nodeGuardDown)).To(BeTrue())
			})
			It("should allow disruption for already disrupted control plane node (guard pod ready condition missing)", func() {
				podGuard := &corev1.Pod{}
				Expect(fakeClient.Get(context.Background(), client.ObjectKeyFromObject(getPodGuard(nodeGuardDown.Name)), podGuard)).To(Succeed())
				podGuard.Status.Conditions = []corev1.PodCondition{}
				Expect(fakeClient.Status().Update(context.Background(), podGuard)).To(Succeed())
				Expect(IsEtcdDisruptionAllowed(context.Background(), fakeClient, log, nodeGuardDown)).To(BeTrue())
			})
			It("should reject disruptions for any non-disrupted control plane node", func() {
				for _, node := range controlPlaneNodes {
					if node.Name != nodeGuardDown.Name {
						Expect(IsEtcdDisruptionAllowed(context.Background(), fakeClient, log, node)).To(BeFalse())
					}
				}
			})
		})
	})
})

// getPodGuard returns guard pod with expected label and Ready condition is True for a given nodeName
func getPodGuard(nodeName string) *corev1.Pod {
	dummyContainer := corev1.Container{
		Name:  "container-name",
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
			Name:      etcdQuorumPDBName,
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
