# September 22 copy and prompt review

Work in progress. Base: `092170b8` (fresh `origin/main`); previous reviewed commit: `10f065d6`.

Scope: changed application source, shared vocabulary, marketing pages, backend rendered messages and production prompt builders. Preserve student input, quoted teaching examples, original exam questions, identifiers and schema/state contracts. Audit inventory and final test evidence will be recorded here.

Work batches:
1. Read the changed copy and prompts; clarify static instructions and shared writing guidance.
2. Revise problematic prompt language without changing model routing, cache-prefix order or functional contracts.
3. Run offline tests and production-builder live model cases; inspect rendered UI; record limits and results.

Writing-flow check: `docs/2026-08-09-all-statuses.md` remains authoritative. This review changes wording; it does not change stage transitions, student authorship, optional reviews or automatic plan derivation.

Current model catalog: dialogue `dashscope/deepseek-v4.1-flash`, compose/review `dashscope/glm-5.3`, assess `dashscope/deepseek-v4-pro`. Environment bindings must be checked during live runs.
