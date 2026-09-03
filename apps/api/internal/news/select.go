package news

import (
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/disciplines"
)

// select.go —— 从候选池里挑五颗星，并把每一颗写成学生看得懂的样子。
//
// **一天一次调用。** 抓取、去重、过滤、排序都在前面做完了，模型只负责它唯一
// 做得比规则好的那件事：判断哪五条对一个中学生**有意思**，以及怎么用两句话
// 把一篇 Nature 摘要说清楚。

// PlanetCount 是一天几颗星。
//
// 五。原型定的数，而它是对的：这一屏的承诺是「五条今天值得知道的事」，不是一个
// 信息流。一个刷不完的列表会把「每天来看一眼」变成「每天在这里待着」，那是另一
// 种产品。
const PlanetCount = 5

// maxCandidates 是最多送几条进 prompt。
//
// 候选池一天大概五六十条。全塞进去既贵又没用 —— 模型在第四十条之后就开始
// 敷衍。按新鲜度排完取前 40 条。
const maxCandidates = 40

// Planet 是星图上的一颗星。
type Planet struct {
	// 候选池里的下标，模型用它指认自己挑了哪条。
	Index int
	// TitleZh 是给学生看的标题：**重写过的**，不是原标题的翻译。
	TitleZh string
	TitleEn string
	// Summary 两句话，说清楚发生了什么以及为什么值得知道。
	Summary string
	// Hook 是一个她可以立刻追问的问题 —— 这一屏的钩子。
	Hook string
	// Field 是七根主枝之一；DisciplineID 是学科表里的一条。
	Field        string
	DisciplineID string
	// Keyword 是这条新闻的兴趣关键词。她收藏这颗星时，它会被种进树。
	Keyword string
}

const selectSystemPrompt = `你在为一个中学生挑今天值得知道的五条科学新闻，并把它们
写成她看得懂的样子。她 15-18 岁，读国际课程（IB / A-Level / AP）。

挑的标准，按重要性排：
- **能引出一个她可以自己追问的问题**。一条只能被记住、不能被追问的新闻不要。
- **五条必须落在至少三根不同的主枝上**（见下面 field 的七选一）。候选里本来就
  有人文、社会、艺术、心理类的条目，请真的用上它们 —— 五条全是自然科学不合格。
- 具体的发现优于综述，有数字、有方法、有争议的优于「科学家表示」。
- 不要政治、战争、灾难报道。不要健康建议类的软文。

写的要求：
- titleZh：**重写**，不是翻译。一句中文，20 字以内，说出这件事本身。
  好：「深海珊瑚在 30 度水里活下来了」。差：「研究人员发现珊瑚耐热性新机制」。
- titleEn：原标题即可。
- summary：两句话。第一句发生了什么，第二句为什么这值得知道。不超过 80 字。
- hook：**一个她能追问的问题**，不是一句感叹。以问号结尾。
  好：「四平方公里的珊瑚，能代表一整片海吗？」。差：「是不是很神奇？」
- field：七选一 —— formal（数学与形式）/ science（科学与自然）/ making（技术与创造）
  / society（社会与世界）/ humanities（人文与写作）/ arts（艺术与表达）/ self（自我与成长）
  按**这条新闻在问什么**判，不是按它发在哪个网站。
- disciplineId：从候选学科 id 里选一个最贴的。
- keyword：4-10 字的中文，是**她会关心的问题或方法**，不是话题标签。
  好：「样本代表性」「耐热机制」。差：「珊瑚」「气候」。

只输出一个 JSON 对象，不要任何解释：
{"planets":[{"index":0,"titleZh":"","titleEn":"","summary":"","hook":"","field":"","disciplineId":"","keyword":""}]}`

// BuildSelectPrompt 拼出选星用的 system 与 user 两段，**并把真正写进 prompt 的
// 那批候选一起返回**。
//
// 🚨 第三个返回值不是为了方便：候选在这里会被截到 maxCandidates，而
// `ParseSelectReply` 要按下标回查链接与出处。调用方如果把**完整的池子**传给解析器，
// 下标的含义就和 prompt 里的不一样了。让这个函数交出它实际描述过的那个切片，
// 这一整类错位就不可能发生。
//
// 候选学科**全表都给**（42 条，只给 id 与中文名，不给方法与考纲）：这一步和
// 关键词路由不同 —— 路由时我们已经知道那个词属于哪根枝，所以只给同枝六门；
// 这里主枝本身就是待判定的，限定候选等于替模型先做了那个判断。
func BuildSelectPrompt(items []Item) (system, user string, candidates []Item) {
	if len(items) > maxCandidates {
		items = items[:maxCandidates]
	}
	var b strings.Builder
	b.WriteString("候选学科（id · 中文名）：\n")
	for _, d := range disciplines.All() {
		fmt.Fprintf(&b, "%s · %s\n", d.ID, d.Zh)
	}
	fmt.Fprintf(&b, "\n今天的候选新闻，共 %d 条。请挑 %d 条：\n\n", len(items), PlanetCount)
	for i, it := range items {
		fmt.Fprintf(&b, "[%d] (%s) %s\n", i, it.Source, it.Title)
		if s := strings.TrimSpace(it.Summary); s != "" {
			fmt.Fprintf(&b, "    %s\n", truncRunes(s, 220))
		}
	}
	return selectSystemPrompt, b.String(), items
}

type selectReply struct {
	Planets []struct {
		Index        int    `json:"index"`
		TitleZh      string `json:"titleZh"`
		TitleEn      string `json:"titleEn"`
		Summary      string `json:"summary"`
		Hook         string `json:"hook"`
		Field        string `json:"field"`
		DisciplineID string `json:"disciplineId"`
		Keyword      string `json:"keyword"`
	} `json:"planets"`
}

// ParseSelectReply 读选星的回话。
//
// 丢弃规则，按这个顺序：
//
//  1. 解析不出 JSON → **报错**，今天不出星图。绝不用昨天的冒充今天的，也绝不
//     编五条新闻（memory: ai-errors-must-surface-never-fake）。
//  2. index 越界 → 丢。模型偶尔会指一个不存在的候选。
//  3. titleZh 或 hook 为空 → 丢。一颗没有钩子的星球是一条只能被记住的新闻。
//  4. field 不是七根主枝之一 → 丢。
//  5. 模型自己写出来的中文里带政治信号 → 丢。抓取那一层看的是英文原标题，
//     漏得掉；中文重写漏不掉。
//  6. disciplineId 不在学科表里 → **不丢这颗星，只清空这条边**。学科连错比
//     没连上糟，但为了一条连错的边扔掉一条好新闻更糟。
//  6. 同一个 index 重复 → 只留第一个。
//  7. 超过五颗 → 截断。
func ParseSelectReply(raw string, candidates []Item) ([]Planet, error) {
	body, err := sliceJSONObject(raw)
	if err != nil {
		return nil, err
	}
	var rep selectReply
	if err := json.Unmarshal(body, &rep); err != nil {
		return nil, fmt.Errorf("select reply is not the expected object: %w", err)
	}

	out := make([]Planet, 0, PlanetCount)
	seen := map[int]bool{}
	for _, p := range rep.Planets {
		zh := strings.TrimSpace(p.TitleZh)
		hook := strings.TrimSpace(p.Hook)
		if zh == "" || hook == "" || !disciplines.IsField(p.Field) {
			continue
		}
		// 🚨 **不信任模型给的下标。** 2026-09-03 实测：模型描述了五条真实存在的
		// 新闻，却把它们一律编号成 0,1,2,3,4 —— 下标指向的候选和它自己写的标题
		// 毫不相干。后果是每颗星球挂着一篇**无关文章**的链接与出处，而学生点
		// 「读原文」就落在那篇上。
		//
		// 所以按 titleEn（prompt 要求填原标题）回查真正的那一条；查不到就
		// **丢掉这颗星**。四颗真的星，好过五颗里有一颗指向随机文章。
		idx := anchorByTitle(p.TitleEn, candidates)
		if idx < 0 || seen[idx] {
			continue
		}
		// 🚨 **对模型自己的产物再过一遍政治过滤。** 抓取那一层看的是英文原标题，
		// 而一条政治新闻的英文标题可能一个信号词都不含 —— 实测漏过一条
		// "Venice Biennale President Defends Russia Inclusion"，它的中文改写却是
		// 「文化制裁边界」，制裁两个字明明白白。
		//
		// 模型的中文重写常常比英文原标题更直白地暴露这条新闻在谈什么，所以这里
		// 是第二道、也是更灵敏的一道闸。
		if IsPolitical(zh+" "+p.Keyword, p.Summary+" "+hook) {
			continue
		}
		did := strings.TrimSpace(p.DisciplineID)
		if _, ok := disciplines.ByID(did); !ok {
			did = ""
		}
		seen[idx] = true
		out = append(out, Planet{
			Index:        idx,
			TitleZh:      zh,
			TitleEn:      strings.TrimSpace(p.TitleEn),
			Summary:      strings.TrimSpace(p.Summary),
			Hook:         hook,
			Field:        p.Field,
			DisciplineID: did,
			Keyword:      strings.TrimSpace(p.Keyword),
		})
		if len(out) == PlanetCount {
			break
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("select reply 里没有一颗可用的星球")
	}
	return out, nil
}

// sliceJSONObject 从模型的回话里切出那个 JSON 对象：去掉代码围栏，取第一个
// `{` 到最后一个 `}`。没有 schema 强制的 JSON 模式，所以这一层必须容错。
func sliceJSONObject(raw string) ([]byte, error) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	i := strings.Index(s, "{")
	j := strings.LastIndex(s, "}")
	if i < 0 || j < i {
		return nil, fmt.Errorf("reply 里没有 JSON 对象")
	}
	return []byte(s[i : j+1]), nil
}

func truncRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}


/* ── 按标题回查候选 ─────────────────────────────────────────────────────── */

// titleMatchThreshold 是判定「同一条新闻」所需的词重合度。
//
// 0.5：模型经常把长标题截短或去掉副标题，所以不能要求全等；但低于一半重合就
// 已经是两条不同的新闻了。宁可丢掉一颗星，也不要挂错一个链接。
const titleMatchThreshold = 0.5

// anchorByTitle 在候选里找出模型实际描述的那一条，返回它的下标；找不到返回 -1。
//
// 先试归一化后的完全相等与包含（最常见的情形：模型原样抄了标题，或者截短了
// 它）；都不中时退到词集合的重合度。
func anchorByTitle(titleEn string, candidates []Item) int {
	want := normalizeTitle(titleEn)
	if want == "" {
		return -1
	}
	// 一轮：归一化相等或互相包含。
	for i, c := range candidates {
		got := normalizeTitle(c.Title)
		if got == "" {
			continue
		}
		if got == want || strings.Contains(got, want) || strings.Contains(want, got) {
			return i
		}
	}
	// 二轮：词重合度最高的那条，且要过阈值。
	best, bestScore := -1, 0.0
	wantWords := wordSet(titleEn)
	if len(wantWords) == 0 {
		return -1
	}
	for i, c := range candidates {
		if sc := overlap(wantWords, wordSet(c.Title)); sc > bestScore {
			best, bestScore = i, sc
		}
	}
	if bestScore < titleMatchThreshold {
		return -1
	}
	return best
}

func wordSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(s)) {
		w = strings.Trim(w, ".,:;!?\"'()[]—-")
		// 一两个字母的词（a / of / in）不承载信息，只会把重合度虚抬上去。
		if len([]rune(w)) > 2 {
			out[w] = true
		}
	}
	return out
}

// overlap 是 |A∩B| / |A| —— 以模型给的标题为分母，因为它常常是候选标题的一个
// 截断版本，用并集当分母会把这种正确匹配压到阈值以下。
func overlap(a, b map[string]bool) float64 {
	if len(a) == 0 {
		return 0
	}
	n := 0
	for w := range a {
		if b[w] {
			n++
		}
	}
	return float64(n) / float64(len(a))
}
