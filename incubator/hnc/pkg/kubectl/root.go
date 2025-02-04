/*

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

// Package kubectl implements the HNC kubectl plugin
package kubectl

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	tenancy "github.com/kubernetes-sigs/multi-tenancy/incubator/hnc/api/v1alpha1"
)

var k8sClient *kubernetes.Clientset
var hncClient *rest.RESTClient

func init() {
	tenancy.AddToScheme(scheme.Scheme)
}

var rootCmd = &cobra.Command{
	Use:   "kubectl-hnc",
	Short: "Manipulate the hierarchy",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		// use the current context in kubeconfig
		kubeconfig := os.Getenv("KUBECONFIG") // TODO: check args first
		if len(kubeconfig) == 0 {
			kubeconfig = filepath.Join(homeDir(), ".kube", "config")
		}
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return err
		}

		// create the K8s clientset
		k8sClient, err = kubernetes.NewForConfig(config)
		if err != nil {
			return err
		}

		// create the HNC clientset
		hncConfig := *config
		hncConfig.ContentConfig.GroupVersion = &tenancy.GroupVersion
		hncConfig.APIPath = "/apis"
		hncConfig.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}
		hncConfig.UserAgent = rest.DefaultKubernetesUserAgent()
		hncClient, err = rest.UnversionedRESTClientFor(&hncConfig)
		if err != nil {
			return err
		}

		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func getHierarchy(nnm string) *tenancy.Hierarchy {
	if _, err := k8sClient.CoreV1().Namespaces().Get(nnm, metav1.GetOptions{}); err != nil {
		fmt.Printf("Error reading namespace %s: %s\n", nnm, err)
		os.Exit(1)
	}
	hier := &tenancy.Hierarchy{}
	hier.Name = tenancy.Singleton
	hier.Namespace = nnm
	err := hncClient.Get().Resource("hierarchies").Namespace(nnm).Name(tenancy.Singleton).Do().Into(hier)
	if err != nil && !errors.IsNotFound(err) {
		fmt.Printf("Error reading hierarchy for %s: %s\n", nnm, err)
		os.Exit(1)
	}
	return hier
}

func updateHierarchy(hier *tenancy.Hierarchy, reason string) {
	nnm := hier.Namespace
	var err error
	if hier.CreationTimestamp.IsZero() {
		err = hncClient.Post().Resource("hierarchies").Namespace(nnm).Name(tenancy.Singleton).Body(hier).Do().Error()
	} else {
		err = hncClient.Put().Resource("hierarchies").Namespace(nnm).Name(tenancy.Singleton).Body(hier).Do().Error()
	}
	if err != nil {
		fmt.Printf("Error %s: %s\n", reason, err)
		os.Exit(1)
	}
}
