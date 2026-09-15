package api

// lite_personalized_reading.go — personalised reading homework: the class
// preview, the article a student starts on, and each recipient's article on
// the teacher's homework detail. No model call: reasons come from
// library.StudentPick.Reason.

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/library"
	"mindimprint/api/internal/liteassign"
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
