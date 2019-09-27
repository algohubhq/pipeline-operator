package controller

import (
	pipelinedeploymentcontroller "pipeline-operator/pkg/controller/pipeline_deployment"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, pipelinedeploymentcontroller.Add)
}
