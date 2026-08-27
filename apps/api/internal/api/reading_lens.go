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
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

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

// liteModelWorkTimeout caps a detached lens call. Same 150s ceiling
// postLiteReadingTurn's turnCtx uses, and for the same reason: WithoutCancel
// drops cancellation entirely, so something has to bound a genuinely stuck
// provider. 选句复核 has been measured at 45–62 seconds, which this clears
// with room to spare.
const liteModelWorkTimeout = 150 * time.Second

// detachedModelCtx is the one seam this file shares with reading_turn.go: once
// a model call is under way, let the work RUN TO COMPLETION even if the
// student navigates away mid-call.
//
// These are synchronous POSTs on r.Context(), which net/http cancels the
// instant the browser disconnects (refresh / tab-close). Left on that context,
// a refresh during the multi-second flagship call aborts the model call, the
// llm_call metering row AND the write that records the result — money spent,
// nothing recorded, and under 铁律④ a piece of the evidence a later report is
// generated from silently missing. /evaluate is the worst case in the whole
// product: 45–62 seconds of staring at a sentence is exactly when a student
// reloads.
//
// WithoutCancel keeps auth / request-id and drops only cancellation. The
// response write to `w` afterwards is best-effort — it fails harmlessly if she
// already left, but the work is persisted.
func detachedModelCtx(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(r.Context()), liteModelWorkTimeout)
}

// liteSummonCard mints the card the student chose from the lens library.
// Deliberately never consults the router: she already decided, and asking a
// model whether she may is both slower and a small insult.
//
// NOT curried by kind (Task 1.5), on purpose: unlike the eight handlers below
// that only touch atom_card, this one reaches directly into
// GetReadingSource — the reading-only article table — to find something to
// hang the lens on, and its whole shape (blocks-of-an-article) is reading's,
// not a generic atom concept. A writing room summons a lens over an outline
// or a draft snippet, not an article body, so forcing this handler through a
// `kind string` parameter would either lie about supporting writing or grow
// a kind-switch inside the body — worse than an honest reading-only handler.
// Writing's own summon endpoint is a later task's to write.
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

	// Everything from here on runs detached from the request — see
	// detachedModelCtx. The reads below feed the model call directly, so they
	// belong on the same context as the work they set up.
	lensCtx, cancelLens := detachedModelCtx(r)
	defer cancelLens()

	cardRows, err := a.d.Queries.ListAtomCards(lensCtx, at.ID)
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

	src, err := a.d.Queries.GetReadingSource(lensCtx, at.ID)
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
	anchor, resolved, usage, exampleOK := agent.ProposeCardExample(lensCtx, a.d.Provider, groundResolver, spec, at.ID.String(), blocks)
	a.recordLiteLLMCall(lensCtx, u.ID, at.ID, "read_card_example", resolved, usage)
	if !exampleOK && usedChaperone && a.d.EvalResolver != nil {
		anchor, resolved, usage, exampleOK = agent.ProposeCardExample(lensCtx, a.d.Provider, a.d.EvalResolver, spec, at.ID.String(), blocks)
		a.recordLiteLLMCall(lensCtx, u.ID, at.ID, "read_card_example", resolved, usage)
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

	row, err := a.d.Queries.CreateAtomCard(lensCtx, sqlc.CreateAtomCardParams{
		AtomID: at.ID, CardID: cardID, BlockID: blockID, Status: "proposed",
		FieldValues: []byte("{}"), EventTrace: []byte("[]"), Anchors: anchorsJSON,
		// 铁律④ — SHE chose this lens out of the 透镜库. That is the autonomy
		// signal itself, it is not reconstructible from any other column on
		// the row, and the router's own creation site (reading_turn.go)
		// records 'router' for the same reason.
		Origin: cardOriginStudent,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// atom_card_one_open_idx (0096) refused a SECOND open lens: another
		// request opened one between the scan above and this insert. Answer
		// with the same 先完成当前这副透镜 line the scan would have given —
		// from the student's side nothing unusual happened, and the card she
		// can actually see is the one that won.
		//
		// Without the index this branch is unreachable and the insert simply
		// succeeds, leaving a second `proposed` row that getOpenCard never
		// returns: invisible to her, yet blocking every future summon.
		slog.Info("lite summon: lost the one-open-lens race; declining",
			"atom_id", at.ID, "card_id", cardID,
			"request_id", httpx.RequestIDFromContext(r.Context()))
		liteSummonDecline(w, liteSummonBusyReply)
		return
	}
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

// liteEvaluateCardSelectionFor judges the sentence the student picked against
// the open card's lens. Mirrors evaluateProjectCard (readeval.go) — same
// agent.EvaluateSelection call, same flagship resolver (never downgraded), same
// "this endpoint never flips status" rule — with the atom substrate underneath
// and the review persisted on the card row instead of card_instances.
//
// Curried by kind (Task 1.5) — see reading_cards.go's liteListCardsFor:
// everything below reads/writes only atom_card, never a reading-specific
// table, so the same handler serves both editions' card loops. The metering
// purpose string stays "read_eval" — reading is the only kind wired to a
// route today; a later task mounting this under /writings/* should decide
// then whether a kind-specific purpose label is worth adding.
func (a *API) liteEvaluateCardSelectionFor(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		at, ok := a.loadOwnedAtom(w, r, kind)
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

		// THE most abandonable call in the product — 45–62 seconds with nothing on
		// screen but the sentence she picked. Detached, so a refresh at second 50
		// still leaves the metering row and her review behind. See detachedModelCtx.
		evalCtx, cancelEval := detachedModelCtx(r)
		defer cancelEval()

		eval, resolved, usage, _ := agent.EvaluateSelection(evalCtx, a.d.Provider, a.d.EvalResolver, spec, dimension, studentSpan)
		a.recordLiteLLMCall(evalCtx, u.ID, at.ID, "read_eval", resolved, usage)

		dto := toSelectionEvalDTO(eval)
		// 过程即数据 — the AI's judgment of her pick is recorded, not just returned.
		// A persistence failure only warns: she already has her review on screen,
		// and losing it to a transient DB error would be the worse outcome.
		//
		// dto.Degraded rides along: agent.EvaluateSelection NEVER returns an error
		// — it degrades internally to fallbackEval's canned, deliberately generic
		// text ("你选了这句作为证据。") on a resolver failure, a provider failure,
		// or an unparseable reply. That text is not her finding and not the AI's
		// reading of her sentence; the known MaxTokens truncation makes it a
		// COMMON path, not a rare one. Persisting it unmarked would let a P2
		// report count a canned sentence as her own work (铁律①/④), so the flag
		// travels with the payload and a report can exclude it.
		if frameworkJSON, merr := json.Marshal(dto); merr == nil {
			if _, serr := a.d.Queries.SetAtomCardFramework(evalCtx, sqlc.SetAtomCardFrameworkParams{
				ID: card.ID, FrameworkFill: frameworkJSON,
			}); serr != nil {
				slog.Warn("lite evaluate selection: persist framework failed",
					"err", serr, "request_id", httpx.RequestIDFromContext(r.Context()))
			}
		}
		if dto.Degraded {
			slog.Warn("lite evaluate selection: degraded fallback review persisted",
				"atom_id", at.ID, "card_id", card.CardID,
				"request_id", httpx.RequestIDFromContext(r.Context()))
		}
		httpx.WriteJSON(w, http.StatusOK, dto)
	}
}
