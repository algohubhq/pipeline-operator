package v1beta1

// KafkaClientAuthentication defines the desired state of KafkaClientAuthentication
type KafkaClientAuthentication struct {
	Type KafkaClientAuthenticationTypes `json:"type,omitempty"`

	CertificateAndKey *SecretSource `json:"certificateAndKey,omitempty"`

	Username       string        `json:"username,omitempty"`
	PasswordSecret *SecretSource `json:"passwordSecret,omitempty"`

	ClientID                       string         `json:"clientId,omitempty"`
	TokenEndpointURI               string         `json:"tokenEndpointUri,omitempty"`
	ClientSecret                   *SecretSource  `json:"clientSecret,omitempty"`
	AccessToken                    *SecretSource  `json:"accessToken,omitempty"`
	RefreshToken                   *SecretSource  `json:"refreshToken,omitempty"`
	TLSTrustedCertificates         []SecretSource `json:"tlsTrustedCertificates,omitempty"`
	DisableTLSHostnameVerification bool           `json:"disableTlsHostnameVerification,omitempty"`
	MaxTokenExpirySeconds          int            `json:"maxTokenExpirySeconds,omitempty"`
	AccessTokenIsJwt               bool           `json:"accessTokenIsJwt,omitempty"`
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

// SecretSource defines the desired state of SecretSource
type SecretSource struct {
	// SecretName always needed
	SecretName string `json:"secretName,omitempty"`
	// Certificate is the secret subpath name needed for TLS Auth and the Trusted CA Cert
	Certificate string `json:"certificate,omitempty"`
	// Key is the secret subpath name for the cert private key
	Key string `json:"key,omitempty"`
	// Password is is the secret subpath name used for scram-sha-512 and plain auth (not the actual password!)
	Password string `json:"password,omitempty"`
}
