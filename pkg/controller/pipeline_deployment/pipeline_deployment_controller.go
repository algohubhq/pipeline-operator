package pipelinedeploymentcontroller

import (
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"pipeline-operator/pkg/apis/algorun/v1beta1"
	algov1beta1 "pipeline-operator/pkg/apis/algorun/v1beta1"
	recon "pipeline-operator/pkg/reconciler"
	utils "pipeline-operator/pkg/utilities"

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
	err = c.Watch(&source.Kind{Type: &algov1beta1.PipelineDeployment{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner PipelineDeployment
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &algov1beta1.PipelineDeployment{},
	})
	if err != nil {
		return err
	}
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &algov1beta1.PipelineDeployment{},
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

	ctx := context.TODO()
	// Fetch the PipelineDeployment instance
	instance := &algov1beta1.PipelineDeployment{}
	err := r.client.Get(ctx, request.NamespacedName, instance)
	if err != nil {

		r.updateMetrics(&request)

		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("PipelineDeployment resource not found. Ignoring since object must be deleted.")
			// Sending a notification that an pipelineDeployment was deleted. Just not sure which one!
			notifMessage := &v1beta1.NotifMessage{
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

	// Check if the PipelineDeployment CR was marked to be deleted
	isPipelineDeploymentMarkedToBeDeleted := instance.GetDeletionTimestamp() != nil
	if isPipelineDeploymentMarkedToBeDeleted {

		// This pipelineDeployment is queued for deletion.

		// Update finalizer to allow delete CR
		instance.SetFinalizers(nil)

		// Update CR
		err := r.client.Update(ctx, instance)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Send the delete notification
		notifMessage := &v1beta1.NotifMessage{
			MessageTimestamp: time.Now(),
			Level:            "Info",
			Type_:            "PipelineDeploymentDeleted",
		}
		utils.Notify(notifMessage)

		return reconcile.Result{}, nil
	}

	// Add finalizer for this CR
	if err := r.addFinalizer(instance); err != nil {
		return reconcile.Result{}, err
	}

	// Check for the KAFKA-TLS env variable and certs
	kafkaTLS := utils.CheckForKafkaTLS()

	if kafkaTLS {
		kubeUtil := utils.NewKubeUtil(r.client, &request)
		kubeUtil.CopyKafkaClusterCASecret()
	}

	var wg sync.WaitGroup

	// Create the storage bucket
	reqLogger.Info("Reconciling the Storage Bucket")
	wg.Add(1)
	go func(pipelineDeployment *algov1beta1.PipelineDeployment) {
		bucketReconciler := recon.NewBucketReconciler(instance, &request, r.client)
		err = bucketReconciler.Reconcile()
		if err != nil {
			reqLogger.Error(err, "Error in Bucket reconcile.")
		}
		wg.Done()
	}(instance)

	// Create / update the kafka topics
	reqLogger.Info("Reconciling Kakfa Topics")
	// Iterate the topics
	for _, topicConfig := range instance.Spec.PipelineSpec.TopicConfigs {
		wg.Add(1)
		go func(currentTopicConfig algov1beta1.TopicConfigModel) {
			topicReconciler := recon.NewTopicReconciler(instance, &currentTopicConfig, &request, r.client, r.scheme)
			topicReconciler.Reconcile()
			wg.Done()
		}(topicConfig)
	}

	// Create the kafka user
	reqLogger.Info("Reconciling the Kafka User")
	wg.Add(1)
	go func(pipelineDeployment *algov1beta1.PipelineDeployment) {
		kafkaUserReconciler := recon.NewKafkaUserReconciler(instance, instance.Spec.PipelineSpec.TopicConfigs, &request, r.client, r.scheme)
		kafkaUserReconciler.Reconcile()
		wg.Done()
	}(instance)

	// Reconcile all algo deployments
	reqLogger.Info("Reconciling Algos")
	// Iterate the AlgoConfigs
	for _, algoConfig := range instance.Spec.PipelineSpec.AlgoConfigs {
		wg.Add(1)
		go func(currentAlgoConfig algov1beta1.AlgoConfig) {
			defer wg.Done()
			algoReconciler := recon.NewAlgoReconciler(instance, &currentAlgoConfig, &request, r.client, r.scheme, kafkaTLS)
			err = algoReconciler.Reconcile()
			if err != nil {
				reqLogger.Error(err, "Error in AlgoConfig reconcile loop.")
			}
		}(algoConfig)
	}

	// Reconcile the algo metrics service
	reqLogger.Info("Reconciling Algo Metrics Service")
	wg.Add(1)
	go func() {
		algoReconciler := recon.NewAlgoReconciler(instance, nil, &request, r.client, r.scheme, kafkaTLS)
		algoReconciler.ReconcileService()
		wg.Done()
	}()

	// Reconcile all data connectors
	reqLogger.Info("Reconciling Data Connectors")
	// Iterate the DataConnectors
	for _, dcConfig := range instance.Spec.PipelineSpec.DataConnectorConfigs {
		wg.Add(1)
		go func(currentDcConfig algov1beta1.DataConnectorConfig) {
			dcReconciler := recon.NewDataConnectorReconciler(instance, &currentDcConfig, &request, r.client, r.scheme)
			err = dcReconciler.Reconcile()
			if err != nil {
				reqLogger.Error(err, "Error in DataConnectorConfigs reconcile loop.")
			}
			wg.Done()
		}(dcConfig)
	}

	// Reconcile hook container
	if instance.Spec.PipelineSpec.HookConfig != nil &&
		len(instance.Spec.PipelineSpec.HookConfig.WebHooks) > 0 {
		reqLogger.Info("Reconciling Hooks")
		wg.Add(1)
		go func(pipelineDeployment *algov1beta1.PipelineDeployment) {
			hookReconciler := recon.NewHookReconciler(instance, &request, r.client, r.scheme, kafkaTLS)
			err = hookReconciler.Reconcile()
			if err != nil {
				reqLogger.Error(err, "Error in Hook reconcile.")
			}
			wg.Done()
		}(instance)
	}

	// Reconcile endpoint container
	if instance.Spec.PipelineSpec.EndpointConfig != nil &&
		len(instance.Spec.PipelineSpec.EndpointConfig.Paths) > 0 {
		reqLogger.Info("Reconciling Endpoints")
		wg.Add(1)
		go func(pipelineDeployment *algov1beta1.PipelineDeployment) {
			endpointReconciler := recon.NewEndpointReconciler(instance, &request, r.client, r.scheme, kafkaTLS)
			err = endpointReconciler.Reconcile()
			if err != nil {
				reqLogger.Error(err, "Error in Endpoint reconcile.")
			}
			wg.Done()
		}(instance)
	}

	// Wait for algo, data connector and topic reconciliation to complete
	wg.Wait()

	r.updateMetrics(&request)

	// Run status reconciler
	statusReconciler := recon.NewStatusReconciler(instance, &request, r.client, r.scheme)
	err = statusReconciler.Reconcile()
	if err != nil {
		reqLogger.Error(err, "Error in Status reconcile.")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil

}

//addFinalizer will add this attribute to the PipelineDeployment CR
func (r *ReconcilePipelineDeployment) addFinalizer(pipelineDeployment *algov1beta1.PipelineDeployment) error {
	if len(pipelineDeployment.GetFinalizers()) < 1 && pipelineDeployment.GetDeletionTimestamp() == nil {
		log.Info("Adding Finalizer for the PipelineDeployment")
		pipelineDeployment.SetFinalizers([]string{"finalizer.pipelineDeployment.algorun"})

		// Update CR
		err := r.client.Update(context.TODO(), pipelineDeployment)
		if err != nil {
			log.Error(err, "Failed to update PipelineDeployment with finalizer")
			return err
		}
	}
	return nil
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

	opts := []client.ListOption{
		client.InNamespace(request.Namespace),
		client.MatchingLabels{
			"app.kubernetes.io/part-of":   "algo.run",
			"app.kubernetes.io/component": "pipeline-deployment",
		},
	}

	list := &unstructured.UnstructuredList{}
	ctx := context.TODO()
	err := r.client.List(ctx, list, opts...)

	if err != nil && errors.IsNotFound(err) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return len(list.Items), nil

}

func (r *ReconcilePipelineDeployment) getAlgoCount(request *reconcile.Request) (int, error) {

	opts := []client.ListOption{
		client.InNamespace(request.Namespace),
		client.MatchingLabels{
			"app.kubernetes.io/part-of":   "algo.run",
			"app.kubernetes.io/component": "algo",
		},
	}

	deploymentList := &appsv1.DeploymentList{}
	ctx := context.TODO()
	err := r.client.List(ctx, deploymentList, opts...)

	if err != nil && errors.IsNotFound(err) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return len(deploymentList.Items), nil

}

func (r *ReconcilePipelineDeployment) getDataConnectorCount(request *reconcile.Request) (int, error) {

	opts := []client.ListOption{
		client.InNamespace(request.Namespace),
		client.MatchingLabels{
			"app.kubernetes.io/part-of":   "algo.run",
			"app.kubernetes.io/component": "dataconnector",
		},
	}

	list := &unstructured.UnstructuredList{}
	ctx := context.TODO()
	err := r.client.List(ctx, list, opts...)

	if err != nil && errors.IsNotFound(err) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return len(list.Items), nil

}

func (r *ReconcilePipelineDeployment) getTopicCount(request *reconcile.Request) (int, error) {

	opts := []client.ListOption{
		client.InNamespace(request.Namespace),
		client.MatchingLabels{
			"app.kubernetes.io/part-of":   "algo.run",
			"app.kubernetes.io/component": "topic",
		},
	}

	list := &unstructured.UnstructuredList{}
	ctx := context.TODO()
	err := r.client.List(ctx, list, opts...)

	if err != nil && errors.IsNotFound(err) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return len(list.Items), nil

}
