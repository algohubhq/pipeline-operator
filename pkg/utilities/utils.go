package utilities

import (
	"pipeline-operator/pkg/apis/algorun/v1beta1"
	"strings"
)

func GetTopicName(topic string, pipelineSpec *v1beta1.PipelineDeploymentSpecV1beta1) string {
	topicName := strings.ToLower(strings.Replace(topic, "{deploymentownerusername}", pipelineSpec.DeploymentOwner, -1))
	topicName = strings.ToLower(strings.Replace(topicName, "{deploymentname}", pipelineSpec.DeploymentName, -1))

	return topicName
}

func GetAllTopicConfigs(pipelineSpec *v1beta1.PipelineDeploymentSpecV1beta1) []v1beta1.TopicConfigModel {

	allTopics := make([]v1beta1.TopicConfigModel, 0)
	for _, algo := range pipelineSpec.Algos {
		for _, topicConfig := range algo.Topics {
			allTopics = append(allTopics, topicConfig)
		}
	}
	for _, dc := range pipelineSpec.DataConnectors {
		for _, topicConfig := range dc.Topics {
			allTopics = append(allTopics, topicConfig)
		}
	}
	for _, topicConfig := range pipelineSpec.Endpoint.Topics {
		allTopics = append(allTopics, topicConfig)
	}
	for _, topicConfig := range pipelineSpec.Hook.Topics {
		allTopics = append(allTopics, topicConfig)
	}

	return allTopics
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
