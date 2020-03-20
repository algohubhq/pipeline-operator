package reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"pipeline-operator/pkg/apis/algorun/v1beta1"
	algov1beta1 "pipeline-operator/pkg/apis/algorun/v1beta1"
	utils "pipeline-operator/pkg/utilities"

	"github.com/go-test/deep"
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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

// AlgoReconciler reconciles an AlgoConfig object
type AlgoReconciler struct {
	pipelineDeployment *algov1beta1.PipelineDeployment
	algoConfig         *v1beta1.AlgoConfig
	allTopicConfigs    []algov1beta1.TopicConfigModel
	request            *reconcile.Request
	client             client.Client
	scheme             *runtime.Scheme
	kafkaTLS           bool
}

var log = logf.Log.WithName("reconciler")

// NewAlgoReconciler returns a new AlgoReconciler
func NewAlgoReconciler(pipelineDeployment *algov1beta1.PipelineDeployment,
	algoConfig *v1beta1.AlgoConfig,
	allTopicConfigs []algov1beta1.TopicConfigModel,
	request *reconcile.Request,
	client client.Client,
	scheme *runtime.Scheme,
	kafkaTLS bool) AlgoReconciler {
	return AlgoReconciler{
		pipelineDeployment: pipelineDeployment,
		algoConfig:         algoConfig,
		allTopicConfigs:    allTopicConfigs,
		request:            request,
		client:             client,
		scheme:             scheme,
		kafkaTLS:           kafkaTLS,
	}
}

// ReconcileService creates or updates all services for the algos
func (algoReconciler *AlgoReconciler) ReconcileService() error {

	kubeUtil := utils.NewKubeUtil(algoReconciler.client, algoReconciler.request)

	// Check to see if the metrics / health service is already created (All algos share the same service port)
	opts := []client.ListOption{
		client.InNamespace(algoReconciler.request.NamespacedName.Namespace),
		client.MatchingLabels{
			"app.kubernetes.io/part-of":   "algo.run",
			"app.kubernetes.io/component": "algo",
		},
	}

	existingService, err := kubeUtil.CheckForService(opts)
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

		_, err = kubeUtil.CreateService(algoService)
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
		"app.kubernetes.io/part-of":    "algo.run",
		"app.kubernetes.io/component":  "algo",
		"app.kubernetes.io/managed-by": "pipeline-operator",
		"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", pipelineDeployment.Spec.DeploymentOwnerUserName,
			pipelineDeployment.Spec.DeploymentName),
		"algo.run/pipeline": fmt.Sprintf("%s.%s", pipelineDeployment.Spec.PipelineOwnerUserName,
			pipelineDeployment.Spec.PipelineName),
		"algo.run/algo": fmt.Sprintf("%s.%s", algoConfig.AlgoOwnerUserName,
			algoConfig.AlgoName),
		"algo.run/algo-version": algoConfig.AlgoVersionTag,
		"algo.run/index":        strconv.Itoa(int(algoConfig.AlgoIndex)),
	}

	kubeUtil := utils.NewKubeUtil(algoReconciler.client, algoReconciler.request)

	// Check to make sure the algo isn't already created
	opts := []client.ListOption{
		client.InNamespace(request.NamespacedName.Namespace),
		client.MatchingLabels{
			"app.kubernetes.io/part-of":   "algo.run",
			"app.kubernetes.io/component": "algo",
			"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s",
				pipelineDeployment.Spec.DeploymentOwnerUserName,
				pipelineDeployment.Spec.DeploymentName),
			"algo.run/algo": fmt.Sprintf("%s.%s",
				algoConfig.AlgoOwnerUserName,
				algoConfig.AlgoName),
			"algo.run/algo-version": algoConfig.AlgoVersionTag,
			"algo.run/index":        fmt.Sprintf("%v", algoConfig.AlgoIndex),
		},
	}

	// Create the runner config
	runnerConfig := algoReconciler.createRunnerConfig(&pipelineDeployment.Spec, algoConfig)

	// Create the configmap for the algo
	configMapName, err := algoReconciler.createConfigMap(algoConfig, runnerConfig, labels)

	existingDeployment, err := kubeUtil.CheckForDeployment(opts)

	var algoName string
	if existingDeployment != nil {
		algoName = existingDeployment.GetName()
		algoConfig.DeploymentName = existingDeployment.GetName()
	}

	// Generate the k8s deployment for the algoconfig
	algoDeployment, err := algoReconciler.createDeploymentSpec(name, labels, runnerConfig, configMapName, existingDeployment != nil)
	if err != nil {
		algoLogger.Error(err, "Failed to create algo deployment spec")
		return err
	}

	// Set PipelineDeployment instance as the owner and controller
	if err := controllerutil.SetControllerReference(pipelineDeployment, algoDeployment, algoReconciler.scheme); err != nil {
		return err
	}

	if existingDeployment == nil {
		algoName, err = kubeUtil.CreateDeployment(algoDeployment)
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
			algoName, err = kubeUtil.UpdateDeployment(algoDeployment)
			if err != nil {
				algoLogger.Error(err, "Failed to update algo deployment")
				return err
			}
		}
	}

	// Setup the horizontal pod autoscaler
	if algoConfig.Resource.AutoScale {

		labels := map[string]string{
			"app.kubernetes.io/part-of":    "algo.run",
			"app.kubernetes.io/component":  "algo-hpa",
			"app.kubernetes.io/managed-by": "pipeline-operator",
			"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", pipelineDeployment.Spec.DeploymentOwnerUserName,
				pipelineDeployment.Spec.DeploymentName),
			"algo.run/pipeline": fmt.Sprintf("%s.%s", pipelineDeployment.Spec.PipelineOwnerUserName,
				pipelineDeployment.Spec.PipelineName),
			"algo.run/algo": fmt.Sprintf("%s.%s", algoReconciler.algoConfig.AlgoOwnerUserName,
				algoReconciler.algoConfig.AlgoName),
			"algo.run/algo-version": algoReconciler.algoConfig.AlgoVersionTag,
			"algo.run/index":        strconv.Itoa(int(algoReconciler.algoConfig.AlgoIndex)),
		}

		opts := []client.ListOption{
			client.InNamespace(request.NamespacedName.Namespace),
			client.MatchingLabels(labels),
		}

		existingHpa, err := kubeUtil.CheckForHorizontalPodAutoscaler(opts)

		hpaSpec, err := kubeUtil.CreateHpaSpec(algoName, labels, pipelineDeployment, &algoConfig.Resource)
		if err != nil {
			algoLogger.Error(err, "Failed to create Algo horizontal pod autoscaler spec")
			return err
		}

		// Set PipelineDeployment instance as the owner and controller
		if err := controllerutil.SetControllerReference(pipelineDeployment, hpaSpec, algoReconciler.scheme); err != nil {
			return err
		}

		if existingHpa == nil {
			_, err = kubeUtil.CreateHorizontalPodAutoscaler(hpaSpec)
			if err != nil {
				algoLogger.Error(err, "Failed to create Algo horizontal pod autoscaler")
				return err
			}
		} else {
			var deplChanged bool

			if existingHpa.Spec.Metrics != nil && hpaSpec.Spec.Metrics != nil {
				if diff := deep.Equal(existingHpa.Spec, hpaSpec.Spec); diff != nil {
					algoLogger.Info("Algo Horizontal Pod Autoscaler Changed. Updating...", "Differences", diff)
					deplChanged = true
				}
			}
			if deplChanged {
				_, err := kubeUtil.UpdateHorizontalPodAutoscaler(hpaSpec)
				if err != nil {
					algoLogger.Error(err, "Failed to update horizontal pod autoscaler")
					return err
				}
			}
		}

	}

	return nil

}

func (algoReconciler *AlgoReconciler) createMetricServiceSpec(pipelineDeployment *algov1beta1.PipelineDeployment) (*corev1.Service, error) {

	labels := map[string]string{
		"app.kubernetes.io/part-of":    "algo.run",
		"app.kubernetes.io/component":  "algo",
		"app.kubernetes.io/managed-by": "pipeline-operator",
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
				"app.kubernetes.io/part-of":   "algo.run",
				"app.kubernetes.io/component": "algo",
			},
		},
	}

	return algoServiceSpec, nil

}

// CreateDeploymentSpec generates the k8s spec for the algo deployment
func (algoReconciler *AlgoReconciler) createDeploymentSpec(name string, labels map[string]string, runnerConfig *v1beta1.AlgoRunnerConfig, configMapName string, update bool) (*appsv1.Deployment, error) {

	pipelineDeployment := algoReconciler.pipelineDeployment
	algoConfig := algoReconciler.algoConfig

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

	// Create kafka tls volumes and mounts if tls enabled

	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}
	if algoReconciler.kafkaTLS {

		kafkaUsername := fmt.Sprintf("kafka-%s-%s", pipelineDeployment.Spec.DeploymentOwnerUserName,
			pipelineDeployment.Spec.DeploymentName)
		kafkaCaSecretName := fmt.Sprintf("%s-cluster-ca-cert", utils.GetKafkaClusterName())

		kafkaTLSVolumes := []corev1.Volume{
			{
				Name: "kafka-certs",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  kafkaUsername,
						DefaultMode: utils.Int32p(0444),
					},
				},
			},
			{
				Name: "kafka-ca-certs",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  kafkaCaSecretName,
						DefaultMode: utils.Int32p(0444),
					},
				},
			},
		}
		volumes = append(volumes, kafkaTLSVolumes...)

		kafkaTLSMounts := []corev1.VolumeMount{
			{
				Name:      "kafka-ca-certs",
				SubPath:   "ca.crt",
				MountPath: "/etc/ssl/certs/kafka-ca.crt",
				ReadOnly:  true,
			},
			{
				Name:      "kafka-certs",
				SubPath:   "user.crt",
				MountPath: "/etc/ssl/certs/kafka-user.crt",
				ReadOnly:  true,
			},
			{
				Name:      "kafka-certs",
				SubPath:   "user.key",
				MountPath: "/etc/ssl/certs/kafka-user.key",
				ReadOnly:  true,
			},
		}
		volumeMounts = append(volumeMounts, kafkaTLSMounts...)
	}

	configMapVolume := corev1.Volume{
		Name: "algo-config-volume",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: v1.LocalObjectReference{Name: configMapName},
				DefaultMode:          utils.Int32p(0444),
			},
		},
	}
	volumes = append(volumes, configMapVolume)

	// Add runner config
	runnerConfigVolumeMount := corev1.VolumeMount{
		Name:      "algo-config-volume",
		SubPath:   "runner-config",
		MountPath: "/algo-runner/algo-runner-config.json",
	}
	volumeMounts = append(volumeMounts, runnerConfigVolumeMount)

	// Add all config mounts
	for _, configMount := range algoConfig.ConfigMounts {
		cmVolumeMount := corev1.VolumeMount{
			Name:      "algo-config-volume",
			SubPath:   configMount.Name,
			MountPath: configMount.MountPath,
		}
		volumeMounts = append(volumeMounts, cmVolumeMount)
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
	if algoConfig.Executor == "Executable" {

		algoCommand = []string{"/algo-runner/algo-runner"}
		algoArgs = []string{"--config=/algo-runner/algo-runner-config.json"}

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

	} else if algoConfig.Executor == "Delegated" {

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
		sidecarArgs := []string{"--config=/algo-runner/algo-runner-config.json"}

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
			Args:                     sidecarArgs,
			Env:                      sidecarEnvVars,
			LivenessProbe:            sidecarLivenessProbe,
			ReadinessProbe:           sidecarReadinessProbe,
			ImagePullPolicy:          imagePullPolicy,
			TerminationMessagePath:   "/dev/termination-log",
			TerminationMessagePolicy: "File",
			VolumeMounts:             volumeMounts,
		}

		containers = append(containers, sidecarContainer)

	}

	kubeUtil := utils.NewKubeUtil(algoReconciler.client, algoReconciler.request)
	resources, resourceErr := kubeUtil.CreateResourceReqs(&algoConfig.Resource)

	if resourceErr != nil {
		return nil, resourceErr
	}

	// Create the volumes
	algoVolumes := []corev1.Volume{
		{
			Name: "algo-runner-volume",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "algorun-input-volume",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "algorun-output-volume",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	volumes = append(volumes, algoVolumes...)

	// Create the volume mounts
	algoVolumeMounts := []corev1.VolumeMount{
		{
			Name:      "algo-runner-volume",
			MountPath: "/algo-runner",
		},
		{
			Name:      "algorun-input-volume",
			MountPath: "/input",
		},
		{
			Name:      "algorun-output-volume",
			MountPath: "/output",
		},
	}

	volumeMounts = append(volumeMounts, algoVolumeMounts...)

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
		VolumeMounts:             volumeMounts,
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
			Replicas: &algoConfig.Resource.Instances,
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
					Volumes:        volumes,
					RestartPolicy:  corev1.RestartPolicyAlways,
					DNSPolicy:      corev1.DNSClusterFirst,
				},
			},
		},
	}

	// if err := UpdateSecrets(request, deploymentSpec, existingSecrets); err != nil {
	// 	return nil, err
	// }

	return deploymentSpec, nil

}

func (algoReconciler *AlgoReconciler) createConfigMap(algoConfig *v1beta1.AlgoConfig, runnerConfig *v1beta1.AlgoRunnerConfig, labels map[string]string) (configMapName string, err error) {

	kubeUtil := utils.NewKubeUtil(algoReconciler.client, algoReconciler.request)
	// Create all config mounts
	name := fmt.Sprintf("%s-%s-%s-%s-config",
		algoReconciler.pipelineDeployment.Spec.DeploymentOwnerUserName,
		algoReconciler.pipelineDeployment.Spec.DeploymentName,
		algoConfig.AlgoOwnerUserName,
		algoConfig.AlgoName)
	data := make(map[string]string)

	// Add the runner-config
	// serialize the runner config to json string
	runnerConfigBytes, err := json.Marshal(runnerConfig)
	if err != nil {
		log.Error(err, "Failed deserializing runner config")
	}
	data["runner-config"] = string(runnerConfigBytes)

	// Add all config mounts
	for _, configMount := range algoConfig.ConfigMounts {
		data[configMount.Name] = configMount.Data
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: algoReconciler.pipelineDeployment.Namespace,
			Name:      name,
			Labels:    labels,
			// Annotations: annotations,
		},
		Data: data,
	}

	// Set PipelineDeployment instance as the owner and controller
	if err := controllerutil.SetControllerReference(algoReconciler.pipelineDeployment, configMap, algoReconciler.scheme); err != nil {
		return name, err
	}

	existingConfigMap := &corev1.ConfigMap{}
	err = algoReconciler.client.Get(context.TODO(), types.NamespacedName{Name: name,
		Namespace: algoReconciler.request.NamespacedName.Namespace},
		existingConfigMap)

	if err != nil && errors.IsNotFound(err) {
		// Create the ConfigMap
		name, err = kubeUtil.CreateConfigMap(configMap)
		if err != nil {
			log.Error(err, "Failed creating algo ConfigMap")
		}

	} else if err != nil {
		log.Error(err, "Failed to check if algo ConfigMap exists.")
	} else {

		if !reflect.DeepEqual(existingConfigMap.Data, configMap.Data) {
			// Update configmap
			name, err = kubeUtil.UpdateConfigMap(configMap)
			if err != nil {
				log.Error(err, "Failed to update algo configmap")
				return name, err
			}
		}

	}

	return name, err

}

func (algoReconciler *AlgoReconciler) createEnvVars(cr *algov1beta1.PipelineDeployment, runnerConfig *v1beta1.AlgoRunnerConfig, algoConfig *v1beta1.AlgoConfig) []corev1.EnvVar {

	envVars := []corev1.EnvVar{}

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

	// Append the required kafka servers
	envVars = append(envVars, corev1.EnvVar{
		Name:  "KAFKA-BROKERS",
		Value: cr.Spec.KafkaBrokers,
	})

	// Append kafka tls indicator
	envVars = append(envVars, corev1.EnvVar{
		Name:  "KAFKA-TLS",
		Value: strconv.FormatBool(algoReconciler.kafkaTLS),
	})

	// Append the storage server connection
	kubeUtil := utils.NewKubeUtil(algoReconciler.client, algoReconciler.request)
	storageSecretName, err := kubeUtil.GetStorageSecretName(&algoReconciler.pipelineDeployment.Spec)
	if storageSecretName != "" && err == nil {
		envVars = append(envVars, corev1.EnvVar{
			Name: "MC_HOST_algorun",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: storageSecretName},
					Key:                  "connection-string",
				},
			},
		})
	}

	// Append the path to mc
	envVars = append(envVars, corev1.EnvVar{
		Name:  "MC_PATH",
		Value: "/algo-runner/mc",
	})

	// Append all KafkaTopic Inputs
	for _, input := range algoConfig.Inputs {
		if input.InputDeliveryType == v1beta1.INPUTDELIVERYTYPES_KAFKA_TOPIC {
			for _, pipe := range algoReconciler.pipelineDeployment.Spec.Pipes {
				if pipe.DestInputName == input.Name {
					for _, tc := range algoReconciler.allTopicConfigs {
						if pipe.SourceName == tc.SourceName &&
							pipe.SourceOutputName == tc.SourceOutputName {
							topicName := utils.GetTopicName(tc.TopicName, &algoReconciler.pipelineDeployment.Spec)
							envVars = append(envVars, corev1.EnvVar{
								Name:  fmt.Sprintf("KAFKA_INPUT_TOPIC_%s", strings.ToUpper(input.Name)),
								Value: topicName,
							})
						}
					}
				}
			}
		}
	}

	// Append all KafkaTopic Outputs
	for _, output := range algoConfig.Outputs {
		if output.OutputDeliveryType == v1beta1.OUTPUTDELIVERYTYPES_KAFKA_TOPIC {
			for _, tc := range algoConfig.TopicConfigs {
				if output.Name == tc.SourceOutputName {
					topicName := utils.GetTopicName(tc.TopicName, &algoReconciler.pipelineDeployment.Spec)
					envVars = append(envVars, corev1.EnvVar{
						Name:  fmt.Sprintf("KAFKA_INPUT_TOPIC_%s", strings.ToUpper(output.Name)),
						Value: topicName,
					})
				}
			}
		}
	}

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

// CreateRunnerConfig creates the config struct to be sent to the runner
func (algoReconciler *AlgoReconciler) createRunnerConfig(pipelineDeploymentSpec *algov1beta1.PipelineDeploymentSpecV1beta1, algoConfig *v1beta1.AlgoConfig) *v1beta1.AlgoRunnerConfig {

	runnerConfig := &v1beta1.AlgoRunnerConfig{
		DeploymentOwnerUserName: pipelineDeploymentSpec.DeploymentOwnerUserName,
		DeploymentName:          pipelineDeploymentSpec.DeploymentName,
		PipelineOwnerUserName:   pipelineDeploymentSpec.PipelineOwnerUserName,
		PipelineName:            pipelineDeploymentSpec.PipelineName,
		Pipes:                   pipelineDeploymentSpec.Pipes,
		TopicConfigs:            algoReconciler.allTopicConfigs,
		AlgoOwnerUserName:       algoConfig.AlgoOwnerUserName,
		AlgoName:                algoConfig.AlgoName,
		AlgoVersionTag:          algoConfig.AlgoVersionTag,
		AlgoIndex:               algoConfig.AlgoIndex,
		Entrypoint:              algoConfig.Entrypoint,
		Executor:                algoConfig.Executor,
		AlgoParams:              algoConfig.AlgoParams,
		Inputs:                  algoConfig.Inputs,
		Outputs:                 algoConfig.Outputs,
		WriteAllOutputs:         algoConfig.WriteAllOutputs,
		GpuEnabled:              algoConfig.GpuEnabled,
		TimeoutSeconds:          algoConfig.TimeoutSeconds,
	}

	return runnerConfig

}
