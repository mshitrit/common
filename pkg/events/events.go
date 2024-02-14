package events

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

// Event message format "medik8s <operator shortname> <message>"
const (
	customFmt = "[remediation] %s"

	RemediationStartedEventReason                      = "RemediationStarted"
	RemediationStoppedEventReason                      = "RemediationStopped"
	RemediationFinishedEventReason                     = "RemediationFinished"
	RemediationCannotStartEventReason                  = "RemediationCannotStart"
	remediationStartedEventMessage                     = "Remediation started"
	remediationStoppedEventMessage                     = "NHC added the timed-out annotation, remediation will be stopped"
	remediationFinishedEventMessage                    = "Remediation finished"
	remediationCannotStartTargetNodeFailedEventMessage = "Could not get remediation target Node"
)

// NormalEvent will record an event with type Normal and fixed message.
func NormalEvent(recorder record.EventRecorder, object runtime.Object, reason, message string) {
	recorder.Event(object, corev1.EventTypeNormal, reason, fmt.Sprintf(customFmt, message))
}

// NormalEventf will record an event with type Normal and formatted message.
func NormalEventf(recorder record.EventRecorder, object runtime.Object, reason, messageFmt string, a ...interface{}) {
	message := fmt.Sprintf(messageFmt, a...)
	recorder.Event(object, corev1.EventTypeNormal, reason, fmt.Sprintf(customFmt, message))
}

// WarningEvent will record an event with type Warning and fixed message.
func WarningEvent(recorder record.EventRecorder, object runtime.Object, reason, message string) {
	recorder.Event(object, corev1.EventTypeWarning, reason, fmt.Sprintf(customFmt, message))
}

// WarningEventf will record an event with type Warning and formatted message.
func WarningEventf(recorder record.EventRecorder, object runtime.Object, reason, messageFmt string, a ...interface{}) {
	message := fmt.Sprintf(messageFmt, a...)
	recorder.Event(object, corev1.EventTypeWarning, reason, fmt.Sprintf(customFmt, message))
}

// Special case events

// RemediationStarted will record a Normal event with reason RemediationStarted and message "Remediation started".
func RemediationStarted(recorder record.EventRecorder, object runtime.Object) {
	NormalEvent(recorder, object, RemediationStartedEventReason, remediationStartedEventMessage)
}

// RemediationStoppedByNHC will record a Normal event with reason RemediationStopped and message "NHC added the timed-out annotation, remediation will be stopped".
func RemediationStoppedByNHC(recorder record.EventRecorder, object runtime.Object) {
	NormalEvent(recorder, object, RemediationStoppedEventReason, remediationStoppedEventMessage)
}

// RemediationFinished will record a Normal event with reason RemediationFinished and message "Remediation finished".
func RemediationFinished(recorder record.EventRecorder, object runtime.Object) {
	NormalEvent(recorder, object, RemediationFinishedEventReason, remediationFinishedEventMessage)
}

// RemediationCannotStart will record a Warning event with reason RemediationCannotStart and custom message.
func RemediationCannotStart(recorder record.EventRecorder, object runtime.Object, message string) {
	WarningEvent(recorder, object, RemediationCannotStartEventReason, message)
}

// GetTargetNodeFailed will record a Warning event with reason RemediationFailed and message "Could not get remediation target node".
func GetTargetNodeFailed(recorder record.EventRecorder, object runtime.Object) {
	RemediationCannotStart(recorder, object, remediationCannotStartTargetNodeFailedEventMessage)
}
