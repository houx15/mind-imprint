package api

// reading_outline.go — 导读：一句话、结构、每一段的承重。
//
// # 为什么有这个文件
//
// 印记 开场那一轮原来的活儿是「介绍一下你排的读法，然后领她进第一步」，同时
// 又被要求「不要说出这篇文章的结论、主旨、答案」。这是一道自相矛盾的题，真模型
// 给出的答案是把文章讲了一遍：
//
//	我们先一起把整篇文章通读一遍。这篇文章讲的是冲突里援助组织在忙什么——
//	以色列和哈马斯在打仗，援助组织得想办法把物资送进去、把人救出来。……
//	读完告诉我一声。
//
// 一句话里同时违反了三条已经写在 prompt 里的规矩（讲了内容、替她减压、
// 以「读完告诉我一声」收尾）。修法不是把规矩再写重一遍 —— 是把这一轮的活儿
// 拿走：她一进来该看见的三样东西由**确定性的渲染**摆出来，模型那一轮只剩
// 递第一张卡片。见 docs/2026-09-10-reading-guidance-redesign.md。
//
// # 三样东西
//
//	oneLine  这篇在**问**什么。不是它的结论 —— 结论说出来，后面每一步都是走过场。
//	shape    它怎么组织的：问题 → 证据 → 让步 → 结论。
//	load     每一段的承重：核心 / 支撑 / 过渡。
//
// 承重这一层来自 ljg-read 的段落分类（它叫骨/肌/筋）。它是整份调研里唯一真的
// 回答了「该在哪儿停下来」的启发式 —— 按论证承重，不按生词多少。核心段停下来
// 问一轮，支撑段连着读过去，过渡段一句带过。
//
// # 能验的都在代码里
//
// [[prompt-output-must-be-verifiable-2026-09-03]]：prompt 里的「必须」如果代码里
// 验不了，它就只是一句期望。这里能验的有三条，`validateOutline` 全做了：
//
//	承重只能取三个值            越界的那一段退回「支撑」
//	核心段不超过全文的三分之一   超了整份 outline 作废（全是核心 = 没有核心）
//	一段核心都没有              整份作废（那份分类没在分类）
//	一句话必须是中文的          一个汉字都没有 → 整份作废
//	两个字段都有字数上限        超了截断
//
// 第二条是真正有意义的那条。模型很容易把每一段都标成核心 —— 那份 outline 在
// 屏幕上仍然长得像一份导读，但它一个字的信息都没有，而带读的节奏会退回「每段
// 都停」，也就是没有节奏。
//
// 第四条是线上第一次跑就撞上的：文章是英文的，模型顺着文章的语言把导读也写成了
// 英文（「outbreak → blockade → aid scramble → war」），摆在中文界面上对她等于
// 不存在。

import (
	"encoding/json"
	"strings"
)

// 承重的三个取值。闭表，和学科表同一个理由：模型能编出第四种，而第四种在
// 界面上没有颜色、在带读规矩里没有对应的节奏。
const (
	loadCore    = "core"    // 核心段：承载主张
	loadSupport = "support" // 支撑段：证据、例子、数据
	loadBridge  = "bridge"  // 过渡段：连接
)

// loadLabels 是给学生看的名字。
//
// 🚨 不叫骨/肌/筋。那是比喻，而 ui-copy-style 第 10 条不让在界面上用比喻替代
// 本来就有的说法。核心/支撑/过渡是说明文，而且更准。
var loadLabels = map[string]string{
	loadCore:    "核心",
	loadSupport: "支撑",
	loadBridge:  "过渡",
}

// readingOutline 就是落进 reading_source.outline 的那份 jsonb。
type readingOutline struct {
	OneLine string            `json:"oneLine"`
	Shape   string            `json:"shape"`
	Load    map[string]string `json:"load"`
}

const (
	outlineOneLineMaxRunes = 60
	outlineShapeMaxRunes   = 40
)

// coreShareCap 是核心段占全文的上限。
//
// 三分之一：一篇十二段的报道，核心段最多四段。这个数字不是精算出来的，它是
// 「还能构成一个判断」的边界 —— 一半以上都是核心，这份分类就没有在分类。
const coreShareCap = 3

// blank 说的是这份导读什么都没说。空的 outline 不落库，前端因此不必区分
// 「没有导读」和「有一份空导读」。
func (o readingOutline) blank() bool {
	return strings.TrimSpace(o.OneLine) == "" && strings.TrimSpace(o.Shape) == "" && len(o.Load) == 0
}

// validateOutline 把模型给的那份导读收进我们能保证的范围里。
//
// 返回的 bool 是「这份还值不值得存」。不值得的时候整份丢掉而不是修补：一份
// 修补过的导读在屏幕上和一份真的导读长得一模一样，而她没有办法分辨。
func validateOutline(got readingOutline, blocks []Block) (readingOutline, bool) {
	out := readingOutline{
		OneLine: trimRunes(strings.TrimSpace(got.OneLine), outlineOneLineMaxRunes),
		Shape:   trimRunes(strings.TrimSpace(got.Shape), outlineShapeMaxRunes),
		Load:    map[string]string{},
	}

	// 每一段都要有一个标签，而且只能是我们认识的那三个。模型漏掉的、写错的，
	// 一律退回「支撑」—— 那是最不打扰的一档：它不会让带读在一段普通的证据上
	// 停下来，也不会让一段真正的核心被跳过去（跳过去的代价更大）。
	core := 0
	for _, b := range blocks {
		if strings.TrimSpace(b.Text) == "" {
			continue
		}
		kind := strings.TrimSpace(got.Load[b.ID])
		switch kind {
		case loadCore, loadSupport, loadBridge:
		default:
			kind = loadSupport
		}
		if kind == loadCore {
			core++
		}
		out.Load[b.ID] = kind
	}

	// 🚨 导读必须是中文的。
	//
	// 线上第一次跑就写成了英文（「War is escalating — can aid groups still
	// reach…」「outbreak → blockade → aid scramble → war」）—— 文章是英文的，
	// 模型顺着文章的语言写了下去。那份导读摆在中文界面上，对她等于不存在。
	//
	// prompt 里当然也写了这一条，但一句 prompt 里的「必须」如果代码里验不了，
	// 它就只是一句期望（[[prompt-output-must-be-verifiable-2026-09-03]]）。
	// 判据取最宽的那一个：**一个汉字都没有**才算没写中文。这样英文的专有名词
	// （人名、地名、机构名）照抄原文不会被误伤 —— 那本来就是对的做法。
	if !hasCJK(out.OneLine) {
		return readingOutline{}, false
	}

	// 全是核心 = 没有核心。见 coreShareCap。
	if core*coreShareCap > len(out.Load) {
		return readingOutline{}, false
	}
	// 一段核心都没有，这份分类也没在分类 —— 带读会退回「哪儿都不停」。
	if core == 0 {
		return readingOutline{}, false
	}
	if out.blank() {
		return readingOutline{}, false
	}
	return out, true
}

// coreBlockIDs 是核心段的 id，按正文顺序。
func (o readingOutline) coreBlockIDs(blocks []Block) []string {
	out := make([]string, 0, 4)
	for _, b := range blocks {
		if o.Load[b.ID] == loadCore {
			out = append(out, b.ID)
		}
	}
	return out
}

// decodeOutline 读一行 reading_source 上的那一列。读不动就当没有 —— 一份坏掉的
// 导读不该让整篇文章打不开。
func decodeOutline(raw []byte) readingOutline {
	// Load 永远返回一张真的 map，不是 nil。读一张 nil map 在 Go 里是安全的，
	// 但**写**它会 panic —— 这里不留那个坑给以后。
	empty := readingOutline{Load: map[string]string{}}
	if len(raw) == 0 {
		return empty
	}
	var o readingOutline
	if err := json.Unmarshal(raw, &o); err != nil {
		return empty
	}
	if o.Load == nil {
		o.Load = map[string]string{}
	}
	return o
}

// hasCJK —— 这段字里有没有汉字。判的是 CJK 统一汉字那一段，不含假名和谚文：
// 我们要分辨的是「中文」和「英文」，不是「东亚文字」和别的。
func hasCJK(s string) bool {
	for _, r := range s {
		if r >= 0x4E00 && r <= 0x9FFF {
			return true
		}
	}
	return false
}

func trimRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
