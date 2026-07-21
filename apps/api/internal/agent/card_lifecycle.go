package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"mindimprint/api/internal/cards"
)

// SurfaceCard instantiates a `proposed` card_instance for spec onto
// materialID (design §3: `surface_card(card_id, target, entry_stage)`) —
// no model call, no enforcement: surfacing just offers the card. The
// student still confirms opening it (AGENTS.md rule 2, "打开由学生确认");
// that confirmation is a Slice-5 UI/transport concern, out of scope here.
// Records a card_instance->material graph_edge so a later
// SurfaceCardCandidates pass (classifier.go) sees the material as already
// spoken for and never re-proposes it.
func SurfaceCard(ctx context.Context, deps AgentDeps, projectID uuid.UUID, spec cards.Spec, materialID uuid.UUID) (*Action, error) {
	row, err := deps.Store.CreateCardInstance(ctx, projectID, materialID, spec.ID, spec.ID)
	if err != nil {
		return nil, err
	}

	// A project-scoped card (materialID == uuid.Nil, e.g. toulmin) is ABOUT the
	// whole project, not one source: there is no material to point an
	// `evaluates` edge at, so skip the edge entirely. The frame the client sees
	// must carry the EMPTY string as material_id — never uuid.Nil.String() (the
	// all-zeros uuid), which would read as a real, missing material — so the
	// client knows this card has no material.
	materialRef := ""
	if materialID != uuid.Nil {
		materialRef = materialID.String()
		edge := MintEdge{
			Type:     "evaluates",
			FromKind: "card_instance",
			FromID:   row.ID.String(),
			ToKind:   "material",
			ToID:     materialRef,
		}
		if err := deps.Store.InsertGraphEdge(ctx, projectID, edge); err != nil {
			return nil, err
		}
	}

	eventPayload, err := json.Marshal(map[string]any{
		"card_instance_id": row.ID.String(),
		"card_id":          spec.ID,
		"material_id":      materialRef,
	})
	if err != nil {
		return nil, err
	}
	if err := deps.Store.AppendEvent(ctx, EventRow{
		ProjectID: projectID,
		Surface:   "studio",
		Type:      "card_surfaced",
		Payload:   eventPayload,
	}); err != nil {
		return nil, err
	}

	return &Action{Kind: "surface_card", CardInstanceID: row.ID.String(), CardID: spec.ID, MaterialID: materialRef}, nil
}

// CompleteCard runs EvaluateCompletion (card_completion.go) over
// cardInstanceID's live anchors. On complete, it applies spec's
// graph_effects (card_effects.go) — inserting the minted nodes first,
// then resolving each mint edge's "$new:<index>" placeholder against the
// just-inserted nodes' real ids before inserting the edges — and writes
// spec's consolidation payload to framework_fill (R-9: the framework is
// revealed only after completion, never before).
//
// CompleteCard never sets card_instance.status. DEC-3 (product spec §7.1):
// automated judgment may mark work draft/flagged-weak, never solid — a
// final "completed" adjudication needs a passed challenge or an explicit
// confirmation, neither of which this slice builds (Slice 4's gate
// engine). Returns whether completion held; false with a nil error means
// "still active, nothing to apply yet" (missing tags/fields), the same
// silence-is-first-class shape as the rest of the loop.
func CompleteCard(ctx context.Context, deps AgentDeps, spec cards.Spec, cardInstanceID uuid.UUID) (bool, error) {
	row, err := deps.Store.GetCardInstance(ctx, cardInstanceID)
	if err != nil {
		return false, err
	}

	// Idempotency (design §6): a completed card already carries its
	// consolidation framework. Re-running must not mint a second evidence
	// node/edge, so a card whose framework_fill is already set is a no-op.
	if isFrameworkSet(row.FrameworkFill) {
		return true, nil
	}

	var anchors []Anchor
	if len(row.Anchors) > 0 {
		if err := json.Unmarshal(row.Anchors, &anchors); err != nil {
			return false, fmt.Errorf("card: unmarshal anchors for %s: %w", cardInstanceID, err)
		}
	}

	complete, _ := EvaluateCompletion(spec, anchors)
	if !complete {
		return false, nil
	}

	materialID := checkedMaterialID(spec, anchors)
	if materialID == "" && needsMaterial(spec) {
		return false, fmt.Errorf("card: card_instance %s has no material-anchored answers to promote", cardInstanceID)
	}

	nodes, edges := GraphEffects(spec, materialID, anchors)

	// A cross_check's re-tier is only meaningful measured against what she
	// thought the source was BEFORE checking it — read the log's
	// ingestion-time tier now, into the node body, before the mint's own
	// write (below) overwrites that same row with her post-check tier.
	var lateral *LateralRead
	for _, n := range nodes {
		if n.Type != "cross_check" {
			continue
		}
		checkedUUID, err := uuid.Parse(materialID)
		if err != nil {
			return false, fmt.Errorf("card: checked material id %q: %w", materialID, err)
		}
		before, err := deps.Store.GetSourceLogByMaterial(ctx, checkedUUID)
		if err == nil {
			// A missing log entry is not fatal — tier_before is simply absent.
			n.Body["tier_before"] = before.Tier
		}
		tierAfter, _ := n.Body["tier_after"].(string)
		lateral = &LateralRead{MaterialID: checkedUUID, TierAfter: tierAfter}
	}

	framework, err := json.Marshal(ConsolidationPayload(spec))
	if err != nil {
		return false, err
	}
	// One transaction (CommitCardMint, agentstore.go): the framework write is
	// the idempotency guard checked above, and it must land atomically with
	// the nodes/edges it guards — otherwise a failure between the node
	// insert and the guard write leaves a partial mint a retry would
	// duplicate (Task 6). LateralRead (Task 7) rides the same transaction:
	// the graph node the gate reads and the log row the dossier/ledger read
	// must never drift apart.
	if err := deps.Store.CommitCardMint(ctx, row.ProjectID, cardInstanceID, CardMint{
		Nodes: nodes, Edges: edges, Framework: framework, LateralRead: lateral,
	}); err != nil {
		return false, err
	}
	return true, nil
}

// isFrameworkSet reports whether a card_instance's framework_fill jsonb holds
// a real consolidation payload (vs the default empty '{}' / null / unset).
func isFrameworkSet(b []byte) bool {
	s := strings.TrimSpace(string(b))
	return s != "" && s != "{}" && s != "null"
}

// materialConsumingEffects is the closed set of graph_effect kinds whose
// minted edges are ADDRESSED TO the checked material (see GraphEffects,
// card_effects.go): `promote` builds material --evaluated-as--> evidence, and
// `cross_check` builds material --cross-checked-by--> cross_check. The other
// kinds never touch it: `toulmin` mints slot nodes and cites edges addressed
// to per-anchor materials, and `perspectives` mints free-standing project
// nodes with no edges at all.
//
// Keying off effect KIND, not card id, is what keeps this true for cards that
// do not exist yet: a new material-less JSON card needs no change here.
var materialConsumingEffects = map[string]bool{
	"promote":     true,
	"cross_check": true,
}

// needsMaterial reports whether spec declares any graph_effect that would mint
// an edge against the checked material. Only then is a missing material a hard
// failure: a card ABOUT the project rather than a source (sort/scale/matrix —
// fact-opinion-value, certainty-spectrum, perspective-matrix — all write
// material_id "" on every anchor, correctly) has nothing to point an edge at,
// and completing it must proceed with an empty materialID, which GraphEffects
// already ignores for those kinds. Before this guard existed, every COMPLETE
// submit of such a card hard-failed on the server while an INCOMPLETE one
// succeeded — the card broke only once the student did the whole job.
func needsMaterial(spec cards.Spec) bool {
	for _, effect := range spec.GraphEffects {
		if materialConsumingEffects[effect.Kind] {
			return true
		}
	}
	return false
}

// checkedMaterialID reads the material the card is ABOUT — the source under
// review. For a single-material card (no lateral_dimension: every card but a
// compare card) this is the first anchored material, exactly as before. For a
// compare card, whose anchors span two materials by design, the lateral
// anchor is excluded by declaration — never by array order, which is a coin
// flip.
func checkedMaterialID(spec cards.Spec, anchors []Anchor) string {
	for _, a := range anchors {
		if a.MaterialID == "" {
			continue
		}
		if spec.Params.LateralDimension != "" && a.Dimension == spec.Params.LateralDimension {
			continue
		}
		return a.MaterialID
	}
	return ""
}

// lateralAnchor returns the anchor carrying the lateral source (the source
// used to check the card's own source). Only compare cards declare one.
func lateralAnchor(spec cards.Spec, anchors []Anchor) (Anchor, bool) {
	if spec.Params.LateralDimension == "" {
		return Anchor{}, false
	}
	for _, a := range anchors {
		if a.Dimension == spec.Params.LateralDimension && a.MaterialID != "" {
			return a, true
		}
	}
	return Anchor{}, false
}

// resolveMintRef resolves one MintNode/MintEdge endpoint id (card_effects.go):
// either an already-real uuid, or a "$new:<index>" placeholder into the
// node slice GraphEffects returned (resolved against nodeIDs, built by
// CompleteCard after inserting each MintNode in order).
func resolveMintRef(id string, nodeIDs map[int]uuid.UUID) (uuid.UUID, error) {
	if idx, ok := strings.CutPrefix(id, "$new:"); ok {
		n, err := strconv.Atoi(idx)
		if err != nil {
			return uuid.UUID{}, fmt.Errorf("card: malformed mint ref %q: %w", id, err)
		}
		real, ok := nodeIDs[n]
		if !ok {
			return uuid.UUID{}, fmt.Errorf("card: mint ref %q has no matching minted node", id)
		}
		return real, nil
	}
	return uuid.Parse(id)
}

// ErrDispositionReasonTooShort is the typed sentinel for the <15-rune reason
// guard in RecordDisposition. Callers (e.g. the HTTP handler) should use
// errors.Is against this instead of matching the error string, so a future
// reword of the message text can't silently regress a 400 into a 500.
var ErrDispositionReasonTooShort = errors.New("disposition: reason must be at least 15 characters")

// RecordDisposition persists the student's three-key disposition on an
// intervention (product spec §7.1 rule 6): accept / reject / rewrite, with
// a reason. A reason under 15 characters is rejected and nothing is
// persisted — the same "on any enforcement error, persist nothing" rule
// the rest of the loop follows (Slice 2).
func RecordDisposition(ctx context.Context, deps AgentDeps, interventionID uuid.UUID, action, reason string) error {
	// Count characters (runes), not bytes — the product is Chinese-first, where
	// a byte count would let ~5 characters clear a ≥15-character gate.
	if n := utf8.RuneCountInString(strings.TrimSpace(reason)); n < 15 {
		return fmt.Errorf("%w, got %d", ErrDispositionReasonTooShort, n)
	}
	_, err := deps.Store.InsertDisposition(ctx, interventionID, action, reason)
	return err
}
