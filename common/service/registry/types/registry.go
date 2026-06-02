package types

import (
	"time"
)

// const definition
const (
	RegistryTypeDockerRegistry = "docker-registry"
	RegistryTypeAliAcr         = "ali-acr"
	RegistryTypeTencentTcr     = "tencent-tcr"

	FilterStyleTypeText = "input"

	// CredentialTypeBasic indicates credential by user name, password
	CredentialTypeBasic = "basic"
	// CredentialTypeOAuth indicates credential by OAuth token
	CredentialTypeOAuth = "oauth"
	// CredentialTypeSecret is only used by the communication of internal components
	CredentialTypeSecret = "secret"

	// EndpointPatternTypeStandard ...
	EndpointPatternTypeStandard = "EndpointPatternTypeStandard"
	// EndpointPatternTypeFix ...
	EndpointPatternTypeFix = "EndpointPatternTypeFix"
	// EndpointPatternTypeList ...
	EndpointPatternTypeList = "EndpointPatternTypeList"

	// AccessKeyTypeStandard ...
	AccessKeyTypeStandard = "AccessKeyTypeStandard"
	// AccessKeyTypeFix ...
	AccessKeyTypeFix = "AccessKeyTypeFix"

	// AccessSecretTypeStandard ...
	AccessSecretTypeStandard = "AccessSecretTypePass"
	// AccessSecretTypeFile ...
	AccessSecretTypeFile = "AccessSecretTypeFile"

	// Healthy indicates registry is healthy
	Healthy = "healthy"
	// Unhealthy indicates registry is unhealthy
	Unhealthy = "unhealthy"
)

// Credential keeps the access key and/or secret for the related registry
type Credential struct {
	// Type of the credential
	Type string `json:"type"`
	// The key of the access account, for OAuth token, it can be empty
	AccessKey string `json:"access_key"`
	// The secret or password for the key
	AccessSecret string `json:"access_secret"`

	Token string `json:"token"`

	Region string `json:"region"`
}

// Registry keeps the related info of registry
// Data required for the secure access way is not contained here.
// DAO layer is not considered here
type Registry struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	URL         string `json:"url"`
	// TokenServiceURL is only used for local instance to
	// avoid the requests passing through the external proxy for now
	TokenServiceURL string      `json:"token_service_url"`
	Credential      *Credential `json:"credential"`
	ProxyUrl        string      `json:"proxy_url"`
	Insecure        bool        `json:"insecure"`
	Status          string      `json:"status"`
	CreationTime    time.Time   `json:"creation_time"`
	UpdateTime      time.Time   `json:"update_time"`
}

// FilterStyle ...
type FilterStyle struct {
	Type   string   `json:"type"`
	Style  string   `json:"style"`
	Values []string `json:"values,omitempty"`
}

// EndpointPattern ...
type EndpointPattern struct {
	EndpointType string      `json:"endpoint_type"`
	Endpoints    []*Endpoint `json:"endpoints"`
}

// Endpoint ...
type Endpoint struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// CredentialPattern ...
type CredentialPattern struct {
	AccessKeyType    string `json:"access_key_type"`
	AccessKeyData    string `json:"access_key_data"`
	AccessSecretType string `json:"access_secret_type"`
	AccessSecretData string `json:"access_secret_data"`
}

// RegistryInfo provides base info and capability declarations of the registry
type RegistryInfo struct {
	Type                                 string         `json:"type"`
	Description                          string         `json:"description"`
	SupportedResourceTypes               []string       `json:"-"`
	SupportedResourceFilters             []*FilterStyle `json:"supported_resource_filters"`
	SupportedTriggers                    []string       `json:"supported_triggers"`
	SupportedRepositoryPathComponentType string         `json:"supported_repository_path_component_type"` // how many path components are allowed in the repository name
	SupportedCopyByChunk                 bool           `json:"supported_copy_by_chunk,omitempty"`
}

// AdapterPattern provides base info and capability declarations of the registry
type AdapterPattern struct {
	EndpointPattern   *EndpointPattern   `json:"endpoint_pattern"`
	CredentialPattern *CredentialPattern `json:"credential_pattern"`
}

// NewDefaultAdapterPattern ...
func NewDefaultAdapterPattern() *AdapterPattern {
	return &AdapterPattern{
		EndpointPattern:   NewDefaultEndpointPattern(),
		CredentialPattern: NewDefaultCredentialPattern(),
	}
}

// NewDefaultEndpointPattern ...
func NewDefaultEndpointPattern() *EndpointPattern {
	return &EndpointPattern{
		EndpointType: EndpointPatternTypeStandard,
	}
}

// NewDefaultCredentialPattern ...
func NewDefaultCredentialPattern() *CredentialPattern {
	return &CredentialPattern{
		AccessKeyType:    AccessKeyTypeStandard,
		AccessSecretType: AccessSecretTypeStandard,
	}
}
