package logic

import (
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type K8s struct {
	logic
}

func (l K8s) MakeK8sConfig(k8sConfig string) (*rest.Config, error) {
	var config *rest.Config
	var err error
	if k8sConfig != "" {
		config, err = clientcmd.RESTConfigFromKubeConfig([]byte(k8sConfig))
		if err != nil {
			return nil, err
		}
	} else {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}

	return config, nil
}
