// NOTE: Boilerplate only.  Ignore this file.

// NOTE: Added +kubebuilder:skip to stop generation of the Strimzi CRDs as they are managed by strimzi
// Package v1alpha1 contains API Schema definitions for the strimzi kafka v1alpha1 API group
// +k8s:deepcopy-gen=package,register
// +groupName=kafka.strimzi.io
// +kubebuilder:skip
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: "kafka.strimzi.io", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}
)
