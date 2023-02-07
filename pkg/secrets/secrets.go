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
package secrets

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// KubeSecretGetter is an implementation of SecretGetter.
type KubeSecretGetter struct {
	kubeClient client.Client
}

// New creates and returns a KubeSecretGetter that looks up secrets in k8s.
func New(c client.Client) *KubeSecretGetter {
	return &KubeSecretGetter{
		kubeClient: c,
	}
}

// SecretToken looks for a namespaced secret, and returns the key from
// it, or an error if not found.
func (k KubeSecretGetter) SecretToken(ctx context.Context, id types.NamespacedName, key string) (string, error) {
	loaded := &corev1.Secret{}
	err := k.kubeClient.Get(context.TODO(), id, loaded)
	if err != nil {
		return "", fmt.Errorf("error getting secret %s/%s: %w", id.Namespace, id.Name, err)
	}
	token, ok := loaded.Data[key]
	if !ok {
		return "", fmt.Errorf("secret invalid, no %#v key in %s/%s", key, id.Namespace, id.Name)
	}
	return string(token), nil
}
