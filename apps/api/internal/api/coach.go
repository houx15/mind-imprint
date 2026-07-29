package api

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// coach.go — S1's ONE continuous per-project coach (POST /coach). There is one
// agent, one session per project; the four rooms (计划/阅读/写作/回顾) are views,
// tagged by `scope`, never separate conversations. Each turn:
//   - persists the student turn to the project's ONE thread (surface=scope),
//   - loads the active (non-folded) window across the WHOLE thread,
//   - builds the always-on spine projection + names the active room,
//   - asks the always-reply conversational coach for one restrained reply,
//   - persists that reply, meters, and returns {reply}.
// It embodies the 四条铁律: AI 克制, one question at a time, never gives answers,
// never writes for the student. Card offers / interventions are S4 — not here.

// coachFallbackReply is returned (and persisted) when the model yields nothing
// usable (empty text, a provider error, or an enforcement reject). The student
// is never 500'd on a coach turn — the point is to keep thinking, and a
// restrained nudge does that even offline.
const coachFallbackReply = "先自己说说看——现在你最想弄清楚的是哪一点？"

// coachHistoryWindow caps the raw turns fed into the coach's context (the rest
// live in the spine via fold-on-solidify). Matches the loop's 12-turn window.
const coachHistoryWindow = 12

// postCoach runs one continuous, spine-aware coach turn. Spend endpoint: gates
// on HasEntitlement, meters via RecordLLMCall before any bail.
func (a *API) postCoach(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
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

	var body struct {
		Scope     string `json:"scope"`
		UserInput string `json:"user_input"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	userInput := strings.TrimSpace(body.UserInput)
	if userInput == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "user_input 不能为空", nil))
		return
	}
	scope := body.Scope

	resolved, err := a.d.ChatResolver(r.Context())
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)

	// Load the PRIOR active window across the whole thread (continuity ignores
	// surface), then append the current turn ourselves so the coach's context
	// always ends with what the student just said — independent of whether the
	// persist below succeeds. On a load error, prior turns are simply absent.
	history, herr := store.LoadActiveCoachHistory(r.Context(), projectID, coachHistoryWindow)
	if herr != nil {
		slog.Warn("coach: load history failed; proceeding on current turn only", "err", herr, "request_id", httpx.RequestIDFromContext(r.Context()))
		history = nil
	}
	history = append(history, agent.ChatTurn{Role: "user", Content: userInput})

	// Persist the student turn to the ONE per-project thread (durability for the
	// NEXT turn's context). Best-effort — the reply does not depend on it, since
	// the current turn is already in `history` above.
	if err := store.AppendProjectCoachMessage(r.Context(), projectID, "user", userInput, scope); err != nil {
		slog.Warn("coach: persist student turn failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	// Always-on spine projection (D2). A build error degrades to no projection
	// rather than failing the turn.
	projection, perr := a.buildSpineProjection(r.Context(), projectID)
	if perr != nil {
		slog.Warn("coach: build spine projection failed; proceeding without it", "err", perr, "request_id", httpx.RequestIDFromContext(r.Context()))
		projection = ""
	}

	out, usage, cerr := agent.ProposeProjectCoachReply(r.Context(), a.d.Provider, resolved, history, projection, coachSurfaceLabel(scope))

	// Meter BEFORE any bail — a call that yields nothing (or was enforcement-
	// rejected) still cost money. Only when a real call happened (usage > 0).
	// A metering failure never fails the turn.
	if usage.InputTokens > 0 || usage.OutputTokens > 0 {
		if rerr := store.RecordLLMCall(r.Context(), agent.LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "coach",
			Resolved: resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
		}); rerr != nil {
			slog.Warn("coach: record llm call failed", "err", rerr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}

	reply := strings.TrimSpace(out.Body)
	if cerr != nil || reply == "" {
		if cerr != nil {
			slog.Warn("coach: reply not produced", "err", cerr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
		reply = coachFallbackReply
	}

	// Persist the reply (real or fallback) as the assistant turn, so the thread
	// stays a coherent record for the next turn's context. Best-effort.
	if err := store.AppendProjectCoachMessage(r.Context(), projectID, "assistant", reply, scope); err != nil {
		slog.Warn("coach: persist reply failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	// Record the exchange as a coach_turn event so the process tree carries it
	// (unchanged from the stateless coach). Best-effort.
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "coach_turn",
		Payload: mustJSON(map[string]any{"scope": scope, "student": userInput, "ai": reply}),
	}); err != nil {
		slog.Warn("coach: append coach_turn event failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	// A turn is activity — the roster's 最近活跃 depends on it. Best-effort.
	if err := a.d.Queries.TouchProject(r.Context(), projectID); err != nil {
		slog.Warn("coach: touch project failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{"reply": reply})
}

// coachHistoryMsg is the display shape a room loads for its surface-slice of the
// one thread: the workspace ChatMsg shape (role "student"|"ai"), mapped from the
// stored DB roles (user|assistant).
type coachHistoryMsg struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

// getCoachHistory returns one room's surface-slice of the ONE per-project thread
// (folded turns included — a folded turn is still part of the visible
// conversation). No spend. Continuity across rooms lives in the coach's context
// (LoadActiveCoachHistory), not in this display slice.
func (a *API) getCoachHistory(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	surface := strings.TrimSpace(r.URL.Query().Get("surface"))
	if surface == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "surface 不能为空", nil))
		return
	}
	rows, err := a.d.Queries.ListChatMessagesByProjectSurface(r.Context(), sqlc.ListChatMessagesByProjectSurfaceParams{
		SeededProjectID: pgtype.UUID{Bytes: projectID, Valid: true},
		Surface:         &surface,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	msgs := make([]coachHistoryMsg, 0, len(rows))
	for _, m := range rows {
		role := "student"
		if m.Role == "assistant" {
			role = "ai"
		}
		msgs = append(msgs, coachHistoryMsg{Role: role, Text: m.Content})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}
