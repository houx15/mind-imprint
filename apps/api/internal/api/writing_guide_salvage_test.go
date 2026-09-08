package api

// writing_guide_salvage_test.go —— 段落那一屏的引导，一批只到了一半时怎么办。
//
// 2026-09-08 的全链路走查抓到的：她从兴趣树进写作间，谈完结构走到「段落」，
// 三块都摆着「获取引导」，屏幕上一条红字：
//
//	后台错误：AI 响应错误（model_unavailable）
//	WARN writing block guide batch: reply unparseable
//
// 这是同一个毛病的第四次（星图 / 带读 / 排读法 / 这里）：模型正常收尾，JSON 却
// 停在某一块中间。**这一批尤其疼** —— 那一屏上每一块的引导都来自这一次调用，
// 整批丢掉的结果是她面前三块全空，而先写完的那两块本来是好的。
//
// 到齐的留下，没写完的当没给。没拿到引导的块照旧摆「获取引导」，那个按钮本来
// 就在，所以半批是一个界面上本来就成立的状态，不是一个新的坏状态。

import (
	"testing"

	"github.com/google/uuid"
)

// 三个块 id，其中前两块的引导写完了，第三块断在半路。
func guideBatchIDs() (a, b, c uuid.UUID, known map[uuid.UUID]bool) {
	a = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	b = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	c = uuid.MustParse("33333333-3333-3333-3333-333333333333")
	return a, b, c, map[uuid.UUID]bool{a: true, b: true, c: true}
}

// 断在第三块的 questions 数组里 —— 走查线上遇到的那一种。
func TestGuideBatchSurvivesATruncatedReply(t *testing.T) {
	a, b, _, known := guideBatchIDs()
	raw := `{"blocks":[` +
		`{"id":"` + a.String() + `","job":"立起你的主张","methodIds":[],"questions":["你最想让读者相信哪一句？"]},` +
		`{"id":"` + b.String() + `","job":"给第一条理由找一个例子","methodIds":[],"questions":["你见过哪类工厂必须连着跑？"]},` +
		`{"id":"33333333-3333-3333-3333-333333333333","job":"说清电池还能做什么","questions":["电网的波动`

	guides, ok := parseWritingGuideBatch(raw, known)
	if !ok {
		t.Fatal("整批丢掉了 —— 她那一屏三块全空，而前两块的引导是齐的")
	}
	if len(guides) != 2 {
		t.Fatalf("留下了 %d 块，应该是写完了的那 2 块", len(guides))
	}
	if g, has := guides[a]; !has || len(g.Questions) != 1 {
		t.Errorf("第一块没留下来：%+v", guides[a])
	}
	if g, has := guides[b]; !has || g.Job == "" {
		t.Errorf("第二块没留下来：%+v", guides[b])
	}
	// 断掉的那一块当没给。她那一格摆「获取引导」，按得动。
	if _, has := guides[uuid.MustParse("33333333-3333-3333-3333-333333333333")]; has {
		t.Error("断在半路的那一块被当成写完了")
	}
}

// 救援不许放宽校验：问号过滤、以及「块 id 必须是这次真的缺引导的那几块」，
// 两条在救回来的块上照样要跑。
func TestGuideBatchSalvageStillFilters(t *testing.T) {
	a, _, _, _ := guideBatchIDs()
	onlyA := map[uuid.UUID]bool{a: true}
	raw := `{"blocks":[` +
		// 不是这次缺引导的块 —— 丢掉。
		`{"id":"44444444-4444-4444-4444-444444444444","job":"x","questions":["这算一个问题吗？"]},` +
		// 问题不带问号 —— 过滤光了，这一块也就没了。
		`{"id":"` + a.String() + `","job":"y","questions":["这是一句陈述"]},` +
		`{"id":"55555555`

	guides, ok := parseWritingGuideBatch(raw, onlyA)
	if ok && len(guides) != 0 {
		t.Fatalf("救援放宽了校验，留下了 %d 块：%+v", len(guides), guides)
	}
}

// 完整的一份回复不能因为多了一层救援就走样 —— 守住原来那条路。
func TestGuideBatchWholeReplyUnchanged(t *testing.T) {
	a, b, _, known := guideBatchIDs()
	raw := `{"blocks":[` +
		`{"id":"` + a.String() + `","job":"立起你的主张","methodIds":[],"questions":["你最想让读者相信哪一句？"]},` +
		`{"id":"` + b.String() + `","job":"找一个例子","methodIds":[],"questions":["你见过哪类工厂？"]}]}`

	guides, ok := parseWritingGuideBatch(raw, known)
	if !ok || len(guides) != 2 {
		t.Fatalf("完整的回复读出了 %d 块（ok=%v）", len(guides), ok)
	}
}

// 一个字都读不出来的回复仍然要失败 —— 救援不是「什么都算数」。
func TestGuideBatchStillFailsOnGarbage(t *testing.T) {
	_, _, _, known := guideBatchIDs()
	for _, raw := range []string{"", "对不起，我不太明白你的意思。", `{"blocks":`, `{"blocks":[{"id":`} {
		if _, ok := parseWritingGuideBatch(raw, known); ok {
			t.Errorf("这份回复不该被当成可用的：%q", raw)
		}
	}
}

// 一份写完了、但一块都没给的回复不是失败 —— 它是「这一批我没有可说的」，
// 原来就是这个行为（返回空 map + true），救援不许把它改成 502。
func TestGuideBatchEmptyButWellFormedIsNotAFailure(t *testing.T) {
	_, _, _, known := guideBatchIDs()
	guides, ok := parseWritingGuideBatch(`{"blocks":[]}`, known)
	if !ok {
		t.Fatal("写完了但一块都没给，被当成了解析失败")
	}
	if len(guides) != 0 {
		t.Fatalf("凭空多出了 %d 块", len(guides))
	}
}
