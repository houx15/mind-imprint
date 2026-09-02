package interest

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"mindimprint/api/internal/disciplines"
)

// How 是一条「词 → 学科」的边怎么来的。界面上要说得出「凭什么把这个词放进
// 统计推断」，所以来路本身是数据，不是日志。
const (
	HowAlias   = "alias"   // 别名直接命中。免费，且确定。
	HowCooccur = "cooccur" // 和一个已路由的词共享 >=2 条来源，继承它的学科。
	HowLLM     = "llm"     // 前两档都没命中时才花的那一次模型调用。
	HowStudent = "student" // 她自己改的。永远压过前三档。
)

// Confidence for each cheap tier. T1 是确定的；T2 是一个有根据的猜测，写成 0.6
// 是为了让界面能把「这是推断出来的」如实说出来，而不是把两档混成一样的自信。
const (
	confAlias   float32 = 1.0
	confCooccur float32 = 0.6
)

// cooccurMinShared 是共现继承的门槛。
//
// 一条共享来源不够：一篇文章里同时出现的两个词经常毫无关系（「珊瑚」和「代表
// 性」共享一篇阅读，但前者是话题，后者是方法）。两条才开始像同一条线索。
const cooccurMinShared = 2

// Route 是一条候选的「词 → 学科」边。
type Route struct {
	DisciplineID string
	Confidence   float32
	How          string
	Rationale    string
}

// Known 是她**已经**有的一个词，路由 T2 档要用。
type Known struct {
	KeywordNorm   string
	DisciplineIDs []string
	// SourceRefs 是这个词的来源标识（atom id 的字符串形式，或 news/quiz 的 id）。
	SourceRefs []string
}

// RouteCheap 跑免费的两档：先别名，再共现。
//
// **两档都没命中时返回 nil**，调用方据此决定要不要花那一次模型调用。返回空切片
// 和返回 nil 在这里是两件事，所以实现里不要「顺手」初始化成 `[]Route{}`。
func RouteCheap(text string, sourceRefs []string, known []Known) []Route {
	// T1 —— 别名。整个词直接命中一门学科。
	if d, ok := disciplines.MatchAlias(disciplines.Normalize(text)); ok {
		return []Route{{
			DisciplineID: d.ID,
			Confidence:   confAlias,
			How:          HowAlias,
			Rationale:    fmt.Sprintf("「%s」是%s的说法之一。", text, d.Zh),
		}}
	}

	// T2 —— 共现。和一个已经路由过的词共享足够多的来源，就继承它的学科。
	if len(sourceRefs) == 0 {
		return nil
	}
	mine := make(map[string]bool, len(sourceRefs))
	for _, r := range sourceRefs {
		if r != "" {
			mine[r] = true
		}
	}

	seen := map[string]bool{}
	var out []Route
	for _, k := range known {
		if k.KeywordNorm == disciplines.Normalize(text) {
			continue // 它自己
		}
		shared := 0
		for _, r := range k.SourceRefs {
			if mine[r] {
				shared++
			}
		}
		if shared < cooccurMinShared {
			continue
		}
		for _, id := range k.DisciplineIDs {
			if id == "" || seen[id] {
				continue
			}
			d, ok := disciplines.ByID(id)
			if !ok {
				continue // 数据里留下的旧 id，静默跳过好过造一门不存在的学科
			}
			seen[id] = true
			out = append(out, Route{
				DisciplineID: d.ID,
				Confidence:   confCooccur,
				How:          HowCooccur,
				Rationale: fmt.Sprintf("它和你已有的「%s」出自同样的 %d 处来源。",
					k.KeywordNorm, shared),
			})
		}
	}
	return out
}

const routeSystemPrompt = `你在给一个中学生的兴趣关键词找它属于哪一门学科。

规则：
1. 只能从候选清单里选，用清单给的 id。清单以外的学科一律不选。
2. 最多选 2 门，通常 1 门就够。宁可少选，不要凑。
3. 判断依据首先是「学生自己那句话」，其次才是词面 —— 同一个词在不同的话里
   可以属于不同学科。
4. 一门都不合适时返回空数组。硬塞一门比留空更糟。
5. 只输出一个 JSON 对象，不要解释、不要代码块以外的任何文字：
   {"routes":[{"id":"<候选 id>","why":"<一句话，不超过 30 字>"}]}`

// BuildRoutePrompt 拼出 T3 档的 system 与 user 两段。
//
// **候选只带同一根主枝下的六门，不带全部 42 门。** 这样 prompt 小、便宜、快，
// 而且判错时错误被关在一根枝的范围内 —— 一个把「记忆怎么形成」放错到神经科学
// 隔壁的模型是可以接受的，一个把它放到艺术史的模型会让整棵树失去可信度。
//
// field 不是七根主枝之一时，退回全表候选：这只会发生在采集器出了 bug 的情况下，
// 而那时给一个更差的候选表，好过直接不路由、让这个词永远悬着。
func BuildRoutePrompt(text, evidence, field string) (system, user string) {
	cands := disciplines.ByField(field)
	if len(cands) == 0 {
		cands = disciplines.All()
	}
	var b strings.Builder
	b.WriteString("候选学科：\n")
	for _, d := range cands {
		fmt.Fprintf(&b, "- %s ｜ %s（%s）｜ 它研究：%s ｜ 方法：%s\n",
			d.ID, d.Zh, d.En, d.Asks, d.Method)
	}
	fmt.Fprintf(&b, "\n学生的关键词：%s\n", text)
	if strings.TrimSpace(evidence) != "" {
		fmt.Fprintf(&b, "学生自己那句话：%s\n", evidence)
	}
	return routeSystemPrompt, b.String()
}

type routeReply struct {
	Routes []struct {
		ID  string `json:"id"`
		Why string `json:"why"`
	} `json:"routes"`
}

// ParseRouteReply 读模型的回话。
//
// 三件事按这个顺序发生，顺序本身是设计：
//
//  1. 解析不出 JSON → **报错**。调用方据此不长任何边。绝不返回一个像样的假路由
//     ——一条编出来的「你的兴趣属于艺术史」比没有路由伤害大得多。
//  2. id 不在学科表里 → 丢掉那一条。模型偶尔会发明 "astrology"。
//  3. id 存在但不属于这根主枝 → 也丢掉。候选表里本来就没有它，出现即幻觉。
//
// 全部条目都被丢掉时返回空切片和 nil error：模型给了合法回话，只是没有一门合适。
func ParseRouteReply(raw, field string) ([]Route, error) {
	body, err := sliceJSONObject(raw)
	if err != nil {
		return nil, err
	}
	var rep routeReply
	if err := json.Unmarshal(body, &rep); err != nil {
		return nil, fmt.Errorf("route reply is not the expected object: %w", err)
	}

	out := make([]Route, 0, len(rep.Routes))
	for _, r := range rep.Routes {
		d, ok := disciplines.ByID(strings.TrimSpace(r.ID))
		if !ok {
			continue
		}
		if disciplines.IsField(field) && d.Field != field {
			continue
		}
		out = append(out, Route{
			DisciplineID: d.ID,
			Confidence:   0.8,
			How:          HowLLM,
			Rationale:    strings.TrimSpace(r.Why),
		})
		if len(out) == 2 { // 提示词里说了最多两门；这里不指望模型守规矩。
			break
		}
	}
	return out, nil
}

// sliceJSONObject 从一段可能带 code fence、带前后废话的回话里取出 JSON 对象。
//
// 和 internal/api/reading_questions.go 的 parseReadingQuestionsReply 同一套办法：
// 网关层没有 schema 强约束的 JSON 模式，所以每个调用方自己做一次容错切片。
func sliceJSONObject(raw string) ([]byte, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, errors.New("empty model reply")
	}
	start, end := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("no JSON object in model reply (%d bytes)", len(s))
	}
	return []byte(s[start : end+1]), nil
}
