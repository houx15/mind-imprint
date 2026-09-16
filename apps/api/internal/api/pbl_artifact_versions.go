package api

import (
	"encoding/json"
	"github.com/google/uuid"
	"mindimprint/api/internal/store/sqlc"
)

// Derive replacement state from saved revisions, without changing historical
// review decisions. Unrelated artifacts remain separate review tasks.
func supersededArtifactIDs(rows []sqlc.PblArtifact) map[uuid.UUID]bool {
	byID := make(map[uuid.UUID]sqlc.PblArtifact, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	superseded := make(map[uuid.UUID]bool)
	for _, row := range rows {
		var links struct {
			Base     string `json:"baseArtifactId"`
			Replaces string `json:"replacesArtifactId"`
		}
		if json.Unmarshal(row.Payload, &links) != nil {
			continue
		}
		for _, raw := range []string{links.Base, links.Replaces} {
			id, err := uuid.Parse(raw)
			if err != nil || id == row.ID {
				continue
			}
			source, ok := byID[id]
			if ok && source.AtomID == row.AtomID && source.Kind == row.Kind && row.CreatedAt.After(source.CreatedAt) {
				superseded[id] = true
			}
		}
	}
	return superseded
}
