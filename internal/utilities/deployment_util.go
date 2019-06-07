package utilities

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("utilities")

// NewDeploymentUtil returns a new DeploymentUtil
func NewDeploymentUtil(client client.Client) DeploymentUtil {
	return DeploymentUtil{
		client: client,
	}
}

// DeploymentUtil some helper methods for managing kubernetes deployments
type DeploymentUtil struct {
	client client.Client
}

func (d *DeploymentUtil) CheckForDeployment(listOptions *client.ListOptions) (*appsv1.Deployment, error) {

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

func (d *DeploymentUtil) CreateDeployment(deployment *appsv1.Deployment) error {

	logData := map[string]interface{}{
		"labels": deployment.Labels,
	}

	if err := d.client.Create(context.TODO(), deployment); err != nil {
		log.WithValues("data", logData)
		log.Error(err, "Failed creating the deployment")
		return err
	}

	logData["name"] = deployment.GetName()
	log.WithValues("data", logData)
	log.Info("Created deployment")

	return nil

}

func (d *DeploymentUtil) UpdateDeployment(deployment *appsv1.Deployment) error {

	logData := map[string]interface{}{
		"labels": deployment.Labels,
	}

	if err := d.client.Update(context.TODO(), deployment); err != nil {
		log.WithValues("data", logData)
		log.Error(err, "Failed updating the deployment")
		return err
	}

	logData["name"] = deployment.GetName()
	log.WithValues("data", logData)
	log.Info("Updated deployment")

	return nil

}
