# k8s-webhook-handler
Create Kubernetes resources in response to (GitHub) webhooks!

## How does it work?
When the k8s-webhook-handler receives a webhook, it downloads a manifest
(`.ci/workflow.yaml` by default) from the repository.

For push events, it downloads the manifest from the given revision. Otherwise
it's checked out from the repository's default branch.

After that, it applies the manifest and adds the following annotations:

 - `k8s-webhook-handler.io/ref`: Git reference (e.g. `refs/heads/master`)
 - `k8s-webhook-handler.io/revision`: Revision of HEAD
 - `k8s-webhook-handler.io/repo_name`: Repo name including user
   (e.g. `itskoko/k8s-webhook-handler`)
 - `k8s-webhook-handler.io/repo_url`: git URL (e.g.
   `git://github.com/itskoko/k8s-webhook-handler.git`)
 - `k8s-webhook-handler.io/repo_ssh`: ssh URL (e.g.
   `git@github.com:itskoko/k8s-webhook-handler.git`)

## Binaries
- cmd/webhook is the actual webhook handling server

## Usage
Beside the manifests and templates in `deploy/`, a secret 'webhook-handler' with
the following fields is expected:

- `GITHUB_TOKEN` Personal Access Token for API access
- `WEBHOOK_SECRET` Secret for validating the webhook

The value should match the "Secret" field in the GitHub webhook settings and can be created like this:

```
kubectl create secret generic k8s-ci --from-literal=GITHUB_SECRET=github-secret ...
```
