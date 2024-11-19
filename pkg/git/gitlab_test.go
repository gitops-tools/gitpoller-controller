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
package git

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	pollingv1alpha1 "github.com/gitops-tools/gitpoller-controller/api/v1alpha1"
)

var _ CommitPoller = (*GitLabPoller)(nil)

func TestNewGitLabPoller(t *testing.T) {
	newTests := []struct {
		endpoint     string
		wantEndpoint string
	}{
		{"", "https://gitlab.com"},
		{"https://gl.example.com", "https://gl.example.com"},
	}

	for _, tt := range newTests {
		c := NewGitLabPoller(http.DefaultClient, tt.endpoint, "testToken")

		if c.endpoint != tt.wantEndpoint {
			t.Errorf("%#v got %#v, want %#v", tt.endpoint, c.endpoint, tt.wantEndpoint)
		}
	}
}

func TestGitLabWithUnknownETag(t *testing.T) {
	as := makeGitLabAPIServer(t, testToken, "/api/v4/projects/testing/repo/repository/commits", "master", testEtag, mustReadFile(t, "testdata/gitlab_commit.json"))
	t.Cleanup(as.Close)
	g := NewGitLabPoller(as.Client(), as.URL, testToken)
	g.endpoint = as.URL

	polled, body, err := g.Poll(context.TODO(), "testing/repo", pollingv1alpha1.PollStatus{Ref: "master"})
	if err != nil {
		t.Fatal(err)
	}

	if polled.ETag != testEtag {
		t.Errorf("Poll() ETag got %s, want %s", polled.ETag, testEtag)
	}
	if polled.SHA != "ed899a2f4b50b4370feeea94676502b42383c746" {
		t.Errorf("Poll() SHA got %s, want %s", polled.SHA, "ed899a2f4b50b4370feeea94676502b42383c746")
	}
	if m := body["author_email"].(string); m != "user@example.com" {
		t.Fatalf("got author email %s, want %s", m, "user@example.com")
	}
}

func TestGitLabWithKnownTag(t *testing.T) {
	as := makeGitLabAPIServer(t, testToken, "/api/v4/projects/testing/repo/repository/commits", "master", testEtag, nil)
	t.Cleanup(as.Close)

	g := NewGitLabPoller(as.Client(), as.URL, testToken)
	g.endpoint = as.URL

	polled, body, err := g.Poll(context.TODO(), "testing/repo", pollingv1alpha1.PollStatus{Ref: "master", ETag: testEtag})
	if err != nil {
		t.Fatal(err)
	}
	if polled.ETag != testEtag {
		t.Fatalf("Poll() got %s, want %s", polled.ETag, testEtag)
	}
	if body != nil {
		t.Fatalf("expected an empty body, got %#v\n", body)
	}
}

func TestGitLabWithNotFoundResponse(t *testing.T) {
	as := makeGitLabAPIServer(t, testToken, "/api/v4/projects/testing/repo/repository/commits", "master", testEtag, nil)
	t.Cleanup(as.Close)
	g := NewGitLabPoller(as.Client(), as.URL, testToken)
	g.endpoint = as.URL

	_, _, err := g.Poll(context.TODO(), "testing/testing", pollingv1alpha1.PollStatus{Ref: "master", ETag: testEtag})
	if err.Error() != "server error: 404" {
		t.Fatal(err)
	}
}

// It's impossible to distinguish between unknown repo, and bad auth token, both
// respond with a 404.
func TestGitLabWithBadAuthentication(t *testing.T) {
	as := makeGitLabAPIServer(t, testToken, "/api/v4/projects/testing/repo/repository/commits", "master", testEtag, nil)
	t.Cleanup(as.Close)
	g := NewGitLabPoller(as.Client(), as.URL, "anotherToken")
	g.endpoint = as.URL

	_, _, err := g.Poll(context.TODO(), "testing/repo", pollingv1alpha1.PollStatus{Ref: "master", ETag: testEtag})
	if err.Error() != "server error: 404" {
		t.Fatal(err)
	}
}

// With no auth-token, no auth header should be sent.
func TestGitLabWithNoAuthentication(t *testing.T) {
	as := makeGitLabAPIServer(t, "", "/api/v4/projects/testing/repo/repository/commits", "master", testEtag, nil)
	t.Cleanup(as.Close)
	g := NewGitLabPoller(as.Client(), as.URL, "")
	g.endpoint = as.URL

	_, _, err := g.Poll(context.TODO(), "testing/repo", pollingv1alpha1.PollStatus{Ref: "master", ETag: testEtag})
	if err != nil {
		t.Fatal(err)
	}
}

// makeAPIServer is used during testing to create an HTTP server to return
// fixtures if the request matches.
func makeGitLabAPIServer(t *testing.T, authToken, wantPath, wantRef, etag string, response []byte) *httptest.Server {
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != wantPath {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if queryRef := r.URL.Query().Get("ref_name"); queryRef != wantRef {
			w.WriteHeader(http.StatusNotAcceptable)
		}
		if authToken != "" {
			if auth := r.Header.Get("Private-Token"); auth != authToken {
				w.WriteHeader(http.StatusNotFound)
				return
			}
		}
		if auth := r.Header.Get("Private-Token"); auth != "" && authToken == "" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if etag == r.Header.Get("If-None-Match") {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		if r.Header.Get("Accept") != chitauriPreview {
			w.WriteHeader(http.StatusUnsupportedMediaType)
			return
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(response); err != nil {
			t.Errorf("failed to write response: %s", err)
		}
	}))
}
