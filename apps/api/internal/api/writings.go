package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// writings.go — the lite edition's 写作 atom: "type one sentence into a box
// and you're started." Like reading (readings.go), it owns its storage
// outright (atom + writing + the shared atom_* machinery) and never touches
// any project-scoped table.

type writingDTO struct {
	ID          string `json:"id"` // the ATOM id — every writing endpoint is keyed by it
	Title       string `json:"title"`
	Lang        string `json:"lang"`
	Stage       string `json:"stage"`
	TargetWords *int32 `json:"targetWords"`
	// StructureKey names the skeleton she picked out of the fixed structure
	// library (writing_structures.go); "" = not chosen yet. SetupAt is when
	// the entry 设定 dialog was completed; null = never, which is exactly
	// what the frontend gates that dialog on. Both are 0100 columns.
	StructureKey string  `json:"structureKey"`
	SetupAt      *string `json:"setupAt"`
	Status       string  `json:"status"`
	CreatedAt    string  `json:"createdAt"`
	UpdatedAt    string  `json:"updatedAt"`
	FinishedAt   *string `json:"finishedAt"`
}

func writingDTOOf(wr sqlc.Writing, createdAt time.Time) writingDTO {
	out := writingDTO{
		ID: wr.AtomID.String(), Title: wr.Title, Lang: wr.Lang, Stage: wr.Stage,
		TargetWords: wr.TargetWords, StructureKey: wr.StructureKey, Status: wr.Status,
		CreatedAt: createdAt.Format(time.RFC3339),
		UpdatedAt: wr.UpdatedAt.Format(time.RFC3339),
	}
	if wr.SetupAt.Valid {
		s := wr.SetupAt.Time.Format(time.RFC3339)
		out.SetupAt = &s
	}
	if wr.FinishedAt.Valid {
		s := wr.FinishedAt.Time.Format(time.RFC3339)
		out.FinishedAt = &s
	}
	return out
}

// loadOwnedWritingAtom is loadOwnedAtom (readings.go) curried to "writing" —
// the exact mirror of loadOwnedReadingAtom. Every W3-W7 handler funnels
// through this one chokepoint for existence/ownership/kind checks, the
// finished-write gate, and the last_activity_at touch.
func (a *API) loadOwnedWritingAtom(w http.ResponseWriter, r *http.Request) (sqlc.Atom, bool) {
	return a.loadOwnedAtom(w, r, "writing")
}

// loadOwnedWritingAtomRow is loadOwnedWritingAtom's ungated sibling, curried
// to "writing" for the same reason loadOwnedReadingAtomRow is (readings.go):
// finishWritingAtom (writing_compose.go) uses this directly so a second
// POST /finish stays callable after the first one already succeeded.
func (a *API) loadOwnedWritingAtomRow(w http.ResponseWriter, r *http.Request) (sqlc.Atom, bool) {
	return a.loadOwnedAtomRow(w, r, "writing")
}

// createWriting is the "type one sentence into a box" entry point. The
// sentence she types — idea — does double duty: truncated to 200 runes it
// becomes the writing's initial title, and verbatim (untruncated) it becomes
// the first atom_message (role='student') — because 先聊's first line really
// is the one she just said, and it must not vanish from the transcript.
//
// Unlike reading, an empty idea is refused outright (400 missing_idea):
// reading can be created first and have its article pasted in later, but a
// writing has nothing to talk about without one.
//
// atom + writing + the first atom_message are inserted in ONE transaction:
// a writing whose title exists but whose opening line does not (or vice
// versa) is a half-created writing, and the point of the transaction is that
// no caller can ever observe that state — either all three rows exist or
// none do.
func (a *API) createWriting(w http.ResponseWriter, r *http.Request) {
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
	var req struct {
		Idea string `json:"idea"`
		Lang string `json:"lang"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	idea := strings.TrimSpace(req.Idea)
	if idea == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_idea", "先说说你想写点什么。", nil))
		return
	}
	title := idea
	if len([]rune(title)) > 200 {
		title = string([]rune(title)[:200])
	}
	lang := strings.TrimSpace(req.Lang)
	if lang != "zh" && lang != "en" {
		lang = "zh"
	}

	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	at, err := qtx.CreateAtom(r.Context(), sqlc.CreateAtomParams{Kind: "writing", UserID: u.ID})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.CreateWriting(r.Context(), sqlc.CreateWritingParams{
		AtomID: at.ID, Title: title, Lang: lang,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// seq=1 literal, not NextAtomMessageSeq: this atom_id was just minted
	// inside this same transaction, so it is unconditionally the first
	// message — no concurrent writer can have raced it.
	if _, err := qtx.AppendAtomMessage(r.Context(), sqlc.AppendAtomMessageParams{
		AtomID: at.ID, Seq: 1, Role: "student", Content: idea,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": at.ID.String()})
}

func (a *API) listWritings(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	rows, err := a.d.Queries.ListWritingsByUser(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]writingDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, writingDTOOf(sqlc.Writing{
			AtomID: row.AtomID, Title: row.Title, Lang: row.Lang, Stage: row.Stage,
			TargetWords: row.TargetWords, StructureKey: row.StructureKey, SetupAt: row.SetupAt,
			Status:    row.Status,
			UpdatedAt: row.UpdatedAt, FinishedAt: row.FinishedAt,
		}, row.AtomCreatedAt))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"writings": out})
}

func (a *API) getWriting(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	wr, err := a.d.Queries.GetWriting(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, writingDTOOf(wr, at.CreatedAt))
}

// renameWriting is PATCH /writings/{id}: the only field this task's PATCH
// touches is title, mirroring renameReading. Stage/targetWords get their own
// endpoints in later tasks (W3-W7) — this one does not reach into them.
func (a *API) renameWriting(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_title", "标题不能为空。", nil))
		return
	}
	if len([]rune(title)) > 200 {
		title = string([]rune(title)[:200])
	}
	if err := a.d.Queries.RenameWriting(r.Context(), sqlc.RenameWritingParams{AtomID: at.ID, Title: title}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	wr, err := a.d.Queries.GetWriting(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, writingDTOOf(wr, at.CreatedAt))
}
