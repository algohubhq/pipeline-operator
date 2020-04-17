package reconciler

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"pipeline-operator/pkg/apis/algorun/v1beta1"
	algov1beta1 "pipeline-operator/pkg/apis/algorun/v1beta1"
	utils "pipeline-operator/pkg/utilities"

	"github.com/go-test/deep"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewHookReconciler returns a new HookReconciler
func NewHookReconciler(pipelineDeployment *algov1beta1.PipelineDeployment,
	allTopicConfigs map[string]*v1beta1.TopicConfigModel,
	request *reconcile.Request,
	client client.Client,
	scheme *runtime.Scheme,
	kafkaTLS bool) HookReconciler {
	return HookReconciler{
		pipelineDeployment: pipelineDeployment,
		allTopics:          allTopicConfigs,
		request:            request,
		client:             client,
		scheme:             scheme,
		kafkaTLS:           kafkaTLS,
	}
}

// HookReconciler reconciles an Hook object
type HookReconciler struct {
	pipelineDeployment *algov1beta1.PipelineDeployment
	allTopics          map[string]*v1beta1.TopicConfigModel
	request            *reconcile.Request
	client             client.Client
	scheme             *runtime.Scheme
	kafkaTLS           bool
}

// Reconcile creates or updates the hook deployment for the pipelineDeployment
func (hookReconciler *HookReconciler) Reconcile() error {

	pipelineDeployment := hookReconciler.pipelineDeployment

	hookLogger := log

	hookLogger.Info("Reconciling Hook")

	name := "pipe-depl-hook"

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

	kubeUtil := utils.NewKubeUtil(hookReconciler.client, hookReconciler.request)

	var hookName string
	existingDeployment, err := kubeUtil.CheckForDeployment(opts)
	if existingDeployment != nil {
		hookName = existingDeployment.GetName()
	}

	// Generate the k8s deployment
	hookDeployment, err := hookReconciler.createDeploymentSpec(name, labels, existingDeployment)
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

		hpaSpec, err := kubeUtil.CreateHpaSpec(hookName, labels, pipelineDeployment, pipelineDeployment.Spec.Hook.Autoscaling)
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
func (hookReconciler *HookReconciler) createDeploymentSpec(name string, labels map[string]string, existingDeployment *appsv1.Deployment) (*appsv1.Deployment, error) {

	// Convert map to slice of values.
	topics := []algov1beta1.TopicConfigModel{}
	for _, value := range hookReconciler.allTopics {
		topics = append(topics, *value)
	}

	pipelineDeployment := hookReconciler.pipelineDeployment
	pipelineSpec := hookReconciler.pipelineDeployment.Spec
	hookConfig := pipelineSpec.Hook
	hookConfig.DeploymentOwner = pipelineSpec.DeploymentOwner
	hookConfig.DeploymentName = pipelineSpec.DeploymentName
	hookConfig.PipelineOwner = pipelineSpec.PipelineOwner
	hookConfig.PipelineName = pipelineSpec.PipelineName
	hookConfig.WebHooks = pipelineSpec.Hook.WebHooks
	hookConfig.Pipes = pipelineSpec.Pipes
	hookConfig.Topics = topics

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

	kubeUtil := utils.NewKubeUtil(hookReconciler.client, hookReconciler.request)
	resources, resourceErr := kubeUtil.CreateResourceReqs(hookConfig.Resources)

	if resourceErr != nil {
		return nil, resourceErr
	}

	// Hook container
	hookContainer := corev1.Container{
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

func (hookReconciler *HookReconciler) createEnvVars(cr *algov1beta1.PipelineDeployment, hookConfig *v1beta1.HookSpec) []corev1.EnvVar {

	envVars := []corev1.EnvVar{}

	// serialize the runner config to json string
	hookConfigBytes, err := json.Marshal(hookConfig)
	if err != nil {
		log.Error(err, "Failed deserializing hook config")
	}

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

	// Append the required runner config
	envVars = append(envVars, corev1.EnvVar{
		Name:  "HOOK-RUNNER-CONFIG",
		Value: string(hookConfigBytes),
	})

	// Append the required kafka servers
	envVars = append(envVars, corev1.EnvVar{
		Name:  "KAFKA_BROKERS",
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

func (hookReconciler *HookReconciler) createSelector(constraints []string) map[string]string {
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
