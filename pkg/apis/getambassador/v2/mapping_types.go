package v2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CircuitBreakersItems
type CircuitBreakersItems struct {
	MaxConnections     int    `json:"max_connections,omitempty"`
	MaxPendingRequests int    `json:"max_pending_requests,omitempty"`
	MaxRequests        int    `json:"max_requests,omitempty"`
	MaxRetries         int    `json:"max_retries,omitempty"`
	Priority           string `json:"priority,omitempty"`
}

// Cookie
type Cookie struct {
	Name string `json:"name"`
	Path string `json:"path,omitempty"`
	TTL  string `json:"ttl,omitempty"`
}

// Cors
type Cors struct {
	Credentials    bool     `json:"credentials,omitempty"`
	ExposedHeaders []string `json:"exposed_headers,omitempty"`
	Headers        []string `json:"headers,omitempty"`
	MaxAge         string   `json:"max_age,omitempty"`
	Methods        []string `json:"methods,omitempty"`
	Origins        []string `json:"origins,omitempty"`
}

// EnvoyOverride
type EnvoyOverride struct {
}

// Labels
type Labels struct {
}

// LoadBalancer
type LoadBalancer struct {
	Cookie   *Cookie `json:"cookie,omitempty"`
	Header   string  `json:"header,omitempty"`
	Policy   string  `json:"policy"`
	SourceIp bool    `json:"source_ip,omitempty"`
}

// ModulesItems
type ModulesItems struct {
}

// RetryPolicy
type RetryPolicy struct {
	NumRetries    int    `json:"num_retries,omitempty"`
	PerTryTimeout string `json:"per_try_timeout,omitempty"`
	RetryOn       string `json:"retry_on,omitempty"`
}

// MappingSpec defines the desired state of Mapping
type MappingSpec struct {
	AddLinkerdHeaders     bool                    `json:"add_linkerd_headers,omitempty"`
	AddRequestHeaders     map[string]string       `json:"add_request_headers,omitempty"`
	AddResponseHeaders    map[string]string       `json:"add_response_headers,omitempty"`
	AutoHostRewrite       bool                    `json:"auto_host_rewrite,omitempty"`
	BypassAuth            bool                    `json:"bypass_auth,omitempty"`
	CaseSensitive         bool                    `json:"case_sensitive,omitempty"`
	CircuitBreakers       []*CircuitBreakersItems `json:"circuit_breakers,omitempty"`
	ClusterIdleTimeoutMs  int                     `json:"cluster_idle_timeout_ms,omitempty"`
	ClusterTag            string                  `json:"cluster_tag,omitempty"`
	ConnectTimeoutMs      int                     `json:"connect_timeout_ms,omitempty"`
	Cors                  *Cors                   `json:"cors,omitempty"`
	EnableIpv4            bool                    `json:"enable_ipv4,omitempty"`
	EnableIpv6            bool                    `json:"enable_ipv6,omitempty"`
	EnvoyOverride         *EnvoyOverride          `json:"envoy_override,omitempty"`
	Grpc                  bool                    `json:"grpc,omitempty"`
	Headers               map[string]string       `json:"headers,omitempty"`
	Host                  string                  `json:"host,omitempty"`
	HostRedirect          bool                    `json:"host_redirect,omitempty"`
	HostRegex             bool                    `json:"host_regex,omitempty"`
	HostRewrite           string                  `json:"host_rewrite,omitempty"`
	IdleTimeoutMs         int                     `json:"idle_timeout_ms,omitempty"`
	LoadBalancer          *LoadBalancer           `json:"load_balancer,omitempty"`
	Method                string                  `json:"method,omitempty"`
	MethodRegex           bool                    `json:"method_regex,omitempty"`
	Modules               []*ModulesItems         `json:"modules,omitempty"`
	OutlierDetection      string                  `json:"outlier_detection,omitempty"`
	PathRedirect          string                  `json:"path_redirect,omitempty"`
	Precedence            int                     `json:"precedence,omitempty"`
	Prefix                string                  `json:"prefix"`
	PrefixExact           bool                    `json:"prefix_exact,omitempty"`
	PrefixRegex           bool                    `json:"prefix_regex,omitempty"`
	Priority              string                  `json:"priority,omitempty"`
	RegexHeaders          map[string]string       `json:"regex_headers,omitempty"`
	RemoveRequestHeaders  []string                `json:"remove_request_headers,omitempty"`
	RemoveResponseHeaders []string                `json:"remove_response_headers,omitempty"`
	Resolver              string                  `json:"resolver,omitempty"`
	RetryPolicy           *RetryPolicy            `json:"retry_policy,omitempty"`
	Rewrite               string                  `json:"rewrite,omitempty"`
	Service               string                  `json:"service"`
	Shadow                bool                    `json:"shadow,omitempty"`
	TimeoutMs             int                     `json:"timeout_ms,omitempty"`
	TLS                   bool                    `json:"tls,omitempty"`
	UseWebsocket          bool                    `json:"use_websocket,omitempty"`
	Weight                int                     `json:"weight,omitempty"`
}

// MappingStatus defines the observed state of Mapping
type MappingStatus struct {
	State string `json:"state,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Mapping is the Schema for the mappings API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=mappings,scope=Namespaced
type Mapping struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MappingSpec   `json:"spec,omitempty"`
	Status MappingStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MappingList contains a list of Mapping
type MappingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Mapping `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Mapping{}, &MappingList{})
}
