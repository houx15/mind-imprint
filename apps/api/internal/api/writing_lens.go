package api

// writing_lens.go — writing's own 工具卡 door: the student-picked summon
// endpoint, POST /writings/{id}/summon. 工具卡 behave IDENTICALLY in the
// reading and writing rooms (AGENTS.md), but writing has a different shape
// of material to hang a card on: it has no article (reading_lens.go's
// liteSummonCard reaches into GetReadingSource for exactly that reason, and
// is deliberately reading-only — see its file comment). What writing DOES
// have is her outline (writing_outline.go) and her fragments
// (writing_snippets.go), so this file grounds the card there instead.
//
// Mirrors reading_lens.go's liteSummonCard shape as closely as the two
// domains allow: same one-open-card mutex (atom_card_one_open_idx, 0096 —
// atom-scoped, not reading-scoped, so it applies here unchanged), same
// chaperone-then-flagship grounding ladder via agent.ProposeCardExample, same
// graceful "opens whether or not an example could be grounded" rule, same
// explicit cardOriginStudent (she picked this card out of the writing tool
// library — nobody routed it to her; writing's own coach turn does not
// summon cards at all yet, see writing_turn.go's file comment).
//
// internal/agent is UNCHANGED by this file. agent.ProposeCardExample and
// agent.Anchor take a domain-agnostic []MaterialBlock (ID+Text) — the exact
// same reuse thesis writing_turn.go already established for
// agent.ProposeProjectCoachReply ("internal/agent required ZERO changes to
// serve this room"). This summon feeds it MaterialBlocks built from outline
// rows and snippet rows instead of article blocks; the brain that picks one
// groundable sentence out of whatever blocks it is handed does not know or
// care where they came from.

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// writingDeckIDs is the fixed set of writing-room tool cards: the four
// 论证写作-category cards in the registry (argument-map/concession/pee/
// toulmin). reading_deck.go's own comment already draws this line — the
// deep-reading tool cards "stay in the writing studio" because their purpose
// ("拆成结构图" / build an argument's scaffolding) doesn't fit reading's
// pick-one-sentence lens mechanic, which is why reading grew its own
// purpose-built 学科透镜 deck instead. This is that studio's side of the
// same split. Growing this deck = adding an id here; inWritingDeck checks it
// against the registry so a typo'd/retired id fails closed rather than
// silently minting a card no renderer can resolve.
var writingDeckIDs = []string{"argument-map", "concession", "pee", "toulmin"}

func inWritingDeck(id string) bool {
	for _, d := range writingDeckIDs {
		if d == id {
			return true
		}
	}
	return false
}

// liteWritingSummonBusyReply/liteWritingSummonUnknownCard/
// liteWritingSummonNoExampleHint mirror reading_lens.go's three constants —
// same restrained, plain-sentence-not-an-error posture. Writing has no
// CRAAP-before-SIFT-style ordering rule (there is nothing analogous to
// source-check-first among the four argument cards), so there is no ordering
// decline here.
const (
	liteWritingSummonBusyReply     = "先完成当前这张工具卡，再换一张。"
	liteWritingSummonUnknownCard   = "这不是可用的写作工具卡。"
	liteWritingSummonNoExampleHint = "这张工具卡就位了——直接从你的提纲或已经写的段落里挑一处来用它。"
)

// writingMaterialBlocks converts her outline rows and snippet rows into the
// domain-agnostic []agent.MaterialBlock ProposeCardExample reasons over.
// Blank rows (an empty outline point, an unstarted snippet slot) contribute
// nothing to ground on and are skipped. IDs are prefixed by kind
// ("outline:"/"snippet:") so a block_id the model echoes back round-trips
// unambiguously to the row it named — the two tables' uuids could otherwise
// collide in this combined list with no other way to tell them apart.
func writingMaterialBlocks(outline []sqlc.WritingOutline, snippets []sqlc.WritingSnippet) []agent.MaterialBlock {
	out := make([]agent.MaterialBlock, 0, len(outline)+len(snippets))
	for _, o := range outline {
		text := strings.TrimSpace(o.Text)
		if text == "" {
			continue
		}
		out = append(out, agent.MaterialBlock{ID: "outline:" + o.ID.String(), Text: text})
	}
	for _, s := range snippets {
		text := strings.TrimSpace(s.Text)
		if text == "" {
			continue
		}
		out = append(out, agent.MaterialBlock{ID: "snippet:" + s.ID.String(), Text: text})
	}
	return out
}

// liteSummonWritingCard mints the card the student chose from the writing
// tool library. NOT curried by kind (mirrors liteSummonCard, for the same
// reason: this handler reaches into writing-only tables — writing_outline,
// writing_snippet — not the generic atom substrate the eight curried
// handlers share).
//
// Grounding: this summon builds its MaterialBlocks from her CURRENT outline
// rows and snippet rows (writingMaterialBlocks above) and asks the same
// flagship-then-chaperone brain reading uses to pick one groundable sentence
// out of them. If she has written NEITHER an outline point nor a snippet yet
// (a brand-new writing, still in 构思), blocks is empty and the model call is
// skipped outright — 铁律②'s restraint applies to spend as much as to
// dialogue, and there is nothing to ask a model to find a sentence in. The
// card still opens: same as reading, "a student-chosen summon must never
// dead-end" — she gets the tool card ungrounded, with a nudge pointing her at
// the outline/snippets herself, exactly like reading's no-example path.
func (a *API) liteSummonWritingCard(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
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

	if !inWritingDeck(cardID) {
		liteWritingSummonDecline(w, liteWritingSummonUnknownCard)
		return
	}
	spec, specOK := cards.ByID(cardID)
	if !specOK {
		// writingDeckIDs named an id the registry no longer has — degrade
		// rather than mint a card no renderer can resolve, mirroring
		// liteSummonCard's identical guard.
		liteWritingSummonDecline(w, liteWritingSummonUnknownCard)
		return
	}

	// Everything from here on runs detached from the request — see
	// detachedModelCtx (reading_lens.go). The reads below feed the model call
	// directly, so they belong on the same context as the work they set up.
	lensCtx, cancelLens := detachedModelCtx(r)
	defer cancelLens()

	cardRows, err := a.d.Queries.ListAtomCards(lensCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// One-active mutex — atom_card_one_open_idx (0096) is atom-scoped, not
	// reading-scoped, so the same rule applies here even before the DB
	// constraint would refuse a second insert.
	for _, c := range cardRows {
		if c.Status == "proposed" || c.Status == "active" {
			liteWritingSummonDecline(w, liteWritingSummonBusyReply)
			return
		}
	}

	outline, err := a.d.Queries.ListWritingOutline(lensCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	snippets, err := a.d.Queries.ListWritingSnippets(lensCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	blocks := writingMaterialBlocks(outline, snippets)

	// Same chaperone-then-flagship ladder as liteSummonCard: grounding ONE
	// illustrative sentence is a lightweight pick, not a reasoning task, with
	// one flagship retry as the safety net. Skipped entirely when there is
	// nothing to ground on (see the handler comment) — a model call with an
	// empty material list would only ever fail to find a block_id anyway, and
	// 铁律② means we don't spend to learn that.
	var (
		anchor    agent.Anchor
		exampleOK bool
	)
	if len(blocks) > 0 {
		groundResolver := a.d.ChatResolver
		usedChaperone := groundResolver != nil
		if !usedChaperone {
			groundResolver = a.d.EvalResolver
		}
		var resolved gateway.Resolved
		var usage gateway.ChatUsage
		anchor, resolved, usage, exampleOK = agent.ProposeCardExample(lensCtx, a.d.Provider, groundResolver, spec, at.ID.String(), blocks)
		a.recordLiteLLMCall(lensCtx, u.ID, at.ID, "write_card_example", resolved, usage)
		if !exampleOK && usedChaperone && a.d.EvalResolver != nil {
			anchor, resolved, usage, exampleOK = agent.ProposeCardExample(lensCtx, a.d.Provider, a.d.EvalResolver, spec, at.ID.String(), blocks)
			a.recordLiteLLMCall(lensCtx, u.ID, at.ID, "write_card_example", resolved, usage)
		}
	}

	// The card opens whether or not the model could ground an example — a
	// student-chosen summon must never dead-end. Without one she simply goes
	// straight to using it on her own outline/snippet text, which is the
	// loop's whole point.
	anchorsJSON := []byte("[]")
	nudge := liteWritingSummonNoExampleHint
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
		// 铁律④ — SHE chose this card out of the writing tool library. Same
		// reasoning as liteSummonCard's identical field: writing's own coach
		// turn does not summon cards at all yet (writing_turn.go), so every
		// card minted through this door is, by construction, her initiative.
		Origin: cardOriginStudent,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// atom_card_one_open_idx refused a SECOND open card: another request
		// opened one between the scan above and this insert. Same race
		// handling as liteSummonCard.
		slog.Info("lite writing summon: lost the one-open-card race; declining",
			"atom_id", at.ID, "card_id", cardID,
			"request_id", httpx.RequestIDFromContext(r.Context()))
		liteWritingSummonDecline(w, liteWritingSummonBusyReply)
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

// liteWritingSummonDecline mirrors liteSummonDecline (reading_lens.go): a
// plain coach-shaped 200, never an HTTP error — nothing went wrong, the room
// is just already busy or she named a card that isn't offered here.
func liteWritingSummonDecline(w http.ResponseWriter, reply string) {
	httpx.WriteJSON(w, http.StatusOK, liteTurnDTO{Reply: reply, Decision: "respond"})
}
