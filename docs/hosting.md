# Hosting Reconcile

Cloud Run serves the complete app. A private Cloud Storage bucket preserves bounded SQLite workspace snapshots. The local server continues to use `data/reconcile.db`; local files and credentials are excluded from the build upload.

## Reproduce a deployment

From Google Cloud Shell, clone the repository and run from its root:

```sh
bash deploy/bootstrap.sh YOUR_PROJECT_ID europe-west1
bash deploy/deploy.sh YOUR_PROJECT_ID europe-west1
```

The bootstrap creates dedicated build/runtime service accounts, private data/build buckets, and an Artifact Registry repository. The runtime account has object access only on the data bucket; the build account accesses the build bucket and image repository. Cloud Run's HTTPS endpoint is public, while the data bucket remains private.

The deployment builds the committed source into a container. It uses request-based billing, zero minimum instances, one maximum instance per revision and two concurrent requests. These settings reduce idle costs but are not a spending cap or a promise of free hosting. The project must have billing enabled. Both buckets have seven-day lifecycle rules; Cloud Storage soft-delete retention can retain recoverable copies longer.

## Ownership and persistence

A random 256-bit Secure, HttpOnly, SameSite=Lax cookie selects a browser's workspace. Lax preserves the workspace when returning through a GitHub link. The server rejects cross-origin mutation requests. Run IDs alone grant no access across workspaces.

For each request, the app restores the selected snapshot into a private temporary directory. Mutations publish a checkpointed SQLite file with a generation precondition, and report success only after Cloud Storage accepts it. If two requests start from the same generation, only one can publish; the other receives a conflict. Operation IDs permit safe retries after an uncertain response. The temporary container filesystem is never the durable source of truth.

This approach deliberately reads/writes the full snapshot. It is limited to a small interview demo: 20 runs and 32 MiB per workspace. For production, move to a transactional hosted database with authenticated users, finer-grained storage and explicit operational retention requirements.

## Verify the real service

```sh
go run ./cmd/smoke -url https://YOUR_SERVICE_URL
# After deploying a fresh revision, verify the same saved review still exists:
go run ./cmd/smoke -url https://YOUR_SERVICE_URL -verify
```

The smoke command uses fictional samples and checks import, mapping, rerun, deltas, review, export and independent workspace isolation. It saves its private test cookie only under ignored `data/`; never share that file. Browser workflows remain covered by Playwright.
