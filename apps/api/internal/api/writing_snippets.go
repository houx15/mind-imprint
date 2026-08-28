package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// writing_snippets.go — Task 6 of the lite writing phase: 段落 (paragraphs).
// Two routes:
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
//
// A third route, POST .../{sid} generating an English-only model paragraph
// plus guiding questions, lived here through Task 6; Task 7 removed it — a
// demonstration paragraph about her own thesis was always a paragraph she
// could retype, and English support moved to sentence frames with blanks
// instead.
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
// OutlineHeading resolves strictly by id (writingSnippetHeadingByID) — see
// its comment for why a position fallback is deliberately not used here.
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
// coach turn / outline paths, ever writes model output here.
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

// extractWritingJSONObject strips code fences and clamps to the outermost
// '{'..'}' — the object-shaped sibling of writing_outline.go's
// extractWritingOutlineJSONArray. Shared by every JSON-object-replying prompt
// in this package (writing_comment.go's parseWritingComment reuses it below);
// there is no shared logic worth factoring beyond "trim fences, clamp
// brackets", which differs only in which bracket character it clamps to.
func extractWritingJSONObject(text string) string {
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

// writingSnippetHeadingByID resolves a snippet's outline heading for the
// CLIENT-facing DTO, strictly by id — no position fallback, deliberately.
// Positions are array indices, so the moment she REORDERS her outline or
// deletes a middle point, position-matching would show paragraph 1 the
// heading belonging to paragraph 2 — specific, confident and wrong, with
// nothing in the payload marking it as a guess. Showing her the wrong
// heading is worse than showing none.
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
