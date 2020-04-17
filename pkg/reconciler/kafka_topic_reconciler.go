package reconciler

import (
	"context"
	"fmt"
	"math"
	"pipeline-operator/pkg/apis/algorun/v1beta1"
	algov1beta1 "pipeline-operator/pkg/apis/algorun/v1beta1"
	kafkav1beta1 "pipeline-operator/pkg/apis/kafka/v1beta1"
	utils "pipeline-operator/pkg/utilities"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewTopicReconciler returns a new TopicReconciler
func NewTopicReconciler(pipelineDeployment *algov1beta1.PipelineDeployment,
	componentName string,
	topicConfig *v1beta1.TopicConfigModel,
	request *reconcile.Request,
	client client.Client,
	scheme *runtime.Scheme) TopicReconciler {
	return TopicReconciler{
		pipelineDeployment: pipelineDeployment,
		componentName:      componentName,
		topicConfig:        topicConfig,
		request:            request,
		client:             client,
		scheme:             scheme,
	}
}

// TopicReconciler reconciles an Topic object
type TopicReconciler struct {
	pipelineDeployment *algov1beta1.PipelineDeployment
	componentName      string
	topicConfig        *v1beta1.TopicConfigModel
	request            *reconcile.Request
	client             client.Client
	scheme             *runtime.Scheme
}

func (topicReconciler *TopicReconciler) Reconcile() {

	pipelineDeploymentSpec := topicReconciler.pipelineDeployment.Spec

	// Replace the pipelineDeployment username and name in the topic string
	topicName := utils.GetTopicName(topicReconciler.topicConfig.TopicName, &pipelineDeploymentSpec)
	// Replace _ and . for the k8s name
	resourceName := strings.Replace(topicName, ".", "-", -1)
	resourceName = strings.Replace(resourceName, "_", "-", -1)

	newTopicSpec, err := topicReconciler.buildTopicSpec(pipelineDeploymentSpec, topicReconciler.topicConfig)
	if err != nil {
		log.Error(err, "Error creating new topic config")
	}

	// check to see if topic already exists
	existingTopic := &kafkav1beta1.KafkaTopic{}
	existingTopic.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kafka.strimzi.io",
		Kind:    "KafkaTopic",
		Version: "v1beta1",
	})
	err = topicReconciler.client.Get(context.TODO(), types.NamespacedName{Name: resourceName, Namespace: topicReconciler.request.NamespacedName.Namespace}, existingTopic)

	if err != nil && errors.IsNotFound(err) {
		// Create the topic
		labels := map[string]string{
			"strimzi.io/cluster":           utils.GetKafkaClusterName(),
			"app.kubernetes.io/part-of":    "algo.run",
			"app.kubernetes.io/component":  "topic",
			"app.kubernetes.io/managed-by": "pipeline-operator",
			"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", pipelineDeploymentSpec.DeploymentOwner,
				pipelineDeploymentSpec.DeploymentName),
			"algo.run/pipeline": fmt.Sprintf("%s.%s", pipelineDeploymentSpec.PipelineOwner,
				pipelineDeploymentSpec.PipelineName),
		}

		newTopic := &kafkav1beta1.KafkaTopic{}
		newTopic.Spec = newTopicSpec
		newTopic.Spec.TopicName = topicName
		newTopic.SetName(resourceName)
		newTopic.SetNamespace(topicReconciler.request.NamespacedName.Namespace)
		newTopic.SetLabels(labels)
		newTopic.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "kafka.strimzi.io",
			Kind:    "KafkaTopic",
			Version: "v1beta1",
		})

		// Set PipelineDeployment instance as the owner and controller
		if err := controllerutil.SetControllerReference(topicReconciler.pipelineDeployment, newTopic, topicReconciler.scheme); err != nil {
			log.Error(err, "Failed setting the topic controller owner")
		}

		err := topicReconciler.client.Create(context.TODO(), newTopic)
		if err != nil {
			log.Error(err, "Failed creating topic")
		}

	} else if err != nil {
		log.Error(err, "Failed to check if Kafka topic exists.")
	} else {

		if existingTopic.Spec.Partitions > newTopicSpec.Partitions {
			logData := map[string]interface{}{
				"partitionsCurrent": existingTopic.Spec.Partitions,
				"partitionsNew":     newTopicSpec.Partitions,
			}
			log.WithValues("data", logData)
			log.Error(err, "Partition count cannot be decreased. Keeping current partition count.")
			newTopicSpec.Partitions = existingTopic.Spec.Partitions
		}

		if !reflect.DeepEqual(existingTopic, newTopicSpec) {

			// Update the existing spec
			existingTopic.Spec = newTopicSpec

			err := topicReconciler.client.Update(context.TODO(), existingTopic)
			if err != nil {
				log.Error(err, "Failed updating topic")
			}

		}

	}

}

func (topicReconciler *TopicReconciler) buildTopicSpec(pipelineSpec algov1beta1.PipelineDeploymentSpecV1beta1, topicConfig *algov1beta1.TopicConfigModel) (kafkav1beta1.KafkaTopicSpec, error) {

	var topicPartitions int64 = 1
	if topicConfig.AutoPartition {
		// Set the topic partitions by summing the destination instance count + 50%
		for _, pipe := range pipelineSpec.Pipes {

			// Match the Source Pipe
			if pipe.SourceName == topicReconciler.componentName &&
				pipe.SourceOutputName == topicConfig.OutputName {

				// Find all destination Algos
				for _, algoConfig := range pipelineSpec.Algos {
					algoName := utils.GetAlgoFullName(&algoConfig)
					if algoName == pipe.DestName {
						// In case MaxInstances is Zero, use the topicPartitions
						maxPartitions := utils.Max(int64(algoConfig.Autoscaling.MaxReplicas), topicPartitions)
						maxPartitions = utils.Max(int64(algoConfig.Replicas), maxPartitions)
						topicPartitions = topicPartitions + maxPartitions
					}
				}

				// Find all destination Sinks
				for _, dcConfig := range pipelineSpec.DataConnectors {
					dcName := fmt.Sprintf("%s:%s[%d]", dcConfig.Name, dcConfig.VersionTag, dcConfig.Index)
					if dcName == pipe.DestName {
						// TODO: Implement resource scaling for data connectors
						// For now, just use one instance
						// In case MaxInstances is Zero, use the topicPartitions
						// maxPartitions := utils.Max(int64(dcConfig.Resource.MaxInstances), topicPartitions)
						// maxPartitions = utils.Max(int64(dcConfig.Resource.Instances), maxPartitions)
						topicPartitions = topicPartitions + 1
					}
				}

				// Find all destination Hooks
				if strings.ToLower(pipe.DestName) == "hook" {
					// In case MaxInstances is Zero, use the topicPartitions
					maxPartitions := utils.Max(int64(pipelineSpec.Hook.Autoscaling.MaxReplicas), topicPartitions)
					maxPartitions = utils.Max(int64(pipelineSpec.Hook.Replicas), maxPartitions)
					topicPartitions = topicPartitions + maxPartitions
				}

			}
		}

		// Pad the count with an extra 50%. It's better to over provision partitions in Kafka
		topicPartitions = topicPartitions + int64(math.Round(float64(topicPartitions)*0.5))

	} else {
		if topicConfig.Partitions > 0 {
			topicPartitions = int64(topicConfig.Partitions)
		}
	}

	config := make(map[string]string)
	for _, topicParam := range topicConfig.TopicParams {
		config[topicParam.Name] = topicParam.Value
	}

	newTopicSpec := kafkav1beta1.KafkaTopicSpec{
		Partitions: topicPartitions,
		Replicas:   topicConfig.ReplicationFactor,
		Config:     config,
	}

	return newTopicSpec, nil

}
