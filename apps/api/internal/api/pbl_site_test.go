package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

// openSiteGate 走一遍她真实要走的路：写字 → 定版式与配色 → 发布。
// 走完 §4 那道门才开。
//
// 🚨 这个 helper 是整个 PBL 测试面的地基（85 个测试直接或间接走它），所以它
// 一旦对不上真实路由，红的不是一个测试而是一整片——而且症状是
// 「挑版式 = 404」，看起来像权限或路由注册坏了，跟真正的原因（第三关改版）
// 差得很远。2026-09-04 就是这样：
//
// 旧的 `PUT /pbl/site/layout` 要她挑一个版式**并写一句理由**，是 SiteStudio
// 那张表单里的一格。第三关（配色 + 风格 + 头图，全做成判断题）把它换成了
// `PUT /pbl/site/look`：版式和配色一起定，不再要 `why`。api.go 那一行的注释
// 写着「取代了旧的 PUT /pbl/site/layout」，但这个 helper 没跟着改。
func openSiteGate(t *testing.T, h http.Handler, c *http.Cookie) string {
	t.Helper()
	if rec := siteReq(t, h, c, "PUT", "/api/v1/pbl/site/content", theWordsSheWrote); rec.Code != http.StatusOK {
		t.Fatalf("写内容 = %d; body=%s", rec.Code, rec.Body)
	}
	// 三个颜色都必须是 #RRGGBB —— putPblSiteLook 会验（一个 "warm beige" 存进去
	// 之后浏览器会把整条 CSS 声明丢掉，坏的是她已经发布出去的那一页）。
	if rec := siteReq(t, h, c, "PUT", "/api/v1/pbl/site/look",
		`{"layout":"ledger","palette":{"label":"工作台","why":"配「拆东西」这个词",`+
			`"paper":"#F5F1E8","ink":"#1F1B16","accent":"#B4552D"}}`); rec.Code != http.StatusOK {
		t.Fatalf("定版式与配色 = %d; body=%s", rec.Code, rec.Body)
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

// 定版式与配色时，两样都要验得住。
//
// ⚠️ 这条测试原本叫 `TestSiteLayout_DoesNotSettleWithoutAReason`，钉的是旧
// `PUT /pbl/site/layout` 的规矩：**挑版式必须写一句理由**，理由是
// 「一旦『就用这个』自己能按下去，这就是一台负责生成、而她只负责点头的机器」。
//
// 第三关（配色 + 风格 + 头图，全做成判断题）把那一步整个换掉了：新的
// `PUT /pbl/site/look` 一次定版式和配色，**没有 `why` 这个字段**——api.go 上
// 那行注释把这次替换写得很清楚。所以「没写理由就不能落定」不再是产品的规矩，
// 继续断言它就是在钉一条已经被推翻的设计。
//
// 留下来的是这个端点真正还担保的两件事，而且第二件正是她自己发现不了的那种：
// 一个不是 #RRGGBB 的颜色存进去之后，浏览器会把整条 CSS 声明丢掉，坏掉的是
// 她**已经发布出去**的那一页。
func TestSiteLook_RefusesAnUnknownLayoutOrABrokenPalette(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	const goodPalette = `"palette":{"label":"工作台","why":"配「拆东西」这个词",` +
		`"paper":"#F5F1E8","ink":"#1F1B16","accent":"#B4552D"}`

	rec := siteReq(t, h, cookie, "PUT", "/api/v1/pbl/site/look", `{"layout":"blog",`+goodPalette+`}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("不存在的版式被接受了：status = %d; body=%s", rec.Code, rec.Body)
	}
	rec = siteReq(t, h, cookie, "PUT", "/api/v1/pbl/site/look",
		`{"layout":"ledger","palette":{"label":"暖","why":"暖","paper":"warm beige","ink":"#1F1B16","accent":"#B4552D"}}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("不是 #RRGGBB 的颜色被接受了：status = %d; body=%s", rec.Code, rec.Body)
	}
	// 两样都对 → 落定。
	if rec := siteReq(t, h, cookie, "PUT", "/api/v1/pbl/site/look",
		`{"layout":"ledger",`+goodPalette+`}`); rec.Code != http.StatusOK {
		t.Fatalf("版式和配色都合法却没落定：status = %d; body=%s", rec.Code, rec.Body)
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

	// 🚨 允许清单，不是数个数。
	//
	// 这条测试要防的是**漏**，而按 `len(keys) != 2` 写的话，任何一个合法的新
	// 字段都会让它红——2026-09-04 就是这样：第三关给她的主页加了配色和头图，
	// 两样都是她自己的、本来就该出现在她公开页上的东西，而这条测试报的是
	// 「顶层字段变了」，读起来像是漏了什么。
	//
	// （同一个教训在 atom_report_share_test.go 里已经吃过一次，那边的注释写着：
	// 「pinning the count made adding a legitimate report field look like a
	// leak」。这里照它的形状改。）
	//
	// 真正的安全断言是下面那一组 leak 检查：它们钉的是**不该出现的东西**，
	// 和这一页长出多少个字段无关。
	allowed := map[string]bool{
		"content": true, "layout": true,
		// 第三关：她定的配色和她生成的头图，都是这一页要拿来渲染的。
		"palette": true, "heroUrl": true,
	}
	keys := make([]string, 0, len(out))
	for k := range out {
		keys = append(keys, k)
		if !allowed[k] {
			t.Errorf("公开载荷多了一个字段 %q —— 多漏一个字段，就是对所有人永远地漏", k)
		}
	}
	for _, required := range []string{"content", "layout"} {
		if out[required] == nil {
			t.Fatalf("公开载荷缺了 %q：%v", required, keys)
		}
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

// 🚨 草稿里的列表永远是 `[]`，绝不是 `null`。
//
// Go 的 nil 切片 marshal 出来是 `null`，前端一句 `draft.motto.filter(...)` 就把
// 整个主页工作面打成白屏。这个 bug 真的发生过，而且 456 条前端单测一条都没响
// ——是 2026-09-03 的浏览器 walk 抓到的。它是接口形状的约定，所以钉在这里。
func TestGetSite_ListsAreNeverNull(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)

	rec := siteReq(t, h, cookie, "GET", "/api/v1/pbl/site", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	for _, field := range []string{"motto", "tags", "about", "nowList"} {
		if strings.Contains(body, `"`+field+`":null`) {
			t.Errorf("%s 是 null，前端会在 .filter() 上崩掉：%s", field, body)
		}
	}
	if strings.Contains(body, `"blurbs":null`) {
		t.Errorf("blurbs 是 null：%s", body)
	}
}

// Reproduce pre-routine accounts without changing their existing project or page.
func TestSiteProject_RepairsLegacyState(t *testing.T) {
	for _, missingSite := range []bool{true, false} {
		t.Run(map[bool]string{true: "missing site", false: "missing association"}[missingSite], func(t *testing.T) {
			h, c, _, pool := liteHandler(t)
			first := siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", "")
			id := decodePblProject(t, first)["id"].(string)
			ctx := context.Background()
			if _, err := pool.Exec(ctx, `DELETE FROM pbl_plan_version WHERE atom_id=$1`, id); err != nil {
				t.Fatal(err)
			}
			if missingSite {
				if _, err := pool.Exec(ctx, `DELETE FROM pbl_site WHERE atom_id=$1`, id); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := pool.Exec(ctx, `UPDATE pbl_site SET atom_id=NULL, content='{"headline":"keep my words"}' WHERE atom_id=$1`, id); err != nil {
					t.Fatal(err)
				}
			}
			for i := 0; i < 2; i++ {
				rec := siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", "")
				if rec.Code != http.StatusOK || decodePblProject(t, rec)["id"] != id {
					t.Fatalf("resume: %d %s", rec.Code, rec.Body)
				}
			}
			var versions, steps, sites int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM pbl_plan_version WHERE atom_id=$1`, id).Scan(&versions); err != nil {
				t.Fatal(err)
			}
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM pbl_plan_step WHERE version_id IN (SELECT id FROM pbl_plan_version WHERE atom_id=$1)`, id).Scan(&steps); err != nil {
				t.Fatal(err)
			}
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM pbl_site WHERE atom_id=$1`, id).Scan(&sites); err != nil {
				t.Fatal(err)
			}
			if versions != 1 || steps != 5 || sites != 1 {
				t.Fatalf("versions=%d steps=%d sites=%d", versions, steps, sites)
			}
			if !missingSite {
				var headline string
				if err := pool.QueryRow(ctx, `SELECT content->>'headline' FROM pbl_site WHERE atom_id=$1`, id).Scan(&headline); err != nil {
					t.Fatal(err)
				}
				if headline != "keep my words" {
					t.Fatal("lost existing content")
				}
			}
		})
	}
}

func TestSiteProject_PreservesCanonicalProjectAndPlan(t *testing.T) {
	h, c, _, pool := liteHandler(t)
	id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	ctx := context.Background()
	// An older unrelated website must not replace the explicitly associated homepage.
	_, err := pool.Exec(ctx, `WITH older AS (
 INSERT INTO atom (kind,user_id,created_at) SELECT 'project',user_id,created_at-interval '1 day' FROM atom WHERE id=$1 RETURNING id
 ) INSERT INTO pbl_project(atom_id,idea,kind,name) SELECT id,'legacy idea','website','legacy' FROM older`, id)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `UPDATE pbl_plan_version SET summary='student plan', approved_at=now() WHERE atom_id=$1`, id)
	if err != nil {
		t.Fatal(err)
	}
	rec := siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", "")
	if rec.Code != http.StatusOK || decodePblProject(t, rec)["id"] != id {
		t.Fatalf("wrong homepage: %s", rec.Body)
	}
	var summary string
	if err := pool.QueryRow(ctx, `SELECT summary FROM pbl_plan_version WHERE atom_id=$1`, id).Scan(&summary); err != nil {
		t.Fatal(err)
	}
	if summary != "student plan" {
		t.Fatal("replaced existing plan")
	}
}

func TestSiteProject_ConcurrentStartsShareOneProject(t *testing.T) {
	h, c, _, pool := liteHandler(t)
	const requests = 6
	results := make(chan *httptest.ResponseRecorder, requests)
	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", "") }()
	}
	wg.Wait()
	close(results)
	id := ""
	created := 0
	for rec := range results {
		if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
			t.Fatalf("start: %d %s", rec.Code, rec.Body)
		}
		if rec.Code == http.StatusCreated {
			created++
		}
		got := decodePblProject(t, rec)["id"].(string)
		if id != "" && got != id {
			t.Fatalf("duplicate project: %s != %s", got, id)
		}
		id = got
	}
	if created != 1 {
		t.Fatalf("created=%d, want one", created)
	}
	var plans int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM pbl_plan_version WHERE atom_id=$1`, id).Scan(&plans); err != nil {
		t.Fatal(err)
	}
	if plans != 1 {
		t.Fatalf("plans=%d", plans)
	}
}
