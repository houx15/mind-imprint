package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
	"net/http"
)

func comparisonSummary(brief []byte) map[string]string {
	var snapshot struct {
		Comparison *pbl.ProcessComparison `json:"processComparison"`
	}
	if json.Unmarshal(brief, &snapshot) != nil || snapshot.Comparison == nil {
		return nil
	}
	return map[string]string{"feedback": snapshot.Comparison.Feedback, "observation": snapshot.Comparison.Observation}
}

// Public callers select a side of the published snapshot, never an arbitrary ID.
// The same resolver serves the owner's preview. It does not recurse into history.
func (a *API) comparisonVersion(r *http.Request, root sqlc.PblCodeVersion) (sqlc.PblCodeVersion, error) {
	side := r.URL.Query().Get("comparison")
	if side == "" {
		return root, nil
	}
	var snapshot struct {
		Comparison *pbl.ProcessComparison `json:"processComparison"`
	}
	if json.Unmarshal(root.Brief, &snapshot) != nil || snapshot.Comparison == nil {
		return root, httpx.ErrNotFound("资源不存在")
	}
	c := snapshot.Comparison
	before, err := uuid.Parse(c.BeforeVersionID)
	if err != nil {
		return root, httpx.ErrNotFound("资源不存在")
	}
	after, err := uuid.Parse(c.AfterVersionID)
	if err != nil {
		return root, httpx.ErrNotFound("资源不存在")
	}
	revised, err := a.d.Queries.GetPblCodeVersion(r.Context(), sqlc.GetPblCodeVersionParams{AtomID: root.AtomID, ID: after})
	if err != nil {
		return root, err
	}
	if !revised.ParentVersionID.Valid || uuid.UUID(revised.ParentVersionID.Bytes) != before {
		return root, httpx.ErrNotFound("资源不存在")
	}
	switch side {
	case "after":
		return revised, nil
	case "before":
		return a.d.Queries.GetPblCodeVersion(r.Context(), sqlc.GetPblCodeVersionParams{AtomID: root.AtomID, ID: before})
	default:
		return root, httpx.ErrNotFound("资源不存在")
	}
}

func publicationRenderKey(id uuid.UUID) string {
	sum := sha256.Sum256([]byte("pbl-publication:" + id.String()))
	return hex.EncodeToString(sum[:])
}
