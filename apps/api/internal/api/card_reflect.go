package api

// card_reflect.go — Slice 2 · the card-reflect turn. Persists a completed card
// AND runs a coach turn that RESPONDS TO THE CARD'S CONTENT — fixing the old
// forming path, which persisted a card then appended a hardcoded "记下了…" string
// no model ever saw. The used card now leaves a visible, content-bearing student
// turn in the thread, and the coach reacts to what the student actually wrote.
//
// Reusable across rooms (the writing room will call it later), so it stays
// generic: `surface` is the active room's coach scope (e.g. "forming"), which
// both tags the persisted turns and steers the spine projection.
//
// Spend endpoint: it costs a coach turn, so it gates on HasEntitlement and meters
// an llm_call exactly like postCoach. An EMPTY card (no non-empty field) is a
// no-op — no persist, no spend (铁律 · 不操纵 / 过程即数据 without fake work).

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/httpx"
)

// postReflectProjectCard persists a completed card envelope and returns a coach
// reply that responds to the card's compiled content.
func (a *API) postReflectProjectCard(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())

	var body struct {
		CardID      string          `json:"card_id"`
		FieldValues json.RawMessage `json:"field_values"`
		EventTrace  json.RawMessage `json:"event_trace"`
		Surface     string          `json:"surface"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !persistableCard(body.CardID) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "card_id 不在可持久化的卡片范围内", nil))
		return
	}
	surface := strings.TrimSpace(body.Surface)

	// Compile the student-turn text from the card spec + field_values. The spec
	// is looked up from the loader (single source of truth); a card that passed
	// persistableCard is always known, but degrade to a name-only spec so an
	// unexpected miss falls back to the generic key dump rather than dropping
	// the turn.
	var fv map[string]any
	if len(body.FieldValues) > 0 {
		_ = json.Unmarshal(body.FieldValues, &fv) // a non-object decodes to nil → empty compile → no-op
	}
	spec := cards.Spec{ID: body.CardID}
	if a.d.SpecByID != nil {
		if s, okSpec := a.d.SpecByID(body.CardID); okSpec {
			spec = s
		}
	}
	compiled := agent.CompileCardForCoach(spec, fv)

	// Empty card → no-op: do NOT persist, do NOT spend, return empty reply.
	if strings.TrimSpace(compiled) == "" {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"cardInstanceId": "", "reply": ""})
		return
	}

	// This turn spends a coach call → gate before persist so an unentitled
	// student neither persists nor spends.
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	resolved, err := a.d.ChatResolver(r.Context())
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}

	// Persist the completed card (card_instance + card_logged event).
	cardInstanceID, err := a.persistProjectCardEnvelope(r.Context(), projectID, body.CardID, body.FieldValues, body.EventTrace, "card_logged")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)

	// Load the current studio_state's stage so this card turn's persisted rows
	// carry WHERE in the project lifecycle it happened (过程即数据), same
	// convention as postCoach. Any load/unmarshal error → "" (stored as NULL).
	stage := ""
	if raw, gerr := a.d.Queries.GetStudioState(r.Context(), projectID); gerr == nil && len(raw) > 0 {
		var loaded agent.StudioState
		if json.Unmarshal(raw, &loaded) == nil {
			stage = string(loaded.Stage)
		}
	}

	// Load the PRIOR active window, then append the compiled card text as the
	// student's current turn — independent of whether the persist below succeeds
	// (mirrors postCoach). The coach's context always ends with what the student
	// just submitted.
	history, herr := store.LoadActiveCoachHistory(r.Context(), projectID, coachHistoryWindow)
	if herr != nil {
		slog.Warn("card reflect: load history failed; proceeding on current turn only", "err", herr, "request_id", httpx.RequestIDFromContext(r.Context()))
		history = nil
	}
	history = append(history, agent.ChatTurn{Role: "user", Content: compiled})

	// Persist the compiled card as a STUDENT chat_message on this surface so the
	// used card leaves a visible, content-bearing trace (best-effort — the reply
	// does not depend on it, since the current turn is already in `history`).
	// A structured card reference rides in the attachments jsonb so a RELOADED
	// thread re-renders this turn as a content-first clickable chip (opening a
	// read-only record), not raw compiled text; `compiled` stays the fallback.
	fvRaw := body.FieldValues
	if len(fvRaw) == 0 {
		fvRaw = json.RawMessage("{}")
	}
	cardRef := chatCardRef{CardID: body.CardID, FieldValues: fvRaw}
	cardAttachments, merr := json.Marshal(chatAttachments{Card: &cardRef})
	if merr != nil {
		// Fall back to a plain turn — never drop the trace over a marshal error.
		slog.Warn("card reflect: marshal card attachment failed; persisting plain turn", "err", merr, "request_id", httpx.RequestIDFromContext(r.Context()))
		cardAttachments = nil
	}
	if err := store.AppendProjectCoachCardMessage(r.Context(), projectID, "user", compiled, surface, stage, cardAttachments); err != nil {
		slog.Warn("card reflect: persist student card turn failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	projection, perr := a.buildSpineProjection(r.Context(), projectID, surface)
	if perr != nil {
		slog.Warn("card reflect: build spine projection failed; proceeding without it", "err", perr, "request_id", httpx.RequestIDFromContext(r.Context()))
		projection = ""
	}

	out, usage, cerr := agent.ProposeProjectCoachReply(r.Context(), a.d.Provider, resolved, history, projection, coachSurfaceLabel(surface))

	// Meter BEFORE any bail — a call that yields nothing (or was enforcement-
	// rejected) still cost money. Only when a real call happened (usage > 0). A
	// metering failure never fails the turn.
	if usage.InputTokens > 0 || usage.OutputTokens > 0 {
		if rerr := store.RecordLLMCall(r.Context(), agent.LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "coach",
			Resolved: resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
		}); rerr != nil {
			slog.Warn("card reflect: record llm call failed", "err", rerr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}

	reply := strings.TrimSpace(out.Body)
	if cerr != nil || reply == "" {
		if cerr != nil {
			slog.Warn("card reflect: reply not produced", "err", cerr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
		reply = coachFallbackReply
	}

	// Persist the reply as the assistant turn, so the thread stays coherent for
	// the next turn's context. Best-effort.
	if err := store.AppendProjectCoachMessage(r.Context(), projectID, "assistant", reply, surface, stage); err != nil {
		slog.Warn("card reflect: persist reply failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	// Record the exchange as a coach_turn event so the process tree carries it
	// (parity with postCoach). Best-effort.
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "coach_turn",
		Payload: mustJSON(map[string]any{"scope": surface, "student": compiled, "ai": reply, "cardId": body.CardID}),
	}); err != nil {
		slog.Warn("card reflect: append coach_turn event failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	// A turn is activity — the roster's 最近活跃 depends on it. Best-effort.
	if err := a.d.Queries.TouchProject(r.Context(), projectID); err != nil {
		slog.Warn("card reflect: touch project failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	// Echo the persisted card reference so the LIVE turn renders the same
	// content-first chip a reloaded thread does (the client already holds these,
	// but returning them keeps live + reloaded rendering on one shape).
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"cardInstanceId": cardInstanceID.String(), "reply": reply, "card": cardRef})
}
