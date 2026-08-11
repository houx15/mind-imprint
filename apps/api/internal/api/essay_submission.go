package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// essay_submission.go — slice 4c · the essay SUBMISSION stage's guide-step track
// REST, mirroring essay_statement.go. Steps are fixed (引言 → 结论 → 成文 → 润色);
// the current step's essay-worded guide card is generated + cached (StepGuides,
// keyed by the sub:* step key). 成文/润色 assemble + review the whole draft on the
// writing surface; finishing the paper (完成整篇论文) reuses the existing essay
// finish + status→review advance from the writing room.

// buildEssaySubmissionStep derives the current submission step and (started only)
// generates/reads its cached guide card. Mutates state; reports whether changed.
func (a *API) buildEssaySubmissionStep(ctx context.Context, projectID uuid.UUID, state *agent.StudioState) (proposalGuideStepDTO, bool) {
	et := state.EssayTrack
	steps := agent.DeriveSubmissionSteps()
	total := len(steps)
	changed := false

	idx := agent.ClampStepIndex(et.SubmissionStep, total)
	if idx != et.SubmissionStep {
		et.SubmissionStep = idx
		changed = true
	}
	cur := steps[idx]

	dto := proposalGuideStepDTO{
		Key: cur.Key, Title: cur.Title, Kind: string(cur.Kind),
		Index: idx, Total: total, Mode: "guided", Started: et.SubmissionStarted,
		SubQuestions: []agent.SubQuestion{}, Steps: toStepRefs(steps, et.StepGuides),
	}

	if et.SubmissionStarted {
		if cached := et.StepGuides[cur.Key]; cached != "" {
			var c guideCardDTO
			if json.Unmarshal([]byte(cached), &c) == nil {
				dto.Card = &c
			}
		}
		if dto.Card == nil {
			if c, ok := a.generateEssayGuideCard(ctx, projectID, *state, cur); ok {
				et.StepGuides[cur.Key] = string(mustJSON(c))
				dto.Card = &c
				changed = true
			}
		}
	}
	return dto, changed
}

func (a *API) finishEssaySubmissionWrite(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, state agent.StudioState) {
	dto, _ := a.buildEssaySubmissionStep(r.Context(), projectID, &state)
	if serr := a.saveTrackState(r.Context(), projectID, state); serr != nil {
		httpx.WriteError(w, r, serr)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}

// GET /projects/{id}/essay-submission
func (a *API) getEssaySubmission(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	state, err := a.loadEssayState(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	dto, changed := a.buildEssaySubmissionStep(r.Context(), projectID, &state)
	if changed {
		if serr := a.saveTrackState(r.Context(), projectID, state); serr != nil {
			slog.Warn("essay submission: persist failed", "err", serr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}

// POST /projects/{id}/essay-submission/start
func (a *API) startEssaySubmission(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	state, err := a.loadEssayState(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	state.EssayTrack.SubmissionStarted = true
	state.EssayTrack.SubmissionStep = 0
	a.finishEssaySubmissionWrite(w, r, projectID, state)
}

// POST /projects/{id}/essay-submission/advance {dir}
func (a *API) advanceEssaySubmission(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Dir string `json:"dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "请求格式不对", nil))
		return
	}
	state, err := a.loadEssayState(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	switch body.Dir {
	case "next":
		state.EssayTrack.SubmissionStep++
	case "prev":
		state.EssayTrack.SubmissionStep--
	default:
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "dir 必须是 next 或 prev", nil))
		return
	}
	a.finishEssaySubmissionWrite(w, r, projectID, state)
}
