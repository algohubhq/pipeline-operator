package reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"pipeline-operator/pkg/apis/algorun/v1beta1"
	algov1beta1 "pipeline-operator/pkg/apis/algorun/v1beta1"
	kafkav1beta1 "pipeline-operator/pkg/apis/kafka/v1beta1"
	utils "pipeline-operator/pkg/utilities"

	"github.com/go-test/deep"
	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewEventHookReconciler returns a new NewEventHookReconciler
func NewEventHookReconciler(pipelineDeployment *algov1beta1.PipelineDeployment,
	allTopicConfigs map[string]*v1beta1.TopicConfigModel,
	kafkaUtil *utils.KafkaUtil,
	request *reconcile.Request,
	manager manager.Manager,
	scheme *runtime.Scheme) EventHookReconciler {

	hookConfig := &eventHookConfig{
		DeploymentOwner: pipelineDeployment.Spec.DeploymentOwner,
		DeploymentName:  pipelineDeployment.Spec.DeploymentName,
		PipelineOwner:   pipelineDeployment.Spec.PipelineOwner,
		PipelineName:    pipelineDeployment.Spec.PipelineName,
		Pipes:           pipelineDeployment.Spec.Pipes,
		Topics:          allTopicConfigs,
		EventHookSpec:   pipelineDeployment.Spec.EventHook,
	}

	return EventHookReconciler{
		pipelineDeployment: pipelineDeployment,
		hookConfig:         hookConfig,
		allTopics:          allTopicConfigs,
		kafkaUtil:          kafkaUtil,
		request:            request,
		manager:            manager,
		scheme:             scheme,
	}
}

// HookReconciler reconciles an Hook object
type EventHookReconciler struct {
	pipelineDeployment *algov1beta1.PipelineDeployment
	hookConfig         *eventHookConfig
	allTopics          map[string]*v1beta1.TopicConfigModel
	kafkaUtil          *utils.KafkaUtil
	request            *reconcile.Request
	manager            manager.Manager
	scheme             *runtime.Scheme
}

// hookConfig holds the config sent to the hook container
type eventHookConfig struct {
	DeploymentOwner string
	DeploymentName  string
	PipelineOwner   string
	PipelineName    string
	Pipes           []v1beta1.PipeModel
	Topics          map[string]*v1beta1.TopicConfigModel
	// embed the hook Spec
	*v1beta1.EventHookSpec
}

// Reconcile creates or updates the hook deployment for the pipelineDeployment
func (hookReconciler *EventHookReconciler) Reconcile() error {

	pipelineDeployment := hookReconciler.pipelineDeployment

	hookLogger := log

	hookLogger.Info("Reconciling Hook")

	name := fmt.Sprintf("hook-%s-%s", pipelineDeployment.Spec.DeploymentOwner,
		pipelineDeployment.Spec.DeploymentName)

	labels := map[string]string{
		"app.kubernetes.io/part-of":    "algo.run",
		"app.kubernetes.io/component":  "hook",
		"app.kubernetes.io/managed-by": "pipeline-operator",
		"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", pipelineDeployment.Spec.DeploymentOwner,
			pipelineDeployment.Spec.DeploymentName),
		"algo.run/pipeline": fmt.Sprintf("%s.%s", pipelineDeployment.Spec.PipelineOwner,
			pipelineDeployment.Spec.PipelineName),
	}

	// Check to make sure the hook isn't already created
	opts := []client.ListOption{
		client.InNamespace(hookReconciler.pipelineDeployment.Spec.DeploymentNamespace),
		client.MatchingLabels{
			"app.kubernetes.io/part-of":   "algo.run",
			"app.kubernetes.io/component": "hook",
			"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", hookReconciler.pipelineDeployment.Spec.DeploymentOwner,
				hookReconciler.pipelineDeployment.Spec.DeploymentName),
		},
	}

	// Create the configmap for the endpoint
	configMapName, err := hookReconciler.createConfigMap(labels)

	kubeUtil := utils.NewKubeUtil(hookReconciler.manager, hookReconciler.request)

	var hookName string
	existingDeployment, err := kubeUtil.CheckForDeployment(opts)
	if existingDeployment != nil {
		hookName = existingDeployment.GetName()
	}

	// Generate the k8s deployment
	hookDeployment, err := hookReconciler.createDeploymentSpec(name, labels, configMapName, existingDeployment)
	if err != nil {
		hookLogger.Error(err, "Failed to create hook deployment spec")
		return err
	}

	// Set PipelineDeployment instance as the owner and controller
	if err := controllerutil.SetControllerReference(pipelineDeployment, hookDeployment, hookReconciler.scheme); err != nil {
		return err
	}

	if existingDeployment == nil {
		hookName, err = kubeUtil.CreateDeployment(hookDeployment)
		if err != nil {
			hookLogger.Error(err, "Failed to create hook deployment")
			return err
		}
	} else {
		var deplChanged bool

		// Set some values that are defaulted by k8s but shouldn't trigger a change
		hookDeployment.Spec.Template.Spec.TerminationGracePeriodSeconds = existingDeployment.Spec.Template.Spec.TerminationGracePeriodSeconds
		hookDeployment.Spec.Template.Spec.SecurityContext = existingDeployment.Spec.Template.Spec.SecurityContext
		hookDeployment.Spec.Template.Spec.SchedulerName = existingDeployment.Spec.Template.Spec.SchedulerName

		if *existingDeployment.Spec.Replicas != *hookDeployment.Spec.Replicas {
			hookLogger.Info("Hook Replica Count Changed. Updating deployment.",
				"Old Replicas", existingDeployment.Spec.Replicas,
				"New Replicas", hookDeployment.Spec.Replicas)
			deplChanged = true
		} else if diff := deep.Equal(existingDeployment.Spec.Template.Spec, hookDeployment.Spec.Template.Spec); diff != nil {
			hookLogger.Info("Hook Changed. Updating deployment.", "Differences", diff)
			deplChanged = true

		}
		if deplChanged {
			hookName, err = kubeUtil.UpdateDeployment(hookDeployment)
			if err != nil {
				hookLogger.Error(err, "Failed to update hook deployment")
				return err
			}
		}
	}

	// Setup the horizontal pod autoscaler
	if pipelineDeployment.Spec.Endpoint.Autoscaling != nil &&
		pipelineDeployment.Spec.Endpoint.Autoscaling.Enabled {

		labels := map[string]string{
			"app.kubernetes.io/part-of":    "algo.run",
			"app.kubernetes.io/component":  "hook-hpa",
			"app.kubernetes.io/managed-by": "pipeline-operator",
			"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", pipelineDeployment.Spec.DeploymentOwner,
				pipelineDeployment.Spec.DeploymentName),
			"algo.run/pipeline": fmt.Sprintf("%s.%s", pipelineDeployment.Spec.PipelineOwner,
				pipelineDeployment.Spec.PipelineName),
		}

		opts := []client.ListOption{
			client.InNamespace(hookReconciler.pipelineDeployment.Spec.DeploymentNamespace),
			client.MatchingLabels(labels),
		}

		existingHpa, err := kubeUtil.CheckForHorizontalPodAutoscaler(opts)

		hpaSpec, err := kubeUtil.CreateHpaSpec(hookName, labels, pipelineDeployment, pipelineDeployment.Spec.EventHook.Autoscaling)
		if err != nil {
			hookLogger.Error(err, "Failed to create Hook horizontal pod autoscaler spec")
			return err
		}

		// Set PipelineDeployment instance as the owner and controller
		if err := controllerutil.SetControllerReference(pipelineDeployment, hpaSpec, hookReconciler.scheme); err != nil {
			return err
		}

		if existingHpa == nil {
			_, err = kubeUtil.CreateHorizontalPodAutoscaler(hpaSpec)
			if err != nil {
				hookLogger.Error(err, "Failed to create Hook horizontal pod autoscaler")
				return err
			}
		} else {
			var deplChanged bool

			if existingHpa.Spec.Metrics != nil && hpaSpec.Spec.Metrics != nil {
				if diff := deep.Equal(existingHpa.Spec, hpaSpec.Spec); diff != nil {
					hookLogger.Info("Hook Horizontal Pod Autoscaler Changed. Updating...", "Differences", diff)
					deplChanged = true
				}
			}
			if deplChanged {
				_, err := kubeUtil.UpdateHorizontalPodAutoscaler(hpaSpec)
				if err != nil {
					hookLogger.Error(err, "Failed to update hook horizontal pod autoscaler")
					return err
				}
			}
		}

	}

	return nil

}

// CreateDeploymentSpec generates the k8s spec for the algo deployment
func (hookReconciler *EventHookReconciler) createDeploymentSpec(name string, labels map[string]string, configMapName string, existingDeployment *appsv1.Deployment) (*appsv1.Deployment, error) {

	pipelineDeployment := hookReconciler.pipelineDeployment
	hookConfig := hookReconciler.hookConfig

	// Set the image name
	imagePullPolicy := corev1.PullIfNotPresent
	imageName := os.Getenv("HOOK_IMAGE")
	if imageName == "" {
		if hookConfig.Image == nil {
			imageName = "algohub/hook-runner:latest"
		} else {
			if hookConfig.Image.Tag == "" {
				imageName = fmt.Sprintf("%s:latest", hookConfig.Image.Repository)
			} else {
				imageName = fmt.Sprintf("%s:%s", hookConfig.Image.Repository, hookConfig.Image.Tag)
			}
			switch *hookConfig.Image.ImagePullPolicy {
			case "Never":
				imagePullPolicy = corev1.PullNever
			case "PullAlways":
				imagePullPolicy = corev1.PullAlways
			case "IfNotPresent":
				imagePullPolicy = corev1.PullIfNotPresent
			default:
				imagePullPolicy = corev1.PullIfNotPresent
			}
		}
	}

	// Configure the readiness and liveness
	handler := corev1.Handler{
		HTTPGet: &corev1.HTTPGetAction{
			Scheme: "HTTP",
			Path:   "/health",
			Port:   intstr.FromInt(10080),
		},
	}

	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}

	// Create kafka tls volumes and mounts if tls enabled
	kafkaUtil := hookReconciler.kafkaUtil
	if hookReconciler.kafkaUtil.TLS != nil {

		kafkaTLSVolumes := []corev1.Volume{
			{
				Name: "kafka-ca-certs",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  kafkaUtil.TLS.TrustedCertificates[0].SecretName,
						DefaultMode: utils.Int32p(0444),
					},
				},
			},
		}
		volumes = append(volumes, kafkaTLSVolumes...)

		kafkaTLSMounts := []corev1.VolumeMount{
			{
				Name:      "kafka-ca-certs",
				SubPath:   kafkaUtil.TLS.TrustedCertificates[0].Certificate,
				MountPath: "/etc/ssl/certs/kafka-ca.crt",
				ReadOnly:  true,
			},
		}
		volumeMounts = append(volumeMounts, kafkaTLSMounts...)
	}

	if kafkaUtil.Authentication != nil {

		kafkaAuthVolumes := []corev1.Volume{
			{
				Name: "kafka-certs",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  kafkaUtil.Authentication.CertificateAndKey.SecretName,
						DefaultMode: utils.Int32p(0444),
					},
				},
			},
		}
		volumes = append(volumes, kafkaAuthVolumes...)

		kafkaAuthMounts := []corev1.VolumeMount{
			{
				Name:      "kafka-certs",
				SubPath:   kafkaUtil.Authentication.CertificateAndKey.Certificate,
				MountPath: "/etc/ssl/certs/kafka-user.crt",
				ReadOnly:  true,
			},
			{
				Name:      "kafka-certs",
				SubPath:   kafkaUtil.Authentication.CertificateAndKey.Key,
				MountPath: "/etc/ssl/certs/kafka-user.key",
				ReadOnly:  true,
			},
		}
		volumeMounts = append(volumeMounts, kafkaAuthMounts...)
	}

	configMapVolume := corev1.Volume{
		Name: "hook-config-volume",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: v1.LocalObjectReference{Name: configMapName},
				DefaultMode:          utils.Int32p(0444),
			},
		},
	}
	volumes = append(volumes, configMapVolume)

	// Add config mount
	configVolumeMount := corev1.VolumeMount{
		Name:      "hook-config-volume",
		SubPath:   "hook-config",
		MountPath: "/hook-config/hook-config.json",
	}
	volumeMounts = append(volumeMounts, configVolumeMount)

	var containers []corev1.Container

	hookCommand := []string{"/hook-runner/hook-runner"}
	hookEnvVars := hookReconciler.createEnvVars(pipelineDeployment, hookConfig)

	readinessProbe := &corev1.Probe{
		Handler:             handler,
		InitialDelaySeconds: 10,
		TimeoutSeconds:      10,
		PeriodSeconds:       20,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}

	livenessProbe := &corev1.Probe{
		Handler:             handler,
		InitialDelaySeconds: 10,
		TimeoutSeconds:      10,
		PeriodSeconds:       20,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}

	kubeUtil := utils.NewKubeUtil(hookReconciler.manager, hookReconciler.request)
	resources, resourceErr := kubeUtil.CreateResourceReqs(hookConfig.Resources)

	if resourceErr != nil {
		return nil, resourceErr
	}

	configArgs := []string{"--config=/hook-config/hook-config.json"}

	// Hook container
	hookContainer := corev1.Container{
		Name:                     name,
		Image:                    imageName,
		Command:                  hookCommand,
		Args:                     configArgs,
		Env:                      hookEnvVars,
		Resources:                *resources,
		ImagePullPolicy:          imagePullPolicy,
		LivenessProbe:            livenessProbe,
		ReadinessProbe:           readinessProbe,
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: "File",
	}
	containers = append(containers, hookContainer)

	// nodeSelector := createSelector(request.Constraints)

	// If this is an update, need to set the existing deployment name
	var nameMeta metav1.ObjectMeta
	if existingDeployment != nil {
		nameMeta = metav1.ObjectMeta{
			Namespace: pipelineDeployment.Spec.DeploymentNamespace,
			Name:      existingDeployment.Name,
			Labels:    labels,
			// Annotations: annotations,
		}
	} else {
		nameMeta = metav1.ObjectMeta{
			Namespace:    pipelineDeployment.Spec.DeploymentNamespace,
			GenerateName: fmt.Sprintf("%s-", name),
			Labels:       labels,
			// Annotations: annotations,
		}
	}

	// annotations := buildAnnotations(request)
	deploymentSpec := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: nameMeta,
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas: &hookConfig.Replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: int32(0),
					},
					MaxSurge: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: int32(1),
					},
				},
			},
			RevisionHistoryLimit: utils.Int32p(10),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: nameMeta,
				Spec: corev1.PodSpec{
					// SecurityContext: &corev1.PodSecurityContext{
					//	FSGroup: int64p(1431),
					// },
					// NodeSelector: nodeSelector,
					Containers:    containers,
					RestartPolicy: corev1.RestartPolicyAlways,
					DNSPolicy:     corev1.DNSClusterFirst,
				},
			},
		},
	}

	// if err := UpdateSecrets(request, deploymentSpec, existingSecrets); err != nil {
	// 	return nil, err
	// }

	return deploymentSpec, nil

}

func (hookReconciler *EventHookReconciler) createConfigMap(labels map[string]string) (configMapName string, err error) {

	kubeUtil := utils.NewKubeUtil(hookReconciler.manager, hookReconciler.request)
	// Create all config mounts
	name := fmt.Sprintf("%s-%s-%s-config",
		hookReconciler.pipelineDeployment.Spec.DeploymentOwner,
		hookReconciler.pipelineDeployment.Spec.DeploymentName,
		"hook")
	data := make(map[string]string)

	// serialize the hook config to json string
	hookConfigBytes, err := json.Marshal(hookReconciler.hookConfig)
	if err != nil {
		log.Error(err, "Failed deserializing hook config")
	}
	data["hook-config"] = string(hookConfigBytes)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: hookReconciler.pipelineDeployment.Spec.DeploymentNamespace,
			Name:      name,
			Labels:    labels,
			// Annotations: annotations,
		},
		Data: data,
	}

	// Set PipelineDeployment instance as the owner and controller
	if err := controllerutil.SetControllerReference(hookReconciler.pipelineDeployment, configMap, hookReconciler.scheme); err != nil {
		return name, err
	}

	existingConfigMap := &corev1.ConfigMap{}
	err = hookReconciler.manager.GetClient().Get(context.TODO(), types.NamespacedName{Name: name,
		Namespace: hookReconciler.pipelineDeployment.Spec.DeploymentNamespace},
		existingConfigMap)

	if err != nil && errors.IsNotFound(err) {
		// Create the ConfigMap
		name, err = kubeUtil.CreateConfigMap(configMap)
		if err != nil {
			log.Error(err, "Failed creating hook ConfigMap")
		}

	} else if err != nil {
		log.Error(err, "Failed to check if hook ConfigMap exists.")
	} else {

		if !cmp.Equal(existingConfigMap.Data, configMap.Data) {
			// Update configmap
			name, err = kubeUtil.UpdateConfigMap(configMap)
			if err != nil {
				log.Error(err, "Failed to update hook configmap")
				return name, err
			}
		}

	}

	return name, err

}

func (hookReconciler *EventHookReconciler) createEnvVars(cr *algov1beta1.PipelineDeployment, hookConfig *eventHookConfig) []corev1.EnvVar {

	envVars := []corev1.EnvVar{}

	// Append the algo instance name
	envVars = append(envVars, corev1.EnvVar{
		Name: "INSTANCE_NAME",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				APIVersion: "v1",
				FieldPath:  "metadata.name",
			},
		},
	})

	// Append the required kafka servers
	envVars = append(envVars, corev1.EnvVar{
		Name:  "KAFKA_BROKERS",
		Value: cr.Spec.KafkaBrokers,
	})

	// Append kafka tls indicator
	if hookReconciler.kafkaUtil.TLS != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "KAFKA_TLS",
			Value: strconv.FormatBool(hookReconciler.kafkaUtil.CheckForKafkaTLS()),
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "KAFKA_TLS_CA_LOCATION",
			Value: "/etc/ssl/certs/kafka-ca.crt",
		})
	}

	// Append kafka auth variables
	if hookReconciler.kafkaUtil.Authentication != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "KAFKA_AUTH_TYPE",
			Value: string(hookReconciler.kafkaUtil.Authentication.Type),
		})
		if hookReconciler.kafkaUtil.Authentication.Type == kafkav1beta1.KAFKA_AUTH_TYPE_TLS {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "KAFKA_AUTH_TLS_USER_LOCATION",
				Value: "/etc/ssl/certs/kafka-user.crt",
			})
			envVars = append(envVars, corev1.EnvVar{
				Name:  "KAFKA_AUTH_TLS_KEY_LOCATION",
				Value: "/etc/ssl/certs/kafka-user.key",
			})
		}
		if hookReconciler.kafkaUtil.Authentication.Type == kafkav1beta1.KAFKA_AUTH_TYPE_SCRAMSHA512 ||
			hookReconciler.kafkaUtil.Authentication.Type == kafkav1beta1.KAFKA_AUTH_TYPE_PLAIN {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "KAFKA_AUTH_USERNAME",
				Value: hookReconciler.kafkaUtil.Authentication.Username,
			})
			envVars = append(envVars, corev1.EnvVar{
				Name: "KAFKA_AUTH_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: hookReconciler.kafkaUtil.Authentication.PasswordSecret.SecretName,
						},
						Key: hookReconciler.kafkaUtil.Authentication.PasswordSecret.Password,
					},
				},
			})
		}
	}

	return envVars
}

func (hookReconciler *EventHookReconciler) createSelector(constraints []string) map[string]string {
	selector := make(map[string]string)

	if len(constraints) > 0 {
		for _, constraint := range constraints {
			parts := strings.Split(constraint, "=")

			if len(parts) == 2 {
				selector[parts[0]] = parts[1]
			}
		}
	}

	return selector
}
