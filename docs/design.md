# Reconcile — Investment Data Workbench

Design for review · 12 October 2026

## Purpose

Build a separate interview project around investment-data integration and reconciliation. The user has professional Angular, React, C# and Java experience and wants a project that is visibly different from a friend's trade-compliance demo. Success is a short, explainable demonstration of conflicting provider data, deterministic matching, human review and persistent history.

The existing Ledger project remains intact. This project uses Go, Vue 3, TypeScript, Protobuf and gRPC-Web, with SQLite for local persistence. Initial delivery runs locally on Windows without Docker or a cloud account. The earlier few-day target is treated as a reason to keep scope small, not as a confirmed new deadline.

## Demonstration

Two fictional providers describe the same portfolio on the same date. One uses internal instrument identifiers and names; the other uses its own codes and column names. Import both files, compare them, inspect discrepancies, approve one instrument mapping and rerun. Record a review note on a remaining difference, restart the app and show that the runs, mapping and review history remain available.

This project answers: why do these records disagree, and what did the reviewer decide? It does not evaluate concentration limits or simulate purchases.

## Three focused views

1. **Import and compare.** Pick an internal holdings file and an administrator file. Downloadable samples cover agreement, inconsistent identifiers, quantity differences and missing holdings. Validation shows file, physical row and field. Invalid pairs create no reconciliation run.
2. **Exception workbench.** Summary counts and filters show matched records, quantity differences, value differences, both differences, internal-only and administrator-only records. Selecting an item opens a side-by-side record comparison and its review history. Display the source values, signed differences and matching basis.
3. **Run history and mappings.** Open earlier runs, inspect their source fingerprints and export a report. View approved mappings. When investigating two unmatched records, explicitly pair the administrator record with an internal instrument and save the mapping with a required explanation. Rerun to apply it; the earlier run retains its original results.

An editorial workbench layout uses a compact run list, central exception table and detail panel. Text labels distinguish matching from reviewer decisions. Responsive layouts stack these panels on narrow screens. There are no inert navigation items or invented live metrics.

## Input contract

Two fixed fictional providers, one portfolio per run and ZAR only. Fixed adapters demonstrate different schemas without introducing a general field-mapping builder.

**Internal file:**

```csv
instrument_id,instrument_name,as_of,currency,units,market_value
FUND-A,Example Equity Fund Class A,2026-10-01,ZAR,1000,120000.00
```

**Administrator file:**

```csv
provider_code,security_id,description,valuation_date,ccy,quantity,value
EXT-001,,EX Equity A,2026-10-01,ZAR,980,117600.00
```

`security_id`, when populated, uses the same identifier scheme as internal `instrument_id`. It is not the administrator's own `provider_code`. Blank security IDs require an approved mapping to be paired; names are context for the reviewer, never proof of identity.

Both files must be UTF-8 CSV, optionally with a BOM, with exact headers and no extra fields. Ignore empty lines; support quoted and multiline fields. Each file is limited to 1 MiB and 1,000 rows. Require a nonempty name/description, a valid YYYY-MM-DD date and exactly ZAR. Every row in both files must have the same date. Reject duplicates after trimming and uppercasing instrument IDs/provider codes, and duplicate nonempty security IDs within the administrator file.

Units are positive plain decimals with up to six fractional digits and a maximum of 1,000,000,000. Market value is a positive plain decimal with at most two fractional digits and a maximum of 1,000,000,000.00. Reject symbols, grouping separators, exponents and excessive precision. Go stores units as scaled int64 integers and money as integer cents; the frontend uses generated bigint values for these fields. No FX conversion or value inference from price is performed.

## Matching and comparison

1. Pair identical nonempty shared instrument IDs.
2. For administrator rows without a shared ID, use an approved mapping from that provider's code to an internal instrument ID.
3. Leave the rest unmatched. A nonempty shared ID cannot be overridden by a mapping, and names never trigger an automatic match.

Every pair is one-to-one. If mappings would assign two administrator records to one internal holding, fail the run with a clear mapping-conflict error rather than selecting a record silently. A mapping whose target is absent from this file leaves the administrator holding unmatched.

For matched pairs, compare units and market value independently with exact equality; there is no configurable tolerance in version one. Signed deltas are administrator minus internal. An item can have a units difference, a value difference, both, or neither. Missing records show missing values rather than invented zero balances. Preserve original row numbers and values for both sides.

The example above remains unmatched until the reviewer approves EXT-001 to FUND-A. A rerun then shows a units delta of -20 and a value delta of -ZAR 2,400.00. Approving identity does not make unequal quantities agree.

## Review and history

Runs and their source snapshots, comparison results and mapping snapshots are immutable. Mapping changes affect future runs only; rerunning creates a new run linked to the earlier one. Repeated imports may create distinct runs, each clearly timestamped; the application does not claim that an upload has been deduplicated.

Discrepancies have a separate review state: Open or Reviewed. Marking Reviewed requires a reviewer display name, a reason category (timing difference, source correction requested, or other) and a note. Reopening also requires a note. Every transition appends an event; it never edits source holdings or removes difference flags. Display names are self-entered demo metadata, not authenticated identity.

Approved mappings have one active target per administrator code. The first version permits creating mappings from currently unmatched records and viewing them; editing/deleting existing mappings is deferred. Require an explanation and record the reviewer display name and timestamp. Prevent an existing mapping from being silently replaced.

Use SQLite transactions so run creation and its result rows either all persist or none do. Review changes use an expected revision to reject stale updates from another browser tab. Mutation requests carry an operation ID: a retry with the same ID and payload returns the original result; reuse with a different payload is rejected. Controls disable duplicate submissions and report failures without implying success.

## Architecture and storage

Vue calls one Go service using generated Protobuf contracts and an explicitly selected gRPC-Web transport. Connect Go serves the browser protocol and native gRPC. The production Go process serves the frontend as well; the Vite development server proxies RPC requests. Use a separate local port, 8081, so Ledger can continue running on 8080.

Keep CSV adapters, matching/comparison rules, storage and transport separate. Domain rules do not import Protobuf or SQL. SQLite stores source snapshots, runs, result items, mappings, mutation IDs and append-only review events. Source fingerprints use SHA-256. The database is stored in an ignored local data directory and is never included in GitHub. This is an application audit trail, not cryptographic tamper-proof storage.

RPC capabilities cover creating/listing/getting runs, listing/creating mappings, marking/reopening reviews and exporting a run. Validation failures have structured locations; unexpected server failures do not expose internal paths or SQL. Apply transport size limits large enough for two allowed files plus framing. Do not log raw file contents.

## Export

Download a CSV report containing each item, source IDs/names/row numbers, date, original values, deltas, matching basis, discrepancy classification, current review state and latest review note. Include run ID, source fingerprints and export time. Export includes unresolved items as well as reviewed ones and does not label reviewed discrepancies as reconciled. Protect spreadsheet-facing text fields against formula interpretation; serialize numeric values as numeric decimal text. The UI provides the full event history even though the export includes only the latest review note.

## Verification and acceptance

- A full demo shows exact-ID matches, an approved mapping, an unresolved holding, units/value differences, a recorded review and an export.
- Domain tests cover exact scaled arithmetic, maximum values, identical data, mismatches in either direction, missing records, case-normalised duplicates, malformed inputs, date disagreement and ambiguous mappings.
- Persistence tests reopen the database and recover runs, mappings and review events. Failed imports leave no partial run. Mutation retries and stale revisions are tested.
- RPC tests use real gRPC-Web and native gRPC calls.
- Browser tests exercise two uploads, side-by-side investigation, mapping plus rerun, reviewed/reopened transitions, historical run retrieval and export.
- UI tests cover loading, errors, cancellation/stale responses, keyboard access and literal rendering of imported text. Inspect desktop and mobile layouts.
- Repository deliverables include a README with a system flowchart, reproducible Windows setup, fictional sample files, tests and an interview walkthrough explaining the choices in terms of the user's existing experience.

## Delivery order and boundaries

Build one vertical slice first: two-file import, deterministic matching and differences. Add persistence and history, then mapping/review actions, then export and presentation polish. This is one application with one workflow; no microservice split is needed.

AI suggestions, live provider integrations, authentication, multiple portfolios per file, configurable tolerances and collaborative roles are outside the initial local build. The user has requested GitHub hosting for this separate project; the existing Ledger repository is not repurposed.

## GitHub and live-demo delivery

The source and README will be prepared for a separate GitHub repository. GitHub Pages can serve the compiled Vue frontend, but cannot run the Go service or SQLite database. A fully functional public demo therefore also needs a backend host with suitable persistent storage. Hosting source code alone does not make localhost accessible to employers.

The hosting interpretation is pending user clarification: source repository only, or source repository plus a live full-stack demo. If a live demo is required, revise storage and access requirements for the selected host before implementation. In particular, do not assume that a local SQLite file survives restarts on an ephemeral container host. No hosting account, paid resource or public access configuration has been selected or created.
