package server

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/openshift/machine-config-operator/pkg/generated/clientset/versioned/typed/machineconfiguration.openshift.io/v1"
)

type ignitionRenderer struct {
	// machineClient is used to interact with the
	// machine config, pool objects.
	machineClient v1.MachineconfigurationV1Interface
	kubeconfigFunc kubeconfigFunc
}

// NewClusterServer is used to initialize the machine config
// server that will be used to fetch the requested MachineConfigPool
// objects from within the cluster.
// It accepts a kubeConfig, which is not required when it's
// run from within a cluster(useful in testing).
// It accepts the apiserverURL which is the location of the KubeAPIServer.
func NewIgnitionRenderer(kubeConfig, apiserverURL string) (*ignitionRenderer, error) {
	restConfig, err := getClientConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("Failed to create Kubernetes rest client: %v", err)
	}

	mc := v1.NewForConfigOrDie(restConfig)
	return &ignitionRenderer{
		machineClient:  mc,
		kubeconfigFunc: func() ([]byte, []byte, error) { return kubeconfigFromFile(kubeConfig) },
	}, nil
}

// GetConfig fetches the machine config(type - Ignition) from the cluster,
// based on the pool request.
func (ir *ignitionRenderer) GetConfig(mcPool string) (*runtime.RawExtension, error) {
	mp, err := ir.machineClient.MachineConfigPools().Get(context.TODO(), mcPool, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not fetch pool. err: %v", err)
	}

	currConf := mp.Status.Configuration.Name

	mc, err := ir.machineClient.MachineConfigs().Get(context.TODO(), currConf, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not fetch config %s, err: %v", currConf, err)
	}

	appenders := getAppenders(currConf, ir.kubeconfigFunc, mc.Spec.OSImageURL)
	for _, a := range appenders {
		if err := a(mc); err != nil {
			return nil, err
		}
	}

	rawIgn, err := machineConfigToRawIgnition(mc)
	if err != nil {
		return nil, fmt.Errorf("server: could not convert MachineConfig to raw Ignition: %v", err)
	}

	return rawIgn, nil
}

