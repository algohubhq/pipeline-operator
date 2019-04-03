package endpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"endpoint-operator/pkg/apis/algo/v1alpha1"
	algov1alpha1 "endpoint-operator/pkg/apis/algo/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_endpoint")

// Add creates a new Endpoint Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileEndpoint{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("endpoint-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Endpoint
	err = c.Watch(&source.Kind{Type: &algov1alpha1.Endpoint{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Endpoint
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &algov1alpha1.Endpoint{},
	})
	if err != nil {
		return err
	}
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &algov1alpha1.Endpoint{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileEndpoint{}

// ReconcileEndpoint reconciles a Endpoint object
type ReconcileEndpoint struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Endpoint object and makes changes based on the state read
// and what is in the Endpoint.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileEndpoint) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Endpoint")

	// Fetch the Endpoint instance
	instance := &algov1alpha1.Endpoint{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("Endpoint resource not found. Ignoring since object must be deleted.")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Info("Error reading the Endpoint Instance object - requeuing the request")
		return reconcile.Result{}, err
	}

	// Create / update the kafka topics
	r.createTopics(&instance.Spec, request)

	// Reconcile all algo deployments
	reqLogger.Info("Reconciling Algos")
	err = r.reconcileAlgos(instance, request)

	if err != nil {
		reqLogger.Error(err, "Error in AlgoConfig reconcile loop.")
		// TODO: Depending on the error, determine if it should be requeued
		return reconcile.Result{}, err
	}

	// Update status if needed
	pods, err := r.getPods(instance, request)
	if err != nil {
		reqLogger.Error(err, "Failed to get pod list to determine the status.")
		return reconcile.Result{}, err
	}

	deployments, err := r.getDeployments(instance, request)
	if err != nil {
		reqLogger.Error(err, "Failed to get deployment list to determine the status.")
		return reconcile.Result{}, err
	}

	// TODO: Get the overall endpoint State

	if !reflect.DeepEqual(pods, instance.Status.Pods) ||
		!reflect.DeepEqual(deployments, instance.Status.Deployments) {

		instance.Status.Pods = *pods
		instance.Status.Deployments = *deployments
		err := r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update Endpoint status.")
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil

}

func (r *ReconcileEndpoint) getDeployments(cr *algov1alpha1.Endpoint, request reconcile.Request) (*appsv1.DeploymentList, error) {

	// Watch all algo deployments
	listOptions := &client.ListOptions{}
	listOptions.SetLabelSelector(fmt.Sprintf("system=algorun, tier=algo, endpointowner=%s, endpoint=%s",
		cr.Spec.EndpointConfig.EndpointOwnerUserName,
		cr.Spec.EndpointConfig.EndpointName))
	listOptions.InNamespace(request.NamespacedName.Namespace)

	deploymentList := &appsv1.DeploymentList{}
	ctx := context.TODO()
	err := r.client.List(ctx, listOptions, deploymentList)

	if err != nil {
		return nil, err
	}

	return deploymentList, nil

}

func (r *ReconcileEndpoint) getPods(cr *algov1alpha1.Endpoint, request reconcile.Request) (*corev1.PodList, error) {

	// Watch all algo deployments
	listOptions := &client.ListOptions{}
	listOptions.SetLabelSelector(fmt.Sprintf("system=algorun, tier=algo, endpointowner=%s, endpoint=%s",
		cr.Spec.EndpointConfig.EndpointOwnerUserName,
		cr.Spec.EndpointConfig.EndpointName))
	listOptions.InNamespace(request.NamespacedName.Namespace)

	podList := &corev1.PodList{}
	ctx := context.TODO()
	err := r.client.List(ctx, listOptions, podList)

	if err != nil {
		return nil, err
	}

	return podList, nil

}

func calculateStatus(cr *algov1alpha1.Endpoint, deployments []*appsv1.Deployment) (string, error) {

	// SET Status = COALESCE(
	// 	(SELECT
	// 	  CASE WHEN (COUNT(ad.Id) > 0 AND COUNT(ad.Id) = e.AlgoCount) THEN 'Started'
	// 	  WHEN (COUNT(ad.Id) > 0 AND COUNT(ad.Id) < e.AlgoCount) THEN 'Updating'
	// 	  ELSE 'Stopped' END AS Status
	// 	  FROM (SELECT Id, AlgoCount FROM endpoint) AS e
	// 	  LEFT JOIN algo_deployment ad ON e.Id = ad.EndpointId
	// 	  WHERE e.Id = @EndpointId
	// 	  GROUP BY e.Id), Status)
	// 	  WHERE Id = @EndpointId";

	return "Stopped", nil

}

// reconcileAlgos creates or updates all algos for the endpoint
func (r *ReconcileEndpoint) reconcileAlgos(cr *algov1alpha1.Endpoint, request reconcile.Request) error {

	// Iterate the AlgoConfigs
	for _, algoConfig := range cr.Spec.EndpointConfig.AlgoConfigs {

		algoLogger := log.WithValues("AlgoOwner", algoConfig.AlgoOwnerUserName, "AlgoName", algoConfig.AlgoName, "AlgoVersionTag", algoConfig.AlgoVersionTag, "Index", algoConfig.AlgoIndex)
		algoLogger.Info("Reconciling Algo")

		// Truncate the name of the deployment / pod just in case
		name := strings.TrimRight(short(algoConfig.AlgoName, 20), "-")

		// Generate the runnerconfig
		runnerConfig := createRunnerConfig(&cr.Spec, &algoConfig)

		labels := map[string]string{
			"system":        "algorun",
			"tier":          "algo",
			"endpointowner": runnerConfig.EndpointOwnerUserName,
			"endpoint":      runnerConfig.EndpointName,
			"pipeline":      runnerConfig.PipelineName,
			"algoowner":     algoConfig.AlgoOwnerUserName,
			"algo":          algoConfig.AlgoName,
			"algoversion":   algoConfig.AlgoVersionTag,
			"algoindex":     strconv.Itoa(int(algoConfig.AlgoIndex)),
			"env":           "production",
		}

		// Check to make sure the algo isn't already created
		listOptions := &client.ListOptions{}
		listOptions.SetLabelSelector(fmt.Sprintf("system=algorun, tier=algo, endpointowner=%s, endpoint=%s, algoowner=%s, algo=%s, algoversion=%s, algoindex=%v",
			runnerConfig.EndpointOwnerUserName,
			runnerConfig.EndpointName,
			algoConfig.AlgoOwnerUserName,
			algoConfig.AlgoName,
			algoConfig.AlgoVersionTag,
			algoConfig.AlgoIndex))
		listOptions.InNamespace(request.NamespacedName.Namespace)

		deploymentExists := r.checkForDeployment(listOptions)

		// Generate the k8s deployment for the algoconfig
		algoDeployment, err := createDeploymentSpec(cr, name, labels, &algoConfig, &runnerConfig, deploymentExists)
		if err != nil {
			algoLogger.Error(err, "Failed to create algo deployment spec")
			return err
		}

		// Set Endpoint instance as the owner and controller
		if err := controllerutil.SetControllerReference(cr, algoDeployment, r.scheme); err != nil {
			return err
		}

		if !deploymentExists {
			err := r.createDeployment(algoDeployment)
			if err != nil {
				algoLogger.Error(err, "Failed to create algo deployment")
				return err
			}
		} else {
			err := r.updateDeployment(algoDeployment)
			if err != nil {
				algoLogger.Error(err, "Failed to update algo deployment")
				return err
			}
		}

		// TODO: Setup the horizontal pod autoscaler
		if algoConfig.AutoScale {

		}

	}

	return nil

}

func (r *ReconcileEndpoint) checkForDeployment(listOptions *client.ListOptions) bool {

	deploymentList := &appsv1.DeploymentList{}
	ctx := context.TODO()
	err := r.client.List(ctx, listOptions, deploymentList)

	if err != nil && errors.IsNotFound(err) {
		return false
	} else if err != nil {
		return false
	}

	return len(deploymentList.Items) > 0

}

func (r *ReconcileEndpoint) createDeployment(deployment *appsv1.Deployment) error {

	if err := r.client.Create(context.TODO(), deployment); err != nil {
		log.WithValues("Data", deployment.Labels)
		log.Error(err, "Failed creating the algo deployment")
		return err
	}

	log.WithValues("Name", deployment.GetName())
	log.Info("Created deployment")

	return nil

}

func (r *ReconcileEndpoint) updateDeployment(deployment *appsv1.Deployment) error {

	if err := r.client.Update(context.TODO(), deployment); err != nil {
		log.WithValues("Data", deployment.Labels)
		log.Error(err, "Failed updating the algo deployment")
		return err
	}

	log.WithValues("Name", deployment.GetName())
	log.Info("Updated deployment")

	return nil

}

func (r *ReconcileEndpoint) createTopics(endpointSpec *algov1alpha1.EndpointSpec, request reconcile.Request) {

	for _, topicConfig := range endpointSpec.EndpointConfig.TopicConfigs {

		// Replace the endpoint username and name in the topic string
		topicName := strings.ToLower(strings.Replace(topicConfig.TopicName, "{endpointownerusername}", endpointSpec.EndpointConfig.EndpointOwnerUserName, -1))
		topicName = strings.ToLower(strings.Replace(topicName, "{endpointname}", endpointSpec.EndpointConfig.EndpointName, -1))

		log.WithValues("Topic", topicName)

		topicPartitions := 1
		if topicConfig.TopicAutoPartition {
			// Set the topic partitions based on the max destination instance count
			for _, pipe := range endpointSpec.EndpointConfig.Pipes {

				switch pipeType := pipe.PipeType; pipeType {
				case "Algo":

					// Match the Source Algo Pipe
					if pipe.SourceAlgoOwnerName == topicConfig.AlgoOwnerName &&
						pipe.SourceAlgoName == topicConfig.AlgoName &&
						pipe.SourceAlgoIndex == topicConfig.AlgoIndex &&
						pipe.SourceAlgoOutputName == topicConfig.AlgoOutputName {

						// Find the destination Algo
						for _, algoConfig := range endpointSpec.EndpointConfig.AlgoConfigs {

							if algoConfig.AlgoOwnerUserName == pipe.DestAlgoOwnerName &&
								algoConfig.AlgoName == pipe.DestAlgoName &&
								algoConfig.AlgoIndex == pipe.DestAlgoIndex {
								topicPartitions = max(int(algoConfig.MinInstances), topicPartitions)
								topicPartitions = max(int(algoConfig.Instances), topicPartitions)
							}

						}

					}

				case "DataSource":

					// Match the Data Source Pipe
					if pipe.PipelineDataSourceName == topicConfig.PipelineDataSourceName &&
						pipe.PipelineDataSourceIndex == topicConfig.PipelineDataSourceIndex {

						// Find the destination Algo
						for _, algoConfig := range endpointSpec.EndpointConfig.AlgoConfigs {

							if algoConfig.AlgoOwnerUserName == pipe.DestAlgoOwnerName &&
								algoConfig.AlgoName == pipe.DestAlgoName &&
								algoConfig.AlgoIndex == pipe.DestAlgoIndex {
								topicPartitions = max(int(algoConfig.MinInstances), topicPartitions)
								topicPartitions = max(int(algoConfig.Instances), topicPartitions)
							}

						}

					}

				case "EndpointConnector":

					// Match the Endpoint Connector Pipe
					if pipe.PipelineEndpointConnectorOutputName == topicConfig.EndpointConnectorOutputName {

						// Find the destination Algo
						for _, algoConfig := range endpointSpec.EndpointConfig.AlgoConfigs {

							if algoConfig.AlgoOwnerUserName == pipe.DestAlgoOwnerName &&
								algoConfig.AlgoName == pipe.DestAlgoName &&
								algoConfig.AlgoIndex == pipe.DestAlgoIndex {
								topicPartitions = max(int(algoConfig.MinInstances), topicPartitions)
								topicPartitions = max(int(algoConfig.Instances), topicPartitions)
							}

						}

					}

				}

			}

		} else {
			if topicConfig.TopicPartitions > 0 {
				topicPartitions = int(topicConfig.TopicPartitions)
			}
		}

		params := make(map[string]string)
		for _, topicParam := range topicConfig.TopicParams {
			params[topicParam.Name] = topicParam.Value
		}

		// check to see if topic already exists
		u := &unstructured.Unstructured{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: topicName, Namespace: request.NamespacedName.Namespace}, u)

		t := &unstructured.Unstructured{}
		t.Object = map[string]interface{}{
			"name":      topicName,
			"namespace": request.NamespacedName.Namespace,
			"spec": map[string]interface{}{
				"partitions": topicPartitions,
				"replicas":   int(topicConfig.TopicReplicationFactor),
				"config":     params,
			},
		}
		t.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "kafka.strimzi.io",
			Kind:    "KafkaTopic",
			Version: "v1alpha1",
		})

		if err != nil && errors.IsNotFound(err) {
			// Create the topic
			// Using a unstructured object to submit a strimzi topic creation.
			err := r.client.Create(context.TODO(), t)
			if err != nil {
				log.Error(err, "Failed creating topic")
			}
		} else if err != nil {
			log.Error(err, "Failed to check if Kafka topic exists.")
		} else {
			// Update the topic if changed
			// TODO: Check that the partition count did not go down (kafka doesn't support)

			if !reflect.DeepEqual(u, t) {
				err := r.client.Update(context.TODO(), t)
				if err != nil {
					log.Error(err, "Failed updating topic")
				}
			}

		}

	}

}

func createRunnerConfig(endpointSpec *algov1alpha1.EndpointSpec, algoConfig *v1alpha1.AlgoConfig) v1alpha1.RunnerConfig {

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

func createDeploymentSpec(cr *algov1alpha1.Endpoint, name string, labels map[string]string, algoConfig *v1alpha1.AlgoConfig, runnerConfig *v1alpha1.RunnerConfig, update bool) (*appsv1.Deployment, error) {

	// Set the image name
	var imageFullName string
	if algoConfig.ImageTag == "" || algoConfig.ImageTag == "latest" {
		imageFullName = fmt.Sprintf("%s:latest", algoConfig.ImageRepository)
	} else {
		imageFullName = fmt.Sprintf("%s:%s", algoConfig.ImageRepository, algoConfig.ImageTag)
	}

	// Configure the readiness and liveness
	handler := corev1.Handler{
		HTTPGet: &corev1.HTTPGetAction{
			Path: "/health",
			Port: intstr.FromInt(10080),
		},
	}

	readinessProbe := &corev1.Probe{
		Handler:             handler,
		InitialDelaySeconds: cr.Spec.ReadinessInitialDelaySeconds,
		TimeoutSeconds:      cr.Spec.ReadinessTimeoutSeconds,
		PeriodSeconds:       cr.Spec.ReadinessPeriodSeconds,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}
	livenessProbe := &corev1.Probe{
		Handler:             handler,
		InitialDelaySeconds: cr.Spec.LivenessInitialDelaySeconds,
		TimeoutSeconds:      cr.Spec.LivenessTimeoutSeconds,
		PeriodSeconds:       cr.Spec.LivenessPeriodSeconds,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}

	// nodeSelector := createSelector(request.Constraints)

	resources, resourceErr := createResources(algoConfig)

	if resourceErr != nil {
		return nil, resourceErr
	}

	var imagePullPolicy corev1.PullPolicy
	switch cr.Spec.ImagePullPolicy {
	case "Never":
		imagePullPolicy = corev1.PullNever
	case "IfNotPresent":
		imagePullPolicy = corev1.PullIfNotPresent
	default:
		imagePullPolicy = corev1.PullAlways
	}

	envVars := createEnvVars(cr, runnerConfig, algoConfig)

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
			RevisionHistoryLimit: int32p(10),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: nameMeta,
				Spec: corev1.PodSpec{
					// SecurityContext: &corev1.PodSecurityContext{
					//	FSGroup: int64p(1431),
					// },
					// NodeSelector: nodeSelector,
					Containers: []corev1.Container{
						{
							Name:            name,
							Image:           imageFullName,
							Command:         []string{"/algo-runner/algo-runner"},
							Env:             envVars,
							Resources:       *resources,
							ImagePullPolicy: imagePullPolicy,
							LivenessProbe:   livenessProbe,
							ReadinessProbe:  readinessProbe,
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
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "algo-runner-volume",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "algo-runner-pv-claim",
									ReadOnly:  true,
								},
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
		log.WithValues("Data", runnerConfig)
		log.Error(err, "Failed deserializing runner config")
	}

	// Append the algo instance name
	envVars = append(envVars, corev1.EnvVar{
		Name: "INSTANCE-NAME",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.name",
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
		Name:  "KAFKA-SERVERS",
		Value: cr.Spec.KafkaBrokers,
	})

	// Append the log topic
	envVars = append(envVars, corev1.EnvVar{
		Name:  "LOG-TOPIC",
		Value: cr.Spec.LogTopic,
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
	resources := &corev1.ResourceRequirements{
		Limits:   corev1.ResourceList{},
		Requests: corev1.ResourceList{},
	}

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

func max(x, y int) int {
	if x < y {
		return y
	}
	return x
}

func int32p(i int32) *int32 {
	return &i
}

func int64p(i int64) *int64 {
	return &i
}

func short(s string, i int) string {
	runes := []rune(s)
	if len(runes) > i {
		return string(runes[:i])
	}
	return s
}
