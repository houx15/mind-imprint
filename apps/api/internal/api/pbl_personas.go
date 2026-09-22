package api

// pbl_personas.go — 主页项目第一关：她的页面给谁看。
//
// 产品负责人 2026-09-03：「draw a persona board with ai generated photos, and
// tagged keywords.」
//
// ## 为什么画像是单独一个端点
//
// 实测一张图 69 秒（internal/gateway/images.go 的记录）。三张连着画就是三分多钟
// 的一个请求——她那边看到的是一个转了三分钟然后超时的圈。所以先出三张**文字**
// 候选（几秒），画像每张自己一个请求，界面上三张卡各转各的。

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

type pblPersonaDTO struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	WhyKnows string   `json:"whyKnows"`
	Wants    string   `json:"wants"`
	Feeling  string   `json:"feeling"`
	Keywords []string `json:"keywords"`
	// 画像地址。空 = 还没画，界面上是一块占位。
	PortraitURL string `json:"portraitUrl"`
	Chosen      bool   `json:"chosen"`
}

func (a *API) toPblPersonaDTO(p sqlc.PblPersona) pblPersonaDTO {
	kws := []string{}
	if len(p.Keywords) > 0 {
		_ = json.Unmarshal(p.Keywords, &kws)
	}
	return pblPersonaDTO{
		ID: p.ID.String(), Label: p.Label, WhyKnows: p.WhyKnows,
		Wants: p.Wants, Feeling: p.Feeling, Keywords: kws,
		PortraitURL: a.signedOrEmpty(p.PortraitKey), Chosen: p.Chosen,
	}
}

func (a *API) listPblPersonas(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListPblPersonas(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]pblPersonaDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, a.toPblPersonaDTO(row))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// generatePblPersonas —— 印记先动：从她真做过的事里推出两三个可能的读者。
//
// 整批重来是允许的（「都不像」）：删掉旧的，再生成一批。她否掉三个人本身就是
// 一次判断，值得让它便宜。
func (a *API) generatePblPersonas(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	material, err := a.studentMaterialFor(r, u.ID, atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if strings.TrimSpace(material) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_material",
			"生成失败：当前没有可用于分析目标读者的阅读、写作或项目记录", nil))
		return
	}

	resolved, rerr := a.routeE(r.Context(), gateway.ClassCompose)
	if rerr != nil {
		httpx.WriteError(w, r, rerr)
		return
	}
	cands, usage, gerr := pbl.GeneratePersonas(r.Context(), a.d.Provider, resolved, material)
	a.recordLiteLLMCall(r.Context(), u.ID, atomID, "pbl_personas", resolved, usage)
	if gerr != nil {
		slog.Warn("pbl personas: generation failed", "err", gerr,
			"request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrBadRequest("generate_failed", "生成失败："+gerr.Error(), nil))
		return
	}

	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)
	if _, err := qtx.LockAtom(r.Context(), atomID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	if err := qtx.DeletePblPersonasByAtom(r.Context(), atomID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]pblPersonaDTO, 0, len(cands))
	for _, c := range cands {
		kw, _ := json.Marshal(c.Keywords)
		row, err := qtx.CreatePblPersona(r.Context(), sqlc.CreatePblPersonaParams{
			AtomID: atomID, Label: c.Label, WhyKnows: c.WhyKnows,
			Wants: c.Wants, Feeling: c.Feeling, Keywords: kw,
		})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		dto := a.toPblPersonaDTO(row)
		out = append(out, dto)
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// 画像的提示词由印记写，但它不进库——每张画像自己一个请求，那时候重新按
	// 这一行的字段拼一句就够了。存一份只会多一列会和显示内容走散的东西。
	httpx.WriteJSON(w, http.StatusCreated, out)
}

// drawPblPersonaPortrait —— 给某一个候选画一张画像。一张一个请求，理由见文件头。
func (a *API) drawPblPersonaPortrait(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	pid, err := uuid.Parse(strings.TrimSpace(r.PathValue("pid")))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这个读者不存在"))
		return
	}
	row, err := a.d.Queries.GetPblPersona(r.Context(),
		sqlc.GetPblPersonaParams{AtomID: atomID, ID: pid})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这个读者不存在"))
		return
	}
	// 已经画过就直接给回去。重画是另一件事（她按「换一张」时先清空再来）。
	if row.PortraitKey != "" {
		httpx.WriteJSON(w, http.StatusOK, a.toPblPersonaDTO(row))
		return
	}

	kws := []string{}
	if len(row.Keywords) > 0 {
		_ = json.Unmarshal(row.Keywords, &kws)
	}
	prompt := pbl.PortraitPrompt(pbl.PersonaCandidate{
		Label: row.Label, Wants: row.Wants, Feeling: row.Feeling, Keywords: kws,
		Portrait: row.Label + "：" + row.WhyKnows,
	})
	key, derr := a.drawAndStore(r.Context(), u.ID, atomID, "persona", prompt)
	if derr != nil {
		slog.Warn("pbl personas: portrait failed", "err", derr,
			"request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrBadRequest("draw_failed", "生成失败："+derr.Error(), nil))
		return
	}
	updated, err := a.d.Queries.SetPblPersonaPortrait(r.Context(),
		sqlc.SetPblPersonaPortraitParams{AtomID: atomID, ID: pid, PortraitKey: key})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a.toPblPersonaDTO(updated))
}

// choosePblPersona —— 她留下的那一个，以及她留下的那些关键词。
//
// 关键词一起收，因为划掉几个词和留下这个人是**同一次判断**：她说的是「就是这个
// 读者，但不是那几个词」。分成两次提交，那句话就被拆散了。
func (a *API) choosePblPersona(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	pid, err := uuid.Parse(strings.TrimSpace(r.PathValue("pid")))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这个读者不存在"))
		return
	}
	var req struct {
		Keywords []string `json:"keywords"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	kept := make([]string, 0, len(req.Keywords))
	for _, k := range req.Keywords {
		if t := strings.TrimSpace(k); t != "" {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_keywords",
			"配色与结构将参考这些关键词，请至少保留一个", nil))
		return
	}

	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)
	if _, err := qtx.LockAtom(r.Context(), atomID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// 🚨 先清再置：唯一索引 pbl_persona_one_chosen 不允许两个同时为真。
	if err := qtx.ClearPblPersonaChosen(r.Context(), atomID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	kw, _ := json.Marshal(kept)
	if _, err := qtx.SetPblPersonaKeywords(r.Context(),
		sqlc.SetPblPersonaKeywordsParams{AtomID: atomID, ID: pid, Keywords: kw}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	row, err := qtx.ChoosePblPersona(r.Context(),
		sqlc.ChoosePblPersonaParams{AtomID: atomID, ID: pid})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这个读者不存在"))
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a.toPblPersonaDTO(row))
}

// studentMaterialFor 是喂给生成的那份「她真做过的事」。
//
// 🚨 用真材料，不用一句泛泛的自我介绍。凭空生成三个受众会得到三个模板人
// （「对你的领域感兴趣的同龄人」），而三个一样的候选，她挑哪个都一样——这一关
// 就退化成一次点击。
func (a *API) studentMaterialFor(r *http.Request, userID, atomID uuid.UUID) (string, error) {
	ctx := r.Context()
	var b strings.Builder

	row, err := a.ensureSite(r, userID)
	if err != nil {
		return "", err
	}
	u, _ := UserFromContext(ctx)
	content, err := a.loadSiteContent(r, userID, u.DisplayName, row)
	if err != nil {
		return "", err
	}
	for _, p := range content.Posts {
		b.WriteString("写过：" + p.Title + "\n")
	}
	for _, rd := range content.Reads {
		b.WriteString("读过：" + rd.Title + "\n")
	}
	for _, p := range content.Projects {
		b.WriteString("做过：" + p.Title + "\n")
	}

	// 她在这个项目里自己说过的话也算材料——第一关往往就发生在她刚说完
	// 「我想做个网站放我拆电器的事」之后。
	own, err := a.studentOwnWords(ctx, atomID)
	if err != nil {
		return "", err
	}
	if t := strings.TrimSpace(own); t != "" {
		b.WriteString("她自己说过：\n" + t + "\n")
	}
	return b.String(), nil
}

// createPblPersona lets a student define the reader without model suggestions.
// This also works for the first project, before any reading or writing exists.
func (a *API) createPblPersona(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	var in pbl.PersonaCandidate
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	in.Label = strings.TrimSpace(in.Label)
	in.Wants = strings.TrimSpace(in.Wants)
	keywords := []string{}
	seen := map[string]bool{}
	for _, word := range in.Keywords {
		word = strings.TrimSpace(word)
		if word != "" && !seen[word] {
			keywords = append(keywords, word)
			seen[word] = true
		}
		if len([]rune(word)) > 40 {
			httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_persona", "保存失败：关键词不能超过40字", nil))
			return
		}
	}
	if in.Label == "" || in.Wants == "" || len([]rune(in.Label)) > 100 || len([]rune(in.Wants)) > 1000 || len(keywords) < 1 || len(keywords) > 6 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_persona", "保存失败：请填写读者、展示内容和1至6个关键词", nil))
		return
	}
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	q := a.d.Queries.WithTx(tx)
	if _, err = q.LockAtom(r.Context(), atomID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	kw, _ := json.Marshal(keywords)
	row, err := q.CreatePblPersona(r.Context(), sqlc.CreatePblPersonaParams{AtomID: atomID, Label: in.Label, Wants: in.Wants, Feeling: strings.Join(keywords, "、"), Keywords: kw})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err = q.ClearPblPersonaChosen(r.Context(), atomID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	row, err = q.ChoosePblPersona(r.Context(), sqlc.ChoosePblPersonaParams{AtomID: atomID, ID: row.ID})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, a.toPblPersonaDTO(row))
}
