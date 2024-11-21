# gitpoller-controller

This is a Kubernetes [controller](https://kubernetes.io/docs/concepts/architecture/controller/) that polls Git hosting service APIs to detect changes to hosted Git reference.

When a change is detected (a push to a branch), a [CloudEvent](https://cloudevents.io/) is sent to a configurable endpoint.

The polling mechanism uses ETags to minimise the API overhead of hitting the GitHub API.

## Installation

Install the controller

```shell
$ kubectl apply -f <FIXME>!
```

Create a `PolledRepository`:

```yaml
apiVersion: polling.gitops.tools/v1alpha1
kind: PolledRepository
metadata:
  name: polledrepository-sample
  namespace: default
spec:
  url: https://github.com/bigkevmcd/go-demo.git
  ref: main
  type: github
  frequency: 5m
  endpoint: http://el-polling-listener.polling-demo.svc.cluster.local:8080
```

This will check the repo above, using the GitHub API, and when the `HEAD` of the `main` branch changes, a cloud event will be sent to the endpoint.

NOTE: The cloud event is not the same as the hook event, it is the response from
the respective API.

You can see what the JSON looks like by running this command (for GitHub).

```shell
$ curl "https://api.github.com/repos/bigkevmcd/go-demo/commits/main" -H "Accept: application/vnd.github.chitauri-preview+sha"
```

## CloudEvent

The endpoint will receive a CloudEvent:

```
POST / HTTP/1.1
Ce-Subject: /apis/polling.gitops.tools/v1alpha1/namespaces/testing/PolledRepository/test-repository
Ce-Source:  https://github.com/bigkevmcd/go-demo.git
Ce-Type: commit
{
  // This will contain the API response from GitHub or GitLab.
}
```

The `Subject` of the event is the object reference, and the `Source` is the `spec.url` field from the PolledRepository.

You can parse the incoming event in your own HTTP handlers, and there are SDKs for various languages, including the [Go SDK](https://github.com/cloudevents/sdk-go#receive-your-first-cloudevent).

## Using with Tekton Triggers

This can also be used to drive Tekton Triggers, ordinarily you'd hook up a GitHub webhook, but many organisations don't allow incoming events from the Internet, if this is the case, you can drive your hooks with the poller.

First of all, install [Tekton Pipeline](https://github.com/tektoncd/pipeline/blob/main/docs/install.md) and [Triggers](https://github.com/tektoncd/triggers/blob/main/docs/install.md).

```shell
$ kubectl apply -f https://storage.googleapis.com/tekton-releases/pipeline/previous/v0.32.0/release.yaml
$ kubectl apply -f https://storage.googleapis.com/tekton-releases/triggers/previous/v0.18.0/release.yaml
$ kubectl apply -f https://storage.googleapis.com/tekton-releases/triggers/previous/v0.18.0/interceptors.yaml
```

Then apply the example Triggers [files](./examples/triggers.yaml) this create
the following resources in the `polling-demo` namespace.

 * Service Account  `polling-demo-sa`
 * ClusterRole `polling-demo-clusterrole`
 * RoleBinding `polling-demo-rolebinding`
 * TriggerTemplate `polling-demo`
 * TriggerBinding `polling-demo`
 * EventListener `polling-listener`
 * Pipeline `github-poll-pipeline`

The example `TriggerBinding` extracts two fields from the commit.

```yaml
apiVersion: triggers.tekton.dev/v1alpha1
kind: TriggerBinding
metadata:
  name: polling-pipeline-binding
  namespace: polling-demo
spec:
  params:
    - name: sha
      value: $(body.commit.tree.sha)
    - name: repoURL
      value: "$(header.Ce-source)"
```

In the example, the Pipeline contains a very simple task:

```yaml
apiVersion: tekton.dev/v1beta1
kind: Pipeline
metadata:
  name: github-poll-pipeline
  namespace: polling-demo
spec:
  params:
  - name: sha
    type: string
    description: "the SHA of the recently detected change"
  - name: repoURL
    type: string
    description: "the cloneURL that the change was detected in"
  tasks:
  - name: echo-changes
    params:
    - name: sha
      value: $(params.sha)
    - name: repoURL
      value: $(params.repoURL)
    taskSpec:
      params:
      - name: sha
        type: string
      - name: repoURL
        type: string
      steps:
      - name: echo
        image: alpine
        script: |
          echo "SHA change detected $(inputs.params.sha)"
          echo "                    $(inputs.params.repoURL)"
```

## Building

If you want to build this, you will need Go installed and you can use the
Makefile:

```shell
$ IMG=my-org/my-image:tag make docker-build docker-push
```

You can deploy your own image with:

```shell
$ IMG=my-org/my-image:tag make deploy
```

There's an additional `release` target that will generate a file `release-<version>.yaml` which contains all the necessary files to deploy your controller.

## Examples of the Cloud Event Body

## GitHub

```json
{
  "author": {
    "avatar_url": "https://avatars.githubusercontent.com/u/867746?v=4",
    "events_url": "https://api.github.com/users/example/events{/privacy}",
    "followers_url": "https://api.github.com/users/example/followers",
    "following_url": "https://api.github.com/users/example/following{/other_user}",
    "gists_url": "https://api.github.com/users/example/gists{/gist_id}",
    "gravatar_id": "",
    "html_url": "https://github.com/example",
    "id": 867746,
    "login": "example",
    "node_id": "MDQ6VXNlcjg2Nzc0Ng==",
    "organizations_url": "https://api.github.com/users/example/orgs",
    "received_events_url": "https://api.github.com/users/example/received_events",
    "repos_url": "https://api.github.com/users/example/repos",
    "site_admin": false,
    "starred_url": "https://api.github.com/users/example/starred{/owner}{/repo}",
    "subscriptions_url": "https://api.github.com/users/example/subscriptions",
    "type": "User",
    "url": "https://api.github.com/users/example",
    "user_view_type": "public"
  },
  "comments_url": "https://api.github.com/repos/example/repo/commits/0469c9b4a9fdbec5fe7a06000ba0c5dad99b0384/comments",
  "commit": {
    "author": {
      "date": "2023-12-15T09:35:55Z",
      "email": "example@gmail.com",
      "name": "Kevin McDermott"
    },
    "comment_count": 0,
    "committer": {
      "date": "2023-12-15T09:35:55Z",
      "email": "noreply@github.com",
      "name": "GitHub"
    },
    "message": "Update kustomization.yaml",
    "tree": {
      "sha": "bb194c136b49ab0cd44a06e51d3ed5c8c32b7d39",
      "url": "https://api.github.com/repos/example/repo/git/trees/bb194c136b49ab0cd44a06e51d3ed5c8c32b7d39"
    },
    "url": "https://api.github.com/repos/example/repo/git/commits/0469c9b4a9fdbec5fe7a06000ba0c5dad99b0384",
    "verification": {
      "payload": "tree bb194c136b49ab0cd44a06e51d3ed5c8c32b7d39\nparent 72c6f14b1be29dd6cc80a722018165a0e10ff378\nauthor Example User <example@example.com> 1702632955 +0000\ncommitter GitHub <noreply@github.com> 1702632955 +0000\n\nUpdate kustomization.yaml",
      "reason": "valid",
      "signature": "-----BEGIN PGP SIGNATURE-----\n\nwsBcBAABCAAQBQJlfB37CRBK7hj4Ov3rIwAAVSsIAJoFQvKj76XKhSt90JGl2D+L\nlNtr1+t4u7AMUwtFQRyAdjgPXZ+Z6r/4echXWHTKtBuNAhpbyXjWnY1BIaqN1xm/\n4BSILbA4VTmoQ9ICATdlzoxNOmO5xineSCFth/bMguZpfoNkoJIoIMBzU1wDZP7L\nruC4I4lc4JaD1SqNvdBSLt3cq3aT3iTqdFjP6CTNN+g0C3WlL+8BfYZoWOvVywDD\nxDkBm/ApMuzEE0YGGyyJcZ8k9r+1pNq2g2qblab1zZKrdcKls48OvyWIQoWwcX5Z\nskTrg/wsXI1lI9EBH3ooIyrWvutWLUxaVoar3kAl9EghobTJPhEHlRCtkqmRcaI=\n=WJHF\n-----END PGP SIGNATURE-----\n",
      "verified": true,
      "verified_at": "2024-01-16T19:59:59Z"
    }
  },
  "committer": {
    "avatar_url": "https://avatars.githubusercontent.com/u/19864447?v=4",
    "events_url": "https://api.github.com/users/web-flow/events{/privacy}",
    "followers_url": "https://api.github.com/users/web-flow/followers",
    "following_url": "https://api.github.com/users/web-flow/following{/other_user}",
    "gists_url": "https://api.github.com/users/web-flow/gists{/gist_id}",
    "gravatar_id": "",
    "html_url": "https://github.com/web-flow",
    "id": 19864447,
    "login": "web-flow",
    "node_id": "MDQ6VXNlcjE5ODY0NDQ3",
    "organizations_url": "https://api.github.com/users/web-flow/orgs",
    "received_events_url": "https://api.github.com/users/web-flow/received_events",
    "repos_url": "https://api.github.com/users/web-flow/repos",
    "site_admin": false,
    "starred_url": "https://api.github.com/users/web-flow/starred{/owner}{/repo}",
    "subscriptions_url": "https://api.github.com/users/web-flow/subscriptions",
    "type": "User",
    "url": "https://api.github.com/users/web-flow",
    "user_view_type": "public"
  },
  "files": [
    {
      "additions": 1,
      "blob_url": "https://github.com/example/repo/blob/0469c9b4a9fdbec5fe7a06000ba0c5dad99b0384/examples%2Fkustomize%2Fenvironments%2Fstaging%2Fkustomization.yaml",
      "changes": 8,
      "contents_url": "https://api.github.com/repos/example/repo/contents/examples%2Fkustomize%2Fenvironments%2Fstaging%2Fkustomization.yaml?ref=0469c9b4a9fdbec5fe7a06000ba0c5dad99b0384",
      "deletions": 7,
      "filename": "examples/kustomize/environments/staging/kustomization.yaml",
      "patch": "@@ -1,13 +1,7 @@\n namespace: staging\n images:\n - name: example/repo\n-  newTag: demo\n-apiVersion: kustomize.config.k8s.io/v1beta1\n-kind: Kustomization\n+  newTag: v1.2.3\n resources:\n - ../../base\n - namespace.yaml\n-labels:\n-- includeSelectors: true\n-  pairs:\n-    gitops.pro/pipeline-environment: staging",
      "raw_url": "https://github.com/example/repo/raw/0469c9b4a9fdbec5fe7a06000ba0c5dad99b0384/examples%2Fkustomize%2Fenvironments%2Fstaging%2Fkustomization.yaml",
      "sha": "5f136971df5821e8c39dd3515cae156ffc8887ad",
      "status": "modified"
    }
  ],
  "html_url": "https://github.com/example/repo/commit/0469c9b4a9fdbec5fe7a06000ba0c5dad99b0384",
  "node_id": "C_kwDOEAWX1doAKDA0NjljOWI0YTlmZGJlYzVmZTdhMDYwMDBiYTBjNWRhZDk5YjAzODQ",
  "parents": [
    {
      "html_url": "https://github.com/example/repo/commit/72c6f14b1be29dd6cc80a722018165a0e10ff378",
      "sha": "72c6f14b1be29dd6cc80a722018165a0e10ff378",
      "url": "https://api.github.com/repos/example/repo/commits/72c6f14b1be29dd6cc80a722018165a0e10ff378"
    }
  ],
  "sha": "0469c9b4a9fdbec5fe7a06000ba0c5dad99b0384",
  "stats": {
    "additions": 1,
    "deletions": 7,
    "total": 8
  },
  "url": "https://api.github.com/repos/example/repo/commits/0469c9b4a9fdbec5fe7a06000ba0c5dad99b0384"
}
```


## Development

This is a kubebuilder derived controller.

## Roadmap

 - [ ] Support for custom API endpoints
 - [ ] Support generic Git repository polling
 - [ ] Allow more generic HTTP event sending (cloud-events don't appear to allow signing)
 - [ ] Support for custom TLS CAs in the client
 - [ ] Support for [CDEvents](https://cdevents.dev/) if possible
