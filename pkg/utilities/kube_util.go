package utilities

import (
	"context"
	"fmt"
	"os"
	"pipeline-operator/pkg/apis/algorun/v1beta1"
	"strings"

	algov1beta1 "pipeline-operator/pkg/apis/algorun/v1beta1"
	ambv2 "pipeline-operator/pkg/apis/getambassador/v2"

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
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("utilities")

// NewKubeUtil returns a new DeploymentUtil
func NewKubeUtil(manager manager.Manager,
	request *reconcile.Request) KubeUtil {
	return KubeUtil{
		request: request,
		manager: manager,
	}
}

// KubeUtil some helper methods for managing kubernetes deployments
type KubeUtil struct {
	manager manager.Manager
	request *reconcile.Request
}

func (d *KubeUtil) CheckForDeployment(listOptions []client.ListOption) (*appsv1.Deployment, error) {

	deploymentList := &appsv1.DeploymentList{}
	ctx := context.TODO()
	err := d.manager.GetClient().List(ctx, deploymentList, listOptions...)

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

	if err := d.manager.GetClient().Create(context.TODO(), deployment); err != nil {
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

	if err := d.manager.GetClient().Update(context.TODO(), deployment); err != nil {
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
	err := d.manager.GetClient().List(ctx, secretList, listOptions...)

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
	err := d.manager.GetClient().List(ctx, serviceList, listOptions...)

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

	if err := d.manager.GetClient().Create(context.TODO(), service); err != nil {
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
	err := d.manager.GetClient().List(ctx, configMapList, listOptions...)

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

	if err := d.manager.GetClient().Create(context.TODO(), configMap); err != nil {
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

	if err := d.manager.GetClient().Update(context.TODO(), configMap); err != nil {
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
	err := d.manager.GetClient().List(ctx, statefulSetList, listOptions...)

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

	if err := d.manager.GetClient().Create(context.TODO(), statefulSet); err != nil {
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

	if err := d.manager.GetClient().Update(context.TODO(), statefulSet); err != nil {
		log.WithValues("data", logData)
		log.Error(err, "Failed updating the StatefulSet")
		return "", err
	}

	logData["name"] = statefulSet.GetName()
	log.WithValues("data", logData)
	log.Info("Updated StatefulSet")

	return statefulSet.GetName(), nil

}

func (d *KubeUtil) CheckForAlgoCR(listOptions []client.ListOption) (*v1beta1.Algo, error) {

	algoList := &v1beta1.AlgoList{}
	algoList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "algo.run",
		Kind:    "Algo",
		Version: "v1beta1",
	})

	ctx := context.TODO()
	// Use the APIReader to query across namespaces
	err := d.manager.GetAPIReader().List(ctx, algoList, listOptions...)

	if err != nil && errors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	if len(algoList.Items) > 0 {
		return &algoList.Items[0], nil
	}

	return nil, nil

}

func (d *KubeUtil) CheckForDataConnectorCR(listOptions []client.ListOption) (*v1beta1.DataConnector, error) {

	dcList := &v1beta1.DataConnectorList{}
	dcList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "algo.run",
		Kind:    "DataConnector",
		Version: "v1beta1",
	})

	ctx := context.TODO()
	// Use the APIReader to query across namespaces
	err := d.manager.GetAPIReader().List(ctx, dcList, listOptions...)

	if err != nil && errors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	if len(dcList.Items) > 0 {
		return &dcList.Items[0], nil
	}

	return nil, nil

}

func (d *KubeUtil) CheckForUnstructured(listOptions []client.ListOption, groupVersionKind schema.GroupVersionKind) (*unstructured.Unstructured, error) {

	unstructuredList := &unstructured.UnstructuredList{}
	unstructuredList.SetGroupVersionKind(groupVersionKind)
	ctx := context.TODO()
	err := d.manager.GetClient().List(ctx, unstructuredList, listOptions...)

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

func (d *KubeUtil) CheckForAmbassadorMapping(listOptions []client.ListOption) (*ambv2.Mapping, error) {

	list := &ambv2.MappingList{}
	ctx := context.TODO()
	err := d.manager.GetClient().List(ctx, list, listOptions...)

	if err != nil && errors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}

	return nil, nil

}

func (d *KubeUtil) CreateResourceReqs(r *v1beta1.ResourceRequirementsV1) (*corev1.ResourceRequirements, error) {

	resources := &corev1.ResourceRequirements{}

	if r != nil {
		// Set Memory limits
		if r.Limits["memory"] != "" {
			qty, err := resource.ParseQuantity(r.Limits["memory"])
			if err != nil {
				return resources, err
			}
			resources.Limits.Memory().Add(qty)
		}

		if r.Requests["memory"] != "" {
			qty, err := resource.ParseQuantity(r.Requests["memory"])
			if err != nil {
				return resources, err
			}
			resources.Requests.Memory().Add(qty)
		}

		// Set CPU limits
		if r.Limits["cpu"] != "" {
			qty, err := resource.ParseQuantity(r.Limits["cpu"])
			if err != nil {
				return resources, err
			}
			resources.Limits.Cpu().Add(qty)
		}

		if r.Requests["cpu"] != "" {
			qty, err := resource.ParseQuantity(r.Requests["cpu"])
			if err != nil {
				return resources, err
			}
			resources.Requests.Cpu().Add(qty)
		}

		// Set GPU limits
		if r.Limits["nvidia.com/gpu"] != "" {
			qty, err := resource.ParseQuantity(r.Limits["nvidia.com/gpu"])
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
	err := d.manager.GetClient().List(ctx, hpaList, listOptions...)

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

	if err := d.manager.GetClient().Create(context.TODO(), hpa); err != nil {
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

	if err := d.manager.GetClient().Update(context.TODO(), hpa); err != nil {
		log.WithValues("data", logData)
		log.Error(err, "Failed updating the Horizontal Pod Autoscaler")
		return "", err
	}

	logData["name"] = hpa.GetName()
	log.WithValues("data", logData)
	log.Info("Updated Horizontal Pod Autoscaler")

	return hpa.GetName(), nil

}

func (d *KubeUtil) CreateHpaSpec(targetName string, labels map[string]string, pipelineDeployment *algov1beta1.PipelineDeployment, autoscale *algov1beta1.AutoScalingSpec) (*autoscalev2beta2.HorizontalPodAutoscaler, error) {

	name := fmt.Sprintf("%s-hpa", strings.TrimRight(Short(targetName, 20), "-"))

	var scaleMetrics []autoscalev2beta2.MetricSpec
	for _, metric := range autoscale.Metrics {

		metricSpec := autoscalev2beta2.MetricSpec{}

		// Create the Metric target
		metricTarget := autoscalev2beta2.MetricTarget{}
		switch *metric.TargetType {
		case "Utilization":
			metricTarget.Type = autoscalev2beta2.UtilizationMetricType
			metricTarget.AverageUtilization = &metric.AverageUtilization
		case "Value":
			metricTarget.Type = autoscalev2beta2.ValueMetricType
			qty, _ := resource.ParseQuantity(fmt.Sprintf("%s", metric.Value))
			metricTarget.Value = &qty
		case "AverageValue":
			metricTarget.Type = autoscalev2beta2.AverageValueMetricType
			qty, _ := resource.ParseQuantity(fmt.Sprintf("%s", metric.AverageValue))
			metricTarget.AverageValue = &qty
		default:
			metricTarget.Type = autoscalev2beta2.UtilizationMetricType
			metricTarget.AverageUtilization = Int32p(90)
		}

		var metricSelector *metav1.LabelSelector
		if metric.MetricSelector != "" {
			metricSelector, _ = metav1.ParseToLabelSelector(metric.MetricSelector)
		}

		// Get the metric source type constant
		switch *metric.SourceType {
		case "Resource":
			metricSpec.Type = autoscalev2beta2.ResourceMetricSourceType
			metricSpec.Resource = &autoscalev2beta2.ResourceMetricSource{
				Name:   corev1.ResourceName(metric.ResourceName),
				Target: metricTarget,
			}
		case "Object":
			metricSpec.Type = autoscalev2beta2.ObjectMetricSourceType
			metricSpec.Object = &autoscalev2beta2.ObjectMetricSource{
				DescribedObject: autoscalev2beta2.CrossVersionObjectReference{
					APIVersion: metric.ObjectApiVersion,
					Kind:       metric.ObjectKind,
					Name:       metric.ObjectName,
				},
				Target: metricTarget,
				Metric: autoscalev2beta2.MetricIdentifier{
					Name:     metric.MetricName,
					Selector: metricSelector,
				},
			}
		case "Pods":
			metricSpec.Type = autoscalev2beta2.PodsMetricSourceType
			metricSpec.Pods = &autoscalev2beta2.PodsMetricSource{
				Metric: autoscalev2beta2.MetricIdentifier{
					Name:     metric.MetricName,
					Selector: metricSelector,
				},
				Target: metricTarget,
			}
		case "External":
			metricSpec.Type = autoscalev2beta2.ExternalMetricSourceType
			metricSpec.External = &autoscalev2beta2.ExternalMetricSource{
				Metric: autoscalev2beta2.MetricIdentifier{
					Name:     metric.MetricName,
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
			MinReplicas: &autoscale.MinReplicas,
			MaxReplicas: autoscale.MaxReplicas,
			Metrics:     scaleMetrics,
		},
	}

	return hpa, nil

}

func (d *KubeUtil) GetStorageSecretName(pipelineSpec *v1beta1.PipelineDeploymentSpecV1beta1) (storageSecretName string, err error) {

	// Set the default to "storage-{deploymentowner}-{deploymentname}"
	secretName := fmt.Sprintf("storage-%s-%s", pipelineSpec.DeploymentOwner, pipelineSpec.DeploymentName)
	secretKey := "connection-string"

	// Use env var if set
	secretNameEnv := os.Getenv("STORAGE_SECRET_NAME")
	if secretNameEnv != "" {
		secretName = strings.Replace(secretNameEnv, "{deploymentowner}", pipelineSpec.DeploymentOwner, -1)
		secretName = strings.Replace(secretName, "{deploymentname}", pipelineSpec.DeploymentName, -1)
		secretName = strings.ToLower(secretName)
	}

	// Use env var if set
	secretNameKeyEnv := os.Getenv("STORAGE_SECRET_KEY")
	if secretNameKeyEnv != "" {
		secretKey = secretNameKeyEnv
	}

	// Check for the specific deployment storage secret
	secret := &corev1.Secret{}
	namespacedName := types.NamespacedName{
		Name:      secretName,
		Namespace: pipelineSpec.DeploymentNamespace,
	}

	err = d.manager.GetClient().Get(context.TODO(), namespacedName, secret)
	if err != nil {
		if errors.IsNotFound(err) {

			secretName = "storage-global"
			// Check for the global storage secret
			namespacedName := types.NamespacedName{
				Name:      secretName,
				Namespace: pipelineSpec.DeploymentNamespace,
			}

			err = d.manager.GetClient().Get(context.TODO(), namespacedName, secret)

			if err != nil {
				return "", err
			}

			if secret.Data[secretKey] != nil {
				return secretName, nil
			}

		}
	}

	if secret.Data[secretKey] != nil {
		return secretName, nil
	}

	return "", err

}

func (d *KubeUtil) GetListOptionsFromRef(crRef *algov1beta1.CustomResourceRefV1beta1) []client.ListOption {

	opts := []client.ListOption{}

	if len(crRef.MatchLabels) != 0 {
		matchLabels := client.MatchingLabels{}
		for key, value := range crRef.MatchLabels {
			matchLabels[key] = value
		}
		nsOpt := []client.ListOption{
			matchLabels,
		}
		opts = append(opts, nsOpt...)
	}

	if crRef.Name != "" {
		nameOpt := []client.ListOption{
			client.MatchingFields{
				"metadata.name": crRef.Name,
			},
		}
		opts = append(opts, nameOpt...)
	}

	return opts

}
