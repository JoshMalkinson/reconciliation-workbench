# Local implementation

Approved direction: build locally now; Google Cloud is deferred.

1. Test and implement strict CSV adapters, exact comparisons and one-to-one identity matching.
2. Test and implement transactional SQLite snapshots, mapping approvals, review events, retry protection and CSV export.
3. Generate Protobuf contracts and implement Go handlers; verify gRPC-Web and native gRPC.
4. Build the Vue workbench, samples, run history and investigation panel.
5. Exercise the complete browser workflow, inspect desktop/mobile, document setup and interview walkthrough, then independently review.

Interfaces: domain uses scaled int64 values; persistence owns atomic mutations; RPC converts domain records to generated messages; Vue uses bigint for exact formatting. Production serves frontend and API on localhost:8081. No cloud account is needed.
