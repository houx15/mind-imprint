package api

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

// writing_snippets.go — Task 6 of the lite writing phase: 段落 (paragraphs).
// Three routes:
//   - GET  /writings/{id}/snippets                — list her fragments.
//   - PUT  /writings/{id}/snippets                — upsert one or more
//     fragments (she writes "one fragment at a time", so a PUT usually
//     carries a single item; a small posted array is also accepted, each
//     item independently upserted by position — writing_snippet's natural
//     key, per UpsertWritingSnippet's own comment). Unlike outline's PUT,
//     this is NOT a full replace: positions absent from the posted array are
//     left untouched, because there is no bulk-replace query for snippets
//     (only a single-row upsert) and, more importantly, no product reason to
//     wipe fragment 3 because she is only saving fragment 1 right now.
//   - POST /writings/{id}/snippets/{sid}/exemplar  — English-only: generate a
//     DEMONSTRATION paragraph plus a few guiding questions for the snippet at
//     {sid}. This is the task's 铁律① pressure point (see the file-level
//     comment on generateWritingSnippetExemplar below).
//
// Named writing_snippets.go / getWritingSnippets / putWritingSnippets /
// writingSnippetDTO throughout — pro already owns the plain names
// (putSnippets/getSnippets/snippetDTO, workspace_write.go) for the
// project-scoped 片段 feature. Same two-unrelated-domains-share-a-word
// situation as writing_outline.go vs pro's outline; see that file's comment
// and Ruling W-R3 (queries/writing_atom.sql vs writing.sql) generalised to
// identifiers.

// writingSnippetDTO is a PERSISTED snippet row (GET/PUT response shape).
// outlineId is nullable both in the DB (ON DELETE SET NULL, since W5's
// outline PUT is a full replace that mints fresh outline ids on every save —
// see the task's context note) and here.
//
// outlineHeading is a SEPARATE field from outlineId so a client can render
// the heading without joining the outline itself. It is resolved STRICTLY by
// id (writingSnippetHeadingByID) — never guessed by position.
//
// The reason it can be trusted: outline PUT is a full replace that mints fresh
// ids, so revising the outline while drafting (the normal thing to do) would
// otherwise detach every paragraph. That is repaired at WRITE time instead —
// relinkWritingSnippetsToOutline (writing_outline.go) re-attaches snippets by
// heading TEXT after a replace, so a surviving heading keeps its paragraphs
// wherever it moved to.
//
// "" therefore means something honest and specific: she rewrote that heading,
// so the link is genuinely gone. It never means "we could not be bothered to
// look" and never means "here is our best guess".
type writingSnippetDTO struct {
	ID             string  `json:"id"`
	OutlineID      *string `json:"outlineId"`
	OutlineHeading string  `json:"outlineHeading"`
	Position       int32   `json:"position"`
	Text           string  `json:"text"`
	UpdatedAt      string  `json:"updatedAt"`
}

// toWritingSnippetDTO renders one snippet row. outline is the writing's
// CURRENT outline rows (ListWritingOutline) — passed in rather than
// re-queried per snippet, since every caller already needs the full outline
// once to resolve OutlineHeading for every snippet, not once per row.
// OutlineHeading resolves strictly by id (writingSnippetHeadingByID), NOT via
// findWritingSnippetOutlineTopic's id-then-position fallback. The two callers
// want different things: a model prompt can absorb a loosely-wrong topic hint,
// a client-facing field cannot — see writingSnippetHeadingByID's comment.
func toWritingSnippetDTO(row sqlc.WritingSnippet, outline []sqlc.WritingOutline) writingSnippetDTO {
	out := writingSnippetDTO{
		ID: row.ID.String(), Position: row.Position, Text: row.Text,
		OutlineHeading: writingSnippetHeadingByID(outline, row),
		UpdatedAt:      row.UpdatedAt.Format(time.RFC3339),
	}
	if row.OutlineID.Valid {
		s := uuid.UUID(row.OutlineID.Bytes).String()
		out.OutlineID = &s
	}
	return out
}

// writingSnippetItemReq is PUT's per-item request shape. outlineId is
// optional and, when present, must name an outline row that ACTUALLY belongs
// to this writing (checked against ListWritingOutline below) — otherwise a
// client could link a snippet to another atom's outline id undetected.
type writingSnippetItemReq struct {
	OutlineID *string `json:"outlineId"`
	Position  int32   `json:"position"`
	Text      string  `json:"text"`
}

// getWritingSnippets is GET /api/v1/writings/{id}/snippets. Reads the CURRENT
// outline alongside the snippets so each DTO's outlineHeading can be resolved
// from the live rows — see toWritingSnippetDTO's comment.
func (a *API) getWritingSnippets(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListWritingSnippets(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	outline, err := a.d.Queries.ListWritingOutline(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]writingSnippetDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toWritingSnippetDTO(row, outline))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"snippets": out})
}

// resolveWritingSnippetOutlineID validates a posted outlineId string (if any)
// against the writing's CURRENT outline rows, returning the pgtype.UUID to
// store (invalid/absent input means "not linked" — never an error, since
// linking is optional). A non-empty string that does not match any current
// outline row is rejected outright: silently storing an unrelated uuid would
// let writing_snippet.outline_id point at a row that was never hers.
func resolveWritingSnippetOutlineID(raw *string, outline []sqlc.WritingOutline) (pgtype.UUID, *httpx.APIError) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return pgtype.UUID{Valid: false}, nil
	}
	id, err := uuid.Parse(strings.TrimSpace(*raw))
	if err != nil {
		return pgtype.UUID{}, httpx.ErrBadRequest("outline_id_invalid", "提纲编号格式不对。", nil)
	}
	for _, o := range outline {
		if o.ID == id {
			return pgtype.UUID{Bytes: id, Valid: true}, nil
		}
	}
	return pgtype.UUID{}, httpx.ErrBadRequest("outline_id_invalid", "这个提纲条目不属于这篇写作。", nil)
}

// putWritingSnippets is PUT /api/v1/writings/{id}/snippets — upsert every
// posted item by position, in ONE transaction (so a multi-item PUT either
// all lands or none does; a single-item PUT, the common case, costs nothing
// extra). Never a full replace — see the file comment.
//
// Central to this task: the ONLY source for writing_snippet.text is this
// handler's request body. Nothing in this file, or anywhere in the writing
// coach turn / outline / exemplar paths, ever writes model output here — see
// generateWritingSnippetExemplar's comment and TestWritingSnippetExemplar_
// NeverPersisted for the enforced proof.
func (a *API) putWritingSnippets(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	var body struct {
		Snippets []writingSnippetItemReq `json:"snippets"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(body.Snippets) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_snippets", "没有要保存的段落。", nil))
		return
	}
	for _, it := range body.Snippets {
		if it.Position < 0 {
			httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_position", "段落位置不对。", nil))
			return
		}
	}

	outline, err := a.d.Queries.ListWritingOutline(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Existing rows, so an omitted outlineId can PRESERVE the link she already
	// has rather than silently clearing it. She re-saves a paragraph's text
	// constantly while drafting, and a client that does not resend outlineId
	// every single time would otherwise detach the paragraph from its heading
	// on an ordinary keystroke-save — the same silent-unlink bug the outline
	// replace had, arriving by a different door. Absent means "leave it
	// alone"; there is no request today that needs to clear a link, and
	// inventing that meaning for absence costs her data.
	existing, err := a.d.Queries.ListWritingSnippets(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	outlineIDByPosition := make(map[int32]pgtype.UUID, len(existing))
	for _, row := range existing {
		outlineIDByPosition[row.Position] = row.OutlineID
	}

	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	for _, it := range body.Snippets {
		outlineID, verr := resolveWritingSnippetOutlineID(it.OutlineID, outline)
		if verr != nil {
			httpx.WriteError(w, r, verr)
			return
		}
		if it.OutlineID == nil {
			if prior, ok := outlineIDByPosition[it.Position]; ok {
				outlineID = prior
			}
		}
		if _, err := qtx.UpsertWritingSnippet(r.Context(), sqlc.UpsertWritingSnippetParams{
			AtomID: at.ID, OutlineID: outlineID, Position: it.Position, Text: strings.TrimSpace(it.Text),
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	rows, err := a.d.Queries.ListWritingSnippets(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// `outline` was already read above (to validate each posted outlineId) and
	// this PUT never writes to writing_outline itself, so it is still exactly
	// what a fresh ListWritingOutline would return — reused rather than
	// re-queried for the DTO's outlineHeading.
	out := make([]writingSnippetDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toWritingSnippetDTO(row, outline))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"snippets": out})
}

// loadOwnedWritingSnippet resolves {sid} to a writing_snippet row that
// belongs to atomID, mirroring loadOwnedAtomCard's shape (reading_cards.go):
// bad uuid or a row from a different atom both come back as the same 404 —
// never a 403 that would leak whether the id exists at all.
func (a *API) loadOwnedWritingSnippet(w http.ResponseWriter, r *http.Request, atomID uuid.UUID) (sqlc.WritingSnippet, bool) {
	sid, err := uuid.Parse(r.PathValue("sid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.WritingSnippet{}, false
	}
	rows, err := a.d.Queries.ListWritingSnippets(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return sqlc.WritingSnippet{}, false
	}
	for _, row := range rows {
		if row.ID == sid {
			return row, true
		}
	}
	httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
	return sqlc.WritingSnippet{}, false
}

// writingExemplarGenSystem instructs the model to produce ONLY a JSON object
// {"exemplar":"...","prompts":["...",...]}. Both fields are DEMONSTRATION
// material: the exemplar is a paragraph to learn from, never text handed
// back to her draft, and the prompts are guiding questions, not chat. The
// prompt says this explicitly — 铁律① is at its sharpest right here, because
// this is the one lite endpoint whose whole job is to produce model-written
// English prose.
const writingExemplarGenSystem = `你是「印记」，帮国际课程的学生练习英文写作。这次请求的是一段"示范段落"——目的是让她看看一段高质量的英文段落是怎么组织、怎么论证的，从而自己动笔写自己的版本。这不是替她写作文，她绝不会把这段话直接粘贴进自己的稿子，所以你可以放心按最好的英文写作水准来写，不用刻意降低质量。

请只根据我给你的题目 / 这一段对应的提纲要点 / 目标字数（如果有），做两件事：
1. 用英文写一段完整、有说服力、结构清晰的示范段落（通常 4-8 句），紧扣给定的提纲要点，不要编造她没提过的具体私人经历或数据来源。
2. 再给她 2-4 条用来写自己这一段的引导问题（中文），提示她可以怎么切入、怎么摆证据、怎么收尾——这些只是问题，不是段落正文，不能写成另一段示范。

只输出一个 JSON 对象，形如 {"exemplar":"...","prompts":["...","..."]}，不要输出对象以外的任何文字、解释或代码块标记。exemplar 字段只能是英文；prompts 是字符串数组，每条一句话。`

// findWritingSnippetOutlineTopic looks up the outline text this snippet is
// meant to develop: first by outline_id (the durable link, when it survived
// an outline replace), falling back to matching by position (best-effort —
// W5's outline PUT is a full replace that mints fresh ids, so an older link
// can go stale; see the task's context note). Returns "" if neither resolves,
// which the prompt builder treats as "no specific topic given" rather than
// inventing one — same "degrade honestly" posture as Ruling W-R7.
func findWritingSnippetOutlineTopic(outline []sqlc.WritingOutline, snippet sqlc.WritingSnippet) string {
	if snippet.OutlineID.Valid {
		for _, o := range outline {
			if o.ID == uuid.UUID(snippet.OutlineID.Bytes) {
				return o.Text
			}
		}
	}
	for _, o := range outline {
		if o.Position == snippet.Position {
			return o.Text
		}
	}
	return ""
}

// buildWritingExemplarPrompt assembles the user turn: title, target words
// (only if set, never invented — W-R7), the topic this paragraph is meant to
// cover (if resolvable), and whatever she has already written in this slot
// (context only — the exemplar is never asked to finish HER sentence, just
// to demonstrate a paragraph on the same topic).
func buildWritingExemplarPrompt(wr sqlc.Writing, topic string, existingText string) string {
	var b strings.Builder
	if t := strings.TrimSpace(wr.Title); t != "" {
		b.WriteString("题目：" + t + "\n")
	}
	if wr.TargetWords != nil {
		b.WriteString("目标字数：约 ")
		b.WriteString(strconv.Itoa(int(*wr.TargetWords)))
		b.WriteString(" 字（仅供参考）\n")
	}
	if topic = strings.TrimSpace(topic); topic != "" {
		b.WriteString("这一段对应的提纲要点：" + topic + "\n")
	} else {
		b.WriteString("提纲要点未知，请围绕题目本身给一个通用但扎实的示范段落。\n")
	}
	if existingText = strings.TrimSpace(existingText); existingText != "" {
		b.WriteString("她目前在这一段已经写的内容（仅供参考，示范段落不需要照着补完它，只是同一个话题的另一个完整版本）：\n" + existingText + "\n")
	}
	return b.String()
}

// writingExemplarGenResult is the model's expected JSON reply shape.
type writingExemplarGenResult struct {
	Exemplar string   `json:"exemplar"`
	Prompts  []string `json:"prompts"`
}

// extractWritingExemplarJSONObject strips code fences and clamps to the
// outermost '{'..'}' — the object-shaped sibling of writing_outline.go's
// extractWritingOutlineJSONArray, kept separate because the two callers parse
// different JSON shapes (an object here, an array there) and there is no
// shared logic worth factoring beyond "trim fences, clamp brackets", which
// differs only in which bracket character it clamps to.
func extractWritingExemplarJSONObject(text string) string {
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
	return strings.TrimSpace(c)
}

// parseWritingExemplarGenResult defensively decodes the model's JSON object.
// An empty/malformed reply, or one whose exemplar field is blank, returns ""
// so the caller treats it as a failure (502 ai_dialogue_failed) rather than
// handing back a hollow "demonstration" with nothing in it.
func parseWritingExemplarGenResult(text string) (string, []string) {
	c := extractWritingExemplarJSONObject(text)
	if c == "" {
		return "", nil
	}
	var raw writingExemplarGenResult
	if err := json.Unmarshal([]byte(c), &raw); err != nil {
		return "", nil
	}
	exemplar := strings.TrimSpace(raw.Exemplar)
	if exemplar == "" {
		return "", nil
	}
	prompts := make([]string, 0, len(raw.Prompts))
	for _, p := range raw.Prompts {
		p = strings.TrimSpace(p)
		if p != "" {
			prompts = append(prompts, p)
		}
	}
	return exemplar, prompts
}

// generateWritingSnippetExemplar is POST
// /api/v1/writings/{id}/snippets/{sid}/exemplar — the 铁律① pressure point
// this task exists to prove: it is the only lite endpoint whose job is to
// produce model-written English prose, so every design choice here defends
// the line rather than crosses it.
//
//   - English only: `writing.lang != "en"` is refused with 400
//     `exemplar_not_available` BEFORE any model call — no metering, no spend,
//     for a request the product does not offer at all.
//   - The response carries the exemplar and the guiding prompts in fields of
//     their own ({exemplar, prompts}) — structurally distinct from
//     writingSnippetDTO's {id, outlineId, position, text, updatedAt}, so a
//     client cannot confuse "a demonstration to read" with "a persisted
//     fragment" from the shape alone, the same distinguishing move
//     writingOutlineCandidateDTO makes for outline generation.
//   - NEITHER field is ever written to writing_snippet, writing_draft, or any
//     other table: this handler calls UpsertWritingSnippet nowhere, and does
//     not touch writing_draft at all (that table belongs to Task 7's
//     compose). It only reads (GetWriting, ListWritingOutline, the snippet
//     row itself) and returns the model's JSON reply verbatim in the HTTP
//     response. TestWritingSnippetExemplar_NeverPersisted asserts this
//     against the database directly — the single most important test this
//     phase has, per the task brief.
func (a *API) generateWritingSnippetExemplar(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	snippet, ok := a.loadOwnedWritingSnippet(w, r, at.ID)
	if !ok {
		return
	}
	wr, err := a.d.Queries.GetWriting(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if wr.Lang != "en" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("exemplar_not_available", "示范段落仅支持英文写作。", nil))
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

	// Run to completion even if she navigates away mid-generate — same
	// reasoning as generateWritingOutline / postLiteWritingTurn: a
	// synchronous POST is cancelled by net/http the instant the browser
	// disconnects, and a refresh mid-call would otherwise abort the model
	// call AND the metering row — money spent, nothing recorded.
	turnCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 150*time.Second)
	defer cancel()

	outline, err := a.d.Queries.ListWritingOutline(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	topic := findWritingSnippetOutlineTopic(outline, snippet)
	prompt := buildWritingExemplarPrompt(wr, topic, snippet.Text)

	// §model-routing: a demonstration paragraph is reviewer-tier work — it
	// needs to actually read as good English writing, the same "faithful,
	// never downgrade" reasoning generateWritingOutline applies (mirroring
	// workspace_plan_generate.go's regeneratePlan) — so this resolves
	// EvalResolver (flagship) rather than writing_turn.go's chaperone
	// ChatResolver.
	resolved, ok := a.resolveEval(turnCtx)
	if !ok {
		slog.Warn("writing snippet exemplar: no provider resolved",
			"atom_id", at.ID, "snippet_id", snippet.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	res, cerr := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: writingExemplarGenSystem},
			{Role: gateway.RoleUser, Content: prompt},
		},
	})

	// Meter BEFORE any bail — a call that reached the provider cost money
	// whatever happens to its reply, mirroring writing_outline.go exactly.
	a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "exemplar_gen", resolved, res.Usage)

	if cerr != nil {
		slog.Warn("writing snippet exemplar: provider call failed",
			"err", cerr, "atom_id", at.ID, "snippet_id", snippet.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	exemplar, prompts := parseWritingExemplarGenResult(res.Text)
	if exemplar == "" {
		slog.Warn("writing snippet exemplar: model reply unparseable or empty",
			"atom_id", at.ID, "snippet_id", snippet.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	// NEVER written to writing_snippet (or any other table) — see the
	// handler's file comment. Returned verbatim, in fields of their own,
	// structurally separate from writingSnippetDTO.
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"exemplar": exemplar, "prompts": prompts})
}

// writingSnippetHeadingByID resolves a snippet's outline heading for the
// CLIENT-facing DTO, strictly by id — no position fallback, deliberately.
//
// findWritingSnippetOutlineTopic (above) does fall back to position, and that
// is fine where it is used: a loosely-wrong topic hint inside a model prompt
// costs a slightly-off exemplar. Putting the same guess in the DTO is a
// different thing entirely. Positions are array indices, so the moment she
// REORDERS her outline or deletes a middle point, position-matching would show
// paragraph 1 the heading belonging to paragraph 2 — specific, confident and
// wrong, with nothing in the payload marking it as a guess. Showing her the
// wrong heading is worse than showing none.
//
// The link itself is repaired properly at write time instead:
// relinkWritingSnippetsToOutline (writing_outline.go) re-attaches snippets by
// heading TEXT after an outline replace, so an id match here is a real match.
// When the id does not resolve, she genuinely rewrote that heading, and the
// honest answer is silence — the same "omit, never estimate" rule the report
// generator follows.
func writingSnippetHeadingByID(outline []sqlc.WritingOutline, row sqlc.WritingSnippet) string {
	if !row.OutlineID.Valid {
		return ""
	}
	want := uuid.UUID(row.OutlineID.Bytes)
	for _, o := range outline {
		if o.ID == want {
			return o.Text
		}
	}
	return ""
}
