package news

import "mindimprint/api/internal/prompts"

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// write.go —— 一颗星被选中之后，照着**原文**把它写出来。
//
// # 为什么这是单独的一步（2026-09-10）
//
// 上一版一次调用干两件事：从四十条候选里挑五条，并且把五条都写成中文。写的
// 时候它手上只有 feed 给的那 400 字导语，而导语常常是半句话 —— Quanta 那条
// 的导语原样结束在
//
//	…thus resolving one of the six remaining Millennium Prize Problems… Source
//
// 「争议在哪」一个字都没有。于是模型把它补全了：它写「同行质疑『解决』的定义
// —— 它找到了一个特例，还是真的证明了结论？」。这篇报道通篇没有讨论过特例。
// 学生看到的那个问题，问的是一件没发生过的事。
//
// 所以拆成两步：**挑**只看标题和导语（够了，挑本来就只需要判断值不值得读），
// **写**先去把正文抓回来，再照着正文写。实测 18 条候选里 13 条抓得到正文
// （Quanta 那条 24011 字），抓不到的退回 feed 正文或导语。
//
// # 「不许编」怎么才算数
//
// 写在 prompt 里的规矩只有能验的那部分算数（memory: prompt-output-must-be-
// verifiable）。所以模型必须连**它是从哪一句话读出这个问题的**一起交出来：
// `evidence` 是正文里的一句原话，逐字照抄。服务端拿它回原文里查，查不到就打
// 回去重写一次，再查不到就丢掉这颗星。
//
// 这条约束是这个文件存在的理由。少了它，上面那两段话就只是一段更客气的许愿。
//
// # 标题不再重写
//
// 旧 prompt 写着「titleZh：**重写**，不是翻译」，配着一个「好 / 差」的例子。
// 产出是「AI白天使11天？数学难题争议更大」——原标题（AI Has Solved One of
// Math's $1 Million Millennium Prize Problems）里的每一个信息都没了，而「11天」
// 是模型从 prompt 自己的示例钩子（「AI 十一天做完了数学家几年的活」）里抄来的。
// 一个学生读完这行字，不知道这条新闻在说什么。
//
// 现在 titleZh 是原标题的**如实翻译**，英文原标题并排显示。她看到的标题和她
// 点进去看到的标题是同一个。

// WriteMargin 是选星时多挑几条。
//
// 写这一步会丢东西：正文抓不到、evidence 对不上、数字对不上原文。多挑两条，
// 丢掉一两条之后仍然凑得满五颗。多挑的那几条不额外花抓取的钱（抓取是按选中的
// 条数走的），只多两次写作调用。
const WriteMargin = 2

// SelectCount 是选星那一步挑几条。
func SelectCount() int { return PlanetCount + WriteMargin }

// groundMinRunes 是「抓回来的这份算不算正文」。
//
// 600 字。Nature 那几个源实测返回 `OK 0 chars` —— HTTP 200，抽取器一个段落都
// 找不到（付费墙 / 正文在 JS 里）。一份 80 字的导航残渣当正文用，比退回 feed
// 摘要糟：它会让 evidence 校验永远通不过，而失败的原因看上去像是模型的错。
const groundMinRunes = 600

// evidenceMinRunes / evidenceMaxRunes 是那句原话的长度区间。
//
// 下限 24：短于这个的「原话」在任何一篇文章里都能碰巧撞上（"the results"），
// 校验就变成了走过场。
//
// 上限 300：它是一句话的凭据，不是一段摘录。240 试过一轮，实测被一句 254 字的
// 真原话打回（Colossal 那篇讲编织带的，一句话本来就长）—— 一条通过了「真的在
// 原文里」的原话，因为长了 14 个字被丢掉，丢的是一颗好星。
const (
	evidenceMinRunes = 24
	evidenceMaxRunes = 300
)

// hookMaxRunes 与旧 prompt 同一个数，同一个理由：写得长不会更有力。
const hookMaxRunes = 45

// Written 是「写」这一步的产物。
type Written struct {
	TitleZh string
	Summary string
	Hook    string
	// Evidence 是正文里的一句原话，Hook 从它读出来。它**已经被验过**确实出现
	// 在原文里 —— 一个 Written 存在，就意味着这一条通过了那道校验。
	Evidence string
}

// GroundText 交出这颗星的「原文」——校验 evidence 与数字时唯一算数的那份文本。
//
// 三条路，按可信度排：抓回来的正文 → feed 自带的正文 → feed 的导语。前两条
// 是完整的文章，第三条常常是半句话，所以只在前两条都空的时候才用。
//
// 🚨 抓回来的东西要够长才算正文，见 groundMinRunes。
func GroundText(fetched string, it Item) string {
	if f := strings.TrimSpace(fetched); len([]rune(f)) >= groundMinRunes {
		return f
	}
	if b := strings.TrimSpace(it.Body); b != "" {
		return b
	}
	return strings.TrimSpace(it.Summary)
}

const writeSystemPrompt = prompts.NewsWriteSystemPrompt

// promptGroundRunes 是送进 prompt 的正文上限。
//
// 6000 字：Quanta 一篇长报道实测 24011 字，全塞进去一天五条要多花四倍的钱，
// 而一篇报道的争议在哪，前 6000 字里基本都交代完了。
const promptGroundRunes = 6000

// Turn 是写作那个来回里的一轮。
//
// 用它而不是直接用 gateway 的类型：这个包不认识通道，也不该认识 —— 它只知道
// 「我给出一段话，拿回一段话」。调用方负责把 Turn 翻成它那边的消息。
type Turn struct {
	Role    string // "assistant" | "user"
	Content string
}

// 角色名，和 OpenAI 那套一致，调用方直接用得上。
const (
	TurnAssistant = "assistant"
	TurnUser      = "user"
)

// writeAttempts 是一条最多写几次。
//
// 两次。第一次写，验不过就**把不过的理由原样贴回去**再写一次。只说「格式错了」
// 模型多半原样再写一遍；说清楚「evidence 那句话在正文里找不到」它才知道要换
// 哪一句。第二次还不过就丢掉这一条 —— 一颗写歪的星比四颗星糟得多，而选星那一步
// 为此多挑了 WriteMargin 条。
const writeAttempts = 2

// WriteOne 让模型照着正文写一条，并把写出来的东西验一遍；验不过就带着理由重写。
//
// ask 发一轮请求：system 加上到目前为止的来回，返回模型说的话。它由调用方提供，
// 所以这个循环在服务端和 LIVE_LLM 用例里是**同一段代码** —— 一个只在生产里跑的
// 重试循环，等于没被测过。
func WriteOne(it Item, ground string, ask func(system string, turns []Turn) (string, error)) (Written, error) {
	system, user := BuildWritePrompt(it, ground)
	turns := []Turn{{Role: TurnUser, Content: user}}
	var lastErr error
	for i := 0; i < writeAttempts; i++ {
		raw, err := ask(system, turns)
		if err != nil {
			return Written{}, err
		}
		w, perr := ParseWriteReply(raw, it, ground)
		if perr == nil {
			return w, nil
		}
		lastErr = perr
		turns = append(turns,
			Turn{Role: TurnAssistant, Content: raw},
			Turn{Role: TurnUser, Content: "这一版不能用：" + perr.Error() + "。按同一个 JSON 格式重写一遍。"},
		)
	}
	return Written{}, lastErr
}

// BuildWritePrompt 拼出写一颗星用的两段。
func BuildWritePrompt(it Item, ground string) (system, user string) {
	var b strings.Builder
	fmt.Fprintf(&b, "原标题（%s）：%s\n\n", it.Source, it.Title)
	b.WriteString("正文：\n")
	b.WriteString(truncRunes(ground, promptGroundRunes))
	b.WriteString("\n")
	return writeSystemPrompt, b.String()
}

type writeReply struct {
	TitleZh  string `json:"titleZh"`
	Summary  string `json:"summary"`
	Hook     string `json:"hook"`
	Evidence string `json:"evidence"`
}

// ParseWriteReply 读「写」的回话，并把它验一遍。
//
// 返回的 error 是**给模型看的**：调用方会把它原样贴回去让它重写一次（那次重写
// 只有说清楚哪里错了才有意义 —— 只说「错了」它多半原样再写一遍）。所以每一句
// 都写成「哪里不对 + 该怎么改」。
func ParseWriteReply(raw string, it Item, ground string) (Written, error) {
	body, err := sliceJSONObject(raw)
	if err != nil {
		return Written{}, fmt.Errorf("你的回复里没有一个能读的 JSON 对象。只输出 {\"titleZh\":\"\",\"summary\":\"\",\"hook\":\"\",\"evidence\":\"\"}，不要任何别的字")
	}
	var rep writeReply
	if uerr := json.Unmarshal(body, &rep); uerr != nil {
		return Written{}, fmt.Errorf("你的 JSON 读不出来（%v）。只输出那四个字段", uerr)
	}

	w := Written{
		TitleZh:  strings.TrimSpace(rep.TitleZh),
		Summary:  strings.TrimSpace(rep.Summary),
		Hook:     strings.TrimSpace(rep.Hook),
		Evidence: strings.TrimSpace(rep.Evidence),
	}
	if w.TitleZh == "" {
		return Written{}, fmt.Errorf("titleZh 是空的。把原标题如实译成中文")
	}
	if !isQuestion(w.Hook) {
		return Written{}, fmt.Errorf("hook 必须是一个问题、以问号结尾，你写的是「%s」", clip(w.Hook, 40))
	}
	if n := len([]rune(w.Hook)); n > hookMaxRunes {
		return Written{}, fmt.Errorf("hook 有 %d 字，超过 %d 字了。删到一句话", n, hookMaxRunes)
	}
	if err := checkTitle(w.TitleZh, it.Title); err != nil {
		return Written{}, err
	}
	// 🚨 evidence 那道校验，是这整个文件唯一挡得住「编一个争议」的东西。
	if err := checkEvidence(w.Evidence, ground); err != nil {
		return Written{}, err
	}
	if IsPolitical(w.TitleZh, w.Summary+" "+w.Hook) {
		return Written{}, fmt.Errorf("这条落在政治／战争／灾难上了，换一个角度写")
	}
	return w, nil
}

/* ── 标题校验 ───────────────────────────────────────────────────────────── */

// titleMaxRunes：长于这个字数的不是标题，是把导语也译进来了。
const titleMaxRunes = 48

// titleCompression 是「一个中文字大约顶几个英文字符」。
//
// 5。英译中实测大致在 2.5 到 4 之间，取 5 是留够余量的下限 —— 这条校验要挡的
// 是「标题被换成了一个话题标签」（63 个字符的原标题译成了「AI 与数学」四个字），
// 不是要审计译得好不好。宽一点，误伤一颗好星比放过一个标签贵。
const titleCompression = 5

// titleMinRunesFor 是这条原标题的译文至少该有多少字。
//
// 跟着原标题的长度走，不是一个固定的数：Aeon 的 "The Chinese room" 译成
// 「中国屋」三个字是对的，而同样三个字放在一条 63 字符的原标题下面就是个标签。
func titleMinRunesFor(en string) int {
	if n := len([]rune(en)) / titleCompression; n > 3 {
		return n
	}
	return 3
}

// checkTitle 验中文标题确实是那个英文标题的翻译，而不是一次改写。
//
// 两条都验得了、也都是实测撞出来的：
//
//  1. **原标题不是问句，译文里不许有问号。** 观察到的那条是
//     「AI白天使11天？数学难题争议更大」——原标题
//     "AI Has Solved One of Math's $1 Million Millennium Prize Problems"
//     里没有任何疑问。一个问号把一句陈述变成了这条新闻没说过的事，而且它和
//     下面「它想问你」那一栏抢同一个位置。
//     原标题本来就是问句时（Quanta 常这么写）当然可以有，所以判据是**跟着
//     原标题走**，不是一刀切。
//
//  2. **凭空出现的数字。** 「11 天」既不在原标题里、也不在正文里 —— 它是模型
//     从旧 prompt 自己的示例钩子（「AI 十一天做完了数学家几年的活」）里抄来的。
//     示例已经删了，这条是那类错误的兜底。
func checkTitle(zh, en string) error {
	if n, min := len([]rune(zh)), titleMinRunesFor(en); n < min {
		return fmt.Errorf("titleZh 只有 %d 字，这不是那个标题的翻译（至少 %d 字）。把原标题整句译出来，别缩成一个话题标签", n, min)
	} else if n > titleMaxRunes {
		return fmt.Errorf("titleZh 有 %d 字，超过 %d 字了。只译标题，不要把导语也译进去", n, titleMaxRunes)
	}
	if isQuestion(zh) && !isQuestion(en) {
		return fmt.Errorf("原标题不是一个问句，你的译文里却有问号。如实译，不要把标题改写成提问")
	}
	if bad, ok := inventedNumber(zh, en); ok {
		return fmt.Errorf("你在标题里写了「%s」这个数，原标题里没有它。如实译，不要添数字", bad)
	}
	return nil
}

/* ── evidence 校验 ──────────────────────────────────────────────────────── */

// checkEvidence 验那句原话真的在原文里。
//
// 比对前两边都抹掉标点、空白与大小写（见 squash）：模型把弯引号写成直引号、把
// 长破折号写成短的、把换行吞掉，都不该算作「编的」。抹掉之后仍然要求**逐字、
// 按原顺序**出现 —— 放宽的只是排版，不是内容。
func checkEvidence(ev, ground string) error {
	if ev == "" {
		return fmt.Errorf("evidence 是空的。从正文里原样抄一句话出来，你的 hook 就是从那句话读出来的")
	}
	if n := len([]rune(ev)); n < evidenceMinRunes {
		return fmt.Errorf("evidence 只有 %d 字，太短了（至少 %d 字）。抄一整句", n, evidenceMinRunes)
	} else if n > evidenceMaxRunes {
		return fmt.Errorf("evidence 有 %d 字，太长了（至多 %d 字）。抄一句，不是一段", n, evidenceMaxRunes)
	}
	if !strings.Contains(squash(ground), squash(ev)) {
		return fmt.Errorf("evidence 那句话在正文里找不到：「%s」。它必须是正文里的原话，逐字照抄，不能改写、不能拼接两句", clip(ev, 60))
	}
	return nil
}

// squash 把一段文本压成只剩字母、数字与汉字，全部小写。
//
// 用来比对「是不是同一句话」：标点、空白、全角半角、弯直引号的差别都不算数，
// 词与顺序算数。
func squash(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

/* ── 数字校验 ───────────────────────────────────────────────────────────── */

// inventedNumber 找出 out 里那个 src 中没有的数。返回它和 true。
//
// 这条校验存心**宽**：它只需要挡住「11 天」那一类，不需要审计每一个数字。误报
// 一次的代价是丢掉一颗好星，而它在一天只有五颗星的屏上很贵。
//
// 三条豁免：
//
//   - 只看**两位及以上**的数字串。原文写 "three dimensions"、译文写「3 维」是
//     对的翻译，按位数一刀切会把它判成编造。「11 天」不是任何英文词的译法。
//   - 后面跟着中文数量单位（万 / 亿 / 千 / 百 / 十）的不看。原文 "$1 Million"
//     的如实译法是「100 万美元」，而 100 这个数在原文里确实不存在 —— 这是中文
//     写数的方式，不是编造。
//   - 比对前去掉千分位逗号：原文的 "10,000" 和译文的 "10000" 是同一个数。
func inventedNumber(out, src string) (string, bool) {
	have := digitRuns(src)
	for n := range digitRuns(out) {
		if len(n) < 2 || have[n] {
			continue
		}
		return n, true
	}
	return "", false
}

// myriadUnits 是中文里跟在数字后面表示量级的字。见 inventedNumber 的第二条豁免。
const myriadUnits = "万亿千百十"

// followedByMyriad —— 从 i 往后跳过空白，看下一个字是不是量级字。
//
// 🚨 要跳空白。中文里数字和单位之间常常有一个空格（「一道 100 万美元的难题」），
// 而不跳空白的版本会把这个「100」判成编造出来的数 —— 那正是我们最不该丢的那
// 一类标题：一条如实译过来的。
func followedByMyriad(rs []rune, i int) bool {
	for ; i < len(rs); i++ {
		if rs[i] == ' ' || rs[i] == '\t' || rs[i] == '\u00a0' {
			continue
		}
		return strings.ContainsRune(myriadUnits, rs[i])
	}
	return false
}

// digitRuns 收集一段文本里所有的数字串。
//
// 千分位逗号当作数字的一部分吃掉；后面紧跟量级字的那一串直接丢弃，不进结果。
func digitRuns(s string) map[string]bool {
	out := map[string]bool{}
	var cur strings.Builder
	flush := func(keep bool) {
		if cur.Len() > 0 && keep {
			out[cur.String()] = true
		}
		cur.Reset()
	}
	rs := []rune(s)
	for i, r := range rs {
		switch {
		case r >= '0' && r <= '9':
			cur.WriteRune(r)
		case (r == ',' || r == '，') && cur.Len() > 0 && i+1 < len(rs) && rs[i+1] >= '0' && rs[i+1] <= '9':
			// 千分位：10,000 → 10000。
		default:
			flush(!followedByMyriad(rs, i))
		}
	}
	flush(true)
	return out
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
