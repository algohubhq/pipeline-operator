package utilities

import (
	"pipeline-operator/pkg/apis/algorun/v1beta1"
	"strings"
)

func GetTopicName(topic string, pipelineSpec *v1beta1.PipelineSpec) string {
	topicName := strings.ToLower(strings.Replace(topic, "{deploymentownerusername}", pipelineSpec.DeploymentOwnerUserName, -1))
	topicName = strings.ToLower(strings.Replace(topicName, "{deploymentname}", pipelineSpec.DeploymentName, -1))

	return topicName
}

func Max(x, y int64) int64 {
	if x < y {
		return y
	}
	return x
}

func Int32p(i int32) *int32 {
	return &i
}

func Int64p(i int64) *int64 {
	return &i
}

func Short(s string, i int) string {
	runes := []rune(s)
	if len(runes) > i {
		return string(runes[:i])
	}
	return s
}
