package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// readings.go — the lite edition's 阅读 atom: one student's single reading
// exercise. It owns its storage outright (atom + reading + the shared atom_*
// machinery); it does NOT borrow a project row and never touches any
// project-scoped table.

type readingDTO struct {
	ID        string `json:"id"` // the ATOM id — every reading endpoint is keyed by it
	Title     string `json:"title"`
	Lang      string `json:"lang"`
	Status    string `json:"status"`
	HasSource bool   `json:"hasSource"`
	CreatedAt string `json:"createdAt"`
	// updatedAt is reading.updated_at: TITLE/status metadata only (rename and
	// finish write it). It is NOT "when she last read this" — see below.
	UpdatedAt string `json:"updatedAt"`
	// lastActivityAt is atom.last_activity_at (0098): the last time she wrote
	// ANYTHING into this reading — a turn, a card, the article, a margin note.
	// This is what 上次读到 means and what 「你有 N 篇还没读完」 orders by.
	// Before it existed both were answered by updatedAt, so an hour of actual
	// reading moved neither.
	LastActivityAt string  `json:"lastActivityAt"`
	FinishedAt     *string `json:"finishedAt"`
}

// readingListDefaultLimit / readingListMaxLimit bound 我的阅读.
//
// The default is deliberately generous for one student and still small enough
// that the drawer stays a shelf rather than an archive. The ceiling exists so
// a hand-crafted `?limit=100000` cannot be used to ask the API to serialize
// everything she has ever read.
const (
	readingListDefaultLimit = 50
	readingListMaxLimit     = 200
)

// readingListLimit reads `?limit=`. Anything missing, unparseable, or ≤0 takes
// the default: a malformed query string should narrow her history to nothing
// even less than it should hand back all of it.
func readingListLimit(r *http.Request) int {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return readingListDefaultLimit
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return readingListDefaultLimit
	}
	if n > readingListMaxLimit {
		return readingListMaxLimit
	}
	return n
}

// readingRecency is when this reading last mattered: the moment she finished
// it, or — while it is still open — the last time she wrote anything into it.
// RFC3339 sorts correctly as a string, which is why these stay formatted.
func readingRecency(d readingDTO) string {
	if d.FinishedAt != nil && *d.FinishedAt != "" {
		return *d.FinishedAt
	}
	return d.LastActivityAt
}

func (a *API) readingDTOOf(rd sqlc.Reading, hasSource bool, createdAt, lastActivityAt time.Time) readingDTO {
	out := readingDTO{
		ID: rd.AtomID.String(), Title: rd.Title, Lang: rd.Lang, Status: rd.Status,
		HasSource:      hasSource,
		CreatedAt:      createdAt.Format(time.RFC3339),
		UpdatedAt:      rd.UpdatedAt.Format(time.RFC3339),
		LastActivityAt: lastActivityAt.Format(time.RFC3339),
	}
	if rd.FinishedAt.Valid {
		s := rd.FinishedAt.Time.Format(time.RFC3339)
		out.FinishedAt = &s
	}
	return out
}

// loadOwnedAtom parses {id} as an atom of the given kind owned by the
// caller. Every failure — malformed id, missing row, wrong kind, wrong owner —
// is a flat 404, so atom existence is never leaked. Shared by every lite
// handler whose body is genuinely kind-agnostic (Task 1.5 generalised this
// from loadOwnedReadingAtom, which every ~20-route reading handler funnelled
// through since Tasks 3-8).
//
// Every non-GET request against an atom that is already *finished* is
// additionally refused with the kind's own 403 (reading_finished /
// writing_finished — see atomFinishedError): 铁律④ makes the process record
// evidence a report is generated from, and a raw API call — or a tab that
// had the room open before the atom finished — must not be able to keep
// writing to it after the client-side read-only view says otherwise. 403,
// not 404, because the atom genuinely exists and is hers; mirrors
// loadOwnedProject's demo_readonly shape for the same reason that one does.
//
// Callers pass an explicit, literal kind at the mux ("reading" / "writing")
// — never derive it by sniffing r.URL.Path. A future route rename would
// otherwise silently break authorization, and an authorization check that
// fails open is the worst kind of bug.
//
// Being the single chokepoint is also why last_activity_at (0098) is bumped
// HERE rather than at each write. Every non-GET on a still-open atom is, by
// definition, her doing something to it — a turn, a card transition, a
// margin note — so one bump at the one door covers all of them and, unlike a
// per-handler call, cannot be forgotten by the next endpoint someone adds.
func (a *API) loadOwnedAtom(w http.ResponseWriter, r *http.Request, kind string) (sqlc.Atom, bool) {
	at, ok := a.loadOwnedAtomRow(w, r, kind)
	if !ok {
		return sqlc.Atom{}, false
	}
	if r.Method != http.MethodGet {
		finished, err := a.atomIsFinished(r.Context(), at.ID, kind)
		if err != nil {
			httpx.WriteError(w, r, err)
			return sqlc.Atom{}, false
		}
		if finished {
			httpx.WriteError(w, r, atomFinishedError(kind))
			return sqlc.Atom{}, false
		}
		// AFTER the finished gate: a refused write is not activity.
		// Best-effort — an activity timestamp must never be the reason a
		// student's actual work fails. The returned row replaces `at` so a
		// handler that answers with a DTO reports the fresh value rather
		// than one write's worth of stale.
		if touched, terr := a.d.Queries.TouchAtom(r.Context(), at.ID); terr == nil {
			at = touched
		} else {
			slog.Warn("lite atom: touch last_activity_at failed",
				"err", terr, "atom_id", at.ID, "kind", kind, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}
	return at, true
}

// loadOwnedAtomRow is loadOwnedAtom's ungated sibling: same
// existence/ownership/kind checks (still a flat 404 on any failure), but
// applies no finished-atom write gate. finishReading uses this directly so a
// second POST /finish stays callable after the first one succeeded.
func (a *API) loadOwnedAtomRow(w http.ResponseWriter, r *http.Request, kind string) (sqlc.Atom, bool) {
	u, _ := UserFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.Atom{}, false
	}
	at, err := a.d.Queries.GetAtom(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err) // pgx.ErrNoRows → 404
		return sqlc.Atom{}, false
	}
	if at.Kind != kind || at.UserID != u.ID {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.Atom{}, false
	}
	return at, true
}

// atomIsFinished answers "is this atom already finished?" per kind. Each
// lite kind owns its own table and its own 'active'|'finished' status column
// (reading.status; writing.status, added alongside the writing table in
// commit c0ce1d46) — there is no shared status column on atom itself, so this
// is a small per-kind dispatch rather than one query.
func (a *API) atomIsFinished(ctx context.Context, atomID uuid.UUID, kind string) (bool, error) {
	switch kind {
	case "reading":
		rd, err := a.d.Queries.GetReading(ctx, atomID)
		if err != nil {
			return false, err
		}
		return rd.Status == "finished", nil
	case "writing":
		wr, err := a.d.Queries.GetWriting(ctx, atomID)
		if err != nil {
			return false, err
		}
		return wr.Status == "finished", nil
	default:
		return false, fmt.Errorf("atomIsFinished: no finished-gate wired for kind %q", kind)
	}
}

// atomFinishedError returns the kind-appropriate 403 for a write attempt
// against an atom that is already finished. Mirrors ErrReadingFinished's
// shape exactly (see httpx/errors.go) — same 403, a distinct stable code per
// kind so a client can react to "this is a writing, not a reading" without
// parsing the message.
func atomFinishedError(kind string) error {
	if kind == "writing" {
		return httpx.ErrWritingFinished()
	}
	return httpx.ErrReadingFinished()
}

// loadOwnedReadingAtom is loadOwnedAtom curried to "reading". Kept as a thin
// wrapper (rather than updating every call site to pass the kind) so this
// refactor's blast radius stays inside the loader itself: ~20 existing
// reading call sites (readings.go, reading_source.go, reading_source_file.go,
// reading_notes.go's brief/takeaway handlers, reading_lens.go's liteSummonCard)
// need no edit.
func (a *API) loadOwnedReadingAtom(w http.ResponseWriter, r *http.Request) (sqlc.Atom, bool) {
	return a.loadOwnedAtom(w, r, "reading")
}

// loadOwnedReadingAtomRow is loadOwnedReadingAtom's ungated sibling, curried
// to "reading" for the same reason. finishReading uses this directly so a
// second POST /finish stays callable after the first one succeeded.
func (a *API) loadOwnedReadingAtomRow(w http.ResponseWriter, r *http.Request) (sqlc.Atom, bool) {
	return a.loadOwnedAtomRow(w, r, "reading")
}

// hasSource reports whether the article body has been pasted yet. A missing
// row is a legitimate state (a brand-new reading), not an error.
func (a *API) hasSource(r *http.Request, atomID uuid.UUID) (bool, error) {
	if _, err := a.d.Queries.GetReadingSource(r.Context(), atomID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (a *API) createReading(w http.ResponseWriter, r *http.Request) {
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
		Title string `json:"title"`
		Lang  string `json:"lang"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "未命名阅读"
	}
	if len([]rune(title)) > 200 {
		title = string([]rune(title)[:200])
	}
	lang := strings.TrimSpace(req.Lang)
	if lang != "zh" && lang != "en" {
		lang = "zh"
	}

	// atom + reading in ONE transaction: an atom with no reading row would be
	// an identity nothing can render.
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	at, err := qtx.CreateAtom(r.Context(), sqlc.CreateAtomParams{Kind: "reading", UserID: u.ID})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.CreateReading(r.Context(), sqlc.CreateReadingParams{
		AtomID: at.ID, Title: title, Lang: lang,
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

func (a *API) listReadings(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	rows, err := a.d.Queries.ListReadingsByUser(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// ONE query, no per-row follow-up: hasSource now rides along on the list
	// (ListReadingsByUser LEFT JOINs reading_source). This used to run a
	// GetReadingSource per reading — an N+1 on the landing's first paint that
	// also dragged every article's whole BODY across the wire just to ask
	// whether the row existed.
	out := make([]readingDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, a.readingDTOOf(sqlc.Reading{
			AtomID: row.AtomID, Title: row.Title, Lang: row.Lang,
			Status: row.Status, UpdatedAt: row.UpdatedAt, FinishedAt: row.FinishedAt,
		}, row.HasSource, row.AtomCreatedAt, row.LastActivityAt))
	}
	// 我的阅读 is a shelf she picks up from, not an archive she scrolls: the
	// answer is ordered by when each reading last mattered (finished → when
	// she finished it; still open → when she last touched it) and cut to the
	// most recent `limit`.
	//
	// The ordering is done here rather than in the query because the query's
	// ORDER BY is `atom.created_at` — which answers "when did she open this",
	// not "when was she last in it". Sorting the row set costs nothing at a
	// student's scale; if it ever stops being nothing, this same ordering
	// belongs in the SQL with the LIMIT beside it.
	sort.SliceStable(out, func(i, j int) bool { return readingRecency(out[i]) > readingRecency(out[j]) })
	total := len(out)
	if n := readingListLimit(r); n < len(out) {
		out = out[:n]
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"readings": out, "total": total})
}

func (a *API) getReading(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	rd, err := a.d.Queries.GetReading(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	hasSrc, err := a.hasSource(r, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a.readingDTOOf(rd, hasSrc, at.CreatedAt, at.LastActivityAt))
}

func (a *API) renameReading(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
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
	if err := a.d.Queries.RenameReading(r.Context(), sqlc.RenameReadingParams{AtomID: at.ID, Title: title}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rd, err := a.d.Queries.GetReading(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	hasSrc, err := a.hasSource(r, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a.readingDTOOf(rd, hasSrc, at.CreatedAt, at.LastActivityAt))
}

// finishReading marks the reading finished. Named finishReading, not
// finishProject — that name is already the pro side's terminal
// (project_finish.go), and internal/api is one shared package.
//
// It used to be gated on a non-empty takeaway; see the body for why that gate
// is gone. Unlike finishProject there is no async report to generate, so this
// is a plain synchronous flip.
//
// Idempotent, and idempotent means NO-OP, not "do it again": SetReadingFinished
// carries `AND status <> 'finished'`, so a second POST answers 200 with the
// unchanged row instead of re-stamping finished_at. 铁律④ makes finished_at
// evidence — when she finished is a fact about the past, and a replayed
// request (a double-click, a retry, a stale tab) must not be able to move it
// hours later.
func (a *API) finishReading(w http.ResponseWriter, r *http.Request) {
	// loadOwnedReadingAtomRow, NOT loadOwnedReadingAtom: this endpoint must
	// stay callable (idempotently) after the reading is already finished —
	// see the finished-reading write gate's doc comment above.
	at, ok := a.loadOwnedReadingAtomRow(w, r)
	if !ok {
		return
	}
	// 🚨 There is no longer a takeaway gate here, and that is deliberate.
	//
	// It used to refuse a finish with an empty takeaway — 「先写下你的收获，再
	// 完成这次阅读」 — because at the time the only record a reading left was
	// that one textarea, and finishing with it blank meant finishing with
	// nothing. That stopped being true: 带读 walks her through a planned
	// route, every step is recorded, her picked sentences and lens findings
	// are rows, and the report is generated from all of it.
	//
	//   > we have give abundant steps for the reading. so we don't need to
	//   > ask student to enter the form again.
	//
	// So 完成这篇 now finishes and lands on the report. The gate's purpose —
	// don't let a reading be marked done with nothing behind it — is served
	// by the whole record, which is far more than a form ever collected.
	// A takeaway she DID write (older readings, and PUT /takeaway is still
	// live) is still stored and still shown.
	if err := a.d.Queries.SetReadingFinished(r.Context(), at.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// 请后台采一次兴趣。一条 INSERT，不等待；失败只 warn，扫尾会捞回来。
	// 这个动作仍然是一次瞬时翻转 —— 队列存在的意义就在这里。
	a.EnqueueHarvest(r.Context(), at.ID)
	rd, err := a.d.Queries.GetReading(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	hasSrc, err := a.hasSource(r, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a.readingDTOOf(rd, hasSrc, at.CreatedAt, at.LastActivityAt))
}
