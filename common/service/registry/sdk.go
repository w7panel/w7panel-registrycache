package registry

import (
	"strings"

	"gitee.com/we7coreteam/w7-registry-cache/common/service/registry/client"
	"gitee.com/we7coreteam/w7-registry-cache/common/service/registry/types"
)

func NewRegistryClient(url string, credential *types.Credential, proxyUrl string) (client.Client, error) {
	return client.NewClientWithRegistry(&types.Registry{
		Type:       types.RegistryTypeDockerRegistry,
		URL:        strings.TrimRight(url, "/"),
		Credential: credential,
		Insecure:   true,
		ProxyUrl:   proxyUrl,
	}), nil
}
