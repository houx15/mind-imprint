Lite prompt inspection (offline)
================================
Run from apps/api. No model key, database or network connection is needed.

  go run ./cmd/promptinspect -list
      Feature index: purpose, builder/context/contract files, relevant tests.

  go run ./cmd/promptinspect -templates
  go run ./cmd/promptinspect -template api.readingCoachSystem
      Static Go definitions. Comments explain purpose but are never model input.

  go run ./cmd/promptinspect -cases
  go run ./cmd/promptinspect -case reading/open-lens
  go run ./cmd/promptinspect -case writing/plan/en/argument
  go run ./cmd/promptinspect -case writing/feedback/long-context
  go run ./cmd/promptinspect -case teacher/assignment
  go run ./cmd/promptinspect -all > /tmp/lite-prompt-examples.json
      Render synthetic inputs using production builder functions. Output
      includes ordered messages, tool schemas, and request settings represented
      by the fixture. Reading/writing planning additionally expose section byte
      offsets and context-selection counts. The long-context feedback example
      also shows the embedded shared piece-context fragment and its truncation. These are example requests, not a
      capture of a live student's complete tool loop or the HTTP wire envelope.
      No timestamp, key or model override is injected by this tool.

Where to edit
-------------
internal/prompts/*.go
    Fixed prompt text, rules, and retry text. Go constants allow comments,
    compile-time composition and ordinary IDE navigation. Existing fmt/replace
    placeholders retain their original behavior. No new template language.

internal/prompts/assemblies.go
    Review entry point, including instructions that are dynamically rendered
    rather than constants. Tool descriptions, card/method/rubric registries and
    awakening node/guide definitions are explicitly linked here. Those typed
    registries keep their existing owner; they are not copied into prompt text.

internal/api/*_prompt.go, internal/agent/*_prompt.go, domain builders
    Conditional instructions and language choices; render selected facts.
    Small pure domains (awakening/interest/news/litegrade/liteworkspace) retain
    their typed input builders. Output parsers and API handlers keep ownership
    of schemas, permission checks, retries, persistence and state transitions.

internal/api/reading_coach_context.go
internal/api/writing_plan_context.go
internal/api/writing_piece_context.go
internal/api/writing_turn_context.go
    Selection and projection of data already loaded by the authorized handler.
    The first three have explicit selected-context types. The turn projection
    retains its existing pure string interface. They do not load model keys or
    query the database. They must not be mistaken for authorization boundaries.

Trace interpretation
--------------------
Sections cover ranges of the final string; annotations do not add prompt bytes.
"mixed" means a retained legacy section contains both facts and instructions.
Selection counts use paragraphs/messages/runes, not tokens. History counts are
before role/empty-message filtering. An omitted/truncated source is not absent
student work. The first-12-block/300-rune writing policy is preserved, not newly
endorsed as sufficient for every future diagnosis.

Verification
------------
  go test ./...
      Existing functional tests plus 68 request snapshots, 36 additional
      main-built request snapshots (including tools), and 8 shared writing
      context boundary snapshots. Opt-in live tests still require their flags.
      Snapshots check bytes/options/order, not the quality of model responses.

  PROMPT_CAPTURE_DIR=/tmp/prompt-candidate go test ./internal/api ./internal/agent ./internal/awakening -run '^TestClarity' -count=1
      Capture synthetic requests without a model call. Review the diff before
      deliberately updating internal/claritytest/testdata/request_hashes.json.
      A baseline change does not substitute for functional/live validation.

Real-model validation uses the existing CLARITY_LIVE/LIVE_LLM tests and current
model routing. An example for the clarity helper (set paths for your machine):

  CLARITY_LIVE=1 CLARITY_ENV_FILE=/path/to/server.env CLARITY_OUT=/tmp/prompt-live-new go test ./internal/api -run '^TestClarityWriting$' -count=1 -v

The helper reads model settings only, uses synthetic student input and records
requests, visible responses, provider-reported model IDs and token usage. Never
put an environment file or real student conversations in the evidence folder.
