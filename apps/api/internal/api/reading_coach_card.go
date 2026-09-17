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
	// Where 是这句话在哪一段，她看得懂的那个写法（「第4段」）。
	//
	// 🚨 **服务端填，模型给不了**（解析出来的那一份在 validateCoachCardWhy 里
	// 被原样丢弃），理由和段号在别处一样：让模型从 b1 数出「第一段」，它会数错，
	// 而她屏幕上那个号码是服务端给的 —— 两边对不上，她照着去找就找不到。
	//
	// 为什么要有它：产品负责人 2026-09-17 逐字报的「对整体拆分时，选择的都是
	// 单句，并未标注段落，有时候单独的句子拆出来很难看出属于什么部分」。
	// 一块标注板上四句话摆在一起，不说它们各自从哪儿来，她没法判断哪句在撑哪句。
	Where string `json:"where,omitempty"`
}

// stampOptionWhere 给每个选项补上「第几段」。段号和 readingBlockTag /
// readingPickOrdinal 数的是同一套（从 1 起、每一段都算），所以卡片上的号码
// 和正文旁边那个号码永远是同一个。
func stampOptionWhere(opts []coachCardOption, blocks []Block) []coachCardOption {
	ord := make(map[string]int, len(blocks))
	for i, b := range blocks {
		ord[b.ID] = i + 1
	}
	out := make([]coachCardOption, 0, len(opts))
	for _, o := range opts {
		where := ""
		if n := ord[o.BlockID]; n > 0 {
			where = "第" + itoaSmall(n) + "段"
		}
		out = append(out, coachCardOption{BlockID: o.BlockID, Quote: o.Quote, Where: where})
	}
	return out
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
	cardRejectPromised    cardReject = "the reply promises a card but none was attached"
	cardRejectLensWon     cardReject = "a lens was given this turn, so the card was dropped (铁律③)"
	cardRejectCutOff      cardReject = "the reply ends mid-sentence"
	cardRejectDeadTurn    cardReject = "the turn hands her nothing to do"
	cardRejectNoArgument  cardReject = "a 主张/证据/限制 board on an article whose author makes no argument"
)

// replyLooksCutOff —— 这句话像不像说到一半断掉了。
//
// 🚨 线上实测（2026-09-11 走查）她看到的：
//
//	「……他们同样在讲封锁的后果，但说得更具体：不是」
//
// 159 个字，就这么断在「不是」上。她读到的是半句话，当场不知道这一步要干嘛。
// 我们这边不截断任何东西（max_tokens 是 16000），所以它是模型自己写出来的。
//
// 判据：**中文的一句话总要有个收尾**。句号、问号、叹号、引号、右括号都算收尾；
// 冒号和逗号不算 —— 「现在点出那一句：」后面本该跟着一张卡片。
//
// 所以这条只在**没有附卡片、也没有透镜**的时候才判：真的带着卡片时，一个冒号
// 收尾是完全正常的写法（卡片自己会把话说完）。
func replyLooksCutOff(reply string) bool {
	r := []rune(strings.TrimSpace(reply))
	if len(r) == 0 {
		return false
	}
	switch r[len(r)-1] {
	case '。', '！', '？', '」', '』', '）', '…', '.', '!', '?', ')', '"', '\'', '~':
		return false
	}
	return true
}

// cardFixIt —— 每一种理由对应的**怎么改**，中文，一句话。
//
// 🚨 光把理由喂回去不够。第一版喂的是那句英文标识（「every surviving option
// came from one paragraph」），线上实测 印记 收到之后连着六轮出同一张卡：
// 它知道自己错了，不知道该改哪儿。理由是**给日志看的**，这一句是**给它看的**。
var cardFixIt = map[cardReject]string{
	cardRejectOneBlock: "选项不能全出自同一段。换成跨段落取：这一段一句、" +
		"另一段一句，让她非把两处放在一起比不可。做不到就这一轮不发卡。",
	cardRejectFewOptions: "选项没能和原文对上。每一句都要**逐字**抄自你标的那一段，" +
		"一个字都不能改；而且要从一个标点后面开始、到一个标点为止，不要从半句中间截。",
	cardRejectFewWords: "生词板上的词没能和原文对上。每个词都要原样出现在你标的那一段里，" +
		"至少三个。",
	cardRejectBannedForm: "换一个问法：问动作（他是怎么做到的）、问对比（为什么是 A 不是 B）、" +
		"问因果（这一步凭什么成立）、或者问边界（它什么时候不成立）。",
	cardRejectPromptLen: "问题写成一句话，不超过 60 个字。",
	cardRejectUnknownType: "type 只能是 choose_span / pick_in_article / short_text / " +
		"label_roles / word_bank 五个之一。",
	cardRejectPromised: "你在话里提到了一张卡片，但 JSON 里没有 card 这个键 —— " +
		"她那边什么都没出现。要给就真的给，不给就别提。",
	cardRejectCutOff: "你上一轮那句话断在半句上，她读到的是半截。" +
		"这一轮把话说完整，句子要有收尾。",
	cardRejectDeadTurn: "你上一轮讲完就停了，没有请她做任何事 —— 她屏幕上没有卡片、" +
		"没有透镜，也没有一句话告诉她下一步。这一轮结尾要么给一张卡片，" +
		"要么明确请她做一件事。",
	cardRejectLensWon: "你同一轮既给了透镜又给了卡片。一次只交给她一件事，" +
		"所以卡片被拿掉了 —— 她那边只有那副透镜。想让她点卡片，这一轮就别给透镜。",
	cardRejectNoArgument: "这篇文章的作者没有在说服谁（它是报道 / 记叙），" +
		"所以「主张 / 证据 / 限制」这块板在这篇上没有指称对象 —— 她只能猜。" +
		"换一种：choose_span（在几句里挑一句）、short_text（请她写一句）、" +
		"word_bank（生词板），或者 pick_in_article（请她在文章里点一句）。",
}

// cardPromiseWords —— 这句回复是不是在**指着一张卡片说话**。
//
// 🚨 它抓的是另一种失败，和「卡片被丢掉」不是一回事：模型压根没在 JSON 里给
// card，却在 reply 里说「我给你一张标注板」「点这张卡」「把这四句拖到格子里」。
// 线上实测（2026-09-10，OSIRIS-REx 那篇）：印记 连着三轮在说
// 「标注板还在屏幕上，四句话等着你」——而它一次都没有真的发出去过。
// 日志里干干净净，因为没有东西被丢掉，是根本没有东西。
//
// 对她来说这两种失败长得一模一样：屏幕上有一句指着空气的话。
// 所以处理也一样 —— 记下来，下一轮当面告诉它。
//
// 判据只认**明确指着一个可点对象**的说法。「选一句」「找一句」不算：
// 那些话在没有卡片的时候也成立（她可以在正文里划选）。
//
// 🚨 这张表宁可宽一点。第一版写得太紧，漏掉了「五格的板让我拖句子进去」——
// 「拖进」不是「拖句子进去」的子串，于是这一轮照样发了一句指着空气的话出去。
// 漏判的代价是她卡死；误判的代价只是多问模型一次（见调用点那条重试）。
// 所以「板」只要带上量词或「格」就算，「格子」本身也算 —— 那是板上格子的名字。
var cardPromiseWords = []string{
	// 卡片
	"卡片", "这张卡", "那张卡", "点这张", "点下面", "下面这张", "上面这张",
	// 板
	"标注板", "生词板", "块板", "格的板", "格子",
	// 动作
	"拖到", "拖进", "拖句", "各自拖",
	// 🚨 上一条 prompt 教它改口说「把这几句各自放进它的角色里」，而这张表里
	// 一个字都没对上 —— 于是板一块都没建，她看到的是「放进它的角色里」加上
	// 一片空白。教它换一种说法的时候，这张表要跟着换。
	"放进", "各自放", "归到", "分到", "角色里", "哪个角色", "它的角色",
}

// replyPromisesACard —— 这句回复有没有在指着一张卡片。
func replyPromisesACard(reply string) bool {
	for _, w := range cardPromiseWords {
		if strings.Contains(reply, w) {
			return true
		}
	}
	return false
}

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
		if !ok {
			continue
		}
		if !coachCardQuoteIsClause(body, q) {
			// 🚨 对不上就**贴回原文里最像的那一句**，而不是整张卡丢掉。
			//
			// 模型抄原句时最常见的失手是差一点点：少一个逗号、把两句并成一句、
			// 从半句中间起头。整张卡丢掉的代价她全担着 —— 实测那一幕是 印记
			// 说「我们集中看第 2 段」然后什么都没给，她逐字报的是「只有一个
			// 输入框，不知道该往里面打什么字」。
			//
			// 贴回去只会让卡片**更**忠于原文：落点段是它自己标的，最终上卡的
			// 那句话逐字来自那一段，下面那道从句边界照常还要过一遍。
			snapped := snapQuoteToArticle(body, q)
			if snapped == "" || !coachCardQuoteIsClause(body, snapped) {
				continue
			}
			q = snapped
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
	// 🚨 选项全来自同一段就丢掉这张卡 —— 扫一眼选项里的名词就能点，等于一个字
	// 都没读懂。丢掉是对的，但**不能让这一轮空着**：调用点会立刻重来一次
	// （见 reading_coach.go 那条重试），把「怎么改」当面交给它。
	//
	// 线上逐字证据（atom 609f3910，2026-09-11）seq 36 就是这么掉的，而当时没有
	// 重试：接着 印记 连着两轮跟她道歉「卡没送到你手里」，她连着两轮回「没有卡啊」。
	if c.Type == coachCardChooseSpan && !coachCardSpansBlocks(out) {
		return nil, cardRejectOneBlock
	}
	// 段号在这里补上，最后一步 —— 补在最终的那几条上，不在候选池上：
	// 中间还有去重、套娃剔除和截断，补早了等于给一批不会上卡的选项算号码。
	card := &coachCard{Type: c.Type, Prompt: prompt, Options: stampOptionWhere(out, blocks)}
	if c.Type == coachCardLabelRoles {
		// 格子由服务端填。模型自己塞的那份（如果有）在这里被覆盖掉：
		// 见 coachCardRoleLabels。
		card.Labels = coachCardRoleLabels
		// 🚨 题目里另起一套格子名的，把题目换成标准那一句。
		//
		// 格子被覆盖了，题目没有 —— 于是屏幕上是「把卡片放进『进不去/动不了/
		// 快撑不住了』三个格子」，而下面摆着的是主张/证据/限制/背景/对比。
		// 她逐字报的：「名字完全不一样，我不知道哪个对应哪个，没法往下做。」
		//
		// 覆盖而不是丢卡：格子本来就是我们的，这一句也是（兜底摆板时用的就是
		// 它）。丢掉的话她这一步什么都没有。
		if labelPromptInventsBins(card.Prompt) ||
			promptTellsHerHowToDrag(card.Prompt) ||
			promptCountMismatch(card.Prompt, len(card.Options)) {
			card.Prompt = coachLabelBoardPrompt
		}
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
	// Dropped 是这一轮 印记 写了一张卡、而它没能发出去的时候，那条理由。
	//
	// 🚨 它存在的理由是「闭环」也要覆盖失败那一侧。线上实测：印记 连着六轮在说
	// 「点这张卡，它会让你从第 2 段里挑一句」，而那张卡每一轮都被跨段落那条规则
	// 丢掉 —— 她屏幕上只有一句句「点这张卡」，指着一张不存在的卡。
	//
	// 模型没有办法自己发现这件事：它写完就交出去了，下一轮的上文里只有它自己
	// 说过的话，看不出卡片有没有到。所以理由存下来，下一轮当面告诉它。
	// 存在消息的 payload 上而不是另开一张表：它属于**那一条回复**，
	// 一起写、一起读、一起被删。
	Dropped string `json:"dropped,omitempty"`
	// Incomplete 表示这条回复**没说完**就交给她了。
	//
	// 🚨 断句的回复本来就会重来一次，但两次都断的时候我们仍然把第一次那句给她
	// （不编、不改写它的话）。产品负责人 2026-09-12 逐字报的那一幕：屏幕上是
	// 「对，调用数据是一个方向。**但」，然后就没有了，她只能自己打一个「?」去问。
	//
	// 半句话本身不是错 —— 错的是**没有任何东西告诉她这是半句**。所以这里只做
	// 一件事：把「这条没说完」这个事实标在那条消息上，由界面照实说出来。
	Incomplete bool `json:"incomplete,omitempty"`
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
	return coachCardPayloadWithDrop(c, cardOK)
}

// coachCardPayloadWithDrop 同上，外加「这一轮那张卡为什么没发出去」。
// 两个都空的时候不写 payload —— 大多数轮本来就是这样。
func coachCardPayloadWithDrop(c *coachCard, why cardReject) []byte {
	return coachCardPayloadFull(c, why, false)
}

// coachCardPayloadFull —— 同上，外加「这条回复没说完」这个事实。
func coachCardPayloadFull(c *coachCard, why cardReject, incomplete bool) []byte {
	if c == nil && !incomplete && (why == cardOK || why == cardRejectNoCard) {
		return nil
	}
	b, err := json.Marshal(coachMessagePayload{Card: c, Dropped: string(why), Incomplete: incomplete})
	if err != nil {
		// A struct of strings cannot fail to marshal; if it somehow did, the
		// turn is still hers — she loses the card, not the reply.
		return nil
	}
	return b
}

// snapQuoteToArticle —— 把一句对不上的引文贴回这一段里最像的那句话。
//
// 判据是**这句引文里的字有多少落在那句话里**，不是反过来：模型常见的失手是把
// 两句并成一句、或者从半句中间起头，那种情况下引文比真句子长。门槛定在七成，
// 低于它就当它说的是别的句子，宁可丢掉 —— 贴错一句比没有卡片更糟，她会照着
// 一句文章里没有的话去找。
func snapQuoteToArticle(body, quote string) string {
	want := quoteRuneSet(quote)
	if len(want) == 0 {
		return ""
	}
	best, bestScore := "", 0.0
	for _, sent := range splitSentences(body) {
		if utf8.RuneCountInString(sent) < coachCardMinQuoteRunes {
			continue
		}
		have := quoteRuneSet(sent)
		hit := 0
		for r := range want {
			if have[r] {
				hit++
			}
		}
		score := float64(hit) / float64(len(want))
		if score > bestScore {
			best, bestScore = strings.TrimSpace(sent), score
		}
	}
	if bestScore < 0.7 {
		return ""
	}
	return best
}

// quoteRuneSet —— 一句话里出现过哪些字，标点和空白不算。
func quoteRuneSet(s string) map[rune]bool {
	out := make(map[rune]bool, len(s))
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			out[r] = true
		}
	}
	return out
}

// replyAsksForSomething —— 这一轮有没有请她做点什么。
//
// 🚨 一轮里既没有卡片、也没有透镜、又没有推进步骤，还不请她做任何事，那她
// 屏幕上就只剩一句讲完的话和一个灰着的发送键。实测她逐字报的：
// 「它说完了 scramble 的意思……但没有告诉我下一步要做什么，发送按钮也是灰的，
// 我不知道该继续等还是要点别的地方。」
//
// 判据取最宽的那一个：一个问号，或者一个请她动手的词。宽是故意的 —— 这条要
// 触发的是**真的什么都没说**的那一轮，不是去评判它问得好不好。
func replyAsksForSomething(reply string) bool {
	if strings.ContainsAny(reply, "？?") {
		return true
	}
	for _, w := range replyAskWords {
		if strings.Contains(reply, w) {
			return true
		}
	}
	return false
}

var replyAskWords = []string{
	"请", "说说", "写下", "写一", "挑一", "选一", "找一", "找出", "标出", "圈出",
	"告诉我", "试试", "想一想", "读一读", "看一看", "接着读", "往下读", "点开", "点一下",
	// 🚨 「划」是这个房间最核心的动作（在文章里划出一句），第一版这张表里**没有它** ——
	// 于是「在文章里划出你最不服气的那一句」被判成「什么都没请她做」。
	// 守着这条的是 TestReadingPlusSomethingToDoIsFine。
	"划出", "划一", "划到", "标一",
}

// coachLabelBoardPrompt —— 标注板的标准题目。服务端兜底摆板时用它，模型把题目
// 写坏时也换回它。
//
// 🚨 措辞是产品负责人 2026-09-12 定的。他看到的那一张写着
// 「这三句各自在算账的哪一步？拖到角色各自里。」并指出两件事：数目对不上，
// 而且这不像一道题。他给的样子是「分析下列句子，观察他们分别属于哪一类论证模式。」
// —— 一句书面的分析题，**不写怎么拖**（怎么拖是界面的事，卡片下面那行字在说）。
const coachLabelBoardPrompt = "分析下列句子，判断它们各自属于哪一类论证成分。"

// labelPromptInventsBins —— 这道题目是不是另起了一套格子名。
//
// 格子是闭表（coachCardRoleLabels），由服务端填。模型有时在题目里自己编一套
// （「进不去 / 动不了 / 快撑不住了」），而屏幕上的格子仍然是那五个。
//
// 判据看**题目里被引号框起来、或者用斜杠并列起来的短词**：那是它在点名格子。
// 只要其中有一个不在闭表里，这套名字就是它自己编的。
func labelPromptInventsBins(prompt string) bool {
	for _, seg := range quotedSegments(prompt) {
		for _, part := range splitBinCandidates(seg) {
			if part != "" && !isRoleLabel(part) {
				return true
			}
		}
	}
	// 没加引号也能并列：「放进进不去/动不了/快撑不住了」。
	//
	// 🚨 闭表里五个名字**都是两个字**，所以判据就取斜杠两边各两个字：
	// 「分成主张/证据两类」两边是主张、证据，都在表里，是一句正常的话；
	// 「放进进不去/动不了/……」左边是「不去」，不在表里，那就是它自己编的。
	//
	// 先按整句切斜杠的那一版在这里栽过：那样切出来的是「分成主张」和
	// 「证据两类」，两个都不在表里，一句完全正常的话被判成编格子名。
	r := []rune(prompt)
	for i, c := range r {
		if c != '/' {
			continue
		}
		if i < 2 || i+2 >= len(r) {
			return true
		}
		if !isRoleLabel(string(r[i-2:i])) || !isRoleLabel(string(r[i+1:i+3])) {
			return true
		}
	}
	return false
}

func isRoleLabel(s string) bool {
	s = strings.TrimSpace(s)
	for _, l := range coachCardRoleLabels {
		if s == l {
			return true
		}
	}
	return false
}

// quotedSegments —— 「」『』 里面的东西。
func quotedSegments(s string) []string {
	var out []string
	for _, pair := range [][2]rune{{'「', '」'}, {'『', '』'}} {
		r := []rune(s)
		for i := 0; i < len(r); i++ {
			if r[i] != pair[0] {
				continue
			}
			for j := i + 1; j < len(r); j++ {
				if r[j] == pair[1] {
					out = append(out, string(r[i+1:j]))
					i = j
					break
				}
			}
		}
	}
	return out
}

// splitBinCandidates —— 按并列的分隔符拆开，得到一串候选格子名。
func splitBinCandidates(s string) []string {
	f := func(r rune) bool {
		return r == '/' || r == '、' || r == '｜' || r == '|'
	}
	parts := strings.FieldsFunc(s, f)
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// promptTellsHerHowToDrag —— 题目里在讲怎么操作。
//
// 怎么拖、怎么点是**界面**的事（卡片下面那行字一直在说），题目要留给那道题。
// 两边都写，出来的就是「这三句各自在算账的哪一步？拖到角色各自里。」这种句子。
func promptTellsHerHowToDrag(prompt string) bool {
	for _, w := range []string{"拖到", "拖进", "拖入", "拖下面", "拖过去", "点一句", "点格子", "放进格"} {
		if strings.Contains(prompt, w) {
			return true
		}
	}
	return false
}

// promptCountMismatch —— 题目里说了几句，和板上真有的对不上。
//
// 🚨 这是必然会发生的：模型先写题目再写选项，而选项要逐字核对原文，对不上的
// 会被刷掉 —— 于是「这三句」剩下两句。产品负责人 2026-09-12 报的就是这一张。
func promptCountMismatch(prompt string, n int) bool {
	re := regexp.MustCompile(`([0-9]|[一二三四五六七八九十])\s*(句|个词|个句子)`)
	for _, m := range re.FindAllStringSubmatch(prompt, -1) {
		if said := parseChineseOrdinal(m[1]); said > 0 && said != n {
			return true
		}
	}
	return false
}
