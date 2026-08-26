package api

import (
	"bytes"
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
	ID      string  `json:"id"`
	CardID  string  `json:"cardId"`
	BlockID *string `json:"blockId"`
	Status  string  `json:"status"`
	// anchors is what the card HANGS ON — the AI's example sentence while the
	// card is proposed, the student's own picked sentence once she submits.
	// The reading room needs the full span (block + rune offsets + quote), not
	// just blockId: it highlights the example sentence inside the paragraph and
	// enforces "pick a DIFFERENT sentence" by range overlap.
	Anchors     json.RawMessage `json:"anchors"`
	FieldValues json.RawMessage `json:"fieldValues"`
	EventTrace  json.RawMessage `json:"eventTrace"`
	// framework is the persisted selection review (verdict + 3 checks + finding)
	// from the evaluate endpoint — `{}` until she has picked a sentence.
	Framework   json.RawMessage `json:"framework"`
	CreatedAt   string          `json:"createdAt"`
	SubmittedAt *string         `json:"submittedAt"`
}

func cardDTOOf(row sqlc.AtomCard) cardDTO {
	out := cardDTO{
		ID: row.ID.String(), CardID: row.CardID, BlockID: row.BlockID, Status: row.Status,
		Anchors:     jsonOr(row.Anchors, "[]"),
		FieldValues: json.RawMessage(row.FieldValues),
		EventTrace:  json.RawMessage(row.EventTrace),
		Framework:   jsonOr(row.FrameworkFill, "{}"),
		CreatedAt:   row.CreatedAt.Format(time.RFC3339),
	}
	if row.SubmittedAt.Valid {
		s := row.SubmittedAt.Time.Format(time.RFC3339)
		out.SubmittedAt = &s
	}
	return out
}

// jsonOr keeps a jsonb column from ever reaching the client as a literal
// `null`. The columns are NOT NULL with defaults, so this is belt-and-braces
// for rows written before 0095 — but a stored null slipping past a Go
// boundary check into a strict Zod parse is exactly how the teacher report
// blanked once already, and the fix costs one line.
func jsonOr(raw []byte, fallback string) json.RawMessage {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return json.RawMessage(fallback)
	}
	return json.RawMessage(raw)
}

// validateEnvelope is the boundary check the spec mandates: Go verifies only
// the envelope's OUTER shape — field_values is a JSON object, event_trace is
// a JSON array — and stores the inner structure verbatim. The deep truth
// lives in packages/contracts' Zod schemas; duplicating it here would
// guarantee the two drift apart.
//
// The literal `null` must be rejected explicitly: unmarshaling JSON `null`
// into a map or slice pointer succeeds with no error and simply leaves the
// target nil, so a bare type-switch on unmarshal error alone would let
// `null` slip past this check and get stored verbatim by SubmitAtomCard —
// exactly the class of bug that blanked the teacher report elsewhere in this
// codebase (a stored `null` surviving a Go boundary check and later hitting
// a strict Zod `.array()` parse). A genuinely missing field (absent JSON
// key, so the json.RawMessage arg is nil/empty) still fails the unmarshal
// itself with "unexpected end of JSON input" and needs no separate case.
func validateEnvelope(fieldValues, eventTrace json.RawMessage) error {
	var obj map[string]any
	if isJSONNull(fieldValues) {
		return httpx.ErrBadRequest("bad_field_values", "字段值格式不对", nil)
	}
	if err := json.Unmarshal(fieldValues, &obj); err != nil {
		return httpx.ErrBadRequest("bad_field_values", "字段值格式不对", nil)
	}
	var arr []any
	if isJSONNull(eventTrace) {
		return httpx.ErrBadRequest("bad_event_trace", "事件轨迹格式不对", nil)
	}
	if err := json.Unmarshal(eventTrace, &arr); err != nil {
		return httpx.ErrBadRequest("bad_event_trace", "事件轨迹格式不对", nil)
	}
	return nil
}

// isJSONNull reports whether raw is exactly the JSON literal `null` (modulo
// surrounding whitespace) — the one input json.Unmarshal accepts into a
// map/slice pointer without error while leaving it nil.
func isJSONNull(raw json.RawMessage) bool {
	return string(bytes.TrimSpace(raw)) == "null"
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
		// anchors — the sentence the student actually picked. 过程即数据: a
		// submit that recorded only the envelope would lose WHICH sentence she
		// chose, which is the whole point of the reading loop. Omitted → the
		// card keeps the anchors it was summoned with.
		Anchors json.RawMessage `json:"anchors"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := validateEnvelope(body.FieldValues, body.EventTrace); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	anchors := card.Anchors
	if len(bytes.TrimSpace(body.Anchors)) > 0 {
		var arr []any
		if err := json.Unmarshal(body.Anchors, &arr); err != nil || arr == nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_anchors", "anchors 必须是一个 JSON 数组。", nil))
			return
		}
		anchors = []byte(body.Anchors)
	}
	row, err := a.d.Queries.SubmitAtomCard(r.Context(), sqlc.SubmitAtomCardParams{
		ID: card.ID, FieldValues: []byte(body.FieldValues), EventTrace: []byte(body.EventTrace),
		Anchors: anchors,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, cardDTOOf(row))
}
