package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// pbl_projects.go — the 项目 tab's three endpoints.
//
// 🚨 Everything here is named with a Pbl prefix, and the routes live under
// /api/v1/pbl/projects. pro already owns the `project` table, the methods
// a.createProject / a.listProjects / a.renameProject, and the route
// /api/v1/projects — and edition_test.go asserts a LITE student gets 404 on
// that route. Reusing any of those names is how a lite change breaks pro.

// pblProjectDTO is what the 项目 tab reads.
//
// `idea` rides along because the kanban card shows her own opening sentence
// until she has named the project — and after she names it, that sentence is
// still the only record of how she first put it.
type pblProjectDTO struct {
	ID          string `json:"id"`
	Idea        string `json:"idea"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	CoverGround string `json:"coverGround"`
	CoverGlyph  string `json:"coverGlyph"`
	Status      string `json:"status"`
	// 便签板的坐标视图开着没有。
	BoardAxes bool `json:"boardAxes"`
	// Assigned: the project came from an assignment, so idea is the teacher's
	// driving question. The room does not post it as her first turn.
	Assigned       bool   `json:"assigned"`
	CreatedAt      string `json:"createdAt"`
	LastActivityAt string `json:"lastActivityAt"`
	// 卡片上要显示"现在走到哪一步"。没有计划时 currentStep 是空串。
	CurrentStep string `json:"currentStep"`
	StepsDone   int32  `json:"stepsDone"`
	StepsTotal  int32  `json:"stepsTotal"`
	PlanPending bool   `json:"planPending"`
}

// pblProjectStatuses mirrors pbl_project's status CHECK (0108). Validated here
// so a bad status is a 400 with a sentence, rather than a constraint violation
// surfacing as a 500.
var pblProjectStatuses = map[string]bool{
	"talking": true, "running": true, "review": true, "keeping": true, "archived": true,
}

const maxPblIdeaRunes = 4000
const maxPblNameRunes = 60

func (a *API) createPblProject(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		Idea string `json:"idea"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	idea := strings.TrimSpace(req.Idea)
	if idea == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty_idea", "先写一句你想做什么", nil))
		return
	}

	// The idea's rune cap, the empty category and the one transaction live in
	// createPblProjectWithAtomFor.
	at, p, err := a.createPblProjectWithAtomFor(r.Context(), u.ID, idea)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, pblProjectDTO{
		ID: p.AtomID.String(), Idea: p.Idea, Kind: p.Kind, Name: p.Name,
		CoverGround: p.CoverGround, CoverGlyph: p.CoverGlyph, Status: p.Status,
		CreatedAt:      at.CreatedAt.Format(time.RFC3339),
		LastActivityAt: at.CreatedAt.Format(time.RFC3339),
	})
}

func (a *API) listPblProjects(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	rows, err := a.d.Queries.ListPblProjectsByUser(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// A non-nil empty slice: `[]` is an empty board, `null` is a frontend crash.
	out := make([]pblProjectDTO, 0, len(rows))
	homepage, homepageErr := a.d.Queries.GetPblSite(r.Context(), u.ID)
	for _, p := range rows {
		item := pblProjectDTO{
			ID: p.AtomID.String(), Idea: p.Idea, Kind: p.Kind, Name: p.Name,
			CoverGround: p.CoverGround, CoverGlyph: p.CoverGlyph, Status: p.Status,
			BoardAxes:      p.BoardAxes,
			Assigned:       p.Assigned,
			CreatedAt:      p.AtomCreatedAt.Format(time.RFC3339),
			LastActivityAt: p.LastActivityAt.Format(time.RFC3339),
			CurrentStep:    p.CurrentStep,
			StepsDone:      p.StepsDone,
			StepsTotal:     p.StepsTotal,
			PlanPending:    p.PlanPending,
		}
		if homepageErr == nil && homepage.AtomID.Valid && uuid.UUID(homepage.AtomID.Bytes) == p.AtomID {
			if version, err := a.d.Queries.GetPblShownPlan(r.Context(), p.AtomID); err == nil {
				if steps, err := a.d.Queries.ListPblPlanSteps(r.Context(), version.ID); err == nil {
					plan := pblPlanDTO{Steps: make([]pblStepDTO, 0, len(steps))}
					for _, step := range steps {
						plan.Steps = append(plan.Steps, toPblStepDTO(step))
					}
					a.attachHomepageProgress(r, p.AtomID, &plan)
					item.StepsDone, item.StepsTotal, item.CurrentStep = 0, 0, ""
					for _, step := range plan.Steps {
						if step.Status == "cancelled" {
							continue
						}
						item.StepsTotal++
						if step.Progress == "done" || (step.Progress == "" && step.Status == "done") {
							item.StepsDone++
						} else if item.CurrentStep == "" {
							item.CurrentStep = step.Title
						}
					}
				}
			}
		}
		out = append(out, item)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// patchPblProject carries the name-and-cover modal and the kanban's status moves.
func (a *API) patchPblProject(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("项目不存在"))
		return
	}
	row, err := a.d.Queries.GetPblProject(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("项目不存在"))
		return
	}
	// Not 403: a project she does not own should not be distinguishable from
	// one that does not exist.
	if row.UserID != u.ID {
		httpx.WriteError(w, r, httpx.ErrNotFound("项目不存在"))
		return
	}

	var req struct {
		Name        *string `json:"name"`
		CoverGround *string `json:"coverGround"`
		CoverGlyph  *string `json:"coverGlyph"`
		Status      *string `json:"status"`
		// 坐标视图开着没有。见 migration 0122：板上的位置只有在她按坐标摆过
		// 之后才是一句判断。
		BoardAxes *bool `json:"boardAxes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}

	if req.Status != nil {
		s := strings.TrimSpace(*req.Status)
		if !pblProjectStatuses[s] {
			httpx.WriteError(w, r, httpx.ErrBadRequest("bad_status", "不合法状态", nil))
			return
		}
		p, err := a.d.Queries.SetPblProjectStatus(r.Context(),
			sqlc.SetPblProjectStatusParams{AtomID: id, Status: s})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		row.Status = p.Status
		if s == "review" || s == "keeping" {
			// 作业「按时 / 逾期」按这一刻算；只写第一次。
			if err := a.d.Queries.StampPblProjectFinished(r.Context(), id); err != nil {
				httpx.WriteError(w, r, err)
				return
			}
		}
		// 走到复盘就算完成了，可以采兴趣了 —— 和 ListPendingHarvestAtoms 里对
		// 「项目算完成」的判断保持同一套状态。见 interest_jobs.go。
		if s == "review" || s == "keeping" || s == "archived" {
			a.EnqueueHarvest(r.Context(), id)
		}
	}

	if req.BoardAxes != nil {
		p, err := a.d.Queries.SetPblBoardAxes(r.Context(),
			sqlc.SetPblBoardAxesParams{AtomID: id, BoardAxes: *req.BoardAxes})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		row.BoardAxes = p.BoardAxes
	}

	if req.Name != nil || req.CoverGround != nil || req.CoverGlyph != nil {
		name, ground, glyph := row.Name, row.CoverGround, row.CoverGlyph
		if req.Name != nil {
			name = strings.TrimSpace(*req.Name)
			if len([]rune(name)) > maxPblNameRunes {
				name = string([]rune(name)[:maxPblNameRunes])
			}
		}
		if req.CoverGround != nil {
			ground = strings.TrimSpace(*req.CoverGround)
		}
		if req.CoverGlyph != nil {
			glyph = strings.TrimSpace(*req.CoverGlyph)
		}
		p, err := a.d.Queries.UpdatePblProjectMeta(r.Context(),
			sqlc.UpdatePblProjectMetaParams{AtomID: id, Name: name, CoverGround: ground, CoverGlyph: glyph})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		row.Name, row.CoverGround, row.CoverGlyph = p.Name, p.CoverGround, p.CoverGlyph
	}

	httpx.WriteJSON(w, http.StatusOK, pblProjectDTO{
		ID: row.AtomID.String(), Idea: row.Idea, Kind: row.Kind, Name: row.Name,
		CoverGround: row.CoverGround, CoverGlyph: row.CoverGlyph, Status: row.Status,
		BoardAxes:      row.BoardAxes,
		Assigned:       row.Assigned,
		CreatedAt:      row.AtomCreatedAt.Format(time.RFC3339),
		LastActivityAt: row.LastActivityAt.Format(time.RFC3339),
	})
}

// errBadJSON —— 请求体解不开时给的那句话。
//
// 产品负责人 2026-09-02：「尽可能给出详细报错信息，方便 debug」。而且
// apiErrorText 现在会把后端原话原样显示给学生，所以「请求格式不对」这五个字
// 是她和我们同时能看到的**全部**信息——等于什么都没说。解码器自己知道是哪一
// 行、哪个字段坏了，带上它。
func errBadJSON(err error) error { return httpx.ErrBadJSON(err) }
