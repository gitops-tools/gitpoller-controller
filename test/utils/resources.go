package utils

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewSecret creates and returns a new Secret.
//
// The data is converted to map[string][]byte as a convenience.
func NewSecret(data map[string]string, opts ...func(*corev1.Secret)) *corev1.Secret {
	cm := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "demo-secret",
			// TODO: pass in a types.NamespacedName
			Namespace: "testing",
		},
		Data: dataToBytes(data),
	}

	for _, o := range opts {
		o(cm)
	}

	return cm
}

func dataToBytes(src map[string]string) map[string][]byte {
	result := map[string][]byte{}
	for k, v := range src {
		result[k] = []byte(v)
	}

	return result
}
