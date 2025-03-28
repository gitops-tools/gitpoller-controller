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
	"os"
	"testing"

	pollingv1alpha1 "github.com/gitops-tools/gitpoller-controller/api/v1alpha1"
	"github.com/gitops-tools/gitpoller-controller/test/utils"
)

const (
	testToken string = "test12345"
	testEtag  string = `W/"878f43039ad0553d0d3122d8bc171b01"`
)

var _ CommitPoller = (*GitHubPoller)(nil)

func TestNewGitHubPoller(t *testing.T) {
	newTests := []struct {
		endpoint     string
		wantEndpoint string
	}{
		{"https://gh.example.com", "https://gh.example.com"},
	}

	for _, tt := range newTests {
		c := NewGitHubPoller(http.DefaultClient, tt.endpoint, "testToken")
		if c.endpoint != tt.wantEndpoint {
			t.Errorf("%#v got %#v, want %#v", tt.endpoint, c.endpoint, tt.wantEndpoint)
		}
	}
}

func TestGitHubWithUnknownETag(t *testing.T) {
	as := utils.MakeGitHubAPIServer(t, testToken, "/repos/testing/repo/commits/master", testEtag, mustReadFile(t, "testdata/github_commit.json"))
	t.Cleanup(as.Close)
	g := NewGitHubPoller(as.Client(), as.URL, testToken)
	g.endpoint = as.URL

	polled, body, err := g.Poll(context.TODO(), "testing/repo", pollingv1alpha1.PollStatus{Ref: "master"})
	if err != nil {
		t.Fatal(err)
	}

	if polled.ETag != testEtag {
		t.Errorf("Poll() ETag got %s, want %s", polled.ETag, testEtag)
	}
	if polled.SHA != "7638417db6d59f3c431d3e1f261cc637155684cd" {
		t.Errorf("Poll() SHA got %s, want %s", polled.SHA, "7638417db6d59f3c431d3e1f261cc637155684cd")
	}
	if m := body["message"]; m != "added readme, because im a good github citizen" {
		t.Fatalf("body doesn't match:\n%s", m)
	}
}

func TestGitHubWithKnownTag(t *testing.T) {
	as := utils.MakeGitHubAPIServer(t, testToken, "/repos/testing/repo/commits/master", testEtag, nil)
	t.Cleanup(as.Close)
	g := NewGitHubPoller(as.Client(), as.URL, testToken)
	g.endpoint = as.URL

	polled, body, err := g.Poll(context.TODO(), "testing/repo", pollingv1alpha1.PollStatus{Ref: "master", ETag: testEtag})
	if err != nil {
		t.Fatal(err)
	}

	if polled.ETag != testEtag {
		t.Fatalf("Poll() got %s, want %s", polled.ETag, testEtag)
	}
	if body != nil {
		t.Fatalf("for unknown tag, got %#v, want nil", body)
	}
}

func TestGitHubWithNotFoundResponse(t *testing.T) {
	as := utils.MakeGitHubAPIServer(t, testToken, "/repos/testing/repo/commits/master", testEtag, nil)
	t.Cleanup(as.Close)
	g := NewGitHubPoller(as.Client(), as.URL, testToken)
	g.endpoint = as.URL

	_, _, err := g.Poll(context.TODO(), "testing/testing", pollingv1alpha1.PollStatus{Ref: "master", ETag: testEtag})
	if err.Error() != "server error: 404" {
		t.Fatal(err)
	}
}

// It's impossible to distinguish between unknown repo, and bad auth token, both
// respond with a 404.
func TestGitHubWithBadAuthentication(t *testing.T) {
	as := utils.MakeGitHubAPIServer(t, testToken, "/repos/testing/repo/commits/master", testEtag, nil)
	t.Cleanup(as.Close)
	g := NewGitHubPoller(as.Client(), as.URL, "anotherToken")
	g.endpoint = as.URL

	_, _, err := g.Poll(context.TODO(), "testing/repo", pollingv1alpha1.PollStatus{Ref: "master", ETag: testEtag})
	if err.Error() != "server error: 404" {
		t.Fatal(err)
	}
}

// With no auth-token, no auth header should be sent.
func TestGitHubWithNoAuthentication(t *testing.T) {
	as := utils.MakeGitHubAPIServer(t, "", "/repos/testing/repo/commits/master", testEtag, nil)
	t.Cleanup(as.Close)
	g := NewGitHubPoller(as.Client(), as.URL, "")
	g.endpoint = as.URL

	_, _, err := g.Poll(context.TODO(), "testing/repo", pollingv1alpha1.PollStatus{Ref: "master", ETag: testEtag})
	if err != nil {
		t.Fatal(err)
	}
}

func mustReadFile(t *testing.T, filename string) []byte {
	t.Helper()
	d, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}

	return d
}
