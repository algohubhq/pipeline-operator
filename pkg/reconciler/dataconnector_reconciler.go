package reconciler

import (
	"context"
	errorsbase "errors"
	"fmt"
	"pipeline-operator/pkg/apis/algorun/v1beta1"
	algov1beta1 "pipeline-operator/pkg/apis/algorun/v1beta1"
	kafkav1beta1 "pipeline-operator/pkg/apis/kafka/v1beta1"
	utils "pipeline-operator/pkg/utilities"
	"strconv"
	"strings"

	kc "github.com/go-kafka/connect"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewDataConnectorReconciler returns a new DataConnectorReconciler
func NewDataConnectorReconciler(pipelineDeployment *algov1beta1.PipelineDeployment,
	dataConnectorSpec *v1beta1.DataConnectorDeploymentV1beta1,
	allTopicConfigs map[string]*v1beta1.TopicConfigModel,
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
		pipelineDeployment: pipelineDeployment,
		dataConnectorSpec:  dataConnectorSpec,
		allTopicConfigs:    allTopicConfigs,
		request:            request,
		manager:            manager,
		scheme:             scheme,
	}, nil
}

// DataConnectorReconciler reconciles an dataConnectorConfig object
type DataConnectorReconciler struct {
	pipelineDeployment         *algov1beta1.PipelineDeployment
	dataConnectorSpec          *v1beta1.DataConnectorDeploymentV1beta1
	activeDataConnectorVersion *v1beta1.DataConnectorVersionModel
	allTopicConfigs            map[string]*v1beta1.TopicConfigModel
	request                    *reconcile.Request
	manager                    manager.Manager
	scheme                     *runtime.Scheme
}

// Reconcile creates or updates the data connector for the pipelineDeployment
func (dataConnectorReconciler *DataConnectorReconciler) Reconcile() error {

	pipelineDeployment := dataConnectorReconciler.pipelineDeployment
	dataConnectorConfig := dataConnectorReconciler.dataConnectorSpec

	// Creat the Kafka Topics
	log.Info("Reconciling Kakfa Topics for Data Connector outputs")
	for _, topic := range dataConnectorConfig.Topics {
		dcName := utils.GetDcFullName(dataConnectorConfig)
		go func(currentTopicConfig algov1beta1.TopicConfigModel) {
			topicReconciler := NewTopicReconciler(dataConnectorReconciler.pipelineDeployment,
				dcName,
				&currentTopicConfig,
				dataConnectorReconciler.request,
				dataConnectorReconciler.manager,
				dataConnectorReconciler.scheme)
			topicReconciler.Reconcile()
		}(topic)
	}

	kcName := strings.ToLower(fmt.Sprintf("%s-%s", pipelineDeployment.Spec.DeploymentName, dataConnectorConfig.Spec.Name))
	dcName := strings.ToLower(fmt.Sprintf("%s-%s-%d", pipelineDeployment.Spec.DeploymentName, dataConnectorConfig.Spec.Name, dataConnectorConfig.Index))
	// Set the image name
	var imageName string
	if dataConnectorReconciler.activeDataConnectorVersion.Image == nil {
		err := errorsbase.New("Data Connector Image is empty")
		log.Error(err,
			fmt.Sprintf("Data Connector image cannot be empty for [%s]", dataConnectorConfig.Spec.Name))
		return err
	}
	if dataConnectorReconciler.activeDataConnectorVersion.Image.Tag == "" || dataConnectorReconciler.activeDataConnectorVersion.Image.Tag == "latest" {
		imageName = fmt.Sprintf("%s:latest", dataConnectorReconciler.activeDataConnectorVersion.Image.Repository)
	} else {
		imageName = fmt.Sprintf("%s:%s", dataConnectorReconciler.activeDataConnectorVersion.Image.Repository, dataConnectorReconciler.activeDataConnectorVersion.Image.Tag)
	}
	// dcName := strings.TrimRight(utils.Short(dcName, 20), "-")
	// check to see if data connector already exists
	existingDc := &kafkav1beta1.KafkaConnect{}
	existingDc.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kafka.strimzi.io",
		Kind:    "KafkaConnect",
		Version: "v1beta1",
	})
	err := dataConnectorReconciler.manager.GetClient().Get(context.TODO(),
		types.NamespacedName{
			Name:      kcName,
			Namespace: dataConnectorReconciler.pipelineDeployment.Spec.DeploymentNamespace,
		},
		existingDc)

	if err != nil && errors.IsNotFound(err) {
		// Create the connector cluster
		// Using a unstructured object to submit a data connector.

		labels := map[string]string{
			"app.kubernetes.io/part-of":    "algo.run",
			"app.kubernetes.io/component":  "dataconnector",
			"app.kubernetes.io/managed-by": "pipeline-operator",
			"algo.run/pipeline-deployment": fmt.Sprintf("%s.%s", pipelineDeployment.Spec.DeploymentOwner,
				pipelineDeployment.Spec.DeploymentName),
			"algo.run/pipeline": fmt.Sprintf("%s.%s", pipelineDeployment.Spec.PipelineOwner,
				pipelineDeployment.Spec.PipelineName),
			"algo.run/dataconnector":         dataConnectorConfig.Spec.Name,
			"algo.run/dataconnector-version": dataConnectorConfig.Version,
			"algo.run/index":                 strconv.Itoa(int(dataConnectorConfig.Index)),
		}

		newDc := &kafkav1beta1.KafkaConnect{}
		newDc.Spec = kafkav1beta1.KafkaConnectSpec{
			Version:          "2.4.0",
			Replicas:         int(dataConnectorConfig.Replicas),
			Image:            imageName,
			BootstrapServers: pipelineDeployment.Spec.KafkaBrokers,
			Metrics: kafkav1beta1.JMXExporter{
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

		newDc.SetName(kcName)
		newDc.SetNamespace(dataConnectorReconciler.pipelineDeployment.Spec.DeploymentNamespace)
		newDc.SetLabels(labels)
		newDc.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "kafka.strimzi.io",
			Kind:    "KafkaConnect",
			Version: "v1beta1",
		})

		// Set PipelineDeployment instance as the owner and controller
		if err := controllerutil.SetControllerReference(pipelineDeployment, newDc, dataConnectorReconciler.scheme); err != nil {
			log.Error(err, "Failed setting the kafka connect cluster controller owner")
		}

		err := dataConnectorReconciler.manager.GetClient().Create(context.TODO(), newDc)
		if err != nil {
			log.Error(err, "Failed creating kafka connect cluster")
		}

	} else if err != nil {
		log.Error(err, "Failed to check if kafka connect cluster exists.")
	} else {

		log.Info("The existing connector", "connector", existingDc)
		// TODO: Get the deployment and ensure the status is running

		// If the cluster node exists, then check if connector exists
		// Use the dns name of the connector cluster
		host := fmt.Sprintf("http://%s-connect-api.%s:8083", kcName, dataConnectorReconciler.pipelineDeployment.Spec.DeploymentNamespace)
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

			dcConfig["connector.class"] = dataConnectorConfig.Spec.ConnectorClass
			dcConfig["tasks.max"] = strconv.Itoa(int(dataConnectorConfig.TasksMax))

			// If Sink. need to add the source topics
			if *dataConnectorConfig.Spec.DataConnectorType == v1beta1.DATACONNECTORTYPES_SINK {
				topicName, err := dataConnectorReconciler.getDcSourceTopic(pipelineDeployment, dataConnectorConfig)
				dcConfig["topics"] = topicName

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
