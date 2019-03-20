package controller

import (
	"endpoint-operator/pkg/controller/endpoint"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, endpoint.Add)
}
