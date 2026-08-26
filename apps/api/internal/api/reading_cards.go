package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// reading_cards.go — 工具卡生命周期 over the lite atom substrate: list the
// cards proposed against a reading, and the three transitions a student can
// drive (activate on open, submit on fill, skip on pass). The AI turn that
// actually proposes a card lands in Task 7 (seedCard fills that gap in
// tests); this file only owns the lifecycle once a card row exists.
//
// The envelope (field_values/event_trace) is stored verbatim past a boundary
// check — Go never validates the inner shape. That truth lives in
// packages/contracts' Zod schemas; duplicating it here would guarantee drift.

type cardDTO struct {
	ID          string          `json:"id"`
	CardID      string          `json:"cardId"`
	BlockID     *string         `json:"blockId"`
	Status      string          `json:"status"`
	FieldValues json.RawMessage `json:"fieldValues"`
	EventTrace  json.RawMessage `json:"eventTrace"`
	CreatedAt   string          `json:"createdAt"`
	SubmittedAt *string         `json:"submittedAt"`
}

func cardDTOOf(row sqlc.AtomCard) cardDTO {
	out := cardDTO{
		ID: row.ID.String(), CardID: row.CardID, BlockID: row.BlockID, Status: row.Status,
		FieldValues: json.RawMessage(row.FieldValues),
		EventTrace:  json.RawMessage(row.EventTrace),
		CreatedAt:   row.CreatedAt.Format(time.RFC3339),
	}
	if row.SubmittedAt.Valid {
		s := row.SubmittedAt.Time.Format(time.RFC3339)
		out.SubmittedAt = &s
	}
	return out
}

// validateEnvelope is the boundary check the spec mandates: Go verifies only
// the envelope's OUTER shape — field_values is a JSON object, event_trace is
// a JSON array — and stores the inner structure verbatim. The deep truth
// lives in packages/contracts' Zod schemas; duplicating it here would
// guarantee the two drift apart.
func validateEnvelope(fieldValues, eventTrace json.RawMessage) error {
	var obj map[string]any
	if err := json.Unmarshal(fieldValues, &obj); err != nil {
		return httpx.ErrBadRequest("bad_field_values", "字段值格式不对", nil)
	}
	var arr []any
	if err := json.Unmarshal(eventTrace, &arr); err != nil {
		return httpx.ErrBadRequest("bad_event_trace", "事件轨迹格式不对", nil)
	}
	return nil
}

// loadOwnedAtomCard parses {cid} and verifies it belongs to atomID. Every
// failure — malformed id, missing row, or a card that names a DIFFERENT
// atom — is a flat 404: without the atom_id comparison, a caller who owns
// atomID (any reading of theirs) could reach any other student's card by
// guessing/observing its id, since only the atom in the path is checked for
// ownership. Reused by Task 7.
func (a *API) loadOwnedAtomCard(w http.ResponseWriter, r *http.Request, atomID uuid.UUID) (sqlc.AtomCard, bool) {
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.AtomCard{}, false
	}
	card, err := a.d.Queries.GetAtomCard(r.Context(), cid)
	if err != nil {
		httpx.WriteError(w, r, err) // pgx.ErrNoRows → 404
		return sqlc.AtomCard{}, false
	}
	if card.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.AtomCard{}, false
	}
	return card, true
}

// cardIsTerminal reports whether status is one of the two end states the DB
// CHECK constraint allows no further transition out of. 'submitted' and
// 'skipped' are both final — the student's action is on record either way
// (铁律④: a skip is itself the data, not less-than a submit) — so every
// handler below rejects re-activating, re-skipping, or re-submitting a card
// already in one of these states with 409, rather than silently overwriting
// evidence that already exists.
func cardIsTerminal(status string) bool {
	return status == "submitted" || status == "skipped"
}

// liteListCards lists every card proposed against this reading, oldest
// first (ListAtomCards orders by created_at).
func (a *API) liteListCards(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListAtomCards(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]cardDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, cardDTOOf(row))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"cards": out})
}

// liteActivateCard marks a proposed card "active" once the student opens it
// (design's "触发是自动的，但「打开」由学生确认" — opening is a distinct,
// recorded step from being surfaced).
func (a *API) liteActivateCard(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	card, ok := a.loadOwnedAtomCard(w, r, at.ID)
	if !ok {
		return
	}
	if cardIsTerminal(card.Status) {
		httpx.WriteError(w, r, httpx.ErrConflict("这张卡片已经结束，不能再打开。"))
		return
	}
	row, err := a.d.Queries.UpdateAtomCardStatus(r.Context(), sqlc.UpdateAtomCardStatusParams{
		ID: card.ID, Status: "active",
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, cardDTOOf(row))
}

// liteSkipCard records a skipped card. 铁律④ 过程即数据: the row survives —
// status flips to 'skipped', nothing is deleted. Friction is a signal, not
// something to erase.
func (a *API) liteSkipCard(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	card, ok := a.loadOwnedAtomCard(w, r, at.ID)
	if !ok {
		return
	}
	if cardIsTerminal(card.Status) {
		httpx.WriteError(w, r, httpx.ErrConflict("这张卡片已经结束，不能再跳过。"))
		return
	}
	row, err := a.d.Queries.UpdateAtomCardStatus(r.Context(), sqlc.UpdateAtomCardStatusParams{
		ID: card.ID, Status: "skipped",
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, cardDTOOf(row))
}

// liteSubmitCard persists a filled card envelope verbatim past the boundary
// check.
func (a *API) liteSubmitCard(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	card, ok := a.loadOwnedAtomCard(w, r, at.ID)
	if !ok {
		return
	}
	if cardIsTerminal(card.Status) {
		httpx.WriteError(w, r, httpx.ErrConflict("这张卡片已经结束，不能再提交。"))
		return
	}
	var body struct {
		FieldValues json.RawMessage `json:"fieldValues"`
		EventTrace  json.RawMessage `json:"eventTrace"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := validateEnvelope(body.FieldValues, body.EventTrace); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	row, err := a.d.Queries.SubmitAtomCard(r.Context(), sqlc.SubmitAtomCardParams{
		ID: card.ID, FieldValues: []byte(body.FieldValues), EventTrace: []byte(body.EventTrace),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, cardDTOOf(row))
}
