package etcd

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const etcdNamespace = "openshift-etcd"

var log = logf.Log.WithName("etcd-pdb-checker")

// isEtcdDisruptionAllowed checks if etcd disruption is allowed and refuse the todoAction (remediation/manitenance) when it isn't allowed
func isEtcdDisruptionAllowed(ctx context.Context, cl client.Client, todoAction string) (bool, *policyv1.PodDisruptionBudget, error) {
	pdbList := &policyv1.PodDisruptionBudgetList{}
	if err := cl.List(ctx, pdbList, &client.ListOptions{Namespace: etcdNamespace}); err != nil {
		return false, nil, err
	}
	if len(pdbList.Items) == 0 {
		log.Info("No PDB found, can't check if etcd quorum will be violated! Refusing "+todoAction+"!", "namespace", etcdNamespace)
		return false, nil, nil
	}
	if len(pdbList.Items) > 1 {
		log.Info("More than one PDB found, can't check if etcd quorum will be violated! Refusing "+todoAction+"!", "namespace", etcdNamespace)
		return false, nil, nil
	}
	pdb := pdbList.Items[0]
	if pdb.Status.DisruptionsAllowed >= 1 {
		return true, &pdb, nil
	}
	return false, &pdb, nil
}

// IsControlPlaneNodeReady checks if etcd disruption is allowed and accpet/refuse the todoAction (remediation/manitenance)
func IsControlPlaneNodeReady(ctx context.Context, cl client.Client, node *corev1.Node, todoAction string) (bool, error) {
	allowedDisruption, pdb, err := isEtcdDisruptionAllowed(ctx, cl, todoAction)
	if pdb == nil {
		return false, err
	}
	if allowedDisruption {
		log.Info("Etcd disruption is allowed, so "+todoAction+" is allowed", "Node", node.Name)
		return true, nil
	}
	log.Info("ETCD PDB was found but etcd disruption isn't allowed - DisruptionsAllowed = 0", "Node", node.Name)

	// No disruptions allowed, so the only case we should remediate is that the node in question is already one of the disrupted ones
	// The PDB doesn't disclose which node is disrupted
	// So we have to check the etcd guard pods
	selector, err := metav1.LabelSelectorAsMap(pdb.Spec.Selector)
	if err != nil {
		log.Info("Could not parse PDB selector, can't check if etcd quorum will be violated! Refusing "+todoAction+"!", "selector", pdb.Spec.Selector.String())
		return false, err
	}
	podList := &corev1.PodList{}
	if err := cl.List(ctx, podList, &client.ListOptions{
		Namespace:     etcdNamespace,
		LabelSelector: labels.SelectorFromSet(selector),
	}); err != nil {
		return false, err
	}
	for _, pod := range podList.Items {
		if pod.Spec.NodeName == node.Name {
			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionFalse {
					log.Info("Node is disrupted, so "+todoAction+" is allowed", "Node", node.Name, "Guard pod", pod.Name)
					return true, nil
				}
			}
			log.Info("Node is not disrupted, so "+todoAction+" is not allowed", "Node", node.Name, "Guard pod", pod.Name)
			return false, nil
		}
	}

	log.Info("Node is not disrupted, so "+todoAction+" is not allowed", "Node", node.Name)
	return false, nil
}
