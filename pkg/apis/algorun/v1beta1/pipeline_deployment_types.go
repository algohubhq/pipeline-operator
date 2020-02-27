package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PipelineDeploymentSpec defines the desired state of the PipelineDeployment
// +k8s:openapi-gen=true
type PipelineDeploymentSpec struct {
	KafkaBrokers string       `json:"kafkaBrokers,omitempty"`
	PipelineSpec PipelineSpec `json:"pipelineSpec,omitempty"`
}

// PipelineDeploymentStatus defines the observed state of PipelineDeployment
// +k8s:openapi-gen=true
type PipelineDeploymentStatus struct {
	DeploymentOwnerUserName string               `json:"deploymentOwnerUserName,omitempty"`
	DeploymentName          string               `json:"deploymentName,omitempty"`
	Status                  string               `json:"status,omitempty"`
	ComponentStatuses       []ComponentStatus    `json:"componentStatuses,omitempty"`
	PodStatuses             []ComponentPodStatus `json:"podStatuses,omitempty"`
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineDeployment is the Schema for the PipelineDeployments API
// +kubebuilder:subresource:status
// +k8s:openapi-gen=true
type PipelineDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PipelineDeploymentSpec   `json:"spec,omitempty"`
	Status PipelineDeploymentStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineDeploymentList contains a list of PipelineDeployments
type PipelineDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PipelineDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PipelineDeployment{}, &PipelineDeploymentList{})
}
