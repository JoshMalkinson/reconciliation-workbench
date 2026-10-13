# Explaining Reconcile in an interview

## Opening explanation

“I built a small investment-data reconciliation tool. It compares an internal holdings extract with an administrator statement. The useful part is separating identity matching, numerical differences and human review. A reviewer can approve a mapping, rerun the comparison and explain an exception without rewriting the original evidence.”

Use this as a starting point in your own words. Be clear about which parts you implemented with Codex and which behavior you verified and can explain yourself.

## Connect the stack to what you know

- **Vue and TypeScript:** `ref` holds reactive state and `computed` derives it. These serve similar purposes to React state/derived values and Angular signals/computed values. The template declares the visible workflow.
- **Go:** the domain structs are the data model, while methods on `Store` encapsulate persistence. Small packages separate responsibilities you might otherwise express with C# or Java service classes.
- **Protobuf:** a shared typed service definition generates both client and server contracts. Compare it with an interface plus DTOs, but with a defined binary wire format.
- **gRPC-Web:** the browser calls the generated client through a browser-compatible transport. Connect Go handles that protocol as well as native gRPC; the backend tests exercise both over real HTTP.
- **SQLite:** transactions publish a whole comparison at once. The storage layer can later be adapted for hosted persistence without moving matching logic into the UI.

## Decisions you should be able to defend

**Why no name matching?** Two instruments can have similar names while representing different classes. Identity needs a shared ID or an explicit decision. The app preserves that decision and its explanation.

**Why integers?** Binary floating-point does not represent every decimal exactly. Units use six-place scaled integers and money uses cents; comparison is exact, and TypeScript uses bigint to format them without losing precision.

**Why immutable runs?** If mappings change, a historical report should still explain what was known at that time. A rerun creates a new snapshot, with a parent-run ID and source SHA-256 fingerprints.

**What happens if a request is retried?** The operation ID and payload fingerprint are saved in the same transaction as the result. The same request gets the saved result; the same ID with a changed payload is rejected.

**What about two tabs?** Each review has a revision. A stale tab cannot overwrite a newer review; it must reload the run first.

**Does Reviewed mean the numbers reconcile?** No. It means a person recorded a decision. The units/value flags remain visible, and reopening appends a new event.

## Code tour

1. `proto/reconcile/v1/workbench.proto`: show the shared contract.
2. `internal/domain/domain.go`: explain parsing, scaling and match precedence.
3. `internal/store/store.go`: show the mutation transaction and revision check.
4. `web/src/App.vue`: follow the mapping-and-rerun interaction.
5. `internal/store/store_test.go` and `web/tests/workbench.spec.ts`: demonstrate evidence for persistence and the user journey.

## Next engineering step

The local app binds to loopback. Hosted mode uses Cloud Run and private Cloud Storage snapshots, with separate cookie-based visitor workspaces. Explain the throughput tradeoff of reading and conditionally publishing a whole SQLite snapshot; for a production system, use authenticated users and a transactional hosted database. Cloud Run's temporary filesystem is never the durable store.
