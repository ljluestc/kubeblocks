/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package k8sconfig

import (
	"context"
	"os"
	"path/filepath"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var k8sClient client.Client

// GetClientSet returns a client for interacting with the Kubernetes API
func GetClientSet() client.Client {
	if k8sClient != nil {
		return k8sClient
	}

	// Try to use in-cluster config first
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			if home := homedir.HomeDir(); home != "" {
				kubeconfig = filepath.Join(home, ".kube", "config")
			}
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(err)
		}
	}

	// Create a new client using the config
	c, err := client.New(config, client.Options{})
	if err != nil {
		panic(err)
	}

	k8sClient = c
	return k8sClient
}

// WithContext returns a context-aware client
func WithContext(ctx context.Context, c client.Client) client.Client {
	return client.NewNamespacedClient(c, "default")
}

// SetClient sets the client for testing purposes
func SetClient(c client.Client) {
	k8sClient = c
}
