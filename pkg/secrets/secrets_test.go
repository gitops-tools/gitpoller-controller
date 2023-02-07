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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ SecretGetter = (*KubeSecretGetter)(nil)

var testID = types.NamespacedName{Name: "test-secret", Namespace: "test-ns"}

func TestSecretToken(t *testing.T) {
	g := New(fake.NewFakeClient(createSecret(testID, "secret-token")))

	secret, err := g.SecretToken(context.TODO(), testID, "token")
	if err != nil {
		t.Fatal(err)
	}

	if secret != "secret-token" {
		t.Fatalf("got %s, want secret-token", secret)
	}
}

func TestSecretTokenWithMissingKey(t *testing.T) {
	g := New(fake.NewFakeClient(createSecret(testID, "secret-token")))

	_, err := g.SecretToken(context.TODO(), testID, "unknown")
	if err.Error() != `secret invalid, no "unknown" key in test-ns/test-secret` {
		t.Fatal(err)
	}
}

func TestSecretTokenWithMissingSecret(t *testing.T) {
	g := New(fake.NewFakeClient())

	_, err := g.SecretToken(context.TODO(), testID, "token")
	if err.Error() != `error getting secret test-ns/test-secret: secrets "test-secret" not found` {
		t.Fatal(err)
	}
}

func createSecret(id types.NamespacedName, token string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      id.Name,
			Namespace: id.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"token": []byte(token),
		},
	}
}
