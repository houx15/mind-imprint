package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

type pblStepSubmissionDTO struct {
	ID          string `json:"id"`
	Note        string `json:"note"`
	URL         string `json:"url"`
	ConfirmedAt string `json:"confirmedAt"`
}

func toPblStepSubmissionDTO(row sqlc.PblStepSubmission) pblStepSubmissionDTO {
	return pblStepSubmissionDTO{
		ID: row.ID.String(), Note: row.Note, URL: row.Url,
		ConfirmedAt: row.ConfirmedAt.Format(time.RFC3339),
	}
}

// submitPblStepDeliverable records what the student says they completed. It is
// evidence of completion, not an AI review verdict. Existing PATCH status paths
// remain valid for historical plans and steps that do not need a deliverable.
func (a *API) submitPblStepDeliverable(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	stepID, err := uuid.Parse(r.PathValue("sid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一步不存在"))
		return
	}
	step, err := a.d.Queries.GetPblPlanStep(r.Context(), stepID)
	if err != nil || step.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一步不存在"))
		return
	}
	live, err := a.d.Queries.GetPblLivePlan(r.Context(), atomID)
	if err != nil || live.ID != step.VersionID {
		httpx.WriteError(w, r, httpx.ErrConflict("请先确认当前计划，再提交这一步的产出"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	var req struct {
		Note      string `json:"note"`
		URL       string `json:"url"`
		Confirmed bool   `json:"confirmed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	note, rawURL := strings.TrimSpace(req.Note), strings.TrimSpace(req.URL)
	if len([]rune(note)) > 8000 || len([]rune(rawURL)) > 2048 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("submission_too_long", "产出说明或链接过长", nil))
		return
	}
	if note == "" && rawURL == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty_submission", "请填写产出说明或链接", nil))
		return
	}
	if !req.Confirmed {
		httpx.WriteError(w, r, httpx.ErrBadRequest("not_confirmed", "请确认这是本步实际完成的产出", nil))
		return
	}
	if rawURL != "" {
		parsed, parseErr := url.ParseRequestURI(rawURL)
		if parseErr != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("bad_submission_url", "请填写完整的 http 或 https 链接", nil))
			return
		}
	}

	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)
	if _, err := qtx.LockAtom(r.Context(), atomID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	lockedStep, err := qtx.GetPblPlanStep(r.Context(), stepID)
	if err != nil || lockedStep.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一步不存在"))
		return
	}
	lockedLive, err := qtx.GetPblLivePlan(r.Context(), atomID)
	if err != nil || lockedLive.ID != lockedStep.VersionID {
		httpx.WriteError(w, r, httpx.ErrConflict("计划已更新，请刷新后提交当前步骤"))
		return
	}
	submission, err := qtx.CreatePblStepSubmission(r.Context(), sqlc.CreatePblStepSubmissionParams{
		StepID: stepID, Note: note, Url: rawURL,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.SetPblStepStatus(r.Context(), sqlc.SetPblStepStatusParams{ID: stepID, Status: "done"}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"submission": toPblStepSubmissionDTO(submission),
		"status":     "done",
	})
}
