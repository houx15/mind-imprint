package studio

// spotcheck.go — N3f Task 4. Builds the target list a station spot-check
// (S3 evaluate_sources / S4 build_argument) reads, over the SAME ProjectData
// the rest of this package projects from. This lives in `studio`, not `api`,
// because Task 5's projection needs to compute `orderable` by comparing the
// CURRENT fingerprint against the stored one — which means it needs the same
// target list the ordering handler feeds the model. Two independent builders
// would let the projection's `orderable` and the handler's fingerprint drift
// apart (the same "computed twice, differently" failure this codebase has
// hit before — see materialByCardInstance's comment, projection.go:219-223).
// One definition, called from both places.

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// SpotCheckTargets builds what a station's spot-check reads. Deterministic
// order — the fingerprint depends on it, so unchanged work must always
// serialize identically. Returns an empty slice when the station has nothing
// to check (or the station is not one of the two that have a spot-check).
func SpotCheckTargets(d ProjectData, station string) []agent.SpotCheckTarget {
	switch station {
	case agent.SpotCheckSources:
		return sourceSpotCheckTargets(d)
	case agent.SpotCheckArgument:
		return argumentSpotCheckTargets(d)
	default:
		return []agent.SpotCheckTarget{}
	}
}

// evaluatedEvidenceByMaterial maps a material id to the evidence node its
// evaluated-as edge points at — the same graph fact projectMaterials derives
// (projection.go) and attest.go's allArticlesHaveRiskNote derives
// independently (that copy is local to `api` because `api` cannot depend on
// `studio` internals; this one is the `studio`-side copy, built once here so
// SpotCheckTargets stays self-contained).
func evaluatedEvidenceByMaterial(d ProjectData) map[string]sqlc.GraphNode {
	nodesByID := make(map[string]sqlc.GraphNode, len(d.Nodes))
	for _, n := range d.Nodes {
		nodesByID[n.ID.String()] = n
	}
	out := map[string]sqlc.GraphNode{}
	for _, e := range d.Edges {
		if e.Type != "evaluated-as" || e.FromKind != "material" {
			continue
		}
		if n, ok := nodesByID[e.ToID.String()]; ok {
			out[e.FromID.String()] = n
		}
	}
	return out
}

// sourceLogByMaterial maps a material id to its source-log entry.
func sourceLogByMaterial(d ProjectData) map[string]sqlc.SourceLogEntry {
	out := map[string]sqlc.SourceLogEntry{}
	for _, s := range d.SourceLog {
		if s.MaterialID.Valid {
			out[uuid.UUID(s.MaterialID.Bytes).String()] = s
		}
	}
	return out
}

// sourceSpotCheckTargets: one target per kind:"article" material, ordered by
// the material's created_at then id (a stable tiebreak — created_at alone
// can collide within the same test/request). Detail reads tier/takeaway/
// lateral_read from the source log and risk_note via the evaluated-as edge,
// exactly as attest.go's allArticlesHaveRiskNote does — an unevaluated
// article is still a target (体检 covers what she has done AND has not done),
// its risk_note just reads as the explicit "not written" placeholder.
func sourceSpotCheckTargets(d ProjectData) []agent.SpotCheckTarget {
	evidence := evaluatedEvidenceByMaterial(d)
	log := sourceLogByMaterial(d)

	articles := make([]sqlc.Material, 0, len(d.Materials))
	for _, m := range d.Materials {
		if m.Kind == "article" {
			articles = append(articles, m)
		}
	}
	sort.Slice(articles, func(i, j int) bool {
		if !articles[i].CreatedAt.Equal(articles[j].CreatedAt) {
			return articles[i].CreatedAt.Before(articles[j].CreatedAt)
		}
		return articles[i].ID.String() < articles[j].ID.String()
	})

	out := make([]agent.SpotCheckTarget, 0, len(articles))
	for _, m := range articles {
		id := m.ID.String()

		riskNoteText := "「（未写）」"
		if n, ok := evidence[id]; ok {
			if rn := strings.TrimSpace(riskNote(n.Body)); rn != "" {
				riskNoteText = rn
			}
		}

		var tier, takeaway string
		lateral := "未横向核查"
		if s, ok := log[id]; ok {
			takeaway = s.Takeaway
			if s.Tier != nil {
				tier = *s.Tier
			}
			if s.LateralRead {
				lateral = "横向核查过"
			}
		}

		detail := fmt.Sprintf("档位：%s；一句话收获：%s；作用与风险：%s；%s", tier, takeaway, riskNoteText, lateral)
		out = append(out, agent.SpotCheckTarget{ID: id, Name: m.Title, Detail: detail})
	}
	return out
}

// argumentSpotCheckTargets: one target per Toulmin slot, in the card spec's
// slot order, skipping slots with no node text. Mirrors projectStructure's
// own text-extraction rule (projection.go:687-714) exactly — a slot is
// "written" iff a node typed for it carries a non-blank body.text — so this
// list and the 结构 pane can never disagree about what counts as written.
func argumentSpotCheckTargets(d ProjectData) []agent.SpotCheckTarget {
	spec, ok := cards.ByID("toulmin")
	if !ok || len(spec.Params.Slots) == 0 {
		return []agent.SpotCheckTarget{}
	}
	text := map[string]string{}
	for _, n := range d.Nodes {
		if _, seen := text[n.Type]; seen {
			continue
		}
		var b struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(n.Body, &b) == nil && strings.TrimSpace(b.Text) != "" {
			text[n.Type] = b.Text
		}
	}
	out := make([]agent.SpotCheckTarget, 0, len(spec.Params.Slots))
	for _, slot := range spec.Params.Slots {
		t, ok := text[slot.ID]
		if !ok {
			continue
		}
		out = append(out, agent.SpotCheckTarget{ID: slot.ID, Name: slot.Role, Detail: t})
	}
	return out
}
