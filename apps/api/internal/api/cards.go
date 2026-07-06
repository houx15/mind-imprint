package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// traceKinds is the closed set of TraceEvent kinds (unchanged contract).
var traceKinds = map[string]bool{
	"field_change": true, "step_expand": true, "note_open": true, "skip": true, "submit": true,
}

// validateFieldValues requires a JSON object.
func validateFieldValues(raw json.RawMessage) error {
	var obj map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &obj) != nil || obj == nil {
		return httpx.ErrBadRequest("validation_failed", "field_values 必须是对象", nil)
	}
	return nil
}

// validateEventTrace requires a JSON array whose every element has a known kind.
func validateEventTrace(raw json.RawMessage) error {
	var arr []map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &arr) != nil {
		return httpx.ErrBadRequest("validation_failed", "event_trace 必须是对象数组", nil)
	}
	for _, ev := range arr {
		kind, _ := ev["kind"].(string)
		if !traceKinds[kind] {
			return httpx.ErrBadRequest("validation_failed", "event_trace 含未知事件类型", nil)
		}
	}
	return nil
}

// validateAnchors requires a JSON array whose every element is a well-formed
// anchor (structural check only — existence of material/block is not verified).
func validateAnchors(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil // absent anchors is allowed (defaults to [])
	}
	var arr []map[string]any
	if json.Unmarshal(raw, &arr) != nil {
		return httpx.ErrBadRequest("validation_failed", "anchors 必须是数组", nil)
	}
	for _, a := range arr {
		author, _ := a["author"].(string)
		if author != "ai" && author != "student" {
			return httpx.ErrBadRequest("validation_failed", "anchors.author 非法", nil)
		}
		for _, k := range []string{"block_id", "dimension", "question", "quote", "answer", "material_id", "id"} {
			if _, ok := a[k].(string); !ok {
				return httpx.ErrBadRequest("validation_failed", "anchors 字段缺失或类型错误", nil)
			}
		}
		if _, ok := a["start"].(float64); !ok {
			return httpx.ErrBadRequest("validation_failed", "anchors.start 必须是数字", nil)
		}
		if _, ok := a["end"].(float64); !ok {
			return httpx.ErrBadRequest("validation_failed", "anchors.end 必须是数字", nil)
		}
	}
	return nil
}

// ownedCardIDs confirms the task is owned and parses {cid}. On failure writes a
// 404 and returns ok=false.
func (a *API) ownedCardIDs(w http.ResponseWriter, r *http.Request) (taskID, cardID uuid.UUID, ok bool) {
	t, owned := a.loadOwnedTask(w, r)
	if !owned {
		return uuid.Nil, uuid.Nil, false
	}
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return uuid.Nil, uuid.Nil, false
	}
	return t.ID, cid, true
}

// writeCardOrNotFound maps a scoped-update result to {card} or 404 (ErrNoRows).
func writeCardOrNotFound(w http.ResponseWriter, r *http.Request, c sqlc.CardInstance, err error) {
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"card": toCardDTO(c)})
}

func (a *API) patchCard(w http.ResponseWriter, r *http.Request) {
	taskID, cardID, ok := a.ownedCardIDs(w, r)
	if !ok {
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if body.Status != "active" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "status 必须为 active", nil))
		return
	}
	c, err := a.d.Queries.SetCardActive(r.Context(), sqlc.SetCardActiveParams{ID: cardID, TaskID: taskID})
	writeCardOrNotFound(w, r, c, err)
}

func (a *API) putCard(w http.ResponseWriter, r *http.Request) {
	taskID, cardID, ok := a.ownedCardIDs(w, r)
	if !ok {
		return
	}
	var body struct {
		FieldValues json.RawMessage `json:"field_values"`
		EventTrace  json.RawMessage `json:"event_trace"`
		Status      string          `json:"status"`
		Anchors     json.RawMessage `json:"anchors"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if body.Status != "completed" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "status 必须为 completed", nil))
		return
	}
	if err := validateFieldValues(body.FieldValues); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := validateEventTrace(body.EventTrace); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := validateAnchors(body.Anchors); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	c, err := a.d.Queries.SubmitCard(r.Context(), sqlc.SubmitCardParams{
		ID: cardID, TaskID: taskID,
		FieldValues: []byte(body.FieldValues), EventTrace: []byte(body.EventTrace),
	})
	if err != nil {
		writeCardOrNotFound(w, r, c, err)
		return
	}
	anchors := body.Anchors
	if len(anchors) == 0 {
		anchors = json.RawMessage("[]")
	}
	c, err = a.d.Queries.SetCardAnchors(r.Context(), sqlc.SetCardAnchorsParams{
		ID: cardID, TaskID: taskID, Anchors: []byte(anchors),
	})
	if err == nil {
		// A completed card may cross a milestone — best-effort, never blocks the response.
		a.maybeTriggerMilestoneEval(r.Context(), taskID)
	}
	writeCardOrNotFound(w, r, c, err)
}

func (a *API) skipCard(w http.ResponseWriter, r *http.Request) {
	taskID, cardID, ok := a.ownedCardIDs(w, r)
	if !ok {
		return
	}
	var body struct {
		EventTrace json.RawMessage `json:"event_trace"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := validateEventTrace(body.EventTrace); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	c, err := a.d.Queries.SkipCard(r.Context(), sqlc.SkipCardParams{
		ID: cardID, TaskID: taskID, EventTrace: []byte(body.EventTrace),
	})
	writeCardOrNotFound(w, r, c, err)
}
