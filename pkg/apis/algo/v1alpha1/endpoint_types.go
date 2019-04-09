package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// EndpointSpec defines the desired state of Endpoint
// +k8s:openapi-gen=true
type EndpointSpec struct {
	AlgoRunnerImage    string `json:"algoRunnerUrl,omitempty"`
	AlgoRunnerImageTag string `json:"algoRunnerVersion,omitempty"`
	InCluster          bool   `json:"inCluster,omitempty"`
	Namespace          string `json:"namespace,omitempty"`
	ImagePullPolicy    string `json:"imagePullPolicy,omitempty"`

	KafkaBrokers string `json:"kafkaBrokers,omitempty"`
	LogTopic     string `json:"logTopic,omitempty"`
	CommandTopic string `json:"commandTopic,omitempty"`

	LivenessInitialDelaySeconds  int32 `json:"livenessInitialDelaySeconds,omitempty"`
	LivenessTimeoutSeconds       int32 `json:"livenessTimeoutSeconds,omitempty"`
	LivenessPeriodSeconds        int32 `json:"livenessPeriodSeconds,omitempty"`
	ReadinessInitialDelaySeconds int32 `json:"readinessInitialDelaySeconds,omitempty"`
	ReadinessTimeoutSeconds      int32 `json:"readinessTimeoutSeconds,omitempty"`
	ReadinessPeriodSeconds       int32 `json:"readinessPeriodSeconds,omitempty"`

	EndpointConfig EndpointConfig `json:"endpointConfig,omitempty"`
}

// EndpointStatus defines the observed state of Endpoint
// +k8s:openapi-gen=true
type EndpointStatus struct {
	EndpointOwnerUserName  string                 `json:"endpointOwnerUserName,omitempty"`
	EndpointName           string                 `json:"endpointName,omitempty"`
	Status                 string                 `json:"status,omitempty"`
	AlgoDeploymentStatuses []AlgoDeploymentStatus `json:"algoDeploymentStatuses,omitempty"`
	AlgoPodStatuses        []AlgoPodStatus        `json:"algoPodStatuses,omitempty"`
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Endpoint is the Schema for the endpoints API
// +k8s:openapi-gen=true
type Endpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EndpointSpec   `json:"spec,omitempty"`
	Status EndpointStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EndpointList contains a list of Endpoint
type EndpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Endpoint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Endpoint{}, &EndpointList{})
}
