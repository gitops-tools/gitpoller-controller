package controllers

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

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	pollingv1alpha1 "github.com/gitops-tools/gitpoller-controller/api/v1alpha1"
)

const (
	testRepositoryName = "test-repository"
	testSecret         = "test-secret"
)

var _ = Describe("PolledRepositoryReconciler", func() {
	var (
		repo   pollingv1alpha1.PolledRepository
		secret *corev1.Secret
	)

	Context("when a PolledRepository is created", func() {
		JustBeforeEach(func() {
			ctx := context.TODO()
			repo = pollingv1alpha1.PolledRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testRepositoryName,
					Namespace: testNamespace,
				},
				Spec: pollingv1alpha1.PolledRepositorySpec{
					URL:      testRepoURL,
					Ref:      testRef,
					Endpoint: "https://example.com/testing",
					Type:     pollingv1alpha1.GitHub,
				},
			}
			if requiredAuth != "" {
				secret = makeTestSecret()
				repo.Spec.Auth = &pollingv1alpha1.AuthSecret{
					SecretReference: corev1.SecretReference{
						Name:      testSecret,
						Namespace: testNamespace,
					},
					Key: "token",
				}
				Expect(k8sClient.Create(ctx, secret)).To(Succeed())
			}
			Expect(k8sClient.Create(ctx, &repo)).To(Succeed())
		})

		AfterEach(func() {
			ctx := context.TODO()
			Expect(k8sClient.Delete(ctx, &repo)).To(Succeed())
			if requiredAuth != "" {
				Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
			}
			requiredAuth = ""
			mockPoller.Reset()
		})

		It("polls the git repository", func() {
			ctx := context.TODO()
			completeStatus := pollingv1alpha1.PollStatus{Ref: testRef, SHA: testCommitSHA,
				ETag: testCommitETag}
			responseBody := map[string]interface{}{"id": testRef}

			mockPoller.AddMockResponse("bigkevmcd/go-demo", pollingv1alpha1.PollStatus{Ref: testRef},
				responseBody,
				completeStatus)

			mockPoller.AddMockResponse("bigkevmcd/go-demo",
				completeStatus,
				responseBody,
				completeStatus)

			Eventually(func() *pollingv1alpha1.PolledRepositoryStatus {
				var loaded pollingv1alpha1.PolledRepository
				if err := k8sClient.Get(ctx, keyForObj(&repo), &loaded); err != nil {
					return nil
				}
				return &loaded.Status
			}, timeout, time.Millisecond*500).Should(Equal(
				&pollingv1alpha1.PolledRepositoryStatus{
					PollStatus: pollingv1alpha1.PollStatus{
						Ref:  testRef,
						SHA:  testCommitSHA,
						ETag: testCommitETag,
					},
				}))

			It("dispatches a notification", func() {
				Eventually(func() *dispatch {
					if len(dispatcher.dispatched) > 0 {
						return &dispatcher.dispatched[0]
					}
					return nil
				}, timeout, time.Millisecond*500).Should(Equal(&dispatch{
					endpoint: "https://example.com/testing",
					commit:   nil,
				}))
			})
		})

		It("passes authentication to the service", func() {
			Eventually(func() map[string]interface{} {
				return nil
			}, timeout, time.Millisecond*500).Should(Equal(""))
		})

	})
})

func makeTestSecret() *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      testSecret,
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"token": []byte(requiredAuth),
		},
	}
}

func keyForObj(r runtime.Object) types.NamespacedName {
	oa, err := meta.Accessor(r)
	Expect(err).NotTo(HaveOccurred())
	return types.NamespacedName{
		Name:      oa.GetName(),
		Namespace: oa.GetNamespace(),
	}
}
