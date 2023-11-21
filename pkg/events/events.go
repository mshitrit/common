package events

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

// Event message format "medik8s <operator shortname> <message>"
const customFmt = "[remediation] %s"

// NormalEventf will record an event with type Normal and fixed message.
func NormalEvent(recorder record.EventRecorder, object runtime.Object, reason, message string) {
	recorder.Event(object, "Normal", reason, fmt.Sprintf("[remediation] %s", message))
}

// NormalEventf will record an event with type Normal and formatted message.
func NormalEventf(recorder record.EventRecorder, object runtime.Object, reason, messageFmt string, a ...interface{}) {
	message := fmt.Sprintf(messageFmt, a...)
	recorder.Event(object, "Normal", reason, fmt.Sprintf("[remediation] %s", message))
}

// WarningEventf will record an event with type Warning and fixed message.
func WarningEvent(recorder record.EventRecorder, object runtime.Object, reason, message string) {
	recorder.Event(object, "Warning", reason, fmt.Sprintf("[remediation] %s", message))
}

// WarningEventf will record an event with type Warning and formatted message.
func WarningEventf(recorder record.EventRecorder, object runtime.Object, reason, messageFmt string, a ...interface{}) {
	message := fmt.Sprintf(messageFmt, a...)
	recorder.Event(object, "Warning", reason, fmt.Sprintf("[remediation] %s", message))
}

// Special case events

// RemediationStarted will record a Normal event with reason RemediationStarted and message Remediation started.
func RemediationStarted(recorder record.EventRecorder, object runtime.Object) {
	NormalEvent(recorder, object, "RemediationStarted", "Remediation started")
}

// RemediationStoppedByNHC will record a Normal event with reason RemediationStopped.
func RemediationStoppedByNHC(recorder record.EventRecorder, object runtime.Object) {
	NormalEvent(recorder, object, "RemediationStopped", "NHC added the timed-out annotation, remediation will be stopped")
}

// RemediationFinished will record a Normal event with reason RemediationFinished and message Remediation finished.
func RemediationFinished(recorder record.EventRecorder, object runtime.Object) {
	NormalEvent(recorder, object, "RemediationFinished", "Remediation finished")
}
