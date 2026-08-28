package api

// atom_report.go — the end-of-session report: a finished reading or writing,
// turned into the picture a student sees of what she actually did.
//
// One generator, two projections (GET /readings/{id}/report and
// GET /writings/{id}/report), the same "generate if absent, otherwise
// serve" shape S8/postWritingOpening pioneered in this codebase: a cheap
// pre-check outside any transaction, then — only if absent —
// pg_advisory_xact_lock, a RE-CHECK under that lock, and only then the one
// model call. ensureAtomReport is the WHOLE body, not just the HTTP
// handler's half of it, because Task 5's share endpoint must generate a
// report too and a second copy of this logic is exactly the drift this
// design forbids.
//
// The deterministic half (stats, the corpus, the time estimate) comes
// straight from report_facts.go — that file's doc comment is R4's
// enforcement point and this file must never widen what it feeds the model.
// The one model call returns only `moments` and `gains`: `keep` is NEVER
// asked of the model — for a reading it is her `reading_takeaway.text`
// verbatim (she wrote it; it *is* "one thing to take away"), and for a
// writing it is null. That ruling removes an entire validation surface: R4
// holds for `keep` by construction, not by checking.
//
// Best-effort prose: if the model call errors, or nothing survives
// validateMoments, the report is still built and stored from the
// deterministic half alone. A student who finished her work always gets her
// stats and her own 收获 — a report must never be blocked on prose.

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// --- the report envelope (v1) ------------------------------------------

// reportStat is one big numeral on the report: "专注时长 12 分钟", rendered
// from {label, value, unit}. key is a stable machine name for the client.
type reportStat struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value int    `json:"value"`
	Unit  string `json:"unit"`
}

// reportMoment is one 金句: a literal quote of HER OWN words (validateMoments
// is what makes that a guarantee, not a hope) plus a short label of where it
// came from. Doubles as the model's raw reply shape — no extra field
// distinguishes "proposed" from "kept", so no separate draft type earns its
// keep here the way readingQuestionDraft does for a shape with more fields.
type reportMoment struct {
	Quote string `json:"quote"`
	Where string `json:"where"`
}

// reportKeep is 「这次最值得记住的」 — deterministic, never model output. See
// the file comment: for a reading it is her takeaway verbatim; a writing has
// none (its 金句 carry that weight instead).
type reportKeep struct {
	Label string `json:"label"`
	Text  string `json:"text"`
}

// liteReportDTO is the stored+served shape, matching the design spec's
// LiteReport v1 type field-for-field. Moments/Gains use omitempty: a section
// that produced nothing is ABSENT, never an empty array — "a thin session
// shows a thin report" per the spec, and the client renders each section
// independently.
type liteReportDTO struct {
	Version     int            `json:"version"`
	Kind        string         `json:"kind"`
	Title       string         `json:"title"`
	StudentName string         `json:"studentName"`
	FinishedAt  string         `json:"finishedAt"`
	Stats       []reportStat   `json:"stats"`
	Moments     []reportMoment `json:"moments,omitempty"`
	Keep        *reportKeep    `json:"keep"`
	Gains       []string       `json:"gains,omitempty"`
}

// --- validation (R4) -----------------------------------------------------

// validateMoments keeps a proposed moment only if its quote, trimmed, is a
// literal substring of corpus — the same "a guarantee you can check beats
// one you asked for" shape as validateReadingQuestions
// (reading_questions.go). An empty quote (including whitespace-only) is
// dropped before the substring check even runs. Capped at 3: the report is a
// poster, not a transcript.
func validateMoments(ms []reportMoment, corpus string) []reportMoment {
	out := make([]reportMoment, 0, len(ms))
	for _, m := range ms {
		quote := strings.TrimSpace(m.Quote)
		if quote == "" || !strings.Contains(corpus, quote) {
			continue
		}
		out = append(out, reportMoment{Quote: quote, Where: strings.TrimSpace(m.Where)})
		if len(out) == 3 {
			break
		}
	}
	return out
}

// cleanGains trims each line, drops empties, and caps at 4 — "2-4 short
// lines" per the spec. Fewer than 2 is still shown; a thin session is the
// honest outcome, not an error.
func cleanGains(gains []string) []string {
	out := make([]string, 0, len(gains))
	for _, g := range gains {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		out = append(out, g)
		if len(out) == 4 {
			break
		}
	}
	return out
}

// --- the one model call ---------------------------------------------------

// liteReportSystem — 印记 writing to the student about her own session. No
// score, no grade, no rank, no comparison to anyone, no praise inflation
// (铁律②): this is a record of what she did, not a verdict on it. Voice per
// the standing rule — real specifics, never a clipped AI-shrug line.
const liteReportSystem = `你是"印记"。学生刚完成了一次阅读或写作，你要为这次学习写一份记录——
不是打分，不是排名，也不是和任何人比较，只是如实说说她这次做了什么、往前走了
哪一步。

给你的材料是她自己写下的所有文字：她的收获、她的批注、她和你聊天时说的话、她
记下的笔记或写的段落。除了这些材料里的原句，别的话都不算她说的。

你要做两件事：

1. moments：从材料里挑出最多 3 句她自己的原话——**逐字复制**，不要改写、不要
   翻译、不要加标点、不要把两句拼成一句。配一句极短的说明，交代这是她在做什么
   的时候说的（比如"写论证的时候""读到关键段落时""和你商量怎么开头的时候"）。
   挑真正有想法、有判断的句子，不要挑她随手打的字或者客套话。挑不出来就留空，
   不要硬凑。
2. gains：用 2-4 句话说说她这次真正做到了什么、用了什么方法、想清楚了什么问
   题——要具体，要说得出名字，不要说"她表现很好""很棒"这种空话，也绝对不要打
   分、不要暗示名次、不要和任何别的学生比。像一个老师在记录一个学生真实发生的
   进步，不是在写一封表扬信。

只输出一个 JSON 对象：
{"moments":[{"quote":"...","where":"..."}],"gains":["...","..."]}

不要输出对象以外的任何文字或代码块标记。`

// buildReportPrompt hands the model the one thing it is allowed to draw
// moments from: corpus.Text, exactly as report_facts.go assembled it (her
// own words only, in fragment order). Nothing from the article, nothing
// AI-authored, is reachable here — see report_facts.go's file comment.
func buildReportPrompt(kind, title string, corpus reportCorpus) string {
	var b strings.Builder
	b.WriteString("类型：")
	if kind == "writing" {
		b.WriteString("写作")
	} else {
		b.WriteString("阅读")
	}
	b.WriteString("\n")
	if t := strings.TrimSpace(title); t != "" {
		b.WriteString("题目：" + t + "\n")
	}
	b.WriteString("\n【她自己写下的材料】\n")
	b.WriteString(corpus.Text)
	b.WriteString("\n")
	return b.String()
}

type reportModelReply struct {
	Moments []reportMoment `json:"moments"`
	Gains   []string       `json:"gains"`
}

// parseReportReply decodes the model's JSON object, tolerating the same
// code-fence wrapping parseReadingQuestionsReply (reading_questions.go)
// tolerates.
func parseReportReply(text string) (reportModelReply, bool) {
	c := strings.TrimSpace(text)
	if strings.HasPrefix(c, "```json") {
		c = strings.TrimLeft(strings.TrimPrefix(c, "```json"), " \t\r\n")
	} else if strings.HasPrefix(c, "```") {
		c = strings.TrimLeft(c[3:], " \t\r\n")
	}
	if strings.HasSuffix(c, "```") {
		c = strings.TrimRight(c[:len(c)-3], " \t\r\n")
	}
	if i := strings.IndexByte(c, '{'); i > 0 {
		c = c[i:]
	}
	if j := strings.LastIndexByte(c, '}'); j >= 0 && j < len(c)-1 {
		c = c[:j+1]
	}
	var got reportModelReply
	if err := json.Unmarshal([]byte(strings.TrimSpace(c)), &got); err != nil {
		return reportModelReply{}, false
	}
	return got, true
}

// generateReportProse is the ONE flagship call this whole file makes: it
// asks for moments+gains together (one editorial judgment over one corpus,
// not three calls for three fields) and is best-effort throughout — every
// failure path returns (nil, nil) rather than an error, because a report is
// never blocked on prose (see the file comment).
func (a *API) generateReportProse(ctx context.Context, userID, atomID uuid.UUID, kind, title string, corpus reportCorpus) ([]reportMoment, []string) {
	if strings.TrimSpace(corpus.Text) == "" {
		// Nothing of hers to quote or reflect on — a real, if rare, state
		// (a reading finished on takeaway alone, with no notes/chat/cards).
		// Calling the model over an empty corpus would only earn a made-up
		// reply that could never survive validateMoments anyway.
		return nil, nil
	}
	resolved, ok := a.resolveEval(ctx)
	if !ok {
		slog.Warn("lite report: no provider resolved", "atom_id", atomID, "kind", kind)
		return nil, nil
	}
	res, cerr := gateway.Collect(ctx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: liteReportSystem},
			{Role: gateway.RoleUser, Content: buildReportPrompt(kind, title, corpus)},
		},
	})
	a.recordLiteLLMCall(ctx, userID, atomID, "lite_report", resolved, res.Usage)
	if cerr != nil {
		slog.Warn("lite report: provider call failed", "err", cerr, "atom_id", atomID)
		return nil, nil
	}
	reply, okParse := parseReportReply(res.Text)
	if !okParse {
		slog.Warn("lite report: unparseable reply", "atom_id", atomID)
		return nil, nil
	}
	return validateMoments(reply.Moments, corpus.Text), cleanGains(reply.Gains)
}

// --- the deterministic half: stats + the time estimate --------------------

// reportGapCap mirrors report_facts.go's cappedGapSeconds cap for the
// fallback time estimate: 5 minutes, per the design spec.
const reportGapCap = 5 * time.Minute

// reportFocusMinutes prefers the real heartbeat total (atom.active_seconds,
// Task 1/2); only when it is 0 — a session that predates heartbeats, or one
// where the tab was never visible long enough to post one — does it fall
// back to cappedGapSeconds over the event trail. Rounds to the nearest
// minute rather than truncating, so a 90-second reading is not reported as
// "0 分钟".
func reportFocusMinutes(activeSeconds int32, stamps []time.Time) int {
	seconds := int(activeSeconds)
	if seconds <= 0 {
		seconds = cappedGapSeconds(stamps, reportGapCap)
	}
	return (seconds + 30) / 60
}

func countStudentMessages(msgs []sqlc.AtomMessage) int {
	n := 0
	for _, m := range msgs {
		if m.Role == "student" {
			n++
		}
	}
	return n
}

// countAnnotationsWithNote is 笔记 N 条: atom_annotation rows that carry an
// actual note, not a highlight left bare — "her margin notes, not highlights
// without a note" per the design spec.
func countAnnotationsWithNote(notes []sqlc.AtomAnnotation) int {
	n := 0
	for _, note := range notes {
		if strings.TrimSpace(note.Note) != "" {
			n++
		}
	}
	return n
}

func countDoneReadingTasks(tasks []sqlc.ReadingTask) int {
	n := 0
	for _, t := range tasks {
		if t.Status == "done" {
			n++
		}
	}
	return n
}

// --- reading -------------------------------------------------------------

// buildReadingReportDTO assembles the full report for a FINISHED reading.
// Called only from inside ensureAtomReport's transaction, on qtx, once the
// re-check under the lock has confirmed no report exists yet.
func (a *API) buildReadingReportDTO(ctx context.Context, qtx *sqlc.Queries, userID uuid.UUID, at sqlc.Atom, studentName string) (liteReportDTO, error) {
	rd, err := qtx.GetReading(ctx, at.ID)
	if err != nil {
		return liteReportDTO{}, err
	}
	takeaway, err := qtx.GetReadingTakeaway(ctx, at.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return liteReportDTO{}, err
	}
	notes, err := qtx.ListAtomAnnotations(ctx, at.ID)
	if err != nil {
		return liteReportDTO{}, err
	}
	msgs, err := qtx.ListAtomMessages(ctx, at.ID)
	if err != nil {
		return liteReportDTO{}, err
	}
	cards, err := qtx.ListAtomCards(ctx, at.ID)
	if err != nil {
		return liteReportDTO{}, err
	}
	tasks, err := qtx.ListReadingTasks(ctx, at.ID)
	if err != nil {
		return liteReportDTO{}, err
	}

	corpus := buildReadingCorpus(takeaway.Text, notes, msgs, cards)

	stamps := make([]time.Time, 0, len(msgs)+len(notes)+len(cards))
	for _, m := range msgs {
		stamps = append(stamps, m.CreatedAt)
	}
	for _, n := range notes {
		stamps = append(stamps, n.CreatedAt)
	}
	for _, c := range cards {
		stamps = append(stamps, c.CreatedAt)
	}

	stats := []reportStat{
		{Key: "focusMinutes", Label: "专注时长", Value: reportFocusMinutes(at.ActiveSeconds, stamps), Unit: "分钟"},
		{Key: "chatTurns", Label: "和印记聊了", Value: countStudentMessages(msgs), Unit: "轮"},
		{Key: "notes", Label: "笔记", Value: countAnnotationsWithNote(notes), Unit: "条"},
		{Key: "stepsDone", Label: "读完", Value: countDoneReadingTasks(tasks), Unit: "步"},
	}

	var keep *reportKeep
	if text := strings.TrimSpace(takeaway.Text); text != "" {
		keep = &reportKeep{Label: "我的收获", Text: text}
	}

	moments, gains := a.generateReportProse(ctx, userID, at.ID, "reading", rd.Title, corpus)

	finishedAt := ""
	if rd.FinishedAt.Valid {
		finishedAt = rd.FinishedAt.Time.Format(time.RFC3339)
	}

	return liteReportDTO{
		Version: 1, Kind: "reading", Title: rd.Title, StudentName: studentName,
		FinishedAt: finishedAt, Stats: stats, Moments: moments, Keep: keep, Gains: gains,
	}, nil
}

// --- writing ---------------------------------------------------------------

// writingAllMessages gathers the main-thread transcript AND every 深入一层
// block thread (one per writing_outline node — see writing_deepen.go, where
// the block id IS the outline node's uuid). "和印记聊了 N 轮" counts both per
// the design spec, since both are conversations she actually had; the
// corpus builder gets the same combined set for the same reason — a
// sentence she said inside a block thread is no less hers than one she said
// in the main room.
func (a *API) writingAllMessages(ctx context.Context, qtx *sqlc.Queries, atomID uuid.UUID, outline []sqlc.WritingOutline) ([]sqlc.AtomMessage, error) {
	main, err := qtx.ListAtomMessages(ctx, atomID)
	if err != nil {
		return nil, err
	}
	all := make([]sqlc.AtomMessage, 0, len(main))
	all = append(all, main...)
	for _, o := range outline {
		blockID := o.ID.String()
		block, err := qtx.ListAtomBlockMessages(ctx, sqlc.ListAtomBlockMessagesParams{AtomID: atomID, BlockID: &blockID})
		if err != nil {
			return nil, err
		}
		all = append(all, block...)
	}
	return all, nil
}

func (a *API) buildWritingReportDTO(ctx context.Context, qtx *sqlc.Queries, userID uuid.UUID, at sqlc.Atom, studentName string) (liteReportDTO, error) {
	wr, err := qtx.GetWriting(ctx, at.ID)
	if err != nil {
		return liteReportDTO{}, err
	}
	draft, err := qtx.GetWritingDraft(ctx, at.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return liteReportDTO{}, err
	}
	snippets, err := qtx.ListWritingSnippets(ctx, at.ID)
	if err != nil {
		return liteReportDTO{}, err
	}
	outline, err := qtx.ListWritingOutline(ctx, at.ID)
	if err != nil {
		return liteReportDTO{}, err
	}
	msgs, err := a.writingAllMessages(ctx, qtx, at.ID, outline)
	if err != nil {
		return liteReportDTO{}, err
	}
	comments, err := qtx.ListWritingComments(ctx, at.ID)
	if err != nil {
		return liteReportDTO{}, err
	}

	corpus := buildWritingCorpus(draft.Body, snippets, outline, msgs)

	stamps := make([]time.Time, 0, len(msgs)+len(snippets)+len(comments))
	for _, m := range msgs {
		stamps = append(stamps, m.CreatedAt)
	}
	for _, s := range snippets {
		stamps = append(stamps, s.UpdatedAt)
	}
	for _, c := range comments {
		stamps = append(stamps, c.CreatedAt)
	}

	stats := []reportStat{
		{Key: "words", Label: "写了", Value: countWordsForLang(draft.Body, wr.Lang), Unit: "字"},
		{Key: "focusMinutes", Label: "专注时长", Value: reportFocusMinutes(at.ActiveSeconds, stamps), Unit: "分钟"},
		{Key: "chatTurns", Label: "和印记聊了", Value: countStudentMessages(msgs), Unit: "轮"},
		{Key: "snippets", Label: "改了", Value: len(snippets), Unit: "段"},
	}

	moments, gains := a.generateReportProse(ctx, userID, at.ID, "writing", wr.Title, corpus)

	finishedAt := ""
	if wr.FinishedAt.Valid {
		finishedAt = wr.FinishedAt.Time.Format(time.RFC3339)
	}

	// keep is always nil for a writing — see the file comment: a writing has
	// no single deterministic "one thing to take away" the way a reading's
	// takeaway is; its 金句 carry that weight instead.
	return liteReportDTO{
		Version: 1, Kind: "writing", Title: wr.Title, StudentName: studentName,
		FinishedAt: finishedAt, Stats: stats, Moments: moments, Keep: nil, Gains: gains,
	}, nil
}

// --- the generator (Task 5 depends on this exact signature) ---------------

// ensureAtomReport returns the atom's report, generating and storing it on
// first call. The bool is false when the atom is not finished: no report,
// no generation, nothing stamped. Both the GET handler below and Task 5's
// share handler go through here, so there is exactly one generator — see
// the file comment.
//
// ctx is expected to already be detached from the caller's request (see
// detachedModelCtx, reading_lens.go): once the model call is under way it
// must run to completion even if she navigates away mid-call.
func (a *API) ensureAtomReport(ctx context.Context, userID, atomID uuid.UUID, kind string) (sqlc.AtomReport, bool, error) {
	finished, err := a.atomIsFinished(ctx, atomID, kind)
	if err != nil {
		return sqlc.AtomReport{}, false, err
	}
	if !finished {
		// F5-shaped gate (reading_questions.go): a generate-if-absent
		// endpoint reachable early must not burn its one generation on
		// partial data, permanently. Not an error — an unfinished atom
		// simply has no report yet.
		return sqlc.AtomReport{}, false, nil
	}

	// Cheap pre-check outside any transaction: after the first generation
	// this is the only cost, for every reopen, forever.
	if row, err := a.d.Queries.GetAtomReport(ctx, atomID); err == nil {
		return row, true, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.AtomReport{}, false, err
	}

	at, err := a.d.Queries.GetAtom(ctx, atomID)
	if err != nil {
		return sqlc.AtomReport{}, false, err
	}

	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return sqlc.AtomReport{}, false, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	// hashtext of the atom uuid, namespaced so this can never collide with
	// another advisory lock keyed on the same uuid (reading_questions.go's
	// ":questions", writing_setup.go's ":opening" — same discipline).
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", atomID.String()+":report"); err != nil {
		return sqlc.AtomReport{}, false, err
	}
	qtx := a.d.Queries.WithTx(tx)

	// Re-check UNDER the lock: a racer that already generated (and
	// committed) while this one waited wins outright — this one returns its
	// rows without ever reaching the provider.
	if row, err := qtx.GetAtomReport(ctx, atomID); err == nil {
		if cerr := tx.Commit(ctx); cerr != nil {
			return sqlc.AtomReport{}, false, cerr
		}
		return row, true, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.AtomReport{}, false, err
	}

	name, err := a.studentDisplayName(ctx, qtx, userID)
	if err != nil {
		return sqlc.AtomReport{}, false, err
	}

	var report liteReportDTO
	switch kind {
	case "reading":
		report, err = a.buildReadingReportDTO(ctx, qtx, userID, at, name)
	case "writing":
		report, err = a.buildWritingReportDTO(ctx, qtx, userID, at, name)
	default:
		err = errUnknownAtomKind(kind)
	}
	if err != nil {
		return sqlc.AtomReport{}, false, err
	}

	raw, merr := json.Marshal(report)
	if merr != nil {
		return sqlc.AtomReport{}, false, merr
	}
	row, err := qtx.UpsertAtomReport(ctx, sqlc.UpsertAtomReportParams{AtomID: atomID, Kind: kind, Report: raw})
	if err != nil {
		return sqlc.AtomReport{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return sqlc.AtomReport{}, false, err
	}
	return row, true, nil
}

// studentDisplayName reads the caller's own display name for the "printed
// on the exported picture" field the user explicitly asked for. Reading it
// fresh from the user row (rather than trusting a caller-supplied string)
// keeps this generator self-contained: the only input it takes is ids.
func (a *API) studentDisplayName(ctx context.Context, qtx *sqlc.Queries, userID uuid.UUID) (string, error) {
	u, err := qtx.GetUserByID(ctx, userID)
	if err != nil {
		return "", err
	}
	return u.DisplayName, nil
}

func errUnknownAtomKind(kind string) error {
	return httpx.ErrBadRequest("unknown_kind", "未知的原子类型。", map[string]any{"kind": kind})
}

// --- the HTTP handler -------------------------------------------------

// getAtomReportFor is GET /api/v1/readings/{id}/report and its writing twin
// — a thin wrapper over ensureAtomReport (see that function's comment for
// the whole idempotent/single-charge shape).
func (a *API) getAtomReportFor(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		at, ok := a.loadOwnedAtom(w, r, kind)
		if !ok {
			return
		}
		u, _ := UserFromContext(r.Context())
		entitled, err := HasEntitlement(r.Context(), u)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if !entitled {
			httpx.WriteError(w, r, httpx.ErrNotEntitled())
			return
		}

		// Run to completion even if she navigates away mid-call — same
		// posture as every other lite provider call (detachedModelCtx,
		// reading_lens.go).
		ctx, cancel := detachedModelCtx(r)
		defer cancel()

		row, found, err := a.ensureAtomReport(ctx, u.ID, at.ID, kind)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if !found {
			httpx.WriteJSON(w, http.StatusOK, map[string]any{"report": nil})
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"report": json.RawMessage(row.Report)})
	}
}
