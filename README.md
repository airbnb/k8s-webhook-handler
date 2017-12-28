# k8s-ci-purger
The k8s-ci-purger listens for (GitHub) webhooks on branch deletion and deletes
all resources in a kubernetes cluster that have a label matching the repo name
and are in a namespace matching the branch name.

## Usage
Currently only github delete webhooks in json format are supported.
Beside the manifests and templates in `deploy/`, a secret 'k8s-ci' with
`GITHUB_SECRET` is expected. The value should match the "Secret" field in the
GitHub webhook settings and can be created like this:

```
kubectl create secret generic k8s-ci --from-literal=GITHUB_SECRET=github-secret
```
