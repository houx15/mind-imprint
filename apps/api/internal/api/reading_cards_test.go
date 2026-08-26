package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// seedCard inserts a proposed card directly — the AI turn that would normally
// propose one lands in Task 7.
func seedCard(t *testing.T, pool *pgxpool.Pool, atomID, blockID string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(t.Context(),
		`INSERT INTO atom_card (atom_id, card_id, block_id, status)
		 VALUES ($1, 'craap', $2, 'proposed') RETURNING id::text`,
		mustUUID(atomID), blockID).Scan(&id); err != nil {
		t.Fatalf("seed card: %v", err)
	}
	return id
}

func TestCardSubmit_StoresEnvelopeAndStamps(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	atomID := createReadingAtom(t, h, cookie)
	cardID := seedCard(t, pool, atomID, "b1")

	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"fieldValues":{"currency":"2024"},"eventTrace":[{"t":"open"}]}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+atomID+"/cards/"+cardID+"/submit", body), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("submit = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Status      string `json:"status"`
		SubmittedAt string `json:"submittedAt"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Status != "submitted" || out.SubmittedAt == "" {
		t.Fatalf("card = %+v, want submitted with a timestamp", out)
	}
}

func TestCardSubmit_RejectsMalformedEnvelope(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	atomID := createReadingAtom(t, h, cookie)
	cardID := seedCard(t, pool, atomID, "b1")

	for _, tc := range []struct{ name, body string }{
		{"fieldValues is an array", `{"fieldValues":[],"eventTrace":[]}`},
		{"eventTrace is an object", `{"fieldValues":{},"eventTrace":{}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
				"/api/v1/readings/"+atomID+"/cards/"+cardID+"/submit", strings.NewReader(tc.body)), cookie))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("%s = %d, want 400; body=%s", tc.name, rec.Code, rec.Body)
			}
		})
	}
}

// 铁律④ 过程即数据：跳过是信号，必须留痕，不能删行。
func TestCardSkip_IsRecordedNotDeleted(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	atomID := createReadingAtom(t, h, cookie)
	cardID := seedCard(t, pool, atomID, "b1")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+atomID+"/cards/"+cardID+"/skip", strings.NewReader("{}")), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("skip = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var status string
	if err := pool.QueryRow(t.Context(),
		`SELECT status FROM atom_card WHERE id = $1`, mustUUID(cardID)).Scan(&status); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if status != "skipped" {
		t.Fatalf("status = %q, want \"skipped\" (the row must survive)", status)
	}
}

func TestCardActivate_MovesProposedToActive(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	atomID := createReadingAtom(t, h, cookie)
	cardID := seedCard(t, pool, atomID, "b1")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+atomID+"/cards/"+cardID+"/activate", strings.NewReader("{}")), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("activate = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Status != "active" {
		t.Fatalf("status = %q, want \"active\"", out.Status)
	}
}

// A card belonging to another atom must not be reachable through this one.
func TestCard_CrossAtomIs404(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	atomA := createReadingAtom(t, h, cookie)
	atomB := createReadingAtom(t, h, cookie)
	cardOfB := seedCard(t, pool, atomB, "b1")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+atomA+"/cards/"+cardOfB+"/activate", strings.NewReader("{}")), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-atom card = %d, want 404", rec.Code)
	}
}
