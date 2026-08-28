# Lite 阅读报告 + 写作报告 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give both lite loops a real end-of-session report — colourful, short, shareable — replacing the two 「报告还在路上」 placeholders and making 「看报告」 true.

**Architecture:** One `atom_report` table, one generator, two projections. Stats are computed deterministically in Go; one flagship call produces 金句/收获/gains over a corpus assembled **only** from the student's own words. A poster component exports to PNG. An unguessable, revocable token exposes the report — and only the report — at a public route that renders before the auth gate.

**Tech Stack:** Go (`net/http`, `pgx`, sqlc **pinned `@v1.27.0`**, goose), PostgreSQL + testcontainers, React + Vite + TypeScript + Tailwind, vitest, Playwright. Two new lite-web deps: `html-to-image`, `qrcode`.

**Spec:** `docs/superpowers/specs/2026-08-29-lite-reports-design.md`

## Global Constraints

- **R4 — an exported picture must never put words she did not write under her name.** The corpus is assembled only from her own fields, **with leading-`>` lines stripped from student messages** (since sub-project A those carry sentences she pointed at *in the article*). Every model-returned quote must be a literal substring of that corpus or it is dropped.
- **R2 — 印记 selects the moments and shows them.** No confirm step. But what is shown is only her own words, selected: never a score, rank, streak, badge, or comparison to anyone (铁律②).
- **R1 — sharing is opt-in, unguessable, revocable, no expiry.** Token from `crypto/rand`, never the atom id. Revoking sets it NULL. The public payload is the report envelope and nothing else.
- **Never charge the provider twice for one thing.** `pg_advisory_xact_lock` **before** the provider call, re-check inside the same transaction. An unfinished atom generates nothing and stamps nothing.
- **Never break pro.** Changes under `apps/web/` must be additive and capability-gated. Pro's full suite must stay green.
- **印记 talks like a teacher** — real language, never clipped AI-shrug copy.
- **Tailwind `mk-*` tokens are bare CSS custom properties**, so `/NN` alpha syntax emits NO CSS. Use `color-mix()` or explicit tokens.
- **Fetch-on-mount uses `useAlive`** (`apps/lite-web/src/shared/useAlive.ts`), never a `useRef` latch plus a per-invocation `cancelled` flag (a StrictMode trap this codebase has already paid for).
- **A test of an unexported Go symbol goes in `*_internal_test.go` (`package api`)** — every existing reading/writing test file is `package api_test` and cannot see them.
- Go: `CGO_ENABLED=0`, `-timeout 1800s`. Run only your task's targeted tests; the controller runs full suites at the end. **Verify your `-run` pattern matched** by counting `=== RUN`/`--- PASS`. **Paste RAW output; never hand-count.**
- Never `git add -A`; stage the specific files you changed.

---

### Task 1: Migration 0105 + report queries

**Files:**
- Create: `apps/api/internal/store/migrations/0105_lite_reports.sql`
- Modify: `apps/api/internal/store/queries/atom.sql`
- Generated: `apps/api/internal/store/sqlc/*`

**Interfaces:**
- Produces: `atom.active_seconds int32`; `sqlc.AtomReport`; `GetAtomReport(ctx, atomID)`, `UpsertAtomReport(ctx, params{AtomID, Kind, Report []byte})`, `SetAtomReportShare(ctx, params{AtomID, ShareToken *string})`, `GetAtomReportByShareToken(ctx, token string)`, `AddAtomActiveSeconds(ctx, params{AtomID, Seconds int32})`.

- [ ] **Step 1: Write the migration**

```sql
-- +goose Up
-- 报告，以及它需要的那个「她到底待了多久」。
--
-- active_seconds 放在 atom 上而不是 reading/writing 上，和 last_activity_at 同一个
-- 理由：每一种原子都有「专注了多久」这件事，不是阅读的专属字段。默认 0 表示
-- 「这一条早于心跳存在」——报告端据此回退到时间戳估算，而不是显示 0 分钟。
ALTER TABLE atom ADD COLUMN active_seconds integer NOT NULL DEFAULT 0;

-- 一篇原子一份报告。share_token 为 NULL = 没有分享；撤回就是把它设回 NULL，
-- 于是「撤回」是真的：公开路由按 token 找行，下一次请求就什么也找不到了。
CREATE TABLE atom_report (
  atom_id     uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  kind        text NOT NULL CHECK (kind IN ('reading','writing')),
  report      jsonb NOT NULL,
  share_token text UNIQUE,
  shared_at   timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX atom_report_share_idx ON atom_report (share_token) WHERE share_token IS NOT NULL;

-- +goose Down
DROP TABLE atom_report;
ALTER TABLE atom DROP COLUMN active_seconds;
```

- [ ] **Step 2: Add the queries**

Append to `apps/api/internal/store/queries/atom.sql`:

```sql
-- name: GetAtomReport :one
SELECT * FROM atom_report WHERE atom_id = $1;

-- name: UpsertAtomReport :one
INSERT INTO atom_report (atom_id, kind, report)
VALUES ($1, $2, $3)
ON CONFLICT (atom_id) DO UPDATE SET report = EXCLUDED.report
RETURNING *;

-- name: SetAtomReportShare :one
UPDATE atom_report
SET share_token = sqlc.narg(share_token),
    shared_at = CASE WHEN sqlc.narg(share_token) IS NULL THEN NULL ELSE now() END
WHERE atom_id = sqlc.arg(atom_id)
RETURNING *;

-- name: GetAtomReportByShareToken :one
SELECT * FROM atom_report WHERE share_token = $1;

-- name: AddAtomActiveSeconds :exec
UPDATE atom SET active_seconds = active_seconds + $2 WHERE id = $1;
```

- [ ] **Step 3: Regenerate sqlc**

```bash
CGO_ENABLED=0 go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 generate -f apps/api/sqlc.yaml
```

The pin is mandatory. `emit_pointers_for_null_types: true` means `share_token`/`shared_at` generate as pointers.

- [ ] **Step 4: Verify**

Run: `cd apps/api && CGO_ENABLED=0 go build ./...`
Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/store/migrations/0105_lite_reports.sql apps/api/internal/store/queries/atom.sql apps/api/internal/store/sqlc
git commit -m "feat(lite): a place to keep a report, and a place to keep the minutes"
```

---

### Task 2: The heartbeat

**Files:**
- Create: `apps/api/internal/api/atom_heartbeat.go`
- Create: `apps/api/internal/api/atom_heartbeat_internal_test.go`
- Modify: `apps/api/internal/api/api.go`

**Interfaces:**
- Consumes: Task 1's `AddAtomActiveSeconds`.
- Produces: `POST /api/v1/readings/{id}/heartbeat` and `POST /api/v1/writings/{id}/heartbeat`, body `{"seconds": n}`, response `204`. Helper `clampHeartbeatSeconds(n int32) int32`.

- [ ] **Step 1: Write the failing test**

```go
func TestClampHeartbeatSeconds(t *testing.T) {
	cases := []struct{ in, want int32 }{
		{60, 60},
		{120, 120},
		{600, 120},  // a tab that slept, or a client lying — 120 is the ceiling
		{0, 0},
		{-5, 0},     // never subtract from her focus time
	}
	for _, c := range cases {
		if got := clampHeartbeatSeconds(c.in); got != c.want {
			t.Errorf("clampHeartbeatSeconds(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestClampHeartbeatSeconds -timeout 1800s`
Expected: FAIL — undefined.

- [ ] **Step 3: Implement**

```go
// heartbeatCeiling bounds ONE heartbeat's contribution. The client posts every
// 60s while the tab is visible, so a well-behaved call is 60. A call claiming
// more means the tab slept, the machine suspended, or the client is lying —
// none of which is focus time. 120 leaves room for a late post without
// letting an abandoned tab accrue an hour.
const heartbeatCeiling int32 = 120

func clampHeartbeatSeconds(n int32) int32 {
	if n < 0 {
		return 0
	}
	if n > heartbeatCeiling {
		return heartbeatCeiling
	}
	return n
}
```

Handler: `loadOwnedAtom` for the kind (which also bumps `last_activity_at`), decode `{seconds}`, clamp, `AddAtomActiveSeconds`, `204`. **No entitlement check** — this consumes no tokens and gating it would silently stop counting for a student whose entitlement lapses mid-session.

A heartbeat on a **finished** atom is accepted as a no-op (return 204 without adding) — she may reopen a finished reading to read it again, and that is not focus time on the work.

Register both routes with `liteOnly`, beside their siblings.

- [ ] **Step 4: Run**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run 'TestClampHeartbeat' -v -timeout 1800s`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/atom_heartbeat.go apps/api/internal/api/atom_heartbeat_internal_test.go apps/api/internal/api/api.go
git commit -m "feat(lite): count the minutes she was actually here"
```

---

### Task 3: The deterministic half — corpus, counts, and the time estimate

**Files:**
- Create: `apps/api/internal/api/report_facts.go`
- Create: `apps/api/internal/api/report_facts_internal_test.go`

**Interfaces:**
- Produces:
```go
type reportCorpus struct {
	Text  string            // everything she wrote, newline-joined — the substring oracle
	Where map[string]string // a fragment → a human label like "我的收获" / "第 2 段"
}
func buildReadingCorpus(takeaway string, notes []sqlc.AtomAnnotation, msgs []sqlc.AtomMessage, cards []sqlc.AtomCard) reportCorpus
func buildWritingCorpus(draft string, snippets []sqlc.WritingSnippet, outline []sqlc.WritingOutline, msgs []sqlc.AtomMessage) reportCorpus
func stripQuotedLines(s string) string
func countWordsForLang(text, lang string) int
func cappedGapSeconds(stamps []time.Time, capPerGap time.Duration) int
```

- [ ] **Step 1: Write the failing tests**

```go
// R4's enforcement point. Since sub-project A a student message carries the
// sentences she POINTED AT in the article as leading "> " lines. They live in
// a row whose role says 'student' but they are the ARTICLE's words, and an
// exported picture must never put them under her name.
func TestStripQuotedLines(t *testing.T) {
	in := "> 中国的碳排放总量位居世界第一。\n> 但人均排放仍低于多数发达国家。\n\n我觉得人均更能说明责任。"
	got := stripQuotedLines(in)
	if strings.Contains(got, "碳排放总量位居世界第一") {
		t.Error("an article sentence survived into her corpus")
	}
	if !strings.Contains(got, "我觉得人均更能说明责任") {
		t.Error("her own sentence was stripped")
	}
}

func TestCountWordsForLang(t *testing.T) {
	// The counter MUST branch on lang. This repo has already shipped a count
	// that counted characters for English and was ~5x wrong.
	cases := []struct {
		name, text, lang string
		want             int
	}{
		{"zh counts characters", "人均排放更能说明责任", "zh", 10},
		{"zh ignores spaces and punctuation", "人均排放，更能说明责任。", "zh", 10},
		{"en counts words", "Per capita emissions tell a fairer story", "en", 7},
		{"en collapses runs of spaces", "one   two\nthree", "en", 3},
		{"empty", "   ", "zh", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := countWordsForLang(c.text, c.lang); got != c.want {
				t.Errorf("countWordsForLang(%q,%s) = %d, want %d", c.text, c.lang, got, c.want)
			}
		})
	}
}

func TestCappedGapSeconds(t *testing.T) {
	base := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	at := func(m int) time.Time { return base.Add(time.Duration(m) * time.Minute) }
	// 0→2 (2m), 2→5 (3m), 5→45 (capped to 5m), 45→46 (1m) = 11m
	got := cappedGapSeconds([]time.Time{at(0), at(2), at(5), at(45), at(46)}, 5*time.Minute)
	if got != 11*60 {
		t.Errorf("cappedGapSeconds = %d, want %d", got, 11*60)
	}
	if cappedGapSeconds([]time.Time{at(0)}, 5*time.Minute) != 0 {
		t.Error("a single event is not a duration")
	}
	if cappedGapSeconds(nil, 5*time.Minute) != 0 {
		t.Error("no events is not a duration")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run 'TestStripQuotedLines|TestCountWordsForLang|TestCappedGapSeconds' -timeout 1800s`
Expected: FAIL — undefined.

- [ ] **Step 3: Implement**

`stripQuotedLines`: split on `\n`, drop every line whose `strings.TrimSpace` starts with `>`, rejoin, trim.

`countWordsForLang`: for `en`, `len(strings.Fields(text))`. For anything else (zh is the default), count runes that are **not** whitespace and not punctuation — use `unicode.IsSpace` and `unicode.IsPunct`, and treat CJK fullwidth punctuation (，。！？；：、「」『』（）) via `unicode.IsPunct` plus an explicit set for the ones Go does not classify. Test the fixtures above; they are the contract.

`cappedGapSeconds`: sort, sum `min(gap, cap)` over consecutive pairs, return seconds.

`buildReadingCorpus`: join, in this order and each with a `Where` label — the takeaway (「我的收获」), each annotation's `note` (「批注」), each student message's `stripQuotedLines(content)` (「和印记聊的时候」), and each **submitted** card's `field_values` string leaves (「读的时候记下的」). **Never** touch `framework_fill`, `anchors[].quote`, or `atom_annotation.quote` — those are AI's or the article's.

`buildWritingCorpus`: the draft body (「成稿」), each snippet (「第 N 段」), each outline node's `text` (「提纲」), each student message stripped (「和印记聊的时候」). **Never** `role`, `guide`, or any `writing_comment` field.

Add a doc comment on each builder listing, explicitly, which fields are excluded and why — this is the file where R4 is either kept or lost.

- [ ] **Step 4: Run**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run 'TestStripQuotedLines|TestCountWordsForLang|TestCappedGapSeconds|TestBuildReadingCorpus|TestBuildWritingCorpus' -v -timeout 1800s`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/report_facts.go apps/api/internal/api/report_facts_internal_test.go
git commit -m "feat(lite): the facts a report may state, and only her words in the corpus"
```

---

### Task 4: The generator and the two GET endpoints

**Files:**
- Create: `apps/api/internal/api/atom_report.go`
- Create: `apps/api/internal/api/atom_report_internal_test.go`
- Create: `apps/api/internal/api/atom_report_test.go`
- Modify: `apps/api/internal/api/api.go`

**Interfaces:**
- Consumes: Tasks 1 and 3.
- Produces: `GET /api/v1/readings/{id}/report`, `GET /api/v1/writings/{id}/report` → `{"report": <LiteReport>|null}`; `validateMoments(ms []reportMoment, corpus string) []reportMoment`.

- [ ] **Step 1: Write the failing validator test**

```go
func TestValidateMoments(t *testing.T) {
	corpus := "我觉得人均排放更能说明责任。\n作者只讲了总量，没有讲人口。"
	got := validateMoments([]reportMoment{
		{Quote: "人均排放更能说明责任", Where: "我的收获"},
		{Quote: "她展现了批判性思维", Where: "我的收获"},   // AI's own prose — dropped
		{Quote: "中国碳排放世界第一", Where: "批注"},       // the article's — dropped
		{Quote: "  ", Where: "批注"},                    // empty — dropped
	}, corpus)
	if len(got) != 1 {
		t.Fatalf("kept %d, want 1: %+v", len(got), got)
	}
	if got[0].Quote != "人均排放更能说明责任" {
		t.Errorf("kept the wrong moment: %+v", got[0])
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestValidateMoments -timeout 1800s`
Expected: FAIL.

- [ ] **Step 3: Implement**

`validateMoments`: trim; drop empty; drop any quote that is not `strings.Contains(corpus, quote)`; cap at 3.

Handler `getAtomReportFor(kind string) http.HandlerFunc` (curried like `liteListMessagesFor`):

1. `loadOwnedAtom`; `HasEntitlement`.
2. **Finished gate.** Load the `reading`/`writing` row; if `status != "finished"`, write `{"report": null}` and return — no generation, no lock, no stamp.
3. Cheap read: `GetAtomReport`; if found, return it.
4. Open a transaction; `pg_advisory_xact_lock(hashtextextended(atom_id::text || ':report', 0))`; **re-run `GetAtomReport` inside the transaction** and return it if a racer won.
5. Assemble the deterministic half: stats (per the spec's per-kind list), `keep` from her own takeaway (reading) or nil, `studentName`, `title`, `finishedAt`. Time = `active_seconds` if > 0, else `cappedGapSeconds` over the event stamps.
6. Build the corpus (Task 3). **One** `a.d.EvalResolver` call via `gateway.Collect` returning `{moments:[{quote,where}], keep, gains:[]}`; `recordLiteLLMCall(..., "lite_report", ...)`.
7. Validate moments against the corpus. **Best-effort:** if the call errors or nothing survives, keep going with the deterministic half — a report is never blocked on prose.
8. `UpsertAtomReport`; commit; return.

Prompt: 印记 writing to the student about her own session. Name real things she did. No score, no grade, no comparison, no praise inflation. `gains` are 2–4 short lines. Quotes must be copied **verbatim** from the corpus. Teacher's voice per the standing rule.

- [ ] **Step 4: Add the two integration tests**

```go
// An unfinished atom must generate nothing and stamp nothing — the lesson
// sub-project A paid for: a generate-if-absent endpoint reachable early
// burns its one generation on partial data, permanently.
func TestReportUnfinishedGeneratesNothing(t *testing.T) { /* assert provider count 0, report null, no row */ }

// Two concurrent first-opens cost exactly one provider call.
func TestReportChargesOnce(t *testing.T) { /* assert fake provider count == 1 */ }
```

Read `apps/api/internal/api/writing_race_test.go` and copy its shape; assert on the fake provider's **call count**, never on timing.

- [ ] **Step 5: Run**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run 'TestValidateMoments|TestReport' -v -timeout 1800s`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/atom_report.go apps/api/internal/api/atom_report_internal_test.go apps/api/internal/api/atom_report_test.go apps/api/internal/api/api.go
git commit -m "feat(lite): a report built from what she actually did"
```

---

### Task 5: Sharing — mint, revoke, and the public route

**Files:**
- Create: `apps/api/internal/api/atom_report_share.go`
- Create: `apps/api/internal/api/atom_report_share_test.go`
- Modify: `apps/api/internal/api/api.go`

**Interfaces:**
- Produces: `POST /api/v1/readings/{id}/report/share` and the writing twin → `{"token","url"}`; `DELETE` on the same paths → `204`; `GET /api/v1/public/reports/{token}` → `{"report": …}`, **no auth**.

- [ ] **Step 1: Write the failing tests**

```go
func TestShareTokenIsUnguessable(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		tok, err := newShareToken()
		if err != nil { t.Fatal(err) }
		if len(tok) != 32 { t.Fatalf("token %q is %d chars, want 32", tok, len(tok)) }
		if seen[tok] { t.Fatal("newShareToken repeated a token") }
		seen[tok] = true
	}
}

// Revocation must be REAL: the next request must not resolve the old link.
func TestRevokedShareIs404(t *testing.T) { /* share → GET public 200 → DELETE → GET public 404 */ }

// The public payload is the report envelope and nothing else — no account,
// no transcript, no article, no draft body, no ids addressing anything else.
func TestPublicPayloadCarriesNothingExtra(t *testing.T) {
	// decode into map[string]any and assert the key set is exactly {"report"},
	// and that the report object's keys are exactly the envelope's fields.
}
```

- [ ] **Step 2: Run to verify failure**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run 'TestShareToken|TestRevokedShare|TestPublicPayload' -timeout 1800s`
Expected: FAIL.

- [ ] **Step 3: Implement**

```go
// newShareToken mints 16 random bytes as 32 hex chars. NOT the atom id: an id
// is guessable from any other link she has ever shared, and it addresses a row
// she owns. This token addresses one report and nothing else.
func newShareToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
```

Share handler: owned + finished + report exists (generate first if absent by calling the same path Task 4 uses — do not duplicate the generator); mint; `SetAtomReportShare`; return token and the absolute public URL. Re-sharing an already-shared report returns the existing token rather than minting a second (one link per report, so a link she has already sent stays valid).

Revoke handler: `SetAtomReportShare(atomID, nil)`; `204`. Idempotent.

Public handler: `GetAtomReportByShareToken`; `pgx.ErrNoRows` → `404` (plain, no detail — an unknown token and a revoked token must be indistinguishable). Return **only** `{"report": <the stored jsonb>}`.

Register it **without** `protected`/`liteOnly`:
```go
mux.Handle("GET /api/v1/public/reports/{token}", http.HandlerFunc(a.getPublicReport))
```
following the same bare-registration shape the admin-key routes use.

- [ ] **Step 4: Run**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run 'TestShareToken|TestRevokedShare|TestPublicPayload' -v -timeout 1800s`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/atom_report_share.go apps/api/internal/api/atom_report_share_test.go apps/api/internal/api/api.go
git commit -m "feat(lite): a link she can take down"
```

---

### Task 6: The lite-web API client

**Files:**
- Create: `apps/lite-web/src/api/reports.ts`
- Create: `apps/lite-web/test/reportsApi.test.ts`

**Interfaces:**
- Produces: `type LiteReport` (the spec's envelope), `getReport(kind,id)`, `shareReport(kind,id)`, `unshareReport(kind,id)`, `getPublicReport(token)`, `sendHeartbeat(kind,id,seconds)`.

- [ ] **Step 1: Write the failing test**

Assert `getReport("reading", id)` calls `/api/v1/readings/{id}/report` and returns `null` when the server sends `{"report": null}`; `getPublicReport(tok)` calls `/api/v1/public/reports/{tok}`; `unshareReport` issues a DELETE.

- [ ] **Step 2: Run to verify failure**

Run: `cd apps/lite-web && pnpm vitest run test/reportsApi.test.ts`
Expected: FAIL.

- [ ] **Step 3: Implement**

Mirror the shape of `apps/lite-web/src/api/readingRoom.ts` — same `apiFetch`, same `?? null` defaulting. `kind` is `"reading" | "writing"` and maps to the `readings`/`writings` path segment.

- [ ] **Step 4: Run**

Run: `cd apps/lite-web && pnpm vitest run test/reportsApi.test.ts && pnpm typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/lite-web/src/api/reports.ts apps/lite-web/test/reportsApi.test.ts
git commit -m "feat(lite): the report, from the client's side"
```

---

### Task 7: `<ReportView>` — the report itself

**Files:**
- Create: `apps/lite-web/src/reports/ReportView.tsx`
- Create: `apps/lite-web/test/reportView.test.tsx`

**Interfaces:**
- Consumes: Task 6's `LiteReport`.
- Produces: `<ReportView report={LiteReport} />` — pure presentation, no fetching, no share controls (Task 10 adds those around it).

- [ ] **Step 1: Write the failing test**

```tsx
it("renders a thin report without breaking", () => {
  // one stat, no moments, no keep, no gains
  // assert: the stat is visible; no empty headings; nothing throws
});
it("shows each 金句 and where it came from", () => { /* … */ });
it("never renders a score, grade, rank or comparison", () => {
  // assert the rendered text matches none of /分数|评分|等级|排名|超过|第\s*\d+\s*名/
});
```

The last test is 铁律② as an assertion. Keep it.

- [ ] **Step 2: Run to verify failure**

Run: `cd apps/lite-web && pnpm vitest run test/reportView.test.tsx`
Expected: FAIL.

- [ ] **Step 3: Implement**

A warm poster, not a dashboard: title and date at the top; the stats as **large numerals** with small labels in a row that wraps; 金句 as pull-quotes with generous leading, each with its small 「来自：我的收获」 label; her 收获 quoted as hers; `gains` as 2–4 short lines. Every section renders `null` when its data is absent — no empty headings, no "暂无" placeholders.

Colour: use the existing `mk-` accent tokens. **No `/NN` alpha syntax on `mk-` tokens** — it emits no CSS. Use `color-mix()` for tints.

- [ ] **Step 4: Run**

Run: `cd apps/lite-web && pnpm vitest run test/reportView.test.tsx && pnpm typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/lite-web/src/reports/ReportView.tsx apps/lite-web/test/reportView.test.tsx
git commit -m "feat(lite): the report, as something she'd want to show someone"
```

---

### Task 8: Both finished screens show it

**Files:**
- Create: `apps/lite-web/src/reports/ReportPanel.tsx`
- Modify: `apps/lite-web/src/readings/ReadingRoomHost.tsx`
- Modify: `apps/lite-web/src/writings/WritingRoomHost.tsx`
- Create: `apps/lite-web/test/reportPanel.test.tsx`

**Interfaces:**
- Produces: `<ReportPanel kind={"reading"|"writing"} atomId={string} />` — fetches on mount with `useAlive`, renders `<ReportView>`, and owns the loading/absent states.

- [ ] **Step 1: Write the failing test**

```tsx
it("replaces the placeholder once the report arrives", async () => { /* … */ });
it("says something honest while the report is being written", async () => {
  // a never-resolving fetch → a waiting line, not a blank panel and not an error
});
it("renders nothing loud when the server says null", async () => { /* … */ });
```

- [ ] **Step 2: Run to verify failure**

Run: `cd apps/lite-web && pnpm vitest run test/reportPanel.test.tsx`
Expected: FAIL.

- [ ] **Step 3: Implement**

Fetch with `useAlive` — **not** a `useRef` latch plus a per-invocation `cancelled` flag.

The first open generates the report, which is a flagship call and can take tens of seconds. The waiting copy must say so like a person would — 「印记正在把这次读的东西整理成一份报告，稍等一下」 — never a bare spinner and never 「加载中」.

In `ReadingRoomHost`'s `FinishedReadingPanel`, **replace** the line 「这次阅读的报告还在路上。…」 with `<ReportPanel kind="reading" atomId={readingId} />`. Same for `FinishedWritingPanel` and 「这篇写作的报告还在路上。…」. These two placeholder lines are the thing this whole sub-project exists to remove — they go now.

- [ ] **Step 4: Run**

Run: `cd apps/lite-web && pnpm vitest run test/reportPanel.test.tsx test/readingRoomHost.test.tsx && pnpm typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/lite-web/src/reports/ReportPanel.tsx apps/lite-web/src/readings/ReadingRoomHost.tsx apps/lite-web/src/writings/WritingRoomHost.tsx apps/lite-web/test/reportPanel.test.tsx
git commit -m "feat(lite): 报告还在路上 is no longer true, so it no longer says so"
```

---

### Task 9: The heartbeat, from the room

**Files:**
- Create: `apps/lite-web/src/shared/useHeartbeat.ts`
- Modify: `apps/lite-web/src/readings/ReadingRoomHost.tsx`
- Modify: `apps/lite-web/src/writings/WritingRoomHost.tsx`
- Create: `apps/lite-web/test/useHeartbeat.test.tsx`

**Interfaces:**
- Produces: `useHeartbeat(kind, atomId, enabled: boolean)`.

- [ ] **Step 1: Write the failing test**

```tsx
it("posts once per interval while the tab is visible", async () => { /* fake timers */ });
it("posts nothing while the tab is hidden", async () => {
  // set document.visibilityState to "hidden", advance timers, assert no calls
});
it("posts nothing when disabled (a finished atom)", async () => { /* … */ });
it("stops on unmount", async () => { /* … */ });
```

- [ ] **Step 2: Run to verify failure**

Run: `cd apps/lite-web && pnpm vitest run test/useHeartbeat.test.tsx`
Expected: FAIL.

- [ ] **Step 3: Implement**

`setInterval` at 60s; each tick posts `{seconds: 60}` only when `document.visibilityState === "visible"` and `enabled`. Clear on unmount. Swallow errors — a failed heartbeat must never surface to her or interrupt anything.

Wire into both hosts with `enabled` = the atom is loaded and **not** finished.

- [ ] **Step 4: Run**

Run: `cd apps/lite-web && pnpm vitest run test/useHeartbeat.test.tsx test/readingRoomHost.test.tsx && pnpm typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/lite-web/src/shared/useHeartbeat.ts apps/lite-web/src/readings/ReadingRoomHost.tsx apps/lite-web/src/writings/WritingRoomHost.tsx apps/lite-web/test/useHeartbeat.test.tsx
git commit -m "feat(lite): the minutes only count while she is actually here"
```

---

### Task 10: Sharing, with a QR code

**Files:**
- Create: `apps/lite-web/src/reports/SharePanel.tsx`
- Modify: `apps/lite-web/src/reports/ReportPanel.tsx`
- Modify: `apps/lite-web/package.json` (add `qrcode`)
- Create: `apps/lite-web/test/sharePanel.test.tsx`

**Interfaces:**
- Consumes: Task 6's `shareReport`/`unshareReport`.

- [ ] **Step 1: Add the dependency**

```bash
cd apps/lite-web && pnpm add qrcode && pnpm add -D @types/qrcode
```

- [ ] **Step 2: Write the failing test**

```tsx
it("shows no link and no QR until she turns sharing on", async () => { /* … */ });
it("shows the link and a QR image once shared", async () => { /* … */ });
it("停止分享 removes the link and the QR", async () => { /* … */ });
```

- [ ] **Step 3: Run to verify failure**

Run: `cd apps/lite-web && pnpm vitest run test/sharePanel.test.tsx`
Expected: FAIL.

- [ ] **Step 4: Implement**

Off state: one line explaining plainly what sharing does — that anyone with the link can open the report, and that she can take it down at any time — and a 生成分享链接 button. On state: the URL with a 复制 button, a QR rendered via `QRCode.toDataURL(url)`, and 停止分享.

Write the copy like a person explaining a real consequence to a student, not like a consent dialog. She is a minor deciding to publish her own work; the sentence should be honest and calm.

- [ ] **Step 5: Run**

Run: `cd apps/lite-web && pnpm vitest run test/sharePanel.test.tsx && pnpm typecheck`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/lite-web/src/reports/SharePanel.tsx apps/lite-web/src/reports/ReportPanel.tsx apps/lite-web/package.json apps/lite-web/test/sharePanel.test.tsx pnpm-lock.yaml
git commit -m "feat(lite): a QR she can show someone, and take back"
```

---

### Task 11: The exported picture

**Files:**
- Create: `apps/lite-web/src/reports/ReportPoster.tsx`
- Create: `apps/lite-web/src/reports/exportPoster.ts`
- Modify: `apps/lite-web/src/reports/ReportPanel.tsx`
- Modify: `apps/lite-web/package.json` (add `html-to-image`)
- Create: `apps/lite-web/test/exportPoster.test.tsx`

- [ ] **Step 1: Add the dependency**

```bash
cd apps/lite-web && pnpm add html-to-image
```

- [ ] **Step 2: Write the failing test**

```tsx
it("produces a PNG data URL of non-trivial length", async () => {
  // mock html-to-image's toPng to assert it is called with the poster node
  // and the expected pixel size, and assert the download is triggered with
  // a filename containing the title
});
it("the poster carries her name, the date, the stats and at most three 金句", () => { /* … */ });
```

Assert on the produced data URL's shape and length — **not** merely that the button did not throw.

- [ ] **Step 3: Run to verify failure**

Run: `cd apps/lite-web && pnpm vitest run test/exportPoster.test.tsx`
Expected: FAIL.

- [ ] **Step 4: Implement**

`<ReportPoster>` renders at a fixed 1080×1440 in a container positioned offscreen (`position:fixed; left:-99999px`), **not** `display:none` — a hidden node has no layout and rasterizes empty.

**System font stack only.** The export rasterizes through an SVG `foreignObject`, which does not load external fonts; a web font would silently fall back and produce a picture that differs from the preview.

Content: her name, the title, the date, the four stats, up to three 金句, and nothing else. `exportPoster(node, filename)` calls `toPng(node, { pixelRatio: 2, cacheBust: true })` and triggers a download.

- [ ] **Step 5: Run**

Run: `cd apps/lite-web && pnpm vitest run test/exportPoster.test.tsx && pnpm typecheck && pnpm build`
Expected: PASS and a clean build (the new dep must not break the bundle).

- [ ] **Step 6: Commit**

```bash
git add apps/lite-web/src/reports/ReportPoster.tsx apps/lite-web/src/reports/exportPoster.ts apps/lite-web/src/reports/ReportPanel.tsx apps/lite-web/package.json apps/lite-web/test/exportPoster.test.tsx pnpm-lock.yaml
git commit -m "feat(lite): a picture worth showing someone"
```

---

### Task 12: The public page, before the auth gate

**Files:**
- Create: `apps/lite-web/src/reports/PublicReportPage.tsx`
- Modify: `apps/lite-web/src/routing.ts`
- Modify: `apps/lite-web/src/LiteApp.tsx`
- Create: `apps/lite-web/test/publicReport.test.tsx`

- [ ] **Step 1: Write the failing test**

```tsx
it("renders a shared report with no session at all", async () => {
  // getMe mocked to REJECT (no session); pathname "/s/<token>"
  // assert the report renders and AuthScreen never appears
});
it("says the link is no longer available on a 404", async () => { /* … */ });
```

The first test is the whole point of this task: today `LiteApp` calls `getMe()` and renders `AuthScreen` **before any route is inspected**, so a visitor with a valid link would be shown a sign-in screen.

- [ ] **Step 2: Run to verify failure**

Run: `cd apps/lite-web && pnpm vitest run test/publicReport.test.tsx`
Expected: FAIL.

- [ ] **Step 3: Implement**

Add `{ tab: "share"; token: string }` to `LiteRoute`, parsed from `/s/:token`.

In `LiteApp`, **before** the `getMe()` effect and before any auth branch, check `parseLiteRoute(window.location.pathname)` for the share route and return `<PublicReportPage token={…} />` immediately. Put a comment there saying why the order matters, because it is the kind of thing a later refactor tidies away.

The page renders `<ReportView>` plus a quiet footer line naming the product. On 404: a plain sentence that the link is no longer available — no error styling, no stack, no invitation to sign in. Nothing on this page links into the app's authenticated surfaces.

- [ ] **Step 4: Run**

Run: `cd apps/lite-web && pnpm vitest run test/publicReport.test.tsx test/liteAppAuth.test.tsx test/routing.test.ts && pnpm typecheck`
Expected: PASS. `liteAppAuth` and `routing` are the pre-existing suites this task can break — they must stay green.

- [ ] **Step 5: Commit**

```bash
git add apps/lite-web/src/reports/PublicReportPage.tsx apps/lite-web/src/routing.ts apps/lite-web/src/LiteApp.tsx apps/lite-web/test/publicReport.test.tsx
git commit -m "feat(lite): a shared report opens for someone with no account"
```

---

### Task 13: 「看报告」 becomes true

**Files:**
- Modify: `apps/lite-web/src/readings/ReadingHistoryPanel.tsx`
- Modify: `apps/lite-web/src/writings/WritingHistoryPanel.tsx`
- Modify: `apps/lite-web/test/readingsLanding.test.tsx` (or the nearest covering suite)

- [ ] **Step 1: Confirm what the labels do today**

Both drawers label a finished row 「看报告」 and route to the same room path, which resolves to the finished panel. After Task 8 that panel contains the report — so the label is now honest and the only work is confirming it, plus adding a test that pins it.

- [ ] **Step 2: Write the test**

Assert that selecting a finished reading from the drawer routes to `/readings/:id`, and that the finished panel mounts the report panel. If the existing suites already cover the routing half, add only the missing half.

- [ ] **Step 3: Run**

Run: `cd apps/lite-web && pnpm vitest run test/readingsLanding.test.tsx test/reportPanel.test.tsx`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git commit -m "test(lite): 看报告 now goes to a report"
```

---

### Task 14: e2e — finish, report, share, open cold, revoke

**Files:**
- Create: `apps/lite-web/e2e/report-walk.spec.ts`

- [ ] **Step 1: Write the spec**

Follow the existing e2e files' conventions (`coach-walk.spec.ts`, `reading-walk.spec.ts`). One walk:

1. finish a reading (reuse the existing helpers);
2. the report appears where the placeholder used to be;
3. turn sharing on — the link and the QR appear;
4. **open the share URL in a fresh browser context with no session** and see the report. This is the assertion that matters: `browser.newContext()`, not a new tab in the signed-in one, or the test proves nothing about public access;
5. revoke; the same URL now shows the unavailable line.

- [ ] **Step 2: Run**

Run: `cd apps/lite-web && pnpm exec playwright test -c e2e/playwright.config.ts report-walk.spec.ts`
Expected: PASS. If a pre-existing spec fails for unrelated reasons, report it — do not edit it into passing.

- [ ] **Step 3: Commit**

```bash
git add apps/lite-web/e2e/report-walk.spec.ts
git commit -m "test(lite): walk a report from finish to a link someone else opens"
```

---

## Final verification (controller)

- `cd apps/api && CGO_ENABLED=0 go test ./... -timeout 1800s`
- `cd apps/lite-web && pnpm test && pnpm typecheck && pnpm build`
- `cd apps/web && pnpm test && pnpm typecheck && pnpm build` — pro must be green.
- Update `docs/2026-08-28-lite-edition-feedback-backlog.md`: tick C1–C6 and D1–D5, link this spec and plan, and strike the 「看报告」 dead-label line from "Also found on the walk".
