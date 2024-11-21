/*
Copyright 2024.

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

package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	pollingv1 "github.com/gitops-tools/gitpoller-controller/api/v1alpha1"
	"github.com/gitops-tools/gitpoller-controller/pkg/cloudevents"
	"github.com/gitops-tools/gitpoller-controller/pkg/git"
	"github.com/gitops-tools/gitpoller-controller/test/utils"
	"github.com/google/go-cmp/cmp"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

const (
	timeout = 5 * time.Second
)

func TestPolledRepositoryController(t *testing.T) {
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		t.Skip("Not setup for envtest correctly please set KUBEBUILDER_ASSETS")
	}

	scheme := runtime.NewScheme()
	utils.AssertNoError(t, clientgoscheme.AddToScheme(scheme))
	utils.AssertNoError(t, pollingv1.AddToScheme(scheme))

	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	if err != nil {
		t.Fatalf("failed to start test environment: %s", err)
	}

	t.Cleanup(func() {
		if err := testEnv.Stop(); err != nil {
			t.Fatalf("failed to stop test environment: %s", err)
		}
	})

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0", // Do not start the metrics server.
		},
	})
	utils.AssertNoError(t, err)

	k8sClient := k8sManager.GetClient()

	if err := (&PolledRepositoryReconciler{
		Client:     k8sClient,
		Scheme:     scheme,
		HTTPClient: http.DefaultClient,
		PollerFactory: func(cl *http.Client, repo *pollingv1.PolledRepository, endpoint, token string) git.CommitPoller {
			return MakeCommitPoller(cl, repo, endpoint, token)
		},
		EventDispatcher: cloudevents.CloudEventDispatcher{},
	}).SetupWithManager(k8sManager); err != nil {
		t.Fatalf("Failed to start PolledRepositoryReconciler: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		utils.AssertNoError(t, k8sManager.Start(ctx))
	}()

	<-k8sManager.Elected()

	utils.AssertNoError(t, k8sClient.Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "testing"}}))

	t.Run("polling a github repository", func(t *testing.T) {
		dispatches := make(chan map[string]any, 1)
		ts := httptest.NewServer(http.HandlerFunc(notificationHandler(dispatches)))
		t.Cleanup(ts.Close)

		repo := newPolledRepository(withEndpoint(ts.URL))
		repoKey := client.ObjectKeyFromObject(repo)

		utils.AssertNoError(t, k8sClient.Create(context.Background(), repo))
		t.Cleanup(func() {
			deleteObject(t, k8sClient, repo)
		})

		gomega.NewWithT(t).Eventually(func() string {
			utils.AssertNoError(t, client.IgnoreNotFound(k8sClient.Get(context.Background(), repoKey, repo)))
			return repo.Status.PollStatus.SHA
		}, timeout).Should(gomega.Not(gomega.Equal("")))

		received := <-dispatches

		if diff := cmp.Diff(repo.Status.PollStatus.SHA, received["sha"]); diff != "" {
			t.Errorf("incorrect notification: diff -want +got\n%s", diff)
		}
	})
}

func deleteObject(t *testing.T, cl client.Client, obj client.Object) {
	t.Helper()
	if err := cl.Delete(context.TODO(), obj); err != nil {
		t.Fatal(err)
	}
}

func notificationHandler(dispatches chan<- map[string]any) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		decoder := json.NewDecoder(req.Body)
		// TODO: This could do so much more, but we have tests for the
		// CloudEvent dispatcher so maybe not?
		result := map[string]any{}
		if err := decoder.Decode(&result); err != nil {
			http.Error(w, "failed to decode", http.StatusBadRequest)
			return
		}

		dispatches <- result
	}
}

func withEndpoint(s string) func(*pollingv1.PolledRepository) {
	return func(r *pollingv1.PolledRepository) {
		r.Spec.Endpoint = s
	}
}
