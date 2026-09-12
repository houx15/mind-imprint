package api

import (
	"errors"
	"strings"

	"mindimprint/api/internal/coachwalk"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// ReadingWalkDriver 驱动阅读室陪练那一条（postReadingCoachTurn）。
//
// 它住在这个包里，和 benchcases.go 同一个理由：buildReadingCoachSystem /
// buildReadingCoachPrompt / parseReadingCoachReply 都是未导出的。
// 请求路径上没有任何东西调用它。
//
// 🚨 这一条走查有一件别的陪练没有的可验判据：`advance`。
// 当前这一步要她「找出最关键的那个数字，说说它衡量的是什么」。她说对了数字
// （装机量第一），但**没有回答「它衡量的是什么」**——装机容量衡量的是发电能力，
// 不等于发电量，更不等于被替代掉的化石燃料。所以这一步还没做完，
// 而模型在她真的答上来之前把 advance 设成 done，就是**放她过去**。
// 那是这个调用点存在的理由本身，也是一条数得出来的判断错误，不需要判官。
type ReadingWalkDriver struct {
	blocks []Block
	lang   string
	tasks  []sqlc.ReadingTask
	msgs   []sqlc.AtomMessage
	picks  []readingPick
	// student 是这一轮交给 prompt 的「她刚说的话」。
	student string
	// answered 记她到底有没有回答过「它衡量的是什么」。走查开始时是 false，
	// 只有她自己说出那个区别才会翻成 true。
	answered bool
	seq      int32
}

func NewReadingWalkDriver() *ReadingWalkDriver {
	blocks := SplitBlocks(benchReadingArticle)
	return &ReadingWalkDriver{
		blocks: blocks,
		lang:   readingLangOf(benchReadingArticle),
		tasks: []sqlc.ReadingTask{
			{Position: 1, Kind: "read", Label: "通读全文，说说作者到底在主张什么", BlockID: "", Status: "done"},
			{Position: 2, Kind: "locate", Label: "找出文章里最关键的那个数字，说说它衡量的是什么", BlockID: "b3", Status: "active"},
			{Position: 3, Kind: "question", Label: "提一个这篇文章没有回答的问题", BlockID: "", Status: "todo"},
		},
		msgs: []sqlc.AtomMessage{
			{Seq: 1, Role: "assistant", Content: "先通读一遍。读完告诉我，作者到底想让你接受什么？"},
			{Seq: 2, Role: "user", Content: "他想说中国在可再生能源上投了很多钱。"},
			{Seq: 3, Role: "assistant", Content: "投了很多钱——这是他想让你接受的结论，还是他用来支持结论的证据？"},
			{Seq: 4, Role: "user", Content: "……是证据吧。结论应该是中国在让地球变得更可持续。"},
		},
		picks:   []readingPick{{BlockID: "b3", Quote: "中国的可再生能源新增装机量连续八年位居世界第一。"}},
		student: "我觉得这句最关键，装机量连续八年第一，说明投入是真的很大。",
		seq:     4,
	}
}

func (d *ReadingWalkDriver) Site() string {
	return "postReadingCoachTurn (POST /readings/{id}/coach)"
}

func (d *ReadingWalkDriver) Request() gateway.ChatRequest {
	return gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: buildReadingCoachSystem(d.lang)},
			{Role: gateway.RoleUser, Content: buildReadingCoachPrompt(
				"中国的能源转型：投入与结果", d.blocks, readingOutline{},
				d.tasks, d.msgs, d.picks, d.student, nil)},
		},
	}
}

func (d *ReadingWalkDriver) Parse(raw string) (string, []coachwalk.Violation, error) {
	parsed, ok := parseReadingCoachReply(raw, d.blocks, d.lang, func(string) bool { return true })
	if !ok {
		return "", nil, errors.New("reply unparseable — 生产这一轮回的是 502，学生看到「AI 暂时没接上」")
	}
	var extra []coachwalk.Violation
	if parsed.Advance != "" && !d.answered {
		extra = append(extra, coachwalk.Violation{
			Kind: "advanced-too-early",
			Note: "她还没说出「装机量衡量的是什么」，这一步就被设成 " + parsed.Advance + " —— 放她过去了",
		})
	}
	if parsed.Advance != "" {
		d.advanceTask(parsed.Advance)
	}
	return parsed.Reply, extra, nil
}

// advanceTask 把当前这一步标掉并把下一步点亮，和生产同一个语义
// （见 reading_coach.go 里 currentReadingTask 那一段）。
func (d *ReadingWalkDriver) advanceTask(status string) {
	for i := range d.tasks {
		if d.tasks[i].Status == "active" {
			d.tasks[i].Status = status
			if i+1 < len(d.tasks) {
				d.tasks[i+1].Status = "active"
			}
			return
		}
	}
}

func (d *ReadingWalkDriver) Advance(reply, said string) {
	d.seq++
	d.msgs = append(d.msgs, sqlc.AtomMessage{Seq: d.seq, Role: "assistant", Content: reply})
	d.seq++
	d.msgs = append(d.msgs, sqlc.AtomMessage{Seq: d.seq, Role: "user", Content: said})
	d.student = said
	if readingStepAnswered(said) {
		d.answered = true
	}
}

// readingStepAnswered 判断她这句话有没有真的回答「装机量衡量的是什么」。
//
// 判据是可验的：她得说出「装机/容量」和「发电量 / 实际发的电 / 替代化石燃料 /
// 减排」之间的**区别**。只说「投入很大」「第一」不算——那是她一开始就说过的。
// 这个判据宽一点没关系（漏判会让 advanced-too-early 少记一条，是保守方向），
// 但它绝不能把「只是重复了数字」算成答对了。
func readingStepAnswered(said string) bool {
	s := coachwalk.Fold(said)
	capacityWords := []string{"发电能力", "装机容量", "能发多少", "可以发", "能力"}
	outcomeWords := []string{"发电量", "实际发", "真发", "替代", "化石", "减排", "少烧", "不等于"}
	hasCap, hasOut := false, false
	for _, w := range capacityWords {
		if strings.Contains(s, coachwalk.Fold(w)) {
			hasCap = true
			break
		}
	}
	for _, w := range outcomeWords {
		if strings.Contains(s, coachwalk.Fold(w)) {
			hasOut = true
			break
		}
	}
	return hasCap && hasOut
}

func (d *ReadingWalkDriver) HerWords() string { return d.student }

func (d *ReadingWalkDriver) Screen(reply string) string {
	var cur string
	for _, t := range d.tasks {
		if t.Status == "active" {
			cur = t.Label
		}
	}
	s := "你正在阅读室读一篇文章《中国的能源转型：投入与结果》。\n"
	if cur != "" {
		s += "当前这一步：" + cur + "\n"
	}
	s += "\nAI 刚才对你说：\n" + reply
	return s
}

func (d *ReadingWalkDriver) Persona() string { return readingPersona }

const readingPersona = `你在扮演一个国际课程高中生，正在阅读室读一篇文章：
《中国的能源转型：投入与结果》。

文章里你注意到的那句：「中国的可再生能源新增装机量连续八年位居世界第一。」
你刚才说：这句最关键，装机量连续八年第一，说明投入是真的很大。

你就是一个真实的高中生：
- 说话口语、句子短，一次一两句，不分点罗列。
- 你**还没有**想到「装机容量」和「实际发出的电、实际替代掉的化石燃料」不是一回事。
  只有 AI 用问题把你逼到那儿，你才会自己说出这个区别；它要是直接讲给你听，
  你就只会说「哦，好像是」，不会自己说出来。
- 你会想让 AI 直接告诉你答案，就直接问「所以它到底衡量啥」「是不是就是发电量」。
- 问到你没想过的地方，你就说不知道，或者猜一个。

只输出你这一轮说的话本身，不要加引号，不要写「学生：」，不要解释你在做什么。`

// ReadingWalkJudge 评整条阅读室对话。
const ReadingWalkJudge = `下面是一个高中生和 AI 阅读陪练的完整多轮对话。
当前这一步要她「找出文章里最关键的那个数字，说说它衡量的是什么」。
她说对了数字（装机量连续八年第一），但一开始**没有**回答「它衡量的是什么」——
装机容量衡量的是发电能力，不等于实际发电量，更不等于被替代掉的化石燃料。

请只评价 AI（印记）这一侧，按 1–5 打分。看两件事：
（1）判断力——它有没有守住这一步？她还没说出那个区别就说「很好，我们看下一步」
    是错的；八轮之后她自己说出来了再推进才是对的。
（2）姿态——铁律①不替她定论（直接告诉她「装机量不等于发电量」也是错的）、
    铁律③一次只问一个。
1 = 放她过去，或直接把答案讲了；3 = 守住了但把答案讲了一半/问了不止一个；
5 = 守住这一步，并用问题让她自己说出那个区别。
只输出 JSON：{"score": <1-5>, "why": "<一句话，指出具体第几轮>"}`
