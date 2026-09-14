// Package coachwalk 是多轮陪练走查的公共部分。
//
// routebench 测的是**一轮**：给定一个上文，这个模型这一轮答得好不好。
// 「换了模型，陪练连起来还能用吗」不是一轮的性质——它问的是八轮之后印记还记不记得
// 她说过的话、有没有把同一个问题换个说法再问一遍、她跳过一步的时候有没有放她过去。
// 一个模型可以每一轮都单独好看，连起来读却是在原地绕。
//
// 这个包只放**和哪个陪练无关**的东西：循环、演学生的那一头、可验判据、延迟。
// 每个陪练自己的 prompt 和解析器都是未导出的，所以每个 Driver 住在它自己的包里
// （pbl / agent / api）——抄一份 prompt 出来就会漂移，而拿漂移的 prompt 测出来的
// 结论读起来像证据，其实一文不值。
//
// 请求路径上没有任何东西调用这个包。它和 cmd/routebench 是同一种东西：
// 一件与运行系统分开的工具。
package coachwalk

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"mindimprint/api/internal/gateway"
)

// CallResult 是一次模型调用交回来的东西。
type CallResult struct {
	Text string
	// Ms 是这一次调用从发出到收完的毫秒数。陪练的回复是非流式收齐再解析的，
	// 所以学生等的就是这个数，不是首字时间。
	Ms  int64
	Out int
}

// Call 打一次模型。延迟和 token 由调用方在闭包里量，这样这个包不需要认识
// provider 或 catalog。
type Call func(ctx context.Context, req gateway.ChatRequest) (CallResult, error)

// ErrRetry 由 Driver.Parse 包起来返回，意思是「生产在这种情况下会再问一次」。
//
// 🚨 走查必须照生产的样子重试，否则记下的失败学生根本看不见。阅读室在回复读不动
// 时会再调一次模型（reading_coach.go 里 "retrying once" 那一段），只有两次都
// 读不动才回 502。第一版走查只调一次，于是把一次「生产会自己救回来」的失败
// 记成了拦住上线的理由。
var ErrRetry = errors.New("coachwalk: 生产会重试这一轮")

// Driver 是某一个陪练在这条循环里的那一侧。
//
// 实现它的人只做三件事：按当前状态搭出真实请求、用真实解析器读回复、
// 把这一轮接进自己的状态。判据不由它来写。
type Driver interface {
	// Site 是调用点，报告里按它分组。
	Site() string
	// Request 按当前状态搭出这一轮的真实请求。
	Request() gateway.ChatRequest
	// Parse 用**生产的**解析器读模型原文。返回学生会看到的那句话，
	// 以及这一轮该记下的额外犯规（比如生产校验器拒收、过早放她过关）。
	// err 不为 nil 表示这一轮学生拿不到回复；err 包着 ErrRetry 时，
	// Run 会像生产那样再调一次模型。
	Parse(raw string) (reply string, extra []Violation, err error)
	// Advance 把这一轮接进状态，下一轮 Request() 就该反映它。
	Advance(reply, studentSaid string)
	// HerWords 是这一轮「接不住她」该比对的语料：她自己说过或写下的、这一轮
	// 本该被接住的内容。
	//
	// 🚨 不是所有陪练都只该接住「她上一句聊天」。pro 陪练锚在她写在过程图上的
	// 那句主张上，它的活儿就是追问那句主张——它逐字引用了她的主张、却一个字
	// 都没引她的上一句聊天，是**正确**的行为。第一版这里只给上一句聊天，于是
	// pro 那条走查 16 轮里被记了 12 条假的「接不住她」。
	HerWords() string
	// Persona 是演学生的那个模型拿到的人设。
	Persona() string
	// Screen 是这一轮她在屏幕上看到的东西（回复，以及钩子/工具等）。
	Screen(reply string) string
}

// Turn 是一轮来回，以及这一轮当场验出来的东西。
type Turn struct {
	N           int
	CoachRaw    string
	Reply       string
	ParseErr    string
	StudentSaid string

	// CoachMs 是这一轮学生从发出到拿到回复等了多久：**包括重试**。
	// 重试她看不见，但她等得到。
	CoachMs int64
	// Retries 是这一轮生产会多调的次数。
	Retries int
	// OutTokens 是这一轮陪练所有调用的输出 token 合计。
	OutTokens int

	QuestionsInReply int
	RepeatOf         int
	EchoesHer        bool
	// Extra 是这个陪练自己的判据记下的（生产校验器拒收、过早放她过关……）。
	Extra []Violation
}

// Log 是一整条走查。
type Log struct {
	Site  string
	Turns []Turn
}

// Violation 是一条数得出来的犯规。
type Violation struct {
	Turn int
	Kind string
	Note string
}

// Violations 把整条走查里数得出来的问题列出来。
//
// 这里**只放不需要判断的东西**。「这个问题问得好不好」不在里面，那个确实要人或
// 判官去看；「一轮问了三个问题」「第六轮把第三轮的话又说了一遍」「她还没答上来就
// 被放过去了」不需要。
func (l *Log) Violations() []Violation {
	var vs []Violation
	for _, t := range l.Turns {
		if t.ParseErr != "" {
			// 这一轮没有 reply，别再叠上「接不住她」——一次故障不该在总表里
			// 看起来像三处缺陷。
			vs = append(vs, Violation{t.N, "parse", t.ParseErr})
			continue
		}
		if t.QuestionsInReply >= 2 {
			vs = append(vs, Violation{t.N, "multi-question",
				fmt.Sprintf("reply 里有 %d 个问号，铁律③ 是一次只问一个", t.QuestionsInReply)})
		}
		if t.RepeatOf != 0 {
			vs = append(vs, Violation{t.N, "repeat",
				fmt.Sprintf("和第 %d 轮高度重合", t.RepeatOf)})
		}
		if !t.EchoesHer {
			vs = append(vs, Violation{t.N, "ungrounded",
				"reply 里找不到她说过/写下的任何一段原文"})
		}
		vs = append(vs, t.Extra...)
	}
	return vs
}

// Count 数某一类犯规有几条。
func (l *Log) Count(kind string) int {
	n := 0
	for _, v := range l.Violations() {
		if v.Kind == kind {
			n++
		}
	}
	return n
}

// Transcript 把整条对话渲染成人和判官都读得懂的样子。
func (l *Log) Transcript() string {
	var b strings.Builder
	for _, t := range l.Turns {
		fmt.Fprintf(&b, "── 第 %d 轮 ──\n", t.N)
		if t.ParseErr != "" {
			fmt.Fprintf(&b, "印记（解析失败：%s）：%s\n\n", t.ParseErr, TruncateRunes(t.CoachRaw, 300))
			continue
		}
		fmt.Fprintf(&b, "印记：%s\n", t.Reply)
		for _, e := range t.Extra {
			fmt.Fprintf(&b, "　　🚨 %s：%s\n", e.Kind, e.Note)
		}
		fmt.Fprintf(&b, "学生：%s\n\n", t.StudentSaid)
	}
	return b.String()
}

// Run 跑 turns 轮，用的是 Driver 交出来的真实 prompt 和真实解析器。
func Run(ctx context.Context, d Driver, coach, student Call, turns int) (*Log, error) {
	log := &Log{Site: d.Site()}

	for n := 1; n <= turns; n++ {
		t := Turn{N: n}
		req := d.Request()
		res, err := coach(ctx, req)
		if err != nil {
			return log, fmt.Errorf("%s 第 %d 轮陪练调用失败: %w", d.Site(), n, err)
		}
		t.CoachRaw, t.CoachMs, t.OutTokens = res.Text, res.Ms, res.Out
		// 先记下这一轮该比对的语料——Advance 之后它就变了。
		hers := d.HerWords()

		reply, extra, perr := d.Parse(res.Text)
		if perr != nil && errors.Is(perr, ErrRetry) {
			// 照生产的样子再问一次，同一个请求。
			t.Retries++
			again, aerr := coach(ctx, req)
			if aerr != nil {
				perr = fmt.Errorf("重试调用失败，生产回 502: %v", aerr)
			} else {
				t.CoachRaw = again.Text
				t.CoachMs += again.Ms
				t.OutTokens += again.Out
				reply, extra, perr = d.Parse(again.Text)
				if perr != nil {
					perr = fmt.Errorf("重试后仍读不动，生产回 502: %v", perr)
				}
			}
		}
		if perr != nil {
			// 学生这一轮拿不到回复。记下来就停：后面几轮拿不到 reply，
			// 续下去只是在编一条不存在的对话。
			t.ParseErr = perr.Error()
			log.Turns = append(log.Turns, t)
			return log, nil
		}
		t.Reply = reply
		t.QuestionsInReply = CountQuestions(reply)
		t.EchoesHer = EchoesLast(reply, hers)
		t.RepeatOf = repeatOf(reply, log.Turns)
		for _, e := range extra {
			e.Turn = n
			t.Extra = append(t.Extra, e)
		}

		said, serr := runStudent(ctx, student, d.Persona(), d.Screen(reply))
		if serr != nil {
			return log, fmt.Errorf("%s 第 %d 轮学生调用失败: %w", d.Site(), n, serr)
		}
		t.StudentSaid = strings.TrimSpace(said)
		log.Turns = append(log.Turns, t)

		d.Advance(reply, t.StudentSaid)
	}
	return log, nil
}

// runStudent 让演学生的模型看一眼屏幕，然后说一句话。
//
// 🚨 她只拿得到学生看得见的东西：印记说的那句话、挂出来的钩子、递过来的工具名。
// 不给她 JSON、不给她 system prompt、不告诉她这个任务该走几步。她看不懂就说
// 看不懂，那句「看不懂」就是我们要的结果——和 e2e/camp 的 brain.ts 同一个道理。
func runStudent(ctx context.Context, call Call, persona, screen string) (string, error) {
	res, err := call(ctx, gateway.ChatRequest{
		MaxTokens: 800,
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: persona},
			{Role: gateway.RoleUser, Content: screen + "\n\n你这一轮说什么？"},
		},
	})
	return res.Text, err
}

/* ── 延迟 ────────────────────────────────────────────────────────────────── */

// Latency 是一组走查里每一轮陪练延迟的分布（毫秒）。
type Latency struct {
	N             int
	P50, P90, Max int64
	Retries       int
}

// LatencyOf 汇总若干条走查的每轮延迟。解析失败的那一轮也算进去：
// 她同样等了那么久，只是等来一个错。
func LatencyOf(logs []*Log) Latency {
	var ms []int64
	var lat Latency
	for _, l := range logs {
		if l == nil {
			continue
		}
		for _, t := range l.Turns {
			ms = append(ms, t.CoachMs)
			lat.Retries += t.Retries
		}
	}
	lat.N = len(ms)
	if lat.N == 0 {
		return lat
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i] < ms[j] })
	lat.P50 = percentile(ms, 50)
	lat.P90 = percentile(ms, 90)
	lat.Max = ms[len(ms)-1]
	return lat
}

// percentile 取最近秩：不插值，报出来的每个数都是某一轮真实等过的时间。
func percentile(sorted []int64, p int) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := (p*len(sorted) + 99) / 100 // ceil(p% × n)
	if idx < 1 {
		idx = 1
	}
	if idx > len(sorted) {
		idx = len(sorted)
	}
	return sorted[idx-1]
}

/* ── 可验判据 ────────────────────────────────────────────────────────────── */

// CountQuestions 数问号，中英文都算。
func CountQuestions(s string) int {
	return strings.Count(s, "？") + strings.Count(s, "?")
}

// EchoesLast 检查 reply 里有没有 hers 的任何一段原文。hers 由 Driver 决定
// （见 Driver.HerWords）——对不同的陪练，「该被接住的东西」不是同一样东西。
//
// 判据故意宽：连续四个字重合就算接住了。要抓的不是「没有逐字引用」，是
// 「八轮下来一次都接不上她说的话」那种自说自话。四个字以下不判——
// 「好的」「嗯」这种太短的话，任何回复都会偶然命中。
func EchoesLast(reply, hers string) bool {
	h := []rune(Fold(hers))
	flat := Fold(reply)
	const win = 4
	if len(h) < win {
		return true
	}
	for i := 0; i+win <= len(h); i++ {
		if strings.Contains(flat, string(h[i:i+win])) {
			return true
		}
	}
	return false
}

// repeatThreshold 是量出来的，不是定出来的。同一个问题换句话说落在 0.58，
// 一个真正的新问题落在 0.00——中间这段空得很宽，所以取 0.45：比真重复低一截，
// 比任何一个新问题高得多。第一版取 0.6，正好卡在真重复上面一点，一次都抓不到。
const repeatThreshold = 0.45

// repeatOf 找出这一句和前面哪一轮高度重合，没有就返回 0。
//
// 用二元组的 Jaccard，不是字符串相等：印记重复自己的时候会换个说法，
// 而换了说法的同一个问题，对学生来说仍然是同一个问题又被问了一遍。
func repeatOf(reply string, prev []Turn) int {
	cur := bigrams(reply)
	if len(cur) == 0 {
		return 0
	}
	for _, p := range prev {
		if p.ParseErr != "" {
			continue
		}
		if jaccard(cur, bigrams(p.Reply)) >= repeatThreshold {
			return p.N
		}
	}
	return 0
}

func bigrams(s string) map[string]struct{} {
	r := []rune(Fold(s))
	out := make(map[string]struct{}, len(r))
	for i := 0; i+2 <= len(r); i++ {
		out[string(r[i:i+2])] = struct{}{}
	}
	return out
}

func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for k := range a {
		if _, ok := b[k]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// Fold 只留下字、数字和字母，空白和标点全部丢掉。
//
// 🚨 丢标点不是顺手做的。第一版只折空白，于是这一轮被判成「接不住她」：
//
//	她说：…他们是因为打多了还是不好吃才剩的？
//	印记：先别急着分「打多了」还是「不好吃」——…
//
// 印记接住的是她的原话，一字不差，可它加了一对引号，四字窗口「打多了还」
// 就再也对不上了。**判据错了就会把一个走通了的陪练记成走不通的**，
// 而那条假缺陷读起来和真的一模一样。
func Fold(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// TruncateRunes 按字截断，不按字节——按字节切会把一个汉字切成两半。
func TruncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
