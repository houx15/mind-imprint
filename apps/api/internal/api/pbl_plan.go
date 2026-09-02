package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

// pbl_plan.go — the living tasklist and Plan Check.
//
// 🚨 The rule this file exists to enforce: a STRUCTURAL change cannot reach the
// live plan. It is staged in pbl_pending_change and moves only when a Plan
// Check records her decision. Everything else here is bookkeeping around that.

type pblStepDTO struct {
	ID        string `json:"id"`
	Ordinal   int    `json:"ordinal"`
	Title     string `json:"title"`
	Blurb     string `json:"blurb"`
	Goal      string `json:"goal"`
	YouBring  string `json:"youBring"`
	IBring    string `json:"iBring"`
	Decide    string `json:"decide"`
	ThenBring string `json:"thenBring"`
	Status    string `json:"status"`
}

type pblPlanDTO struct {
	VersionID  string       `json:"versionId"`
	Version    int          `json:"version"`
	Summary    string       `json:"summary"`
	Reason     string       `json:"reason"`
	ApprovedAt *string      `json:"approvedAt"`
	Steps      []pblStepDTO `json:"steps"`
}

type pblChangeDTO struct {
	ID         string          `json:"id"`
	Kind       string          `json:"kind"`
	Diff       json.RawMessage `json:"diff"`
	Evidence   string          `json:"evidence"`
	Resolution *string         `json:"resolution"`
	Reason     string          `json:"reason"`
	CreatedAt  string          `json:"createdAt"`
}

func toPblStepDTO(s sqlc.PblPlanStep) pblStepDTO {
	return pblStepDTO{
		ID: s.ID.String(), Ordinal: int(s.Ordinal), Title: s.Title, Blurb: s.Blurb,
		Goal: s.Goal, YouBring: s.YouBring, IBring: s.IBring, Decide: s.Decide,
		ThenBring: s.ThenBring, Status: s.Status,
	}
}

// getPblPlan returns the live plan plus any changes waiting for her.
//
// The open changes ride along deliberately: a plan read without them would show
// a settled picture while 印记 is holding a proposal she has not seen.
func (a *API) getPblPlan(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	v, err := a.d.Queries.GetPblLivePlan(r.Context(), atomID)
	if err != nil {
		// No approved plan yet is a normal state, not an error: she has not
		// agreed to anything, so nothing runs.
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"plan": nil, "pending": []pblChangeDTO{}})
		return
	}
	steps, err := a.d.Queries.ListPblPlanSteps(r.Context(), v.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := pblPlanDTO{
		VersionID: v.ID.String(), Version: int(v.Version),
		Summary: v.Summary, Reason: v.Reason,
		Steps: make([]pblStepDTO, 0, len(steps)),
	}
	if v.ApprovedAt.Valid {
		s := v.ApprovedAt.Time.Format(time.RFC3339)
		out.ApprovedAt = &s
	}
	for _, s := range steps {
		out.Steps = append(out.Steps, toPblStepDTO(s))
	}

	pending, err := a.d.Queries.ListPblOpenChanges(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	changes := make([]pblChangeDTO, 0, len(pending))
	for _, c := range pending {
		changes = append(changes, pblChangeDTO{
			ID: c.ID.String(), Kind: c.Kind, Diff: json.RawMessage(c.Diff),
			Evidence: c.Evidence, Reason: c.Reason,
			CreatedAt: c.CreatedAt.Format(time.RFC3339),
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"plan": out, "pending": changes})
}

type pblProposeStepReq struct {
	Title     string `json:"title"`
	Blurb     string `json:"blurb"`
	Goal      string `json:"goal"`
	YouBring  string `json:"youBring"`
	IBring    string `json:"iBring"`
	Decide    string `json:"decide"`
	ThenBring string `json:"thenBring"`
}

// proposePblPlan writes a new, UNAPPROVED version. Nothing runs on it until
// she approves (spec §12.5).
func (a *API) proposePblPlan(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	var req struct {
		Summary string              `json:"summary"`
		Reason  string              `json:"reason"`
		Steps   []pblProposeStepReq `json:"steps"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	if len(req.Steps) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty_plan", "计划至少要有一步", nil))
		return
	}
	// 🚨 Every step names what SHE decides. A step where she decides nothing is
	// a step she should not sit through — refused here so a generated plan gets
	// regenerated rather than quietly shipped hollow.
	for i, s := range req.Steps {
		if strings.TrimSpace(s.Decide) == "" {
			httpx.WriteError(w, r, httpx.ErrBadRequest("step_without_decision",
				"每一步都要说清楚这一步你判断什么", map[string]any{"step": i + 1}))
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

	// Version numbers are allocated read-then-insert, same shape as seq — so
	// the same lock applies.
	if _, err := qtx.LockAtom(r.Context(), atomID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	next, err := qtx.NextPblPlanVersion(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	v, err := qtx.CreatePblPlanVersion(r.Context(), sqlc.CreatePblPlanVersionParams{
		AtomID: atomID, Version: next, Summary: strings.TrimSpace(req.Summary),
		Reason: strings.TrimSpace(req.Reason), DecidedBy: "ai",
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	for i, s := range req.Steps {
		if _, err := qtx.CreatePblPlanStep(r.Context(), sqlc.CreatePblPlanStepParams{
			VersionID: v.ID, Ordinal: int32(i + 1),
			Title: strings.TrimSpace(s.Title), Blurb: s.Blurb, Goal: s.Goal,
			YouBring: s.YouBring, IBring: s.IBring,
			Decide: strings.TrimSpace(s.Decide), ThenBring: s.ThenBring,
			Status: "tentative",
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"versionId": v.ID.String(), "version": int(v.Version),
	})
}

// approvePblPlan — 「准备好了吗 · 开始」. Nothing runs before this.
func (a *API) approvePblPlan(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	var req struct {
		VersionID string `json:"versionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	vid, err := uuid.Parse(strings.TrimSpace(req.VersionID))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一版计划不存在"))
		return
	}
	versions, err := a.d.Queries.ListPblPlanVersions(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	mine := false
	for _, v := range versions {
		if v.ID == vid {
			mine = true
			break
		}
	}
	if !mine {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一版计划不存在"))
		return
	}
	v, err := a.d.Queries.ApprovePblPlanVersion(r.Context(), vid)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("already_approved", "这一版已经开始了", nil))
		return
	}
	// The project leaves 在聊 the moment a plan is running.
	if _, err := a.d.Queries.SetPblProjectStatus(r.Context(), sqlc.SetPblProjectStatusParams{
		AtomID: atomID, Status: "running",
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"version": int(v.Version)})
}

// setPblStepStatus — progress. Applied silently; it never interrupts her.
func (a *API) setPblStepStatus(w http.ResponseWriter, r *http.Request) {
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
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	status := strings.TrimSpace(req.Status)
	if !pbl.IsStepStatus(status) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_status", "不认识这个状态", nil))
		return
	}
	out, err := a.d.Queries.SetPblStepStatus(r.Context(), sqlc.SetPblStepStatusParams{
		ID: stepID, Status: status,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPblStepDTO(out))
}

// stagePblChange — 印记 proposes a structural change.
//
// 🚨 This endpoint has NO path to pbl_plan_step. That absence is the design:
// 隐形重规划 (doc 03 §十四.4) is not discouraged here, it is unreachable.
func (a *API) stagePblChange(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	var req struct {
		Kind     string            `json:"kind"`
		Fields   []pbl.ChangeField `json:"fields"`
		Diff     json.RawMessage   `json:"diff"`
		Evidence string            `json:"evidence"`
		Fork     bool              `json:"keepsBothDirections"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	kind := strings.TrimSpace(req.Kind)
	switch kind {
	case "add", "modify", "remove", "defer":
	default:
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_change_kind", "不认识这种改动", nil))
		return
	}
	// A proposal with no evidence is a proposal she cannot judge — every change
	// has to connect to a specific finding, decision or constraint.
	if strings.TrimSpace(req.Evidence) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_evidence",
			"说清楚这个改动是因为什么", nil))
		return
	}
	grade := pbl.GradeChange(pbl.ProposedChange{Fields: req.Fields, KeepsBothDirections: req.Fork})
	if !pbl.NeedsPlanCheck(grade) {
		// Progress and local changes do not belong here; they are applied
		// directly (step status, or a local edit). Saying so is better than
		// silently staging something that will sit unresolved forever.
		httpx.WriteError(w, r, httpx.ErrBadRequest("not_structural",
			"这个改动不用等她拍板，直接改就行", map[string]any{"grade": string(grade)}))
		return
	}
	if len(req.Diff) == 0 {
		req.Diff = json.RawMessage(`{}`)
	}
	c, err := a.d.Queries.StagePblPendingChange(r.Context(), sqlc.StagePblPendingChangeParams{
		AtomID: atomID, Kind: kind, Diff: req.Diff, Evidence: strings.TrimSpace(req.Evidence),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, pblChangeDTO{
		ID: c.ID.String(), Kind: c.Kind, Diff: json.RawMessage(c.Diff),
		Evidence: c.Evidence, CreatedAt: c.CreatedAt.Format(time.RFC3339),
	})
}

// resolvePblChange — Plan Check. Her decision, recorded.
func (a *API) resolvePblChange(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这条改动不存在"))
		return
	}
	c, err := a.d.Queries.GetPblPendingChange(r.Context(), cid)
	if err != nil || c.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这条改动不存在"))
		return
	}
	var req struct {
		Resolution string `json:"resolution"`
		Reason     string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	res := strings.TrimSpace(req.Resolution)
	switch res {
	// 「kept」 is a first-class outcome: she looked and decided to stay. It is
	// recorded with her reason, not treated as a failure to engage.
	case "accepted", "edited", "kept", "forked", "deferred":
	default:
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_resolution", "不认识这个决定", nil))
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_reason",
			"写一句为什么——这是你的判断，不是一次点击", nil))
		return
	}

	out, err := a.d.Queries.ResolvePblPendingChange(r.Context(), sqlc.ResolvePblPendingChangeParams{
		ID: cid, Resolution: &res, Reason: strings.TrimSpace(req.Reason),
	})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("already_resolved", "这条已经定过了", nil))
		return
	}
	// Her decision is part of the process record, whichever way it went.
	if _, err := a.d.Queries.RecordPblDecision(r.Context(), sqlc.RecordPblDecisionParams{
		AtomID: atomID, SessionID: pgtype.UUID{},
		Subject: "plan_change:" + cid.String(), Choice: res,
		Why: strings.TrimSpace(req.Reason),
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	resolution := res
	httpx.WriteJSON(w, http.StatusOK, pblChangeDTO{
		ID: out.ID.String(), Kind: out.Kind, Diff: json.RawMessage(out.Diff),
		Evidence: out.Evidence, Resolution: &resolution, Reason: out.Reason,
		CreatedAt: out.CreatedAt.Format(time.RFC3339),
	})
}
