package reconciler

import (
	"context"
	"encoding/json"
	e "errors"
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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientpkg "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

// AlgoReconciler reconciles an AlgoDeployment object
type AlgoReconciler struct {
	pipelineDeployment *algov1beta1.PipelineDeployment
	algoDeployment     *v1beta1.AlgoDeploymentV1beta1
	activeAlgoVersion  *v1beta1.AlgoVersionModel
	allTopicConfigs    map[string]*v1beta1.TopicConfigModel
	kafkaUtil          *utils.KafkaUtil
	request            *reconcile.Request
	manager            manager.Manager
	scheme             *runtime.Scheme
}

// algoRunnerConfig struct for AlgoRunnerConfig
type algoRunnerConfig struct {
	DeploymentOwner string                              `json:"deploymentOwner"`
	DeploymentName  string                              `json:"deploymentName"`
	PipelineOwner   string                              `json:"pipelineOwner"`
	PipelineName    string                              `json:"pipelineName"`
	Owner           string                              `json:"owner"`
	Name            string                              `json:"name"`
	Version         string                              `json:"version"`
	Index           int32                               `json:"index"`
	Entrypoint      string                              `json:"entrypoint,omitempty"`
	Executor        *v1beta1.Executors                  `json:"executor,omitempty"`
	Parameters      []v1beta1.AlgoParamSpec             `json:"parameters,omitempty"`
	Inputs          []v1beta1.AlgoInputSpec             `json:"inputs,omitempty"`
	Outputs         []v1beta1.AlgoOutputSpec            `json:"outputs,omitempty"`
	WriteAllOutputs bool                                `json:"writeAllOutputs,omitempty"`
	Pipes           []v1beta1.PipeModel                 `json:"pipes,omitempty"`
	Topics          map[string]v1beta1.TopicConfigModel `json:"topics,omitempty"`
	RetryEnabled    bool                                `json:"retryEnabled,omitempty"`
	RetryStrategy   *v1beta1.TopicRetryStrategyModel    `json:"retryStrategy,omitempty"`
	GpuEnabled      bool                                `json:"gpuEnabled,omitempty"`
	TimeoutSeconds  int32                               `json:"timeoutSeconds,omitempty"`
}

var log = logf.Log.WithName("reconciler")

// NewAlgoReconciler returns a new AlgoReconciler
func NewAlgoReconciler(pipelineDeployment *algov1beta1.PipelineDeployment,
	algoDeployment *v1beta1.AlgoDeploymentV1beta1,
	allTopicConfigs map[string]*v1beta1.TopicConfigModel,
	kafkaUtil *utils.KafkaUtil,
	request *reconcile.Request,
	manager manager.Manager,
	scheme *runtime.Scheme) (*AlgoReconciler, error) {

	// Ensure the algo has a matching version defined
	var activeAlgoVersion *v1beta1.AlgoVersionModel
	for _, version := range algoDeployment.Spec.Versions {
		if algoDeployment.Version == version.VersionTag {
			activeAlgoVersion = &version
		}
	}

	if activeAlgoVersion == nil {
		err := e.New(fmt.Sprintf("There is no matching Algo Version with requested version tag [%s]", algoDeployment.Version))
		return nil, err
	}

	return &AlgoReconciler{
		pipelineDeployment: pipelineDeployment,
		algoDeployment:     algoDeployment,
		activeAlgoVersion:  activeAlgoVersion,
		allTopicConfigs:    allTopicConfigs,
		kafkaUtil:          kafkaUtil,
		request:            request,
		manager:            manager,
		scheme:             scheme,
	}, nil
}

// NewAlgoReconciler returns a new AlgoReconciler
func NewAlgoServiceReconciler(pipelineDeployment *algov1beta1.PipelineDeployment,
	allTopicConfigs map[string]*v1beta1.TopicConfigModel,
	request *reconcile.Request,
	manager manager.Manager,
	scheme *runtime.Scheme) (*AlgoReconciler, error) {

	return &AlgoReconciler{
		pipelineDeployment: pipelineDeployment,
		allTopicConfigs:    allTopicConfigs,
		request:            request,
		manager:            manager,
		scheme:             scheme,
	}, nil
}

// Reconcile creates or updates all algos for the pipelineDeployment
func (algoReconciler *AlgoReconciler) Reconcile() error {

	algoDepl := algoReconciler.algoDeployment
	pipelineDeployment := algoReconciler.pipelineDeployment

	logData := map[string]interface{}{
		"AlgoOwner":      algoDepl.Spec.Owner,
		"AlgoName":       algoDepl.Spec.Name,
		"AlgoVersionTag": algoDepl.Version,
		"Index":          algoDepl.Index,
	}
	algoLogger := log.WithValues("data", logData)

	algoLogger.Info("Reconciling Algo")

	// Truncate the name of the deployment / pod just in case
	name := strings.TrimRight(utils.Short(algoDepl.Spec.Name, 20), "-")

	labels := map[string]string{
		"app.kubernetes.io/part-of":    "algo.run",
		"app.kubernetes.io/component":  "algo",
		"app.kubernetes.io/managed-by": "pipeline-operator",
		"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", pipelineDeployment.Spec.DeploymentOwner,
			pipelineDeployment.Spec.DeploymentName),
		"algo.run/pipeline": fmt.Sprintf("%s.%s", pipelineDeployment.Spec.PipelineOwner,
			pipelineDeployment.Spec.PipelineName),
		"algo.run/algo": fmt.Sprintf("%s.%s", algoDepl.Spec.Owner,
			algoDepl.Spec.Name),
		"algo.run/algo-version": algoDepl.Version,
		"algo.run/index":        strconv.Itoa(int(algoDepl.Index)),
	}

	kubeUtil := utils.NewKubeUtil(algoReconciler.manager, algoReconciler.request)

	// Check to make sure the algo isn't already created
	opts := []clientpkg.ListOption{
		clientpkg.InNamespace(pipelineDeployment.Spec.DeploymentNamespace),
		clientpkg.MatchingLabels{
			"app.kubernetes.io/part-of":   "algo.run",
			"app.kubernetes.io/component": "algo",
			"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s",
				pipelineDeployment.Spec.DeploymentOwner,
				pipelineDeployment.Spec.DeploymentName),
			"algo.run/algo": fmt.Sprintf("%s.%s",
				algoDepl.Spec.Owner,
				algoDepl.Spec.Name),
			"algo.run/algo-version": algoDepl.Version,
			"algo.run/index":        fmt.Sprintf("%v", algoDepl.Index),
		},
	}

	// Create the runner config
	runnerConfig := algoReconciler.createRunnerConfig(&pipelineDeployment.Spec, algoDepl)

	// Create the configmap for the algo
	configMapName, err := algoReconciler.createConfigMap(algoDepl, runnerConfig, labels)

	existingDeployment, err := kubeUtil.CheckForDeployment(opts)

	var algoName string
	var existingDeploymentName string
	if existingDeployment != nil {
		algoName = existingDeployment.GetName()
		existingDeploymentName = existingDeployment.GetName()
	}

	// Generate the k8s deployment for the algo deployment
	algoDeployment, err := algoReconciler.createDeploymentSpec(name, existingDeploymentName, labels, runnerConfig, configMapName, existingDeployment != nil)
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
	if algoDepl.Autoscaling != nil && algoDepl.Autoscaling.Enabled {

		labels := map[string]string{
			"app.kubernetes.io/part-of":    "algo.run",
			"app.kubernetes.io/component":  "algo-hpa",
			"app.kubernetes.io/managed-by": "pipeline-operator",
			"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", pipelineDeployment.Spec.DeploymentOwner,
				pipelineDeployment.Spec.DeploymentName),
			"algo.run/pipeline": fmt.Sprintf("%s.%s", pipelineDeployment.Spec.PipelineOwner,
				pipelineDeployment.Spec.PipelineName),
			"algo.run/algo": fmt.Sprintf("%s.%s", algoReconciler.algoDeployment.Spec.Owner,
				algoReconciler.algoDeployment.Spec.Name),
			"algo.run/algo-version": algoReconciler.algoDeployment.Version,
			"algo.run/index":        strconv.Itoa(int(algoReconciler.algoDeployment.Index)),
		}

		opts := []clientpkg.ListOption{
			clientpkg.InNamespace(pipelineDeployment.Spec.DeploymentNamespace),
			clientpkg.MatchingLabels(labels),
		}

		existingHpa, err := kubeUtil.CheckForHorizontalPodAutoscaler(opts)

		hpaSpec, err := kubeUtil.CreateHpaSpec(algoName, labels, pipelineDeployment, algoDepl.Autoscaling)
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

	// Creat the Kafka Topics
	algoLogger.Info("Reconciling Kakfa Topics for Algo outputs")
	compName := utils.GetAlgoFullName(algoDepl)
	for _, topic := range algoDepl.Topics {
		go func(currentTopicConfig algov1beta1.TopicConfigModel) {
			topicReconciler := NewTopicReconciler(algoReconciler.pipelineDeployment,
				compName,
				&currentTopicConfig,
				algoReconciler.kafkaUtil,
				algoReconciler.request,
				algoReconciler.manager,
				algoReconciler.scheme)
			topicReconciler.Reconcile()
		}(topic)
	}

	return nil

}

// ReconcileService creates or updates all services for the algos
func (algoReconciler *AlgoReconciler) ReconcileService() error {

	kubeUtil := utils.NewKubeUtil(algoReconciler.manager, algoReconciler.request)

	// Check to see if the metrics / health service is already created (All algos share the same service port)
	opts := []clientpkg.ListOption{
		clientpkg.InNamespace(algoReconciler.pipelineDeployment.Spec.DeploymentNamespace),
		clientpkg.MatchingLabels{
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

func (algoReconciler *AlgoReconciler) createMetricServiceSpec(pipelineDeployment *algov1beta1.PipelineDeployment) (*corev1.Service, error) {

	labels := map[string]string{
		"app.kubernetes.io/part-of":    "algo.run",
		"app.kubernetes.io/component":  "algo",
		"app.kubernetes.io/managed-by": "pipeline-operator",
	}

	algoServiceSpec := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: pipelineDeployment.Spec.DeploymentNamespace,
			Name:      "algo-metrics-service",
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: "metrics",
					Port: 10080,
				},
			},
			Selector: map[string]string{
				"app.kubernetes.io/part-of":    "algo.run",
				"app.kubernetes.io/component":  "algo",
				"algo.run/create-algo-service": "true",
			},
		},
	}

	return algoServiceSpec, nil

}

// CreateDeploymentSpec generates the k8s spec for the algo deployment
func (algoReconciler *AlgoReconciler) createDeploymentSpec(name string, existingDeploymentName string, labels map[string]string, runnerConfig *algoRunnerConfig, configMapName string, update bool) (*appsv1.Deployment, error) {

	pipelineDeployment := algoReconciler.pipelineDeployment
	algoDepl := algoReconciler.algoDeployment

	// Set the image name
	var imageName string
	if algoReconciler.activeAlgoVersion.Image != nil {
		imageName = fmt.Sprintf("%s:%s", algoReconciler.activeAlgoVersion.Image.Repository, algoReconciler.activeAlgoVersion.Image.Tag)
	} else {
		imageName = fmt.Sprintf("%s:latest", algoReconciler.activeAlgoVersion.Image.Repository)
	}

	// Set the algo-runner-sidecar name
	var sidecarImageName string
	imagePullPolicy := corev1.PullIfNotPresent
	if algoDepl.AlgoRunnerImage == nil {
		algoRunnerImage := os.Getenv("ALGORUNNER_IMAGE")
		if algoRunnerImage == "" {
			sidecarImageName = "algohub/algo-runner:latest"
		} else {
			sidecarImageName = algoRunnerImage
		}
	} else {
		if algoDepl.AlgoRunnerImage.Tag == "" {
			sidecarImageName = fmt.Sprintf("%s:latest", algoDepl.AlgoRunnerImage.Repository)
		} else {
			sidecarImageName = fmt.Sprintf("%s:%s", algoDepl.AlgoRunnerImage.Repository, algoDepl.AlgoRunnerImage.Tag)
		}
		switch *algoDepl.AlgoRunnerImage.ImagePullPolicy {
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

	// Configure the readiness and liveness
	handler := corev1.Handler{
		HTTPGet: &corev1.HTTPGetAction{
			Scheme: "HTTP",
			Path:   "/health",
			Port:   intstr.FromInt(10080),
		},
	}

	readinessProbe := &corev1.Probe{
		Handler:          handler,
		PeriodSeconds:    10,
		TimeoutSeconds:   1,
		SuccessThreshold: 1,
		FailureThreshold: 3,
	}
	livenessProbe := &corev1.Probe{
		Handler:          handler,
		PeriodSeconds:    10,
		TimeoutSeconds:   1,
		SuccessThreshold: 1,
		FailureThreshold: 3,
	}

	if algoDepl.ReadinessProbe != nil {
		readinessProbe = &corev1.Probe{
			InitialDelaySeconds: algoDepl.ReadinessProbe.InitialDelaySeconds,
			FailureThreshold:    algoDepl.ReadinessProbe.FailureThreshold,
			PeriodSeconds:       algoDepl.ReadinessProbe.PeriodSeconds,
			TimeoutSeconds:      algoDepl.ReadinessProbe.TimeoutSeconds,
			SuccessThreshold:    algoDepl.ReadinessProbe.SuccessThreshold,
		}
	}
	if algoDepl.LivenessProbe != nil {
		livenessProbe = &corev1.Probe{
			InitialDelaySeconds: algoDepl.LivenessProbe.InitialDelaySeconds,
			FailureThreshold:    algoDepl.LivenessProbe.FailureThreshold,
			PeriodSeconds:       algoDepl.LivenessProbe.PeriodSeconds,
			TimeoutSeconds:      algoDepl.LivenessProbe.TimeoutSeconds,
			SuccessThreshold:    algoDepl.LivenessProbe.SuccessThreshold,
		}
	}

	// Create kafka tls volumes and mounts if tls enabled
	kafkaUtil := algoReconciler.kafkaUtil
	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}
	if algoReconciler.kafkaUtil.TLS != nil {

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

	// If TLS authentication, mount the certs to the container
	if kafkaUtil.Authentication != nil &&
		kafkaUtil.Authentication.Type == kafkav1beta1.KAFKA_AUTH_TYPE_TLS {

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
		Name: "algo-config-volume",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
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
	for _, configMount := range algoDepl.ConfigMounts {
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

	if *algoDepl.Spec.Executor == v1beta1.EXECUTORS_EXECUTABLE {

		labels["algo.run/create-algo-service"] = "true"

		algoCommand = []string{"/bin/algo-runner"}
		algoArgs = []string{"--config=/algo-runner/algo-runner-config.json"}

		initCommand := []string{"/bin/sh", "-c"}
		initArgs := []string{
			"cp /bin/algo-runner /algo-runner-dest/algo-runner && " +
				"cp /bin/mc /algo-runner-dest/mc && " +
				"chmod +x /algo-runner-dest/algo-runner && " +
				"chmod +x /algo-runner-dest/mc",
		}

		algoEnvVars = algoReconciler.createEnvVars(pipelineDeployment, runnerConfig, algoDepl)

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

	} else if *algoDepl.Spec.Executor == v1beta1.EXECUTORS_DELEGATED {

		labels["algo.run/create-algo-service"] = "false"

		// If delegated there is no sidecar or init container
		// the entrypoint is ran "as is" and the kafka config is passed to the container
		entrypoint := strings.Split(runnerConfig.Entrypoint, " ")

		algoCommand = []string{entrypoint[0]}
		algoArgs = entrypoint[1:]

		algoEnvVars = algoReconciler.createEnvVars(pipelineDeployment, runnerConfig, algoDepl)

		// TODO: Add user defined liveness/readiness probes to algo

	} else {

		labels["algo.run/create-algo-service"] = "true"

		entrypoint := strings.Split(runnerConfig.Entrypoint, " ")

		algoCommand = []string{entrypoint[0]}
		algoArgs = entrypoint[1:]

		sidecarCommand := []string{"/bin/algo-runner"}
		sidecarArgs := []string{"--config=/algo-runner/algo-runner-config.json"}

		sidecarEnvVars = algoReconciler.createEnvVars(pipelineDeployment, runnerConfig, algoDepl)

		sidecarContainer := corev1.Container{
			Name:                     "algo-runner-sidecar",
			Image:                    sidecarImageName,
			Command:                  sidecarCommand,
			Args:                     sidecarArgs,
			Env:                      sidecarEnvVars,
			LivenessProbe:            livenessProbe,
			ReadinessProbe:           readinessProbe,
			ImagePullPolicy:          imagePullPolicy,
			TerminationMessagePath:   "/dev/termination-log",
			TerminationMessagePolicy: "File",
			VolumeMounts:             volumeMounts,
		}

		containers = append(containers, sidecarContainer)

	}

	kubeUtil := utils.NewKubeUtil(algoReconciler.manager, algoReconciler.request)
	resources, resourceErr := kubeUtil.CreateResourceReqs(algoDepl.Resources)

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
		LivenessProbe:            livenessProbe,
		ReadinessProbe:           readinessProbe,
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
			Namespace: pipelineDeployment.Spec.DeploymentNamespace,
			Name:      existingDeploymentName,
			Labels:    labels,
		}
	} else {
		nameMeta = metav1.ObjectMeta{
			Namespace:    pipelineDeployment.Spec.DeploymentNamespace,
			GenerateName: fmt.Sprintf("%s-", name),
			Labels:       labels,
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
			Replicas: &algoDepl.Replicas,
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

func (algoReconciler *AlgoReconciler) createConfigMap(algoDepl *v1beta1.AlgoDeploymentV1beta1,
	runnerConfig *algoRunnerConfig, labels map[string]string) (configMapName string, err error) {

	kubeUtil := utils.NewKubeUtil(algoReconciler.manager, algoReconciler.request)
	// Create all config mounts
	name := fmt.Sprintf("%s-%s-%s-%s-config",
		algoReconciler.pipelineDeployment.Spec.DeploymentOwner,
		algoReconciler.pipelineDeployment.Spec.DeploymentName,
		algoDepl.Spec.Owner,
		algoDepl.Spec.Name)
	data := make(map[string]string)

	// Add the runner-config
	// serialize the runner config to json string
	runnerConfigBytes, err := json.Marshal(runnerConfig)
	if err != nil {
		log.Error(err, "Failed deserializing runner config")
	}
	data["runner-config"] = string(runnerConfigBytes)

	// Add all config mounts
	for _, configMount := range algoDepl.ConfigMounts {
		data[configMount.Name] = configMount.Data
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: algoReconciler.pipelineDeployment.Spec.DeploymentNamespace,
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
	err = algoReconciler.manager.GetClient().Get(context.TODO(), types.NamespacedName{Name: name,
		Namespace: algoReconciler.pipelineDeployment.Spec.DeploymentNamespace},
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

		if !cmp.Equal(existingConfigMap.Data, configMap.Data) {
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

func (algoReconciler *AlgoReconciler) createEnvVars(cr *algov1beta1.PipelineDeployment,
	runnerConfig *algoRunnerConfig,
	algoDepl *v1beta1.AlgoDeploymentV1beta1) []corev1.EnvVar {

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
	if algoReconciler.kafkaUtil.TLS != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "KAFKA_TLS",
			Value: strconv.FormatBool(algoReconciler.kafkaUtil.CheckForKafkaTLS()),
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "KAFKA_TLS_CA_LOCATION",
			Value: "/etc/ssl/certs/kafka-ca.crt",
		})
	}

	// Append kafka auth variables
	if algoReconciler.kafkaUtil.Authentication != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "KAFKA_AUTH_TYPE",
			Value: string(algoReconciler.kafkaUtil.Authentication.Type),
		})
		if algoReconciler.kafkaUtil.Authentication.Type == kafkav1beta1.KAFKA_AUTH_TYPE_TLS {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "KAFKA_AUTH_TLS_USER_LOCATION",
				Value: "/etc/ssl/certs/kafka-user.crt",
			})
			envVars = append(envVars, corev1.EnvVar{
				Name:  "KAFKA_AUTH_TLS_KEY_LOCATION",
				Value: "/etc/ssl/certs/kafka-user.key",
			})
		}
		if algoReconciler.kafkaUtil.Authentication.Type == kafkav1beta1.KAFKA_AUTH_TYPE_SCRAMSHA512 ||
			algoReconciler.kafkaUtil.Authentication.Type == kafkav1beta1.KAFKA_AUTH_TYPE_PLAIN {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "KAFKA_AUTH_USERNAME",
				Value: algoReconciler.kafkaUtil.Authentication.Username,
			})
			envVars = append(envVars, corev1.EnvVar{
				Name: "KAFKA_AUTH_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: algoReconciler.kafkaUtil.Authentication.PasswordSecret.SecretName,
						},
						Key: algoReconciler.kafkaUtil.Authentication.PasswordSecret.Password,
					},
				},
			})
		}
	}

	// Append the storage server connection
	kubeUtil := utils.NewKubeUtil(algoReconciler.manager, algoReconciler.request)
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
		Value: "/bin/mc",
	})

	// Append all KafkaTopic Inputs
	for _, input := range algoDepl.Spec.Inputs {
		if *input.InputDeliveryType == v1beta1.INPUTDELIVERYTYPES_KAFKA_TOPIC {
			for _, pipe := range algoReconciler.pipelineDeployment.Spec.Pipes {
				if pipe.DestInputName == input.Name {
					tc := algoReconciler.allTopicConfigs[fmt.Sprintf("%s|%s", pipe.SourceName, pipe.SourceOutputName)]
					topicName := utils.GetTopicName(tc.TopicName, &algoReconciler.pipelineDeployment.Spec)
					envVars = append(envVars, corev1.EnvVar{
						Name:  fmt.Sprintf("KAFKA_INPUT_TOPIC_%s", strings.ToUpper(input.Name)),
						Value: topicName,
					})
				}
			}
		}
	}

	// Append all KafkaTopic Outputs
	for _, output := range algoDepl.Spec.Outputs {
		if *output.OutputDeliveryType == v1beta1.OUTPUTDELIVERYTYPES_KAFKA_TOPIC {
			for _, tc := range algoDepl.Topics {
				if output.Name == tc.OutputName {
					topicName := utils.GetTopicName(tc.TopicName, &algoReconciler.pipelineDeployment.Spec)
					envVars = append(envVars, corev1.EnvVar{
						Name:  fmt.Sprintf("KAFKA_OUTPUT_TOPIC_%s", strings.ToUpper(output.Name)),
						Value: topicName,
					})
				}
			}
		}
	}

	// for k, v := range algoDepl.EnvVars {
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
func (algoReconciler *AlgoReconciler) createRunnerConfig(pipelineDeploymentSpec *algov1beta1.PipelineDeploymentSpecV1beta1,
	algoDepl *v1beta1.AlgoDeploymentV1beta1) *algoRunnerConfig {

	topics := make(map[string]algov1beta1.TopicConfigModel)
	for key, value := range algoReconciler.allTopicConfigs {
		topics[key] = *value
	}

	runnerConfig := &algoRunnerConfig{
		DeploymentOwner: pipelineDeploymentSpec.DeploymentOwner,
		DeploymentName:  pipelineDeploymentSpec.DeploymentName,
		PipelineOwner:   pipelineDeploymentSpec.PipelineOwner,
		PipelineName:    pipelineDeploymentSpec.PipelineName,
		Pipes:           pipelineDeploymentSpec.Pipes,
		Topics:          topics,
		Owner:           algoDepl.Spec.Owner,
		Name:            algoDepl.Spec.Name,
		Version:         algoDepl.Version,
		Index:           algoDepl.Index,
		Entrypoint:      algoReconciler.activeAlgoVersion.Entrypoint,
		Executor:        algoDepl.Spec.Executor,
		Parameters:      algoDepl.Spec.Parameters,
		Inputs:          algoDepl.Spec.Inputs,
		Outputs:         algoDepl.Spec.Outputs,
		RetryEnabled:    algoDepl.RetryEnabled,
		RetryStrategy:   algoDepl.RetryStrategy,
		WriteAllOutputs: algoDepl.WriteAllOutputs,
		GpuEnabled:      algoDepl.Spec.GpuEnabled,
		TimeoutSeconds:  algoDepl.TimeoutSeconds,
	}

	return runnerConfig

}
