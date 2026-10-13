<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from "vue";
import { ConnectError } from "@connectrpc/connect";
import type { Run, Mapping, Holding } from "./gen/reconcile/v1/workbench_pb";
import { api, decimal, labels } from "./api";

const runs = ref<Run[]>([]),
  run = ref<Run>(),
  mappings = ref<Mapping[]>([]);
const selected = ref(""),
  filter = ref("exceptions"),
  search = ref(""),
  view = ref("workbench");
const busy = ref(false),
  loading = ref(true),
  error = ref(""),
  notice = ref(""),
  importOpen = ref(false);
const internalFile = ref<File>(),
  adminFile = ref<File>(),
  reviewer = ref(""),
  note = ref(""),
  reason = ref("timing_difference"),
  target = ref("");
const item = computed(() =>
  run.value?.items.find((i) => i.id === selected.value),
);
const exceptions = computed(
  () => run.value?.items.filter((i) => i.kind !== "matched") ?? [],
);
const openCount = computed(
  () => exceptions.value.filter((i) => !i.reviewed).length,
);
const matchedCount = computed(
  () => run.value?.items.filter((i) => i.kind === "matched").length ?? 0,
);
const unmatched = computed(
  () => run.value?.items.filter((i) => i.kind === "internal_only") ?? [],
);
const filtered = computed(
  () =>
    run.value?.items.filter((i) => {
      const matches =
        filter.value === "all" ||
        (filter.value === "exceptions"
          ? i.kind !== "matched"
          : filter.value === "open"
            ? i.kind !== "matched" && !i.reviewed
            : i.kind === filter.value);
      return (
        matches &&
        [
          i.internal?.id,
          i.internal?.name,
          i.administrator?.id,
          i.administrator?.name,
        ]
          .join(" ")
          .toLowerCase()
          .includes(search.value.toLowerCase())
      );
    }) ?? [],
);
const hasMapping = computed(() =>
  mappings.value.some((m) => m.providerCode === item.value?.administrator?.id),
);
const operationIDs = new Map<string, string>();
const hosted = ref(false);
onMounted(async () => {
  try {
    const config = await fetch("/api/config");
    if (config.ok) hosted.value = (await config.json()).hosted === true;
  } catch {
    /* The main RPC loading path reports connection failures. */
  }
});
const investigationAction = ref("map");
let previousFocus: HTMLElement | null = null;
watch(importOpen, async (open) => {
  if (open) {
    internalFile.value = undefined;
    adminFile.value = undefined;
    previousFocus = document.activeElement as HTMLElement;
    await nextTick();
    document
      .querySelector<HTMLButtonElement>('[aria-label="Close import"]')
      ?.focus();
  } else {
    await nextTick();
    previousFocus?.focus();
  }
});
function trapFocus(e: KeyboardEvent) {
  if (e.key !== "Tab") return;
  const controls = Array.from(
    (e.currentTarget as HTMLElement).querySelectorAll<HTMLElement>(
      "button:not(:disabled), input:not(:disabled), a[href], select:not(:disabled), textarea:not(:disabled)",
    ),
  );
  const first = controls[0],
    last = controls.at(-1);
  if (e.shiftKey && document.activeElement === first) {
    e.preventDefault();
    last?.focus();
  } else if (!e.shiftKey && document.activeElement === last) {
    e.preventDefault();
    first?.focus();
  }
}
let loadVersion = 0;
function op(payload: unknown) {
  const key = JSON.stringify(payload);
  let id = operationIDs.get(key);
  if (!id) {
    id = crypto.randomUUID();
    operationIDs.set(key, id);
  }
  return id;
}
function time(s: string) {
  return new Date(s).toLocaleString("en-ZA", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
function choose(id: string) {
  selected.value = id;
  note.value = "";
  target.value = "";
  reason.value = "timing_difference";
  investigationAction.value = "map";
}
function showRun(value: Run) {
  run.value = value;
  choose(
    value.items.find((i) => i.kind !== "matched")?.id ??
      value.items[0]?.id ??
      "",
  );
  filter.value = "exceptions";
  search.value = "";
  view.value = "workbench";
}
async function refresh() {
  const [a, b] = await Promise.all([api.listRuns({}), api.listMappings({})]);
  runs.value = a.runs;
  mappings.value = b.mappings;
}
async function action(fn: () => Promise<void>) {
  if (busy.value) return;
  busy.value = true;
  error.value = "";
  notice.value = "";
  loadVersion++;
  try {
    await fn();
  } catch (e) {
    error.value =
      e instanceof ConnectError
        ? e.rawMessage
        : e instanceof Error
          ? e.message
          : "Request failed. Please retry.";
  } finally {
    busy.value = false;
  }
}
async function openRun(id: string) {
  await action(async () => {
    showRun(await api.getRun({ id }));
  });
}
function fileChanged(event: Event, admin: boolean) {
  const file = (event.target as HTMLInputElement).files?.[0];
  if (admin) adminFile.value = file;
  else internalFile.value = file;
  error.value = "";
}
async function compare() {
  await action(async () => {
    if (!internalFile.value || !adminFile.value)
      throw new Error("Choose both CSV files to compare.");
    if (internalFile.value.size > 1048576 || adminFile.value.size > 1048576)
      throw new Error("Each file must be 1 MiB or smaller.");
    const a = new Uint8Array(await internalFile.value.arrayBuffer()),
      b = new Uint8Array(await adminFile.value.arrayBuffer());
    const key = [
      "import",
      internalFile.value.name,
      adminFile.value.name,
      await hash(a),
      await hash(b),
    ];
    const result = await api.createRun({
      operationId: op(key),
      internal: a,
      administrator: b,
      internalName: internalFile.value.name,
      administratorName: adminFile.value.name,
    });
    showRun(result);
    importOpen.value = false;
    operationIDs.delete(JSON.stringify(key));
    notice.value = "Comparison saved. Select a record to investigate.";
    await refresh();
  });
}
async function hash(bytes: Uint8Array) {
  return Array.from(
    new Uint8Array(
      await crypto.subtle.digest("SHA-256", new Uint8Array(bytes)),
    ),
  )
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}
async function demo() {
  if (busy.value) return;
  error.value = "";
  busy.value = true;
  try {
    const responses = await Promise.all([
      fetch("/samples/internal.csv"),
      fetch("/samples/administrator.csv"),
    ]);
    if (responses.some((r) => !r.ok))
      throw new Error("Sample files could not be loaded.");
    internalFile.value = new File([await responses[0]!.blob()], "internal.csv");
    adminFile.value = new File(
      [await responses[1]!.blob()],
      "administrator.csv",
    );
  } catch (e) {
    error.value = String(e);
    return;
  } finally {
    busy.value = false;
  }
  await compare();
}
async function rerun() {
  await action(async () => {
    if (!run.value) return;
    const key = ["rerun", run.value.id];
    const result = await api.createRun({
      operationId: op(key),
      parentId: run.value.id,
    });
    showRun(result);
    operationIDs.delete(JSON.stringify(key));
    notice.value = "New run saved with the latest approved mappings.";
    await refresh();
  });
}
async function approve() {
  await action(async () => {
    if (!run.value || !item.value?.administrator) return;
    const payload = {
      runId: run.value.id,
      providerCode: item.value.administrator.id,
      internalId: target.value,
      reviewer: reviewer.value,
      note: note.value,
    };
    await api.approveMapping({ ...payload, operationId: op(["map", payload]) });
    await refresh();
    note.value = "";
    notice.value =
      "Mapping approved. Rerun comparison to apply it; this run is unchanged.";
  });
}
async function review() {
  await action(async () => {
    if (!run.value || !item.value) return;
    const payload = {
      runId: run.value.id,
      itemId: item.value.id,
      reviewer: reviewer.value,
      reason: reason.value,
      note: note.value,
      reviewed: !item.value.reviewed,
      expectedRevision: item.value.revision,
    };
    const updated = await api.reviewItem({
      ...payload,
      operationId: op(["review", payload]),
    });
    const index = run.value.items.findIndex((i) => i.id === updated.id);
    run.value.items[index] = updated;
    note.value = "";
    notice.value = updated.reviewed
      ? "Review recorded. The source discrepancy remains visible."
      : "Item reopened. Your note was added to its history.";
  });
}
async function download() {
  await action(async () => {
    if (!run.value) return;
    const result = await api.exportRun({ id: run.value.id });
    const url = URL.createObjectURL(
      new Blob([new Uint8Array(result.csv)], {
        type: "text/csv;charset=utf-8",
      }),
    );
    const a = document.createElement("a");
    a.href = url;
    a.download = result.filename;
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
    notice.value = "CSV report exported.";
  });
}
function holding(side: "internal" | "administrator"): Holding | undefined {
  return item.value?.[side];
}
onMounted(async () => {
  const version = ++loadVersion;
  try {
    const [a, b] = await Promise.all([api.listRuns({}), api.listMappings({})]);
    if (version !== loadVersion) return;
    runs.value = a.runs;
    mappings.value = b.mappings;
    if (a.runs[0]) {
      const latest = await api.getRun({ id: a.runs[0].id });
      if (version === loadVersion) showRun(latest);
    }
  } catch (e) {
    if (version === loadVersion)
      error.value =
        "Could not load saved runs. Check your connection and reload the page.";
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <div class="shell">
    <aside
      class="sidebar"
      :inert="importOpen"
      :aria-hidden="importOpen || undefined"
    >
      <a class="brand" href="/" aria-label="Reconcile home"
        ><span class="brand-icon">r<span>↔</span></span
        ><span
          >reconcile<span class="brand-sub"
            >INVESTMENT DATA WORKBENCH</span
          ></span
        ></a
      >
      <div class="workspace-label">
        <span class="avatar">J</span>
        <div>
          {{ hosted ? "Your demo workspace" : "Local workspace"
          }}<small>Investment operations</small>
        </div>
        <span class="dot"></span>
      </div>
      <nav aria-label="Workspace">
        <button
          :class="{ active: view === 'workbench' }"
          @click="view = 'workbench'"
        >
          ▦ <span>Reconciliation</span></button
        ><button
          :class="{ active: view === 'mappings' }"
          @click="view = 'mappings'"
        >
          ⇄ <span>Instrument mappings</span
          ><span class="nav-count">{{ mappings.length }}</span>
        </button>
      </nav>
      <div class="sidebar-heading">
        RUN HISTORY <span>{{ runs.length }}</span>
      </div>
      <p v-if="!runs.length" class="history-empty">
        Your saved comparisons will appear here.
      </p>
      <div class="history-list">
        <button
          v-for="(r, index) in runs"
          :key="r.id"
          :disabled="busy"
          :class="['history-item', { selected: run?.id === r.id }]"
          :aria-label="`Open run ${r.id}`"
          @click="openRun(r.id)"
        >
          <span class="history-dot"></span
          ><span
            ><strong>{{ r.date }}</strong
            ><small
              >{{ time(r.at) }} · {{ r.parentId ? "Rerun" : "Import" }}</small
            ></span
          ><span v-if="index === 0" class="latest">Latest</span>
        </button>
      </div>
      <div class="sidebar-bottom">
        <span class="dot"></span>
        {{ hosted ? "Saved in your demo workspace" : "Stored on this computer"
        }}<small>Runs, mappings and review notes</small>
      </div>
    </aside>
    <main :inert="importOpen" :aria-hidden="importOpen || undefined">
      <header class="topbar">
        <div>
          Workspace <span>/</span>
          {{ view === "mappings" ? "Instrument mappings" : "Reconciliation" }}
        </div>
        <div class="demo-tag">
          {{
            hosted ? "PUBLIC DEMO · PRIVATE WORKSPACE" : "LOCAL DEMO WORKSPACE"
          }}
        </div>
      </header>
      <div class="content">
        <div class="page-heading">
          <div>
            <div class="eyebrow">INVESTMENT OPERATIONS</div>
            <h1>
              {{
                view === "mappings"
                  ? "Instrument mappings"
                  : "Portfolio reconciliation"
              }}
            </h1>
            <p>
              {{
                view === "mappings"
                  ? "Explicit identity decisions, with a record of who approved them."
                  : "Two sources. One clear picture of what needs attention."
              }}
            </p>
          </div>
          <button
            class="primary"
            aria-label="New import"
            :disabled="busy || loading"
            @click="
              importOpen = true;
              error = '';
            "
          >
            ＋ New import
          </button>
        </div>
        <p v-if="hosted" class="message">
          Your browser has its own demo workspace. Use fictional data only.
          Sessions last 7 days, with up to 20 runs; clearing browser cookies
          starts a new workspace. Export anything you want to keep.
        </p>
        <div v-if="error && !importOpen" role="alert" class="message error">
          {{ error }}
          <button v-if="run" :disabled="busy" @click="openRun(run.id)">
            Reload run
          </button>
        </div>
        <div v-if="notice" role="status" class="message success">
          {{ notice }}
        </div>
        <div v-if="loading" class="empty">Loading saved work…</div>
        <template v-else-if="view === 'mappings'">
          <section class="card mappings">
            <div class="section-heading">
              <h2>Approved mappings</h2>
              <span class="muted"
                >{{ mappings.length }} identity decisions</span
              >
            </div>
            <p class="muted">
              Mappings apply to future comparisons. Shared identifiers take
              priority; names never trigger an automatic match.
            </p>
            <p v-if="!mappings.length" class="empty">
              Open an unmatched administrator record in a comparison to approve
              its internal instrument.
            </p>
            <article
              v-for="m in mappings"
              :key="m.providerCode"
              class="mapping"
            >
              <div>
                <code>{{ m.providerCode }}</code
                ><span class="arrow">→</span><code>{{ m.internalId }}</code>
              </div>
              <p>{{ m.note }}</p>
              <small>{{ m.reviewer }} · {{ time(m.at) }}</small
              ><button :disabled="busy" @click="openRun(m.runId)">
                Open source run
              </button>
            </article>
          </section>
        </template>
        <template v-else-if="!run">
          <section class="welcome card">
            <div class="welcome-art">
              <span
                >01<br /><b>Internal<br />holdings</b></span
              ><i>⇄</i
              ><span
                >02<br /><b>Administrator<br />statement</b></span
              >
            </div>
            <div class="eyebrow">START WITH THE DIFFERENCES</div>
            <h2>A clearer close starts here.</h2>
            <p>
              Compare holdings from two providers, investigate exceptions, and
              keep a traceable record of every review.
            </p>
            <button class="primary" :disabled="busy" @click="demo">
              {{ busy ? "Comparing…" : "Try the sample portfolio" }}</button
            ><small>Six holdings per source · ZAR · 1 October 2026</small>
            <div class="welcome-steps">
              <span><b>01</b> Import two files</span
              ><span><b>02</b> Investigate differences</span
              ><span><b>03</b> Review & export</span>
            </div>
          </section>
        </template>
        <template v-else>
          <section class="run-bar">
            <div>
              <span class="run-icon">▤</span>
              <div>
                <strong>Holdings · {{ run.date }}</strong
                ><small
                  >Run {{ run.id.slice(0, 8) }} · {{ time(run.at) }}
                  <span v-if="run.parentId"
                    >· Rerun of {{ run.parentId.slice(0, 8) }}</span
                  ></small
                >
              </div>
            </div>
            <div class="actions">
              <button
                aria-label="Rerun comparison"
                :disabled="busy"
                @click="rerun"
              >
                ↻ Rerun comparison</button
              ><button
                aria-label="Export CSV"
                :disabled="busy"
                @click="download"
              >
                ↓ Export CSV
              </button>
            </div>
          </section>
          <section class="stats" aria-label="Comparison summary">
            <div>
              <span>Records compared</span
              ><strong>{{ run.items.length }}</strong
              ><small>Matched pairs + unpaired holdings</small>
            </div>
            <div>
              <span>Matched</span
              ><strong class="green">{{ matchedCount }}<em>✓</em></strong
              ><small>Units and value agree</small>
            </div>
            <div>
              <span>Open exceptions</span
              ><strong class="orange">{{ openCount }}<em>!</em></strong
              ><small>Awaiting a recorded review</small>
            </div>
            <div>
              <span>Reviewed</span
              ><strong>{{ exceptions.length - openCount }}</strong
              ><small>Decision recorded, difference retained</small>
            </div>
          </section>
          <div class="workbench">
            <section class="card results">
              <div class="section-heading">
                <div>
                  <h2>
                    Comparison results
                    <span class="count">{{ filtered.length }}</span>
                  </h2>
                  <p>Administrator minus internal · exact comparison</p>
                </div>
                <span class="live-label">SAVED SNAPSHOT</span>
              </div>
              <div class="filters">
                <label class="sr-only" for="filter">Filter records</label
                ><select id="filter" v-model="filter">
                  <option value="exceptions">All exceptions</option>
                  <option value="all">All records</option>
                  <option value="open">Open exceptions</option>
                  <option value="matched">Matched</option>
                  <option value="internal_only">Internal only</option>
                  <option value="administrator_only">Administrator only</option>
                  <option value="units_difference">Units difference</option>
                  <option value="value_difference">Value difference</option>
                  <option value="both_difference">Units & value</option></select
                ><input
                  v-model="search"
                  type="search"
                  placeholder="Search instrument or code…"
                  aria-label="Search records"
                />
              </div>
              <div class="table-scroll">
                <table>
                  <thead>
                    <tr>
                      <th>Instrument</th>
                      <th>Comparison</th>
                      <th>Review</th>
                      <th><span class="sr-only">Inspect</span></th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr
                      v-for="i in filtered"
                      :key="i.id"
                      :class="{ selected: i.id === selected }"
                    >
                      <td>
                        <strong>{{
                          i.internal?.id ?? i.administrator?.id
                        }}</strong
                        ><small>{{
                          i.internal?.name ?? i.administrator?.name
                        }}</small>
                      </td>
                      <td>
                        <span
                          :class="[
                            'badge',
                            i.kind === 'matched'
                              ? 'matched'
                              : i.kind.includes('only')
                                ? 'unmatched'
                                : 'difference',
                          ]"
                          >{{ labels[i.kind] }}</span
                        >
                      </td>
                      <td>
                        <span :class="['review-label', { done: i.reviewed }]">{{
                          i.kind === "matched"
                            ? "—"
                            : i.reviewed
                              ? "✓ Reviewed"
                              : "○ Open"
                        }}</span>
                      </td>
                      <td>
                        <button
                          class="inspect"
                          :disabled="busy"
                          :aria-label="`Inspect ${i.internal?.id ?? i.administrator?.id}`"
                          @click="choose(i.id)"
                        >
                          ↗
                        </button>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div v-if="!filtered.length" class="empty">
                No records match this filter.
              </div>
              <footer class="results-footer">
                <span>{{ run.items.length }} comparison records</span
                ><span>Currency: ZAR</span>
              </footer>
            </section>
            <section
              v-if="item"
              class="card detail"
              aria-label="Investigation panel"
            >
              <div class="detail-top">
                <div class="eyebrow">RECORD INVESTIGATION</div>
                <span
                  :class="[
                    'badge',
                    item.kind === 'matched' ? 'matched' : 'difference',
                  ]"
                  data-testid="classification"
                  >{{ labels[item.kind] }}</span
                >
              </div>
              <h2>{{ item.internal?.name ?? item.administrator?.name }}</h2>
              <p class="identity">
                {{ item.internal?.id ?? "No internal ID" }} <span>↔</span>
                {{ item.administrator?.id ?? "No administrator record" }}
              </p>
              <div class="basis">
                {{ labels[item.basis] }}
                <span
                  v-if="item.kind !== 'matched'"
                  data-testid="review-state"
                  >{{ item.reviewed ? "Reviewed" : "Open" }}</span
                >
              </div>
              <div class="source-grid">
                <div
                  v-for="side in ['internal', 'administrator'] as const"
                  :key="side"
                >
                  <h3>
                    {{ side === "internal" ? "INTERNAL" : "ADMINISTRATOR" }}
                  </h3>
                  <template v-if="holding(side)"
                    ><dl>
                      <dt>Units</dt>
                      <dd>{{ decimal(holding(side)!.units, 6) }}</dd>
                      <dt>Market value · ZAR</dt>
                      <dd>{{ decimal(holding(side)!.value, 2) }}</dd>
                      <dt>Source row</dt>
                      <dd>{{ holding(side)!.row }}</dd>
                    </dl></template
                  >
                  <p v-else class="missing">No matching<br />holding</p>
                </div>
              </div>
              <div v-if="item.internal && item.administrator" class="deltas">
                <div>
                  <small>Units delta</small
                  ><strong data-testid="units-delta">{{
                    decimal(item.unitsDelta, 6, true)
                  }}</strong>
                </div>
                <div>
                  <small>Value delta · ZAR</small
                  ><strong data-testid="value-delta">{{
                    decimal(item.valueDelta, 2, true)
                  }}</strong>
                </div>
              </div>
              <details class="originals">
                <summary>View original source fields</summary>
                <div
                  v-for="side in ['internal', 'administrator'] as const"
                  :key="side"
                >
                  <strong>{{ side }}</strong>
                  <pre>{{ holding(side)?.original ?? "No record" }}</pre>
                </div>
              </details>
              <div
                v-if="
                  item.kind === 'administrator_only' &&
                  !item.administrator?.sharedId
                "
                class="investigation-tabs"
              >
                <button
                  :disabled="busy"
                  :aria-pressed="investigationAction === 'map'"
                  @click="
                    investigationAction = 'map';
                    note = '';
                  "
                >
                  Map identity</button
                ><button
                  :disabled="busy"
                  :aria-pressed="investigationAction === 'review'"
                  @click="
                    investigationAction = 'review';
                    note = '';
                  "
                >
                  Review exception
                </button>
              </div>
              <template
                v-if="
                  item.kind === 'administrator_only' &&
                  !item.administrator?.sharedId &&
                  investigationAction === 'map'
                "
              >
                <div class="form-section" v-if="hasMapping">
                  <h3>Identity mapping approved</h3>
                  <p>
                    Rerun comparison to apply the mapping. This saved result
                    stays unchanged.
                  </p>
                </div>
                <form v-else class="form-section" @submit.prevent="approve">
                  <h3>Map this instrument</h3>
                  <p>
                    Confirm its identity against an unmatched internal holding.
                  </p>
                  <label
                    >Internal instrument<select
                      v-model="target"
                      required
                      :disabled="busy"
                    >
                      <option value="" disabled>
                        Select internal instrument
                      </option>
                      <option
                        v-for="i in unmatched"
                        :value="i.internal!.id"
                        :key="i.id"
                      >
                        {{ i.internal!.id }} · {{ i.internal!.name }}
                      </option>
                    </select></label
                  ><label
                    >Reviewer name<input
                      v-model="reviewer"
                      required
                      maxlength="100"
                      :disabled="busy"
                      placeholder="Your name" /></label
                  ><label
                    >Mapping explanation<textarea
                      v-model="note"
                      required
                      maxlength="2000"
                      :disabled="busy"
                      placeholder="What confirms these are the same instrument?"
                    ></textarea></label
                  ><button class="primary full" :disabled="busy || !target">
                    Approve mapping
                  </button>
                </form>
              </template>
              <form
                v-else-if="item.kind !== 'matched'"
                class="form-section"
                @submit.prevent="review"
              >
                <h3>
                  {{ item.reviewed ? "Reopen this item" : "Record a review" }}
                </h3>
                <p>
                  A review records your decision; it keeps the source difference
                  visible.
                </p>
                <label
                  >Reviewer name<input
                    v-model="reviewer"
                    required
                    maxlength="100"
                    :disabled="busy"
                    placeholder="Your name" /></label
                ><label
                  >Reason<select v-model="reason" :disabled="busy">
                    <option value="timing_difference">Timing difference</option>
                    <option value="source_correction_requested">
                      Source correction requested
                    </option>
                    <option value="other">Other</option>
                  </select></label
                ><label
                  >Review note<textarea
                    v-model="note"
                    required
                    maxlength="2000"
                    :disabled="busy"
                    placeholder="Explain what you checked and the next step."
                  ></textarea></label
                ><button class="primary full" :disabled="busy">
                  {{ item.reviewed ? "Reopen item" : "Mark reviewed" }}</button
                ><small
                  >Reviewer names are self-entered in this local demo.</small
                >
              </form>
              <div v-if="item.events.length" class="timeline">
                <h3>Review history</h3>
                <article
                  v-for="event in [...item.events].reverse()"
                  :key="event.revision"
                >
                  <strong
                    >{{ event.reviewed ? "Reviewed" : "Reopened" }} ·
                    {{ event.reviewer }}</strong
                  ><small
                    >{{ time(event.at) }} · {{ labels[event.reason] }}</small
                  >
                  <p>{{ event.note }}</p>
                </article>
              </div>
            </section>
          </div>
          <details class="provenance">
            <summary>Source fingerprints & run provenance</summary>
            <p>
              Original files and mapping decisions are saved with this run.
              Reviews are recorded separately.
            </p>
            <dl>
              <dt>{{ run.internalName }}</dt>
              <dd>{{ run.internalHash }}</dd>
              <dt>{{ run.administratorName }}</dt>
              <dd>{{ run.administratorHash }}</dd>
              <dt>Mappings at comparison time</dt>
              <dd>{{ run.mappings.length }} approved mappings</dd>
            </dl>
          </details>
        </template>
        <footer class="page-footer">
          <span>RECONCILE <span> / </span> An investment data workbench</span
          ><span>Deterministic matching. Traceable decisions.</span>
        </footer>
      </div>
    </main>
    <div
      v-if="importOpen"
      class="modal-backdrop"
      @click.self="!busy && (importOpen = false)"
    >
      <section
        class="modal card"
        role="dialog"
        aria-modal="true"
        aria-labelledby="import-title"
        @keydown="trapFocus"
        @keydown.esc="!busy && (importOpen = false)"
      >
        <div class="section-heading">
          <h2 id="import-title">New comparison</h2>
          <button
            aria-label="Close import"
            :disabled="busy"
            @click="importOpen = false"
          >
            ×
          </button>
        </div>
        <p>
          Upload both sources for one valuation date. ZAR only; up to 1,000
          holdings and 1 MiB per file.
        </p>
        <div v-if="error" role="alert" class="message error">{{ error }}</div>
        <form @submit.prevent="compare">
          <label
            >Internal holdings CSV<input
              type="file"
              accept=".csv,text/csv"
              :disabled="busy"
              @change="fileChanged($event, false)" /></label
          ><a href="/samples/internal.csv" download
            >Download internal sample ↗</a
          ><label
            >Administrator CSV<input
              type="file"
              accept=".csv,text/csv"
              :disabled="busy"
              @change="fileChanged($event, true)" /></label
          ><a href="/samples/administrator.csv" download
            >Download administrator sample ↗</a
          >
          <a href="/reconcile-test-files.zip" download
            >Download test pack: matching, differences and invalid data ↗</a
          >
          <div class="import-note">
            Identifiers are matched exactly or through an approved mapping.
            Similar names alone never create a match.
          </div>
          <button
            class="primary full"
            :disabled="busy || !internalFile || !adminFile"
          >
            {{ busy ? "Comparing…" : "Compare files" }}
          </button>
        </form>
        <button class="sample-link" :disabled="busy" @click="demo">
          Try the sample portfolio
        </button>
      </section>
    </div>
  </div>
</template>
