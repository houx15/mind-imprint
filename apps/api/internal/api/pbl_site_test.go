package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

/* ── helpers ──────────────────────────────────────────────────────────── */

func siteReq(t *testing.T, h http.Handler, c *http.Cookie, method, url, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, url, nil)
	} else {
		r = httptest.NewRequest(method, url, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	// c == nil 是这个 helper 存在的一半理由：公开页面必须能在**完全没有 session**
	// 的情况下打开——一个家长扫码进来时就是这样。
	if c != nil {
		r = withCookie(r, c)
	}
	h.ServeHTTP(rec, r)
	return rec
}

func decodeSite(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	return out
}

// theWordsSheWrote 是能通过发布门槛的最小一份内容：她是谁、这一页在问什么、
// 关于她自己的那段话。
const theWordsSheWrote = `{
  "headline": "一件还能修的东西，是谁决定它该被扔的？",
  "role": "读 IB 的高二学生",
  "about": ["我在拆家里所有还能拆的东西，然后写为什么它们修不好。"]
}`

// openSiteGate 走一遍她真实要走的路：写字 → 挑版式并说明理由 → 发布。
// 走完 §4 那道门才开。
func openSiteGate(t *testing.T, h http.Handler, c *http.Cookie) string {
	t.Helper()
	if rec := siteReq(t, h, c, "PUT", "/api/v1/pbl/site/content", theWordsSheWrote); rec.Code != http.StatusOK {
		t.Fatalf("写内容 = %d; body=%s", rec.Code, rec.Body)
	}
	if rec := siteReq(t, h, c, "PUT", "/api/v1/pbl/site/layout",
		`{"layout":"ledger","why":"我做的东西比我写的字多，索引式一屏能看到十几条。"}`); rec.Code != http.StatusOK {
		t.Fatalf("挑版式 = %d; body=%s", rec.Code, rec.Body)
	}
	rec := siteReq(t, h, c, "POST", "/api/v1/pbl/site/publish", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("发布 = %d; body=%s", rec.Code, rec.Body)
	}
	url, _ := decodeSite(t, rec)["url"].(string)
	if url == "" {
		t.Fatal("发布成功却没给出链接")
	}
	return url
}

func tokenOf(url string) string {
	i := strings.LastIndex(url, "/p/")
	if i < 0 {
		return ""
	}
	return url[i+3:]
}

/* ── §4 的那道门 ──────────────────────────────────────────────────────── */

// 🚨 这个文件里最重要的一条。
//
// spec §4 说第一个项目就是做她自己的主页，产品负责人 2026-09-03 把它定成一道
// 完整的门。在这之前这条规则只以注释和一句灰字提示存在过——所以从来没有人触发
// 过它。这条测试是那道门本身。
func TestSiteGate_AFreeProjectWaitsUntilHerPageIsLive(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)

	rec := postPblProject(t, h, cookie, "我们学校每天剩好多饭，我想弄明白这些饭去哪了。")
	if rec.Code != http.StatusConflict {
		t.Fatalf("主页还没发布，自由项目却建成了：status = %d; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "主页") {
		t.Errorf("拦住了，但没说清为什么：%s", rec.Body)
	}

	openSiteGate(t, h, cookie)

	rec = postPblProject(t, h, cookie, "我们学校每天剩好多饭，我想弄明白这些饭去哪了。")
	if rec.Code != http.StatusCreated {
		t.Fatalf("主页发布之后门还关着：status = %d; body=%s", rec.Code, rec.Body)
	}
}

// 门关着的时候，主页项目本身必须走得通——否则这不是一道门，是一堵墙。
func TestSiteGate_TheWebsiteProjectIsTheWayThrough(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)

	rec := siteReq(t, h, cookie, "POST", "/api/v1/pbl/site/project", "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("主页项目开不了：status = %d; body=%s", rec.Code, rec.Body)
	}
	first := decodePblProject(t, rec)
	if first["kind"] != "website" {
		t.Errorf("kind = %v, 想要 website —— 这一个项目的类别由 §4 定死，不由她填", first["kind"])
	}

	// 幂等：页面是单数的，项目也应该是。
	rec = siteReq(t, h, cookie, "POST", "/api/v1/pbl/site/project", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("第二次 = %d, 想要 200（回到原来那个）; body=%s", rec.Code, rec.Body)
	}
	if again := decodePblProject(t, rec); again["id"] != first["id"] {
		t.Errorf("建了第二个主页项目：%v ≠ %v", again["id"], first["id"])
	}
}

/* ── 发布门槛 ─────────────────────────────────────────────────────────── */

// 🚨 原型最严重的缺陷的解药：一个一个字都不是她的页面，发不出去。
//
// 原型能发布，是因为空的地方全被示例内容填满了，看上去是满的。这一版没有示例
// 内容，所以空的地方是空的，而空的地方拦住发布。
func TestPublishSite_RefusesAPageWithNoWordsOfHers(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)

	rec := siteReq(t, h, cookie, "POST", "/api/v1/pbl/site/publish", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("一个字都没写就发布出去了：status = %d; body=%s", rec.Code, rec.Body)
	}
	for _, want := range []string{"首屏那句话", "关于你自己的那段话"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("没说清缺什么，缺 %q：%s", want, rec.Body)
		}
	}
}

// 挑版式必须写理由。设计原则：「一旦『就用这个』自己能按下去，这就是一台负责
// 生成、而她只负责点头的机器。」
func TestSiteLayout_DoesNotSettleWithoutAReason(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)

	rec := siteReq(t, h, cookie, "PUT", "/api/v1/pbl/site/layout", `{"layout":"essay","why":"  "}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("没写理由也选定了版式：status = %d; body=%s", rec.Code, rec.Body)
	}
	rec = siteReq(t, h, cookie, "PUT", "/api/v1/pbl/site/layout", `{"layout":"blog","why":"随便"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("不存在的版式被接受了：status = %d", rec.Code)
	}
}

// 写了字、但没挑版式，也发不出去。
func TestPublishSite_RefusesBeforeSheHasChosenALayout(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	if rec := siteReq(t, h, cookie, "PUT", "/api/v1/pbl/site/content", theWordsSheWrote); rec.Code != http.StatusOK {
		t.Fatalf("写内容 = %d; body=%s", rec.Code, rec.Body)
	}
	rec := siteReq(t, h, cookie, "POST", "/api/v1/pbl/site/publish", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("没挑版式就发布了：status = %d; body=%s", rec.Code, rec.Body)
	}
}

// 再发一次给的是**同一条**链接。铸一个新的会让她已经发出去的链接悄悄失效。
func TestPublishSite_ResharingKeepsTheSameLink(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	first := openSiteGate(t, h, cookie)

	rec := siteReq(t, h, cookie, "POST", "/api/v1/pbl/site/publish", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("再发一次 = %d; body=%s", rec.Code, rec.Body)
	}
	if again, _ := decodeSite(t, rec)["url"].(string); again != first {
		t.Errorf("重新发布换了链接：%s → %s", first, again)
	}
}

/* ── 公开的那一面 ─────────────────────────────────────────────────────── */

// 页面上的名字是她的名字，从 users.display_name 来。
//
// 这条测试盯的是那个具体的 bug：原型里 STUDENT = { name: "林知遥" } 是个模块
// 常量，于是每个学生建出来的都是同一个人的主页。
func TestPublicSite_CarriesHerNameNotAFixture(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	url := openSiteGate(t, h, cookie)

	rec := siteReq(t, h, nil, "GET", "/api/v1/public/sites/"+tokenOf(url), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("公开页打不开：status = %d; body=%s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	content, _ := decodeSite(t, rec)["content"].(map[string]any)
	if content["name"] != "Phoebe" {
		t.Errorf("name = %v, 想要 Phoebe（种子学生的 display_name）", content["name"])
	}
	if content["headline"] != "一件还能修的东西，是谁决定它该被扔的？" {
		t.Errorf("她写的那句话没上页面：%v", content["headline"])
	}
	// 原型里那些人的痕迹，一个都不该出现。
	for _, ghost := range []string{"林知遥", "zhiyao", "初二", "台灯", "213"} {
		if strings.Contains(body, ghost) {
			t.Errorf("公开页上出现了别人的内容：%q", ghost)
		}
	}
}

// 公开载荷只有这一页。多漏一个字段，就是对所有人永远地漏。
func TestPublicSite_PayloadCarriesNothingExtra(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	url := openSiteGate(t, h, cookie)

	rec := siteReq(t, h, nil, "GET", "/api/v1/public/sites/"+tokenOf(url), "")
	out := decodeSite(t, rec)

	keys := make([]string, 0, len(out))
	for k := range out {
		keys = append(keys, k)
	}
	if len(keys) != 2 || out["content"] == nil || out["layout"] == nil {
		t.Fatalf("顶层字段变了：%v —— 公开端点的载荷是被钉住的", keys)
	}
	body := rec.Body.String()
	// 账号、学校、token 本身、以及任何能寻址到她别的东西的 id，都不该在里面。
	for _, leak := range []string{
		"phoebe@demo.mindimprint.local", "school", "user_id", "userId",
		"00000000-0000-0000-0000-000000000003", tokenOf(url),
	} {
		if strings.Contains(body, leak) {
			t.Errorf("公开载荷里漏了 %q", leak)
		}
	}
}

// 不可索引。她是未成年人，这条链接是给人的，不是给搜索引擎的。
func TestPublicSite_IsNotIndexable(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	url := openSiteGate(t, h, cookie)

	rec := siteReq(t, h, nil, "GET", "/api/v1/public/sites/"+tokenOf(url), "")
	if got := rec.Header().Get("X-Robots-Tag"); !strings.Contains(got, "noindex") {
		t.Fatalf("X-Robots-Tag = %q, 想要带 noindex", got)
	}
}

// 撤销即时且彻底，而且撤销过的地址和从来不存在的地址长得一模一样。
func TestPublicSite_RevokedAndUnknownAreIndistinguishable(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	url := openSiteGate(t, h, cookie)
	token := tokenOf(url)

	if rec := siteReq(t, h, cookie, "DELETE", "/api/v1/pbl/site/publish", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("撤销 = %d; body=%s", rec.Code, rec.Body)
	}

	revoked := siteReq(t, h, nil, "GET", "/api/v1/public/sites/"+token, "")
	unknown := siteReq(t, h, nil, "GET", "/api/v1/public/sites/deadbeefdeadbeefdeadbeefdeadbeef", "")
	if revoked.Code != http.StatusNotFound {
		t.Fatalf("撤销之后还打得开：status = %d", revoked.Code)
	}
	if revoked.Code != unknown.Code || revoked.Body.String() != unknown.Body.String() {
		t.Errorf("撤销过的地址和不存在的地址能被区分开：%d %s ≠ %d %s",
			revoked.Code, revoked.Body, unknown.Code, unknown.Body)
	}

	// 撤销之后 §4 那道门重新关上——线上没有页面，就是没有页面。
	if rec := postPblProject(t, h, cookie, "另一个想法"); rec.Code != http.StatusConflict {
		t.Errorf("页面撤下来了，门却还开着：status = %d", rec.Code)
	}
}

// 撤销是幂等的：从来没发布过也返回 204。
func TestRevokeSite_IsIdempotent(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	if rec := siteReq(t, h, cookie, "DELETE", "/api/v1/pbl/site/publish", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("撤销一个从没发布过的主页 = %d; body=%s", rec.Code, rec.Body)
	}
}

/* ── 她自己那一侧 ─────────────────────────────────────────────────────── */

// 她自己看的时候，页面告诉她还缺什么——而不是给她一个满的假页面。
func TestGetSite_TellsHerWhatIsStillMissing(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)

	rec := siteReq(t, h, cookie, "GET", "/api/v1/pbl/site", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body)
	}
	out := decodeSite(t, rec)
	missing, _ := out["missing"].([]any)
	if len(missing) == 0 {
		t.Fatal("一个字都没写，却说这一页已经齐了")
	}
	if out["published"] != false {
		t.Errorf("published = %v, 想要 false", out["published"])
	}
	content, _ := out["content"].(map[string]any)
	if content["name"] != "Phoebe" {
		t.Errorf("name = %v, 想要 Phoebe", content["name"])
	}
	// 空页面就是空的，不该有兜底文案。
	if s, _ := content["headline"].(string); s != "" {
		t.Errorf("空页面长出了一句 headline：%q", s)
	}
}
