package api

// reading_lens.go — the two halves of the reading-room card loop that the
// student drives herself, over the lite atom substrate:
//
//	POST /readings/{id}/summon                  —— 透镜库: SHE picks the lens
//	POST /readings/{id}/cards/{cid}/evaluate    —— 选句复核: SHE picks the sentence
//
// reading_turn.go is the AI deciding on its own initiative whether a lens
// helps; these two are the student deciding. Both reuse the same brain
// (`agent.ProposeCardExample`, `agent.EvaluateSelection`) and the same
// restraint rules (one-active mutex, CRAAP-before-SIFT ordering) the pro
// reading room already established — nothing under internal/agent changes.
//
// The summon answers with the SAME liteTurnDTO the coach turn answers with,
// on purpose: the client then has exactly one shape to map into the room's
// card/intervention vocabulary, whether the lens came from the router or from
// her own hand.

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// liteSummonReq is liteSummonCard's body: the id of the card she picked out of
// the lens library.
type liteSummonReq struct {
	CardID string `json:"cardId"`
}

// The three 克制 declines a student-chosen summon can hit, and the nudge for a
// lens that opened without a groundable AI example. All of them still answer
// 200 with a plain coach line rather than an error: being told "finish the one
// already open" is a conversation, not a failure.
const (
	liteSummonBusyReply     = "先完成文章里当前这副透镜，再换一副。"
	liteSummonSiftFirst     = "先做完信源体检（CRAAP），再用 SIFT 深挖这篇文章。"
	liteSummonCraapDone     = "这篇文章已经做过信源体检了，换一副深读的透镜看看？"
	liteSummonUnknownCard   = "这不是可用的阅读透镜。"
	liteSummonNoExampleHint = "这副透镜就位了——直接在文章里挑一句你最想用它来读的话。"
)

// liteSummonCard mints the card the student chose from the lens library.
// Deliberately never consults the router: she already decided, and asking a
// model whether she may is both slower and a small insult.
func (a *API) liteSummonCard(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
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

	var req liteSummonReq
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	cardID := strings.TrimSpace(req.CardID)

	catalog, err := agent.ReadingDeck()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !inReadingDeck(catalog, cardID) {
		liteSummonDecline(w, liteSummonUnknownCard)
		return
	}
	spec, specOK := cards.ByID(cardID)
	if !specOK {
		// The deck named an id the registry no longer has. ReadingDeck() errors
		// on that drift, so this should be unreachable — degrade rather than
		// mint a card no renderer can resolve.
		liteSummonDecline(w, liteSummonUnknownCard)
		return
	}

	cardRows, err := a.d.Queries.ListAtomCards(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// One-active mutex — the same rule the router's PacingState.OpenCard
	// enforces, applied here because this path never reaches the router.
	for _, c := range cardRows {
		if c.Status == "proposed" || c.Status == "active" {
			liteSummonDecline(w, liteSummonBusyReply)
			return
		}
	}
	// Source-check ordering, derived exactly like the router's guard.
	ordering := readingOrderingGuard(cardRows)
	if cardID == "sift" && !ordering.AllowSift {
		liteSummonDecline(w, liteSummonSiftFirst)
		return
	}
	if cardID == "craap" && !ordering.AllowCraap {
		liteSummonDecline(w, liteSummonCraapDone)
		return
	}

	src, err := a.d.Queries.GetReadingSource(r.Context(), at.ID)
	if err != nil {
		// No article pasted yet — there is nothing to hang a lens on.
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	blocks := materialBlocks(SplitBlocks(src.Body))

	// Grounding ONE illustrative sentence is a lightweight pick, not a
	// reasoning task — the pro side measured the chaperone tier grounding just
	// as reliably at a fraction of the latency, with one flagship retry as the
	// safety net. Same ladder here.
	groundResolver := a.d.ChatResolver
	usedChaperone := groundResolver != nil
	if !usedChaperone {
		groundResolver = a.d.EvalResolver
	}
	anchor, resolved, usage, exampleOK := agent.ProposeCardExample(r.Context(), a.d.Provider, groundResolver, spec, at.ID.String(), blocks)
	a.recordLiteLLMCall(r.Context(), u.ID, at.ID, "read_card_example", resolved, usage)
	if !exampleOK && usedChaperone && a.d.EvalResolver != nil {
		anchor, resolved, usage, exampleOK = agent.ProposeCardExample(r.Context(), a.d.Provider, a.d.EvalResolver, spec, at.ID.String(), blocks)
		a.recordLiteLLMCall(r.Context(), u.ID, at.ID, "read_card_example", resolved, usage)
	}

	// The lens opens whether or not the model could ground an example — a
	// student-chosen summon must never dead-end. Without one she simply goes
	// straight to finding her own sentence, which is the loop's whole point.
	anchorsJSON := []byte("[]")
	nudge := liteSummonNoExampleHint
	var blockID *string
	if exampleOK {
		if aj, merr := json.Marshal([]agent.Anchor{anchor}); merr == nil {
			anchorsJSON = aj
			b := anchor.BlockID
			blockID = &b
			nudge = anchor.Question
		}
	}

	row, err := a.d.Queries.CreateAtomCard(r.Context(), sqlc.CreateAtomCardParams{
		AtomID: at.ID, CardID: cardID, BlockID: blockID, Status: "proposed",
		FieldValues: []byte("{}"), EventTrace: []byte("[]"), Anchors: anchorsJSON,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	dto := cardDTOOf(row)
	// 触发是自动的，但「打开」由学生确认 (铁律②) — even a card she asked for
	// arrives 'proposed'; activate is still a separate, recorded step.
	httpx.WriteJSON(w, http.StatusOK, liteTurnDTO{
		Reply: "", Decision: "summon", Card: &dto, Nudge: nudge,
	})
}

// liteSummonDecline answers a refused summon the way the coach would: a plain
// sentence, decision "respond", no card. Not an HTTP error — nothing went
// wrong, the room is just already busy or the ordering says not yet.
func liteSummonDecline(w http.ResponseWriter, reply string) {
	httpx.WriteJSON(w, http.StatusOK, liteTurnDTO{Reply: reply, Decision: "respond"})
}

// liteEvaluateCardSelection judges the sentence the student picked against the
// open card's lens. Mirrors evaluateProjectCard (readeval.go) — same
// agent.EvaluateSelection call, same flagship resolver (never downgraded), same
// "this endpoint never flips status" rule — with the atom substrate underneath
// and the review persisted on the card row instead of card_instances.
func (a *API) liteEvaluateCardSelection(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	card, ok := a.loadOwnedAtomCard(w, r, at.ID)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	// Entitlement before the model call — this spends a flagship-tier call.
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}
	if cardIsTerminal(card.Status) {
		httpx.WriteError(w, r, httpx.ErrConflict("这张卡片已经结束，不能再复核选句。"))
		return
	}

	var body evaluateSelectionReq
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if strings.TrimSpace(body.Quote) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_quote", "先在文章里选一句话。", nil))
		return
	}

	dimension := body.Dimension
	if dimension == "" {
		dimension = card.CardID
	}
	studentSpan := agent.Anchor{
		ID: "sel0", MaterialID: at.ID.String(), BlockID: body.BlockID,
		Start: body.Start, End: body.End, Quote: body.Quote,
		Dimension: dimension, Author: "student",
	}
	spec, _ := cards.ByID(card.CardID)

	eval, resolved, usage, _ := agent.EvaluateSelection(r.Context(), a.d.Provider, a.d.EvalResolver, spec, dimension, studentSpan)
	a.recordLiteLLMCall(r.Context(), u.ID, at.ID, "read_eval", resolved, usage)

	dto := toSelectionEvalDTO(eval)
	// 过程即数据 — the AI's judgment of her pick is recorded, not just returned.
	// A persistence failure only warns: she already has her review on screen,
	// and losing it to a transient DB error would be the worse outcome.
	if frameworkJSON, merr := json.Marshal(dto); merr == nil {
		if _, serr := a.d.Queries.SetAtomCardFramework(r.Context(), sqlc.SetAtomCardFrameworkParams{
			ID: card.ID, FrameworkFill: frameworkJSON,
		}); serr != nil {
			slog.Warn("lite evaluate selection: persist framework failed",
				"err", serr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}
