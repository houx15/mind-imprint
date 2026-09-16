package api

import (
	"encoding/json"
	"github.com/google/uuid"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
	"net/http"
	"strings"
)

func chosenPersonasReady(personas []sqlc.PblPersona) bool {
	chosen := false
	for _, persona := range personas {
		if !persona.Chosen {
			continue
		}
		chosen = true
		var keys []string
		if json.Unmarshal(persona.Keywords, &keys) != nil || len(keys) < 1 || len(keys) > 6 {
			return false
		}
		seen := map[string]bool{}
		for _, key := range keys {
			key = strings.TrimSpace(key)
			if key == "" || len([]rune(key)) > 40 || seen[key] {
				return false
			}
			seen[key] = true
		}
	}
	return chosen
}

// Execution progress is separate from the plan's commitment/status. Derive it
// from saved work rather than marking a whole step done because a tool closed.
func (a *API) attachHomepageProgress(r *http.Request, atom uuid.UUID, plan *pblPlanDTO) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		return
	}
	site, err := a.d.Queries.GetPblSite(r.Context(), u.ID)
	if err != nil || !site.AtomID.Valid || uuid.UUID(site.AtomID.Bytes) != atom {
		return
	}
	tools, err := a.d.Queries.ListPblTools(r.Context(), atom)
	if err != nil {
		return
	}
	personas, err := a.d.Queries.ListPblPersonas(r.Context(), atom)
	if err != nil {
		return
	}
	content, err := a.loadSiteContent(r, u.ID, u.DisplayName, site)
	if err != nil {
		return
	}
	done, started := map[string]bool{}, map[string]bool{}
	for _, t := range tools {
		if t.Status == "accepted" || t.Status == "done" {
			started[t.Tool] = true
		}
		if t.Status == "done" {
			done[t.Tool] = true
		}
	}
	chosen := chosenPersonasReady(personas)
	creativeReady := false
	if row, err := a.d.Queries.GetPblCreativeDirection(r.Context(), atom); err == nil {
		var doc pbl.CreativeDirection
		if json.Unmarshal(row.Document, &doc) == nil {
			_, validationErr := pbl.NormalizeCreativeDirection(doc, true)
			creativeReady = validationErr == nil && doc.Trial != nil
		}
	}
	complete := map[string]bool{
		"creative":  done["creative"] && creativeReady,
		"structure": done["structure"] && len(content.Sections) > 0,
		"persona":   chosen && done["persona"],
		"sites":     done["sites"] && done["structure"] && len(content.Sections) > 0,
		"look":      done["look"] && pbl.ValidPalette(sitePalette(site)),
		"split":     done["split"] && len(pbl.SiteMissing(content)) == 0,
		"review":    site.ShareToken != nil && *site.ShareToken != "",
	}
	routineSteps := pbl.WebsiteRoutine()
	for i, routine := range routineSteps {
		for j := range plan.Steps {
			step := &plan.Steps[j]
			// A renamed/custom step has no verified mapping; retain its own status.
			if step.Title != routine.Title || step.Status == "cancelled" {
				continue
			}
			step.Progress = "todo"
			if complete[routine.Tool] {
				step.Progress = "done"
			} else if started[routine.Tool] || (i > 0 && complete[routineSteps[i-1].Tool]) {
				step.Progress = "doing"
			}
		}
	}
}
