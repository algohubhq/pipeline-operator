package reconciler

import (
	"context"
	utils "endpoint-operator/internal/utilities"
	"endpoint-operator/pkg/apis/algo/v1alpha1"
	algov1alpha1 "endpoint-operator/pkg/apis/algo/v1alpha1"
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewTopicReconciler returns a new TopicReconciler
func NewTopicReconciler(endpoint *algov1alpha1.Endpoint,
	topicConfig *v1alpha1.TopicConfigModel,
	request *reconcile.Request,
	client client.Client,
	scheme *runtime.Scheme) TopicReconciler {
	return TopicReconciler{
		endpoint:    endpoint,
		topicConfig: topicConfig,
		request:     request,
		client:      client,
		scheme:      scheme,
	}
}

// TopicReconciler reconciles an Topic object
type TopicReconciler struct {
	endpoint    *algov1alpha1.Endpoint
	topicConfig *v1alpha1.TopicConfigModel
	request     *reconcile.Request
	client      client.Client
	scheme      *runtime.Scheme
}

type TopicConfig struct {
	Name       string
	Partitions int64
	Replicas   int64
	Params     map[string]string
}

func (topicReconciler *TopicReconciler) Reconcile() {

	endpointSpec := topicReconciler.endpoint.Spec

	newTopicConfig, err := BuildTopic(endpointSpec.EndpointConfig, topicReconciler.topicConfig)
	if err != nil {
		log.Error(err, "Error creating new topic config")
	}

	// check to see if topic already exists
	existingTopic := &unstructured.Unstructured{}
	existingTopic.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kafka.strimzi.io",
		Kind:    "KafkaTopic",
		Version: "v1alpha1",
	})
	err = topicReconciler.client.Get(context.TODO(), types.NamespacedName{Name: newTopicConfig.Name, Namespace: topicReconciler.request.NamespacedName.Namespace}, existingTopic)

	if err != nil && errors.IsNotFound(err) {
		// Create the topic
		// Using a unstructured object to submit a strimzi topic creation.
		newTopic := &unstructured.Unstructured{}
		newTopic.Object = map[string]interface{}{
			"name":      newTopicConfig.Name,
			"namespace": topicReconciler.request.NamespacedName.Namespace,
			"spec": map[string]interface{}{
				"partitions": newTopicConfig.Partitions,
				"replicas":   int(topicReconciler.topicConfig.TopicReplicationFactor),
				"config":     newTopicConfig.Params,
			},
		}
		newTopic.SetName(newTopicConfig.Name)
		newTopic.SetNamespace(topicReconciler.request.NamespacedName.Namespace)
		newTopic.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "kafka.strimzi.io",
			Kind:    "KafkaTopic",
			Version: "v1alpha1",
		})

		// Set Endpoint instance as the owner and controller
		if err := controllerutil.SetControllerReference(topicReconciler.endpoint, newTopic, topicReconciler.scheme); err != nil {
			log.Error(err, "Failed setting the topic controller owner")
		}

		err := topicReconciler.client.Create(context.TODO(), newTopic)
		if err != nil {
			log.Error(err, "Failed creating topic")
		} else {
			utils.TopicCountGuage.Add(1)
		}
	} else if err != nil {
		log.Error(err, "Failed to check if Kafka topic exists.")
	} else {
		// Update the topic if changed
		// Check that the partition count did not go down (kafka doesn't support)
		var partitionsCurrent, replicasCurrent int64
		var paramsCurrent map[string]string
		spec, ok := existingTopic.Object["spec"].(map[string]interface{})
		if ok {
			replicasCurrent, ok = spec["replicas"].(int64)
			paramsCurrent, ok = spec["config"].(map[string]string)
			partitionsCurrent, ok = spec["partitions"].(int64)
			if ok {
				if partitionsCurrent > newTopicConfig.Partitions {
					logData := map[string]interface{}{
						"partitionsCurrent": partitionsCurrent,
						"partitionsNew":     newTopicConfig.Partitions,
					}
					log.WithValues("data", logData)
					log.Error(err, "Partition count cannot be decreased. Keeping current partition count.")
					newTopicConfig.Partitions = partitionsCurrent
				}
			}
		}

		if partitionsCurrent != newTopicConfig.Partitions ||
			replicasCurrent != newTopicConfig.Replicas ||
			(len(paramsCurrent) != len(newTopicConfig.Params) &&
				!reflect.DeepEqual(paramsCurrent, newTopicConfig.Params)) {

			fmt.Printf("%v", paramsCurrent)
			// !reflect.DeepEqual(paramsCurrent, params)
			// Update the existing spec
			spec["partitions"] = newTopicConfig.Partitions
			spec["replicas"] = newTopicConfig.Replicas
			spec["config"] = newTopicConfig.Params

			existingTopic.Object["spec"] = spec

			err := topicReconciler.client.Update(context.TODO(), existingTopic)
			if err != nil {
				log.Error(err, "Failed updating topic")
			}

		}

	}

}

func BuildTopic(endpointConfig algov1alpha1.EndpointConfig, topicConfig *algov1alpha1.TopicConfigModel) (TopicConfig, error) {

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
						topicPartitions = utils.Max(int64(algoConfig.MinInstances), topicPartitions)
						topicPartitions = utils.Max(int64(algoConfig.Instances), topicPartitions)
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
