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

	edge := MintEdge{
		Type:     "evaluates",
		FromKind: "card_instance",
		FromID:   row.ID.String(),
		ToKind:   "material",
		ToID:     materialID.String(),
	}
	if err := deps.Store.InsertGraphEdge(ctx, projectID, edge); err != nil {
		return nil, err
	}

	eventPayload, err := json.Marshal(map[string]any{
		"card_instance_id": row.ID.String(),
		"card_id":          spec.ID,
		"material_id":      materialID.String(),
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

	return &Action{Kind: "surface_card", CardInstanceID: row.ID.String(), CardID: spec.ID}, nil
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
	if materialID == "" {
		return false, fmt.Errorf("card: card_instance %s has no material-anchored answers to promote", cardInstanceID)
	}

	nodes, edges := GraphEffects(spec, materialID, anchors)
	nodeIDs := make(map[int]uuid.UUID, len(nodes))
	for i, n := range nodes {
		id, err := deps.Store.InsertGraphNode(ctx, row.ProjectID, n)
		if err != nil {
			return false, err
		}
		nodeIDs[i] = id
	}
	for _, e := range edges {
		fromID, err := resolveMintRef(e.FromID, nodeIDs)
		if err != nil {
			return false, err
		}
		toID, err := resolveMintRef(e.ToID, nodeIDs)
		if err != nil {
			return false, err
		}
		e.FromID, e.ToID = fromID.String(), toID.String()
		if err := deps.Store.InsertGraphEdge(ctx, row.ProjectID, e); err != nil {
			return false, err
		}
	}

	framework, err := json.Marshal(ConsolidationPayload(spec))
	if err != nil {
		return false, err
	}
	if err := deps.Store.SetCardInstanceFramework(ctx, row.ProjectID, cardInstanceID, framework); err != nil {
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
