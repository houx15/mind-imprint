package api_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

// createPblProjectForTest inserts a project atom for the seed user directly
// via sqlc, bypassing the §4 homepage gate the HTTP create endpoint enforces.
// pbl_project has no id column of its own — atom_id is its primary key — so
// the route's {id} and the atom id are the same uuid; both are returned so
// callers can pick whichever name fits the assertion.
func createPblProjectForTest(t *testing.T, pool *pgxpool.Pool) (string, uuid.UUID) {
	t.Helper()
	q := sqlc.New(pool)
	at, err := q.CreateAtom(context.Background(), sqlc.CreateAtomParams{Kind: "project", UserID: SeedUserID})
	if err != nil {
		t.Fatalf("CreateAtom: %v", err)
	}
	if _, err := q.CreatePblProject(context.Background(), sqlc.CreatePblProjectParams{
		AtomID: at.ID, Idea: "心跳测试项目", Kind: "investigation",
	}); err != nil {
		t.Fatalf("CreatePblProject: %v", err)
	}
	return at.ID.String(), at.ID
}

func TestPblHeartbeatAddsSecondsAndBucket(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	projectID, atomID := createPblProjectForTest(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/pbl/projects/"+projectID+"/heartbeat",
		bytes.NewReader([]byte(`{"seconds":60}`))), cookie))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("heartbeat = %d body=%s", rec.Code, rec.Body)
	}
	var total, bucket int32
	if err := pool.QueryRow(context.Background(), `SELECT active_seconds FROM atom WHERE id=$1`, atomID).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT COALESCE(sum(seconds),0) FROM atom_active_day WHERE atom_id=$1`, atomID).Scan(&bucket); err != nil {
		t.Fatal(err)
	}
	if total != 60 || bucket != 60 {
		t.Fatalf("total=%d bucket=%d, want 60/60", total, bucket)
	}
}

func TestPblHeartbeatFinishedProjectIsNoop(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	projectID, atomID := createPblProjectForTest(t, pool)
	if _, err := pool.Exec(context.Background(), `UPDATE pbl_project SET status='review' WHERE atom_id=$1`, atomID); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/pbl/projects/"+projectID+"/heartbeat",
		bytes.NewReader([]byte(`{"seconds":60}`))), cookie))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("heartbeat = %d", rec.Code)
	}
	var total int32
	_ = pool.QueryRow(context.Background(), `SELECT active_seconds FROM atom WHERE id=$1`, atomID).Scan(&total)
	if total != 0 {
		t.Fatalf("finished project accrued %d seconds", total)
	}
}

func TestPblHeartbeatOtherStudentsProject404(t *testing.T) {
	h, _, _, pool := liteHandler(t)
	projectID, _ := createPblProjectForTest(t, pool)
	other := signInAs(t, pool, createStudent(t, pool, SeedSchoolID, "hb-other@demo.local"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/pbl/projects/"+projectID+"/heartbeat",
		bytes.NewReader([]byte(`{"seconds":60}`))), other))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("other student's heartbeat = %d, want 404", rec.Code)
	}
}
