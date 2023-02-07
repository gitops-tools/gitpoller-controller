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

	pollingv1alpha1 "github.com/gitops-tools/gitpoller-controller/api/v1alpha1"
)

// Commit is a polled Commit, specific to each implementation.
type Commit map[string]interface{}

// CommitPoller implementations can check with an upstream Git hosting service
// to determine the current SHA and ETag.
type CommitPoller interface {
	// Poll polls and updates the status, it returns the updated status, along
	// with the commit details.
	Poll(ctx context.Context, repo string, ps pollingv1alpha1.PollStatus) (pollingv1alpha1.PollStatus, Commit, error)
}
