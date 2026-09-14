package api

import (
	"strings"

	"mindimprint/api/internal/store/sqlc"
)

// writing_ghostquote.go —— 陪练在对话里引了一句**她正文里根本没有**的话。
//
// # 这是从线上量出来的，不是想出来的
//
// 2026-09-12 第二十四轮走查，她连着八步在说同一件事：
//
//	「印记说的 them 跟我框里实际写的 the food 对不上，不知道它在指哪一版」
//	「印记引用的那段话在我现在的正文框[0]里根本找不到」
//	「它之前一直说有个[3]段重复了让我删，但我看正文框里根本没有重复的段落」
//	「它前面说让我删掉过渡句，现在又让我加过渡句，到底要哪样？」
//
// 上一轮她已经说出了真正的代价：**「我不知道该听它的还是按我现在的正文来。」**
//
// # 为什么会这样，以及为什么光靠提示词不行
//
// 上文里同时摆着两份东西：她此刻的正文，和最近十二轮对话 —— 后者带着印记
// **自己**说过的话，逐字引着她当时写的句子。顺着自己上一轮往下说，比回头
// 重读那几段省力。
//
// 我已经用提示词打过两次：一次让它「指着那一句说」，一次在正文抬头写明
//「以这里为准」。第二十三轮三条，第二十四轮八条 —— 没压下去。
// 这正是 [[prompt-output-must-be-verifiable-2026-09-03]] 那条：
// **代码验不了的「必须」只是一个希望。**
//
// 结构化那条路（请印记看看这一段）从第一天起就是可验的：引文必须逐字出现在
// 她写的东西里，对不上的整条丢掉。这个文件把同一条纪律搬到对话这一路上。
//
// # 判据故意收得很紧
//
// 自由对话里引号的用法太多，所以只认**最像「引她原文」**的那一种，
// 其余一概放过：
//
//   - 只看中文书名号式引号「」和直角引号，以及英文成对双引号；
//   - 引文至少 8 个 rune —— 短引号多半是术语（「让步」「主张」）或一个词，
//     不是在复述她的句子；
//   - 语料里包含她的每一段、成稿全文，**还有她自己在对话里说过的话** ——
//     印记复述她刚说的一句话是正常的，不该判成幻引。
//
// 判错的方向：宁可放过，不可误伤。误判一次会让一条本来好的回复被重试
// （花钱、让她多等），而漏掉一次只是维持现状。
const writingGhostQuoteMinRunes = 8

// 🚨 **「她写的」和「她说的」要分开，不能揉成一份语料。**
//
// 原来这里只有一份：每一段 + 成稿 + 她在对话里说过的话。于是一句她只在聊天里
// 提过、正文里从来没有的话，判据说「她写过」，印记就可以指着它让她改 ——
// 而她会去正文里找，找不到。
//
// 第三十六、三十七两轮里这条各出现一次，措辞几乎一样：
//
//	「它引用的那句「全校一天倒掉的饭真的很多」在我现在框里的字里没找到」
//	「印记引用的那句『看到什么就拿什么』在我现在的框[0]里根本找不到，
//	  不知道它在读哪一版」
//
// 注意她找的地方：**框里**。她把印记的引文理解成「我正文里的句子」，
// 这个理解是对的 —— 错的是我们允许它引一句只出现在聊天里的话而不说明出处。
//
// 所以分成两份，判据也分三种：在正文里（好）、只在对话里（要说明出处）、
// 哪儿都没有（幻引）。不是简单地把对话踢出语料 —— 印记 引她刚才说的话
// 「你刚才说那个男生一口没动红烧肉」是好教学，不该为此重试。

// writingWrittenCorpus 是**她写进作品里**的字：每一段 + 成稿。
func writingWrittenCorpus(snippets []sqlc.WritingSnippet, draftBody string) string {
	var b strings.Builder
	for _, s := range snippets {
		b.WriteString(s.Text)
		b.WriteString("\n")
	}
	b.WriteString(draftBody)
	return normalizeQuoteText(b.String())
}

// writingSaidCorpus 是她**在对话里说过**的话。
//
// 只收她说的。印记自己说过的话正是幻引的来源，收进来等于自证。
func writingSaidCorpus(msgs []sqlc.AtomMessage) string {
	var b strings.Builder
	for _, m := range msgs {
		if m.Role == "student" {
			b.WriteString(m.Content)
			b.WriteString("\n")
		}
	}
	return normalizeQuoteText(b.String())
}

// writingQuoteCorpus 两份合起来 —— 「哪儿都没有」用它判。
func writingQuoteCorpus(snippets []sqlc.WritingSnippet, draftBody string, msgs []sqlc.AtomMessage) string {
	return writingWrittenCorpus(snippets, draftBody) + writingSaidCorpus(msgs)
}

// normalizeQuoteText 抹掉空白和常见句读再比 —— 模型复述一句话时最常变的就是
// 句末那个标点，为这个判它幻引是误伤。
func normalizeQuoteText(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r', '　',
			'。', '，', '、', '；', '：', '！', '？', '…',
			'.', ',', ';', ':', '!', '?':
		default:
			b.WriteRune(r)
		}
	}
	return strings.ToLower(b.String())
}

// firstGhostQuote 返回回复里第一句「她根本没写过」的引文。没有就返回 ""。
func firstGhostQuote(reply, corpus string) string {
	for _, q := range extractQuotedSpans(reply) {
		n := normalizeQuoteText(q)
		if len([]rune(n)) < writingGhostQuoteMinRunes {
			continue
		}
		if !strings.Contains(corpus, n) {
			return q
		}
	}
	return ""
}

// extractQuotedSpans 把成对引号里的内容取出来。
//
// 只认成对的：落单的引号（英文撇号、她正文里的缩写）取不出东西来，正好。
func extractQuotedSpans(s string) []string {
	pairs := []struct{ open, close rune }{
		{'「', '」'},
		{'『', '』'},
		{'“', '”'},
	}
	var out []string
	for _, p := range pairs {
		rs := []rune(s)
		for i := 0; i < len(rs); i++ {
			if rs[i] != p.open {
				continue
			}
			for j := i + 1; j < len(rs); j++ {
				if rs[j] == p.close {
					out = append(out, string(rs[i+1:j]))
					i = j
					break
				}
			}
		}
	}
	// 🚨 **英文直双引号不认。**
	//
	// 它没有方向，只能「成对地数」，而她的正文里本来就有它 ——
	// 2026-09-12 第二十五轮那个英文学生写的就是 `By "more" I mean two things`。
	// 印记复述这句话时，成对数下来切出来的是 `more`、或者跨句拼起来的一段，
	// 两者都不在语料里，于是一句**她真的写过**的话会被判成幻引。
	//
	// 那条重试会告诉印记「这句在她正文里找不到」，而印记接着就会告诉**她**
	// 这句不存在 —— 正是这一轮她撞上的那种伤：
	// 「印记非说正文里没有我加的句子，但我看框[0]里面明明就在最后一句写着呢」。
	//（那一次的真凶是截断，不是这里；但这条路能造出一模一样的伤，先堵上。）
	//
	// 印记跟她说话用的是中文，引她的句子用的是「」—— 上面那三对够了。
	// 少抓几个，好过把她写过的话说成没写过。
	return out
}
