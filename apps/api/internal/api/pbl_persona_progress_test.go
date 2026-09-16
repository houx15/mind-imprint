package api

import (
	"mindimprint/api/internal/store/sqlc"
	"testing"
)

func TestChosenPersonaProgressRequiresEverySelectedBoard(t *testing.T) {
	ready := sqlc.PblPersona{Chosen: true, Keywords: []byte(`["作品"]`)}
	if chosenPersonasReady(nil) || chosenPersonasReady([]sqlc.PblPersona{{Keywords: ready.Keywords}}) {
		t.Fatal("no selected board must not complete")
	}
	if !chosenPersonasReady([]sqlc.PblPersona{ready, ready, {Chosen: false, Keywords: []byte(`invalid`)}}) {
		t.Fatal("unselected suggestions must not block confirmed readers")
	}
	for _, invalid := range []string{`[]`, `null`, `invalid`, `[" "]`, `["作品"," 作品 "]`, `["1","2","3","4","5","6","7"]`} {
		incomplete := sqlc.PblPersona{Chosen: true, Keywords: []byte(invalid)}
		for _, order := range [][]sqlc.PblPersona{{ready, incomplete}, {incomplete, ready}} {
			if chosenPersonasReady(order) {
				t.Fatalf("incomplete board passed depending on order: %s", invalid)
			}
		}
	}
}
