package reconciler

import (
	"context"
	"fmt"
	"pipeline-operator/pkg/apis/algorun/v1beta1"
	algov1beta1 "pipeline-operator/pkg/apis/algorun/v1beta1"
	utils "pipeline-operator/pkg/utilities"

	"github.com/go-test/deep"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewKafkaUserReconciler returns a new KafkaUserReconciler
func NewKafkaUserReconciler(pipelineDeployment *algov1beta1.PipelineDeployment,
	topicConfigs []v1beta1.TopicConfigModel,
	request *reconcile.Request,
	client client.Client,
	scheme *runtime.Scheme) KafkaUserReconciler {
	return KafkaUserReconciler{
		pipelineDeployment: pipelineDeployment,
		topicConfigs:       topicConfigs,
		request:            request,
		client:             client,
		scheme:             scheme,
	}
}

// KafkaUserReconciler reconciles the Kakfa user for a pipeline
type KafkaUserReconciler struct {
	pipelineDeployment *algov1beta1.PipelineDeployment
	topicConfigs       []v1beta1.TopicConfigModel
	request            *reconcile.Request
	client             client.Client
	scheme             *runtime.Scheme
}

// Reconcile reconciles the Kakfa user for a pipeline
func (kafkaUserReconciler *KafkaUserReconciler) Reconcile() {

	pipelineDeploymentSpec := kafkaUserReconciler.pipelineDeployment.Spec

	kafkaUsername := fmt.Sprintf("kafka-%s-%s", pipelineDeploymentSpec.PipelineSpec.DeploymentOwnerUserName,
		pipelineDeploymentSpec.PipelineSpec.DeploymentName)

	kafkaUserSpec := buildKafkaUserSpec(&pipelineDeploymentSpec.PipelineSpec, kafkaUserReconciler.topicConfigs)

	// check to see if topic already exists
	existingUser := &unstructured.Unstructured{}
	existingUser.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kafka.strimzi.io",
		Kind:    "KafkaUser",
		Version: "v1beta1",
	})
	err := kafkaUserReconciler.client.Get(context.TODO(), types.NamespacedName{Name: kafkaUsername, Namespace: kafkaUserReconciler.request.NamespacedName.Namespace}, existingUser)

	if err != nil && errors.IsNotFound(err) {
		// Create the topic
		// Using a unstructured object to submit a strimzi topic creation.
		labels := map[string]string{
			"strimzi.io/cluster":           utils.GetKafkaClusterName(),
			"app.kubernetes.io/part-of":    "algo.run",
			"app.kubernetes.io/component":  "kafka-user",
			"app.kubernetes.io/managed-by": "pipeline-operator",
			"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", pipelineDeploymentSpec.PipelineSpec.DeploymentOwnerUserName,
				pipelineDeploymentSpec.PipelineSpec.DeploymentName),
			"algo.run/pipeline": fmt.Sprintf("%s.%s", pipelineDeploymentSpec.PipelineSpec.PipelineOwnerUserName,
				pipelineDeploymentSpec.PipelineSpec.PipelineName),
		}

		newUser := &unstructured.Unstructured{}
		newUser.Object = map[string]interface{}{
			"name":      kafkaUsername,
			"namespace": kafkaUserReconciler.request.NamespacedName.Namespace,
			"spec":      kafkaUserSpec,
		}
		newUser.SetName(kafkaUsername)
		newUser.SetNamespace(kafkaUserReconciler.request.NamespacedName.Namespace)
		newUser.SetLabels(labels)
		newUser.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "kafka.strimzi.io",
			Kind:    "KafkaUser",
			Version: "v1beta1",
		})

		// Set PipelineDeployment instance as the owner and controller
		if err := controllerutil.SetControllerReference(kafkaUserReconciler.pipelineDeployment, newUser, kafkaUserReconciler.scheme); err != nil {
			log.Error(err, "Failed setting the topic controller owner")
		}

		err := kafkaUserReconciler.client.Create(context.TODO(), newUser)
		if err != nil {
			log.Error(err, "Failed creating kafka user")
		}
	} else if err != nil {
		log.Error(err, "Failed to check if Kafka user exists.")
	} else {
		// Update the user if changed
		var deplChanged bool

		spec, ok := existingUser.Object["spec"].(map[string]interface{})
		if ok {
			if diff := deep.Equal(spec, kafkaUserSpec); diff != nil {
				log.Info("Kafka User Changed. Updating User.", "Differences", diff)
				deplChanged = true
			}
		}

		if deplChanged {

			// Update the existing spec
			existingUser.Object["spec"] = kafkaUserSpec

			// err := kafkaUserReconciler.client.Update(context.TODO(), existingUser)
			// if err != nil {
			// 	log.Error(err, "Failed updating kafka user")
			// }

		}

	}

}

func buildKafkaUserSpec(pipelineSpec *algov1beta1.PipelineSpec, topicConfigs []algov1beta1.TopicConfigModel) map[string]interface{} {

	// Create the acl list based on all of the Topic configs for this deployment
	resources := make([]map[string]interface{}, 0)
	for _, topicConfig := range topicConfigs {
		topicName := utils.GetTopicName(topicConfig.TopicName, pipelineSpec)
		resource := map[string]interface{}{
			"operation": "All",
			"resource": map[string]interface{}{
				"type":        "topic",
				"name":        topicName,
				"patternType": "literal",
			},
		}
		resources = append(resources, resource)
	}

	kafkaUserSpec := map[string]interface{}{
		"authentication": map[string]interface{}{
			"type": "tls",
		},
		"authorization": map[string]interface{}{
			"type": "simple",
			"acls": resources,
		},
	}

	return kafkaUserSpec

}
