package command

import corev1 "k8s.io/api/core/v1"

type optionType int

const (
	useCustomizedPod optionType = iota
	noOutputExpected
)

type RunOption interface {
	getOptionType() optionType
	getOptionValue() interface{}
}

type runOption struct {
	optionType
	value interface{}
}

func (ro *runOption) getOptionType() optionType {
	return ro.optionType
}

func (ro *runOption) getOptionValue() interface{} {
	return ro.value
}

// CreateOptionUseCustomizedExecutePod allows executing a command on a pod provided by this option instead of the default one
func CreateOptionUseCustomizedExecutePod(pod *corev1.Pod) RunOption {
	return &runOption{useCustomizedPod, pod}
}

// CreateOptionNoExpectedOutput allows executing a command on a pod when no output is expected from the command
func CreateOptionNoExpectedOutput() RunOption {
	return &runOption{optionType: noOutputExpected}
}

func convertToMap(opts []RunOption) map[optionType]interface{} {
	runOptions := make(map[optionType]interface{})
	for _, option := range opts {
		runOptions[option.getOptionType()] = option.getOptionValue()
	}

	return runOptions
}
