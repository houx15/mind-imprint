package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

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

// openingFallback is what postCoachOpening persists/returns when
// ProposeOpeningTurn's real-AI call fails or yields nothing (provider error,
// empty text). It still names all four kick-off things — the design 铁律
// (AI 克制, one framing message, never a barrage) holds even offline.
const openingFallback = "欢迎来到这个写作空间。完整做完一个写作项目，会一路经过 立项 → 阅读 → 写作 → 回顾。我们先一起把研究计划的四件事讨论清楚：目标（research question）、缘由（motivation）、活动与时间（plan）、资源（resources）。准备好开始了吗？"

// coachHistoryWindow caps the raw turns fed into the coach's context (the rest
// live in the spine via fold-on-solidify). Matches the loop's 12-turn window.
const coachHistoryWindow = 12

// postCoach runs one continuous, spine-aware coach turn. Spend endpoint: gates
// on HasEntitlement, meters via RecordLLMCall before any bail.
func (a *API) postCoach(w http.ResponseWriter, r *http.Request) {
	row, ok := a.loadOwnedProjectRow(w, r)
	if !ok {
		return
	}
	projectID := row.ID
	if row.IsDemo {
		httpx.WriteJSON(w, http.StatusOK, cannedCoachReply())
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
		// orchestrator below. The two SUB-AGENTS — reading-library find_sources
		// (ReadingBlock) and reflection (ReviewBlock) — set it so their turns take
		// the retained legacy per-surface path instead (Task 9a): they must NOT be
		// driven by the studio orchestrator. NOTE this is isolation by POSTURE
		// (a different, tool-less producer) and STORAGE SURFACE tag, and by never
		// mutating studio_state — NOT by context window: LoadActiveCoachHistory is
		// surface-agnostic (same project thread), so the LLM context DOES include
		// these sub-agent turns when the orchestrator next runs, and vice versa.
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

	// Load the AI-managed studio_state — the orchestrator reads where the project
	// is from it and returns the next state. Any error/empty → fresh default.
	// Loaded BEFORE the student-turn persist below so that persist can tag the
	// turn with the stage in effect at the START of this turn (过程即数据).
	state := agent.DefaultStudioState()
	if raw, gerr := a.d.Queries.GetStudioState(r.Context(), projectID); gerr == nil && len(raw) > 0 {
		var loaded agent.StudioState
		if json.Unmarshal(raw, &loaded) == nil {
			state = loaded
		}
	}

	// Persist the student turn to the ONE per-project thread under the single
	// continuous `studio` surface — the four rooms are views of one thread now,
	// not separate scope-tagged conversations. Best-effort — the reply does not
	// depend on it, since the current turn is already in `history` above.
	if err := store.AppendProjectCoachMessage(r.Context(), projectID, "user", userInput, "studio", string(state.Stage)); err != nil {
		slog.Warn("coach: persist student turn failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	// Always-on spine projection (D2). The room scope is derived from the open
	// tool (the studio_state), not a client-supplied scope. A build error
	// degrades to no projection rather than failing the turn.
	projection, perr := a.buildSpineProjection(r.Context(), projectID, spineScopeForTool(state.OpenTool))
	if perr != nil {
		slog.Warn("coach: build spine projection failed; proceeding without it", "err", perr, "request_id", httpx.RequestIDFromContext(r.Context()))
		projection = ""
	}

	// Status-router dispatch: derive the project's coarse status, load its short
	// posture (goal + 2-4 tools), and run the turn on the FAST model. The small
	// per-status prompt is what makes the non-reasoning model reliable here.
	fastResolved := resolved
	if a.d.FastChatResolver != nil {
		if fr, ferr := a.d.FastChatResolver(r.Context()); ferr == nil {
			fastResolved = fr
		}
	}
	status := agent.StatusForStage(state.Stage)
	def := agent.StatusRegistry()[status]
	dec, usage, cerr := agent.ProposeStatusTurn(r.Context(), a.d.Provider, fastResolved, def, projection, state, history)

	// Meter BEFORE any bail — a call that yields nothing (or was enforcement-
	// rejected) still cost money. Only when a real call happened (usage > 0).
	// A metering failure never fails the turn.
	if usage.InputTokens > 0 || usage.OutputTokens > 0 {
		if rerr := store.RecordLLMCall(r.Context(), agent.LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "coach",
			Resolved: fastResolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
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
	// Strip any workspace id the model copied into prose (belongs in tool args,
	// never shown to the student). Done here so the persisted thread copy AND the
	// reply are both clean.
	narrate = agent.SanitizeNarrate(narrate)

	// Apply the emitted tools in order onto the loaded state — shared with
	// postCoachStart so the two paths can never drift on how a tool call
	// mutates studio_state / the reply.
	var effects orchestratorToolEffects
	state, effects = a.applyOrchestratorTools(r.Context(), projectID, dec, state, store)
	// Note extraction (2026-08-10): the conversational coach runs REASONING-OFF
	// (v4-pro, thinking disabled — the product owner's routing decision), and a
	// reasoning-off coach does not reliably emit propose_note (or even claim a
	// recording in its narrate). So note-proposing moves to a DEDICATED
	// reasoning-ON extraction: in the framework status, whenever the coach didn't
	// itself propose a note and the student said something substantive, run ONE
	// focused reasoning-on `ExtractProposalNote` to recover the note. Reasoning
	// stays exactly where the judgment is; the chat stays fast. Only in framework
	// (the sole status with propose_note) so it never fills a dim during writing.
	if status == agent.FlowFramework && effects.Note == nil && len(strings.TrimSpace(userInput)) >= 12 {
		noteResolved := fastResolved
		if er, ok := a.resolveEval(r.Context()); ok {
			noteResolved = er // flagship → reasoning ON for the extraction
		}
		if args, nusage, ok := agent.ExtractProposalNote(r.Context(), a.d.Provider, noteResolved, userInput, narrate); ok {
			effects.Note = &noteProposalDTO{Section: args.Section, Value: args.Value}
			if nusage.InputTokens > 0 || nusage.OutputTokens > 0 {
				if rerr := store.RecordLLMCall(r.Context(), agent.LLMCallRow{
					ProjectID: projectID, Surface: "studio", Purpose: "coach_note_recover",
					Resolved: noteResolved, PromptTokens: int32(nusage.InputTokens), CompletionTokens: int32(nusage.OutputTokens),
				}); rerr != nil {
					slog.Warn("coach: record note-recover llm call failed", "err", rerr, "request_id", httpx.RequestIDFromContext(r.Context()))
				}
			}
		}
	}
	// Deterministic flow router: run the funnel (stage floor + plan auto-gen) and
	// compute the one-tap nextStep to the following status. Never auto-advances.
	state, next, autoPlan := a.advanceStudioFlow(r.Context(), projectID, state, effects)
	if autoPlan {
		effects.PlanGenerated = true
	}
	reply.Note = effects.Note
	reply.Question = effects.Question
	reply.Card = effects.Card
	reply.ReviewRequested = effects.ReviewRequested
	reply.PlanGenerated = effects.PlanGenerated
	reply.ResourceNeedAdded = effects.ResourceNeedAdded
	reply.PlanChanged = effects.PlanChanged
	reply.NextStep = next
	reply.ReviewVerdict = takePendingFrameworkVerdict(&state)

	// Persist the AI-managed state (best-effort — a failure logs, never fails the
	// turn). The reply carries the same state back as the directive.
	if b, merr := json.Marshal(state); merr == nil {
		if serr := a.d.Queries.SetStudioState(r.Context(), sqlc.SetStudioStateParams{ID: projectID, StudioState: b}); serr != nil {
			slog.Warn("coach: persist studio_state failed", "err", serr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}
	reply.Narrate = narrate
	reply.Directive = state

	// Persist the assistant narration to the ONE thread (studio surface), tagged
	// with the FINAL stage (post-tool-effects) — where the turn landed. Best-effort.
	if err := store.AppendProjectCoachMessage(r.Context(), projectID, "assistant", narrate, "studio", string(state.Stage)); err != nil {
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

	// Revision recording (Mechanism 1): a coach turn landing in the 写作 or
	// 提案 room is the student asking 印记 for feedback on that room's
	// artifacts — record checkpoints. Best-effort, additive: never changes
	// this handler's response. feedback_ref stays nil (the evaluator pairs
	// checkpoints to the coach's reply by trigger + timestamp).
	//
	// Branches on the SERVER-SIDE studio_state stage, not the client's
	// `scope` field: the studio frontend never sends `scope` on ordinary
	// coach turns (it's reserved for the two isolated sub-agent coaches,
	// find_sources/reflection — see isSubagentCoachScope above, which
	// already dispatched and returned before this point for those). The
	// room a turn actually landed in is `state.Stage`, already reassigned
	// above by applyOrchestratorTools + advanceStudioFlow to its final,
	// post-turn value.
	switch state.Stage {
	case agent.StageBodyWriting:
		a.recordCheckpoints(r.Context(), projectID, triggerAskFeedback, nil, checkpointSnippets, checkpointDraft, checkpointOutline)
	case agent.StageProposalWriting, agent.StageProposalReview:
		a.recordCheckpoint(r.Context(), projectID, checkpointProposal, triggerAskFeedback, nil)
	}

	// A turn is activity — the roster's 最近活跃 depends on it. Best-effort.
	if err := a.d.Queries.TouchProject(r.Context(), projectID); err != nil {
		slog.Warn("coach: touch project failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	// S4 · size-threshold compaction backstop: if the active window overflows,
	// fold the oldest turns into the rolling conversation_digest. Best-effort;
	// never disturbs the reply — only its Compacted flag reflects the outcome.
	reply.Compacted = a.maybeCompactBackstop(r.Context(), projectID)

	// Phase-agnostic link bridge: if the student dropped a new URL in this turn,
	// offer to read it — in ANY phase (topic / proposal / writing / retro), not
	// just the reading room. Free (string scan + one library read), best-effort:
	// a nil offer never changes the reply.
	reply.LinkOffer = a.detectLinkOffer(r.Context(), projectID, userInput)

	httpx.WriteJSON(w, http.StatusOK, reply)
}

// isSubagentCoachScope reports whether scope names one of the two SUB-AGENT
// coaches (Task 9a): reading-library find_sources (ReadingBlock, spawned while
// checking a source) and reflection (ReviewBlock, spawned while writing the
// retrospective). Both are stored under their own `surface` tag and run their
// own posture (ProposeProjectCoachReply — no tools, no studio_state mutation),
// so the studio orchestrator never DRIVES them. But they are NOT isolated by
// context window: they live on the same project thread, and
// LoadActiveCoachHistory (surface-agnostic) means the orchestrator's context
// on its next turn DOES include these sub-agent turns, and these sub-agents'
// context includes the orchestrator's turns too.
func isSubagentCoachScope(scope string) bool {
	return scope == "find_sources" || scope == "reflection"
}

// postCoachSubagentTurn runs one turn of a sub-agent coach (find_sources /
// reflection) — the RETAINED legacy per-surface path. Turns are stored under
// `scope` (never "studio") — same project thread, different surface tag — and
// the reply comes from the always-reply conversational producer
// ProposeProjectCoachReply (no tools, no studio_state mutation — a sub-agent
// never drives project status). The isolation from the studio orchestrator is
// by POSTURE and by never mutating studio_state, NOT by context window:
// LoadActiveCoachHistory pulls the whole thread regardless of surface, so
// these turns still land in the orchestrator's context (and the orchestrator's
// turns in this sub-agent's). The response is still shaped as an
// OrchestratorReply so the frontend contract parses uniformly: `directive`
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

	// Load the current studio_state's stage — sub-agents never mutate
	// studio_state (see the doc comment above), but their turns still carry
	// the stage in effect so the evaluation layer's arc-of-thinking read stays
	// complete across surfaces. Any load/unmarshal error → "" (stored as NULL).
	stage := ""
	if raw, gerr := a.d.Queries.GetStudioState(ctx, projectID); gerr == nil && len(raw) > 0 {
		var loaded agent.StudioState
		if json.Unmarshal(raw, &loaded) == nil {
			stage = string(loaded.Stage)
		}
	}

	// Persist the student turn to THIS sub-agent's own surface (durability for
	// the NEXT turn's context). Best-effort — this turn's reply does not depend
	// on it, since the current turn is already in `history` above.
	if err := store.AppendProjectCoachMessage(ctx, projectID, "user", userInput, scope, stage); err != nil {
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
	narrate = agent.SanitizeNarrate(narrate)

	// Persist the assistant narration to the same sub-agent surface. Best-effort.
	if err := store.AppendProjectCoachMessage(ctx, projectID, "assistant", narrate, scope, stage); err != nil {
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
	// Best-effort; never disturbs the reply — only its Compacted flag reflects
	// the outcome.
	compacted := a.maybeCompactBackstop(ctx, projectID)

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
		Question:  nil,
		Compacted: compacted,
	})
}

// orchestratorToolStore is applyOrchestratorTools' minimal persistence seam —
// summon_card records a coach_proposed event when it offers a card. A small
// local interface (rather than the concrete *agent.sqlcAgentStore, which is
// unexported) so both postCoach's and postCoachStart's
// agent.NewSqlcAgentStore-built stores satisfy it structurally.
type orchestratorToolStore interface {
	AppendEvent(ctx context.Context, row agent.EventRow) error
}

// orchestratorToolEffects carries the reply-shaped bits a tool-apply pass
// produces, alongside the mutated agent.StudioState applyOrchestratorTools
// returns — everything orchestratorReplyDTO needs beyond narrate/directive.
type orchestratorToolEffects struct {
	Note                *noteProposalDTO
	Question            *questionProposalDTO
	Card                *cardProposalWireDTO
	ReviewRequested     bool
	PlanGenerated       bool
	FinishPartRequested bool
	// ResourceNeedAdded — the coach's note_resource_need tool appended a keyword
	// to the 还需要探索的 box this turn; the client refetches the box so it shows.
	ResourceNeedAdded bool
	// PlanChanged — the coach's update_plan tool mutated the plan this turn; the
	// client refetches the plan board so the change shows without a remount.
	PlanChanged bool
}

// applyOrchestratorTools applies one orchestrator turn's emitted tool calls,
// in order, onto state — the DRY extraction shared by postCoach and
// postCoachStart so the two paths can never drift on how a tool call mutates
// studio_state / the reply. Always advances state.UpdatedAtTurn, regardless
// of which (if any) tools fired — mirrors postCoach's prior inline behavior
// exactly.
func (a *API) applyOrchestratorTools(ctx context.Context, projectID uuid.UUID, dec agent.OrchestratorDecision, state agent.StudioState, store orchestratorToolStore) (agent.StudioState, orchestratorToolEffects) {
	var effects orchestratorToolEffects
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
			// id-validation (P3): filterCurateReferenceCall already dropped bad
			// `kind`s; this drops items whose id isn't a real material/snippet id
			// for THIS project, so a hallucinated id never reaches studio_state
			// (the panel a later task renders from these ids).
			if args, aerr := agent.CurateReferenceArgs(tc); aerr == nil {
				state.Reference = a.filterKnownReferences(ctx, projectID, args.Items)
			}
		case "propose_note":
			// Last one wins; NO db write — the student confirms via putProposal.
			if args, aerr := agent.ProposeNoteArgs(tc); aerr == nil {
				effects.Note = &noteProposalDTO{Section: args.Section, Value: args.Value}
			}
		case "summon_card":
			// The orchestrator picked the card; gate on (1) status validity — the
			// card must be in THIS status's deck ∪ the cross-cutting pool (a
			// reading-toolkit card summoned mid-essay is dropped) — and (2) a
			// simple in-flight guard (never offer over a card already
			// proposed/active for this project).
			if args, aerr := agent.SummonCardToolArgs(tc); aerr == nil {
				status := agent.StatusForStage(state.Stage)
				if !agent.IsSummonable(args.CardID, status) {
					// (question-card is intentionally NOT summonable — it's a
					// chatbox button that opens the modal directly, §2.)
					slog.Info("coach: dropped summon_card not valid for status",
						"card", args.CardID, "status", status, "request_id", httpx.RequestIDFromContext(ctx))
				} else if a.cardEligibleForSummon(ctx, projectID, args.CardID) {
					effects.Card = &cardProposalWireDTO{CardID: args.CardID, Reason: args.Reason, NudgeText: args.NudgeText}
					if eerr := store.AppendEvent(ctx, agent.EventRow{
						ProjectID: projectID, Surface: "studio", Type: "coach_proposed",
						Payload: mustJSON(map[string]any{"cardId": args.CardID}),
					}); eerr != nil {
						slog.Warn("coach: append coach_proposed event failed", "err", eerr, "request_id", httpx.RequestIDFromContext(ctx))
					}
				}
			}
		case "request_review":
			state.OpenTool = agent.ToolWriting
			state.WidthTier = agent.WidthWide
			effects.ReviewRequested = true
		case "generate_plan":
			// 印记 triggers plan generation itself (the 生成计划 button is gone). Best-
			// effort: a proposal-empty project just doesn't generate — the narration
			// still lands, 印记 will have nudged for the dims. Open 管理 on success.
			if _, gerr := a.regeneratePlan(ctx, projectID); gerr == nil {
				state.OpenTool = agent.ToolPlan
				state.WidthTier = agent.WidthForTool(agent.ToolPlan)
				// Advance the stage off the proposal side too. The frontend's
				// roomForResume routes openTool=plan by STAGE, so a still-
				// proposal_forming stage would snap the view back to 提案 even
				// though the plan just generated — bump it so 管理 opens.
				if state.Stage == agent.StageTopicDiscussion || state.Stage == agent.StageProposalForming {
					state.Stage = agent.StagePlanGeneration
				}
				effects.PlanGenerated = true
			}
		case "propose_question":
			if args, aerr := agent.ProposeQuestionArgs(tc); aerr == nil && strings.TrimSpace(args.Text) != "" {
				effects.Question = &questionProposalDTO{Text: args.Text}
			}
		case "open_reading":
			// The status router's cross-cutting detour: open the reading room.
			// Returns to the writing status when the student finalizes a source.
			state.OpenTool = agent.ToolReading
			state.WidthTier = agent.WidthForTool(agent.ToolReading)
		case "finish_part":
			// Student signals this writing part is done — the deterministic router
			// (advanceStudioFlow) turns it into the one-tap nextStep to the next
			// status. Per-doc finish persistence is Phase B.
			effects.FinishPartRequested = true
		case "update_plan":
			// 印记 manages the project plan on the student's behalf: mark a task
			// done/doing/todo, add/edit/remove one. A deterministic system action
			// on a system artifact (NOT the student's body text — 铁律 scope), so
			// it applies directly; the client refetches the plan every turn.
			if args, aerr := agent.UpdatePlanArgs(tc); aerr == nil {
				if logMsg, ok := a.applyPlanUpdate(ctx, projectID, args); ok {
					effects.PlanChanged = true
					if logMsg != "" {
						if lerr := a.appendAutoLog(ctx, a.d.Queries, projectID, logMsg); lerr != nil {
							slog.Warn("update_plan: append auto-log failed", "err", lerr, "request_id", httpx.RequestIDFromContext(ctx))
						}
					}
				}
			}
		case "note_resource_need":
			// 印记 drops a keyword/lead into the 还需要探索的 box (§113). Low-stakes,
			// reversible (the student can delete it) — applied directly, deduped.
			if args, aerr := agent.NoteResourceNeedArgs(tc); aerr == nil {
				text := strings.TrimSpace(args.Text)
				if why := strings.TrimSpace(args.Why); why != "" {
					text = text + "（" + why + "）"
				}
				if text != "" && !resourceNeedExists(state.ResourceNeeds, args.Text) {
					state.ResourceNeeds = append(state.ResourceNeeds, agent.ResourceNeed{ID: uuid.NewString(), Text: text})
					effects.ResourceNeedAdded = true
				}
			}
		}
	}
	return state, effects
}

// stageOrder gives a stage's funnel position, for monotonic reconciliation
// (never send a project backward).
func stageOrder(s agent.StudioStage) int {
	switch s {
	case agent.StageTopicDiscussion:
		return 0
	case agent.StageProposalForming:
		return 1
	case agent.StagePlanGeneration:
		return 2
	case agent.StageProposalWriting:
		return 3
	case agent.StageProposalReview:
		return 4
	case agent.StageBodyWriting:
		return 5
	case agent.StageRetrospective:
		return 6
	}
	return 0
}

// reconcileStudioFunnel makes the EARLY funnel deterministic in Go instead of
// trusting the coach model to drive it via set_status/generate_plan. A fast
// (reasoning-off) chaperone reliably proposes notes + narrates but is erratic at
// those structured, MECHANICAL decisions — which don't need an LLM at all. This:
//
//	(1) auto-generates the plan the moment all four required proposal dims are
//	    filled and none exists yet (same gate regeneratePlan enforces), and
//	(2) advances stage + open room to at least the canonical minimum for the
//	    concrete state (started → proposal_forming; plan exists → plan_generation),
//	    never downgrading a project already further along (writing/review/回顾).
//
// The model may still ADVANCE beyond the minimum (into writing etc.) via
// set_status — this only stops it sitting too early or skipping the plan.
// Returns the reconciled state and whether it generated the plan this call.
// reviewVerdictDTO is the wire shape of a gate reviewer's read (slice 2:
// framework readiness). Mirrors contracts' ReviewVerdict.
type reviewVerdictDTO struct {
	Ready       bool     `json:"ready"`
	Why         string   `json:"why"`
	Suggestions []string `json:"suggestions"`
}

func (a *API) reconcileStudioFunnel(ctx context.Context, projectID uuid.UUID, state agent.StudioState) (agent.StudioState, bool) {
	if !state.Started {
		return state, false
	}
	planExists := false
	if items, err := a.d.Queries.ListPlanItems(ctx, projectID); err == nil {
		planExists = len(items) > 0
	}
	planGenerated := false
	if !planExists {
		if prop, err := a.d.Queries.GetProjectProposal(ctx, projectID); err == nil && frameworkReadyForPlan(prop, state.CounterpointsWaived) {
			if _, gerr := a.regeneratePlan(ctx, projectID); gerr == nil {
				planExists = true
				planGenerated = true
				// The framework just became a plan — the reasoning reviewer reads
				// the whole framework once and offers suggestions (all-statuses.md
				// §2). Strong-advisory: does NOT gate plan-gen (already ran). Stash
				// it on the state; the next coach turn surfaces + clears it (this
				// funnel runs on the note-confirm save AND on coach turns).
				if v := a.reviewFrameworkReadiness(ctx, projectID, prop); v != nil {
					state.PendingFrameworkVerdict = v
				}
			}
		}
	}
	minStage := agent.StageProposalForming
	if planExists {
		minStage = agent.StagePlanGeneration
	}
	if stageOrder(state.Stage) < stageOrder(minStage) {
		state.Stage = minStage
		if minStage == agent.StagePlanGeneration {
			state.OpenTool = agent.ToolPlan
		} else {
			state.OpenTool = agent.ToolForming
		}
		state.WidthTier = agent.WidthForTool(state.OpenTool)
	}
	return state, planGenerated
}

// reviewFrameworkReadiness runs the flagship reasoning reviewer over the
// just-completed framework. Best-effort: a nil EvalResolver, a provider error,
// or an unparseable reply → nil (the turn + plan-gen proceed unchanged). Meters
// the call (purpose="framework_review") even on failure — a rejected call still
// cost money.
func (a *API) reviewFrameworkReadiness(ctx context.Context, projectID uuid.UUID, prop sqlc.ProjectProposal) *agent.FrameworkVerdict {
	if a.d.EvalResolver == nil || a.d.Provider == nil {
		return nil
	}
	resolved, rerr := a.d.EvalResolver(ctx)
	if rerr != nil {
		slog.Warn("framework review: no eval resolver", "err", rerr)
		return nil
	}
	title := ""
	if p, err := a.d.Queries.GetProject(ctx, projectID); err == nil {
		title = p.Title
	}
	v, usage, err := agent.ReviewFramework(ctx, a.d.Provider, resolved, agent.FrameworkReviewInput{
		Title: title, Objective: prop.Objective, Reason: prop.Reason,
		Activities: prop.Activities, Resources: prop.Resources, Counterpoints: prop.Counterpoints,
	})
	if usage.InputTokens > 0 || usage.OutputTokens > 0 {
		store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
		if rerr := store.RecordLLMCall(ctx, agent.LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "framework_review",
			Resolved: resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
		}); rerr != nil {
			slog.Warn("framework review: record llm call failed", "err", rerr)
		}
	}
	if err != nil {
		slog.Warn("framework review: reviewer failed — no verdict", "err", err, "request_id", httpx.RequestIDFromContext(ctx))
		return nil
	}
	return &v
}

// takePendingFrameworkVerdict surfaces + clears a stashed framework verdict onto
// the reply. Called after advanceStudioFlow so the verdict shows on the coach
// turn following plan-gen (whether the plan was auto-generated on a note-confirm
// save or on this very turn), exactly once.
func takePendingFrameworkVerdict(state *agent.StudioState) *reviewVerdictDTO {
	if state.PendingFrameworkVerdict == nil {
		return nil
	}
	v := state.PendingFrameworkVerdict
	state.PendingFrameworkVerdict = nil
	return &reviewVerdictDTO{Ready: v.Ready, Why: v.Why, Suggestions: v.Suggestions}
}

// nextStepDTO is the one-tap next-step the deterministic router offers when a
// status milestone is reached (铁律②: 触发自动，打开由学生确认 — the student taps
// to advance; the server never auto-advances the status). Surface is the room
// that opens on advance; ToStatus is the FlowStatus code.
type nextStepDTO struct {
	Label    string `json:"label"`
	ToStatus string `json:"toStatus"`
	Surface  string `json:"surface"`
}

// advanceStudioFlow is the deterministic flow router: it runs the funnel
// (stage floor + plan auto-gen) then, from the concrete project state, computes
// the one-tap nextStep to the following status. It NEVER auto-advances the
// status — it only offers. Transitions: framework (plan exists) → 写提案;
// proposal (finish_part) → 写正文; essay (finish_part) → 复盘.
func (a *API) advanceStudioFlow(ctx context.Context, projectID uuid.UUID, state agent.StudioState, effects orchestratorToolEffects) (agent.StudioState, *nextStepDTO, bool) {
	state, planGenerated := a.reconcileStudioFunnel(ctx, projectID, state)
	if !state.Started {
		return state, nil, planGenerated
	}
	planExists := false
	if items, err := a.d.Queries.ListPlanItems(ctx, projectID); err == nil {
		planExists = len(items) > 0
	}
	next := nextStepFor(agent.StatusForStage(state.Stage), planExists, effects.FinishPartRequested)
	return state, next, planGenerated
}

// nextStepFor is the pure transition decision (no DB): given the current
// status, whether a plan exists, and whether the student asked to finish the
// current part, return the one-tap next step to offer (or nil). Never offers a
// transition the student hasn't earned (framework needs a plan; proposal/essay
// need finish_part).
func nextStepFor(status agent.FlowStatus, planExists, finishPart bool) *nextStepDTO {
	switch status {
	case agent.FlowFramework:
		if planExists {
			return &nextStepDTO{Label: "写研究提案", ToStatus: string(agent.FlowProposal), Surface: string(agent.ToolWriting)}
		}
	case agent.FlowProposal:
		if finishPart {
			// slice 4a · 完成提案 → research (reading room) first, not the writing
			// surface (§5/§6). The essay track lands in the research stage on advance.
			return &nextStepDTO{Label: "去做研究", ToStatus: string(agent.FlowEssay), Surface: string(agent.ToolReading)}
		}
	case agent.FlowEssay:
		if finishPart {
			return &nextStepDTO{Label: "进入复盘", ToStatus: string(agent.FlowReview), Surface: string(agent.ToolReflection)}
		}
	}
	return nil
}

// postCoachOpening runs 印记's real-AI opening welcome — the very first thing
// the student sees on entering a project's studio, before she has said
// anything (Task 3, studio onboarding). ONE tool-less LLM call
// (agent.ProposeOpeningTurn); the reply is persisted as the sole assistant
// turn opening the studio thread — deliberately NO student turn, since the
// student hasn't spoken yet. Idempotent: if the studio thread already carries
// ANY turn (a page reload, a resumed session), this returns 200 with an EMPTY
// narrate and the current directive — no new turn, no spend.
func (a *API) postCoachOpening(w http.ResponseWriter, r *http.Request) {
	row, ok := a.loadOwnedProjectRow(w, r)
	if !ok {
		return
	}
	projectID := row.ID
	if row.IsDemo {
		httpx.WriteJSON(w, http.StatusOK, cannedCoachReply())
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

	state := agent.DefaultStudioState()
	if raw, gerr := a.d.Queries.GetStudioState(r.Context(), projectID); gerr == nil && len(raw) > 0 {
		var loaded agent.StudioState
		if json.Unmarshal(raw, &loaded) == nil {
			state = loaded
		}
	}

	// Idempotency gate: any existing studio turn means the opening already
	// happened. No spend, no new turn — just hand back the current directive.
	surface := "studio"
	rows, lerr := a.d.Queries.ListChatMessagesByProjectSurface(r.Context(), sqlc.ListChatMessagesByProjectSurfaceParams{
		SeededProjectID: pgtype.UUID{Bytes: projectID, Valid: true}, Surface: &surface,
	})
	if lerr != nil {
		httpx.WriteError(w, r, lerr)
		return
	}
	if len(rows) > 0 {
		httpx.WriteJSON(w, http.StatusOK, orchestratorReplyDTO{Narrate: "", Directive: state})
		return
	}

	resolved, rerr := a.d.ChatResolver(r.Context())
	if rerr != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)

	// forming: the opening frames the upcoming 立项 discussion (the four
	// kick-off dims), same scope postCoachStart's first turn projects into.
	projection, perr := a.buildSpineProjection(r.Context(), projectID, "forming")
	if perr != nil {
		slog.Warn("coach opening: build spine projection failed; proceeding without it", "err", perr, "request_id", httpx.RequestIDFromContext(r.Context()))
		projection = ""
	}

	narrate, usage, operr := agent.ProposeOpeningTurn(r.Context(), a.d.Provider, resolved, projection)

	// Meter BEFORE any bail — a call that yields nothing still cost money.
	// Only when a real call happened (usage > 0).
	if usage.InputTokens > 0 || usage.OutputTokens > 0 {
		if mrerr := store.RecordLLMCall(r.Context(), agent.LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "opening",
			Resolved: resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
		}); mrerr != nil {
			slog.Warn("coach opening: record llm call failed", "err", mrerr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}

	narrate = strings.TrimSpace(narrate)
	if operr != nil || narrate == "" {
		if operr != nil {
			slog.Warn("coach opening: propose turn not produced", "err", operr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
		narrate = openingFallback
	}
	narrate = agent.SanitizeNarrate(narrate)

	// Force the landing directive: chat-only, not started — the student has
	// not confirmed she's ready yet (postCoachStart is the explicit gate).
	state.OpenTool = agent.ToolChat
	state.WidthTier = agent.WidthChat
	state.Started = false
	if b, merr := json.Marshal(state); merr == nil {
		if serr := a.d.Queries.SetStudioState(r.Context(), sqlc.SetStudioStateParams{ID: projectID, StudioState: b}); serr != nil {
			slog.Warn("coach opening: persist studio_state failed", "err", serr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}

	// Persist ONLY the assistant welcome — the student hasn't spoken, so no
	// student turn opens the thread.
	if err := store.AppendProjectCoachMessage(r.Context(), projectID, "assistant", narrate, "studio", string(state.Stage)); err != nil {
		slog.Warn("coach opening: persist reply failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	httpx.WriteJSON(w, http.StatusOK, orchestratorReplyDTO{Narrate: narrate, Directive: state})
}

// postCoachStart is the explicit "start" gate (Task 3, studio onboarding):
// the student confirms she's ready, so 印记 opens 提案 (forming) and begins
// the 开题 discussion for real, via a synthetic student turn standing in for
// the button press. Idempotent: once state.Started is already true, returns
// the current directive with no spend and no new turn.
func (a *API) postCoachStart(w http.ResponseWriter, r *http.Request) {
	row, ok := a.loadOwnedProjectRow(w, r)
	if !ok {
		return
	}
	projectID := row.ID
	if row.IsDemo {
		httpx.WriteJSON(w, http.StatusOK, cannedCoachReply())
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

	state := agent.DefaultStudioState()
	if raw, gerr := a.d.Queries.GetStudioState(r.Context(), projectID); gerr == nil && len(raw) > 0 {
		var loaded agent.StudioState
		if json.Unmarshal(raw, &loaded) == nil {
			state = loaded
		}
	}
	if state.Started {
		httpx.WriteJSON(w, http.StatusOK, orchestratorReplyDTO{Narrate: "", Directive: state})
		return
	}

	resolved, rerr := a.d.ChatResolver(r.Context())
	if rerr != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)

	state.Started = true
	state.OpenTool = agent.ToolForming
	state.WidthTier = agent.WidthForTool(agent.ToolForming)
	if state.Stage == agent.StageTopicDiscussion {
		state.Stage = agent.StageProposalForming
	}

	// The button press stands in for a real student utterance — 印记 needs
	// something to open the 提案 discussion FROM. It is persisted (studio
	// surface) like any other turn, not synthesized only in-memory.
	const startUtterance = "我准备好了，开始吧"

	history, herr := store.LoadActiveCoachHistory(r.Context(), projectID, coachHistoryWindow)
	if herr != nil {
		slog.Warn("coach start: load history failed; proceeding on current turn only", "err", herr, "request_id", httpx.RequestIDFromContext(r.Context()))
		history = nil
	}
	history = append(history, agent.ChatTurn{Role: "user", Content: startUtterance})

	if err := store.AppendProjectCoachMessage(r.Context(), projectID, "user", startUtterance, "studio", string(state.Stage)); err != nil {
		slog.Warn("coach start: persist student turn failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	projection, perr := a.buildSpineProjection(r.Context(), projectID, "forming")
	if perr != nil {
		slog.Warn("coach start: build spine projection failed; proceeding without it", "err", perr, "request_id", httpx.RequestIDFromContext(r.Context()))
		projection = ""
	}

	fastResolved := resolved
	if a.d.FastChatResolver != nil {
		if fr, ferr := a.d.FastChatResolver(r.Context()); ferr == nil {
			fastResolved = fr
		}
	}
	status := agent.StatusForStage(state.Stage)
	def := agent.StatusRegistry()[status]
	dec, usage, cerr := agent.ProposeStatusTurn(r.Context(), a.d.Provider, fastResolved, def, projection, state, history)

	// Meter BEFORE any bail — mirrors postCoach exactly.
	if usage.InputTokens > 0 || usage.OutputTokens > 0 {
		if mrerr := store.RecordLLMCall(r.Context(), agent.LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "coach",
			Resolved: fastResolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
		}); mrerr != nil {
			slog.Warn("coach start: record llm call failed", "err", mrerr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}

	reply := orchestratorReplyDTO{}
	narrate := strings.TrimSpace(dec.Narrate)
	if cerr != nil || narrate == "" {
		if cerr != nil {
			slog.Warn("coach start: orchestrator turn not produced", "err", cerr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
		narrate = coachFallbackReply
	}
	narrate = agent.SanitizeNarrate(narrate)

	// Apply the emitted tools — state.Started is untouched by every case in
	// applyOrchestratorTools, so it survives exactly as set above; open_tool
	// may move the room off "forming" if the orchestrator itself named one.
	var effects orchestratorToolEffects
	state, effects = a.applyOrchestratorTools(r.Context(), projectID, dec, state, store)
	// Note backstop (same as postCoach): recover a note 印记 claimed but didn't
	// emit — framework only.
	if status == agent.FlowFramework && effects.Note == nil && agent.ClaimsNoteRecording(narrate) {
		if args, nusage, ok := agent.ExtractProposalNote(r.Context(), a.d.Provider, fastResolved, startUtterance, narrate); ok {
			effects.Note = &noteProposalDTO{Section: args.Section, Value: args.Value}
			if nusage.InputTokens > 0 || nusage.OutputTokens > 0 {
				if rerr := store.RecordLLMCall(r.Context(), agent.LLMCallRow{
					ProjectID: projectID, Surface: "studio", Purpose: "coach_note_recover",
					Resolved: fastResolved, PromptTokens: int32(nusage.InputTokens), CompletionTokens: int32(nusage.OutputTokens),
				}); rerr != nil {
					slog.Warn("coach start: record note-recover llm call failed", "err", rerr, "request_id", httpx.RequestIDFromContext(r.Context()))
				}
			}
		}
	}
	// Deterministic flow router — same as postCoach.
	state, next, autoPlan := a.advanceStudioFlow(r.Context(), projectID, state, effects)
	if autoPlan {
		effects.PlanGenerated = true
	}
	reply.Note = effects.Note
	reply.Question = effects.Question
	reply.Card = effects.Card
	reply.ReviewRequested = effects.ReviewRequested
	reply.PlanGenerated = effects.PlanGenerated
	reply.ResourceNeedAdded = effects.ResourceNeedAdded
	reply.PlanChanged = effects.PlanChanged
	reply.NextStep = next
	reply.ReviewVerdict = takePendingFrameworkVerdict(&state)

	if b, merr := json.Marshal(state); merr == nil {
		if serr := a.d.Queries.SetStudioState(r.Context(), sqlc.SetStudioStateParams{ID: projectID, StudioState: b}); serr != nil {
			slog.Warn("coach start: persist studio_state failed", "err", serr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}
	reply.Narrate = narrate
	reply.Directive = state

	if err := store.AppendProjectCoachMessage(r.Context(), projectID, "assistant", narrate, "studio", string(state.Stage)); err != nil {
		slog.Warn("coach start: persist reply failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "coach_turn",
		Payload: mustJSON(map[string]any{"student": startUtterance, "ai": narrate}),
	}); err != nil {
		slog.Warn("coach start: append coach_turn event failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	if err := a.d.Queries.TouchProject(r.Context(), projectID); err != nil {
		slog.Warn("coach start: touch project failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	reply.Compacted = a.maybeCompactBackstop(r.Context(), projectID)

	httpx.WriteJSON(w, http.StatusOK, reply)
}

// postCoachAdvance acts on a one-tap nextStep (铁律②: 打开由学生确认). The student
// taps "写研究提案"/"开始写正文"/"进入复盘"; the server sets the target status'
// stage floor + opens its surface, then runs ONE coach turn in the NEW status so
// 印记 greets the phase and guides the first move. This is the ONLY way the
// status advances forward — the coach no longer has set_status/open_tool.
func (a *API) postCoachAdvance(w http.ResponseWriter, r *http.Request) {
	row, ok := a.loadOwnedProjectRow(w, r)
	if !ok {
		return
	}
	projectID := row.ID
	if row.IsDemo {
		httpx.WriteJSON(w, http.StatusOK, cannedCoachReply())
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
		ToStatus string `json:"to_status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	target := agent.FlowStatus(strings.TrimSpace(body.ToStatus))
	def, known := agent.StatusRegistry()[target]
	if !known {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "to_status 无效", nil))
		return
	}

	resolved, rerr := a.d.ChatResolver(r.Context())
	if rerr != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	fastResolved := resolved
	if a.d.FastChatResolver != nil {
		if fr, ferr := a.d.FastChatResolver(r.Context()); ferr == nil {
			fastResolved = fr
		}
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)

	state := agent.DefaultStudioState()
	if raw, gerr := a.d.Queries.GetStudioState(r.Context(), projectID); gerr == nil && len(raw) > 0 {
		var loaded agent.StudioState
		if json.Unmarshal(raw, &loaded) == nil {
			state = loaded
		}
	}
	// Advance: set the target status' stage floor + open its surface. Monotonic —
	// never move a project backward (a stray tap can't rewind writing to framework).
	if stageOrder(target.StageFloor()) >= stageOrder(state.Stage) {
		state.Stage = target.StageFloor()
	}
	state.Started = true
	state.OpenTool = def.Surface
	state.WidthTier = agent.WidthForTool(def.Surface)

	// slice 4a · entering the essay lands in the RESEARCH stage first — the
	// reading room, not the writing surface (§5/§6, supersedes Phase B). Seed the
	// 证据地图 backbone into the warren from the proposal once (idempotent).
	if target == agent.FlowEssay {
		if state.EssayTrack == nil {
			state.EssayTrack = &agent.EssayTrack{Stage: agent.EssayResearch}
			mainRQ := ""
			if prop, perr := a.d.Queries.GetProjectProposal(r.Context(), projectID); perr == nil {
				mainRQ = prop.Objective
			}
			var subs []agent.SubQuestion
			if state.ProposalTrack != nil {
				subs = state.ProposalTrack.SubQuestions
			}
			if serr := store.SeedEvidenceMapFromProposal(r.Context(), projectID, mainRQ, subs); serr != nil {
				slog.Warn("coach advance: seed evidence map failed", "err", serr, "request_id", httpx.RequestIDFromContext(r.Context()))
			}
		}
		if state.EssayTrack.Stage == agent.EssayResearch {
			state.OpenTool = agent.ToolReading
			state.WidthTier = agent.WidthForTool(agent.ToolReading)
		}
	}

	const advanceUtterance = "好，我们进入下一步。"
	history, herr := store.LoadActiveCoachHistory(r.Context(), projectID, coachHistoryWindow)
	if herr != nil {
		history = nil
	}
	history = append(history, agent.ChatTurn{Role: "user", Content: advanceUtterance})
	if err := store.AppendProjectCoachMessage(r.Context(), projectID, "user", advanceUtterance, "studio", string(state.Stage)); err != nil {
		slog.Warn("coach advance: persist student turn failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	projection, perr := a.buildSpineProjection(r.Context(), projectID, spineScopeForTool(state.OpenTool))
	if perr != nil {
		projection = ""
	}
	// Run the greeting turn in the NEW status.
	dec, usage, cerr := agent.ProposeStatusTurn(r.Context(), a.d.Provider, fastResolved, def, projection, state, history)
	if usage.InputTokens > 0 || usage.OutputTokens > 0 {
		if mrerr := store.RecordLLMCall(r.Context(), agent.LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "coach",
			Resolved: fastResolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
		}); mrerr != nil {
			slog.Warn("coach advance: record llm call failed", "err", mrerr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}
	reply := orchestratorReplyDTO{}
	narrate := strings.TrimSpace(dec.Narrate)
	if cerr != nil || narrate == "" {
		narrate = coachFallbackReply
	}
	narrate = agent.SanitizeNarrate(narrate)
	var effects orchestratorToolEffects
	state, effects = a.applyOrchestratorTools(r.Context(), projectID, dec, state, store)
	state, next, autoPlan := a.advanceStudioFlow(r.Context(), projectID, state, effects)
	if autoPlan {
		effects.PlanGenerated = true
	}
	reply.Note = effects.Note
	reply.Question = effects.Question
	reply.Card = effects.Card
	reply.ReviewRequested = effects.ReviewRequested
	reply.PlanGenerated = effects.PlanGenerated
	reply.ResourceNeedAdded = effects.ResourceNeedAdded
	reply.PlanChanged = effects.PlanChanged
	reply.NextStep = next
	reply.ReviewVerdict = takePendingFrameworkVerdict(&state)
	reply.Narrate = narrate
	reply.Directive = state

	if b, merr := json.Marshal(state); merr == nil {
		if serr := a.d.Queries.SetStudioState(r.Context(), sqlc.SetStudioStateParams{ID: projectID, StudioState: b}); serr != nil {
			slog.Warn("coach advance: persist studio_state failed", "err", serr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}
	if err := store.AppendProjectCoachMessage(r.Context(), projectID, "assistant", narrate, "studio", string(state.Stage)); err != nil {
		slog.Warn("coach advance: persist reply failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	if err := a.d.Queries.TouchProject(r.Context(), projectID); err != nil {
		slog.Warn("coach advance: touch project failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	// Mechanism-1 advance trigger (best-effort, additive): snapshot the
	// writing-room artifacts as they stand once the advance has landed.
	a.recordCheckpoints(r.Context(), projectID, triggerAdvance, nil, checkpointDraft, checkpointOutline, checkpointSnippets)
	httpx.WriteJSON(w, http.StatusOK, reply)
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
	Question        *questionProposalDTO `json:"question"`
	Card            *cardProposalWireDTO `json:"card"`
	ReviewRequested bool                 `json:"reviewRequested"`
	PlanGenerated   bool                 `json:"planGenerated"`
	Compacted       bool                 `json:"compacted"`
	// ResourceNeedAdded — the coach added a keyword to the 还需要探索的 box this
	// turn (note_resource_need); the client refetches the box. Optional on the wire.
	ResourceNeedAdded bool `json:"resourceNeedAdded,omitempty"`
	// PlanChanged — the coach mutated the plan this turn (update_plan); the client
	// refetches the plan board. Optional on the wire.
	PlanChanged   bool              `json:"planChanged,omitempty"`
	NextStep      *nextStepDTO      `json:"nextStep"`
	ReviewVerdict *reviewVerdictDTO `json:"reviewVerdict"`
	// LinkOffer — a phase-agnostic "you dropped a link, want to read it?" chip
	// when the student's turn carries a new URL not yet in the library. nil when
	// there's no new link (omitted-as-null on the wire).
	LinkOffer *linkOfferDTO `json:"linkOffer"`
}

// noteProposalDTO mirrors the contract's NoteProposal {section, value}.
type noteProposalDTO struct {
	Section string `json:"section"`
	Value   string `json:"value"`
}

// questionProposalDTO mirrors the contract's QuestionProposal {text} — a
// research question 印记 proposes while reading/exploring; the student
// confirms before it joins the exploration graph (mirrors noteProposalDTO).
type questionProposalDTO struct {
	Text string `json:"text"`
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

// coachHistoryDefaultLimit/coachHistoryMaxLimit bound the `limit` query param
// on GET /coach/history — default page size and the hard clamp so a caller
// can't force a load-everything scan via a huge limit.
const (
	coachHistoryDefaultLimit = 20
	coachHistoryMaxLimit     = 100
)

// encodeCoachCursor/decodeCoachCursor: an opaque pagination cursor over the
// composite (created_at, id) total order — base64 of "<unixNano>|<uuid>". id
// breaks created_at ties deterministically (ids are random UUIDs, created_at
// alone is not unique), so paging over this pair never skips or duplicates a
// row. Opaque so the wire format can change without breaking clients that
// just round-trip the string.
func encodeCoachCursor(t time.Time, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%d|%s", t.UnixNano(), id.String())))
}

func decodeCoachCursor(s string) (time.Time, uuid.UUID, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, false
	}
	ns, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	return time.Unix(0, ns).UTC(), id, true
}

// getCoachHistory returns one CURSOR-PAGINATED page of coach turns from the
// ONE per-project thread (folded turns included — a folded turn is still part
// of the visible conversation). No spend — recap reuses the already-stored
// conversation_digest, never a fresh LLM call. Since Task 5 the continuous 印记
// conversation across the working rooms (立项/写作) is stored under one `studio`
// surface, so `surface=studio` reads that surface directly; any other surface
// returns just that room's slice (reading/reflection sub-agents keep their own
// surfaces). Continuity for the AI's own context lives in LoadActiveCoachHistory.
//
// Paging: newest page first (no `before`), `limit+1` rows fetched newest-first
// so hasMore is detectable without a second COUNT query; trimmed to `limit`
// and reversed to oldest→newest for display. `nextCursor` is the (created_at,
// id) of the oldest row on the page, set only when there's more above it.
// `recap` — the digest prose — is only offered on the first page, and only
// when there IS more history hiding behind it (a short thread doesn't need a
// summary of itself).
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

	limit := coachHistoryDefaultLimit
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}
	if limit < 1 {
		limit = 1
	}
	if limit > coachHistoryMaxLimit {
		limit = coachHistoryMaxLimit
	}

	before := strings.TrimSpace(r.URL.Query().Get("before"))
	var (
		beforeCreatedAt time.Time
		beforeID        uuid.UUID
	)
	if before != "" {
		var okCursor bool
		beforeCreatedAt, beforeID, okCursor = decodeCoachCursor(before)
		if !okCursor {
			httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_cursor", "before 游标无效", nil))
			return
		}
	}

	pgProjectID := pgtype.UUID{Bytes: projectID, Valid: true}
	var (
		rows []sqlc.ChatMessage
		err  error
	)
	if before == "" {
		rows, err = a.d.Queries.ListChatMessagesPageLatest(r.Context(), sqlc.ListChatMessagesPageLatestParams{
			SeededProjectID: pgProjectID,
			Surface:         &surface,
			Limit:           int32(limit + 1),
		})
	} else {
		rows, err = a.d.Queries.ListChatMessagesPageBefore(r.Context(), sqlc.ListChatMessagesPageBeforeParams{
			SeededProjectID: pgProjectID,
			Surface:         &surface,
			BeforeCreatedAt: beforeCreatedAt,
			BeforeID:        beforeID,
			PageLimit:       int32(limit + 1),
		})
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	var nextCursor *string
	if hasMore && len(rows) > 0 {
		oldest := rows[len(rows)-1]
		c := encodeCoachCursor(oldest.CreatedAt, oldest.ID)
		nextCursor = &c
	}

	// rows arrive newest→oldest; reverse to oldest→newest for display.
	msgs := make([]coachHistoryMsg, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		m := rows[i]
		role := "student"
		text := m.Content
		if m.Role == "assistant" {
			role = "ai"
			// Clean already-persisted id leaks on read (older messages predate the
			// write-side sanitizer); the student's own text is never touched.
			text = agent.SanitizeNarrate(text)
		}
		msgs = append(msgs, coachHistoryMsg{Role: role, Text: text, Card: cardRefFromAttachments(m.Attachments)})
	}

	var recap *string
	if before == "" && hasMore {
		digest, err := a.d.Queries.GetConversationDigest(r.Context(), projectID)
		if err == nil && strings.TrimSpace(digest.Prose) != "" {
			recap = &digest.Prose
		}
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"messages": msgs, "hasMore": hasMore, "recap": recap, "nextCursor": nextCursor,
	})
}
