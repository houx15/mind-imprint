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

type fakeFetcher struct {
	title, text string
	err         error
}

func (f fakeFetcher) FetchReadable(_ context.Context, _ string) (string, string, error) {
	return f.title, f.text, f.err
}

func TestMaterialPasteCreateListScratch(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	task, err := q.CreateTask(context.Background(), sqlc.CreateTaskParams{UserID: mustUUID(SeedUserID.String()), Title: "t"})
	if err != nil {
		t.Fatal(err)
	}
	h := New(Deps{Queries: q, Pool: pool, Fetcher: fakeFetcher{}}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/tasks/" + task.ID.String() + "/materials"

	// paste create
	body, _ := json.Marshal(map[string]any{"kind": "draft", "title": "我的初稿", "text": "第一段。\n\n第二段。"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", base, bytes.NewReader(body)), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var created struct {
		Material struct {
			ID     string          `json:"id"`
			Source string          `json:"source"`
			Blocks json.RawMessage `json:"blocks"`
		} `json:"material"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Material.Source != "pasted" || !bytes.Contains(created.Material.Blocks, []byte(`"第一段。"`)) {
		t.Fatalf("unexpected material: %s", rec.Body)
	}

	// list
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", base, nil), cookie))
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte("我的初稿")) {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}

	// scratch
	sbody, _ := json.Marshal(map[string]any{"scratch": "记一笔"})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", base+"/"+created.Material.ID+"/scratch", bytes.NewReader(sbody)), cookie))
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte("记一笔")) {
		t.Fatalf("scratch: %d %s", rec.Code, rec.Body)
	}
}

func TestMaterialPasteValidation(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	task, _ := q.CreateTask(context.Background(), sqlc.CreateTaskParams{UserID: mustUUID(SeedUserID.String()), Title: "t"})
	h := New(Deps{Queries: q, Pool: pool, Fetcher: fakeFetcher{}}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/tasks/" + task.ID.String() + "/materials"
	for _, tc := range []map[string]any{
		{"kind": "pdf", "title": "x", "text": "y"},    // bad kind
		{"kind": "draft", "title": "x", "text": ""},   // empty text
		{"kind": "draft", "title": "x", "text": "  "}, // whitespace-only → no blocks
	} {
		body, _ := json.Marshal(tc)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", base, bytes.NewReader(body)), cookie))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400 for %v, got %d %s", tc, rec.Code, rec.Body)
		}
	}
}

func TestMaterialFromSeedNoSeed(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	task, _ := q.CreateTask(context.Background(), sqlc.CreateTaskParams{UserID: mustUUID(SeedUserID.String()), Title: "t"}) // no seed
	h := New(Deps{Queries: q, Pool: pool, Fetcher: fakeFetcher{}}).Handler()
	cookie := signInSeed(t, pool)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/tasks/"+task.ID.String()+"/materials/from-seed", nil), cookie))
	if rec.Code != http.StatusUnprocessableEntity || !bytes.Contains(rec.Body.Bytes(), []byte("no_seed")) {
		t.Fatalf("want 422 no_seed, got %d %s", rec.Code, rec.Body)
	}
}

func TestMaterialFromSeedSuccess(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	seed := "https://example.com/a"
	task, _ := q.CreateTask(context.Background(), sqlc.CreateTaskParams{UserID: mustUUID(SeedUserID.String()), Title: "t", Seed: &seed})
	h := New(Deps{Queries: q, Pool: pool, Fetcher: fakeFetcher{title: "标题", text: "第一段。\n\n第二段。"}}).Handler()
	cookie := signInSeed(t, pool)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/tasks/"+task.ID.String()+"/materials/from-seed", nil), cookie))
	if rec.Code != http.StatusCreated || !bytes.Contains(rec.Body.Bytes(), []byte(`"fetched"`)) || !bytes.Contains(rec.Body.Bytes(), []byte("第一段。")) {
		t.Fatalf("want 201 fetched material, got %d %s", rec.Code, rec.Body)
	}
}

func TestMaterialListOwnershipHiddenAs404(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	// A task owned by the admin, requested by the seed student → 404.
	task, _ := q.CreateTask(context.Background(), sqlc.CreateTaskParams{UserID: mustUUID(SeedAdminID.String()), Title: "t"})
	h := New(Deps{Queries: q, Pool: pool, Fetcher: fakeFetcher{}}).Handler()
	cookie := signInSeed(t, pool)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/tasks/"+task.ID.String()+"/materials", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d %s", rec.Code, rec.Body)
	}
}
