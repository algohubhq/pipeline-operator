package reconciler

import (
	"context"
	"fmt"
	"pipeline-operator/pkg/apis/algorun/v1beta1"
	algov1beta1 "pipeline-operator/pkg/apis/algorun/v1beta1"
	kafkav1beta1 "pipeline-operator/pkg/apis/kafka/v1beta1"
	utils "pipeline-operator/pkg/utilities"
	"sort"

	"github.com/go-test/deep"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewKafkaUserReconciler returns a new KafkaUserReconciler
func NewKafkaUserReconciler(pipelineDeployment *algov1beta1.PipelineDeployment,
	topicConfigs map[string]*v1beta1.TopicConfigModel,
	request *reconcile.Request,
	apiReader client.Reader,
	client client.Client,
	scheme *runtime.Scheme) KafkaUserReconciler {
	return KafkaUserReconciler{
		pipelineDeployment: pipelineDeployment,
		topicConfigs:       topicConfigs,
		request:            request,
		apiReader:          apiReader,
		client:             client,
		scheme:             scheme,
	}
}

// KafkaUserReconciler reconciles the Kakfa user for a pipeline
type KafkaUserReconciler struct {
	pipelineDeployment *algov1beta1.PipelineDeployment
	topicConfigs       map[string]*v1beta1.TopicConfigModel
	request            *reconcile.Request
	apiReader          client.Reader
	client             client.Client
	scheme             *runtime.Scheme
}

// Reconcile reconciles the Kakfa user for a pipeline
func (kafkaUserReconciler *KafkaUserReconciler) Reconcile() {

	kafkaNamespace := utils.GetKafkaNamespace()
	pipelineDeploymentSpec := kafkaUserReconciler.pipelineDeployment.Spec

	kafkaUsername := fmt.Sprintf("kafka-%s-%s", pipelineDeploymentSpec.DeploymentOwner,
		pipelineDeploymentSpec.DeploymentName)

	kafkaUserSpec := buildKafkaUserSpec(&pipelineDeploymentSpec, kafkaUserReconciler.topicConfigs)

	// check to see if topic already exists
	existingUser := &kafkav1beta1.KafkaUser{}
	err := kafkaUserReconciler.apiReader.Get(context.TODO(),
		types.NamespacedName{
			Name:      kafkaUsername,
			Namespace: kafkaNamespace,
		},
		existingUser)

	if err != nil && errors.IsNotFound(err) {
		// Create the topic
		labels := map[string]string{
			"strimzi.io/cluster":           utils.GetKafkaClusterName(),
			"app.kubernetes.io/part-of":    "algo.run",
			"app.kubernetes.io/component":  "kafka-user",
			"app.kubernetes.io/managed-by": "pipeline-operator",
			"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", pipelineDeploymentSpec.DeploymentOwner,
				pipelineDeploymentSpec.DeploymentName),
			"algo.run/pipeline": fmt.Sprintf("%s.%s", pipelineDeploymentSpec.PipelineOwner,
				pipelineDeploymentSpec.PipelineName),
		}

		newUser := &kafkav1beta1.KafkaUser{}
		newUser.SetName(kafkaUsername)
		newUser.SetNamespace(kafkaNamespace)
		newUser.SetLabels(labels)
		newUser.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "kafka.strimzi.io",
			Kind:    "KafkaUser",
			Version: "v1beta1",
		})

		newUser.Spec = kafkaUserSpec

		// Set PipelineDeployment instance as the owner and controller
		// if err := controllerutil.SetControllerReference(kafkaUserReconciler.pipelineDeployment, newUser, kafkaUserReconciler.scheme); err != nil {
		// 	log.Error(err, "Failed setting the topic controller owner")
		// }

		err := kafkaUserReconciler.client.Create(context.TODO(), newUser)
		if err != nil {
			log.Error(err, "Failed creating kafka user")
		}

	} else if err != nil {
		log.Error(err, "Failed to check if Kafka user exists.")
	} else {
		// Update the user if changed
		var deplChanged bool
		if diff := deep.Equal(existingUser.Spec, kafkaUserSpec); diff != nil {
			log.Info("Kafka User Changed. Updating User.", "Differences", diff)
			deplChanged = true
		}

		if deplChanged {

			// Update the existing spec
			existingUser.Spec = kafkaUserSpec

			err := kafkaUserReconciler.client.Update(context.TODO(), existingUser)
			if err != nil {
				log.Error(err, "Failed updating kafka user")
			}

		}

	}

}

func buildKafkaUserSpec(pipelineSpec *algov1beta1.PipelineDeploymentSpecV1beta1, allTopicConfigs map[string]*v1beta1.TopicConfigModel) kafkav1beta1.KafkaUserSpec {

	// Sort the topics as the order will matter when reconciling differences
	var keys []string
	for k := range allTopicConfigs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Create the acl list based on all of the Topic configs for this deployment
	resources := make([]kafkav1beta1.KakfaUserAcl, 0)
	for _, k := range keys {
		topicConfig := allTopicConfigs[k]
		topicName := utils.GetTopicName(topicConfig.TopicName, pipelineSpec)
		resource := kafkav1beta1.KakfaUserAcl{
			Operation: "All",
			Resource: kafkav1beta1.KakfaUserAclResource{
				Type:        "topic",
				Name:        topicName,
				PatternType: "literal",
			},
		}
		resources = append(resources, resource)
	}

	groupResource := kafkav1beta1.KakfaUserAcl{
		Operation: "All",
		Resource: kafkav1beta1.KakfaUserAclResource{
			Type:        "group",
			Name:        "algorun",
			PatternType: "prefix",
		},
	}
	resources = append(resources, groupResource)

	kafkaUserSpec := kafkav1beta1.KafkaUserSpec{
		Authentication: kafkav1beta1.KakfaUserAuthentication{
			Type: "tls",
		},
		Authorization: kafkav1beta1.KakfaUserAuthorization{
			Type: "simple",
			Acls: resources,
		},
	}

	return kafkaUserSpec

}
