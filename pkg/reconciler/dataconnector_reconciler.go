package reconciler

import (
	"context"
	errorsbase "errors"
	"fmt"
	"pipeline-operator/pkg/apis/algorun/v1beta1"
	algov1beta1 "pipeline-operator/pkg/apis/algorun/v1beta1"
	kafkav1alpha1 "pipeline-operator/pkg/apis/kafka/v1alpha1"
	kafkav1beta1 "pipeline-operator/pkg/apis/kafka/v1beta1"
	utils "pipeline-operator/pkg/utilities"
	"strconv"
	"strings"

	patch "github.com/banzaicloud/k8s-objectmatcher/patch"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewDataConnectorReconciler returns a new DataConnectorReconciler
func NewDataConnectorReconciler(pipelineDeployment *algov1beta1.PipelineDeployment,
	dataConnectorSpec *v1beta1.DataConnectorDeploymentV1beta1,
	allTopicConfigs map[string]*v1beta1.TopicConfigModel,
	kafkaUtil *utils.KafkaUtil,
	request *reconcile.Request,
	manager manager.Manager,
	scheme *runtime.Scheme) (*DataConnectorReconciler, error) {

	// Ensure the algo has a matching version defined
	var activeDcVersion *v1beta1.DataConnectorVersionModel
	for _, version := range dataConnectorSpec.Spec.Versions {
		if dataConnectorSpec.Version == version.VersionTag {
			activeDcVersion = &version
		}
	}

	if activeDcVersion == nil {
		err := errorsbase.New(fmt.Sprintf("There is no matching Data Connector Version with requested version tag [%s]", dataConnectorSpec.Version))
		return nil, err
	}

	return &DataConnectorReconciler{
		pipelineDeployment:         pipelineDeployment,
		dataConnectorSpec:          dataConnectorSpec,
		activeDataConnectorVersion: activeDcVersion,
		allTopicConfigs:            allTopicConfigs,
		kafkaUtil:                  kafkaUtil,
		request:                    request,
		manager:                    manager,
		scheme:                     scheme,
	}, nil
}

// DataConnectorReconciler reconciles an dataConnectorConfig object
type DataConnectorReconciler struct {
	pipelineDeployment         *algov1beta1.PipelineDeployment
	dataConnectorSpec          *v1beta1.DataConnectorDeploymentV1beta1
	activeDataConnectorVersion *v1beta1.DataConnectorVersionModel
	allTopicConfigs            map[string]*v1beta1.TopicConfigModel
	kafkaUtil                  *utils.KafkaUtil
	request                    *reconcile.Request
	manager                    manager.Manager
	scheme                     *runtime.Scheme
}

// Reconcile creates or updates the data connector for the pipelineDeployment
func (dataConnectorReconciler *DataConnectorReconciler) Reconcile() error {

	dcSpec := dataConnectorReconciler.dataConnectorSpec

	if dcSpec.Topics != nil && len(dcSpec.Topics) > 0 {
		// Create the Kafka Topics
		log.Info("Reconciling Kakfa Topics for Data Connector outputs")
		for _, topic := range dcSpec.Topics {
			dcName := utils.GetDcFullName(dcSpec)
			go func(currentTopicConfig algov1beta1.TopicConfigModel) {
				topicReconciler := NewTopicReconciler(dataConnectorReconciler.pipelineDeployment,
					dcName,
					&currentTopicConfig,
					dataConnectorReconciler.kafkaUtil,
					dataConnectorReconciler.request,
					dataConnectorReconciler.manager,
					dataConnectorReconciler.scheme)
				topicReconciler.Reconcile()
			}(topic)
		}
	}

	kcName, err := dataConnectorReconciler.reconcileConnectCluster()
	if err != nil {
		return err
	}

	err = dataConnectorReconciler.reconcileConnector(kcName)
	if err != nil {
		return err
	}

	return nil
}

func (dataConnectorReconciler *DataConnectorReconciler) reconcileConnectCluster() (string, error) {

	dcSpec := dataConnectorReconciler.dataConnectorSpec
	pipelineDeployment := dataConnectorReconciler.pipelineDeployment
	kcName := strings.ToLower(fmt.Sprintf("%s-%s", dataConnectorReconciler.pipelineDeployment.Spec.DeploymentName, dcSpec.Spec.Name))

	// Create the connector cluster
	labels := map[string]string{
		"app.kubernetes.io/part-of":    "algo.run",
		"app.kubernetes.io/component":  "dataconnector",
		"app.kubernetes.io/managed-by": "pipeline-operator",
		"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", pipelineDeployment.Spec.DeploymentOwner,
			pipelineDeployment.Spec.DeploymentName),
		"algo.run/pipeline": fmt.Sprintf("%s.%s", pipelineDeployment.Spec.PipelineOwner,
			pipelineDeployment.Spec.PipelineName),
		"algo.run/dataconnector":         dcSpec.Spec.Name,
		"algo.run/dataconnector-version": dcSpec.Version,
		"algo.run/index":                 strconv.Itoa(int(dcSpec.Index)),
	}

	annotations := map[string]string{
		"strimzi.io/use-connector-resources": "true",
	}

	newDcSpec, err := dataConnectorReconciler.buildKafkaConnectSpec()
	if err != nil {
		log.Error(err, "Error creating new Kafka Connect Cluster Spec")
		return kcName, err
	}

	// check to see if data connector already exists
	existingDc := &kafkav1beta1.KafkaConnect{}
	err = dataConnectorReconciler.manager.GetClient().Get(context.TODO(),
		types.NamespacedName{
			Name:      kcName,
			Namespace: dataConnectorReconciler.pipelineDeployment.Spec.DeploymentNamespace,
		},
		existingDc)

	dc := &kafkav1beta1.KafkaConnect{}
	// If this is an update, need to set the existing deployment name
	if existingDc != nil {
		dc = existingDc.DeepCopy()
	} else {
		dc.SetName(kcName)
		dc.SetNamespace(dataConnectorReconciler.pipelineDeployment.Spec.DeploymentNamespace)
		dc.SetLabels(labels)
		dc.SetAnnotations(annotations)
	}
	dc.Spec = *newDcSpec

	// Set PipelineDeployment instance as the owner and controller
	if err := controllerutil.SetControllerReference(pipelineDeployment, dc, dataConnectorReconciler.scheme); err != nil {
		log.Error(err, "Failed setting the kafka connect cluster controller owner")
	}

	if err != nil && errors.IsNotFound(err) {

		err := dataConnectorReconciler.manager.GetClient().Create(context.TODO(), dc)
		if err != nil {
			log.Error(err, "Failed creating Data Connector Cluster")
			return kcName, err
		}

	} else if err != nil {
		log.Error(err, "Failed to check if exists.")
		return kcName, err
	} else {

		patchMaker := patch.NewPatchMaker(patch.NewAnnotator("algo.run/last-applied"))
		patchResult, err := patchMaker.Calculate(existingDc, dc)
		if err != nil {
			log.Error(err, "Failed to calculate Data Connector Cluster resource changes")
		}

		if !patchResult.IsEmpty() {
			log.Info("Data Connector Cluster Changed. Updating...")
			err := dataConnectorReconciler.manager.GetClient().Update(context.TODO(), existingDc)
			if err != nil {
				log.Error(err, "Failed updating Data Connector Cluster")
				return kcName, err
			}
		}

	}

	return kcName, nil

}

func (dataConnectorReconciler *DataConnectorReconciler) buildKafkaConnectSpec() (*kafkav1beta1.KafkaConnectSpec, error) {

	dcDepl := dataConnectorReconciler.dataConnectorSpec
	// Set the image name
	var imageName string
	if dataConnectorReconciler.activeDataConnectorVersion.Image == nil {
		err := errorsbase.New("Data Connector Image is empty")
		log.Error(err,
			fmt.Sprintf("Data Connector image cannot be empty for [%s]", dcDepl.Spec.Name))
		return nil, err
	}
	if dataConnectorReconciler.activeDataConnectorVersion.Image.Tag == "" || dataConnectorReconciler.activeDataConnectorVersion.Image.Tag == "latest" {
		imageName = fmt.Sprintf("%s:latest", dataConnectorReconciler.activeDataConnectorVersion.Image.Repository)
	} else {
		imageName = fmt.Sprintf("%s:%s", dataConnectorReconciler.activeDataConnectorVersion.Image.Repository, dataConnectorReconciler.activeDataConnectorVersion.Image.Tag)
	}

	spec := kafkav1beta1.KafkaConnectSpec{
		Version:          "2.4.0",
		Replicas:         int(dcDepl.Replicas),
		Image:            imageName,
		BootstrapServers: dataConnectorReconciler.pipelineDeployment.Spec.KafkaBrokers,
		Metrics: &kafkav1beta1.JMXExporter{
			LowercaseOutputName:       true,
			LowercaseOutputLabelNames: true,
			Rules: []kafkav1beta1.JMXExporterRule{
				{
					Pattern: "kafka.connect<type=connect-worker-metrics>([^:]+):",
					Name:    "kafka_connect_connect_worker_metrics_$1",
				},
				{
					Pattern: "kafka.connect<type=connect-metrics, client-id=([^:]+)><>([^:]+)",
					Name:    "kafka_connect_connect_metrics_$1_$2",
				},
			},
		},
		Config: map[string]string{
			"group.id":                          "connect-cluster",
			"offset.storage.topic":              "connect-cluster-offsets",
			"config.storage.topic":              "connect-cluster-configs",
			"status.storage.topic":              "connect-cluster-status",
			"config.storage.replication.factor": "1",
			"offset.storage.replication.factor": "1",
			"status.storage.replication.factor": "1",
		},
	}

	if dataConnectorReconciler.kafkaUtil.TLS != nil {
		spec.TLS = dataConnectorReconciler.kafkaUtil.TLS
	}

	if dataConnectorReconciler.kafkaUtil.Authentication != nil {
		spec.Authentication = dataConnectorReconciler.kafkaUtil.Authentication
	}

	return &spec, nil

}

func (dataConnectorReconciler *DataConnectorReconciler) reconcileConnector(connectClusterName string) error {

	dcSpec := dataConnectorReconciler.dataConnectorSpec
	pipelineDeployment := dataConnectorReconciler.pipelineDeployment
	dcName := strings.ToLower(fmt.Sprintf("%s-%s-%d", dataConnectorReconciler.pipelineDeployment.Spec.DeploymentName,
		dcSpec.Spec.Name,
		dcSpec.Index))

	newConnectorSpec, err := dataConnectorReconciler.buildKafkaConnectorSpec()
	if err != nil {
		log.Error(err, "Error creating new Strimzi Kafka Connector Spec")
		return err
	}

	// Create the connector cluster
	labels := map[string]string{
		"app.kubernetes.io/part-of":    "algo.run",
		"app.kubernetes.io/component":  "dataconnector",
		"app.kubernetes.io/managed-by": "pipeline-operator",
		"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", pipelineDeployment.Spec.DeploymentOwner,
			pipelineDeployment.Spec.DeploymentName),
		"algo.run/pipeline": fmt.Sprintf("%s.%s", pipelineDeployment.Spec.PipelineOwner,
			pipelineDeployment.Spec.PipelineName),
		"algo.run/dataconnector":         dcSpec.Spec.Name,
		"algo.run/dataconnector-version": dcSpec.Version,
		"algo.run/index":                 strconv.Itoa(int(dcSpec.Index)),
		"strimzi.io/cluster":             connectClusterName,
	}

	// check to see if data connector already exists
	existingConnector := &kafkav1alpha1.KafkaConnector{}
	err = dataConnectorReconciler.manager.GetClient().Get(context.TODO(),
		types.NamespacedName{
			Name:      dcName,
			Namespace: dataConnectorReconciler.pipelineDeployment.Spec.DeploymentNamespace,
		},
		existingConnector)

	connector := &kafkav1alpha1.KafkaConnector{}
	// If this is an update, need to set the existing name
	if existingConnector != nil {
		connector = existingConnector.DeepCopy()
	} else {
		connector.SetName(dcName)
		connector.SetNamespace(dataConnectorReconciler.pipelineDeployment.Spec.DeploymentNamespace)
		connector.SetLabels(labels)
	}
	connector.Spec = *newConnectorSpec

	// Set PipelineDeployment instance as the owner and controller
	if err := controllerutil.SetControllerReference(pipelineDeployment, connector, dataConnectorReconciler.scheme); err != nil {
		log.Error(err, "Failed setting the kafka connector cluster controller owner")
	}

	if err != nil && errors.IsNotFound(err) {
		err := dataConnectorReconciler.manager.GetClient().Create(context.TODO(), connector)
		if err != nil {
			log.Error(err, "Failed creating kafka connector")
			return err
		}
	} else if err != nil {
		log.Error(err, "Failed to check if kafka connector exists.")
		return err
	} else {

		patchMaker := patch.NewPatchMaker(patch.NewAnnotator("algo.run/last-applied"))
		patchResult, err := patchMaker.Calculate(existingConnector, connector)
		if err != nil {
			log.Error(err, "Failed to calculate Data Connector resource changes")
		}

		if !patchResult.IsEmpty() {
			log.Info("Data Connector Changed. Updating...")
			err := dataConnectorReconciler.manager.GetClient().Update(context.TODO(), connector)
			if err != nil {
				log.Error(err, "Failed updating Data Connector")
				return err
			}
		}

	}

	return nil

}

func (dataConnectorReconciler *DataConnectorReconciler) buildKafkaConnectorSpec() (*kafkav1alpha1.KafkaConnectorSpec, error) {

	dcSpec := dataConnectorReconciler.dataConnectorSpec

	// If Sink. need to add the source topics
	if *dcSpec.Spec.DataConnectorType == v1beta1.DATACONNECTORTYPES_SINK {
		topicName, err := dataConnectorReconciler.getDcSourceTopic(dataConnectorReconciler.pipelineDeployment, dcSpec)
		if err != nil {
			// connector wasn't created.
			log.Error(err, "Could not get sink data connector source topic.")
			return nil, err
		}
		dcSpec.Options["topics"] = topicName
	}

	spec := kafkav1alpha1.KafkaConnectorSpec{
		Class:    dcSpec.Spec.ConnectorClass,
		TasksMax: int(dcSpec.TasksMax),
		Pause:    false,
		Config:   dcSpec.Options,
	}

	return &spec, nil

}

func (dataConnectorReconciler *DataConnectorReconciler) getDcSourceTopic(pipelineDeployment *algov1beta1.PipelineDeployment,
	dataConnectorConfig *algov1beta1.DataConnectorDeploymentV1beta1) (string, error) {

	config := pipelineDeployment.Spec

	for _, pipe := range config.Pipes {

		dcName := fmt.Sprintf("%s:%s[%d]", dataConnectorConfig.Spec.Name, dataConnectorConfig.Version, dataConnectorConfig.Index)

		if pipe.DestName == dcName {

			// Get the source topic connected to this pipe
			topic := dataConnectorReconciler.allTopicConfigs[fmt.Sprintf("%s|%s", pipe.SourceName, pipe.SourceOutputName)]
			topicName := utils.GetTopicName(topic.TopicName, &pipelineDeployment.Spec)
			return topicName, nil

		}

	}

	return "", errorsbase.New(fmt.Sprintf("No topic config found for data connector source. [%s-%d]",
		dataConnectorConfig.Spec.Name,
		dataConnectorConfig.Index))

}
