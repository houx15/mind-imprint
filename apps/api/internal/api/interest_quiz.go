package api

// interest_quiz.go —— 觉醒协议（兴趣测试）的 HTTP 层。
//
// 三个端点，一个比一个短：
//
//	GET  /api/v1/interest/quiz         她做过没有（树上那条邀请要不要出现）
//	POST /api/v1/interest/quiz         开一次作答，拿到 id
//	PUT  /api/v1/interest/quiz/{id}    交卷：落库 → 采集 → 种进同一棵树
//
// # 为什么「开」和「交」是两次请求
//
// 多一次往返，换的是**一次中途退出也留下痕迹**。她在第三屏关掉页面，库里就有
// 一行 finished_at 为空的作答；那一行不算「做过了」（邀请还会再出现），但它记着
// 她走到过这里。摩擦被转成信号，而不是被消灭（铁律④）。
//
// # 交卷这一步会慢，而且必须慢
//
// PUT 里同步跑一次采集调用，然后才返回结果。这里不用 detachedModelCtx 的
// 「先回、后台补」写法，因为**结果页要显示的就是那几个词**：先返回一个空结果、
// 几秒后才悄悄长出来，会让她在最该看见回报的那一刻看见一片空白。所以这一次
// 让她等，并且前端要为此显示「正在生成」。
//
// # 它可能长出零个词，而那不是错误
//
// 她如果只写了「很帅」，就没有可摘的原话（interest.ShouldHarvest 会直接不发那次
// 调用）。返回的 keywords 是空数组，结果页照实说，并请她多写一句。**绝不编一个
// 像样的词填进去** —— 见 memory: ai-errors-must-surface-never-fake。

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/disciplines"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/interest"
	"mindimprint/api/internal/store/sqlc"
)

/* ── DTO ────────────────────────────────────────────────────────────────── */

type quizStatusDTO struct {
	// Taken 只在她**做完过**至少一次时为真。中途退出不算。
	Taken bool `json:"taken"`
	// Latest 是她最近做完的那一次，没有则为 nil。
	Latest *quizAttemptDTO `json:"latest"`
}

type quizAttemptDTO struct {
	ID                string `json:"id"`
	Navigator         string `json:"navigator"`
	AnchorWork        string `json:"anchorWork"`
	AnchorReason      string `json:"anchorReason"`
	Hook              string `json:"hook"`
	ChallengeChoice   string `json:"challengeChoice"`
	ChallengeAttempts int    `json:"challengeAttempts"`
	FinishedAt        string `json:"finishedAt"`
}

// quizLensDTO 是结果页上的一片学科透镜。
//
// 带 syllabus 是这一屏最值钱的地方：她刚说完自己喜欢什么，下一句话就是
// 「这在 IB 里叫发展心理学，HL 的 Unit 2」。原型给的是一个手写的学科名字，
// 那句话说不出来。
type quizLensDTO struct {
	ID       string              `json:"id"`
	Zh       string              `json:"zh"`
	En       string              `json:"en"`
	Field    string              `json:"field"`
	Asks     string              `json:"asks"`
	Method   string              `json:"method"`
	Exemplar string              `json:"exemplar"`
	Syllabus []quizSyllabusRefTO `json:"syllabus"`
}

type quizSyllabusRefTO struct {
	Board string `json:"board"`
	Code  string `json:"code"`
	Label string `json:"label"`
	Level string `json:"level,omitempty"`
}

type quizResultDTO struct {
	Attempt quizAttemptDTO `json:"attempt"`
	Lenses  []quizLensDTO  `json:"lenses"`
	// Keywords 是这次真的种到树上的词。**可能是空的**，界面必须照实说。
	Keywords []quizPlantedDTO `json:"keywords"`
	// Harvested 说采集到底跑没跑。false 表示她写得太短，我们没有发那次调用 ——
	// 这和「跑了但一个词都没长出来」是两回事，界面要说的话也不一样。
	Harvested bool `json:"harvested"`
}

type quizPlantedDTO struct {
	TextZh   string `json:"textZh"`
	TextEn   string `json:"textEn"`
	Field    string `json:"field"`
	Note     string `json:"note"`
	Evidence string `json:"evidence"`
}

/* ── 端点 ───────────────────────────────────────────────────────────────── */

// getInterestQuizStatus —— GET /api/v1/interest/quiz
func (a *API) getInterestQuizStatus(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	ctx := r.Context()

	n, err := a.d.Queries.CountFinishedInterestQuizzes(ctx, u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := quizStatusDTO{Taken: n > 0}
	if n > 0 {
		if row, err := a.d.Queries.LatestFinishedInterestQuiz(ctx, u.ID); err == nil {
			dto := quizAttemptToDTO(row)
			out.Latest = &dto
		}
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// startInterestQuiz —— POST /api/v1/interest/quiz
func (a *API) startInterestQuiz(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	row, err := a.d.Queries.StartInterestQuiz(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, quizAttemptToDTO(row))
}

type finishQuizBody struct {
	Navigator         string `json:"navigator"`
	AnchorWork        string `json:"anchorWork"`
	AnchorReason      string `json:"anchorReason"`
	Hook              string `json:"hook"`
	ChallengeChoice   string `json:"challengeChoice"`
	ChallengeAttempts int    `json:"challengeAttempts"`
}

// finishInterestQuiz —— PUT /api/v1/interest/quiz/{id}
func (a *API) finishInterestQuiz(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_quiz_id", "作答 id 无效", nil))
		return
	}
	var body finishQuizBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "请求体解析失败", nil))
		return
	}

	att := interest.Attempt{
		Navigator:         body.Navigator,
		Work:              body.AnchorWork,
		Reason:            body.AnchorReason,
		Hook:              interest.Hook(body.Hook),
		ChallengeChoice:   body.ChallengeChoice,
		ChallengeAttempts: body.ChallengeAttempts,
	}.Clean()

	// 先落库。采集失败、模型不在、她中途断网 —— 这次作答本身都已经存下来了。
	row, err := a.d.Queries.FinishInterestQuiz(r.Context(), sqlc.FinishInterestQuizParams{
		ID: id, UserID: u.ID,
		Navigator: att.Navigator, AnchorWork: att.Work, AnchorReason: att.Reason,
		Hook: string(att.Hook), ChallengeChoice: att.ChallengeChoice,
		ChallengeAttempts: int32(att.ChallengeAttempts),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	out := quizResultDTO{
		Attempt:  quizAttemptToDTO(row),
		Lenses:   lensesToDTO(interest.HookLenses(att.Hook)),
		Keywords: []quizPlantedDTO{},
	}

	// 采集。用脱离请求生命周期的 context：她如果在等待时切走，那次已经花掉的
	// 调用不该被取消 —— 词照样种进树里，她下次打开就看见了。
	mCtx, cancel := detachedModelCtx(r)
	defer cancel()
	if hs, ran := a.harvestQuiz(mCtx, u.ID, row.ID, att); ran {
		out.Harvested = true
		for _, h := range hs {
			out.Keywords = append(out.Keywords, quizPlantedDTO{
				TextZh: h.TextZh, TextEn: h.TextEn, Field: h.Field,
				Note: h.Note, Evidence: h.Evidence,
			})
		}
	}

	httpx.WriteJSON(w, http.StatusOK, out)
}

/* ── 采集 ───────────────────────────────────────────────────────────────── */

// harvestQuiz 从这次作答里长词，并种进树。
//
// 返回的 ran 说**这次调用到底发没发**：她写得太短时我们根本不发（那不是失败，
// 是没有可采的），而发了却一个词都没长出来是另一件事。界面对这两种情况说的话
// 不一样，所以这里必须把它们分开。
func (a *API) harvestQuiz(
	ctx context.Context,
	userID, quizID uuid.UUID,
	att interest.Attempt,
) ([]interest.Harvested, bool) {
	if !att.ShouldHarvest() {
		return nil, false
	}
	// 没有 provider 就没有这次调用 —— gateway.Collect 会对 nil provider 直接
	// panic，而一次交卷 panic 掉的代价是她刚花的五分钟。测试里的 provider 就是
	// nil（liteHandler 不装模型），所以这条不是防御性代码，是走得到的分支。
	if a.d.Provider == nil {
		return nil, false
	}
	resolved, ok := a.route(ctx, gateway.ClassCompose)
	if !ok {
		slog.Warn("interest quiz: no provider resolved", "quiz_id", quizID)
		return nil, false
	}
	system, user := att.BuildQuizPrompt()
	res, cerr := gateway.Collect(ctx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: system},
			{Role: gateway.RoleUser, Content: user},
		},
	})
	// 兴趣测试没有 atom，所以 atomID 传 uuid.Nil —— llm_call.atom_id 自 0094
	// 起可空，这次调用照样记账。
	a.recordLiteLLMCall(ctx, userID, uuid.Nil, "interest_quiz", resolved, res.Usage)
	if cerr != nil {
		slog.Warn("interest quiz: provider call failed", "err", cerr, "quiz_id", quizID)
		return nil, false
	}
	hs, perr := interest.ParseHarvestReply(res.Text)
	if perr != nil {
		// 不长词，也不编词。
		slog.Warn("interest quiz: unparseable reply", "err", perr, "quiz_id", quizID)
		return nil, true
	}
	// 🚨 evidence 必须真的出自她敲进去的字。实测抓到过一次：模型把 prompt 里
	// 我自己写的脚手架文字当成她的原话返回，长度合格、语义通顺、能过解析器，
	// 然后挂在她树上标着「你自己写的」。见 interest.KeepGrounded。
	before := len(hs)
	hs = interest.KeepGrounded(hs, att.OwnWords())
	if n := before - len(hs); n > 0 {
		slog.Warn("interest quiz: dropped ungrounded keywords", "dropped", n, "quiz_id", quizID)
	}
	// 🚨 ref_id 用这一行作答的 id，不是 uuid.Nil。keyword_source 的
	// UNIQUE (keyword_id, kind, ref_id) 会因此允许**下一次作答给同一个词再添
	// 一条来源**（强度上升）；用 Nil 的话第二次重做会撞进唯一约束，一个词都
	// 加不上，而「重做是再长几个词」正是这个测试的设计。
	a.plantKeywords(ctx, userID, "quiz", quizID, interest.QuizSourceLabel, hs)
	return hs, true
}

/* ── 转换 ───────────────────────────────────────────────────────────────── */

func quizAttemptToDTO(row sqlc.InterestQuiz) quizAttemptDTO {
	finished := ""
	if row.FinishedAt.Valid {
		finished = row.FinishedAt.Time.Format(time.RFC3339)
	}
	return quizAttemptDTO{
		ID:                row.ID.String(),
		Navigator:         row.Navigator,
		AnchorWork:        row.AnchorWork,
		AnchorReason:      row.AnchorReason,
		Hook:              row.Hook,
		ChallengeChoice:   row.ChallengeChoice,
		ChallengeAttempts: int(row.ChallengeAttempts),
		FinishedAt:        finished,
	}
}

func lensesToDTO(ds []disciplines.Discipline) []quizLensDTO {
	out := make([]quizLensDTO, 0, len(ds))
	for _, d := range ds {
		refs := make([]quizSyllabusRefTO, 0, len(d.Syllabus))
		for _, s := range d.Syllabus {
			refs = append(refs, quizSyllabusRefTO{
				Board: s.Board, Code: s.Code, Label: s.Label, Level: s.Level,
			})
		}
		out = append(out, quizLensDTO{
			ID: d.ID, Zh: d.Zh, En: d.En, Field: d.Field,
			Asks: d.Asks, Method: d.Method, Exemplar: d.Exemplar,
			Syllabus: refs,
		})
	}
	return out
}
