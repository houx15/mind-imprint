package api_test

// reading_block_test.go — the per-paragraph tools.
//
// 「for lite-level students, they first need to be taught about the
// paragraphs.」 These are the step below the lens: 翻译 / 关键单词 / 语法 /
// 写作解析 for an English paragraph, 成语修辞 / 案例 / 结构解析 for a Chinese
// one.
//
// The 铁律 line these tests hold is NOT "the AI must not explain" — explaining
// someone else's published paragraph is what a teacher does. It is that the
// explanations are keyed on the ARTICLE and can never reach anything SHE
// wrote: her 摘要, her 批注, her takeaway.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const enArticle = `The Urban Heat Island

If you cycle out of the city centre on a summer evening, the air gets cooler as you go. That is not your imagination.

Asphalt and concrete absorb an enormous amount of heat during the day and release it slowly after dark, which is why city nights often run several degrees warmer than the countryside around them.

Parks work the other way. Water evaporating from leaves carries heat away, and a large enough park measurably cools the streets for hundreds of metres around it.`

func listBlockTools(t *testing.T, h http.Handler, cookie *http.Cookie, id string) (string, []string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/blocks/tools", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET tools = %d; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Lang  string `json:"lang"`
		Tools []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode tools: %v — body=%s", err, rec.Body)
	}
	ids := make([]string, 0, len(out.Tools))
	for _, tool := range out.Tools {
		ids = append(ids, tool.ID)
	}
	return out.Lang, ids
}

func explainBlock(t *testing.T, h http.Handler, cookie *http.Cookie, id, blockID, tool string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+id+"/blocks/"+blockID+"/explain",
		strings.NewReader(`{"tool":"`+tool+`"}`)), cookie))
	return rec
}

// TestReadingBlockTools_DifferByArticleLanguage — an English paragraph is hard
// for different reasons than a Chinese one, so the toolset differs. And the
// language comes from the ARTICLE, never from a question put to the student:
// she already said what she wants to read by pasting it.
func TestReadingBlockTools_DifferByArticleLanguage(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))

	zhID := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, zhID, "城市为什么比郊区热？", zhArticle)
	lang, ids := listBlockTools(t, h, cookie, zhID)
	if lang != "zh" {
		t.Fatalf("lang = %q for a Chinese article", lang)
	}
	for _, want := range []string{"rhetoric", "examples", "structure"} {
		if !containsString(ids, want) {
			t.Fatalf("zh tools = %v, missing %q", ids, want)
		}
	}
	if containsString(ids, "grammar") {
		t.Fatalf("zh tools include 语法讲解 (%v) — that is the English set", ids)
	}

	enID := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, enID, "The Urban Heat Island", enArticle)
	lang2, ids2 := listBlockTools(t, h, cookie, enID)
	if lang2 != "en" {
		t.Fatalf("lang = %q for an English article", lang2)
	}
	for _, want := range []string{"translate", "vocabulary", "grammar", "craft"} {
		if !containsString(ids2, want) {
			t.Fatalf("en tools = %v, missing %q", ids2, want)
		}
	}
}

func containsString(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// TestReadingBlockExplain_ExplainsAndThenReplaysFromCache — a paragraph's
// translation does not change, so the second click must cost nothing. The
// cache is what makes the tools feel like tools rather than like chat.
func TestReadingBlockExplain_ExplainsAndThenReplaysFromCache(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("柏油路和水泥白天吸热，夜里放热，所以城里晚上更热。"))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)

	rec := explainBlock(t, h, cookie, id, "b3", "structure")
	if rec.Code != http.StatusOK {
		t.Fatalf("explain = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var first struct {
		BlockID string `json:"blockId"`
		Tool    string `json:"tool"`
		Body    string `json:"body"`
		Cached  bool   `json:"cached"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &first); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if first.Cached {
		t.Fatalf("the FIRST call reported a cache hit")
	}
	if strings.TrimSpace(first.Body) == "" {
		t.Fatalf("explanation was empty")
	}

	rec2 := explainBlock(t, h, cookie, id, "b3", "structure")
	var second struct {
		Body   string `json:"body"`
		Cached bool   `json:"cached"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode second: %v", err)
	}
	if !second.Cached {
		t.Fatalf("the second click paid for the same explanation again")
	}
	if second.Body != first.Body {
		t.Fatalf("replay differs:\n first=%q\nsecond=%q", first.Body, second.Body)
	}

	// And it survives a reload, so a refresh does not wipe what she opened.
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/blocks/notes", nil), cookie))
	var notes struct {
		Notes []struct {
			BlockID string `json:"blockId"`
			Tool    string `json:"tool"`
			Body    string `json:"body"`
		} `json:"notes"`
	}
	if err := json.Unmarshal(rec3.Body.Bytes(), &notes); err != nil {
		t.Fatalf("decode notes: %v — body=%s", err, rec3.Body)
	}
	if len(notes.Notes) != 1 || notes.Notes[0].BlockID != "b3" || notes.Notes[0].Tool != "structure" {
		t.Fatalf("notes = %+v, want the one explanation she opened", notes.Notes)
	}
}

// TestReadingBlockExplain_RefusesTheWrongLanguageTool — 语法讲解 on a Chinese
// paragraph would produce something confidently useless. Refuse rather than
// spend.
func TestReadingBlockExplain_RefusesTheWrongLanguageTool(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)

	if rec := explainBlock(t, h, cookie, id, "b3", "grammar"); rec.Code != http.StatusBadRequest {
		t.Fatalf("grammar on a Chinese paragraph = %d, want 400; body=%s", rec.Code, rec.Body)
	}
}

// TestReadingBlockExplain_UnknownToolAndBlockAreRefused — fail closed on both.
func TestReadingBlockExplain_UnknownToolAndBlockAreRefused(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)

	if rec := explainBlock(t, h, cookie, id, "b3", "rewrite-it-for-me"); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown tool = %d, want 400; body=%s", rec.Code, rec.Body)
	}
	if rec := explainBlock(t, h, cookie, id, "b99", "structure"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown block = %d, want 404; body=%s", rec.Code, rec.Body)
	}
}

// TestReadingBlockExplain_NeverTouchesWhatSheWrote is the 铁律 assertion.
//
// The tools explain the ARTICLE. Her 摘要 and her takeaway must read exactly
// as she left them afterwards — the explanations live in their own table with
// no write path into hers, and this proves it end to end rather than by
// reading the code.
func TestReadingBlockExplain_NeverTouchesWhatSheWrote(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("这一段是全文的原理部分。"))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)

	const herTakeaway = "我以前以为市区热是因为人多，其实是水泥和柏油在放热。"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/readings/"+id+"/takeaway",
		strings.NewReader(`{"text":"`+herTakeaway+`"}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("put takeaway = %d; body=%s", rec.Code, rec.Body)
	}

	for _, tool := range []string{"structure", "examples", "rhetoric"} {
		if rec := explainBlock(t, h, cookie, id, "b3", tool); rec.Code != http.StatusOK {
			t.Fatalf("explain %s = %d; body=%s", tool, rec.Code, rec.Body)
		}
	}

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/takeaway", nil), cookie))
	var got struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode takeaway: %v — body=%s", err, rec2.Body)
	}
	if got.Text != herTakeaway {
		t.Fatalf("her takeaway changed after three explanations:\n want %q\n  got %q", herTakeaway, got.Text)
	}
}

// TestReadingBlockExplain_ModelFailureSurfaces — USER RULE: a real 502, never
// a canned stand-in explanation.
func TestReadingBlockExplain_ModelFailureSurfaces(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, streamErrorProvider{})
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)
	if rec := explainBlock(t, h, cookie, id, "b3", "structure"); rec.Code != http.StatusBadGateway {
		t.Fatalf("explain on model failure = %d, want 502; body=%s", rec.Code, rec.Body)
	}
}
