package reconciler

import (
	"encoding/json"
	"fmt"
	utils "pipeline-operator/internal/utilities"
	"pipeline-operator/pkg/apis/algo/v1alpha1"
	algov1alpha1 "pipeline-operator/pkg/apis/algo/v1alpha1"
	"strings"

	"github.com/go-test/deep"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewEndpointReconciler returns a new EndpointReconciler
func NewEndpointReconciler(pipelineDeployment *algov1alpha1.PipelineDeployment,
	request *reconcile.Request,
	client client.Client) EndpointReconciler {
	return EndpointReconciler{
		pipelineDeployment: pipelineDeployment,
		request:            request,
		client:             client,
	}
}

// EndpointReconciler reconciles the endpoint container and mappings
type EndpointReconciler struct {
	pipelineDeployment *algov1alpha1.PipelineDeployment
	request            *reconcile.Request
	client             client.Client
}

func (endpointReconciler *EndpointReconciler) Reconcile() error {

	pipelineDeployment := endpointReconciler.pipelineDeployment
	request := endpointReconciler.request

	hookLogger := log

	hookLogger.Info("Reconciling Endpoint")

	name := "pipeline-deployment-endpoint"

	labels := map[string]string{
		"system":                  "algorun",
		"tier":                    "backend",
		"component":               "endpoint",
		"pipelinedeploymentowner": pipelineDeployment.Spec.PipelineSpec.DeploymentOwnerUserName,
		"pipelinedeployment":      pipelineDeployment.Spec.PipelineSpec.DeploymentName,
		"pipeline":                pipelineDeployment.Spec.PipelineSpec.PipelineName,
	}

	// Check to make sure the algo isn't already created
	listOptions := &client.ListOptions{}
	listOptions.SetLabelSelector(fmt.Sprintf("system=algorun, component=hook, pipelinedeploymentowner=%s, pipelinedeployment=%s",
		pipelineDeployment.Spec.PipelineSpec.DeploymentOwnerUserName,
		pipelineDeployment.Spec.PipelineSpec.DeploymentName))
	listOptions.InNamespace(request.NamespacedName.Namespace)

	deplUtil := utils.NewDeploymentUtil(endpointReconciler.client)

	existingDeployment, err := deplUtil.CheckForDeployment(listOptions)

	// Generate the k8s deployment
	hookDeployment, err := endpointReconciler.createDeploymentSpec(name, labels, existingDeployment)
	if err != nil {
		hookLogger.Error(err, "Failed to create hook deployment spec")
		return err
	}

	// Set PipelineDeployment instance as the owner and controller
	if err := controllerutil.SetControllerReference(pipelineDeployment, hookDeployment, hookReconciler.scheme); err != nil {
		return err
	}

	if existingDeployment == nil {
		err := deplUtil.CreateDeployment(hookDeployment)
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
			err := deplUtil.UpdateDeployment(hookDeployment)
			if err != nil {
				hookLogger.Error(err, "Failed to update hook deployment")
				return err
			}
		}
	}

	return nil

}

// CreateDeploymentSpec generates the k8s spec for the endpoint deployment
func (endpointReconciler *EndpointReconciler) createDeploymentSpec(name string, labels map[string]string, existingDeployment *appsv1.Deployment) (*appsv1.Deployment, error) {

	pipelineDeployment := endpointReconciler.pipelineDeployment
	endpointConfig := endpointReconciler.pipelineDeployment.Spec.PipelineSpec.EndpointConfig

	// Set the image name
	var imageName string
	if hookConfig.ImageTag == "" || hookConfig.ImageTag == "latest" {
		imageName = fmt.Sprintf("%s:latest", hookConfig.ImageRepository)
	} else {
		imageName = fmt.Sprintf("%s:%s", hookConfig.ImageRepository, hookConfig.ImageTag)
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

	var containers []corev1.Container

	hookCommand := []string{"/hook-runner/hook-runner"}
	hookEnvVars := hookReconciler.createEnvVars(pipelineDeployment, hookRunnerConfig)

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
	}
	containers = append(containers, hookContainer)

	// nodeSelector := createSelector(request.Constraints)

	// If this is an update, need to set the existing deployment name
	var nameMeta metav1.ObjectMeta
	if existingDeployment != nil {
		nameMeta = metav1.ObjectMeta{
			Namespace: pipelineDeployment.Namespace,
			Name:      existingDeployment.Name,
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

func (endpointReconciler *EndpointReconciler) createEnvVars(cr *algov1alpha1.PipelineDeployment, hookConfig *v1alpha1.HookRunnerConfig) []corev1.EnvVar {

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
		Name:  "HOOK-RUNNER-CONFIG",
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
