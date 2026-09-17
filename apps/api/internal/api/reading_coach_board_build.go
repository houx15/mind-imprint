package api

// reading_coach_board_build.go — 标注论证那一步的板，由服务端摆出来。
//
// # 为什么不能等模型给
//
// 「标注论证」这一步的**全部内容**就是那块板。而实测下来，模型一遍遍在话里说
// 「把这三句拖到对应的格子里」，却没有在 JSON 里附上 card —— 十条走查里这是
// 唯一一个反复挡住整条链子的东西。加了检测、加了重试、把提示喂回去，
// 都只是降低概率：她还是会撞上「屏幕上根本没有板」。
//
// 而这一步根本不需要模型的判断力来决定「有没有板」：读法库已经规定了这一步是
// 标注论证（reading_routines.go 里那三条的第 1 条 —— 步骤由 routine 拥有）。
// 所以这里改成：**走到这一步而模型没给板，服务端自己摆一块。**
//
// 挑哪几句仍然优先用模型的判断（它给了就用它的），只有它没给的时候才用下面这条
// 确定性的规则兜底。这和 reading_plan.go 的关系是一样的：模型挑选和调参，
// 结构由我们保证。
//
// # 兜底怎么挑句子
//
// 从这一步的落点段开始，按长度取最能承载一个角色的几句 —— 太短的（「他说。」）
// 贴不上任何角色，太长的在板上摆不下。跨到第二段去取一句，因为「主张」和
// 「证据」常常不在同一段，全从一段里取的话这块板只能练一种关系。

import (
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	// 板上句子的长度窗口（rune）。下限比卡片选项的 4 高很多：一个角色标签要贴得
	// 上去，这句话本身得说完一件事。上限是屏幕上一张卡片还读得下来的长度。
	boardSentenceMinRunes = 12
	boardSentenceMaxRunes = 160
	// 三到四句。两句不够练关系，五句以上她要在屏幕上翻半天。
	boardWantSentences = 4
)

// buildLabelBoard 给「标注论证」那一步摆一块板，用文章里真实的句子。
//
// focus 是这一步的落点段（可以是空的）。返回 nil 表示这篇文章里挑不出足够的
// 句子 —— 那时候宁可没有板，也不要一块空板。
func buildLabelBoard(blocks []Block, focus string) *coachCard {
	return buildLabelBoardFromReply(blocks, focus, "")
}

// buildLabelBoardFromReply —— 同上，但**印记 在话里点了名的那几句排最前面**。
//
// 🚨 实测她卡住的那一幕：印记 说「把『以色列下令空袭』那句挪到对的格子里」，
// 而板上四张卡片全是第 1 段的话。她逐字报的是「根本没有以色列空袭那句，
// 我找不到要挪的那张卡片」。
//
// 板是服务端在它写完之后补的，只按段落取句子 —— 它嘴上点名的那一句因此可能
// 根本不在板上。它对她说话时指的是**那句话**，不是「第几段」，所以先看它引了
// 什么，引到的那几句优先上板。
func buildLabelBoardFromReply(blocks []Block, focus, reply string) *coachCard {
	if len(blocks) == 0 {
		return nil
	}
	// 落点段排在最前面，其余按原文顺序跟在后面：优先从 印记 刚讲过的那一段取。
	ordered := make([]Block, 0, len(blocks))
	for _, b := range blocks {
		if b.ID == focus {
			ordered = append(ordered, b)
		}
	}
	for _, b := range blocks {
		if b.ID != focus {
			ordered = append(ordered, b)
		}
	}

	type cand struct {
		opt  coachCardOption
		rank int // 0 = 落点段，1 = 其它段
	}
	cands := make([]cand, 0, 16)
	for i, b := range ordered {
		rank := 3
		if focus != "" && b.ID == focus {
			rank = 2
		}
		for _, s := range splitSentences(b.Text) {
			n := utf8.RuneCountInString(s)
			if n < boardSentenceMinRunes || n > boardSentenceMaxRunes {
				continue
			}
			r := rank
			// 它在话里点了名的那一句，排在所有人前面。
			if reply != "" && replyMentionsSentence(reply, s) {
				r = 0
			}
			cands = append(cands, cand{opt: coachCardOption{BlockID: b.ID, Quote: s}, rank: r})
		}
		// 扫够两段就停：再往后取的句子离 印记 刚讲的地方太远，她得满篇找。
		// 🚨 除非它话里点了名 —— 点名的那一句可能在任何一段，要扫到。
		if reply != "" && i < len(ordered)-1 {
			continue
		}
		if i >= 1 && len(cands) >= boardWantSentences {
			break
		}
	}
	if len(cands) < 2 {
		return nil
	}
	// 🚨 落点段的**最后一句**一定要上板。
	//
	// 它常常按位置指句子（「第二段最后一句」「开头那句」）而不是把原话引出来，
	// 那样 replyMentionsSentence 认不出来 —— 实测她逐字报的：「它让我把第二段
	// 最后一句拖到主张格，但板上根本没有这张卡片。」
	// 段里句子多于四句时，前几句会把名额占满，最后一句正好是被挤掉的那个。
	if focus != "" {
		last := -1
		for i, c := range cands {
			if c.opt.BlockID == focus {
				last = i
			}
		}
		// 排在它点名的那一句后面（0），但在落点段其余句子前面（2）。
		if last >= 0 && cands[last].rank > 1 {
			cands[last].rank = 1
		}
	}

	sort.SliceStable(cands, func(i, j int) bool { return cands[i].rank < cands[j].rank })

	out := make([]coachCardOption, 0, boardWantSentences)
	seen := map[string]bool{}
	for _, c := range cands {
		if seen[c.opt.Quote] {
			continue
		}
		seen[c.opt.Quote] = true
		out = append(out, c.opt)
		if len(out) == boardWantSentences {
			break
		}
	}
	if len(out) < 2 {
		return nil
	}
	return &coachCard{
		Type:   coachCardLabelRoles,
		Prompt: coachLabelBoardPrompt,
		// 🚨 逐字来自正文，所以它照样过得了 validateCoachCard 那一关 ——
		// 调用方仍然会把它送进去校验，这里不是一条绕过校验的后门。
		Options: out,
		Labels:  coachArgueBinsBasic,
	}
}

// splitSentences 把一段话切成句子。
//
// 中英文都要切，所以认两套句末标点。引号和右括号跟在句号后面时一起带走
// （「……。」 是一句，不是一句加一个引号）。
func splitSentences(text string) []string {
	runes := []rune(strings.TrimSpace(text))
	out := make([]string, 0, 8)
	start := 0
	for i := 0; i < len(runes); i++ {
		switch runes[i] {
		case '。', '！', '？', '.', '!', '?':
			end := i + 1
			// 把紧跟着的收尾符号并进这一句。
			for end < len(runes) {
				switch runes[end] {
				case '」', '』', '"', '\'', '）', ')', '”':
					end++
					continue
				}
				break
			}
			// 🚨 英文缩写里的点不是句末：U.S. / Mr. / Dr. / J.K.
			//
			// 判据看**点前面那个词有多长**，不是后面那个字母的大小写 ——
			// 「The U.S. sent aid」里，第一个点后面跟的是大写的 S，按大小写判
			// 会把它切成「The U.」「S.」「sent aid…」（实测切出来就是这样）。
			// 缩写和首字母几乎总是一两个字母，真正的句末词通常更长。
			if runes[i] == '.' && end < len(runes) {
				if isLowerASCII(runes[end]) || asciiWordLenBefore(runes, i) < 3 {
					continue
				}
			}
			if s := strings.TrimSpace(string(runes[start:end])); s != "" {
				out = append(out, s)
			}
			start = end
			i = end - 1
		}
	}
	if s := strings.TrimSpace(string(runes[start:])); s != "" {
		out = append(out, s)
	}
	return out
}

func isLowerASCII(r rune) bool { return r >= 'a' && r <= 'z' }

// asciiWordLenBefore —— 位置 i 之前连着多少个英文字母。用来认缩写点。
func asciiWordLenBefore(runes []rune, i int) int {
	n := 0
	for j := i - 1; j >= 0; j-- {
		r := runes[j]
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			n++
			continue
		}
		break
	}
	return n
}

// replyMentionsSentence —— 印记 那句话里，是不是提到了正文这一句。
//
// 它引原文时很少一字不差：会截短、会换掉标点、会只引其中最要紧的半句。所以
// 判据是**这一句里连续的一小段出现在它的回复里**。
//
// 🚨 这一小段该多长，要看是哪种文字。中文八个字已经足够独一无二（「以色列下令
// 空袭」就是八个），英文八个字母到处都是（"the same"），得要二十。按十二一刀切
// 的那一版漏掉了实测里那句真实的截短引用 —— 它只引了十一个字。
func replyMentionsSentence(reply, sentence string) bool {
	window := 20
	if hasCJK(sentence) {
		window = 8
	}
	r := []rune(sentence)
	if len(r) < window {
		return strings.Contains(reply, strings.TrimSpace(sentence))
	}
	for i := 0; i+window <= len(r); i++ {
		if strings.Contains(reply, string(r[i:i+window])) {
			return true
		}
	}
	return false
}
