package api

// lite_teacher_library.go — the class-wide reading recommendation:
//
//	GET /api/v1/lite/teacher/classes/{id}/library/recommended?limit=8
//
// Every enrolled student's own library.Profile (interests + read history +
// suggested tier) is folded into one class-wide profile and scored through
// library.RecommendForGroup. No model call: the ranking is pure computation
// over inputs the library package already knows how to build for one
// student. The workspace tool recommend_articles (lite_teacher_workspace.go)
// reads the same class profiles through classLibraryProfiles below.

import (
	"context"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/library"
	"mindimprint/api/internal/store/sqlc"
)

// liteTeacherLibraryRecommendDefault is ?limit= when the query omits it —
// same count as the student shelf's own recommendation strip.
const liteTeacherLibraryRecommendDefault = libraryRecommendCount * 2

// libraryGroupRecommendationDTO is one article recommended to the class: the
// same shape GET /library gives one article, plus why it was picked and how
// many of the class have already read it.
type libraryGroupRecommendationDTO struct {
	libraryArticleDTO
	Why       []string `json:"why"`
	ReadCount int      `json:"readCount"`
}

type libraryGroupRecommendedDTO struct {
	Articles []libraryGroupRecommendationDTO `json:"articles"`
	// Tier is the class's shared difficulty — see library.GroupTier.
	Tier int `json:"tier"`
}

// classLibraryProfiles loads every enrolled student's library.Profile: the
// input a class-wide recommendation aggregates. Same loop
// previewLitePersonalizedReading already runs over the class roster
// (lite_personalized_reading.go) — no new query.
func classLibraryProfiles(ctx context.Context, q *sqlc.Queries, classID uuid.UUID) ([]library.Profile, error) {
	students, err := q.ListLiteWeekClassStudents(ctx, classID)
	if err != nil {
		return nil, err
	}
	profiles := make([]library.Profile, 0, len(students))
	for _, s := range students {
		prof, err := libraryProfileIn(ctx, q, s.ID)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, prof)
	}
	return profiles, nil
}

// getLiteTeacherLibraryRecommended handles
// GET /api/v1/lite/teacher/classes/{id}/library/recommended?limit=8.
func (a *API) getLiteTeacherLibraryRecommended(w http.ResponseWriter, r *http.Request) {
	cls, ok := a.authTeacherClass(w, r)
	if !ok {
		return
	}
	limit := liteTeacherLibraryRecommendDefault
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			httpx.WriteError(w, r, httpx.ErrBadRequest("bad_limit", "limit 必须是正整数", nil))
			return
		}
		limit = n
	}

	ctx := r.Context()
	members, err := classLibraryProfiles(ctx, a.d.Queries, cls.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	recs := library.RecommendForGroup(library.All(), members, limit)
	out := libraryGroupRecommendedDTO{
		Articles: make([]libraryGroupRecommendationDTO, 0, len(recs)),
		Tier:     library.GroupTier(members),
	}
	for _, rec := range recs {
		out.Articles = append(out.Articles, libraryGroupRecommendationDTO{
			libraryArticleDTO: a.libraryArticleDTOFor(rec.Article),
			Why:               libraryWhyZh(rec.Why),
			ReadCount:         rec.ReadCount,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}
