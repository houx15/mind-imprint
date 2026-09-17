package materialize

import (
	"strings"
	"testing"
)

// 产品负责人 2026-09-17：「有时候我们抓取的文章最后会有：You Might Also Like。这个及
// 之后的内容其实可以不要，不然会干扰正常阅读。」用例照线上那两篇 Smithsonian 的形状。

func TestCutRelatedTrailer(t *testing.T) {
	body := strings.Join([]string{
		"Researchers mapped every nerve cell in the fly's brain.",
		"The map took years of imaging and careful tracing.",
		"It may help explain how simple circuits produce behavior.",
		"You Might Also Like",
		"This New Brain Map Shows Every Nerve Cell. Here's Why That Matters September 16, 2026",
		"Three Wild Dogs Made a Record-Breaking Trek Across Zambia September 16, 2026",
	}, "\n\n")
	got := CutRelatedTrailer(body)
	if strings.Contains(got, "You Might Also Like") || strings.Contains(got, "Wild Dogs") {
		t.Errorf("推荐阅读那一栏没切掉：%q", got)
	}
	if !strings.HasSuffix(got, "produce behavior.") {
		t.Errorf("正文被切多了或切少了：%q", got)
	}
}

func TestCutRelatedTrailerVariants(t *testing.T) {
	for _, head := range []string{"YOU MAY ALSO LIKE", "Related Articles:", "  Read Next  ", "推荐阅读", "相关阅读："} {
		body := "第一段正文。\n\n第二段正文。\n\n" + head + "\n\n别的文章的标题"
		if got := CutRelatedTrailer(body); got != "第一段正文。\n\n第二段正文。" {
			t.Errorf("%q 没认出来：%q", head, got)
		}
	}
}

// Quanta 的页尾（2026-09-17 入口走查）：订阅提示 → 「Also in Biology」→ 几个标题 →
// 评论须知 → 「Next article」。
func TestCutRelatedTrailerQuantaFooter(t *testing.T) {
	body := strings.Join([]string{
		"A round 700 million years ago, a group of organisms resembling glowing blobs split off.",
		"Over the past decade, ctenophores have helped answer long-standing questions.",
		"C. veneris uses cilia and muscular undulation to glide through the water column.",
		"Get highlights of the most important news delivered to your email inbox",
		"Also in Biology",
		"Genome Duplication Is a Radical Evolutionary Gamble",
		"Comment on this article",
		"Quanta Magazine moderates comments to facilitate an informed conversation.",
		"Next article",
	}, "\n\n")
	got := CutRelatedTrailer(body)
	if !strings.HasSuffix(got, "through the water column.") {
		t.Errorf("页尾没切干净：%q", got)
	}
	// 正文里的「also in」句子不动。
	fine := "One.\n\nTwo.\n\nThe same pattern appears also in birds and bats, the authors note.\n\nThree."
	if CutRelatedTrailer(fine) != fine {
		t.Error("正文里带 also in 的句子被当成栏目标题了")
	}
}

// 🚨 只认「整段就是这句话」，而且前面至少有两段正文。
func TestCutRelatedTrailerLeavesRealTextAlone(t *testing.T) {
	cases := []string{
		// 正文里的一句话，不是栏目标题。
		"First.\n\nSecond.\n\nYou might also like this approach, the author argues.\n\nThird.",
		// 太靠前：多半是导航漏进来的，切掉的会是整篇。
		"Read more\n\nThe whole article starts here.\n\nAnd continues.",
		"Intro.\n\nRelated Articles\n\nThe real body is still here.",
		// 没有尾巴。
		"Just one paragraph.",
	}
	for _, body := range cases {
		if got := CutRelatedTrailer(body); got != body {
			t.Errorf("不该动的文章被切了：\n in: %q\nout: %q", body, got)
		}
	}
}

// 走一遍真正的 HTML 抽取：栏目标题是一个普通的 <h2>，不在 <aside> 里。
func TestExtractHTMLDropsTheRecommendationBlock(t *testing.T) {
	page := `<html><head><title>Fly brain</title></head><body><article>
<p>Researchers mapped every nerve cell in the fly's central nervous system, a project that took years.</p>
<p>The map may help explain how relatively simple circuits give rise to complex behavior in animals.</p>
<p>Scientists say similar maps of larger brains are still decades away, but the method now exists.</p>
<h2>You Might Also Like</h2>
<p>Three Wild Dogs Made a Record-Breaking Trek Across Zambia September 16, 2026</p>
</article></body></html>`
	_, text := extractHTML([]byte(page))
	if strings.Contains(text, "Wild Dogs") || strings.Contains(text, "You Might Also Like") {
		t.Errorf("抽出来的正文还带着推荐阅读：%q", text)
	}
	if !strings.Contains(text, "decades away") {
		t.Errorf("正文被切坏了：%q", text)
	}
}
