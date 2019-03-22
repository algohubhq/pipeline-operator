package endpoint

import (
	"context"
	"encoding/json"
	"endpoint-operator/pkg/apis/algo/v1alpha1/swagger"
	"fmt"
	"strconv"
	"strings"

	algov1alpha1 "endpoint-operator/pkg/apis/algo/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

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

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Endpoint
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
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Reconcile all algo deployments
	if result, err := r.reconcileAlgos(instance, request); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil

}

// reconcileAlgos creates or updates all algos for the endpoint
func (r *ReconcileEndpoint) reconcileAlgos(cr *algov1alpha1.Endpoint, request reconcile.Request) (reconcile.Result, error) {

	// Iterate the AlgoConfigs
	for _, algoConfig := range cr.Spec.AlgoConfigs {

		if !algoConfig.Applied {

			// Truncate the name of the deployment / pod just in case
			name := strings.TrimRight(short(algoConfig.AlgoName, 20), "-")

			// Generate the runnerconfig
			runnerConfig := createRunnerConfig(&cr.Spec, &algoConfig)

			// orchLog.Key = fmt.Sprintf("%s/%s", runnerConfig.EndpointOwnerUserName, runnerConfig.EndpointName)
			// orchLog.OrchestratorLogData.AlgoOwnerUserName = algoConfig.AlgoOwnerUserName
			// orchLog.OrchestratorLogData.AlgoName = algoConfig.AlgoName

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
			algoDeployment, err := createDeploymentSpec(cr, name, labels, &algoConfig, &runnerConfig, !deploymentExists)
			if err != nil {
				return reconcile.Result{}, err
				// orchLog.logOrch("Failed", fmt.Sprintf("Failed to create algo deployment for [%s/%s:%s]\nError: %s",
				// 	algoConfig.AlgoOwnerUserName,
				// 	algoConfig.AlgoName,
				// 	algoConfig.AlgoVersionTag,
				// 	err))
			}

			// Set Endpoint instance as the owner and controller
			if err := controllerutil.SetControllerReference(cr, algoDeployment, r.scheme); err != nil {
				return reconcile.Result{}, err
			}

			if !deploymentExists {
				r.createDeployment(algoDeployment)
			} else {
				r.updateDeployment(algoDeployment)
			}

			// TODO: Setup the horizontal pod autoscaler
			if algoConfig.AutoScale {

			}

		}

	}

	return reconcile.Result{}, nil

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

	return true

}

func (r *ReconcileEndpoint) createDeployment(deployment *appsv1.Deployment) error {

	// existingSecrets, err := getSecrets(clientset, functionNamespace, request.Secrets)
	// if err != nil {
	// 	// TODO: log the error
	// 	return
	// }

	if err := r.client.Create(context.TODO(), deployment); err != nil {
		// orchLog.logOrch("Failed", fmt.Sprintf("Failed creating the algo deployment [%s]\nError: %s",
		// 	deployment,
		// 	err))
		return err
	}

	log.Info("Created deployment - " + deployment.GetName())

	return nil

}

func (r *ReconcileEndpoint) updateDeployment(deployment *appsv1.Deployment) error {

	// existingSecrets, err := getSecrets(clientset, functionNamespace, request.Secrets)
	// if err != nil {
	// 	// TODO: log the error
	// 	return
	// }

	if err := r.client.Update(context.TODO(), deployment); err != nil {
		// orchLog.logOrch("Failed", fmt.Sprintf("Failed updating the algo deployment [%s]\nError: %s",
		// 	deployment,
		// 	err))
		return err
	}

	log.Info("Updated deployment - " + deployment.GetName())

	return nil

}

func createRunnerConfig(endpointSpec *algov1alpha1.EndpointSpec, algoConfig *swagger.AlgoConfig) swagger.RunnerConfig {

	runnerConfig := swagger.RunnerConfig{
		EndpointOwnerUserName: endpointSpec.EndpointOwnerUserName,
		EndpointName:          endpointSpec.EndpointName,
		PipelineOwnerUserName: endpointSpec.PipelineOwnerUserName,
		PipelineName:          endpointSpec.PipelineName,
		Pipes:                 endpointSpec.Pipes,
		TopicConfigs:          endpointSpec.TopicConfigs,
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

func createDeploymentSpec(cr *algov1alpha1.Endpoint, name string, labels map[string]string, algoConfig *swagger.AlgoConfig, runnerConfig *swagger.RunnerConfig, update bool) (*appsv1.Deployment, error) {

	// LEFT OFF HERE
	// Set the image name
	if algoConfig.ImageRepository

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

	envVars := createEnvVars(runnerConfig, algoConfig)

	// If this is an update, need to set the existing deployment name
	var nameMeta metav1.ObjectMeta
	if update {
		nameMeta = metav1.ObjectMeta{
			Name:   algoConfig.DeploymentName,
			Labels: labels,
			// Annotations: annotations,
		}
	} else {
		nameMeta = metav1.ObjectMeta{
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
							Image:           algoConfig.ImageRepository,
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

func createEnvVars(runnerConfig *swagger.RunnerConfig, algoConfig *swagger.AlgoConfig) []corev1.EnvVar {

	// Create the base log message
	orchLog := logMessage{
		LogMessageType: "Orchestrator",
		Status:         "Started",
	}

	envVars := []corev1.EnvVar{}

	// serialize the runner config to json string
	runnerConfigBytes, err := json.Marshal(runnerConfig)
	if err != nil {
		orchLog.logOrch("Failed", fmt.Sprintf("Failed deserializing runner config [%+v]\nError: %s",
			runnerConfig,
			err))
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
		Value: *kafkaBrokers,
	})

	// Append the log topic
	envVars = append(envVars, corev1.EnvVar{
		Name:  "LOG-TOPIC",
		Value: config.Settings.LogTopic,
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

	log.Println(constraints)
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

func createResources(algoConfig *swagger.AlgoConfig) (*corev1.ResourceRequirements, error) {
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
