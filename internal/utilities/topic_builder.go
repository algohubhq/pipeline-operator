package utilities

import (
	algov1alpha1 "endpoint-operator/pkg/apis/algo/v1alpha1"
	"fmt"
	"strings"
)

type TopicConfig struct {
	Name       string
	Partitions int64
	Replicas   int64
	Params     map[string]string
}

func BuildTopic(endpointConfig algov1alpha1.EndpointConfig, topicConfig algov1alpha1.TopicConfigModel) (TopicConfig, error) {

	// Replace the endpoint username and name in the topic string
	topicName := strings.ToLower(strings.Replace(topicConfig.TopicName, "{endpointownerusername}", endpointConfig.EndpointOwnerUserName, -1))
	topicName = strings.ToLower(strings.Replace(topicName, "{endpointname}", endpointConfig.EndpointName, -1))

	logData := map[string]interface{}{
		"Topic": topicName,
	}
	log.WithValues("data", logData)

	var topicPartitions int64 = 1
	if topicConfig.TopicAutoPartition {
		// Set the topic partitions based on the max destination instance count
		for _, pipe := range endpointConfig.Pipes {

			// Match the Source Pipe
			if pipe.SourceName == topicConfig.SourceName &&
				pipe.SourceOutputName == topicConfig.SourceOutputName {

				// Find the destination Algo
				for _, algoConfig := range endpointConfig.AlgoConfigs {
					algoName := fmt.Sprintf("%s/%s:%s[%d]", algoConfig.AlgoOwnerUserName, algoConfig.AlgoName, algoConfig.AlgoVersionTag, algoConfig.AlgoIndex)
					if algoName == pipe.DestName {
						topicPartitions = Max(int64(algoConfig.MinInstances), topicPartitions)
						topicPartitions = Max(int64(algoConfig.Instances), topicPartitions)
					}

				}

			}
		}

	} else {
		if topicConfig.TopicPartitions > 0 {
			topicPartitions = int64(topicConfig.TopicPartitions)
		}
	}

	params := make(map[string]string)
	for _, topicParam := range topicConfig.TopicParams {
		params[topicParam.Name] = topicParam.Value
	}

	newTopicConfig := TopicConfig{
		Name:       topicName,
		Partitions: topicPartitions,
		Replicas:   int64(topicConfig.TopicReplicationFactor),
		Params:     params,
	}

	return newTopicConfig, nil

}
