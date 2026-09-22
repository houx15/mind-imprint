package awakening

import (
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/interest"
	"mindimprint/api/internal/prompts"
)

// report.go —— 走完之后那份报告。
//
// # 报告里哪些字是她的，哪些字是模型的
//
// 这是这个文件最要紧的一条线，所以写在最前面：
//
//	她的问题     她在 NODE 05 敲的那句话，**原样**              她的
//	她的作品设想 她在 NODE 08 敲的那句话，**原样**              她的
//	她在追什么   闭表里的词 + 她的原话作 evidence               她的
//	怎么靠近     天赋卡牌那一屏她自己的分堆                      她的
//	下一步阅读   articles.json 里真实存在的文章                  库里的
//	可能的驱动力 一次 compose 调用                              模型的
//	一句话总结   同一次调用                                      模型的
//
// 只有最后两行是模型写的，而它们在界面上都带着「这是推测」的说法。**她的问题
// 绝不让模型改写** —— 那是她自己想出来的一句话，重写一遍只会让她看到一个更
// 漂亮但不是她的句子，然后不知道该认哪一个
// （memory: prompt-twice-then-make-it-checkable-2026-09-12 里她的原话：
// 「我不知道该听它的还是按我现在的正文来」）。
//
// # 因此这一次调用很小
//
// 它只要两样东西：几条驱动力假设，和一句总结。参考设计让同一次调用连研究问题
// 和阅读地图一起生成，那是在让模型替她写她已经写过的东西。

// ReportVersion 是 payload 的结构版本。
//
// 一份已经生成的报告不该因为我们后来改了结构就变成另一份，所以读的时候按它
// 分支，而不是假设库里每一行都是今天的形状。
const ReportVersion = 1

// Driver 是一条驱动力假设。
//
// Confidence 会显示给她看，而且用的是「可能」这种说法 —— 它是一条假设，
// 不是一个结论。Evidence 是她的原话，逐字可查，所以她能自己判断这条推测
// 立不立得住。
type Driver struct {
	Label      string  `json:"label"`
	Evidence   string  `json:"evidence"`
	Confidence float64 `json:"confidence"`
}

// ReadingPick 是报告推荐的一篇真实文章。
type ReadingPick struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	ZhTitle string `json:"zhTitle"`
	Field   string `json:"field"`
	Tier    int    `json:"tier"`
	// Why 是命中的学科 id。**空的不会出现在报告里** —— 一条没有交集的推荐是
	// 补位，而报告这一块的承诺是「按你刚说的东西挑的」。
	Why []string `json:"why"`
}

// TalentPile 是天赋卡牌的一堆。
type TalentPile struct {
	Key   string   `json:"key"` // energy | learned | latent
	Label string   `json:"label"`
	Cards []string `json:"cards"` // 卡的 id
}

// Diff 是「和上次比」。只有第二趟起才有。
type Diff struct {
	// Stronger 是这次又出现、因此强度上升的词（中文名）。
	Stronger []string `json:"stronger"`
	// New 是这次新长出来的词。
	New []string `json:"new"`
	// PreviousQuestion 是上一趟她写下的那个问题，原样。
	PreviousQuestion string `json:"previousQuestion"`
	// DaysBetween 是两趟之间隔了多少天。
	DaysBetween int `json:"daysBetween"`
}

// Report 是整份报告，整体存进 awakening_report.payload。
type Report struct {
	Version   int    `json:"version"`
	AttemptNo int    `json:"attemptNo"`
	Navigator string `json:"navigator"`

	// Pursuing 是这次动了的词，带 confirm / grow 判定。可能是空的 ——
	// 她写得太少时一个词都长不出来，报告要照实说，不补
	// （memory: ai-errors-must-surface-never-fake）。
	Pursuing []Planted `json:"pursuing"`

	// Drivers 是模型给的驱动力假设。可能是空的。
	Drivers []Driver `json:"drivers"`

	// Question 是她自己写的那个问题，原样。
	Question string `json:"question"`
	// WorkConcept 是她自己写的作品设想，原样。
	WorkConcept string `json:"workConcept"`

	// Talent 是她自己的分堆。
	Talent []TalentPile `json:"talent"`

	// Readings 是从库里真的挑出来的文章。空表示库里没有对得上的。
	Readings []ReadingPick `json:"readings"`
	// OpenFields 是她仍然一个词都没有的主枝（field id）。
	OpenFields []string `json:"openFields"`

	// SelectionFailed 说的是「选词这一步没跑成」，不是「她没有可落的词」。
	//
	// 🚨 这两件事在报告上必须说不一样的话。Pursuing 为空有两种来路：模型读完
	// 她的八段话确实没挑出闭表里的词（实情，照实说），或者调用失败 / 回话读
	// 不懂（我们的故障）。2026-09-19 线上实测到后者：选词那次回了写坏的 JSON，
	// 报告于是对一个写了八段具体经历的学生说「你写下的内容里还没有足够具体的
	// 原话可以作为根据」—— 那是在冤枉她。
	SelectionFailed bool `json:"selectionFailed"`

	// Summary 是模型写的那一句话。失败时为空串，界面据此少显示一块，
	// 而不是填一句像样的话进去。
	Summary string `json:"summary"`

	// Diff 只有第二趟起才有。
	Diff *Diff `json:"diff,omitempty"`

	// Answers 是她八轮的原话。留在 payload 里，因为报告要引用它，而引用的那
	// 几句必须和当时说的一模一样 —— 去库里重查一次也行，但那样一份分享出去的
	// 报告就依赖另一张表还在。
	Answers []string `json:"answers"`
}

/* ── 那一次 compose 调用 ────────────────────────────────────────────────── */

const reportSystemPrompt = prompts.AwakeningReportSystemPrompt

// BuildReportPrompt 拼出报告那一次调用的 system 与 user 两段。
func BuildReportPrompt(answers []string) (system, user string) {
	var b strings.Builder
	b.WriteString("学生的回答，按顺序：\n\n")
	for i, a := range answers {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		label := fmt.Sprintf("第 %d 问", i+1)
		if i < len(Nodes) {
			label = Nodes[i].Code + " · " + Nodes[i].Objective
		}
		fmt.Fprintf(&b, "【%s】\n%s\n\n", label, a)
	}
	return reportSystemPrompt, b.String()
}

type reportReply struct {
	Drivers []struct {
		Label      string  `json:"label"`
		Evidence   string  `json:"evidence"`
		Confidence float64 `json:"confidence"`
	} `json:"drivers"`
	Summary string `json:"summary"`
}

// maxDrivers 是报告里最多几条驱动力假设。
//
// 三条。五条假设摆在一起，她读到第四条时已经不再把它们当成假设，而是当成
// 一份对她的描述 —— 而这一块恰恰是整份报告里最不确定的部分。
const maxDrivers = 3

// driverEvidenceMinRunes 和采集那边同一个门槛：太短的 evidence 是敷衍。
const driverEvidenceMinRunes = 4

// ParseReportReply 读报告那一次的回话。
//
// 丢弃规则和 interest.ParseHarvestReply 一致，理由也一致：
//
//  1. 解析不出 JSON → 报错。调用方这时把 Drivers 和 Summary 留空，
//     报告照样生成 —— 一份报告绝不该卡在散文上（同 ensureAtomReport）。
//  2. evidence 空或太短 → 丢这一条。
//  3. confidence 夹到 0..1。
//  4. 超过三条 → 截断。
//
// **这里不做逐字比对**：那一步在 KeepGroundedDrivers 里，和采集一样分开两段，
// 因为语料由调用方提供，而解析器不该知道语料是什么。
func ParseReportReply(raw string) ([]Driver, string, error) {
	body, err := sliceJSON(raw)
	if err != nil {
		return nil, "", err
	}
	var rep reportReply
	if err := json.Unmarshal(body, &rep); err != nil {
		return nil, "", fmt.Errorf("report reply is not the expected object: %w", err)
	}
	out := make([]Driver, 0, maxDrivers)
	for _, d := range rep.Drivers {
		ev := strings.TrimSpace(d.Evidence)
		if runeLen(ev) < driverEvidenceMinRunes {
			continue
		}
		c := d.Confidence
		if c < 0 {
			c = 0
		}
		if c > 1 {
			c = 1
		}
		out = append(out, Driver{
			Label:      truncRunes(strings.TrimSpace(d.Label), 40),
			Evidence:   ev,
			Confidence: c,
		})
		if len(out) == maxDrivers {
			break
		}
	}
	return out, truncRunes(strings.TrimSpace(rep.Summary), 120), nil
}

// KeepGroundedDrivers 丢掉 evidence 并非真的出自她所写内容的那几条。
//
// 和 interest.KeepGrounded 是同一条不变量，只是作用在另一个结构上。实测抓到
// 过的那一次事故（模型把 prompt 里的脚手架文字当成她的原话返回）在这里同样
// 成立，而且后果更重：驱动力假设摆在报告最显眼的地方，下面署着「你自己说的」。
func KeepGroundedDrivers(ds []Driver, corpus string) []Driver {
	flat := foldSpace(corpus)
	out := make([]Driver, 0, len(ds))
	for _, d := range ds {
		if strings.Contains(flat, foldSpace(d.Evidence)) {
			out = append(out, d)
		}
	}
	return out
}

// sliceJSON 从一段可能带着解释文字的回话里切出那个 JSON 对象。
//
// 和 interest 包里的 sliceJSONObject 做同一件事。没有复用它是因为那个是小写
// 开头的包内函数；把它导出会让 interest 的 API 面为了一个九行的工具函数变宽，
// 而这九行在这里读起来是自洽的。
func sliceJSON(raw string) ([]byte, error) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("reply contains no JSON object")
	}
	return []byte(s[start : end+1]), nil
}

/* ── 「和上次比」 ───────────────────────────────────────────────────────── */

// BuildDiff 算这一趟和上一趟的差。
//
// prev 为 nil（第一趟）时返回 nil，报告少一块。这比显示一块写着「这是你的
// 第一次，没有可比较的」的空卡片好：她知道这是第一次。
func BuildDiff(planted []Planted, prev *Report, daysBetween int) *Diff {
	if prev == nil {
		return nil
	}
	// 🚨 两个切片必须是**空切片**，不是 nil。Go 把 nil 切片 marshal 成 `null`，
	// 而前端写的是 `diff.stronger.join(...)` —— 于是一趟没有长出词的重做会让
	// 整个报告页崩掉。2026-09-19 线上走查里，第二趟正好一个词都没长出来，
	// 这条才露出来。
	d := &Diff{
		Stronger:         []string{},
		New:              []string{},
		PreviousQuestion: prev.Question,
		DaysBetween:      daysBetween,
	}
	for _, p := range planted {
		switch p.Verdict {
		case VerdictConfirm:
			d.Stronger = append(d.Stronger, p.Zh)
		case VerdictGrow:
			d.New = append(d.New, p.Zh)
		}
	}
	return d
}

/* ── 她自己写的那几句 ───────────────────────────────────────────────────── */

// nodeQuestion / nodeWork 是「她的问题」和「她的作品设想」分别来自第几问。
//
// 写成常量而不是硬编码的下标，因为节点顺序如果改了，这两个必须跟着改 ——
// 而一个写死的 `answers[4]` 改动时没有人会注意到。
const (
	nodeQuestionIndex = 4 // NODE 05 / QUESTION
	nodeWorkIndex     = 7 // NODE 08 / CREATE
)

// AnswerAt 取她第 i 问的原话。越界或为空时返回空串。
//
// 🚨 **不截断。** 她自己写的字一个都不要切
// （memory: observation-tool-is-the-bug-2026-09-12）。
func AnswerAt(answers []string, i int) string {
	if i < 0 || i >= len(answers) {
		return ""
	}
	return strings.TrimSpace(answers[i])
}

// HerQuestion 是她在 NODE 05 写下的那个问题，原样。
func HerQuestion(answers []string) string { return AnswerAt(answers, nodeQuestionIndex) }

// HerWorkConcept 是她在 NODE 08 写下的作品设想，原样。
func HerWorkConcept(answers []string) string { return AnswerAt(answers, nodeWorkIndex) }

// 编译期守着一件事：这两个下标必须真的落在节点表里。节点表改短了就编译不过，
// 而不是在生产上静悄悄地取到空串。
var _ = func() struct{} {
	if nodeQuestionIndex >= len(Nodes) || nodeWorkIndex >= len(Nodes) {
		panic("awakening: report node index out of range")
	}
	return struct{}{}
}()

// Harvested 是 interest.Harvested 的别名，省得调用方为了一个类型 import 两个包。
type Harvested = interest.Harvested
