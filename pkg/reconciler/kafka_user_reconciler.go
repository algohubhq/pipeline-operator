package reconciler

import (
	"context"
	"fmt"
	"pipeline-operator/pkg/apis/algorun/v1beta1"
	algov1beta1 "pipeline-operator/pkg/apis/algorun/v1beta1"
	kafkav1beta1 "pipeline-operator/pkg/apis/kafka/v1beta1"
	utils "pipeline-operator/pkg/utilities"
	"sort"

	patch "github.com/banzaicloud/k8s-objectmatcher/patch"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewKafkaUserReconciler returns a new KafkaUserReconciler
func NewKafkaUserReconciler(pipelineDeployment *algov1beta1.PipelineDeployment,
	topicConfigs map[string]*v1beta1.TopicConfigModel,
	kafkaUtil *utils.KafkaUtil,
	request *reconcile.Request,
	apiReader client.Reader,
	client client.Client,
	scheme *runtime.Scheme) KafkaUserReconciler {
	return KafkaUserReconciler{
		pipelineDeployment: pipelineDeployment,
		topicConfigs:       topicConfigs,
		kafkaUtil:          kafkaUtil,
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
	kafkaUtil          *utils.KafkaUtil
	request            *reconcile.Request
	apiReader          client.Reader
	client             client.Client
	scheme             *runtime.Scheme
}

// Reconcile reconciles the Kakfa user for a pipeline
func (kafkaUserReconciler *KafkaUserReconciler) Reconcile() {

	kafkaNamespace := kafkaUserReconciler.kafkaUtil.GetKafkaNamespace()
	pipelineDeploymentSpec := kafkaUserReconciler.pipelineDeployment.Spec

	kafkaUsername := fmt.Sprintf("kafka-%s-%s", pipelineDeploymentSpec.DeploymentOwner,
		pipelineDeploymentSpec.DeploymentName)

	labels := map[string]string{
		"strimzi.io/cluster":           kafkaUserReconciler.kafkaUtil.GetKafkaClusterName(),
		"app.kubernetes.io/part-of":    "algo.run",
		"app.kubernetes.io/component":  "kafka-user",
		"app.kubernetes.io/managed-by": "pipeline-operator",
		"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", pipelineDeploymentSpec.DeploymentOwner,
			pipelineDeploymentSpec.DeploymentName),
		"algo.run/pipeline": fmt.Sprintf("%s.%s", pipelineDeploymentSpec.PipelineOwner,
			pipelineDeploymentSpec.PipelineName),
	}

	kafkaUserSpec := buildKafkaUserSpec(&pipelineDeploymentSpec, kafkaUserReconciler.topicConfigs)

	// check to see if topic already exists
	existingUser := &kafkav1beta1.KafkaUser{}
	err := kafkaUserReconciler.apiReader.Get(context.TODO(),
		types.NamespacedName{
			Name:      kafkaUsername,
			Namespace: kafkaNamespace,
		},
		existingUser)

	user := &kafkav1beta1.KafkaUser{}
	// If this is an update, need to set the existing deployment name
	if existingUser != nil {
		user = existingUser.DeepCopy()
	} else {
		user.SetName(kafkaUsername)
		user.SetNamespace(kafkaNamespace)
		user.SetLabels(labels)
	}
	user.Spec = kafkaUserSpec

	if err != nil && errors.IsNotFound(err) {

		// Set PipelineDeployment instance as the owner and controller
		// if err := controllerutil.SetControllerReference(kafkaUserReconciler.pipelineDeployment, newUser, kafkaUserReconciler.scheme); err != nil {
		// 	log.Error(err, "Failed setting the topic controller owner")
		// }

		err := kafkaUserReconciler.client.Create(context.TODO(), user)
		if err != nil {
			log.Error(err, "Failed creating kafka user")
		}

	} else if err != nil {
		log.Error(err, "Failed to check if Kafka user exists.")
	} else {

		patchMaker := patch.NewPatchMaker(patch.NewAnnotator("algo.run/last-applied"))
		patchResult, err := patchMaker.Calculate(existingUser, user)
		if err != nil {
			log.Error(err, "Failed to calculate Kafka User resource changes")
		}

		if !patchResult.IsEmpty() {
			log.Info("Kafka User Changed. Updating...")
			err := kafkaUserReconciler.client.Update(context.TODO(), user)
			if err != nil {
				log.Error(err, "Failed updating Kafka User")
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
