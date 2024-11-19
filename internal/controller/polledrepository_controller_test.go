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
	"net/http"
	"testing"
	"time"

	"github.com/gitops-tools/gitpoller-controller/api/v1alpha1"
	pollingv1alpha1 "github.com/gitops-tools/gitpoller-controller/api/v1alpha1"
	"github.com/gitops-tools/gitpoller-controller/pkg/git"
	"github.com/gitops-tools/gitpoller-controller/test/utils"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	timeout            = 3 * time.Second
	testNamespace      = "testing"
	testRepoURL        = "https://github.com/bigkevmcd/go-demo.git"
	testRef            = "main"
	testCommitSHA      = "24317a55785cd98d6c9bf50a5204bc6be17e7316"
	testRepositoryName = "test-repository"
	testCommitETag     = `W/"878f43039ad0553d0d3122d8bc171b01"`
)

func TestReconciliation(t *testing.T) {
	scheme := runtime.NewScheme()
	utils.AssertNoError(t, pollingv1alpha1.AddToScheme(scheme))

	t.Run("polling a github repository", func(t *testing.T) {
		repository := newPolledRepository()
		repositoryKey := client.ObjectKeyFromObject(repository)
		k8sClient := newFakeClient(scheme, repository)
		mockPoller := git.NewFakePoller()
		dispatcher := &mockDispatcher{}
		reconciler := &PolledRepositoryReconciler{
			Client: k8sClient,
			Scheme: scheme,
			PollerFactory: func(cl *http.Client, repo *pollingv1alpha1.PolledRepository, endpoint, token string) git.CommitPoller {
				return mockPoller
			},
			EventDispatcher: dispatcher,
		}

		completeStatus := pollingv1alpha1.PollStatus{
			Ref:  testRef,
			SHA:  testCommitSHA,
			ETag: testCommitETag,
		}
		responseBody := map[string]interface{}{"id": testRef}

		// The poll status here is empty.
		mockPoller.AddFakeResponse("bigkevmcd/go-demo",
			pollingv1alpha1.PollStatus{Ref: testRef},
			responseBody,
			completeStatus)

		mockPoller.AddFakeResponse("bigkevmcd/go-demo",
			completeStatus,
			responseBody,
			completeStatus)

		result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: repositoryKey})
		utils.AssertNoError(t, err)

		want := ctrl.Result{RequeueAfter: time.Minute * 5}
		if diff := cmp.Diff(want, result); diff != "" {
			t.Errorf("incorrect result:\n%s", diff)
		}

		wantDispatches := []dispatch{
			{
				Endpoint: "https://example.com/testing",
				Commit:   map[string]any{"id": "main"},
			},
		}
		if diff := cmp.Diff(wantDispatches, dispatcher.dispatched); diff != "" {
			t.Errorf("failed to dispatch events:\n%s", diff)
		}
		utils.AssertNoError(t, k8sClient.Get(context.Background(), repositoryKey, repository))
		wantStatus := pollingv1alpha1.PolledRepositoryStatus{
			PollStatus: v1alpha1.PollStatus{
				Ref:  testRef,
				SHA:  testCommitSHA,
				ETag: testCommitETag,
			},
		}
		if diff := cmp.Diff(wantStatus, repository.Status); diff != "" {
			t.Errorf("failed to update repository status:\n%s", diff)
		}
	})

	t.Run("checking repository with no updates", func(t *testing.T) {
		repository := newPolledRepository()
		repository.Status.PollStatus = v1alpha1.PollStatus{
			Ref:  testRef,
			SHA:  testCommitSHA,
			ETag: testCommitETag,
		}

		repositoryKey := client.ObjectKeyFromObject(repository)
		k8sClient := newFakeClient(scheme, repository)
		mockPoller := git.NewFakePoller()
		dispatcher := &mockDispatcher{}
		reconciler := &PolledRepositoryReconciler{
			Client: k8sClient,
			Scheme: scheme,
			PollerFactory: func(cl *http.Client, repo *pollingv1alpha1.PolledRepository, endpoint, token string) git.CommitPoller {
				return mockPoller
			},
			EventDispatcher: dispatcher,
		}

		completeStatus := pollingv1alpha1.PollStatus{
			Ref:  testRef,
			SHA:  testCommitSHA,
			ETag: testCommitETag,
		}
		responseBody := map[string]interface{}{"id": testRef}

		mockPoller.AddFakeResponse("bigkevmcd/go-demo",
			completeStatus,
			responseBody,
			completeStatus)

		result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: repositoryKey})
		utils.AssertNoError(t, err)

		want := ctrl.Result{RequeueAfter: time.Minute * 5}
		if diff := cmp.Diff(want, result); diff != "" {
			t.Errorf("incorrect result:\n%s", diff)
		}

		if len(dispatcher.dispatched) > 0 {
			t.Errorf("dispatched %v events when no change detected", len(dispatcher.dispatched))
		}
		utils.AssertNoError(t, k8sClient.Get(context.Background(), repositoryKey, repository))
		wantStatus := pollingv1alpha1.PolledRepositoryStatus{
			PollStatus: v1alpha1.PollStatus{
				Ref:  testRef,
				SHA:  testCommitSHA,
				ETag: testCommitETag,
			},
		}
		if diff := cmp.Diff(wantStatus, repository.Status); diff != "" {
			t.Errorf("updated repository status when no change:\n%s", diff)
		}
	})
}

func newPolledRepository() *pollingv1alpha1.PolledRepository {
	repo := &pollingv1alpha1.PolledRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testRepositoryName,
			Namespace: testNamespace,
		},
		Spec: pollingv1alpha1.PolledRepositorySpec{
			URL:       testRepoURL,
			Ref:       testRef,
			Endpoint:  "https://example.com/testing",
			Type:      pollingv1alpha1.GitHub,
			Frequency: &metav1.Duration{Duration: time.Minute * 5},
		},
	}
	return repo
}

func newFakeClient(scheme *runtime.Scheme, objs ...runtime.Object) client.WithWatch {
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		WithStatusSubresource(&pollingv1alpha1.PolledRepository{}).Build()
}

type mockDispatcher struct {
	dispatched []dispatch
}

func (m *mockDispatcher) Dispatch(ctx context.Context, repo pollingv1alpha1.PolledRepository, commit map[string]any) error {
	m.dispatched = append(m.dispatched, dispatch{Endpoint: repo.Spec.Endpoint, Commit: commit})
	return nil
}

type dispatch struct {
	Endpoint string
	Commit   map[string]any
}
