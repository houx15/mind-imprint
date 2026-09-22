package api

import (
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// 她把同一句话说了两遍 —— 这件事数得出来，就不要让模型去感觉。
//
// 产品负责人 2026-09-12 的第三条：
//
//	「增加重复语句和内容连贯性检查，指出重复、逻辑混乱的问题并引导修改。」
//
// 症状表里**论点层面**的重复本来就有（middle_collapse「同一个论点说了几遍」、
// ending_only_summary「结尾把前面说过的再说一遍」），连贯也有
// （paragraph_jump、英文那张表里的 reference_linking / paragraph_function_order）。
// 缺的是**字句层面**的重复：同一个短语、同一种说法，在文章里反复出现。
//
// 而这一条和上面那些不同：它**不需要判断力**，它是个字符串问题。所以按这个仓库
// 一贯的做法办 —— 服务端数出来当事实喂进去，别让模型每轮从两千字里重新找一遍
// （它找不全，而且每轮找出来的还不一样）。同 writingPlanShape.promptBlock、
// 同 writingCoachProjection 里那几个数。
//
// 🚨 只报事实，不下判断。重复不一定是毛病：排比是重复，回环也是重复，
// 关键词在一篇议论文里本来就该反复出现。所以这里交出去的是「这几个说法各出现了
// 几次」，由模型结合上下文决定要不要说、算不算 issue —— 我们不在代码里判文章。

// writingRepeatMinRunes 一个中文片段至少这么长才值得报。
//
// 六个字。再短就全是「的时候」「我觉得」这类结构词，报出来只会让模型去改她的
// 虚词，那不是重复，那是中文。
const writingRepeatMinRunes = 6

// writingRepeatMinWords 英文至少连着这么多个词。
//
// 四个。三个词的 "a lot of" 到处都是；四个词开始才像一个她自己的说法。
const writingRepeatMinWords = 4

// writingRepeatMaxReported 最多报几条。
//
// 一轮意见最多也就三四条，报十条重复只会把上文撑大、把真正的问题挤掉。
const writingRepeatMaxReported = 5

// writingRepeatedPhrases 找出反复出现的说法，按「出现次数 × 长度」排序。
//
// 中文按 rune 切滑动窗口，英文按词。两边都先把空白和标点抹掉再比 ——
// 同一句话第二次出现时，最常变的就是句末那个标点（同 quotematch.Normalize 的理由）。
func writingRepeatedPhrases(text, lang string) []writingRepeat {
	if lang == langEnglish {
		return repeatedWordRuns(text)
	}
	return repeatedRuneRuns(text)
}

type writingRepeat struct {
	Phrase string
	Count  int
}

// repeatedRuneRuns 中文那一路：长度固定的 rune 窗口，数每个窗口出现几次。
//
// 只取**极大**的那些：一个出现两次的十字片段，同时包含两个出现两次的六字片段，
// 三条都报出来是同一件事说三遍。所以报完长的，就把被它包住的短的丢掉。
func repeatedRuneRuns(text string) []writingRepeat {
	clean := []rune(stripForRepeat(text))
	if len(clean) < writingRepeatMinRunes*2 {
		return nil
	}
	counts := map[string]int{}
	// 从长到短扫，长的先占坑。上限给一个合理的句子长度，再长就不是「说法重复」
	// 而是整段复制，那种一眼就看得见。
	maxRunes := 24
	if maxRunes > len(clean)/2 {
		maxRunes = len(clean) / 2
	}
	for n := maxRunes; n >= writingRepeatMinRunes; n-- {
		for i := 0; i+n <= len(clean); i++ {
			counts[string(clean[i:i+n])]++
		}
	}
	return topRepeats(counts, func(a, b string) bool { return strings.Contains(a, b) })
}

// repeatedWordRuns 英文那一路：按词的滑动窗口。
func repeatedWordRuns(text string) []writingRepeat {
	words := strings.Fields(strings.ToLower(stripForRepeat(text)))
	if len(words) < writingRepeatMinWords*2 {
		return nil
	}
	counts := map[string]int{}
	maxWords := 12
	if maxWords > len(words)/2 {
		maxWords = len(words) / 2
	}
	for n := maxWords; n >= writingRepeatMinWords; n-- {
		for i := 0; i+n <= len(words); i++ {
			counts[strings.Join(words[i:i+n], " ")]++
		}
	}
	return topRepeats(counts, func(a, b string) bool { return strings.Contains(a, b) })
}

// topRepeats 把出现两次以上的挑出来，去掉被更长的那条包住的，再截断。
func topRepeats(counts map[string]int, contains func(long, short string) bool) []writingRepeat {
	var out []writingRepeat
	for p, c := range counts {
		if c >= 2 {
			out = append(out, writingRepeat{Phrase: p, Count: c})
		}
	}
	// 长的、次数多的排前面 —— 长度优先，因为要靠它去吃掉被包住的短片段。
	sort.Slice(out, func(i, j int) bool {
		li, lj := len([]rune(out[i].Phrase)), len([]rune(out[j].Phrase))
		if li != lj {
			return li > lj
		}
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Phrase < out[j].Phrase
	})

	var kept []writingRepeat
	for _, r := range out {
		covered := false
		for _, k := range kept {
			// 同样次数、而且是更长那条的一部分 ⇒ 同一件事，别报第二遍。
			if r.Count == k.Count && contains(k.Phrase, r.Phrase) {
				covered = true
				break
			}
		}
		if covered {
			continue
		}
		kept = append(kept, r)
		if len(kept) >= writingRepeatMaxReported {
			break
		}
	}
	return kept
}

// stripForRepeat 去掉空白和标点 —— 同一句话第二次出现时最常变的就是它们。
func stripForRepeat(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsSpace(r) {
			// 英文那一路靠空格切词，空格要留着；中文那一路留一个空格也不影响，
			// 因为中文本来就没有词间空格，窗口是按 rune 切的。
			b.WriteRune(' ')
			continue
		}
		if unicode.IsPunct(r) || unicode.IsSymbol(r) || cjkFullwidthPunct[r] {
			b.WriteRune(' ')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// writingRepeatBlock 渲染成 prompt 里那一段。空的时候一个字都不写。
func writingRepeatBlock(text, lang string) string {
	reps := writingRepeatedPhrases(text, lang)
	if len(reps) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n【系统统计的重复表达】\n")
	for _, r := range reps {
		b.WriteString("- 「" + r.Phrase + "」出现 " + strconv.Itoa(r.Count) + " 次\n")
	}
	b.WriteString("请结合上下文判断重复的作用。排比、强调和必要的关键词重复可以保留；" +
		"只有重复没有增加信息、影响表达时，才作为 issue 提出修改建议。\n")
	return b.String()
}
