package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"mindimprint/api/internal/gateway"
)

// 「查词」走一遍**真的 HTTP 处理函数**。
//
// 🚨 这个文件存在是因为 2026-09-17 的那个 bug 单元测试和实测都没抓到：两者都
// 直接调用 prompt 拼装函数，而处理函数在那之前有一道闸，把「不是按句子讲」的
// 工具带来的文字**清空**。查词的那个词因此从来没到过模型那儿 —— 模型自己挑了
// 一个词（线上：点 prolonged，讲 starved to death），而且整段只缓存一份，
// 这一段里点哪个词都会重放同一张卡。用例的形状要从生产路径上抄。

// echoWordProvider 读 prompt 里【要讲解的这一个词】后面那一行，照着它回一张
// 词卡 —— 词没传到，它回的就是空词，这个测试就挂。
type echoWordProvider struct {
	mu    sync.Mutex
	calls int
}

func (p *echoWordProvider) Stream(_ context.Context, _ gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	p.mu.Lock()
	p.calls++
	p.mu.Unlock()
	word := ""
	for _, m := range req.Messages {
		if m.Role != gateway.RoleUser {
			continue
		}
		if _, after, ok := strings.Cut(m.Content, "【要讲解的这一个词】\n"); ok {
			word, _, _ = strings.Cut(after, "\n")
		}
	}
	reply, _ := json.Marshal(map[string]any{"words": []map[string]string{
		{"term": word, "pos": "形容词", "meaning": "意思：" + word},
	}})
	ch := make(chan gateway.StreamEvent, 3)
	ch <- gateway.StreamEvent{Kind: gateway.EventTextDelta, TextDelta: string(reply)}
	ch <- gateway.StreamEvent{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 10, OutputTokens: 10}}
	ch <- gateway.StreamEvent{Kind: gateway.EventDone, StopReason: gateway.StopStop}
	close(ch)
	return ch, nil
}

const lookupArticle = "During this prolonged period of cold and darkness, plants were unable to photosynthesize.\n\nThe fluffy feathers kept the birds warm."

func explainLookup(t *testing.T, h http.Handler, cookie *http.Cookie, id, block, word string) (int, []map[string]string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"tool": "lookup", "sentence": word})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+id+"/blocks/"+block+"/explain", strings.NewReader(string(body))), cookie))
	var out struct {
		Words []map[string]string `json:"words"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out.Words
}

func TestLookupSendsTheTappedWordAndCachesPerWord(t *testing.T) {
	prov := &echoWordProvider{}
	h, cookie, _, _ := liteHandlerWithProvider(t, prov)
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "羽毛与大灭绝", lookupArticle)

	code, words := explainLookup(t, h, cookie, id, "b1", "prolonged")
	if code != http.StatusOK || len(words) != 1 || words[0]["term"] != "prolonged" {
		t.Fatalf("点 prolonged：HTTP %d，卡片 %+v —— 那个词没传到模型那儿", code, words)
	}
	// 🚨 同一段里点另一个词，拿到的必须是那个词的卡，不是上一张的重放。
	code, words = explainLookup(t, h, cookie, id, "b1", "darkness")
	if code != http.StatusOK || len(words) != 1 || words[0]["term"] != "darkness" {
		t.Fatalf("点 darkness：HTTP %d，卡片 %+v —— 重放了上一个词的卡", code, words)
	}
	// 同一个词第二次点是重放，不再花一次调用。
	before := prov.calls
	code, words = explainLookup(t, h, cookie, id, "b1", "prolonged")
	if code != http.StatusOK || words[0]["term"] != "prolonged" || prov.calls != before {
		t.Errorf("第二次点 prolonged：HTTP %d，卡片 %+v，调用 %d → %d（应该是重放）", code, words, before, prov.calls)
	}
}

func TestLookupRefusesAnEmptyWord(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, &echoWordProvider{})
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "羽毛与大灭绝", lookupArticle)
	if code, _ := explainLookup(t, h, cookie, id, "b1", ""); code != http.StatusBadRequest {
		t.Errorf("没带词：HTTP %d，want 400", code)
	}
}

// 讲整段的工具仍然把多余的文字当没带（原来那条理由照旧成立）。
func TestWholeParagraphToolIgnoresAStraySentence(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("这一段的译文。"))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "羽毛与大灭绝", lookupArticle)
	body := `{"tool":"translate","sentence":"plants were unable to photosynthesize"}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+id+"/blocks/b1/explain", strings.NewReader(body)), cookie))
	var out struct {
		Subject string `json:"subject"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if rec.Code != http.StatusOK || out.Subject != "" {
		t.Errorf("翻译：HTTP %d subject=%q，want 200 且 subject 为空", rec.Code, out.Subject)
	}
}
