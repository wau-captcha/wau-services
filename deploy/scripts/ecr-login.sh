#!/usr/bin/env bash
# Refreshes the image pull secret ecr-pull in namespace wau. Runs on the EC2 node, as root (the
# k3s kubeconfig is root's), with the instance role's credentials. An ECR token lasts 12 hours,
# so cron runs this every 6:
#   0 */6 * * * /usr/local/bin/ecr-login.sh >> /var/log/ecr-login.log 2>&1
set -euo pipefail

export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
# cron starts with a bare PATH; kubectl (k3s) and the AWS CLI live in /usr/local/bin.
export PATH="/usr/local/bin:/usr/bin:/bin:$PATH"
REGION=ap-southeast-1
NAMESPACE=wau

ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
REGISTRY="$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com"

kubectl create secret docker-registry ecr-pull \
	--namespace "$NAMESPACE" \
	--docker-server="$REGISTRY" \
	--docker-username=AWS \
	--docker-password="$(aws ecr get-login-password --region "$REGION")" \
	--dry-run=client --output yaml | kubectl apply --filename -
echo "$(date -u +%FT%TZ) ecr-pull refreshed for $REGISTRY"
