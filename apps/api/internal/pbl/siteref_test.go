package pbl

import "testing"

/* ── 网址归一化 ───────────────────────────────────────────────────────── */

// 同一站的几种写法要归成同一个，否则去重形同虚设：她贴第二遍是手滑，而屏幕上
// 会出现两张一模一样的卡，还各花了一次模型调用。
func TestNormalizeSiteURL_FoldsTheSameSite(t *testing.T) {
	want := "https://example.com"
	for _, raw := range []string{
		"example.com",
		"https://example.com",
		"https://example.com/",
		"  https://EXAMPLE.com/  ",
		"https://example.com/#about",
	} {
		got, err := NormalizeSiteURL(raw)
		if err != nil {
			t.Fatalf("%q: %v", raw, err)
		}
		if got != want {
			t.Errorf("%q 归一化成 %q，want %q", raw, got, want)
		}
	}
}

// 路径不折。很多站的路径是大小写敏感的，折了就 404。
func TestNormalizeSiteURL_KeepsThePath(t *testing.T) {
	got, err := NormalizeSiteURL("https://example.com/Blog/Post-1")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://example.com/Blog/Post-1" {
		t.Errorf("路径被改了：%q", got)
	}
}

func TestNormalizeSiteURL_RejectsWhatIsNotAWebPage(t *testing.T) {
	for _, raw := range []string{"", "   ", "javascript:alert(1)", "file:///etc/passwd"} {
		if got, err := NormalizeSiteURL(raw); err == nil {
			t.Errorf("%q 应该被拒，实际得到 %q", raw, got)
		}
	}
}

/* ── 卡片解析 ─────────────────────────────────────────────────────────── */

func TestParseSiteRefCard_ReadsTheThreeSentences(t *testing.T) {
	got, err := ParseSiteRefCard(`{"title":"Lil'Log","what":"一个研究者写的长文站",
		"structure":"身份块 → 文章列表（带日期和标签）→ 站点信息","best":"每篇开头一段摘要"}`)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Lil'Log" || got.What == "" || got.Structure == "" || got.Best == "" {
		t.Errorf("解析结果不完整：%+v", got)
	}
}

// 模型爱在 JSON 外面包一层解释。前后都容忍，中间不猜。
func TestParseSiteRefCard_ToleratesProseAroundTheJSON(t *testing.T) {
	got, err := ParseSiteRefCard("好的，这是结果：\n{\"what\":\"a\",\"structure\":\"b\",\"best\":\"c\"}\n希望有帮助。")
	if err != nil {
		t.Fatal(err)
	}
	if got.What != "a" {
		t.Errorf("没解析出来：%+v", got)
	}
}

// 🚨 缺一句就报错，不给一张半张的卡。
//
// 一张 structure 为空的卡，在第二关的界面上是一行空白——而她正要照着那一行搭
// 自己的结构。报错她还能重贴一次；一张空卡她只会以为这一站没有结构。
// 见 memory · ai-errors-must-surface-never-fake。
func TestParseSiteRefCard_RefusesAHalfCard(t *testing.T) {
	for _, raw := range []string{
		`{"what":"a","structure":"b"}`,
		`{"what":"a","structure":"","best":"c"}`,
		`not json at all`,
	} {
		if _, err := ParseSiteRefCard(raw); err == nil {
			t.Errorf("%q 应该被拒", raw)
		}
	}
}
