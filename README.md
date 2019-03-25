# k8s-webhook-handler
The k8s-webhook-handler listens for (GitHub) webhooks and acts on various events

## Event Handlers
### DeleteEvent
On branch deletion and deletes all resources in a kubernetes cluster that have a
label matching the repo name and are in a namespace matching the branch name.
If there are no other objects with the given label key in the namespace, it also
deletes the namespace and all remaining objects.

### PushEvent
On push events, k8s-webhook-handler will checkout `.ci/workflow.yaml` from the
repo the push was and submit it to the k8s api with the following annotations
added:

 - `k8s-webhook-handler.io/ref`: event.Ref
 - `k8s-webhook-handler.io/revision`: event.HeadCommit.ID
 - `k8s-webhook-handler.io/repo_name`: event.Repo.FullName
 - `k8s-webhook-handler.io/repo_url`: event.Repo.GitURL
 - `k8s-webhook-handler.io/repo_ssh`: event.Repo.SSHURL

## Binaries
- cmd/webhook is the actual webhook handling server
- cmd/reconciler iterates over all k8s namespaces and deletes all objects that
  are labeled for which there is no remote branch anymore.

## Usage
Currently only github delete webhooks in json format are supported.
Beside the manifests and templates in `deploy/`, a secret 'webhook-handler' with
the following fields is expected:

- `GITHUB_TOKEN` Personal Access Token for API access
- `WEBHOOK_SECRET` Secret for validating the webhook

The value should match the "Secret" field in the GitHub webhook settings and can be created like this:

```
kubectl create secret generic k8s-ci --from-literal=GITHUB_SECRET=github-secret ...
```
