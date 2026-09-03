package interest

import (
	"encoding/json"
	"fmt"
	"strings"
)

// dig.go —— 「继续深挖」：一个关键词后面的四颗种子。
//
// # 它补的是哪个洞
//
// 兴趣树的抽屉能证明一个词是从哪来的（她自己的原话、可点回去的来源），但它到此
// 为止 —— 一个模型对她有观察，却对「那接下来干嘛」一无所知。
//
// 原型在这里摆过四个动词：再读一篇 / 写一篇 / 做个项目 / 问印记。**四个空动词
// 摆在一个真的观察后面，教的是这个模型没有真的在看她。** 所以这四颗种子必须是
// 具体的、从她**这个词的真实来源**里长出来的：一个她还答不上来的问题、一篇具体
// 该读的东西、一段她可以写的、一个真能存在的项目。
//
// # 四颗，四种，各一颗
//
// 不是「最多四颗」而是**四种各一颗**：她需要的是四个不同方向的出口，不是四个
// 同类的建议。少了任何一种就少一个出口，所以解析器要求四种齐全（缺的丢掉，
// 但至少要有一颗才算成功）。

// DigKind 是一颗种子的类型。三种能直接变成 lite 里的一件真东西
// （阅读 / 写作 / 项目都是一个字段的 create），想一想不能 —— 它是一个
// **拿着走的问题**，不是一个可以点出去的任务。假装它是，就又变回四个空动词了。
type DigKind string

const (
	DigThink DigKind = "think" // 想一想：一个她现在还答不上来的问题
	DigRead  DigKind = "read"  // 去读：一篇具体该读的东西
	DigWrite DigKind = "write" // 去写：一段她可以写的
	DigMake  DigKind = "make"  // 去做：一个真能存在的小项目
)

var digKinds = map[DigKind]bool{DigThink: true, DigRead: true, DigWrite: true, DigMake: true}

// DigSeedCount 是齐整的一套有几颗。
const DigSeedCount = 4

// Seed 是一颗种子。
type Seed struct {
	Kind DigKind
	// Text 是种子本身。**它会被当作标题/立意直接送进创建接口**，所以它必须是
	// 一句能独立成立的话，不是一个片段。
	Text string
	// Why 是一句「为什么是你」——把这颗种子和她这个词的来源连起来。
	Why string
}

const digSystemPrompt = `一个中学生的兴趣树上有一个关键词。下面给你这个词、我们对它的
一句话理解、以及**她自己写下的、让这个词出现的那几句话**。

请给出四颗「继续深挖」的种子，四种各一颗：

- think 想一想：一个她**现在还答不上来**的问题。不是复习题，是那种想起来会卡住的
  问题。以问号结尾。
- read  去读：一篇具体该读的东西。写成一个**可以直接当阅读标题**的句子，
  比如「珊瑚白化到底是死了还是把藻吐出去了」。不要写「找一些关于 X 的资料」。
- write 去写：一段她可以写的。写成一个**可以直接当写作立意**的句子。
- make  去做：一个真能存在的小项目。要她两周内做得完、用她手边的东西做得了。

四颗都必须**扣住她自己写的那几句话**，不是围着这个词泛泛地想。每颗附一句 why，
说清楚为什么是给她的（可以引用她的原话）。

字数：text 不超过 30 字；why 不超过 40 字。

只输出一个 JSON 对象，不要任何解释：
{"seeds":[{"kind":"think","text":"","why":""},{"kind":"read","text":"","why":""},{"kind":"write","text":"","why":""},{"kind":"make","text":"","why":""}]}`

// BuildDigPrompt 拼出深挖用的 system 与 user 两段。
//
// evidences 是她在这个词上留下的每一句原话（来自 keyword_source.evidence）。
// **它们是这次调用唯一真正重要的输入** —— 没有它们，模型只能围着一个词泛泛地
// 想，而那正是原型那四个空动词的来源。
func BuildDigPrompt(textZh, note string, evidences []string) (system, user string) {
	var b strings.Builder
	fmt.Fprintf(&b, "关键词：%s\n", textZh)
	if n := strings.TrimSpace(note); n != "" {
		fmt.Fprintf(&b, "我们对它的理解：%s\n", n)
	}
	b.WriteString("\n她自己写下的、让这个词出现的话：\n")
	if len(evidences) == 0 {
		// 不该发生（evidence 是 NOT NULL 且应用层挡空串），但如果发生了，
		// 说实话比编一句更好。
		b.WriteString("（没有留下原话）\n")
	}
	for _, e := range evidences {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		fmt.Fprintf(&b, "- 「%s」\n", truncRunes(e, 200))
	}
	return digSystemPrompt, b.String()
}

type digReply struct {
	Seeds []struct {
		Kind string `json:"kind"`
		Text string `json:"text"`
		Why  string `json:"why"`
	} `json:"seeds"`
}

// ParseDigReply 读深挖的回话。
//
// 丢弃规则：
//
//  1. 解析不出 JSON → **报错**，这个词就没有种子。绝不摆四个通用动词顶上 ——
//     那正是这套东西要取代的东西。
//  2. kind 不是四种之一 → 丢。
//  3. text 为空 → 丢。它会被当作标题直接送进创建接口，空的会建出一个无名的东西。
//  4. 同一种重复 → 只留第一颗。四种各一颗。
func ParseDigReply(raw string) ([]Seed, error) {
	body, err := sliceJSONObject(raw)
	if err != nil {
		return nil, err
	}
	var rep digReply
	if err := json.Unmarshal(body, &rep); err != nil {
		return nil, fmt.Errorf("dig reply is not the expected object: %w", err)
	}

	out := make([]Seed, 0, DigSeedCount)
	seen := map[DigKind]bool{}
	for _, s := range rep.Seeds {
		k := DigKind(strings.TrimSpace(s.Kind))
		text := strings.TrimSpace(s.Text)
		if !digKinds[k] || seen[k] || text == "" {
			continue
		}
		seen[k] = true
		out = append(out, Seed{Kind: k, Text: text, Why: strings.TrimSpace(s.Why)})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("dig reply 里没有一颗可用的种子")
	}
	return out, nil
}

// IsDigKind 供落库前的边界校验用（kind 列上有 CHECK 约束）。
func IsDigKind(s string) bool { return digKinds[DigKind(s)] }
