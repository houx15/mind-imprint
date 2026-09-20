package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// writing_outline.go — the blocks of a writing, stored. Two routes:
//   - GET  /writings/{id}/outline  — read them.
//   - PUT  /writings/{id}/outline  — full replace (学生可改), same semantics as
//     pro's putOutline (workspace_write.go): wipe everything, reinsert the
//     posted array in order, position = array index, depth clamped to 0..2.
//     Carries `role` alongside `text` (0100) — the skeleton's generic label
//     and HER sentence, rewritten together so an ordinary text save cannot
//     wipe the labels.
//
// There was a third route, POST /outline/generate, which derived a candidate
// outline from everything she had said. It is DELETED (2026-08-27): the
// product ruling for lite is that the AI never authors an outline at all.
// AGENTS.md's 铁律 boundary — outline derivation as a deterministic system
// step — still holds for PRO, where the outline is scaffolding around an
// existing research question; in lite the outline IS the lesson, so
// generating it steals it. What replaced it lives in writing_structure.go
// (pick a generic skeleton) and writing_guide.go (ask her questions about a
// block). Do not reintroduce it here.
//
// Named writing_outline.go / getWritingOutline / putWritingOutline /
// writingOutlineItemDTO throughout — pro already
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
	ID string `json:"id"`
	// Text is HER sentence for this block; Role is the GENERIC block label the
	// skeleton contributed (writing_structures.go, stored in the 0100 column).
	// They are separate fields for the same reason they are separate columns:
	// once merged there is no way to tell her thinking from the template, and
	// that distinction is exactly what the process report reads.
	Text string `json:"text"`
	// Role 现在是**派生值**：kind 的标题（writingKindLabel），不是模型写的散文。
	// 它仍然落库并回传，因为报告、教师端和老前端都还读这一列。
	Role string `json:"role"`
	// Kind 是这一块是什么，闭表见 writing_kind.go。
	// 🚨 json 标签不能少：缺标签会让前端读到的字段名全错，而 Go 测试全绿、
	// 日志干净（[[go-nil-slice-becomes-null]]）。
	Kind     string `json:"kind"`
	Depth    int32  `json:"depth"`
	Position int32  `json:"position"`
	// Source 是这条材料从哪来（0158）。空串 = 她自己的经历，或者她没写出处；
	// 界面据此不显示出处那一行，也不编一个「本人」出来。
	Source string `json:"source,omitempty"`
	// Guide is the stored 引导 (Task 4, writing_guide.go): job + resolved
	// methods + questions, painted on first render instead of waiting for
	// her to click 卡住了？. nil (omitted) when this block has never been
	// guided yet.
	Guide *writingGuideDTO `json:"guide,omitempty"`
}

func toWritingOutlineItemDTO(row sqlc.WritingOutline) writingOutlineItemDTO {
	// 老行（0182 之前）没有 kind：现算一个，别让前端拿到空字符串去 switch。
	kind := writingKindOf(row)
	role := row.Role
	if lbl := writingKindLabel(kind, row.Source); lbl != "" {
		role = lbl
	}
	dto := writingOutlineItemDTO{
		ID: row.ID.String(), Text: row.Text, Role: role, Kind: kind, Depth: row.Depth, Position: row.Position,
		Source: row.Source,
	}
	if g, ok := storedWritingGuide(row); ok {
		dto.Guide = &g
	}
	return dto
}

// storedWritingGuide decodes writing_outline.guide, and is the ONE definition
// of "this block already has a guide" — read both here (what GET /outline
// paints) and by the batch route (which blocks still need a model call). Two
// definitions would mean a block that renders a guide but keeps getting
// regenerated, or the reverse.
//
// A guide with no questions does not count as one: JSON null decodes into a
// zero-valued struct without error (the 2026-08-25 报告加载失败 lesson), and a
// guide with nothing to ask teaches her nothing — parseWritingGuide refuses to
// produce one, so neither should reading one back.
func storedWritingGuide(row sqlc.WritingOutline) (writingGuideDTO, bool) {
	if len(row.Guide) == 0 {
		return writingGuideDTO{}, false
	}
	var g writingGuideDTO
	if err := json.Unmarshal(row.Guide, &g); err != nil {
		return writingGuideDTO{}, false
	}
	if len(g.Questions) == 0 {
		return writingGuideDTO{}, false
	}
	return g, true
}

// writingOutlineItemReq is PUT's per-item request shape (no id/position — the
// whole array is a full replace, ids are always freshly minted and position
// is always the array index, mirroring pro's putOutline).
type writingOutlineItemReq struct {
	// Role is echoed back by the client from what the server served. The
	// skeleton owns it; the PUT only has to avoid destroying it. An absent
	// role is stored as "" — a hand-added free block genuinely has no role.
	Role string `json:"role"`
	Text string `json:"text"`
	// Kind 是这一块是什么（0182）。客户端把服务端发给它的那一份原样回传；
	// 她拖动之后前端会按 outlineKind.ts 改写它。不合法或缺失时服务端按
	// role + depth 兜底，见 buildWritingOutlineArrays。
	Kind  string `json:"kind"`
	Depth int32  `json:"depth"`
	// Source 是这条材料从哪来（0158）。空串 = 她自己的经历，或者她没写出处。
	// 客户端把服务端发给它的那一份原样回传 —— 和 Role 同一个道理：全量替换
	// 这条路只负责别把它弄丢。
	Source string `json:"source"`
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
func buildWritingOutlineArrays(items []writingOutlineItemReq) (texts []string, roles []string, depths []int32, positions []int32, sources []string, kinds []string) {
	texts = make([]string, len(items))
	roles = make([]string, len(items))
	depths = make([]int32, len(items))
	positions = make([]int32, len(items))
	sources = make([]string, len(items))
	kinds = make([]string, len(items))
	for i, it := range items {
		texts[i] = strings.TrimSpace(it.Text)
		// kind 不合法就按她给的 role 和深度兜底 —— 老前端不会传这个字段，
		// 而一次保存不该因为少一个字段就把她的提纲清空。
		k := strings.ToLower(strings.TrimSpace(it.Kind))
		if !writingKindValid(k) {
			k = writingKindFromRole(strings.TrimSpace(it.Role), clampDepth(it.Depth))
		}
		kinds[i] = k
		// 🚨 深度由 kind 算出来，不采信客户端给的。她拖动之后前端已经按同一张表
		// 算过一次（outlineKind.ts），这里再算一次是为了让服务端不依赖它算对。
		depths[i] = clampDepth(writingKindDepth(k))
		// Role 是派生的标题，不是她输入的字段。
		roles[i] = writingKindLabel(k, it.Source)
		positions[i] = int32(i)
		sources[i] = trimRunes(strings.TrimSpace(it.Source), writingSourceMaxRunes)
	}
	return texts, roles, depths, positions, sources, kinds
}

// writingSourceMaxRunes 是一条出处能有多长。
//
// 300：一个链接加一句刊名、期号绰绰有余，而再长的那些是她把整段材料粘进了
// 出处栏 —— 那段材料该待在节点的正文里。截断而不是拒绝：她已经打完了，
// 为一个格式问题把整次保存退回去，代价比截掉一截大。
const writingSourceMaxRunes = 300

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
func validateWritingOutlineArrayLengths(texts, roles []string, depths, positions []int32, sources, kinds []string) error {
	if len(texts) != len(roles) || len(texts) != len(depths) || len(texts) != len(positions) ||
		len(texts) != len(sources) || len(texts) != len(kinds) {
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

	texts, roles, depths, positions, sources, kinds := buildWritingOutlineArrays(body.Outline)
	if verr := validateWritingOutlineArrayLengths(texts, roles, depths, positions, sources, kinds); verr != nil {
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
		AtomID: at.ID, Texts: texts, Roles: roles, Depths: depths, Positions: positions,
		Sources: sources, Kinds: kinds,
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
// EXACT TEXT IS THE ONLY MATCH. If a heading's text survives the save, the
// paragraph written under it is genuinely still about it, wherever a reorder
// moved it to. If the text is gone, the link is gone, and the caller says
// nothing — the same "omit, never estimate" rule the report generator follows
// for missing facts.
//
// This function has now been wrong TWICE by trying to be cleverer than that,
// and both attempts failed the same way — they produced a confident, specific,
// wrong heading, which is worse than a blank one:
//
//  1. Pairing every leftover by POSITION. Broke as soon as one save both
//     reordered and inserted: old [原因, 结果] → [引言, 原因：排放结构, 结果]
//     handed her 原因 paragraph the heading 引言, a point she had just written.
//
//  2. Pairing the SINGLE leftover when exactly one was unmatched on each side,
//     on the theory that "the same point, reworded" was then the only reading.
//     It is not. Deleting one point and adding an unrelated one in the same
//     save — old [原因, 结果] → [原因, 反驳] — produces exactly that shape, and
//     reattached her 结果 paragraph to 反驳.
//
// The lesson both times: a full-replace PUT carries no per-row intent, so
// nothing in the payload distinguishes "I reworded this point" from "I
// replaced it with a different one". They are the same bytes. No heuristic can
// recover an intent the request never expressed, and every attempt buys a
// small convenience by occasionally lying to her about what she wrote.
//
// The cost is accepted deliberately: rewording a heading drops its
// paragraphs' labels. That is visible, harmless, and she can relink by
// rewriting the heading back or simply carrying on — whereas a wrong label is
// silent and misleads her about her own work. If reword-survival is wanted
// later, the fix is to carry row IDENTITY in the PUT (send ids for rows she
// kept), not to guess here.
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
	for _, o := range oldOutline {
		if newID, ok := newByText[o.Text]; ok && !claimed[newID] {
			out[o.ID] = newID
			claimed[newID] = true
		}
		// No else. An old row whose text is gone is simply absent from the
		// result — see the doc comment on why every attempt to pair the
		// leftovers has produced a wrong heading instead of a missing one.
	}
	return out
}
