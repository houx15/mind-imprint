# Evalbench v2

Evalbench is an offline, file-backed experiment runner for the current
`EvaluationReport` v1 generator. It never connects to a project database and
never writes production report rows.

## Contract

- Configuration uses `schemaVersion: 3` and the baseline evaluator ID
  `production-evalreport-v1`. Schema v1 targeted the retired DualAxis report
  and is rejected rather than silently reinterpreted.
- Persona exports are parsed through a whitelist. Historical assessment,
  mirror, summary and evaluation-report fields are not modelled and cannot
  become candidate input.
- The adapter creates anonymous, deterministic offline IDs for records and
  produces a frozen Evalbench `Input`: FACT fields, report digests and an
  evidence-candidate index. `generatedAt` and per-attempt report IDs are not
  included in the input hash.
- `production-evalreport-v1` locally replays the current production algorithm:
  rubric, prompt lens and risk subagents run in parallel; abstract runs after
  rubric. It deliberately does not share the API execution core.
  `single-prompt-evalreport-v1` is the sole offline single-call
  comparison variant: it uses the same frozen input, flagship model and report
  contract, with 24K output, reasoning, DeepSeek JSON mode, the Zod-derived
  model-output JSON Schema and a complete validated example. The prompt,
  schema, example and rubric are fingerprinted in the manifest; it is not a
  production code path.
- A report is valid only when Evalbench's internal strict validator accepts the
  `EvaluationReport` v1 shape. The single-prompt evaluator rejects missing,
  duplicate or unknown D/A IDs rather than filling them in.

## Attempts and observation

Every real provider stream is recorded with its logical purpose, model, token
usage, timing, request and raw output. A production candidate attempt has four
logical purposes: `eval_report_rubric`, `eval_report_promptlens`,
`eval_report_risks`, and `eval_report_abstract`. The single-prompt candidate
uses one purpose, `single_prompt_evalreport_v1`, and has no internal retry.
Retries are counted per purpose, not by subtracting one from total calls.

Every experiment snapshots the shared gateway USD cache-miss price table in
`manifest.json` under `pricing`, alongside the pricing-table version. Evalbench
does not maintain price entries in configuration. Cost is estimated from input
and output tokens (including provider-reported reasoning tokens) for every
attempted call, including failed streams and retries. Discounts, tax, cache-hit
pricing and currency conversion are intentionally out of scope.

`summary.json` keeps candidate and comparator cost separate. `knownCostUsd` is
the subtotal for calls that have complete usage and a known price; `costUsd` is
present only when every call in that total is priced. `costedCalls`,
`unpricedCalls`, and `usageMissing` are mutually exclusive and sum to `calls`.
An unpriced model or missing usage never becomes a fabricated zero. A zero-call
total is known to cost `$0.000000`.

Production degrades a single failed section to a conservative empty/floor
section. Evalbench mirrors that result locally and persists its diagnostics as
`partial`, but does not count it as a successful run. A successful run requires
all four sections, complete provider streams, strict report validation, and a
successful comparison.

## Gold and comparison

Gold is a user-provided JSON file with a required `report: EvaluationReportV1`.
The manual-editor provenance fields `status` and `_meta` are allowed but are
never evaluated; any other top-level field is rejected. Evalbench validates the
wrapper and the complete inner report contract before any model call, then
snapshots the exact source file as `gold-report.json`. The LLM comparator evaluates only
model-authored sections: D/A judgement, evidence and suggestion; abstract;
prompt lens; and risks. FACT sections (`basics`, `events`, `materials`,
`toolUsage`) are never judged by the model.
Every comparison item is `aligned`, `overstates`, `understates`, or
`not_comparable`; low confidence is forced to `not_comparable` plus manual
review. No student total score is produced.

## Human-readable report

EvalBench writes two deterministic Chinese Markdown artifacts after all JSON
artifacts have been persisted. Rendering performs no provider calls and does
not change comparison or success semantics. A write failure for either file is
a CLI error; the command must not report success without both deliverables.

`report.md` is the default, compact result summary for internal R&D and product
review. It contains experiment scope, overall and area-level Gold alignment,
error profile, performance, USD cost, reliability, sample limits, and links to
machine-readable artifacts. It never chooses a variant, makes a recommendation,
or emits an action plan. Alignment is explicitly scoped to the supplied Gold
and comparator, not presented as an absolute correctness rate. For multiple
cases it includes a compact case/variant coverage table.

`report-details.md` is the audit attachment. It contains every attempt and safe
failure classification, representative candidate reports, comparator
differences and the complete 46-item matrix, cross-run D/A and verdict
stability, hashes, execution order, and relative paths to raw artifacts.
Candidate suggestions or next steps are preserved only as original model output
and are labelled as such, not as EvalBench guidance. Canonical D/A labels come
from `internal/rubric`; neither renderer maintains a second label table.

Both reports present USD cost statistics: candidate spend, comparator spend,
combined experiment spend, and amortized candidate/total cost per complete
success. The former measures evaluator efficiency; the latter includes the
comparator and represents experimental budget. Per-attempt and per-purpose
breakdowns in the detailed report include failed and retried calls. Partial
cost coverage is rendered as a known subtotal plus coverage diagnostics rather
than a total.

For each case/variant, the representative run is the lowest-numbered attempt
whose candidate is complete, strict validation passes, and comparison succeeds.
If no such run exists, the earliest persisted candidate may be shown as an
explicit diagnostic preview that does not count toward success. With no
candidate report, only attempt diagnostics are shown. Other successful reports
are summarized through D/A distributions and comparator verdict consistency.

Student and model text is HTML-escaped and Markdown-safe; block quotes prefix
every source line, and table cells escape pipes, backslashes, and newlines.
Missing observations are rendered as `未提供`, never as a fabricated zero. The
autonomy bands are described as behavioral count bands, and no aggregate
student score is generated. JSON artifacts remain the machine-readable source
of truth.
