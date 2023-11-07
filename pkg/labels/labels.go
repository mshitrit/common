package labels

const (
	// WorkerRole is the role label of worker nodes
	WorkerRole = "node-role.kubernetes.io/worker"
	// MasterRole is the old role label of control plane nodes
	MasterRole = "node-role.kubernetes.io/master"
	// ControlPlaneRole is the new role label of control plane nodes
	ControlPlaneRole = "node-role.kubernetes.io/control-plane"
	// DefaultTemplate is a label that would be set in case a remediator may have several remediation templates in order to signal to the UI which one is the default
	DefaultTemplate = "remediation.medik8s.io/default-template"
)
