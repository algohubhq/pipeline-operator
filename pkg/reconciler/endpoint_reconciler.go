package reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"pipeline-operator/pkg/apis/algorun/v1beta1"
	algov1beta1 "pipeline-operator/pkg/apis/algorun/v1beta1"
	utils "pipeline-operator/pkg/utilities"
	"reflect"
	"strconv"
	"strings"

	ambv2 "pipeline-operator/pkg/apis/getambassador/v2"

	"github.com/go-test/deep"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewEndpointReconciler returns a new EndpointReconciler
func NewEndpointReconciler(pipelineDeployment *algov1beta1.PipelineDeployment,
	kafkaUtil *utils.KafkaUtil,
	request *reconcile.Request,
	manager manager.Manager,
	scheme *runtime.Scheme) EndpointReconciler {
	endpointConfig := &endpointConfig{
		DeploymentOwner: pipelineDeployment.Spec.DeploymentOwner,
		DeploymentName:  pipelineDeployment.Spec.DeploymentName,
		PipelineOwner:   pipelineDeployment.Spec.PipelineOwner,
		PipelineName:    pipelineDeployment.Spec.PipelineName,
		EndpointSpec:    pipelineDeployment.Spec.Endpoint,
	}
	return EndpointReconciler{
		pipelineDeployment: pipelineDeployment,
		endpointConfig:     endpointConfig,
		kafkaUtil:          kafkaUtil,
		request:            request,
		manager:            manager,
		scheme:             scheme,
	}
}

// EndpointReconciler reconciles the endpoint container and mappings
type EndpointReconciler struct {
	pipelineDeployment *algov1beta1.PipelineDeployment
	endpointConfig     *endpointConfig
	kafkaUtil          *utils.KafkaUtil
	request            *reconcile.Request
	manager            manager.Manager
	scheme             *runtime.Scheme
	serviceConfig      *serviceConfig
}

// endpointConfig holds the config sent to the endpoint container
type endpointConfig struct {
	DeploymentOwner string
	DeploymentName  string
	PipelineOwner   string
	PipelineName    string
	// embed the endpoint Spec
	*v1beta1.EndpointSpec
}

// serviceConfig holds the service name and port for ambassador
type serviceConfig struct {
	serviceSpec *corev1.Service
	serviceName string
	httpPort    int32
	gRPCPort    int32
}

// Reconcile executes the Endpoint reconciliation process
func (endpointReconciler *EndpointReconciler) Reconcile() error {

	labels := map[string]string{
		"app.kubernetes.io/part-of":    "algo.run",
		"app.kubernetes.io/component":  "endpoint",
		"app.kubernetes.io/managed-by": "pipeline-operator",
		"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", endpointReconciler.pipelineDeployment.Spec.DeploymentOwner,
			endpointReconciler.pipelineDeployment.Spec.DeploymentName),
		"algo.run/pipeline": fmt.Sprintf("%s.%s", endpointReconciler.pipelineDeployment.Spec.PipelineOwner,
			endpointReconciler.pipelineDeployment.Spec.PipelineName),
	}

	sc, err := endpointReconciler.reconcileService()
	if err != nil {
		return err
	}
	endpointReconciler.serviceConfig = sc

	err = endpointReconciler.reconcileHTTPMapping()
	err = endpointReconciler.reconcileGRPCMapping()

	endpointReconciler.endpointConfig.Kafka = &algov1beta1.EndpointKafkaConfig{
		Brokers: []string{endpointReconciler.pipelineDeployment.Spec.KafkaBrokers},
	}

	// Create the configmap for the endpoint
	configMapName, err := endpointReconciler.createConfigMap(labels)

	err = endpointReconciler.reconcileDeployment(labels, configMapName)
	if err != nil {
		return err
	}

	return err

}

func (endpointReconciler *EndpointReconciler) reconcileService() (*serviceConfig, error) {

	kubeUtil := utils.NewKubeUtil(endpointReconciler.manager, endpointReconciler.request)

	// Check to see if the endpoint service is already created (All algos share the same service port)
	opts := []client.ListOption{
		client.InNamespace(endpointReconciler.pipelineDeployment.Spec.DeploymentNamespace),
		client.MatchingLabels{
			"app.kubernetes.io/part-of":   "algo.run",
			"app.kubernetes.io/component": "endpoint",
			"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", endpointReconciler.pipelineDeployment.Spec.DeploymentOwner,
				endpointReconciler.pipelineDeployment.Spec.DeploymentName),
		},
	}

	existingService, err := kubeUtil.CheckForService(opts)
	if err != nil {
		log.Error(err, "Failed to check for existing endpoint service")
		return nil, err
	}
	if existingService == nil {

		// Generate the service for the endpoint
		ms, err := endpointReconciler.createServiceSpec(endpointReconciler.pipelineDeployment)
		if err != nil {
			log.Error(err, "Failed to create pipeline deployment endpoint service spec")
			return nil, err
		}

		// Set PipelineDeployment instance as the owner and controller
		if err := controllerutil.SetControllerReference(endpointReconciler.pipelineDeployment, ms.serviceSpec, endpointReconciler.scheme); err != nil {
			return ms, err
		}

		serviceName, err := kubeUtil.CreateService(ms.serviceSpec)
		if err != nil {
			log.Error(err, "Failed to create pipeline deployment endpoint service")
			return nil, err
		}
		ms.serviceName = serviceName

		return ms, nil
	}

	ms, err := endpointReconciler.createServiceSpec(endpointReconciler.pipelineDeployment)
	if err != nil {
		log.Error(err, "Failed to create pipeline deployment endpoint service spec")
		return nil, err
	}
	ms.serviceName = existingService.GetName()

	return ms, nil

}

func (endpointReconciler *EndpointReconciler) reconcileDeployment(labels map[string]string, configMapName string) error {

	pipelineDeployment := endpointReconciler.pipelineDeployment

	endpointLogger := log

	endpointLogger.Info("Reconciling Endpoint")

	// Creat the Kafka Topics
	log.Info("Reconciling Kakfa Topics for Endpoint Path Outputs")
	for _, path := range pipelineDeployment.Spec.Endpoint.Paths {
		go func(currentTopicConfig algov1beta1.TopicConfigModel) {
			topicReconciler := NewTopicReconciler(endpointReconciler.pipelineDeployment,
				"Endpoint",
				&currentTopicConfig,
				endpointReconciler.kafkaUtil,
				endpointReconciler.request,
				endpointReconciler.manager,
				endpointReconciler.scheme)
			topicReconciler.Reconcile()
		}(*path.Topic)
	}

	name := fmt.Sprintf("endpoint-%s-%s", pipelineDeployment.Spec.DeploymentOwner,
		pipelineDeployment.Spec.DeploymentName)

	// Check to make sure the endpoint isn't already created
	opts := []client.ListOption{
		client.InNamespace(endpointReconciler.pipelineDeployment.Spec.DeploymentNamespace),
		client.MatchingLabels{
			"app.kubernetes.io/part-of":   "algo.run",
			"app.kubernetes.io/component": "endpoint",
			"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", endpointReconciler.pipelineDeployment.Spec.DeploymentOwner,
				endpointReconciler.pipelineDeployment.Spec.DeploymentName),
		},
	}

	kubeUtil := utils.NewKubeUtil(endpointReconciler.manager, endpointReconciler.request)

	var endpointName string
	existingSf, err := kubeUtil.CheckForStatefulSet(opts)
	if existingSf != nil {
		endpointName = existingSf.GetName()
	}

	// Generate the k8s deployment
	endpointSf, err := endpointReconciler.createSpec(name, labels, configMapName, existingSf)
	if err != nil {
		endpointLogger.Error(err, "Failed to create endpoint deployment spec")
		return err
	}

	// Set PipelineDeployment instance as the owner and controller
	if err := controllerutil.SetControllerReference(pipelineDeployment, endpointSf, endpointReconciler.scheme); err != nil {
		return err
	}

	if existingSf == nil {
		endpointName, err = kubeUtil.CreateStatefulSet(endpointSf)
		if err != nil {
			endpointLogger.Error(err, "Failed to create endpoint statefulset")
			return err
		}
	} else {
		var deplChanged bool

		// Set some values that are defaulted by k8s but shouldn't trigger a change
		endpointSf.Spec.Template.Spec.TerminationGracePeriodSeconds = existingSf.Spec.Template.Spec.TerminationGracePeriodSeconds
		endpointSf.Spec.Template.Spec.SecurityContext = existingSf.Spec.Template.Spec.SecurityContext
		endpointSf.Spec.Template.Spec.SchedulerName = existingSf.Spec.Template.Spec.SchedulerName

		if *existingSf.Spec.Replicas != *endpointSf.Spec.Replicas {
			endpointLogger.Info("Endpoint Replica Count Changed. Updating deployment.",
				"Old Replicas", existingSf.Spec.Replicas,
				"New Replicas", endpointSf.Spec.Replicas)
			deplChanged = true
		} else if diff := deep.Equal(existingSf.Spec.Template.Spec, endpointSf.Spec.Template.Spec); diff != nil {
			endpointLogger.Info("Endpoint Changed. Updating deployment.", "Differences", diff)
			deplChanged = true

		}
		if deplChanged {
			endpointName, err = kubeUtil.UpdateStatefulSet(endpointSf)
			if err != nil {
				endpointLogger.Error(err, "Failed to update endpoint deployment")
				return err
			}
		}
	}

	// Setup the horizontal pod autoscaler
	if pipelineDeployment.Spec.Endpoint.Autoscaling != nil &&
		pipelineDeployment.Spec.Endpoint.Autoscaling.Enabled {

		labels := map[string]string{
			"app.kubernetes.io/part-of":    "algo.run",
			"app.kubernetes.io/component":  "endpoint-hpa",
			"app.kubernetes.io/managed-by": "pipeline-operator",
			"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", pipelineDeployment.Spec.DeploymentOwner,
				pipelineDeployment.Spec.DeploymentName),
			"algo.run/pipeline": fmt.Sprintf("%s.%s", pipelineDeployment.Spec.PipelineOwner,
				pipelineDeployment.Spec.PipelineName),
		}

		opts := []client.ListOption{
			client.InNamespace(endpointReconciler.pipelineDeployment.Spec.DeploymentNamespace),
			client.MatchingLabels(labels),
		}

		existingHpa, err := kubeUtil.CheckForHorizontalPodAutoscaler(opts)

		hpaSpec, err := kubeUtil.CreateHpaSpec(endpointName, labels, pipelineDeployment, pipelineDeployment.Spec.Endpoint.Autoscaling)
		if err != nil {
			endpointLogger.Error(err, "Failed to create Endpoint horizontal pod autoscaler spec")
			return err
		}

		// Set PipelineDeployment instance as the owner and controller
		if err := controllerutil.SetControllerReference(pipelineDeployment, hpaSpec, endpointReconciler.scheme); err != nil {
			return err
		}

		if existingHpa == nil {
			_, err = kubeUtil.CreateHorizontalPodAutoscaler(hpaSpec)
			if err != nil {
				endpointLogger.Error(err, "Failed to create Endpoint horizontal pod autoscaler")
				return err
			}
		} else {
			var deplChanged bool

			if existingHpa.Spec.Metrics != nil && hpaSpec.Spec.Metrics != nil {
				if diff := deep.Equal(existingHpa.Spec, hpaSpec.Spec); diff != nil {
					endpointLogger.Info("Endpoint Horizontal Pod Autoscaler Changed. Updating...", "Differences", diff)
					deplChanged = true
				}
			}
			if deplChanged {
				_, err := kubeUtil.UpdateHorizontalPodAutoscaler(hpaSpec)
				if err != nil {
					endpointLogger.Error(err, "Failed to update horizontal pod autoscaler")
					return err
				}
			}
		}

	}

	return nil

}

func (endpointReconciler *EndpointReconciler) reconcileHTTPMapping() error {

	serviceName := fmt.Sprintf("http://%s.%s:%d", endpointReconciler.serviceConfig.serviceName,
		endpointReconciler.pipelineDeployment.Spec.DeploymentNamespace,
		endpointReconciler.serviceConfig.httpPort)
	endpointReconciler.reconcileMapping(serviceName, "http")

	return nil
}

func (endpointReconciler *EndpointReconciler) reconcileGRPCMapping() error {

	serviceName := fmt.Sprintf("%s.%s:%d", endpointReconciler.serviceConfig.serviceName,
		endpointReconciler.pipelineDeployment.Spec.DeploymentNamespace,
		endpointReconciler.serviceConfig.gRPCPort)
	endpointReconciler.reconcileMapping(serviceName, "grpc")

	return nil
}

func (endpointReconciler *EndpointReconciler) reconcileMapping(serviceName string, protocol string) error {

	pipelineDeployment := endpointReconciler.pipelineDeployment

	// check to see if mapping already exists
	// Check to make sure the algo isn't already created
	opts := []client.ListOption{
		client.InNamespace(endpointReconciler.pipelineDeployment.Spec.DeploymentNamespace),
		client.MatchingLabels{
			"app.kubernetes.io/part-of":   "algo.run",
			"app.kubernetes.io/component": "mapping",
			"algo.run/mapping-protocol":   protocol,
			"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", endpointReconciler.pipelineDeployment.Spec.DeploymentOwner,
				endpointReconciler.pipelineDeployment.Spec.DeploymentName),
		},
	}

	var prefix string
	if protocol == "grpc" {
		prefix = fmt.Sprintf("/run/grpc/%s/%s/", pipelineDeployment.Spec.DeploymentOwner,
			pipelineDeployment.Spec.DeploymentName)
	} else {
		prefix = fmt.Sprintf("/run/http/%s/%s/", pipelineDeployment.Spec.DeploymentOwner,
			pipelineDeployment.Spec.DeploymentName)
	}
	rewrite := fmt.Sprintf("/%s/%s/", pipelineDeployment.Spec.DeploymentOwner,
		pipelineDeployment.Spec.DeploymentName)

	mappingSpec := ambv2.MappingSpec{
		Prefix:  prefix,
		Rewrite: rewrite,
		Grpc:    protocol == "grpc",
		Service: serviceName,
	}

	kubeUtil := utils.NewKubeUtil(endpointReconciler.manager, endpointReconciler.request)
	existingMapping, err := kubeUtil.CheckForAmbassadorMapping(opts)

	if (err == nil && existingMapping == nil) || (err != nil && errors.IsNotFound(err)) {

		labels := map[string]string{
			"app.kubernetes.io/part-of":    "algo.run",
			"app.kubernetes.io/component":  "mapping",
			"app.kubernetes.io/managed-by": "pipeline-operator",
			"algo.run/mapping-protocol":    protocol,
			"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", pipelineDeployment.Spec.DeploymentOwner,
				pipelineDeployment.Spec.DeploymentName),
			"algo.run/pipeline": fmt.Sprintf("%s.%s", pipelineDeployment.Spec.PipelineOwner,
				pipelineDeployment.Spec.PipelineName),
		}

		newMapping := &ambv2.Mapping{}
		newMapping.Spec = mappingSpec
		newMapping.SetGenerateName("endpoint-mapping-")
		newMapping.SetNamespace(endpointReconciler.pipelineDeployment.Spec.DeploymentNamespace)
		newMapping.SetLabels(labels)

		// Set PipelineDeployment instance as the owner and controller
		if err := controllerutil.SetControllerReference(endpointReconciler.pipelineDeployment, newMapping, endpointReconciler.scheme); err != nil {
			log.Error(err, "Failed setting the pipeline deployment endpoint mapping controller owner")
		}

		err := endpointReconciler.manager.GetClient().Create(context.TODO(), newMapping)
		if err != nil {
			log.Error(err, "Failed creating pipeline deployment endpoint mapping")
		}
	} else if err != nil {
		log.Error(err, "Failed to check if pipeline deployment endpoint mapping exists.")
	} else {

		// Update the endpoint mapping if changed
		if !reflect.DeepEqual(existingMapping.Spec, mappingSpec) {

			// Update the existing spec
			existingMapping.Spec = mappingSpec

			err := endpointReconciler.manager.GetClient().Update(context.TODO(), existingMapping)
			if err != nil {
				log.Error(err, "Failed updating pipeline deployment endpoint mapping")
			}

		}

	}

	return nil

}

// createSpec generates the k8s spec for the endpoint statefulset
func (endpointReconciler *EndpointReconciler) createSpec(name string, labels map[string]string, configMapName string, existingSf *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {

	pipelineDeployment := endpointReconciler.pipelineDeployment
	endpointConfig := endpointReconciler.endpointConfig

	// Set the image name
	imagePullPolicy := corev1.PullIfNotPresent
	imageName := os.Getenv("ENDPOINT_IMAGE")
	if imageName == "" {
		if endpointConfig.Image == nil {
			imageName = "algohub/deployment-endpoint:latest"
		} else {
			if endpointConfig.Image.Tag == "" {
				imageName = fmt.Sprintf("%s:latest", endpointConfig.Image.Repository)
			} else {
				imageName = fmt.Sprintf("%s:%s", endpointConfig.Image.Repository, endpointConfig.Image.Tag)
			}
			switch *endpointConfig.Image.ImagePullPolicy {
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
			Port:   intstr.FromInt(int(endpointReconciler.serviceConfig.httpPort)),
		},
	}

	var containers []corev1.Container

	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}

	kafkaUtil := endpointReconciler.kafkaUtil

	if endpointReconciler.kafkaUtil.TLS != nil {

		if endpointConfig.Kafka.Params == nil {
			endpointConfig.Kafka.Params = make(map[string]string, 0)
		}

		endpointConfig.Kafka.Params["security.protocol"] = "ssl"
		endpointConfig.Kafka.Params["ssl.ca.location"] = "/etc/ssl/certs/kafka-ca.crt"

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
				SubPath:   "ca.crt",
				MountPath: "/etc/ssl/certs/kafka-ca.crt",
				ReadOnly:  true,
			},
		}
		volumeMounts = append(volumeMounts, kafkaTLSMounts...)
	}

	if kafkaUtil.Authentication != nil {

		if endpointConfig.Kafka.Params == nil {
			endpointConfig.Kafka.Params = make(map[string]string, 0)
		}

		endpointConfig.Kafka.Params["ssl.certificate.location"] = "ssl"
		endpointConfig.Kafka.Params["ssl.key.location"] = "/etc/ssl/certs/kafka-ca.crt"

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
		Name: "endpoint-config-volume",
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
		Name:      "endpoint-config-volume",
		SubPath:   "endpoint-config",
		MountPath: "/endpoint-config/endpoint-config.json",
	}
	volumeMounts = append(volumeMounts, configVolumeMount)

	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      fmt.Sprintf("%s-wal-data", name),
		MountPath: "/data/wal",
	})

	endpointCommand := []string{"/bin/deployment-endpoint"}
	endpointEnvVars := endpointReconciler.createEnvVars(pipelineDeployment, endpointConfig)

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

	kubeUtil := utils.NewKubeUtil(endpointReconciler.manager, endpointReconciler.request)
	resources, resourceErr := kubeUtil.CreateResourceReqs(endpointConfig.Resources)

	if resourceErr != nil {
		return nil, resourceErr
	}

	configArgs := []string{"--config=/endpoint-config/endpoint-config.json"}

	// Endpoint container
	endpointContainer := corev1.Container{
		Name:                     name,
		Image:                    imageName,
		Command:                  endpointCommand,
		Args:                     configArgs,
		Env:                      endpointEnvVars,
		Resources:                *resources,
		ImagePullPolicy:          imagePullPolicy,
		LivenessProbe:            livenessProbe,
		ReadinessProbe:           readinessProbe,
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: "File",
		VolumeMounts:             volumeMounts,
	}
	containers = append(containers, endpointContainer)

	// nodeSelector := createSelector(request.Constraints)

	// If this is an update, need to set the existing deployment name
	var nameMeta metav1.ObjectMeta
	if existingSf != nil {
		nameMeta = metav1.ObjectMeta{
			Namespace: pipelineDeployment.Spec.DeploymentNamespace,
			Name:      existingSf.Name,
			Labels:    labels,
			// Annotations: annotations,
		}
	} else {
		nameMeta = metav1.ObjectMeta{
			Namespace: pipelineDeployment.Spec.DeploymentNamespace,
			Name:      name,
			Labels:    labels,
			// Annotations: annotations,
		}
	}

	walSize := resource.MustParse("1Gi")
	if endpointConfig.Producer != nil &&
		endpointConfig.Producer.Wal != nil &&
		endpointConfig.Producer.Wal.Size != "" {
		var err error
		walSize, err = resource.ParseQuantity(endpointConfig.Producer.Wal.Size)
		if err != nil {
			walSize = resource.MustParse("1Gi")
		}
	}

	// annotations := buildAnnotations(request)
	sfSpec := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: nameMeta,
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas:             &endpointConfig.Replicas,
			RevisionHistoryLimit: utils.Int32p(10),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: nameMeta,
				Spec: corev1.PodSpec{
					// SecurityContext: &corev1.PodSecurityContext{
					//	FSGroup: int64p(1431),
					// },
					// NodeSelector: nodeSelector,
					Containers:    containers,
					Volumes:       volumes,
					RestartPolicy: corev1.RestartPolicyAlways,
					DNSPolicy:     corev1.DNSClusterFirst,
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("%s-wal-data", name),
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							"ReadWriteOnce",
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: walSize,
							},
						},
					},
				},
			},
		},
	}

	// if err := UpdateSecrets(request, deploymentSpec, existingSecrets); err != nil {
	// 	return nil, err
	// }

	return sfSpec, nil

}

func (endpointReconciler *EndpointReconciler) createConfigMap(labels map[string]string) (configMapName string, err error) {

	kubeUtil := utils.NewKubeUtil(endpointReconciler.manager, endpointReconciler.request)
	// Create all config mounts
	name := fmt.Sprintf("%s-%s-%s-config",
		endpointReconciler.pipelineDeployment.Spec.DeploymentOwner,
		endpointReconciler.pipelineDeployment.Spec.DeploymentName,
		"endpoint")
	data := make(map[string]string)

	// serialize the endpoint config to json string
	endpointConfigBytes, err := json.Marshal(endpointReconciler.endpointConfig)
	if err != nil {
		log.Error(err, "Failed deserializing endpoint config")
	}
	data["endpoint-config"] = string(endpointConfigBytes)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: endpointReconciler.pipelineDeployment.Spec.DeploymentNamespace,
			Name:      name,
			Labels:    labels,
			// Annotations: annotations,
		},
		Data: data,
	}

	// Set PipelineDeployment instance as the owner and controller
	if err := controllerutil.SetControllerReference(endpointReconciler.pipelineDeployment, configMap, endpointReconciler.scheme); err != nil {
		return name, err
	}

	existingConfigMap := &corev1.ConfigMap{}
	err = endpointReconciler.manager.GetClient().Get(context.TODO(), types.NamespacedName{Name: name,
		Namespace: endpointReconciler.pipelineDeployment.Spec.DeploymentNamespace},
		existingConfigMap)

	if err != nil && errors.IsNotFound(err) {
		// Create the ConfigMap
		name, err = kubeUtil.CreateConfigMap(configMap)
		if err != nil {
			log.Error(err, "Failed creating endpoint ConfigMap")
		}

	} else if err != nil {
		log.Error(err, "Failed to check if endpoint ConfigMap exists.")
	} else {

		if !reflect.DeepEqual(existingConfigMap.Data, configMap.Data) {
			// Update configmap
			name, err = kubeUtil.UpdateConfigMap(configMap)
			if err != nil {
				log.Error(err, "Failed to update endpoint configmap")
				return name, err
			}
		}

	}

	return name, err

}

func (endpointReconciler *EndpointReconciler) createEnvVars(cr *algov1beta1.PipelineDeployment, endpointConfig *endpointConfig) []corev1.EnvVar {

	envVars := []corev1.EnvVar{}

	// Append the storage server connection
	kubeUtil := utils.NewKubeUtil(endpointReconciler.manager, endpointReconciler.request)
	storageSecretName, err := kubeUtil.GetStorageSecretName(&endpointReconciler.pipelineDeployment.Spec)
	if storageSecretName != "" && err == nil {
		envVars = append(envVars, corev1.EnvVar{
			Name: "EP_UPLOADER_HOST",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: storageSecretName},
					Key:                  "connection-string",
				},
			},
		})
	}

	return envVars

}

func (endpointReconciler *EndpointReconciler) createSelector(constraints []string) map[string]string {
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

func (endpointReconciler *EndpointReconciler) createServiceSpec(pipelineDeployment *algov1beta1.PipelineDeployment) (*serviceConfig, error) {

	ms := &serviceConfig{}

	labels := map[string]string{
		"app.kubernetes.io/part-of":    "algo.run",
		"app.kubernetes.io/component":  "endpoint",
		"app.kubernetes.io/managed-by": "pipeline-operator",
		"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", pipelineDeployment.Spec.DeploymentOwner,
			pipelineDeployment.Spec.DeploymentName),
		"algo.run/pipeline": fmt.Sprintf("%s.%s", pipelineDeployment.Spec.PipelineOwner,
			pipelineDeployment.Spec.PipelineName),
	}

	var httpPort int32
	var gRPCPort int32
	if pipelineDeployment.Spec.Endpoint.Server != nil &&
		pipelineDeployment.Spec.Endpoint.Server.Http != nil {
		u, err := url.Parse(pipelineDeployment.Spec.Endpoint.Server.Http.Listen)
		if err != nil || u == nil {
			httpPort = 18080
		} else {
			i64, err := strconv.Atoi(u.Port())
			if err != nil {
				httpPort = 18080
			}
			httpPort = int32(i64)
		}
	} else {
		httpPort = 18080
	}

	if pipelineDeployment.Spec.Endpoint.Server != nil &&
		pipelineDeployment.Spec.Endpoint.Server.Grpc != nil {
		uGrpc, err := url.Parse(pipelineDeployment.Spec.Endpoint.Server.Grpc.Listen)
		if err != nil || uGrpc == nil {
			gRPCPort = 18282
		} else {
			i64, err := strconv.Atoi(uGrpc.Port())
			if err != nil {
				gRPCPort = 18282
			}
			gRPCPort = int32(i64)
		}

	} else {
		gRPCPort = 18282
	}

	ms.httpPort = httpPort
	ms.gRPCPort = gRPCPort

	name := fmt.Sprintf("%s-%s-endpoint-service",
		pipelineDeployment.Spec.DeploymentOwner,
		pipelineDeployment.Spec.DeploymentName)

	endpointServiceSpec := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: pipelineDeployment.Spec.DeploymentNamespace,
			Name:      name,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: "http",
					Port: httpPort,
				},
				{
					Name: "grpc",
					Port: gRPCPort,
				},
				{
					Name: "metrics",
					Port: 28080,
				},
			},
			Selector: labels,
		},
	}

	ms.serviceSpec = endpointServiceSpec

	return ms, nil

}
