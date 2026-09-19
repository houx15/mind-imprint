package awakening

import (
	"fmt"
	"sort"
	"strings"

	"mindimprint/api/internal/interest"
	"mindimprint/api/internal/interests"
)

// selection.go —— 走完之后那一次选词调用，以及三种写回的判定。
//
// # 这一步为什么必须存在
//
// 迁移 0134 把树的词表关成了闭表：树上的词只能是
// packages/contracts/interests/interests.json 里那 249 条之一。那次迁移把生产库
// 里的词整表归档之后清空，理由写在它自己的注释里 —— 模型自由命名长出来的是
// 「例外与代表性」这种粒度的词，和「金融」「裁缝」混在一棵树上，那棵树说不清
// 自己是什么。
//
// 终端产出的全是自由文本（她八轮说的话、那个研究问题、几个学科名字）。它**不能
// 直接种进树**。这一步把自由文本翻译成闭表里的 id。
//
// # 复用哪一半、重写哪一半
//
// 解析器、闭表校验、逐字比对（interest.ParseHarvestReply / KeepGrounded）整个
// 复用 —— 一个解析器，两处不会对「这条能不能落库」有不同看法。
//
// **判据段落重写。** 这里有一条踩过的坑：兴趣测试第一版整条照搬了采集的 system
// prompt，把「不是这篇材料的话题」也带了过去，而测试里根本没有材料 —— 她说的
// 每个字都是她自己选择写下的。真模型因此 0/3 长出词，改掉判据之后 3/3
// （memory: quiz-borrowed-a-rule-that-is-backwards-2026-09-11）。
// **共用要按段落挑，不是整条照搬。**

// selectionRules 是这一次调用的判据段落。
//
// 和采集那份的区别只有一处，而那一处是反的：采集要防「材料的话题被当成她的
// 兴趣」，这里没有材料，她说的每个字都是她自己敲的。
const selectionRules = `怎么算选中一个领域：
- 她在这场谈话里**自己写下的文字**就是根据。这里没有别人指定的材料，
  每一句都是她选择说出来的，所以不需要再去分辨「这是材料的话题还是她的兴趣」。
- 看她的理由**落在哪一层**：同一件事，有人关心它怎么运作，有人关心它对谁有影响，
  有人关心它好不好看。她反复回到的那一层就是她在关心的方向。
- 作品名、游戏名、人名本身不是领域（《进击的巨人》不等于「动画」）。
  把她**说明原因的那些话**落到表里。
- 她后面几轮写的那个问题和那件作品设想，比第一轮的「我喜欢 X」更能说明方向。`

const selectionPromptTail = `
字段要求：
- id：**上表里的 id 原样照抄**，不要改写，不要翻译，不要自己发明。
- note：一句话，**直接对她说，用「你」**，讲这个领域在她身上是什么。不超过 40 字。
  🚨 这一句会原样显示在报告里她的名字下面。写成「她从……」是在她面前谈论她，
  照抄上面那句判据的口吻就会写成这样 —— 写「你从……」。
- evidence：**她自己写的原话，原样摘录**，一个字都不要改，不要总结，不要拼接
  两句话。找不到能作为根据的原话，就不要输出这一条。

宁可只给一个，也不要凑满三个。一个都选不出来就返回空数组。

只输出一个 JSON 对象，不要任何解释：
{"keywords":[{"id":"","note":"","evidence":""}]}`

const selectionPromptHead = `你在读一个中学生刚刚走完的一次兴趣探询。她被连续问了
八个问题：她最近主动靠近的是什么、其中哪个细节抓住了她、这个细节连着她的什么
经历、哪一点让她想不通、她想追问的问题、她还需要读什么、她目前的想法、她想做成
什么作品。

判断她正在关心哪几个领域，最多三个。

`

// BuildSelectionPrompt 拼出选词那一次调用的 system 与 user 两段。
//
// answers 是她八轮的原话，按节点顺序。它同时是后面 KeepGrounded 的语料，
// 所以这里进 prompt 的和那里比对的必须是**同一份文字**。
func BuildSelectionPrompt(answers []string, researchQuestion string) (system, user string) {
	system = selectionPromptHead + interests.PromptList() + "\n" + selectionRules + selectionPromptTail

	var b strings.Builder
	b.WriteString("她在这场谈话里说的话，按顺序：\n\n")
	for i, a := range answers {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		label := "第 " + fmt.Sprint(i+1) + " 问"
		if i < len(Nodes) {
			label = Nodes[i].Code
		}
		fmt.Fprintf(&b, "【%s】%s\n\n", label, a)
	}
	if q := strings.TrimSpace(researchQuestion); q != "" {
		fmt.Fprintf(&b, "她最后确定下来的问题：%s\n", q)
	}
	return system, b.String()
}

// Corpus 是逐字比对用的语料。
//
// 🚨 **只含她自己写下的文字。** 印记说过的话一个字都不进来 —— 幻引的来源就是
// 它的上文，把上文放进语料等于允许它引用自己编的句子，再声称那是她说的
// （memory: prompt-twice-then-make-it-checkable-2026-09-12）。
func Corpus(answers []string) string {
	return strings.Join(answers, "\n")
}

/* ── 三种写回 ───────────────────────────────────────────────────────────── */

// Verdict 是一个选中的词该怎么写回树。
type Verdict string

const (
	// VerdictConfirm 树上已经有这个词，这次它又出现了。加一条来源，强度上升。
	VerdictConfirm Verdict = "confirm"
	// VerdictGrow 闭表里的新词，她树上还没有。新长一个。
	VerdictGrow Verdict = "grow"
)

// Planted 是一个真的写回了树的词，带上它的判定。
//
// 报告要分开说这两件事：「这几个词你又说了一次，它们更确定了」和「这几个是
// 这次新长出来的」。合成一句「长出了 3 个词」，会让一个第五次走协议的学生
// 看到和第一次一模一样的句子。
// 🚨 json 标签不能省。这个结构整个进 awakening_report.payload，前端逐字段读
// 小驼峰。少了标签它会序列化成 `Zh` / `Evidence`，于是报告里每张卡都是空的 ——
// 而树上的词是对的，日志里也没有任何报错。2026-09-19 的接口走查抓到过一次。
type Planted struct {
	InterestID string  `json:"interestId"`
	Zh         string  `json:"zh"`
	En         string  `json:"en"`
	Field      string  `json:"field"`
	Note       string  `json:"note"`
	Evidence   string  `json:"evidence"`
	Verdict    Verdict `json:"verdict"`
	// Strength 是写回之后的强度读数。confirm 的那几个会比上次高。
	Strength int `json:"strength"`
}

// Classify 判定每个选中的词是 confirm 还是 grow。
//
// **不问模型。** 这是一次集合运算：选词结果里的 id 和她树上已有的 id 求交集，
// 交集是 confirm，其余是 grow。让模型判这件事，等于为一个查表就有答案的问题
// 花一次调用，并且给它一次判错的机会。
//
// known 是她树上已有的 interest id。
func Classify(hs []interest.Harvested, known []string) []Planted {
	have := make(map[string]bool, len(known))
	for _, id := range known {
		have[id] = true
	}
	out := make([]Planted, 0, len(hs))
	for _, h := range hs {
		it, ok := interests.ByID(h.InterestID)
		if !ok {
			// 解析器已经挡过一遍。这里再站一个人，理由同 plantKeywords：
			// 闭表是树的地基，值得重复。
			continue
		}
		v := VerdictGrow
		if have[h.InterestID] {
			v = VerdictConfirm
		}
		out = append(out, Planted{
			InterestID: h.InterestID,
			Zh:         it.Zh,
			En:         it.En,
			Field:      it.Field,
			Note:       h.Note,
			Evidence:   h.Evidence,
			Verdict:    v,
		})
	}
	return out
}

// StillOpen 是这次走完之后，她仍然一个词都没有的主枝。
//
// 它是第三种写回，而且是**不往库里写**的那一种：树上已经有空枝邀请那套界面
// （TreeView 的 inviteField），报告只要把这几根枝说出来，她点过去就能接上。
//
// 参数是她原来空着的枝，和这次新长出来的词 —— 这次在「艺术与表达」上长出了
// 一个词，那根枝就不再空着了。
func StillOpen(wasEmpty []string, planted []Planted) []string {
	filled := make(map[string]bool, len(planted))
	for _, p := range planted {
		if p.Verdict == VerdictGrow {
			filled[p.Field] = true
		}
	}
	out := make([]string, 0, len(wasEmpty))
	for _, f := range wasEmpty {
		if !filled[f] {
			out = append(out, f)
		}
	}
	return out
}

// FieldZh 把主枝 id 翻成中文名。查不到给原值 —— 一个显示成 "making" 的标签
// 难看，但比一个空标签说得出更多。
func FieldZh(field string) string {
	if zh, ok := fieldZh[field]; ok {
		return zh
	}
	return field
}

// DisciplineIDs 收集这几个词在词表里挂着的学科 id，按出现次数降序。
//
// 报告第五块的阅读推荐拿它去匹配 articles.json。**从词表查，不从模型的回话里
// 读** —— 模型给的是「心理学」「社会学」这种中文名字，而文章索引里存的是
// cognitive-psychology 这样的 id，两者对不上，照着中文名匹配的结果永远是零篇。
func DisciplineIDs(planted []Planted) []string {
	count := map[string]int{}
	for _, p := range planted {
		it, ok := interests.ByID(p.InterestID)
		if !ok {
			continue
		}
		for _, d := range it.Disciplines {
			count[d]++
		}
	}
	out := make([]string, 0, len(count))
	for d := range count {
		out = append(out, d)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if count[out[i]] != count[out[j]] {
			return count[out[i]] > count[out[j]]
		}
		return out[i] < out[j]
	})
	return out
}
