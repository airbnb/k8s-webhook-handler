#!/bin/bash
set -euo pipefail
API_URL="${API_URL:-https://api.github.com}"

NAMESPACE="${NAMESPACE:-ci}"
SECRET=webhook-handler

repo="$1"
url="$2"

data=$(kubectl -n "$NAMESPACE" get secret "$SECRET" -o json | jq -r .data)
GITHUB_TOKEN=$(echo "$data" | jq -r .GITHUB_TOKEN | openssl base64 -d)
WEBHOOK_SECRET=$(echo "$data" | jq -r .WEBHOOK_SECRET | openssl base64 -d)

cat <<EOF | curl -X POST -H "Authorization: token $GITHUB_TOKEN" -f --data @- "$API_URL/repos/$repo/hooks"
{
  "name": "web",
  "active": true,
  "events": [
    "push",
    "delete"
  ],
  "config": {
    "url": "$2",
    "content_type": "json",
    "secret": "$WEBHOOK_SECRET"
  }
}
EOF
