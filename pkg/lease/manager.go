package lease

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	log "github.com/sirupsen/logrus"

	coordv1 "k8s.io/api/coordination/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Manager interface {
	CreateOrGetLease(ctx context.Context, obj client.Object, duration time.Duration, holderIdentity string, namespace string) (*coordv1.Lease, bool, error)
	UpdateLease(ctx context.Context, obj client.Object, lease *coordv1.Lease, currentTime *metav1.MicroTime, leaseDuration, leaseDeadline time.Duration, holderIdentity string) (bool, error)
	InvalidateLease(ctx context.Context, objName string, leaseNamespace string) error
}

type manager struct {
	client.Client
	log logr.Logger
}

func (l *manager) CreateOrGetLease(ctx context.Context, obj client.Object, duration time.Duration, holderIdentity string, namespace string) (*coordv1.Lease, bool, error) {
	return l.createOrGetExistingLease(ctx, obj, duration, holderIdentity, namespace)
}

func (l *manager) UpdateLease(ctx context.Context, obj client.Object, lease *coordv1.Lease, currentTime *metav1.MicroTime, leaseDuration, leaseDeadline time.Duration, holderIdentity string) (bool, error) {
	return l.updateLease(ctx, obj, lease, currentTime, leaseDuration, leaseDeadline, holderIdentity)
}

func (l *manager) InvalidateLease(ctx context.Context, objName string, leaseNamespace string) error {
	return l.invalidateLease(ctx, objName, leaseNamespace)
}

func NewManager(cl client.Client) Manager {
	return NewManagerWithCustomLogger(cl, ctrl.Log.WithName("leaseManager"))

}

func NewManagerWithCustomLogger(cl client.Client, log logr.Logger) Manager {
	return &manager{
		Client: cl,
		log:    log,
	}
}

func (l *manager) createOrGetExistingLease(ctx context.Context, obj client.Object, duration time.Duration, holderIdentity string, leaseNamespace string) (*coordv1.Lease, bool, error) {
	owner := makeExpectedOwnerOfLease(obj)
	microTimeNow := metav1.NowMicro()

	lease := &coordv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:            obj.GetName(),
			Namespace:       leaseNamespace,
			OwnerReferences: []metav1.OwnerReference{*owner},
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       obj.GetObjectKind().GroupVersionKind().Kind,
			APIVersion: obj.GetObjectKind().GroupVersionKind().Version,
		},
		Spec: coordv1.LeaseSpec{
			HolderIdentity:       &holderIdentity,
			LeaseDurationSeconds: pointer.Int32(int32(duration.Seconds())),
			AcquireTime:          &microTimeNow,
			RenewTime:            &microTimeNow,
			LeaseTransitions:     pointer.Int32(0),
		},
	}

	if err := l.Client.Create(ctx, lease); err != nil {
		if errors.IsAlreadyExists(err) {

			objName := obj.GetName()
			key := apitypes.NamespacedName{Namespace: leaseNamespace, Name: objName}

			if err := l.Client.Get(ctx, key, lease); err != nil {
				return nil, false, err
			}
			return lease, true, nil
		}
		return nil, false, err
	}
	return lease, false, nil
}

func (l *manager) updateLease(ctx context.Context, obj client.Object, lease *coordv1.Lease, currentTime *metav1.MicroTime, leaseDuration, leaseDeadline time.Duration, holderIdentity string) (bool, error) {
	needUpdateLease := false
	setAcquireAndLeaseTransitions := false
	updateAlreadyOwnedLease := false

	if lease.Spec.HolderIdentity != nil && *lease.Spec.HolderIdentity == holderIdentity {
		needUpdateLease, setAcquireAndLeaseTransitions = needUpdateOwnedLease(lease, *currentTime, leaseDeadline)
		if needUpdateLease {
			updateAlreadyOwnedLease = true

			log.Infof("renew lease owned by nmo setAcquireTime=%t", setAcquireAndLeaseTransitions)

		}
	} else {
		// can't update the lease if it is currently valid.
		if isValidLease(lease, currentTime.Time) {
			return false, fmt.Errorf("can't update valid lease held by different owner")
		}
		needUpdateLease = true

		log.Info("taking over foreign lease")
		setAcquireAndLeaseTransitions = true
	}

	if needUpdateLease {
		if setAcquireAndLeaseTransitions {
			lease.Spec.AcquireTime = currentTime
			if lease.Spec.LeaseTransitions != nil {
				*lease.Spec.LeaseTransitions += int32(1)
			} else {
				lease.Spec.LeaseTransitions = pointer.Int32(1)
			}
		}
		owner := makeExpectedOwnerOfLease(obj)
		lease.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*owner}
		lease.Spec.HolderIdentity = &holderIdentity
		lease.Spec.LeaseDurationSeconds = pointer.Int32(int32(leaseDuration.Seconds()))
		lease.Spec.RenewTime = currentTime
		if err := l.Client.Update(ctx, lease); err != nil {
			log.Errorf("Failed to update the lease. obj %s error: %v", obj.GetName(), err)
			return updateAlreadyOwnedLease, err
		}
	}

	return false, nil
}

func (l *manager) invalidateLease(ctx context.Context, objName string, leaseNamespace string) error {
	log.Info("invalidating lease")
	nName := apitypes.NamespacedName{Namespace: leaseNamespace, Name: objName}
	lease := &coordv1.Lease{}

	if err := l.Client.Get(ctx, nName, lease); err != nil {

		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	lease.Spec.AcquireTime = nil
	lease.Spec.LeaseDurationSeconds = nil
	lease.Spec.RenewTime = nil
	lease.Spec.LeaseTransitions = nil

	if err := l.Client.Update(ctx, lease); err != nil {
		return err
	}
	return nil
}

func makeExpectedOwnerOfLease(obj client.Object) *metav1.OwnerReference {

	return &metav1.OwnerReference{
		APIVersion: obj.GetObjectKind().GroupVersionKind().Version,
		Kind:       obj.GetObjectKind().GroupVersionKind().Kind,
		Name:       obj.GetName(),
		UID:        obj.GetUID(),
	}
}

func leaseDueTime(lease *coordv1.Lease) time.Time {
	return lease.Spec.RenewTime.Time.Add(time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second)
}

func needUpdateOwnedLease(lease *coordv1.Lease, currentTime metav1.MicroTime, leaseDeadline time.Duration) (bool, bool) {

	if lease.Spec.RenewTime == nil || lease.Spec.LeaseDurationSeconds == nil {
		log.Info("empty renew time or duration in sec")
		return true, true
	}
	dueTime := leaseDueTime(lease)

	// if lease expired right now, then both update the lease and the acquire time (second rvalue)
	// if the acquire time has been previously nil
	if dueTime.Before(currentTime.Time) {
		return true, lease.Spec.AcquireTime == nil
	}

	deadline := currentTime.Add(leaseDeadline)

	// about to expire, update the lease but not the acquired time (second value)
	return dueTime.Before(deadline), false
}

func isValidLease(lease *coordv1.Lease, currentTime time.Time) bool {

	if lease.Spec.RenewTime == nil || lease.Spec.LeaseDurationSeconds == nil {
		return false
	}

	renewTime := (*lease.Spec.RenewTime).Time
	dueTime := leaseDueTime(lease)

	// valid lease if: due time not in the past and renew time not in the future
	return !dueTime.Before(currentTime) && !renewTime.After(currentTime)
}
