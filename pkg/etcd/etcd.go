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

const (
	etcdNamespace = "openshift-etcd"
	errNoEtcdChec = "can't check if etcd quorum will be violated!"
)

var log = logf.Log.WithName("etcd-pdb-checker")

// CanNodeDisruptEtcd checks if a node can disrupt etcd quorum, and it returns error when it fails in the process
func CanNodeDisruptEtcd(ctx context.Context, cl client.Client, node *corev1.Node) (bool, error) {
	// Check if etcd is already disrupted, and if new disruption is allowed
	pdbList := &policyv1.PodDisruptionBudgetList{}
	if err := cl.List(ctx, pdbList, &client.ListOptions{Namespace: etcdNamespace}); err != nil {
		return false, err
	}
	if len(pdbList.Items) == 0 {
		log.Info("No PDB were found, "+errNoEtcdChec, "namespace", etcdNamespace)
		return false, nil
	}
	if len(pdbList.Items) > 1 {
		log.Info("More than one PDB found, "+errNoEtcdChec, "namespace", etcdNamespace)
		return false, nil
	}
	pdb := pdbList.Items[0]
	if pdb.Status.DisruptionsAllowed >= 1 {
		log.Info("Node disruption is allowed, since etcd disruption is allowed", "Node", node.Name)
		return true, nil
	}

	log.Info("etcd PDB was found, but etcd disruption isn't allowed", "Node", node.Name, "etcd allowed disruptions", pdb.Status.DisruptionsAllowed)

	// No etcd disruptions are allowed, but we still need to check if the given node will violate etcd quorum
	// If it is disrupted, then it doesn't violate etcd quorum. Otherwise, it would violate etcd quorum
	// The PDB doesn't disclose which node is disrupted
	// So we have to check the etcd guard pods
	selector, err := metav1.LabelSelectorAsMap(pdb.Spec.Selector)
	if err != nil {
		log.Info("Could not parse PDB selector, "+errNoEtcdChec, "selector", pdb.Spec.Selector.String())
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
					log.Info("Node is disrupted, so it won't violate etcd quorum", "Node", node.Name, "Guard pod", pod.Name)
					return true, nil
				}
			}
			log.Info("Node is not disrupted, but it will violate etcd quorum", "Node", node.Name, "Guard pod", pod.Name)
			return false, nil
		}
	}

	log.Info("Node is not disrupted, but it will violate etcd quorum", "Node", node.Name)
	return false, nil
}
