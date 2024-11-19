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

package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	pollingv1 "github.com/gitops-tools/gitpoller-controller/api/v1alpha1"
	"github.com/gitops-tools/gitpoller-controller/pkg/git"
	"github.com/gitops-tools/gitpoller-controller/pkg/secrets"
)

// EventDispatcher implementations publish the commit to the endpoint in the
// PolledRepository.
type EventDispatcher interface {
	Dispatch(ctx context.Context, repo pollingv1.PolledRepository, commit map[string]any) error
}

type pollerFactoryFunc func(cl *http.Client, repo *pollingv1.PolledRepository, endpoint, authToken string) git.CommitPoller

// PolledRepositoryReconciler reconciles a PolledRepository object
type PolledRepositoryReconciler struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	HTTPClient    *http.Client
	PollerFactory pollerFactoryFunc
	EventDispatcher
	secrets.SecretGetter
}

// +kubebuilder:rbac:groups=polling.gitops.tools,resources=polledrepositories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=polling.gitops.tools,resources=polledrepositories/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=polling.gitops.tools,resources=polledrepositories/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is part of the main Kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PolledRepositoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := logr.FromContextOrDiscard(ctx)
	reqLogger.Info("reconciling PolledRepository")

	repo := pollingv1.PolledRepository{}
	err := r.Client.Get(ctx, req.NamespacedName, &repo)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to load repository %s: %w", req, err)
	}

	repoName, endpoint, err := repoFromURL(repo.Spec.URL)
	if err != nil {
		reqLogger.Error(err, "Parsing the repo from the URL failed", "repoURL", repo.Spec.URL)
		return ctrl.Result{}, err
	}
	repo.Status.PollStatus.Ref = repo.Spec.Ref

	authToken, err := r.authTokenForRepo(ctx, reqLogger, req.Namespace, repo)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get auth token")
	}

	// TODO: handle pollerFactory returning nil/error
	newStatus, commit, err := r.PollerFactory(r.HTTPClient, &repo, endpoint, authToken).Poll(ctx, repoName, repo.Status.PollStatus)
	if err != nil {
		repo.Status.LastError = err.Error()
		reqLogger.Error(err, "repository poll failed")
		if err := r.Client.Status().Update(ctx, &repo); err != nil {
			reqLogger.Error(err, "unable to update Repository status")
		}
		// TODO: improve the error
		return ctrl.Result{}, err
	}

	reqLogger.Info("polled", "status", newStatus)

	repo.Status.LastError = ""
	changed := !newStatus.Equal(repo.Status.PollStatus)
	if repo.Status.LastError != "" {
		repo.Status.LastError = ""
		changed = true
	}
	if !changed {
		reqLogger.Info("poll status unchanged, requeueing next check", "frequency", repo.Spec.Frequency)
		return ctrl.Result{RequeueAfter: repo.Spec.Frequency.Duration}, nil
	}

	reqLogger.Info("poll status changed", "status", newStatus)
	repo.Status.PollStatus = newStatus
	if err := r.Client.Status().Update(ctx, &repo); err != nil {
		reqLogger.Error(err, "unable to update Repository status")
		// TODO: improve the error
		return ctrl.Result{}, err
	}

	if err := r.EventDispatcher.Dispatch(ctx, repo, commit); err != nil {
		reqLogger.Error(err, "failed to dispatch commit")
		// TODO: improve the error
		return ctrl.Result{}, err
	}

	reqLogger.Info("requeueing next check", "frequency", repo.Spec.Frequency.Duration)
	return ctrl.Result{RequeueAfter: repo.Spec.Frequency.Duration}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PolledRepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pollingv1.PolledRepository{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}

func (r *PolledRepositoryReconciler) authTokenForRepo(ctx context.Context, logger logr.Logger, namespace string, repo pollingv1.PolledRepository) (string, error) {
	if repo.Spec.Auth == nil {
		return "", nil
	}
	key := "token"
	if repo.Spec.Auth.Key != "" {
		key = repo.Spec.Auth.Key
	}
	authToken, err := r.SecretGetter.SecretToken(ctx, types.NamespacedName{Name: repo.Spec.Auth.Name, Namespace: namespace}, key)
	if err != nil {
		logger.Error(err, "Getting the auth token failed", "name", repo.Spec.Auth.Name, "namespace", namespace, "key", key)
		return "", err
	}
	return authToken, nil
}

func repoFromURL(s string) (string, string, error) {
	parsed, err := url.Parse(s)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse repo from URL %#v: %s", s, err)
	}
	host := parsed.Host
	if strings.HasSuffix(host, "github.com") {
		host = "api." + host
	}
	endpoint := fmt.Sprintf("%s://%s", parsed.Scheme, host)
	return strings.TrimPrefix(strings.TrimSuffix(parsed.Path, ".git"), "/"), endpoint, nil
}

// MakeCommitPoller creates the correct poller from the repository with
// authentication.
// TODO: allow custom TLS
func MakeCommitPoller(cl *http.Client, repo *pollingv1.PolledRepository, endpoint, authToken string) git.CommitPoller {
	switch repo.Spec.Type {
	case pollingv1.GitHub:
		return git.NewGitHubPoller(cl, endpoint, authToken)
	case pollingv1.GitLab:
		return git.NewGitLabPoller(cl, endpoint, authToken)
	}
	return nil
}
