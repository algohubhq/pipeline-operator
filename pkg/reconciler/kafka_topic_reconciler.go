package reconciler

import (
	"context"
	"fmt"
	"math"
	"pipeline-operator/pkg/apis/algorun/v1beta1"
	algov1beta1 "pipeline-operator/pkg/apis/algorun/v1beta1"
	kafkav1beta1 "pipeline-operator/pkg/apis/kafka/v1beta1"
	utils "pipeline-operator/pkg/utilities"
	"strings"

	patch "github.com/banzaicloud/k8s-objectmatcher/patch"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewTopicReconciler returns a new TopicReconciler
func NewTopicReconciler(pipelineDeployment *algov1beta1.PipelineDeployment,
	componentName string,
	topicConfig *v1beta1.TopicConfigModel,
	kafkaUtil *utils.KafkaUtil,
	request *reconcile.Request,
	manager manager.Manager,
	scheme *runtime.Scheme) TopicReconciler {

	return TopicReconciler{
		pipelineDeployment: pipelineDeployment,
		componentName:      componentName,
		topicConfig:        topicConfig,
		kafkaUtil:          kafkaUtil,
		request:            request,
		manager:            manager,
		scheme:             scheme,
	}
}

// TopicReconciler reconciles an Topic object
type TopicReconciler struct {
	pipelineDeployment *algov1beta1.PipelineDeployment
	componentName      string
	topicConfig        *v1beta1.TopicConfigModel
	kafkaUtil          *utils.KafkaUtil
	request            *reconcile.Request
	manager            manager.Manager
	scheme             *runtime.Scheme
}

// Reconcile executes the Kafka Topic reconciliation process
func (topicReconciler *TopicReconciler) Reconcile() {

	kafkaNamespace := topicReconciler.kafkaUtil.GetKafkaNamespace()
	pipelineDeploymentSpec := topicReconciler.pipelineDeployment.Spec

	// Replace the pipelineDeployment username and name in the topic string
	topicName := utils.GetTopicName(topicReconciler.topicConfig.TopicName, &pipelineDeploymentSpec)
	// Replace _ and . for the k8s name
	resourceName := strings.Replace(topicName, ".", "-", -1)
	resourceName = strings.Replace(resourceName, "_", "-", -1)

	// Create the topic
	labels := map[string]string{
		"strimzi.io/cluster":           topicReconciler.kafkaUtil.GetKafkaClusterName(),
		"app.kubernetes.io/part-of":    "algo.run",
		"app.kubernetes.io/component":  "topic",
		"app.kubernetes.io/managed-by": "pipeline-operator",
		"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", pipelineDeploymentSpec.DeploymentOwner,
			pipelineDeploymentSpec.DeploymentName),
		"algo.run/pipeline": fmt.Sprintf("%s.%s", pipelineDeploymentSpec.PipelineOwner,
			pipelineDeploymentSpec.PipelineName),
	}

	newTopicSpec, err := topicReconciler.buildTopicSpec(pipelineDeploymentSpec, topicReconciler.topicConfig)
	if err != nil {
		log.Error(err, "Error creating new topic config")
	}

	// check to see if topic already exists
	existingTopic := &kafkav1beta1.KafkaTopic{}
	// Use the APIReader to query across namespaces
	err = topicReconciler.manager.GetAPIReader().Get(context.TODO(),
		types.NamespacedName{
			Name:      resourceName,
			Namespace: kafkaNamespace,
		},
		existingTopic)

	topic := &kafkav1beta1.KafkaTopic{}
	// If this is an update, need to set the existing deployment name
	if existingTopic != nil {
		topic = existingTopic.DeepCopy()
	} else {
		topic.SetName(resourceName)
		topic.SetNamespace(kafkaNamespace)
		topic.SetLabels(labels)
	}
	topic.Spec = newTopicSpec
	topic.Spec.TopicName = topicName

	if err != nil && errors.IsNotFound(err) {
		// Set PipelineDeployment instance as the owner and controller
		// if err := controllerutil.SetControllerReference(topicReconciler.pipelineDeployment, newTopic, topicReconciler.scheme); err != nil {
		// 	log.Error(err, "Failed setting the topic controller owner")
		// }

		err := topicReconciler.manager.GetClient().Create(context.TODO(), topic)
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

		patchMaker := patch.NewPatchMaker(patch.NewAnnotator("algo.run/last-applied"))
		patchResult, err := patchMaker.Calculate(existingTopic, topic)
		if err != nil {
			log.Error(err, "Failed to calculate Kafka Topic resource changes")
		}

		if !patchResult.IsEmpty() {
			log.Info("Kafka Topic Changed. Updating...")
			err := topicReconciler.manager.GetClient().Update(context.TODO(), topic)
			if err != nil {
				log.Error(err, "Failed updating topic")
			}
		}

	}

}

func (topicReconciler *TopicReconciler) buildTopicSpec(pipelineSpec algov1beta1.PipelineDeploymentSpecV1beta1,
	topicConfig *algov1beta1.TopicConfigModel) (kafkav1beta1.KafkaTopicSpec, error) {

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
						maxPartitions := utils.Max(int64(algoConfig.Replicas), topicPartitions)
						if algoConfig.Autoscaling != nil {
							maxPartitions = utils.Max(int64(algoConfig.Autoscaling.MaxReplicas), maxPartitions)
						}

						topicPartitions = topicPartitions + maxPartitions
					}
				}

				// Find all destination Sinks
				for _, dcConfig := range pipelineSpec.DataConnectors {
					dcName := fmt.Sprintf("%s:%s[%d]", dcConfig.Spec.Name, dcConfig.Version, dcConfig.Index)
					if dcName == pipe.DestName {
						maxPartitions := utils.Max(int64(dcConfig.Replicas), topicPartitions)
						if dcConfig.Autoscaling != nil {
							maxPartitions = utils.Max(int64(dcConfig.Autoscaling.MaxReplicas), maxPartitions)
						}

						topicPartitions = topicPartitions + maxPartitions
					}
				}

				// Find all destination Hooks
				if strings.ToLower(pipe.DestName) == "hook" {
					maxPartitions := utils.Max(int64(pipelineSpec.EventHook.Replicas), topicPartitions)
					if pipelineSpec.EventHook.Autoscaling != nil {
						maxPartitions = utils.Max(int64(pipelineSpec.EventHook.Autoscaling.MaxReplicas), maxPartitions)
					}

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
