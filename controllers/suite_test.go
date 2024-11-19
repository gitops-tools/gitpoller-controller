/*
Copyright 2021.

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
	"io/ioutil"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	pollingv1alpha1 "github.com/gitops-tools/gitpoller-controller/api/v1alpha1"
	"github.com/gitops-tools/gitpoller-controller/pkg/git"
	"github.com/gitops-tools/gitpoller-controller/pkg/secrets"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.
var (
	cfg          *rest.Config
	testEnv      *envtest.Environment
	k8sClient    client.Client
	k8sManager   ctrl.Manager
	requiredAuth string
	mockPoller   *git.MockPoller
	dispatcher   *fakeDispatcher
)

const (
	timeout        = 3 * time.Second
	testNamespace  = "testing"
	testRepoURL    = "https://github.com/bigkevmcd/go-demo.git"
	testRef        = "main"
	testCommitSHA  = "24317a55785cd98d6c9bf50a5204bc6be17e7316"
	testCommitETag = `W/"878f43039ad0553d0d3122d8bc171b01"`
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	Expect(pollingv1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())

	//+kubebuilder:scaffold:scheme

	k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	mockPoller = git.NewMockPoller()
	dispatcher = &fakeDispatcher{}

	reconciler := &PolledRepositoryReconciler{
		Client:       k8sClient,
		Log:          ctrl.Log.WithName("controllers").WithName("PolledRepositoryReconciler"),
		Scheme:       scheme.Scheme,
		SecretGetter: secrets.New(k8sClient),
		PollerFactory: func(cl *http.Client, repo *pollingv1alpha1.PolledRepository, endpoint, token string) git.CommitPoller {
			return mockPoller
		},
		EventDispatcher: dispatcher,
	}
	Expect(reconciler.SetupWithManager(k8sManager)).To(Succeed())

	Expect(k8sClient.Create(
		context.TODO(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}})).
		To(Succeed())

	go func() {
		Expect(k8sManager.Start(ctrl.SetupSignalHandler())).To(Succeed())
	}()
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Expect(testEnv.Stop()).To(Succeed())
})

func mustReadFile(t GinkgoTInterface, filename string) []byte {
	t.Helper()
	d, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

type fakeDispatcher struct {
	dispatched []dispatch
}

func (m *fakeDispatcher) Dispatch(ctx context.Context, repo pollingv1alpha1.PolledRepository, commit map[string]any) error {
	m.dispatched = append(m.dispatched, dispatch{endpoint: repo.Spec.Endpoint, commit: commit})
	return nil
}

type dispatch struct {
	endpoint string
	commit   map[string]any
}
