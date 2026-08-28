package api_test

// writing_race_test.go — F2: the two check-then-write endpoints that could be
// billed twice.
//
// Both were read-whether-it-exists, call a model, write — with nothing holding
// the gap. Production on 2026-08-28 caught POST /opening doing it 386 ms apart
// (two charges, two greetings), and POST /guide was worse: it re-guided the
// WHOLE outline on every call, with only a client-side `needsBatch` flag
// standing between a second tab and a second full flagship bill.
//
// These tests are about MONEY, so every one of them asserts a call COUNT, not
// just the resulting rows. A fix that produces one row after paying for two
// calls is not a fix.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/gateway"
)

// countingProvider counts Stream calls (mutex-guarded — the opening race test
// calls it from two goroutines at once, which is the entire point) and can
// delay each one, to hold the race window open long enough that a broken
// handler would reliably fail rather than occasionally.
//
// `inner` is swappable between calls so one handler — one testcontainer, one
// pool — can be driven with different scripted replies across a test.
type countingProvider struct {
	mu    sync.Mutex
	calls int
	inner gateway.Provider
	delay time.Duration
}

func (p *countingProvider) Stream(ctx context.Context, r gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	p.mu.Lock()
	p.calls++
	inner, delay := p.inner, p.delay
	p.mu.Unlock()
	if delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return inner.Stream(ctx, r, req)
}

func (p *countingProvider) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

func (p *countingProvider) setInner(inner gateway.Provider) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.inner = inner
}

// TestWritingOpening_ConcurrentCallsGenerateExactlyOnce — the 386 ms bug, as a
// test. Two POSTs fired at the same instant must produce ONE model call and
// ONE stored greeting. The advisory lock in postWritingOpening is what makes
// the loser block before it reaches the provider; without it both requests
// read an empty transcript, both generate, and both insert.
//
// Not timing-dependent: whichever order the two land in, the answer is one
// call. If the second arrives during the first's model call it blocks on the
// lock; if it arrives after the commit its pre-check finds the greeting.
func TestWritingOpening_ConcurrentCallsGenerateExactlyOnce(t *testing.T) {
	const greeting = "你想写的是学校该不该禁手机。我们先一起把要说的想清楚。你最想让读者相信哪一句话？"
	prov := &countingProvider{inner: writingTextStubProvider(greeting), delay: 250 * time.Millisecond}
	h, cookie, q, _ := liteHandlerWithProvider(t, prov)
	id := createWritingAtomHTTP(t, h, cookie, "该不该禁止学生带手机进校园。")

	var wg sync.WaitGroup
	recs := make([]*httptest.ResponseRecorder, 2)
	start := make(chan struct{})
	for i := range recs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			recs[i] = postWritingOpening(t, h, cookie, id)
		}(i)
	}
	close(start)
	wg.Wait()

	generated := 0
	for i, rec := range recs {
		if rec.Code != http.StatusOK {
			t.Fatalf("opening[%d] = %d, want 200; body=%s", i, rec.Code, rec.Body)
		}
		var out struct {
			Reply     string `json:"reply"`
			Generated bool   `json:"generated"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode opening[%d]: %v — body=%s", i, err, rec.Body)
		}
		if out.Reply != greeting {
			t.Fatalf("opening[%d] reply = %q, want the one greeting %q — both racers must see the SAME line", i, out.Reply, greeting)
		}
		if out.Generated {
			generated++
		}
	}
	if generated != 1 {
		t.Fatalf("generated=true on %d of 2 concurrent openings, want exactly 1", generated)
	}

	if n := prov.count(); n != 1 {
		t.Fatalf("model called %d times for one opening, want 1 — a concurrent double-open is billed twice", n)
	}

	atomID, perr := uuid.Parse(id)
	if perr != nil {
		t.Fatalf("parse writing id: %v", perr)
	}
	msgs, err := q.ListAtomMessages(context.Background(), atomID)
	if err != nil {
		t.Fatalf("ListAtomMessages: %v", err)
	}
	ai := 0
	for _, m := range msgs {
		if m.Role == "ai" {
			ai++
		}
	}
	if ai != 1 {
		t.Fatalf("stored ai messages = %d, want 1 — the room greets her twice; messages=%+v", ai, msgs)
	}
}

func postWritingGuideBatchHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, id string) map[string]writingGuideDTOForTest {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/writings/"+id+"/guide", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /guide = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Guides map[string]writingGuideDTOForTest `json:"guides"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode batch guide response: %v — body=%s", err, rec.Body)
	}
	return out.Guides
}

// writingGuideUserPrompt pulls the assembled user message back out of the
// request the handler built, so a stub can script a reply against the REAL
// block ids.
func writingGuideUserPrompt(req gateway.ChatRequest) string {
	var prompt string
	for _, m := range req.Messages {
		if m.Role == gateway.RoleUser {
			prompt = m.Content
		}
	}
	return prompt
}

// writingGuideFirstBlockOnlyStub guides ONLY the first block in the prompt,
// leaving the second without one — the "some blocks have guides" state the
// partial-regeneration test needs, produced the way production produces it
// (a model reply that covered fewer blocks than the outline had).
type writingGuideFirstBlockOnlyStub struct{}

const writingGuideFirstBlockQuestion = "第一块原来的问题？"

func (writingGuideFirstBlockOnlyStub) Stream(ctx context.Context, resolved gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	prompt := writingGuideUserPrompt(req)
	matches := writingGuideOutlineIDPattern.FindAllStringSubmatch(prompt, -1)
	if len(matches) < 1 {
		return nil, fmt.Errorf("writingGuideFirstBlockOnlyStub: no block id in prompt — prompt=%s", prompt)
	}
	reply := `{"blocks":[{"id":"` + matches[0][1] + `","job":"让读者愿意读下去",` +
		`"method_ids":["opening_suspense"],"questions":["` + writingGuideFirstBlockQuestion + `"]}]}`
	return writingTextStubProvider(reply).Stream(ctx, resolved, req)
}

// TestGuideWritingBlocks_AllStoredMakesNoModelCall — the re-bill, killed. Once
// every block has a stored guide, opening 段落 again (a second tab, a refresh,
// coming back tomorrow) must cost nothing and still return every guide.
func TestGuideWritingBlocks_AllStoredMakesNoModelCall(t *testing.T) {
	prov := &countingProvider{inner: writingGuideBatchStubProvider{}}
	h, cookie, _, _ := liteHandlerWithProvider(t, prov)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于手机是否该带进校园的议论文")

	putBody := `{"outline":[{"role":"开头","text":"用一个真实场景开头","depth":0},{"role":"理由一","text":"手机分散注意力","depth":0}]}`
	if rec := putWritingOutlineHTTP(t, h, cookie, id, putBody); rec.Code != http.StatusOK {
		t.Fatalf("put outline = %d; body=%s", rec.Code, rec.Body)
	}

	first := postWritingGuideBatchHTTP(t, h, cookie, id)
	if len(first) != 2 {
		t.Fatalf("first batch returned %d guides, want 2: %+v", len(first), first)
	}
	if n := prov.count(); n != 1 {
		t.Fatalf("first batch made %d model calls, want 1", n)
	}

	second := postWritingGuideBatchHTTP(t, h, cookie, id)
	if n := prov.count(); n != 1 {
		t.Fatalf("second batch made the model call again (total %d) — a second tab re-bills the whole outline", n)
	}
	if len(second) != 2 {
		t.Fatalf("second batch returned %d guides, want all 2 from storage: %+v", len(second), second)
	}
	for blockID, want := range first {
		got, ok := second[blockID]
		if !ok {
			t.Fatalf("block %s missing from the stored-only response: %+v", blockID, second)
		}
		if len(got.Questions) != len(want.Questions) || (len(want.Questions) > 0 && got.Questions[0] != want.Questions[0]) {
			t.Fatalf("block %s replayed differently: got %v, want %v", blockID, got.Questions, want.Questions)
		}
	}
}

// TestGuideWritingBlocks_OnlyGeneratesTheBlocksThatLackAGuide — the middle
// case, and the one with teeth: the model IS called (one block still has
// nothing), and the block that already had a guide keeps the guide it had —
// even though the stub, like a real model, happily volunteers a NEW one for
// it. Dropping that is what makes 已经付过钱的 guidance stable across visits.
func TestGuideWritingBlocks_OnlyGeneratesTheBlocksThatLackAGuide(t *testing.T) {
	prov := &countingProvider{inner: writingGuideFirstBlockOnlyStub{}}
	h, cookie, _, _ := liteHandlerWithProvider(t, prov)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于手机是否该带进校园的议论文")

	putBody := `{"outline":[{"role":"开头","text":"用一个真实场景开头","depth":0},{"role":"理由一","text":"手机分散注意力","depth":0}]}`
	if rec := putWritingOutlineHTTP(t, h, cookie, id, putBody); rec.Code != http.StatusOK {
		t.Fatalf("put outline = %d; body=%s", rec.Code, rec.Body)
	}
	rows := getWritingOutlineHTTP(t, h, cookie, id)
	if len(rows) != 2 {
		t.Fatalf("want 2 outline rows, got %d", len(rows))
	}

	first := postWritingGuideBatchHTTP(t, h, cookie, id)
	if len(first) != 1 {
		t.Fatalf("setup: first batch returned %d guides, want only block 0's: %+v", len(first), first)
	}
	if _, ok := first[rows[1].ID]; ok {
		t.Fatalf("setup: block 1 should have been left unguided")
	}

	// Now the real call: block 1 still has nothing, so the model runs; the
	// batch stub returns a guide for BOTH ids it finds in the prompt.
	prov.setInner(writingGuideBatchStubProvider{})
	second := postWritingGuideBatchHTTP(t, h, cookie, id)
	if n := prov.count(); n != 2 {
		t.Fatalf("model calls = %d, want 2 — a block with no guide must still be generated", n)
	}
	if len(second) != 2 {
		t.Fatalf("second batch returned %d guides, want 2 (one stored + one fresh): %+v", len(second), second)
	}

	g0 := second[rows[0].ID]
	if len(g0.Questions) != 1 || g0.Questions[0] != writingGuideFirstBlockQuestion {
		t.Fatalf("block 0 guide = %v, want the STORED %q unchanged — the batch route overwrote a guide it was not asked to regenerate",
			g0.Questions, writingGuideFirstBlockQuestion)
	}
	g1 := second[rows[1].ID]
	if len(g1.Questions) != 1 || g1.Questions[0] != "有没有具体的数据或事件？" {
		t.Fatalf("block 1 guide = %v, want the freshly generated one", g1.Questions)
	}

	// And the same on a fresh read: storage, not just the response body.
	recOutline := httptest.NewRecorder()
	h.ServeHTTP(recOutline, withCookie(httptest.NewRequest("GET", "/api/v1/writings/"+id+"/outline", nil), cookie))
	if recOutline.Code != http.StatusOK {
		t.Fatalf("GET outline = %d, want 200; body=%s", recOutline.Code, recOutline.Body)
	}
	var outlineResp struct {
		Outline []struct {
			ID    string                  `json:"id"`
			Guide *writingGuideDTOForTest `json:"guide"`
		} `json:"outline"`
	}
	if err := json.Unmarshal(recOutline.Body.Bytes(), &outlineResp); err != nil {
		t.Fatalf("decode outline: %v — body=%s", err, recOutline.Body)
	}
	for _, row := range outlineResp.Outline {
		if row.Guide == nil {
			t.Fatalf("row %s has no stored guide after the batch", row.ID)
		}
		if row.ID == rows[0].ID && row.Guide.Questions[0] != writingGuideFirstBlockQuestion {
			t.Fatalf("stored block 0 guide was overwritten: %v", row.Guide.Questions)
		}
	}
}

// TestGuideWritingBlock_SingleRouteStillRegenerates — the single-block route
// keeps its meaning. It is the EXPLICIT 重新生成 (she clicked it), so a stored
// guide is exactly what it is supposed to replace. The batch route's new
// short-circuit must not have leaked into it.
func TestGuideWritingBlock_SingleRouteStillRegenerates(t *testing.T) {
	prov := &countingProvider{inner: writingGuideBatchStubProvider{}}
	h, cookie, _, _ := liteHandlerWithProvider(t, prov)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于手机是否该带进校园的议论文")

	putBody := `{"outline":[{"role":"开头","text":"用一个真实场景开头","depth":0},{"role":"理由一","text":"手机分散注意力","depth":0}]}`
	if rec := putWritingOutlineHTTP(t, h, cookie, id, putBody); rec.Code != http.StatusOK {
		t.Fatalf("put outline = %d; body=%s", rec.Code, rec.Body)
	}
	rows := getWritingOutlineHTTP(t, h, cookie, id)
	if len(rows) != 2 {
		t.Fatalf("want 2 outline rows, got %d", len(rows))
	}
	if g := postWritingGuideBatchHTTP(t, h, cookie, id); len(g) != 2 {
		t.Fatalf("setup: batch returned %d guides, want 2", len(g))
	}
	if n := prov.count(); n != 1 {
		t.Fatalf("setup: %d model calls, want 1", n)
	}

	const regenerated = "重新生成之后的问题？"
	prov.setInner(writingTextStubProvider(
		`{"job":"证明这一点站得住","method_ids":["point_pee"],"questions":["` + regenerated + `"]}`))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/writings/"+id+"/outline/"+rows[1].ID+"/guide", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("single-block guide = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if n := prov.count(); n != 2 {
		t.Fatalf("model calls = %d, want 2 — 重新生成 must actually call the model even when a guide is stored", n)
	}
	var got writingGuideDTOForTest
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode single guide: %v — body=%s", err, rec.Body)
	}
	if len(got.Questions) != 1 || got.Questions[0] != regenerated {
		t.Fatalf("regenerate returned %v, want the NEW question %q", got.Questions, regenerated)
	}

	rowsAfter := getWritingOutlineHTTPWithGuides(t, h, cookie, id)
	if q := rowsAfter[rows[1].ID]; len(q.Questions) != 1 || q.Questions[0] != regenerated {
		t.Fatalf("stored guide for block 1 = %v, want the regenerated one persisted", q.Questions)
	}
	if q := rowsAfter[rows[0].ID]; len(q.Questions) != 1 || q.Questions[0] != "你有没有见过类似的事？" {
		t.Fatalf("regenerating block 1 disturbed block 0's guide: %v", q.Questions)
	}
}

func getWritingOutlineHTTPWithGuides(t *testing.T, h http.Handler, cookie *http.Cookie, id string) map[string]writingGuideDTOForTest {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/writings/"+id+"/outline", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET outline = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Outline []struct {
			ID    string                  `json:"id"`
			Guide *writingGuideDTOForTest `json:"guide"`
		} `json:"outline"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode outline: %v — body=%s", err, rec.Body)
	}
	byID := make(map[string]writingGuideDTOForTest, len(out.Outline))
	for _, row := range out.Outline {
		if row.Guide != nil {
			byID[row.ID] = *row.Guide
		}
	}
	return byID
}
