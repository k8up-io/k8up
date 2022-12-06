/*
 * Copyright Rivtower Technologies LLC.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package switchover

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var scheme = runtime.NewScheme()

func init() {
	// Register all types of our clientset into the standard scheme.
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

}

func loadClientConfig() (*rest.Config, error) {
	// The default loading rules try to read from the files specified in the
	// environment or from the home directory.
	loader := clientcmd.NewDefaultClientConfigLoadingRules()

	// The deferred loader tries an in-cluster config if the default loading
	// rules produce no results.
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loader, &clientcmd.ConfigOverrides{},
	).ClientConfig()
}

//var K8sClient client.Client

func InitK8sClient(namespace string) (client.Client, error) {
	config, err := loadClientConfig()
	if err != nil {
		return nil, err
	}
	// Match the settings applied by sigs.k8s.io/controller-runtime@v0.6.0;
	// see https://github.com/kubernetes-sigs/controller-runtime/issues/365.
	if config.QPS == 0.0 {
		config.QPS = 20.0
		config.Burst = 30.0
	}

	k8sManager, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:             scheme,
		LeaderElection:     false,
		MetricsBindAddress: "0",
		Namespace:          namespace,
	})
	if err != nil {
		return nil, err
	}

	go func() {
		_ = k8sManager.Start(ctrl.SetupSignalHandler())
	}()

	if !k8sManager.GetCache().WaitForCacheSync(context.Background()) {
		return nil, fmt.Errorf("wait cache failed")
	}

	return k8sManager.GetClient(), nil
}
