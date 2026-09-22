package api

// reading_plan.go — 任务清单：印记 给一篇文章排出的读法。
//
// 产品的原话：「an intelligent reading coach would generate a task list after
// students giving a paragraph.」
//
// 模型在这条链路上做的是**挑选和调参**，不是自由编任务：
//
//   - 挑哪一套 routine（reading_routines.go 里写死的那几套之一，跟着体裁走）
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
	Genre   string            `json:"genre"`
	Shape   string            `json:"shape"`
	Load    map[string]string `json:"load"`
	Parts   []readingPart     `json:"parts"`
}

// outline 把这份回复里属于导读的那几样东西拿出来。
func (p readingPlanReply) outline() readingOutline {
	return readingOutline{
		OneLine: p.OneLine, Gist: p.Gist, Genre: p.Genre,
		Shape: p.Shape, Load: p.Load, Parts: p.Parts,
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
		case "genre":
			_ = json.Unmarshal(raw, &got.Genre)
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

// outlineFixIt —— 导读因为这一条被丢掉时，还给模型的那句话。没有这一句的理由不重问
// （「什么都没填」重问一次也填不出来）。
var outlineFixIt = map[outlineReject]string{
	outlineRejectNotCJK: "导读（oneLine、gist、shape、parts 的 title 和 does）写成了英文。" +
		"界面是中文的，请把整份 JSON 原样重给一遍，只把导读这几项改用中文写（人名、地名、机构名照抄原文）。",
	outlineRejectTooMuchCore: "load 里一半以上的段落标成了 core —— 处处是重点就等于没有重点。" +
		"请把整份 JSON 原样重给一遍，只改 load：core 只留真正承重的那几段（不超过全文的一半）。",
	outlineRejectNoCore: "load 里一段 core 都没有。请把整份 JSON 原样重给一遍，只改 load：标出真正承重的那一两段为 core。",
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

	// 🚨 导读写成了英文：再问一次，把它自己那一份和理由一起还给它。
	//
	// 2026-09-18 线上走查（英文故事那篇）：oneLine、gist、shape、parts 全是英文，
	// 整份导读被丢掉 —— 通读因此没切成几部分，读完也没有「全文总结」。prompt
	// 里「全部用中文写」写了两遍，判据（hasCJK）早就有，缺的是**判出来之后**
	// 做点什么（[[prompt-twice-then-make-it-checkable-2026-09-12]]）。
	// 重问那一份整份过了校验才换上；没过就用第一份（导读照旧丢掉，体裁留下）。
	//
	// 同一天又走出另一种：一篇故事一半以上的段落都标成了 core，整份导读照样丢掉。
	// 同一条路：理由不同，还回去的那句话不同（outlineFixIt）。
	if _, why := validateOutlineWhy(plan.outline(), blocks); outlineFixIt[why] != "" {
		retry, rerr := gateway.Collect(ctx, a.d.Provider, resolved, gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: readingPlanSystem},
				{Role: gateway.RoleUser, Content: buildReadingPlanPrompt(lang, src.Title, blocks)},
				{Role: gateway.RoleAssistant, Content: res.Text},
				{Role: gateway.RoleUser, Content: outlineFixIt[why]},
			},
		})
		a.recordLiteLLMCall(ctx, userID, atomID, "reading_plan", resolved, retry.Usage)
		if rerr == nil {
			if p2, r2, rj2 := parseReadingPlan(retry.Text, lang); rj2 == planOK {
				if _, why2 := validateOutlineWhy(p2.outline(), blocks); why2 == outlineOK {
					plan, routine = p2, r2
				}
			}
		}
		slog.Info("reading plan: outline rejected, asked again", "atom_id", atomID, "why", string(why),
			"fixed", func() bool { _, w := validateOutlineWhy(plan.outline(), blocks); return w == outlineOK }())
	}

	// 读法跟着体裁走（reading_genre.go）。换过读法，模型对着原来那一套写的
	// 每一步说明就对不上了 —— 丢掉，用读法库自己那几句。
	if fitted := pickRoutineForGenre(routine, plan.Genre); fitted.Key != routine.Key {
		slog.Info("reading plan: routine refit to the genre", "atom_id", atomID,
			"genre", plan.Genre, "from", routine.Key, "to", fitted.Key)
		routine = fitted
		plan.Steps = nil
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
	} else if g := validateGenre(plan.Genre); g != "" {
		// 🚨 **导读整份作废，体裁也要留下。**
		//
		// 2026-09-18 实测（TestLiveGenreRoutesEachArticle，那篇故事）：导读因为
		// oneLine 写成了英文被整份丢掉，而体裁就搭在那一份里 —— 于是带读那一侧
		// 读回来是空的，一篇记叙文按议论文带。整个体裁那条链子，被一句英文的
		// 导语拆掉了。
		//
		// 两样东西的判据本来就不一样：导读是**摆给她看的**（写错了语言，那份
		// 地图对她等于不存在），体裁是**给系统看的一个词**，它没有语言可写错。
		// 所以这里单独存那一个词，别的字段留空 —— 导读卡因此照样不显示
		// （ReadingOutlineCard 一个字段都没有就整个不渲染）。
		if raw, merr := json.Marshal(readingOutline{Genre: g}); merr == nil {
			if _, err := qtx.UpdateReadingSourceOutline(ctx, sqlc.UpdateReadingSourceOutlineParams{
				AtomID: atomID, Outline: raw,
			}); err != nil {
				return nil, err
			}
		}
		slog.Info("reading plan: outline rejected, kept the genre", "atom_id", atomID,
			"why", string(outlineWhy), "genre", g)
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
