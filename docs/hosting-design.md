# Public interview demo

The user authorised hosting on 13 October 2026. Use their existing Google Cloud project; preserve the local SQLite workflow.

## Runtime and ownership

Cloud Run serves the Vue build and Go API together over HTTPS. Each browser receives a random 256-bit HttpOnly, Secure, SameSite=Lax cookie. Its workspace is independent: run IDs do not grant access to another workspace. This is anonymous demo access, not an authenticated financial application. Use fictional data.

## Durable storage

For this small demo, a workspace is a bounded SQLite snapshot in a private Cloud Storage bucket. Each request restores its workspace into a private temporary directory and uses the existing tested transactional store. A successful mutation closes/checkpoints SQLite and conditionally publishes the snapshot against the object generation it read. Return success only after publication. Generation conflicts reject the losing write, preventing lost updates across instances. Operation IDs make retries after uncertain responses safe.

This deliberately trades throughput for a small, consistent implementation: each request reads the workspace snapshot, and each mutation writes it. Maximum snapshot size is 32 MiB, with at most 20 saved runs per workspace. It is suitable for a low-traffic demonstration, not a production reconciliation service. A production system would use a shared transactional database and authenticated ownership.

A seven-day object lifecycle removes old demo data; the UI explains retention and browser-cookie ownership. Local databases never upload during deployment. Cloud Run uses request-based billing, zero minimum instances, a small instance cap and a dedicated service account limited to the demo bucket. Cloud trial/free allocations are not a guaranteed zero bill.

## Implementation and checks

1. Repair the CI test timing race and add the requested diagrams.
2. Add a repository interface and a conditional snapshot adapter. Test persistence across adapter recreation, workspace isolation, failed publication and concurrency conflicts.
3. Add secure cookie middleware, hosted metadata, request origin checks and bounded snapshot/run limits.
4. Add container and reproducible cloud setup/deploy scripts, excluding local data and credentials.
5. Run Go, browser and CI checks. Deploy the reviewed revision and verify the real HTTPS workflow from two independent browser contexts and after a fresh revision/restart.
