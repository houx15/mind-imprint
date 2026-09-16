package api

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"mindimprint/api/internal/store/sqlc"
	"testing"
)

func TestToolDTOIncludesOriginSession(t *testing.T) {
	id := uuid.New()
	for _, valid := range []bool{true, false} {
		dto := toPblToolDTO(sqlc.PblToolInstance{SessionID: pgtype.UUID{Bytes: id, Valid: valid}})
		raw, err := json.Marshal(dto)
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]any
		if err := json.Unmarshal(raw, &result); err != nil {
			t.Fatal(err)
		}
		value, exists := result["sessionId"]
		if !exists || valid && value != id.String() || !valid && value != nil {
			t.Fatalf("wrong scope: %s", raw)
		}
	}
}
