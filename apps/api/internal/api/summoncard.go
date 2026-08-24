package api

// summoncard.go — POST /projects/{id}/materials/{mid}/summon-card (SSE): the
// lens-library summon path. readturn.go's read-together router is the AI
// deciding, on its own initiative, whether to summon a reading card; this
// endpoint is the STUDENT deciding — she opens the 透镜库, picks a card
// herself, and this handler mints it directly rather than asking the router.
// Reuses the same reading-room machinery readturn.go already established:
// card_instance status derivation (open-card mutex, source-check ordering),
// the SSE card/intervention/done vocabulary, and example-anchor persistence
// (agent.ProposeCardExample mirrors readturn.go's ResolveExampleAnchor
// integrity rule — never a (0,0)/empty anchor). Once summoned, the existing
// proposed→active→feedback pick-your-evidence flow takes over unchanged.

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
)

// summonCardReq is summonProjectCard's request body: the id of the card the
// student picked from the lens library.
type summonCardReq struct {
	CardID string `json:"card_id"`
}

// readingDeckIDSet indexes agent.ReadingDeckIDs for O(1) membership checks —
// the lens library may only summon a card that is actually in the reading
// deck, never an arbitrary card id from the whole registry.
func readingDeckIDSet() map[string]bool {
	set := make(map[string]bool, len(agent.ReadingDeckIDs))
	for _, id := range agent.ReadingDeckIDs {
		set[id] = true
	}
	return set
}

// summonProjectCardExampleFallback is the coach line emitted whenever a
// student-chosen summon cannot mint a real card — either ProposeCardExample
// failed to ground an example in the article, or persistence failed after a
// real (billed) LLM call already ran. Never a broken/empty card.
const summonProjectCardExampleFallback = "这篇文章里我一时没找到适合这副透镜的好例子，换个视角或直接问我都行。"

// summonProjectCardNoExampleNudge is the nudge_text on a card minted WITHOUT a
// grounded AI example. A student-chosen summon must never dead-end: when the
// model can't ground a single illustrative sentence, the lens still opens and
// she goes straight to finding her own evidence (the core you-find-the-evidence
// loop) rather than being turned away with summonProjectCardExampleFallback.
const summonProjectCardNoExampleNudge = "这副透镜就位了——直接在文章里挑一句你最想用它来读的话。"

// summonProjectCard lets the student summon a CHOSEN reading card onto a
// material herself. Mirrors postReadingTurn's SSE scaffold (project/material
// load + ownership 404, heartbeat, studioEmitter, RecordLLMCall, done frame)
// but never consults the router — the student already chose.
func (a *API) summonProjectCard(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	mid, err := uuid.Parse(r.PathValue("mid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	u, _ := UserFromContext(r.Context())

	// Entitlement gate — JSON error BEFORE committing to the stream.
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	var body summonCardReq
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	g, err := store.LoadGraph(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	// Ownership: mid must be one of THIS project's materials — mirrors
	// postReadingTurn's parse-based compare (a MaterialView.ID rendering may
	// differ in dashing/case from google/uuid's canonical String()).
	foundMaterial := false
	for _, m := range g.Materials {
		if pid, perr := uuid.Parse(m.ID); perr == nil && pid == mid {
			foundMaterial = true
			break
		}
	}
	if !foundMaterial {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	// Commit to streaming. After this, errors are SSE frames, not JSON.
	sse, err := gateway.NewSSEWriter(w)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	em := &studioEmitter{sse: sse}
	stop, hbDone := startHeartbeat(r.Context(), em)
	defer func() {
		close(stop)
		<-hbDone
	}()

	if !readingDeckIDSet()[body.CardID] {
		_ = em.Intervention("", "这不是可用的阅读透镜。", "", "", "reply")
		_ = em.Done()
		return
	}

	// One-active mutex, project-wide — same rule readturn.go's openCard guard
	// enforces: while any card is proposed/active, nothing new fires.
	for _, ci := range g.CardInstances {
		if ci.Status == "proposed" || ci.Status == "active" {
			_ = em.Intervention("", "先完成文章里当前这副透镜，再换一副。", "", "", "reply")
			_ = em.Done()
			return
		}
	}

	// Source-check ordering, derived the SAME way readturn.go derives
	// AllowCraap/AllowSift — scoped to THIS material, from card_instance
	// status + the anchors persisted on each instance (the only place a
	// card_instance minted by store.CreateCardInstance carries a material id;
	// see readturn.go's onMaterial doc comment for why the evaluates-edge is
	// the wrong signal here).
	matID := mid.String()
	onMaterial := func(ci agent.CardInstanceView) bool {
		for _, an := range ci.Anchors {
			if an.MaterialID == matID {
				return true
			}
		}
		return false
	}
	craapCompleted := false
	for _, ci := range g.CardInstances {
		if ci.Status == "completed" && ci.CardID == "craap" && len(ci.Anchors) > 0 && onMaterial(ci) {
			craapCompleted = true
			break
		}
	}
	if body.CardID == "sift" && !craapCompleted {
		_ = em.Intervention("", "先做完信源体检（CRAAP），再用 SIFT 深挖这篇文章。", "", "", "reply")
		_ = em.Done()
		return
	}
	if body.CardID == "craap" && craapCompleted {
		_ = em.Intervention("", "这篇文章已经做过信源体检了，换一副深读的透镜看看？", "", "", "reply")
		_ = em.Done()
		return
	}

	spec, specOK := cards.ByID(body.CardID)
	if !specOK {
		// The deck listed an id the registry no longer has — should never
		// happen (ReadingDeck() itself errors on drift), but degrade rather
		// than emit a card frame with no spec behind it.
		_ = em.Intervention("", "这不是可用的阅读透镜。", "", "", "reply")
		_ = em.Done()
		return
	}

	materials, err := a.projectMaterials(r.Context(), projectID)
	if err != nil {
		_ = em.Intervention("", summonProjectCardExampleFallback, "", "", "reply")
		_ = em.Done()
		return
	}
	var blocks []agent.MaterialBlock
	for _, m := range materials {
		if m.ID == matID {
			blocks = m.Blocks
			break
		}
	}

	// Grounding one illustrative sentence is a lightweight pick, not a reasoning
	// task: a live A/B showed the chaperone tier (thinking OFF) grounds just as
	// reliably (6/6) at ~2.5s versus the flagship reasoning model's 13–120s. Use
	// it first; on the rare miss, fall back once to the flagship — best of both
	// (fast common path, reasoning safety net) before the no-example degrade.
	groundResolver := a.d.ChatResolver
	usedChaperone := groundResolver != nil
	if !usedChaperone {
		groundResolver = a.d.EvalResolver
	}
	exampleAnchor, resolved, usage, exampleOK := agent.ProposeCardExample(r.Context(), a.d.Provider, groundResolver, spec, matID, blocks)
	a.recordReadingLLMCall(r.Context(), store, projectID, "read_card_example", resolved, usage)
	if !exampleOK && usedChaperone && a.d.EvalResolver != nil {
		exampleAnchor, resolved, usage, exampleOK = agent.ProposeCardExample(r.Context(), a.d.Provider, a.d.EvalResolver, spec, matID, blocks)
		a.recordReadingLLMCall(r.Context(), store, projectID, "read_card_example", resolved, usage)
	}

	// Mint the card whether or not an AI example grounded — the fallback message
	// now fires ONLY on a true persistence error, never merely because the model
	// couldn't distill one illustrative sentence.
	row, cerr := store.CreateCardInstance(r.Context(), projectID, mid, body.CardID, "")
	if cerr != nil {
		slog.Error("summon card: create card instance failed",
			"err", cerr, "request_id", httpx.RequestIDFromContext(r.Context()))
		_ = em.Intervention("", summonProjectCardExampleFallback, "", "", "reply")
		_ = em.Done()
		return
	}

	anchorsJSON := []byte("[]")
	nudge := spec.Name
	if exampleOK {
		if aj, merr := json.Marshal([]agent.Anchor{exampleAnchor}); merr == nil {
			anchorsJSON = aj
		}
		if serr := store.SetCardInstanceAnchors(r.Context(), projectID, row.ID, anchorsJSON); serr != nil {
			slog.Warn("summon card: persist anchors failed",
				"err", serr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	} else {
		// No groundable example — open the lens anyway; she finds her own sentence.
		nudge = summonProjectCardNoExampleNudge
	}
	_ = em.Card(row.ID.String(), body.CardID, nudge, anchorsJSON, matID)
	_ = em.Done()
}
