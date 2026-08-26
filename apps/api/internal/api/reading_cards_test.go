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
	// Retire whatever this reading already has open first. Since 0096 a
	// reading can hold at most ONE proposed/active card
	// (atom_card_one_open_idx), so a helper whose contract is "hand me an open
	// card to act on" has to make room for it. Callers that seed a fresh card
	// per subtest (see TestCardSubmit_RejectsMalformedEnvelope, which does so
	// deliberately) would otherwise trip the constraint instead of exercising
	// what they came to test.
	if _, err := pool.Exec(t.Context(),
		`UPDATE atom_card SET status = 'skipped'
		 WHERE atom_id = $1 AND status IN ('proposed','active')`,
		mustUUID(atomID)); err != nil {
		t.Fatalf("retire open cards: %v", err)
	}
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

	for _, tc := range []struct{ name, body string }{
		{"fieldValues is an array", `{"fieldValues":[],"eventTrace":[]}`},
		{"eventTrace is an object", `{"fieldValues":{},"eventTrace":{}}`},
		{"fieldValues is null", `{"fieldValues":null,"eventTrace":[]}`},
		{"eventTrace is null", `{"fieldValues":{},"eventTrace":null}`},
		{"fieldValues is missing", `{"eventTrace":[]}`},
		{"eventTrace is missing", `{"fieldValues":{}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A fresh card per subtest: if a rejection ever regressed into a
			// successful submit, reusing one card across subtests would make
			// every later subtest fail on the 409 terminal guard instead of
			// exercising validation — the wrong reason entirely.
			cardID := seedCard(t, pool, atomID, "b1")
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

// Terminal states ('submitted', 'skipped') accept no further transition —
// a re-activate, re-skip, or submit-after-skip must not silently overwrite
// evidence that is already on record. 409, not a silent no-op or 200.

func TestCardActivate_AfterSubmitIs409(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	atomID := createReadingAtom(t, h, cookie)
	cardID := seedCard(t, pool, atomID, "b1")

	submitRec := httptest.NewRecorder()
	h.ServeHTTP(submitRec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+atomID+"/cards/"+cardID+"/submit",
		strings.NewReader(`{"fieldValues":{},"eventTrace":[]}`)), cookie))
	if submitRec.Code != http.StatusOK {
		t.Fatalf("submit setup = %d, want 200; body=%s", submitRec.Code, submitRec.Body)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+atomID+"/cards/"+cardID+"/activate", strings.NewReader("{}")), cookie))
	if rec.Code != http.StatusConflict {
		t.Fatalf("activate-after-submit = %d, want 409; body=%s", rec.Code, rec.Body)
	}
}

func TestCardSkip_AfterSkipIs409(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	atomID := createReadingAtom(t, h, cookie)
	cardID := seedCard(t, pool, atomID, "b1")

	firstSkip := httptest.NewRecorder()
	h.ServeHTTP(firstSkip, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+atomID+"/cards/"+cardID+"/skip", strings.NewReader("{}")), cookie))
	if firstSkip.Code != http.StatusOK {
		t.Fatalf("skip setup = %d, want 200; body=%s", firstSkip.Code, firstSkip.Body)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+atomID+"/cards/"+cardID+"/skip", strings.NewReader("{}")), cookie))
	if rec.Code != http.StatusConflict {
		t.Fatalf("skip-after-skip = %d, want 409; body=%s", rec.Code, rec.Body)
	}
}

func TestCardSubmit_AfterSkipIs409(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	atomID := createReadingAtom(t, h, cookie)
	cardID := seedCard(t, pool, atomID, "b1")

	skipRec := httptest.NewRecorder()
	h.ServeHTTP(skipRec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+atomID+"/cards/"+cardID+"/skip", strings.NewReader("{}")), cookie))
	if skipRec.Code != http.StatusOK {
		t.Fatalf("skip setup = %d, want 200; body=%s", skipRec.Code, skipRec.Body)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+atomID+"/cards/"+cardID+"/submit",
		strings.NewReader(`{"fieldValues":{},"eventTrace":[]}`)), cookie))
	if rec.Code != http.StatusConflict {
		t.Fatalf("submit-after-skip = %d, want 409; body=%s", rec.Code, rec.Body)
	}
}
