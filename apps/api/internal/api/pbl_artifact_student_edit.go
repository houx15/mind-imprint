package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// Serialize revision creation with both student and model writers. The source
// body is immutable; its ID is the optimistic concurrency token.
func (a *API) createPblArtifactVersion(ctx context.Context, params sqlc.CreatePblArtifactParams, source string) (sqlc.PblArtifact, error) {
	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return sqlc.PblArtifact{}, err
	}
	defer tx.Rollback(ctx)
	q := a.d.Queries.WithTx(tx)
	if _, err = q.LockAtom(ctx, params.AtomID); err != nil {
		return sqlc.PblArtifact{}, err
	}
	if source != "" {
		id, parseErr := uuid.Parse(source)
		if parseErr != nil {
			return sqlc.PblArtifact{}, httpx.ErrBadRequest("bad_source", "原成果无效", nil)
		}
		rows, listErr := q.ListPblArtifacts(ctx, params.AtomID)
		if listErr != nil {
			return sqlc.PblArtifact{}, listErr
		}
		found := false
		for _, row := range rows {
			if row.ID == id && row.Kind == params.Kind {
				found = true
			}
		}
		if !found {
			return sqlc.PblArtifact{}, httpx.ErrNotFound("原成果不存在")
		}
		if supersededArtifactIDs(rows)[id] {
			return sqlc.PblArtifact{}, httpx.ErrConflict("这份成果已有新版，请查看最新版本后修改；当前输入仍保留")
		}
	}
	row, err := q.CreatePblArtifact(ctx, params)
	if err != nil {
		return row, err
	}
	return row, tx.Commit(ctx)
}

// Student-authored text never invokes a model or inherits an approval. Layout
// documents are excluded because their body is derived from structured data.
func (a *API) editPblArtifactText(w http.ResponseWriter, r *http.Request) {
	atomID, aid, ok := a.loadOwnedPblArtifact(w, r)
	if !ok {
		return
	}
	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512<<10)).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	if strings.TrimSpace(req.Body) == "" || utf8.RuneCountInString(req.Body) > 100000 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_body", "正文不能为空，且不能超过100000字", nil))
		return
	}
	base, err := a.d.Queries.GetPblArtifact(r.Context(), aid)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var payload map[string]json.RawMessage
	if err = json.Unmarshal(base.Payload, &payload); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var before string
	_ = json.Unmarshal(payload["body"], &before)
	structured := func(key string) bool { value := payload[key]; return len(value) > 0 && string(value) != "null" }
	if (base.Kind != "draft" && base.Kind != "spec" && base.Kind != "options") || before == "" || structured("paperLayout") || structured("printLayout") {
		httpx.WriteError(w, r, httpx.ErrBadRequest("unsupported_edit", "此成果不支持直接编辑正文", nil))
		return
	}
	if before == req.Body {
		// No revision or approval mutation for an unchanged save.
		rows, err := a.d.Queries.ListPblArtifacts(r.Context(), atomID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		for _, row := range rows {
			if row.ID == aid {
				if supersededArtifactIDs(rows)[aid] {
					httpx.WriteError(w, r, httpx.ErrConflict("这份成果已有新版，请查看最新版本"))
					return
				}
				httpx.WriteJSON(w, http.StatusOK, toPblArtifactDTO(row))
				return
			}
		}
	}
	encoded, _ := json.Marshal(map[string]any{
		"body": req.Body, "replacesArtifactId": aid.String(), "previousBody": before,
		"previousTitle": base.Title, "editedByStudent": true,
	})
	row, err := a.createPblArtifactVersion(r.Context(), sqlc.CreatePblArtifactParams{
		AtomID: atomID, SessionID: base.SessionID, Kind: base.Kind, Title: base.Title,
		Payload: encoded, Guessed: base.Guessed, Admits: base.Admits,
	}, aid.String())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toPblArtifactDTO(row))
}
