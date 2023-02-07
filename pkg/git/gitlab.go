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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	pollingv1alpha1 "github.com/gitops-tools/gitpoller-controller/api/v1alpha1"
	"github.com/go-logr/logr"
)

// TODO: add logging - especially of the response body.

type GitLabPoller struct {
	client    *http.Client
	endpoint  string
	authToken string
}

// NewGitLabPoller creates a new GitLab poller.
func NewGitLabPoller(c *http.Client, endpoint, authToken string) *GitLabPoller {
	if endpoint == "" {
		endpoint = "https://gitlab.com"
	}
	return &GitLabPoller{client: c, endpoint: endpoint, authToken: authToken}
}

func (g GitLabPoller) Poll(ctx context.Context, repo string, pr pollingv1alpha1.PollStatus) (pollingv1alpha1.PollStatus, Commit, error) {
	logger := logr.FromContextOrDiscard(ctx).WithValues("endpoint", g.endpoint, "repo", repo)
	requestURL := makeGitLabURL(g.endpoint, repo, pr.Ref)
	logger.Info("polling GitLab repo", "url", requestURL)
	req, err := http.NewRequest("GET", requestURL, nil)
	if pr.ETag != "" {
		req.Header.Add("If-None-Match", pr.ETag)
	}
	req.Header.Add("Accept", chitauriPreview)
	if g.authToken != "" {
		req.Header.Add("Private-Token", g.authToken)
	}
	resp, err := g.client.Do(req)
	if err != nil {
		logger.Error(err, "polling GitLab repo")
		return pollingv1alpha1.PollStatus{}, nil, fmt.Errorf("failed to get current commit: %v", err)
	}
	// TODO: Return an error type that we can identify as a NotFound, likely
	// this is either a security token issue, or an unknown repo.
	logger.Info("polled GitLab repo", "status", resp.StatusCode)
	if resp.StatusCode >= http.StatusBadRequest {
		return pollingv1alpha1.PollStatus{}, nil, fmt.Errorf("server error: %d", resp.StatusCode)
	}
	if resp.StatusCode == http.StatusNotModified {
		return pr, nil, nil
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	var gc []map[string]interface{}
	err = json.Unmarshal(body, &gc)
	if err != nil {
		logger.Error(err, "unmarshalling GitLab response")
		return pollingv1alpha1.PollStatus{}, nil, fmt.Errorf("failed to decode response body: %w", err)
	}
	commit := gc[0]
	logger.Info("poll complete", "ref", pr.Ref, "sha", commit["id"])
	return pollingv1alpha1.PollStatus{Ref: pr.Ref, SHA: commit["id"].(string), ETag: resp.Header.Get("ETag")}, commit, nil
}

func makeGitLabURL(endpoint, repo, ref string) string {
	values := url.Values{
		"ref_name": []string{ref},
	}
	return fmt.Sprintf("%s/api/v4/projects/%s/repository/commits?%s",
		endpoint, strings.Replace(repo, "/", "%2F", -1),
		values.Encode())
}

type gitlabCommit struct {
	ID string `json:"id"`
}
