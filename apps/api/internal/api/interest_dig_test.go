package api_test

// interest_dig_test.go — 「继续深挖」端点的库级测试。
//
// 三件事：归属（别人的词一律 404）、失败时不摆通用动词、以及第四次出现的
// 「盖章在生成之前」。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

type digResp struct {
	KeywordID string `json:"keywordId"`
	Note      string `json:"note"`
	Seeds     []struct {
		Kind string `json:"kind"`
		Text string `json:"text"`
		Why  string `json:"why"`
	} `json:"seeds"`
}

func getDig(h http.Handler, cookie *http.Cookie, id string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/interest/keywords/"+id+"/dig", nil)
	h.ServeHTTP(rec, withCookie(req, cookie))
	return rec
}

func decodeDig(t *testing.T, rec *httptest.ResponseRecorder) digResp {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("GET dig = %d; body=%s", rec.Code, rec.Body)
	}
	var out digResp
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rec.Body)
	}
	return out
}

// 🚨 没有模型通道时（测试就是），这里必须是**零颗种子 + 一句原话**，
// 绝不回退到「再读一篇 / 写一篇 / 做个项目 / 问印记」那四个通用动词 ——
// 那正是这套东西要取代的东西。
func TestKeywordDig_NoGenericVerbsWhenGenerationFails(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	id := seedKeyword(t, pool, "climate-ocean")

	got := decodeDig(t, getDig(h, cookie, id))
	if len(got.Seeds) != 0 {
		t.Fatalf("没有模型通道却给了 %d 颗种子：%+v", len(got.Seeds), got.Seeds)
	}
	if got.Note == "" {
		t.Error("没有说出为什么没有种子")
	}
}

// 第四次的同一条教训（0104 / 0116 / 0118 / 0119）。章盖在生成之前，语义是
// 「这个词试过一次」——按产出判断的话，她每次打开这个抽屉都会重发一次调用。
func TestKeywordDig_StampsEvenWhenNothingWasGenerated(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	id := seedKeyword(t, pool, "climate-ocean")

	getDig(h, cookie, id)

	var stamped bool
	if err := pool.QueryRow(t.Context(),
		`SELECT dig_at IS NOT NULL FROM interest_keyword WHERE id = $1`, mustUUID(id)).Scan(&stamped); err != nil {
		t.Fatalf("query: %v", err)
	}
	if !stamped {
		t.Fatal("零产出的一次生成没有盖章 —— 每次打开抽屉都会重发一次调用")
	}
}

// 归属：别人的词一律 404，而不是「存在但看不了」。
func TestKeywordDig_SomeoneElsesKeywordIs404(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	id := seedKeyword(t, pool, "climate-ocean")

	if _, err := pool.Exec(t.Context(), `
		UPDATE interest_keyword SET user_id = (
			SELECT id FROM users WHERE id <> interest_keyword.user_id LIMIT 1
		) WHERE id = $1`, mustUUID(id)); err != nil {
		t.Fatalf("reassign: %v", err)
	}
	if rec := getDig(h, cookie, id); rec.Code != http.StatusNotFound {
		t.Errorf("别人的词得到 %d，want 404", rec.Code)
	}
}

func TestKeywordDig_MalformedIDIs400(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	if rec := getDig(h, cookie, "not-a-uuid"); rec.Code != http.StatusBadRequest {
		t.Errorf("坏 id 得到 %d，want 400", rec.Code)
	}
}

// 存过的种子直接读出来，并且**按固定顺序**（想一想 → 去读 → 去写 → 去做）。
// 顺序不稳的话，同一个抽屉两次打开会把四颗种子换个位置，那看起来像是内容变了。
func TestKeywordDig_ReturnsStoredSeedsInAStableOrder(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	id := seedKeyword(t, pool, "climate-ocean")

	// 故意乱序插入。
	for _, s := range []struct{ kind, text string }{
		{"make", "记录一个月水温"}, {"think", "四平方公里凭什么代表一整片海？"},
		{"write", "一个避难所不是一个计划"}, {"read", "珊瑚白化是死了还是吐藻"},
	} {
		if _, err := pool.Exec(t.Context(),
			`INSERT INTO keyword_dig (keyword_id, kind, text) VALUES ($1,$2,$3)`,
			mustUUID(id), s.kind, s.text); err != nil {
			t.Fatalf("seed dig: %v", err)
		}
	}
	if _, err := pool.Exec(t.Context(),
		`UPDATE interest_keyword SET dig_at = now() WHERE id = $1`, mustUUID(id)); err != nil {
		t.Fatalf("stamp: %v", err)
	}

	got := decodeDig(t, getDig(h, cookie, id))
	kinds := make([]string, 0, 4)
	for _, s := range got.Seeds {
		kinds = append(kinds, s.Kind)
	}
	want := []string{"think", "read", "write", "make"}
	for i := range want {
		if i >= len(kinds) || kinds[i] != want[i] {
			t.Fatalf("顺序不对：%v，want %v", kinds, want)
		}
	}
	if got.Note != "" {
		t.Errorf("有种子时不该还带一句解释：%q", got.Note)
	}
}

var _ = pgxpool.Pool{}
