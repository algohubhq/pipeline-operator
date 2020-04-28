package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KafkaConnectSpec defines the desired state of KafkaConnect
type KafkaConnectSpec struct {
	Replicas         int                         `json:"replicas,omitempty"`
	Version          string                      `json:"version,omitempty"`
	Image            string                      `json:"image,omitempty"`
	BootstrapServers string                      `json:"bootstrapServers,omitempty"`
	TLS              KafkaConnectTLS             `json:"tls,omitempty"`
	Authentication   KafkaClientAuthentication   `json:"authentication,omitempty"`
	Config           map[string]string           `json:"config,omitempty"`
	Resources        corev1.ResourceRequirements `json:"resources,omitempty"`
	LivenessProbe    corev1.Probe                `json:"livenessProbe,omitempty"`
	ReadinessProbe   corev1.Probe                `json:"readinessProbe,omitempty"`
	JvmOptions       JvmOptions                  `json:"jvmOptions,omitempty"`
	Metrics          JMXExporter                 `json:"metrics,omitempty"`
}

// KafkaConnectTemplate defines the desired state of KafkaConnectTemplate
type KafkaConnectTemplate struct {
	Deployment          ResourceTemplate            `json:"deployment,omitempty"`
	Pod                 PodTemplate                 `json:"pod,omitempty"`
	APIService          ResourceTemplate            `json:"apiService,omitempty"`
	ConnectContainer    ContainerTemplate           `json:"connectContainer,omitempty"`
	PodDisruptionBudget PodDisruptionBudgetTemplate `json:"podDisruptionBudget,omitempty"`
}

// ResourceTemplate defines the desired state of ResourceTemplate
type ResourceTemplate struct {
	Metadata MetadataTemplate `json:"metadata,omitempty"`
}

// MetadataTemplate defines the desired state of MetadataTemplate
type MetadataTemplate struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// PodTemplate defines the desired state of PodTemplate
type PodTemplate struct {
	Metadata                      MetadataTemplate            `json:"metadata,omitempty"`
	ImagePullSecrets              corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	SecurityContext               corev1.PodSecurityContext   `json:"securityContext,omitempty"`
	TerminationGracePeriodSeconds int                         `json:"terminationGracePeriodSeconds,omitempty"`
	Affinity                      corev1.Affinity             `json:"affinity,omitempty"`
	PriorityClassName             string                      `json:"priorityClassName,omitempty"`
	SchedulerName                 string                      `json:"schedulerName,omitempty"`
	Tolerations                   []corev1.Toleration         `json:"tolerations,omitempty"`
}

// ContainerTemplate defines the desired state of ContainerTemplate
type ContainerTemplate struct {
	Env []NameValue `json:"env,omitempty"`
}

// PodDisruptionBudgetTemplate defines the desired state of PodDisruptionBudgetTemplate
type PodDisruptionBudgetTemplate struct {
	Metadata       MetadataTemplate `json:"metadata,omitempty"`
	MaxUnavailable int              `json:"maxUnavailable,omitempty"`
}

// KafkaConnectTLS defines the desired state of KafkaConnectTls
type KafkaConnectTLS struct {
	TrustedCertificates []CertSecretSource `json:"trustedCertificates,omitempty"`
}

// CertSecretSource defines the desired state of CertSecretSource
type CertSecretSource struct {
	SecretName  string `json:"secretName,omitempty"`
	Certificate string `json:"certificate,omitempty"`
}

// KafkaClientAuthentication defines the desired state of KafkaClientAuthentication
type KafkaClientAuthentication struct {
	Type KafkaClientAuthenticationTypes `json:"type,omitempty"`

	CertificateAndKey CertAndKeySecretSource `json:"certificateAndKey,omitempty"`

	Username       string               `json:"username,omitempty"`
	PasswordSecret PasswordSecretSource `json:"passwordSecret,omitempty"`

	ClientID                       string              `json:"clientId,omitempty"`
	TokenEndpointURI               string              `json:"tokenEndpointUri,omitempty"`
	ClientSecret                   GenericSecretSource `json:"clientSecret,omitempty"`
	AccessToken                    GenericSecretSource `json:"accessToken,omitempty"`
	RefreshToken                   GenericSecretSource `json:"refreshToken,omitempty"`
	TLSTrustedCertificates         []CertSecretSource  `json:"tlsTrustedCertificates,omitempty"`
	DisableTLSHostnameVerification bool                `json:"disableTlsHostnameVerification,omitempty"`
	MaxTokenExpirySeconds          int                 `json:"maxTokenExpirySeconds,omitempty"`
	AccessTokenIsJwt               bool                `json:"accessTokenIsJwt,omitempty"`
}

// KafkaClientAuthenticationTypes
type KafkaClientAuthenticationTypes string

// List of ComponentTypes
const (
	KAFKA_AUTH_TYPE_TLS         KafkaClientAuthenticationTypes = "tls"
	KAFKA_AUTH_TYPE_SCRAMSHA512 KafkaClientAuthenticationTypes = "scram-sha-512"
	KAFKA_AUTH_TYPE_PLAIN       KafkaClientAuthenticationTypes = "plain"
	KAFKA_AUTH_TYPE_OAUTH       KafkaClientAuthenticationTypes = "oauth"
)

// CertAndKeySecretSource defines the desired state of CertAndKeySecretSource
type CertAndKeySecretSource struct {
	Key string `json:"key,omitempty"`
}

// PasswordSecretSource defines the desired state of PasswordSecretSource
type PasswordSecretSource struct {
	SecretName string `json:"secretName,omitempty"`
	Password   string `json:"password,omitempty"`
}

// GenericSecretSource defines the desired state of GenericSecretSource
type GenericSecretSource struct {
	Key        string `json:"key,omitempty"`
	SecretName string `json:"secretName,omitempty"`
}

// JvmOptions defines the desired state of JvmOptions
type JvmOptions struct {
	XX                   map[string]string `json:"-XX,omitempty"`
	Xms                  string            `json:"-Xms,omitempty"`
	Xmx                  string            `json:"-Xmx,omitempty"`
	GcLoggingEnabled     bool              `json:"gcLoggingEnabled,omitempty"`
	JavaSystemProperties []NameValue       `json:"javaSystemProperties,omitempty"`
}

// JMXExporter defines the desired state of JMXExporter
type JMXExporter struct {
	StartDelaySeconds         int               `json:"startDelaySeconds,omitempty"`
	HostPort                  string            `json:"hostPort,omitempty"`
	Username                  string            `json:"username,omitempty"`
	Password                  string            `json:"password,omitempty"`
	JmxUrl                    string            `json:"jmxUrl,omitempty"`
	SSL                       bool              `json:"ssl,omitempty"`
	LowercaseOutputName       bool              `json:"lowercaseOutputName,omitempty"`
	LowercaseOutputLabelNames bool              `json:"lowercaseOutputLabelNames,omitempty"`
	WhitelistObjectNames      []string          `json:"whitelistObjectNames,omitempty"`
	BlacklistObjectNames      []string          `json:"blacklistObjectNames,omitempty"`
	Rules                     []JMXExporterRule `json:"rules,omitempty"`
}

// JMXExporterRule defines the desired state of JMXExporterRule
type JMXExporterRule struct {
	Pattern     string            `json:"pattern,omitempty"`
	Name        string            `json:"name,omitempty"`
	Value       string            `json:"value,omitempty"`
	ValueFactor string            `json:"valueFactor,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Help        string            `json:"help,omitempty"`
	Type        string            `json:"type,omitempty"`
}

// NameValue defines the desired state of NameValue
type NameValue struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

// Condition defines the desired state of Condition
type Condition struct {
	Type               string `json:"type,omitempty"`
	Status             string `json:"status,omitempty"`
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Message            string `json:"message,omitempty"`
}

// ConnectorPlugin defines the observed state of ConnectorPlugin
type ConnectorPlugin struct {
	Type    string `json:"type,omitempty"`
	Version string `json:"version,omitempty"`
	Class   string `json:"class,omitempty"`
}

// KafkaConnectStatus defines the observed state of KafkaTopic
type KafkaConnectStatus struct {
	Conditions         []Condition       `json:"conditions,omitempty"`
	ObservedGeneration int               `json:"observedGeneration,omitempty"`
	URL                string            `json:"url,omitempty"`
	ConnectorPlugins   []ConnectorPlugin `json:"connectorPlugins,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KafkaConnect is the Schema for the KafkaConnect API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=kafkaconnects,scope=Namespaced
type KafkaConnect struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaConnectSpec   `json:"spec,omitempty"`
	Status KafkaConnectStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KafkaConnectList contains a list of KafkaConnect instances
type KafkaConnectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaConnect `json:"items"`
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
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=kafkaconnectors,scope=Namespaced
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
	SchemeBuilder.Register(&KafkaConnect{},
		&KafkaConnectList{},
		&KafkaConnectorList{},
		&KafkaConnectorList{})
}
