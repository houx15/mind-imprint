package interest

import (
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/prompts"
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
//
// # 判据是「看法有可能变」（2026-09-07）
//
// 四颗种子原来只要求「具体、扣住她的原话」。具体是够了，但一颗具体的种子照样
// 可以只让她把已经知道的再说一遍 —— 「电池还有哪些应用」是具体的，也是无用的。
//
// 所以 prompt 现在给每一种都写了一条**方向**：想一想指着她原话里的一个前提，
// 去读换一个可能不同意她的立场，去写要求一个她得辩护的判断，去做要产出一份能
// 拿去检验她想法的证据。同一件事在探索地图那边也做了一遍（`news/select.go` 的
// hook）—— 两处都是同一条：问题要打在这个说法上，不是把它当跳板。

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
	//
	// 🚨 read 那一颗例外：它的 Text 由服务端用 LibrarySlug 那篇文章的**真标题**
	// 覆盖掉，模型写的那句不算数。见 ParseDigReply。
	Text string
	// Why 是一句「为什么是你」——把这颗种子和她这个词的来源连起来。
	Why string
	// LibrarySlug 只在 read 那一颗上非空：分级阅读库里真的有的那一篇。
	LibrarySlug string
}

// LibraryCandidate 是送进 prompt 的一篇候选文章。
//
// 候选由服务端按她的兴趣从 internal/library 里算出来 —— 模型**只在这份名单里
// 挑**，挑不中就没有「去读」那一颗。
type LibraryCandidate struct {
	Slug string
	// Title 是给模型看的标题。中文标题有就用中文的，她读到的也是这一个。
	Title string
	// Reason 是目录里那句「这篇讲什么」，帮模型判断它和这个词有没有关系。
	Reason string
}

const digSystemPrompt = prompts.InterestDigSystemPrompt

// BuildDigPrompt 拼出深挖用的 system 与 user 两段。
//
// evidences 是她在这个词上留下的每一句原话（来自 keyword_source.evidence）。
// **它们是这次调用唯一真正重要的输入** —— 没有它们，模型只能围着一个词泛泛地
// 想，而那正是原型那四个空动词的来源。
// candidates 是「去读」那一颗能挑的全部文章。空名单时 prompt 里会明说一句
// 「这次没有可挑的文章」，模型因此该省略那一颗 —— 而解析那一侧不认任何 slug，
// 所以它编一个出来也进不来。
func BuildDigPrompt(textZh, note string, evidences []string, candidates []LibraryCandidate) (system, user string) {
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

	b.WriteString("\n【阅读库里的文章】（去读那一颗只能从这里挑，按 slug）\n")
	if len(candidates) == 0 {
		b.WriteString("（这次一篇都没有。请省略 read 那一颗。）\n")
	}
	for _, c := range candidates {
		fmt.Fprintf(&b, "- %s ｜ %s", c.Slug, truncRunes(strings.TrimSpace(c.Title), 80))
		if reason := strings.TrimSpace(c.Reason); reason != "" {
			fmt.Fprintf(&b, " ｜ %s", truncRunes(reason, 80))
		}
		b.WriteString("\n")
	}
	return digSystemPrompt, b.String()
}

type digReply struct {
	Seeds []struct {
		Kind string `json:"kind"`
		Text string `json:"text"`
		Why  string `json:"why"`
		Slug string `json:"slug"`
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
//  5. **read 那一颗的 slug 必须在 candidates 里**，否则整颗丢掉；它的 text 一律
//     用那篇文章的真标题覆盖，模型写的那句不算数。
//
// 第 5 条是 2026-09-16 加的，为的是让「只推荐库里有的文章」成为一条**能验的**
// 判据而不是 prompt 里的一句话（[[prompt-output-must-be-verifiable-2026-09-03]]）。
// 模型编一个 slug、或者把标题写成一篇不存在的论文，结果都一样：这一颗不存在。
// 三颗种子是正常结果 —— 产品负责人的原话是「if there is not suitable ones,
// then we don't recommend. don't fake these articles.」
func ParseDigReply(raw string, candidates []LibraryCandidate) ([]Seed, error) {
	body, err := sliceJSONObject(raw)
	if err != nil {
		return nil, err
	}
	var rep digReply
	if err := json.Unmarshal(body, &rep); err != nil {
		return nil, fmt.Errorf("dig reply is not the expected object: %w", err)
	}

	titleOf := make(map[string]string, len(candidates))
	for _, c := range candidates {
		if slug := strings.TrimSpace(c.Slug); slug != "" {
			titleOf[slug] = strings.TrimSpace(c.Title)
		}
	}

	out := make([]Seed, 0, DigSeedCount)
	seen := map[DigKind]bool{}
	for _, s := range rep.Seeds {
		k := DigKind(strings.TrimSpace(s.Kind))
		if !digKinds[k] || seen[k] {
			continue
		}
		seed := Seed{Kind: k, Text: strings.TrimSpace(s.Text), Why: strings.TrimSpace(s.Why)}
		if k == DigRead {
			slug := strings.TrimSpace(s.Slug)
			title, known := titleOf[slug]
			if !known || title == "" {
				// 库里没有这一篇 —— 这一颗整个不存在。
				continue
			}
			seed.LibrarySlug, seed.Text = slug, title
		}
		if seed.Text == "" {
			continue
		}
		seen[k] = true
		out = append(out, seed)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("dig reply 里没有一颗可用的种子")
	}
	return out, nil
}

// IsDigKind 供落库前的边界校验用（kind 列上有 CHECK 约束）。
func IsDigKind(s string) bool { return digKinds[DigKind(s)] }
