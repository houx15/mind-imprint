package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// reading_turn.go — 陪练一轮, the lite edition's coach turn.
//
// This file is the whole reuse thesis in one handler. The reading brain —
// agent.RouteReading, agent.ApplyReadingGate, agent.ResolveExampleAnchor,
// agent.ReadingDeck — is a set of PURE functions over plain values: strings,
// slices and small structs, with no reference to a project, a material, or any
// pro table. So it runs UNCHANGED over the atom substrate. Nothing under
// internal/agent was touched to make this work.
//
// All this handler does is the three jobs the brain deliberately does not:
//   1. assemble  — build agent.ReadingRouteInput from the lite tables
//   2. persist   — student turn + AI turn in ONE transaction, plus the card
//   3. meter     — one llm_call row (surface='lite', atom_id)

// recentTurnsWindow bounds how much transcript goes into the router prompt:
// the last 12 atom_message rows, i.e. roughly six student/AI exchanges.
//
// This is load-bearing, not a nicety. The pro side folds a long thread through
// a conversation_digest table; the lite edition has NO compaction layer by
// design, so this window is the ONLY thing standing between a long reading and
// an unbounded prompt (cost, latency, and eventually a context-limit failure
// mid-reading). Twelve is chosen to be short enough that the article — the
// thing the router must actually reason over — keeps dominating the prompt,
// and long enough to carry the local thread of "what were we just discussing".
const recentTurnsWindow = 12

// readingTurnsSinceUnbounded is PacingState.TurnsSinceLastPropose when no card
// has ever been proposed on this reading: "well past the breathing-room
// window", so the gate's proposalBreathingTurns check cannot soften the very
// first summon into a hint. Once a card exists the real count is derived from
// the transcript (student turns since that card was created) — the lite tables
// carry both timestamps, so unlike the pro side this needs no approximation.
const readingTurnsSinceUnbounded = 99

// liteTurnFocusSpan is one span the student has highlighted while reading.
type liteTurnFocusSpan struct {
	BlockID string `json:"blockId"`
	Quote   string `json:"quote"`
}

// liteTurnReq is postLiteReadingTurn's request body.
type liteTurnReq struct {
	Text         string              `json:"text"`
	FocusedSpans []liteTurnFocusSpan `json:"focusedSpans"`
}

// liteTurnDTO is one turn's answer: what the coach said, what it decided, the
// card it proposed (nil unless the decision survived the gate as a summon),
// and the student-facing one-liner inviting her to open it.
//
// HintCardID is the other half of that invitation. On a `hint` the router is
// instructed to name the card it has in mind ("当学生的问题明显对应某副卡…
// 优先给 hint 并在 card_id 填上那张卡——这会变成一个「要不要用它看看」的邀请",
// buildRouterPrompt) — and a summon the gate SOFTENS for breathing room lands
// on this path too, carrying its CardID. Without the id the client can only
// render a naked sentence, and 铁律②'s invitation half ("触发是自动的，但打开
// 由学生确认") degrades into an unanswerable remark. Null whenever the decision
// names no card.
type liteTurnDTO struct {
	Reply      string   `json:"reply"`
	Decision   string   `json:"decision"`
	Card       *cardDTO `json:"card"`
	Nudge      string   `json:"nudge"`
	HintCardID *string  `json:"hintCardId"`
}

type liteMessageDTO struct {
	Seq       int32  `json:"seq"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
}

// liteListMessages returns the whole transcript, oldest first. The client
// renders it; the ROUTER only ever sees the windowed tail (recentTurnsWindow).
func (a *API) liteListMessages(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListAtomMessages(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]liteMessageDTO, 0, len(rows))
	for _, m := range rows {
		out = append(out, liteMessageDTO{
			Seq: m.Seq, Role: m.Role, Content: m.Content,
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"messages": out})
}

// buildReadingRouteInput assembles agent.ReadingRouteInput entirely from the
// lite tables. Split out from the handler so the assembly can be reasoned
// about (and tested) on its own — it is the one place where "new storage" is
// translated into "the plain values the old brain already speaks".
//
// It also returns the article's blocks, which the caller needs to hand to
// agent.ResolveExampleAnchor on a summon. They come from the same
// reading_source row the Article is rendered from, so returning them here is
// what keeps the turn to a single read of the body instead of two.
//
// A missing reading_source surfaces as pgx.ErrNoRows, which the caller maps to
// a 404: reading WITH her before she has pasted anything is not a thing.
func (a *API) buildReadingRouteInput(ctx context.Context, atomID uuid.UUID, studentText string, spans []agent.FocusSpan) (agent.ReadingRouteInput, []Block, error) {
	src, err := a.d.Queries.GetReadingSource(ctx, atomID)
	if err != nil {
		return agent.ReadingRouteInput{}, nil, err
	}
	blocks := SplitBlocks(src.Body)

	// Article — the blocks WITH their ids. The router is asked to name an
	// example_block_id on a summon, so the ids have to be visible to it;
	// agent.ResolveExampleAnchor then validates the quote it picked back
	// against the very same block.
	var art strings.Builder
	for _, b := range blocks {
		fmt.Fprintf(&art, "[%s] %s\n", b.ID, b.Text)
	}

	msgs, err := a.d.Queries.ListAtomMessages(ctx, atomID)
	if err != nil {
		return agent.ReadingRouteInput{}, nil, err
	}
	// RecentTurns — the windowed TAIL, never the whole thread. See
	// recentTurnsWindow: this is the lite edition's only bound on prompt growth.
	tail := msgs
	if len(tail) > recentTurnsWindow {
		tail = tail[len(tail)-recentTurnsWindow:]
	}
	recent := make([]string, 0, len(tail))
	for _, m := range tail {
		who := "教练"
		if m.Role == "student" {
			who = "学生"
		}
		recent = append(recent, who+"："+m.Content)
	}

	// Brief — why she is reading THIS. Absent is normal (she may not have
	// written one), so a missing row degrades to the zero brief, which
	// buildReadingRouteUserPrompt renders byte-for-byte as no brief block at
	// all. ProposalSnap stays empty: a lite reading has no owning proposal.
	// A MISSING row is normal; any other error is not, and swallowing it would
	// quietly strip her reading purpose out of the prompt on a transient DB
	// failure — the degrade-quietly pattern this turn otherwise rejects.
	var brief agent.ReadingBrief
	row, berr := a.d.Queries.GetReadingBrief(ctx, atomID)
	switch {
	case berr == nil:
		brief = agent.ReadingBrief{
			Reason: row.ReadingReason, Focus: row.ReadingFocus, PhaseTag: derefOr(row.PhaseTag, ""),
		}
	case !errors.Is(berr, pgx.ErrNoRows):
		return agent.ReadingRouteInput{}, nil, berr
	}

	catalog, err := agent.ReadingDeck()
	if err != nil {
		return agent.ReadingRouteInput{}, nil, err
	}

	cardRows, err := a.d.Queries.ListAtomCards(ctx, atomID)
	if err != nil {
		return agent.ReadingRouteInput{}, nil, err
	}
	pacing := readingPacing(cardRows, msgs, len(spans) > 0)

	return agent.ReadingRouteInput{
		StudentText:  studentText,
		Article:      art.String(),
		FocusedSpans: spans,
		RecentTurns:  recent,
		Catalog:      catalog,
		// ScaffoldLevels is the pro side's per-card guidance ladder; the lite
		// reading has no such ladder, and the router does not read the field
		// today. Left nil rather than faked.
		ScaffoldLevels: nil,
		Pacing:         pacing,
		Brief:          brief,
	}, blocks, nil
}

// readingPacing derives PacingState from this reading's own card rows and
// transcript — the same restraint signals the pro side reads off card_instance
// statuses, translated to atom_card's vocabulary ('submitted' is lite's
// 'completed'; a skip is a first-class status either way, 铁律④).
func readingPacing(cards []sqlc.AtomCard, msgs []sqlc.AtomMessage, hasNewFocus bool) agent.PacingState {
	p := agent.PacingState{
		TurnsSinceLastPropose: readingTurnsSinceUnbounded,
		HasNewFocus:           hasNewFocus,
	}
	var latestProposedAt time.Time
	for _, c := range cards {
		switch c.Status {
		case "proposed", "active":
			// One-active mutex: while a card is open, nothing new fires.
			p.OpenCard = true
		case "submitted":
			p.CompletedCards = append(p.CompletedCards, c.CardID)
		case "skipped":
			p.RecentlySkipped = append(p.RecentlySkipped, c.CardID)
		}
		if c.CreatedAt.After(latestProposedAt) {
			latestProposedAt = c.CreatedAt
		}
	}
	if !latestProposedAt.IsZero() {
		n := 0
		for _, m := range msgs {
			if m.Role == "student" && m.CreatedAt.After(latestProposedAt) {
				n++
			}
		}
		p.TurnsSinceLastPropose = n
	}
	return p
}

// readingOrderingGuard is the source-check ordering prior: don't summon SIFT
// before this article has been through CRAAP, and don't re-summon CRAAP on an
// article already evaluated (or with one in flight). Same rule the pro side
// applies, derived here from atom_card statuses.
func readingOrderingGuard(cards []sqlc.AtomCard) agent.OrderingGuard {
	var craapDone, craapInFlight, siftDone, siftInFlight bool
	for _, c := range cards {
		inFlight := c.Status == "proposed" || c.Status == "active"
		switch c.CardID {
		case "craap":
			craapDone = craapDone || c.Status == "submitted"
			craapInFlight = craapInFlight || inFlight
		case "sift":
			siftDone = siftDone || c.Status == "submitted"
			siftInFlight = siftInFlight || inFlight
		}
	}
	return agent.OrderingGuard{
		AllowCraap: !craapDone && !craapInFlight,
		AllowSift:  craapDone && !siftDone && !siftInFlight,
	}
}

// postLiteReadingTurn drives one coach turn.
//
// Named postLiteReadingTurn, not postReadingTurn: internal/api is ONE package
// and readturn.go's postReadingTurn (the pro read-together SSE endpoint)
// already owns that name.
func (a *API) postLiteReadingTurn(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	// Entitlement is checked BEFORE the model call — this is a token-spending
	// endpoint, which is exactly where the HasEntitlement seam belongs.
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	var req liteTurnReq
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	studentText := strings.TrimSpace(req.Text)
	if studentText == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_text", "先说点什么，我在听。", nil))
		return
	}
	spans := make([]agent.FocusSpan, 0, len(req.FocusedSpans))
	for _, s := range req.FocusedSpans {
		spans = append(spans, agent.FocusSpan{BlockID: s.BlockID, Quote: s.Quote})
	}

	// Once the turn is under way, let it RUN TO COMPLETION even if the student
	// navigates away mid-reply. This is a synchronous POST on r.Context(), which
	// net/http cancels the instant the browser disconnects (refresh / tab-close)
	// — so a refresh during the multi-second flagship call would abort the model
	// call, the metering row, AND the transaction below, spending money and
	// recording nothing while she loses the answer she already paid for
	// (2026-08-25 edge findings; same pattern and cap as coach.go's turnCtx).
	// WithoutCancel keeps auth / request-id and drops only cancellation; the
	// 150s cap still bounds a genuinely stuck call. The response write to `w` at
	// the end is best-effort — it fails harmlessly if she already left, but the
	// work is persisted.
	turnCtx, cancelTurn := context.WithTimeout(context.WithoutCancel(r.Context()), 150*time.Second)
	defer cancelTurn()

	// 1. 装配 — everything below comes from the lite tables alone.
	in, blocks, err := a.buildReadingRouteInput(turnCtx, at.ID, studentText, spans)
	if err != nil {
		httpx.WriteError(w, r, err) // pgx.ErrNoRows (no article pasted) → 404
		return
	}
	// The ordering guard is NOT part of ReadingRouteInput — ApplyReadingGate
	// takes it separately — so it is derived here. That re-reads atom_card,
	// which buildReadingRouteInput also read for pacing: a deliberate second
	// read of a small per-atom indexed table, kept so buildReadingRouteInput
	// stays self-contained and independently testable. It is immaterial next
	// to the flagship call this turn is about to make.
	cardRows, err := a.d.Queries.ListAtomCards(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	ordering := readingOrderingGuard(cardRows)

	// 2. 复用 AI 大脑，一行没改.
	raw, resolved, usage, rerr := agent.RouteReading(turnCtx, a.d.Provider, a.d.EvalResolver, in)

	// Meter BEFORE any bail: a call that reached a provider cost money whatever
	// happens to its reply. A metering failure only warns — it never fails the
	// turn, and it must never be the reason a student loses her answer.
	a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "reading_turn", resolved, usage)

	// USER RULE: an AI-dialogue failure is surfaced as a real 502, NEVER masked
	// by a canned stand-in sentence — a fake reply disguises a dead turn as
	// normal conversation and the student keeps talking to nothing.
	//
	// RouteReading is deliberately forgiving: it returns (respond, …, nil) on a
	// resolver error, a network error, or an unparseable reply — its `error`
	// return is in practice always nil. So the honest failure signal here is
	// its own contract's invariant instead: `reply` is ALWAYS filled on a real
	// answer ("reply 永远不留空"), and only the degraded fallback comes back
	// empty. Checked on the RAW decision, before the gate — ApplyReadingGate
	// legitimately returns a bare respond (empty Reply) when it suppresses a
	// summon, AND an empty-Reply hint when it softens one, and that is restraint
	// working, not a failure. DO NOT move this below the gate: it would 502 on
	// every act of restraint. reading_turn_gate_test.go holds the tripwire.
	if rerr != nil || strings.TrimSpace(raw.Reply) == "" {
		slog.Warn("lite reading turn: model turn failed; surfacing to student",
			"err", rerr, "atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	// The reply is captured from the RAW decision for the same reason: the gate
	// drops it when it downgrades, and she should still get the answer the
	// model actually gave her.
	reply := strings.TrimSpace(raw.Reply)
	decision := agent.ApplyReadingGate(raw, in.Pacing, ordering)

	// A summon must land on a real, verbatim sentence in a real block —
	// ResolveExampleAnchor is what proves that. If it can't, the card is
	// dropped and the turn degrades to a plain answer: a card hanging off a
	// quote that isn't in the article lights up nothing and teaches nothing.
	// (The pro side retries the router once here; lite does not — every call
	// is metered and a second flagship call to rescue a rare bad quote costs
	// more than it saves, and she still gets her answer either way.)
	var blockID *string
	if decision.Decision == "summon" {
		if !inReadingDeck(in.Catalog, decision.CardID) {
			// The router named a card outside the deck it was given. Never mint
			// a card row for an id no renderer can resolve.
			slog.Warn("lite reading turn: router named a card outside the deck",
				"card_id", decision.CardID, "atom_id", at.ID)
			decision = agent.ReadingDecision{Decision: "respond"}
		} else if anchor, aok := agent.ResolveExampleAnchor(decision, at.ID.String(), materialBlocks(blocks)); aok {
			b := anchor.BlockID
			blockID = &b
		} else {
			slog.Info("lite reading turn: example quote not verbatim; degrading to respond",
				"card_id", decision.CardID, "block_id", decision.ExampleBlockID, "atom_id", at.ID)
			decision = agent.ReadingDecision{Decision: "respond"}
		}
	}

	// 3. 落库 — the student turn, the AI turn and (on a summon) the card, all in
	// ONE transaction. seq is allocated inside it and the (atom_id, seq) unique
	// index is what actually protects the transcript's order against a
	// concurrent second turn: a racing pair either serializes or one fails
	// outright, never interleaves into a scrambled thread.
	tx, err := a.d.Pool.Begin(turnCtx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(turnCtx) }()
	qtx := a.d.Queries.WithTx(tx)

	next, err := qtx.NextAtomMessageSeq(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.AppendAtomMessage(turnCtx, sqlc.AppendAtomMessageParams{
		AtomID: at.ID, Seq: next, Role: "student", Content: studentText,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.AppendAtomMessage(turnCtx, sqlc.AppendAtomMessageParams{
		AtomID: at.ID, Seq: next + 1, Role: "ai", Content: reply,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	var card *cardDTO
	if decision.Decision == "summon" {
		row, cerr := qtx.CreateAtomCard(turnCtx, sqlc.CreateAtomCardParams{
			AtomID: at.ID, CardID: decision.CardID, BlockID: blockID, Status: "proposed",
			FieldValues: []byte("{}"), EventTrace: []byte("[]"),
		})
		if cerr != nil {
			httpx.WriteError(w, r, cerr)
			return
		}
		dto := cardDTOOf(row)
		card = &dto
	}
	if err := tx.Commit(turnCtx); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// On a hint, forward the card the router named so the nudge can be rendered
	// as a real 要不要用它看看 invitation rather than a naked sentence. Validated
	// against the same deck as a summon — an id no renderer can resolve is worse
	// than no id. A gate-SOFTENED summon arrives here too, which is exactly the
	// case where the invitation matters most.
	var hintCardID *string
	if decision.Decision == "hint" && inReadingDeck(in.Catalog, decision.CardID) {
		id := decision.CardID
		hintCardID = &id
	}

	// 触发是自动的，但「打开」由学生确认 (铁律②): the card comes back
	// 'proposed' with an invitation, never already open.
	httpx.WriteJSON(w, http.StatusOK, liteTurnDTO{
		Reply: reply, Decision: decision.Decision, Card: card,
		Nudge: decision.Reason, HintCardID: hintCardID,
	})
}

// inReadingDeck reports whether id is one of the cards the router was actually
// offered. agent.ReadingDeck already resolved every entry against the card
// registry, so membership here is both "a real card" and "a card allowed in
// the reading room" — the catalog handed to the model is the authority.
func inReadingDeck(catalog []agent.ReadingCard, id string) bool {
	for _, c := range catalog {
		if c.CardID == id {
			return true
		}
	}
	return false
}

// materialBlocks converts the lite Block slice to the agent's MaterialBlock —
// the same two fields, which is precisely why the reading brain needs no
// knowledge of where the paragraphs came from.
func materialBlocks(blocks []Block) []agent.MaterialBlock {
	out := make([]agent.MaterialBlock, 0, len(blocks))
	for _, b := range blocks {
		out = append(out, agent.MaterialBlock{ID: b.ID, Text: b.Text})
	}
	return out
}

// recordLiteLLMCall meters one lite model call onto llm_call: surface 'lite',
// project_id NULL (a lite atom owns no project, exactly as course/chat calls
// own none), atom_id pointing back at the reading. Cost is computed here from
// the shared pricing table rather than trusted from a caller.
//
// A call that never reached a provider (resolved.Provider empty) is not
// recorded — there was nothing to bill. Metering failures warn and are
// swallowed: usage accounting must never cost a student her turn.
func (a *API) recordLiteLLMCall(ctx context.Context, userID, atomID uuid.UUID, purpose string, resolved gateway.Resolved, usage gateway.ChatUsage) {
	if resolved.Provider == "" {
		return
	}
	cost, priced := gateway.EstimateCost(resolved.Provider, resolved.Model, usage.InputTokens, usage.OutputTokens)
	if !priced {
		slog.Warn("lite llm_call: unpriced model — cost recorded as 0",
			"provider", resolved.Provider, "model", resolved.Model)
	}
	if _, err := a.d.Queries.RecordAtomLLMCall(ctx, sqlc.RecordAtomLLMCallParams{
		UserID:  userID,
		AtomID:  pgtype.UUID{Bytes: atomID, Valid: true},
		Surface: "lite", Purpose: purpose,
		Provider: resolved.Provider, Model: resolved.Model, Tier: resolved.Tier,
		PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
		CostEstimate: gateway.CostNumeric(cost, true),
	}); err != nil {
		slog.Warn("lite reading turn: record llm usage failed",
			"err", err, "purpose", purpose, "request_id", httpx.RequestIDFromContext(ctx))
	}
}
