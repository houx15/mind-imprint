# N3b · Moment classifier + live refeed seam — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire N3a's two unwired cards to a real semantic classifier, and make a
completed card actually reach the coach (摘要回灌) — on both the Studio and Chat
surfaces.

**Architecture:** Two seams. **Seam A** = `agent.ClassifyMoment`, a new
chaperone-tier subagent that maps the student's latest text to one of three
closed-set moments (or none), behind a four-part structural pre-gate; its
answer becomes a `surface_card` candidate in Studio and a card offer + coach
flag in Chat. **Seam B** = the live refeed: `Trigger` carries the just-completed
card instance, `BuildCoachContext` renders `SerializeCardForRefeed`'s payload
for a `card_instance`-anchored candidate (Studio), and `BuildChatContext` folds
completed-card summaries into the next turn (Chat). Plus two carry-forward
fixes.

**Tech Stack:** Go (`apps/api`) only. No migration, no sqlc regeneration, no
`packages/contracts` change, no web change.

**Spec:** `docs/superpowers/specs/2026-07-21-n3b-moment-classifier-and-refeed-design.md`

## Global Constraints

- **NO migration. NO sqlc regeneration. NO new sqlc query.** `GetCardInstance`
  is already `SELECT *` and already on the `AgentStore` interface.
- **NO change to `packages/contracts`, no card JSON change, no web change.**
  If a task believes it needs one, it must stop and report, not proceed.
- **NO new model tier.** The classifier uses the caller's already-resolved
  chaperone `gateway.Resolved`. Never construct a resolver inside `agent`.
- **Every LLM call is metered**, including when its answer is `none` or
  unparseable (AGENTS.md 记录档位 + token + 成本). A `RecordLLMCall` failure
  logs and continues — it must never fail the student's turn.
- **Classifier failure is silence.** Transport error, timeout, or unparseable
  reply → no candidate, no error propagated to the caller's turn.
- **One mapping table.** `momentCard` in `moment.go` is the ONLY place a
  moment's card id, flag sentence, reason, or criterion is written. No task may
  hardcode `"fact-opinion-value"`, `"certainty-spectrum"`, or `"steelman"`
  anywhere else in non-test code.
- **An offer is never a wall** (铁律 2): a moment is ineligible once its target
  card has a `card_instance` in **any** status, including `skipped`.
- **AI 克制** (铁律 1 / RL-4): the classifier outputs one identifier, never
  student-facing prose. The coach still writes only ONE question (铁律 3).
- **`GraphView.CardInstanceView` must NOT be widened.** Every pure classifier
  test constructs it as a fixture; N1 and N2 each cost a fixture-cascade wave.
  The refeed path loads what it needs directly.
- **Go gate: FULL packages, never `-run` subsets.** From `apps/api`:
  `DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...`
  For a task touching only `internal/agent`, `... CGO_ENABLED=0 go test ./internal/agent/...`
  is an acceptable per-task gate; the final whole-branch gate is always `./...`.
- **NEVER `git add` a bare directory.** The pre-existing `M package.json` and
  the untracked files under `docs/` and the repo root are NOT ours. Stage named
  files only.
- Criteria come from `internal/rubric/dualaxis.json`: `D3` 证据与信源意识,
  `D4` 论证结构意识, `D6` 元认知与反思.
- **Reuse the existing test fakes; never invent one.** Tasks 3–8 give test
  *names and intent* rather than literal bodies, deliberately: each needs the
  in-memory `AgentStore`/`ChatStore` and provider fakes that already live in
  those `_test.go` files, and a plan that guessed at their APIs would hand you
  a mock encoding a shape the backend cannot produce — the exact
  test-mock-infidelity failure that cost Slice 12 five Criticals behind a green
  suite. **Read the file, find the fake, use it.** Every listed test must be
  written with real assertions and must fail before its implementation step.
  If no reusable fake exists in a file, say so in your report.

---

### Task 1: The moment vocabulary + mapping table

**Files:**
- Create: `apps/api/internal/agent/moment.go`
- Test: `apps/api/internal/agent/moment_test.go`

**Interfaces:**
- Consumes: nothing (pure new file).
- Produces: `Moment` (string type), the four constants, `momentCard`
  (unexported map), and `EligibleMoments(cards []CardInstanceView) []Moment` —
  used by Task 3 (Studio). Task 4 (Chat) gets its own overload; do not try to
  share one signature across both view types.

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/agent/moment_test.go`:

```go
package agent

import "testing"

func TestMomentCardTableIsComplete(t *testing.T) {
	for _, m := range AllMoments {
		e, ok := momentCard[m]
		if !ok {
			t.Fatalf("moment %q has no momentCard entry", m)
		}
		if e.CardID == "" || e.Desc == "" || e.Flag == "" || e.Reason == "" || e.Criterion == "" {
			t.Fatalf("moment %q has an incomplete entry: %+v", m, e)
		}
	}
	if len(momentCard) != len(AllMoments) {
		t.Fatalf("momentCard has %d entries, AllMoments has %d", len(momentCard), len(AllMoments))
	}
}

func TestEligibleMomentsDropsAnyStatus(t *testing.T) {
	// An offer is never a wall: a card_instance in ANY status — including
	// skipped — makes its moment permanently ineligible.
	for _, status := range []string{"proposed", "active", "completed", "skipped"} {
		got := EligibleMoments([]CardInstanceView{{CardID: "steelman", Status: status}})
		for _, m := range got {
			if m == MomentOneSided {
				t.Fatalf("status %q: one_sided still eligible after an offer", status)
			}
		}
		if len(got) != len(AllMoments)-1 {
			t.Fatalf("status %q: got %d eligible, want %d", status, len(got), len(AllMoments)-1)
		}
	}
}

func TestEligibleMomentsAllWhenNoCards(t *testing.T) {
	if got := EligibleMoments(nil); len(got) != len(AllMoments) {
		t.Fatalf("got %d eligible moments, want %d", len(got), len(AllMoments))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run TestMoment`
Expected: FAIL — `undefined: AllMoments`, `undefined: momentCard`, `undefined: EligibleMoments`.

- [ ] **Step 3: Write the implementation**

Create `apps/api/internal/agent/moment.go`:

```go
package agent

// Moment is one member of the CLOSED SET of semantic card-moments the
// classifier may name. It is a closed enum on purpose: the classifier picks
// from a fixed vocabulary and can never invent a card id, which is the same
// principle that makes the C1 interaction primitives a hand-built library
// rather than model-generated UI (agent-spec §3).
type Moment string

const (
	// MomentNone is the classifier's "nothing here" answer, and the zero value.
	MomentNone Moment = ""

	// MomentFactOpinion: the student stated an opinion that needs arguing as
	// if it were a checkable fact.
	MomentFactOpinion Moment = "fact_opinion"

	// MomentOverclaim: the student's expressed certainty outruns the evidence
	// he has actually given.
	MomentOverclaim Moment = "overclaim"

	// MomentOneSided: the student argues from one side only, never engaging
	// the strongest opposing case.
	MomentOneSided Moment = "one_sided"
)

// AllMoments is the vocabulary in stable priority order — when the classifier
// is offered several, this is the order it sees them in, and the order
// EligibleMoments returns.
var AllMoments = []Moment{MomentFactOpinion, MomentOverclaim, MomentOneSided}

// momentEntry is everything the rest of the system needs to know about one
// moment. This table is the SINGLE SOURCE for a moment's card id, its
// description in the classifier prompt, the coach-facing flag sentence, the
// candidate's internal reason, and its CT criterion. No other file may
// hardcode these card ids (Global Constraints).
type momentEntry struct {
	CardID string
	// Desc is how the moment is described TO THE CLASSIFIER (Chinese, one line).
	Desc string
	// Flag is the sentence handed to the COACH as context ("刚刚发生的思考时机").
	Flag string
	// Reason is the internal-only candidate reason (never shown to a student).
	Reason string
	// Criterion is the CT dimension tag (internal/rubric/dualaxis.json).
	Criterion string
}

var momentCard = map[Moment]momentEntry{
	MomentFactOpinion: {
		CardID:    "fact-opinion-value",
		Desc:      "学生把一个需要论证的观点，当成可以直接查证的事实陈述来用。",
		Flag:      "学生把一个需要论证的观点当成了事实——这是分辨「事实/观点/价值判断」的时机。",
		Reason:    "student stated an arguable opinion as a checkable fact",
		Criterion: "D3",
	},
	MomentOverclaim: {
		CardID:    "certainty-spectrum",
		Desc:      "学生表达的确定程度，高于他给出的证据所能支撑的程度。",
		Flag:      "学生的语气比他的证据更确定——这是给结论标定「确定度」的时机。",
		Reason:    "student's certainty outruns the evidence given",
		Criterion: "D3",
	},
	MomentOneSided: {
		CardID:    "steelman",
		Desc:      "学生只从一侧论证，没有处理最强的反面意见。",
		Flag:      "学生只从一侧论证——这是构造对方最强版本的时机。",
		Reason:    "student argues from one side only",
		Criterion: "D4",
	},
}

// EligibleMoments returns the moments still open for a project, in AllMoments
// order. A moment is ineligible as soon as its target card has a card_instance
// in ANY status — including "skipped". An offer is never a wall: once she has
// said no, we do not ask again (铁律 2 · 不操纵).
func EligibleMoments(cards []CardInstanceView) []Moment {
	seen := make(map[string]bool, len(cards))
	for _, c := range cards {
		seen[c.CardID] = true
	}
	out := make([]Moment, 0, len(AllMoments))
	for _, m := range AllMoments {
		if !seen[momentCard[m].CardID] {
			out = append(out, m)
		}
	}
	return out
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/`
Expected: PASS (whole package, not just the new tests).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/moment.go apps/api/internal/agent/moment_test.go
git commit -m "feat(n3b): the closed-set moment vocabulary + its single mapping table"
```

---

### Task 2: `ClassifyMoment` — the classifier subagent

**Files:**
- Modify: `apps/api/internal/agent/moment.go` (append)
- Test: `apps/api/internal/agent/moment_test.go` (append)

**Interfaces:**
- Consumes: Task 1's `Moment`, `momentCard`, `AllMoments`;
  `gateway.Provider`, `gateway.Resolved`, `gateway.Collect`, `gateway.ChatUsage`.
- Produces:
  `ClassifyMoment(ctx, prov, r, text string, eligible []Moment) (Moment, gateway.ChatUsage, error)`
  and the exported `MinClassifyRunes = 12`. Used by Task 3 (Studio) and Task 4 (Chat).

**Reference the existing pattern:** `apps/api/internal/agent/chat_coach.go`'s
`ProposeChatReply` is the shape to mirror for the `gateway.Collect` call and the
usage-returned-even-on-error contract. Read it before writing.

- [ ] **Step 1: Write the failing test**

Append to `apps/api/internal/agent/moment_test.go`. Use the package's existing
fake provider — find it by grepping `internal/agent/*_test.go` for the type
implementing `gateway.Provider`; if none is reusable, define a local one in this
file with a settable reply text and error.

```go
func TestClassifyMomentParsesExactMatchOnly(t *testing.T) {
	cases := []struct {
		name  string
		reply string
		want  Moment
	}{
		{"exact", "one_sided", MomentOneSided},
		{"trimmed", "  overclaim\n", MomentOverclaim},
		{"none", "none", MomentNone},
		{"chatty", "我认为这是 one_sided 的时机。", MomentNone},
		{"unknown id", "steelman", MomentNone},
		{"empty", "", MomentNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prov := &fakeMomentProvider{text: tc.reply}
			got, usage, err := ClassifyMoment(t.Context(), prov, gateway.Resolved{}, "一段足够长的学生发言内容在这里。", AllMoments)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("reply %q: got %q, want %q", tc.reply, got, tc.want)
			}
			// Usage is reported whatever the answer — a `none` still cost money.
			if usage.InputTokens == 0 && usage.OutputTokens == 0 {
				t.Fatalf("reply %q: usage not reported", tc.reply)
			}
		})
	}
}

func TestClassifyMomentRejectsIneligible(t *testing.T) {
	// The model named a real moment that is NOT in the eligible set — it must
	// not be honoured, or a suppressed card would be re-offered.
	prov := &fakeMomentProvider{text: "one_sided"}
	got, _, err := ClassifyMoment(t.Context(), prov, gateway.Resolved{}, "一段足够长的学生发言内容在这里。", []Moment{MomentOverclaim})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != MomentNone {
		t.Fatalf("got %q, want none", got)
	}
}

func TestClassifyMomentNoCallWhenNothingEligible(t *testing.T) {
	prov := &fakeMomentProvider{text: "one_sided"}
	got, _, err := ClassifyMoment(t.Context(), prov, gateway.Resolved{}, "一段足够长的学生发言内容在这里。", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != MomentNone {
		t.Fatalf("got %q, want none", got)
	}
	if prov.calls != 0 {
		t.Fatalf("provider called %d times with no eligible moments; want 0", prov.calls)
	}
}

func TestClassifyMomentPromptListsOnlyEligible(t *testing.T) {
	prov := &fakeMomentProvider{text: "none"}
	if _, _, err := ClassifyMoment(t.Context(), prov, gateway.Resolved{}, "一段足够长的学生发言内容在这里。", []Moment{MomentOverclaim}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	joined := prov.lastPrompt
	if !strings.Contains(joined, string(MomentOverclaim)) {
		t.Fatalf("prompt does not offer the eligible moment: %s", joined)
	}
	if strings.Contains(joined, string(MomentOneSided)) {
		t.Fatalf("prompt leaked an INELIGIBLE moment: %s", joined)
	}
}

func TestClassifyMomentErrorsSurfaceForMetering(t *testing.T) {
	prov := &fakeMomentProvider{err: errors.New("boom")}
	got, _, err := ClassifyMoment(t.Context(), prov, gateway.Resolved{}, "一段足够长的学生发言内容在这里。", AllMoments)
	if err == nil {
		t.Fatal("want an error from a failed provider call")
	}
	if got != MomentNone {
		t.Fatalf("got %q, want none on error", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run TestClassifyMoment`
Expected: FAIL — `undefined: ClassifyMoment`.

- [ ] **Step 3: Write the implementation**

Append to `apps/api/internal/agent/moment.go` (add `context`, `fmt`, `strings`,
`mindimprint/api/internal/gateway` to the imports):

```go
// MinClassifyRunes is the floor below which a student turn is never
// classified. Cost discipline AND product: 「嗯」 carries no moment, and paying
// a model call to be told so is waste.
const MinClassifyRunes = 12

// momentSystemPrompt is the classifier's posture. It is deliberately unlike
// every other prompt in this package: the classifier does not talk to the
// student, does not coach, and does not write prose. Its entire output is one
// identifier from a closed set.
const momentSystemPrompt = `# 角色
你是「思维印记」的时机识别器。你不与学生对话，也不给任何建议——你唯一的任务，是判断学生刚写下的这段话里，是否正在发生下面列出的某一个「思考时机」。

# 规则
- 只能从下面给出的候选 id 中选**一个**，或者回答 none。
- 拿不准就回答 none。宁可错过，也不要打断学生。
- 只输出那个 id 本身，不要解释、不要标点、不要任何其他文字。`

// ClassifyMoment asks the chaperone-tier model whether the student's latest
// text exhibits one of the eligible moments.
//
// Contract:
//   - Empty eligible set → MomentNone with NO model call (cost discipline).
//   - The reply is matched by EXACT equality against the eligible ids after
//     trimming. A chatty reply, an unknown id, or an id that is real but NOT
//     eligible all collapse to MomentNone — the last case matters most: it is
//     what stops a suppressed card from being re-offered by a model that
//     ignored its instructions.
//   - Usage is returned whenever gateway.Collect succeeded, INCLUDING when the
//     answer is `none` or unparseable. That call cost real money and the caller
//     must meter it (AGENTS.md 记录档位 + token + 成本). Usage is the zero
//     value only when Collect itself errored.
//   - An error is returned ONLY for a failed model call. Callers treat it as
//     silence, never as a turn failure.
func ClassifyMoment(ctx context.Context, prov gateway.Provider, r gateway.Resolved, text string, eligible []Moment) (Moment, gateway.ChatUsage, error) {
	if len(eligible) == 0 {
		return MomentNone, gateway.ChatUsage{}, nil
	}

	var b strings.Builder
	b.WriteString("# 候选时机\n")
	for _, m := range eligible {
		fmt.Fprintf(&b, "- %s：%s\n", m, momentCard[m].Desc)
	}
	b.WriteString("\n# 学生刚写下的话\n")
	b.WriteString(text)
	b.WriteString("\n\n只输出一个 id，或 none。")

	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: momentSystemPrompt},
			{Role: gateway.RoleUser, Content: b.String()},
		},
	})
	if err != nil {
		return MomentNone, gateway.ChatUsage{}, err
	}

	answer := strings.TrimSpace(res.Text)
	for _, m := range eligible {
		if answer == string(m) {
			return m, res.Usage, nil
		}
	}
	return MomentNone, res.Usage, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/moment.go apps/api/internal/agent/moment_test.go
git commit -m "feat(n3b): ClassifyMoment — closed-set semantic classifier, exact-match parse"
```

---

### Task 3: Studio wiring — `Trigger.StudentText` + the pre-gate + the candidate

**Files:**
- Modify: `apps/api/internal/agent/runtime.go` (the `Trigger` struct)
- Modify: `apps/api/internal/agent/loop.go` (`RunAgentStep`)
- Modify: `apps/api/internal/api/studioturn.go` (pass the student's text)
- Test: `apps/api/internal/agent/loop_test.go` (append)

**Interfaces:**
- Consumes: Task 1's `EligibleMoments`, `momentCard`; Task 2's `ClassifyMoment`,
  `MinClassifyRunes`.
- Produces: `Trigger.StudentText`. Task 5 adds `Trigger.CardInstanceID` to the
  SAME struct — do not remove or rename `StudentText`.

**Read first:** `loop.go`'s `RunAgentStep` in full, especially the candidate
ordering comment above it and the existing `RecordLLMCall` block (lines ~295-309)
— the metering you add must follow the same "never fail the turn" policy.

- [ ] **Step 1: Write the failing test**

Append to `apps/api/internal/agent/loop_test.go`. Reuse that file's existing
in-memory `AgentStore` fake and fake provider — do NOT define new ones.

```go
// A semantic candidate must NOT be produced when a structural surface_card
// already won: decide-one would discard the answer, so we must not pay for it.
func TestRunAgentStepSkipsClassifierWhenStructuralCardWins(t *testing.T) {
	// GraphView with one un-evaluated article material → CRAAP fires.
	// Assert: the action is the CRAAP surface_card AND the provider recorded
	// zero classify calls.
}

// The pre-gate: text shorter than MinClassifyRunes is never classified.
func TestRunAgentStepSkipsClassifierOnShortText(t *testing.T) {
	// Graph with no structural candidate at all; Trigger.StudentText = "嗯".
	// Assert zero provider calls.
}

// A named moment becomes a surface_card candidate for that moment's card.
func TestRunAgentStepSemanticMomentSurfacesItsCard(t *testing.T) {
	// Graph with no structural candidate; classifier replies "one_sided".
	// Assert: action.Kind == "surface_card" && action.CardID == "steelman".
}

// Suppression: the card already exists in ANY status → no classify call.
func TestRunAgentStepSemanticSuppressedByAnyStatus(t *testing.T) {
	// Graph carries CardInstanceView{CardID:"steelman", Status:"skipped"}
	// and the other two cards' instances too, so nothing is eligible.
	// Assert zero provider classify calls and a nil action.
}

// A classifier failure is silence, never a turn failure.
func TestRunAgentStepClassifierErrorIsSilent(t *testing.T) {
	// Provider errors. Assert err == nil and action == nil.
}

// SkipSurfaceCards suppresses the SEMANTIC candidate too.
func TestRunAgentStepSkipSurfaceCardsSuppressesSemantic(t *testing.T) {
	// deps.SkipSurfaceCards = true, classifier would reply "one_sided".
	// Assert zero provider classify calls.
}
```

Write each of these as a REAL test with real assertions — the comments above
state the intent, not the body. Every one must fail before Step 3.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run TestRunAgentStep`
Expected: FAIL — the semantic path does not exist.

- [ ] **Step 3: Write the implementation**

**3a.** In `runtime.go`, extend `Trigger`:

```go
// Trigger mirrors the agent-spec tiers that can invoke RunAgentStep: T-A
// (summon), T-B (structural), T-C (fine-grained — context-only in Slice 2).
type Trigger struct {
	Kind string

	// StudentText is the message that provoked this step, carried on the
	// trigger rather than re-read from history: RunAgentStep loads chat
	// history only AFTER the surface-card branch, and the semantic pre-gate
	// runs before it. Set for Kind == "student_turn"; empty elsewhere, which
	// disables the semantic classifier by construction (N3b).
	StudentText string
}
```

**3b.** In `loop.go`'s `RunAgentStep`, immediately after
`cands = append(cands, checkGateCands...)` and BEFORE `if len(cands) == 0`,
insert the semantic pass. The placement rule: the semantic candidate is
inserted ahead of every `post_intervention`/`observe`/`check_gate` candidate
but behind structural `surface_card`s.

```go
	// N3b Seam A — the semantic moment classifier. It runs ONLY when the
	// structural classifier said nothing about cards: decide-one acts on
	// cands[0], so a semantic answer produced alongside a structural
	// surface_card would simply be discarded, and paying a model call for a
	// discarded answer is waste. The resulting behaviour is also the right
	// one: structure first, semantics as the fallback that notices what
	// structure cannot see.
	if !deps.SkipSurfaceCards && trigger.Kind == "student_turn" && !hasSurfaceCard(cands) {
		if c, ok := semanticCardCandidate(ctx, deps, projectID, g, trigger.StudentText); ok {
			cands = append([]Candidate{c}, cands...)
		}
	}
```

and add, at the bottom of `loop.go`:

```go
// hasSurfaceCard reports whether any structural surface_card candidate already
// fired this turn.
func hasSurfaceCard(cands []Candidate) bool {
	for _, c := range cands {
		if c.Verb == "surface_card" {
			return true
		}
	}
	return false
}

// semanticCardCandidate runs N3b's classifier behind its structural pre-gate
// and turns a named moment into a project-scoped surface_card candidate.
//
// The call is metered even when the answer is `none` or the reply was
// unparseable — it cost real money either way. A metering failure logs and
// continues; a classifier failure is silence. Neither ever fails the turn.
func semanticCardCandidate(ctx context.Context, deps AgentDeps, projectID uuid.UUID, g GraphView, text string) (Candidate, bool) {
	if len([]rune(strings.TrimSpace(text))) < MinClassifyRunes {
		return Candidate{}, false
	}
	eligible := EligibleMoments(g.CardInstances)
	if len(eligible) == 0 {
		return Candidate{}, false
	}
	moment, usage, err := ClassifyMoment(ctx, deps.Provider, deps.Resolved, text, eligible)
	if usage.InputTokens > 0 || usage.OutputTokens > 0 {
		if rerr := deps.Store.RecordLLMCall(ctx, LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "classify",
			Resolved: deps.Resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
		}); rerr != nil {
			slog.Warn("agent: record classifier usage failed", "project_id", projectID.String(), "err", rerr.Error())
		}
	}
	if err != nil {
		slog.Warn("agent: moment classifier failed; staying silent", "project_id", projectID.String(), "err", err.Error())
		return Candidate{}, false
	}
	if moment == MomentNone {
		return Candidate{}, false
	}
	e := momentCard[moment]
	return Candidate{
		Verb:       "surface_card",
		AnchorKind: "project",
		AnchorID:   "",
		CardID:     e.CardID,
		Criterion:  e.Criterion,
		Reason:     e.Reason,
	}, true
}
```

Add `"strings"` to `loop.go`'s imports if absent.

**3c.** In `apps/api/internal/api/studioturn.go`, change the `RunAgentStep`
call at line ~218 to carry the student's text. The handler already decoded and
persisted that message — use the same variable, do not re-read it:

```go
	action, err := agent.RunAgentStep(r.Context(), deps, projectID, agent.Trigger{Kind: "student_turn", StudentText: <the student's message variable>})
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/agent/... ./internal/api/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/runtime.go apps/api/internal/agent/loop.go apps/api/internal/agent/loop_test.go apps/api/internal/api/studioturn.go
git commit -m "feat(n3b): Studio wiring — semantic moment candidate behind a structural pre-gate"
```

---

### Task 4: Chat wiring — the classifier replaces the hardcoded flag

**Files:**
- Modify: `apps/api/internal/agent/chat_step.go`
- Test: `apps/api/internal/agent/chat_step_test.go` (append)

**Interfaces:**
- Consumes: Task 1's `momentCard`/`AllMoments`; Task 2's `ClassifyMoment`,
  `MinClassifyRunes`. `ChatStore.RecordChatLLMCall(ctx, userID, purpose, resolved, prompt, completion)`.
- Produces: nothing new for later tasks.

**Read first:** `chat_step.go` in full. `RunChatStep`'s existing structure is
(1) URL → material, (2) `ChatCardCandidate` → flag, (3) coach reply, (4) offer.
Your change lives in (2) and (4) only; do not restructure the function.

**Key existing facts you must preserve:**
- The structural `ChatCardCandidate` (link → CRAAP) **wins when it fires**. The
  classifier runs only when it does not.
- A semantic chat offer has **no material**: `CardOffer.MaterialID` is
  `uuid.Nil`. This is safe — the chat card path is thin (no `CompleteCard`) and
  neither `StudioCardSheet` nor `ChatSurface` reads `materialId`.
- Chat meters via `RecordChatLLMCall`, NOT `RecordLLMCall`.

- [ ] **Step 1: Write the failing test**

Append to `apps/api/internal/agent/chat_step_test.go`, reusing that file's
existing in-memory `ChatStore` fake and provider fake.

```go
// The structural link→CRAAP moment still wins; no classifier call is made.
func TestRunChatStepStructuralMomentWinsOverClassifier(t *testing.T) { /* ... */ }

// With no link and no CRAAP moment, a named semantic moment mints that card.
func TestRunChatStepSemanticMomentOffersItsCard(t *testing.T) { /* ... */ }

// The semantic offer carries a nil material and still emits card_surfaced.
func TestRunChatStepSemanticOfferHasNoMaterial(t *testing.T) { /* ... */ }

// Any-status suppression: a skipped steelman is never re-offered in the thread.
func TestRunChatStepSemanticSuppressedByAnyStatus(t *testing.T) { /* ... */ }

// The classify call is metered through RecordChatLLMCall with purpose "classify".
func TestRunChatStepMetersClassifyCall(t *testing.T) { /* ... */ }

// A classifier failure leaves the coach reply intact — silence, not failure.
func TestRunChatStepClassifierErrorStillReplies(t *testing.T) { /* ... */ }
```

Write each as a real test with real assertions.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run TestRunChatStep`
Expected: FAIL.

- [ ] **Step 3: Write the implementation**

Add an `EligibleMomentsScoped([]ScopedCard) []Moment` helper to `moment.go`
(mirroring `EligibleMoments`, over the `ScopedCard` shape — Chat/Course use a
different view type than the project GraphView), then in `RunChatStep` replace
the flag block:

```go
	materialID, cardID, moment := ChatCardCandidate(mats, cards)
	flag := ""
	if moment {
		flag = "学生贴进了一个来源链接，并把它当成论据——这是做「信源辨识（CRAAP）」的时机。"
	} else if len([]rune(strings.TrimSpace(studentMessage))) >= MinClassifyRunes {
		// N3b Seam A in Chat: the structural link moment did not fire, so ask
		// the classifier whether a SEMANTIC one did. Metered even on `none`
		// or a parse failure; a classifier error is silence, never a failed
		// turn — the coach still replies below.
		eligible := EligibleMomentsScoped(cards)
		m, usage, cerr := ClassifyMoment(ctx, deps.Provider, deps.Resolved, studentMessage, eligible)
		if usage.InputTokens > 0 || usage.OutputTokens > 0 {
			if rerr := deps.Store.RecordChatLLMCall(ctx, deps.UserID, "classify", deps.Resolved, int32(usage.InputTokens), int32(usage.OutputTokens)); rerr != nil {
				slog.Warn("chat: record classifier usage failed", "thread_id", deps.ThreadID.String(), "err", rerr.Error())
			}
		}
		if cerr != nil {
			slog.Warn("chat: moment classifier failed; staying silent", "thread_id", deps.ThreadID.String(), "err", cerr.Error())
		} else if m != MomentNone {
			e := momentCard[m]
			flag = e.Flag
			cardID = e.CardID
			materialID = uuid.Nil // a semantic offer is not about one source
			moment = true
		}
	}
```

Step (4)'s existing offer block then works unchanged.

Add `"strings"` to the imports if absent.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/agent/... ./internal/api/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/moment.go apps/api/internal/agent/chat_step.go apps/api/internal/agent/chat_step_test.go
git commit -m "feat(n3b): Chat wiring — semantic moment offers, structural link moment still wins"
```

---

### Task 5: Seam B (Studio) — the live refeed candidate

**Files:**
- Modify: `apps/api/internal/agent/runtime.go` (`Trigger.CardInstanceID`)
- Modify: `apps/api/internal/agent/loop.go` (`CardInstanceRow.FieldValues`, the refeed branch)
- Modify: `apps/api/internal/agent/agentstore.go` (`toCardInstanceRow`)
- Modify: `apps/api/internal/agent/coach_prompt.go` (`BuildCoachContext`)
- Modify: `apps/api/internal/api/projectcards.go` (pass the card instance id)
- Test: `apps/api/internal/agent/coach_prompt_test.go` + `loop_test.go` (append)

**Interfaces:**
- Consumes: `SerializeCardForRefeed(spec cards.Spec, inst CardInstance) RefeedPayload`
  (refeed.go, unchanged); `AgentStore.GetCardInstance(ctx, id) (CardInstanceRow, error)`
  (already on the interface).
- Produces: nothing for later tasks.

**Hard constraint:** do NOT widen `GraphView.CardInstanceView`, and do NOT add a
new sqlc query or store-interface method. `GetCardInstance` is already
`SELECT * FROM card_instances WHERE id = $1`; only the Go struct it maps into
needs the extra column.

- [ ] **Step 1: Write the failing test**

In `coach_prompt_test.go`:

```go
func TestBuildCoachContextRendersRefeedForCardInstanceAnchor(t *testing.T) {
	// c.AnchorKind == "card_instance"; pass a RefeedPayload with one step and
	// two answers. Assert the output contains the 「学生刚完成的工具卡」heading,
	// the step title, both answer labels and values, AND that it does NOT
	// contain the graph-node heading 「当前锚点节点」.
}

func TestBuildCoachContextUnchangedForGraphNodeAnchor(t *testing.T) {
	// Regression: an ordinary graph_node candidate renders exactly as before.
}
```

In `loop_test.go`:

```go
func TestRunAgentStepRefeedAsksAboutTheCompletedCard(t *testing.T) {
	// Trigger{Kind:"card_refeed", CardInstanceID: <completed instance>}.
	// Assert the emitted intervention's Anchor.Kind == "card_instance",
	// Anchor.ID == the instance id, and that the prompt the fake provider
	// received carried the card's field values.
}

func TestRunAgentStepRefeedSilentOnSkipped(t *testing.T) {
	// Same, but the instance status is "skipped". Assert action == nil and
	// the provider was never called. A skip is data, not a cue to nag.
}

func TestRunAgentStepRefeedOutranksOtherCandidates(t *testing.T) {
	// A graph that would also yield a post_intervention. Assert the refeed
	// candidate wins (decide-one takes cands[0]).
}
```

Write each as a real test with real assertions.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/`
Expected: FAIL.

- [ ] **Step 3: Write the implementation**

**5a.** `runtime.go` — add to `Trigger` (keeping Task 3's `StudentText`):

```go
	// CardInstanceID names the card_instance whose completion provoked this
	// step. Set for Kind == "card_refeed"; empty elsewhere, which disables the
	// refeed branch by construction (N3b Seam B).
	CardInstanceID string
```

**5b.** `loop.go` — add `FieldValues []byte` to `CardInstanceRow`, documented as
"the card's submitted field_values jsonb — the refeed serializer's other half
beside Anchors". `agentstore.go`'s `toCardInstanceRow` gains
`FieldValues: row.FieldValues,`.

**5c.** `loop.go` — in `RunAgentStep`, right after `g, err := deps.Store.LoadGraph(...)`
and BEFORE the candidate collection, insert the refeed branch. It is
first-priority by construction: it returns its own candidate list.

```go
	// N3b Seam B — 摘要回灌. A card the student just COMPLETED gets exactly one
	// coach question about what she wrote in it. This is the acceptance
	// mainline's 摘要回灌, and it outranks every other candidate: it is a direct
	// response to something she just did.
	//
	// A SKIPPED card gets silence. The skip is recorded as data (铁律 4 ·
	// 过程即数据); answering a decline with a question is the nagging posture
	// 铁律 2 forbids.
	var refeedCand *Candidate
	var refeedPayload *RefeedPayload
	if trigger.Kind == "card_refeed" && trigger.CardInstanceID != "" {
		if cand, payload, ok := refeedCandidate(ctx, deps, trigger.CardInstanceID); ok {
			refeedCand, refeedPayload = &cand, &payload
		}
	}
```

then, where `cands` is assembled, prepend `*refeedCand` when non-nil (before the
`SurfaceCardCandidates` result), and thread `refeedPayload` into the
`ProposeIntervention` call site so `BuildCoachContext` can render it.
`ProposeIntervention` gains one parameter, `refeed *RefeedPayload`, passed
straight through to `BuildCoachContext`; every existing call site passes `nil`.

Add the helper:

```go
// refeedCandidate loads the just-submitted card instance and, when it is
// COMPLETED, returns the coach candidate + the serialized payload the coach
// context renders. Any failure — unknown instance, unknown card id, load error
// — is silence: the submit itself already succeeded and must not be failed by
// its follow-up question.
func refeedCandidate(ctx context.Context, deps AgentDeps, cardInstanceID string) (Candidate, RefeedPayload, bool) {
	id, err := uuid.Parse(cardInstanceID)
	if err != nil {
		return Candidate{}, RefeedPayload{}, false
	}
	row, err := deps.Store.GetCardInstance(ctx, id)
	if err != nil {
		slog.Warn("agent: refeed load card instance failed", "card_instance_id", cardInstanceID, "err", err.Error())
		return Candidate{}, RefeedPayload{}, false
	}
	if row.Status != "completed" {
		return Candidate{}, RefeedPayload{}, false
	}
	spec, ok := cards.ByID(row.CardID)
	if !ok {
		return Candidate{}, RefeedPayload{}, false
	}
	inst := CardInstance{ID: cardInstanceID, CardID: row.CardID, Status: row.Status}
	_ = json.Unmarshal(row.Anchors, &inst.Anchors)         // absent/invalid → no anchor steps
	_ = json.Unmarshal(row.FieldValues, &inst.FieldValues) // absent/invalid → no field steps
	return Candidate{
		Verb:       "post_intervention",
		AnchorKind: "card_instance",
		AnchorID:   cardInstanceID,
		Criterion:  "D6", // 元认知与反思
		Level:      "I2",
		Reason:     "学生刚完成了一张工具卡",
	}, SerializeCardForRefeed(spec, inst), true
}
```

**5d.** `coach_prompt.go` — `BuildCoachContext(g GraphView, c Candidate, history []ChatTurn, refeed *RefeedPayload) string`.
When `c.AnchorKind == "card_instance"` and `refeed != nil`, render:

```
# 学生刚完成的工具卡
卡片：<card_name>
## <step title>
- <answer label>：<answer value>
```

in place of the 「当前锚点节点」/「相关边」 blocks. The history, 「为什么此刻需要介入」
and 「CT 维度」 blocks are unchanged and still emitted. For every other anchor
kind the function's output is byte-identical to today.

**5e.** `projectcards.go` — pass the instance id at the refeed call site:

```go
	action, err := agent.RunAgentStep(r.Context(), deps, projectID, agent.Trigger{Kind: "card_refeed", CardInstanceID: cid.String()})
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...`
Expected: PASS — FULL packages. This task changes a shared function signature;
a `-run` subset would hide breakage.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/runtime.go apps/api/internal/agent/loop.go apps/api/internal/agent/agentstore.go apps/api/internal/agent/coach.go apps/api/internal/agent/coach_prompt.go apps/api/internal/agent/coach_prompt_test.go apps/api/internal/agent/loop_test.go apps/api/internal/api/projectcards.go
git commit -m "feat(n3b): Seam B — the live refeed, one coach question per completed card"
```

---

### Task 6: Seam B (Chat) — completed-card summaries join the next turn

**Files:**
- Modify: `apps/api/internal/agent/chat_step.go` (`ScopedCard`, the summary block)
- Modify: `apps/api/internal/agent/chat_coach.go` (`BuildChatContext`)
- Modify: `apps/api/internal/agent/chatstore.go` + `coursestore.go` (populate the new fields)
- Test: `apps/api/internal/agent/chat_coach_test.go` + `chat_step_test.go` (append)

**Interfaces:**
- Consumes: `SerializeCardForRefeed`; Task 4's chat structure.
- Produces: nothing for later tasks.

**Why chat refeeds differently (do not "fix" this):** chat's card submit is thin
by policy — no `CompleteCard`, no graph effects, and no post-submit turn at all.
Manufacturing one so the coach can speak unprompted into a free chat is the
wrong posture for that surface. So the summary rides the student's NEXT message.
Zero additional LLM calls.

- [ ] **Step 1: Write the failing test**

```go
func TestBuildChatContextIncludesCompletedCardSummary(t *testing.T) {
	// Pass one completed-card summary string; assert it appears under its own
	// heading and that a thread with none renders byte-identically to today.
}

func TestRunChatStepFoldsCompletedCardsIntoContext(t *testing.T) {
	// A thread whose ScopedCard list has one "completed" instance with real
	// field_values. Assert the prompt the fake provider received contains the
	// card's name and a submitted answer.
}

func TestRunChatStepIgnoresIncompleteCardsInContext(t *testing.T) {
	// proposed/active/skipped cards contribute nothing to the context.
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/`
Expected: FAIL.

- [ ] **Step 3: Write the implementation**

`ScopedCard` gains two additive fields:

```go
type ScopedCard struct {
	ID     uuid.UUID
	CardID string
	Status string

	// FieldValues/Anchors carry the submitted card body so a COMPLETED card
	// can be refed into the coach's context on the student's next message
	// (N3b Seam B, chat variant). Additive and optional: zero values are
	// valid and every existing fixture keeps compiling.
	FieldValues []byte
	Anchors     []byte
}
```

`chatstore.go` and `coursestore.go` populate them from the rows they already
select (`ListCardInstancesByThread` / the course equivalent are both `SELECT *`).

Add to `chat_step.go`:

```go
// completedCardSummary serializes the thread's COMPLETED cards for the coach's
// context. Only completed cards contribute: a proposed or active card has
// nothing finished to say, and a skipped one is a decline we do not re-raise.
func completedCardSummary(cards []ScopedCard) string { /* SerializeCardForRefeed per card, joined */ }
```

`BuildChatContext(history []ChatTurn, threadSummary, cardSummary, flag string) string`
gains the block, emitted only when non-empty:

```
此对话中已完成的工具卡：<summary>
```

Update `ProposeChatReply` and its call site to thread the new argument through.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...`
Expected: PASS (full packages — shared signature change).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/chat_step.go apps/api/internal/agent/chat_coach.go apps/api/internal/agent/chatstore.go apps/api/internal/agent/coursestore.go apps/api/internal/agent/chat_coach_test.go apps/api/internal/agent/chat_step_test.go
git commit -m "feat(n3b): chat refeed — completed-card summaries ride the next turn"
```

---

### Task 7: The two carry-forwards

**Files:**
- Modify: `apps/api/internal/agent/classifier.go` (toulmin skip-suppression)
- Test: `apps/api/internal/agent/classifier_test.go` (append)
- Test: `apps/api/internal/agent/card_lifecycle_test.go` (append the guard test)

**Interfaces:** none produced.

- [ ] **Step 1: Write the failing tests**

```go
func TestSurfaceCardCandidatesToulminNotReofferedAfterSkip(t *testing.T) {
	// A graph with an evaluated material, NO claim node, and a toulmin
	// card_instance with status "skipped". Assert no toulmin candidate.
	// Today this re-offers forever: skip → no claim → the predicate stays true.
}

func TestNeedsMaterialCoversEveryGraphEffectKind(t *testing.T) {
	// Enumerate every effect kind GraphEffects' switch handles (read it and
	// list them literally). Assert each is present in materialConsumingEffects
	// with the RIGHT boolean — promote/cross_check consume a material, the
	// rest do not. The two switches have no compile-time link; this test is it.
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/`
Expected: the toulmin test FAILS; the guard test may pass immediately (it is a
regression net, not a bug fix — that is fine and expected, note it in the report).

- [ ] **Step 3: Write the implementation**

In `classifier.go`, mirror `perspective-matrix`'s any-status rule for toulmin:

```go
	// Toulmin skip-suppression (N3b): like perspective-matrix, ANY toulmin
	// card_instance — including a skipped one — retires the offer. Without
	// this, skipping toulmin mints no claim node, so `anyEvaluated &&
	// !hasClaim` stays true and the card re-offers forever. An offer is never
	// a wall (铁律 2 · 不操纵).
	toulminSeen := false
	for _, ci := range g.CardInstances {
		if ci.CardID == toulminCardID {
			toulminSeen = true
		}
	}
	if anyEvaluated && !hasClaim && !toulminSeen {
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...`
Expected: PASS.

**Watch:** the Slice-7 keystone `TestRefactor2CardsLoop_ToulminBuildsArgument`
exercises the toulmin surface path. If it breaks, the premise — not the fix — is
what to re-examine; report rather than weakening the assertion.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/classifier.go apps/api/internal/agent/classifier_test.go apps/api/internal/agent/card_lifecycle_test.go
git commit -m "fix(n3b): toulmin skip-suppression + a needsMaterial completeness guard"
```

---

### Task 8: End-to-end proof over the real stack

**Files:**
- Test: `apps/api/internal/agent/refactor2_cards_loop_sqlc_test.go` (append)

**Interfaces:** none produced.

**Why this task exists.** N3a's whole-branch review found a CRITICAL that every
per-task review structurally could not see: `CompleteCard` hard-failed on all
three new cards, because every existing test exercised the predicates as **pure
functions** and none went through the real lifecycle. Tasks 1-7 are again mostly
pure-function tests. This task closes that hole with one test over the real
(testcontainers) stack.

- [ ] **Step 1: Write the failing test**

```go
// TestRefactor2CardsLoop_SemanticMomentToRefeed walks N3b's whole loop against
// a real database:
//   1. a project whose graph yields NO structural surface_card candidate;
//   2. RunAgentStep with Trigger{Kind:"student_turn", StudentText: <a real
//      one-sided paragraph>} and a stubbed provider replying "one_sided";
//   3. assert a REAL card_instance row for "steelman" exists, proposed;
//   4. submit it completed with real field_values through the same path the
//      handler uses;
//   5. RunAgentStep with Trigger{Kind:"card_refeed", CardInstanceID: <it>};
//   6. assert the intervention row persisted anchors to that card_instance AND
//      that the prompt the provider received carried the submitted answers;
//   7. assert the classifier is NOT re-offered — a second student_turn with
//      the same text yields no second steelman instance (any-status
//      suppression over a REAL row, not a fixture).
func TestRefactor2CardsLoop_SemanticMomentToRefeed(t *testing.T) { /* ... */ }
```

Follow the existing tests in this file for container setup, seeding, and the
store adapter — do not invent a new harness.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/agent/`
Expected: FAIL initially (write it before wiring the assertions to real ids).

- [ ] **Step 3: Make it pass**

No production change should be needed. **If one is, that is a real defect Tasks
1-7 missed — report it before changing anything.**

- [ ] **Step 4: Run the FULL gate**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 -count=1 ./...`
Expected: all packages ok.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/refactor2_cards_loop_sqlc_test.go
git commit -m "test(n3b): E2E — semantic moment to card to refeed over the real stack"
```

---

## Final gate (controller-run, before the whole-branch review)

```bash
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 -count=1 ./...
cd apps/web && npm test && npx tsc --noEmit
cd packages/contracts && npm test && npx tsc --noEmit
```

`apps/web` and `packages/contracts` must be **unchanged** by this slice — run
them to prove it. `packages/contracts`' `tsc` reports exactly ONE known
pre-existing error, `test/interactionPrimitive.test.ts(49,12) TS2532`; anything
more is ours.
