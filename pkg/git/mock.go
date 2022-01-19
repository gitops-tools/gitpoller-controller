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
	"strings"

	pollingv1alpha1 "github.com/gitops-tools/gitpoller-controller/api/v1alpha1"
)

var _ CommitPoller = (*MockPoller)(nil)

// NewMockPoller creates and returns a new mock Git poller.
func NewMockPoller() *MockPoller {
	return &MockPoller{
		responses: make(map[string]pollingv1alpha1.PollStatus),
		commits:   make(map[string]Commit),
	}
}

// MockPoller is a mock Git poller.
type MockPoller struct {
	pollError error
	responses map[string]pollingv1alpha1.PollStatus
	commits   map[string]Commit
}

// Poll is an implementation of the CommitPoller interface.
func (m *MockPoller) Poll(ctx context.Context, repo string, ps pollingv1alpha1.PollStatus) (pollingv1alpha1.PollStatus, Commit, error) {
	if m.pollError != nil {
		return pollingv1alpha1.PollStatus{}, nil, m.pollError
	}
	k := mockKey(repo, ps)
	return m.responses[k], m.commits[k], nil
}

// AddMockResponse sets up the response for a Poll call.
func (m *MockPoller) AddMockResponse(repo string, in pollingv1alpha1.PollStatus, c Commit, out pollingv1alpha1.PollStatus) {
	k := mockKey(repo, in)
	m.responses[k] = out
	m.commits[k] = c
}

// FailWithError configures the poller to return errors.
func (m *MockPoller) FailWithError(err error) {
	m.pollError = err
}

func mockKey(repo string, ps pollingv1alpha1.PollStatus) string {
	return strings.Join([]string{repo, ps.Ref, ps.SHA, ps.ETag}, ":")
}
