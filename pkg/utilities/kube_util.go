package utilities

import (
	"context"
	"fmt"
	"os"
	"pipeline-operator/pkg/apis/algorun/v1beta1"
	"strings"

	algov1beta1 "pipeline-operator/pkg/apis/algorun/v1beta1"

	appsv1 "k8s.io/api/apps/v1"
	autoscalev2beta2 "k8s.io/api/autoscaling/v2beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("utilities")

// NewKubeUtil returns a new DeploymentUtil
func NewKubeUtil(client client.Client,
	request *reconcile.Request) KubeUtil {
	return KubeUtil{
		client:  client,
		request: request,
	}
}

// KubeUtil some helper methods for managing kubernetes deployments
type KubeUtil struct {
	client  client.Client
	request *reconcile.Request
}

func (d *KubeUtil) CheckForDeployment(listOptions []client.ListOption) (*appsv1.Deployment, error) {

	deploymentList := &appsv1.DeploymentList{}
	ctx := context.TODO()
	err := d.client.List(ctx, deploymentList, listOptions...)

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

func (d *KubeUtil) CreateDeployment(deployment *appsv1.Deployment) (deploymentName string, error error) {

	logData := map[string]interface{}{
		"labels": deployment.Labels,
	}

	if err := d.client.Create(context.TODO(), deployment); err != nil {
		log.WithValues("data", logData)
		log.Error(err, "Failed creating the deployment")
		return "", err
	}

	logData["name"] = deployment.GetName()
	log.WithValues("data", logData)
	log.Info("Created deployment")

	return deployment.GetName(), nil

}

func (d *KubeUtil) UpdateDeployment(deployment *appsv1.Deployment) (deploymentName string, error error) {

	logData := map[string]interface{}{
		"labels": deployment.Labels,
	}

	if err := d.client.Update(context.TODO(), deployment); err != nil {
		log.WithValues("data", logData)
		log.Error(err, "Failed updating the deployment")
		return "", err
	}

	logData["name"] = deployment.GetName()
	log.WithValues("data", logData)
	log.Info("Updated deployment")

	return deployment.GetName(), nil

}

func (d *KubeUtil) CheckForSecret(listOptions []client.ListOption) (*corev1.Secret, error) {

	secretList := &corev1.SecretList{}
	ctx := context.TODO()
	err := d.client.List(ctx, secretList, listOptions...)

	if err != nil && errors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	if len(secretList.Items) > 0 {
		return &secretList.Items[0], nil
	}

	return nil, nil

}

func (d *KubeUtil) CheckForService(listOptions []client.ListOption) (*corev1.Service, error) {

	serviceList := &corev1.ServiceList{}
	ctx := context.TODO()
	err := d.client.List(ctx, serviceList, listOptions...)

	if err != nil && errors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	if len(serviceList.Items) > 0 {
		return &serviceList.Items[0], nil
	}

	return nil, nil

}

func (d *KubeUtil) CreateService(service *corev1.Service) (serviceName string, error error) {

	logData := map[string]interface{}{
		"labels": service.Labels,
	}

	if err := d.client.Create(context.TODO(), service); err != nil {
		log.WithValues("data", logData)
		log.Error(err, "Failed creating the service")
		return "", err
	}

	logData["name"] = service.GetName()
	log.WithValues("data", logData)
	log.Info("Created service")

	return service.GetName(), nil

}

func (d *KubeUtil) CheckForConfigMap(listOptions []client.ListOption) (*corev1.ConfigMap, error) {

	configMapList := &corev1.ConfigMapList{}
	ctx := context.TODO()
	err := d.client.List(ctx, configMapList, listOptions...)

	if err != nil && errors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	if len(configMapList.Items) > 0 {
		return &configMapList.Items[0], nil
	}

	return nil, nil

}

func (d *KubeUtil) CreateConfigMap(configMap *corev1.ConfigMap) (configMapName string, error error) {

	logData := map[string]interface{}{
		"labels": configMap.Labels,
	}

	if err := d.client.Create(context.TODO(), configMap); err != nil {
		log.WithValues("data", logData)
		log.Error(err, "Failed creating the ConfigMap")
		return "", err
	}

	logData["name"] = configMap.GetName()
	log.WithValues("data", logData)
	log.Info("Created ConfigMap")

	return configMap.GetName(), nil

}

func (d *KubeUtil) UpdateConfigMap(configMap *corev1.ConfigMap) (configMapName string, error error) {

	logData := map[string]interface{}{
		"labels": configMap.Labels,
	}

	if err := d.client.Update(context.TODO(), configMap); err != nil {
		log.WithValues("data", logData)
		log.Error(err, "Failed updating the ConfigMap")
		return "", err
	}

	logData["name"] = configMap.GetName()
	log.WithValues("data", logData)
	log.Info("Updated ConfigMap")

	return configMap.GetName(), nil

}

func (d *KubeUtil) CheckForStatefulSet(listOptions []client.ListOption) (*appsv1.StatefulSet, error) {

	statefulSetList := &appsv1.StatefulSetList{}
	ctx := context.TODO()
	err := d.client.List(ctx, statefulSetList, listOptions...)

	if err != nil && errors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	if len(statefulSetList.Items) > 0 {
		return &statefulSetList.Items[0], nil
	}

	return nil, nil

}

func (d *KubeUtil) CreateStatefulSet(statefulSet *appsv1.StatefulSet) (sfName string, error error) {

	logData := map[string]interface{}{
		"labels": statefulSet.Labels,
	}

	if err := d.client.Create(context.TODO(), statefulSet); err != nil {
		log.WithValues("data", logData)
		log.Error(err, "Failed creating the StatefulSet")
		return "", err
	}

	logData["name"] = statefulSet.GetName()
	log.WithValues("data", logData)
	log.Info("Created StatefulSet")

	return statefulSet.GetName(), nil

}

func (d *KubeUtil) UpdateStatefulSet(statefulSet *appsv1.StatefulSet) (sfName string, error error) {

	logData := map[string]interface{}{
		"labels": statefulSet.Labels,
	}

	if err := d.client.Update(context.TODO(), statefulSet); err != nil {
		log.WithValues("data", logData)
		log.Error(err, "Failed updating the StatefulSet")
		return "", err
	}

	logData["name"] = statefulSet.GetName()
	log.WithValues("data", logData)
	log.Info("Updated StatefulSet")

	return statefulSet.GetName(), nil

}

func (d *KubeUtil) CheckForUnstructured(listOptions []client.ListOption, groupVersionKind schema.GroupVersionKind) (*unstructured.Unstructured, error) {

	unstructuredList := &unstructured.UnstructuredList{}
	unstructuredList.SetGroupVersionKind(groupVersionKind)
	ctx := context.TODO()
	err := d.client.List(ctx, unstructuredList, listOptions...)

	if err != nil && errors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	if len(unstructuredList.Items) > 0 {
		return &unstructuredList.Items[0], nil
	}

	return nil, nil

}

func (d *KubeUtil) CreateResourceReqs(r *v1beta1.ResourceModel) (*corev1.ResourceRequirements, error) {

	resources := &corev1.ResourceRequirements{}

	if r != nil {
		// Set Memory limits
		if r.MemoryLimitBytes > 0 {
			qty, err := resource.ParseQuantity(string(r.MemoryLimitBytes))
			if err != nil {
				return resources, err
			}
			resources.Limits.Memory().Add(qty)
		}

		if r.MemoryRequestBytes > 0 {
			qty, err := resource.ParseQuantity(string(r.MemoryRequestBytes))
			if err != nil {
				return resources, err
			}
			resources.Requests.Memory().Add(qty)
		}

		// Set CPU limits
		if r.CpuLimitMillicores > 0 {
			qty, err := resource.ParseQuantity(fmt.Sprintf("%dm", r.CpuLimitMillicores))
			if err != nil {
				return resources, err
			}
			resources.Limits.Cpu().Add(qty)
		}

		if r.CpuRequestMillicores > 0 {
			qty, err := resource.ParseQuantity(fmt.Sprintf("%dm", r.CpuRequestMillicores))
			if err != nil {
				return resources, err
			}
			resources.Requests.Cpu().Add(qty)
		}

		// Set GPU limits
		if r.GpuLimitMillicores > 0 {
			qty, err := resource.ParseQuantity(fmt.Sprintf("%dm", r.GpuLimitMillicores))
			if err != nil {
				return resources, err
			}
			if resources.Limits == nil {
				resources.Limits = make(corev1.ResourceList)
			}
			resources.Limits["nvidia.com/gpu"] = qty
		}
	}

	return resources, nil
}

func (d *KubeUtil) CheckForHorizontalPodAutoscaler(listOptions []client.ListOption) (*autoscalev2beta2.HorizontalPodAutoscaler, error) {

	hpaList := &autoscalev2beta2.HorizontalPodAutoscalerList{}
	ctx := context.TODO()
	err := d.client.List(ctx, hpaList, listOptions...)

	if err != nil && errors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	if len(hpaList.Items) > 0 {
		return &hpaList.Items[0], nil
	}

	return nil, nil

}

func (d *KubeUtil) CreateHorizontalPodAutoscaler(hpa *autoscalev2beta2.HorizontalPodAutoscaler) (sfName string, error error) {

	logData := map[string]interface{}{
		"labels": hpa.Labels,
	}

	if err := d.client.Create(context.TODO(), hpa); err != nil {
		log.WithValues("data", logData)
		log.Error(err, "Failed creating the HorizontalPodAutoscaler")
		return "", err
	}

	logData["name"] = hpa.GetName()
	log.WithValues("data", logData)
	log.Info("Created HorizontalPodAutoscaler")

	return hpa.GetName(), nil

}

func (d *KubeUtil) UpdateHorizontalPodAutoscaler(hpa *autoscalev2beta2.HorizontalPodAutoscaler) (hpaName string, error error) {

	logData := map[string]interface{}{
		"labels": hpa.Labels,
	}

	if err := d.client.Update(context.TODO(), hpa); err != nil {
		log.WithValues("data", logData)
		log.Error(err, "Failed updating the Horizontal Pod Autoscaler")
		return "", err
	}

	logData["name"] = hpa.GetName()
	log.WithValues("data", logData)
	log.Info("Updated Horizontal Pod Autoscaler")

	return hpa.GetName(), nil

}

func (d *KubeUtil) CreateHpaSpec(targetName string, labels map[string]string, pipelineDeployment *algov1beta1.PipelineDeployment, r *algov1beta1.ResourceModel) (*autoscalev2beta2.HorizontalPodAutoscaler, error) {

	name := fmt.Sprintf("%s-hpa", strings.TrimRight(Short(targetName, 20), "-"))

	var scaleMetrics []autoscalev2beta2.MetricSpec
	for _, metric := range r.ScaleMetrics {

		metricSpec := autoscalev2beta2.MetricSpec{}

		// Create the Metric target
		metricTarget := autoscalev2beta2.MetricTarget{}
		switch metric.TargetType {
		case "Utilization":
			metricTarget.Type = autoscalev2beta2.UtilizationMetricType
			metricTarget.AverageUtilization = metric.AverageUtilization
		case "Value":
			metricTarget.Type = autoscalev2beta2.ValueMetricType
			qty, _ := resource.ParseQuantity(fmt.Sprintf("%s", *metric.Value))
			metricTarget.Value = &qty
		case "AverageValue":
			metricTarget.Type = autoscalev2beta2.AverageValueMetricType
			qty, _ := resource.ParseQuantity(fmt.Sprintf("%s", *metric.AverageValue))
			metricTarget.AverageValue = &qty
		default:
			metricTarget.Type = autoscalev2beta2.UtilizationMetricType
			metricTarget.AverageUtilization = Int32p(90)
		}

		var metricSelector *metav1.LabelSelector
		if *metric.MetricSelector != "" {
			metricSelector, _ = metav1.ParseToLabelSelector(*metric.MetricSelector)
		}

		// Get the metric source type constant
		switch metric.SourceType {
		case "Resource":
			metricSpec.Type = autoscalev2beta2.ResourceMetricSourceType
			metricSpec.Resource = &autoscalev2beta2.ResourceMetricSource{
				Name:   corev1.ResourceName(*metric.ResourceName),
				Target: metricTarget,
			}
		case "Object":
			metricSpec.Type = autoscalev2beta2.ObjectMetricSourceType
			metricSpec.Object = &autoscalev2beta2.ObjectMetricSource{
				DescribedObject: autoscalev2beta2.CrossVersionObjectReference{
					APIVersion: *metric.ObjectApiVersion,
					Kind:       *metric.ObjectKind,
					Name:       *metric.ObjectName,
				},
				Target: metricTarget,
				Metric: autoscalev2beta2.MetricIdentifier{
					Name:     *metric.MetricName,
					Selector: metricSelector,
				},
			}
		case "Pods":
			metricSpec.Type = autoscalev2beta2.PodsMetricSourceType
			metricSpec.Pods = &autoscalev2beta2.PodsMetricSource{
				Metric: autoscalev2beta2.MetricIdentifier{
					Name:     *metric.MetricName,
					Selector: metricSelector,
				},
				Target: metricTarget,
			}
		case "External":
			metricSpec.Type = autoscalev2beta2.ExternalMetricSourceType
			metricSpec.External = &autoscalev2beta2.ExternalMetricSource{
				Metric: autoscalev2beta2.MetricIdentifier{
					Name:     *metric.MetricName,
					Selector: metricSelector,
				},
				Target: metricTarget,
			}
		}

		scaleMetrics = append(scaleMetrics, metricSpec)
	}

	hpa := &autoscalev2beta2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: pipelineDeployment.Namespace,
			Name:      name,
			Labels:    labels,
		},
		Spec: autoscalev2beta2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalev2beta2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       targetName,
			},
			MinReplicas: &r.MinInstances,
			MaxReplicas: r.MaxInstances,
			Metrics:     scaleMetrics,
		},
	}

	return hpa, nil

}

func (d *KubeUtil) GetStorageSecretName(pipelineSpec *v1beta1.PipelineDeploymentSpecV1beta1) (storageSecretName string, err error) {

	// Check for the specific deployment storage secret
	secretName := fmt.Sprintf("storage-%s-%s", pipelineSpec.DeploymentOwnerUserName, pipelineSpec.DeploymentName)
	secret := &corev1.Secret{}
	namespacedName := types.NamespacedName{
		Name:      secretName,
		Namespace: d.request.Namespace,
	}

	err = d.client.Get(context.TODO(), namespacedName, secret)
	if err != nil {
		if errors.IsNotFound(err) {

			secretName = "storage-global"
			// Check for the global storage secret
			namespacedName := types.NamespacedName{
				Name:      secretName,
				Namespace: d.request.Namespace,
			}

			err = d.client.Get(context.TODO(), namespacedName, secret)

			if err != nil {
				return "", err
			}

			if secret.Data["connection-string"] != nil {
				return secretName, nil
			}

		}
	}

	if secret.Data["connection-string"] != nil {
		return secretName, nil
	}

	return "", err

}

func (d *KubeUtil) CopyKafkaClusterCASecret() error {

	// Get the kafka namespace
	kafkaNamespace := os.Getenv("KAFKA-NAMESPACE")
	kafkaClusterName := os.Getenv("KAFKA-CLUSTER-NAME")
	kafkaCaSecretName := fmt.Sprintf("%s-cluster-ca-cert", kafkaClusterName)

	kafkaDeploymentSecret := &corev1.Secret{}
	namespacedDeploymentName := types.NamespacedName{
		Name:      kafkaCaSecretName,
		Namespace: d.request.Namespace,
	}

	err := d.client.Get(context.TODO(), namespacedDeploymentName, kafkaDeploymentSecret)
	if err != nil {
		if errors.IsNotFound(err) {

			// Get the secret from the kafka namespace
			kafkaSecret := &corev1.Secret{}
			namespacedName := types.NamespacedName{
				Name:      kafkaCaSecretName,
				Namespace: kafkaNamespace,
			}

			err = d.client.Get(context.TODO(), namespacedName, kafkaSecret)
			if err != nil {
				log.Error(err, fmt.Sprintf("Failed to get the Kafka Certificate Authority Secret named %s in namespace %s",
					kafkaCaSecretName, kafkaNamespace))
				return err
			}

			kafkaDeploymentSecret.SetName(kafkaCaSecretName)
			kafkaDeploymentSecret.SetNamespace(d.request.Namespace)

			kafkaDeploymentSecret.Data = kafkaSecret.Data

			err = d.client.Create(context.TODO(), kafkaDeploymentSecret)
			if err != nil {
				log.Error(err, fmt.Sprintf("Failed Creating the Deployment Kafka Certificate Authority Secret named %s in namespace %s",
					kafkaCaSecretName, d.request.Namespace))
				return err
			}

		}
	}

	return err

}
