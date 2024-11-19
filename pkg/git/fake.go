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
package git

import (
	"context"
	"strings"

	pollingv1alpha1 "github.com/gitops-tools/gitpoller-controller/api/v1alpha1"
)

var _ CommitPoller = (*FakePoller)(nil)

// NewFakePoller creates and returns a new fake Git poller.
func NewFakePoller() *FakePoller {
	return &FakePoller{
		responses: make(map[string]pollingv1alpha1.PollStatus),
		commits:   make(map[string]Commit),
	}
}

// FakePoller is a fake Git poller.
//
// It can be configured with responses and errors.
type FakePoller struct {
	pollError error
	responses map[string]pollingv1alpha1.PollStatus
	commits   map[string]Commit
}

// Poll is an implementation of the CommitPoller interface.
func (m *FakePoller) Poll(ctx context.Context, repo string, ps pollingv1alpha1.PollStatus) (pollingv1alpha1.PollStatus, Commit, error) {
	if m.pollError != nil {
		return pollingv1alpha1.PollStatus{}, nil, m.pollError
	}
	k := mockKey(repo, ps)
	return m.responses[k], m.commits[k], nil
}

// AddFakeResponse sets up the response for a Poll call.
func (m *FakePoller) AddFakeResponse(repo string, in pollingv1alpha1.PollStatus, c Commit, out pollingv1alpha1.PollStatus) {
	k := mockKey(repo, in)
	m.responses[k] = out
	m.commits[k] = c
}

// FailWithError configures the poller to return errors.
func (m *FakePoller) FailWithError(err error) {
	m.pollError = err
}

func mockKey(repo string, ps pollingv1alpha1.PollStatus) string {
	return strings.Join([]string{repo, ps.Ref, ps.SHA, ps.ETag}, ":")
}
