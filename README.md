# k8s-ci-purger
The k8s-ci-purger listens for (GitHub) webhooks on branch deletion and deletes
all resources in a kubernetes cluster that have a label matching the repo name
and are in a namespace matching the branch name.

