package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"mindimprint/api/internal/gateway"
)

// writing_guide_test.go — Task 4 fix round 1: handler-level regression
// coverage for the batch guide route (guideWritingBlocks) and its GET
// /outline round trip. Nothing committed previously exercised
// POST /writings/{id}/guide end to end — the only verification during Task 4
// was a throwaway HTTP smoke test, deleted before that commit. Tasks 9 and
// 11 build the frontend directly on this wiring, so a silent break here
// would surface late and look like a frontend bug.

// writingGuideOutlineIDPattern extracts the block ids buildWritingGuideBatchPrompt
// embeds in the prompt it sends ("- id=<uuid> · <role>...", writing_guide.go),
// so the stub below can script a reply against REAL, freshly-minted ids
// without a two-pass create/read/recreate dance.
var writingGuideOutlineIDPattern = regexp.MustCompile(`id=([0-9a-fA-F-]{36})`)

// writingGuideBatchStubProvider scripts a batch reply for exactly the two
// block ids it finds in the outgoing prompt. Block 0's reply mixes a VALID
// method id with an UNKNOWN one, and a real question with a DECLARATIVE
// sentence — proving both of parseWritingGuideBatch's filters (unknown
// method id dropped, ？ question filter) survive the batch path, not just
// the single-block one covered by writing_guide_internal_test.go. Block 1's
// reply is clean, to prove the ordinary case also round-trips.
type writingGuideBatchStubProvider struct{}

func (writingGuideBatchStubProvider) Stream(ctx context.Context, resolved gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	var prompt string
	for _, m := range req.Messages {
		if m.Role == gateway.RoleUser {
			prompt = m.Content
		}
	}
	matches := writingGuideOutlineIDPattern.FindAllStringSubmatch(prompt, -1)
	if len(matches) < 2 {
		return nil, fmt.Errorf("writingGuideBatchStubProvider: expected at least 2 block ids in prompt, found %d — prompt=%s", len(matches), prompt)
	}
	id0, id1 := matches[0][1], matches[1][1]
	reply := `{"blocks":[
		{"id":"` + id0 + `","job":"让读者愿意读下去","method_ids":["opening_suspense","bogus_id"],"questions":["你有没有见过类似的事？","你可以写：手机让人分心。"]},
		{"id":"` + id1 + `","job":"证明这一点站得住","method_ids":["point_pee"],"questions":["有没有具体的数据或事件？"]}
	]}`
	return writingTextStubProvider(reply).Stream(ctx, resolved, req)
}

// writingGuideDTOForTest mirrors writingGuideDTO's wire shape (job/methods/
// questions) for decoding responses in this file — a local copy rather than
// referencing the unexported api-package type, since this file is api_test.
type writingGuideDTOForTest struct {
	Job     string `json:"job"`
	Methods []struct {
		Name       string `json:"name"`
		FormalName string `json:"formalName"`
	} `json:"methods"`
	Questions []string `json:"questions"`
}

// TestGuideWritingBlocks_BatchPersistsAndOutlineEchoesIt — the round trip
// this fix exists to cover: seed a 2-block outline, POST the batch route,
// assert BOTH blocks got a persisted guide via the batch response, then
// assert GET /outline (a fresh request) echoes the SAME stored guide for
// both blocks, with the unknown method id and the declarative "question"
// dropped and the surviving question intact.
func TestGuideWritingBlocks_BatchPersistsAndOutlineEchoesIt(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingGuideBatchStubProvider{})
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于手机是否该带进校园的议论文")

	putBody := `{"outline":[{"role":"开头","text":"用一个真实场景开头","depth":0},{"role":"理由一","text":"手机分散注意力","depth":0}]}`
	if rec := putWritingOutlineHTTP(t, h, cookie, id, putBody); rec.Code != http.StatusOK {
		t.Fatalf("put outline = %d; body=%s", rec.Code, rec.Body)
	}
	rows := getWritingOutlineHTTP(t, h, cookie, id)
	if len(rows) != 2 {
		t.Fatalf("want 2 outline rows, got %d", len(rows))
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/writings/"+id+"/guide", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /guide = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	var batchOut struct {
		Guides map[string]writingGuideDTOForTest `json:"guides"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &batchOut); err != nil {
		t.Fatalf("decode batch guide response: %v — body=%s", err, rec.Body)
	}
	if len(batchOut.Guides) != 2 {
		t.Fatalf("guides = %d, want 2 (both blocks persisted): %+v", len(batchOut.Guides), batchOut.Guides)
	}

	g0, ok0 := batchOut.Guides[rows[0].ID]
	if !ok0 {
		t.Fatalf("block 0 (%s) missing from batch response: %+v", rows[0].ID, batchOut.Guides)
	}
	if len(g0.Questions) != 1 || g0.Questions[0] != "你有没有见过类似的事？" {
		t.Fatalf("block 0 questions = %v, want only the real question (declarative sentence dropped)", g0.Questions)
	}
	// 留个悬念 is the STUDENT-FACING name; 留悬念 is the curriculum term the
	// explainer card reveals. Both must reach the client (2026-08-28 ruling).
	if len(g0.Methods) != 1 || g0.Methods[0].Name != "留个悬念" || g0.Methods[0].FormalName != "留悬念" {
		t.Fatalf("block 0 methods = %+v, want only opening_suspense resolved to 留个悬念/留悬念 (bogus_id dropped)", g0.Methods)
	}

	g1, ok1 := batchOut.Guides[rows[1].ID]
	if !ok1 {
		t.Fatalf("block 1 (%s) missing from batch response: %+v", rows[1].ID, batchOut.Guides)
	}
	if len(g1.Questions) != 1 || g1.Questions[0] != "有没有具体的数据或事件？" {
		t.Fatalf("block 1 questions = %v, want the one real question", g1.Questions)
	}

	// The persistence half: a FRESH GET /outline (no reliance on the
	// POST /guide response body) must echo the same stored guide for both
	// blocks — this is what Task 9/11's frontend reads on first paint.
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
	if len(outlineResp.Outline) != 2 {
		t.Fatalf("outline rows = %d, want 2", len(outlineResp.Outline))
	}
	byID := map[string]writingGuideDTOForTest{}
	for _, row := range outlineResp.Outline {
		if row.Guide == nil {
			t.Fatalf("row %s: GET /outline returned no guide — persistence or the DTO wiring is broken", row.ID)
		}
		byID[row.ID] = *row.Guide
	}

	got0 := byID[rows[0].ID]
	if len(got0.Questions) != 1 || got0.Questions[0] != "你有没有见过类似的事？" {
		t.Fatalf("GET /outline block 0 questions = %v, want only the real question surviving", got0.Questions)
	}
	if len(got0.Methods) != 1 || got0.Methods[0].Name != "留个悬念" || got0.Methods[0].FormalName != "留悬念" {
		t.Fatalf("GET /outline block 0 methods = %+v, want only 留个悬念/留悬念 (unknown id stayed dropped on read-back)", got0.Methods)
	}

	got1 := byID[rows[1].ID]
	if len(got1.Questions) != 1 || got1.Questions[0] != "有没有具体的数据或事件？" {
		t.Fatalf("GET /outline block 1 questions = %v, want the one real question", got1.Questions)
	}
}
