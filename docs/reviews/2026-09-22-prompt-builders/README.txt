Lite prompt / context builder refactor — 2026-09-22
===================================================
Base: latest origin/main checked with pull --ff-only, 03c31b99.
Branch: codex/lite-prompt-builders
Worktree: /private/tmp/mind-imprint-prompt-builders
Implementation commits: b151e7ed, 0255cd28.
Scope: the Lite reading/writing/awakening/interest/report/teacher/news/course
prompt families reviewed in this conversation, including shared Pro consumers.
This is an organization/inspection refactor, not a teaching-policy rewrite.

Review entry points
-------------------
apps/api/internal/prompts/doc.go
apps/api/internal/prompts/assemblies.go
apps/api/cmd/promptinspect/README.txt

64 fixed prompt definitions now live in a dedicated Go package. Comments are
ordinary Go comments, never model input. Existing constant composition and
placeholders compile as before. No new runtime format parser or dependency.

20 feature entries link fixed text, dynamic builders, context, contracts and
tests. Dynamic instructions, tool descriptions and typed teaching registries
remain at explicit linked owners; they have not been duplicated as static text.
Pure API/agent prompt assembly functions were separated from handlers/parsers.

Reading coach, writing planning and the shared whole-piece context now have
explicit selection types plus renderers. The existing writing-room projection
is in writing_turn_context.go. Small domain builders retain typed inputs.
Selection metadata records history windows, paragraph omissions and body
truncation. Legacy mixed instruction/fact sections are labeled mixed. No claim
is made that all context across the entire product has become a generic DTO.

36 offline examples expose assembled messages and tools. Reading/planning
examples show section byte ranges; the long-context feedback example also shows
the shared piece fragment. The CLI uses production builders with synthetic
inputs. It is not an authenticated live replay and not a full HTTP envelope.

Preserved behavior
------------------
No intentional prompt wording change, message-order change, cache-prefix
reordering, model-routing change, schema change, context-budget change, state
transition change or database migration. Missing input remains missing input.
This was checked against the architecture's server-only gateway ownership and
docs/2026-08-09-all-statuses.md; no workflow decisions were changed.

Validation
----------
104 deterministic main-baseline comparisons pass:
- 68 production-built requests captured before refactoring;
- 28 additional requests rendered by builders in an archive of main 03c31b99,
  including teacher tool schemas, interest/news and shared coach cases;
- 8 whole-piece context cases rendered against that same main archive, including
  zh/en assigned tasks, empty/missing text, Unicode truncation, excess blocks,
  and prior feedback.
The tests persist hashes, not a second editable set of production prompts.
Hash equality validates bytes/roles/order/options, not model quality.

Full Go test suite and targeted context/preview/catalog tests pass; see checks.
No frontend code was changed, so no new browser/UI test was needed for this work.

Real-model validation:
- 49 recorded samples, 0 specified validator failures: 38 on
  dashscope/deepseek-v4.1-flash and 11 on dashscope/glm-5.3. Provider response model
  IDs were recorded too. Covers reading/advancement/plan/lens, writing planning,
  English/narrative distinctions, personal-experience readiness, retry invitation,
  feedback, grading, shared coach/course and awakening opening/retry/last turns.
- 3 report composers pass on configured deepseek-v4-pro, 5 calls total. Parent
  report first attempt failed the Chinese-number rule (两个); class weekly first
  attempt duplicated a student card ID. Existing production retries recovered
  both. These are not reported as first-attempt passes.
- 1 teacher assignment scenario passes through the real HTTP handler and tool
  loop against a disposable test database, 3 model calls. No production student
  data or external class was modified.

Estimated total model cost upper bound: CNY 0.7858272 using checked-in prices.
Cached input is included where recorded; the teacher log does not expose cache
counts, so its input is estimated at full price. This is not a billing invoice.
See summary.json for counts/usage, live-requests.json.gz for the 49 recorded
requests/responses, and checks/live-*.txt for production report/teacher runs.

Limits and observations
-----------------------
The suite does not prove every future response is correct or teacher-like.
Report output still contains some evaluative language (e.g. inferring good study
habits from on-time completion); this round preserves the existing prompts,
validators and retry behavior so that future editorial changes can be isolated.
No online deployment, historical-conversation rewrite or full authenticated
frontend E2E was performed. Main was not merged or pushed by this refactor.

Evidence
--------
main-requests.json.gz: 28 main-generated example requests.
final-previews.json.gz: 36 current synthetic previews, including traces.
assemblies.json / templates.json: exported navigation metadata.
live-requests.json.gz: 49 live samples, visible responses and usage only.
summary.json: counts, model IDs, retry observations and cost calculation.
checks/: test output; all fixtures are synthetic. Provider secret values were
checked against the evidence content and had zero matches.
