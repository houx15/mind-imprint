package promptlib

// options.go —— 「从下面两个题目中任选一题」那句话的处理。
//
// # 它修的那两件事（同事 2026-09-22 的意见 6）
//
//	「下图说从两个题目中任选一题，但下面只显示了一个题，展开后也只有一个题」
//
// 全库扫一遍，带这句引子的有 24 道，分成两种，**两种都是坏的**：
//
//   - **11 道的引子在说谎。** 源数据已经把两个题目拆成了两条（GKYW-004 拿
//     （1）、GKYW-005 拿（2）），可那句共用的引子留在了每一条上。她看到的是
//     「任选一题」，底下只有一题。
//   - **13 道真的把两三个题目挤在一张卡上。**（ZKYW-001 的「题目一：语言的
//     滋味」和「题目二：攀登是幸福的」；GKYW-003 的三条微写作。）这张卡按下
//     「用这道题写」之后，整段题面进 `writing.assigned_prompt` —— 印记于是
//     拿着**两篇不同的文章**在陪她想一篇。
//
// 所以这里做两件事：说谎的引子去掉，真的多题的拆成各自一张卡。
//
// # 🚨 共用的那几句要跟着走
//
// 拆开不是把行分给谁那么简单。中文作文题的形状常常是
//
//	引子（任选一题）
//	材料（两个题目共用的那段话）
//	（1）……
//	（2）……
//
// 材料**在选项前面**（ZKYW-098 就是这样），拆开之后每一份都得带着它，
// 否则第二张卡上只剩一句「请自选角度，自拟题目，写一篇议论文」——
// 那道题的材料整段不见了。
//
// 结尾那一段反过来，要看它和最后一个选项之间有没有空行：
// ZKYW-039 的「注意：①请在作文第一行居中写『给父母的一封信』」紧贴着（2），
// 它只属于（2）；ZKYY-073 的答题模板（`Title: ____`）隔着一个空行，
// 两个选择都要用它。**空行就是那条界线**，这是这批数据里唯一说得出口的判据。

import (
	"regexp"
	"strings"
)

// PromptPart 是一道题面拆出来的一份。
//
// Suffix 拼在原来那个 id 后面（`ZKYW-001` → `ZKYW-001-1`）。不用拆的时候是
// 空串，id 一个字都不动 —— 库里绝大多数题都走这条路。
//
// 🚨 改 id 是安全的：题库的 id 一处都没有落库。`POST /{id}/start` 把题面
// **抄进** `writing.assigned_prompt`，之后这篇写作和题库再无关系
//（分级阅读库那边有 `reading.library_slug`，这边没有对应的列）。
type PromptPart struct {
	Suffix string
	Text   string
}

var (
	// 「任选一题」那句引子的各种说法。
	//
	// 🚨 「选择一个」**不在**这里：「请从“说谢谢”“往前走”“看见美”中选择一个，
	// 写一篇作文」挑的是一个词，不是一道题 —— 那句话是题目自己的话，一个字
	// 都不能动。收进来就会把那道题的题面砍掉。
	optionStem = regexp.MustCompile(`任选一[题个篇道]|任选其中一[题个篇]|任选其一|选做一[题个篇]|任选两[题个]`)

	// 一行开头的选项标号。
	//
	// 🚨 第一组括号就是**要剥掉的那一截**，可能比整个匹配短：
	// 「26.题目：______长伴我左右」剥掉的只有「26.」，「题目：」要留着 ——
	// 砍了它，那一行就只剩一个填空，看不出是个要她补全的标题。
	// （Go 的 RE2 没有前向断言，所以用捕获组表达这件事。）
	optMarkers = []*regexp.Regexp{
		regexp.MustCompile(`^(\s*[（(]\s*[0-9０-９一二三四五六七八九①②③④⑤]+\s*[）)]\s*)`),
		regexp.MustCompile(`^(\s*题目\s*[一二三四五①②③④⑤0-9]\s*[：:、.．]?\s*)`),
		regexp.MustCompile(`^(\s*选择\s*[一二三四五①②③④⑤0-9]\s*[：:、.．]?\s*)`),
		regexp.MustCompile(`^(\s*\d{1,2}\s*[.．、]\s*)题目`),
		regexp.MustCompile(`^(\s*作文\s*[（(]?\s*[0-9０-９一二三四五①②③④⑤]+\s*[）)]?\s*[：:、.．]?\s*)`),
		regexp.MustCompile(`^(\s*任务\s*[一二三四五0-9]\s*[：:、.．]?\s*)`),
		regexp.MustCompile(`^(\s*[①②③④⑤]\s*)`),
	}

	// 光秃秃的编号行：「22. 宁宁在……」。只在兜底那一步用，见 SplitPromptText。
	bareNumberedLine = regexp.MustCompile(`^(\s*\d{1,2}\s*[.．、]\s*)\S`)
)

// optionMarker 说这一行是不是一个选项的开头，以及要剥掉的那一截有多长。
func optionMarker(line string) (int, bool) {
	for _, re := range optMarkers {
		if m := re.FindStringSubmatchIndex(line); m != nil {
			return m[3], true
		}
	}
	return 0, false
}

// stripMarker 把一行开头的选项标号剥掉。认不出就原样返回。
func stripMarker(line string) string {
	if n, ok := optionMarker(line); ok {
		return line[n:]
	}
	if m := bareNumberedLine.FindStringSubmatchIndex(line); m != nil {
		return line[m[3]:]
	}
	return line
}

// SplitPromptText 把一道题面整理成一份或几份。
//
// 没有那句引子、或者底下确实只有一个题目时，返回一份，Suffix 是空串
// （引子和说谎的标号都已经去掉）。真的有两三个题目时，一个题目一份。
func SplitPromptText(s string) []PromptPart {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")

	// 引子在最前面，但不一定是第一行：ZKYW-018 顶着「三、综合性学习与写作」
	// 和「（二）写作」两行栏目名，引子排第三；ZKYY-021 顶着「文段表达」。
	// 所以往下看**前三行非空的**，看完就停 —— 再往下的「任选」就是题面自己
	// 的话了（ZKYW-056 的题目二里有一句「任选两个」，那是题目在说话）。
	stemAt := -1
	seen := 0
	for i, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if optionStem.MatchString(l) {
			stemAt = i
			break
		}
		seen++
		if seen >= 3 {
			break
		}
	}
	if stemAt < 0 {
		return []PromptPart{{Text: strings.TrimSpace(s)}}
	}

	// 引子上面那几行是栏目名（「三、综合性学习与写作」「文段表达」），
	// CleanPromptText 认不出来的那几种。它们不是题目，拆开之后一并丢掉。
	rest := lines[stemAt+1:]

	// 选项从哪一行开始。
	starts := make([]int, 0, 4)
	for i, l := range rest {
		if _, ok := optionMarker(l); ok {
			starts = append(starts, i)
		}
	}
	// 兜底：光秃秃的「22.」「23.」（ZKYW-064）。
	//
	// 🚨 三道闸，缺一不可。ZKYY-073 的正文里有「1．你对这句话的理解；
	// 2．结合实际生活举例说明；3．你得到的启发。」—— 那是**一道题的内容要求**，
	// 不是三道题。
	//   ① 别的标号一个都没匹配上（ZKYY-073 的「选择一/选择二」匹配上了，
	//      所以它根本走不到这里）；
	//   ② 引子里明说了「题」（ZKYY-031 的引子说「从调查结果中任选其一」，
	//      说的不是题）；
	//   ③ 数出来是两三条。四条以上就不是「两三个题目」，是一张清单。
	if len(starts) == 0 && strings.Contains(lines[stemAt], "题") {
		for i, l := range rest {
			if bareNumberedLine.MatchString(l) {
				starts = append(starts, i)
			}
		}
		if len(starts) > 3 {
			starts = starts[:0]
		}
	}

	// 引子那一行里除了「任选一题」还可能有真话
	// （ZKYY-033：「……根据所给的中文和英文提示，完成一篇不少于50词的英语文段
	// 写作。」）。所以按小句拆，只丢掉说「任选一题」的那一小句。
	//
	// 🚨 宽窄由「拆没拆开」决定。「任选其一」是两义的：
	// GKYW-045「阅读下面两材料，任选其一」说的是两道题，
	// ZKYY-028「从以下两种学习方式中任选其一」说的是这道题要她选的立场 ——
	// 后者砍掉，那道题就不知道要写什么了。
	// **数出了两三个标号**就是「说的是题」的证据；数不出来时按窄的来，
	// 只丢明说「题」的那几种说法。
	head := dropStemClauses(lines[stemAt], len(starts) >= 2)

	// 选项前面那一段是共用的：引子剩下的那句话，加上材料。
	sharedTop := make([]string, 0, 4)
	if head != "" {
		sharedTop = append(sharedTop, head)
	}
	upTo := len(rest)
	if len(starts) > 0 {
		upTo = starts[0]
	}
	sharedTop = append(sharedTop, rest[:upTo]...)

	// 只有一个（或没有）选项：引子在说谎。去掉它，标号也去掉。
	if len(starts) < 2 {
		body := rest[upTo:]
		if len(body) > 0 {
			body = append([]string{stripMarker(body[0])}, body[1:]...)
		}
		return []PromptPart{{Text: joinLines(append(append([]string{}, sharedTop...), body...))}}
	}

	// 真的有两三个题目。最后那一段要不要算共用的 —— 看空行。
	tail := []string(nil)
	last := starts[len(starts)-1]
	for i := last + 1; i < len(rest); i++ {
		if strings.TrimSpace(rest[i]) == "" {
			// 空行之后的东西两个选项都要用（ZKYY-073 的答题模板）。
			// 空行本身不进任何一份。
			for j := i + 1; j < len(rest); j++ {
				tail = append(tail, rest[j])
			}
			rest = rest[:i]
			break
		}
	}

	out := make([]PromptPart, 0, len(starts))
	for k, from := range starts {
		to := len(rest)
		if k+1 < len(starts) {
			to = starts[k+1]
		}
		body := append([]string{}, rest[from:to]...)
		body[0] = stripMarker(body[0])
		text := joinLines(concat(sharedTop, body, tail))
		if strings.TrimSpace(text) == "" {
			continue
		}
		out = append(out, PromptPart{Suffix: suffixFor(k), Text: text})
	}
	if len(out) < 2 {
		// 拆出来不足两份就不算拆（比如每一份都是空的）。原样返回，只去引子。
		return []PromptPart{{Text: joinLines(append(append([]string{}, sharedTop...), rest[upTo:]...))}}
	}
	return out
}

func concat(parts ...[]string) []string {
	out := make([]string, 0, 8)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// suffixFor 是第 k 份的 id 后缀。数字，不用字母 —— 卷面上那几个标号本来就是
// 数字或「一二三」，读起来对得上。
func suffixFor(k int) string {
	return "-" + string(rune('1'+k))
}

// joinLines 拼行，顺手把两头和多余的空行收掉。
func joinLines(lines []string) string {
	var b strings.Builder
	for _, l := range lines {
		t := strings.TrimRight(l, " \t　")
		if strings.TrimSpace(t) == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(t)
	}
	return strings.TrimSpace(b.String())
}

// —— 引子那一行 ——

var (
	// 小句的分界。中文题面里逗号、顿号、分号都可能分开两件事。
	clauseSplit = regexp.MustCompile(`[，,；;。！!]`)
	// 说了「任选一**题**」的小句 —— 明说了「题」，两义不了，一律丢掉。
	// 「按要求作答」「按要求作文」也一起丢：它一个字的信息都没有，
	// 而引子被砍掉之后它常常是唯一剩下的那一句。
	// 光说字数的那一小句也丢：字数是卡片上单独一栏（WordLimit），
	// 留在这里，题面的第一行就成了「不少于700字。」。
	stemClause = regexp.MustCompile(`任选一[题个篇道]|任选其中一[题个篇]|选做一[题个篇]|^\s*[请]?[按根]据?[要求题目]{0,4}要求作[答文]\s*$|^\s*[请]?按要求作[答文]\s*$|^\s*[不少于超过约]{1,3}\s*\d{2,4}\s*[字词]\s*$|^\s*\d{2,4}\s*[字词](左右|以上|以内)?\s*$`)
	// 只有确实拆出了两三个题目时才丢的那几种 —— 见 head 那一段的注释。
	stemClauseWhenSplit = regexp.MustCompile(`任选其一|任选两[题个]`)
)

// dropStemClauses 把引子那一行里说「任选一题」的小句去掉，别的留着。
//
// 🚨 按**小句**而不是按句子：ZKYY-033 的引子是
// 「从下面两个题目中任选一题，根据所给的中文和英文提示，完成一篇不少于50词的
// 英语文段写作。」—— 整句丢掉，那道题就不知道要写英文短文了。
func dropStemClauses(line string, split bool) string {
	// 保留原来的标点：按分隔符切的时候把它们一起记下来。
	idx := clauseSplit.FindAllStringIndex(line, -1)
	type piece struct{ text, sep string }
	pieces := make([]piece, 0, 8)
	at := 0
	for _, r := range idx {
		pieces = append(pieces, piece{line[at:r[0]], line[r[0]:r[1]]})
		at = r[1]
	}
	if at < len(line) {
		pieces = append(pieces, piece{line[at:], ""})
	}

	var b strings.Builder
	for _, p := range pieces {
		if strings.TrimSpace(p.text) == "" {
			continue
		}
		if stemClause.MatchString(p.text) {
			continue
		}
		if split && stemClauseWhenSplit.MatchString(p.text) {
			continue
		}
		b.WriteString(p.text)
		b.WriteString(p.sep)
	}
	out := strings.TrimSpace(b.String())
	// 砍完之后开头可能剩下一个标点。
	out = strings.TrimLeft(out, "，,；;。、 　")
	return strings.TrimSpace(out)
}
