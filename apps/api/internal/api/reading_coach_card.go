package api

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// reading_coach_card.go — 聊天里那张可点的卡片，以及它的守门人。
//
// 一张卡片值得建，当且仅当它的答案不进文章就产生不出来。choose_span 的选项
// 是文章里真实存在的句子，这才是逼她真的去读的那个东西；模型随手编一句读着
// 很像的话，就把这个保证悄悄拆掉了，而学生永远不会知道。所以这里跟
// validateReadingPicks / validateReadingQuestions 是同一个思路：
// **保证靠输出类型，不靠嘴上叮嘱** —— 不求模型好好引用，而是逐条核对，
// 过不了的整张丢掉。丢掉不是错误：这一轮照常成功，她只是收到一条没有卡片的回复。

// coachCardType 五种。前三种是「说」，后两种是「摆」：
//
//   - choose_span     —— 从文章的几句原话里点一句（options 必填）
//   - pick_in_article —— 请她自己去正文里划一句（没有 options）
//   - short_text      —— 请她用自己的话写一小段（没有 options）
//   - label_roles     —— 把几句原话各自摆到一个角色下面（options 必填）
//   - word_bank       —— 把这一段里的几个词分到「认识 / 不确定 / 不认识」（words 必填）
//
// # 后两种为什么存在（2026-09-10）
//
// 产品负责人的原话：
//
//	> the guiding, now only texts, or questions/choices/text input, is not
//	> enough. we have great 透镜 interaction. and we can be richer.
//	> we can make our ai be able to call out an interactive part and then back
//
// 也就是说：结构没问题（印记 排步骤、领着走），缺的是**手上能摆的东西**。
// 透镜是唯一一件，而它一篇文章只用一两次。
//
// 这两种沿用**同一条回路**（印记 在回复里给一张卡 → 她动手 → 作答回灌成真的
// 一轮），所以它们不是一套平行的运行时，是这张卡片多了两种形状。
//
// 两种都来自 docs/2026-09-10-reading-guidance-redesign.md 里那份调研：
//
//   - label_roles 是 article-active-reading-tutor 的 role labeling。
//     「给这几句各自贴一个角色」是一次**不问「你懂了吗」的理解检查**：
//     贴不出来就是没读懂，而她一个字都不用写。
//   - word_bank 是那份调研里反复出现的「选材来自她自己的回答」：印记 不再对着
//     一整段讲生词，而是先看她把哪几个词划到「不认识」，下一轮只讲那几个。
const (
	coachCardChooseSpan    = "choose_span"
	coachCardPickInArticle = "pick_in_article"
	coachCardShortText     = "short_text"
	coachCardLabelRoles    = "label_roles"
	coachCardWordBank      = "word_bank"
)

// coachCardRoleLabels 是 label_roles 那块板上的格子。
//
// 🚨 **闭表，而且由服务端填**，模型只给句子、给不了标签。理由和学科表一样：
// 模型能编出第六个角色，而第六个角色在板上没有格子、在带读规矩里没有对应的
// 说法。它们也是她真正要学会认的那五种东西 —— 换一篇文章还是这五个格子，
// 那个「认得出」就是学习本身（reading_routines.go 开头那三条里的第 2 条）。
var coachCardRoleLabels = []string{"主张", "证据", "限制", "背景", "对比"}

// coachCardWord 是生词板上的一个词：它出现在哪一段（BlockID），词本身（Term）。
//
// 🚨 没有「释义」这个字段，而且是故意的。板上不摆答案 —— 她先分「认识 /
// 不确定 / 不认识」，印记 下一轮只讲她划到后两格的那几个。把释义一起发下来，
// 这块板当场退化成一张单词表，而那正是调研里说的「把答案先给了，后面每一步
// 都成了走过场」。
type coachCardWord struct {
	BlockID string `json:"blockId"`
	Term    string `json:"term"`
}

const (
	// coachCardPromptMaxRunes 按 rune 数，不是 byte 数 —— 正文是中文，
	// 按 byte 算等于只给了三分之一的额度。
	coachCardPromptMaxRunes = 60
	// coachCardMaxOptions 超过 4 个就不是「点一句」而是阅读理解选择题了。
	coachCardMaxOptions = 4
	// coachCardMinOptions 存活不到 2 个，剩下的那一个就没有可选性可言 ——
	// 跟 validateReadingQuestions 的地板同一个理由：一份单薄的东西比没有更糟。
	coachCardMinOptions = 2
	// coachCardMinQuoteRunes 一两个字确实也是文章的子串，但那不是「一句话」，
	// 渲染出来是张废卡。
	coachCardMinQuoteRunes = 4
	// coachCardMinBlocks 选项必须来自至少两个不同的段落。
	//
	// 🚨 这一条是整个校验器里唯一一条不看单个选项、只看**选项集**的规则，
	// 而它管的恰恰是这张卡片存在的理由。真实走查里出过这么一张：
	//
	//	「第三段里，哪一句让你最清楚地看到钱去了哪里？」
	//	(A)(B)(C) = 第三段的全部三句，按原文顺序排下来
	//
	// 每一条都逐字来自原文、每一条都落在从句边界上、互不包含——校验器全放行了。
	// 但那三个选项**就是第三段本身**：她不必读第 1、2、4 段，甚至不必读第 3 段，
	// 扫一眼选项里的名词就能点。这不是「从文章里挑几句让她选」，这是**把一段话
	// 剁开**。逐字校验保证的是她**看**了文章，保证不了她**读懂**了。
	//
	// 跨段落取选项就是那个保证：比较发生在两段之间，她非把两处都读懂不可。
	// 这同时干掉了「第 X 段里哪一句…」这个本来就最弱的问法。
	coachCardMinBlocks = 2
)

// coachCardOption 是卡片上的一个选项：文章里某一段（BlockID）的某一句原话
// （Quote）。跟 readingPick 同形，也是同一个理由。
type coachCardOption struct {
	BlockID string `json:"blockId"`
	Quote   string `json:"quote"`
}

// coachCard 是 印记 在这一轮回复里附带的一张卡片。**它不带 answer key** ——
// 没有对错、没有分数（铁律②）；卡片只是把「想」这一步交回给她。
type coachCard struct {
	Type   string `json:"type"`
	Prompt string `json:"prompt"` // ≤ 60 字的一句话提问
	// choose_span 和 label_roles 用它：文章里的几句原话。
	Options []coachCardOption `json:"options"`
	// word_bank 用它：这一段里的几个词。
	Words []coachCardWord `json:"words,omitempty"`
	// label_roles 那块板上的格子。**服务端填的**，模型给不了 —— 见
	// coachCardRoleLabels。模型如果自己塞了一份，这里会被覆盖掉。
	Labels []string `json:"labels,omitempty"`
}

const (
	// 生词板上的词：少于 3 个不成一块板，多于 6 个就变成一张单词表了。
	// 上限跟调研里那几个 skill 的口径一致（一次 4–6 个高价值词，宁缺毋滥）。
	coachCardMinWords = 3
	coachCardMaxWords = 6
	// 一个「词」最长四个英文单词 —— 再长就不是词，是句子，那是 label_roles
	// 该做的事。
	coachCardMaxWordTokens = 4
)

// cardReject 说的是这张卡片为什么没发出去。
//
// 🚨 它存在的理由和 planReject 一模一样（reading_plan.go）：卡片被丢掉是**静默**
// 的，屏幕上只是少了一张卡，日志里一个字都没有。2026-09-10 的走查里 印记 连着
// 两轮在说「这张板上有四句话，把它们拖到格子里」而板从来没出现过 —— 查不出为
// 什么，只能靠猜。丢掉仍然是对的做法，但**丢掉的理由必须说出来**。
type cardReject string

const (
	cardOK                cardReject = ""
	cardRejectNoCard      cardReject = "no card in the reply"
	cardRejectUnknownType cardReject = "unknown card type"
	cardRejectPromptLen   cardReject = "prompt empty or over the cap"
	cardRejectBannedForm  cardReject = "question is one of the banned forms"
	cardRejectFewWords    cardReject = "fewer than 3 words survived the article check"
	cardRejectFewOptions  cardReject = "fewer than 2 options survived the article check"
	cardRejectOneBlock    cardReject = "every surviving option came from one paragraph"
)

// validateCoachCard 校验模型给出的这张卡片，不合格返回 nil —— 静默丢弃，
// 不报错、不渲染残卡。返回的是一张新卡片，调用方手里那张不会被就地改写。
// 只关心「发不发得出去」的调用方用它；要知道为什么没过的用 validateCoachCardWhy。
//
// 规则：
//   - Type 必须是五种之一；
//   - Prompt 去空白后非空、≤ 60 runes，且不是被禁的那几种问法
//     （见 rejectBannedQuestion）；
//   - choose_span / label_roles：每个 Quote 必须是**它自己那个 BlockID** 的字面
//     子串（挂错段落 = 不算）、**落在从句边界上**（见 coachCardQuoteIsClause）、
//     去空、去太短、去重（含**包含式**去重：一个选项是另一个的子串就丢掉短的）、
//     截断到 4 个，存活 < 2 → 整张丢掉；
//     **choose_span 还要求存活的选项跨至少两段**（见 coachCardMinBlocks），
//     label_roles **不要求** —— 理由见下面那段；
//   - word_bank：每个词必须按词边界出现在它那一段里，存活 < 3 → 整张丢掉；
//   - pick_in_article / short_text：忽略并清空 options
//     （问题本身就是「去文章里找」，给了选项反而把这件事替她做了）。
func validateCoachCard(c *coachCard, blocks []Block) *coachCard {
	card, _ := validateCoachCardWhy(c, blocks)
	return card
}

// validateCoachCardWhy 多返回一个「为什么没过」，给日志用。
func validateCoachCardWhy(c *coachCard, blocks []Block) (*coachCard, cardReject) {
	if c == nil {
		return nil, cardRejectNoCard
	}
	switch c.Type {
	case coachCardChooseSpan, coachCardPickInArticle, coachCardShortText,
		coachCardLabelRoles, coachCardWordBank:
	default:
		return nil, cardRejectUnknownType
	}
	prompt := strings.TrimSpace(c.Prompt)
	if prompt == "" || utf8.RuneCountInString(prompt) > coachCardPromptMaxRunes {
		return nil, cardRejectPromptLen
	}
	// 🚨 一句一句定义就能打发的问题，整张卡丢掉。见 rejectBannedQuestion。
	if rejectBannedQuestion(prompt) {
		return nil, cardRejectBannedForm
	}
	if c.Type == coachCardWordBank {
		words := validateCardWords(c.Words, blocks)
		if len(words) < coachCardMinWords {
			return nil, cardRejectFewWords
		}
		return &coachCard{Type: c.Type, Prompt: prompt, Words: words}, cardOK
	}
	if c.Type != coachCardChooseSpan && c.Type != coachCardLabelRoles {
		return &coachCard{Type: c.Type, Prompt: prompt}, cardOK
	}

	byID := make(map[string]string, len(blocks))
	for _, b := range blocks {
		byID[b.ID] = b.Text
	}
	seen := make(map[string]bool, len(c.Options))
	kept := make([]coachCardOption, 0, len(c.Options))
	for _, o := range c.Options {
		q := strings.TrimSpace(o.Quote)
		if q == "" || utf8.RuneCountInString(q) < coachCardMinQuoteRunes {
			continue
		}
		body, ok := byID[o.BlockID]
		if !ok || !coachCardQuoteIsClause(body, q) {
			continue
		}
		// 按她**看得见的那句话**去重：同一句从两个段落各来一次，屏幕上就是
		// 两个一模一样的选项，点哪个都没有区别。
		if seen[q] {
			continue
		}
		seen[q] = true
		kept = append(kept, coachCardOption{BlockID: o.BlockID, Quote: q})
	}
	// 包含式去重，在截断到 4 之前跑：两个各自都落在合法边界上的重叠片段
	// （「白天吸热、夜里放热」和「夜里放热」）边界规则合并不掉，但摆在同一张
	// 卡片上就是一组套娃 —— 「挑一句」这件事当场变得莫名其妙。留长的那个：
	// 它信息更完整，短的那半句她在长的里面照样读得到。
	//
	// 🚨 比较的对象必须是**会活到最后的那批**，不是原始候选池。拿整个未截断的
	// 候选池当参照会这样翻车：候选 `[A, B, C, D, E, G]`，A 短、G 长且包含 A、
	// G 排在最后 —— A 因为「池子里有更完整的 G」被丢掉，然后 B–E 填满 4 个名额、
	// 循环收工，G 根本没被走到。她拿到的卡片上 A 和 G **都没有**，而丢掉 A 的
	// 那条理由（「更完整的那句她照样读得到」）指的正是那张卡片上不存在的 G。
	// 所以这里改成边扫边攒：只跟已经进了 out 的比，长的**就地顶掉**短的那个位置
	// （不是排到队尾 —— 排到队尾照样会被截断切掉，等于白留），最后再截到 4。
	out := make([]coachCardOption, 0, len(kept))
	for _, o := range kept {
		if coachCardIsSwallowedBy(o.Quote, out) {
			continue
		}
		out = coachCardPlace(out, o)
	}
	if len(out) > coachCardMaxOptions {
		out = out[:coachCardMaxOptions]
	}
	if len(out) < coachCardMinOptions {
		return nil, cardRejectFewOptions
	}
	// 🚨 跨段落这一条必须在**截断之后**判，判的是她屏幕上真正会出现的那几条。
	// 放在截断之前判会这样漏：候选里第 5 条来自另一段，前 4 条全在同一段——
	// 截断把那唯一的第二段切掉，卡片照样发出去，而她看到的仍然是「一段话被剁开」。
	// 规则管的是最终的选项集，那就只能在选项集定下来之后判。
	// 🚨 跨段落这一条**只管 choose_span，不管 label_roles**。
	//
	// 它存在的理由（见 coachCardMinBlocks）是：几个选项全出自同一段的时候，
	// **选项就是那一段** —— 她不必读别的段落，扫一眼选项里的名词就能点。
	//
	// 那条推理在标注板上不成立。标注板要她给每一句**贴一个角色**（主张 / 证据 /
	// 限制 / 背景 / 对比），而角色是扫名词扫不出来的：同一段里的两句「后果」和
	// 「原因」，正是关系最紧、也最值得让她分辨的一对。硬要跨段反而把这块板最好
	// 的用法禁掉了。
	//
	// 这不是推测：线上第一次跑，印记 连着两轮想给她一块板（「哪一句是马上会发生
	// 的后果，哪一句是原因？」），两次都被这条规则丢掉，而她屏幕上只看到 印记
	// 在描述一块从来没出现过的板。日志里那两行写着
	// 「every surviving option came from one paragraph」。
	if c.Type == coachCardChooseSpan && !coachCardSpansBlocks(out) {
		return nil, cardRejectOneBlock
	}
	card := &coachCard{Type: c.Type, Prompt: prompt, Options: out}
	if c.Type == coachCardLabelRoles {
		// 格子由服务端填。模型自己塞的那份（如果有）在这里被覆盖掉：
		// 见 coachCardRoleLabels。
		card.Labels = coachCardRoleLabels
	}
	return card, cardOK
}

// validateCardWords 把生词板上那几个词收进「确实在那一段里」的范围。
//
// 和句子那一侧同一条思路：**保证靠核对，不靠嘱咐**。一个模型顺手写出来的、
// 文章里根本没有的词，在板上和一个真词长得一模一样，而她会把它背下来。
//
// 核对是**整词**核对，不是子串：`art` 是 `article` 的子串，但它不是这一段里
// 出现过的词。英文按词边界判；中文没有词边界，退回子串（中文文章的生词板本来
// 也不是主要用途 —— 段落工具里的「关键单词」才是）。
func validateCardWords(words []coachCardWord, blocks []Block) []coachCardWord {
	byID := make(map[string]string, len(blocks))
	for _, b := range blocks {
		byID[b.ID] = b.Text
	}
	seen := make(map[string]bool, len(words))
	out := make([]coachCardWord, 0, len(words))
	for _, w := range words {
		term := strings.TrimSpace(w.Term)
		if term == "" {
			continue
		}
		if len(strings.Fields(term)) > coachCardMaxWordTokens {
			continue
		}
		body, ok := byID[w.BlockID]
		if !ok || !blockContainsTerm(body, term) {
			continue
		}
		// 大小写不敏感地去重：`Aid` 和 `aid` 摆在板上是同一个词。
		key := strings.ToLower(term)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, coachCardWord{BlockID: w.BlockID, Term: term})
		if len(out) == coachCardMaxWords {
			break
		}
	}
	return out
}

// blockContainsTerm —— term 是不是 body 里真的出现过的一个词。
//
// 英文（term 里有 ASCII 字母）按**词边界**判：两侧不能再接字母或数字。
// 否则退回子串（中文没有词边界可判）。大小写不敏感 —— 句首那个词首字母大写，
// 而板上摆的是词本身。
func blockContainsTerm(body, term string) bool {
	if !hasASCIILetter(term) {
		return strings.Contains(body, term)
	}
	lowerBody := strings.ToLower(body)
	lowerTerm := strings.ToLower(term)
	for i := 0; ; {
		j := strings.Index(lowerBody[i:], lowerTerm)
		if j < 0 {
			return false
		}
		start := i + j
		end := start + len(lowerTerm)
		if !isWordByte(lowerBody, start-1) && !isWordByte(lowerBody, end) {
			return true
		}
		i = start + 1
		if i >= len(lowerBody) {
			return false
		}
	}
}

func hasASCIILetter(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

// isWordByte —— s 的第 i 个字节是不是一个会把词粘住的字符。越界当成不是。
func isWordByte(s string, i int) bool {
	if i < 0 || i >= len(s) {
		return false
	}
	c := s[i]
	return c == '\'' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// bannedQuestionForms —— 一句定义就能打发的问题，没有承重。
//
// 来自 ljg-qa 的 QuestionDesign（见 docs/2026-09-10-reading-guidance-redesign.md）。
// 它顺带点出了模型的默认毛病：「AI 默认会写「什么是 X」型问题 —— 教科书腔」。
//
// 🚨 写成代码而不是只写进 prompt，是因为
// [[prompt-output-must-be-verifiable-2026-09-03]]：prompt 里的「必须」如果代码
// 里验不了，它就只是一句期望。命中就把整张卡丢掉 —— 这一轮她收到一条没有卡片
// 的回复，和引文核不上原文时是同一种处理。
var bannedQuestionForms = []*regexp.Regexp{
	// 「什么是 X？」「X 是什么？」—— 一句定义打发。
	regexp.MustCompile(`^什么是[^？?]{1,20}[？?]?$`),
	regexp.MustCompile(`^[^？?]{1,20}是什么[？?]?$`),
	// 「X 有几个步骤 / 几个部分 / 哪几点」—— 问的是目录。
	regexp.MustCompile(`有(几个|哪几个|哪些)(步骤|部分|阶段|要点|方面)`),
	// 「X 重要吗」—— 答案预设。
	regexp.MustCompile(`(重要|关键)吗[？?]?$`),
	// 「我们应当如何看待 X」—— 学术腔，没有具体动作。
	regexp.MustCompile(`(我们)?应(当|该)(如何|怎样|怎么)看待`),
	// 「X 的优缺点是什么」—— 商学院八股。
	regexp.MustCompile(`(优缺点|优点和缺点|利弊)(是什么|有哪些)`),
	// 「这段讲了什么」—— prompt 里本来就点名禁止的那种空问题。
	regexp.MustCompile(`^这(一)?段(主要)?(讲|说)了?什么[？?]?$`),
}

// rejectBannedQuestion —— 这个问题是不是上面那几种。
func rejectBannedQuestion(prompt string) bool {
	p := strings.TrimSpace(prompt)
	for _, re := range bannedQuestionForms {
		if re.MatchString(p) {
			return true
		}
	}
	return false
}

// coachCardSpansBlocks —— 这批选项是不是来自至少 coachCardMinBlocks 个不同的
// 段落。理由见 coachCardMinBlocks：选项全挤在一段里的时候，选项就是那一段。
func coachCardSpansBlocks(opts []coachCardOption) bool {
	seen := make(map[string]bool, len(opts))
	for _, o := range opts {
		seen[o.BlockID] = true
		if len(seen) >= coachCardMinBlocks {
			return true
		}
	}
	return false
}

// coachCardSwallows —— outer 是不是把 inner 整个吞掉了。相等的两条在这之前
// 已经被 seen 去掉了，所以这里只认真子串（严格更长的那个）。
func coachCardSwallows(outer, inner string) bool {
	return len(outer) > len(inner) && strings.Contains(outer, inner)
}

// coachCardIsSwallowedBy —— quote 是不是 out 里某一条的真子串。参照系是 out
// （已经站住脚的那批），不是原始候选池：理由见 validateCoachCard 里的那段。
func coachCardIsSwallowedBy(quote string, out []coachCardOption) bool {
	for _, e := range out {
		if coachCardSwallows(e.Quote, quote) {
			return true
		}
	}
	return false
}

// coachCardPlace —— 把 o 放进 out，并顶掉 out 里被 o 包含的那些。
// **顶替是就地的**：长的接手第一条被它吞掉的那个位置，顺序不变。排到队尾就会被
// 后面的截断切掉，那样「留长的」这条规则留下来的东西根本到不了卡片上。
func coachCardPlace(out []coachCardOption, o coachCardOption) []coachCardOption {
	next := make([]coachCardOption, 0, len(out)+1)
	placed := false
	for _, e := range out {
		if coachCardSwallows(o.Quote, e.Quote) {
			if !placed {
				next = append(next, o)
				placed = true
			}
			continue
		}
		next = append(next, e)
	}
	if !placed {
		next = append(next, o)
	}
	return next
}

// coachCardIsBoundary —— 从句边界字符。两套标点都要有：正文可能是中文，也可能
// 是英文（或者中英混排的一段）。
//
// 破折号和省略号也在里面：中学生读的说明文 / 科普 / 新闻里，`——` 和 `……`
// 一篇通常至少出现一处（`他说了一件事——城市在夜里更热。`）。少了它们，破折号
// 后面起头的那一句就被判成「从句子中间截的」，而误杀是**静默的**：选项被丢 →
// 存活不足 2 → 整张卡片消失，屏幕上看起来就像 印记 这一轮没想出卡片。成对出现的
// `——` / `……` 不用单独处理：撞到第一个就已经是边界了。
func coachCardIsBoundary(r rune) bool {
	switch r {
	case '，', '。', '！', '？', '；', '：', '、', '\n',
		',', '.', '!', '?', ';', ':',
		'—', '…':
		return true
	}
	return false
}

// coachCardIsSkippable —— 找边界的路上可以跳过不计的字符：空白，以及成对的
// 引号 / 括号。
//
// 为什么引号必须跳过：`他说：“城市在夜里更热。”` 里那一句的边界标点（`：`）
// 落在引号**外面**，引号本身不是边界字符。不跳过它，这一整类正常引文——中文
// 「“ ”」「‘ ’」「「 」」「『 』」、括号、英文 `" '`——全都会被误杀，而误杀是
// 静默的：选项被丢 → 存活不足 2 → 整张卡片消失，屏幕上看起来就像 印记 这一轮
// 没想出卡片。跳过引号并不放宽「一句话」这件事：引号里从句子中间切的窗口，
// 跳过引号之后撞到的仍然是一个汉字，照样过不了。
func coachCardIsSkippable(r rune) bool {
	if unicode.IsSpace(r) {
		return true
	}
	switch r {
	case '“', '”', '‘', '’', '「', '」', '『', '』', '（', '）', '(', ')', '"', '\'':
		return true
	}
	return false
}

// coachCardQuoteIsClause —— Quote 必须是 body 的字面子串，**并且落在从句边界上**。
//
// 为什么光有 ≥4 runes 的地板不够：地板管的是**长度**，不是「是不是一句话」。
// 「表以沥青和混凝土」从词中间切开，两头都不在标点上，却确确实实是原文的子串 ——
// 这种窗口靠扫几个字就能凑出来，而一句话必须被当作一个整体读过。整个校验器存在
// 的理由就是后面这件事（不进文章就答不出来），所以边界这一层不能省。
//
// 边界的定义（三头都是「一句话」的合法收口）：
//   - 开头：block 的开头，或前面紧挨着一个边界字符；
//   - 结尾：block 的结尾，或后面紧挨着一个边界字符，或它**自己以边界字符收尾**
//     （模型经常把句号一起引进来，那是正常引用，不是毛病）。
//
// 🚨 故意往**松**了收：同一句话在段里出现多次时，只要**有一次**落在边界上就算数；
// 边界字符和引文之间夹着的空白（英文 "…day. They…" 的那个空格）跳过不计，
// 成对的引号 / 括号（`他说：“…”` 的那对引号）同样跳过不计（见 coachCardIsSkippable）。
// 收得过紧是看不见的 —— 误杀的卡片不报错、不打日志，看起来就像模型这一轮
// 没想出卡片；宁可放过一个窗口，也不能让正常的句子静悄悄消失。
func coachCardQuoteIsClause(body, quote string) bool {
	if quote == "" {
		return false
	}
	last, _ := utf8.DecodeLastRuneInString(quote)
	endsOnItsOwnBoundary := coachCardIsBoundary(last)
	for off := 0; off <= len(body)-len(quote); {
		i := strings.Index(body[off:], quote)
		if i < 0 {
			return false
		}
		start := off + i
		if coachCardClauseStarts(body, start) &&
			(endsOnItsOwnBoundary || coachCardClauseEnds(body, start+len(quote))) {
			return true
		}
		off = start + 1
	}
	return false
}

// coachCardClauseStarts —— start 这个字节位置是不是一句话的开头。
func coachCardClauseStarts(body string, start int) bool {
	for start > 0 {
		r, size := utf8.DecodeLastRuneInString(body[:start])
		if coachCardIsBoundary(r) {
			return true
		}
		if !coachCardIsSkippable(r) {
			return false
		}
		start -= size
	}
	return true // block 的开头
}

// coachCardClauseEnds —— end 这个字节位置是不是一句话的收口。
func coachCardClauseEnds(body string, end int) bool {
	for end < len(body) {
		r, size := utf8.DecodeRuneInString(body[end:])
		if coachCardIsBoundary(r) {
			return true
		}
		if !coachCardIsSkippable(r) {
			return false
		}
		end += size
	}
	return true // block 的结尾
}

// coachMessagePayload is what a chat message carries besides its words
// (`atom_message.payload`, migration 0106). On the AI side that is the card it
// just wrote. It is an envelope rather than the bare card on purpose: a reader
// holding only the JSON has to be able to tell 「这条消息带了一张卡」 apart
// from whatever else a message will carry later (her answer to one).
//
// 🚨 Deliberately NOT atom_card: `atom_card_one_open_idx` (0096) permits one
// open card per atom, so a chat card stored there would deadlock every lens
// summon — and `card_id` there must resolve in cards.ByID / CARD_REGISTRY,
// which a card the model wrote on the spot never will.
type coachMessagePayload struct {
	Card *coachCard `json:"card,omitempty"`
	// Answer is the other half of the envelope: on HER side of the transcript,
	// what she tapped. The words themselves are already in `content` (as `> `
	// lines when they are the article's), but a reader of the content alone
	// cannot tell 「她点了卡片上的第二个选项」 apart from 「她引用了一句然后打字」.
	// Storing the structured answer is what lets the room re-render the card
	// she already answered after a refresh, instead of showing a live card
	// waiting for a tap she has made.
	Answer *coachCardAnswer `json:"answer,omitempty"`
}

// coachCardAnswer is her answer to a chat card: which card it was, what it
// asked, and what she chose. `Choice` is ARTICLE TEXT for choose_span and
// pick_in_article, and HER OWN words for short_text — which is exactly why
// nothing downstream is allowed to trust this field's `Type` to tell the two
// apart. See composeCardAnswerMessage: what goes into the transcript is
// classified by CHECKING the choice against the article, not by believing
// the type the client declared.
type coachCardAnswer struct {
	Type    string `json:"type,omitempty"`
	Prompt  string `json:"prompt,omitempty"`
	Choice  string `json:"choice,omitempty"`
	BlockID string `json:"blockId,omitempty"`
}

// coachCardAnswerPayload renders her answer into the jsonb column's bytes,
// the mirror of coachCardPayload. Nil answer → nil bytes → SQL NULL.
func coachCardAnswerPayload(a *coachCardAnswer) []byte {
	if a == nil {
		return nil
	}
	// 每个字段都是 omitempty，所以一个空壳 answer 会 marshal 成
	// `{"answer":{}}` —— 一条她普通打字的消息就此被重渲染逻辑当成「卡片回答」，
	// 屏幕上凭空多出一张她从没答过的卡。至少要有 type 或 choice 才叫答案。
	if strings.TrimSpace(a.Type) == "" && strings.TrimSpace(a.Choice) == "" {
		return nil
	}
	b, err := json.Marshal(coachMessagePayload{Answer: a})
	if err != nil {
		// Same reasoning as coachCardPayload: a struct of strings cannot fail
		// to marshal, and if it somehow did, her turn still stands — the words
		// are in `content`, which is the part the next turn actually reads.
		return nil
	}
	return b
}

// coachCardPayload renders a card into the jsonb column's bytes. A nil card
// gives nil bytes, which the column stores as SQL NULL — that is the whole
// reason the column is nullable: carrying nothing is the normal case, not one
// every caller has to build an empty shell for.
func coachCardPayload(c *coachCard) []byte {
	if c == nil {
		return nil
	}
	b, err := json.Marshal(coachMessagePayload{Card: c})
	if err != nil {
		// A struct of strings cannot fail to marshal; if it somehow did, the
		// turn is still hers — she loses the card, not the reply.
		return nil
	}
	return b
}
