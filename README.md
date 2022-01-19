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

## Development

This is an [operator-sdk](https://sdk.operatorframework.io/) derived controller.

## Roadmap

 - [ ] Support for custom API endpoints
 - [ ] Support generic Git repository polling
 - [ ] Allow more generic HTTP event sending (cloud-events don't appear to allow signing)
 - [ ] Support for custom TLS CAs in the client
