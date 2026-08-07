package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
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
		UserInput string `json:"user_input"`
		// Scope is OPTIONAL. Empty (the studio callers — 计划/写作) drives the
		// orchestrator below. The two context-isolated SUB-AGENTS — reading-library
		// find_sources (ReadingBlock) and reflection (ReviewBlock) — set it so
		// their turns take the retained legacy per-surface path instead (Task 9a):
		// they must NOT be driven by the studio orchestrator (different thread,
		// different posture, never touches studio_state).
		Scope string `json:"scope"`
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

	resolved, err := a.d.ChatResolver(r.Context())
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}

	if scope := strings.TrimSpace(body.Scope); isSubagentCoachScope(scope) {
		a.postCoachSubagentTurn(w, r, resolved, projectID, scope, userInput)
		return
	}

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)

	// Load the PRIOR active window across the whole thread (continuity ignores
	// surface), then append the current turn ourselves so the orchestrator's
	// context always ends with what the student just said — independent of
	// whether the persist below succeeds. On a load error, prior turns are absent.
	history, herr := store.LoadActiveCoachHistory(r.Context(), projectID, coachHistoryWindow)
	if herr != nil {
		slog.Warn("coach: load history failed; proceeding on current turn only", "err", herr, "request_id", httpx.RequestIDFromContext(r.Context()))
		history = nil
	}
	history = append(history, agent.ChatTurn{Role: "user", Content: userInput})

	// Persist the student turn to the ONE per-project thread under the single
	// continuous `studio` surface — the four rooms are views of one thread now,
	// not separate scope-tagged conversations. Best-effort — the reply does not
	// depend on it, since the current turn is already in `history` above.
	if err := store.AppendProjectCoachMessage(r.Context(), projectID, "user", userInput, "studio"); err != nil {
		slog.Warn("coach: persist student turn failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	// Load the AI-managed studio_state — the orchestrator reads where the project
	// is from it and returns the next state. Any error/empty → fresh default.
	state := agent.DefaultStudioState()
	if raw, gerr := a.d.Queries.GetStudioState(r.Context(), projectID); gerr == nil && len(raw) > 0 {
		var loaded agent.StudioState
		if json.Unmarshal(raw, &loaded) == nil {
			state = loaded
		}
	}

	// Always-on spine projection (D2). The room scope is derived from the open
	// tool (the studio_state), not a client-supplied scope. A build error
	// degrades to no projection rather than failing the turn.
	projection, perr := a.buildSpineProjection(r.Context(), projectID, spineScopeForTool(state.OpenTool))
	if perr != nil {
		slog.Warn("coach: build spine projection failed; proceeding without it", "err", perr, "request_id", httpx.RequestIDFromContext(r.Context()))
		projection = ""
	}

	dec, usage, cerr := agent.ProposeOrchestratorTurn(r.Context(), a.d.Provider, resolved, projection, state, history)

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

	// Build the reply. Narration falls back to a restrained nudge when the
	// orchestrator produced nothing (empty narrate or a hard error).
	reply := orchestratorReplyDTO{}
	narrate := strings.TrimSpace(dec.Narrate)
	if cerr != nil || narrate == "" {
		if cerr != nil {
			slog.Warn("coach: orchestrator turn not produced", "err", cerr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
		narrate = coachFallbackReply
	}

	// Apply the emitted tools in order onto the loaded state. This turn advances
	// the turn counter regardless of which tools fired.
	state.UpdatedAtTurn++
	for _, tc := range dec.Tools {
		switch tc.Name {
		case "set_status":
			if args, aerr := agent.SetStatusArgs(tc); aerr == nil {
				state.Stage = args.Stage
			}
		case "open_tool":
			if args, aerr := agent.OpenToolArgs(tc); aerr == nil {
				state.OpenTool = args.Tool
				state.WidthTier = agent.WidthForTool(args.Tool)
			}
		case "curate_reference":
			if args, aerr := agent.CurateReferenceArgs(tc); aerr == nil {
				state.Reference = args.Items
			}
		case "propose_note":
			// Last one wins; NO db write — the student confirms via putProposal.
			if args, aerr := agent.ProposeNoteArgs(tc); aerr == nil {
				reply.Note = &noteProposalDTO{Section: args.Section, Value: args.Value}
			}
		case "summon_card":
			// The orchestrator picked the card; gate on a simple in-flight guard
			// (never offer over a card already proposed/active for this project).
			if args, aerr := agent.SummonCardToolArgs(tc); aerr == nil {
				if a.cardEligibleForSummon(r.Context(), projectID, args.CardID) {
					reply.Card = &cardProposalWireDTO{CardID: args.CardID, Reason: args.Reason, NudgeText: args.NudgeText}
					if eerr := store.AppendEvent(r.Context(), agent.EventRow{
						ProjectID: projectID, Surface: "studio", Type: "coach_proposed",
						Payload: mustJSON(map[string]any{"cardId": args.CardID}),
					}); eerr != nil {
						slog.Warn("coach: append coach_proposed event failed", "err", eerr, "request_id", httpx.RequestIDFromContext(r.Context()))
					}
				}
			}
		case "request_review":
			state.OpenTool = agent.ToolWriting
			state.WidthTier = agent.WidthWide
			reply.ReviewRequested = true
		}
	}

	// Persist the AI-managed state (best-effort — a failure logs, never fails the
	// turn). The reply carries the same state back as the directive.
	if b, merr := json.Marshal(state); merr == nil {
		if serr := a.d.Queries.SetStudioState(r.Context(), sqlc.SetStudioStateParams{ID: projectID, StudioState: b}); serr != nil {
			slog.Warn("coach: persist studio_state failed", "err", serr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}
	reply.Narrate = narrate
	reply.Directive = state

	// Persist the assistant narration to the ONE thread (studio surface). Best-effort.
	if err := store.AppendProjectCoachMessage(r.Context(), projectID, "assistant", narrate, "studio"); err != nil {
		slog.Warn("coach: persist reply failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	// Record the exchange as a coach_turn event so the process tree carries it.
	// Best-effort.
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "coach_turn",
		Payload: mustJSON(map[string]any{"student": userInput, "ai": narrate}),
	}); err != nil {
		slog.Warn("coach: append coach_turn event failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	// A turn is activity — the roster's 最近活跃 depends on it. Best-effort.
	if err := a.d.Queries.TouchProject(r.Context(), projectID); err != nil {
		slog.Warn("coach: touch project failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	// S4 · size-threshold compaction backstop: if the active window overflows,
	// fold the oldest turns into the rolling conversation_digest. Best-effort;
	// never disturbs the reply.
	a.maybeCompactBackstop(r.Context(), projectID)

	httpx.WriteJSON(w, http.StatusOK, reply)
}

// isSubagentCoachScope reports whether scope names one of the two
// context-isolated SUB-AGENT coaches (Task 9a) that must stay OUTSIDE the
// studio orchestrator's one continuous thread: reading-library find_sources
// (ReadingBlock, spawned while checking a source) and reflection (ReviewBlock,
// spawned while writing the retrospective). Both keep their own retained
// legacy per-surface conversation and their own posture — the orchestrator
// never sees or drives them.
func isSubagentCoachScope(scope string) bool {
	return scope == "find_sources" || scope == "reflection"
}

// postCoachSubagentTurn runs one turn of a context-isolated sub-agent coach
// (find_sources / reflection) — the RETAINED legacy per-surface path, isolated
// from the studio orchestrator's one continuous thread. Turns are stored under
// `scope` (never "studio"), the reply comes from the always-reply conversational
// producer ProposeProjectCoachReply (no tools, no studio_state mutation — a
// sub-agent never drives project status), and the response is still shaped as
// an OrchestratorReply so the frontend contract parses uniformly: `directive`
// carries the project's CURRENT studio_state UNCHANGED, and note/card/
// reviewRequested are always the zero value (nil/nil/false).
func (a *API) postCoachSubagentTurn(w http.ResponseWriter, r *http.Request, resolved gateway.Resolved, projectID uuid.UUID, scope, userInput string) {
	ctx := r.Context()
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)

	// Load the PRIOR active window FIRST, then append the current turn ourselves
	// so the producer's context always ends with what the student just said —
	// independent of whether the persist below succeeds. Mirrors the orchestrator
	// path (and the legacy code this restores) EXACTLY: load-then-append-then-
	// persist. Persisting first would commit the row before this read, so
	// LoadActiveCoachHistory would read it back — duplicating the student's turn
	// in the model's context (fix round 1).
	history, herr := store.LoadActiveCoachHistory(ctx, projectID, coachHistoryWindow)
	if herr != nil {
		slog.Warn("coach: load history failed; proceeding on current turn only", "err", herr, "request_id", httpx.RequestIDFromContext(ctx))
		history = nil
	}
	history = append(history, agent.ChatTurn{Role: "user", Content: userInput})

	// Persist the student turn to THIS sub-agent's own surface (durability for
	// the NEXT turn's context). Best-effort — this turn's reply does not depend
	// on it, since the current turn is already in `history` above.
	if err := store.AppendProjectCoachMessage(ctx, projectID, "user", userInput, scope); err != nil {
		slog.Warn("coach: persist student turn failed", "err", err, "scope", scope, "request_id", httpx.RequestIDFromContext(ctx))
	}

	projection, perr := a.buildSpineProjection(ctx, projectID, scope)
	if perr != nil {
		slog.Warn("coach: build spine projection failed; proceeding without it", "err", perr, "request_id", httpx.RequestIDFromContext(ctx))
		projection = ""
	}

	out, usage, cerr := agent.ProposeProjectCoachReply(ctx, a.d.Provider, resolved, history, projection, coachSurfaceLabel(scope))

	// Meter BEFORE any bail — a call that yields nothing (or was enforcement-
	// rejected) still cost money. Only when a real call happened (usage > 0).
	if usage.InputTokens > 0 || usage.OutputTokens > 0 {
		if rerr := store.RecordLLMCall(ctx, agent.LLMCallRow{
			ProjectID: projectID, Surface: scope, Purpose: "coach",
			Resolved: resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
		}); rerr != nil {
			slog.Warn("coach: record llm call failed", "err", rerr, "request_id", httpx.RequestIDFromContext(ctx))
		}
	}

	narrate := strings.TrimSpace(out.Body)
	if cerr != nil || narrate == "" {
		if cerr != nil {
			slog.Warn("coach: subagent reply not produced", "err", cerr, "scope", scope, "request_id", httpx.RequestIDFromContext(ctx))
		}
		narrate = coachFallbackReply
	}

	// Persist the assistant narration to the same sub-agent surface. Best-effort.
	if err := store.AppendProjectCoachMessage(ctx, projectID, "assistant", narrate, scope); err != nil {
		slog.Warn("coach: persist reply failed", "err", err, "request_id", httpx.RequestIDFromContext(ctx))
	}

	// Record the exchange as a coach_turn event so the process tree carries it.
	// event.surface carries a DB CHECK constraint (migration 0016) admitting only
	// 'studio'|'course'|'chat' — find_sources/reflection are NOT valid values
	// there (unlike chat_message.surface, which is unconstrained), so — mirroring
	// the pre-orchestrator legacy code this path restores (card_reflect.go /
	// e052f95^:coach.go) — the event keeps Surface: "studio" and carries the
	// sub-agent's room scope in the payload instead. Best-effort.
	if err := store.AppendEvent(ctx, agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "coach_turn",
		Payload: mustJSON(map[string]any{"scope": scope, "student": userInput, "ai": narrate}),
	}); err != nil {
		slog.Warn("coach: append coach_turn event failed", "err", err, "request_id", httpx.RequestIDFromContext(ctx))
	}

	// A turn is activity — the roster's 最近活跃 depends on it. Best-effort.
	if err := a.d.Queries.TouchProject(ctx, projectID); err != nil {
		slog.Warn("coach: touch project failed", "err", err, "request_id", httpx.RequestIDFromContext(ctx))
	}

	// S4 · size-threshold compaction backstop, same as the orchestrator path.
	// Best-effort; never disturbs the reply.
	a.maybeCompactBackstop(ctx, projectID)

	// The directive mirrors the project's CURRENT studio_state, UNCHANGED — a
	// sub-agent turn never advances or mutates project status. Loaded exactly
	// as the orchestrator path does: any error/empty → fresh default.
	state := agent.DefaultStudioState()
	if raw, gerr := a.d.Queries.GetStudioState(ctx, projectID); gerr == nil && len(raw) > 0 {
		var loaded agent.StudioState
		if json.Unmarshal(raw, &loaded) == nil {
			state = loaded
		}
	}

	httpx.WriteJSON(w, http.StatusOK, orchestratorReplyDTO{
		Narrate:   narrate,
		Directive: state,
	})
}

// spineScopeForTool maps the AI-managed open tool to the room scope the spine
// projection expects (buildSpineProjection still steers per-room). The single
// studio thread replaced the client-supplied scope, so we derive it here.
func spineScopeForTool(t agent.OpenTool) string {
	switch t {
	case agent.ToolWriting:
		return "writing"
	case agent.ToolReading:
		return "find_sources"
	case agent.ToolReflection:
		return "reflection"
	default: // chat, plan → the立项/计划 forming surface
		return "forming"
	}
}

// orchestratorReplyDTO is the /coach response — mirrors the Task 2
// OrchestratorReply contract field-for-field (camelCase). `note` and `card` are
// nil pointers WITHOUT omitempty so they serialize as JSON null (matching the
// contract's .nullable()), never absent.
type orchestratorReplyDTO struct {
	Narrate         string               `json:"narrate"`
	Directive       agent.StudioState    `json:"directive"`
	Note            *noteProposalDTO     `json:"note"`
	Card            *cardProposalWireDTO `json:"card"`
	ReviewRequested bool                 `json:"reviewRequested"`
}

// noteProposalDTO mirrors the contract's NoteProposal {section, value}.
type noteProposalDTO struct {
	Section string `json:"section"`
	Value   string `json:"value"`
}

// cardProposalWireDTO mirrors the contract's CardProposalWire {cardId, reason,
// nudgeText} — same shape as the retired coachProposalDTO.
type cardProposalWireDTO struct {
	CardID    string `json:"cardId"`
	Reason    string `json:"reason"`
	NudgeText string `json:"nudgeText"`
}

// chatCardRef is the structured card reference stored in a card-turn's
// attachments jsonb (see AppendProjectCoachCardMessage) and projected onto the
// coach-history DTO, so a reloaded thread re-renders a completed card as a
// content-first clickable chip (opening a read-only record) — self-contained,
// no separate fetch. FieldValues is the student's raw answers (the record).
type chatCardRef struct {
	CardID      string          `json:"cardId"`
	FieldValues json.RawMessage `json:"fieldValues"`
}

// chatAttachments is the attachments-jsonb envelope: an optional card reference
// alongside whatever else the column may hold. A plain turn stores '[]', which
// unmarshals to a zero value with Card == nil.
type chatAttachments struct {
	Card *chatCardRef `json:"card,omitempty"`
}

// cardRefFromAttachments decodes a chat_message's attachments jsonb into a card
// reference, returning nil for a plain turn ('[]', empty, or malformed) or a
// card entry with no id. Tolerant: attachments defaults to a JSON array ('[]'),
// which fails object-unmarshal — that's expected and simply means "no card".
func cardRefFromAttachments(raw []byte) *chatCardRef {
	if len(raw) == 0 {
		return nil
	}
	var a chatAttachments
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil
	}
	if a.Card == nil || strings.TrimSpace(a.Card.CardID) == "" {
		return nil
	}
	return a.Card
}

// coachHistoryMsg is the display shape a room loads for its surface-slice of the
// one thread: the workspace ChatMsg shape (role "student"|"ai"), mapped from the
// stored DB roles (user|assistant). Card is set on a card-turn so the room
// renders a chip instead of raw text; omitted on a plain turn.
type coachHistoryMsg struct {
	Role string       `json:"role"`
	Text string       `json:"text"`
	Card *chatCardRef `json:"card,omitempty"`
}

// getCoachHistory returns coach turns from the ONE per-project thread (folded
// turns included — a folded turn is still part of the visible conversation). No
// spend. Since Task 5 the continuous 印记 conversation across the working rooms
// (立项/写作) is stored under one `studio` surface, so `surface=studio` reads that
// surface directly; any other surface returns just that room's slice
// (reading/reflection sub-agents keep their own surfaces). Continuity for the
// AI's own context lives in LoadActiveCoachHistory.
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
		msgs = append(msgs, coachHistoryMsg{Role: role, Text: m.Content, Card: cardRefFromAttachments(m.Attachments)})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}
