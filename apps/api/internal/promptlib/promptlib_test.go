package promptlib

import "testing"

// 库本身得装得下、认得出。
func TestLibraryLoads(t *testing.T) {
	all, err := All()
	if err != nil {
		t.Fatalf("装不进来：%v", err)
	}
	if len(all) < 700 {
		t.Fatalf("只有 %d 道题 —— 源数据 705 道、拆开多选题之后 728 道，少了就是编译那一步丢了东西", len(all))
	}
	for _, p := range all {
		if p.ID == "" || p.Text == "" {
			t.Fatalf("有一道题没有 id 或题面：%+v", p)
		}
		if p.Lang != LangZH && p.Lang != LangEN {
			t.Fatalf("%s 的语言是 %q —— 闭表之外", p.ID, p.Lang)
		}
		if p.Difficulty < DiffBasic || p.Difficulty > DiffExpert {
			t.Fatalf("%s 的难度是 %d —— 闭表之外", p.ID, p.Difficulty)
		}
		// 🚨 话题必须在闭表里。模型标的东西不许直接落库，
		// 这条守的是 cmd/prompttag 里那道 keepValid 真的在起作用。
		for _, tp := range p.Topics {
			if !TopicValid(tp) {
				t.Fatalf("%s 带了一个闭表外的话题 %q", p.ID, tp)
			}
		}
		if len(p.Topics) > 3 {
			t.Fatalf("%s 有 %d 个话题 —— 最多三个", p.ID, len(p.Topics))
		}
	}
}

func TestByID(t *testing.T) {
	all, _ := All()
	want := all[len(all)/2]
	got, ok := ByID(want.ID)
	if !ok || got.ID != want.ID {
		t.Fatalf("按 id 取不到 %s", want.ID)
	}
	if _, ok := ByID("没有这个 id"); ok {
		t.Fatal("取到了一个不存在的 id")
	}
}

// 翻页：页与页之间不重不漏，全部加起来正好是总数。
//
// 🚨 这是这一族最容易出错的地方，而错了她看到的是「有几道题我怎么都翻不到」——
// 一个不会报错、只会让人觉得库不全的毛病。
func TestPagingCoversEveryItemExactlyOnce(t *testing.T) {
	first, err := Search(Query{PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for page := 1; page <= first.Pages; page++ {
		r, err := Search(Query{PageSize: 50, Page: page})
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range r.Items {
			seen[p.ID]++
		}
	}
	if len(seen) != first.Total {
		t.Fatalf("翻完所有页拿到 %d 道，总数说是 %d 道", len(seen), first.Total)
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("%s 出现了 %d 次 —— 页与页之间重了", id, n)
		}
	}
}

// 🚨 翻过头给空页，不要回卷到第一页。
// 回卷的样子是她按「下一页」按到底、屏幕突然跳回开头，读起来像东西丢了。
func TestPageBeyondTheEndIsEmptyNotWrapped(t *testing.T) {
	r, err := Search(Query{Page: 9999})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Items) != 0 {
		t.Fatalf("第 9999 页给了 %d 道题", len(r.Items))
	}
	if r.Total == 0 {
		t.Fatal("总数不该是 0")
	}
}

func TestPageSizeIsClamped(t *testing.T) {
	r, _ := Search(Query{PageSize: 100000})
	if len(r.Items) > MaxPageSize {
		t.Fatalf("一次给了 %d 道 —— 上限是 %d", len(r.Items), MaxPageSize)
	}
	d, _ := Search(Query{})
	if d.PageSize != DefaultPageSize {
		t.Fatalf("默认每页 %d，该是 %d", d.PageSize, DefaultPageSize)
	}
}

func TestFilters(t *testing.T) {
	zh, _ := Search(Query{Lang: LangZH, PageSize: MaxPageSize})
	for _, p := range zh.Items {
		if p.Lang != LangZH {
			t.Fatalf("筛了中文却给了 %s（%s）", p.Lang, p.ID)
		}
	}
	en, _ := Search(Query{Lang: LangEN})
	if zh.Total+en.Total != mustTotal(t) {
		t.Fatalf("中文 %d + 英文 %d 不等于总数 %d", zh.Total, en.Total, mustTotal(t))
	}

	hard, _ := Search(Query{Difficulty: DiffExpert, PageSize: MaxPageSize})
	for _, p := range hard.Items {
		if p.Difficulty != DiffExpert {
			t.Fatalf("筛了高阶却给了难度 %d", p.Difficulty)
		}
	}
}

func mustTotal(t *testing.T) int {
	t.Helper()
	r, err := Search(Query{})
	if err != nil {
		t.Fatal(err)
	}
	return r.Total
}

// 🚨 筛选器的计数要按「其他几维筛完之后」算。
// 否则她选了中文，话题那一列还显示着 GRE 才有的话题各有多少道 ——
// 一个能点、点了却空的筛选项，比没有这个筛选项更糟。
func TestFacetCountsRespectTheOtherFilters(t *testing.T) {
	zh, err := Search(Query{Lang: LangZH})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zh.Facets.Categories {
		sub, _ := Search(Query{Lang: LangZH, Category: f.Value})
		if sub.Total != f.Count {
			t.Fatalf("筛中文时「%s」说有 %d 道，真点进去是 %d 道", f.Value, f.Count, sub.Total)
		}
		if sub.Total == 0 {
			t.Fatalf("「%s」是个点进去为空的筛选项", f.Value)
		}
	}
	for _, f := range zh.Facets.Topics {
		sub, _ := Search(Query{Lang: LangZH, Topic: f.Value})
		if sub.Total != f.Count {
			t.Fatalf("筛中文时话题「%s」说有 %d 道，真点进去是 %d 道", f.Value, f.Count, sub.Total)
		}
	}
}

// 话题这一维只出闭表里的值，而且顺序固定 —— 筛选器不该每次刷新换一个排法。
func TestTopicFacetsFollowTheClosedSetOrder(t *testing.T) {
	r, _ := Search(Query{})
	last := -1
	for _, f := range r.Facets.Topics {
		if !TopicValid(f.Value) {
			t.Fatalf("话题筛选里混进了闭表外的 %q", f.Value)
		}
		idx := -1
		for i, x := range Topics {
			if x == f.Value {
				idx = i
			}
		}
		if idx <= last {
			t.Fatalf("话题顺序乱了：%q 排在前一个之前", f.Value)
		}
		last = idx
	}
}

func TestSearchMatchesTheText(t *testing.T) {
	all, _ := All()
	// 拿一道真题里的一小段字去搜，必须搜得到它自己。
	probe := []rune(all[0].Text)
	if len(probe) > 8 {
		probe = probe[:8]
	}
	r, err := Search(Query{Q: string(probe), PageSize: MaxPageSize})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range r.Items {
		if p.ID == all[0].ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("拿 %q 搜不到它自己（%s）", string(probe), all[0].ID)
	}
	if n, _ := Search(Query{Q: "这几个字整库都不会有zzzq"}); n.Total != 0 {
		t.Fatalf("搜一个不存在的串还给了 %d 道", n.Total)
	}
}

// 🚨 英文按词 AND，不是 OR。敲两个词的人要的是同时谈这两件事的题，
// 不是谈其中任何一件的两百道。
func TestEnglishSearchIsAndNotOr(t *testing.T) {
	both, _ := Search(Query{Q: "technology education"})
	one, _ := Search(Query{Q: "technology"})
	if both.Total > one.Total {
		t.Fatalf("两个词搜出 %d 道，比一个词的 %d 道还多 —— 那是 OR", both.Total, one.Total)
	}
}

// 搜索和筛选叠加，不是互相取代。
func TestSearchComposesWithFilters(t *testing.T) {
	r, _ := Search(Query{Lang: LangEN, Q: "the", PageSize: MaxPageSize})
	for _, p := range r.Items {
		if p.Lang != LangEN {
			t.Fatalf("又搜又筛的时候漏了一道中文题：%s", p.ID)
		}
	}
}
