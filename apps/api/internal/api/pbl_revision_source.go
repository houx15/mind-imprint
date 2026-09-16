package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"mindimprint/api/internal/pbl"
)

// The submitted completion identifies the source; no latest-title heuristic.
func (a *API) completedPblReviewSource(r *http.Request, atomID uuid.UUID, raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return "", errors.New("完成的工具不存在")
	}
	tool, err := a.d.Queries.GetPblTool(r.Context(), id)
	if err != nil || tool.AtomID != atomID || tool.Status != "done" {
		return "", errors.New("工具尚未完成或不属于当前项目")
	}
	if tool.Tool != "review" {
		return "", nil
	}
	var result struct {
		ArtifactID string `json:"artifactId"`
	}
	if json.Unmarshal(tool.Result, &result) != nil {
		return "", errors.New("审核结果无效")
	}
	artifactID, err := uuid.Parse(result.ArtifactID)
	if err != nil {
		return "", errors.New("审核成果不存在")
	}
	artifact, err := a.d.Queries.GetPblArtifact(r.Context(), artifactID)
	if err != nil || artifact.AtomID != atomID {
		return "", errors.New("审核成果不属于当前项目")
	}
	if artifact.Verdict == nil || (*artifact.Verdict != "revise" && *artifact.Verdict != "dropped") || !artifact.SettledAt.Valid {
		return "", nil
	}
	return artifact.ID.String(), nil
}

func bindPblReviewRevision(produced *pbl.Produced, source string) {
	if source == "" || produced.Kind != "artifact" {
		return
	}
	var payload map[string]json.RawMessage
	if json.Unmarshal(produced.Payload, &payload) != nil {
		return
	}
	// Local edits already carry their explicit source and preserve unchanged text.
	if len(payload["edits"]) > 0 {
		return
	}
	if len(payload["body"]) == 0 && len(payload["printLayout"]) == 0 {
		return
	}
	payload["replacesArtifactId"], _ = json.Marshal(source)
	produced.Payload, _ = json.Marshal(payload)
}
