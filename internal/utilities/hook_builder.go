package utilities

import (
	"encoding/json"
	"fmt"
	"strings"

	"endpoint-operator/pkg/apis/algo/v1alpha1"
	algov1alpha1 "endpoint-operator/pkg/apis/algo/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type HookBuilder struct {
	Endpoint *algov1alpha1.Endpoint
}

// CreateDeploymentSpec generates the k8s spec for the algo deployment
func (hookBuilder *HookBuilder) CreateDeploymentSpec(name string, labels map[string]string, existingDeployment *appsv1.Deployment) (*appsv1.Deployment, error) {

	endpoint := hookBuilder.Endpoint
	hookConfig := hookBuilder.createHookRunnerConfig(&endpoint.Spec)

	// Set the image name
	var imageName string
	if hookConfig.ImageTag == "" || hookConfig.ImageTag == "latest" {
		imageName = fmt.Sprintf("%s:latest", hookConfig.ImageRepository)
	} else {
		imageName = fmt.Sprintf("%s:%s", hookConfig.ImageRepository, hookConfig.ImageTag)
	}

	var imagePullPolicy corev1.PullPolicy
	switch endpoint.Spec.ImagePullPolicy {
	case "Never":
		imagePullPolicy = corev1.PullNever
	case "PullAlways":
		imagePullPolicy = corev1.PullAlways
	case "IfNotPresent":
		imagePullPolicy = corev1.PullIfNotPresent
	default:
		imagePullPolicy = corev1.PullIfNotPresent
	}

	// Configure the readiness and liveness
	handler := corev1.Handler{
		HTTPGet: &corev1.HTTPGetAction{
			Scheme: "HTTP",
			Path:   "/health",
			Port:   intstr.FromInt(10080),
		},
	}

	var containers []corev1.Container

	hookCommand := []string{"/pipeline-hook/pipeline-hook"}
	hookEnvVars := hookBuilder.createEnvVars(endpoint, hookConfig)

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

	// Algo container
	hookContainer := corev1.Container{
		Name:    name,
		Image:   imageName,
		Command: hookCommand,
		Env:     hookEnvVars,
		// Resources:                *resources,
		ImagePullPolicy:          imagePullPolicy,
		LivenessProbe:            livenessProbe,
		ReadinessProbe:           readinessProbe,
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: "File",
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "algorun-data-volume",
				MountPath: "/data",
			},
		},
	}
	containers = append(containers, hookContainer)

	// nodeSelector := createSelector(request.Constraints)

	// If this is an update, need to set the existing deployment name
	var nameMeta metav1.ObjectMeta
	if existingDeployment != nil {
		nameMeta = metav1.ObjectMeta{
			Namespace: endpoint.Namespace,
			Name:      existingDeployment.Name,
			Labels:    labels,
			// Annotations: annotations,
		}
	} else {
		nameMeta = metav1.ObjectMeta{
			Namespace:    endpoint.Namespace,
			GenerateName: name,
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
			Replicas: &hookConfig.Instances,
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
			RevisionHistoryLimit: Int32p(10),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: nameMeta,
				Spec: corev1.PodSpec{
					// SecurityContext: &corev1.PodSecurityContext{
					//	FSGroup: int64p(1431),
					// },
					// NodeSelector: nodeSelector,
					Containers: containers,
					Volumes: []corev1.Volume{
						{
							Name: "algorun-data-volume",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "algorun-data-pv-claim",
									ReadOnly:  false,
								},
							},
						},
					},
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

func (hookBuilder *HookBuilder) createEnvVars(cr *algov1alpha1.Endpoint, hookConfig *v1alpha1.HookRunnerConfig) []corev1.EnvVar {

	envVars := []corev1.EnvVar{}

	// serialize the runner config to json string
	hookConfigBytes, err := json.Marshal(hookConfig)
	if err != nil {
		log.Error(err, "Failed deserializing runner config")
	}

	// Append the algo instance name
	envVars = append(envVars, corev1.EnvVar{
		Name: "INSTANCE-NAME",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				APIVersion: "v1",
				FieldPath:  "metadata.name",
			},
		},
	})

	// Append the required runner config
	envVars = append(envVars, corev1.EnvVar{
		Name:  "HOOK-CONFIG",
		Value: string(hookConfigBytes),
	})

	// Append the required kafka servers
	envVars = append(envVars, corev1.EnvVar{
		Name:  "KAFKA-BROKERS",
		Value: cr.Spec.KafkaBrokers,
	})

	// for k, v := range algoConfig.EnvVars {
	// 	envVars = append(envVars, corev1.EnvVar{
	// 		Name:  k,
	// 		Value: v,
	// 	})
	// }

	return envVars
}

func (hookBuilder *HookBuilder) createSelector(constraints []string) map[string]string {
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

// CreateRunnerConfig creates the config struct to be sent to the runner
func (hookBuilder *HookBuilder) createHookRunnerConfig(endpointSpec *algov1alpha1.EndpointSpec) *v1alpha1.HookRunnerConfig {

	hookConfig := &v1alpha1.HookRunnerConfig{
		EndpointOwnerUserName: endpointSpec.EndpointConfig.EndpointOwnerUserName,
		EndpointName:          endpointSpec.EndpointConfig.EndpointName,
		PipelineOwnerUserName: endpointSpec.EndpointConfig.PipelineOwnerUserName,
		PipelineName:          endpointSpec.EndpointConfig.PipelineName,
		ImageRepository:       endpointSpec.EndpointConfig.HookConfig.ImageRepository,
		ImageTag:              endpointSpec.EndpointConfig.HookConfig.ImageTag,
		Instances:             endpointSpec.EndpointConfig.HookConfig.Instances,
		WebHooks:              endpointSpec.EndpointConfig.HookConfig.WebHooks,
		Pipes:                 endpointSpec.EndpointConfig.Pipes,
		TopicConfigs:          endpointSpec.EndpointConfig.TopicConfigs,
	}

	return hookConfig

}
