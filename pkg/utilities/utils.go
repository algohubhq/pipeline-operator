package utilities

import (
	"fmt"
	"pipeline-operator/pkg/apis/algorun/v1beta1"
	"strings"
)

func GetAlgoFullName(algoSpec *v1beta1.AlgoSpec) string {
	algoName := fmt.Sprintf("%s/%s:%s[%d]", algoSpec.Owner, algoSpec.Name, algoSpec.Version, algoSpec.Index)
	return algoName
}

func GetDcFullName(dcSpec *v1beta1.DataConnectorSpec) string {
	dcName := fmt.Sprintf("%s:%s[%d]", dcSpec.Name, dcSpec.VersionTag, dcSpec.Index)
	return dcName
}

func GetTopicName(topic string, pipelineSpec *v1beta1.PipelineDeploymentSpecV1beta1) string {
	topicName := strings.ToLower(strings.Replace(topic, "{deploymentownerusername}", pipelineSpec.DeploymentOwner, -1))
	topicName = strings.ToLower(strings.Replace(topicName, "{deploymentname}", pipelineSpec.DeploymentName, -1))
	return topicName
}

func GetAllTopicConfigs(pipelineSpec *v1beta1.PipelineDeploymentSpecV1beta1) map[string]*v1beta1.TopicConfigModel {

	allTopics := make(map[string]*v1beta1.TopicConfigModel, 0)
	for _, algo := range pipelineSpec.Algos {
		for _, output := range algo.Outputs {
			source := fmt.Sprintf("%s|%s", GetAlgoFullName(&algo), output.Name)
			allTopics[source] = output.Topic
		}
	}
	for _, dc := range pipelineSpec.DataConnectors {
		for _, topicConfig := range dc.Topics {
			source := fmt.Sprintf("%s|%s", GetDcFullName(&dc), topicConfig.OutputName)
			allTopics[source] = &topicConfig
		}
	}
	for _, path := range pipelineSpec.Endpoint.Paths {
		source := fmt.Sprintf("Endpoint|%s", path.Name)
		allTopics[source] = path.Topic
	}
	for _, topicConfig := range pipelineSpec.Hook.Topics {
		source := fmt.Sprintf("Hook|%s", topicConfig.OutputName)
		allTopics[source] = &topicConfig
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
