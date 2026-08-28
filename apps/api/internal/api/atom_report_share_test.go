package api_test

// atom_report_share_test.go — Task 5's HTTP-level guarantees: mint, revoke,
// and the public route. newShareToken itself is tested separately in
// atom_report_share_internal_test.go (package api, since it is unexported).
//
// Every test here shares atom_report_test.go's fixtures (reportStubProvider,
// createReadingAtomHTTP, putReadingTakeawayHTTP, finishReadingHTTP,
// liteHandlerWithProvider, countingProvider) rather than inventing new ones.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// finishedSharedReadingID creates a reading atom, gives it a takeaway, plants
// a separate student chat message carrying reportStubProvider's canned quote
// (so a moment survives BOTH validateMoments and F4's dedupeMomentsAgainstKeep
// deterministically — see reportStubProvider's own comment for why the quote
// can no longer just live inside the takeaway), and finishes it — the state
// every share test starts from.
func finishedReadingID(t *testing.T, h http.Handler, cookie *http.Cookie, q *sqlc.Queries, title string) string {
	t.Helper()
	id := createReadingAtomHTTP(t, h, cookie, title)
	putReadingTakeawayHTTP(t, h, cookie, id, "我觉得应该多看数据来源，而不是只看结论")
	atomID := mustUUID(id)
	ctx := context.Background()
	seq, err := q.NextAtomMessageSeq(ctx, atomID)
	if err != nil {
		t.Fatalf("NextAtomMessageSeq: %v", err)
	}
	if _, err := q.AppendAtomMessage(ctx, sqlc.AppendAtomMessageParams{
		AtomID: atomID, Seq: seq, Role: "student",
		Content: "我又想了想，数据来源要能查到出处，这样才可信。",
	}); err != nil {
		t.Fatalf("AppendAtomMessage: %v", err)
	}
	finishReadingHTTP(t, h, cookie, id)
	return id
}

func shareReportHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, base, id string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest(http.MethodPost, "/api/v1/"+base+"/"+id+"/report/share", nil), cookie))
	return rec
}

func revokeReportHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, base, id string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest(http.MethodDelete, "/api/v1/"+base+"/"+id+"/report/share", nil), cookie))
	return rec
}

func getPublicReportHTTP(h http.Handler, token string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/public/reports/"+token, nil))
	return rec
}

func decodeShareResponse(t *testing.T, rec *httptest.ResponseRecorder) (token, url string) {
	t.Helper()
	var out struct {
		Token string `json:"token"`
		URL   string `json:"url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode share response: %v — body=%s", err, rec.Body)
	}
	if out.Token == "" {
		t.Fatalf("share response carries no token — body=%s", rec.Body)
	}
	return out.Token, out.URL
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestRevokedShareIs404 — revocation must be REAL: the next request must not
// resolve the old link. share -> public GET 200 -> revoke -> public GET 404.
func TestRevokedShareIs404(t *testing.T) {
	prov := &countingProvider{inner: reportStubProvider()}
	h, cookie, q, _ := liteHandlerWithProvider(t, prov)
	id := finishedReadingID(t, h, cookie, q, "一篇关于气候变化的文章")

	shareRec := shareReportHTTP(t, h, cookie, "readings", id)
	if shareRec.Code != http.StatusOK {
		t.Fatalf("share = %d, want 200; body=%s", shareRec.Code, shareRec.Body)
	}
	token, url := decodeShareResponse(t, shareRec)
	if !strings.Contains(url, "/s/"+token) {
		t.Fatalf("share url %q does not carry the token via /s/", url)
	}

	pubRec := getPublicReportHTTP(h, token)
	if pubRec.Code != http.StatusOK {
		t.Fatalf("public GET before revoke = %d, want 200; body=%s", pubRec.Code, pubRec.Body)
	}

	revokeRec := revokeReportHTTP(t, h, cookie, "readings", id)
	if revokeRec.Code != http.StatusNoContent {
		t.Fatalf("revoke = %d, want 204; body=%s", revokeRec.Code, revokeRec.Body)
	}

	pubRec2 := getPublicReportHTTP(h, token)
	if pubRec2.Code != http.StatusNotFound {
		t.Fatalf("public GET after revoke = %d, want 404; body=%s", pubRec2.Code, pubRec2.Body)
	}
}

// TestUnknownTokenIs404AndLooksLikeRevoked — an unknown token and a revoked
// token must be indistinguishable: both a plain 404 with the same generic
// body, so a stranger probing tokens learns nothing about whether one ever
// existed.
func TestUnknownTokenIs404AndLooksLikeRevoked(t *testing.T) {
	prov := &countingProvider{inner: reportStubProvider()}
	h, cookie, q, _ := liteHandlerWithProvider(t, prov)
	id := finishedReadingID(t, h, cookie, q, "一篇没人分享过的文章")

	shareRec := shareReportHTTP(t, h, cookie, "readings", id)
	token, _ := decodeShareResponse(t, shareRec)
	revokeReportHTTP(t, h, cookie, "readings", id)

	revoked := getPublicReportHTTP(h, token)
	unknown := getPublicReportHTTP(h, "0000000000000000000000000000000000")
	if revoked.Code != http.StatusNotFound || unknown.Code != http.StatusNotFound {
		t.Fatalf("revoked=%d unknown=%d, want both 404", revoked.Code, unknown.Code)
	}
	if revoked.Body.String() != unknown.Body.String() {
		t.Fatalf("revoked and unknown tokens produced different bodies — revoked=%s unknown=%s",
			revoked.Body, unknown.Body)
	}
}

// TestPublicPayloadCarriesNothingExtra — the public payload is the report
// envelope and nothing else. Decoded into map[string]any and checked against
// the EXACT key set so a future field silently added to the handler fails
// this test rather than shipping to the open internet.
func TestPublicPayloadCarriesNothingExtra(t *testing.T) {
	prov := &countingProvider{inner: reportStubProvider()}
	h, cookie, q, _ := liteHandlerWithProvider(t, prov)
	id := finishedReadingID(t, h, cookie, q, "一篇关于气候变化的文章")

	shareRec := shareReportHTTP(t, h, cookie, "readings", id)
	if shareRec.Code != http.StatusOK {
		t.Fatalf("share = %d, want 200; body=%s", shareRec.Code, shareRec.Body)
	}
	token, _ := decodeShareResponse(t, shareRec)

	pubRec := getPublicReportHTTP(h, token)
	if pubRec.Code != http.StatusOK {
		t.Fatalf("public GET = %d, want 200; body=%s", pubRec.Code, pubRec.Body)
	}

	var payload map[string]any
	if err := json.Unmarshal(pubRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v — body=%s", err, pubRec.Body)
	}
	if len(payload) != 1 || payload["report"] == nil {
		t.Fatalf("public payload top-level keys = %v, want exactly {\"report\"}", keysOf(payload))
	}
	report, ok := payload["report"].(map[string]any)
	if !ok {
		t.Fatalf("payload[\"report\"] is not an object: %#v", payload["report"])
	}

	want := []string{"version", "kind", "title", "studentName", "finishedAt", "stats", "moments", "keep", "gains"}
	got := keysOf(report)
	if len(got) != len(want) {
		t.Fatalf("report keys = %v, want exactly %v", got, want)
	}
	wantSet := map[string]bool{}
	for _, k := range want {
		wantSet[k] = true
	}
	for _, k := range got {
		if !wantSet[k] {
			t.Fatalf("report carries an extra field %q it must not — full key set %v", k, got)
		}
	}
}

// TestReshareReturnsExistingToken — re-sharing an already-shared report must
// hand back the SAME token, never mint a second one: a link she already sent
// someone must stay valid.
func TestReshareReturnsExistingToken(t *testing.T) {
	prov := &countingProvider{inner: reportStubProvider()}
	h, cookie, q, _ := liteHandlerWithProvider(t, prov)
	id := finishedReadingID(t, h, cookie, q, "一篇会被分享两次的文章")

	first := shareReportHTTP(t, h, cookie, "readings", id)
	if first.Code != http.StatusOK {
		t.Fatalf("first share = %d, want 200; body=%s", first.Code, first.Body)
	}
	token1, _ := decodeShareResponse(t, first)

	second := shareReportHTTP(t, h, cookie, "readings", id)
	if second.Code != http.StatusOK {
		t.Fatalf("second share = %d, want 200; body=%s", second.Code, second.Body)
	}
	token2, _ := decodeShareResponse(t, second)

	if token1 != token2 {
		t.Fatalf("re-share minted a NEW token: first=%q second=%q, want identical", token1, token2)
	}

	// The original link must still resolve — re-sharing did not orphan it.
	if rec := getPublicReportHTTP(h, token1); rec.Code != http.StatusOK {
		t.Fatalf("original token after re-share = %d, want 200; body=%s", rec.Code, rec.Body)
	}
}

// TestRevokeIsIdempotent — revoking a report is a 204 in every "nothing to
// revoke" shape: no atom_report row at all yet, and a report that was
// already revoked once.
func TestRevokeIsIdempotent(t *testing.T) {
	prov := &countingProvider{inner: reportStubProvider()}
	h, cookie, q, _ := liteHandlerWithProvider(t, prov)

	t.Run("no report row yet (never finished, never shared)", func(t *testing.T) {
		id := createReadingAtomHTTP(t, h, cookie, "还没读完也没分享过的一篇")
		rec := revokeReportHTTP(t, h, cookie, "readings", id)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("revoke with no report row = %d, want 204; body=%s", rec.Code, rec.Body)
		}
	})

	t.Run("already revoked once", func(t *testing.T) {
		id := finishedReadingID(t, h, cookie, q, "分享过又停止分享的一篇")
		if rec := shareReportHTTP(t, h, cookie, "readings", id); rec.Code != http.StatusOK {
			t.Fatalf("share = %d, want 200; body=%s", rec.Code, rec.Body)
		}
		if rec := revokeReportHTTP(t, h, cookie, "readings", id); rec.Code != http.StatusNoContent {
			t.Fatalf("first revoke = %d, want 204; body=%s", rec.Code, rec.Body)
		}
		if rec := revokeReportHTTP(t, h, cookie, "readings", id); rec.Code != http.StatusNoContent {
			t.Fatalf("second revoke (already revoked) = %d, want 204; body=%s", rec.Code, rec.Body)
		}
	})
}

// TestShareBeforeFinishIsRefused — sharing an unfinished atom must not
// silently succeed (nor spend a provider call on a report that cannot yet
// exist): ensureAtomReport's `found=false` for an unfinished atom must turn
// into an explicit refusal here, not a 200 with a token to nothing.
func TestShareBeforeFinishIsRefused(t *testing.T) {
	prov := &countingProvider{inner: reportStubProvider()}
	h, cookie, _, _ := liteHandlerWithProvider(t, prov)
	id := createReadingAtomHTTP(t, h, cookie, "还没读完的一篇")

	rec := shareReportHTTP(t, h, cookie, "readings", id)
	if rec.Code == http.StatusOK {
		t.Fatalf("share on an unfinished atom = 200, want a refusal; body=%s", rec.Body)
	}
	if n := prov.count(); n != 0 {
		t.Fatalf("provider called %d times sharing an unfinished atom, want 0", n)
	}
}

// TestWritingReportShareRoundTrip — the writing twin of the same three
// routes, wired the same way as reading's.
func TestWritingReportShareRoundTrip(t *testing.T) {
	prov := &countingProvider{inner: reportStubProvider()}
	h, cookie, q, _ := liteHandlerWithProvider(t, prov)
	id := createWritingAtomHTTP(t, h, cookie, "我想写中国的可持续发展")
	if err := q.SetWritingFinished(context.Background(), mustUUID(id)); err != nil {
		t.Fatalf("SetWritingFinished: %v", err)
	}

	shareRec := shareReportHTTP(t, h, cookie, "writings", id)
	if shareRec.Code != http.StatusOK {
		t.Fatalf("share writing report = %d, want 200; body=%s", shareRec.Code, shareRec.Body)
	}
	token, _ := decodeShareResponse(t, shareRec)

	pubRec := getPublicReportHTTP(h, token)
	if pubRec.Code != http.StatusOK {
		t.Fatalf("public GET writing report = %d, want 200; body=%s", pubRec.Code, pubRec.Body)
	}

	revokeRec := revokeReportHTTP(t, h, cookie, "writings", id)
	if revokeRec.Code != http.StatusNoContent {
		t.Fatalf("revoke writing report = %d, want 204; body=%s", revokeRec.Code, revokeRec.Body)
	}
	if rec := getPublicReportHTTP(h, token); rec.Code != http.StatusNotFound {
		t.Fatalf("public GET writing report after revoke = %d, want 404; body=%s", rec.Code, rec.Body)
	}
}

// decodeAuthedReportEnvelope decodes the AUTHENTICATED GET .../report
// response's top-level share fields — distinct from decodeShareResponse
// (the POST .../report/share response) and from the public envelope
// (TestPublicPayloadCarriesNothingExtra), which must never carry these.
func decodeAuthedReportEnvelope(t *testing.T, rec *httptest.ResponseRecorder) (shared bool, shareToken *string) {
	t.Helper()
	var out struct {
		Shared     bool    `json:"shared"`
		ShareToken *string `json:"shareToken"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode authed report envelope: %v — body=%s", err, rec.Body)
	}
	return out.Shared, out.ShareToken
}

// TestReportReflectsLiveShareState is F2: a live share must survive a
// reload. Before this fix, GET .../report said nothing about share_token at
// all, so SharePanel always mounted at {phase:"off"} — closed copy, and a
// 停止分享 button that is nowhere on screen — even though the link was still
// fully live and revocable server-side. share -> GET report -> the
// AUTHENTICATED envelope must say shared:true with the SAME token; revoke ->
// GET report again -> shared:false, token gone.
func TestReportReflectsLiveShareState(t *testing.T) {
	prov := &countingProvider{inner: reportStubProvider()}
	h, cookie, q, _ := liteHandlerWithProvider(t, prov)
	id := finishedReadingID(t, h, cookie, q, "一篇分享后又刷新页面的文章")

	// Before sharing: the report exists (she finished), but nothing is shared.
	beforeRec := getReadingReportHTTP(h, cookie, id)
	if beforeRec.Code != http.StatusOK {
		t.Fatalf("GET report before share = %d, want 200; body=%s", beforeRec.Code, beforeRec.Body)
	}
	if shared, token := decodeAuthedReportEnvelope(t, beforeRec); shared || (token != nil && *token != "") {
		t.Fatalf("before sharing: shared=%v token=%v, want false/empty", shared, token)
	}

	shareRec := shareReportHTTP(t, h, cookie, "readings", id)
	mintedToken, _ := decodeShareResponse(t, shareRec)

	// This is the reload: a FRESH GET of the same report, as if she closed
	// the tab and came back — must reflect the share that is already live.
	afterRec := getReadingReportHTTP(h, cookie, id)
	if afterRec.Code != http.StatusOK {
		t.Fatalf("GET report after share = %d, want 200; body=%s", afterRec.Code, afterRec.Body)
	}
	shared, token := decodeAuthedReportEnvelope(t, afterRec)
	if !shared {
		t.Fatalf("after sharing: shared=false, want true — body=%s", afterRec.Body)
	}
	if token == nil || *token != mintedToken {
		t.Fatalf("after sharing: shareToken=%v, want %q", token, mintedToken)
	}

	revokeReportHTTP(t, h, cookie, "readings", id)

	revokedRec := getReadingReportHTTP(h, cookie, id)
	shared, token = decodeAuthedReportEnvelope(t, revokedRec)
	if shared || (token != nil && *token != "") {
		t.Fatalf("after revoking: shared=%v token=%v, want false/empty", shared, token)
	}
}
