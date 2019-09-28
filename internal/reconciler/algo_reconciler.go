package reconciler

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	utils "pipeline-operator/internal/utilities"
	"pipeline-operator/pkg/apis/algo/v1alpha1"
	algov1alpha1 "pipeline-operator/pkg/apis/algo/v1alpha1"

	"github.com/go-test/deep"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

// AlgoReconciler reconciles an AlgoConfig object
type AlgoReconciler struct {
	pipelineDeployment *algov1alpha1.PipelineDeployment
	algoConfig         *v1alpha1.AlgoConfig
	request            *reconcile.Request
	client             client.Client
	scheme             *runtime.Scheme
}

var log = logf.Log.WithName("reconciler")

// NewAlgoReconciler returns a new AlgoReconciler
func NewAlgoReconciler(pipelineDeployment *algov1alpha1.PipelineDeployment,
	algoConfig *v1alpha1.AlgoConfig,
	request *reconcile.Request,
	client client.Client,
	scheme *runtime.Scheme) AlgoReconciler {
	return AlgoReconciler{
		pipelineDeployment: pipelineDeployment,
		algoConfig:         algoConfig,
		request:            request,
		client:             client,
		scheme:             scheme,
	}
}

// ReconcileService creates or updates all services for the algos
func (algoReconciler *AlgoReconciler) ReconcileService() error {

	deplUtil := utils.NewDeploymentUtil(algoReconciler.client)

	// Check to see if the metrics / health service is already created (All algos share the same service port)
	srvListOptions := &client.ListOptions{}
	srvListOptions.SetLabelSelector(fmt.Sprintf("system=algorun, component=algo"))
	srvListOptions.InNamespace(algoReconciler.request.NamespacedName.Namespace)

	existingService, err := deplUtil.CheckForService(srvListOptions)
	if err != nil {
		log.Error(err, "Failed to check for existing algo metric service")
		return err
	}
	if existingService == nil {

		// Generate the service for the all algos
		algoService, err := algoReconciler.createMetricServiceSpec(algoReconciler.pipelineDeployment)
		if err != nil {
			log.Error(err, "Failed to create algo metrics / health service spec")
			return err
		}

		err = deplUtil.CreateService(algoService)
		if err != nil {
			log.Error(err, "Failed to create algo metrics / health service")
			return err
		}
	}

	return nil

}

// Reconcile creates or updates all algos for the pipelineDeployment
func (algoReconciler *AlgoReconciler) Reconcile() error {

	algoConfig := algoReconciler.algoConfig
	pipelineDeployment := algoReconciler.pipelineDeployment
	request := algoReconciler.request

	logData := map[string]interface{}{
		"AlgoOwner":      algoConfig.AlgoOwnerUserName,
		"AlgoName":       algoConfig.AlgoName,
		"AlgoVersionTag": algoConfig.AlgoVersionTag,
		"Index":          algoConfig.AlgoIndex,
	}
	algoLogger := log.WithValues("data", logData)

	algoLogger.Info("Reconciling Algo")

	// Truncate the name of the deployment / pod just in case
	name := strings.TrimRight(utils.Short(algoConfig.AlgoName, 20), "-")

	labels := map[string]string{
		"system":                  "algorun",
		"tier":                    "backend",
		"component":               "algo",
		"pipelinedeploymentowner": pipelineDeployment.Spec.PipelineSpec.DeploymentOwnerUserName,
		"pipelinedeployment":      pipelineDeployment.Spec.PipelineSpec.DeploymentName,
		"pipeline":                pipelineDeployment.Spec.PipelineSpec.PipelineName,
		"algoowner":               algoConfig.AlgoOwnerUserName,
		"algo":                    algoConfig.AlgoName,
		"algoversion":             algoConfig.AlgoVersionTag,
		"algoindex":               strconv.Itoa(int(algoConfig.AlgoIndex)),
	}

	deplUtil := utils.NewDeploymentUtil(algoReconciler.client)

	// Check to make sure the algo isn't already created
	listOptions := &client.ListOptions{}
	listOptions.SetLabelSelector(fmt.Sprintf("system=algorun, component=algo, pipelinedeploymentowner=%s, pipelinedeployment=%s, algoowner=%s, algo=%s, algoversion=%s, algoindex=%v",
		pipelineDeployment.Spec.PipelineSpec.DeploymentOwnerUserName,
		pipelineDeployment.Spec.PipelineSpec.DeploymentName,
		algoConfig.AlgoOwnerUserName,
		algoConfig.AlgoName,
		algoConfig.AlgoVersionTag,
		algoConfig.AlgoIndex))
	listOptions.InNamespace(request.NamespacedName.Namespace)

	existingDeployment, err := deplUtil.CheckForDeployment(listOptions)

	if existingDeployment != nil {
		algoConfig.DeploymentName = existingDeployment.GetName()
	}

	// Generate the k8s deployment for the algoconfig
	algoDeployment, err := algoReconciler.createDeploymentSpec(name, labels, existingDeployment != nil)
	if err != nil {
		algoLogger.Error(err, "Failed to create algo deployment spec")
		return err
	}

	// Set PipelineDeployment instance as the owner and controller
	if err := controllerutil.SetControllerReference(pipelineDeployment, algoDeployment, algoReconciler.scheme); err != nil {
		return err
	}

	if existingDeployment == nil {
		err := deplUtil.CreateDeployment(algoDeployment)
		if err != nil {
			algoLogger.Error(err, "Failed to create algo deployment")
			return err
		}
	} else {
		var deplChanged bool

		// Set some values that are defaulted by k8s but shouldn't trigger a change
		algoDeployment.Spec.Template.Spec.TerminationGracePeriodSeconds = existingDeployment.Spec.Template.Spec.TerminationGracePeriodSeconds
		algoDeployment.Spec.Template.Spec.SecurityContext = existingDeployment.Spec.Template.Spec.SecurityContext
		algoDeployment.Spec.Template.Spec.SchedulerName = existingDeployment.Spec.Template.Spec.SchedulerName

		if *existingDeployment.Spec.Replicas != *algoDeployment.Spec.Replicas {
			algoLogger.Info("Algo Replica Count Changed. Updating deployment.",
				"Old Replicas", existingDeployment.Spec.Replicas,
				"New Replicas", algoDeployment.Spec.Replicas)
			deplChanged = true
		} else if diff := deep.Equal(existingDeployment.Spec.Template.Spec, algoDeployment.Spec.Template.Spec); diff != nil {
			algoLogger.Info("Algo Changed. Updating deployment.", "Differences", diff)
			deplChanged = true

		}
		if deplChanged {
			err := deplUtil.UpdateDeployment(algoDeployment)
			if err != nil {
				algoLogger.Error(err, "Failed to update algo deployment")
				return err
			}
		}
	}

	// TODO: Setup the horizontal pod autoscaler
	if algoConfig.AutoScale {

	}

	return nil

}

func (algoReconciler *AlgoReconciler) createMetricServiceSpec(pipelineDeployment *algov1alpha1.PipelineDeployment) (*corev1.Service, error) {

	labels := map[string]string{
		"system":    "algorun",
		"component": "algo",
	}

	algoServiceSpec := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: pipelineDeployment.Namespace,
			Name:      "algo-metrics-service",
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				corev1.ServicePort{
					Name: "metrics",
					Port: 10080,
				},
			},
			Selector: map[string]string{
				"system":    "algorun",
				"component": "algo",
			},
		},
	}

	return algoServiceSpec, nil

}

// CreateDeploymentSpec generates the k8s spec for the algo deployment
func (algoReconciler *AlgoReconciler) createDeploymentSpec(name string, labels map[string]string, update bool) (*appsv1.Deployment, error) {

	pipelineDeployment := algoReconciler.pipelineDeployment
	algoConfig := algoReconciler.algoConfig
	runnerConfig := algoReconciler.createRunnerConfig(&pipelineDeployment.Spec, algoConfig)

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
		algoRunnerImage := os.Getenv("ALGORUNNER_IMAGE")
		if algoRunnerImage == "" {
			sidecarImageName = "algohub/algo-runner:latest"
		} else {
			sidecarImageName = algoRunnerImage
		}
	} else {
		if algoConfig.AlgoRunnerImageTag == "" {
			sidecarImageName = fmt.Sprintf("%s:latest", algoConfig.AlgoRunnerImage)
		} else {
			sidecarImageName = fmt.Sprintf("%s:%s", algoConfig.AlgoRunnerImage, algoConfig.AlgoRunnerImageTag)
		}
	}

	var imagePullPolicy corev1.PullPolicy
	switch pipelineDeployment.Spec.ImagePullPolicy {
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
				"cp /algo-runner/mc /algo-runner-dest/mc && " +
				"chmod +x /algo-runner-dest/algo-runner && " +
				"chmod +x /algo-runner-dest/mc",
		}

		algoEnvVars = algoReconciler.createEnvVars(pipelineDeployment, runnerConfig, algoConfig)

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

	} else if algoConfig.ServerType == "Delegated" {

		// If delegated there is no sidecar or init container
		// the entrypoint is ran "as is" and the kafka config is passed to the container
		entrypoint := strings.Split(runnerConfig.Entrypoint, " ")

		algoCommand = []string{entrypoint[0]}
		algoArgs = entrypoint[1:]

		algoEnvVars = algoReconciler.createEnvVars(pipelineDeployment, runnerConfig, algoConfig)

		// TODO: Add user defined liveness/readiness probes to algo

	} else {

		entrypoint := strings.Split(runnerConfig.Entrypoint, " ")

		algoCommand = []string{entrypoint[0]}
		algoArgs = entrypoint[1:]

		sidecarCommand := []string{"/algo-runner/algo-runner"}

		sidecarEnvVars = algoReconciler.createEnvVars(pipelineDeployment, runnerConfig, algoConfig)

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

	resources, resourceErr := algoReconciler.createResources(algoConfig)

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
				MountPath: "/output",
			},
		},
	}
	containers = append(containers, algoContainer)

	// nodeSelector := createSelector(request.Constraints)

	// If this is an update, need to set the existing deployment name
	var nameMeta metav1.ObjectMeta
	if update {
		nameMeta = metav1.ObjectMeta{
			Namespace: pipelineDeployment.Namespace,
			Name:      algoConfig.DeploymentName,
			Labels:    labels,
			// Annotations: annotations,
		}
	} else {
		nameMeta = metav1.ObjectMeta{
			Namespace:    pipelineDeployment.Namespace,
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
			RevisionHistoryLimit: utils.Int32p(10),
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
								EmptyDir: &corev1.EmptyDirVolumeSource{},
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

func (algoReconciler *AlgoReconciler) createEnvVars(cr *algov1alpha1.PipelineDeployment, runnerConfig *v1alpha1.AlgoRunnerConfig, algoConfig *v1alpha1.AlgoConfig) []corev1.EnvVar {

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

	// Append the storage server connection
	envVars = append(envVars, corev1.EnvVar{
		Name: "MC_HOST_algorun",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "storage-endpoint"},
				Key:                  "mc",
			},
		},
	})

	// Append the path to mc
	envVars = append(envVars, corev1.EnvVar{
		Name:  "MC_PATH",
		Value: "/algo-runner/mc",
	})

	// for k, v := range algoConfig.EnvVars {
	// 	envVars = append(envVars, corev1.EnvVar{
	// 		Name:  k,
	// 		Value: v,
	// 	})
	// }

	return envVars
}

func (algoReconciler *AlgoReconciler) createSelector(constraints []string) map[string]string {
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

func (algoReconciler *AlgoReconciler) createResources(algoConfig *v1alpha1.AlgoConfig) (*corev1.ResourceRequirements, error) {
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
func (algoReconciler *AlgoReconciler) createRunnerConfig(pipelineDeploymentSpec *algov1alpha1.PipelineDeploymentSpec, algoConfig *v1alpha1.AlgoConfig) *v1alpha1.AlgoRunnerConfig {

	runnerConfig := &v1alpha1.AlgoRunnerConfig{
		DeploymentOwnerUserName: pipelineDeploymentSpec.PipelineSpec.DeploymentOwnerUserName,
		DeploymentName:          pipelineDeploymentSpec.PipelineSpec.DeploymentName,
		PipelineOwnerUserName:   pipelineDeploymentSpec.PipelineSpec.PipelineOwnerUserName,
		PipelineName:            pipelineDeploymentSpec.PipelineSpec.PipelineName,
		Pipes:                   pipelineDeploymentSpec.PipelineSpec.Pipes,
		TopicConfigs:            pipelineDeploymentSpec.PipelineSpec.TopicConfigs,
		AlgoOwnerUserName:       algoConfig.AlgoOwnerUserName,
		AlgoName:                algoConfig.AlgoName,
		AlgoVersionTag:          algoConfig.AlgoVersionTag,
		AlgoIndex:               algoConfig.AlgoIndex,
		Entrypoint:              algoConfig.Entrypoint,
		ServerType:              algoConfig.ServerType,
		AlgoParams:              algoConfig.AlgoParams,
		Inputs:                  algoConfig.Inputs,
		Outputs:                 algoConfig.Outputs,
		WriteAllOutputs:         algoConfig.WriteAllOutputs,
		GpuEnabled:              algoConfig.GpuEnabled,
		TimeoutSeconds:          algoConfig.TimeoutSeconds,
	}

	return runnerConfig

}
