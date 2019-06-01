package endpoint

import (
	"context"
	errorsbase "errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	kc "github.com/go-kafka/connect"
	"github.com/go-test/deep"

	utils "endpoint-operator/internal/utilities"
	"endpoint-operator/pkg/apis/algo/v1alpha1"
	algov1alpha1 "endpoint-operator/pkg/apis/algo/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileEndpoint) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	logData := map[string]interface{}{
		"Request.Namespace": request.Namespace,
		"Request.Name":      request.Name,
	}
	reqLogger := log.WithValues("data", logData)
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
			// Sending a notification that an endpoint was deleted. Just not sure which one!
			notifMessage := &v1alpha1.NotifMessage{
				MessageTimestamp: time.Now(),
				Level:            "Info",
				Type_:            "EndpointDeleted",
			}
			utils.Notify(notifMessage)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Info("Error reading the Endpoint Instance object - requeuing the request")
		return reconcile.Result{}, err
	}

	var wg sync.WaitGroup
	wg.Add(1)

	// Create / update the kafka topics
	go func() {
		r.reconcileTopics(instance, request)
		wg.Done()
	}()

	// Reconcile all algo deployments
	reqLogger.Info("Reconciling Algos")
	// Iterate the AlgoConfigs
	for _, algoConfig := range instance.Spec.EndpointConfig.AlgoConfigs {
		wg.Add(1)
		go func(currentAlgoConfig algov1alpha1.AlgoConfig) {
			err = r.reconcileAlgo(instance, &currentAlgoConfig, request)
			if err != nil {
				reqLogger.Error(err, "Error in AlgoConfig reconcile loop.")
			}
			wg.Done()
		}(algoConfig)
	}

	// Reconcile all data connectors
	reqLogger.Info("Reconciling Data Connectors")
	// Iterate the DataConnectors
	for _, dcConfig := range instance.Spec.EndpointConfig.DataConnectorConfigs {
		wg.Add(1)
		go func(currentDcConfig algov1alpha1.DataConnectorConfig) {
			err = r.reconcileDataConnector(instance, &currentDcConfig, request)
			if err != nil {
				reqLogger.Error(err, "Error in DataConnectorConfigs reconcile loop.")
			}
			wg.Done()
		}(dcConfig)
	}

	// Reconcile hook container
	reqLogger.Info("Reconciling Hooks")
	wg.Add(1)
	go func(endpoint *algov1alpha1.Endpoint) {
		err = r.reconcileHook(endpoint, request)
		if err != nil {
			reqLogger.Error(err, "Error in Hook reconcile.")
		}
		wg.Done()
	}(instance)

	// Wait for algo, data connector and topic reconciliation to complete
	wg.Wait()

	endpointStatus, err := r.getStatus(instance, request)
	if err != nil {
		reqLogger.Error(err, "Failed to get Endpoint status.")
		return reconcile.Result{}, err
	}

	statusChanged := false

	if instance.Status.Status != endpointStatus.Status {
		instance.Status.Status = endpointStatus.Status
		statusChanged = true

		notifMessage := &v1alpha1.NotifMessage{
			MessageTimestamp: time.Now(),
			Level:            "Info",
			Type_:            "EndpointStatus",
			EndpointStatusMessage: &v1alpha1.EndpointStatusMessage{
				EndpointOwnerUserName: instance.Spec.EndpointConfig.EndpointOwnerUserName,
				EndpointName:          instance.Spec.EndpointConfig.EndpointName,
				Status:                instance.Status.Status,
			},
		}

		utils.Notify(notifMessage)
	}

	// Iterate the existing deployment statuses and update if changed
	for _, deplStatus := range instance.Status.AlgoDeploymentStatuses {
		for _, newDeplStatus := range endpointStatus.AlgoDeploymentStatuses {
			if newDeplStatus.Name == deplStatus.Name {

				if diff := deep.Equal(deplStatus, newDeplStatus); diff != nil {
					deplStatus = newDeplStatus
					statusChanged = true
					// reqLogger.Info("Differences", "Differences", diff)
					notifMessage := &v1alpha1.NotifMessage{
						MessageTimestamp: time.Now(),
						Level:            "Info",
						Type_:            "EndpointDeployment",
						EndpointStatusMessage: &v1alpha1.EndpointStatusMessage{
							EndpointOwnerUserName: instance.Spec.EndpointConfig.EndpointOwnerUserName,
							EndpointName:          instance.Spec.EndpointConfig.EndpointName,
							Status:                instance.Status.Status,
						},
					}

					utils.Notify(notifMessage)
				}

			}
		}
	}

	// Iterate the existing pod statuses and update if changed
	for _, podStatus := range instance.Status.AlgoPodStatuses {
		for _, newPodStatus := range endpointStatus.AlgoPodStatuses {
			if newPodStatus.Name == podStatus.Name {

				if diff := deep.Equal(podStatus, newPodStatus); diff != nil {
					podStatus = newPodStatus
					statusChanged = true
					// reqLogger.Info("Differences", "Differences", diff)
					notifMessage := &v1alpha1.NotifMessage{
						MessageTimestamp: time.Now(),
						Level:            "Info",
						Type_:            "EndpointPod",
						EndpointStatusMessage: &v1alpha1.EndpointStatusMessage{
							EndpointOwnerUserName: instance.Spec.EndpointConfig.EndpointOwnerUserName,
							EndpointName:          instance.Spec.EndpointConfig.EndpointName,
							Status:                instance.Status.Status,
						},
					}

					utils.Notify(notifMessage)
				}

			}
		}
	}

	if statusChanged {
		instance.Status = *endpointStatus

		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update Endpoint status.")
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil

}

func (r *ReconcileEndpoint) getStatus(cr *algov1alpha1.Endpoint, request reconcile.Request) (*algov1alpha1.EndpointStatus, error) {

	endpointStatus := algov1alpha1.EndpointStatus{
		EndpointOwnerUserName: cr.Spec.EndpointConfig.EndpointOwnerUserName,
		EndpointName:          cr.Spec.EndpointConfig.EndpointName,
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

	endpointStatus.AlgoDeploymentStatuses = deploymentStatuses

	// Calculate endpoint status
	endpointStatusString, err := calculateStatus(cr, &deploymentStatuses)
	if err != nil {
		reqLogger.Error(err, "Failed to calculate Endpoint status.")
	}

	endpointStatus.Status = endpointStatusString

	podStatuses, err := r.getPodStatuses(cr, request)
	if err != nil {
		reqLogger.Error(err, "Failed to get pod statuses.")
		return nil, err
	}

	endpointStatus.AlgoPodStatuses = podStatuses

	return &endpointStatus, nil

}

func (r *ReconcileEndpoint) getDeploymentStatuses(cr *algov1alpha1.Endpoint, request reconcile.Request) ([]algov1alpha1.AlgoDeploymentStatus, error) {

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

func (r *ReconcileEndpoint) getPodStatuses(cr *algov1alpha1.Endpoint, request reconcile.Request) ([]algov1alpha1.AlgoPodStatus, error) {

	// Get all algo pods for this endpoint
	listOptions := &client.ListOptions{}
	listOptions.SetLabelSelector(fmt.Sprintf("system=algorun, tier=algo, endpointowner=%s, endpoint=%s",
		cr.Spec.EndpointConfig.EndpointOwnerUserName,
		cr.Spec.EndpointConfig.EndpointName))
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

func calculateStatus(cr *algov1alpha1.Endpoint, deploymentStatuses *[]algov1alpha1.AlgoDeploymentStatus) (string, error) {

	var unreadyDeployments int
	algoCount := len(cr.Spec.EndpointConfig.AlgoConfigs)
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

// reconcileAlgos creates or updates all algos for the endpoint
func (r *ReconcileEndpoint) reconcileAlgo(endpoint *algov1alpha1.Endpoint, algoConfig *algov1alpha1.AlgoConfig, request reconcile.Request) error {

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
		"system":        "algorun",
		"tier":          "algo",
		"endpointowner": endpoint.Spec.EndpointConfig.EndpointOwnerUserName,
		"endpoint":      endpoint.Spec.EndpointConfig.EndpointName,
		"pipeline":      endpoint.Spec.EndpointConfig.PipelineName,
		"algoowner":     algoConfig.AlgoOwnerUserName,
		"algo":          algoConfig.AlgoName,
		"algoversion":   algoConfig.AlgoVersionTag,
		"algoindex":     strconv.Itoa(int(algoConfig.AlgoIndex)),
		"env":           "production",
	}

	// Check to make sure the algo isn't already created
	listOptions := &client.ListOptions{}
	listOptions.SetLabelSelector(fmt.Sprintf("system=algorun, tier=algo, endpointowner=%s, endpoint=%s, algoowner=%s, algo=%s, algoversion=%s, algoindex=%v",
		endpoint.Spec.EndpointConfig.EndpointOwnerUserName,
		endpoint.Spec.EndpointConfig.EndpointName,
		algoConfig.AlgoOwnerUserName,
		algoConfig.AlgoName,
		algoConfig.AlgoVersionTag,
		algoConfig.AlgoIndex))
	listOptions.InNamespace(request.NamespacedName.Namespace)

	existingDeployment, err := r.checkForDeployment(listOptions)

	if existingDeployment != nil {
		algoConfig.DeploymentName = existingDeployment.GetName()
	}

	// Generate the k8s deployment for the algoconfig
	algoBuilder := utils.AlgoBuilder{endpoint, algoConfig}
	algoDeployment, err := algoBuilder.CreateDeploymentSpec(name, labels, existingDeployment != nil)
	if err != nil {
		algoLogger.Error(err, "Failed to create algo deployment spec")
		return err
	}

	// Set Endpoint instance as the owner and controller
	if err := controllerutil.SetControllerReference(endpoint, algoDeployment, r.scheme); err != nil {
		return err
	}

	if existingDeployment == nil {
		err := r.createDeployment(algoDeployment)
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
			err := r.updateDeployment(algoDeployment)
			if err != nil {
				algoLogger.Error(err, "Failed to update algo deployment")
				return err
			}
		}
	}

	// TODO: Setup the horizontal pod autoscaler
	if algoConfig.AutoScale {

	}

	return nil

}

// reconcileHook creates or updates the hook deployment for the endpoint
func (r *ReconcileEndpoint) reconcileHook(endpoint *algov1alpha1.Endpoint, request reconcile.Request) error {

	hookLogger := log

	hookLogger.Info("Reconciling Hook")

	name := "endpoint-hook"

	labels := map[string]string{
		"system":        "algorun",
		"tier":          "hook",
		"endpointowner": endpoint.Spec.EndpointConfig.EndpointOwnerUserName,
		"endpoint":      endpoint.Spec.EndpointConfig.EndpointName,
		"pipeline":      endpoint.Spec.EndpointConfig.PipelineName,
		"env":           "production",
	}

	// Check to make sure the algo isn't already created
	listOptions := &client.ListOptions{}
	listOptions.SetLabelSelector(fmt.Sprintf("system=algorun, tier=hook, endpointowner=%s, endpoint=%s",
		endpoint.Spec.EndpointConfig.EndpointOwnerUserName,
		endpoint.Spec.EndpointConfig.EndpointName))
	listOptions.InNamespace(request.NamespacedName.Namespace)

	existingDeployment, err := r.checkForDeployment(listOptions)

	// Generate the k8s deployment
	hookBuilder := utils.HookBuilder{endpoint}
	hookDeployment, err := hookBuilder.CreateDeploymentSpec(name, labels, existingDeployment)
	if err != nil {
		hookLogger.Error(err, "Failed to create hook deployment spec")
		return err
	}

	// Set Endpoint instance as the owner and controller
	if err := controllerutil.SetControllerReference(endpoint, hookDeployment, r.scheme); err != nil {
		return err
	}

	if existingDeployment == nil {
		err := r.createDeployment(hookDeployment)
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
			err := r.updateDeployment(hookDeployment)
			if err != nil {
				hookLogger.Error(err, "Failed to update hook deployment")
				return err
			}
		}
	}

	return nil

}

func (r *ReconcileEndpoint) checkForDeployment(listOptions *client.ListOptions) (*appsv1.Deployment, error) {

	deploymentList := &appsv1.DeploymentList{}
	ctx := context.TODO()
	err := r.client.List(ctx, listOptions, deploymentList)

	if err != nil && errors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	if len(deploymentList.Items) > 0 {
		return &deploymentList.Items[0], nil
	}

	return nil, nil

}

func (r *ReconcileEndpoint) createDeployment(deployment *appsv1.Deployment) error {

	logData := map[string]interface{}{
		"labels": deployment.Labels,
	}

	if err := r.client.Create(context.TODO(), deployment); err != nil {
		log.WithValues("data", logData)
		log.Error(err, "Failed creating the algo deployment")
		return err
	}

	logData["name"] = deployment.GetName()
	log.WithValues("data", logData)
	log.Info("Created deployment")

	return nil

}

func (r *ReconcileEndpoint) updateDeployment(deployment *appsv1.Deployment) error {

	logData := map[string]interface{}{
		"labels": deployment.Labels,
	}

	if err := r.client.Update(context.TODO(), deployment); err != nil {
		log.WithValues("data", logData)
		log.Error(err, "Failed updating the algo deployment")
		return err
	}

	logData["name"] = deployment.GetName()
	log.WithValues("data", logData)
	log.Info("Updated deployment")

	return nil

}

// reconcileDataConnector creates or updates all data connectors for the endpoint
func (r *ReconcileEndpoint) reconcileDataConnector(endpoint *algov1alpha1.Endpoint, dataConnectorConfig *algov1alpha1.DataConnectorConfig, request reconcile.Request) error {

	kcName := strings.ToLower(fmt.Sprintf("%s-%s", endpoint.Spec.EndpointConfig.EndpointName, dataConnectorConfig.Name))
	dcName := strings.ToLower(fmt.Sprintf("%s-%s-%d", endpoint.Spec.EndpointConfig.EndpointName, dataConnectorConfig.Name, dataConnectorConfig.Index))
	// Set the image name
	var imageName string
	if dataConnectorConfig.ImageTag == "" || dataConnectorConfig.ImageTag == "latest" {
		imageName = fmt.Sprintf("%s:latest", dataConnectorConfig.ImageRepository)
	} else {
		imageName = fmt.Sprintf("%s:%s", dataConnectorConfig.ImageRepository, dataConnectorConfig.ImageTag)
	}
	// dcName := strings.TrimRight(utils.Short(dcName, 20), "-")
	// check to see if data connector already exists
	existingDc := &unstructured.Unstructured{}
	existingDc.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kafka.strimzi.io",
		Kind:    "KafkaConnect",
		Version: "v1alpha1",
	})
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: kcName, Namespace: request.NamespacedName.Namespace}, existingDc)

	if err != nil && errors.IsNotFound(err) {
		// Create the connector cluster
		// Using a unstructured object to submit a strimzi topic creation.
		newDc := &unstructured.Unstructured{}
		newDc.Object = map[string]interface{}{
			"name":      kcName,
			"namespace": request.NamespacedName.Namespace,
			"spec": map[string]interface{}{
				"version":          "2.1.0",
				"replicas":         1,
				"image":            imageName,
				"bootstrapServers": endpoint.Spec.KafkaBrokers,
				// "bootstrapServers": "algorun-kafka-kafka-bootstrap.algorun:9092",
				"config": map[string]interface{}{
					"group.id":                          "connect-cluster",
					"offset.storage.topic":              "connect-cluster-offsets",
					"config.storage.topic":              "connect-cluster-configs",
					"status.storage.topic":              "connect-cluster-status",
					"config.storage.replication.factor": 1,
					"offset.storage.replication.factor": 1,
					"status.storage.replication.factor": 1,
				},
			},
		}
		newDc.SetName(kcName)
		newDc.SetNamespace(request.NamespacedName.Namespace)
		newDc.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "kafka.strimzi.io",
			Kind:    "KafkaConnect",
			Version: "v1alpha1",
		})

		// Set Endpoint instance as the owner and controller
		if err := controllerutil.SetControllerReference(endpoint, newDc, r.scheme); err != nil {
			log.Error(err, "Failed setting the kafka connect cluster controller owner")
		}

		err := r.client.Create(context.TODO(), newDc)
		if err != nil {
			log.Error(err, "Failed creating kafka connect cluster")
		}

	} else if err != nil {
		log.Error(err, "Failed to check if kafka connect cluster exists.")
	} else {

		// TODO: Get the deployment and ensure the status is running

		// If the cluster node exists, then check if connector exists
		// Use the dns name of the connector cluster
		host := fmt.Sprintf("http://%s-connect-api.%s:8083", kcName, request.NamespacedName.Namespace)
		// host := fmt.Sprintf("http://192.168.99.100:30383")
		client := kc.NewClient(host)

		_, http, err := client.GetConnector(dcName)
		if err != nil && http != nil && http.StatusCode != 404 {
			log.Error(err, "Failed to check if data connector exists.")
			return err
		}

		// If connector doesn't exist, then create it
		if http != nil && http.StatusCode == 404 {
			dcConfig := make(map[string]string)
			// iterate the options to create the map
			for _, dcOption := range dataConnectorConfig.Options {
				dcConfig[dcOption.Name] = dcOption.Value
			}

			dcConfig["connector.class"] = dataConnectorConfig.ConnectorClass
			dcConfig["tasks.max"] = strconv.Itoa(int(dataConnectorConfig.TasksMax))

			// If Sink. need to add the source topics
			if strings.ToLower(dataConnectorConfig.DataConnectorType) == "sink" {
				topicConfig, err := r.getDcSourceTopic(endpoint, dataConnectorConfig)
				dcConfig["topics"] = topicConfig.Name

				if err != nil {
					// connector wasn't created.
					log.Error(err, "Could not get sink data connector source topic.")
					return err
				}
			}

			newConnector := kc.Connector{
				Name:   dcName,
				Config: dcConfig,
			}
			_, err = client.CreateConnector(&newConnector)
			if err != nil {
				// connector wasn't created.
				log.Error(err, "Fatal error creating data connector. Kafka connect instance exists but REST API create failed.")
				return err
			}
		}

	}

	return nil
}

func (r *ReconcileEndpoint) getDcSourceTopic(endpoint *algov1alpha1.Endpoint, dataConnectorConfig *algov1alpha1.DataConnectorConfig) (utils.TopicConfig, error) {

	config := endpoint.Spec.EndpointConfig

	for _, pipe := range config.Pipes {

		dcName := fmt.Sprintf("%s:%s[%d]", dataConnectorConfig.Name, dataConnectorConfig.VersionTag, dataConnectorConfig.Index)

		if pipe.DestName == dcName {

			// Get the source topic connected to this pipe
			for _, topic := range endpoint.Spec.EndpointConfig.TopicConfigs {
				if pipe.SourceName == topic.SourceName &&
					pipe.SourceOutputName == topic.SourceOutputName {
					newTopicConfig, err := utils.BuildTopic(config, topic)
					if err != nil {
						return newTopicConfig, err
					}
					return newTopicConfig, nil
				}
			}

		}

	}

	return utils.TopicConfig{}, errorsbase.New(fmt.Sprintf("No topic config found for data connector source. [%s-%d]",
		dataConnectorConfig.Name,
		dataConnectorConfig.Index))

}

func (r *ReconcileEndpoint) reconcileTopics(cr *algov1alpha1.Endpoint, request reconcile.Request) {

	endpointSpec := cr.Spec
	for _, topicConfig := range endpointSpec.EndpointConfig.TopicConfigs {

		newTopicConfig, err := utils.BuildTopic(endpointSpec.EndpointConfig, topicConfig)
		if err != nil {
			log.Error(err, "Error creating new topic config")
		}

		// check to see if topic already exists
		existingTopic := &unstructured.Unstructured{}
		existingTopic.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "kafka.strimzi.io",
			Kind:    "KafkaTopic",
			Version: "v1alpha1",
		})
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: newTopicConfig.Name, Namespace: request.NamespacedName.Namespace}, existingTopic)

		if err != nil && errors.IsNotFound(err) {
			// Create the topic
			// Using a unstructured object to submit a strimzi topic creation.
			newTopic := &unstructured.Unstructured{}
			newTopic.Object = map[string]interface{}{
				"name":      newTopicConfig.Name,
				"namespace": request.NamespacedName.Namespace,
				"spec": map[string]interface{}{
					"partitions": newTopicConfig.Partitions,
					"replicas":   int(topicConfig.TopicReplicationFactor),
					"config":     newTopicConfig.Params,
				},
			}
			newTopic.SetName(newTopicConfig.Name)
			newTopic.SetNamespace(request.NamespacedName.Namespace)
			newTopic.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "kafka.strimzi.io",
				Kind:    "KafkaTopic",
				Version: "v1alpha1",
			})

			// Set Endpoint instance as the owner and controller
			if err := controllerutil.SetControllerReference(cr, newTopic, r.scheme); err != nil {
				log.Error(err, "Failed setting the topic controller owner")
			}

			err := r.client.Create(context.TODO(), newTopic)
			if err != nil {
				log.Error(err, "Failed creating topic")
			}
		} else if err != nil {
			log.Error(err, "Failed to check if Kafka topic exists.")
		} else {
			// Update the topic if changed
			// Check that the partition count did not go down (kafka doesn't support)
			var partitionsCurrent, replicasCurrent int64
			var paramsCurrent map[string]string
			spec, ok := existingTopic.Object["spec"].(map[string]interface{})
			if ok {
				replicasCurrent, ok = spec["replicas"].(int64)
				paramsCurrent, ok = spec["config"].(map[string]string)
				partitionsCurrent, ok = spec["partitions"].(int64)
				if ok {
					if partitionsCurrent > newTopicConfig.Partitions {
						logData := map[string]interface{}{
							"partitionsCurrent": partitionsCurrent,
							"partitionsNew":     newTopicConfig.Partitions,
						}
						log.WithValues("data", logData)
						log.Error(err, "Partition count cannot be decreased. Keeping current partition count.")
						newTopicConfig.Partitions = partitionsCurrent
					}
				}
			}

			if partitionsCurrent != newTopicConfig.Partitions ||
				replicasCurrent != newTopicConfig.Replicas ||
				(len(paramsCurrent) != len(newTopicConfig.Params) &&
					!reflect.DeepEqual(paramsCurrent, newTopicConfig.Params)) {

				fmt.Printf("%v", paramsCurrent)
				// !reflect.DeepEqual(paramsCurrent, params)
				// Update the existing spec
				spec["partitions"] = newTopicConfig.Partitions
				spec["replicas"] = newTopicConfig.Replicas
				spec["config"] = newTopicConfig.Params

				existingTopic.Object["spec"] = spec

				err := r.client.Update(context.TODO(), existingTopic)
				if err != nil {
					log.Error(err, "Failed updating topic")
				}

			}

		}

	}

}
