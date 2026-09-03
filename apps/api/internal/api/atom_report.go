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
// The one model call returns `moments`, `gains` and `summary`.
//
// `keep` — the report's loudest card, 我的收获 — prefers HER OWN takeaway,
// copied verbatim, and falls back to the model's `summary` only when she left
// none. That fallback exists because 完成这篇 stopped asking for a takeaway at
// all, which left the card empty on every new reading. `keep.Source` records
// which of the two it is, and the client renders a different attribution line
// for each — so a generated paragraph is never printed as her own words. R4
// is untouched by this: R4 governs 金句 on the exported picture, which still
// come only from `moments`, still validated as literal substrings of a
// hers-only corpus.
//
// Best-effort prose: if the model call errors, or nothing survives
// validateMoments, the report is still built and stored from the
// deterministic half alone. A student who finished her work always gets her
// stats and her own 收获 — a report must never be blocked on prose.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
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

// reportKeep is 我的收获 — the report's loudest card. Her own takeaway when she
// left one, otherwise the model's `summary`. See Source below and the file
// comment.
type reportKeep struct {
	Label string `json:"label"`
	Text  string `json:"text"`
	// Source is WHO WROTE Text: "student" when it is her own takeaway copied
	// verbatim, "coach" when 印记 wrote it from her session because she never
	// left one.
	//
	// 完成这篇 stopped asking for a takeaway (the finalize form is gone), so
	// on a new reading `keep` was simply absent and the report's loudest card
	// never rendered — the product owner's report on the redesign was "there
	// is no 我的收获 part. we should generate a summary from students'
	// interactions." This field is how that generated paragraph fills the
	// same slot WITHOUT quietly turning into words she never said: the client
	// renders a different attribution line per source and must never omit it.
	//
	// Reports stored before this field existed decode it as "" — the client
	// treats an absent source as "student", which is correct for every one of
	// them, since back then a keep could only ever be her own takeaway.
	Source string `json:"source"`
}

// keepSourceStudent / keepSourceCoach — the two values of reportKeep.Source.
const (
	keepSourceStudent = "student"
	keepSourceCoach   = "coach"
)

// reportLensNote is what her 透镜 work produced: for each lens she
// submitted, the sentence SHE picked out of the article and the 发现 the
// room drew from it. Deterministic — assembled from atom_card rows, never
// asked of a model. This is a DIFFERENT kind of thing from `moments`: a
// moment is a sentence of hers picked out by the MODEL as noteworthy prose;
// a lens note is HER sentence-picking itself — the selecting is the
// thinking (see this feature's own rationale) — paired with the room's
// already-recorded 发现 on that exact card. Nothing here is re-asked of a
// model at report time: the quote is a literal value already on the card's
// anchors row, and the finding already went through agent.EvaluateSelection
// when she submitted the card.
type reportLensNote struct {
	Lens    string `json:"lens"`    // the card's display name, from the registry
	Quote   string `json:"quote"`   // the sentence she picked, from the article
	Finding string `json:"finding"` // the 发现 recorded on that card
}

// reportNote is one of HER reading notes: the sentence she marked in the
// article, and what she wrote next to it.
//
// R4 boundary, and the reason the two fields are named this bluntly: Quote
// is the ARTICLE's words (atom_annotation.quote — the passage she
// highlighted) and Note is HERS. They are never conflated. This struct is
// safe on the report PAGE, where the client labels each half for whose
// words it is; it must never feed the 金句 corpus, and buildReadingCorpus
// (report_facts.go) already excludes annotation.quote for exactly this
// reason. The exported picture keeps quoting `moments` only.
type reportNote struct {
	Quote string `json:"quote"`
	Note  string `json:"note"`
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
	// LensNotes is reading-kind only (nil on every writing report — a
	// writing room has no lens cards to draw one from). Reports generated
	// before this field existed simply lack it on re-serve; no backfill.
	LensNotes []reportLensNote `json:"lensNotes,omitempty"`
	// Notes is reading-kind only: her own margin notes, asked for by name
	// ("my reading notes"). Same no-backfill rule as LensNotes — a report
	// generated before this field existed re-serves without it, and the
	// client renders the section as absent rather than empty.
	Notes []reportNote `json:"notes,omitempty"`
	// Piece is writing-kind only: the finished draft, in full, HER OWN words.
	//
	// It exists because of what the share link is FOR — "if students agree to
	// share, can scan a code to view their writings". Before this, a parent
	// who scanned the QR on a writing report got the stats, the 金句 and the
	// 收获 but could not read the thing she wrote, which is the one thing
	// they opened the link for.
	//
	// R4 is not at risk here and it is worth saying why: every other quoting
	// section on this report pairs words of two different origins (the
	// article's sentence beside her note), so each half must be labelled.
	// This field is unmixed — it is the draft body verbatim, nothing of the
	// article's and nothing of 印记's, since 印记 never authors her prose
	// (铁律①) — and the client still labels it 「我写的」 rather than letting
	// it float unattributed.
	//
	// Deliberately NOT fed to the 金句 corpus: `buildWritingCorpus` already
	// draws on the draft, and the corpus is a validation input, not a
	// presentation one.
	//
	// Same no-backfill rule as the two fields above: a writing report
	// generated before this existed re-serves without it, and the section is
	// simply absent.
	Piece string `json:"piece,omitempty"`
	// ProsePending says the DETERMINISTIC half of this report is stored and
	// serveable, and the one model call (moments / gains / summary) has not
	// run yet. The client renders everything else immediately and asks again;
	// the next request is what generates the prose. See ensureAtomReport's
	// two-phase comment.
	//
	// 🚨 `omitempty`, and the polarity is deliberately PENDING rather than
	// READY. Every report stored before this field existed already has its
	// prose, and decodes this as `false` — i.e. "not pending", complete,
	// which is correct. A `proseReady` field would have decoded those same
	// blobs as `false` too and left the client polling finished reports
	// forever.
	ProsePending bool `json:"prosePending,omitempty"`
	// ProseClaimedAt is bookkeeping, never rendered: the RFC3339 instant at
	// which some request took the lease on generating this report's prose,
	// so a second concurrent reader does not buy the same flagship call. See
	// claimReportProse.
	//
	// It is written onto the STORED BYTES by patchReportClaim rather than by
	// re-marshalling this struct, and it is absent from the completed report
	// this builder produces (`omitempty`, and phase 2 marshals a fresh DTO) —
	// a finished report carries neither flag. Declared here anyway so the
	// field is discoverable from the shape rather than only from the patcher.
	ProseClaimedAt string `json:"proseClaimedAt,omitempty"`
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

// dedupeMomentsAgainstKeep is F4: her 收获 is both rendered verbatim as
// `keep` AND part of the corpus a moment's quote is validated against
// (buildReadingCorpus includes the takeaway) — so the model can legally
// quote her own takeaway sentence back as a 金句, and it then prints twice
// on the same report. Post-hoc dedupe, applied AFTER validateMoments, not a
// corpus exclusion: stripping the takeaway out of the corpus before the
// model call would leave the corpus EMPTY for a thin reading finished on
// takeaway alone, and generateReportProse short-circuits an empty corpus —
// killing `gains` too, for a session that had real material to reflect on.
// A moment whose quote equals `keep.Text`, or is a literal substring of it,
// is dropped; everything else survives untouched.
func dedupeMomentsAgainstKeep(moments []reportMoment, keep *reportKeep) []reportMoment {
	if keep == nil || strings.TrimSpace(keep.Text) == "" {
		return moments
	}
	out := make([]reportMoment, 0, len(moments))
	for _, m := range moments {
		if strings.Contains(keep.Text, m.Quote) {
			continue
		}
		out = append(out, m)
	}
	return out
}

// studentAnchorQuote is the server-side mirror of readingRoom.ts's
// studentAnchorOf (apps/lite-web/src/api/readingRoom.ts): the anchor whose
// author is "student" — the sentence SHE picked out of the article — never
// anchors[0], which is the AI's own grounding example (summonReadingLens,
// reading_lens.go) and must never be quoted back to her as if it were her
// own pick.
func studentAnchorQuote(anchorsJSON []byte) string {
	if len(anchorsJSON) == 0 {
		return ""
	}
	var anchors []agent.Anchor
	if json.Unmarshal(anchorsJSON, &anchors) != nil {
		return ""
	}
	for _, an := range anchors {
		if an.Author == "student" {
			return strings.TrimSpace(an.Quote)
		}
	}
	return ""
}

// buildReadingLensNotes assembles LensNotes from every SUBMITTED atom_card:
// her picked sentence (studentAnchorQuote) paired with the 发现 the room
// drew from it (framework_fill.finding — selectionEvalDTO's wire shape,
// readeval.go). An unknown card id is skipped rather than printed as a raw
// id — cards.ByID (the registry, the same lookup summonReadingLens uses) is
// the single source of display names.
//
// ev.Degraded (selectionEvalDTO's own doc comment names this exact consumer)
// means NO model ever produced this finding — agent.fallbackEval's canned
// "你选了这句作为证据。", not a genuine reading of her sentence. A degraded
// card's finding is dropped, never shown as if the room had actually read
// her pick; her quote is kept regardless — the sentence she chose is her
// work whether or not the model managed to say anything useful about it,
// which is the whole reason this section exists. A card that ends up with
// neither a quote nor a finding is skipped: nothing to show. Submission
// order is preserved — ListAtomCards already returns rows in creation
// order, and this function does no reordering of its own.
func buildReadingLensNotes(atomCards []sqlc.AtomCard) []reportLensNote {
	out := make([]reportLensNote, 0, len(atomCards))
	for _, card := range atomCards {
		if card.Status != "submitted" {
			continue
		}
		spec, ok := cards.ByID(card.CardID)
		if !ok {
			continue
		}
		quote := studentAnchorQuote(card.Anchors)
		var finding string
		if len(card.FrameworkFill) > 0 {
			var ev selectionEvalDTO
			if json.Unmarshal(card.FrameworkFill, &ev) == nil && !ev.Degraded {
				finding = strings.TrimSpace(ev.Finding)
			}
		}
		if quote == "" && finding == "" {
			continue
		}
		out = append(out, reportLensNote{Lens: spec.Name, Quote: quote, Finding: finding})
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

// maxKeepRunes caps the generated 我的收获 paragraph. It is the largest type
// on the report and it sits in a fixed-height card on the exported picture;
// a model that ignores "3-4 句" and writes an essay must not be able to blow
// either layout out. Runes, not bytes — this text is Chinese.
const maxKeepRunes = 220

// cleanKeepSummary trims the model's 我的收获 paragraph and truncates it at
// maxKeepRunes on a sentence boundary where one is available, so a cut never
// lands mid-clause. Returns "" for anything blank, which the caller treats as
// "no keep at all" rather than an empty card.
func cleanKeepSummary(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxKeepRunes {
		return s
	}
	cut := runes[:maxKeepRunes]
	// Prefer the last sentence end inside the window; fall back to a hard cut
	// with an ellipsis so the truncation is visible rather than pretending the
	// paragraph simply ended there.
	for i := len(cut) - 1; i >= maxKeepRunes/2; i-- {
		switch cut[i] {
		case '。', '！', '？':
			return string(cut[:i+1])
		}
	}
	return string(cut) + "…"
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

你要做三件事：

1. moments：从材料里挑出最多 3 句她自己的原话——**逐字复制**，不要改写、不要
   翻译、不要加标点、不要把两句拼成一句。配一句极短的说明，交代这是她在做什么
   的时候说的（比如"写论证的时候""读到关键段落时""和你商量怎么开头的时候"）。
   挑真正有想法、有判断的句子，不要挑她随手打的字或者客套话。挑不出来就留空，
   不要硬凑。
2. gains：用 2-4 句话说说她这次真正做到了什么、用了什么方法、想清楚了什么问
   题——要具体，要说得出名字，不要说"她表现很好""很棒"这种空话，也绝对不要打
   分、不要暗示名次、不要和任何别的学生比。像一个老师当面跟她说话，不是在写一
   封表扬信。**gains 里的每一句都要用"你"称呼她本人，直接对她说**——例如"你
   抓住了……""你把问题从……推进到了……""你调整了……"；绝不要用第三人称去描述
   这件事，那是写给别人看的评语，不是说给她本人听的话。

3. summary：一段 3-4 句的话，写"这次她最值得带走的是什么"。这一段会被放在报告
   最显眼的位置，标题就是「我的收获」——所以它要像**她自己会写下的那种收获**：
   不是流水账（"你先读了第一段，然后……"），而是**一个想法**：她这次弄明白了
   什么、原来以为什么、现在改成怎么看、下次遇到同类东西要注意哪一点。全部用
   "你"直接对她说，具体到能说出名字（哪个概念、哪一步、哪句话），不要空话、不
   要打分、不要和别人比。**不要和 gains 里的句子重复**——gains 是几条并列的、
   短的事实，summary 是一段有转折、有结论的话。材料太薄写不出来就留空字符串。

只输出一个 JSON 对象：
{"moments":[{"quote":"...","where":"..."}],"gains":["你...","你..."],"summary":"你..."}

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
	Summary string         `json:"summary"`
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

// reportProse is what the one model call yields. A struct rather than three
// return values: the third one (Summary) arrived after two callers were
// already destructuring the pair, and a bare `(nil, nil, "")` on five failure
// paths reads as noise next to a named zero value.
type reportProse struct {
	Moments []reportMoment
	Gains   []string
	// Summary is the 我的收获 paragraph, used ONLY when she left no takeaway
	// of her own — see buildReadingReportDTO.
	Summary string
}

// generateReportProse is the ONE flagship call this whole file makes: it asks
// for moments+gains+summary together (one editorial judgment over one corpus,
// not three calls for three fields) and is best-effort throughout — every
// failure path returns the zero value rather than an error, because a report
// is never blocked on prose (see the file comment).
func (a *API) generateReportProse(ctx context.Context, userID, atomID uuid.UUID, kind, title string, corpus reportCorpus) reportProse {
	if strings.TrimSpace(corpus.Text) == "" {
		// Nothing of hers to quote or reflect on — a real, if rare, state
		// (a reading finished on takeaway alone, with no notes/chat/cards).
		// Calling the model over an empty corpus would only earn a made-up
		// reply that could never survive validateMoments anyway.
		return reportProse{}
	}
	resolved, ok := a.route(ctx, gateway.ClassAssess)
	if !ok {
		slog.Warn("lite report: no provider resolved", "atom_id", atomID, "kind", kind)
		return reportProse{}
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
		return reportProse{}
	}
	reply, okParse := parseReportReply(res.Text)
	if !okParse {
		slog.Warn("lite report: unparseable reply", "atom_id", atomID)
		return reportProse{}
	}
	return reportProse{
		Moments: validateMoments(reply.Moments, corpus.Text),
		Gains:   cleanGains(reply.Gains),
		Summary: cleanKeepSummary(reply.Summary),
	}
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

// countSubmittedCards is 用了透镜 N 个 — only SUBMITTED cards count. A card
// she summoned and abandoned is not a lens she used, and printing it as one
// would inflate the record; this is the same status filter buildReadingCorpus
// and buildReadingLensNotes already apply.
func countSubmittedCards(cards []sqlc.AtomCard) int {
	n := 0
	for _, c := range cards {
		if c.Status == "submitted" {
			n++
		}
	}
	return n
}

// buildReadingNotes is 我的笔记: every annotation that carries an actual note,
// paired with the article sentence it hangs off, oldest first (the order
// ListAtomAnnotations already returns, which is the order she read in).
//
// Capped at maxReportNotes: the report is a poster, not a transcript. A bare
// highlight with no note is skipped — the same rule countAnnotationsWithNote
// counts by, so the tile and the section can never disagree about what a
// 笔记 is.
func buildReadingNotes(notes []sqlc.AtomAnnotation) []reportNote {
	out := make([]reportNote, 0, len(notes))
	for _, n := range notes {
		note := strings.TrimSpace(n.Note)
		if note == "" {
			continue
		}
		out = append(out, reportNote{Quote: strings.TrimSpace(n.Quote), Note: note})
		if len(out) == maxReportNotes {
			break
		}
	}
	return out
}

// maxReportNotes caps 我的笔记 — see buildReadingNotes.
const maxReportNotes = 12

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
func (a *API) buildReadingReportDTO(ctx context.Context, qtx *sqlc.Queries, userID uuid.UUID, at sqlc.Atom, studentName string, wantProse bool) (liteReportDTO, error) {
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
	// F1: the article's own paragraphs, so buildReadingCorpus can drop any
	// article sentence that reached a student message without a "> " prefix
	// (see stripArticleLines, report_facts.go). A source that no longer
	// exists (should not happen for a finished reading, but this is a
	// best-effort report, not the source-loading path) degrades to nil
	// blocks — stripArticleLines is then a no-op, same as before this fix.
	src, err := qtx.GetReadingSource(ctx, at.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return liteReportDTO{}, err
	}
	blocks := SplitBlocks(src.Body)

	corpus := buildReadingCorpus(takeaway.Text, notes, msgs, cards, blocks)

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

	// Seven facts, not four. The report is meant to read as a record of a
	// real session — a wide strip of numbers she recognises — and every one
	// of these is already sitting in the rows loaded above, so none of them
	// costs a query or a model call. Order is the render order: time first
	// (the thing she feels), then volume, then the marks she left.
	//
	// A zero is dropped client-side, so a thin session still shows a short
	// honest strip rather than a wall of noughts.
	stats := []reportStat{
		{Key: "focusMinutes", Label: "阅读时长", Value: reportFocusMinutes(at.ActiveSeconds, stamps), Unit: "分钟"},
		{Key: "wordsRead", Label: "读了", Value: countWordsForLang(src.Body, rd.Lang), Unit: "字"},
		// 「AI 教练对话轮数」 and 「完成阅读任务」 are the product owner's own
		// wording, replacing 「和印记聊了 N 轮」 and 「读完 N 步」: the first
		// assumed the reader already knows who 印记 is, and the second read as
		// an unfinished sentence on a tile. Both carry an empty unit — the
		// label already names the quantity, and "17 轮 / AI 教练对话轮数"
		// stutters.
		{Key: "chatTurns", Label: "AI 对话轮数", Value: countStudentMessages(msgs)},
		{Key: "highlights", Label: "划线", Value: len(notes), Unit: "处"},
		{Key: "notes", Label: "笔记", Value: countAnnotationsWithNote(notes), Unit: "条"},
		{Key: "lenses", Label: "用了透镜", Value: countSubmittedCards(cards), Unit: "个"},
		{Key: "stepsDone", Label: "阅读任务完成数", Value: countDoneReadingTasks(tasks)},
	}

	var keep *reportKeep
	if text := strings.TrimSpace(takeaway.Text); text != "" {
		keep = &reportKeep{Label: "我的收获", Text: text, Source: keepSourceStudent}
	}

	// Phase 1 skips this entirely — see ensureAtomReport. The deterministic
	// half above is everything she actually did; the prose is editorial gloss
	// on top of it, and it must never be what she waits for.
	var prose reportProse
	if wantProse {
		prose = a.generateReportProse(ctx, userID, at.ID, "reading", rd.Title, corpus)
	}
	// F4: the takeaway stays IN the corpus (see this function's caller
	// context and dedupeMomentsAgainstKeep's own doc comment for why), so
	// strip it back out of moments here, after generation, rather than
	// before — otherwise a thin reading finished on takeaway alone would
	// hand the model an empty corpus and lose `gains` too.
	moments := dedupeMomentsAgainstKeep(prose.Moments, keep)
	gains := prose.Gains

	// Her own words win; the model's summary only fills a slot she left empty.
	// 完成这篇 no longer asks for a takeaway, so on a new reading this branch
	// is the NORMAL one and the student branch above is the legacy path.
	if keep == nil && prose.Summary != "" {
		keep = &reportKeep{Label: "我的收获", Text: prose.Summary, Source: keepSourceCoach}
	}

	finishedAt := ""
	if rd.FinishedAt.Valid {
		finishedAt = rd.FinishedAt.Time.Format(time.RFC3339)
	}

	lensNotes := buildReadingLensNotes(cards)

	return liteReportDTO{
		Version: 1, Kind: "reading", Title: rd.Title, StudentName: studentName,
		FinishedAt: finishedAt, Stats: stats, Moments: moments, Keep: keep, Gains: gains,
		LensNotes: lensNotes, Notes: buildReadingNotes(notes),
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

func (a *API) buildWritingReportDTO(ctx context.Context, qtx *sqlc.Queries, userID uuid.UUID, at sqlc.Atom, studentName string, wantProse bool) (liteReportDTO, error) {
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
		// 写作时长, not 阅读时长 — this is the writing report. The client
		// resolves stat labels from `Key` anyway (statLabels.ts, so that a
		// rename reaches reports already stored), and it carries the same
		// per-kind override; this string is what a fresh blob stores and what
		// anything reading the blob directly would see.
		{Key: "focusMinutes", Label: "写作时长", Value: reportFocusMinutes(at.ActiveSeconds, stamps), Unit: "分钟"},
		{Key: "chatTurns", Label: "AI 对话轮数", Value: countStudentMessages(msgs)},
		{Key: "outline", Label: "搭了提纲", Value: len(outline), Unit: "条"},
		{Key: "snippets", Label: "改了", Value: len(snippets), Unit: "段"},
		{Key: "comments", Label: "印记读了", Value: len(comments), Unit: "遍"},
	}

	// See buildReadingReportDTO's twin comment: phase 1 stores without prose.
	var prose reportProse
	if wantProse {
		prose = a.generateReportProse(ctx, userID, at.ID, "writing", wr.Title, corpus)
	}

	// A writing has no takeaway field at all, so its 我的收获 is always the
	// model's summary or nothing — which is why this card was simply missing
	// from every writing report before now.
	var keep *reportKeep
	if prose.Summary != "" {
		keep = &reportKeep{Label: "我的收获", Text: prose.Summary, Source: keepSourceCoach}
	}

	finishedAt := ""
	if wr.FinishedAt.Valid {
		finishedAt = wr.FinishedAt.Time.Format(time.RFC3339)
	}

	return liteReportDTO{
		Version: 1, Kind: "writing", Title: wr.Title, StudentName: studentName,
		FinishedAt: finishedAt, Stats: stats, Moments: prose.Moments, Keep: keep, Gains: prose.Gains,
		// See `Piece`. Trimmed so a draft of nothing but whitespace stores as
		// "" and the section is absent rather than an empty bordered slab.
		Piece: strings.TrimSpace(draft.Body),
	}, nil
}

// --- the generator (Task 5 depends on this exact signature) ---------------

// reportWithPiece fills in a writing report's `piece` at SERVE time when the
// stored blob does not carry one.
//
// ## Why this has to exist
//
// A report is generated ONCE and stored as a JSON blob, then re-served
// verbatim forever — and `piece` was added to that blob after reports already
// existed. "No backfill, the section is simply absent" was fine while the
// piece was one section at the foot of the page. It stopped being fine the
// moment the piece became THE PAGE: with no `piece`, `PublicReportPage` sees
// `hasArticle === false` and opens the record, so every writing finished
// before that deploy shares as a page of statistics with the article missing
// entirely. Checked on production: 2 of 2 writing reports had no `piece`, i.e.
// the article page was live and unreachable for every existing piece.
//
// ## Why read-time rather than a backfill migration
//
// The same reasoning as `statLabels.ts` resolving stat wording on the client:
// the blob is the RECORD of a generation, and anything derivable from data
// that still exists should be derived rather than frozen. The draft is a live
// row (`writing_draft`) that nothing rewrites after 完成这篇, so reading it
// here yields exactly the text the generator would have stored — and it
// self-heals every old report at once, with no migration to run and nothing
// to re-run if it is added to again.
//
// ## Failure posture
//
// Every error path returns `stored` unchanged. A report that cannot be
// hydrated must still be served: losing the article section is a smaller harm
// than 500-ing on a link someone was sent. Nothing here can substitute the
// wrong atom's prose either — the draft is looked up by this report's own
// `atom_id`.
// hasNonEmptyString reports whether the field is present AND decodes to a
// string with something in it. A stored `""` (or `"   "`) counts as ABSENT:
// treating it as present is what would let a blank field pin itself forever.
func hasNonEmptyString(fields map[string]json.RawMessage, key string) bool {
	raw, ok := fields[key]
	if !ok {
		return false
	}
	var s string
	return json.Unmarshal(raw, &s) == nil && strings.TrimSpace(s) != ""
}

func (a *API) reportWithPiece(ctx context.Context, atomID uuid.UUID, stored []byte) []byte {
	var fields map[string]json.RawMessage
	if json.Unmarshal(stored, &fields) != nil {
		return stored
	}
	// Kind comes off the blob itself rather than a second query — it is
	// written by the generator and is the same value `atom.kind` holds.
	var kind string
	if raw, ok := fields["kind"]; !ok || json.Unmarshal(raw, &kind) != nil || kind != "writing" {
		return stored
	}
	// Both live rows, read unconditionally: the title is always needed (see
	// mergeLiveWritingFields) and the draft is needed whenever the blob has no
	// piece, which for now is every report that already existed. Two indexed
	// single-row lookups on a page that is already one round trip.
	//
	// A failed lookup is not an error here — it contributes nothing and the
	// corresponding field keeps whatever the blob holds.
	draftBody := ""
	if draft, err := a.d.Queries.GetWritingDraft(ctx, atomID); err == nil {
		draftBody = draft.Body
	}
	title := ""
	if wr, err := a.d.Queries.GetWriting(ctx, atomID); err == nil {
		title = wr.Title
	}

	if !mergeLiveWritingFields(fields, draftBody, title) {
		return stored
	}
	out, err := json.Marshal(fields)
	if err != nil {
		return stored
	}
	return out
}

// mergeLiveWritingFields folds the live draft and title into a decoded report
// blob, reporting whether anything changed. Split out from the I/O above so
// the two rules — which differ, deliberately — can be tested without a
// database.
//
// **piece: only when missing.** A report generated after the field shipped is
// authoritative, and reading the draft over it would make two sources of truth
// for the same words. A stored `""` counts as missing.
//
// **title: always.** Here the live row IS the truth: she can still rename a
// finished writing, and 给这篇起个名字 only asks at 完成这篇 — so a piece
// finished before that flow shipped still carries her raw 「我想写：…」 sentence
// in its blob, where a rename would never reach it. A live production example
// was a shared article whose headline was six lines of note-to-self. PATCH
// /writings/{id} writes that row and nothing else does, so refreshing from it
// lands a rename on the article, the report, the poster and the share link at
// once.
//
// 🚨 Do not "tidy" these two into one rule. They are asymmetric on purpose.
func mergeLiveWritingFields(fields map[string]json.RawMessage, draftBody, title string) bool {
	changed := false
	if body := strings.TrimSpace(draftBody); body != "" && !hasNonEmptyString(fields, "piece") {
		if encoded, err := json.Marshal(body); err == nil {
			fields["piece"] = encoded
			changed = true
		}
	}
	if t := strings.TrimSpace(title); t != "" {
		if encoded, err := json.Marshal(t); err == nil {
			// Not a change if it already says exactly this — a no-op rewrite
			// would re-marshal the whole blob on every read for nothing.
			if !bytes.Equal(fields["title"], encoded) {
				fields["title"] = encoded
				changed = true
			}
		}
	}
	return changed
}

// ensureAtomReport returns the atom's report, generating and storing it on
// first call. The bool is false when the atom is not finished: no report,
// no generation, nothing stamped. Both the GET handler below and Task 5's
// share handler go through here, so there is exactly one generator — see
// the file comment.
//
// ## TWO PHASES (2026-09-03) — why this is not one call any more
//
// Reported from the colleague trial as two bugs that are one bug:
// 「报告没有loading状态」 and 「印记正在把这次读的东西整理成一份报告，稍等
// 一下。— it never finishes」.
//
// generateReportProse routes to gateway.ClassAssess, and that class is
// `reasoning: "max"` with a latencyBudgetMs of 180000 (models.json). It is
// bound that way on purpose — 过程评估绝不降级 — and the routing spec itself
// files `assess` as 「异步」. But lite serves it INLINE inside a GET, and the
// enclosing request is capped at liteModelWorkTimeout = 150s. So:
//
//   - the student sat on one static grey sentence, with no spinner and no
//     progress, for 60–150 seconds (production's last lite_report call burned
//     5,976 completion tokens);
//   - and on a big session the 150s cap fires BEFORE the 180s the class is
//     allowed, which killed the whole transaction — so the report was not
//     merely late, it was never stored at all. "It never finishes" was
//     literally true.
//
// P4 (river + workers) is the sanctioned answer and is not built. The house
// pattern in the meantime is LAZY, not fire-and-forget: this repo has no
// fire-and-forget precedent, and the interest-harvest work deliberately
// declined to introduce one (it tops up on open instead). So:
//
//	phase 1 — no model call at all. Build the deterministic half (stats, her
//	          notes, her lens notes, her own 收获, the piece), store it with
//	          ProsePending, and return. Milliseconds.
//	phase 2 — a LATER request sees ProsePending on the stored row and runs
//	          the one model call, then upserts the enriched report.
//
// She is therefore never waiting on a blank screen: her real report is on
// screen immediately and the two prose sections fill in behind it. And a
// prose call that times out now costs only the prose — the report itself is
// already committed, which is what this file's own header always promised
// ("a report must never be blocked on prose") and what the inline version
// quietly failed to deliver.
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

	// Cheap pre-check outside any transaction: after both phases have run
	// this is the only cost, for every reopen, forever.
	if row, err := a.d.Queries.GetAtomReport(ctx, atomID); err == nil {
		if !reportProsePending(row.Report) {
			return row, true, nil
		}
		// Phase 2. The deterministic half is already hers to read; this is
		// the follow-up request that pays for the prose.
		return a.enrichAtomReportProse(ctx, userID, atomID, kind, row)
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

	// Phase 1: wantProse=false. No provider call happens inside this
	// transaction — which is also what lets the advisory lock be held for
	// milliseconds instead of minutes. The old shape kept a Postgres
	// transaction open across a 150-second flagship call, so a second reader
	// blocked on the lock for the whole of it.
	report, err := a.buildAtomReportDTO(ctx, qtx, userID, at, kind, name, false)
	if err != nil {
		return sqlc.AtomReport{}, false, err
	}
	report.ProsePending = true

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

// buildAtomReportDTO dispatches on kind. Extracted so phase 1 and phase 2
// cannot drift into building two subtly different reports — the only thing
// that differs between them is `wantProse`.
func (a *API) buildAtomReportDTO(
	ctx context.Context,
	qtx *sqlc.Queries,
	userID uuid.UUID,
	at sqlc.Atom,
	kind, studentName string,
	wantProse bool,
) (liteReportDTO, error) {
	switch kind {
	case "reading":
		return a.buildReadingReportDTO(ctx, qtx, userID, at, studentName, wantProse)
	case "writing":
		return a.buildWritingReportDTO(ctx, qtx, userID, at, studentName, wantProse)
	default:
		return liteReportDTO{}, errUnknownAtomKind(kind)
	}
}

// reportProsePending reads the one field phase 2 keys off, without decoding
// the whole envelope. A blob that fails to parse is treated as COMPLETE: a
// report we cannot read is not a report we should spend a flagship call
// trying to improve, and every report stored before this field existed
// already has its prose.
func reportProsePending(raw []byte) bool {
	return reportProseProbe(raw).ProsePending
}

// proseClaimLease bounds how long one request's claim on a report's prose
// blocks another's. It has to exceed the longest the model call can take —
// the request ctx caps that at liteModelWorkTimeout (150s) — with enough
// headroom that a claim never expires while its own call is still running.
// It must also stay SHORT enough that a crashed process only delays the
// prose by one reopen, rather than leaving the report pending forever.
const proseClaimLease = 4 * time.Minute

// reportProseProbe decodes only the two bookkeeping fields, so neither
// pending-checking nor claim-checking has to parse (or risk reshaping) the
// whole stored envelope.
func reportProseProbe(raw []byte) struct {
	ProsePending   bool   `json:"prosePending"`
	ProseClaimedAt string `json:"proseClaimedAt"`
} {
	var probe struct {
		ProsePending   bool   `json:"prosePending"`
		ProseClaimedAt string `json:"proseClaimedAt"`
	}
	_ = json.Unmarshal(raw, &probe)
	return probe
}

// claimReportProse takes the lease described in enrichAtomReportProse, in
// one short transaction. Returns false — with no error — whenever somebody
// else already holds it, or the prose has landed in the meantime.
//
// 🚨 The claim is written by patching the STORED bytes, not by re-marshalling
// a decoded DTO. Re-marshalling would silently rewrite every report through
// whatever the current struct happens to look like, quietly dropping fields
// that a newer build had added and an older one has not learned — the report
// is a stored blob and this function has no business reshaping it.
func (a *API) claimReportProse(ctx context.Context, atomID uuid.UUID, kind string, stored sqlc.AtomReport) (bool, error) {
	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", atomID.String()+":report"); err != nil {
		return false, err
	}
	qtx := a.d.Queries.WithTx(tx)
	row, err := qtx.GetAtomReport(ctx, atomID)
	if err != nil {
		return false, err
	}
	probe := reportProseProbe(row.Report)
	if !probe.ProsePending {
		// Another reader finished the prose while this one was getting here.
		return false, nil
	}
	if at, perr := time.Parse(time.RFC3339, probe.ProseClaimedAt); perr == nil && time.Since(at) < proseClaimLease {
		// A live claim. Somebody else is paying for this call right now.
		return false, nil
	}

	patched, err := patchReportClaim(row.Report, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return false, err
	}
	if _, err := qtx.UpsertAtomReport(ctx, sqlc.UpsertAtomReportParams{AtomID: atomID, Kind: kind, Report: patched}); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// stripProseBookkeeping removes the two-phase generator's internal fields
// from an envelope about to be served to someone who is NOT the owner. See
// getPublicReport for why a visitor must never see them.
//
// Leaves everything else byte-identical, and returns the input unchanged if
// it does not parse — a payload we cannot read is one we must not reshape.
func stripProseBookkeeping(raw []byte) []byte {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return raw
	}
	if _, pending := obj["prosePending"]; !pending {
		if _, claimed := obj["proseClaimedAt"]; !claimed {
			// The overwhelmingly common case (a completed report): don't
			// re-marshal at all, so the bytes she shared stay the bytes we
			// serve.
			return raw
		}
	}
	delete(obj, "prosePending")
	delete(obj, "proseClaimedAt")
	out, err := json.Marshal(obj)
	if err != nil {
		return raw
	}
	return out
}

// patchReportClaim sets `proseClaimedAt` on the stored envelope while leaving
// every other key byte-identical — see claimReportProse's 🚨.
func patchReportClaim(raw []byte, stamp string) ([]byte, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	v, err := json.Marshal(stamp)
	if err != nil {
		return nil, err
	}
	obj["proseClaimedAt"] = v
	return json.Marshal(obj)
}

// enrichAtomReportProse is PHASE 2: the stored report is already serveable,
// and this is the request that pays for its prose. See ensureAtomReport.
//
// 🚨 It returns the report either way. A failed or timed-out model call
// leaves the deterministic report exactly as it was and hands it straight
// back — the student reads her stats, her notes and her own 收获, and the
// next time she opens the report the prose is attempted again. Nothing about
// this path may ever be able to turn a stored report into an error.
//
// The provider call happens OUTSIDE any transaction, on purpose: phase 1's
// mistake was holding a Postgres transaction (and its advisory lock) across
// a call that can run for minutes. Here the lock is taken only for the write
// at the end, after the model has already answered.
func (a *API) enrichAtomReportProse(
	ctx context.Context,
	userID, atomID uuid.UUID,
	kind string,
	stored sqlc.AtomReport,
) (sqlc.AtomReport, bool, error) {
	// 🚨 One charge, even with two readers, WITHOUT holding a transaction
	// across the model call.
	//
	// Phase 1's lock is transaction-scoped (pg_advisory_xact_lock), which is
	// right for a write that takes milliseconds and wrong here: this call can
	// run for 150 seconds, and a transaction held open that long is what made
	// a second reader block for the whole of it. But dropping the lock
	// entirely lets two concurrent opens each buy their own flagship call —
	// the double-charge shape this codebase has already had to close twice.
	//
	// So the work is CLAIMED in a short transaction of its own, before the
	// model call, by stamping the stored report. A second reader that sees a
	// live claim serves the report it already has (the deterministic half is
	// complete and hers to read) and picks the prose up on a later open.
	//
	// The claim is a LEASE, not a flag: a process that dies mid-call must not
	// leave the report pending forever, so a claim older than
	// proseClaimLease is ignored and re-taken.
	claimed, err := a.claimReportProse(ctx, atomID, kind, stored)
	if err != nil || !claimed {
		return stored, true, nil
	}

	at, err := a.d.Queries.GetAtom(ctx, atomID)
	if err != nil {
		// Serve what we have. See the 🚨 above.
		return stored, true, nil
	}
	name, err := a.studentDisplayName(ctx, a.d.Queries, userID)
	if err != nil {
		return stored, true, nil
	}

	report, err := a.buildAtomReportDTO(ctx, a.d.Queries, userID, at, kind, name, true)
	if err != nil {
		slog.Warn("lite report: prose phase failed to rebuild", "err", err, "atom_id", atomID)
		return stored, true, nil
	}
	// The model declined, timed out, or produced nothing that survived
	// validateMoments. Leave the row pending so the NEXT open tries again,
	// rather than stamping an empty prose section as final.
	//
	// `keep` is the exception worth naming: on a session where she left no
	// takeaway of her own, `keep` comes from prose.Summary, so a report whose
	// prose never lands keeps an empty 我的收获 — which is the same state it
	// had before this two-phase split, not a new loss.
	if len(report.Moments) == 0 && len(report.Gains) == 0 {
		return stored, true, nil
	}

	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return stored, true, nil
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", atomID.String()+":report"); err != nil {
		return stored, true, nil
	}
	qtx := a.d.Queries.WithTx(tx)
	// Re-read under the lock: a concurrent reader may have already finished
	// phase 2 while this one was talking to the provider. Its prose is as
	// good as ours and it is already committed — do not overwrite it.
	if row, rerr := qtx.GetAtomReport(ctx, atomID); rerr == nil && !reportProsePending(row.Report) {
		if cerr := tx.Commit(ctx); cerr != nil {
			return stored, true, nil
		}
		return row, true, nil
	}

	raw, merr := json.Marshal(report)
	if merr != nil {
		return stored, true, nil
	}
	row, err := qtx.UpsertAtomReport(ctx, sqlc.UpsertAtomReportParams{AtomID: atomID, Kind: kind, Report: raw})
	if err != nil {
		return stored, true, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return stored, true, nil
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
			httpx.WriteJSON(w, http.StatusOK, map[string]any{"report": nil, "shared": false, "shareToken": nil})
			return
		}
		// F2: tell the AUTHENTICATED caller whether this report is already
		// shared, and with what token, so SharePanel can rebuild its own
		// state on reload instead of always starting at {phase:"off"} — a
		// state whose copy describes sharing as hypothetical when it may
		// already be live, and whose only path to 停止分享 is a button
		// labelled 生成分享链接 that most students will never press. The
		// PUBLIC payload (getPublicReport) must NEVER carry this — see that
		// function's own comment and TestPublicPayloadCarriesNothingExtra,
		// which pins its exact key set.
		shared := row.ShareToken != nil && *row.ShareToken != ""
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			// See reportWithPiece: a writing report stored before `piece`
			// existed gets it filled in from her draft here, so her own
			// finished screen opens on the article like a fresh one does.
			"report":     json.RawMessage(a.reportWithPiece(ctx, at.ID, row.Report)),
			"shared":     shared,
			"shareToken": row.ShareToken,
			// The star she gave this reading's EXPERIENCE (0107), so the
			// scorer at the foot of the report comes back filled in rather
			// than asking her again every time she opens it. `null` when she
			// has not answered — never 0, which would read as one star.
			//
			// AUTHENTICATED payload only. getPublicReport must never carry
			// this: it is private feedback about us, addressed to us.
			"rating": at.ExperienceRating,
		})
	}
}
