package reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"pipeline-operator/pkg/apis/algorun/v1beta1"
	algov1beta1 "pipeline-operator/pkg/apis/algorun/v1beta1"
	utils "pipeline-operator/pkg/utilities"
	"strconv"
	"strings"

	"github.com/go-test/deep"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewEndpointReconciler returns a new EndpointReconciler
func NewEndpointReconciler(pipelineDeployment *algov1beta1.PipelineDeployment,
	request *reconcile.Request,
	client client.Client,
	scheme *runtime.Scheme) EndpointReconciler {
	return EndpointReconciler{
		pipelineDeployment: pipelineDeployment,
		request:            request,
		client:             client,
		scheme:             scheme,
	}
}

// EndpointReconciler reconciles the endpoint container and mappings
type EndpointReconciler struct {
	pipelineDeployment *algov1beta1.PipelineDeployment
	request            *reconcile.Request
	client             client.Client
	scheme             *runtime.Scheme
	serviceConfig      *serviceConfig
}

// serviceConfig holds the service name and port for ambassador
type serviceConfig struct {
	serviceSpec *corev1.Service
	serviceName string
	httpPort    int32
	gRPCPort    int32
}

func (endpointReconciler *EndpointReconciler) Reconcile() error {

	sc, err := endpointReconciler.reconcileService()
	if err != nil {
		return err
	}
	endpointReconciler.serviceConfig = sc

	err = endpointReconciler.reconcileHttpMapping()
	err = endpointReconciler.reconcileGRPCMapping()

	err = endpointReconciler.reconcileDeployment()
	if err != nil {
		return err
	}

	return err

}

func (endpointReconciler *EndpointReconciler) reconcileService() (*serviceConfig, error) {

	kubeUtil := utils.NewKubeUtil(endpointReconciler.client)

	// Check to see if the endpoint service is already created (All algos share the same service port)
	srvListOptions := &client.ListOptions{}
	srvListOptions.SetLabelSelector(fmt.Sprintf("app.kubernetes.io/part-of=algorun, app.kubernetes.io/component=endpoint, algorun/pipeline-deployment=%s/%s",
		endpointReconciler.pipelineDeployment.Spec.PipelineSpec.DeploymentOwnerUserName,
		endpointReconciler.pipelineDeployment.Spec.PipelineSpec.DeploymentName))
	srvListOptions.InNamespace(endpointReconciler.request.NamespacedName.Namespace)

	existingService, err := kubeUtil.CheckForService(srvListOptions)
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

func (endpointReconciler *EndpointReconciler) reconcileDeployment() error {

	pipelineDeployment := endpointReconciler.pipelineDeployment
	request := endpointReconciler.request

	endpointLogger := log

	endpointLogger.Info("Reconciling Endpoint")

	name := "pipe-depl-ep"

	labels := map[string]string{
		"app.kubernetes.io/part-of":    "algorun",
		"app.kubernetes.io/component":  "algorun/endpoint",
		"app.kubernetes.io/managed-by": "algorun/pipeline-operator",
		"algorun/pipeline-deployment": fmt.Sprintf("%s/%s", pipelineDeployment.Spec.PipelineSpec.DeploymentOwnerUserName,
			pipelineDeployment.Spec.PipelineSpec.DeploymentName),
		"algorun/pipeline": fmt.Sprintf("%s/%s", pipelineDeployment.Spec.PipelineSpec.PipelineOwnerUserName,
			pipelineDeployment.Spec.PipelineSpec.PipelineName),
	}

	// Check to make sure the algo isn't already created
	listOptions := &client.ListOptions{}
	listOptions.SetLabelSelector(fmt.Sprintf("app.kubernetes.io/part-of=algorun, app.kubernetes.io/component=endpoint, algorun/pipeline-deployment=%s/%s",
		pipelineDeployment.Spec.PipelineSpec.DeploymentOwnerUserName,
		pipelineDeployment.Spec.PipelineSpec.DeploymentName))
	listOptions.InNamespace(request.NamespacedName.Namespace)

	kubeUtil := utils.NewKubeUtil(endpointReconciler.client)

	existingSf, err := kubeUtil.CheckForStatefulSet(listOptions)

	// Generate the k8s deployment
	endpointSf, err := endpointReconciler.createSpec(name, labels, existingSf)
	if err != nil {
		endpointLogger.Error(err, "Failed to create endpoint deployment spec")
		return err
	}

	// Set PipelineDeployment instance as the owner and controller
	if err := controllerutil.SetControllerReference(pipelineDeployment, endpointSf, endpointReconciler.scheme); err != nil {
		return err
	}

	if existingSf == nil {
		_, err := kubeUtil.CreateStatefulSet(endpointSf)
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
			_, err := kubeUtil.UpdateStatefulSet(endpointSf)
			if err != nil {
				endpointLogger.Error(err, "Failed to update endpoint deployment")
				return err
			}
		}
	}

	return nil

}

func (endpointReconciler *EndpointReconciler) reconcileHttpMapping() error {

	serviceName := fmt.Sprintf("http://%s:%d", endpointReconciler.serviceConfig.serviceName, endpointReconciler.serviceConfig.httpPort)
	endpointReconciler.reconcileMapping(serviceName, "http")

	return nil
}

func (endpointReconciler *EndpointReconciler) reconcileGRPCMapping() error {

	serviceName := fmt.Sprintf("%s:%d", endpointReconciler.serviceConfig.serviceName, endpointReconciler.serviceConfig.gRPCPort)
	endpointReconciler.reconcileMapping(serviceName, "grpc")

	return nil
}

func (endpointReconciler *EndpointReconciler) reconcileMapping(serviceName string, protocol string) error {

	pipelineDeployment := endpointReconciler.pipelineDeployment
	request := endpointReconciler.request

	// check to see if mapping already exists
	// Check to make sure the algo isn't already created
	listOptions := &client.ListOptions{}
	listOptions.SetLabelSelector(fmt.Sprintf("app.kubernetes.io/part-of=algorun, app.kubernetes.io/component=mapping, algorun/mapping-protocol=%s, algorun/pipeline-deployment=%s/%s",
		protocol,
		pipelineDeployment.Spec.PipelineSpec.DeploymentOwnerUserName,
		pipelineDeployment.Spec.PipelineSpec.DeploymentName))
	listOptions.InNamespace(request.NamespacedName.Namespace)

	kubeUtil := utils.NewKubeUtil(endpointReconciler.client)
	existingMapping, err := kubeUtil.CheckForUnstructured(listOptions, schema.GroupVersionKind{
		Group:   "getambassador.io",
		Kind:    "Mapping",
		Version: "v1",
	})

	var prefix string
	if protocol == "grpc" {
		prefix = "/run/grpc/"
	} else {
		prefix = "/run/http/"
	}

	if (err == nil && existingMapping == nil) || (err != nil && errors.IsNotFound(err)) {
		// Create the topic
		// Using a unstructured object to submit a ambassador mapping.

		labels := map[string]string{
			"app.kubernetes.io/part-of":    "algorun",
			"app.kubernetes.io/component":  "algorun/mapping",
			"app.kubernetes.io/managed-by": "algorun/pipeline-operator",
			"algorun/mapping-protocol":     protocol,
			"algorun/pipeline-deployment": fmt.Sprintf("%s/%s", pipelineDeployment.Spec.PipelineSpec.DeploymentOwnerUserName,
				pipelineDeployment.Spec.PipelineSpec.DeploymentName),
			"algorun/pipeline": fmt.Sprintf("%s/%s", pipelineDeployment.Spec.PipelineSpec.PipelineOwnerUserName,
				pipelineDeployment.Spec.PipelineSpec.PipelineName),
		}

		newMapping := &unstructured.Unstructured{}
		newMapping.Object = map[string]interface{}{
			"namespace": request.NamespacedName.Namespace,
			"spec": map[string]interface{}{
				"prefix":  prefix,
				"rewrite": "/",
				"grpc":    protocol == "grpc",
				"service": serviceName,
			},
		}
		newMapping.SetGenerateName("pipe-depl-ep-mapping")
		newMapping.SetNamespace(endpointReconciler.request.NamespacedName.Namespace)
		newMapping.SetLabels(labels)
		newMapping.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "getambassador.io",
			Kind:    "Mapping",
			Version: "v1",
		})

		// Set PipelineDeployment instance as the owner and controller
		if err := controllerutil.SetControllerReference(endpointReconciler.pipelineDeployment, newMapping, endpointReconciler.scheme); err != nil {
			log.Error(err, "Failed setting the pipeline deployment endpoint mapping controller owner")
		}

		err := endpointReconciler.client.Create(context.TODO(), newMapping)
		if err != nil {
			log.Error(err, "Failed creating pipeline deployment endpoint mapping")
		}
	} else if err != nil {
		log.Error(err, "Failed to check if pipeline deployment endpoint mapping exists.")
	} else {
		// Update the endpoint mapping if changed
		var prefixCurrent, serviceCurrent string
		spec, ok := existingMapping.Object["spec"].(map[string]interface{})
		if ok {
			prefixCurrent = spec["prefix"].(string)
			serviceCurrent = spec["service"].(string)
		}

		if prefixCurrent != prefix ||
			serviceCurrent != serviceName {

			spec["prefix"] = prefix
			spec["service"] = serviceName

			existingMapping.Object["spec"] = spec

			err := endpointReconciler.client.Update(context.TODO(), existingMapping)
			if err != nil {
				log.Error(err, "Failed updating pipeline deployment endpoint mapping")
			}
		}
	}

	return nil

}

// createSpec generates the k8s spec for the endpoint statefulset
func (endpointReconciler *EndpointReconciler) createSpec(name string, labels map[string]string, existingSf *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {

	pipelineDeployment := endpointReconciler.pipelineDeployment
	pipelineSpec := endpointReconciler.pipelineDeployment.Spec.PipelineSpec
	endpointConfig := pipelineSpec.EndpointConfig
	endpointConfig.DeploymentOwnerUserName = pipelineSpec.DeploymentOwnerUserName
	endpointConfig.DeploymentName = pipelineSpec.DeploymentName
	endpointConfig.PipelineOwnerUserName = pipelineSpec.PipelineOwnerUserName
	endpointConfig.PipelineName = pipelineSpec.PipelineName
	endpointConfig.Paths = pipelineSpec.EndpointConfig.Paths
	endpointConfig.Kafka = &algov1beta1.EndpointKafkaConfig{
		Brokers: []string{endpointReconciler.pipelineDeployment.Spec.KafkaBrokers},
	}

	// Set the image name
	var imageName string
	if endpointConfig.ImageTag == "" || endpointConfig.ImageTag == "latest" {
		imageName = fmt.Sprintf("%s:latest", endpointConfig.ImageRepository)
	} else {
		imageName = fmt.Sprintf("%s:%s", endpointConfig.ImageRepository, endpointConfig.ImageTag)
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
			Port:   intstr.FromInt(int(endpointReconciler.serviceConfig.httpPort)),
		},
	}

	var containers []corev1.Container

	hookCommand := []string{"/bin/deployment-endpoint"}
	hookEnvVars := endpointReconciler.createEnvVars(pipelineDeployment, endpointConfig)

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

	kubeUtil := utils.NewKubeUtil(endpointReconciler.client)
	resources, resourceErr := kubeUtil.CreateResourceReqs(endpointConfig.Resource)

	if resourceErr != nil {
		return nil, resourceErr
	}

	// Endpoint container
	endpointContainer := corev1.Container{
		Name:                     name,
		Image:                    imageName,
		Command:                  hookCommand,
		Env:                      hookEnvVars,
		Resources:                *resources,
		ImagePullPolicy:          imagePullPolicy,
		LivenessProbe:            livenessProbe,
		ReadinessProbe:           readinessProbe,
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: "File",
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "endpoint-wal-data",
				MountPath: "/data/wal",
			},
		},
	}
	containers = append(containers, endpointContainer)

	// nodeSelector := createSelector(request.Constraints)

	// If this is an update, need to set the existing deployment name
	var nameMeta metav1.ObjectMeta
	if existingSf != nil {
		nameMeta = metav1.ObjectMeta{
			Namespace: pipelineDeployment.Namespace,
			Name:      existingSf.Name,
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
			Replicas:             &endpointConfig.Resource.Instances,
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
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "endpoint-wal-data",
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

func (endpointReconciler *EndpointReconciler) createEnvVars(cr *algov1beta1.PipelineDeployment, endpointConfig *v1beta1.EndpointConfig) []corev1.EnvVar {

	envVars := []corev1.EnvVar{}

	// serialize the runner config to json string
	endpointConfigBytes, err := json.Marshal(endpointConfig)
	if err != nil {
		log.Error(err, "Failed deserializing endpoint config")
	}

	// Append the required runner config
	envVars = append(envVars, corev1.EnvVar{
		Name:  "ENDPOINT_CONFIG",
		Value: string(endpointConfigBytes),
	})

	// Append the storage server connection
	envVars = append(envVars, corev1.EnvVar{
		Name: "EP_UPLOADER_HOST",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "storage-config"},
				Key:                  "connection-string",
			},
		},
	})

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
		"app.kubernetes.io/part-of":    "algorun",
		"app.kubernetes.io/component":  "algorun/endpoint",
		"app.kubernetes.io/managed-by": "algorun/pipeline-operator",
		"algorun/pipeline-deployment": fmt.Sprintf("%s/%s", pipelineDeployment.Spec.PipelineSpec.DeploymentOwnerUserName,
			pipelineDeployment.Spec.PipelineSpec.DeploymentName),
		"algorun/pipeline": fmt.Sprintf("%s/%s", pipelineDeployment.Spec.PipelineSpec.PipelineOwnerUserName,
			pipelineDeployment.Spec.PipelineSpec.PipelineName),
	}

	var httpPort int32
	var gRPCPort int32
	if pipelineDeployment.Spec.PipelineSpec.EndpointConfig.Server != nil {
		u, err := url.Parse(pipelineDeployment.Spec.PipelineSpec.EndpointConfig.Server.Http.Listen)
		if err != nil || u == nil {
			httpPort = 18080
		} else {
			i64, err := strconv.Atoi(u.Port())
			if err != nil {
				httpPort = 18080
			}
			httpPort = int32(i64)
		}

		uGrpc, err := url.Parse(pipelineDeployment.Spec.PipelineSpec.EndpointConfig.Server.Grpc.Listen)
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
		httpPort = 18080
		gRPCPort = 18282
	}

	ms.httpPort = httpPort
	ms.gRPCPort = gRPCPort

	endpointServiceSpec := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    pipelineDeployment.Namespace,
			GenerateName: "pipe-depl-ep-service",
			Labels:       labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				corev1.ServicePort{
					Name: "http",
					Port: httpPort,
				},
				corev1.ServicePort{
					Name: "grpc",
					Port: gRPCPort,
				},
			},
			Selector: labels,
		},
	}

	ms.serviceSpec = endpointServiceSpec

	return ms, nil

}
