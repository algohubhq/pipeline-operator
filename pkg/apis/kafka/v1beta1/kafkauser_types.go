package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KafkaUserSpec is the Schema for the Strimzi KafkaUser API
type KafkaUserSpec struct {
	Authentication KakfaUserAuthentication `json:"authentication,omitempty"`
	Authorization  KakfaUserAuthorization  `json:"authorization,omitempty"`
}

// KakfaUserAuthentication is the Schema for the Strimzi KafkaUser API
type KakfaUserAuthentication struct {
	Type string `json:"type,omitempty"`
}

// KakfaUserAuthorization is the Schema for the Strimzi KafkaUser API
type KakfaUserAuthorization struct {
	Type string         `json:"type,omitempty"`
	Acls []KakfaUserAcl `json:"acls,omitempty"`
}

// KakfaUserAcl is the Schema for the Strimzi KafkaUser API
type KakfaUserAcl struct {
	Operation string               `json:"operation,omitempty"`
	Resource  KakfaUserAclResource `json:"resource,omitempty"`
}

// KakfaUserAclResource is the Schema for the Strimzi KafkaUser API
type KakfaUserAclResource struct {
	Type        string `json:"type,omitempty"`
	Name        string `json:"name,omitempty"`
	PatternType string `json:"patternType,omitempty"`
}

// KafkaUserStatus defines the observed state of KafkaUser
type KafkaUserStatus struct {
	Conditions         []Condition `json:"conditions,omitempty"`
	ObservedGeneration int         `json:"observedGeneration,omitempty"`
	Username           string      `json:"username,omitempty"`
	Secret             string      `json:"secret,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KafkaUser is the Schema for the kafkausers API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=kafkausers,scope=Namespaced
type KafkaUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaUserSpec   `json:"spec,omitempty"`
	Status KafkaUserStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KafkaUserList contains a list of KafkaUser
type KafkaUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaUser `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KafkaUser{}, &KafkaUserList{})
}
