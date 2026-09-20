package api

import (
	"fmt"

	"mindimprint/api/internal/coachwalk"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/library"
	"mindimprint/api/internal/store/sqlc"
)

// coachwalk_longread.go —— 拿**分级阅读库里一篇真文章**走查阅读陪练。
//
// 为什么要这一条：原来那条走查的文章只有 286 字，而线上一次阅读陪练平均带
// 11,035 个输入 token —— 也就是说，那条走查量的是一种线上不存在的尺寸。
// 凡是跟「文章有多大」有关的结论（收窄省了多少、长文里陪练会不会跟丢当前这一
// 部分、缓存前缀有多长），在那条走查上都测不出来。这和
// [[fixture-told-coach-session-over-2026-09-14]] 是同一类毛病：用例的形状不对，
// 量出来的东西就不是产品的。
//
// 文章直接从 `internal/library` 取（go:embed 的那 48 篇，线上读的就是它），
// 不在这里抄一份 —— 抄一份的话，库里的文章改了这里会悄悄过时。
//
// # 这一条怎么用：同一篇文章跑两遍
//
// 两个 scenario 只差一件事：**导读里有没有 parts**。
//
//	long-article/full    没有 parts ⇒ readingDisclosureScope 返回 nil ⇒ 整篇都给
//	long-article/scoped  有 parts   ⇒ 只展开当前这一步那个部分
//
// 其余一切（文章、步骤、对话脚本、判官）完全相同，所以两边的差别只可能来自
// 收窄本身。这是「先 mock e2e 再上线」那一关缺的那件工具。
const longReadSlug = "pro-con-data-centers"

// longReadTier 取第 3 档：够长（8,151 字符、33 段）能触发收窄，又不是最长的
// 那一档，跑一条走查不至于贵得离谱。
const longReadTier = 3

// longReadArticle 返回库里那篇文章的正文与标题。
func longReadArticle() (title, body string, err error) {
	a, ok := library.BySlug(longReadSlug)
	if !ok {
		return "", "", fmt.Errorf("coachwalk: 阅读库里没有 %q", longReadSlug)
	}
	lv, ok := a.LevelAt(longReadTier)
	if !ok {
		return "", "", fmt.Errorf("coachwalk: %q 没有第 %d 档", longReadSlug, longReadTier)
	}
	return lv.Title, lv.Body, nil
}

// LongReadWalkDriver 和 ReadingWalkDriver 是同一套机器，只是文章、步骤和导读
// 由构造函数给定 —— 这样「给不给 parts」可以成为两个 scenario 的唯一差别。
type LongReadWalkDriver struct {
	title   string
	blocks  []Block
	lang    string
	outline readingOutline
	tasks   []sqlc.ReadingTask
	msgs    []sqlc.AtomMessage
	student string
	seq     int32
}

// newLongReadWalkDriver 造一条长文走查。scoped=true 时导读带 parts 和承重标注，
// 收窄因此生效；false 时导读是空的，整篇都给。
func newLongReadWalkDriver(scoped bool) *LongReadWalkDriver {
	title, body, err := longReadArticle()
	if err != nil {
		// 库坏了就让走查在第一轮炸掉，而不是悄悄走一篇空文章。
		panic(err)
	}
	blocks := SplitBlocks(body)
	d := &LongReadWalkDriver{
		title:  title,
		blocks: blocks,
		lang:   readingLangOf(body),
	}
	if scoped {
		d.outline = longReadOutline(blocks)
	}
	d.tasks = longReadTasks(blocks, scoped)
	return d
}

// longReadOutline 把文章切成三个部分，并把每个部分的头一段标成承重段 ——
// 这正是 reading_plan 在线上排出来的形状（readingPartSteps 取 p.From 当步骤的
// blockID，loadLabels 把 core 标出来）。
func longReadOutline(blocks []Block) readingOutline {
	n := len(blocks)
	if n < 6 {
		return readingOutline{}
	}
	a, b := n/3, 2*n/3
	parts := []readingPart{
		{From: blocks[0].ID, To: blocks[a-1].ID, Title: "正方的主张", Does: "提出数据中心的好处"},
		{From: blocks[a].ID, To: blocks[b-1].ID, Title: "反方的主张", Does: "提出代价与风险"},
		{From: blocks[b].ID, To: blocks[n-1].ID, Title: "两边的分歧在哪", Does: "把争点收拢"},
	}
	load := map[string]string{
		blocks[0].ID: loadCore,
		blocks[a].ID: loadCore,
		blocks[b].ID: loadCore,
	}
	return readingOutline{
		OneLine: "数据中心值不值得建？",
		Shape:   "正反两方各说一轮，最后收拢争点。",
		Parts:   parts,
		Load:    load,
	}
}

// longReadTasks 排出线上那种「通读一步一个部分」的清单，当前停在第二个部分。
func longReadTasks(blocks []Block, scoped bool) []sqlc.ReadingTask {
	n := len(blocks)
	a, b := n/3, 2*n/3
	first, second, third := blocks[0].ID, blocks[a].ID, blocks[b].ID
	if !scoped {
		// 不收窄那一支不给 parts，步骤上也就没有段落归属 —— 但步骤本身要一样，
		// 否则两边比的就不是同一件事了。BlockID 留着，它只在有 parts 时才被用到。
		first, second, third = blocks[0].ID, blocks[a].ID, blocks[b].ID
	}
	return []sqlc.ReadingTask{
		{ID: fixtureTaskID(1), Position: 1, Kind: string(taskRead),
			Label: "通读正方那几段，说说他们最强的理由是什么", BlockID: first, Status: "done"},
		{ID: fixtureTaskID(2), Position: 2, Kind: string(taskRead),
			Label: "通读反方那几段，说说他们最担心的是什么", BlockID: second, Status: "pending"},
		{ID: fixtureTaskID(3), Position: 3, Kind: string(taskCritique),
			Label: "你怎么看", BlockID: third, Status: "pending"},
	}
}

func (d *LongReadWalkDriver) Site() string {
	return "postReadingCoachTurn (POST /readings/{id}/coach) · 长文"
}

func (d *LongReadWalkDriver) Request() gateway.ChatRequest {
	return gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: buildReadingCoachSystem(d.lang)},
			{Role: gateway.RoleUser, Content: buildReadingCoachPrompt(
				d.title, d.blocks, d.outline, d.tasks, d.msgs, nil, d.student, nil, "")},
		},
	}
}

func (d *LongReadWalkDriver) Parse(raw string) (string, []coachwalk.Violation, error) {
	parsed, ok := parseReadingCoachReply(raw, d.blocks, d.lang, func(string) bool { return true })
	if !ok {
		return "", nil, fmt.Errorf("%w: reply unparseable", coachwalk.ErrRetry)
	}
	var extra []coachwalk.Violation
	// 🚨 这一条能验的那件事：她还在第二个部分，陪练不该把她推过去。
	// 和短文那条走查的 advance 判据同一个形状，但不依赖任何一句具体的话。
	if parsed.Advance == "done" && len(d.msgs) < 4 {
		extra = append(extra, coachwalk.Violation{
			Kind: "advanced-too-early",
			Note: "她才刚开始读这个部分，陪练就把这一步判成完成了",
		})
	}
	return parsed.Reply, extra, nil
}

// Advance 把这一轮接进状态：她说的那句和陪练的回复都进对话，下一轮的
// Request() 因此带着它们 —— 和生产一样。
func (d *LongReadWalkDriver) Advance(raw, reply, said string) {
	if said != "" {
		d.seq++
		d.msgs = append(d.msgs, sqlc.AtomMessage{Seq: d.seq, Role: "student", Content: said})
		d.student = said
	}
	if reply != "" {
		d.seq++
		d.msgs = append(d.msgs, sqlc.AtomMessage{Seq: d.seq, Role: "ai", Content: reply})
	}
}

// HerWords 是这一轮该被接住的语料：她刚说的那句。
func (d *LongReadWalkDriver) HerWords() string { return d.student }

func (d *LongReadWalkDriver) Screen(reply string) string { return reply }

func (d *LongReadWalkDriver) Persona() string { return readingPersona }

// LongReadWalkJudge 不提任何一句具体的话 —— 它问的是「在一篇长文里，陪练有没有
// 守住当前这个部分、有没有让她自己读出来」，所以换一篇文章也还成立。
const LongReadWalkJudge = `下面是一个高中生和 AI 阅读陪练的完整多轮对话。
文章是一篇英文的正反辩论稿（数据中心值不值得建），被切成了三个部分，
她**当前这一步只读第二个部分（反方那几段）**。

给 1–5 分，只看教学，不看文笔：
5 = 每一轮都扣着反方那几段问，一次只问一个问题，问题指向文章里具体的句子或数字；
    她说得含糊时要求她说得更具体，而不是替她说；不越过当前这一部分去讲别的部分。
3 = 大致在带，但有时泛泛而谈（「你觉得呢」），或者替她总结了本该她说的话。
1 = 直接把反方的理由讲完了，或者一轮里堆了好几个问题，或者跑去讲别的部分／整篇结论。

**特别注意**：如果陪练引用了一段它其实没被给到的文字（比如别的部分的句子），
或者指着一个不存在的段落说话，直接记 1 分并在理由里点出来。

先给分，再用一两句说清楚扣在哪里。`

// LongReadWalkScenarios 是那对只差「给不给 parts」的走查。
func LongReadWalkScenarios() []coachwalk.Scenario {
	script := []string{
		"我读完正方了，他们说数据中心能带来工作和税收。",
		"反方好像在说电用得太多了。",
		"还有水，冷却要用很多水。",
		"",
	}
	return []coachwalk.Scenario{
		{
			Suite: liteReadingCoachSuite, ID: "lite-reading-coach/long-article-full", Version: 1,
			Judge: LongReadWalkJudge,
			Make:  func() coachwalk.Driver { return newLongReadWalkDriver(false) },
			Script: script,
		},
		{
			Suite: liteReadingCoachSuite, ID: "lite-reading-coach/long-article-scoped", Version: 1,
			Judge: LongReadWalkJudge,
			Make:  func() coachwalk.Driver { return newLongReadWalkDriver(true) },
			Script: script,
		},
	}
}
