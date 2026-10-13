#!/usr/bin/env bash
set -euo pipefail
PROJECT="${1:?Pass the Google Cloud project ID}"
REGION="${2:-europe-west1}"
COMMIT="$(git rev-parse --short=12 HEAD)"
IMAGE="$REGION-docker.pkg.dev/$PROJECT/reconcile/workbench:$COMMIT"
gcloud builds submit . --project="$PROJECT" --region="$REGION" --tag="$IMAGE" \
  --service-account="projects/$PROJECT/serviceAccounts/reconcile-build@$PROJECT.iam.gserviceaccount.com" \
  --gcs-source-staging-dir="gs://$PROJECT-reconcile-build/source" \
  --gcs-log-dir="gs://$PROJECT-reconcile-build/logs"
gcloud run deploy reconcile --project="$PROJECT" --region="$REGION" --image="$IMAGE" \
  --service-account="reconcile-demo@$PROJECT.iam.gserviceaccount.com" \
  --set-env-vars="RECONCILE_BUCKET=$PROJECT-reconcile-data" \
  --min-instances=0 --max-instances=1 --concurrency=2 --cpu=1 --memory=512Mi --timeout=60 \
  --allow-unauthenticated --quiet
gcloud run services describe reconcile --project="$PROJECT" --region="$REGION" --format='value(status.url)'
