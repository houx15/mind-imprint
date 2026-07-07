package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

func (a *API) renderCourseStep(w http.ResponseWriter, r *http.Request) {
	c, ok := a.loadCourse(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}
	ord, err := strconv.Atoi(r.PathValue("ordinal"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	step, err := a.d.Queries.GetCourseStepByOrdinal(r.Context(), sqlc.GetCourseStepByOrdinalParams{CourseID: c.ID, Ordinal: int32(ord)})
	if err != nil {
		httpx.WriteError(w, r, err) // ErrNoRows → 404
		return
	}

	// Cache hit → return it.
	if cached, err := a.d.Queries.GetCourseStepRender(r.Context(), step.ID); err == nil {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"rendered": map[string]any{
			"ordinal": step.Ordinal, "kind": step.Kind,
			"template": step.Kind, "content": json.RawMessage(cached.Content), "source": cached.Source,
		}})
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, err)
		return
	}

	// Miss → generate.
	assets := []agent.CourseAsset{}
	_ = json.Unmarshal(step.Assets, &assets) // best-effort; empty on error
	in := agent.CourseStepInput{
		Ordinal: step.Ordinal, Kind: step.Kind, Purpose: step.Purpose,
		Assets: assets, ChallengeType: step.ChallengeType, AuthoredContent: json.RawMessage(step.AuthoredContent),
	}
	rendered := agent.RenderCourseStep(r.Context(), in, a.d.Provider, a.d.EvalResolver)

	// Cache only successful generations (never pin the authored fallback).
	if rendered.Source == "generated" {
		_, _ = a.d.Queries.UpsertCourseStepRender(r.Context(), sqlc.UpsertCourseStepRenderParams{
			CourseStepID: step.ID, Content: []byte(rendered.Content), Source: rendered.Source,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rendered": rendered})
}
