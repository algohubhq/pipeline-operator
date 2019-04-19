package utilities

import (
	"encoding/json"
	"fmt"
	"strings"

	"endpoint-operator/pkg/apis/algo/v1alpha1"
	algov1alpha1 "endpoint-operator/pkg/apis/algo/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("utilities")

// CreateDeploymentSpec generates the k8s spec for the algo deployment
func CreateDeploymentSpec(cr *algov1alpha1.Endpoint, name string, labels map[string]string, algoConfig *v1alpha1.AlgoConfig, runnerConfig *v1alpha1.RunnerConfig, update bool) (*appsv1.Deployment, error) {

	// Set the image name
	var imageName string
	if algoConfig.ImageTag == "" || algoConfig.ImageTag == "latest" {
		imageName = fmt.Sprintf("%s:latest", algoConfig.ImageRepository)
	} else {
		imageName = fmt.Sprintf("%s:%s", algoConfig.ImageRepository, algoConfig.ImageTag)
	}

	// Set the algo-runner-sidecar name
	var sidecarImageName string
	if algoConfig.AlgoRunnerImage == "" {
		sidecarImageName = "algohub/algo-runner:latest"
	} else {
		if algoConfig.AlgoRunnerImageTag == "" || algoConfig.AlgoRunnerImageTag == "latest" {
			sidecarImageName = fmt.Sprintf("%s:latest", algoConfig.ImageRepository)
		} else {
			sidecarImageName = fmt.Sprintf("%s:%s", algoConfig.AlgoRunnerImage, algoConfig.AlgoRunnerImageTag)
		}
	}

	var imagePullPolicy corev1.PullPolicy
	switch cr.Spec.ImagePullPolicy {
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
	// Set resonable probe defaults if blank
	if algoConfig.ReadinessInitialDelaySeconds == 0 {
		algoConfig.ReadinessInitialDelaySeconds = 10
	}
	if algoConfig.ReadinessTimeoutSeconds == 0 {
		algoConfig.ReadinessTimeoutSeconds = 10
	}
	if algoConfig.ReadinessPeriodSeconds == 0 {
		algoConfig.ReadinessPeriodSeconds = 20
	}
	if algoConfig.LivenessInitialDelaySeconds == 0 {
		algoConfig.LivenessInitialDelaySeconds = 10
	}
	if algoConfig.LivenessTimeoutSeconds == 0 {
		algoConfig.LivenessTimeoutSeconds = 10
	}
	if algoConfig.LivenessPeriodSeconds == 0 {
		algoConfig.LivenessPeriodSeconds = 20
	}

	// If serverless, then we will copy the algo-runner binary into the algo container using an init container
	// If not serverless, then execute algo-runner within the sidecar
	var initContainers []corev1.Container
	var containers []corev1.Container
	var algoCommand []string
	var algoArgs []string
	var algoEnvVars []corev1.EnvVar
	var sidecarEnvVars []corev1.EnvVar
	var algoReadinessProbe *corev1.Probe
	var algoLivenessProbe *corev1.Probe
	var sidecarReadinessProbe *corev1.Probe
	var sidecarLivenessProbe *corev1.Probe
	if algoConfig.ServerType == "Serverless" {

		algoCommand = []string{"/algo-runner/algo-runner"}

		initCommand := []string{"/bin/sh", "-c"}
		initArgs := []string{
			"cp /algo-runner/algo-runner /algo-runner-dest/algo-runner && " +
				"chmod +x /algo-runner-dest/algo-runner",
		}

		algoEnvVars = createEnvVars(cr, runnerConfig, algoConfig)

		initContainer := corev1.Container{
			Name:                     "algo-runner-init",
			Image:                    sidecarImageName,
			Command:                  initCommand,
			Args:                     initArgs,
			ImagePullPolicy:          imagePullPolicy,
			TerminationMessagePath:   "/dev/termination-log",
			TerminationMessagePolicy: "File",
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "algo-runner-volume",
					MountPath: "/algo-runner-dest",
				},
			},
		}
		initContainers = append(initContainers, initContainer)

		algoReadinessProbe = &corev1.Probe{
			Handler:             handler,
			InitialDelaySeconds: algoConfig.ReadinessInitialDelaySeconds,
			TimeoutSeconds:      algoConfig.ReadinessTimeoutSeconds,
			PeriodSeconds:       algoConfig.ReadinessPeriodSeconds,
			SuccessThreshold:    1,
			FailureThreshold:    3,
		}

		algoLivenessProbe = &corev1.Probe{
			Handler:             handler,
			InitialDelaySeconds: algoConfig.LivenessInitialDelaySeconds,
			TimeoutSeconds:      algoConfig.LivenessTimeoutSeconds,
			PeriodSeconds:       algoConfig.LivenessPeriodSeconds,
			SuccessThreshold:    1,
			FailureThreshold:    3,
		}

	} else {

		entrypoint := strings.Split(runnerConfig.Entrypoint, " ")

		algoCommand = []string{entrypoint[0]}
		algoArgs = entrypoint[1:]

		sidecarCommand := []string{"/algo-runner/algo-runner"}

		sidecarEnvVars = createEnvVars(cr, runnerConfig, algoConfig)

		sidecarReadinessProbe = &corev1.Probe{
			Handler:             handler,
			InitialDelaySeconds: algoConfig.ReadinessInitialDelaySeconds,
			TimeoutSeconds:      algoConfig.ReadinessTimeoutSeconds,
			PeriodSeconds:       algoConfig.ReadinessPeriodSeconds,
			SuccessThreshold:    1,
			FailureThreshold:    3,
		}

		sidecarLivenessProbe = &corev1.Probe{
			Handler:             handler,
			InitialDelaySeconds: algoConfig.LivenessInitialDelaySeconds,
			TimeoutSeconds:      algoConfig.LivenessTimeoutSeconds,
			PeriodSeconds:       algoConfig.LivenessPeriodSeconds,
			SuccessThreshold:    1,
			FailureThreshold:    3,
		}

		sidecarContainer := corev1.Container{
			Name:                     "algo-runner-sidecar",
			Image:                    sidecarImageName,
			Command:                  sidecarCommand,
			Env:                      sidecarEnvVars,
			LivenessProbe:            sidecarLivenessProbe,
			ReadinessProbe:           sidecarReadinessProbe,
			ImagePullPolicy:          imagePullPolicy,
			TerminationMessagePath:   "/dev/termination-log",
			TerminationMessagePolicy: "File",
		}

		containers = append(containers, sidecarContainer)

	}

	resources, resourceErr := createResources(algoConfig)

	if resourceErr != nil {
		return nil, resourceErr
	}

	// Algo container
	algoContainer := corev1.Container{
		Name:                     name,
		Image:                    imageName,
		Command:                  algoCommand,
		Args:                     algoArgs,
		Env:                      algoEnvVars,
		Resources:                *resources,
		ImagePullPolicy:          imagePullPolicy,
		LivenessProbe:            algoLivenessProbe,
		ReadinessProbe:           algoReadinessProbe,
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: "File",
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "algo-runner-volume",
				MountPath: "/algo-runner",
			},
			{
				Name:      "algorun-data-volume",
				MountPath: "/data",
			},
		},
	}
	containers = append(containers, algoContainer)

	// nodeSelector := createSelector(request.Constraints)

	// If this is an update, need to set the existing deployment name
	var nameMeta metav1.ObjectMeta
	if update {
		nameMeta = metav1.ObjectMeta{
			Namespace: cr.Namespace,
			Name:      algoConfig.DeploymentName,
			Labels:    labels,
			// Annotations: annotations,
		}
	} else {
		nameMeta = metav1.ObjectMeta{
			Namespace:    cr.Namespace,
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
			Replicas: &algoConfig.Instances,
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
					InitContainers: initContainers,
					Containers:     containers,
					Volumes: []corev1.Volume{
						{
							Name: "algo-runner-volume",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
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

func createEnvVars(cr *algov1alpha1.Endpoint, runnerConfig *v1alpha1.RunnerConfig, algoConfig *v1alpha1.AlgoConfig) []corev1.EnvVar {

	envVars := []corev1.EnvVar{}

	// serialize the runner config to json string
	runnerConfigBytes, err := json.Marshal(runnerConfig)
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
		Name:  "ALGO-RUNNER-CONFIG",
		Value: string(runnerConfigBytes),
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

func createSelector(constraints []string) map[string]string {
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

func createResources(algoConfig *v1alpha1.AlgoConfig) (*corev1.ResourceRequirements, error) {
	resources := &corev1.ResourceRequirements{}

	// Set Memory limits
	if algoConfig.MemoryLimitBytes > 0 {
		qty, err := resource.ParseQuantity(string(algoConfig.MemoryLimitBytes))
		if err != nil {
			return resources, err
		}
		resources.Limits[corev1.ResourceMemory] = qty
	}

	if algoConfig.MemoryRequestBytes > 0 {
		qty, err := resource.ParseQuantity(string(algoConfig.MemoryRequestBytes))
		if err != nil {
			return resources, err
		}
		resources.Requests[corev1.ResourceMemory] = qty
	}

	// Set CPU limits
	if algoConfig.CpuLimitUnits > 0 {
		qty, err := resource.ParseQuantity(fmt.Sprintf("%f", algoConfig.CpuLimitUnits))
		if err != nil {
			return resources, err
		}
		resources.Limits[corev1.ResourceCPU] = qty
	}

	if algoConfig.CpuRequestUnits > 0 {
		qty, err := resource.ParseQuantity(fmt.Sprintf("%f", algoConfig.CpuRequestUnits))
		if err != nil {
			return resources, err
		}
		resources.Requests[corev1.ResourceCPU] = qty
	}

	// Set GPU limits
	if algoConfig.GpuLimitUnits > 0 {
		qty, err := resource.ParseQuantity(fmt.Sprintf("%f", algoConfig.GpuLimitUnits))
		if err != nil {
			return resources, err
		}
		resources.Limits["nvidia.com/gpu"] = qty
	}

	return resources, nil
}

// CreateRunnerConfig creates the config struct to be sent to the runner
func CreateRunnerConfig(endpointSpec *algov1alpha1.EndpointSpec, algoConfig *v1alpha1.AlgoConfig) v1alpha1.RunnerConfig {

	runnerConfig := v1alpha1.RunnerConfig{
		EndpointOwnerUserName: endpointSpec.EndpointConfig.EndpointOwnerUserName,
		EndpointName:          endpointSpec.EndpointConfig.EndpointName,
		PipelineOwnerUserName: endpointSpec.EndpointConfig.PipelineOwnerUserName,
		PipelineName:          endpointSpec.EndpointConfig.PipelineName,
		Pipes:                 endpointSpec.EndpointConfig.Pipes,
		TopicConfigs:          endpointSpec.EndpointConfig.TopicConfigs,
		AlgoOwnerUserName:     algoConfig.AlgoOwnerUserName,
		AlgoName:              algoConfig.AlgoName,
		AlgoVersionTag:        algoConfig.AlgoVersionTag,
		AlgoIndex:             algoConfig.AlgoIndex,
		Entrypoint:            algoConfig.Entrypoint,
		ServerType:            algoConfig.ServerType,
		AlgoParams:            algoConfig.AlgoParams,
		Inputs:                algoConfig.Inputs,
		Outputs:               algoConfig.Outputs,
		WriteAllOutputs:       algoConfig.WriteAllOutputs,
		GpuEnabled:            algoConfig.GpuEnabled,
		TimeoutSeconds:        algoConfig.TimeoutSeconds,
	}

	return runnerConfig

}
