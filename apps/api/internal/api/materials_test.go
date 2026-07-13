package api_test

// materials_test.go — Task 4: POST /api/v1/projects/{id}/materials, the ONLY
// path that can create a material. The student supplies a URL to fetch or
// pastes text themselves (RL-2: the AI supplies no material, paraphrases no
// unopened source). The material row and its source_log_entry must land in
// one transaction — a material with no log entry is a source that was never
// "opened".

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// The seeded demo project (00000000-0000-0000-0000-000000000101), owned by
// Phoebe/SeedUserID — same fixture projects_test.go and studioturn_test.go use.
const materialsTestProjectID = "00000000-0000-0000-0000-000000000101"

// fakeFetcher is the test double for the api.Fetcher seam. It does a plain
// (unguarded) HTTP GET — fine here because it's test-only code hitting its own
// httptest server — and extracts <title>/<p> text with a couple of regexps.
// This never touches, weakens, or bypasses the production
// materialize.NewFetcher() SSRF guard, which is not used by this test at all.
type fakeFetcher struct{}

var (
	fakeTitleRe = regexp.MustCompile(`(?is)<title>(.*?)</title>`)
	fakeParaRe  = regexp.MustCompile(`(?is)<p>(.*?)</p>`)
)

func (fakeFetcher) FetchReadable(ctx context.Context, rawURL string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	body := string(raw)
	title := ""
	if m := fakeTitleRe.FindStringSubmatch(body); m != nil {
		title = strings.TrimSpace(m[1])
	}
	var paras []string
	for _, m := range fakeParaRe.FindAllStringSubmatch(body, -1) {
		paras = append(paras, strings.TrimSpace(m[1]))
	}
	return title, strings.Join(paras, "\n\n"), nil
}

// countRows returns the row count of table (test-only helper; table is always
// a fixed string literal from this file, never request input).
func countRows(t *testing.T, pool *pgxpool.Pool, table string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM "+table).Scan(&n); err != nil {
		t.Fatalf("countRows(%s): %v", table, err)
	}
	return n
}

func TestIngestMaterialFromURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>Global Greening</title></head><body>
			<p>Leaf area rose about five percent between 2000 and 2017.</p>
			<p>China and India account for a third of the net increase.</p></body></html>`))
	}))
	defer srv.Close()

	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID, Fetcher: fakeFetcher{}}).Handler()
	cookie := signInSeed(t, pool) // Phoebe

	body := fmt.Sprintf(`{"url":%q,"takeaway":"变绿是真的，但不等于可持续。","tier":"一手数据"}`, srv.URL)
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+materialsTestProjectID+"/materials", strings.NewReader(body)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}

	var out struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Origin string `json:"origin"`
		Locked bool   `json:"locked"`
		Role   string `json:"role"`
		Blocks []struct {
			Text string `json:"text"`
		} `json:"blocks"`
		Anchors    []json.RawMessage `json:"anchors"`
		TimeSpentS int32             `json:"timeSpentS"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v — %s", err, rec.Body.String())
	}
	if len(out.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2 (one per paragraph): %s", len(out.Blocks), rec.Body.String())
	}
	if out.Title != "Global Greening" {
		t.Errorf("title = %q, want %q", out.Title, "Global Greening")
	}
	if out.Origin != "fetched" {
		t.Errorf("origin = %q, want fetched", out.Origin)
	}
	if out.Locked {
		t.Errorf("locked = true, want false for a freshly ingested material")
	}
	if out.Role != "" {
		t.Errorf("role = %q, want empty for a freshly ingested material", out.Role)
	}
	if out.Anchors == nil || len(out.Anchors) != 0 {
		t.Errorf("anchors = %v, want empty (never null)", out.Anchors)
	}
	if out.TimeSpentS != 0 {
		t.Errorf("timeSpentS = %d, want 0 for a freshly ingested material (never opened yet)", out.TimeSpentS)
	}

	// The source-log entry landed in the same transaction.
	var takeaway, tier string
	if err := pool.QueryRow(context.Background(),
		`SELECT takeaway, tier FROM source_log_entry WHERE material_id = $1`, out.ID).Scan(&takeaway, &tier); err != nil {
		t.Fatalf("no source_log_entry for the ingested material: %v", err)
	}
	if takeaway != "变绿是真的，但不等于可持续。" || tier != "一手数据" {
		t.Errorf("log entry = %q/%q, want the student's takeaway/tier", takeaway, tier)
	}
}

func TestIngestMaterialFetchFailureWritesNothing(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID, Fetcher: fakeFetcher{}}).Handler()
	cookie := signInSeed(t, pool)

	materialsBefore := countRows(t, pool, "material")
	logBefore := countRows(t, pool, "source_log_entry")

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+materialsTestProjectID+"/materials",
		strings.NewReader(`{"url":"https://example.invalid/nope","takeaway":"x","tier":"y"}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "取不到这个链接的正文") {
		t.Errorf("body = %s, want the honest fetch-failure copy", rec.Body.String())
	}
	if got := countRows(t, pool, "material"); got != materialsBefore {
		t.Errorf("material rows = %d, want %d — a failed fetch must write nothing", got, materialsBefore)
	}
	if got := countRows(t, pool, "source_log_entry"); got != logBefore {
		t.Errorf("source_log_entry rows = %d, want %d — neither row may land without the other", got, logBefore)
	}
}

func TestIngestMaterialFromPastedText(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+materialsTestProjectID+"/materials",
		strings.NewReader(`{"title":"我抄下来的一段","text":"第一段。\n\n第二段。","takeaway":"t","tier":"二手"}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Origin string `json:"origin"`
		Blocks []any  `json:"blocks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v — %s", err, rec.Body.String())
	}
	if out.Origin != "pasted" || len(out.Blocks) != 2 {
		t.Errorf("origin=%q blocks=%d, want pasted/2", out.Origin, len(out.Blocks))
	}
}

func TestIngestMaterialEmptyBodyRejected(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+materialsTestProjectID+"/materials",
		strings.NewReader(`{"title":"空的","text":"   ","takeaway":"t","tier":"二手"}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "正文是空的。") {
		t.Errorf("body = %s, want the honest empty-body copy", rec.Body.String())
	}
}

// TestIngestMaterialPasteBlankTitleRejected — a blank title on the pasted-text
// path must be rejected server-side. The `if title == "" { title = req.URL }`
// fallback is a no-op here (req.URL is "" for a paste), so only the form's
// client-side 标题-required rule was preventing a nameless dossier card; a
// direct API call could still create one before this fix.
func TestIngestMaterialPasteBlankTitleRejected(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+materialsTestProjectID+"/materials",
		strings.NewReader(`{"title":"","text":"第一段。\n\n第二段。","takeaway":"t","tier":"二手"}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "给这条素材起个名字。") {
		t.Errorf("body = %s, want the missing-title copy", rec.Body.String())
	}
}

// materialBlogID is the demo project's seeded blog source-log entry
// (migration 0020): time_spent_s starts at 240, url is the pasted-blog URL.
const materialBlogID = "00000000-0000-0000-0000-000000000110"

func openMaterial(t *testing.T, h http.Handler, cookie *http.Cookie, projectID, mid, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+projectID+"/materials/"+mid+"/open", strings.NewReader(body)), cookie)
	h.ServeHTTP(rec, req)
	return rec
}

// TestLogSourceOpenAccumulatesTime — Task 5. Each open call must ADD to
// time_spent_s, never overwrite it, and must append a source_opened event:
// this is the ledger Slice 10's assessor reads.
func TestLogSourceOpenAccumulatesTime(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)

	for i := 0; i < 2; i++ {
		rec := openMaterial(t, h, cookie, materialsTestProjectID, materialBlogID, `{"time_spent_s":30}`)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204: %s", rec.Code, rec.Body.String())
		}
	}

	var spent int
	if err := pool.QueryRow(context.Background(),
		`SELECT time_spent_s FROM source_log_entry WHERE material_id = $1`, materialBlogID).Scan(&spent); err != nil {
		t.Fatal(err)
	}
	// The seed starts this entry at 240s (migration 0020).
	if spent != 300 {
		t.Errorf("time_spent_s = %d, want 300 (240 seeded + 30 + 30 — it accumulates, never overwrites)", spent)
	}

	rows, err := pool.Query(context.Background(),
		`SELECT surface, payload FROM event WHERE project_id = $1 AND type = 'source_opened' ORDER BY created_at`,
		materialsTestProjectID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var n int
	for rows.Next() {
		n++
		var surface string
		var payload []byte
		if err := rows.Scan(&surface, &payload); err != nil {
			t.Fatal(err)
		}
		if surface != "studio" {
			t.Errorf("event surface = %q, want studio", surface)
		}
		var p struct {
			URL        string `json:"url"`
			TimeSpentS int32  `json:"time_spent_s"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			t.Fatalf("decode payload: %v — %s", err, payload)
		}
		if p.URL != "https://mp.weixin.qq.com/s/demo-china-greening" {
			t.Errorf("payload.url = %q, want the log entry's url", p.URL)
		}
		if p.TimeSpentS != 30 {
			t.Errorf("payload.time_spent_s = %d, want 30 (this open's contribution)", p.TimeSpentS)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("source_opened events = %d, want 2 — this is the ledger Slice 10's assessor reads", n)
	}
}

// TestLogSourceOpenRejectsNegativeTimeSpent — a client error, not a write
// failure: must 400, not 204, and must not touch time_spent_s or the event log.
func TestLogSourceOpenRejectsNegativeTimeSpent(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)

	rec := openMaterial(t, h, cookie, materialsTestProjectID, materialBlogID, `{"time_spent_s":-5}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}

	var spent int
	if err := pool.QueryRow(context.Background(),
		`SELECT time_spent_s FROM source_log_entry WHERE material_id = $1`, materialBlogID).Scan(&spent); err != nil {
		t.Fatal(err)
	}
	if spent != 240 {
		t.Errorf("time_spent_s = %d, want unchanged 240 — a negative sample must never write", spent)
	}
	if events := countRows(t, pool, "event"); events != 0 {
		t.Errorf("event rows = %d, want 0 — a rejected client error appends nothing", events)
	}
}

// TestLogSourceOpenMalformedBodyIs400 — malformed JSON is a client error, not
// a best-effort write failure.
func TestLogSourceOpenMalformedBodyIs400(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)

	rec := openMaterial(t, h, cookie, materialsTestProjectID, materialBlogID, `{not json`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

// TestLogSourceOpenRejectsOtherUsersMaterial — ownership hidden as not-found,
// same as every other project-scoped route (loadOwnedProject).
func TestLogSourceOpenRejectsOtherUsersMaterial(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID}).Handler()
	other := createStudent(t, pool, SeedSchoolID, "material-open-other@demo.local")
	cookie := signInAs(t, pool, other)

	rec := openMaterial(t, h, cookie, materialsTestProjectID, materialBlogID, `{"time_spent_s":30}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (ownership hidden as not-found): %s", rec.Code, rec.Body.String())
	}

	var spent int
	if err := pool.QueryRow(context.Background(),
		`SELECT time_spent_s FROM source_log_entry WHERE material_id = $1`, materialBlogID).Scan(&spent); err != nil {
		t.Fatal(err)
	}
	if spent != 240 {
		t.Errorf("time_spent_s = %d, want unchanged 240 — a non-owner's open must write nothing", spent)
	}
}

func TestIngestMaterialRejectsOtherUsersProject(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID}).Handler()
	other := createStudent(t, pool, SeedSchoolID, "material-other@demo.local")
	cookie := signInAs(t, pool, other)

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+materialsTestProjectID+"/materials",
		strings.NewReader(`{"title":"t","text":"x","takeaway":"t","tier":"二手"}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (ownership hidden as not-found): %s", rec.Code, rec.Body.String())
	}
}
