package api

import (
	"strings"
	"unicode"

	"mindimprint/api/internal/store/sqlc"
)

// writing_points_check.go —— 分论点「扣得住」这一条，服务端数出来。
//
// # 来源
//
// `docs/reference/writing-teaching/议论文写作汇总（四）（六）讲义.md`，
// 拟写分论点的三条原则：
//
//	扣得住：分论点的表述要把中心论点、题目或材料中的关键词嵌入其中，
//	        以保证每一段都扣题。
//	分得开：分论点之间尽量不交叉、不重复。
//	排得顺：逻辑顺序要合理。
//
// 三条里只有「扣得住」是**机械可判**的 —— 它问的是一件关于字的事：
// 这条分论点里有没有中心论点的词。「分得开」已经有 writing_repeats.go 在数；
// 「排得顺」只有模型判得了，写在 prompt 里。
//
// # 🚨 服务端数得出来的事实，就别让模型每轮自己数
//
// 这条是 2026-09-12 那次的正面用法：「提示词里的软话跨不过代码里的硬判据」。
// 反过来也成立 —— 一件我们数得准的事，交给模型每轮重数一遍，它会漏、会飘，
// 而且花钱。所以这里**数完当事实喂进去**，不写成一条「请你检查每条分论点
// 是否扣题」的规矩。
//
// # 🚨 数出来是空的就一个字都不加
//
// 这类提示不做常驻（2026-09-05 的教训：常驻提示会一直抢走那一轮该做的事，
// 六轮里陪练一直在补一张卡，她的主页三处一直是空的）。全都扣得住的时候，
// prompt 里不该出现「扣得住」这三个字。

// writingStopRunes —— 不算「关键词」的字。
//
// 中心论点里出现的虚词几乎每条分论点里都有，拿它们去比，等于每条都扣得住，
// 这条判据就永远不响。
var writingStopRunes = map[rune]bool{
	'的': true, '了': true, '是': true, '在': true, '和': true, '与': true,
	'也': true, '都': true, '就': true, '而': true, '及': true, '等': true,
	'我': true, '你': true, '他': true, '她': true, '它': true, '们': true,
	'这': true, '那': true, '有': true, '为': true, '被': true, '把': true,
	'不': true, '没': true, '很': true, '更': true, '最': true, '会': true,
	'能': true, '要': true, '应': true, '该': true, '可': true, '以': true,
	'之': true, '其': true, '所': true, '者': true, '于': true, '上': true,
	'下': true, '中': true, '个': true, '一': true, '种': true, '样': true,
}

// writingKeyRuns 从一句话里取出「实词串」：连续的、非停用字的汉字/字母段，
// 长度 ≥ 2。
//
// 为什么是 2 而不是 1：单个汉字撞上的概率太高（「学」在半数句子里都有），
// 判出来的「扣得住」是假的。两个字连着出现才说明她真的在说同一件事。
//
// 🚨 英文按词切，而且**整篇英文作文不走这条判据**（见 writingPointsOffThesis）。
// 这里仍然处理字母，只是因为中文句子里常夹着一两个英文词。
func writingKeyRuns(s string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) >= 2 {
			out = append(out, string(cur))
		}
		cur = cur[:0]
	}
	for _, r := range s {
		switch {
		case writingStopRunes[r]:
			flush()
		case unicode.Is(unicode.Han, r):
			cur = append(cur, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			cur = append(cur, unicode.ToLower(r))
		default:
			flush()
		}
	}
	flush()
	return out
}

// writingKeyRunes 交出一句话里的实字集合（去掉虚词和标点）。
func writingKeyRunes(s string) map[rune]bool {
	out := map[rune]bool{}
	for _, r := range s {
		if writingStopRunes[r] {
			continue
		}
		if unicode.Is(unicode.Han, r) {
			out[r] = true
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			out[unicode.ToLower(r)] = true
		}
	}
	return out
}

// writingSharedKeyRunesMin —— 共用几个实字才算扣住。
//
// 🚨 这个数是**量出来的，不是拍的**。1 太松：单个汉字撞上的概率太高
// （「学」在半数句子里都有），判出来的「扣得住」是假的，这条判据等于不存在。
// 3 太紧：「读书要读慢」和「慢读才能发现问题」只共用两个字，而她显然扣住了。
const writingSharedKeyRunesMin = 2

// writingSharesKeyword 说这两句话有没有扣在一起。
//
// # 🚨 为什么数的是**字**，不是词
//
// 第一版切的是「连续两个以上的实字串」，然后比子串。它在
// 「读书要读慢」↔「慢读才能发现问题」上判错了：两句共用的是「读」和「慢」，
// 但一句里是「读慢」，另一句里是「慢读」—— **中文的语序会翻**，
// 而子串比对认不出翻过来的同一个词。判错的方向还正好是最糟的那个：
// 告诉她一条写对了的分论点跑题了。
//
// 所以改成数**共用的实字个数**，顺序不参与。词序、插进来的修饰语、
// 「慢读 / 读得慢 / 读书慢」这些变体都跨得过去。
//
// 长一点的共用词串照旧算数（走 writingKeyRuns 的子串那一路），
// 它比字数这条更强，留着不亏。
func writingSharesKeyword(thesis, point string) bool {
	tRunes := writingKeyRunes(thesis)
	if len(tRunes) < writingSharedKeyRunesMin {
		// 中心论点里实字不够（太短、全是虚词）—— 判不了就当扣得住。
		return true
	}
	var shared int
	for r := range writingKeyRunes(point) {
		if tRunes[r] {
			shared++
			if shared >= writingSharedKeyRunesMin {
				return true
			}
		}
	}
	// 整段共用的长词串也算 —— 比如她把中心论点原样抄了一截进来。
	pNorm := strings.Join(writingKeyRuns(point), "\x00")
	for _, t := range writingKeyRuns(thesis) {
		if strings.Contains(pNorm, t) {
			return true
		}
	}
	return false
}

// writingPointsOffThesis 交出「哪几条分论点一个中心论点的词都没沾上」。
//
// 返回的是她写的**原话**，因为这句话要原样出现在 prompt 里 ——
// 说「第二条分论点」她还要自己去数，说她写的那句话她一眼就认得。
//
// 🚨 只对中文生效。英文作文的「扣题」判据不是共用词（冠词、同义替换、
// 代词回指都会让共用词消失），产品负责人也说了英文要另找托福雅思的路子。
// 判错的代价是告诉她一条她写对了的分论点跑题了 —— 那比不说更糟。
func writingPointsOffThesis(wr sqlc.Writing, rows []sqlc.WritingOutline) []string {
	if wr.Lang != "zh" {
		return nil
	}
	var thesis string
	for _, row := range rows {
		if writingKindOf(row) == writingKindThesis {
			thesis = row.Text
			break
		}
	}
	if strings.TrimSpace(thesis) == "" {
		return nil // 还没有中心论点，扣不扣得住无从谈起。
	}
	var written, off []string
	for _, row := range rows {
		if writingKindOf(row) != writingKindPoint {
			continue
		}
		if strings.TrimSpace(row.Text) == "" {
			continue // 还没写字的空卡不算跑题。
		}
		written = append(written, row.Text)
		if !writingSharesKeyword(thesis, row.Text) {
			off = append(off, row.Text)
		}
	}
	// 🚨 **她只有一条分论点的时候不提这件事。**
	//
	// 讲义里「扣得住」是**分论点都摆出来之后**回头检查的一条，不是写第一条
	// 时的门槛。2026-09-21 的 LIVE_LLM 实测撞上了这一下：她刚说出
	// 「青少年生物钟本来就晚」（中心论点是「上学时间该往后推一小时」），
	// 这条判据响了，于是那一轮陪练不去帮她想，改成请她把句子重新措辞 ——
	// 正是产品负责人说的「吹毛求疵」。
	//
	// 那句话按讲义的标准确实没扣住，这条判据本身没算错；错的是**时机**。
	// 她还在想的时候，一条理由的意思对不对比它的措辞要紧得多。
	if len(written) < writingPointsCheckFrom {
		return nil
	}
	return off
}

// writingPointsCheckFrom —— 有几条分论点之后才查「扣得住」。见上面那段。
const writingPointsCheckFrom = 2

// writingPointsCheckBlock 是加进立题 prompt 的那一段。
//
// 空集就返回空串 —— 见文件头那段「数出来是空的就一个字都不加」。
func writingPointsCheckBlock(wr sqlc.Writing, rows []sqlc.WritingOutline) string {
	off := writingPointsOffThesis(wr, rows)
	if len(off) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n【分论点与中心观点的文字匹配提示】\n")
	b.WriteString("以下句子与中心观点的关键词重合较少。这只是文字匹配结果，不能据此判断跑题。请结合上下文判断它们是否在解释中心观点；意思相关时无需重复关键词或改写。\n")
	for _, text := range off {
		b.WriteString("- 「" + text + "」\n")
	}
	b.WriteString("如果确实缺少逻辑联系，可请学生解释这条理由与中心观点的关系，不要求把指定词语塞进句子。计划已可开始写作或学生要求动笔时，不把措辞修改当作前置条件。\n")
	return b.String()
}

// writingPointAnglesBlock —— 拟写分论点的四个角度。
//
// 讲义（四）第三节：并列式议论文的分论点，从「是什么 / 为什么 /
// 怎么样（怎么办）/ 会怎样」四个角度里**选定一个**，整篇用同一个角度。
//
// 🚨 只在她的分论点还不够的时候加。够了之后再摆一张「可以从哪几个角度想」
// 的表，是在她已经想好之后教她怎么想 —— 那一轮该做的是别的事。
func writingPointAnglesBlock(wr sqlc.Writing, rows []sqlc.WritingOutline, need int) string {
	if wr.Lang != "zh" {
		return ""
	}
	var have int
	for _, row := range rows {
		if writingKindOf(row) == writingKindPoint && strings.TrimSpace(row.Text) != "" {
			have++
		}
	}
	if have >= need {
		return ""
	}
	return `
【她还要再想一条分论点，这四个角度里选一个】
一篇文章的几条分论点最好都从**同一个角度**切进去，读者才觉得它们是一套的：

- **是什么**：这个词到底指什么。（诗意地栖居，是远离喧嚣独自成长）
- **为什么**：为什么该这样。（诗意地栖居，可以让我们的心飞得更高）
- **怎么办**：要做到得怎么做。（诗意地栖居，需要我们积极乐观地面对生活）
- **会怎样**：这样做了会带来什么。（这样做的人，日子会变成什么样）

看她已经写的那几条是从哪个角度切的，请她照着同一个角度再想一条。
🚨 一次只问一个问题，不要把四个角度都摆给她让她挑。
`
}
