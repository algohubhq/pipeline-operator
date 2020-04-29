package utilities

import (
	"context"
	"fmt"
	"os"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

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

func CopyKafkaSecrets(deploymentNamespace string,
	deploymentOwner string,
	deploymentName string,
	manager manager.Manager) {

	kafkaUserSecretName := GetKafkaUserSecretName(deploymentOwner, deploymentName)
	kafkaCaSecretName := GetKafkaCaSecretName()

	// Copy the user cert secret
	copySecret(GetKafkaNamespace(), kafkaUserSecretName, deploymentNamespace, manager)
	// Copy the ca cert secret
	copySecret(GetKafkaNamespace(), kafkaCaSecretName, deploymentNamespace, manager)

}

func copySecret(kafkaNamespace string, kafkaSecretName string, deploymentNamespace string, manager manager.Manager) {

	apiReader := manager.GetAPIReader()
	writeClient := manager.GetClient()

	kafkaDeplSecret := &corev1.Secret{}
	namespacedName := types.NamespacedName{
		Name:      kafkaSecretName,
		Namespace: deploymentNamespace,
	}

	err := apiReader.Get(context.TODO(), namespacedName, kafkaDeplSecret)
	if err != nil {
		if errors.IsNotFound(err) {

			// Get the secret from the kafka namespace
			kafkaExistingSecret := &corev1.Secret{}
			namespacedName := types.NamespacedName{
				Name:      kafkaSecretName,
				Namespace: kafkaNamespace,
			}

			err = apiReader.Get(context.TODO(), namespacedName, kafkaExistingSecret)
			if err != nil {
				log.Error(err, fmt.Sprintf("Failed to get the Kafka Secret named %s in namespace %s",
					kafkaSecretName, kafkaNamespace))
			}

			kafkaNewSecret := &corev1.Secret{}
			kafkaNewSecret.SetName(kafkaSecretName)
			kafkaNewSecret.SetNamespace(deploymentNamespace)

			kafkaNewSecret.Data = kafkaExistingSecret.Data

			err = writeClient.Create(context.TODO(), kafkaNewSecret)
			if err != nil {
				log.Error(err, fmt.Sprintf("Failed Creating the Deployment Kafka Secret named %s in namespace %s",
					kafkaSecretName, deploymentNamespace))
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
