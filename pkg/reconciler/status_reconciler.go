package reconciler

import (
	"context"
	"fmt"
	"pipeline-operator/pkg/apis/algorun/v1beta1"
	algov1beta1 "pipeline-operator/pkg/apis/algorun/v1beta1"
	"strconv"
	"strings"
	"time"

	"github.com/go-test/deep"

	utils "pipeline-operator/pkg/utilities"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewStatusReconciler returns a new StatusReconciler
func NewStatusReconciler(pipelineDeployment *algov1beta1.PipelineDeployment,
	request *reconcile.Request,
	client client.Client,
	scheme *runtime.Scheme) StatusReconciler {
	return StatusReconciler{
		pipelineDeployment: pipelineDeployment,
		request:            request,
		client:             client,
		scheme:             scheme,
	}
}

// StatusReconciler reconciles all Pipeline deployment statuses
type StatusReconciler struct {
	pipelineDeployment *algov1beta1.PipelineDeployment
	request            *reconcile.Request
	client             client.Client
	scheme             *runtime.Scheme
}

// Reconcile creates or updates the hook deployment for the pipelineDeployment
func (r *StatusReconciler) Reconcile() error {

	logData := map[string]interface{}{
		"PipelineDeployment.Namespace": r.pipelineDeployment.Spec.DeploymentNamespace,
		"PipelineDeployment.Name":      r.pipelineDeployment.Spec.DeploymentName,
	}
	reqLogger := log.WithValues("data", logData)

	pipelineDeploymentStatus, err := r.getStatus(r.pipelineDeployment, r.request)
	if err != nil {
		reqLogger.Error(err, "Failed to get PipelineDeployment status.")
		return err
	}

	notifMessages := []*algov1beta1.NotifMessage{}

	if r.pipelineDeployment.Status.Status != pipelineDeploymentStatus.Status {
		r.pipelineDeployment.Status.Status = pipelineDeploymentStatus.Status

		loglevel := v1beta1.LOGLEVELS_INFO
		notifType := v1beta1.NOTIFTYPES_PIPELINE_DEPLOYMENT_STATUS
		notifMessage := &algov1beta1.NotifMessage{
			MessageTimestamp: time.Now(),
			Level:            &loglevel,
			Type:             &notifType,
			DeploymentStatusMessage: &algov1beta1.DeploymentStatusMessage{
				DeploymentOwner: r.pipelineDeployment.Spec.DeploymentOwner,
				DeploymentName:  r.pipelineDeployment.Spec.DeploymentName,
				Status:          r.pipelineDeployment.Status.Status,
			},
		}

		notifMessages = append(notifMessages, notifMessage)

	}

	// Iterate the existing deployment statuses and update if changed
	for _, deplStatus := range r.pipelineDeployment.Status.ComponentStatuses {
		for _, newDeplStatus := range pipelineDeploymentStatus.ComponentStatuses {
			if newDeplStatus.DeploymentName == deplStatus.DeploymentName {

				if diff := deep.Equal(deplStatus, newDeplStatus); diff != nil {
					deplStatus = newDeplStatus
					reqLogger.Info("Deployment Status Differences", "Differences", diff)
					loglevel := v1beta1.LOGLEVELS_INFO
					notifType := v1beta1.NOTIFTYPES_PIPELINE_DEPLOYMENT
					notifMessage := &algov1beta1.NotifMessage{
						MessageTimestamp: time.Now(),
						Level:            &loglevel,
						Type:             &notifType,
						DeploymentStatusMessage: &algov1beta1.DeploymentStatusMessage{
							DeploymentOwner: r.pipelineDeployment.Spec.DeploymentOwner,
							DeploymentName:  r.pipelineDeployment.Spec.DeploymentName,
							Status:          r.pipelineDeployment.Status.Status,
						},
					}

					notifMessages = append(notifMessages, notifMessage)
				}

			}
		}
	}

	// Iterate the existing pod statuses and update if changed
	for _, podStatus := range r.pipelineDeployment.Status.PodStatuses {
		for _, newPodStatus := range pipelineDeploymentStatus.PodStatuses {
			if newPodStatus.PodName == podStatus.PodName {

				if diff := deep.Equal(podStatus, newPodStatus); diff != nil {
					podStatus = newPodStatus

					// reqLogger.Info("Deployment Pod Status Differences", "Differences", diff)
					loglevel := v1beta1.LOGLEVELS_INFO
					notifType := v1beta1.NOTIFTYPES_PIPELINE_DEPLOYMENT_POD
					notifMessage := &algov1beta1.NotifMessage{
						MessageTimestamp: time.Now(),
						Level:            &loglevel,
						Type:             &notifType,
						DeploymentStatusMessage: &algov1beta1.DeploymentStatusMessage{
							DeploymentOwner: r.pipelineDeployment.Spec.DeploymentOwner,
							DeploymentName:  r.pipelineDeployment.Spec.DeploymentName,
							Status:          r.pipelineDeployment.Status.Status,
						},
					}

					notifMessages = append(notifMessages, notifMessage)
				}

			}
		}
	}

	if diff := deep.Equal(r.pipelineDeployment.Status, *pipelineDeploymentStatus); diff != nil {
		// reqLogger.Info("Pipeline Deployment Status Differences", "Differences", diff)

		r.pipelineDeployment.Status = *pipelineDeploymentStatus

		err = r.client.Status().Update(context.TODO(), r.pipelineDeployment)

		if err != nil {
			reqLogger.Error(err, "Failed to update PipelineDeployment status.")
			return err
		}

	}

	// Send all notifications
	if len(notifMessages) > 0 {
		utils.NotifyAll(notifMessages)
	}

	return nil

}

func (r *StatusReconciler) getStatus(cr *algov1beta1.PipelineDeployment, request *reconcile.Request) (*algov1beta1.PipelineDeploymentStatus, error) {

	pipelineDeploymentStatus := algov1beta1.PipelineDeploymentStatus{
		DeploymentOwner: cr.Spec.DeploymentOwner,
		DeploymentName:  cr.Spec.DeploymentName,
	}

	logData := map[string]interface{}{
		"PipelineDeployment.Namespace": r.pipelineDeployment.Spec.DeploymentNamespace,
		"PipelineDeployment.Name":      r.pipelineDeployment.Spec.DeploymentName,
	}
	reqLogger := log.WithValues("data", logData)

	podStatuses, err := r.getPodStatuses(cr, request)
	if err != nil {
		reqLogger.Error(err, "Failed to get pod statuses.")
		return nil, err
	}

	pipelineDeploymentStatus.PodStatuses = podStatuses

	deploymentStatuses, err := r.getDeploymentStatuses(cr, request, podStatuses)
	if err != nil {
		reqLogger.Error(err, "Failed to get deployment statuses.")
		return nil, err
	}

	sfStatuses, err := r.getStatefulSetStatuses(cr, request, podStatuses)
	if err != nil {
		reqLogger.Error(err, "Failed to get StatefulSet statuses.")
		return nil, err
	}

	pipelineDeploymentStatus.ComponentStatuses = append(deploymentStatuses, sfStatuses...)

	// Calculate pipelineDeployment status
	pipelineDeploymentStatusString, err := r.calculateStatus(cr, pipelineDeploymentStatus.ComponentStatuses, pipelineDeploymentStatus.PodStatuses)
	if err != nil {
		reqLogger.Error(err, "Failed to calculate PipelineDeployment status.")
	}

	pipelineDeploymentStatus.Status = pipelineDeploymentStatusString

	return &pipelineDeploymentStatus, nil

}

func (r *StatusReconciler) getDeploymentStatuses(cr *algov1beta1.PipelineDeployment, request *reconcile.Request, podStatuses []algov1beta1.ComponentPodStatus) (
	componentStatuses []algov1beta1.ComponentStatus,
	err error) {

	// Watch all deployments for components of this pipeline
	opts := []client.ListOption{
		client.InNamespace(r.pipelineDeployment.Spec.DeploymentNamespace),
		client.MatchingLabels{
			"app.kubernetes.io/part-of":    "algo.run",
			"app.kubernetes.io/managed-by": "pipeline-operator",
			"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", cr.Spec.DeploymentOwner,
				cr.Spec.DeploymentName),
		},
	}

	deploymentList := &appsv1.DeploymentList{}
	ctx := context.TODO()
	err = r.client.List(ctx, deploymentList, opts...)

	if err != nil {
		log.Error(err, "Failed getting deployment list to determine status")
		return nil, err
	}

	componentStatuses = make([]algov1beta1.ComponentStatus, 0)

	for _, deployment := range deploymentList.Items {

		index, _ := strconv.Atoi(deployment.Labels["algo.run/index"])
		// Create the deployment data
		componentStatus := algov1beta1.ComponentStatus{
			Index:            int32(index),
			DeploymentName:   deployment.GetName(),
			Desired:          deployment.Status.AvailableReplicas + deployment.Status.UnavailableReplicas,
			Current:          deployment.Status.Replicas,
			UpToDate:         deployment.Status.UpdatedReplicas,
			Available:        deployment.Status.AvailableReplicas,
			Ready:            deployment.Status.ReadyReplicas,
			CreatedTimestamp: deployment.CreationTimestamp.String(),
		}

		switch deployment.Labels["app.kubernetes.io/component"] {
		case "algo":
			compType := v1beta1.COMPONENTTYPES_ALGO
			componentStatus.ComponentType = &compType
			componentStatus.Name = strings.Replace(deployment.Labels["algo.run/algo"], ".", "/", 1)
			componentStatus.VersionTag = deployment.Labels["algo.run/algo-version"]
		case "dataconnector":
			compType := v1beta1.COMPONENTTYPES_DATA_CONNECTOR
			componentStatus.ComponentType = &compType
			componentStatus.Name = strings.Replace(deployment.Labels["algo.run/dataconnector"], ".", "/", 1)
			componentStatus.VersionTag = deployment.Labels["algo.run/dataconnector-version"]
		case "hook":
			compType := v1beta1.COMPONENTTYPES_HOOK
			componentStatus.ComponentType = &compType
			componentStatus.Name = deployment.Labels["app.kubernetes.io/component"]
		}

		if componentStatus.Ready < componentStatus.Desired {
			// find the pod associated with this statefulset
			for _, podStatus := range podStatuses {
				if podStatus.Name == componentStatus.Name && podStatus.Index == componentStatus.Index {
					if podStatus.Status == "CrashLoopBackOff" {
						componentStatus.Status = "Error"
					} else {
						componentStatus.Status = "Progressing"
					}
				}
			}
		} else if componentStatus.Ready == componentStatus.Desired {
			componentStatus.Status = "Deployed"
		}

		componentStatuses = append(componentStatuses, componentStatus)

	}

	return componentStatuses, nil

}

func (r *StatusReconciler) getStatefulSetStatuses(cr *algov1beta1.PipelineDeployment, request *reconcile.Request, podStatuses []algov1beta1.ComponentPodStatus) (
	componentStatuses []algov1beta1.ComponentStatus,
	err error) {

	// Watch all deployments for components of this pipeline
	opts := []client.ListOption{
		client.InNamespace(r.pipelineDeployment.Spec.DeploymentNamespace),
		client.MatchingLabels{
			"app.kubernetes.io/part-of":    "algo.run",
			"app.kubernetes.io/managed-by": "pipeline-operator",
			"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", cr.Spec.DeploymentOwner,
				cr.Spec.DeploymentName),
		},
	}

	sfList := &appsv1.StatefulSetList{}
	ctx := context.TODO()
	err = r.client.List(ctx, sfList, opts...)

	if err != nil {
		log.Error(err, "Failed getting statefulset list to determine status")
		return nil, err
	}

	for _, sf := range sfList.Items {

		index, _ := strconv.Atoi(sf.Labels["algo.run/index"])
		// Create the deployment data
		componentStatus := algov1beta1.ComponentStatus{
			Index:            int32(index),
			DeploymentName:   sf.GetName(),
			Desired:          *sf.Spec.Replicas,
			Current:          sf.Status.CurrentReplicas,
			UpToDate:         sf.Status.UpdatedReplicas,
			Available:        sf.Status.CurrentReplicas,
			Ready:            sf.Status.ReadyReplicas,
			CreatedTimestamp: sf.CreationTimestamp.String(),
		}

		switch sf.Labels["app.kubernetes.io/component"] {
		case "endpoint":
			compType := v1beta1.COMPONENTTYPES_ENDPOINT
			componentStatus.ComponentType = &compType
			componentStatus.Name = sf.Labels["app.kubernetes.io/component"]
		}

		if componentStatus.Ready < componentStatus.Desired {
			// find the pod associated with this statefulset
			for _, podStatus := range podStatuses {
				if podStatus.Name == componentStatus.Name && podStatus.Index == componentStatus.Index {
					if podStatus.Status == "CrashLoopBackOff" {
						componentStatus.Status = "Error"
					} else {
						componentStatus.Status = "Progressing"
					}
				}
			}
		} else if componentStatus.Ready == componentStatus.Desired {
			componentStatus.Status = "Deployed"
		}

		componentStatuses = append(componentStatuses, componentStatus)

	}

	return componentStatuses, nil

}

func (r *StatusReconciler) getPodStatuses(cr *algov1beta1.PipelineDeployment, request *reconcile.Request) ([]algov1beta1.ComponentPodStatus, error) {

	// Get all algo pods for this pipelineDeployment
	opts := []client.ListOption{
		client.InNamespace(r.pipelineDeployment.Spec.DeploymentNamespace),
		client.MatchingLabels{
			"app.kubernetes.io/part-of":    "algo.run",
			"app.kubernetes.io/managed-by": "pipeline-operator",
			"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", cr.Spec.DeploymentOwner,
				cr.Spec.DeploymentName),
		},
	}

	podList := &corev1.PodList{}
	ctx := context.TODO()
	err := r.client.List(ctx, podList, opts...)

	if err != nil {
		log.Error(err, "Failed getting pod list to determine status")
		return nil, err
	}

	podStatuses := make([]algov1beta1.ComponentPodStatus, 0)

	for _, pod := range podList.Items {

		index, _ := strconv.Atoi(pod.Labels["algo.run/index"])
		var status string
		var restarts int32
		if pod.Status.ContainerStatuses != nil {
			restarts = pod.Status.ContainerStatuses[0].RestartCount
		}

		if pod.Status.Phase == "Pending" {
			status = "Pending"
		} else if pod.Status.ContainerStatuses[0].State.Terminated != nil {
			status = pod.Status.ContainerStatuses[0].State.Terminated.Reason
		} else if pod.Status.ContainerStatuses[0].State.Waiting != nil {
			status = pod.Status.ContainerStatuses[0].State.Waiting.Reason
		} else if pod.Status.ContainerStatuses[0].State.Running != nil {
			status = "Running"
		}

		// Create the pod status data
		podStatus := algov1beta1.ComponentPodStatus{
			Index:             int32(index),
			PodName:           pod.GetName(),
			Status:            status,
			Restarts:          restarts,
			CreatedTimestamp:  pod.CreationTimestamp.String(),
			Ip:                pod.Status.PodIP,
			Node:              pod.Spec.NodeName,
			ContainerStatuses: append([]corev1.ContainerStatus(nil), pod.Status.ContainerStatuses...),
		}

		switch pod.Labels["app.kubernetes.io/component"] {
		case "algo":
			podStatus.ComponentType = "Algo"
			podStatus.Name = strings.Replace(pod.Labels["algo.run/algo"], ".", "/", 1)
			podStatus.VersionTag = pod.Labels["algo.run/algo-version"]
		case "dataconnector":
			podStatus.ComponentType = "DataConnector"
			podStatus.Name = strings.Replace(pod.Labels["algo.run/dataconnector"], ".", "/", 1)
			podStatus.VersionTag = pod.Labels["algo.run/dataconnector-version"]
		case "endpoint":
			podStatus.ComponentType = "Endpoint"
			podStatus.Name = pod.Labels["app.kubernetes.io/component"]
		case "hook":
			podStatus.ComponentType = "Hook"
			podStatus.Name = pod.Labels["app.kubernetes.io/component"]
		}

		podStatuses = append(podStatuses, podStatus)

	}

	return podStatuses, nil

}

func (r *StatusReconciler) calculateStatus(cr *algov1beta1.PipelineDeployment,
	componentStatuses []algov1beta1.ComponentStatus,
	podStatuses []algov1beta1.ComponentPodStatus) (string, error) {

	var unreadyDeployments int32

	componentCount := len(cr.Spec.Algos)
	componentCount = componentCount + len(cr.Spec.DataConnectors)
	if &cr.Spec.Endpoint != nil {
		componentCount = componentCount + 1
	}
	if &cr.Spec.Hook != nil && len(cr.Spec.Hook.WebHooks) > 0 {
		componentCount = componentCount + 1
	}

	totalCount := len(componentStatuses)

	// iterate the algo deployments for any unread
	for _, deployment := range componentStatuses {
		if deployment.Ready < deployment.Desired {
			unreadyDeployments++
		}
	}

	if unreadyDeployments > 0 {
		// determine if there is a pod crash loop
		for _, podStatus := range podStatuses {
			if podStatus.Status == "CrashLoopBackOff" {
				return "Error", nil
			}
		}
		return "Progressing", nil
	} else if totalCount == componentCount {
		return "Deployed", nil
	} else if totalCount < componentCount {
		return "Progressing", nil
	}

	return "Terminated", nil

}
