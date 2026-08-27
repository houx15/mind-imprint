package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// writing_outline.go — Task 5 of the lite writing phase: 大纲 (outline). Three
// routes:
//   - GET  /writings/{id}/outline           — read the confirmed outline.
//   - PUT  /writings/{id}/outline           — full replace (學生可改), same
//     semantics as pro's putOutline (workspace_write.go): wipe everything,
//     reinsert the posted array in order, position = array index, depth
//     clamped to 0..2.
//   - POST /writings/{id}/outline/generate  — derive a CANDIDATE outline from
//     what she has already said in 构思 (atom_message role='student', plus
//     the title/target words). One model call, deterministic assembly around
//     it. Per 铁律①'s scope (AGENTS.md: the rule is about her PROSE, not
//     system-derived structure) this is explicitly allowed — but the result
//     is a proposal she confirms via PUT, never auto-persisted here.
//
// Named writing_outline.go / getWritingOutline / putWritingOutline /
// generateWritingOutline / writingOutlineItemDTO throughout — pro already
// owns the plain names (listOutline/putOutline/outlineNodeDTO,
// workspace_write.go) for the project-scoped essay+提案 outline. Two
// unrelated "outline" domains share a word, not a table or a handler; see
// Ruling W-R3's file-path lesson (queries/writing_atom.sql vs writing.sql)
// generalised to identifiers.
//
// clampDepth (workspace_write.go) IS reused as-is — it's a pure, stateless
// 0..2 clamp with the exact same semantics this task's brief asks for
// ("depth clamped to 0..2"), so calling the existing helper is reuse, not a
// second definition; nothing about pro's behaviour changes by another
// package-internal caller using it.

// writingOutlineItemDTO is a PERSISTED outline row: id + position identify a
// real writing_outline row (GET/PUT response shape).
type writingOutlineItemDTO struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Depth    int32  `json:"depth"`
	Position int32  `json:"position"`
}

func toWritingOutlineItemDTO(row sqlc.WritingOutline) writingOutlineItemDTO {
	return writingOutlineItemDTO{
		ID: row.ID.String(), Text: row.Text, Depth: row.Depth, Position: row.Position,
	}
}

// writingOutlineItemReq is PUT's per-item request shape (no id/position — the
// whole array is a full replace, ids are always freshly minted and position
// is always the array index, mirroring pro's putOutline).
type writingOutlineItemReq struct {
	Text  string `json:"text"`
	Depth int32  `json:"depth"`
}

// writingOutlineCandidateDTO is what /outline/generate returns: text+depth
// only, deliberately WITHOUT an id or position — structurally distinct from
// writingOutlineItemDTO so a client (and a test) can tell "this is a
// candidate she hasn't confirmed yet" apart from "this is a real, persisted
// row" just from the shape, without needing a separate status flag.
type writingOutlineCandidateDTO struct {
	Text  string `json:"text"`
	Depth int32  `json:"depth"`
}

// getWritingOutline is GET /api/v1/writings/{id}/outline.
func (a *API) getWritingOutline(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListWritingOutline(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]writingOutlineItemDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toWritingOutlineItemDTO(row))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"outline": out})
}

// buildWritingOutlineArrays converts the posted item array into the three
// PARALLEL arrays ReplaceWritingOutline's data-modifying CTE unnest()s in
// lockstep (texts[i]/depths[i]/positions[i] together describe outline row i;
// see writing_atom.sql's comment on why delete+insert is ONE statement).
// Built in a single loop over the SAME source slice, so the three results are
// equal length by construction — position is simply the loop index, and depth
// is clamped here (0..2) rather than left to the caller.
func buildWritingOutlineArrays(items []writingOutlineItemReq) (texts []string, depths []int32, positions []int32) {
	texts = make([]string, len(items))
	depths = make([]int32, len(items))
	positions = make([]int32, len(items))
	for i, it := range items {
		texts[i] = strings.TrimSpace(it.Text)
		depths[i] = clampDepth(it.Depth)
		positions[i] = int32(i)
	}
	return texts, depths, positions
}

// validateWritingOutlineArrayLengths is a hard guard before ReplaceWritingOutline
// runs: Task 1's review of that query found that Postgres NULL-pads the
// shorter of the three unnest()ed arrays and (because every column is NOT
// NULL) a length mismatch fails loudly INSIDE the single statement — an
// acceptable failure mode at the store layer, but a raw constraint violation
// would surface to the client as an opaque 500. buildWritingOutlineArrays
// above can never actually produce mismatched slices (all three are built
// from one loop over one source slice), so in production this check never
// fires — it exists so that stays true by an assertion, not merely by
// happenstance, and so any future caller that assembles the three arrays a
// different way gets a clean 400 instead of a database error.
func validateWritingOutlineArrayLengths(texts []string, depths []int32, positions []int32) error {
	if len(texts) != len(depths) || len(texts) != len(positions) {
		return httpx.ErrBadRequest("outline_array_length_mismatch", "提纲数据格式不对，请重试。", nil)
	}
	return nil
}

// putWritingOutline is PUT /api/v1/writings/{id}/outline — full replace, not
// a merge. Every existing row is deleted and the posted array reinserted in
// order (position = array index); an empty posted array clears the outline
// entirely. Re-reads via ListWritingOutline afterward (rather than trusting
// ReplaceWritingOutline's own RETURNING order) so the response is exactly
// what a subsequent GET would return, ordered by position — mirrors pro's
// putOutline re-read for the same reason.
func (a *API) putWritingOutline(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	var body struct {
		Outline []writingOutlineItemReq `json:"outline"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	texts, depths, positions := buildWritingOutlineArrays(body.Outline)
	if verr := validateWritingOutlineArrayLengths(texts, depths, positions); verr != nil {
		httpx.WriteError(w, r, verr)
		return
	}

	// Capture the OLD outline before it is replaced, so the snippets she has
	// already written can be re-attached to their headings afterwards. Read
	// before the replace or the mapping is gone: ReplaceWritingOutline deletes
	// the old rows, and writing_snippet.outline_id is ON DELETE SET NULL.
	oldOutline, err := a.d.Queries.ListWritingOutline(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	oldSnippets, err := a.d.Queries.ListWritingSnippets(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	if _, err := a.d.Queries.ReplaceWritingOutline(r.Context(), sqlc.ReplaceWritingOutlineParams{
		AtomID: at.ID, Texts: texts, Depths: depths, Positions: positions,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	rows, err := a.d.Queries.ListWritingOutline(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	relinkWritingSnippetsToOutline(r.Context(), a, at.ID, oldOutline, oldSnippets, rows)

	out := make([]writingOutlineItemDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toWritingOutlineItemDTO(row))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"outline": out})
}

// writingOutlineGenRuneBudget bounds how much of her 构思 material feeds the
// generation prompt — lite has no compaction layer (same reasoning as
// writingTurnsWindow, writing_turn.go), so this is the only thing bounding
// prompt growth on a long 构思 thread. Generously larger than the coach
// turn's window because this is a single one-shot call, not a per-turn cost,
// and the whole point is to see everything she has said so far.
const writingOutlineGenRuneBudget = 6000

// writingOutlineGenSystem instructs the model to produce ONLY a JSON array of
// {"text","depth"} outline items, grounded strictly in what the student
// already said. This is the 铁律① line this task sits on (see AGENTS.md's
// "适用边界" paragraph): deriving STRUCTURE from her own material is a
// deterministic system step, never body prose, and the prompt says so
// explicitly rather than leaving it implicit.
const writingOutlineGenSystem = `你是「印记」。学生正在写作，下面会给你她自己在构思阶段说过的话（她的原始想法、角度、素材）、她定的题目，以及目标字数（如果她定了的话）。

请只根据她自己已经说过的内容，给她拟一份可编辑的提纲候选——这是给她看、由她自己确认或修改的候选，不是最终定稿，你也绝不能替她把正文写出来。

规则：
- 只用她自己提到过的内容和角度来搭结构，不要凭空编她没说过的论点、事实或例子。
- 输出的是标题/要点式的提纲条目（每条一句话概括这一部分要讲什么），不是段落正文，不写完整论证内容。
- 如果给了目标字数，把它当作提纲"颗粒度"的参考——字数多，条目和层级可以更丰富；字数少，提纲要精简。没给目标字数也要正常生成，不要因此拒绝或留空。
- depth 用 0/1/2 表示层级（0=一级要点，1/2=其下的子要点），条目数量和层级随她已经说的内容的丰富程度而定，不要套用固定模板凑数。

只输出一个 JSON 数组，每个元素形如 {"text":"...","depth":0}，不要输出数组以外的任何文字、解释或代码块标记。`

// buildWritingOutlineGenPrompt assembles the user turn: title, target words
// (only if set — never invented, per Ruling W-R7), then every role='student'
// atom_message in chronological order, tail-kept to the rune budget above.
// AI turns are deliberately excluded — the brief is explicit that the
// outline is derived from what SHE said, not from the coach's own questions
// or restatements.
func buildWritingOutlineGenPrompt(wr sqlc.Writing, msgs []sqlc.AtomMessage) string {
	var b strings.Builder
	if t := strings.TrimSpace(wr.Title); t != "" {
		fmt.Fprintf(&b, "题目/想法：%s\n", t)
	}
	if wr.TargetWords != nil {
		fmt.Fprintf(&b, "目标字数：约 %d 字（仅作提纲颗粒度的参考，不是硬性要求）\n", *wr.TargetWords)
	}

	lines := make([]string, 0, len(msgs))
	for _, m := range msgs {
		if m.Role != "student" {
			continue
		}
		text := strings.TrimSpace(m.Content)
		if text == "" {
			continue
		}
		lines = append(lines, text)
	}
	start := 0
	total := 0
	for i := len(lines) - 1; i >= 0; i-- {
		total += len([]rune(lines[i]))
		if total > writingOutlineGenRuneBudget {
			start = i + 1
			break
		}
	}

	b.WriteString("\n【学生在构思阶段说过的话（按时间顺序，只有她自己说的，不含 AI 的提问或回复）】\n")
	for _, l := range lines[start:] {
		b.WriteString("- " + l + "\n")
	}
	return b.String()
}

// extractWritingOutlineJSONArray strips code fences and clamps to the
// outermost '['..']' — the same defensive decoding shape as
// workspace_plan_generate.go's parsePlanItems, kept as a SEPARATE local
// function (not a shared helper) because the two callers parse unrelated
// JSON contracts (plan tasks vs. outline items) and pro's function is
// unexported to its own file for its own type.
func extractWritingOutlineJSONArray(text string) string {
	c := strings.TrimSpace(text)
	if strings.HasPrefix(c, "```json") {
		c = strings.TrimLeft(strings.TrimPrefix(c, "```json"), " \t\r\n")
	} else if strings.HasPrefix(c, "```") {
		c = strings.TrimLeft(c[3:], " \t\r\n")
	}
	if strings.HasSuffix(c, "```") {
		c = strings.TrimRight(c[:len(c)-3], " \t\r\n")
	}
	if i := strings.IndexByte(c, '['); i > 0 {
		c = c[i:]
	}
	if j := strings.LastIndexByte(c, ']'); j >= 0 && j < len(c)-1 {
		c = c[:j+1]
	}
	return strings.TrimSpace(c)
}

// writingOutlineGenMaxItems caps a generated candidate to a sane size — a
// model that runs away with the JSON array should not hand the student an
// unusable 200-item outline.
const writingOutlineGenMaxItems = 40

// parseWritingOutlineGenItems defensively decodes the model's JSON array.
// Empty/malformed input returns nil so the caller treats it as a failure
// (502 ai_dialogue_failed) rather than handing back an empty "candidate"
// that looks like a successful, if useless, generation.
func parseWritingOutlineGenItems(text string) []writingOutlineCandidateDTO {
	c := extractWritingOutlineJSONArray(text)
	if c == "" {
		return nil
	}
	var raw []writingOutlineCandidateDTO
	if err := json.Unmarshal([]byte(c), &raw); err != nil {
		return nil
	}
	out := make([]writingOutlineCandidateDTO, 0, len(raw))
	for _, it := range raw {
		t := strings.TrimSpace(it.Text)
		if t == "" {
			continue
		}
		out = append(out, writingOutlineCandidateDTO{Text: t, Depth: clampDepth(it.Depth)})
		if len(out) >= writingOutlineGenMaxItems {
			break
		}
	}
	return out
}

// generateWritingOutline is POST /api/v1/writings/{id}/outline/generate. A
// spend endpoint (one model call): gates on HasEntitlement, meters
// purpose="outline_gen" BEFORE any bail, and — per the brief — NEVER writes
// to writing_outline itself. It returns a candidate; the student confirms
// (or edits) it via PUT. A model failure or an unparseable/empty reply both
// surface as the honest 502 ai_dialogue_failed, never a canned/deterministic
// fallback outline (unlike workspace_plan_generate.go's defaultPlanItems,
// which predates the 2026-08-25 "AI failure must be surfaced, never masked"
// rule this phase holds every writing endpoint to — see writing_turn.go).
func (a *API) generateWritingOutline(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
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

	// Run to completion even if she navigates away mid-generate — same
	// reasoning as postLiteWritingTurn: a synchronous POST is cancelled by
	// net/http the instant the browser disconnects, and a refresh mid-call
	// would otherwise abort the model call AND the metering row — money
	// spent, nothing recorded.
	turnCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 150*time.Second)
	defer cancel()

	wr, err := a.d.Queries.GetWriting(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	msgs, err := a.d.Queries.ListAtomMessages(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	prompt := buildWritingOutlineGenPrompt(wr, msgs)

	// §model-routing: deriving her actual outline structure needs to be
	// faithful to what she said, not merely conversational — the same
	// reviewer-tier reasoning workspace_plan_generate.go's regeneratePlan
	// applies to plan_gen ("reviewer-tier work → flagship, never downgrade"),
	// so this resolves EvalResolver rather than writing_turn.go's chaperone
	// ChatResolver.
	resolved, ok := a.resolveEval(turnCtx)
	if !ok {
		slog.Warn("writing outline generate: no provider resolved",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	res, cerr := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: writingOutlineGenSystem},
			{Role: gateway.RoleUser, Content: prompt},
		},
	})

	// Meter BEFORE any bail — a call that reached the provider cost money
	// whatever happens to its reply, mirroring writing_turn.go exactly.
	a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "outline_gen", resolved, res.Usage)

	if cerr != nil {
		slog.Warn("writing outline generate: provider call failed",
			"err", cerr, "atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	items := parseWritingOutlineGenItems(res.Text)
	if len(items) == 0 {
		slog.Warn("writing outline generate: model reply unparseable or empty",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	// NEVER auto-persisted here — this is a candidate only. She confirms (or
	// edits it first) via PUT /outline. No write to writing_outline happens
	// on this path.
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"outline": items})
}

// relinkWritingSnippetsToOutline restores each snippet's link to its heading
// after a full outline replace, matching on the heading's TEXT.
//
// Why text and not position. Every outline save mints fresh row ids, and
// writing_snippet.outline_id is ON DELETE SET NULL, so a save silently
// detaches every paragraph she has already written — and revising the outline
// while drafting is the normal thing to do, not an edge case. The tempting
// repair is to re-attach by position, since positions are just array indices.
// That is wrong in the most damaging way available: the moment she REORDERS
// her outline (a drag, or a deleted middle point), position-matching hands
// paragraph 1 the heading that now belongs to paragraph 2 — confidently,
// specifically, and silently. Showing her the wrong heading is worse than
// showing none.
//
// Text identity is a real match, not a guess. If a heading's text survives the
// save, the paragraph written under it is genuinely still about it, wherever
// it moved to. If she REWROTE that heading, the link is honestly gone and we
// say nothing rather than invent something — the same "omit, never estimate"
// rule the report generator already follows for missing facts.
//
// Duplicate heading texts bind to the first match: two identical headings are
// indistinguishable by definition, and dropping both links would serve her
// worse than picking one.
//
// Best-effort by design: a failed relink must never fail her outline save. The
// outline is what she asked to store; the heading label is a convenience on
// top of it.
func relinkWritingSnippetsToOutline(
	ctx context.Context, a *API, atomID uuid.UUID,
	oldOutline []sqlc.WritingOutline, oldSnippets []sqlc.WritingSnippet, newOutline []sqlc.WritingOutline,
) {
	if len(oldOutline) == 0 || len(oldSnippets) == 0 || len(newOutline) == 0 {
		return
	}
	newIDByOldID := matchOutlineRows(oldOutline, newOutline)
	for _, s := range oldSnippets {
		if !s.OutlineID.Valid {
			continue
		}
		newID, ok := newIDByOldID[uuid.UUID(s.OutlineID.Bytes)]
		if !ok {
			continue // that heading is genuinely gone — say nothing, guess nothing
		}
		if err := a.d.Queries.RelinkWritingSnippetOutline(ctx, sqlc.RelinkWritingSnippetOutlineParams{
			AtomID:    atomID,
			OutlineID: pgtype.UUID{Bytes: newID, Valid: true},
			ID:        s.ID,
		}); err != nil {
			slog.Warn("lite writing: relinking snippet to outline failed",
				"err", err, "atom_id", atomID, "snippet_id", s.ID)
		}
	}
}

// matchOutlineRows pairs each OLD outline row with the NEW row that is the
// same point, so snippets can be re-attached across a full replace. Returns
// old id → new id; an old row with no counterpart is simply absent.
//
// Two passes, because the two ways she edits an outline fail each other's
// heuristic:
//
//  1. EXACT TEXT. Handles REORDERING — dragging 结果 above 因 keeps each
//     paragraph with the heading it was written under, wherever it moved to.
//     Position-matching gets this catastrophically wrong: it would hand
//     paragraph 1 the heading now sitting at index 0, confidently and
//     silently. A wrong heading is worse than none.
//
//  2. The SINGLE leftover, if exactly one row is left unmatched on each side.
//     Handles REWORDING — "引言" → "引言：问题的提出" is the same point with
//     better words, and her paragraph is still about it.
//
// Pass 2 is deliberately narrow, and an earlier version of it was wrong. It
// used to pair every leftover by position, on the reasoning that a reorder
// leaves no leftovers for position to mispair. That reasoning fails as soon as
// one save both reorders and INSERTS: old [原因@0, 结果@1] → new [引言@0,
// 原因：排放结构@1, 结果@2] leaves 原因 and 引言 both unmatched at position 0,
// and pairs her 原因 paragraph to 引言 — a heading she had just written, that
// she never wrote that paragraph under. Confident, specific, and wrong, which
// is precisely what this function exists to avoid.
//
// One-in, one-out is the case where "the same point, reworded" is the only
// reading available. With more than one leftover on either side there is no
// way to tell a reword from an insertion, so nothing is guessed.
//
// Anything still unmatched is a point she deleted or replaced outright. That
// link is honestly gone, and the caller says nothing rather than guessing —
// the same "omit, never estimate" rule the report generator follows.
//
// Duplicate texts bind to the first match: identical headings are
// indistinguishable by definition, and dropping both links would serve her
// worse than picking one.
func matchOutlineRows(oldOutline, newOutline []sqlc.WritingOutline) map[uuid.UUID]uuid.UUID {
	out := make(map[uuid.UUID]uuid.UUID, len(oldOutline))

	newByText := make(map[string]uuid.UUID, len(newOutline))
	for _, o := range newOutline {
		if _, seen := newByText[o.Text]; !seen {
			newByText[o.Text] = o.ID
		}
	}
	claimed := make(map[uuid.UUID]bool, len(newOutline))
	var leftoverOld []sqlc.WritingOutline
	for _, o := range oldOutline {
		if newID, ok := newByText[o.Text]; ok && !claimed[newID] {
			out[o.ID] = newID
			claimed[newID] = true
			continue
		}
		leftoverOld = append(leftoverOld, o)
	}

	var leftoverNew []sqlc.WritingOutline
	for _, o := range newOutline {
		if !claimed[o.ID] {
			leftoverNew = append(leftoverNew, o)
		}
	}
	// Exactly one on each side, or nothing. See the doc comment: with more than
	// one leftover there is no way to distinguish a reworded point from a newly
	// inserted one, and guessing attaches her paragraph to a heading she never
	// wrote it under.
	if len(leftoverOld) == 1 && len(leftoverNew) == 1 {
		out[leftoverOld[0].ID] = leftoverNew[0].ID
	}
	return out
}
