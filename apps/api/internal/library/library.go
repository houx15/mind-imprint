// Package library 是自建的分级阅读库 —— 二十篇报道，每篇五个难度。
//
// # 它解决的问题
//
// 阅读室原本只有一个入口：她手上已经有一篇要读的文章，粘进来。真正的情况是
// 她常常不知道读什么，而落地页上那四篇写死在前端的示范稿（readings/
// recommendations.ts）既不能筛、不能搜，也和她的兴趣树没有任何关系。
//
// 这个库给出另一个入口：一份能按学科筛、按难度选、按她树上的词往下推荐的
// 真实语料。每个故事有五个难度版本（入门 / 基础 / 进阶 / 高阶 / 原文），
// 同一批照片，同一件事，换一种写法。
//
// # 为什么是文件不是表
//
// 它是内容，不是用户数据：全校共用一份，改它是一次内容编辑加一次评审，不是
// 一次数据迁移；而且它能在 pull request 里 diff。internal/disciplines 出于
// 同样的理由做了同样的选择。只有「边」——她读了哪一篇、读的哪一档——进
// Postgres（reading.library_slug / library_tier，迁移 0142）。
//
// # 数据从哪来
//
// articles.json 由 deploy/reading-library/build.py 生成，源是 gitignore 掉的
// docs/reference/reading-database/（100 个 markdown + 60 张原图）。改内容改的
// 是那条流水线，不是手改这个 JSON。
//
// # 图片
//
// 正文里没有图。阅读室按空行切段、把工具卡挂在段 id 上，一张留在正文里的图
// 会占掉一个段 id，然后被当成一段「课文」引回给学生。所以图单独走 Figures，
// 每张记住自己跟在哪一段之后（After，空串表示题图）。对象键指向 OSS，读的
// 时候由 handler 现签（学生桶是私有的，见 internal/oss）。
package library

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"mindimprint/api/internal/disciplines"
)

//go:embed articles.json
var articlesJSON []byte

// Figure 是正文里的一张图，连同它在段落序列中的位置。
type Figure struct {
	// After 是它跟在哪一段之后，取值是 api.SplitBlocks 给出的段 id（b1、b2…）。
	// 空串表示它站在第一段之前，也就是题图。
	After string `json:"after"`
	// Key 是 OSS 对象键（web/reading/v1/…）。这里不存 URL：学生桶是私有的，
	// 链接由 handler 用 oss.SignDownload 现签，签名有有效期。
	Key     string `json:"key"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Caption string `json:"caption"`
	// Credit 连它的标记词一起存（"Photo: …" / "Graphic: …"），因为那个词
	// 正是读者判断自己在看照片还是看图表的依据。
	Credit string `json:"credit"`
}

// Level 是一个故事的一个难度版本。
type Level struct {
	Tier int    `json:"tier"` // 1..5
	Name string `json:"name"` // 入门 / 基础 / 进阶 / 高阶 / 原文
	// Lexile 是原始分级值；0 表示这一档是没有简写过的原文，分级不适用。
	Lexile   int      `json:"lexile"`
	Words    int      `json:"words"`
	Minutes  int      `json:"minutes"`
	Title    string   `json:"title"`
	Body     string   `json:"body"`
	Headings []string `json:"headings"` // 要当小标题渲染的段 id
	Figures  []Figure `json:"figures"`
}

// Cover 是书架上那张图。取的是原文档那一档的题图。
type Cover struct {
	Key     string `json:"key"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Caption string `json:"caption"`
}

// Article 是一个故事：一个标题、一组学科标签、五个难度。
type Article struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	ZhTitle string `json:"zhTitle"`
	Reason  string `json:"reason"`
	Lang    string `json:"lang"`
	// Disciplines 全部来自 disciplines.json 的闭表。这是文章与兴趣树之间
	// 唯一的一根线：她的关键词路由到学科，文章标注学科，推荐取交集。
	Disciplines []string `json:"disciplines"`
	Field       string   `json:"field"` // 主学科所属的主枝，决定卡片颜色
	Cover       *Cover   `json:"cover"`
	Levels      []Level  `json:"levels"`
}

// LevelAt 返回第 tier 档（1..5）。
func (a Article) LevelAt(tier int) (Level, bool) {
	for _, l := range a.Levels {
		if l.Tier == tier {
			return l, true
		}
	}
	return Level{}, false
}

var (
	once    sync.Once
	all     []Article
	bySlug  map[string]Article
	loadErr error
)

func load() {
	once.Do(func() {
		if err := json.Unmarshal(articlesJSON, &all); err != nil {
			loadErr = fmt.Errorf("library: articles.json: %w", err)
			return
		}
		bySlug = make(map[string]Article, len(all))
		for _, a := range all {
			if err := validate(a); err != nil {
				loadErr = err
				return
			}
			bySlug[a.Slug] = a
		}
		if len(all) == 0 {
			loadErr = fmt.Errorf("library: articles.json is empty")
		}
	})
}

// validate 是启动期的边界校验。一篇标了表外学科的文章会静静地从推荐里消失
// （交集永远为空），所以宁可开机就失败。
func validate(a Article) error {
	if a.Slug == "" || a.Title == "" || a.ZhTitle == "" {
		return fmt.Errorf("library: %q is missing a slug or a title", a.Slug)
	}
	if len(a.Disciplines) == 0 {
		return fmt.Errorf("library: %s carries no discipline", a.Slug)
	}
	for _, id := range a.Disciplines {
		if _, ok := disciplines.ByID(id); !ok {
			return fmt.Errorf("library: %s is tagged %q, which is not in the discipline table", a.Slug, id)
		}
	}
	if !disciplines.IsField(a.Field) {
		return fmt.Errorf("library: %s sits on unknown branch %q", a.Slug, a.Field)
	}
	if len(a.Levels) != 5 {
		return fmt.Errorf("library: %s has %d levels, expected 5", a.Slug, len(a.Levels))
	}
	for i, l := range a.Levels {
		if l.Tier != i+1 {
			return fmt.Errorf("library: %s level %d is numbered %d", a.Slug, i+1, l.Tier)
		}
		if strings.TrimSpace(l.Body) == "" {
			return fmt.Errorf("library: %s tier %d has no body", a.Slug, l.Tier)
		}
	}
	return nil
}

// LoadErr 返回解析 articles.json 时的错误（正常情况下是 nil）。测试用它把
// 「数据坏了」和「查不到」分开，而不是让一个空目录看起来像一个空书架。
func LoadErr() error { load(); return loadErr }

// All 返回全部文章，按 slug 排序（articles.json 本来就是这个顺序）。
func All() []Article { load(); return all }

// BySlug 查一篇文章。
func BySlug(slug string) (Article, bool) {
	load()
	a, ok := bySlug[slug]
	return a, ok
}

// Fields 返回库里出现过的主枝，按 disciplines.Fields 的固定顺序。筛选栏用它，
// 这样一根没有文章的枝子不会摆在那里点了没反应。
func Fields() []string {
	load()
	present := map[string]bool{}
	for _, a := range all {
		present[a.Field] = true
	}
	out := make([]string, 0, len(disciplines.Fields))
	for _, f := range disciplines.Fields {
		if present[f] {
			out = append(out, f)
		}
	}
	return out
}

// disciplineWeight 是学科的稀有度折价。
//
// 兴趣地图那一版的教训（2026-09-05）：不折价的话，覆盖面最广的那门学科每次
// 都排第一，推荐看上去就像随机的。一门挂了八篇文章的学科说明不了什么，一门
// 只挂两篇的学科说明得多，所以权重按它在库里出现的篇数递减。
func disciplineWeight(counts map[string]int, id string) float64 {
	n := counts[id]
	if n <= 0 {
		return 0
	}
	return 1 / float64(n)
}

// Profile 是推荐要用到的、关于这个学生的全部输入。它是普通数据，不是数据库
// 句柄 —— Recommend 因此是纯函数，能直接测。
type Profile struct {
	// Disciplines 是她树上的词路由到的学科，值是这门学科上的证据强度之和。
	Disciplines map[string]float64
	// ReadSlugs 是她已经开过的文章，不再推荐。
	ReadSlugs map[string]bool
	// Tier 是给她的默认难度（1..5）。见 SuggestTier。
	Tier int
}

// Recommendation 是一条推荐：推哪一篇、推哪一档、为什么。
type Recommendation struct {
	Article Article `json:"-"`
	Tier    int     `json:"tier"`
	// Why 是命中的学科 id（最多三个），前端把它们显示成她自己的词旁边的标签。
	// 空表示这条是补位的 —— 她的树还太小，推不出交集。
	Why   []string `json:"why"`
	Score float64  `json:"-"`
}

// SuggestTier 从她读过的档位推一个默认档。
//
// 「往下推荐」在这里有两层意思，两层都要：
//
//   - 冷启动从 2（基础）开始，而不是从 1。一个高中生打开第一篇就被塞一篇
//     四百词的入门稿，那是在告诉她我们觉得她读不了。
//   - 上一篇没读完，就下一档。半途停下最常见的原因就是太难了，而她不会为此
//     专门来点一个「换简单点」的按钮。
//
// finishedTop 是她读完过的最高档，abandonedTop 是她开了没读完的最高档；
// 两个都是 0 表示没有记录。
func SuggestTier(finishedTop, abandonedTop int) int {
	tier := 2
	if finishedTop > 0 {
		tier = finishedTop
	}
	if abandonedTop > finishedTop && abandonedTop > 0 {
		tier = abandonedTop - 1
	}
	if tier < 1 {
		tier = 1
	}
	if tier > 5 {
		tier = 5
	}
	return tier
}

// Recommend 从她的树往下推到具体文章。
//
// 打分 = Σ（这篇文章的学科 ∩ 她的学科）上的 她的强度 × 学科稀有度。没有交集
// 的文章分数为 0，但不被丢掉：一个刚注册、树还是空的学生也必须看到一书架
// 东西，所以零分的那些按库里的顺序排在后面补位（Why 为空，界面据此换一种
// 说法，不会把「猜的」说成「按你的兴趣」）。
func Recommend(articles []Article, p Profile, limit int) []Recommendation {
	counts := map[string]int{}
	for _, a := range articles {
		for _, id := range a.Disciplines {
			counts[id]++
		}
	}

	out := make([]Recommendation, 0, len(articles))
	for i, a := range articles {
		if p.ReadSlugs[a.Slug] {
			continue
		}
		var score float64
		var why []string
		for _, id := range a.Disciplines {
			strength := p.Disciplines[id]
			if strength <= 0 {
				continue
			}
			score += strength * disciplineWeight(counts, id)
			why = append(why, id)
		}
		if len(why) > 3 {
			why = why[:3]
		}
		tier := p.Tier
		if tier < 1 || tier > 5 {
			tier = 2
		}
		out = append(out, Recommendation{
			Article: a,
			Tier:    tier,
			Why:     why,
			Score:   score - float64(i)*1e-9, // 稳定排序：同分按库里的顺序
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
