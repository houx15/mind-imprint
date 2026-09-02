package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// pbl_decide.go —— 做一个决定。
//
// ⚠️ 产品负责人 2026-09-01 的「make a decision」一节是空的，只有标题。以下是
// 我的设计，等她审；界面上的每一步都能改。
//
// 不做加权打分矩阵——那正是 2026-09-01 被否掉的表单。做的是四句话：
//
//	摆开选项 → 说清这里到底什么重要 → 每个选项赢在哪、疼在哪 → 选，并说为什么
//
// 最后还有一格，是这整件事的关键：**什么会让我改主意**。写得出这一句，这个
// 决定才是一个可以被后来的事实推翻的判断，而不是一次表态；复盘（阶段六）也
// 才有东西可回头看。
//
// 三道门槛都在 settle 上，而且都在服务端：
//
//  1. 至少两个选项 —— 一个选项不叫决定，叫已经定了。
//  2. 至少一条"什么重要" —— 这是最常被跳过的一步，跳过之后所谓的比较就只是
//     在挑一个看起来顺眼的。
//  3. 选择、为什么、什么会让我改主意，三样都不能空。

type pblOptionDTO struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Wins    string `json:"wins"`
	Hurts   string `json:"hurts"`
	Author  string `json:"author"`
	Ordinal int32  `json:"ordinal"`
}

type pblCriterionDTO struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Author  string `json:"author"`
	Ordinal int32  `json:"ordinal"`
}

type pblDecisionDTO struct {
	ID        string            `json:"id"`
	Subject   string            `json:"subject"`
	Choice    string            `json:"choice"`
	Why       string            `json:"why"`
	GaveUp    string            `json:"gaveUp"`
	Flip      string            `json:"flip"`
	SettledAt *string           `json:"settledAt"`
	Options   []pblOptionDTO    `json:"options"`
	Criteria  []pblCriterionDTO `json:"criteria"`
	CreatedAt string            `json:"createdAt"`
}

func toPblDecisionDTO(d sqlc.PblDecision, opts []sqlc.PblDecisionOption, crit []sqlc.PblDecisionCriterion) pblDecisionDTO {
	out := pblDecisionDTO{
		ID: d.ID.String(), Subject: d.Subject, Choice: d.Choice, Why: d.Why,
		GaveUp: d.GaveUp, Flip: d.Flip,
		Options: make([]pblOptionDTO, 0, len(opts)), Criteria: make([]pblCriterionDTO, 0, len(crit)),
		CreatedAt: d.CreatedAt.Format(time.RFC3339),
	}
	if d.SettledAt.Valid {
		s := d.SettledAt.Time.Format(time.RFC3339)
		out.SettledAt = &s
	}
	for _, o := range opts {
		out.Options = append(out.Options, pblOptionDTO{
			ID: o.ID.String(), Label: o.Label, Wins: o.Wins, Hurts: o.Hurts,
			Author: o.Author, Ordinal: o.Ordinal,
		})
	}
	for _, c := range crit {
		out.Criteria = append(out.Criteria, pblCriterionDTO{
			ID: c.ID.String(), Label: c.Label, Author: c.Author, Ordinal: c.Ordinal,
		})
	}
	return out
}

func (a *API) loadPblDecisionFull(r *http.Request, id uuid.UUID) (pblDecisionDTO, error) {
	d, err := a.d.Queries.GetPblDecision(r.Context(), id)
	if err != nil {
		return pblDecisionDTO{}, err
	}
	opts, err := a.d.Queries.ListPblDecisionOptions(r.Context(), id)
	if err != nil {
		return pblDecisionDTO{}, err
	}
	crit, err := a.d.Queries.ListPblDecisionCriteria(r.Context(), id)
	if err != nil {
		return pblDecisionDTO{}, err
	}
	return toPblDecisionDTO(sqlc.PblDecision{
		ID: d.ID, AtomID: d.AtomID, SessionID: d.SessionID, Subject: d.Subject,
		Choice: d.Choice, Why: d.Why, GaveUp: d.GaveUp, Flip: d.Flip,
		SettledAt: d.SettledAt, CreatedAt: d.CreatedAt,
	}, opts, crit), nil
}

// loadOwnedPblDecision —— 别人的决定和不存在的决定，对外长得一样。
func (a *API) loadOwnedPblDecision(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return uuid.Nil, false
	}
	did, err := uuid.Parse(r.PathValue("did"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这个决定不存在"))
		return uuid.Nil, false
	}
	row, err := a.d.Queries.GetPblDecision(r.Context(), did)
	if err != nil || row.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这个决定不存在"))
		return uuid.Nil, false
	}
	return did, true
}

func (a *API) listPblDecisions(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListPblDecisions(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]pblDecisionDTO, 0, len(rows))
	for _, d := range rows {
		full, ferr := a.loadPblDecisionFull(r, d.ID)
		if ferr != nil {
			httpx.WriteError(w, r, ferr)
			return
		}
		out = append(out, full)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// openPblDecision —— 摊开一个还没做出来的决定。
func (a *API) openPblDecision(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	var req struct {
		Subject   string `json:"subject"`
		SessionID string `json:"sessionId"`
		Options   []struct {
			Label  string `json:"label"`
			Wins   string `json:"wins"`
			Hurts  string `json:"hurts"`
			Author string `json:"author"`
		} `json:"options"`
		Criteria []struct {
			Label  string `json:"label"`
			Author string `json:"author"`
		} `json:"criteria"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	subject := strings.TrimSpace(req.Subject)
	if subject == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_subject", "这是在定什么？", nil))
		return
	}

	sess := pgtype.UUID{}
	if raw := strings.TrimSpace(req.SessionID); raw != "" {
		sid, perr := uuid.Parse(raw)
		if perr != nil {
			httpx.WriteError(w, r, httpx.ErrNotFound("这一层不存在"))
			return
		}
		s, gerr := a.d.Queries.GetPblSession(r.Context(), sid)
		if gerr != nil || s.AtomID != atomID {
			httpx.WriteError(w, r, httpx.ErrNotFound("这一层不存在"))
			return
		}
		sess = pgtype.UUID{Bytes: sid, Valid: true}
	}

	d, err := a.d.Queries.CreatePblDecision(r.Context(), sqlc.CreatePblDecisionParams{
		AtomID: atomID, SessionID: sess, Subject: subject,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	for i, o := range req.Options {
		if strings.TrimSpace(o.Label) == "" {
			continue
		}
		if _, err := a.d.Queries.CreatePblDecisionOption(r.Context(), sqlc.CreatePblDecisionOptionParams{
			DecisionID: d.ID, Label: strings.TrimSpace(o.Label),
			Wins: strings.TrimSpace(o.Wins), Hurts: strings.TrimSpace(o.Hurts),
			Author: authorOf(o.Author), Ordinal: int32(i),
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	for i, c := range req.Criteria {
		if strings.TrimSpace(c.Label) == "" {
			continue
		}
		if _, err := a.d.Queries.CreatePblDecisionCriterion(r.Context(), sqlc.CreatePblDecisionCriterionParams{
			DecisionID: d.ID, Label: strings.TrimSpace(c.Label),
			Author: authorOf(c.Author), Ordinal: int32(i),
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	full, err := a.loadPblDecisionFull(r, d.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, full)
}

// authorOf —— 默认算她写的。印记要认领自己写的东西，得明说。
func authorOf(s string) string {
	if strings.TrimSpace(s) == "yinji" {
		return "yinji"
	}
	return "student"
}

func (a *API) addPblDecisionOption(w http.ResponseWriter, r *http.Request) {
	did, ok := a.loadOwnedPblDecision(w, r)
	if !ok {
		return
	}
	var req struct {
		Label  string `json:"label"`
		Wins   string `json:"wins"`
		Hurts  string `json:"hurts"`
		Author string `json:"author"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	if strings.TrimSpace(req.Label) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_label", "这个选项是什么？", nil))
		return
	}
	existing, err := a.d.Queries.ListPblDecisionOptions(r.Context(), did)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := a.d.Queries.CreatePblDecisionOption(r.Context(), sqlc.CreatePblDecisionOptionParams{
		DecisionID: did, Label: strings.TrimSpace(req.Label),
		Wins: strings.TrimSpace(req.Wins), Hurts: strings.TrimSpace(req.Hurts),
		Author: authorOf(req.Author), Ordinal: int32(len(existing)),
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	full, err := a.loadPblDecisionFull(r, did)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, full)
}

func (a *API) updatePblDecisionOption(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	oid, err := uuid.Parse(r.PathValue("oid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这个选项不存在"))
		return
	}
	row, err := a.d.Queries.GetPblDecisionOption(r.Context(), oid)
	if err != nil || row.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这个选项不存在"))
		return
	}
	if row.SettledAt.Valid {
		httpx.WriteError(w, r, httpx.ErrBadRequest("already_settled", "这个决定已经定了", nil))
		return
	}
	var req struct {
		Label *string `json:"label"`
		Wins  *string `json:"wins"`
		Hurts *string `json:"hurts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	label, wins, hurts := row.Label, row.Wins, row.Hurts
	if req.Label != nil {
		label = strings.TrimSpace(*req.Label)
	}
	if req.Wins != nil {
		wins = strings.TrimSpace(*req.Wins)
	}
	if req.Hurts != nil {
		hurts = strings.TrimSpace(*req.Hurts)
	}
	if label == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_label", "这个选项是什么？", nil))
		return
	}
	if _, err := a.d.Queries.UpdatePblDecisionOption(r.Context(), sqlc.UpdatePblDecisionOptionParams{
		ID: oid, Label: label, Wins: wins, Hurts: hurts,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	full, err := a.loadPblDecisionFull(r, row.DecisionID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, full)
}

func (a *API) addPblDecisionCriterion(w http.ResponseWriter, r *http.Request) {
	did, ok := a.loadOwnedPblDecision(w, r)
	if !ok {
		return
	}
	var req struct {
		Label  string `json:"label"`
		Author string `json:"author"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	if strings.TrimSpace(req.Label) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_label", "这一条是什么？", nil))
		return
	}
	existing, err := a.d.Queries.ListPblDecisionCriteria(r.Context(), did)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := a.d.Queries.CreatePblDecisionCriterion(r.Context(), sqlc.CreatePblDecisionCriterionParams{
		DecisionID: did, Label: strings.TrimSpace(req.Label),
		Author: authorOf(req.Author), Ordinal: int32(len(existing)),
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	full, err := a.loadPblDecisionFull(r, did)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, full)
}

func (a *API) deletePblDecisionCriterion(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一条不存在"))
		return
	}
	row, err := a.d.Queries.GetPblDecisionCriterion(r.Context(), cid)
	if err != nil || row.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一条不存在"))
		return
	}
	if err := a.d.Queries.DeletePblDecisionCriterion(r.Context(), cid); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	full, err := a.loadPblDecisionFull(r, row.DecisionID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, full)
}

// settlePblDecision —— 定下来。三道门槛都在这里。
func (a *API) settlePblDecision(w http.ResponseWriter, r *http.Request) {
	did, ok := a.loadOwnedPblDecision(w, r)
	if !ok {
		return
	}
	var req struct {
		Choice string `json:"choice"`
		Why    string `json:"why"`
		GaveUp string `json:"gaveUp"`
		Flip   string `json:"flip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}

	opts, err := a.d.Queries.ListPblDecisionOptions(r.Context(), did)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// 🚨 一个选项不叫决定，叫已经定了。
	if len(opts) < 2 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("too_few_options",
			"至少要有两个选项才谈得上选", nil))
		return
	}
	crit, err := a.d.Queries.ListPblDecisionCriteria(r.Context(), did)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// 🚨 最常被跳过的一步。跳过之后，所谓的比较只是在挑一个看起来顺眼的。
	if len(crit) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_criteria",
			"先说清楚这件事上什么最重要", nil))
		return
	}

	choice, why, flip := strings.TrimSpace(req.Choice), strings.TrimSpace(req.Why), strings.TrimSpace(req.Flip)
	missing := []string{}
	if choice == "" {
		missing = append(missing, "选哪个")
	}
	if why == "" {
		missing = append(missing, "为什么")
	}
	// 🚨 这一格是整件事的关键：写得出什么会推翻它，这才是一个判断而不是一次
	// 表态，复盘的时候也才有东西可以回头看。
	if flip == "" {
		missing = append(missing, "什么会让你改主意")
	}
	if len(missing) > 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("incomplete",
			"还差："+strings.Join(missing, "、"), nil))
		return
	}

	if _, err := a.d.Queries.SettlePblDecision(r.Context(), sqlc.SettlePblDecisionParams{
		ID: did, Choice: choice, Why: why,
		GaveUp: strings.TrimSpace(req.GaveUp), Flip: flip,
	}); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("already_settled", "这个决定已经定过了", nil))
		return
	}
	full, err := a.loadPblDecisionFull(r, did)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, full)
}
