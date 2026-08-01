package api

import (
	"net/http"
	"time"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/skills"
)

// cardCatalogEntryDTO is one card in the 工具卡图鉴: its identity + gallery
// metadata + a per-caller proficiency snapshot. CoverURL is a short-lived signed
// GET URL for the chosen theme (empty when the card has no cover art or OSS is
// disabled — the web renders a text face instead). name/purpose are echoed for
// convenience but the web's CARD_REGISTRY stays the single source of truth.
type cardCatalogEntryDTO struct {
	CardID   string   `json:"cardId"`
	Name     string   `json:"name"`
	NameEN   string   `json:"nameEn"`
	Category string   `json:"category"`
	Purpose  string   `json:"purpose"`
	Stages   []string `json:"stages"`
	Example  string   `json:"example"`
	HasAsset bool     `json:"hasAsset"`
	CoverURL string   `json:"coverUrl,omitempty"`
	CourseID string   `json:"courseId,omitempty"`

	// proficiency snapshot (descriptive, never a grade — RL-5)
	Encountered bool     `json:"encountered"`
	Score       int      `json:"score"`
	Stars       int      `json:"stars"`
	Uses        int      `json:"uses"`
	Surfaces    []string `json:"surfaces"`
	LastUsed    string   `json:"lastUsed,omitempty"`
}

// getCardsCatalog returns EVERY registered tool card (encountered or not) with
// its gallery metadata, a cover URL for the requested theme, and the caller's
// proficiency. Pure read, no model call — mirrors getGrowthCards. The theme
// comes from ?theme= (falls back to the default colorway when absent/unknown);
// S2 will default it to the student's saved users.card_theme.
func (a *API) getCardsCatalog(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	ctx := r.Context()

	theme := a.resolveCardTheme(ctx, r, u.ID)

	specs, err := cards.Catalog()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Usage summary per card (completed only, all surfaces).
	collected, err := a.d.Queries.ListCollectedCardsByUser(ctx, u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	type usage struct {
		uses     int
		surfaces []string
		lastUsed string
	}
	byCard := make(map[string]usage, len(collected))
	for _, row := range collected {
		surfaces := row.Surfaces
		if surfaces == nil {
			surfaces = []string{}
		}
		lastUsed := ""
		if ts, ok := row.LastUsed.(time.Time); ok {
			lastUsed = ts.Format(time.RFC3339)
		}
		byCard[row.CardID] = usage{uses: int(row.Uses), surfaces: surfaces, lastUsed: lastUsed}
	}

	// Genuine project-scope completions per card (the practice signal).
	projRows, err := a.d.Queries.ListProjectCardCompletionsByUser(ctx, u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	projCompletions := make(map[string]int, len(projRows))
	for _, row := range projRows {
		projCompletions[row.CardID] = int(row.Completions)
	}

	// Which courses the student has finished (the "learned the method" signal).
	finishedIDs, err := a.d.Queries.FinishedCourseIDsByUser(ctx, u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	finishedCourse := make(map[string]struct{}, len(finishedIDs))
	for _, id := range finishedIDs {
		finishedCourse[id.String()] = struct{}{}
	}

	// card_id -> teaching course_id, from the skill specs' cards[] + course_id.
	cardCourse := cardTeachingCourses()

	out := make([]cardCatalogEntryDTO, 0, len(specs))
	for _, s := range specs {
		use := byCard[s.ID]
		courseID := cardCourse[s.ID]

		surfacesHasCourse := false
		for _, sf := range use.surfaces {
			if sf == "course" {
				surfacesHasCourse = true
				break
			}
		}
		_, teachingFinished := finishedCourse[courseID]
		courseLearned := surfacesHasCourse || (courseID != "" && teachingFinished)

		prof := cards.ComputeProficiency(projCompletions[s.ID], use.surfaces, courseLearned)

		coverURL := ""
		hasAsset := false
		if key, ok := cards.CoverKey(s.AssetID, theme); ok {
			hasAsset = true
			if a.d.OSS != nil {
				if url, err := a.d.OSS.SignDownload(key, ossDownloadTTL); err == nil {
					coverURL = url
				}
			}
		}

		surfaces := use.surfaces
		if surfaces == nil {
			surfaces = []string{}
		}
		stages := s.Stage
		if stages == nil {
			stages = []string{}
		}

		out = append(out, cardCatalogEntryDTO{
			CardID:      s.ID,
			Name:        s.Name,
			NameEN:      s.NameEN,
			Category:    s.Category,
			Purpose:     s.Purpose,
			Stages:      stages,
			Example:     s.Example,
			HasAsset:    hasAsset,
			CoverURL:    coverURL,
			CourseID:    courseID,
			Encountered: prof.Encountered,
			Score:       prof.Score,
			Stars:       prof.Stars,
			Uses:        use.uses,
			Surfaces:    surfaces,
			LastUsed:    use.lastUsed,
		})
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{"cards": out, "theme": theme})
}

// cardTeachingCourses maps each card id to the course_id of a skill that teaches
// it (skill.cards[] + skill.course_id). If two courses teach the same card the
// last one wins — acceptable, since the map only powers a "go learn this" link.
func cardTeachingCourses() map[string]string {
	out := map[string]string{}
	catalog, err := skills.Catalog()
	if err != nil {
		return out
	}
	for _, sk := range catalog {
		if sk.CourseID == "" {
			continue
		}
		for _, cardID := range sk.Cards {
			out[cardID] = sk.CourseID
		}
	}
	return out
}
