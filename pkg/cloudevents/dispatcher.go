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
	"fmt"
	"net/http"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	pollingv1alpha1 "github.com/gitops-tools/gitpoller-controller/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
)

// Dispatch sends the commit as a CloudEvent to the Endpoint provided by the
// repo.
func Dispatch(ctx context.Context, repo pollingv1alpha1.PolledRepository, commit map[string]interface{}) error {
	logger := logr.FromContextOrDiscard(ctx).WithValues("endpoint", repo.Spec.Endpoint)
	var useOnceTransport http.RoundTripper = &http.Transport{
		DisableKeepAlives: true,
	}

	p, err := cloudevents.NewHTTP(cloudevents.WithRoundTripper(useOnceTransport))
	if err != nil {
		logger.Error(err, "failed to create cloud event transport")
		return fmt.Errorf("failed to create cloud event transport: %w", err)
	}

	cloudEventClient, err := cloudevents.NewClient(p, cloudevents.WithUUIDs(), cloudevents.WithTimeNow())
	if err != nil {
		logger.Error(err, "failed to create cloud event client")
		return fmt.Errorf("failed to create cloud event client: %w", err)
	}

	event, err := makeCloudEvent(repo, commit)
	if err != nil {
		return fmt.Errorf("failed to create CloudEvent: %w", err)
	}

	ctx = cloudevents.ContextWithTarget(ctx, repo.Spec.Endpoint)
	if result := cloudEventClient.Send(cloudevents.ContextWithRetriesExponentialBackoff(ctx, 10*time.Millisecond, 10), *event); !cloudevents.IsACK(result) {
		return result
	}
	return nil
}

func makeCloudEvent(repo pollingv1alpha1.PolledRepository, commit map[string]interface{}) (*cloudevents.Event, error) {
	event := cloudevents.NewEvent()
	event.SetID(uuid.New().String())
	event.SetSubject(subjectForRepo(repo))
	event.SetSource(repo.Spec.URL)
	event.SetType("commit")
	if err := event.SetData(cloudevents.ApplicationJSON, commit); err != nil {
		return nil, err
	}
	return &event, nil
}

func subjectForRepo(repo pollingv1alpha1.PolledRepository) string {
	source := repo.GetObjectMeta().GetSelfLink()
	if source == "" {
		gvk := repo.GetObjectKind().GroupVersionKind()
		source = fmt.Sprintf("/apis/%s/%s/namespaces/%s/%s/%s",
			gvk.Group,
			gvk.Version,
			repo.GetObjectMeta().GetNamespace(),
			gvk.Kind,
			repo.GetObjectMeta().GetName())
	}
	return source
}
