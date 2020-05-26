package utilities

import (
	"fmt"
	"pipeline-operator/pkg/apis/algorun/v1beta1"
	"strings"
)

func GetAlgoFullName(algoSpec *v1beta1.AlgoDeploymentV1beta1) string {
	algoName := fmt.Sprintf("%s/%s:%s[%d]", algoSpec.Spec.Owner, algoSpec.Spec.Name, algoSpec.Version, algoSpec.Index)
	return algoName
}

func GetDcFullName(dcSpec *v1beta1.DataConnectorDeploymentV1beta1) string {
	dcName := fmt.Sprintf("%s:%s[%d]", dcSpec.Spec.Name, dcSpec.Version, dcSpec.Index)
	return dcName
}

func GetTopicName(topic string, pipelineSpec *v1beta1.PipelineDeploymentSpecV1beta1) string {
	topicName := strings.ToLower(strings.Replace(topic, "{deploymentowner}", pipelineSpec.DeploymentOwner, -1))
	topicName = strings.ToLower(strings.Replace(topicName, "{deploymentname}", pipelineSpec.DeploymentName, -1))
	return topicName
}

func GetAllTopicConfigs(pipelineSpec *v1beta1.PipelineDeploymentSpecV1beta1) map[string]*v1beta1.TopicConfigModel {

	allTopics := make(map[string]*v1beta1.TopicConfigModel, 0)
	for _, algo := range pipelineSpec.Algos {
		for _, topicConfig := range algo.Topics {
			source := fmt.Sprintf("%s|%s", GetAlgoFullName(&algo), topicConfig.OutputName)
			// Do all topic name string replacements
			topicConfig.TopicName = GetTopicName(topicConfig.TopicName, pipelineSpec)
			allTopics[source] = &topicConfig
		}
	}
	for _, dc := range pipelineSpec.DataConnectors {
		for _, topicConfig := range dc.Topics {
			source := fmt.Sprintf("%s|%s", GetDcFullName(&dc), topicConfig.OutputName)
			// Do all topic name string replacements
			topicConfig.TopicName = GetTopicName(topicConfig.TopicName, pipelineSpec)
			allTopics[source] = &topicConfig
		}
	}
	for _, path := range pipelineSpec.Endpoint.Paths {
		source := fmt.Sprintf("Endpoint|%s", path.Name)
		// Do all topic name string replacements
		path.Topic.TopicName = GetTopicName(path.Topic.TopicName, pipelineSpec)
		allTopics[source] = path.Topic
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
