package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Condition defines the desired state of Condition
type Condition struct {
	Type               string `json:"type,omitempty"`
	Status             string `json:"status,omitempty"`
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Message            string `json:"message,omitempty"`
}

// KafkaConnectorSpec defines the desired state of KafkaConnectorSpec
type KafkaConnectorSpec struct {
	Class    string            `json:"class,omitempty"`
	TasksMax int               `json:"tasksMax,omitempty"`
	Config   map[string]string `json:"config,omitempty"`
	Pause    bool              `json:"pause,omitempty"`
}

// KafkaConnectorStatus defines the observed state of KafkaConnector
type KafkaConnectorStatus struct {
	Conditions         []Condition       `json:"conditions,omitempty"`
	ObservedGeneration int               `json:"observedGeneration,omitempty"`
	ConnectorStatus    map[string]string `json:"connectorStatus,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KafkaConnector is the Schema for the KafkaConnector API
type KafkaConnector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaConnectorSpec   `json:"spec,omitempty"`
	Status KafkaConnectorStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KafkaConnectorList contains a list of KafkaConnector instances
type KafkaConnectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaConnector `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KafkaConnector{},
		&KafkaConnectorList{})
}
