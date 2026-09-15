package api

// lite_personalized_reading.go — personalised reading homework: the class
// preview, the article a student starts on, and each recipient's article on
// the teacher's homework detail. No model call: reasons come from
// library.StudentPick.Reason.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/library"
	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/store/sqlc"
)

type personalizedPreviewRowDTO struct {
	UserID string `json:"userId"`
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Title  string `json:"title"`
	Tier   int    `json:"tier"`
	// SuggestedTier is her SuggestTier, shown when a pick leaves the tier open.
	SuggestedTier int    `json:"suggestedTier"`
	Reason        string `json:"reason"`
}

// previewLitePersonalizedReading handles
// POST /api/v1/lite/teacher/classes/{id}/personalized-reading/preview.
func (a *API) previewLitePersonalizedReading(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	classID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if _, err := a.assertTeacherOwnsClass(ctx, classID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var req struct {
		Disciplines []string `json:"disciplines"`
		Tier        *int     `json:"tier"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	filter, err := liteassign.ValidateDisciplines(req.Disciplines)
	if err != nil {
		httpx.WriteError(w, r, payloadErrorResponse(err))
		return
	}
	if req.Tier != nil && (*req.Tier < 1 || *req.Tier > 5) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_tier", "难度档位需在 1 到 5 之间", nil))
		return
	}
	students, err := a.d.Queries.ListLiteWeekClassStudents(ctx, classID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	articles := library.All()
	rows := make([]personalizedPreviewRowDTO, 0, len(students))
	for _, s := range students {
		prof, err := libraryProfileIn(ctx, a.d.Queries, s.ID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		suggested := prof.Tier
		if req.Tier != nil {
			prof.Tier = *req.Tier
		}
		pick, ok := library.PickForStudent(articles, prof, filter)
		if !ok {
			continue
		}
		rows = append(rows, personalizedPreviewRowDTO{
			UserID: s.ID.String(), Name: s.DisplayName,
			Slug: pick.Article.Slug, Title: pick.Article.ZhTitle,
			Tier: pick.Tier, SuggestedTier: suggested, Reason: pick.Reason(),
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": rows})
}

// pickedTier is the tier fallback shared by a start and by the teacher
// detail: a pick's own tier, else the homework's class-wide tier, else nil
// (her own level — a start resolves that further to her suggested tier;
// the detail leaves it null).
func pickedTier(pickTier, classTier *int) *int {
	if pickTier != nil {
		return pickTier
	}
	return classTier
}

// personalizedTargetIn turns a personalized payload into the library reading
// this student starts: her pick, or, when she has none (she was added after
// the picks were saved) or her pick's article has since left the library,
// the recommendation computed now. Tier order is the same on both branches —
// the pick's own tier, else the homework's class-wide tier, else (a nil Tier
// here) her suggested tier, left to startAssignedLibraryReading.
func personalizedTargetIn(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, p liteassign.ReadingPayload) (liteassign.ReadingPayload, error) {
	if pick, ok := p.Picks[userID.String()]; ok {
		if _, exists := library.BySlug(pick.Slug); exists {
			return liteassign.ReadingPayload{Source: "library", Slug: pick.Slug, Tier: pickedTier(pick.Tier, p.Tier)}, nil
		}
	}
	prof, err := libraryProfileIn(ctx, q, userID)
	if err != nil {
		return liteassign.ReadingPayload{}, err
	}
	if p.Tier != nil {
		prof.Tier = *p.Tier
	}
	pick, ok := library.PickForStudent(library.All(), prof, p.Disciplines)
	if !ok {
		return liteassign.ReadingPayload{}, errLibraryArticleNotFound
	}
	return liteassign.ReadingPayload{Source: "library", Slug: pick.Article.Slug, Tier: p.Tier}, nil
}

// RecipientReadingDTO is one student's article on a personalized reading
// homework. State: started (her reading's slug and tier), picked (the saved
// pick, tier resolved through pickedTier — nil means her own level, not a
// class-wide or per-pick tier) or pending (no pick yet; the article is
// chosen when she starts).
type RecipientReadingDTO struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
	Tier  *int   `json:"tier"`
	State string `json:"state"`
}

func libraryTitle(slug, fallback string) string {
	if art, ok := library.BySlug(slug); ok {
		return art.ZhTitle
	}
	return fallback
}

// personalizedRecipientReadings maps each recipient to her article. It returns
// nil for any payload that is not a personalized reading.
func (a *API) personalizedRecipientReadings(ctx context.Context, kind string, payload json.RawMessage, rows []sqlc.ListLiteAssignmentRecipientsRow) (map[uuid.UUID]*RecipientReadingDTO, error) {
	if kind != "reading" {
		return nil, nil
	}
	var p liteassign.ReadingPayload
	if err := json.Unmarshal(payload, &p); err != nil || p.Source != "personalized" {
		return nil, nil
	}
	out := make(map[uuid.UUID]*RecipientReadingDTO, len(rows))
	for _, row := range rows {
		if row.AtomID.Valid {
			rd, err := a.d.Queries.GetReading(ctx, uuid.UUID(row.AtomID.Bytes))
			if err != nil {
				return nil, err
			}
			tier := int(rd.LibraryTier)
			out[row.UserID] = &RecipientReadingDTO{Slug: rd.LibrarySlug, Title: libraryTitle(rd.LibrarySlug, rd.Title), Tier: &tier, State: "started"}
			continue
		}
		if pick, ok := p.Picks[row.UserID.String()]; ok {
			out[row.UserID] = &RecipientReadingDTO{Slug: pick.Slug, Title: libraryTitle(pick.Slug, pick.Slug), Tier: pickedTier(pick.Tier, p.Tier), State: "picked"}
			continue
		}
		out[row.UserID] = &RecipientReadingDTO{State: "pending"}
	}
	return out, nil
}
