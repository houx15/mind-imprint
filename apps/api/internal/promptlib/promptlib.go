// Package promptlib 是自建的写作题库 —— 705 道 2024–2026 的真题与官方题。
//
// # 它解决的问题
//
// 写作室的落地页上原来有一排写死在前端的题目（「该不该把上学时间往后推？」
// 那四条）。它们不能筛、不能搜、和她在读什么在学什么没有关系，而且四条就是
// 四条 —— 一个学生翻两次就看完了。
//
// 这个库给出另一个入口：一份能按语言筛、按考试筛、按难度选、按话题找、
// 还能全文搜的真实题目。中考 / 高考的语文与英语、托福、雅思、GRE，
// 每道题带着它的出处、年份、字数要求和分值。
//
// # 为什么是文件不是表
//
// 和 internal/library（分级阅读库）同一条理由：它是**内容**，不是用户数据。
// 全校共用一份，改它是一次内容编辑加一次评审，不是一次数据迁移；
// 而且它能在 pull request 里 diff。只有「边」——她从哪道题开了一篇写作——
// 才进 Postgres。
//
// # 数据从哪来
//
// prompts.json 由 `go run ./cmd/prompttag` 生成，源是 gitignore 掉的
// docs/reference/writing-teaching/essay_prompts_2024_2026.json。
// 改内容改的是那条流水线，不是手改这个 JSON。
//
// # 话题是**离线标好、随库提交**的
//
// 源数据里没有话题字段，而中文作文题偏偏最需要它（「语言的滋味」「攀登是
// 幸福的」——按关键词根本分不出这是哪一类）。所以话题由 cmd/prompttag 跑一次
// 模型标出来，落进这个 JSON 里；她翻库的时候**一次模型调用都不发**。
// 判据照这个仓库的老规矩来：闭表之外的标签一律丢掉，不认的就空着。
package promptlib

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

//go:embed prompts.json
var promptsJSON []byte

// Lang 是这道题要用哪种语言写。闭表。
const (
	LangZH = "zh"
	LangEN = "en"
)

// 难度。三档，由考试类型派生（见 cmd/prompttag 的 difficultyOf）——
// 不是模型判的，也不写在源数据里：它是一条关于「谁在考这张卷子」的事实。
const (
	DiffBasic    = 1 // 中考
	DiffAdvanced = 2 // 高考 / 托福 / 雅思
	DiffExpert   = 3 // GRE
)

// Topics 是话题闭表。**只有这 16 个**，模型标出别的一律丢掉。
//
// 选它们的依据是这 705 道题真正在问的东西：中文卷偏个人与成长，
// 英文卷偏社会与公共议题，两头都要装得下。顺序就是筛选器里的顺序。
var Topics = []string{
	"成长与自我",
	"亲情与家庭",
	"友谊与相处",
	"学习与教育",
	"科技与网络",
	"环境与自然",
	"社会与公共生活",
	"文化与传统",
	"艺术与审美",
	"劳动与职业",
	"挫折与坚持",
	"理想与选择",
	"健康与生活",
	"语言与表达",
	"媒体与信息",
	"城市与旅行",
}

// TopicValid 判一个标签在不在闭表里。
func TopicValid(t string) bool {
	for _, x := range Topics {
		if x == t {
			return true
		}
	}
	return false
}

// Prompt 是一道题。
//
// 字段分两类，**来源不同、可信度也不同**，所以注释里写清楚：
//   - 抄来的：Category / Year / Type / Source / Region / TaskType / Text /
//     Requirements / WordLimit / Minutes / FullScore / SourceURL / Notes。
//     这些是原始数据，一个字都没改。
//   - 派生的：Lang / Difficulty（由 Category 推，确定性的）、
//     Topics（离线跑模型标的，过了闭表）。
type Prompt struct {
	ID       string `json:"id"`
	Category string `json:"category"` // 中考语文 / 高考英语 / 托福写作 …
	Lang     string `json:"lang"`
	Year     int    `json:"year"`
	Type     string `json:"type"`   // 真题 / 回忆版 / 模拟题 / 官方题库 / 官方样题
	Source   string `json:"source"` // 那一场考试的全名
	Region   string `json:"region,omitempty"`
	TaskType string `json:"taskType"` // 材料作文 / GRE Issue / 读后续写 …

	Text         string `json:"text"`
	Requirements string `json:"requirements,omitempty"`
	WordLimit    string `json:"wordLimit,omitempty"`
	Minutes      int    `json:"minutes,omitempty"`
	FullScore    int    `json:"fullScore,omitempty"`
	SourceURL    string `json:"sourceUrl,omitempty"`
	Notes        string `json:"notes,omitempty"`

	Difficulty int      `json:"difficulty"`
	Topics     []string `json:"topics,omitempty"`

	// search 是预先拼好的小写检索串，不进 JSON。
	search string
}

var (
	once    sync.Once
	all     []Prompt
	loadErr error
)

func load() {
	once.Do(func() {
		if err := json.Unmarshal(promptsJSON, &all); err != nil {
			loadErr = fmt.Errorf("promptlib: prompts.json: %w", err)
			return
		}
		for i := range all {
			p := &all[i]
			p.search = strings.ToLower(strings.Join([]string{
				p.Text, p.Source, p.TaskType, p.Category, p.Region,
				p.Requirements, p.Notes, strings.Join(p.Topics, " "),
			}, "\n"))
		}
	})
}

// All 是整份题库，按 id 排好。调用方不要改它。
func All() ([]Prompt, error) {
	load()
	return all, loadErr
}

// Query 是一次翻库：筛 + 搜 + 翻页。
//
// 零值意味着「这一维不筛」。Page 从 1 起。
type Query struct {
	Lang       string
	Category   string
	Difficulty int
	Topic      string
	Year       int
	Q          string
	Page       int
	PageSize   int
}

// DefaultPageSize —— 一页几张卡。
//
// 24：三列的时候是整八行，两列是十二行，手机一列也不至于让她滚到天荒地老。
// 705 道题分成 30 页，页码条还排得下。
const DefaultPageSize = 24

// MaxPageSize 挡住 `?pageSize=100000` 把整库一次性拉走。
const MaxPageSize = 100

// Facet 是一个筛选值和它下面有多少道题。
//
// 🚨 **计数要按「其他几维筛完之后」算**，不是按整库算。
// 否则她选了「中文」，话题那一列还显示着 GRE 才有的话题各有多少道，
// 点进去一道都没有 —— 一个能点、点了却空的筛选项比没有这个筛选项更糟。
type Facet struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// Facets 是这一次查询下，每一维还剩哪些值可选。
type Facets struct {
	Langs        []Facet `json:"langs"`
	Categories   []Facet `json:"categories"`
	Difficulties []Facet `json:"difficulties"`
	Topics       []Facet `json:"topics"`
	Years        []Facet `json:"years"`
}

// Result 是一页。
type Result struct {
	Items    []Prompt `json:"items"`
	Total    int      `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"pageSize"`
	Pages    int      `json:"pages"`
	Facets   Facets   `json:"facets"`
}

func (q Query) matchExceptLang(p Prompt) bool {
	return q.matchCategory(p) && q.matchDifficulty(p) && q.matchTopic(p) && q.matchYear(p) && q.matchQ(p)
}
func (q Query) matchExceptCategory(p Prompt) bool {
	return q.matchLang(p) && q.matchDifficulty(p) && q.matchTopic(p) && q.matchYear(p) && q.matchQ(p)
}
func (q Query) matchExceptDifficulty(p Prompt) bool {
	return q.matchLang(p) && q.matchCategory(p) && q.matchTopic(p) && q.matchYear(p) && q.matchQ(p)
}
func (q Query) matchExceptTopic(p Prompt) bool {
	return q.matchLang(p) && q.matchCategory(p) && q.matchDifficulty(p) && q.matchYear(p) && q.matchQ(p)
}
func (q Query) matchExceptYear(p Prompt) bool {
	return q.matchLang(p) && q.matchCategory(p) && q.matchDifficulty(p) && q.matchTopic(p) && q.matchQ(p)
}

func (q Query) matchLang(p Prompt) bool { return q.Lang == "" || p.Lang == q.Lang }
func (q Query) matchCategory(p Prompt) bool {
	return q.Category == "" || p.Category == q.Category
}
func (q Query) matchDifficulty(p Prompt) bool {
	return q.Difficulty == 0 || p.Difficulty == q.Difficulty
}
func (q Query) matchYear(p Prompt) bool { return q.Year == 0 || p.Year == q.Year }
func (q Query) matchTopic(p Prompt) bool {
	if q.Topic == "" {
		return true
	}
	for _, t := range p.Topics {
		if t == q.Topic {
			return true
		}
	}
	return false
}

// matchQ 是全文搜索。
//
// 🚨 中文不分词，所以只能是子串匹配 —— 而这正好是她要的：她在框里敲「环保」，
// 想找的就是题面里出现「环保」的那些题。英文按空格切成词，每个词都要命中
// （AND，不是 OR）：敲 "technology education" 的人要的是同时谈这两件事的题，
// 不是谈其中任何一件的 200 道。
func (q Query) matchQ(p Prompt) bool {
	s := strings.TrimSpace(strings.ToLower(q.Q))
	if s == "" {
		return true
	}
	for _, w := range strings.Fields(s) {
		if !strings.Contains(p.search, w) {
			return false
		}
	}
	return true
}

func (q Query) match(p Prompt) bool {
	return q.matchLang(p) && q.matchCategory(p) && q.matchDifficulty(p) &&
		q.matchTopic(p) && q.matchYear(p) && q.matchQ(p)
}

// Search 跑一次查询。
func Search(q Query) (Result, error) {
	load()
	if loadErr != nil {
		return Result{}, loadErr
	}

	if q.PageSize <= 0 {
		q.PageSize = DefaultPageSize
	}
	if q.PageSize > MaxPageSize {
		q.PageSize = MaxPageSize
	}
	if q.Page <= 0 {
		q.Page = 1
	}

	var hits []Prompt
	langs := map[string]int{}
	cats := map[string]int{}
	diffs := map[int]int{}
	topics := map[string]int{}
	years := map[int]int{}

	for _, p := range all {
		if q.match(p) {
			hits = append(hits, p)
		}
		// 每一维的计数，都排除**这一维自己**的筛选值。见 Facet 上面那段。
		if q.matchExceptLang(p) {
			langs[p.Lang]++
		}
		if q.matchExceptCategory(p) {
			cats[p.Category]++
		}
		if q.matchExceptDifficulty(p) {
			diffs[p.Difficulty]++
		}
		if q.matchExceptYear(p) {
			years[p.Year]++
		}
		if q.matchExceptTopic(p) {
			for _, t := range p.Topics {
				topics[t]++
			}
		}
	}

	total := len(hits)
	pages := (total + q.PageSize - 1) / q.PageSize
	// 🚨 翻过头了就给空的那一页，不要回卷到第一页 ——
	// 她按「下一页」按到底，屏幕突然跳回开头，那读起来像是东西丢了。
	start := (q.Page - 1) * q.PageSize
	if start > total {
		start = total
	}
	end := start + q.PageSize
	if end > total {
		end = total
	}

	return Result{
		Items:    hits[start:end],
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
		Pages:    pages,
		Facets: Facets{
			Langs:        strFacets(langs, Topics[:0], []string{LangZH, LangEN}),
			Categories:   strFacetsSorted(cats),
			Difficulties: intFacets(diffs),
			Topics:       strFacets(topics, nil, Topics),
			Years:        intFacets(years),
		},
	}, nil
}

// strFacets 按 order 给的顺序出，order 里没有的丢掉（闭表在这里生效）。
func strFacets(counts map[string]int, _ []string, order []string) []Facet {
	out := make([]Facet, 0, len(order))
	for _, v := range order {
		if n := counts[v]; n > 0 {
			out = append(out, Facet{Value: v, Count: n})
		}
	}
	return out
}

// strFacetsSorted 给没有固定顺序的那一维（考试名）按数量倒排。
func strFacetsSorted(counts map[string]int) []Facet {
	out := make([]Facet, 0, len(counts))
	for v, n := range counts {
		out = append(out, Facet{Value: v, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Value < out[j].Value
	})
	return out
}

func intFacets(counts map[int]int) []Facet {
	out := make([]Facet, 0, len(counts))
	for v, n := range counts {
		out = append(out, Facet{Value: fmt.Sprint(v), Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Value < out[j].Value })
	return out
}

// ByID 取一道题。
func ByID(id string) (Prompt, bool) {
	load()
	if loadErr != nil {
		return Prompt{}, false
	}
	i := sort.Search(len(all), func(i int) bool { return all[i].ID >= id })
	if i < len(all) && all[i].ID == id {
		return all[i], true
	}
	return Prompt{}, false
}
