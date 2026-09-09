package api

import (
	"testing"

	"mindimprint/api/internal/library"
)

// 这条测试守着分级阅读库里最容易静静坏掉的那件事。
//
// 图不在正文里，每张记着自己跟在哪一段之后（"b7"）；小标题也是段，记的是段 id。
// 这些 id 由 deploy/reading-library/parse.py 用 Python 重算了一遍 SplitBlocks
// 的规则算出来（按空行切、trim、丢空段、给活下来的编号）。两边只要有一处对
// 「什么算一段」的看法不同 —— 一个多余的空行、一次 trim 的差别 —— 全库的图就
// 会集体移位到错误的段落，而且页面照常渲染，没有任何报错。
//
// 所以这里拿真的 SplitBlocks（阅读室实际用的那个）把每一档正文重切一遍，逐条
// 核对锚点。这个包是 library 的下游，所以测试放在这边而不是那边。
func TestEveryFigureAnchorResolvesAgainstSplitBlocks(t *testing.T) {
	if err := library.LoadErr(); err != nil {
		t.Fatalf("库没加载: %v", err)
	}
	figures := 0
	for _, a := range library.All() {
		for _, l := range a.Levels {
			ids := map[string]bool{}
			for _, b := range SplitBlocks(l.Body) {
				ids[b.ID] = true
			}
			if len(ids) == 0 {
				t.Errorf("%s %s: 正文切不出一段", a.Slug, l.Name)
			}
			for _, f := range l.Figures {
				figures++
				// 空串是题图，站在第一段之前，本来就没有可对的段 id。
				if f.After == "" {
					continue
				}
				if !ids[f.After] {
					t.Errorf("%s %s: 图 %s 挂在 %q 上，正文里没有这一段",
						a.Slug, l.Name, f.Key, f.After)
				}
			}
			for _, h := range l.Headings {
				if !ids[h] {
					t.Errorf("%s %s: 小标题指向 %q，正文里没有这一段", a.Slug, l.Name, h)
				}
			}
		}
	}
	if figures == 0 {
		t.Fatal("一张图都没有 —— 这条测试等于没跑")
	}
}

// 正文里不该留下 Markdown 的痕迹。SplitBlocks 不认识 Markdown，一行 "## The
// Mission" 会原样出现在页面上，一行 "![](…)" 会占掉一个段 id 然后被当成课文
// 引回给学生。
func TestLibraryBodiesCarryNoMarkdown(t *testing.T) {
	for _, a := range library.All() {
		for _, l := range a.Levels {
			for _, b := range SplitBlocks(l.Body) {
				if len(b.Text) >= 2 && b.Text[:2] == "##" {
					t.Errorf("%s %s %s: 正文里留着 Markdown 小标题: %.40s", a.Slug, l.Name, b.ID, b.Text)
				}
				if len(b.Text) >= 2 && b.Text[:2] == "![" {
					t.Errorf("%s %s %s: 正文里留着一张图: %.40s", a.Slug, l.Name, b.ID, b.Text)
				}
			}
		}
	}
}
