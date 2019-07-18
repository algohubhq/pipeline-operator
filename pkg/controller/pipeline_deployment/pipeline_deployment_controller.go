package pipeline_deployment

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/go-test/deep"

	recon "pipeline-operator/internal/reconciler"
	utils "pipeline-operator/internal/utilities"
	"pipeline-operator/pkg/apis/algo/v1alpha1"
	algov1alpha1 "pipeline-operator/pkg/apis/algo/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_pipeline_deployment")

// Add creates a new PipelineDeployment Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcilePipelineDeployment{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("pipeline-deployment-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource PipelineDeployment
	err = c.Watch(&source.Kind{Type: &algov1alpha1.PipelineDeployment{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner PipelineDeployment
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &algov1alpha1.PipelineDeployment{},
	})
	if err != nil {
		return err
	}
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &algov1alpha1.PipelineDeployment{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcilePipelineDeployment{}

// ReconcilePipelineDeployment reconciles a PipelineDeployment object
type ReconcilePipelineDeployment struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a PipelineDeployment object and makes changes based on the state read
// and what is in the PipelineDeployment.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcilePipelineDeployment) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	logData := map[string]interface{}{
		"Request.Namespace": request.Namespace,
		"Request.Name":      request.Name,
	}
	reqLogger := log.WithValues("data", logData)
	reqLogger.Info("Reconciling PipelineDeployment")

	// Fetch the PipelineDeployment instance
	instance := &algov1alpha1.PipelineDeployment{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {

		r.updateMetrics(&request)

		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("PipelineDeployment resource not found. Ignoring since object must be deleted.")
			// Sending a notification that an pipelineDeployment was deleted. Just not sure which one!
			notifMessage := &v1alpha1.NotifMessage{
				MessageTimestamp: time.Now(),
				Level:            "Info",
				Type_:            "PipelineDeploymentDeleted",
			}
			utils.Notify(notifMessage)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Info("Error reading the PipelineDeployment Instance object - requeuing the request")
		return reconcile.Result{}, err
	}

	// Check if the APP CR was marked to be deleted
	isPipelineDeploymentMarkedToBeDeleted := instance.GetDeletionTimestamp() != nil
	if isPipelineDeploymentMarkedToBeDeleted {

		// This pipelineDeployment is queued for deletion.

		// Update finalizer to allow delete CR
		instance.SetFinalizers(nil)

		// Update CR
		err := r.client.Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}

		// TODO: Send the delete notification

		return reconcile.Result{}, nil
	}

	// Add finalizer for this CR
	if err := r.addFinalizer(instance); err != nil {
		return reconcile.Result{}, err
	}

	var wg sync.WaitGroup

	// Create / update the kafka topics
	reqLogger.Info("Reconciling Kakfa Topics")
	// Iterate the topics
	for _, topicConfig := range instance.Spec.PipelineSpec.TopicConfigs {
		wg.Add(1)
		go func(currentTopicConfig algov1alpha1.TopicConfigModel) {
			topicReconciler := recon.NewTopicReconciler(instance, &currentTopicConfig, &request, r.client, r.scheme)
			topicReconciler.Reconcile()
			wg.Done()
		}(topicConfig)
	}

	// Reconcile all algo deployments
	reqLogger.Info("Reconciling Algos")
	// Iterate the AlgoConfigs
	for _, algoConfig := range instance.Spec.PipelineSpec.AlgoConfigs {
		wg.Add(1)
		go func(currentAlgoConfig algov1alpha1.AlgoConfig) {
			algoReconciler := recon.NewAlgoReconciler(instance, &currentAlgoConfig, &request, r.client, r.scheme)
			err = algoReconciler.Reconcile()
			if err != nil {
				reqLogger.Error(err, "Error in AlgoConfig reconcile loop.")
			}
			wg.Done()
		}(algoConfig)
	}

	// Reconcile the algo metrics service
	reqLogger.Info("Reconciling Algo Metrics Service")
	wg.Add(1)
	go func() {
		algoReconciler := recon.NewAlgoReconciler(instance, nil, &request, r.client, r.scheme)
		algoReconciler.ReconcileService()
		wg.Done()
	}()

	// Reconcile all data connectors
	reqLogger.Info("Reconciling Data Connectors")
	// Iterate the DataConnectors
	for _, dcConfig := range instance.Spec.PipelineSpec.DataConnectorConfigs {
		wg.Add(1)
		go func(currentDcConfig algov1alpha1.DataConnectorConfig) {
			dcReconciler := recon.NewDataConnectorReconciler(instance, &currentDcConfig, &request, r.client, r.scheme)
			err = dcReconciler.Reconcile()
			if err != nil {
				reqLogger.Error(err, "Error in DataConnectorConfigs reconcile loop.")
			}
			wg.Done()
		}(dcConfig)
	}

	// Reconcile hook container
	reqLogger.Info("Reconciling Hooks")
	wg.Add(1)
	go func(pipelineDeployment *algov1alpha1.PipelineDeployment) {
		hookReconciler := recon.NewHookReconciler(instance, &request, r.client, r.scheme)
		err = hookReconciler.Reconcile()
		if err != nil {
			reqLogger.Error(err, "Error in Hook reconcile.")
		}
		wg.Done()
	}(instance)

	// Wait for algo, data connector and topic reconciliation to complete
	wg.Wait()

	r.updateMetrics(&request)

	pipelineDeploymentStatus, err := r.getStatus(instance, request)
	if err != nil {
		reqLogger.Error(err, "Failed to get PipelineDeployment status.")
		return reconcile.Result{}, err
	}

	statusChanged := false

	if instance.Status.Status != pipelineDeploymentStatus.Status {
		instance.Status.Status = pipelineDeploymentStatus.Status
		statusChanged = true

		notifMessage := &v1alpha1.NotifMessage{
			MessageTimestamp: time.Now(),
			Level:            "Info",
			Type_:            "PipelineDeploymentStatus",
			DeploymentStatusMessage: &v1alpha1.DeploymentStatusMessage{
				DeploymentOwnerUserName: instance.Spec.PipelineSpec.DeploymentOwnerUserName,
				DeploymentName:          instance.Spec.PipelineSpec.DeploymentName,
				Status:                  instance.Status.Status,
			},
		}

		utils.Notify(notifMessage)
	}

	// Iterate the existing deployment statuses and update if changed
	for _, deplStatus := range instance.Status.AlgoDeploymentStatuses {
		for _, newDeplStatus := range pipelineDeploymentStatus.AlgoDeploymentStatuses {
			if newDeplStatus.Name == deplStatus.Name {

				if diff := deep.Equal(deplStatus, newDeplStatus); diff != nil {
					deplStatus = newDeplStatus
					statusChanged = true
					// reqLogger.Info("Differences", "Differences", diff)
					notifMessage := &v1alpha1.NotifMessage{
						MessageTimestamp: time.Now(),
						Level:            "Info",
						Type_:            "PipelineDeployment",
						DeploymentStatusMessage: &v1alpha1.DeploymentStatusMessage{
							DeploymentOwnerUserName: instance.Spec.PipelineSpec.DeploymentOwnerUserName,
							DeploymentName:          instance.Spec.PipelineSpec.DeploymentName,
							Status:                  instance.Status.Status,
						},
					}

					utils.Notify(notifMessage)
				}

			}
		}
	}

	// Iterate the existing pod statuses and update if changed
	for _, podStatus := range instance.Status.AlgoPodStatuses {
		for _, newPodStatus := range pipelineDeploymentStatus.AlgoPodStatuses {
			if newPodStatus.Name == podStatus.Name {

				if diff := deep.Equal(podStatus, newPodStatus); diff != nil {
					podStatus = newPodStatus
					statusChanged = true
					// reqLogger.Info("Differences", "Differences", diff)
					notifMessage := &v1alpha1.NotifMessage{
						MessageTimestamp: time.Now(),
						Level:            "Info",
						Type_:            "PipelineDeploymentPod",
						DeploymentStatusMessage: &v1alpha1.DeploymentStatusMessage{
							DeploymentOwnerUserName: instance.Spec.PipelineSpec.DeploymentOwnerUserName,
							DeploymentName:          instance.Spec.PipelineSpec.DeploymentName,
							Status:                  instance.Status.Status,
						},
					}

					utils.Notify(notifMessage)
				}

			}
		}
	}

	if statusChanged {
		instance.Status = *pipelineDeploymentStatus

		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update PipelineDeployment status.")
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil

}

//addFinalizer will add this attribute to the PipelineDeployment CR
func (r *ReconcilePipelineDeployment) addFinalizer(pipelineDeployment *algov1alpha1.PipelineDeployment) error {
	if len(pipelineDeployment.GetFinalizers()) < 1 && pipelineDeployment.GetDeletionTimestamp() == nil {
		log.Info("Adding Finalizer for the PipelineDeployment")
		pipelineDeployment.SetFinalizers([]string{"finalizer.pipelineDeployment.algo.run"})

		// Update CR
		err := r.client.Update(context.TODO(), pipelineDeployment)
		if err != nil {
			log.Error(err, "Failed to update PipelineDeployment with finalizer")
			return err
		}
	}
	return nil
}

func (r *ReconcilePipelineDeployment) getStatus(cr *algov1alpha1.PipelineDeployment, request reconcile.Request) (*algov1alpha1.PipelineDeploymentStatus, error) {

	pipelineDeploymentStatus := algov1alpha1.PipelineDeploymentStatus{
		DeploymentOwnerUserName: cr.Spec.PipelineSpec.DeploymentOwnerUserName,
		DeploymentName:          cr.Spec.PipelineSpec.DeploymentName,
	}

	logData := map[string]interface{}{
		"Request.Namespace": request.Namespace,
		"Request.Name":      request.Name,
	}
	reqLogger := log.WithValues("data", logData)

	deploymentStatuses, err := r.getDeploymentStatuses(cr, request)
	if err != nil {
		reqLogger.Error(err, "Failed to get deployment statuses.")
		return nil, err
	}

	pipelineDeploymentStatus.AlgoDeploymentStatuses = deploymentStatuses

	// Calculate pipelineDeployment status
	pipelineDeploymentStatusString, err := calculateStatus(cr, &deploymentStatuses)
	if err != nil {
		reqLogger.Error(err, "Failed to calculate PipelineDeployment status.")
	}

	pipelineDeploymentStatus.Status = pipelineDeploymentStatusString

	podStatuses, err := r.getPodStatuses(cr, request)
	if err != nil {
		reqLogger.Error(err, "Failed to get pod statuses.")
		return nil, err
	}

	pipelineDeploymentStatus.AlgoPodStatuses = podStatuses

	return &pipelineDeploymentStatus, nil

}

func (r *ReconcilePipelineDeployment) getDeploymentStatuses(cr *algov1alpha1.PipelineDeployment, request reconcile.Request) ([]algov1alpha1.AlgoDeploymentStatus, error) {

	// Watch all algo deployments
	listOptions := &client.ListOptions{}
	listOptions.SetLabelSelector(fmt.Sprintf("system=algorun, component=algo, pipelinedeploymentowner=%s, pipelinedeployment=%s",
		cr.Spec.PipelineSpec.DeploymentOwnerUserName,
		cr.Spec.PipelineSpec.DeploymentName))
	listOptions.InNamespace(request.NamespacedName.Namespace)

	deploymentList := &appsv1.DeploymentList{}
	ctx := context.TODO()
	err := r.client.List(ctx, listOptions, deploymentList)

	if err != nil {
		log.Error(err, "Failed getting deployment list to determine status")
		return nil, err
	}

	deploymentStatuses := make([]algov1alpha1.AlgoDeploymentStatus, 0)

	for _, deployment := range deploymentList.Items {
		index, _ := strconv.Atoi(deployment.Labels["algoindex"])

		// Create the deployment data
		deploymentStatus := algov1alpha1.AlgoDeploymentStatus{
			AlgoOwnerName:    deployment.Labels["algoowner"],
			AlgoName:         deployment.Labels["algo"],
			AlgoVersionTag:   deployment.Labels["algoversion"],
			AlgoIndex:        int32(index),
			Name:             deployment.GetName(),
			Desired:          deployment.Status.AvailableReplicas + deployment.Status.UnavailableReplicas,
			Current:          deployment.Status.Replicas,
			UpToDate:         deployment.Status.UpdatedReplicas,
			Available:        deployment.Status.AvailableReplicas,
			CreatedTimestamp: deployment.CreationTimestamp.String(),
		}

		deploymentStatuses = append(deploymentStatuses, deploymentStatus)
	}

	return deploymentStatuses, nil

}

func (r *ReconcilePipelineDeployment) getPodStatuses(cr *algov1alpha1.PipelineDeployment, request reconcile.Request) ([]algov1alpha1.AlgoPodStatus, error) {

	// Get all algo pods for this pipelineDeployment
	listOptions := &client.ListOptions{}
	listOptions.SetLabelSelector(fmt.Sprintf("system=algorun, component=algo, pipelinedeploymentowner=%s, pipelinedeployment=%s",
		cr.Spec.PipelineSpec.DeploymentOwnerUserName,
		cr.Spec.PipelineSpec.DeploymentName))
	listOptions.InNamespace(request.NamespacedName.Namespace)

	podList := &corev1.PodList{}
	ctx := context.TODO()
	err := r.client.List(ctx, listOptions, podList)

	if err != nil {
		log.Error(err, "Failed getting pod list to determine status")
		return nil, err
	}

	podStatuses := make([]algov1alpha1.AlgoPodStatus, 0)

	for _, pod := range podList.Items {

		var podStatus string
		var restarts int32

		if pod.Status.Phase == "Pending" {
			podStatus = "Pending"
		} else if pod.Status.ContainerStatuses[0].State.Terminated != nil {
			podStatus = pod.Status.ContainerStatuses[0].State.Terminated.Reason
		} else if pod.Status.ContainerStatuses[0].State.Waiting != nil {
			podStatus = pod.Status.ContainerStatuses[0].State.Waiting.Reason
		} else if pod.Status.ContainerStatuses[0].State.Running != nil {
			podStatus = "Running"
		}

		index, _ := strconv.Atoi(pod.Labels["algoindex"])
		// Create the pod status data
		algoPodStatus := algov1alpha1.AlgoPodStatus{
			AlgoOwnerName:     pod.Labels["algoowner"],
			AlgoName:          pod.Labels["algo"],
			AlgoVersionTag:    pod.Labels["algoversion"],
			AlgoIndex:         int32(index),
			Name:              pod.GetName(),
			Status:            podStatus,
			Restarts:          restarts,
			CreatedTimestamp:  pod.CreationTimestamp.String(),
			Ip:                pod.Status.PodIP,
			Node:              pod.Spec.NodeName,
			ContainerStatuses: append([]corev1.ContainerStatus(nil), pod.Status.ContainerStatuses...),
		}

		podStatuses = append(podStatuses, algoPodStatus)

	}

	return podStatuses, nil

}

func calculateStatus(cr *algov1alpha1.PipelineDeployment, deploymentStatuses *[]algov1alpha1.AlgoDeploymentStatus) (string, error) {

	var unreadyDeployments int
	algoCount := len(cr.Spec.PipelineSpec.AlgoConfigs)
	deploymentCount := len(*deploymentStatuses)

	// iterate the deployments for any unready
	if deploymentCount > 0 {
		for _, deployment := range *deploymentStatuses {
			if deployment.Ready < deployment.Desired {
				unreadyDeployments++
			}
		}

		if unreadyDeployments > 0 {
			return "Updating", nil
		} else if deploymentCount == algoCount {
			return "Started", nil
		} else if deploymentCount < algoCount {
			return "Updating", nil
		}
	}

	return "Stopped", nil

}

func (r *ReconcilePipelineDeployment) updateMetrics(request *reconcile.Request) error {

	pipelineDeploymentCount, err := r.getPipelineDeploymentCount(request)
	utils.PipelineDeploymentCountGuage.Set(float64(pipelineDeploymentCount))

	algoCount, err := r.getAlgoCount(request)
	utils.AlgoCountGuage.Set(float64(algoCount))

	dcCount, err := r.getDataConnectorCount(request)
	utils.DataConnectorCountGuage.Set(float64(dcCount))

	topicCount, err := r.getTopicCount(request)
	utils.TopicCountGuage.Set(float64(topicCount))

	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcilePipelineDeployment) getPipelineDeploymentCount(request *reconcile.Request) (int, error) {

	listOptions := &client.ListOptions{}
	listOptions.SetLabelSelector(fmt.Sprintf("system=algorun, component=pipelinedeployment"))
	listOptions.InNamespace(request.Namespace)

	list := &unstructured.UnstructuredList{}
	ctx := context.TODO()
	err := r.client.List(ctx, listOptions, list)

	if err != nil && errors.IsNotFound(err) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return len(list.Items), nil

}

func (r *ReconcilePipelineDeployment) getAlgoCount(request *reconcile.Request) (int, error) {

	listOptions := &client.ListOptions{}
	listOptions.SetLabelSelector(fmt.Sprintf("system=algorun, component=algo"))
	listOptions.InNamespace(request.Namespace)

	deploymentList := &appsv1.DeploymentList{}
	ctx := context.TODO()
	err := r.client.List(ctx, listOptions, deploymentList)

	if err != nil && errors.IsNotFound(err) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return len(deploymentList.Items), nil

}

func (r *ReconcilePipelineDeployment) getDataConnectorCount(request *reconcile.Request) (int, error) {

	listOptions := &client.ListOptions{}
	listOptions.SetLabelSelector(fmt.Sprintf("system=algorun, component=dataconnector"))
	listOptions.InNamespace(request.Namespace)

	list := &unstructured.UnstructuredList{}
	ctx := context.TODO()
	err := r.client.List(ctx, listOptions, list)

	if err != nil && errors.IsNotFound(err) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return len(list.Items), nil

}

func (r *ReconcilePipelineDeployment) getTopicCount(request *reconcile.Request) (int, error) {

	listOptions := &client.ListOptions{}
	listOptions.SetLabelSelector(fmt.Sprintf("system=algorun, component=topic"))
	listOptions.InNamespace(request.Namespace)

	list := &unstructured.UnstructuredList{}
	ctx := context.TODO()
	err := r.client.List(ctx, listOptions, list)

	if err != nil && errors.IsNotFound(err) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return len(list.Items), nil

}
