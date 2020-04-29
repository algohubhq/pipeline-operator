package utilities

import (
	"context"
	"fmt"
	"os"
	algov1beta1 "pipeline-operator/pkg/apis/algorun/v1beta1"
	"strconv"

	"github.com/go-test/deep"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// NewKafkaUtil returns a new KafkaUtil
func NewKafkaUtil(pipelineDeployment *algov1beta1.PipelineDeployment,
	manager manager.Manager,
	scheme *runtime.Scheme) KafkaUtil {
	return KafkaUtil{
		pipelineDeployment: pipelineDeployment,
		manager:            manager,
		scheme:             scheme,
	}
}

// KafkaUtil some helper methods for managing kafka configuration
type KafkaUtil struct {
	pipelineDeployment *algov1beta1.PipelineDeployment
	manager            manager.Manager
	scheme             *runtime.Scheme
}

// GetKafkaClusterName gets the KAFKA_CLUSTER_NAME
func GetKafkaClusterName() string {

	// Try to load from environment variable
	return os.Getenv("KAFKA_CLUSTER_NAME")

}

// GetKafkaNamespace gets the KAFKA_NAMESPACE
func GetKafkaNamespace() string {

	// Try to load from environment variable
	return os.Getenv("KAFKA_NAMESPACE")

}

// GetKafkaCaSecretName gets the name of the ca cert secret
func GetKafkaCaSecretName() string {

	return fmt.Sprintf("%s-cluster-ca-cert", GetKafkaClusterName())

}

// GetKafkaUserSecretName gets the name of the user cert secret
func GetKafkaUserSecretName(deploymentOwner string, deploymentName string) string {

	return fmt.Sprintf("kafka-%s-%s", deploymentOwner,
		deploymentName)

}

// CheckForKafkaTLS checks for the KAFKA_TLS envar and certs
func CheckForKafkaTLS() bool {

	// Try to load from environment variable
	kafkaTLSEnv := os.Getenv("KAFKA_TLS")
	if kafkaTLSEnv == "" {
		return false
	}

	kafkaTLS, err := strconv.ParseBool(kafkaTLSEnv)
	if err != nil {
		return false
	}

	return kafkaTLS

}

func (k *KafkaUtil) CopyKafkaSecrets() {

	kafkaUserSecretName := GetKafkaUserSecretName(k.pipelineDeployment.Spec.DeploymentOwner, k.pipelineDeployment.Spec.DeploymentName)
	kafkaCaSecretName := GetKafkaCaSecretName()

	// Copy the user cert secret
	k.copySecret(GetKafkaNamespace(), kafkaUserSecretName)
	// Copy the ca cert secret
	k.copySecret(GetKafkaNamespace(), kafkaCaSecretName)

}

func (k *KafkaUtil) copySecret(kafkaNamespace string, kafkaSecretName string) {

	apiReader := k.manager.GetAPIReader()
	writeClient := k.manager.GetClient()
	deploymentNamespace := k.pipelineDeployment.Spec.DeploymentNamespace

	// Get the secret from the kafka namespace
	kafkaExistingSecret := &corev1.Secret{}
	namespacedName := types.NamespacedName{
		Name:      kafkaSecretName,
		Namespace: kafkaNamespace,
	}

	err := apiReader.Get(context.TODO(), namespacedName, kafkaExistingSecret)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to get the Kafka Secret named %s in namespace %s",
			kafkaSecretName, kafkaNamespace))
	}

	// Get the secret from the deployment namespace
	kafkaDeplSecret := &corev1.Secret{}
	namespacedName = types.NamespacedName{
		Name:      kafkaSecretName,
		Namespace: deploymentNamespace,
	}

	errDepl := apiReader.Get(context.TODO(), namespacedName, kafkaDeplSecret)
	if errDepl != nil && errors.IsNotFound(errDepl) {

		kafkaNewSecret := &corev1.Secret{}
		kafkaNewSecret.SetName(kafkaSecretName)
		kafkaNewSecret.SetNamespace(deploymentNamespace)

		kafkaNewSecret.Data = kafkaExistingSecret.Data

		// Set PipelineDeployment instance as the owner and controller
		if err := controllerutil.SetControllerReference(k.pipelineDeployment, kafkaNewSecret, k.scheme); err != nil {
			log.Error(err, "Failed setting the deployment kafka TLS secret controller owner")
		}

		err = writeClient.Create(context.TODO(), kafkaNewSecret)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed Creating the Deployment Kafka Secret named %s in namespace %s",
				kafkaSecretName, deploymentNamespace))
		}

	} else if errDepl != nil {
		log.Error(err, "Failed to check if Kafka TLS Deployment Secret exists.")
	} else {

		if diff := deep.Equal(kafkaExistingSecret.Data, kafkaDeplSecret.Data); diff != nil {
			log.Info("Kafka TLS Cert Secret changed. Updating...")
			// Update the secret data
			kafkaDeplSecret.Data = kafkaExistingSecret.Data

			err := writeClient.Update(context.TODO(), kafkaDeplSecret)
			if err != nil {
				log.Error(err, "Failed updating the Kafka TLS Cert Secret")
			}
		}

	}

}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
