# k8s-webhook-handler
Create Kubernetes resources in response to (GitHub) webhooks!

## How does it work?
When the k8s-webhook-handler receives a webhook, it:

- Validates the payload's signature by using the `WEBHOOK_SECRET` as HMAC hexdigest secret
- Downloads a manifest (`.ci/workflow.yaml` by default) from the repository.

For push events, it downloads the manifest from the given revision. Otherwise
it's checked out from the repository's default branch.

After that, it applies the manifest and adds the following annotations:

 - `k8s-webhook-handler.io/ref`: Git reference (e.g. `refs/heads/master`)
 - `k8s-webhook-handler.io/revision`: The SHA of the most recent commit on `ref`
   after the push.
 - `k8s-webhook-handler.io/before`: The SHA of the most recent commit on `ref`
   before the push.
 - `k8s-webhook-handler.io/repo_name`: Repo name including user (e.g.
   `airbnb/k8s-webhook-handler`)
 - `k8s-webhook-handler.io/repo_url`: git URL (e.g.
   `git://github.com/airbnb/k8s-webhook-handler.git`)
 - `k8s-webhook-handler.io/repo_ssh`: ssh URL (e.g.
   `git@github.com:airbnb/k8s-webhook-handler.git`)
 - `k8s-webhook-handler.io/event_type`: Event type (e.g. `push` or `delete`)
 - `k8s-webhook-handler.io/event_action`: Event type specific action (e.g. `created` or `deleted`)

(For details, see the [GitHub Events Docs](https://developer.github.com/v3/activity/events/).

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

## Security
The `WEBHOOK_SECRET` is required for secure operation. Running without means not
validating the webhooks which effectively grants everyone permission to run
arbitrary manifests on your cluster. If you really need to run without
validation e.g for testing purposes, you can run the handler with the
`-insecure` flag.
