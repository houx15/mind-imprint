package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/pbl"
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
	ID             string `json:"id"`
	Idea           string `json:"idea"`
	Kind           string `json:"kind"`
	Name           string `json:"name"`
	CoverGround    string `json:"coverGround"`
	CoverGlyph     string `json:"coverGlyph"`
	Status         string `json:"status"`
	CreatedAt      string `json:"createdAt"`
	LastActivityAt string `json:"lastActivityAt"`
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
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	idea := strings.TrimSpace(req.Idea)
	if idea == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty_idea", "先写一句你想做什么", nil))
		return
	}
	if len([]rune(idea)) > maxPblIdeaRunes {
		idea = string([]rune(idea)[:maxPblIdeaRunes])
	}

	// How many she already has decides whether this one is the website project.
	// Counted before the insert so this project is not its own predecessor.
	existing, err := a.d.Queries.CountPblProjectsByUser(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// A first project is her homepage whatever she wrote, so skip the model
	// call entirely — the answer cannot change the outcome, and spending a
	// token to be overruled is waste.
	detected := "website"
	if existing > 0 {
		resolved, ok := a.resolveEval(r.Context())
		if !ok {
			httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
			return
		}
		kind, usage, derr := pbl.DetectKind(r.Context(), a.d.Provider, resolved, idea)
		// Meter BEFORE any bail: a call that yielded nothing still cost money.
		// The project has no atom yet, so this one is recorded against no atom
		// (llm_call.atom_id is nullable — see recordLiteLLMCall).
		a.recordLiteLLMCall(r.Context(), u.ID, uuid.Nil, "pbl_classify", resolved, usage)
		if derr != nil {
			// Surface it. A silent fallback would hang a wrong label on her
			// project and hide a broken classifier behind a plausible answer.
			slog.Warn("pbl: classify failed; surfacing to student",
				"err", derr, "request_id", httpx.RequestIDFromContext(r.Context()))
			httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("classify_failed"))
			return
		}
		detected = kind
	}
	kind := pbl.ResolveKind(existing, detected)

	// atom + pbl_project in ONE transaction: an atom with no project row is an
	// identity nothing can render, exactly as in createReading.
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	at, err := qtx.CreateAtom(r.Context(), sqlc.CreateAtomParams{Kind: "project", UserID: u.ID})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	p, err := qtx.CreatePblProject(r.Context(), sqlc.CreatePblProjectParams{
		AtomID: at.ID, Idea: idea, Kind: kind,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
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
	for _, p := range rows {
		out = append(out, pblProjectDTO{
			ID: p.AtomID.String(), Idea: p.Idea, Kind: p.Kind, Name: p.Name,
			CoverGround: p.CoverGround, CoverGlyph: p.CoverGlyph, Status: p.Status,
			CreatedAt:      p.AtomCreatedAt.Format(time.RFC3339),
			LastActivityAt: p.LastActivityAt.Format(time.RFC3339),
		})
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
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}

	if req.Status != nil {
		s := strings.TrimSpace(*req.Status)
		if !pblProjectStatuses[s] {
			httpx.WriteError(w, r, httpx.ErrBadRequest("bad_status", "不认识这个状态", nil))
			return
		}
		p, err := a.d.Queries.SetPblProjectStatus(r.Context(),
			sqlc.SetPblProjectStatusParams{AtomID: id, Status: s})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		row.Status = p.Status
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
		CreatedAt:      row.AtomCreatedAt.Format(time.RFC3339),
		LastActivityAt: row.LastActivityAt.Format(time.RFC3339),
	})
}
