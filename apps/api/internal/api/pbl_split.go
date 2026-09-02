package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// pbl_split.go —— 分工。
//
// 产品负责人 2026-09-01：「for each task, we may have some substeps, like
// discuss, build, review, etc. so AI can propose a card, where each substep
// assigned AI, or student, or AI and student... and also the reason. and
// students can modify and confirm (modify also need to add the reason)」
//
// 🚨 改一格就要写一句为什么。这不是形式，是整张卡唯一的意义：不要求理由的话，
// 学生只会一路点"同意"，那这张卡就只是在帮 AI 领活——而 AI 多领一件，她就少
// 做一件，少做的那一件正是这个产品声称要教的东西。
//
// 印记提的分工留在 owner/reason 里不动；她改成什么写在 student_owner/
// student_reason。两边都留着，因为"AI 本来想自己做，她拿回去了"是这门课上
// 最值得记下来的事之一。

var pblSubstepOwners = map[string]bool{"yinji": true, "student": true, "both": true}

type pblSubstepDTO struct {
	ID string `json:"id"`
	// 印记提的分工，和它的理由。
	Owner   string `json:"owner"`
	Reason  string `json:"reason"`
	Title   string `json:"title"`
	Ordinal int32  `json:"ordinal"`
	// 她改成了什么，以及为什么。null = 她没动。
	StudentOwner  *string `json:"studentOwner"`
	StudentReason string  `json:"studentReason"`
	Status        string  `json:"status"`
	ConfirmedAt   *string `json:"confirmedAt"`
}

func toPblSubstepDTO(s sqlc.PblSubstep) pblSubstepDTO {
	out := pblSubstepDTO{
		ID: s.ID.String(), Owner: s.Owner, Reason: s.Reason, Title: s.Title,
		Ordinal: s.Ordinal, StudentReason: s.StudentReason, Status: s.Status,
	}
	if s.StudentOwner != nil {
		out.StudentOwner = s.StudentOwner
	}
	if s.ConfirmedAt.Valid {
		t := s.ConfirmedAt.Time.String()
		out.ConfirmedAt = &t
	}
	return out
}

// loadOwnedPblStep —— 这一步属于这个项目的当前计划吗。
func (a *API) loadOwnedPblStep(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return uuid.Nil, false
	}
	sid, err := uuid.Parse(r.PathValue("sid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一步不存在"))
		return uuid.Nil, false
	}
	step, err := a.d.Queries.GetPblPlanStep(r.Context(), sid)
	if err != nil || step.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一步不存在"))
		return uuid.Nil, false
	}
	return sid, true
}

func (a *API) listPblSubsteps(w http.ResponseWriter, r *http.Request) {
	sid, ok := a.loadOwnedPblStep(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListPblSubsteps(r.Context(), sid)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]pblSubstepDTO, 0, len(rows))
	for _, s := range rows {
		out = append(out, toPblSubstepDTO(s))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// proposePblSubsteps —— 印记提一张分工卡。
func (a *API) proposePblSubsteps(w http.ResponseWriter, r *http.Request) {
	sid, ok := a.loadOwnedPblStep(w, r)
	if !ok {
		return
	}
	var req struct {
		Substeps []struct {
			Title  string `json:"title"`
			Owner  string `json:"owner"`
			Reason string `json:"reason"`
		} `json:"substeps"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	if len(req.Substeps) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty", "这张卡上没有东西", nil))
		return
	}
	for i, s := range req.Substeps {
		title := strings.TrimSpace(s.Title)
		if title == "" {
			httpx.WriteError(w, r, httpx.ErrBadRequest("no_title", "这一格是做什么？", nil))
			return
		}
		owner := strings.TrimSpace(s.Owner)
		if !pblSubstepOwners[owner] {
			httpx.WriteError(w, r, httpx.ErrBadRequest("bad_owner", "这一格谁做？", nil))
			return
		}
		// 🚨 印记自己也要说明为什么这一格归它。没有理由的分工，她没法反对——
		// 而她要反对的正是"这件事就该 AI 来做"这个默认。
		if strings.TrimSpace(s.Reason) == "" {
			httpx.WriteError(w, r, httpx.ErrBadRequest("no_reason",
				"每一格都要说清为什么归谁", nil))
			return
		}
		if _, err := a.d.Queries.CreatePblSubstep(r.Context(), sqlc.CreatePblSubstepParams{
			StepID: sid, Ordinal: int32(i), Title: title,
			Owner: owner, Reason: strings.TrimSpace(s.Reason),
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	a.listPblSubsteps(w, r)
}

func (a *API) loadOwnedPblSubstep(w http.ResponseWriter, r *http.Request) (sqlc.GetPblSubstepRow, bool) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return sqlc.GetPblSubstepRow{}, false
	}
	ssid, err := uuid.Parse(r.PathValue("ssid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一格不存在"))
		return sqlc.GetPblSubstepRow{}, false
	}
	row, err := a.d.Queries.GetPblSubstep(r.Context(), ssid)
	if err != nil || row.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一格不存在"))
		return sqlc.GetPblSubstepRow{}, false
	}
	return row, true
}

// reassignPblSubstep —— 她把一格改给别人。
//
// 🚨 这里的 reason 门槛是整张卡的意义所在。允许不写理由地改，等于允许一路点
// "同意"——那这张卡就只是在帮 AI 领活。
func (a *API) reassignPblSubstep(w http.ResponseWriter, r *http.Request) {
	row, ok := a.loadOwnedPblSubstep(w, r)
	if !ok {
		return
	}
	var req struct {
		Owner  string `json:"owner"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	owner := strings.TrimSpace(req.Owner)
	if !pblSubstepOwners[owner] {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_owner", "改给谁？", nil))
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_reason",
			"改了就说一句为什么——这一句是这张卡的意义", nil))
		return
	}
	out, err := a.d.Queries.ReassignPblSubstep(r.Context(), sqlc.ReassignPblSubstepParams{
		ID: row.ID, StudentOwner: &owner, StudentReason: strings.TrimSpace(req.Reason),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPblSubstepDTO(out))
}

// confirmPblSubstep —— 她认下印记提的这一格，不改。
func (a *API) confirmPblSubstep(w http.ResponseWriter, r *http.Request) {
	row, ok := a.loadOwnedPblSubstep(w, r)
	if !ok {
		return
	}
	out, err := a.d.Queries.ConfirmPblSubstep(r.Context(), row.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPblSubstepDTO(out))
}

func (a *API) setPblSubstepStatus(w http.ResponseWriter, r *http.Request) {
	row, ok := a.loadOwnedPblSubstep(w, r)
	if !ok {
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
	switch status {
	case "todo", "doing", "done":
	default:
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_status", "不认识这个状态", nil))
		return
	}
	out, err := a.d.Queries.SetPblSubstepStatus(r.Context(), sqlc.SetPblSubstepStatusParams{
		ID: row.ID, Status: status,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPblSubstepDTO(out))
}
