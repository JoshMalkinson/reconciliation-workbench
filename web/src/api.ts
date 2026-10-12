import { createClient } from "@connectrpc/connect";
import { createGrpcWebTransport } from "@connectrpc/connect-web";
import { WorkbenchService } from "./gen/reconcile/v1/workbench_pb";
export const api = createClient(
  WorkbenchService,
  createGrpcWebTransport({ baseUrl: window.location.origin }),
);
export function decimal(n: bigint, places: number, signed = false): string {
  const scale = 10n ** BigInt(places),
    absolute = n < 0n ? -n : n;
  const whole = (absolute / scale).toLocaleString("en-ZA"),
    fraction = (absolute % scale).toString().padStart(places, "0");
  const digits = places === 6 ? fraction.replace(/0+$/, "") : fraction;
  return `${n < 0n ? "−" : signed && n > 0n ? "+" : ""}${whole}${digits ? "." + digits : ""}`;
}
export const labels: Record<string, string> = {
  matched: "Matched",
  units_difference: "Units difference",
  value_difference: "Value difference",
  both_difference: "Units & value",
  internal_only: "Internal only",
  administrator_only: "Administrator only",
  shared_id: "Shared ID",
  approved_mapping: "Approved mapping",
  unmatched: "Unmatched",
  timing_difference: "Timing difference",
  source_correction_requested: "Source correction requested",
  other: "Other",
};
