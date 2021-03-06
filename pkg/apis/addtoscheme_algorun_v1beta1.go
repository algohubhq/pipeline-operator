package apis

import (
	algov1beta1 "pipeline-operator/pkg/apis/algorun/v1beta1"
)

func init() {
	// Register the types with the Scheme so the components can map objects to GroupVersionKinds and back
	AddToSchemes = append(AddToSchemes, algov1beta1.SchemeBuilder.AddToScheme)
}
