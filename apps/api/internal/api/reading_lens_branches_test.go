package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// reading_lens_branches_test.go — the three write-path decisions the lens/card
// slice added without an up-front brief. Each one is a refusal, and a refusal
// that is never exercised is a refusal you only find out about in production.

func postEvaluate(t *testing.T, h http.Handler, cookie *http.Cookie, atomID, cardID, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+atomID+"/cards/"+cardID+"/evaluate", strings.NewReader(body)), cookie))
	return rec
}

func postSubmit(t *testing.T, h http.Handler, cookie *http.Cookie, atomID, cardID, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+atomID+"/cards/"+cardID+"/submit", strings.NewReader(body)), cookie))
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var out struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode error envelope: %v — body=%s", err, rec.Body)
	}
	return out.Error.Code
}

// TestCardSubmit_RejectsAnchorsThatAreNotAnArray — including the literal JSON
// `null`, which is the case that actually bites: unmarshalling `null` into a
// slice succeeds with no error and simply leaves it nil, so an error check
// alone waves it through and it lands in the column verbatim. A stored `null`
// then survives every Go boundary check and blows up later against a strict
// Zod `z.array` in the browser — the exact failure that blanked the teacher
// report once already, and the same subtlety Task 6 had to fix in
// validateEnvelope.
func TestCardSubmit_RejectsAnchorsThatAreNotAnArray(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	atomID := createReadingAtom(t, h, cookie)

	for _, tc := range []struct{ name, anchors string }{
		{"literal null", `null`},
		{"an object", `{"block_id":"b1"}`},
		{"a string", `"b1"`},
		{"a number", `3`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cardID := seedCard(t, pool, atomID, "b1")
			rec := postSubmit(t, h, cookie, atomID, cardID,
				`{"fieldValues":{},"eventTrace":[],"anchors":`+tc.anchors+`}`)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("anchors=%s → %d, want 400; body=%s", tc.anchors, rec.Code, rec.Body)
			}
			if code := errorCode(t, rec); code != "invalid_anchors" {
				t.Fatalf("anchors=%s → code %q, want invalid_anchors", tc.anchors, code)
			}
			// The refusal must be total: nothing was written, so the card is
			// still open and she can retry.
			var status string
			if err := pool.QueryRow(t.Context(),
				`SELECT status FROM atom_card WHERE id = $1`, mustUUID(cardID)).Scan(&status); err != nil {
				t.Fatalf("read card: %v", err)
			}
			if status != "proposed" {
				t.Fatalf("card status = %q after a rejected submit, want it untouched", status)
			}
		})
	}
}

// TestCardSubmit_AcceptsAnEmptyAnchorArray — the other side of the same
// branch: `[]` is a legitimate array (a card submitted with no pick), and must
// NOT be swept up by the null rejection.
func TestCardSubmit_AcceptsAnEmptyAnchorArray(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	atomID := createReadingAtom(t, h, cookie)
	cardID := seedCard(t, pool, atomID, "b1")

	rec := postSubmit(t, h, cookie, atomID, cardID, `{"fieldValues":{},"eventTrace":[],"anchors":[]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("empty anchors = %d, want 200; body=%s", rec.Code, rec.Body)
	}
}

// TestEvaluateSelection_TerminalCardIs409 — a card that is already submitted
// or skipped is finished. Re-reviewing a pick on it would spend a flagship
// call to write a review onto a row nothing will ever read, and the 409 fires
// BEFORE the model call for exactly that reason.
func TestEvaluateSelection_TerminalCardIs409(t *testing.T) {
	for _, status := range []string{"submitted", "skipped"} {
		t.Run(status, func(t *testing.T) {
			h, cookie, _, pool := liteHandlerWithProvider(t, routerStubProvider(evalScript))
			atomID := createReadingAtom(t, h, cookie)
			if rec := putSource(t, h, cookie, atomID, turnArticle); rec.Code != http.StatusOK {
				t.Fatalf("put source = %d; body=%s", rec.Code, rec.Body)
			}
			cardID := seedCard(t, pool, atomID, "b1")
			if _, err := pool.Exec(t.Context(),
				`UPDATE atom_card SET status = $2 WHERE id = $1`, mustUUID(cardID), status); err != nil {
				t.Fatalf("retire card: %v", err)
			}

			rec := postEvaluate(t, h, cookie, atomID, cardID,
				`{"block_id":"b1","start":0,"end":5,"quote":"中国的太阳能装机量在过去十年增长了十倍。","dimension":"craap"}`)
			if rec.Code != http.StatusConflict {
				t.Fatalf("evaluate on a %s card = %d, want 409; body=%s", status, rec.Code, rec.Body)
			}
			// No model call was made and no review was written.
			var n int
			if err := pool.QueryRow(t.Context(),
				`SELECT count(*) FROM llm_call WHERE atom_id = $1`, mustUUID(atomID)).Scan(&n); err != nil {
				t.Fatalf("count llm_call: %v", err)
			}
			if n != 0 {
				t.Fatalf("llm_call rows = %d — a refused evaluate must not spend a flagship call", n)
			}
			var framework string
			if err := pool.QueryRow(t.Context(),
				`SELECT framework_fill::text FROM atom_card WHERE id = $1`, mustUUID(cardID)).Scan(&framework); err != nil {
				t.Fatalf("read card: %v", err)
			}
			if framework != "{}" {
				t.Fatalf("framework_fill = %s, want it untouched", framework)
			}
		})
	}
}

// TestSummonCard_RefusesCraapOnAnArticleAlreadyChecked — the other half of the
// ordering guard. Only the sift-before-craap direction was covered; this is
// the "don't send her round the same loop twice" direction, and it is the one
// a student actually hits (the 透镜库 lists CRAAP first, so re-picking it after
// finishing it is the easy mistake).
func TestSummonCard_RefusesCraapOnAnArticleAlreadyChecked(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, routerStubProvider(exampleScript))
	atomID := createReadingAtom(t, h, cookie)
	if rec := putSource(t, h, cookie, atomID, turnArticle); rec.Code != http.StatusOK {
		t.Fatalf("put source = %d; body=%s", rec.Code, rec.Body)
	}
	// CRAAP already done on this article, and nothing currently open — so the
	// one-active mutex is NOT what answers here; the ordering guard is.
	seedCardWith(t, pool, atomID, "craap", "b1", "submitted")

	rec := postSummon(t, h, cookie, atomID, "craap")
	if rec.Code != http.StatusOK {
		t.Fatalf("repeat craap = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeLensTurn(t, rec)
	if out.Card != nil {
		t.Fatalf("repeat craap minted a card: %+v", out.Card)
	}
	if !strings.Contains(out.Reply, "已经做过信源体检") {
		t.Fatalf("repeat craap reply = %q, want the already-checked decline", out.Reply)
	}
	// And it must not have spent a grounding call to say so.
	var n int
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM llm_call WHERE atom_id = $1`, mustUUID(atomID)).Scan(&n); err != nil {
		t.Fatalf("count llm_call: %v", err)
	}
	if n != 0 {
		t.Fatalf("llm_call rows = %d — a declined summon must not call a model", n)
	}

	// SIFT, however, is now unlocked — the decline is about ordering, not a
	// dead end.
	rec2 := postSummon(t, h, cookie, atomID, "sift")
	if rec2.Code != http.StatusOK {
		t.Fatalf("sift after craap = %d, want 200; body=%s", rec2.Code, rec2.Body)
	}
	if out2 := decodeLensTurn(t, rec2); out2.Card == nil || out2.Card.CardID != "sift" {
		t.Fatalf("sift after craap = %+v, want a real sift card", out2)
	}
}
