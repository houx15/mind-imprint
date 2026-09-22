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
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	// 🚨 谁提的这条路。印记提的是默认，她自己加的那条是另一回事——
	// 「这些都不对，我要的是另一样」是她判断力的证据（铁律④），界面和回灌
	// 都要认得出来。
	Author string `json:"author"`
	// 她排的名次，1 是第一。0 = 还没排过。
	StudentRank int32 `json:"studentRank"`
	Ordinal     int32 `json:"ordinal"`
}

type pblCriterionDTO struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Author string `json:"author"`
	// 她排的名次，1 是第一。0 = 还没排过。
	StudentRank int32 `json:"studentRank"`
	Ordinal     int32 `json:"ordinal"`
}

type pblDecisionDTO struct {
	ContentVersion  int32           `json:"contentVersion"`
	RevisionHistory json.RawMessage `json:"revisionHistory"`
	ID              string          `json:"id"`
	Subject         string          `json:"subject"`
	Choice          string          `json:"choice"`
	// 为什么选它 / 为什么不选别的——确认时要回答的那两个小问题。
	Why    string `json:"why"`
	WhyNot string `json:"whyNot"`
	// 什么情况会让她改主意。复盘时拿它对照——当初写下的那个条件，后来发生了没有。
	Flip      string            `json:"flip"`
	SettledAt *string           `json:"settledAt"`
	Options   []pblOptionDTO    `json:"options"`
	Criteria  []pblCriterionDTO `json:"criteria"`
	CreatedAt string            `json:"createdAt"`
}

func toPblDecisionDTO(d sqlc.PblDecision, opts []sqlc.PblDecisionOption, crit []sqlc.PblDecisionCriterion) pblDecisionDTO {
	out := pblDecisionDTO{
		ContentVersion: d.ContentVersion, RevisionHistory: d.RevisionHistory,
		ID: d.ID.String(), Subject: d.Subject, Choice: d.Choice,
		Why: d.Why, WhyNot: d.WhyNot, Flip: d.Flip,
		Options: make([]pblOptionDTO, 0, len(opts)), Criteria: make([]pblCriterionDTO, 0, len(crit)),
		CreatedAt: d.CreatedAt.Format(time.RFC3339),
	}
	if d.SettledAt.Valid {
		s := d.SettledAt.Time.Format(time.RFC3339)
		out.SettledAt = &s
	}
	for _, o := range opts {
		out.Options = append(out.Options, pblOptionDTO{
			ID: o.ID.String(), Label: o.Label, Description: o.Description,
			Author: o.Author, StudentRank: o.StudentRank, Ordinal: o.Ordinal,
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
		Choice: d.Choice, Why: d.Why, WhyNot: d.WhyNot, GaveUp: d.GaveUp,
		Flip: d.Flip, SettledAt: d.SettledAt, CreatedAt: d.CreatedAt,
		ContentVersion: d.ContentVersion, RevisionHistory: d.RevisionHistory,
	}, opts, crit), nil
}

// addPblDecisionOption —— 她自己往里加一条路。
//
// 🚨 印记给的三条不是全集。「在别人摆好的选项里挑一个」和「决定」是两回事——
// 后者包含「这些都不对，我要的是另一样」。`pbl_decision_option.author` 的 CHECK
// 本来就允许 student，只是 openPblDecision 把它写死成了 yinji，于是这条路一直
// 关着（铁律①：她判断，不是她挑）。
//
// 已经拍板的决定不能再加：那是在改一件已经做完的事。
func (a *API) addPblDecisionOption(w http.ResponseWriter, r *http.Request) {
	did, ok := a.loadOwnedPblDecision(w, r)
	if !ok {
		return
	}
	var req struct {
		Label       string `json:"label"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	label := strings.TrimSpace(req.Label)
	if label == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_label", "请填写选项名称", nil))
		return
	}
	d, err := a.d.Queries.GetPblDecision(r.Context(), did)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if d.SettledAt.Valid {
		httpx.WriteError(w, r, httpx.ErrBadRequest("settled",
			"这个决定已经定了，不能再加选项", nil))
		return
	}
	opts, err := a.d.Queries.ListPblDecisionOptions(r.Context(), did)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := a.d.Queries.CreatePblDecisionOption(r.Context(), sqlc.CreatePblDecisionOptionParams{
		DecisionID: did, Label: label, Description: strings.TrimSpace(req.Description),
		Author: "student", Ordinal: int32(len(opts)),
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	dto, err := a.loadPblDecisionFull(r, did)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, dto)
}

// rankPblDecisionOptions —— 她把几条路排出来的顺序。
//
// 🚨 只挑一个不需要把它们放在一起比：读到顺眼的那张就点了。排成一列才需要——
// 「B 比 C 好在哪」是一个她必须真的想过才答得出的问题。名次也让后面那句
// 「输给第一名的地方」问得具体，而不是一句泛泛的「为什么不选别的」。
//
// 一次收全部：名次是个整体，逐条发会在中途留下两个第一名。
func (a *API) rankPblDecisionOptions(w http.ResponseWriter, r *http.Request) {
	did, ok := a.loadOwnedPblDecision(w, r)
	if !ok {
		return
	}
	var req struct {
		// 按名次从高到低排好的选项 id。
		Order []string `json:"order"`
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
	belongs := map[uuid.UUID]bool{}
	for _, o := range opts {
		belongs[o.ID] = true
	}
	for i, raw := range req.Order {
		oid, perr := uuid.Parse(strings.TrimSpace(raw))
		// 不属于这个决定的 id 一律不认——排序不该成为改别人数据的一条路。
		if perr != nil || !belongs[oid] {
			httpx.WriteError(w, r, httpx.ErrBadRequest("bad_option", "这个选项不在这个决定里", nil))
			return
		}
		if _, err := a.d.Queries.RankPblDecisionOption(r.Context(),
			sqlc.RankPblDecisionOptionParams{ID: oid, StudentRank: int32(i + 1)}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	dto, err := a.loadPblDecisionFull(r, did)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
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
			Label       string `json:"label"`
			Description string `json:"description"`
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
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_subject", "请填写决策主题", nil))
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
			Description: strings.TrimSpace(o.Description),
			// 选项是印记提的，所以作者恒为 yinji。
			Author: "yinji", Ordinal: int32(i),
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

func (a *API) settlePblDecision(w http.ResponseWriter, r *http.Request) {
	did, ok := a.loadOwnedPblDecision(w, r)
	if !ok {
		return
	}
	var req struct {
		ContentVersion int32  `json:"contentVersion"`
		Choice         string `json:"choice"`
		Why            string `json:"why"`
		WhyNot         string `json:"whyNot"`
		// Flip 是「什么情况会让你改主意」。
		//
		// 🚨 不是必填。一个决定不写翻盘条件也仍然是个决定；把它设成门槛，只会
		// 逼出一句应付的话。写了才有意义，所以留给她自己决定写不写（铁律④，
		// 不写也是信号）。
		Flip string `json:"flip"`
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
			"请提供至少两个选项以便比较", nil))
		return
	}
	choice, why, whyNot := strings.TrimSpace(req.Choice),
		strings.TrimSpace(req.Why), strings.TrimSpace(req.WhyNot)
	flip := strings.TrimSpace(req.Flip)
	missing := []string{}
	if choice == "" {
		missing = append(missing, "选哪个")
	}
	if why == "" {
		missing = append(missing, "为什么选它")
	}
	// 🚨 这一句是这件工具真正教的东西。选中一个不难；说得出为什么放掉另外几个，
	// 才说明她真的把它们放在一起比过。
	if whyNot == "" {
		missing = append(missing, "为什么不选别的")
	}
	if len(missing) > 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("incomplete",
			"还差："+strings.Join(missing, "、"), nil))
		return
	}

	if _, err := a.d.Queries.SettlePblDecision(r.Context(), sqlc.SettlePblDecisionParams{
		ID: did, Choice: choice, Why: why, WhyNot: whyNot, Flip: flip, ContentVersion: req.ContentVersion,
	}); err != nil {
		httpx.WriteError(w, r, httpx.ErrConflict("决定已确认或选项已修订，请重新打开核对后确认"))
		return
	}
	full, err := a.loadPblDecisionFull(r, did)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, full)
}
