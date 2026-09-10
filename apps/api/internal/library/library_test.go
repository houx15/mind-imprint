package library

import (
	"strings"
	"testing"

	"mindimprint/api/internal/disciplines"
)

// 这份数据是流水线生成的，不是手写的，所以测的是「流水线换了一版之后，还成不
// 成立」的那些性质：档位连号、学科在闭表里、每篇都有题图。渲染长什么样不在
// 这里测（见 AGENTS.md 关于只写逻辑测试的那一条）。

// 这里是下限，不是等号。库每上一批就长一次（第一批 20 篇，2026-09-10 那批之后
// 48 篇），写等号的话每批都要来改一个数字，而改完之后这条测试当天就不再说明
// 任何事情。它要挡的是另一件事：流水线某次跑出一份缺了大半的 articles.json，
// 而接口照样返回 200、书架照样有东西可看。
const atLeast = 40

func TestLibraryLoads(t *testing.T) {
	if err := LoadErr(); err != nil {
		t.Fatalf("articles.json 没能加载: %v", err)
	}
	if len(All()) < atLeast {
		t.Fatalf("库里只有 %d 篇，少于 %d —— 流水线大概少跑了一批源", len(All()), atLeast)
	}
}

func TestEveryArticleHasFiveNamedTiers(t *testing.T) {
	want := []string{"入门", "基础", "进阶", "高阶", "原文"}
	for _, a := range All() {
		if len(a.Levels) != 5 {
			t.Errorf("%s 有 %d 档", a.Slug, len(a.Levels))
			continue
		}
		for i, l := range a.Levels {
			if l.Tier != i+1 || l.Name != want[i] {
				t.Errorf("%s 第 %d 档是 tier=%d name=%q，期望 tier=%d name=%q",
					a.Slug, i+1, l.Tier, l.Name, i+1, want[i])
			}
		}
	}
}

// 原文那一档没有 Lexile 值（0），其余四档必须有一个，而且从易到难递增。这条
// 挡的是排序错位：档位名对了、正文却排反了的话，「往下推荐」会把最难的那篇
// 当成入门推给她。
func TestTiersRunEasyToHard(t *testing.T) {
	for _, a := range All() {
		prev := 0
		for _, l := range a.Levels[:4] {
			if l.Lexile <= 0 {
				t.Errorf("%s 的 %s 档没有 Lexile 值", a.Slug, l.Name)
				continue
			}
			if l.Lexile <= prev {
				t.Errorf("%s 的 %s 档 %dL 不比上一档 %dL 难", a.Slug, l.Name, l.Lexile, prev)
			}
			prev = l.Lexile
		}
		if a.Levels[4].Lexile != 0 {
			t.Errorf("%s 的原文档带了 Lexile 值 %d，原文没有分级", a.Slug, a.Levels[4].Lexile)
		}
	}
}

func TestEveryTagIsInTheDisciplineTable(t *testing.T) {
	for _, a := range All() {
		if len(a.Disciplines) == 0 {
			t.Errorf("%s 没有学科标签，永远不会被推荐出来", a.Slug)
		}
		for _, id := range a.Disciplines {
			if _, ok := disciplines.ByID(id); !ok {
				t.Errorf("%s 标了 %q，闭表里没有这一门", a.Slug, id)
			}
		}
	}
}

func TestEveryArticleHasACover(t *testing.T) {
	for _, a := range All() {
		if a.Cover == nil || a.Cover.Key == "" {
			t.Errorf("%s 没有题图，书架上会是一个空框", a.Slug)
		}
		if a.Cover != nil && (a.Cover.Width == 0 || a.Cover.Height == 0) {
			t.Errorf("%s 的题图没有尺寸，卡片会在图到位时跳一下", a.Slug)
		}
	}
}

// 图注是显示给学生看的，空的图注等于一张没有说明的图。
func TestEveryFigureIsCaptionedAndCredited(t *testing.T) {
	for _, a := range All() {
		for _, l := range a.Levels {
			for _, f := range l.Figures {
				if f.Caption == "" {
					t.Errorf("%s %s: %s 没有图注", a.Slug, l.Name, f.Key)
				}
				if f.Credit == "" {
					t.Errorf("%s %s: %s 没有署名", a.Slug, l.Name, f.Key)
				}
				if f.Width == 0 || f.Height == 0 {
					t.Errorf("%s %s: %s 没有尺寸", a.Slug, l.Name, f.Key)
				}
			}
		}
	}
}

// 署名要么这一篇每一档都有，要么每一档都没有。
//
// 中间状态才是 bug：署名是从正文里摘出去的一行（不摘就变成 b1，会被当成文章
// 第一句引给学生），所以「有几档摘到了、有几档没摘到」说明解析器在这一批上
// 只对了一半 —— 而没摘到的那几档，那行字这会儿正躺在正文里。
//
// 两批的正常状态不同：第一批的导出根本没有署名行（20 篇全空），第二批每档都有。
// 所以这里不写「必须有」，写的是「这一篇里要一致」。
func TestBylineIsAllLevelsOrNone(t *testing.T) {
	for _, a := range All() {
		with := 0
		for _, l := range a.Levels {
			if l.Byline != "" {
				with++
			}
		}
		if with != 0 && with != len(a.Levels) {
			t.Errorf("%s 有 %d/%d 档带署名 —— 没带的那几档，那行字大概还在正文里",
				a.Slug, with, len(a.Levels))
		}
		for _, l := range a.Levels {
			// "By " 是导出里的前缀，摘的时候要一起去掉：界面自己写「来源 · 」，
			// 留着就成了「来源 · By 美联社」。
			if strings.HasPrefix(l.Byline, "By ") {
				t.Errorf("%s %s 的署名还带着 By 前缀: %q", a.Slug, l.Name, l.Byline)
			}
			if strings.TrimSpace(l.Byline) != l.Byline {
				t.Errorf("%s %s 的署名两头有空白: %q", a.Slug, l.Name, l.Byline)
			}
		}
	}
}

func TestFieldsAreOrderedLikeTheTree(t *testing.T) {
	got := Fields()
	if len(got) == 0 {
		t.Fatal("筛选栏一根枝子都没有")
	}
	seen := 0
	for _, f := range disciplines.Fields {
		if seen < len(got) && got[seen] == f {
			seen++
		}
	}
	if seen != len(got) {
		t.Errorf("Fields() 是 %v，没有按 disciplines.Fields 的顺序", got)
	}
}

func TestSuggestTier(t *testing.T) {
	cases := []struct {
		name                      string
		finishedTop, abandonedTop int
		want                      int
	}{
		{"没有记录时从基础开始，不从入门开始", 0, 0, 2},
		{"读完过高阶就还给高阶", 4, 0, 4},
		{"开了高阶没读完，下一档", 0, 4, 3},
		{"读完过进阶、又在原文那一档卡住，回进阶", 3, 5, 4},
		{"入门都没读完也不会掉到 0 档", 0, 1, 1},
		{"读完过原文就停在原文", 5, 0, 5},
	}
	for _, c := range cases {
		if got := SuggestTier(c.finishedTop, c.abandonedTop); got != c.want {
			t.Errorf("%s: SuggestTier(%d, %d) = %d，期望 %d",
				c.name, c.finishedTop, c.abandonedTop, got, c.want)
		}
	}
}

func TestRecommendSkipsWhatSheAlreadyOpened(t *testing.T) {
	all := All()
	read := map[string]bool{all[0].Slug: true, all[1].Slug: true}
	got := Recommend(all, Profile{ReadSlugs: read, Tier: 2}, 0)
	for _, r := range got {
		if read[r.Article.Slug] {
			t.Errorf("%s 已经开过了，还被推荐出来", r.Article.Slug)
		}
	}
	if len(got) != len(all)-2 {
		t.Errorf("推荐了 %d 篇，期望 %d 篇", len(got), len(all)-2)
	}
}

// 树是空的时候，书架照样要摆满 —— 只是每一条的 Why 都空着，界面据此换一种
// 说法，不会把补位的那几篇说成「按你的兴趣挑的」。
func TestRecommendFillsTheShelfForAnEmptyTree(t *testing.T) {
	got := Recommend(All(), Profile{Tier: 2}, 4)
	if len(got) != 4 {
		t.Fatalf("空的树推出来 %d 篇，期望 4 篇", len(got))
	}
	for _, r := range got {
		if len(r.Why) != 0 {
			t.Errorf("%s 的 Why 是 %v，树是空的，不该有理由", r.Article.Slug, r.Why)
		}
		if r.Tier != 2 {
			t.Errorf("%s 推的是第 %d 档，期望第 2 档", r.Article.Slug, r.Tier)
		}
	}
}

func TestRecommendRanksHerInterestsFirst(t *testing.T) {
	// astronomy 挂在 nasa / rubio / eclipse / hernandez / earthworks 上。
	p := Profile{Disciplines: map[string]float64{"astronomy": 5}, Tier: 3}
	got := Recommend(All(), p, 3)
	for _, r := range got {
		hit := false
		for _, d := range r.Article.Disciplines {
			if d == "astronomy" {
				hit = true
			}
		}
		if !hit {
			t.Errorf("前三名里的 %s 不挂 astronomy，学科交集没起作用", r.Article.Slug)
		}
		if len(r.Why) == 0 {
			t.Errorf("%s 是按兴趣挑的，Why 却是空的", r.Article.Slug)
		}
	}
}

// 稀有度折价：她在两门学科上强度相同时，挂着「篇数少的那门」的文章要排在前面。
// 不折价的话，覆盖面最广的那门每次都赢，推荐看上去就像随机的（2026-09-05 的
// 兴趣地图就是栽在这里）。
func TestRecommendDiscountsBroadDisciplines(t *testing.T) {
	articles := []Article{
		{Slug: "broad-a", Disciplines: []string{"history"}},
		{Slug: "broad-b", Disciplines: []string{"history"}},
		{Slug: "broad-c", Disciplines: []string{"history"}},
		{Slug: "narrow", Disciplines: []string{"probability"}},
	}
	p := Profile{Disciplines: map[string]float64{"history": 3, "probability": 3}, Tier: 2}
	got := Recommend(articles, p, 1)
	if len(got) != 1 || got[0].Article.Slug != "narrow" {
		t.Fatalf("第一名是 %v，期望 narrow —— 挂了三篇的 history 应该被折价", got)
	}
}

func TestLevelAt(t *testing.T) {
	a := All()[0]
	for tier := 1; tier <= 5; tier++ {
		if _, ok := a.LevelAt(tier); !ok {
			t.Errorf("%s 取不到第 %d 档", a.Slug, tier)
		}
	}
	if _, ok := a.LevelAt(6); ok {
		t.Error("第 6 档不存在，却取到了")
	}
	if _, ok := a.LevelAt(0); ok {
		t.Error("第 0 档不存在，却取到了")
	}
}
