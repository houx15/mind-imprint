package api

// reading_takeaway.go — S2 · assembles the record half of the reading
// takeaway (findings/credibility/key_quotes) from the student's ALREADY
// CONFIRMED reading cards. This is the deterministic counterpart to
// agent.ComposeReadingTakeawaySuggestions (Task 4's isolated compose core,
// which seeds only new_leads/proposal_impact) — record fields are never
// re-guessed by an LLM. Same source notesByMaterial (workspace_library.go)
// projects, but read through framework_fill (the persisted selectionEvalDTO,
// readeval.go) so we get the verdict + judgment, not just quote+finding.

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
)

// readingOutcomesByMaterial assembles the record half of the takeaway from
// the student's CONFIRMED (status="completed") reading cards for this
// material. Attribution to a material lives in each card's anchors JSON, not
// in a card_instances column (card_instances has no material_id — see
// notesByMaterial's comment). Best-effort: a card whose anchors or
// framework_fill don't parse is simply skipped, never fabricated.
func (a *API) readingOutcomesByMaterial(r *http.Request, projectID, materialID uuid.UUID) agent.TakeawayRecord {
	var rec agent.TakeawayRecord
	cis, err := a.d.Queries.ListCardInstancesByProject(r.Context(), pgtype.UUID{Bytes: projectID, Valid: true})
	if err != nil {
		return rec
	}
	for _, ci := range cis {
		if ci.Status != "completed" {
			continue
		}
		// Material attribution lives in the anchors (card_instances has no
		// material_id column) — the same read notesByMaterial uses.
		var anchors []agent.Anchor
		if json.Unmarshal(ci.Anchors, &anchors) != nil || len(anchors) == 0 {
			continue
		}
		if anchors[0].MaterialID != materialID.String() {
			continue
		}
		var ev selectionEvalDTO
		if len(ci.FrameworkFill) > 0 {
			_ = json.Unmarshal(ci.FrameworkFill, &ev)
		}
		if ev.Finding != "" {
			rec.Findings = append(rec.Findings, ev.Finding)
		}
		for _, an := range anchors {
			if an.Author == "student" && an.Quote != "" {
				rec.KeyQuotes = append(rec.KeyQuotes, agent.KeyQuote{Quote: an.Quote, Why: ev.Judgment})
			}
		}
		// First CRAAP/SIFT verdict seen sets credibility.
		if rec.Credibility.Verdict == "" && (ci.CardID == "craap" || ci.CardID == "sift") && ev.VerdictLabel != "" {
			rec.Credibility = agent.Credibility{Verdict: ev.VerdictLabel, Why: ev.VerdictReason}
		}
	}
	return rec
}
