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
package cloudevents

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	pollingv1alpha1 "github.com/gitops-tools/gitpoller-controller/api/v1alpha1"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDispatch(t *testing.T) {
	event := map[string]interface{}{
		"testing": true,
	}
	repoURL := "https://github.com/gitops-tools/gitpoller-controller.git"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertJSONRequest(t, r, event)
		assertRequestHeaders(t, r, map[string]string{
			"Ce-Subject": "/apis/polling.gitops.tools/v1alpha1/namespaces/testing/PolledRepository/test-repository",
			"Ce-Source":  repoURL,
			"Ce-Type":    "commit",
		})
	}))
	defer ts.Close()

	repo := pollingv1alpha1.PolledRepository{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "polling.gitops.tools/v1alpha1",
			Kind:       "PolledRepository",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-repository",
			Namespace: "testing",
		},
		Spec: pollingv1alpha1.PolledRepositorySpec{
			URL:      repoURL,
			Ref:      "main",
			Endpoint: ts.URL,
		},
	}

	err := Dispatch(context.TODO(), repo, event)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDispatch_handle_non_200_response(t *testing.T) {
	event := map[string]interface{}{
		"testing": true,
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "test error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	repo := pollingv1alpha1.PolledRepository{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "polling.gitops.tools/v1alpha1",
			Kind:       "PolledRepository",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-repository",
			Namespace: "testing",
		},
		Spec: pollingv1alpha1.PolledRepositorySpec{
			URL:      "https://github.com/gitops-tools/gitpoller-controller",
			Ref:      "main",
			Endpoint: ts.URL,
		},
	}

	err := Dispatch(context.TODO(), repo, event)
	if err == nil {
		t.Fatal("expected an error response from an internal server error")
	}
}

func assertJSONRequest(t *testing.T, req *http.Request, want map[string]interface{}) {
	t.Helper()
	b, err := ioutil.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	if h := req.Header.Get("Content-Type"); h != "application/json" {
		t.Fatalf("wanted 'application/json' got %s", h)
	}
	got := map[string]interface{}{}

	err = json.Unmarshal(b, &got)
	if err != nil {
		t.Fatalf("failed to parse %s: %s", b, err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("JSON request failed:\n%s", diff)
	}
}

func assertRequestHeaders(t *testing.T, req *http.Request, want map[string]string) {
	t.Helper()
	for k, v := range want {
		if val := req.Header.Get(k); val != v {
			t.Errorf("header %s, got %q, want %q", k, val, v)
		}
	}
}
