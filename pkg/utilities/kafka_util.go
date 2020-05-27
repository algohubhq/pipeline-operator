package utilities

import (
	"context"
	"fmt"
	"os"
	algov1beta1 "pipeline-operator/pkg/apis/algorun/v1beta1"
	"strconv"
	"strings"

	kafkav1beta1 "pipeline-operator/pkg/apis/kafka/v1beta1"

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
	scheme *runtime.Scheme) *KafkaUtil {

	kafkaUtil := &KafkaUtil{
		pipelineDeployment: pipelineDeployment,
		manager:            manager,
		scheme:             scheme,
	}

	kafkaUtil.TLS = kafkaUtil.getKafkaTLS()
	kafkaUtil.Authentication = kafkaUtil.getKafkaAuth()

	return kafkaUtil
}

// KafkaUtil some helper methods for managing kafka configuration
type KafkaUtil struct {
	pipelineDeployment *algov1beta1.PipelineDeployment
	manager            manager.Manager
	scheme             *runtime.Scheme
	TLS                *kafkav1beta1.KafkaTLS
	Authentication     *kafkav1beta1.KafkaClientAuthentication
}

func (k *KafkaUtil) getKafkaTLS() (tls *kafkav1beta1.KafkaTLS) {

	if k.CheckForKafkaTLS() {
		tls = &kafkav1beta1.KafkaTLS{
			TrustedCertificates: []kafkav1beta1.SecretSource{
				{
					SecretName:  k.getKafkaCASecretName(),
					Certificate: os.Getenv("KAFKA_TLS_CA_KEY"),
				},
			},
		}

		// Copy the cert secret from the kafka namespace (if necessary)
		k.CopyKafkaCASecret()
	}

	return tls
}

func (k *KafkaUtil) getKafkaAuth() (auth *kafkav1beta1.KafkaClientAuthentication) {

	authType := os.Getenv("KAFKA_AUTH_TYPE")

	if authType != "" {
		auth = &kafkav1beta1.KafkaClientAuthentication{
			Type: kafkav1beta1.KafkaClientAuthenticationTypes(os.Getenv("KAFKA_AUTH_TYPE")),
		}

		if auth.Type == kafkav1beta1.KAFKA_AUTH_TYPE_TLS {
			auth.CertificateAndKey = &kafkav1beta1.SecretSource{
				SecretName: k.getKafkaUserSecretName(os.Getenv("KAFKA_AUTH_SECRETNAME"), k.pipelineDeployment.Spec.DeploymentOwner,
					k.pipelineDeployment.Spec.DeploymentName),
				Certificate: os.Getenv("KAFKA_AUTH_CERTIFICATE_KEY"),
				Key:         os.Getenv("KAFKA_AUTH_KEY_SECRET_KEY"),
			}

			// Copy the cert secret from the kafka namespace (if necessary)
			k.CopyKafkaUserSecret()
		}

		if auth.Type == kafkav1beta1.KAFKA_AUTH_TYPE_SCRAMSHA512 ||
			auth.Type == kafkav1beta1.KAFKA_AUTH_TYPE_PLAIN {
			auth.Username = os.Getenv("KAFKA_AUTH_USERNAME")
			auth.PasswordSecret = &kafkav1beta1.SecretSource{
				SecretName: k.getKafkaUserSecretName(os.Getenv("KAFKA_AUTH_SECRETNAME"), k.pipelineDeployment.Spec.DeploymentOwner,
					k.pipelineDeployment.Spec.DeploymentName),
				Password: os.Getenv("KAFKA_AUTH_PASSWORD_SECRET_KEY"),
			}
		}

	}

	return auth
}

// GetKafkaClusterName gets the KAFKA_CLUSTER_NAME
func (k *KafkaUtil) GetKafkaClusterName() string {

	// Try to load from environment variable
	return os.Getenv("KAFKA_CLUSTER_NAME")

}

// GetKafkaNamespace gets the KAFKA_NAMESPACE
func (k *KafkaUtil) GetKafkaNamespace() string {

	// Try to load from environment variable
	return os.Getenv("KAFKA_NAMESPACE")

}

// GetKafkaAuthType gets the KAFKA_AUTH_TYPE
func (k *KafkaUtil) GetKafkaAuthType() string {

	// Try to load from environment variable
	return os.Getenv("KAFKA_AUTH_TYPE")

}

// getKafkaCASecretName gets the name of the kafka ca cert secret
func (k *KafkaUtil) getKafkaCASecretName() string {

	// Try to load from environment variable
	return os.Getenv("KAFKA_TLS_CA_SECRETNAME")

}

// getKafkaUserSecretName gets the name of the user cert secret
func (k *KafkaUtil) getKafkaUserSecretName(secretName string, deploymentOwner string, deploymentName string) string {

	secretName = strings.Replace(secretName, "{deploymentowner}", deploymentOwner, -1)
	secretName = strings.Replace(secretName, "{deploymentname}", deploymentName, -1)
	secretName = strings.Replace(secretName, "{kafkaclustername}", k.GetKafkaClusterName(), -1)
	return strings.ToLower(secretName)

}

// CheckForKafkaTLS checks for the KAFKA_TLS envar and certs
func (k *KafkaUtil) CheckForKafkaTLS() bool {

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

// CheckForKafkaAuth checks for the KAFKA_TLS envar and certs
func (k *KafkaUtil) CheckForKafkaAuth() bool {

	// Try to load from environment variable
	kafkaAuthEnv := os.Getenv("KAFKA_AUTH_TYPE")
	if kafkaAuthEnv == "" {
		return false
	}

	return true

}

func (k *KafkaUtil) CopyKafkaCASecret() {

	kafkaCaSecretName := k.getKafkaCASecretName()
	// Copy the ca cert secret
	k.copySecret(k.GetKafkaNamespace(), kafkaCaSecretName)

}

func (k *KafkaUtil) CopyKafkaUserSecret() {

	kafkaUserSecretName := k.getKafkaUserSecretName(os.Getenv("KAFKA_AUTH_SECRETNAME"), k.pipelineDeployment.Spec.DeploymentOwner, k.pipelineDeployment.Spec.DeploymentName)

	// Copy the user cert secret
	k.copySecret(k.GetKafkaNamespace(), kafkaUserSecretName)

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
