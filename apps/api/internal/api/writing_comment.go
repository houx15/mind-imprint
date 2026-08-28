package api

// writing_comment.go — B4 + B7: comments that can only point at sentences
// she wrote.
//
// 印记's critique of a draft was already the strongest thing in this room
// (writingReviewSystem, the pre-Task-5 shape of reviewWritingDraft) — but it
// rendered ONCE as a wall of prose and evaporated on the next navigation:
// POST /review returned {"feedback":"<prose>"} and wrote it nowhere. This
// file turns that critique into a structured, PERSISTED object — a one-line
// summary plus a handful of concrete points — at two zoom levels sharing one
// shape: a comment on ONE paragraph (scope='block', snippet_id set, via
// commentOnSnippet) and a comment on the WHOLE draft (scope='draft',
// snippet_id null, via reviewWritingDraft in writing_compose.go). Both
// persist through the same CreateWritingComment / writing_comment table
// (migration 0102).
//
// THE LOAD-BEARING GUARANTEE lives in validateCommentPoints below: every
// point's quote must appear LITERALLY in the text being commented on, or it
// is dropped — never re-prompted, never fuzzy-matched, never rendered with a
// best guess. This is the same discipline pro applies to its evaluation
// report's references (ValidateRefs): it makes a fabricated quote
// structurally impossible to render, rather than merely discouraged by a
// prompt. A trace that lands on the neighbouring sentence is worse than one
// fewer point — so the prompt asks nicely (writingCommentSystem's
// 逐字照抄，不要改标点) but validateCommentPoints is what actually holds the
// line.

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// CommentPoint is one concrete point in a comment: text explains why the
// quoted sentence matters, Quote is that sentence verbatim from her own
// writing. A CommentPoint whose Quote cannot be found in the source it was
// generated against is not a CommentPoint this room will ever render — see
// validateCommentPoints.
type CommentPoint struct {
	Text  string `json:"text"`
	Quote string `json:"quote"`
}

// Comment is the wire AND stored shape at both zoom levels: SnippetID set +
// Scope="block" is a comment on one paragraph; SnippetID nil + Scope="draft"
// is a comment on the whole piece. One shape, two zoom levels — the same
// move migration 0102's comment makes ("one shape at two zoom levels").
type Comment struct {
	ID        string         `json:"id"`
	Scope     string         `json:"scope"`
	SnippetID *string        `json:"snippetId"`
	Summary   string         `json:"summary"`
	Points    []CommentPoint `json:"points"`
	CreatedAt string         `json:"createdAt"`
}

// toCommentDTO decodes a stored writing_comment row into the wire shape.
// Points is jsonb; a row whose points fail to decode (should not happen —
// this file is the only writer, and it always marshals what
// validateCommentPoints returned) degrades to an empty slice rather than
// failing the whole response, the same "an old/odd row must not 500 the
// list" posture writingGuideDTOOf's belt-and-braces id skip takes.
func toCommentDTO(row sqlc.WritingComment) Comment {
	points := []CommentPoint{}
	if len(row.Points) > 0 {
		if err := json.Unmarshal(row.Points, &points); err != nil {
			points = []CommentPoint{}
		}
	}
	out := Comment{
		ID:        row.ID.String(),
		Scope:     row.Scope,
		Summary:   row.Summary,
		Points:    points,
		CreatedAt: row.CreatedAt.Format(time.RFC3339),
	}
	if row.SnippetID.Valid {
		s := uuid.UUID(row.SnippetID.Bytes).String()
		out.SnippetID = &s
	}
	return out
}

// validateCommentPoints keeps only the points whose quote appears LITERALLY
// in the text being commented on.
//
// This is the same discipline pro applies to its evaluation report's
// references (ValidateRefs), and it is a structural guarantee rather than a
// request to the model: whatever it invents, a comment can only ever point
// at a sentence she actually wrote. Dropping is right and re-prompting is
// wrong — a fuzzy match that lands on the neighbouring sentence is worse
// than one fewer point.
func validateCommentPoints(points []CommentPoint, source string) []CommentPoint {
	out := make([]CommentPoint, 0, len(points))
	for _, p := range points {
		q := strings.TrimSpace(p.Quote)
		if q == "" || !strings.Contains(source, q) {
			continue
		}
		p.Quote = q
		out = append(out, p)
	}
	return out
}

// writingCommentSystem instructs the model to comment on a piece of her
// writing — one paragraph or the whole draft — with a one-line overall
// judgment plus concrete points, each anchored to a real sentence.
//
// writingGuideTeachingRules (writing_guide.go) is Task 3's four-part "怎么说话"
// doctrine, copied verbatim rather than re-derived: an engineer reading these
// tasks out of order must not find two different versions of how 印记 is
// supposed to talk.
//
// 逐字照抄，不要改标点 asks the model to do the validator's job for it — but
// the validator, not this sentence, is what actually makes a fabricated
// quote impossible to render.
const writingCommentSystem = `你是「印记」，正在给学生已经写的文字提意见——可能是她正在写的一段，也可能是她写完的整篇稿子。你的任务只是给反馈，绝不是替她改：不要重写、不要润色、不要续写、不要给出可以直接复制粘贴替换的句子或段落。

` + writingGuideTeachingRules + `

请先用一句话说清你的整体判断（summary），再从结构、论证是否站得住、证据是否充分、语言是否清楚这些角度给出若干条具体的意见（points）。如果某部分已经写得不错，也可以肯定，但不要泛泛而谈。每一条意见都必须挂在她原文里的**一句真实存在的话**上：quote 字段必须逐字照抄那句话，包括标点，一个字都不能改；text 字段说清这句话为什么是问题、为什么值得注意，或者写得好在哪里。

输出 JSON：{"summary":"…","points":[{"text":"…","quote":"…"}]}
- summary：一句话，整体判断。
- points：2-6 条，每条都要有 quote（她原文里逐字存在的一句话，包含标点）和 text（针对这句话的具体意见，不是建议她改写成什么样子）。

只输出一个 JSON 对象，不要输出对象以外的任何文字或代码块标记。`

// buildWritingCommentPrompt assembles the user turn shared by both zoom
// levels: title, target words (only if set, never invented — same W-R7
// discipline as buildWritingExemplarPrompt), then the text itself under a
// caller-supplied label ("她写的这一段" vs "她的整篇稿子") so the model knows
// which zoom level it is looking at.
func buildWritingCommentPrompt(wr sqlc.Writing, label, text string) string {
	var b strings.Builder
	if t := strings.TrimSpace(wr.Title); t != "" {
		b.WriteString("题目：" + t + "\n")
	}
	if wr.TargetWords != nil {
		b.WriteString("目标字数：约 " + strconv.Itoa(int(*wr.TargetWords)) + " 字（仅供参考）\n")
	}
	b.WriteString("\n" + label + "：\n" + text + "\n")
	return b.String()
}

// writingCommentResult is the model's expected JSON reply shape, decoded
// before validateCommentPoints ever runs — this parser only checks that the
// reply is well-formed JSON with a non-empty summary; it does not (cannot)
// validate quotes, since it has no access to the source text they must
// appear in.
type writingCommentResult struct {
	Summary string         `json:"summary"`
	Points  []CommentPoint `json:"points"`
}

// parseWritingComment decodes and lightly sanity-checks the model's reply.
// Reuses extractWritingExemplarJSONObject (writing_snippets.go) — same
// "strip fences, clamp to the outermost {..}" extraction every JSON-replying
// prompt in this package already shares.
func parseWritingComment(text string) (writingCommentResult, bool) {
	c := extractWritingExemplarJSONObject(text)
	if c == "" {
		return writingCommentResult{}, false
	}
	var got writingCommentResult
	if err := json.Unmarshal([]byte(c), &got); err != nil {
		return writingCommentResult{}, false
	}
	if strings.TrimSpace(got.Summary) == "" {
		return writingCommentResult{}, false
	}
	return got, true
}

// commentOnSnippet is POST /api/v1/writings/{id}/snippets/{sid}/comment — a
// spend endpoint (one model call), metered as purpose="block_comment".
// Follows guideWritingBlock's shape exactly: loadOwnedWritingAtom →
// loadOwnedWritingSnippet → HasEntitlement → 150s timeout → resolveEval →
// gateway.Collect → recordLiteLLMCall → parse → validate → persist →
// httpx.WriteJSON.
//
// Points are validated against THIS SNIPPET's text, not the whole draft — a
// block comment must never be able to point at a sentence living in a
// different paragraph.
func (a *API) commentOnSnippet(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	snippet, ok := a.loadOwnedWritingSnippet(w, r, at.ID)
	if !ok {
		return
	}
	source := strings.TrimSpace(snippet.Text)
	if source == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_text", "这一段还没有内容，先写点什么再来看看。", nil))
		return
	}

	u, _ := UserFromContext(r.Context())
	entitled, eerr := HasEntitlement(r.Context(), u)
	if eerr != nil {
		httpx.WriteError(w, r, eerr)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	// Run to completion even if she navigates away mid-call — same reasoning
	// and same 150s cap as every other spend endpoint in this room.
	turnCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 150*time.Second)
	defer cancel()

	wr, err := a.d.Queries.GetWriting(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// §model-routing: judging whether an argument holds up is reviewer-tier
	// work, the same "faithful, never downgrade" reasoning every other
	// judgment call in this file's neighbourhood applies — resolves
	// EvalResolver (flagship), not writing_turn.go's chaperone ChatResolver.
	resolved, ok2 := a.resolveEval(turnCtx)
	if !ok2 {
		slog.Warn("writing block comment: no provider resolved",
			"atom_id", at.ID, "snippet_id", snippet.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	res, cerr := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: writingCommentSystem},
			{Role: gateway.RoleUser, Content: buildWritingCommentPrompt(wr, "她写的这一段", source)},
		},
	})
	a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "block_comment", resolved, res.Usage)
	if cerr != nil {
		slog.Warn("writing block comment: provider call failed", "err", cerr,
			"atom_id", at.ID, "snippet_id", snippet.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	parsed, okParse := parseWritingComment(res.Text)
	if !okParse {
		slog.Warn("writing block comment: reply unparseable or empty summary",
			"atom_id", at.ID, "snippet_id", snippet.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	points := validateCommentPoints(parsed.Points, source)
	payload, merr := json.Marshal(points)
	if merr != nil {
		slog.Warn("writing block comment: marshal points failed", "err", merr,
			"atom_id", at.ID, "snippet_id", snippet.ID)
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	row, serr := a.d.Queries.CreateWritingComment(turnCtx, sqlc.CreateWritingCommentParams{
		AtomID:    at.ID,
		SnippetID: pgtype.UUID{Bytes: snippet.ID, Valid: true},
		Scope:     "block",
		Summary:   strings.TrimSpace(parsed.Summary),
		Points:    payload,
	})
	if serr != nil {
		httpx.WriteError(w, r, serr)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"comment": toCommentDTO(row)})
}

// listWritingComments is GET /api/v1/writings/{id}/comments — every stored
// comment for this writing, both scopes together, newest first
// (ListWritingComments already orders by created_at DESC). No entitlement
// gate — no model call, no spend, same reasoning as every other GET in this
// room.
func (a *API) listWritingComments(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListWritingComments(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]Comment, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCommentDTO(row))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"comments": out})
}
