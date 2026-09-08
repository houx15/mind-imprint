package news

// clean_article_test.go —— feed 的正文要清成**和她粘进来的文章一样的形状**。
//
// 取正文有三条路（feed 自带 / 抓原页面 / 她自己粘），产出必须是同一种东西：
// 纯文本，段落之间一个空行。因为后面所有事都建在段落上 —— `SplitBlocks` 按空行
// 切块，每块拿一个 id，段落工具条、挂卡片、排精读段落全靠它。
//
// 🚨 所以这里**不能**用 `clean()`：它把换行也当空白压掉，交出来的是一堵墙。
// 文章在，但阅读室的功能全没了 —— 而且这种坏法不报错，看起来只是「排版怪怪的」。

import (
	"strings"
	"testing"
)

func TestCleanArticleKeepsParagraphs(t *testing.T) {
	in := `<p>第一段说的是错位：太阳能中午最多，用电高峰在傍晚。</p>
	<p>第二段说的是代价：填这个错位要靠储能、需求响应或者跨区输电。</p>
	<p>第三段是那个让人不舒服的事实：白天多出来的电卖不掉，甚至要弃掉。</p>`

	got := cleanArticle(in)
	paras := strings.Split(got, "\n\n")
	if len(paras) != 3 {
		t.Fatalf("切出 %d 段，应该是 3 段：\n%q", len(paras), got)
	}
	if !strings.HasPrefix(paras[0], "第一段") || !strings.HasPrefix(paras[2], "第三段") {
		t.Errorf("段落顺序或内容不对：%q", paras)
	}
	if strings.Contains(got, "<p>") {
		t.Errorf("标签没清干净：%q", got)
	}
}

// 这条才是真正的判据：清出来的东西喂给 SplitBlocks（阅读室用的就是它），
// 要切得出多个块。一堵墙只会切出一块。
func TestCleanArticleSurvivesTheBlockSplitter(t *testing.T) {
	in := `<div><h2>标题也算一段</h2><p>正文第一段。</p><p>正文第二段。</p></div>`
	got := cleanArticle(in)
	if n := len(strings.Split(got, "\n\n")); n < 3 {
		t.Fatalf("只切出 %d 段，阅读室会退化成一堵墙：\n%q", n, got)
	}
	// 对照：老的 clean() 就是那堵墙 —— 记在这里，免得有人觉得「两个清洗函数
	// 太啰嗦，统一成一个」而把段落弄没。
	if wall := clean(in); strings.Contains(wall, "\n") {
		t.Errorf("clean() 居然留下了换行？那这条对照就没意义了：%q", wall)
	}
}

func TestCleanArticleHandlesBrAndEntities(t *testing.T) {
	got := cleanArticle(`一行<br>另一行<br/>第三行 &amp; 一个转义`)
	if n := len(strings.Split(got, "\n\n")); n != 3 {
		t.Errorf("<br> 没有换段：%q", got)
	}
	if !strings.Contains(got, "& 一个转义") {
		t.Errorf("实体没有反转义：%q", got)
	}
}

// 空壳段落（`<p> </p>`、连续换行）在真实 feed 里到处都是。留着会在阅读室里
// 变成一个个点不出东西的空块。
func TestCleanArticleDropsEmptyParagraphs(t *testing.T) {
	got := cleanArticle(`<p>有内容。</p><p> </p><p></p><p>也有内容。</p>`)
	for _, p := range strings.Split(got, "\n\n") {
		if strings.TrimSpace(p) == "" {
			t.Fatalf("留下了空段落：%q", got)
		}
	}
	if n := len(strings.Split(got, "\n\n")); n != 2 {
		t.Errorf("应该只剩两段，得到 %d 段：%q", n, got)
	}
}

// script / style 整段扔掉 —— 不然她读到的第一段是一堆 JavaScript。
func TestCleanArticleDropsScriptAndStyle(t *testing.T) {
	got := cleanArticle(`<script>var a = 1; document.write("x");</script><p>真正的正文。</p><style>.a{color:red}</style>`)
	if strings.Contains(got, "document.write") || strings.Contains(got, "color:red") {
		t.Fatalf("脚本或样式漏进正文了：%q", got)
	}
	if !strings.Contains(got, "真正的正文") {
		t.Errorf("正文丢了：%q", got)
	}
}

// 摘要那一栏仍然是压平的一行 —— 星球卡片上要的就是一行，不是分段的正文。
func TestSummaryStaysOneLine(t *testing.T) {
	items, err := Parse([]byte(wpFeed))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if strings.Contains(items[0].Summary, "\n") {
		t.Errorf("摘要里有换行，星球卡片上会撑开：%q", items[0].Summary)
	}
	if !strings.Contains(items[0].Body, "\n\n") {
		t.Errorf("正文没有分段：%q", items[0].Body)
	}
}
