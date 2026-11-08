#!/usr/bin/env bash
# Create every GCP resource the app needs, once. Safe to re-run.
#
# Usage: deploy/setup.sh PROJECT_ID BILLING_ACCOUNT
#
# Needs an account with Owner on the project and Billing Account Administrator
# on the billing account. The agent writes this; you run it. When it finishes,
# copy the printed values into GitHub repository variables.

set -euo pipefail

usage() {
	cat >&2 <<'EOF'
usage: deploy/setup.sh PROJECT_ID BILLING_ACCOUNT

Creates the Cloud Run, Artifact Registry, Secret Manager, GCS, service account,
Workload Identity, and billing budget resources. Safe to re-run.

Environment overrides:
  REGION         Cloud Run and bucket region (default us-central1)
  GITHUB_REPO    repo trusted by Workload Identity (default kalverra/parlay-club)
  SERVICE        Cloud Run service name (default parlay-club)
  AR_REPO        Artifact Registry repository id (default parlay-club)
  BUCKET         Litestream bucket name (default PROJECT_ID-parlay-litestream)
  BUDGET_AMOUNT  monthly budget in USD (default 5)
EOF
}

if [ "$#" -ne 2 ]; then
	usage
	exit 2
fi

PROJECT_ID="${1#projects/}"
BILLING_ACCOUNT="${2#billingAccounts/}"
if [ -z "$PROJECT_ID" ] || [ -z "$BILLING_ACCOUNT" ]; then
	usage
	exit 2
fi

REGION="${REGION:-us-central1}"
GITHUB_REPO="${GITHUB_REPO:-kalverra/parlay-club}"
SERVICE="${SERVICE:-parlay-club}"
AR_REPO="${AR_REPO:-parlay-club}"
BUCKET="${BUCKET:-${PROJECT_ID}-parlay-litestream}"
BUDGET_AMOUNT="${BUDGET_AMOUNT:-5}"
BUDGET_NAME="${BUDGET_NAME:-parlay-club monthly}"

RUNTIME_SA_NAME="parlay-runtime"
DEPLOY_SA_NAME="parlay-deployer"
RUNTIME_SA="${RUNTIME_SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"
DEPLOY_SA="${DEPLOY_SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"

SECRETS=(session-key google-client-id google-client-secret odds-api-key)

say() { printf '\n==> %s\n' "$*"; }
ok() { printf '    %s\n' "$*"; }

command -v gcloud >/dev/null 2>&1 || {
	echo "setup: gcloud not found; install the Google Cloud SDK" >&2
	exit 1
}

PROJECT_NUMBER=$(gcloud projects describe "$PROJECT_ID" --format='value(projectNumber)')
if [ -z "$PROJECT_NUMBER" ]; then
	echo "setup: cannot read project $PROJECT_ID; check the id and your access" >&2
	exit 1
fi

WIF_POOL="projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/github"
WIF_PROVIDER="${WIF_POOL}/providers/github"

say "Enabling APIs in $PROJECT_ID"
gcloud services enable \
	run.googleapis.com \
	artifactregistry.googleapis.com \
	secretmanager.googleapis.com \
	iamcredentials.googleapis.com \
	sts.googleapis.com \
	billingbudgets.googleapis.com \
	iam.googleapis.com \
	storage.googleapis.com \
	--project="$PROJECT_ID"

say "Creating the Litestream bucket gs://$BUCKET in $REGION"
if gcloud storage buckets describe "gs://$BUCKET" --project="$PROJECT_ID" >/dev/null 2>&1; then
	ok "already exists"
else
	gcloud storage buckets create "gs://$BUCKET" \
		--project="$PROJECT_ID" \
		--location="$REGION" \
		--uniform-bucket-level-access
fi

say "Creating the Artifact Registry repo $AR_REPO"
if gcloud artifacts repositories describe "$AR_REPO" \
	--project="$PROJECT_ID" --location="$REGION" >/dev/null 2>&1; then
	ok "already exists"
else
	gcloud artifacts repositories create "$AR_REPO" \
		--project="$PROJECT_ID" \
		--location="$REGION" \
		--repository-format=docker \
		--description="Parlay Club images"
fi

# Free tier for Artifact Registry is 0.5 GB, so keep the last three images and
# delete everything older. The Keep policy matches the newest three; the
# Delete policy catches the rest.
policy_file=$(mktemp)
trap 'rm -f "$policy_file"' EXIT
cat >"$policy_file" <<'JSON'
[
  {
    "name": "keep-last-three",
    "action": {"type": "Keep"},
    "mostRecentVersions": {"keepCount": 3}
  },
  {
    "name": "delete-the-rest",
    "action": {"type": "Delete"},
    "condition": {"tagState": "ANY"}
  }
]
JSON

say "Setting the image cleanup policy to keep the last three"
gcloud artifacts repositories set-cleanup-policies "$AR_REPO" \
	--project="$PROJECT_ID" \
	--location="$REGION" \
	--policy="$policy_file" \
	--quiet

say "Creating the secrets, empty"
for secret in "${SECRETS[@]}"; do
	if gcloud secrets describe "$secret" --project="$PROJECT_ID" >/dev/null 2>&1; then
		ok "secret $secret already exists"
	else
		gcloud secrets create "$secret" \
			--project="$PROJECT_ID" \
			--replication-policy=automatic
	fi
done

say "Creating the service accounts"
for entry in "${RUNTIME_SA_NAME}:Parlay Club runtime" "${DEPLOY_SA_NAME}:Parlay Club deployer"; do
	name="${entry%%:*}"
	display="${entry#*:}"
	if gcloud iam service-accounts describe "${name}@${PROJECT_ID}.iam.gserviceaccount.com" \
		--project="$PROJECT_ID" >/dev/null 2>&1; then
		ok "service account $name already exists"
	else
		gcloud iam service-accounts create "$name" \
			--project="$PROJECT_ID" \
			--display-name="$display"
	fi
done

say "Granting the runtime account the bucket and the secrets"
gcloud storage buckets add-iam-policy-binding "gs://$BUCKET" \
	--project="$PROJECT_ID" \
	--member="serviceAccount:$RUNTIME_SA" \
	--role=roles/storage.objectAdmin \
	--quiet

for secret in "${SECRETS[@]}"; do
	gcloud secrets add-iam-policy-binding "$secret" \
		--project="$PROJECT_ID" \
		--member="serviceAccount:$RUNTIME_SA" \
		--role=roles/secretmanager.secretAccessor \
		--quiet
done

say "Granting the deployer account what it needs"
for role in roles/run.admin roles/artifactregistry.writer; do
	gcloud projects add-iam-policy-binding "$PROJECT_ID" \
		--member="serviceAccount:$DEPLOY_SA" \
		--role="$role" \
		--quiet
done

gcloud iam service-accounts add-iam-policy-binding "$RUNTIME_SA" \
	--project="$PROJECT_ID" \
	--member="serviceAccount:$DEPLOY_SA" \
	--role=roles/iam.serviceAccountUser \
	--quiet

# Cloud Run pulls the image as its own service agent, not as the runtime
# account. Without read on the repo the first deploy fails to start.
gcloud artifacts repositories add-iam-policy-binding "$AR_REPO" \
	--project="$PROJECT_ID" \
	--location="$REGION" \
	--member="serviceAccount:service-${PROJECT_NUMBER}@serverless-robot-prod.iam.gserviceaccount.com" \
	--role=roles/artifactregistry.reader \
	--quiet

say "Creating the Workload Identity pool and GitHub provider"
if gcloud iam workload-identity-pools describe github \
	--project="$PROJECT_ID" --location=global >/dev/null 2>&1; then
	ok "pool github already exists"
else
	gcloud iam workload-identity-pools create github \
		--project="$PROJECT_ID" \
		--location=global \
		--display-name="GitHub Actions"
fi

if gcloud iam workload-identity-pools providers describe github \
	--project="$PROJECT_ID" \
	--location=global \
	--workload-identity-pool=github >/dev/null 2>&1; then
	ok "provider github already exists"
else
	gcloud iam workload-identity-pools providers create-oidc github \
		--project="$PROJECT_ID" \
		--location=global \
		--workload-identity-pool=github \
		--display-name="GitHub Actions" \
		--issuer-uri="https://token.actions.githubusercontent.com" \
		--attribute-mapping="google.subject=assertion.sub,attribute.repository=assertion.repository,attribute.repository_owner=assertion.repository_owner" \
		--attribute-condition="assertion.repository == '${GITHUB_REPO}'"
fi

say "Letting $GITHUB_REPO use the deployer account"
gcloud iam service-accounts add-iam-policy-binding "$DEPLOY_SA" \
	--project="$PROJECT_ID" \
	--role=roles/iam.workloadIdentityUser \
	--member="principalSet://iam.googleapis.com/${WIF_POOL}/attribute.repository/${GITHUB_REPO}" \
	--quiet

say "Creating the \$$BUDGET_AMOUNT monthly budget with 50/90/100% alerts"
if existing=$(gcloud billing budgets list \
	--billing-account="$BILLING_ACCOUNT" \
	--filter="displayName=\"$BUDGET_NAME\"" \
	--format='value(name)' 2>/dev/null) && [ -n "$existing" ]; then
	ok "budget already exists: $existing"
else
	gcloud billing budgets create \
		--billing-account="$BILLING_ACCOUNT" \
		--display-name="$BUDGET_NAME" \
		--budget-amount="${BUDGET_AMOUNT}USD" \
		--filter-projects="projects/$PROJECT_ID" \
		--threshold-rule=percent=0.5 \
		--threshold-rule=percent=0.9 \
		--threshold-rule=percent=1.0
fi

cat <<EOF

Done. Budget alert emails go to the billing account's administrators and users
by default; add yourself under Billing > Budgets if you are not one.

Set these as GitHub repository variables (Settings -> Secrets and variables ->
Actions -> Variables):

GCP_PROJECT=${PROJECT_ID}
GCP_REGION=${REGION}
WIF_PROVIDER=${WIF_PROVIDER}
DEPLOY_SA=${DEPLOY_SA}
RUNTIME_SA=${RUNTIME_SA}
LITESTREAM_BUCKET=${BUCKET}
AR_REPO=${AR_REPO}

Or with the GitHub CLI, from a clone of the repo:

  gh variable set GCP_PROJECT --body "${PROJECT_ID}"
  gh variable set GCP_REGION --body "${REGION}"
  gh variable set WIF_PROVIDER --body "${WIF_PROVIDER}"
  gh variable set DEPLOY_SA --body "${DEPLOY_SA}"
  gh variable set RUNTIME_SA --body "${RUNTIME_SA}"
  gh variable set LITESTREAM_BUCKET --body "${BUCKET}"
  gh variable set AR_REPO --body "${AR_REPO}"

Next:
  1. Add a version to each secret: session-key (32+ random bytes), the Google
     client pair from the OAuth runbook, and odds-api-key. The deploy fails
     until every secret has a version.
  2. Deploy once, then set BASE_URL to the Cloud Run URL it prints.
  3. Create a group in the app, then restart the service once so
     seed/history.yaml imports. It waits for a group and only runs at boot.
EOF
