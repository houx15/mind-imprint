package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/liteparent"
	"mindimprint/api/internal/liteweek"
)

// parentKeywordReply is for facts with the reading, the moment 「雨水不是废水」
// and the keyword 海绵城市: sections overview, reading, interests, next. It
// quotes nothing.
const parentKeywordReply = `{"overview":"这段时间读完《城市里的雨水花园》。","reading":"读完《城市里的雨水花园》。","interests":"她开始关注海绵城市。","next":"请和她聊一聊雨水花园。"}`

func keysOfRaw(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func parentHiddenJSON(t *testing.T, moments, keywords []string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"hidden": map[string]any{"moments": moments, "keywords": keywords}})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// storedParentHidden reads lite_parent_report.hidden as stored.
func storedParentHidden(t *testing.T, pool *pgxpool.Pool, id string) liteparent.Hidden {
	t.Helper()
	var raw []byte
	if err := pool.QueryRow(context.Background(), `SELECT hidden FROM lite_parent_report WHERE id = $1`, uuid.MustParse(id)).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var h liteparent.Hidden
	if err := json.Unmarshal(raw, &h); err != nil {
		t.Fatalf("stored hidden %s: %v", raw, err)
	}
	return h
}

// seedParentKeyword adds 海绵城市, first seen inside the default range.
func seedParentKeyword(t *testing.T, pool *pgxpool.Pool, studentID uuid.UUID) {
	t.Helper()
	seedWeekKeyword(t, pool, studentID, "海绵城市", liteweek.Day(time.Now()).AddDate(0, 0, -3).Add(9*time.Hour))
}

// parentCaptureProvider records every request before the scripted reply.
type parentCaptureProvider struct {
	inner    *gateway.SequenceStubProvider
	mu       sync.Mutex
	requests []gateway.ChatRequest
}

func (p *parentCaptureProvider) Stream(ctx context.Context, r gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	p.mu.Lock()
	p.requests = append(p.requests, req)
	p.mu.Unlock()
	return p.inner.Stream(ctx, r, req)
}

func (p *parentCaptureProvider) take() []gateway.ChatRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := p.requests
	p.requests = nil
	return out
}

// TestLiteParentReportHiddenPatch: PATCH hidden is validated against the
// frozen facts, replaces the stored set and de-duplicates it; hiding every
// keyword removes interests from sections while its body text stays stored; a
// body-only PATCH keeps hidden; a body key is checked against the visible
// facts of the same request.
func TestLiteParentReportHiddenPatch(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(parentKeywordReply))
	h, pool, teacher, classID, studentID := parentFixture(t, prov)
	seedParentKeyword(t, pool, studentID)
	gen := generateParentReport(t, h, teacher, classID, studentID)
	if gen.DraftError != nil {
		t.Fatalf("generate draftError = %s", *gen.DraftError)
	}
	rep := gen.Report
	path := parentReportPath(rep.ID)
	allSections := []string{"overview", "reading", "interests", "next"}
	if !reflect.DeepEqual(rep.Sections, allSections) || len(rep.Facts.Keywords) != 1 || len(rep.Facts.Moments) != 1 {
		t.Fatalf("generated report = %+v", rep)
	}

	// Valid: one moment hidden.
	oneMoment := liteparent.Hidden{Moments: []string{"雨水不是废水"}, Keywords: []string{}}
	var got parentReportResp
	if code, body := parentDo(t, h, teacher, "PATCH", path, parentHiddenJSON(t, []string{"雨水不是废水"}, nil), &got); code != http.StatusOK {
		t.Fatalf("PATCH hidden = %d %s", code, body)
	}
	if !reflect.DeepEqual(got.Report.Hidden, oneMoment) || !reflect.DeepEqual(got.Report.Sections, allSections) ||
		!reflect.DeepEqual(got.Report.Body, rep.Body) || len(got.Report.Facts.Moments) != 1 {
		t.Fatalf("after hiding a moment = %+v, want hidden %+v, facts in full and body unchanged", got.Report, oneMoment)
	}
	if s := storedParentHidden(t, pool, rep.ID); !reflect.DeepEqual(s, oneMoment) {
		t.Fatalf("stored hidden = %+v, want %+v", s, oneMoment)
	}

	// Invalid: an entry that is not in the frozen facts, exactly.
	for _, tc := range []struct{ what, body string }{
		{"unknown quote", parentHiddenJSON(t, []string{"编造的话"}, nil)},
		{"part of a quote", parentHiddenJSON(t, []string{"雨水"}, nil)},
		{"keyword text as a moment", parentHiddenJSON(t, []string{"海绵城市"}, nil)},
		{"unknown keyword", parentHiddenJSON(t, nil, []string{"气候"})},
		{"quote as a keyword", parentHiddenJSON(t, nil, []string{"雨水不是废水"})},
	} {
		code, body := parentDo(t, h, teacher, "PATCH", path, tc.body, nil)
		wantParentError(t, "PATCH hidden "+tc.what, code, body, http.StatusBadRequest, "invalid_hidden")
		if !strings.Contains(body, "要隐藏的内容不在这份报告里") {
			t.Fatalf("%s message = %s", tc.what, body)
		}
	}
	if s := storedParentHidden(t, pool, rep.ID); !reflect.DeepEqual(s, oneMoment) {
		t.Fatalf("stored hidden after refused PATCHes = %+v, want %+v", s, oneMoment)
	}

	// Every keyword hidden, with duplicates: interests leaves sections, its
	// body text stays.
	both := liteparent.Hidden{Moments: []string{"雨水不是废水"}, Keywords: []string{"海绵城市"}}
	got = parentReportResp{}
	if code, body := parentDo(t, h, teacher, "PATCH", path,
		parentHiddenJSON(t, []string{"雨水不是废水", "雨水不是废水"}, []string{"海绵城市", "海绵城市"}), &got); code != http.StatusOK {
		t.Fatalf("PATCH hide all keywords = %d %s", code, body)
	}
	if !reflect.DeepEqual(got.Report.Hidden, both) || !reflect.DeepEqual(got.Report.Sections, []string{"overview", "reading", "next"}) {
		t.Fatalf("after hiding every keyword: hidden = %+v sections = %v", got.Report.Hidden, got.Report.Sections)
	}
	if got.Report.Body["interests"] != "她开始关注海绵城市。" {
		t.Fatalf("interests body must be kept: %v", got.Report.Body)
	}
	if n := weeklyCount(t, pool, `SELECT count(*) FROM lite_parent_report WHERE id = $1 AND body ? 'interests'`, uuid.MustParse(rep.ID)); n != 1 {
		t.Fatal("stored body lost interests")
	}

	// Body only: hidden is kept.
	got = parentReportResp{}
	if code, body := parentDo(t, h, teacher, "PATCH", path, parentBodyJSON(t, map[string]string{"overview": "老师改过的概述"}), &got); code != http.StatusOK {
		t.Fatalf("PATCH body = %d %s", code, body)
	}
	if !reflect.DeepEqual(got.Report.Hidden, both) || got.Report.Body["overview"] != "老师改过的概述" || got.Report.Body["interests"] != "她开始关注海绵城市。" {
		t.Fatalf("after body-only PATCH = %+v", got.Report)
	}
	if s := storedParentHidden(t, pool, rep.ID); !reflect.DeepEqual(s, both) {
		t.Fatalf("stored hidden after body-only PATCH = %+v, want %+v", s, both)
	}

	// A body key for a section the visible facts no longer support is refused;
	// showing the keyword again in the same request allows it.
	code, body := parentDo(t, h, teacher, "PATCH", path, parentBodyJSON(t, map[string]string{"interests": "改写"}), nil)
	wantParentError(t, "PATCH body of a hidden section", code, body, http.StatusBadRequest, "invalid_section")
	got = parentReportResp{}
	if code, body := parentDo(t, h, teacher, "PATCH", path,
		`{"hidden":{"moments":["雨水不是废水"]},"body":{"interests":"改写"}}`, &got); code != http.StatusOK {
		t.Fatalf("PATCH show keyword and edit interests = %d %s", code, body)
	}
	if !reflect.DeepEqual(got.Report.Hidden, oneMoment) || !reflect.DeepEqual(got.Report.Sections, allSections) || got.Report.Body["interests"] != "改写" {
		t.Fatalf("after showing the keyword = %+v", got.Report)
	}

	// {"hidden":{}} clears the set; the lists are written as arrays, never null.
	code, body = parentDo(t, h, teacher, "PATCH", path, `{"hidden":{}}`, nil)
	if code != http.StatusOK {
		t.Fatalf("PATCH empty hidden = %d %s", code, body)
	}
	if raw := rawWeeklyFields(t, string(rawWeeklyFields(t, body)["report"])); string(raw["hidden"]) != `{"moments":[],"keywords":[]}` {
		t.Fatalf("hidden after clearing = %s", raw["hidden"])
	}
	code, body = parentDo(t, h, teacher, "PATCH", path, `{}`, nil)
	wantParentError(t, "PATCH with neither body nor hidden", code, body, http.StatusBadRequest, "invalid_body")
}

// TestLiteParentReportRedraftHidesItems: a redraft composes from the visible
// facts. The hidden quote and keyword are in neither the system prompt nor the
// facts message, interests is not asked for, and a reply quoting the hidden
// quote in 「」 fails the quote check on both attempts. A reply that quotes
// nothing then passes.
func TestLiteParentReportRedraftHidesItems(t *testing.T) {
	prov := &parentCaptureProvider{inner: gateway.NewSequenceStubProvider(
		weeklyReply(parentKeywordReply), // generate
		weeklyReply(parentValidReply),   // redraft 1, attempt 1: quotes 「雨水不是废水」
		weeklyReply(parentValidReply),   // redraft 1, attempt 2
		weeklyReply(parentPlainReply),   // redraft 2
	)}
	h, pool, teacher, classID, studentID := parentFixture(t, prov)
	seedParentKeyword(t, pool, studentID)
	rep := generateParentReport(t, h, teacher, classID, studentID).Report
	path := parentReportPath(rep.ID)
	if gen := prov.take(); len(gen) != 1 || !strings.Contains(gen[0].Messages[1].Content, "雨水不是废水") || !strings.Contains(gen[0].Messages[1].Content, "海绵城市") {
		t.Fatalf("generate must see every fact: %+v", gen)
	}
	if code, body := parentDo(t, h, teacher, "PATCH", path, parentHiddenJSON(t, []string{"雨水不是废水"}, []string{"海绵城市"}), nil); code != http.StatusOK {
		t.Fatalf("PATCH hidden = %d %s", code, body)
	}

	var got parentReportResp
	if code, body := parentDo(t, h, teacher, "POST", path+"/redraft", `{"replaceBody":true}`, &got); code != http.StatusOK {
		t.Fatalf("redraft = %d %s", code, body)
	}
	if got.DraftError == nil || !strings.Contains(*got.DraftError, "quote not in corpus") {
		t.Fatalf("draftError = %v, want the quote check to reject the hidden 金句", got.DraftError)
	}
	reqs := prov.take()
	if len(reqs) != 2 || prov.inner.Calls != 3 || len(llmCallUsers(t, pool, "lite_parent_report")) != 3 {
		t.Fatalf("redraft requests = %d, calls = %d, want 2 attempts after the generate", len(reqs), prov.inner.Calls)
	}
	for i, req := range reqs {
		if len(req.Messages) < 2 {
			t.Fatalf("attempt %d messages = %+v", i+1, req.Messages)
		}
		// Messages[0] is the system prompt, [1] the facts. A retry adds the
		// model's own reply and the validation error after them.
		system, facts := req.Messages[0].Content, req.Messages[1].Content
		for _, hidden := range []string{"雨水不是废水", "海绵城市"} {
			if strings.Contains(system, hidden) || strings.Contains(facts, hidden) {
				t.Fatalf("attempt %d prompt carries hidden %q:\n%s\n%s", i+1, hidden, system, facts)
			}
		}
		if strings.Contains(system, "interests") || !strings.Contains(facts, "《城市里的雨水花园》") {
			t.Fatalf("attempt %d: system = %s facts = %s", i+1, system, facts)
		}
	}
	if !reflect.DeepEqual(got.Report.Draft, rep.Draft) || !reflect.DeepEqual(got.Report.Body, rep.Body) {
		t.Fatalf("a rejected redraft wrote: draft = %v body = %v", got.Report.Draft, got.Report.Body)
	}

	got = parentReportResp{}
	if code, body := parentDo(t, h, teacher, "POST", path+"/redraft", `{"replaceBody":false}`, &got); code != http.StatusOK || got.DraftError != nil {
		t.Fatalf("redraft with a plain reply = %d %s", code, body)
	}
	if !reflect.DeepEqual(got.Report.Draft, parentSectionsOf(t, parentPlainReply)) || got.Report.Body["interests"] != rep.Body["interests"] {
		t.Fatalf("plain redraft: draft = %v body = %v", got.Report.Draft, got.Report.Body)
	}
}

// TestLiteParentReportHiddenMentions (fix round 1): the body still quoting a
// hidden 金句 is reported in hiddenMentions, per visible section, and the
// entry goes away once the teacher edits the quote out.
func TestLiteParentReportHiddenMentions(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(parentValidReply))
	h, _, teacher, classID, studentID := parentFixture(t, prov)
	gen := generateParentReport(t, h, teacher, classID, studentID)
	if !strings.Contains(gen.Report.Body["overview"], "「雨水不是废水」") {
		t.Fatalf("generated body must quote the moment: %v", gen.Report.Body)
	}
	path := parentReportPath(gen.Report.ID)
	mentions := func(what, body string) string {
		t.Helper()
		return string(rawWeeklyFields(t, string(rawWeeklyFields(t, body)["report"]))["hiddenMentions"])
	}
	if _, body := parentDo(t, h, teacher, "GET", path, "", nil); mentions("GET", body) != `{}` {
		t.Fatalf("hiddenMentions with nothing hidden = %s, want {}", mentions("GET", body))
	}

	code, body := parentDo(t, h, teacher, "PATCH", path, parentHiddenJSON(t, []string{"雨水不是废水"}, nil), nil)
	if code != http.StatusOK || mentions("PATCH hidden", body) != `{"overview":["雨水不是废水"]}` {
		t.Fatalf("PATCH hidden = %d, hiddenMentions = %s", code, mentions("PATCH hidden", body))
	}
	if _, body := parentDo(t, h, teacher, "GET", path, "", nil); mentions("GET", body) != `{"overview":["雨水不是废水"]}` {
		t.Fatalf("GET hiddenMentions = %s", mentions("GET", body))
	}

	code, body = parentDo(t, h, teacher, "PATCH", path, parentBodyJSON(t, map[string]string{"overview": "这段时间读完《城市里的雨水花园》。"}), nil)
	if code != http.StatusOK || mentions("PATCH body", body) != `{}` {
		t.Fatalf("PATCH body without the quote = %d, hiddenMentions = %s, want {}", code, mentions("PATCH body", body))
	}
}

// TestLiteParentReportHiddenMentionsFragment (final fix A): a draft that
// quotes only part of a 金句 passes the prose check. Hiding that moment
// afterwards flags the section with the full quote, so the fragment cannot
// reach the exported picture; editing the fragment out clears it.
func TestLiteParentReportHiddenMentionsFragment(t *testing.T) {
	const fragmentReply = `{"overview":"这段时间读完《城市里的雨水花园》，写下「不是废水」。","reading":"读完《城市里的雨水花园》。","next":"请和她聊一聊雨水花园。"}`
	prov := gateway.NewSequenceStubProvider(weeklyReply(fragmentReply))
	h, _, teacher, classID, studentID := parentFixture(t, prov)
	gen := generateParentReport(t, h, teacher, classID, studentID)
	if gen.DraftError != nil || !strings.Contains(gen.Report.Body["overview"], "「不是废水」") {
		t.Fatalf("generated report must quote the fragment: draftError = %v body = %v", gen.DraftError, gen.Report.Body)
	}
	path := parentReportPath(gen.Report.ID)
	mentions := func(body string) string {
		t.Helper()
		return string(rawWeeklyFields(t, string(rawWeeklyFields(t, body)["report"]))["hiddenMentions"])
	}

	code, body := parentDo(t, h, teacher, "PATCH", path, parentHiddenJSON(t, []string{"雨水不是废水"}, nil), nil)
	if code != http.StatusOK || mentions(body) != `{"overview":["雨水不是废水"]}` {
		t.Fatalf("PATCH hidden = %d, hiddenMentions = %s, want the full quote under overview", code, mentions(body))
	}

	code, body = parentDo(t, h, teacher, "PATCH", path, parentBodyJSON(t, map[string]string{"overview": "这段时间读完《城市里的雨水花园》。"}), nil)
	if code != http.StatusOK || mentions(body) != `{}` {
		t.Fatalf("PATCH body without the fragment = %d, hiddenMentions = %s, want {}", code, mentions(body))
	}
}

// TestLiteParentReportHiddenStrictDecode (fix round 1): an unknown key inside
// hidden is 400 invalid_hidden, never read as an empty set.
func TestLiteParentReportHiddenStrictDecode(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(parentValidReply))
	h, pool, teacher, classID, studentID := parentFixture(t, prov)
	rep := generateParentReport(t, h, teacher, classID, studentID).Report
	path := parentReportPath(rep.ID)
	oneMoment := liteparent.Hidden{Moments: []string{"雨水不是废水"}, Keywords: []string{}}
	if code, body := parentDo(t, h, teacher, "PATCH", path, parentHiddenJSON(t, []string{"雨水不是废水"}, nil), nil); code != http.StatusOK {
		t.Fatalf("PATCH hidden = %d %s", code, body)
	}
	for _, tc := range []struct{ what, body string }{
		{"misspelt key", `{"hidden":{"moment":["雨水不是废水"]}}`},
		{"extra key", `{"hidden":{"moments":[],"keywords":[],"extra":1}}`},
		{"not an object", `{"hidden":["雨水不是废水"]}`},
	} {
		code, body := parentDo(t, h, teacher, "PATCH", path, tc.body, nil)
		wantParentError(t, "PATCH hidden "+tc.what, code, body, http.StatusBadRequest, "invalid_hidden")
		if s := storedParentHidden(t, pool, rep.ID); !reflect.DeepEqual(s, oneMoment) {
			t.Fatalf("%s: stored hidden = %+v, want %+v kept", tc.what, s, oneMoment)
		}
	}
}

// TestLiteParentReportPublishRoutesGone: there is no parent end. Publish,
// revoke, the public page and her own copy are not routes any more.
func TestLiteParentReportPublishRoutesGone(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(parentValidReply))
	h, pool, teacher, classID, studentID := parentFixture(t, prov)
	rep := generateParentReport(t, h, teacher, classID, studentID).Report
	student := signInAs(t, pool, studentID)

	for _, rt := range []struct {
		who    string
		cookie *http.Cookie
		method string
		path   string
	}{
		{"teacher", teacher, "POST", parentReportPath(rep.ID) + "/publish"},
		{"teacher", teacher, "DELETE", parentReportPath(rep.ID) + "/share"},
		{"student", student, "GET", "/api/v1/lite/parent-reports/" + rep.ID},
		{"student", student, "POST", "/api/v1/lite/parent-reports/" + rep.ID + "/seen"},
	} {
		rec := doJSON(t, h, rt.cookie, rt.method, rt.path, "")
		if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s %s %s = %d %s, want no route", rt.who, rt.method, rt.path, rec.Code, rec.Body)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/public/parent-reports/0123456789abcdef0123456789abcdef", nil))
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("public parent page = %d %s, want no route", rec.Code, rec.Body)
	}
	if prov.Calls != 1 {
		t.Fatalf("provider calls = %d, want 1 (the generate)", prov.Calls)
	}
}
