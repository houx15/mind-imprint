package api

// reading_plan.go — 任务清单：印记 给一篇文章排出的读法。
//
// 产品的原话：「an intelligent reading coach would generate a task list after
// students giving a paragraph.」
//
// 模型在这条链路上做的是**挑选和调参**，不是自由编任务：
//
//   - 挑哪一套 routine（reading_routines.go 里写死的四套之一）
//   - 指出哪一两段是重点（focus_block 的 blockId）——这是真判断，也是这个教练
//     能贡献的最有价值的东西
//   - 按这篇文章把每一步的 detail 说得更具体一点
//   - 篇幅短就砍掉几步
//
// 它**不能**发明步骤。步骤的 kind 和顺序来自 routine，模型只填得进那几个槽。
// 「以后按学生能力和文章难度生成」因此不需要任何结构改动——那只是这一次挑选
// 调用的额外输入。
//
// 这一步不是关卡：清单排出来之后，每一步都能直接点「跳过」，跳过被记录
// （reading_task.status='skipped'，铁律④），不被拦住。

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// readingPlanArticleRuneBudget bounds how much article feeds the planning
// call. Generous — the model has to judge which paragraphs matter, which
// means seeing them — but bounded, because lite has no compaction layer.
const readingPlanArticleRuneBudget = 9000

const readingPlanSystem = `你是「印记」，要给一个中学生排出读这篇文章的**任务清单**。

下面会给你：这篇文章（按段落编号）、可选的几套读法、以及她的语言。

你要做的只有五件事：

1. 从给出的读法里**挑一套**（routineKey 必须逐字取自表里）。
2. 指出哪几段值得精读（focusBlocks，用段落编号 b1/b2/…）。这是你最重要的
   判断：挑那种「读懂了这一段，整篇就通了」的段落，或者那种最难、最容易被跳过去
   的段落。不要挑第一段就了事。
   **十段以内挑 1 段，十段以上挑 2 段，二十段以上挑 3 段**，而且要**分散在全篇**，
   不要两段挨着。🚨 你挑几段，清单上就有几个精读步骤 —— 挑得太少，整篇十七段
   的文章最后只有两段被读过。
3. 给每一步写一句**贴着这篇文章**的说明（steps[].detail）。比如不要写「精读重点
   段」，要写「这一段是全文唯一给出数据的地方，值得细读」。
4. 篇幅很短、或者内容很浅的文章，可以少排几步——一步都不能编，但可以不排。
5. 排一份**导读**：oneLine、gist、shape、load、parts。见下。

## 导读

学生一进阅读室就会看见这几样东西，它们是她的地图。**她还没读过这篇文章**——
导读要让一个完全没读过的人看得懂接下来要干什么。

🚨 **全部用中文写，包括 shape 里的那几个词。** 文章是英文的，界面是中文的，
她读的是中文界面。线上第一次跑就写成了英文（「outbreak → blockade → aid
scramble」），那份导读对她等于不存在。**文章里的专有名词照抄原文**
（人名、地名、机构名），其余一律中文。

- **oneLine**：这篇在**问**什么。不超过 30 个字。
  🚨 **问题，不是结论。**「屋顶光伏到底划不划算」可以；
  「屋顶光伏其实并不划算」不行——那句话是 gist 的活儿。
- **gist**：这篇的**中心思想**——作者主张什么。不超过 40 个字。
  写成一句完整的话：「作者认为屋顶光伏在南方划算，在北方要看补贴能撑多久。」
  🚨 **只写作者真的写了的那个主张**，不要替他推一步、不要加你的评价。
  🚨 **写作者的主张，不要写「这篇文章介绍了……」。** 后者是一句目录，不是主旨。
  这篇真的没有主张（纯叙事、纯报道），就写它在讲的那件事是什么，
  一样写成一句完整的话。
- **shape**：它是怎么组织的，四到六个**中文**词，中间用 → 连。
  比如「问题 → 数据 → 让步 → 结论」「事件 → 各方反应 → 未解决的部分」。
- **parts**：把整篇切开，按顺序，用段落编号划界。
  **每个部分 2 到 4 段**——这是一个中学生一口气读得完的量。段数多的文章就多切
  几个部分，最多 6 个（再多就不是「部分」了）；六个部分还装不下的超长文章，
  每部分放到 5 段。
  🚨 这几个部分**各自会成为清单上的一步**：她读完一部分答一次，再进下一部分。
  所以切法直接决定她读这篇文章的节奏，不是一份装饰性的目录。
  每个部分给三样：
  - title：这一部分叫什么，中文，不超过 10 个字。比如「提出争议」「实测数据」。
  - from / to：头尾段编号（闭区间），比如 from=b1, to=b3。
  - does：这一部分**在干什么**，不超过 20 个字。说的是它的作用
    （「摆出两方的说法」「用一组数据支持前面那个判断」），
    🚨 **不是它讲了什么内容**——讲了什么要她自己去读。
  🚨 **必须从第一段开始**，各部分之间**不许重叠**，顺序不许乱。
  段落编号必须真实存在（系统会核对，对不上整份切法作废，她就没有台阶可走）。
  文章太短（少于四段）切不出两部分，就给一个空数组。
- **load**：**每一段**的承重，一段一个值，只能取这三个之一：
  - 「core」（核心）——承载主张的那几段。读到这里要停下来。
  - 「support」（支撑）——证据、例子、数据。它们在撑上面某个主张。
  - 「bridge」（过渡）——转场、连接、背景交代。
  🚨 **核心段不要超过全文的三分之一。** 全都是核心等于没有核心，
  系统会把整份导读丢掉，她就什么地图都看不到。一篇十二段的报道，
  核心段通常是三到四段。
  🚨 **每一段都要给一个值**，包括小标题那一段（小标题算 「bridge」）。

严格规则：
- **不要替她读。** steps[].detail 里不要出现这篇文章的结论、答案、要点总结。
  你在说「这一步要干什么」，不是在说「这篇讲了什么」。
  🚨 这一条**管的是 steps，不管 gist**。gist 那一项就是要写出中心思想 ——
  2026-09-16 定的（见上）。两者不冲突：她拿着中心思想去读，要她做的事是看
  作者**凭什么**这么说，而那件事一步都没被拿走。
- 步骤的 kind 和顺序**只能**来自你挑的那套读法，不能新增、不能改顺序。
- focusBlocks 必须是真实存在的段落编号。
- detail 每条不超过 40 个字。
- **detail 里说段落要用「第几段」，绝对不要写 b1/b2。** 那是给你看的内部标记，
  她的屏幕上没有。（focusBlocks 字段里当然还是用 b1/b2。）

只输出一个 JSON 对象：
{"routineKey":"...","focusBlocks":["b3"],"steps":[{"kind":"read","detail":"..."}],
 "oneLine":"...","gist":"...","shape":"... → ... → ...",
 "parts":[{"title":"...","from":"b1","to":"b3","does":"..."}],
 "load":{"b1":"bridge","b2":"core"}}

steps 按顺序对应你挑的那套读法的步骤；kind 逐字照抄。不要输出对象以外的任何
文字或代码块标记。`

func buildReadingPlanPrompt(lang, title string, blocks []Block) string {
	var b strings.Builder
	if t := strings.TrimSpace(title); t != "" {
		b.WriteString("标题：" + t + "\n")
	}
	b.WriteString("她读这篇用的语言：" + lang + "\n")

	b.WriteString("\n【可选的读法（routineKey 只能从这里挑）】\n")
	for _, r := range readingRoutinesFor(lang) {
		b.WriteString("- routineKey=" + r.Key + " · " + r.Name + " · 适合：" + r.Blurb + "\n")
		for i, s := range r.Steps {
			b.WriteString("    " + itoaSmall(i+1) + ". kind=" + string(s.Kind) + " · " + s.Label + "\n")
		}
	}

	b.WriteString("\n【文章，按段落】\n")
	total := 0
	for i, blk := range blocks {
		text := strings.TrimSpace(blk.Text)
		if text == "" {
			continue
		}
		tag := readingBlockTag(i, blk.ID)
		runes := []rune(text)
		if total+len(runes) > readingPlanArticleRuneBudget {
			// Truncate rather than drop: the model still needs to know that a
			// later paragraph EXISTS, or it can never pick it as a focus.
			keep := readingPlanArticleRuneBudget - total
			if keep > 60 {
				b.WriteString(tag + "：" + string(runes[:keep]) + "…（这一段更长，已截断）\n")
				total = readingPlanArticleRuneBudget
			} else {
				b.WriteString(tag + "：（这一段没放进来，但它存在）\n")
			}
			continue
		}
		total += len(runes)
		b.WriteString(tag + "：" + text + "\n")
	}
	return b.String()
}

// readingBlockTag labels a paragraph with BOTH the id the model must emit and
// the ordinal it must speak. The model is told to say 「第几段」 and never
// b1/b2 — but the prompt used to hand it only the ids, so it had to do that
// mapping in its head on every turn. A production walk caught it splitting:
// the prose said 第三段 while focusBlock came back b4, so a tool opened on a
// paragraph the sentence had not named. Writing both removes the inference
// rather than asking the model to be careful.
//
// The ordinal counts every block, including ones the budget elides, so it
// keeps matching the paragraph she is actually looking at.
func readingBlockTag(i int, id string) string {
	return id + "（第" + itoaSmall(i+1) + "段）"
}

func itoaSmall(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

type readingPlanReply struct {
	RoutineKey  string   `json:"routineKey"`
	FocusBlocks []string `json:"focusBlocks"`
	Steps       []struct {
		Kind   string `json:"kind"`
		Detail string `json:"detail"`
	} `json:"steps"`
	// 导读。这一次调用本来就要把全文读一遍并挑出重点段，所以它顺带给出
	// 「这篇在问什么 / 它怎么组织 / 哪几段承重」。见 reading_outline.go。
	OneLine string            `json:"oneLine"`
	Gist    string            `json:"gist"`
	Shape   string            `json:"shape"`
	Load    map[string]string `json:"load"`
	Parts   []readingPart     `json:"parts"`
}

// outline 把这份回复里属于导读的那几样东西拿出来。
func (p readingPlanReply) outline() readingOutline {
	return readingOutline{
		OneLine: p.OneLine, Gist: p.Gist, Shape: p.Shape, Load: p.Load, Parts: p.Parts,
	}
}

// salvageReadingPlan 逐个字段读一份坏掉的排读法回复，到齐的留下。
//
// # 🚨 实测到的那一份（2026-09-08 线上）
//
//	{"routineKey":"en-argument","focusBlocks":["b5","b8"],
//	 "steps":[{"kind":"read":"detail..."}]}
//
// finish_reason "stop"、92 个字符、写完了 —— 不是断在半路，是**写坏了**：
// steps 里一个冒号该是逗号，detail 干脆留成了占位符。
//
// 但坏掉的只有 steps，而 steps 是**可选的调校**：buildReadingTasks 走的是
// routine 自己的步骤表，模型只能往里填 detail，填不上就用读法库里那一句。
// 真正的判断 —— 挑哪套读法、精读哪两段 —— 两样都完整到齐了。
//
// 整份丢掉，她按下「开始」拿到的是 502，而这一步是进阅读室的唯一那道门。
// 和 salvageCoachReply 同一条：到齐的留下，没写好的当它没给。
func salvageReadingPlan(s string) (readingPlanReply, bool) {
	dec := json.NewDecoder(strings.NewReader(s))
	tok, err := dec.Token()
	if err != nil {
		return readingPlanReply{}, false
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return readingPlanReply{}, false
	}
	var got readingPlanReply
	for {
		key, kerr := dec.Token()
		if kerr != nil {
			break
		}
		if d, isDelim := key.(json.Delim); isDelim && d == '}' {
			break
		}
		name, isStr := key.(string)
		if !isStr {
			break
		}
		var raw json.RawMessage
		if verr := dec.Decode(&raw); verr != nil {
			break
		}
		switch name {
		case "routineKey":
			_ = json.Unmarshal(raw, &got.RoutineKey)
		case "focusBlocks":
			_ = json.Unmarshal(raw, &got.FocusBlocks)
		case "steps":
			_ = json.Unmarshal(raw, &got.Steps)
		case "oneLine":
			_ = json.Unmarshal(raw, &got.OneLine)
		case "gist":
			_ = json.Unmarshal(raw, &got.Gist)
		case "shape":
			_ = json.Unmarshal(raw, &got.Shape)
		case "load":
			_ = json.Unmarshal(raw, &got.Load)
		case "parts":
			_ = json.Unmarshal(raw, &got.Parts)
		}
	}
	// routineKey 是唯一不能少的东西 —— 没有它就没有读法，也就没有清单。
	if strings.TrimSpace(got.RoutineKey) == "" {
		return readingPlanReply{}, false
	}
	return got, true
}

// parseReadingPlan decodes and VALIDATES against the library. The routine key
// must resolve and match her language; anything else is treated as no plan at
// all rather than passed through — a routine key that does not resolve would
// render as an empty task list she cannot act on.
func parseReadingPlan(text, lang string) (readingPlanReply, readingRoutine, planReject) {
	c := strings.TrimSpace(text)
	if strings.HasPrefix(c, "```json") {
		c = strings.TrimLeft(strings.TrimPrefix(c, "```json"), " \t\r\n")
	} else if strings.HasPrefix(c, "```") {
		c = strings.TrimLeft(c[3:], " \t\r\n")
	}
	if strings.HasSuffix(c, "```") {
		c = strings.TrimRight(c[:len(c)-3], " \t\r\n")
	}
	if i := strings.IndexByte(c, '{'); i > 0 {
		c = c[i:]
	}
	whole := strings.TrimSpace(c)
	if j := strings.LastIndexByte(c, '}'); j >= 0 && j < len(c)-1 {
		c = c[:j+1]
	}
	var got readingPlanReply
	if err := json.Unmarshal([]byte(strings.TrimSpace(c)), &got); err != nil {
		// 🚨 坏掉的往往只是 steps。见 salvageReadingPlan。
		var ok bool
		if got, ok = salvageReadingPlan(whole); !ok {
			return readingPlanReply{}, readingRoutine{}, planRejectUnparseable
		}
	}
	routine, ok := findReadingRoutine(strings.TrimSpace(got.RoutineKey))
	if !ok {
		return readingPlanReply{}, readingRoutine{}, planRejectUnknownRoutine
	}
	want := lang
	if want != "en" {
		want = "zh"
	}
	if routine.Lang != want {
		return readingPlanReply{}, readingRoutine{}, planRejectWrongLang
	}
	return got, routine, planOK
}

// planReject 说的是排读法这一步为什么没成。
//
// 🚨 原来这三种（加上「排出来一步都不剩」）共用一个 bool 和一句日志
// 「unparseable or out-of-library reply」。2026-09-08 线上 4 次里错 3 次，
// 而同一个 prompt 在本地实测 6 次全过 —— 日志说不出差在哪，只能靠猜。
// 三种的修法完全不同，所以它们现在各说各的。
type planReject string

const (
	planOK                   planReject = ""
	planRejectUnparseable    planReject = "reply is not usable json"
	planRejectUnknownRoutine planReject = "routineKey not in the library"
	planRejectWrongLang      planReject = "routine language does not match the article"
)

// buildReadingTasks turns the routine plus the model's tuning into the rows to
// insert.
//
// The ROUTINE owns the kinds and their order; the model may only fill in
// `detail` and say which blocks to focus on. A model that returned steps in a
// different order, or a kind the routine does not have, is simply ignored —
// the loop walks the ROUTINE, not the reply.
// `parts` 是**校验过的**那份切法（validateParts 的结果，可以为空）。有切法的
// 时候，「通读全文」那一步摊成一步一个部分 —— 见 readingPartSteps。
func buildReadingTasks(routine readingRoutine, plan readingPlanReply, blocks []Block,
	parts []readingPart,
) (
	positions []int32, kinds, labels, details, blockIDs []string,
) {
	// Ordinal, not just validity: the 精读 step's label now carries 第N段, and
	// the number has to be the one her screen shows for that paragraph —
	// counted exactly the way readingBlockTag / readingPickOrdinal count it.
	ordinal := make(map[string]int, len(blocks))
	for i, blk := range blocks {
		ordinal[blk.ID] = i + 1
	}
	focus := make([]string, 0, len(plan.FocusBlocks))
	seenFocus := map[string]bool{}
	for _, id := range plan.FocusBlocks {
		id = strings.TrimSpace(id)
		// 同一段挑两次就是同一步走两遍。去重在这里做，因为下面每一段都会变成
		// 清单上自己的一步。
		if ordinal[id] > 0 && !seenFocus[id] && len(focus) < maxFocusSteps {
			seenFocus[id] = true
			focus = append(focus, id)
		}
	}
	nextFocus := 0
	pos := int32(0)
	for i, step := range routine.Steps {
		detail := step.Detail
		if i < len(plan.Steps) && strings.TrimSpace(plan.Steps[i].Detail) != "" &&
			plan.Steps[i].Kind == string(step.Kind) {
			detail = strings.TrimSpace(plan.Steps[i].Detail)
		}
		label := step.Label
		blockID := ""
		// 通读切成了几步，一步一个部分。
		if step.Kind == taskRead && len(parts) > 0 {
			for _, ps := range readingPartSteps(parts, ordinal) {
				positions = append(positions, pos)
				kinds = append(kinds, string(taskRead))
				labels = append(labels, ps.label)
				details = append(details, ps.detail)
				blockIDs = append(blockIDs, ps.blockID)
				pos++
			}
			continue
		}
		if step.Kind == taskFocusBlock {
			if nextFocus >= len(focus) {
				// A focus step with no paragraph behind it is a dead step —
				// she would be told to read "the highlighted paragraph" with
				// nothing highlighted. Drop it rather than render a lie.
				continue
			}
			// 🚨 模型被要求挑 1–2 段，而 routine 里只有一个精读步 —— 多出来的
			// 那一段以前**静默丢掉**。产品负责人 2026-09-17 逐字报的正是它的
			// 后果：「整篇的交互就集中在 2-3 个段落，其他的段落完全放置了」。
			// 挑了两段就走两步，各自带着自己的段号。
			for nextFocus < len(focus) {
				blockID = focus[nextFocus]
				// 🚨 模型那句 detail 是对着**一段**写的（「这一段是全文唯一给出
				// 数据的地方」）。把它复制到第二个精读步上，那句话就成了一句
				// 关于别的段落的假话。第一步用它，往后的用读法库自己那一句。
				// 「这一段凭什么值得精读」由 印记 在进入那一步的那一轮说
				// （readingCurrentStepInstruction 的 focus_block 分支），
				// 那时候它看得见是哪一段。
				stepDetail := detail
				if nextFocus > 0 {
					stepDetail = step.Detail
				}
				nextFocus++
				positions = append(positions, pos)
				kinds = append(kinds, string(taskFocusBlock))
				labels = append(labels, focusBlockLabel(ordinal[blockID]))
				details = append(details, stepDetail)
				blockIDs = append(blockIDs, blockID)
				pos++
			}
			continue
		}
		positions = append(positions, pos)
		kinds = append(kinds, string(step.Kind))
		labels = append(labels, label)
		details = append(details, detail)
		blockIDs = append(blockIDs, blockID)
		pos++
	}
	return positions, kinds, labels, details, blockIDs
}

// maxFocusSteps 是精读步骤的上限。三：一篇二十段以上的长文，三段精读已经是
// 一次能撑住的量；再多，整份清单就长到她走不完，而一份走不完的清单和一份
// 只读了两段的清单一样没用。
const maxFocusSteps = 3

// readingPartStep 是通读被切开之后的一步：一个部分。
type readingPartStep struct{ label, detail, blockID string }

// readingPartSteps 把「通读全文」摊成一步一个部分。
//
// # 为什么是步骤，不是一段提示词
//
// 「一部分一部分地走」2026-09-16 是写在 system prompt 里的一段散文，而每一轮
// 末尾那条**判据**（readingCurrentStepInstruction 的 taskRead 分支）写的是
// 「已回答本步的通读卡片就给 done」。散文跨不过判据：真模型发一张卡、她答了、
// 这一步当场 done，下一句就进精读。产品负责人 2026-09-17 在一篇 17 段的文章上
// 逐字指出了这一幕（「马上就转到精读了」）。
//
// [[hardcoded-thresholds-vs-user-set-scale-2026-09-12]]：提示词里的软话跨不过
// 代码里的硬判据。所以「走完一个部分」不再由模型自己数 —— 一个部分就是清单上
// 的一步，走完它就是 advance 一次，和别的步骤一模一样。
//
// 顺带解决了另一半：她在进度盘上看得见通读走到哪儿，而在这之前通读是一颗圆点，
// 点亮之前和点亮之后都不知道自己读了多少。
func readingPartSteps(parts []readingPart, ordinal map[string]int) []readingPartStep {
	out := make([]readingPartStep, 0, len(parts))
	for _, p := range parts {
		from, to := ordinal[p.From], ordinal[p.To]
		if from <= 0 || to < from {
			continue
		}
		where := "第" + itoaSmall(from) + "–" + itoaSmall(to) + "段"
		if from == to {
			where = "第" + itoaSmall(from) + "段"
		}
		// 标签带上段号：她在进度盘上一眼看得出这一步读哪几段，不用点开。
		label := "通读" + where + "·" + p.Title
		detail := "请通读" + where + "。"
		if p.Does != "" {
			// does 说的是这一部分**在干什么**，不是它说了什么 —— 那是她要自己
			// 读出来的。validateParts 保证它不超过 20 个字。
			detail += "这几段" + p.Does + "。"
		}
		out = append(out, readingPartStep{label: label, detail: detail, blockID: p.From})
	}
	return out
}

// planReadingTasks is the plan generation itself, split out of the HTTP
// handler so the guided coach can plan on demand: 开始 is the only button she
// has, and pressing it with no plan yet must produce one rather than refuse.
//
// Returns the persisted rows. Errors are already httpx errors, ready to write.
func (a *API) planReadingTasks(
	ctx context.Context,
	userID uuid.UUID,
	atomID uuid.UUID,
	src sqlc.ReadingSource,
	blocks []Block,
) ([]sqlc.ReadingTask, error) {
	lang := readingLangOf(src.Body)

	resolved, okResolve := a.route(ctx, gateway.ClassCompose)
	if !okResolve {
		slog.Warn("reading plan: no provider resolved", "atom_id", atomID)
		return nil, httpx.ErrAIDialogueFailed("model_unavailable")
	}
	res, cerr := gateway.Collect(ctx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: readingPlanSystem},
			{Role: gateway.RoleUser, Content: buildReadingPlanPrompt(lang, src.Title, blocks)},
		},
	})
	a.recordLiteLLMCall(ctx, userID, atomID, "reading_plan", resolved, res.Usage)
	if cerr != nil {
		slog.Warn("reading plan: provider call failed", "err", cerr, "atom_id", atomID)
		return nil, httpx.ErrAIDialogueFailed("model_unavailable")
	}
	plan, routine, reject := parseReadingPlan(res.Text, lang)
	if reject != planOK {
		// No canned fallback routine. A silently-substituted default would be
		// indistinguishable from a real plan, and she would never know the
		// coach had not actually looked at her article.
		slog.Warn("reading plan: rejected", "atom_id", atomID, "why", string(reject),
			"lang", lang, "stop_reason", res.StopReason, "reply_len", len(res.Text),
			"reply_tail", tailRunes(res.Text, 200))
		return nil, httpx.ErrAIDialogueFailed("model_unavailable")
	}

	// 🚨 导读**先**校验，因为清单要用它切出来的那几个部分：通读摊成一步一个
	// 部分（readingPartSteps）。校验没过的切法是 nil，通读就退回整篇一步 ——
	// 和没有切法的短文章走同一条路。
	outline, outlineWhy := validateOutlineWhy(plan.outline(), blocks)

	positions, kinds, labels, details, blockIDs := buildReadingTasks(routine, plan, blocks, outline.Parts)
	if len(positions) == 0 {
		slog.Warn("reading plan: routine produced no usable steps", "atom_id", atomID, "routine", routine.Key)
		return nil, httpx.ErrAIDialogueFailed("model_unavailable")
	}

	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := a.d.Queries.WithTx(tx)

	if _, err := qtx.SetReadingRoutine(ctx, sqlc.SetReadingRoutineParams{
		AtomID: atomID, RoutineKey: routine.Key,
	}); err != nil {
		return nil, err
	}
	if _, err := qtx.ReplaceReadingTasks(ctx, sqlc.ReplaceReadingTasksParams{
		AtomID: atomID, Positions: positions, Kinds: kinds,
		Labels: labels, Details: details, BlockIds: blockIDs,
	}); err != nil {
		return nil, err
	}
	// 导读。校验不过就不写 —— 那一列留着上一次的（或者 '{}'），阅读室因此
	// 不显示导读卡，而不是显示一份修补过的。见 validateOutline。
	if outlineWhy == outlineOK {
		if raw, merr := json.Marshal(outline); merr == nil {
			if _, err := qtx.UpdateReadingSourceOutline(ctx, sqlc.UpdateReadingSourceOutlineParams{
				AtomID: atomID, Outline: raw,
			}); err != nil {
				return nil, err
			}
		}
	} else {
		// 🚨 理由要写进去。这一行原来只有 atom_id 和段数 —— 于是线上只知道
		// 「导读又没了」，四种理由分不出来，而它们的修法完全不同。
		// 这份一丢，通读那一步的台阶（parts）跟着一起丢。
		core := 0
		for _, b := range blocks {
			if plan.Load[b.ID] == loadCore {
				core++
			}
		}
		slog.Info("reading plan: outline rejected", "atom_id", atomID, "blocks", len(blocks),
			"why", string(outlineWhy), "core", core, "parts", len(plan.Parts))
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return a.d.Queries.ListReadingTasks(ctx, atomID)
}

// generateReadingPlan is POST /api/v1/readings/{id}/plan — the explicit
// 重排. It REPLACES any existing plan; the old statuses go with it, and that
// is correct: a new plan is a new set of steps, not the old ones renumbered.
func (a *API) generateReadingPlan(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	entitled, eerr := HasEntitlement(r.Context(), u)
	if eerr != nil {
		httpx.WriteError(w, r, eerr)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	src, err := a.d.Queries.GetReadingSource(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err) // no article yet → 404
		return
	}
	blocks := SplitBlocks(src.Body)
	if len(blocks) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty_article", "这篇还没有正文，先把文章贴进来。", nil))
		return
	}

	// Run to completion even if she navigates away mid-plan: a synchronous
	// POST is cancelled the instant the browser disconnects, which would
	// otherwise spend the call and record nothing.
	turnCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 150*time.Second)
	defer cancel()

	rows, err := a.planReadingTasks(turnCtx, u.ID, at.ID, src, blocks)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rd, err := a.d.Queries.GetReading(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	name := ""
	if routine, found := findReadingRoutine(rd.RoutineKey); found {
		name = routine.Name
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"routineKey": rd.RoutineKey, "routineName": name, "tasks": readingTaskDTOs(rows),
	})
}

type readingTaskDTO struct {
	ID       string `json:"id"`
	Position int32  `json:"position"`
	Kind     string `json:"kind"`
	Label    string `json:"label"`
	Detail   string `json:"detail"`
	BlockID  string `json:"blockId"`
	// 'pending' | 'done' | 'skipped'. The student never sets this any more —
	// the guided coach does (reading_coach.go) — but it is still rendered, as
	// progress she can see rather than a control she operates.
	Status      string  `json:"status"`
	CompletedAt *string `json:"completedAt"`
}

func readingTaskDTOs(rows []sqlc.ReadingTask) []readingTaskDTO {
	out := make([]readingTaskDTO, 0, len(rows))
	for _, row := range rows {
		dto := readingTaskDTO{
			ID: row.ID.String(), Position: row.Position, Kind: row.Kind,
			Label: row.Label, Detail: row.Detail, BlockID: row.BlockID, Status: row.Status,
		}
		if row.CompletedAt.Valid {
			s := row.CompletedAt.Time.Format(time.RFC3339)
			dto.CompletedAt = &s
		}
		out = append(out, dto)
	}
	return out
}

// getReadingPlan is GET /api/v1/readings/{id}/plan. No model call — an empty
// list is the honest "no plan yet" answer, not an error.
func (a *API) getReadingPlan(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListReadingTasks(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rd, err := a.d.Queries.GetReading(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	name := ""
	if routine, found := findReadingRoutine(rd.RoutineKey); found {
		name = routine.Name
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"routineKey": rd.RoutineKey, "routineName": name, "tasks": readingTaskDTOs(rows),
	})
}

// setReadingTaskStatus is POST /api/v1/readings/{id}/plan/tasks/{tid}.
//
// 铁律②: 'skipped' is a first-class outcome, not a failure. The task list is a
// map she can walk past, and skipping is RECORDED (过程即数据) rather than
// prevented.
func (a *API) setReadingTaskStatus(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	tid, err := uuid.Parse(r.PathValue("tid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	switch req.Status {
	case "pending", "done", "skipped":
	default:
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_status", "无效的状态。", nil))
		return
	}
	row, err := a.d.Queries.SetReadingTaskStatus(r.Context(), sqlc.SetReadingTaskStatusParams{
		AtomID: at.ID, ID: tid, Status: req.Status,
	})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, readingTaskDTOs([]sqlc.ReadingTask{row})[0])
}

// readingLangOf guesses the article's language from its own characters, and
// deliberately does NOT ask the student.
//
// She already told us what she wants to read by pasting it; a language picker
// on top of that is a question whose answer is sitting right there. The rule
// is deliberately blunt — any meaningful run of CJK means the paragraph tools
// should be the Chinese set — because the cost of being wrong is offering the
// wrong four buttons, which she can simply not press.
func readingLangOf(body string) string {
	cjk := 0
	for _, ch := range body {
		if (ch >= 0x4e00 && ch <= 0x9fff) || (ch >= 0x3400 && ch <= 0x4dbf) {
			cjk++
			if cjk >= 24 {
				return "zh"
			}
		}
	}
	return "en"
}
