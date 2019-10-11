package utilities

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("utilities")

// NewKubeUtil returns a new DeploymentUtil
func NewKubeUtil(client client.Client) KubeUtil {
	return KubeUtil{
		client: client,
	}
}

// KubeUtil some helper methods for managing kubernetes deployments
type KubeUtil struct {
	client client.Client
}

func (d *KubeUtil) CheckForDeployment(listOptions *client.ListOptions) (*appsv1.Deployment, error) {

	deploymentList := &appsv1.DeploymentList{}
	ctx := context.TODO()
	err := d.client.List(ctx, listOptions, deploymentList)

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

func (d *KubeUtil) CheckForService(listOptions *client.ListOptions) (*corev1.Service, error) {

	serviceList := &corev1.ServiceList{}
	ctx := context.TODO()
	err := d.client.List(ctx, listOptions, serviceList)

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

func (d *KubeUtil) CheckForStatefulSet(listOptions *client.ListOptions) (*appsv1.StatefulSet, error) {

	statefulSetList := &appsv1.StatefulSetList{}
	ctx := context.TODO()
	err := d.client.List(ctx, listOptions, statefulSetList)

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

func (d *KubeUtil) CheckForUnstructured(listOptions *client.ListOptions, groupVersionKind schema.GroupVersionKind) (*unstructured.Unstructured, error) {

	unstructuredList := &unstructured.UnstructuredList{}
	unstructuredList.SetGroupVersionKind(groupVersionKind)
	ctx := context.TODO()
	err := d.client.List(ctx, listOptions, unstructuredList)

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
