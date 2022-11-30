/*
Copyright 2022.

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

package controllers

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	cachev1alpha1 "github.com/example/memcached-operator/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go te sting framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = cachev1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&HelloworldReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

})

const (
	HelloworldNamespace = "default"
	HelloworldName      = "helloworld-test"
	Content             = "Test content"
)

var _ = Describe("Helloworld controller", func() {
	It("Should create new pods for each Helloworld instance, with count equals to ReplicaCount", func() {
		By("By creating a new Helloworld")
		ctx := context.Background()
		helloworld := &cachev1alpha1.Helloworld{
			TypeMeta:   metav1.TypeMeta{APIVersion: "cache.my.domain/v1alpha1", Kind: "Helloworld"},
			ObjectMeta: metav1.ObjectMeta{Name: HelloworldName, Namespace: HelloworldNamespace},
			Spec:       cachev1alpha1.HelloworldSpec{ReplicaCount: 2, DefaultContent: Content},
		}
		Expect(k8sClient.Create(ctx, helloworld)).Should(Succeed())

		helloworldLookupKey := types.NamespacedName{Name: HelloworldName, Namespace: HelloworldNamespace}
		createdHelloworld := &cachev1alpha1.Helloworld{}

		Eventually(func() bool {
			err := k8sClient.Get(ctx, helloworldLookupKey, createdHelloworld)
			return err == nil
		}).Should(BeTrue())
		Expect(createdHelloworld.Spec.DefaultContent).Should(Equal(Content))

		Eventually(func() int {
			podList := &v1.PodList{}
			opts := []client.ListOption{
				client.InNamespace(HelloworldNamespace),
				client.MatchingLabels{"hw_name": HelloworldName},
			}

			if err := k8sClient.List(ctx, podList, opts...); err != nil {
				return -1
			}

			return len(podList.Items)
		}).Should(Equal(2))

		Eventually(func() int {
			podList := &v1.PodList{}
			opts := []client.ListOption{
				client.InNamespace(HelloworldNamespace),
				client.MatchingLabels{"hw_name": HelloworldName},
			}
			k8sClient.List(ctx, podList, opts...)

			k8sClient.Delete(ctx, &podList.Items[0])

			time.Sleep(time.Second * 10)

			k8sClient.List(ctx, podList, opts...)

			return len(podList.Items)
		}).Should(Equal(2))
	})
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
