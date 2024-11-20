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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RepoType defines the protocol to use to talk to the upstream server.
// +kubebuilder:validation:Enum=github;gitlab
type RepoType string

const (
	GitHub RepoType = "github"
	GitLab RepoType = "gitlab"
)

// PolledRepositorySpec defines the desired state of PolledRepository
type PolledRepositorySpec struct {
	// URL is the Git repository URL to poll.
	// +kubebuilder:validation:Pattern="^https://"
	// +required
	URL string `json:"url"`

	// Ref is the branch or tag to poll within the repository.
	// +required
	Ref string `json:"ref,omitempty"`

	// Auth provides an optional secret for polling the repository.
	// +optional
	Auth *AuthSecret `json:"auth,omitempty"`

	// Type is the protocol to use to access the repository.
	// +kubebuilder:validation:Enum=github;gitlab
	Type RepoType `json:"type,omitempty"`

	// Frequency is how often to poll this repository.
	//+kubebuilder:default:="5m"
	// +required
	Frequency *metav1.Duration `json:"frequency,omitempty"`

	// The notification URL, this is where CloudEvents are dispatched to for
	// this repository.
	// +kubebuilder:validation:Pattern="^(http|https)://"
	// +required
	Endpoint string `json:"endpoint"`

	// TODO: Retries...guarantees around delivery?
	// Errors in delivery will cause rereconciliation?
}

// AuthSecret references a secret for authenticating the request.
type AuthSecret struct {
	// This is a local reference to the named secret to fetch.
	// This secret is expected to have a "token" key with a valid GitHub/GitLab
	// auth token.
	SecretRef corev1.LocalObjectReference `json:"secretRef,omitempty"`
	//+kubebuilder:default:="token"
	Key string `json:"key,omitempty"`
}

// PolledRepositoryStatus defines the observed state of PolledRepository
type PolledRepositoryStatus struct {
	PollStatus         `json:"pollStatus,omitempty"`
	LastError          string `json:"lastError,omitempty"`
	ObservedGeneration int64  `json:"observedGeneration,omitempty"`
}

// PollStatus represents the last polled state of the repo.
type PollStatus struct {
	Ref  string `json:"ref"`
	SHA  string `json:"sha"`
	ETag string `json:"etag"`
}

// Equal returns true if two PollStatus values match.
func (p PollStatus) Equal(o PollStatus) bool {
	return (p.Ref == o.Ref) && (p.SHA == o.SHA) && (p.ETag == o.ETag)
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.spec.url`
//+kubebuilder:printcolumn:name="Ref",type=string,JSONPath=".status.pollStatus.ref",description=""
//+kubebuilder:printcolumn:name="SHA",type=string,JSONPath=".status.pollStatus.sha",description=""
//+kubebuilder:printcolumn:name="Error",type=string,JSONPath=".status.lastError",description=""

// PolledRepository is the Schema for the polledrepositories API
type PolledRepository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PolledRepositorySpec   `json:"spec,omitempty"`
	Status PolledRepositoryStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PolledRepositoryList contains a list of PolledRepository
type PolledRepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PolledRepository `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PolledRepository{}, &PolledRepositoryList{})
}
