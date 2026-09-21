package promptlib

import (
	"regexp"
	"strings"
)

// 把卷面上的脚手架从题面里去掉。
//
// # 为什么要去
//
// 产品负责人 2026-09-21：「clean the prompts. don't appear like 20分、
// 第三节 书面表达 etc. unnecessary contents」。
//
// 源数据是从真卷子上抄下来的，题面前面常常顶着一截只有阅卷才用得上的字：
//
//	第三部分 写作（共50分）
//	五、（1小题，50分）
//	20. 请从"说谢谢""往前走""看见美"中选择一个……
//
// 她要写的是第三行。前两行是**卷子的结构**，不是题目 —— 摆在卡片上只是占位置，
// 而卡片上那点地方本来就不够。分值也是多余的：它已经单独存在 FullScore 里，
// 界面要显示的时候自己取。
//
// # 🚨 只砍**开头连着的那几行**
//
// 中间出现的「第二节」可能是题目自己的话（「材料二：……」「第三部电影」），
// 砍掉就把题面改坏了。所以只从第一行往下砍，碰到第一行真正的内容就停。
//
// 同样的道理，括号里的东西只在**整组括号除了分数什么都没有**的时候才拿掉：
// 「（60分）」「(满分10分)」「（共两节，满分25分）」是分值，
// 「（用不少于两种修辞手法）」不是。

// 一行「只有卷面结构」的样子。
var (
	// 第三部分 写作 / 第二节 书面表达 / 四、读写结合
	sectionLine = regexp.MustCompile(`^\s*(第[一二三四五六七八九十百\d]+\s*[部分节篇]+|[一二三四五六七八九十]+\s*[、.．])\s*`)
	// 纯粹的栏目名。
	bareHeading = regexp.MustCompile(`^\s*(书面表达|写作|作文|微写作|大作文|小作文|读写结合|应用文写作|继续性写作|概要写作|读后续写)\s*$`)
	// 开头的题号：20. / 23．/ 5、
	leadingNumber = regexp.MustCompile(`^\s*\d{1,3}\s*[.．、]\s*`)
	// 一整组只谈分数的括号。
	scoreParen = regexp.MustCompile(`[（(][^（()）]*?[）)]`)
	// 括号里除了这些字什么都没有，就说明它只是分值。
	scoreOnly = regexp.MustCompile(`^[\s\d分满共本题两节小计，,、；;：:]*$`)
	// 没有括号、光秃秃挂着的分值。
	bareScore = regexp.MustCompile(`(满分|共|本题)\s*\d{1,3}\s*分`)
)

// CleanPromptText 去掉题面开头的卷面脚手架和分值标注。
func CleanPromptText(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")

	// ① 开头连着的脚手架行，一行一行砍。
	i := 0
	for i < len(lines) {
		l := stripScores(lines[i])
		t := strings.TrimSpace(l)
		if t == "" {
			i++
			continue
		}
		rest := strings.TrimSpace(sectionLine.ReplaceAllString(t, ""))
		switch {
		// 「第三部分 写作」「四、读写结合」——砍掉编号之后只剩一个栏目名（或什么都不剩）。
		case sectionLine.MatchString(t) && (rest == "" || bareHeading.MatchString(rest)):
			i++
		// 光秃秃的「书面表达」「作文」。
		case bareHeading.MatchString(t):
			i++
		default:
			// 第一行真正的内容，停。
			goto done
		}
	}
done:
	out := strings.Join(lines[i:], "\n")

	// ② 第一行开头的题号。
	out = strings.TrimLeft(out, "\n ")
	out = leadingNumber.ReplaceAllString(out, "")

	// ③ 全文的分值标注。
	out = stripScores(out)

	// ④ 收尾：行尾空格、连着三个以上的空行压成两个、两头修掉。
	var b strings.Builder
	for n, l := range strings.Split(out, "\n") {
		if n > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(strings.TrimRight(l, " \t　"))
	}
	out = b.String()
	for strings.Contains(out, "\n\n\n") {
		out = strings.ReplaceAll(out, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(out)
}

// stripScores 拿掉只谈分数的括号，和光秃秃挂着的分值。
func stripScores(s string) string {
	s = scoreParen.ReplaceAllStringFunc(s, func(m string) string {
		// 🚨 按**符文**剥括号，不要按字节数。全角「（」占三个字节、半角 "("
		// 占一个，而库里真的有「（…)」这种一头全角一头半角的写法 ——
		// 按固定字节数切，那一条会 panic（整份库跑一遍才撞出来）。
		r := []rune(m)
		if len(r) < 2 {
			return m
		}
		inner := string(r[1 : len(r)-1])
		// 必须真的提到分，而且除了分数相关的字什么都没有。
		if strings.Contains(inner, "分") && scoreOnly.MatchString(inner) {
			return ""
		}
		return m
	})
	s = bareScore.ReplaceAllString(s, "")
	// 分值拿掉之后可能留下「，」「、」开头的碎渣。
	s = strings.TrimLeft(s, " \t　")
	return s
}
