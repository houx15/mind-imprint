package api_test

// workspace_library_test.go — Slice 3 (Read room / Library): the collection
// tree + reference table cheap CRUD, GET /library (collections + references with
// projected notes), and enter-reading (the bridge into the Reading Room that
// fetches+creates a material from a reference's URL). The fakeFetcher seam lives
// in materials_test.go (same package).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// libraryTestHandler builds the API handler with the fakeFetcher wired (so
// enter-reading's URL path works against an httptest server).
func libraryTestHandler(pool *pgxpool.Pool) http.Handler {
	return New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID, Fetcher: fakeFetcher{}}).Handler()
}

// doJSON issues an authed request with an optional JSON body and returns the recorder.
func doJSON(t *testing.T, h http.Handler, cookie *http.Cookie, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *strings.Reader
	if body == "" {
		rdr = strings.NewReader("")
	} else {
		rdr = strings.NewReader(body)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest(method, path, rdr), cookie))
	return rec
}

func TestLibraryCollectionsCRUD(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	// Create a parent collection.
	rec := doJSON(t, h, cookie, "POST", base+"/collections", `{"name":"正方证据"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create collection = %d: %s", rec.Code, rec.Body)
	}
	var created struct {
		Collection struct {
			ID       string  `json:"id"`
			Name     string  `json:"name"`
			ParentID *string `json:"parentId"`
		} `json:"collection"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v — %s", err, rec.Body)
	}
	if created.Collection.Name != "正方证据" || created.Collection.ParentID != nil {
		t.Fatalf("unexpected created collection: %+v", created.Collection)
	}
	parentID := created.Collection.ID

	// Create a child under it.
	rec = doJSON(t, h, cookie, "POST", base+"/collections", fmt.Sprintf(`{"name":"生态","parentId":%q}`, parentID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create child = %d: %s", rec.Code, rec.Body)
	}
	var child struct {
		Collection struct {
			ID       string  `json:"id"`
			ParentID *string `json:"parentId"`
		} `json:"collection"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &child)
	if child.Collection.ParentID == nil || *child.Collection.ParentID != parentID {
		t.Fatalf("child parentId = %v, want %q", child.Collection.ParentID, parentID)
	}

	// Rename the parent + detach any parent (present-null is a no-op here).
	rec = doJSON(t, h, cookie, "PATCH", base+"/collections/"+parentID, `{"name":"正方（改）"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch collection = %d: %s", rec.Code, rec.Body)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Collection.Name != "正方（改）" {
		t.Fatalf("rename failed: %q", created.Collection.Name)
	}

	// Self-parent is rejected.
	rec = doJSON(t, h, cookie, "PATCH", base+"/collections/"+parentID, fmt.Sprintf(`{"parentId":%q}`, parentID))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("self-parent = %d, want 400: %s", rec.Code, rec.Body)
	}

	// Delete the parent — child cascades (parent_id FK ON DELETE CASCADE).
	rec = doJSON(t, h, cookie, "DELETE", base+"/collections/"+parentID, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete collection = %d, want 204: %s", rec.Code, rec.Body)
	}
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM collection WHERE project_id = $1`, pid).Scan(&n); err != nil {
		t.Fatalf("count collections: %v", err)
	}
	if n != 0 {
		t.Fatalf("collections after cascade delete = %d, want 0", n)
	}
}

func TestLibraryReferencesCRUD(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	// A collection to file references under.
	rec := doJSON(t, h, cookie, "POST", base+"/collections", `{"name":"背景"}`)
	var col struct {
		Collection struct {
			ID string `json:"id"`
		} `json:"collection"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &col)

	// Create a blank-ish reference.
	rec = doJSON(t, h, cookie, "POST", base+"/references",
		`{"title":"Global Greening","url":"https://example.com/g","classification":"报告"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create reference = %d: %s", rec.Code, rec.Body)
	}
	var refWrap struct {
		Reference referenceView `json:"reference"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &refWrap); err != nil {
		t.Fatalf("decode: %v — %s", err, rec.Body)
	}
	ref := refWrap.Reference
	if ref.Title != "Global Greening" || ref.Classification != "报告" {
		t.Fatalf("unexpected reference: %+v", ref)
	}
	if ref.Pending {
		t.Fatalf("pending defaulted true, want false")
	}
	if ref.Credibility != nil || ref.Decision != nil {
		t.Fatalf("credibility/decision should be null on create: %+v", ref)
	}
	if ref.Notes == nil {
		t.Fatalf("notes must be [] never null")
	}

	// PATCH a metadata round-trip: author/tags/collectionId/credibility/decision/evaluation.
	patch := fmt.Sprintf(`{"author":"NASA","credentials":"官方","year":"2019","tags":["一手","卫星"],"collectionId":%q,"credibility":"strong","decision":"use","evaluation":"变绿≠更可持续"}`, col.Collection.ID)
	rec = doJSON(t, h, cookie, "PATCH", base+"/references/"+ref.ID, patch)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch reference = %d: %s", rec.Code, rec.Body)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &refWrap)
	got := refWrap.Reference
	if got.Author != "NASA" || got.Year != "2019" || got.Evaluation != "变绿≠更可持续" {
		t.Fatalf("metadata not persisted: %+v", got)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "一手" {
		t.Fatalf("tags round-trip failed: %+v", got.Tags)
	}
	if got.CollectionID == nil || *got.CollectionID != col.Collection.ID {
		t.Fatalf("collectionId = %v, want %q", got.CollectionID, col.Collection.ID)
	}
	if got.Credibility == nil || *got.Credibility != "strong" {
		t.Fatalf("credibility = %v, want strong", got.Credibility)
	}
	if got.Decision == nil || *got.Decision != "use" {
		t.Fatalf("decision = %v, want use", got.Decision)
	}

	// Present-null clears a nullable enum.
	rec = doJSON(t, h, cookie, "PATCH", base+"/references/"+ref.ID, `{"credibility":null}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("clear credibility = %d: %s", rec.Code, rec.Body)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &refWrap)
	if refWrap.Reference.Credibility != nil {
		t.Fatalf("credibility not cleared: %v", refWrap.Reference.Credibility)
	}

	// Invalid enum → 400.
	rec = doJSON(t, h, cookie, "PATCH", base+"/references/"+ref.ID, `{"decision":"burn"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid decision = %d, want 400: %s", rec.Code, rec.Body)
	}

	// Delete.
	rec = doJSON(t, h, cookie, "DELETE", base+"/references/"+ref.ID, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete reference = %d, want 204", rec.Code)
	}
}

func TestGetLibraryReturnsBoth(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	doJSON(t, h, cookie, "POST", base+"/collections", `{"name":"A"}`)
	doJSON(t, h, cookie, "POST", base+"/references", `{"title":"R1"}`)
	doJSON(t, h, cookie, "POST", base+"/references", `{"pending":true,"searchHints":["试试 Google Scholar"]}`)

	rec := doJSON(t, h, cookie, "GET", base+"/library", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get library = %d: %s", rec.Code, rec.Body)
	}
	var lib struct {
		Collections []struct {
			Name string `json:"name"`
		} `json:"collections"`
		References []referenceView `json:"references"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &lib); err != nil {
		t.Fatalf("decode library: %v — %s", err, rec.Body)
	}
	if len(lib.Collections) != 1 || len(lib.References) != 2 {
		t.Fatalf("library = %d collections / %d references, want 1/2", len(lib.Collections), len(lib.References))
	}
	// The pending reference carries its search hints.
	foundHint := false
	for _, r := range lib.References {
		if r.Pending && len(r.SearchHints) == 1 && r.SearchHints[0] == "试试 Google Scholar" {
			foundHint = true
		}
	}
	if !foundHint {
		t.Fatalf("pending reference's searchHints not projected: %+v", lib.References)
	}
}

func TestGetLibraryProjectsReadingNotes(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	q := sqlc.New(pool)
	pid := createProjectForTest(t, h, cookie)
	projectID := uuid.MustParse(pid)
	base := "/api/v1/projects/" + pid

	// A material + a reference bound to it.
	matID := ingestMaterialForTest(t, h, cookie, pid, "绿化报告", craapMaterialText)
	rec := doJSON(t, h, cookie, "POST", base+"/references", `{"title":"绿化报告"}`)
	var refWrap struct {
		Reference referenceView `json:"reference"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &refWrap)
	if _, err := q.SetReferenceMaterial(context.Background(), sqlc.SetReferenceMaterialParams{
		ID:         uuid.MustParse(refWrap.Reference.ID),
		ProjectID:  projectID,
		MaterialID: pgtype.UUID{Bytes: uuid.MustParse(matID), Valid: true},
	}); err != nil {
		t.Fatalf("link reference to material: %v", err)
	}

	// A SUBMITTED reading card whose anchors target that material.
	ci, err := q.CreateProjectCardInstance(context.Background(), sqlc.CreateProjectCardInstanceParams{
		ProjectID: pgtype.UUID{Bytes: projectID, Valid: true}, CardID: "craap", Status: "active",
	})
	if err != nil {
		t.Fatalf("create card instance: %v", err)
	}
	anchors := fmt.Sprintf(`[{"id":"a0","material_id":%q,"quote":"根据 NASA 卫星数据","answer":"溯源到 Chen et al. (2019) 才是一手"}]`, matID)
	if _, err := q.SetCardInstanceAnchors(context.Background(), sqlc.SetCardInstanceAnchorsParams{
		ID: ci.ID, ProjectID: pgtype.UUID{Bytes: projectID, Valid: true}, Anchors: []byte(anchors),
	}); err != nil {
		t.Fatalf("set anchors: %v", err)
	}
	// Not yet completed → no notes.
	rec = doJSON(t, h, cookie, "GET", base+"/library", "")
	var lib struct {
		References []referenceView `json:"references"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &lib)
	if len(lib.References) != 1 || len(lib.References[0].Notes) != 0 {
		t.Fatalf("notes before completion = %v, want none", lib.References)
	}

	// Complete the card → notes project.
	if _, err := q.SetCardInstanceStatus(context.Background(), sqlc.SetCardInstanceStatusParams{
		ID: ci.ID, ProjectID: pgtype.UUID{Bytes: projectID, Valid: true}, Status: "completed",
	}); err != nil {
		t.Fatalf("complete card: %v", err)
	}
	rec = doJSON(t, h, cookie, "GET", base+"/library", "")
	_ = json.Unmarshal(rec.Body.Bytes(), &lib)
	if len(lib.References) != 1 || len(lib.References[0].Notes) != 1 {
		t.Fatalf("notes after completion = %+v, want 1", lib.References)
	}
	note := lib.References[0].Notes[0]
	if note.Quote != "根据 NASA 卫星数据" || !strings.Contains(note.Finding, "Chen et al.") {
		t.Fatalf("note projection wrong: %+v", note)
	}
}

func TestEnterReadingFromURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>Global Greening</title></head><body>
			<p>Leaf area rose about five percent between 2000 and 2017.</p>
			<p>China and India account for a third of the net increase.</p></body></html>`))
	}))
	defer srv.Close()

	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	rec := doJSON(t, h, cookie, "POST", base+"/references", fmt.Sprintf(`{"title":"绿化","url":%q}`, srv.URL))
	var refWrap struct {
		Reference referenceView `json:"reference"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &refWrap)
	rid := refWrap.Reference.ID

	rec = doJSON(t, h, cookie, "POST", base+"/references/"+rid+"/enter-reading", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("enter-reading = %d, want 200: %s", rec.Code, rec.Body)
	}
	// Response is the full MaterialSource DTO with real blocks + zero defaults.
	var ms struct {
		ID          string            `json:"id"`
		Title       string            `json:"title"`
		Origin      string            `json:"origin"`
		Locked      bool              `json:"locked"`
		TimeSpentS  int32             `json:"timeSpentS"`
		LateralRead bool              `json:"lateralRead"`
		Blocks      []json.RawMessage `json:"blocks"`
		Anchors     []json.RawMessage `json:"anchors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ms); err != nil {
		t.Fatalf("decode MaterialSource: %v — %s", err, rec.Body)
	}
	if len(ms.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2: %s", len(ms.Blocks), rec.Body)
	}
	if ms.Title != "Global Greening" || ms.Origin != "fetched" {
		t.Fatalf("title/origin = %q/%q", ms.Title, ms.Origin)
	}
	if ms.Locked || ms.TimeSpentS != 0 || ms.LateralRead {
		t.Fatalf("fresh material should carry zero/false defaults: %+v", ms)
	}
	if ms.Anchors == nil || len(ms.Anchors) != 0 {
		t.Fatalf("anchors = %v, want empty (never null)", ms.Anchors)
	}

	// A material row was created and the reference's material_id was set.
	var linked string
	if err := pool.QueryRow(context.Background(),
		`SELECT material_id FROM reference WHERE id = $1`, rid).Scan(&linked); err != nil {
		t.Fatalf("reference.material_id not set: %v", err)
	}
	if linked != ms.ID {
		t.Fatalf("reference.material_id = %q, want %q (the created material)", linked, ms.ID)
	}
	// A source_log_entry landed alongside (RL-2: never a material without a log entry).
	var logs int
	_ = pool.QueryRow(context.Background(),
		`SELECT count(*) FROM source_log_entry WHERE material_id = $1`, ms.ID).Scan(&logs)
	if logs != 1 {
		t.Fatalf("source_log_entry rows = %d, want 1", logs)
	}
	// A second enter-reading reuses the same material (no re-fetch, no duplicate).
	rec = doJSON(t, h, cookie, "POST", base+"/references/"+rid+"/enter-reading", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("second enter-reading = %d: %s", rec.Code, rec.Body)
	}
	var again struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &again)
	if again.ID != ms.ID {
		t.Fatalf("second enter-reading minted a new material %q, want reuse of %q", again.ID, ms.ID)
	}
}

func TestEnterReadingNoContent422(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	// A reference with no URL and no linked material — nothing to read.
	rec := doJSON(t, h, cookie, "POST", base+"/references", `{"title":"还没找到这篇","pending":true}`)
	var refWrap struct {
		Reference referenceView `json:"reference"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &refWrap)

	rec = doJSON(t, h, cookie, "POST", base+"/references/"+refWrap.Reference.ID+"/enter-reading", "")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("enter-reading no-content = %d, want 422: %s", rec.Code, rec.Body)
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || !strings.Contains(body.Error, "还没有可读内容") {
		t.Fatalf("422 body = %s, want the gentle inline message", rec.Body)
	}
}

func TestLibraryOwnership404(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	owner := signInSeed(t, pool)
	pid := createProjectForTest(t, h, owner)
	base := "/api/v1/projects/" + pid

	// A reference + collection owned by Phoebe.
	rec := doJSON(t, h, owner, "POST", base+"/references", `{"title":"私有"}`)
	var refWrap struct {
		Reference referenceView `json:"reference"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &refWrap)
	rid := refWrap.Reference.ID

	// A different student in the same school.
	otherID := createStudent(t, pool, SeedSchoolID, "library-other@demo.local")
	other := signInAs(t, pool, otherID)

	for _, tc := range []struct {
		method, path string
	}{
		{"GET", base + "/library"},
		{"POST", base + "/collections"},
		{"POST", base + "/references"},
		{"PATCH", base + "/references/" + rid},
		{"DELETE", base + "/references/" + rid},
		{"POST", base + "/references/" + rid + "/enter-reading"},
	} {
		rec := doJSON(t, h, other, tc.method, tc.path, `{"name":"x","title":"x"}`)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s as non-owner = %d, want 404", tc.method, tc.path, rec.Code)
		}
	}
	// And the owner's reference is untouched.
	var n int
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM reference WHERE id = $1`, rid).Scan(&n)
	if n != 1 {
		t.Fatalf("owner reference count = %d, want 1 (non-owner delete must not have hit it)", n)
	}
}

// errFetcher is a Fetcher that always fails — for exercising enter-reading's
// fetch-failure branch (BE1: 422 with a parseable code=fetch_failed envelope).
type errFetcher struct{}

func (errFetcher) FetchReadable(ctx context.Context, rawURL string) (string, string, error) {
	return "", "", fmt.Errorf("simulated fetch failure")
}

// TestEnterReadingFetchFailed422 — a reference with a URL that can't be fetched
// returns 422 with a standard {error:{code:"fetch_failed",message}} envelope so
// the reading-room client can offer its paste-body fallback (BE1).
func TestEnterReadingFetchFailed422(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID, Fetcher: errFetcher{}}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	rec := doJSON(t, h, cookie, "POST", base+"/references", `{"title":"取不到的来源","url":"https://blocked.example/article"}`)
	var refWrap struct {
		Reference referenceView `json:"reference"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &refWrap)
	rid := refWrap.Reference.ID

	rec = doJSON(t, h, cookie, "POST", base+"/references/"+rid+"/enter-reading", "")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("enter-reading (fetch fail) = %d, want 422: %s", rec.Code, rec.Body)
	}
	var perr struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &perr); err != nil {
		t.Fatalf("decode error envelope: %v — %s", err, rec.Body)
	}
	if perr.Error.Code != "fetch_failed" {
		t.Fatalf("error code = %q, want fetch_failed; body=%s", perr.Error.Code, rec.Body)
	}
	if !strings.Contains(perr.Error.Message, "粘") {
		t.Fatalf("error message = %q, want the gentle paste-fallback message", perr.Error.Message)
	}
}

// TestPasteContent — pasting an article body creates a source="pasted" material,
// links it to the reference, and returns the full MaterialSource DTO (BE1).
// Blank text is a 400.
func TestPasteContent(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	// A reference whose URL can't be fetched — the paste fallback's real case.
	rec := doJSON(t, h, cookie, "POST", base+"/references", `{"title":"手动粘贴的文章"}`)
	var refWrap struct {
		Reference referenceView `json:"reference"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &refWrap)
	rid := refWrap.Reference.ID

	// Blank text → 400.
	recBlank := doJSON(t, h, cookie, "POST", base+"/references/"+rid+"/paste-content", `{"text":"   "}`)
	if recBlank.Code != http.StatusBadRequest {
		t.Fatalf("paste blank = %d, want 400: %s", recBlank.Code, recBlank.Body)
	}

	// Real paste → 200 MaterialSource with pasted blocks.
	body := `{"text":"中国的可再生能源投资连续五年全球第一。\n\n但碳排放总量同样位居世界前列，这是绕不开的反例。"}`
	rec = doJSON(t, h, cookie, "POST", base+"/references/"+rid+"/paste-content", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("paste-content = %d, want 200: %s", rec.Code, rec.Body)
	}
	var ms struct {
		ID     string            `json:"id"`
		Title  string            `json:"title"`
		Origin string            `json:"origin"`
		Blocks []json.RawMessage `json:"blocks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ms); err != nil {
		t.Fatalf("decode MaterialSource: %v — %s", err, rec.Body)
	}
	if ms.Origin != "pasted" {
		t.Fatalf("origin = %q, want pasted", ms.Origin)
	}
	if len(ms.Blocks) < 2 {
		t.Fatalf("blocks = %d, want ≥2 (two paragraphs segmented)", len(ms.Blocks))
	}
	if ms.Title != "手动粘贴的文章" {
		t.Fatalf("title = %q, want the reference title", ms.Title)
	}

	// The reference now links the created material, and a source_log_entry landed.
	var linked string
	if err := pool.QueryRow(context.Background(),
		`SELECT material_id FROM reference WHERE id = $1`, rid).Scan(&linked); err != nil {
		t.Fatalf("reference.material_id not set: %v", err)
	}
	if linked != ms.ID {
		t.Fatalf("reference.material_id = %q, want %q", linked, ms.ID)
	}
	var logs int
	_ = pool.QueryRow(context.Background(),
		`SELECT count(*) FROM source_log_entry WHERE material_id = $1`, ms.ID).Scan(&logs)
	if logs != 1 {
		t.Fatalf("source_log_entry rows = %d, want 1 (RL-2)", logs)
	}
}

// referenceView is the test's decode shape for a reference wire DTO.
type referenceView struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Classification string   `json:"classification"`
	Author         string   `json:"author"`
	Credentials    string   `json:"credentials"`
	Year           string   `json:"year"`
	URL            string   `json:"url"`
	Tags           []string `json:"tags"`
	CollectionID   *string  `json:"collectionId"`
	Credibility    *string  `json:"credibility"`
	Evaluation     string   `json:"evaluation"`
	Decision       *string  `json:"decision"`
	Pending        bool     `json:"pending"`
	SearchHints    []string `json:"searchHints"`
	MaterialID     *string  `json:"materialId"`
	Notes          []struct {
		Quote   string `json:"quote"`
		Finding string `json:"finding"`
	} `json:"notes"`
}
