package api

// awakening.go —— 觉醒协议的 HTTP 层。
//
//	GET  /api/v1/awakening                她做过没有、有没有一趟没走完
//	POST /api/v1/awakening                开一趟（或者接上没走完的那一趟）
//	PUT  /api/v1/awakening/{id}           存一次进度
//	POST /api/v1/awakening/{id}/turn      终端里的一轮
//	POST /api/v1/awakening/{id}/finish    走完：选词 → 写回树 → 生成报告
//	GET  /api/v1/awakening/{id}/report    读报告
//
// # 为什么「开」和「走完」是两次请求，中间还有一串
//
// 多几次往返，换的是**一次中途退出也留下痕迹**，并且下次能接着走。她在能量
// 卡牌那一屏关掉页面，库里就有一行 finished_at 为空的 run，带着她已经选过的
// 东西；那一行不算「做过了」（树上那条入口还在），但第二天进来不用重走。
//
// # 这一版里模型被调用两次半
//
//	每一轮      dialogue  她当场读到的那句回应
//	走完那一次  compose   从她八轮的话里选词（闭表）
//	走完那一次  compose   驱动力假设 + 一句总结
//
// 前面九屏一次都不发。一个学生走完开场剧情、翻完三张底牌、选完能量卡牌，
// 到这里为止不花一分钱。
//
// # 走完那一步会慢，而且必须慢
//
// finish 里同步跑两次调用然后才返回，因为**报告要显示的就是那些东西**。
// 先返回一份空报告、几秒后才悄悄长出来，会让她在最该看见回报的那一刻看见
// 一片空白。所以让她等，界面为此显示「正在生成」。

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/awakening"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/interest"
	"mindimprint/api/internal/interests"
	"mindimprint/api/internal/library"
	"mindimprint/api/internal/store/sqlc"
)

// awakeningSourceLabel 是这条来源在关键词抽屉里显示的名字。
const awakeningSourceLabel = "觉醒协议"

// awakeningSourceKind 是写进 keyword_source.kind 的值。
//
// 沿用 'quiz' 而不是新加一档，有两条理由：它在树上显示成「测试」已经是对的
// （liveTree.ts 把 quiz 映射成 course），而且旧的七屏作答和这一版在她看来
// 是同一件事的两个版本，没有理由在抽屉里分成两种来源。
//
// 代价是 ref_id 现在可能指向 interest_quiz 也可能指向 awakening_run。
// ref_id 不是外键，也没有任何查询 join 它，所以这只是一条要写下来的事实。
const awakeningSourceKind = "quiz"

/* ── DTO ────────────────────────────────────────────────────────────────── */

type awakeningRunDTO struct {
	ID               string          `json:"id"`
	AttemptNo        int             `json:"attemptNo"`
	Stage            string          `json:"stage"`
	Route            string          `json:"route"`
	Navigator        string          `json:"navigator"`
	EnergyProfile    json.RawMessage `json:"energyProfile"`
	Talent           json.RawMessage `json:"talent"`
	LensChoice       string          `json:"lensChoice"`
	ChallengeChoice  string          `json:"challengeChoice"`
	ArchiveAttempts  int             `json:"archiveAttempts"`
	ObserverQuestion string          `json:"observerQuestion"`
	FinishedAt       string          `json:"finishedAt"`
	// Turns 是终端里已经发生过的轮次，按顺序。刷新之后靠它把对话重画出来。
	Turns []awakeningTurnDTO `json:"turns"`
	// NextNode 是下一轮要问第几个节点（从 0 起）。等于 NodeCount 表示八问
	// 已经问完，可以走完了。
	NextNode int `json:"nextNode"`
	// OpeningAsk 是终端第一屏显示的那个问题。**服务端生成**：她还没说过话，
	// 这一句完全由已知事实决定，没有理由为它花一次调用。
	OpeningAsk string `json:"openingAsk"`
}

type awakeningTurnDTO struct {
	Seq         int    `json:"seq"`
	NodeIndex   int    `json:"nodeIndex"`
	StudentText string `json:"studentText"`
	Reply       string `json:"reply"`
}

type awakeningStatusDTO struct {
	// Taken 只在她**走完过**至少一次时为真。中途退出不算。
	Taken bool `json:"taken"`
	// Open 是她还没走完的那一趟，没有则为 nil。
	Open *awakeningRunDTO `json:"open"`
	// LatestReportRunID 是她最近那份报告对应的 run id，用于从树上直接打开它。
	LatestReportRunID string `json:"latestReportRunId"`
	FinishedCount     int    `json:"finishedCount"`
}

type awakeningTurnReqDTO struct {
	Text string `json:"text"`
}

type awakeningTurnRespDTO struct {
	Reply     string `json:"reply"`
	NodeIndex int    `json:"nodeIndex"`
	// NextNode 是下一轮问第几个节点。和 NodeIndex 相同表示这一轮是换个问法
	// 再问一次（她刚才答得太薄）。
	NextNode int `json:"nextNode"`
	// Retry 为真表示下一轮在同一个节点换问法。界面据此不推进步骤条。
	Retry bool `json:"retry"`
	// Done 为真表示八问问完了。
	Done bool `json:"done"`
	// Ask 是下一轮要问的那句话，服务端给的默认问法。模型的回复里通常已经
	// 包含了它；界面把它留作占位提示。
	Ask string `json:"ask"`
	// Failed 为真表示这一轮模型没回上来。Reply 会是空串，界面照实说，
	// 绝不填一句像样的话进去。
	Failed bool `json:"failed"`
}

/* ── 状态 ───────────────────────────────────────────────────────────────── */

// getAwakeningStatus —— GET /api/v1/awakening
func (a *API) getAwakeningStatus(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	out := awakeningStatusDTO{}

	n, err := a.d.Queries.CountFinishedAwakeningRuns(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out.FinishedCount = int(n)
	out.Taken = n > 0

	if row, err := a.d.Queries.OpenAwakeningRun(r.Context(), u.ID); err == nil {
		dto, derr := a.runToDTO(r.Context(), u.ID, row)
		if derr != nil {
			httpx.WriteError(w, r, derr)
			return
		}
		out.Open = &dto
	} else if !errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, err)
		return
	}

	if rep, err := a.d.Queries.LatestAwakeningReport(r.Context(), u.ID); err == nil {
		out.LatestReportRunID = rep.RunID.String()
	} else if !errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, out)
}

// startAwakeningRun —— POST /api/v1/awakening
//
// 她已经有一趟没走完时**返回那一趟**，不开新的。部分唯一索引
// awakening_run_open_idx 在库里保证同一件事，这里先查一次是为了给出那一行
// 而不是一个 409 —— 她想做的事是「继续」，不是「知道自己不能开新的」。
func (a *API) startAwakeningRun(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	row, err := a.d.Queries.OpenAwakeningRun(r.Context(), u.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		row, err = a.d.Queries.StartAwakeningRun(r.Context(), u.ID)
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	dto, err := a.runToDTO(r.Context(), u.ID, row)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}

type saveAwakeningBody struct {
	Stage            string          `json:"stage"`
	Route            string          `json:"route"`
	Navigator        string          `json:"navigator"`
	EnergyProfile    json.RawMessage `json:"energyProfile"`
	Talent           json.RawMessage `json:"talent"`
	LensChoice       string          `json:"lensChoice"`
	ChallengeChoice  string          `json:"challengeChoice"`
	ArchiveAttempts  int             `json:"archiveAttempts"`
	ObserverQuestion string          `json:"observerQuestion"`
}

// saveAwakeningProgress —— PUT /api/v1/awakening/{id}
//
// 客户端每次换屏把整份状态发上来，服务端整行覆盖。它幂等：一次网络重试的
// 结果和只发一次完全一样。
func (a *API) saveAwakeningProgress(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_run_id", "作答 id 无效", nil))
		return
	}
	var body saveAwakeningBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "请求体解析失败", nil))
		return
	}
	if !awakening.IsStage(body.Stage) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_stage", "屏 "+body.Stage+" 不存在", nil))
		return
	}
	if !awakening.IsRoute(body.Route) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_route", "路线 "+body.Route+" 不存在", nil))
		return
	}
	// 印记助手三选一。空串是合法的（她还没走到那一屏）。
	if body.Navigator != "" {
		if _, ok := awakening.GuideByID(body.Navigator); !ok {
			httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_navigator", "印记助手 "+body.Navigator+" 不存在", nil))
			return
		}
	}
	if body.ArchiveAttempts < 0 {
		body.ArchiveAttempts = 0
	}

	row, err := a.d.Queries.SaveAwakeningProgress(r.Context(), sqlc.SaveAwakeningProgressParams{
		ID: id, UserID: u.ID,
		Stage:            body.Stage,
		Route:            body.Route,
		Navigator:        body.Navigator,
		EnergyProfile:    jsonOrEmptyObject(body.EnergyProfile),
		Talent:           jsonOrEmptyObject(body.Talent),
		LensChoice:       trimTo(body.LensChoice, 120),
		ChallengeChoice:  trimTo(body.ChallengeChoice, 120),
		ArchiveAttempts:  int32(body.ArchiveAttempts),
		ObserverQuestion: trimTo(body.ObserverQuestion, 120),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// 这一趟已经走完了，或者根本不是她的。两种都不该让她继续往里写。
		httpx.WriteError(w, r, httpx.ErrNotFound("这一趟作答不在进行中"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	dto, err := a.runToDTO(r.Context(), u.ID, row)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}

/* ── 终端的一轮 ─────────────────────────────────────────────────────────── */

// postAwakeningTurn —— POST /api/v1/awakening/{id}/turn
//
// 一轮里发生四件事，顺序要紧：
//
//  1. 判她这一句薄不薄（服务端算，不问模型）。
//  2. 拼 prompt，发一次 dialogue 调用。
//  3. 查这一轮有没有引用她没写过的话；有就重试一次。
//  4. **先落库，再返回**。落库在前，所以一次「她收到了回复但网断了」不会让
//     这一轮凭空消失。
func (a *API) postAwakeningTurn(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_run_id", "作答 id 无效", nil))
		return
	}
	var body awakeningTurnReqDTO
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "请求体解析失败", nil))
		return
	}
	// 🚨 她自己写的字一个都不切。这里只去掉首尾空白。
	text := strings.TrimSpace(body.Text)
	if text == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty_answer", "请输入内容", nil))
		return
	}

	run, err := a.d.Queries.GetAwakeningRun(r.Context(), sqlc.GetAwakeningRunParams{ID: id, UserID: u.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一趟作答不存在"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if run.FinishedAt.Valid {
		httpx.WriteError(w, r, httpx.ErrBadRequest("run_finished", "这一趟作答已经结束", nil))
		return
	}

	turns, err := a.d.Queries.ListAwakeningTurns(r.Context(), run.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	nodeIndex := nextNodeIndex(turns)
	if nodeIndex >= awakening.NodeCount {
		httpx.WriteError(w, r, httpx.ErrBadRequest("terminal_done", "八个问题已经问完了", nil))
		return
	}

	brief, err := a.awakeningBrief(r.Context(), u.ID, run)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// 这一句薄不薄，服务端算。薄了就在同一个节点换个问法再问一次 ——
	// 而且**只换一次**（maxTurnsPerNode）：与其把她卡在第三问，不如带着这句
	// 话往下走，让后面几问多给一些线索。
	//
	// 这里算的是「这一轮要不要换个问法问」，和下面那个「下一轮问哪个节点」
	// 是两件事：这一轮的问题在她发送**之前**就已经问出去了。
	retry := awakening.TooThin(nodeIndex, text) && turnsOnNode(turns, nodeIndex) == 0

	in := awakening.DialogueInput{
		Guide:       awakening.GuideOrDefault(run.Navigator),
		Brief:       brief,
		NodeIndex:   nodeIndex,
		Retry:       retry,
		// 她刚答完最后一个节点，而且这一轮不换问法 —— 后面没有问题了。
		Last: !retry && nodeIndex == awakening.NodeCount-1,
		EnergyFocus: energyFocus(run.EnergyProfile),
		History:     toTurns(turns),
		Latest:      text,
	}
	reply, failed := a.awakeningDialogue(r.Context(), u.ID, run.ID, in, corpusOf(turns, text))

	seq := int32(len(turns))
	if _, err := a.d.Queries.AppendAwakeningTurn(r.Context(), sqlc.AppendAwakeningTurnParams{
		RunID: run.ID, Seq: seq, NodeIndex: int32(nodeIndex),
		StudentText: text, Reply: reply,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// 🚨 下一轮问哪个节点，用**和下一个请求同一个函数**算，把刚记下的这一轮
	// 一起算进去。两处各算各的，正是上面那个 400 的来源。
	updated := append(turns, sqlc.AwakeningTurn{
		RunID: run.ID, Seq: seq, NodeIndex: int32(nodeIndex), StudentText: text,
	})
	next := nextNodeIndex(updated)
	out := awakeningTurnRespDTO{
		Reply: reply, NodeIndex: nodeIndex, NextNode: next,
		Retry: next == nodeIndex, Done: next >= awakening.NodeCount, Failed: failed,
	}
	if !out.Done {
		n := awakening.NodeAt(next)
		out.Ask = n.Ask
		if out.Retry {
			out.Ask = n.Retry
		}
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// awakeningDialogue 发那一次对话调用。
//
// 返回 (回复, 失败了没有)。**失败时回复是空串**，调用方照实说 —— 一句编出来
// 的「能再多说一点吗」会让她对着一个死掉的终端继续打字
// （memory: ai-errors-must-surface-never-fake）。
func (a *API) awakeningDialogue(
	ctx context.Context, userID, runID uuid.UUID,
	in awakening.DialogueInput, corpus string,
) (string, bool) {
	if a.d.Provider == nil {
		return "", true
	}
	resolved, ok := a.route(ctx, gateway.ClassDialogue)
	if !ok {
		slog.Warn("awakening: no provider resolved", "run_id", runID)
		return "", true
	}
	system, user := awakening.BuildDialoguePrompt(in)

	// 最多两次：第一次幻引就重来一次。第三次不再试 —— 一个连着两轮引用她
	// 没写过的话的模型，第三次大概率还是这样，而她在等。
	for attempt := 0; attempt < 2; attempt++ {
		res, cerr := gateway.Collect(ctx, a.d.Provider, resolved, gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: system},
				{Role: gateway.RoleUser, Content: user},
			},
		})
		a.recordLiteLLMCall(ctx, userID, uuid.Nil, "awakening_dialogue", resolved, res.Usage)
		if cerr != nil {
			slog.Warn("awakening: dialogue call failed", "err", cerr, "run_id", runID, "attempt", attempt)
			continue
		}
		reply := awakening.CleanReply(res.Text)
		if reply == "" {
			continue
		}
		if bad := awakening.HallucinatedQuotes(reply, corpus); len(bad) > 0 {
			slog.Warn("awakening: reply quotes words she never wrote",
				"run_id", runID, "attempt", attempt, "quotes", bad)
			if attempt == 0 {
				continue // 再要一次，通常第二次就规矩了
			}
			// 🚨 第二次还这样，**去掉引号**而不是丢掉整轮。不变量仍然成立
			// （打了引号说成她原话的，一定逐字出自她），而她不会看见一个
			// 死掉的终端。见 awakening.StripBadQuotes 的说明。
			return awakening.StripBadQuotes(reply, corpus), false
		}
		return reply, false
	}
	return "", true
}

/* ── 走完 ───────────────────────────────────────────────────────────────── */

// finishAwakeningRun —— POST /api/v1/awakening/{id}/finish
func (a *API) finishAwakeningRun(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_run_id", "作答 id 无效", nil))
		return
	}

	// 先盖章。盖不上说明别人已经盖过了 —— 那时直接去读那一份报告，
	// 不再花第二次调用。
	run, err := a.d.Queries.FinishAwakeningRun(r.Context(), sqlc.FinishAwakeningRunParams{ID: id, UserID: u.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		rep, rerr := a.d.Queries.GetAwakeningReportByRun(r.Context(),
			sqlc.GetAwakeningReportByRunParams{RunID: id, UserID: u.ID})
		if rerr != nil {
			httpx.WriteError(w, r, httpx.ErrNotFound("这一趟作答不在进行中"))
			return
		}
		writeRawReport(w, rep)
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// 用脱离请求生命周期的 context：她如果在等待时切走，已经花掉的调用不该
	// 被取消 —— 词照样种进树里，报告照样存下来，她下次打开就看见了。
	mCtx, cancel := detachedModelCtx(r)
	defer cancel()

	report, err := a.buildAwakeningReport(mCtx, u.ID, run)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	payload, err := json.Marshal(report)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	row, err := a.d.Queries.InsertAwakeningReport(mCtx, sqlc.InsertAwakeningReportParams{
		RunID: run.ID, UserID: u.ID, Payload: payload,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// 冲突：别人先写了。读那一份。
		row, err = a.d.Queries.GetAwakeningReportByRun(mCtx,
			sqlc.GetAwakeningReportByRunParams{RunID: run.ID, UserID: u.ID})
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	writeRawReport(w, row)
}

// getAwakeningReport —— GET /api/v1/awakening/{id}/report
func (a *API) getAwakeningReport(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_run_id", "作答 id 无效", nil))
		return
	}
	row, err := a.d.Queries.GetAwakeningReportByRun(r.Context(),
		sqlc.GetAwakeningReportByRunParams{RunID: id, UserID: u.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一趟还没有报告"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	writeRawReport(w, row)
}

// buildAwakeningReport 跑那两次 compose 调用，写回树，拼出整份报告。
//
// 顺序要紧：**先选词、先写回树，再生成散文**。散文那一次失败时报告照样成立
// （树上的词已经在了，她的问题是她自己写的），反过来不成立。
func (a *API) buildAwakeningReport(
	ctx context.Context, userID uuid.UUID, run sqlc.AwakeningRun,
) (awakening.Report, error) {
	turns, err := a.d.Queries.ListAwakeningTurns(ctx, run.ID)
	if err != nil {
		return awakening.Report{}, err
	}
	answers := answersByNode(turns)
	corpus := awakening.Corpus(answersAll(turns))

	rep := awakening.Report{
		Version:     awakening.ReportVersion,
		AttemptNo:   int(run.AttemptNo),
		Navigator:   run.Navigator,
		Question:    awakening.HerQuestion(answers),
		WorkConcept: awakening.HerWorkConcept(answers),
		Talent:      talentPiles(run.Talent),
		Answers:     answers,
		Pursuing:    []awakening.Planted{},
		Drivers:     []awakening.Driver{},
		Readings:    []awakening.ReadingPick{},
		OpenFields:  []string{},
	}

	brief, err := a.awakeningBrief(ctx, userID, run)
	if err != nil {
		return rep, err
	}

	// ── 选词 → 写回树 ────────────────────────────────────────────────────
	planted, selectionOK := a.awakeningSelect(ctx, userID, run, answers, corpus, brief)
	rep.Pursuing = planted
	rep.SelectionFailed = !selectionOK
	rep.OpenFields = awakening.StillOpen(brief.EmptyFields, planted)

	// ── 阅读推荐：只从真实库里挑 ─────────────────────────────────────────
	rep.Readings = a.awakeningReadings(ctx, userID, planted)

	// ── 驱动力假设 + 一句总结 ────────────────────────────────────────────
	drivers, summary := a.awakeningProse(ctx, userID, run.ID, answers, corpus)
	rep.Drivers = drivers
	rep.Summary = summary

	// ── 和上次比 ─────────────────────────────────────────────────────────
	if prev, days := a.previousReport(ctx, userID, run); prev != nil {
		rep.Diff = awakening.BuildDiff(planted, prev, days)
	}
	return rep, nil
}

// awakeningSelect 跑选词那一次调用，把结果写回树。
//
// 一个词都没长出来是一个**正常结果**，不是错误：她写得少、写得抽象，就该长出
// 零个词，而报告照实说。绝不编一个像样的词填进去。
// awakeningSelectAttempts 是选词最多打几次。两次：够接住一次写坏的 JSON，
// 又不会在真的连不上时把她晾在加载动画里太久。
const awakeningSelectAttempts = 2

// retryHarvest 打到读得懂为止，最多 attempts 次。
//
// 第二个返回值是「成没成」，和「挑出来几个」无关 —— 一次成功但零结果
// （她确实没有可落的词）返回 (nil, true)，这和失败必须分得开。
func retryHarvest(
	attempts int, once func(attempt int) ([]interest.Harvested, error),
) ([]interest.Harvested, bool) {
	for attempt := 1; attempt <= attempts; attempt++ {
		hs, err := once(attempt)
		if err == nil {
			return hs, true
		}
	}
	return nil, false
}

// awakeningSelect 选词并写回树。
//
// 第二个返回值是「这一步跑成了没有」，**不是**「有没有挑出词」。挑不出词是
// 一种正常结果（她确实没写出可落的东西），调用失败或回话读不懂是我们的故障；
// 报告对这两件事要说不一样的话。见 awakening.Report.SelectionFailed。
func (a *API) awakeningSelect(
	ctx context.Context, userID uuid.UUID, run sqlc.AwakeningRun,
	answers []string, corpus string, brief awakening.TreeBrief,
) ([]awakening.Planted, bool) {
	if a.d.Provider == nil {
		// 没配模型的环境（本地、测试）不算故障。
		return []awakening.Planted{}, true
	}
	resolved, ok := a.route(ctx, gateway.ClassCompose)
	if !ok {
		slog.Warn("awakening: no provider for selection", "run_id", run.ID)
		return []awakening.Planted{}, false
	}
	system, user := awakening.BuildSelectionPrompt(answers, awakening.HerQuestion(answers))

	// 🚨 读不懂就再问一次。
	//
	// 模型偶尔回写坏的 / 半份的 JSON（memory: model-json-half-arrived-2026-09-08），
	// 而**这一次调用是终点**：报告只生成一次、只写一行，一次解析失败就等于她这
	// 一趟写的八段话一个词都进不了树，而且没有第二次机会。阅读室采集失败还能换
	// 一篇再来，这里不能，所以这里值得多打一次。
	// 2026-09-19 线上实测：`invalid character ',' after object key`，第一趟直接
	// 零词。
	hs, ok := retryHarvest(awakeningSelectAttempts, func(attempt int) ([]interest.Harvested, error) {
		res, cerr := gateway.Collect(ctx, a.d.Provider, resolved, gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: system},
				{Role: gateway.RoleUser, Content: user},
			},
		})
		a.recordLiteLLMCall(ctx, userID, uuid.Nil, "awakening_select", resolved, res.Usage)
		if cerr != nil {
			slog.Warn("awakening: selection call failed",
				"err", cerr, "run_id", run.ID, "attempt", attempt)
			return nil, cerr
		}
		parsed, perr := interest.ParseHarvestReply(res.Text)
		if perr != nil {
			slog.Warn("awakening: unparseable selection reply",
				"err", perr, "run_id", run.ID, "attempt", attempt)
			return nil, perr
		}
		return parsed, nil
	})
	if !ok {
		slog.Error("awakening: selection gave up",
			"run_id", run.ID, "attempts", awakeningSelectAttempts)
		return []awakening.Planted{}, false
	}

	// 🚨 evidence 必须真的出自她敲进去的字。语料只含她写的话，不含印记说的话。
	before := len(hs)
	hs = interest.KeepGrounded(hs, corpus)
	if n := before - len(hs); n > 0 {
		slog.Warn("awakening: dropped ungrounded keywords", "dropped", n, "run_id", run.ID)
	}
	if len(hs) == 0 {
		return []awakening.Planted{}, true
	}

	known := knownIDs(brief)
	planted := awakening.Classify(hs, known)

	// 🚨 ref_id 用这一趟的 id，不是 uuid.Nil。keyword_source 的
	// UNIQUE (keyword_id, kind, ref_id) 因此允许**下一趟给同一个词再添一条
	// 来源**（强度上升）；用 Nil 的话第二趟一个词都加不上，而「重做是再长
	// 几个词」正是这条协议的设计。
	a.plantKeywords(ctx, userID, awakeningSourceKind, run.ID, awakeningSourceLabel, hs)

	// 写回之后再读一次强度，报告上那个数字才是她刷新树之后看到的那个。
	fillStrengths(ctx, a, userID, planted)
	return planted, true
}

// awakeningProse 跑报告那一次调用。失败时两样都给空 —— 报告少两块，不编。
func (a *API) awakeningProse(
	ctx context.Context, userID, runID uuid.UUID, answers []string, corpus string,
) ([]awakening.Driver, string) {
	if a.d.Provider == nil {
		return []awakening.Driver{}, ""
	}
	resolved, ok := a.route(ctx, gateway.ClassCompose)
	if !ok {
		return []awakening.Driver{}, ""
	}
	system, user := awakening.BuildReportPrompt(answers)
	res, cerr := gateway.Collect(ctx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: system},
			{Role: gateway.RoleUser, Content: user},
		},
	})
	a.recordLiteLLMCall(ctx, userID, uuid.Nil, "awakening_report", resolved, res.Usage)
	if cerr != nil {
		slog.Warn("awakening: report call failed", "err", cerr, "run_id", runID)
		return []awakening.Driver{}, ""
	}
	drivers, summary, perr := awakening.ParseReportReply(res.Text)
	if perr != nil {
		slog.Warn("awakening: unparseable report reply", "err", perr, "run_id", runID)
		return []awakening.Driver{}, ""
	}
	before := len(drivers)
	drivers = awakening.KeepGroundedDrivers(drivers, corpus)
	if n := before - len(drivers); n > 0 {
		slog.Warn("awakening: dropped ungrounded drivers", "dropped", n, "run_id", runID)
	}
	if drivers == nil {
		drivers = []awakening.Driver{}
	}
	return drivers, summary
}

// awakeningReadings 从真实的分级阅读库里挑几篇。
//
// **只保留有交集的那几条。** library.Recommend 在交集为空时会拿库里的顺序
// 补位（Why 为空），那对一书架是对的 —— 一个刚注册的学生必须看到东西。
// 但报告这一块的承诺是「按你刚说的东西挑的」，所以补位的一律丢掉，
// 一篇都没有就说一篇都没有。
func (a *API) awakeningReadings(
	ctx context.Context, userID uuid.UUID, planted []awakening.Planted,
) []awakening.ReadingPick {
	out := []awakening.ReadingPick{}
	ids := awakening.DisciplineIDs(planted)
	if len(ids) == 0 {
		return out
	}
	// 读她真实的 profile —— 已经开过哪些文章、该给她哪一档难度，都复用书架
	// 那一套（libraryProfileIn），所以报告推的难度和书架推的是同一个判断。
	profile, err := libraryProfileIn(ctx, a.d.Queries, userID)
	if err != nil {
		slog.Warn("awakening: library profile failed", "err", err, "user_id", userID)
		profile = library.Profile{ReadSlugs: map[string]bool{}, Tier: 2}
	}
	// 学科那一半**换成这一趟的结论**：报告这一块的承诺是「按你刚说的东西挑的」，
	// 而她整棵树的学科分布会把这一趟新长出来的方向淹掉。
	profile.Disciplines = map[string]float64{}
	// 权重按名次递减：排在前面的学科是她反复回到的那一个。
	for i, id := range ids {
		profile.Disciplines[id] = math.Max(1, float64(len(ids)-i))
	}
	for _, rec := range library.Recommend(library.All(), profile, awakeningReadingLimit*3) {
		if len(rec.Why) == 0 {
			continue // 补位的不算
		}
		out = append(out, awakening.ReadingPick{
			Slug: rec.Article.Slug, Title: rec.Article.Title,
			ZhTitle: rec.Article.ZhTitle, Field: rec.Article.Field,
			Tier: rec.Tier, Why: rec.Why,
		})
		if len(out) == awakeningReadingLimit {
			break
		}
	}
	return out
}

// awakeningReadingLimit 是报告里最多推几篇。
//
// 三篇。参考设计的阅读地图分基础概念 / 真实案例 / 不同观点三类，三篇正好一类
// 一篇；而一张列着八篇的清单，她一篇都不会点。
const awakeningReadingLimit = 3

/* ── 分享 ───────────────────────────────────────────────────────────────── */

type awakeningShareBody struct {
	Public bool `json:"public"`
}

// shareAwakeningReport —— POST /api/v1/awakening/{id}/share
//
// 这一条和 atom_report_share.go 受同一条裁定约束（R1）：任何人拿到链接都能看，
// 不需要登录，不过期，随时可撤回，她的名字可能出现。她是未成年人，下面这几条
// 是仅有的保护，每一条都吃劲：
//
//  1. token 不可猜：crypto/rand 的 16 字节，**绝不是 run id**。
//  2. 撤回立即生效：置 NULL 之后下一个请求就查不到，没有缓存，没有宽限期。
//  3. 公开出去的只有 payload，不带账号、不带对话记录、不带任何指向她别的
//     东西的 id。
func (a *API) shareAwakeningReport(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_run_id", "作答 id 无效", nil))
		return
	}
	var body awakeningShareBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "请求体解析失败", nil))
		return
	}
	rep, err := a.d.Queries.GetAwakeningReportByRun(r.Context(),
		sqlc.GetAwakeningReportByRunParams{RunID: id, UserID: u.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一趟还没有报告"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	var token *string
	if body.Public {
		if rep.ShareToken != nil && *rep.ShareToken != "" {
			token = rep.ShareToken // 已经公开过就保持同一个链接
		} else {
			raw := make([]byte, 16)
			if _, err := rand.Read(raw); err != nil {
				httpx.WriteError(w, r, err)
				return
			}
			s := hex.EncodeToString(raw)
			token = &s
		}
	}
	row, err := a.d.Queries.SetAwakeningReportShare(r.Context(), sqlc.SetAwakeningReportShareParams{
		ID: rep.ID, UserID: u.ID, ShareToken: token,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := map[string]any{"public": row.ShareToken != nil, "token": ""}
	if row.ShareToken != nil {
		out["token"] = *row.ShareToken
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// getPublicAwakeningReport —— GET /api/v1/public/awakening/{token}
//
// 没有登录也能读。**只投影 payload**，一个别的字段都不带
// —— 一个公开端点多带一个字段，就是把它永久地交给所有人。
func (a *API) getPublicAwakeningReport(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.PathValue("token"))
	if len(token) != 32 {
		httpx.WriteError(w, r, httpx.ErrNotFound("链接无效"))
		return
	}
	row, err := a.d.Queries.GetAwakeningReportByShareToken(r.Context(), &token)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("链接无效或已撤回"))
		return
	}
	writeRawReport(w, row)
}

/* ── 小工具 ─────────────────────────────────────────────────────────────── */

// writeRawReport 把库里那份 payload 原样发出去。
//
// 不反序列化再序列化：一份旧版本的报告（Version 不是今天这个）应当照它当时
// 的样子发出去，而不是被今天的结构体悄悄改写成另一份。
func writeRawReport(w http.ResponseWriter, row sqlc.AwakeningReport) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(append(append([]byte(`{"report":`), row.Payload...), '}'))
}

// runToDTO 把一行 run 连同它的轮次转成 DTO。
func (a *API) runToDTO(ctx context.Context, userID uuid.UUID, row sqlc.AwakeningRun) (awakeningRunDTO, error) {
	turns, err := a.d.Queries.ListAwakeningTurns(ctx, row.ID)
	if err != nil {
		return awakeningRunDTO{}, err
	}
	brief, err := a.awakeningBrief(ctx, userID, row)
	if err != nil {
		return awakeningRunDTO{}, err
	}
	dto := awakeningRunDTO{
		ID: row.ID.String(), AttemptNo: int(row.AttemptNo),
		Stage: row.Stage, Route: row.Route, Navigator: row.Navigator,
		EnergyProfile: rawOrEmptyObject(row.EnergyProfile),
		Talent:        rawOrEmptyObject(row.Talent),
		LensChoice:    row.LensChoice, ChallengeChoice: row.ChallengeChoice,
		ArchiveAttempts: int(row.ArchiveAttempts), ObserverQuestion: row.ObserverQuestion,
		Turns:      []awakeningTurnDTO{},
		NextNode:   nextNodeIndex(turns),
		OpeningAsk: brief.OpeningAsk(),
	}
	if row.FinishedAt.Valid {
		dto.FinishedAt = row.FinishedAt.Time.Format(time.RFC3339)
	}
	for _, t := range turns {
		dto.Turns = append(dto.Turns, awakeningTurnDTO{
			Seq: int(t.Seq), NodeIndex: int(t.NodeIndex),
			StudentText: t.StudentText, Reply: t.Reply,
		})
	}
	return dto, nil
}

// awakeningBrief 读她树上的词，算出那份简报。
func (a *API) awakeningBrief(ctx context.Context, userID uuid.UUID, run sqlc.AwakeningRun) (awakening.TreeBrief, error) {
	rows, err := a.d.Queries.ListInterestKeywords(ctx, userID)
	if err != nil {
		return awakening.TreeBrief{}, err
	}
	known := make([]awakening.Known, 0, len(rows))
	for _, k := range rows {
		id := ""
		if k.InterestID != nil {
			id = *k.InterestID
		}
		known = append(known, awakening.Known{
			InterestID: id, Zh: k.TextZh, Field: k.Field, Strength: int(k.Strength),
		})
	}
	days := 0
	if prev, err := a.d.Queries.PreviousFinishedAwakeningRun(ctx,
		sqlc.PreviousFinishedAwakeningRunParams{UserID: userID, ID: run.ID}); err == nil && prev.FinishedAt.Valid {
		days = int(time.Since(prev.FinishedAt.Time).Hours() / 24)
	}
	return awakening.BuildBrief(known, int(run.AttemptNo), days), nil
}

// previousReport 读上一趟那份报告，以及两趟之间隔了多少天。
func (a *API) previousReport(ctx context.Context, userID uuid.UUID, run sqlc.AwakeningRun) (*awakening.Report, int) {
	prev, err := a.d.Queries.PreviousFinishedAwakeningRun(ctx,
		sqlc.PreviousFinishedAwakeningRunParams{UserID: userID, ID: run.ID})
	if err != nil {
		return nil, 0
	}
	row, err := a.d.Queries.GetAwakeningReportByRun(ctx,
		sqlc.GetAwakeningReportByRunParams{RunID: prev.ID, UserID: userID})
	if err != nil {
		return nil, 0
	}
	var rep awakening.Report
	if err := json.Unmarshal(row.Payload, &rep); err != nil {
		return nil, 0
	}
	days := 0
	if prev.FinishedAt.Valid && run.FinishedAt.Valid {
		days = int(run.FinishedAt.Time.Sub(prev.FinishedAt.Time).Hours() / 24)
	}
	return &rep, days
}

// maxTurnsPerNode 是一个节点最多问几轮。
//
// 两轮：原来那个问法，加上她答得太薄时换的那一个。第三轮没有新的问法可给，
// 只会把她卡在第三问上 —— 与其如此，不如带着这句话往下走，后面几问多给线索。
const maxTurnsPerNode = 2

// nextNodeIndex 从已经发生的轮次算出**下一轮**问第几个节点。
//
// 🚨 这是「下一个节点是谁」的**唯一一处**答案。
//
// 第一版不是这样：handler 用自己那个 retry 标志算响应里的 nextNode，而下一个
// 请求进来时又用这个函数重算一遍。两者在「一个薄答案落在最后一个节点」上分道
// 扬镳 —— 响应说还在第 8 问，下一个请求算出来已经问完了，于是回 400
// terminal_done。2026-09-19 的接口走查抓到的。
//
// 现在 handler 也调它（把刚记下的那一轮一起算进去），所以两边由构造保证一致。
func nextNodeIndex(turns []sqlc.AwakeningTurn) int {
	if len(turns) == 0 {
		return 0
	}
	last := int(turns[len(turns)-1].NodeIndex)
	// 这个节点已经用满了轮数，往下走。
	if turnsOnNode(turns, last) >= maxTurnsPerNode {
		return last + 1
	}
	// 她刚才那句太薄，在同一个节点换个问法再问一次。
	if awakening.TooThin(last, turns[len(turns)-1].StudentText) {
		return last
	}
	return last + 1
}

// turnsOnNode 数这个节点已经问过几轮。
func turnsOnNode(turns []sqlc.AwakeningTurn, nodeIndex int) int {
	n := 0
	for _, t := range turns {
		if int(t.NodeIndex) == nodeIndex {
			n++
		}
	}
	return n
}

// toTurns 把库里的行转成纯逻辑层的结构。
func toTurns(rows []sqlc.AwakeningTurn) []awakening.Turn {
	out := make([]awakening.Turn, 0, len(rows))
	for _, t := range rows {
		out = append(out, awakening.Turn{
			NodeIndex: int(t.NodeIndex), StudentText: t.StudentText, Reply: t.Reply,
		})
	}
	return out
}

// answersAll 是她说过的每一句，按时间顺序。逐字比对的语料。
func answersAll(rows []sqlc.AwakeningTurn) []string {
	out := make([]string, 0, len(rows))
	for _, t := range rows {
		out = append(out, t.StudentText)
	}
	return out
}

// answersByNode 把她的话按**节点**归位，长度恒为 NodeCount。
//
// 这一步有存在的理由：报告要「第 5 问她写的那句」，而轮次和节点不是一一对应
// （一个节点可能问了两轮）。同一个节点有两轮时取**后一轮** —— 那是她被换了
// 一个更具体的问法之后说的，信息更多。
func answersByNode(rows []sqlc.AwakeningTurn) []string {
	out := make([]string, awakening.NodeCount)
	for _, t := range rows {
		i := int(t.NodeIndex)
		if i < 0 || i >= len(out) {
			continue
		}
		out[i] = t.StudentText
	}
	return out
}

// corpusOf 是这一轮做幻引判定用的语料：她之前说过的全部，加上刚敲的这一句。
func corpusOf(rows []sqlc.AwakeningTurn, latest string) string {
	return awakening.Corpus(append(answersAll(rows), latest))
}

// knownIDs 是她树上已有的 interest id。
//
// **用整棵树，不是简报里那前十个。** 简报截断是为了 prompt 的长度，而
// confirm / grow 的判定必须看全 —— 一个排在第十五位的弱词又出现了一次，
// 那正是它该变强的时候。
func knownIDs(b awakening.TreeBrief) []string {
	out := make([]string, 0, len(b.Top))
	for _, k := range b.Top {
		if k.InterestID != "" {
			out = append(out, k.InterestID)
		}
	}
	return out
}

// jsonOrEmptyObject 把客户端发来的一段 JSON 收成能进 jsonb 的字节。
func jsonOrEmptyObject(raw json.RawMessage) []byte {
	if len(raw) == 0 || !json.Valid(raw) {
		return []byte("{}")
	}
	return raw
}

// rawOrEmptyObject 是反方向：库里那一列为空时给一个空对象，
// 免得前端拿到 null 再各自处理一遍。
func rawOrEmptyObject(b []byte) json.RawMessage {
	if len(b) == 0 || !json.Valid(b) {
		return json.RawMessage("{}")
	}
	return json.RawMessage(b)
}

// trimTo 去首尾空白再按字符截断。只用于**选项值**（她点的那个按钮的 id），
// 不用于她自己写的字。
func trimTo(s string, max int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// energyFocus 从能量卡牌那一列里读出一句话，进 prompt。
//
// 读不出来就给空串：这一段是锦上添花，一个格式不对的 jsonb 不该让一次对话
// 调用失败。
func energyFocus(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var p struct {
		Focus   string   `json:"focus"`
		Domains []struct {
			Name  string `json:"name"`
			Short string `json:"short"`
		} `json:"domains"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return ""
	}
	if p.Focus != "" {
		return p.Focus
	}
	var parts []string
	for _, d := range p.Domains {
		if d.Name == "" {
			continue
		}
		if d.Short != "" {
			parts = append(parts, d.Name+"（"+d.Short+"）")
			continue
		}
		parts = append(parts, d.Name)
	}
	return strings.Join(parts, "、")
}

// talentPiles 把天赋卡牌那一列转成报告里的三堆。
func talentPiles(raw []byte) []awakening.TalentPile {
	out := []awakening.TalentPile{}
	if len(raw) == 0 {
		return out
	}
	var p struct {
		Lanes map[string][]string `json:"lanes"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return out
	}
	labels := []struct{ key, label string }{
		{"energy", "有能量"},
		{"learned", "会做但消耗"},
		{"latent", "想发展"},
	}
	for _, l := range labels {
		cards := p.Lanes[l.key]
		if cards == nil {
			cards = []string{}
		}
		out = append(out, awakening.TalentPile{Key: l.key, Label: l.label, Cards: cards})
	}
	return out
}

// fillStrengths 把写回之后的强度读数填进报告。
//
// 报告上那个数字必须和她刷新树之后看到的一样，所以这里**重新查一次**，
// 不在 Go 里自己推算 —— 强度只有 plantKeywords 一条写入路径。
func fillStrengths(ctx context.Context, a *API, userID uuid.UUID, planted []awakening.Planted) {
	rows, err := a.d.Queries.ListInterestKeywords(ctx, userID)
	if err != nil {
		return
	}
	byID := make(map[string]int32, len(rows))
	for _, k := range rows {
		if k.InterestID != nil {
			byID[*k.InterestID] = k.Strength
		}
	}
	for i := range planted {
		if s, ok := byID[planted[i].InterestID]; ok {
			planted[i].Strength = int(s)
		}
	}
}

// 编译期确认闭表里真有东西 —— 一个空词表会让选词 prompt 变成「从下面这些里
// 选」后面跟着一片空白，而模型会照样交出它自己编的 id。
var _ = func() struct{} {
	if len(interests.All()) == 0 {
		panic("awakening: interests catalog is empty")
	}
	return struct{}{}
}()
