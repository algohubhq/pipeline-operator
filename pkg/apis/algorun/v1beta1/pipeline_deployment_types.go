package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PipelineDeploymentStatus defines the observed state of PipelineDeployment
// +k8s:openapi-gen=true
type PipelineDeploymentStatus struct {
	DeploymentOwner   string               `json:"deploymentOwner,omitempty"`
	DeploymentName    string               `json:"deploymentName,omitempty"`
	Status            string               `json:"status,omitempty"`
	ComponentStatuses []ComponentStatus    `json:"componentStatuses,omitempty"`
	PodStatuses       []ComponentPodStatus `json:"podStatuses,omitempty"`
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineDeployment is the Schema for the PipelineDeployments API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=pipelinedeployments,scope=Namespaced
// +k8s:openapi-gen=true
type PipelineDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PipelineDeploymentSpecV1beta1 `json:"spec,omitempty"`
	Status PipelineDeploymentStatus      `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineDeploymentList contains a list of PipelineDeployments
type PipelineDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PipelineDeployment `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Algo is the Schema for the Algo API
// +kubebuilder:resource:path=algos,scope=Cluster
// +k8s:openapi-gen=true
type Algo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              AlgoSpecV1beta1 `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AlgoList contains a list of Algos
type AlgoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Algo `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DataConnector is the Schema for the DataConnector API
// +kubebuilder:resource:path=dataconnectors,scope=Cluster
// +k8s:openapi-gen=true
type DataConnector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DataConnectorSpecV1beta1 `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DataConnectorList contains a list of DataConnectors
type DataConnectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DataConnector `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PipelineDeployment{},
		&PipelineDeploymentList{},
		&Algo{},
		&AlgoList{},
		&DataConnector{},
		&DataConnectorList{})
}
