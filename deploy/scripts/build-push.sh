#!/usr/bin/env bash
# Builds every image for the k3s node (linux/arm64) and pushes it to ECR under the given tag:
#   deploy/scripts/build-push.sh v2
# The four services and migrate come from this repository, the dashboard from DASHBOARD_DIR
# (default: ../wau-dashboard beside this repository). Needs the AWS CLI, signed in to the
# account, and Docker with buildx. The ECR repositories wau/<name> must exist.
set -euo pipefail

TAG="${1:?usage: build-push.sh <tag>}"
REGION=ap-southeast-1
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DASHBOARD_DIR="$(cd "${DASHBOARD_DIR:-$ROOT/../wau-dashboard}" && pwd)"

ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
REGISTRY="$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com"
aws ecr get-login-password --region "$REGION" | docker login --username AWS --password-stdin "$REGISTRY"

# build <name> <docker buildx build arguments...>
build() {
	local name="$1"
	shift
	echo "==> $REGISTRY/wau/$name:$TAG"
	docker buildx build --platform linux/arm64 --push --tag "$REGISTRY/wau/$name:$TAG" "$@"
}

for service in challenge verify tenant billing; do
	build "$service" --build-arg VERSION="$TAG" --file "$ROOT/services/$service/Dockerfile" "$ROOT"
done
build migrate "$ROOT/infra/postgres"
build dashboard "$DASHBOARD_DIR"
