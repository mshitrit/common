package lease

import (
	"context"
	"fmt"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	coordv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var NowTime = metav1.NowMicro()

const (
	//leaseDeadline       = 60 * time.Second
	leaseDeadline       = leaseDuration
	leaseHolderIdentity = "some-operator"
	leaseDuration       = 3600 * time.Second
	leaseNamespace      = "some-lease-namespace"
)

func getMockNode() *corev1.Node {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "miau",
			UID:  "foobar",
		},
		TypeMeta: metav1.TypeMeta{Kind: "Node", APIVersion: "v1"},
	}
	return node
}

var _ = Describe("Leases", func() {

	// if current time is after this time, the lease is expired
	leaseExpiredTime := NowTime.Add(-leaseDuration).Add(-1 * time.Second)
	// if lease expires after this time, it should be renewed
	renewTriggerTime := NowTime.Add(-leaseDuration).Add(leaseDeadline)

	DescribeTable("Updates",
		func(initialLease *coordv1.Lease, expectedLease *coordv1.Lease, expectedError error) {
			node := getMockNode()
			objs := []runtime.Object{
				initialLease,
			}
			cl := fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()
			manager := NewManager(cl, leaseHolderIdentity, leaseNamespace)
			name := apitypes.NamespacedName{Namespace: leaseNamespace, Name: node.Name}
			currentLease := &coordv1.Lease{}
			err := cl.Get(context.TODO(), name, currentLease)
			Expect(err).NotTo(HaveOccurred())

			err = manager.RequestLease(context.Background(), node, leaseDuration)

			if expectedLease == nil {
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(expectedError))
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(expectedError).NotTo(HaveOccurred())

				actualLease := &coordv1.Lease{}
				err = cl.Get(context.TODO(), name, actualLease)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(actualLease.ObjectMeta.OwnerReferences)).To(Equal(1))
				Expect(len(expectedLease.ObjectMeta.OwnerReferences)).To(Equal(1))

				actualLeaseOwner := actualLease.ObjectMeta.OwnerReferences[0]
				expectedLeaseOwner := expectedLease.ObjectMeta.OwnerReferences[0]

				Expect(actualLeaseOwner.APIVersion).To(Equal(expectedLeaseOwner.APIVersion))
				Expect(actualLeaseOwner.Kind).To(Equal(expectedLeaseOwner.Kind))
				Expect(actualLeaseOwner.Name).To(Equal(expectedLeaseOwner.Name))
				Expect(actualLeaseOwner.UID).To(Equal(expectedLeaseOwner.UID))

				ExpectEqualWithNil(actualLease.Spec.HolderIdentity, expectedLease.Spec.HolderIdentity, "holder identity should match")
				ExpectEqualWithNil(actualLease.Spec.RenewTime, expectedLease.Spec.RenewTime, "renew time should match")
				ExpectEqualWithNil(actualLease.Spec.AcquireTime, expectedLease.Spec.AcquireTime, "acquire time should match")
				ExpectEqualWithNil(actualLease.Spec.LeaseDurationSeconds, expectedLease.Spec.LeaseDurationSeconds, "actualLease duration should match")
				ExpectEqualWithNil(actualLease.Spec.LeaseTransitions, expectedLease.Spec.LeaseTransitions, "actualLease transitions should match")
			}
		},

		Entry("fail to update valid lease with different holder identity",
			&coordv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getMockNode().Name,
					Namespace: leaseNamespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "v1",
							Kind:       "Node",
							Name:       "@",
							UID:        "#",
						},
					},
				},
				Spec: coordv1.LeaseSpec{
					HolderIdentity:       pointer.String("miau"),
					LeaseDurationSeconds: pointer.Int32(32000),
					AcquireTime:          nil,
					RenewTime:            &metav1.MicroTime{Time: NowTime.Add(-1 * time.Second)},
					LeaseTransitions:     nil,
				},
			},
			nil,
			fmt.Errorf("can't update valid lease held by different owner"),
		),
		Entry("update lease with different holder identity (full init)",
			&coordv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getMockNode().Name,
					Namespace: leaseNamespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "v1",
							Kind:       "Node",
							Name:       "@",
							UID:        "#",
						},
					},
				},
				Spec: coordv1.LeaseSpec{
					HolderIdentity:       pointer.String("miau"),
					LeaseDurationSeconds: pointer.Int32(44),
					AcquireTime:          &metav1.MicroTime{Time: time.Unix(42, 0)},
					RenewTime:            &metav1.MicroTime{Time: time.Unix(43, 0)},
					LeaseTransitions:     nil,
				},
			},
			&coordv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getMockNode().Name,
					Namespace: leaseNamespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: corev1.SchemeGroupVersion.WithKind("Node").Version,
						Kind:       corev1.SchemeGroupVersion.WithKind("Node").Kind,
						Name:       getMockNode().Name,
						UID:        getMockNode().UID,
					}},
				},
				Spec: coordv1.LeaseSpec{
					HolderIdentity:       pointer.String(leaseHolderIdentity),
					LeaseDurationSeconds: pointer.Int32(int32(leaseDuration.Seconds())),
					AcquireTime:          &NowTime,
					RenewTime:            &NowTime,
					LeaseTransitions:     pointer.Int32(1),
				},
			},
			nil,
		),
		Entry("update expired lease with different holder identity (with transition update)",
			&coordv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getMockNode().Name,
					Namespace: leaseNamespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "v1",
							Kind:       "Node",
							Name:       "@",
							UID:        "#",
						},
					},
				},
				Spec: coordv1.LeaseSpec{
					HolderIdentity:       pointer.String("miau"),
					LeaseDurationSeconds: pointer.Int32(44),
					AcquireTime:          &metav1.MicroTime{Time: time.Unix(42, 0)},
					RenewTime:            &metav1.MicroTime{Time: time.Unix(43, 0)},
					LeaseTransitions:     pointer.Int32(3),
				},
			},
			&coordv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getMockNode().Name,
					Namespace: leaseNamespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: corev1.SchemeGroupVersion.WithKind("Node").Version,
						Kind:       corev1.SchemeGroupVersion.WithKind("Node").Kind,
						Name:       getMockNode().Name,
						UID:        getMockNode().UID,
					}},
				},
				Spec: coordv1.LeaseSpec{
					HolderIdentity:       pointer.String(leaseHolderIdentity),
					LeaseDurationSeconds: pointer.Int32(int32(leaseDuration.Seconds())),
					AcquireTime:          &NowTime,
					RenewTime:            &NowTime,
					LeaseTransitions:     pointer.Int32(4),
				},
			},
			nil,
		),
		Entry("extend lease if same holder and zero duration and renew time (invalid lease)",
			&coordv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getMockNode().Name,
					Namespace: leaseNamespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "v1",
							Kind:       "Node",
							Name:       "@",
							UID:        "#",
						},
					},
				},
				Spec: coordv1.LeaseSpec{
					HolderIdentity:       pointer.String(leaseHolderIdentity),
					LeaseDurationSeconds: nil,
					AcquireTime:          &metav1.MicroTime{Time: NowTime.Add(-599 * time.Second)},
					RenewTime:            nil,
					LeaseTransitions:     pointer.Int32(3),
				},
			},
			&coordv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getMockNode().Name,
					Namespace: leaseNamespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: corev1.SchemeGroupVersion.WithKind("Node").Version,
						Kind:       corev1.SchemeGroupVersion.WithKind("Node").Kind,
						Name:       getMockNode().Name,
						UID:        getMockNode().UID,
					}},
				},
				Spec: coordv1.LeaseSpec{
					HolderIdentity:       pointer.String(leaseHolderIdentity),
					LeaseDurationSeconds: pointer.Int32(int32(leaseDuration.Seconds())),
					AcquireTime:          &NowTime,
					RenewTime:            &NowTime,
					LeaseTransitions:     pointer.Int32(4),
				},
			},
			nil,
		),
		Entry("update lease if same holder and expired lease - check modified lease duration",
			&coordv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getMockNode().Name,
					Namespace: leaseNamespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "v1",
							Kind:       "Node",
							Name:       "@",
							UID:        "#",
						},
					},
				},
				Spec: coordv1.LeaseSpec{
					HolderIdentity:       pointer.String(leaseHolderIdentity),
					LeaseDurationSeconds: pointer.Int32(int32(leaseDuration.Seconds() - 42)),
					AcquireTime:          nil,
					RenewTime:            &metav1.MicroTime{Time: leaseExpiredTime},
					LeaseTransitions:     nil,
				},
			},
			&coordv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getMockNode().Name,
					Namespace: leaseNamespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: corev1.SchemeGroupVersion.WithKind("Node").Version,
						Kind:       corev1.SchemeGroupVersion.WithKind("Node").Kind,
						Name:       getMockNode().Name,
						UID:        getMockNode().UID,
					}},
				},
				Spec: coordv1.LeaseSpec{
					HolderIdentity:       pointer.String(leaseHolderIdentity),
					LeaseDurationSeconds: pointer.Int32(int32(leaseDuration.Seconds())),
					AcquireTime:          &NowTime,
					RenewTime:            &NowTime,
					LeaseTransitions:     pointer.Int32(1),
				},
			},
			nil,
		),
		Entry("extend lease if same holder and expired lease (acquire time previously not nil)",
			&coordv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getMockNode().Name,
					Namespace: leaseNamespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "v1",
							Kind:       "Node",
							Name:       "@",
							UID:        "#",
						},
					},
				},
				Spec: coordv1.LeaseSpec{
					HolderIdentity:       pointer.String(leaseHolderIdentity),
					LeaseDurationSeconds: pointer.Int32(int32(leaseDuration.Seconds())),
					AcquireTime:          &metav1.MicroTime{Time: leaseExpiredTime},
					RenewTime:            &metav1.MicroTime{Time: leaseExpiredTime},
					LeaseTransitions:     pointer.Int32(1),
				},
			},
			&coordv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getMockNode().Name,
					Namespace: leaseNamespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: corev1.SchemeGroupVersion.WithKind("Node").Version,
						Kind:       corev1.SchemeGroupVersion.WithKind("Node").Kind,
						Name:       getMockNode().Name,
						UID:        getMockNode().UID,
					}},
				},
				Spec: coordv1.LeaseSpec{
					HolderIdentity:       pointer.String(leaseHolderIdentity),
					LeaseDurationSeconds: pointer.Int32(int32(leaseDuration.Seconds())),
					AcquireTime:          &metav1.MicroTime{Time: leaseExpiredTime},
					RenewTime:            &NowTime,
					LeaseTransitions:     pointer.Int32(1),
				},
			},
			nil,
		),
		// TODO why is not setting aquire time and transitions?
		Entry("extend lease if same holder and expired lease (acquire time previously nil)",
			&coordv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getMockNode().Name,
					Namespace: leaseNamespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "v1",
							Kind:       "Node",
							Name:       "@",
							UID:        "#",
						},
					},
				},
				Spec: coordv1.LeaseSpec{
					HolderIdentity:       pointer.String(leaseHolderIdentity),
					LeaseDurationSeconds: pointer.Int32(int32(leaseDuration.Seconds())),
					AcquireTime:          nil,
					RenewTime:            &metav1.MicroTime{Time: leaseExpiredTime},
					LeaseTransitions:     nil,
				},
			},
			&coordv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getMockNode().Name,
					Namespace: leaseNamespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: corev1.SchemeGroupVersion.WithKind("Node").Version,
						Kind:       corev1.SchemeGroupVersion.WithKind("Node").Kind,
						Name:       getMockNode().Name,
						UID:        getMockNode().UID,
					}},
				},
				Spec: coordv1.LeaseSpec{
					HolderIdentity:       pointer.String(leaseHolderIdentity),
					LeaseDurationSeconds: pointer.Int32(int32(leaseDuration.Seconds())),
					AcquireTime:          &NowTime,
					RenewTime:            &NowTime,
					LeaseTransitions:     pointer.Int32(1),
				},
			},
			nil,
		),
		// TODO why not setting aquire time and transitions?
		Entry("extend lease if same holder and lease will expire before current Time + two times the drainer timeout",
			&coordv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getMockNode().Name,
					Namespace: leaseNamespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: corev1.SchemeGroupVersion.WithKind("Node").Version,
							Kind:       corev1.SchemeGroupVersion.WithKind("Node").Kind,
							Name:       getMockNode().Name,
							UID:        getMockNode().UID,
						},
					},
				},
				Spec: coordv1.LeaseSpec{
					HolderIdentity:       pointer.String(leaseHolderIdentity),
					LeaseDurationSeconds: pointer.Int32(int32(leaseDuration.Seconds())),
					AcquireTime:          nil,
					RenewTime:            &metav1.MicroTime{Time: renewTriggerTime.Add(-1 * time.Second)},
					LeaseTransitions:     nil,
				},
			},
			&coordv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getMockNode().Name,
					Namespace: leaseNamespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: corev1.SchemeGroupVersion.WithKind("Node").Version,
						Kind:       corev1.SchemeGroupVersion.WithKind("Node").Kind,
						Name:       getMockNode().Name,
						UID:        getMockNode().UID,
					}},
				},
				Spec: coordv1.LeaseSpec{
					HolderIdentity:       pointer.String(leaseHolderIdentity),
					LeaseDurationSeconds: pointer.Int32(int32(leaseDuration.Seconds())),
					AcquireTime:          nil,
					RenewTime:            &NowTime,
					LeaseTransitions:     nil,
				},
			},
			nil,
		),
		// TODO why not setting aquire time and transitions?
		Entry("dont extend lease if same holder and lease not about to expire before current Time + two times the drainertimeout",
			&coordv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getMockNode().Name,
					Namespace: leaseNamespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "v1",
							Kind:       "Node",
							Name:       "@",
							UID:        "#",
						},
					},
				},
				Spec: coordv1.LeaseSpec{
					HolderIdentity:       pointer.String(leaseHolderIdentity),
					LeaseDurationSeconds: pointer.Int32(int32(leaseDuration.Seconds())),
					AcquireTime:          nil,
					RenewTime:            &metav1.MicroTime{Time: renewTriggerTime.Add(time.Second)},
					LeaseTransitions:     nil,
				},
			},
			&coordv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getMockNode().Name,
					Namespace: leaseNamespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "v1",
						Kind:       "Node",
						Name:       "@",
						UID:        "#",
					}},
				},
				Spec: coordv1.LeaseSpec{
					HolderIdentity:       pointer.String(leaseHolderIdentity),
					LeaseDurationSeconds: pointer.Int32(int32(leaseDuration.Seconds())),
					AcquireTime:          nil,
					RenewTime:            &metav1.MicroTime{Time: renewTriggerTime.Add(time.Second)},
					LeaseTransitions:     nil,
				},
			},
			nil,
		),
	)
})

func ExpectEqualWithNil(actual, expected interface{}, description string) {
	// heads up: interfaces representing a pointer are not nil when the pointer is nil
	if expected == nil || (reflect.ValueOf(expected).Kind() == reflect.Ptr && reflect.ValueOf(expected).IsNil()) {
		// BeNil() handles pointers correctly
		ExpectWithOffset(1, actual).To(BeNil(), description)
	} else {
		// compare unix time, precision of MicroTime is sometimes different
		if e, ok := expected.(*metav1.MicroTime); ok {
			expected = e.Unix()
			if actual != nil && reflect.ValueOf(actual).Kind() == reflect.Ptr && !reflect.ValueOf(actual).IsNil() {
				actual = actual.(*metav1.MicroTime).Unix()
			}
		}
		ExpectWithOffset(1, actual).To(Equal(expected), description)
	}
}
