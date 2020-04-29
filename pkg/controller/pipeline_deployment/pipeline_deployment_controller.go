package pipelinedeploymentcontroller

import (
	"context"
	e "errors"
	"fmt"
	"sync"
	"time"

	"pipeline-operator/pkg/apis/algorun/v1beta1"
	algov1beta1 "pipeline-operator/pkg/apis/algorun/v1beta1"
	kafkav1beta1 "pipeline-operator/pkg/apis/kafka/v1beta1"
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
	return &ReconcilePipelineDeployment{client: mgr.GetClient(),
		manager: mgr,
		scheme:  mgr.GetScheme(),
	}
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
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &algov1beta1.PipelineDeployment{},
	})
	if err != nil {
		return err
	}
	err = c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &algov1beta1.PipelineDeployment{},
	})
	if err != nil {
		return err
	}
	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &algov1beta1.PipelineDeployment{},
	})
	if err != nil {
		return err
	}
	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{
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
	client  client.Client
	manager manager.Manager
	scheme  *runtime.Scheme
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
	deployment := &algov1beta1.PipelineDeployment{}
	err := r.client.Get(ctx, request.NamespacedName, deployment)

	// Check if this pipeline deployment cr request is in the same namespace as this operator.
	// We only want this operator to handle requests for it's own namespace

	if err != nil {

		r.updateMetrics(&request)

		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("PipelineDeployment resource not found. Ignoring since object must be deleted.")
			// Sending a notification that an pipelineDeployment was deleted. Just not sure which one!
			loglevel := v1beta1.LOGLEVELS_INFO
			notifType := v1beta1.NOTIFTYPES_PIPELINE_DEPLOYMENT_DELETED
			notifMessage := &v1beta1.NotifMessage{
				MessageTimestamp: time.Now(),
				Level:            &loglevel,
				Type:             &notifType,
			}
			utils.Notify(notifMessage)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Info("Error reading the PipelineDeployment Instance object - requeuing the request")
		return reconcile.Result{}, err
	}

	// Check if the PipelineDeployment CR was marked to be deleted
	isPipelineDeploymentMarkedToBeDeleted := deployment.GetDeletionTimestamp() != nil
	if isPipelineDeploymentMarkedToBeDeleted {

		// This pipelineDeployment is queued for deletion.

		// Update finalizer to allow delete CR
		deployment.SetFinalizers(nil)

		// Update CR
		err := r.client.Update(ctx, deployment)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Send the delete notification
		loglevel := v1beta1.LOGLEVELS_INFO
		notifType := v1beta1.NOTIFTYPES_PIPELINE_DEPLOYMENT_DELETED
		notifMessage := &v1beta1.NotifMessage{
			MessageTimestamp: time.Now(),
			Level:            &loglevel,
			Type:             &notifType,
		}
		utils.Notify(notifMessage)

		return reconcile.Result{}, nil
	}

	// Add finalizer for this CR
	if err := r.addFinalizer(deployment); err != nil {
		return reconcile.Result{}, err
	}

	// Set the deploymentNamespace to the request namespace
	if deployment.Spec.DeploymentNamespace == "" {
		deployment.Spec.DeploymentNamespace = request.Namespace
	}

	kubeUtil := utils.NewKubeUtil(r.manager, &request)
	kafkaUtil := utils.NewKafkaUtil(deployment, r.manager, r.scheme)

	// Check for the KAFKA_TLS env variable and certs
	kafkaTLS := utils.CheckForKafkaTLS()

	if kafkaTLS {
		kafkaUtil.CopyKafkaSecrets()
	}

	var wg sync.WaitGroup

	// Populate all algoRefs
	// If the algoRef is set, try to pull from the kubernetes cluster
	reqLogger.Info("Populating all AlgoRefs")
	for i, algoDepl := range deployment.Spec.Algos {
		if algoDepl.AlgoRef != nil {
			listOpt := kubeUtil.GetListOptionsFromRef(algoDepl.AlgoRef)
			algo, err := kubeUtil.CheckForAlgoCR(listOpt)
			if err != nil || algo == nil {
				err := e.New(fmt.Sprintf("algoRef set but not Algo matching Algo found [%v]", algoDepl.AlgoRef))
				reqLogger.Error(err, "Failed to populate AlgoRef")
			}
			deployment.Spec.Algos[i].Spec = &algo.Spec
		}
	}

	// Populate all dataConnectorRefs
	// If the dataConnectorRef is set, try to pull from the kubernetes cluster
	reqLogger.Info("Populating all DataConnectorRefs")
	for i, dcDepl := range deployment.Spec.DataConnectors {
		if dcDepl.DataConnectorRef != nil {

			listOpt := kubeUtil.GetListOptionsFromRef(dcDepl.DataConnectorRef)
			dc, err := kubeUtil.CheckForDataConnectorCR(listOpt)
			if err != nil || dc == nil {
				err := e.New(fmt.Sprintf("DataConnectorRef set but no Data Connector matching Data Connector found [%v]", dcDepl.DataConnectorRef))
				reqLogger.Error(err, "Failed to populate DataConnectorRef")
			}
			deployment.Spec.DataConnectors[i].Spec = &dc.Spec
		}
	}

	// Compile all kafka topic configs
	allTopicConfigs := utils.GetAllTopicConfigs(&deployment.Spec)

	// Create the storage bucket
	// NOTE: We aren't adding this reconciliation to the waitgroup.
	reqLogger.Info("Reconciling the Storage Bucket")
	go func(pipelineDeployment *algov1beta1.PipelineDeployment) {
		bucketReconciler := recon.NewBucketReconciler(deployment, &request, r.manager)
		err = bucketReconciler.Reconcile()
		if err != nil {
			reqLogger.Error(err, "Error in Bucket reconcile.")
		}
	}(deployment)

	// Create the kafka user
	reqLogger.Info("Reconciling the Kafka User")
	wg.Add(1)
	go func(pipelineDeployment *algov1beta1.PipelineDeployment) {
		kafkaUserReconciler := recon.NewKafkaUserReconciler(deployment,
			allTopicConfigs,
			&request,
			r.manager.GetAPIReader(),
			r.client,
			r.scheme)
		kafkaUserReconciler.Reconcile()
		wg.Done()
	}(deployment)

	// // Reconcile all algo deployments
	reqLogger.Info("Reconciling Algos")
	for _, algoDepl := range deployment.Spec.Algos {
		wg.Add(1)
		go func(currentAlgoDepl algov1beta1.AlgoDeploymentV1beta1) {
			defer wg.Done()
			algoReconciler, err := recon.NewAlgoReconciler(deployment,
				&currentAlgoDepl,
				allTopicConfigs,
				&request,
				r.manager,
				r.scheme,
				kafkaTLS)

			if err != nil {
				msg := "Failed to create algo reconciler"
				reqLogger.Error(err, msg)
				wg.Done()
				return
			}

			err = algoReconciler.Reconcile()
			if err != nil {
				reqLogger.Error(err, "Error in AlgoConfig reconcile loop.")
			}
		}(algoDepl)
	}

	// // Reconcile the algo metrics service
	reqLogger.Info("Reconciling Algo Metrics Service")
	wg.Add(1)
	go func() {
		algoReconciler, err := recon.NewAlgoServiceReconciler(deployment,
			allTopicConfigs,
			&request,
			r.manager,
			r.scheme,
			kafkaTLS)
		if err != nil {
			msg := "Failed to create algo metric service reconciler"
			reqLogger.Error(err, msg)
			wg.Done()
			return
		}
		algoReconciler.ReconcileService()
		wg.Done()
	}()

	// // Reconcile all data connectors
	reqLogger.Info("Reconciling Data Connectors")
	for _, dcDepl := range deployment.Spec.DataConnectors {
		wg.Add(1)
		go func(currentDcDepl algov1beta1.DataConnectorDeploymentV1beta1) {
			dcReconciler, err := recon.NewDataConnectorReconciler(deployment,
				&currentDcDepl,
				allTopicConfigs,
				&request,
				r.manager,
				r.scheme,
				kafkaTLS)
			if err != nil {
				msg := "Failed to create data connector reconciler"
				reqLogger.Error(err, msg)
				wg.Done()
				return
			}

			err = dcReconciler.Reconcile()
			if err != nil {
				reqLogger.Error(err, "Error in DataConnectorConfigs reconcile loop.")
			}
			wg.Done()
		}(dcDepl)
	}

	// // Reconcile hook container
	if deployment.Spec.EventHook.WebHooks != nil && len(deployment.Spec.EventHook.WebHooks) > 0 {
		reqLogger.Info("Reconciling Event Hooks")
		wg.Add(1)
		go func(pipelineDeployment *algov1beta1.PipelineDeployment) {
			hookReconciler := recon.NewEventHookReconciler(deployment, allTopicConfigs, &request, r.manager, r.scheme, kafkaTLS)
			err = hookReconciler.Reconcile()
			if err != nil {
				reqLogger.Error(err, "Error in Hook reconcile.")
			}
			wg.Done()
		}(deployment)
	}

	// // Reconcile endpoint container
	if deployment.Spec.Endpoint.Paths != nil && len(deployment.Spec.Endpoint.Paths) > 0 {
		reqLogger.Info("Reconciling Endpoints")
		wg.Add(1)
		go func(pipelineDeployment *algov1beta1.PipelineDeployment) {
			endpointReconciler := recon.NewEndpointReconciler(deployment,
				&request,
				r.manager,
				r.scheme,
				kafkaTLS)
			err = endpointReconciler.Reconcile()
			if err != nil {
				reqLogger.Error(err, "Error in Endpoint reconcile.")
			}
			wg.Done()
		}(deployment)
	}

	// Wait for algo, data connector and topic reconciliation to complete
	wg.Wait()

	r.updateMetrics(&request)

	// Run status reconciler
	statusReconciler := recon.NewStatusReconciler(deployment, &request, r.client, r.scheme)
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

	list := &v1beta1.PipelineDeploymentList{}
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

	list := &kafkav1beta1.KafkaConnectList{}
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

	list := &kafkav1beta1.KafkaTopicList{}
	ctx := context.TODO()
	err := r.client.List(ctx, list, opts...)

	if err != nil && errors.IsNotFound(err) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return len(list.Items), nil

}
