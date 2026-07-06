package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

func submitBodyWithAnchors(anchors string) []byte {
	return []byte(`{"status":"completed","field_values":{},"event_trace":[{"kind":"submit","at":"1"}],"anchors":` + anchors + `}`)
}

func TestPutCardPersistsAnchors(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	task, _ := q.CreateTask(context.Background(), sqlc.CreateTaskParams{UserID: mustUUID(SeedUserID.String()), Title: "t"})
	card, _ := q.CreateCardInstance(context.Background(), sqlc.CreateCardInstanceParams{CardID: "sift_craap", TaskID: task.ID})
	h := New(Deps{Queries: q, Pool: pool}).Handler()
	cookie := signInSeed(t, pool)
	url := "/api/v1/tasks/" + task.ID.String() + "/cards/" + card.ID.String()

	anchors := `[{"id":"a0","material_id":"m1","block_id":"b0","start":0,"end":5,"quote":"美航局发现","dimension":"权威性","author":"student","question":"我不信这句","answer":"因为没出处"}]`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", url, bytes.NewReader(submitBodyWithAnchors(anchors))), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("submit: %d %s", rec.Code, rec.Body)
	}
	var resp struct {
		Card struct {
			Status  string          `json:"status"`
			Anchors json.RawMessage `json:"anchors"`
		} `json:"card"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Card.Status != "completed" || !bytes.Contains(resp.Card.Anchors, []byte("我不信这句")) {
		t.Fatalf("anchors not persisted in response: %s", rec.Body)
	}
}

func TestPutCardRejectsBadAnchors(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	task, _ := q.CreateTask(context.Background(), sqlc.CreateTaskParams{UserID: mustUUID(SeedUserID.String()), Title: "t"})
	card, _ := q.CreateCardInstance(context.Background(), sqlc.CreateCardInstanceParams{CardID: "sift_craap", TaskID: task.ID})
	h := New(Deps{Queries: q, Pool: pool}).Handler()
	cookie := signInSeed(t, pool)
	url := "/api/v1/tasks/" + task.ID.String() + "/cards/" + card.ID.String()

	// author not in {ai,student}
	bad := `[{"id":"a0","material_id":"m1","block_id":"b0","start":0,"end":5,"quote":"x","dimension":"d","author":"teacher","question":"q","answer":""}]`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", url, bytes.NewReader(submitBodyWithAnchors(bad))), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for bad anchor author, got %d %s", rec.Code, rec.Body)
	}
}
