#!/usr/bin/env bash
set -euo pipefail
PROJECT="${1:?Pass the Google Cloud project ID}"
REGION="${2:-europe-west1}"
DATA_BUCKET="$PROJECT-reconcile-data"
BUILD_BUCKET="$PROJECT-reconcile-build"
RUNTIME="reconcile-demo@$PROJECT.iam.gserviceaccount.com"
BUILDER="reconcile-build@$PROJECT.iam.gserviceaccount.com"
gcloud services enable run.googleapis.com cloudbuild.googleapis.com artifactregistry.googleapis.com storage.googleapis.com iam.googleapis.com --project="$PROJECT"
for NAME in reconcile-demo reconcile-build; do
  if ! gcloud iam service-accounts describe "$NAME@$PROJECT.iam.gserviceaccount.com" --project="$PROJECT" >/dev/null 2>&1; then
    gcloud iam service-accounts create "$NAME" --project="$PROJECT" --display-name="$NAME"
  fi
done
for BUCKET in "$DATA_BUCKET" "$BUILD_BUCKET"; do
  if ! gcloud storage buckets describe "gs://$BUCKET" --project="$PROJECT" >/dev/null 2>&1; then
    gcloud storage buckets create "gs://$BUCKET" --project="$PROJECT" --location="$REGION" --uniform-bucket-level-access --public-access-prevention
  fi
  gcloud storage buckets update "gs://$BUCKET" --lifecycle-file=deploy/lifecycle.json
done
gcloud storage buckets add-iam-policy-binding "gs://$DATA_BUCKET" --member="serviceAccount:$RUNTIME" --role=roles/storage.objectUser
gcloud storage buckets add-iam-policy-binding "gs://$BUILD_BUCKET" --member="serviceAccount:$BUILDER" --role=roles/storage.objectAdmin
gcloud storage buckets add-iam-policy-binding "gs://$BUILD_BUCKET" --member="serviceAccount:$BUILDER" --role=roles/storage.legacyBucketReader
if ! gcloud artifacts repositories describe reconcile --location="$REGION" --project="$PROJECT" >/dev/null 2>&1; then
  gcloud artifacts repositories create reconcile --repository-format=docker --location="$REGION" --project="$PROJECT" --description='Reconcile demo images'
fi
gcloud artifacts repositories add-iam-policy-binding reconcile --location="$REGION" --project="$PROJECT" --member="serviceAccount:$BUILDER" --role=roles/artifactregistry.writer
