package reconciler

import (
	"context"
	errorsbase "errors"
	"fmt"
	utils "pipeline-operator/internal/utilities"
	"pipeline-operator/pkg/apis/algo/v1alpha1"
	algov1alpha1 "pipeline-operator/pkg/apis/algo/v1alpha1"
	"strconv"
	"strings"

	kc "github.com/go-kafka/connect"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewDataConnectorReconciler returns a new DataConnectorReconciler
func NewDataConnectorReconciler(pipelineDeployment *algov1alpha1.PipelineDeployment,
	dataConnectorConfig *v1alpha1.DataConnectorConfig,
	request *reconcile.Request,
	client client.Client,
	scheme *runtime.Scheme) DataConnectorReconciler {
	return DataConnectorReconciler{
		pipelineDeployment:  pipelineDeployment,
		dataConnectorConfig: dataConnectorConfig,
		request:             request,
		client:              client,
		scheme:              scheme,
	}
}

// DataConnectorReconciler reconciles an dataConnectorConfig object
type DataConnectorReconciler struct {
	pipelineDeployment  *algov1alpha1.PipelineDeployment
	dataConnectorConfig *v1alpha1.DataConnectorConfig
	request             *reconcile.Request
	client              client.Client
	scheme              *runtime.Scheme
}

// Reconcile creates or updates the data connector for the pipelineDeployment
func (dataConnectorReconciler *DataConnectorReconciler) Reconcile() error {

	pipelineDeployment := dataConnectorReconciler.pipelineDeployment
	dataConnectorConfig := dataConnectorReconciler.dataConnectorConfig

	kcName := strings.ToLower(fmt.Sprintf("%s-%s", pipelineDeployment.Spec.PipelineSpec.DeploymentName, dataConnectorConfig.Name))
	dcName := strings.ToLower(fmt.Sprintf("%s-%s-%d", pipelineDeployment.Spec.PipelineSpec.DeploymentName, dataConnectorConfig.Name, dataConnectorConfig.Index))
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
	err := dataConnectorReconciler.client.Get(context.TODO(), types.NamespacedName{Name: kcName, Namespace: dataConnectorReconciler.request.NamespacedName.Namespace}, existingDc)

	if err != nil && errors.IsNotFound(err) {
		// Create the connector cluster
		// Using a unstructured object to submit a strimzi topic creation.
		newDc := &unstructured.Unstructured{}
		newDc.Object = map[string]interface{}{
			"name":      kcName,
			"namespace": dataConnectorReconciler.request.NamespacedName.Namespace,
			"spec": map[string]interface{}{
				"version":          "2.1.0",
				"replicas":         1,
				"image":            imageName,
				"bootstrapServers": pipelineDeployment.Spec.KafkaBrokers,
				"metrics": map[string]interface{}{
					"lowercaseOutputName":       true,
					"lowercaseOutputLabelNames": true,
					"rules": []map[string]interface{}{
						map[string]interface{}{
							"pattern": "kafka.connect<type=connect-worker-metrics>([^:]+):",
							"name":    "kafka_connect_connect_worker_metrics_$1",
						},
						map[string]interface{}{
							"pattern": "kafka.connect<type=connect-metrics, client-id=([^:]+)><>([^:]+)",
							"name":    "kafka_connect_connect_metrics_$1_$2",
						},
					},
				},
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
		newDc.SetNamespace(dataConnectorReconciler.request.NamespacedName.Namespace)
		newDc.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "kafka.strimzi.io",
			Kind:    "KafkaConnect",
			Version: "v1alpha1",
		})

		// Set PipelineDeployment instance as the owner and controller
		if err := controllerutil.SetControllerReference(pipelineDeployment, newDc, dataConnectorReconciler.scheme); err != nil {
			log.Error(err, "Failed setting the kafka connect cluster controller owner")
		}

		err := dataConnectorReconciler.client.Create(context.TODO(), newDc)
		if err != nil {
			log.Error(err, "Failed creating kafka connect cluster")
		} else {
			utils.DataConnectorCountGuage.Add(1)
		}

	} else if err != nil {
		log.Error(err, "Failed to check if kafka connect cluster exists.")
	} else {

		log.Info("The existing connector", "connector", existingDc)
		// TODO: Get the deployment and ensure the status is running

		// If the cluster node exists, then check if connector exists
		// Use the dns name of the connector cluster
		host := fmt.Sprintf("http://%s-connect-api.%s:8083", kcName, dataConnectorReconciler.request.NamespacedName.Namespace)
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
				topicConfig, err := dataConnectorReconciler.getDcSourceTopic(pipelineDeployment, dataConnectorConfig)
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

func (dataConnectorReconciler *DataConnectorReconciler) getDcSourceTopic(pipelineDeployment *algov1alpha1.PipelineDeployment, dataConnectorConfig *algov1alpha1.DataConnectorConfig) (TopicConfig, error) {

	config := pipelineDeployment.Spec.PipelineSpec

	for _, pipe := range config.Pipes {

		dcName := fmt.Sprintf("%s:%s[%d]", dataConnectorConfig.Name, dataConnectorConfig.VersionTag, dataConnectorConfig.Index)

		if pipe.DestName == dcName {

			// Get the source topic connected to this pipe
			for _, topic := range config.TopicConfigs {
				if pipe.SourceName == topic.SourceName &&
					pipe.SourceOutputName == topic.SourceOutputName {
					newTopicConfig, err := BuildTopic(config, &topic)
					if err != nil {
						return newTopicConfig, err
					}
					return newTopicConfig, nil
				}
			}

		}

	}

	return TopicConfig{}, errorsbase.New(fmt.Sprintf("No topic config found for data connector source. [%s-%d]",
		dataConnectorConfig.Name,
		dataConnectorConfig.Index))

}
