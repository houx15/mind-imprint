package api

// interest_proposal.go —— 读完一篇之后提出来的候选词，等她自己认。
//
// # 这一步补的是什么
//
// 采集这条路一直是**默默种上去**的：她读完一篇，后台抽出几个领域，树上就多了
// 几个词。产品负责人 2026-09-07 指出这不对 —— 一棵标着「这就是你的模型」的树，
// 应该由她点过头。于是采集分成两路（见 `landHarvest`）：她已经有的词照旧添一条
// 来源，**新词进 interest_proposal，摆到报告上等她认**。
//
// # 拒绝也要落库
//
// 「不要」不是把这一条丢掉，是把它记成 accepted=false。铁律④：她拒绝了什么，
// 和她认下了什么一样是过程数据。而且这样重跑一次采集不会把同一个词再问一遍。
//
// # 决定只做一次
//
// `DecideInterestProposal` 带 `decided_at IS NULL`，所以第二次点是空转（404）。
// 认下去会真的往树上种一个词，那件事没有撤销 —— 界面必须把这句话说在前面，
// 而不是画一个可以来回切的开关。

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/interest"
	"mindimprint/api/internal/interests"
	"mindimprint/api/internal/store/sqlc"
)

type interestProposalDTO struct {
	InterestID string `json:"interestId"`
	Zh         string `json:"zh"`
	En         string `json:"en"`
	Field      string `json:"field"`
	Note       string `json:"note"`
	// Evidence 是她自己写的那一句。她要看着它决定认不认 —— 一个说不出理由的
	// 候选词，和一句「猜你喜欢」没有区别。
	Evidence string `json:"evidence"`
	Decided  bool   `json:"decided"`
	Accepted bool   `json:"accepted"`
}

// loadOwnedAtomAny 取一个属于她的 atom，不限类型、也不管它做完了没有。
//
// 和 readings.go 那个 `loadOwnedAtom` 不是一回事：那个要指定 kind，而且对非 GET
// 会挡掉已经完成的 atom —— 报告和候选词恰恰只出现在**已经完成**的 atom 上，
// 用那一个会把这条路整个挡住。
func (a *API) loadOwnedAtomAny(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return uuid.Nil, false
	}
	id, err := uuid.Parse(r.PathValue("atomId"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_atom_id", "编号无效", nil))
		return uuid.Nil, false
	}
	row, err := a.d.Queries.GetAtomOwnerAndKind(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, httpx.ErrNotFound("没有这一篇"))
			return uuid.Nil, false
		}
		httpx.WriteError(w, r, err)
		return uuid.Nil, false
	}
	// 别人的东西一律当成不存在，不告诉她「有，但不是你的」。
	if row.UserID != u.ID {
		httpx.WriteError(w, r, httpx.ErrNotFound("没有这一篇"))
		return uuid.Nil, false
	}
	return id, true
}

// getInterestProposals —— GET /api/v1/interest/proposals/{atomId}
//
// `pending` 说的是「采集还没跑完」，不是「还有没决定的候选」。这两件事在界面上
// 完全不同：前者要显示「处理中」并且过一会儿再来问一次，后者是摆出来等她点。
// 分不清的话，一篇采不出词的阅读会永远转圈。
func (a *API) getInterestProposals(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	atomID, ok := a.loadOwnedAtomAny(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	stamped, err := a.d.Queries.GetAtomInterestHarvestedAt(ctx, atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rows, err := a.d.Queries.ListInterestProposals(ctx, sqlc.ListInterestProposalsParams{
		UserID: u.ID, AtomID: atomID,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	out := make([]interestProposalDTO, 0, len(rows))
	for _, p := range rows {
		// 中文名、英文名、主枝全部查词表得到 —— 库里只存 id。词表里删掉一条词
		// 之后，这一行读出来就是一个没有名字的 id，摆出来是一个她读不懂的按钮，
		// 所以直接不摆。
		it, ok := interests.ByID(p.InterestID)
		if !ok {
			continue
		}
		out = append(out, interestProposalDTO{
			InterestID: p.InterestID,
			Zh:         it.Zh,
			En:         it.En,
			Field:      it.Field,
			Note:       p.Note,
			Evidence:   p.Evidence,
			Decided:    p.DecidedAt.Valid,
			Accepted:   p.Accepted != nil && *p.Accepted,
		})
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"proposals": out,
		"pending":   harvestStillRunning(stamped, len(out)),
	})
}

// harvestLandingGrace —— 盖了章之后，还要等多久才承认「这一篇真的没有词」。
const harvestLandingGrace = 90 * time.Second

// harvestStillRunning —— 这一篇的采集跑完了没有。
//
// 🚨 光看 `interest_harvested_at` 是不够的：那个章**盖在模型调用之前**
// （harvestOneAtom 里写着为什么 —— 调用失败也要算尝试过，否则每次打开树都重发
// 一次调用）。也就是说盖章到词落库之间有一两秒，这段时间里库里的样子是
// 「采过了，而且一个词都没有」，和「这篇很薄，真的采不出词」一模一样。
//
// 2026-09-08 的全链路走查逐毫秒抓到了这件事：
//
//	盖章            08:44:58.157
//	她这一次来问     08:44:58.226   ← 70 毫秒之后
//	词落库          08:44:59.613   ← 又过了 1.46 秒
//
// 报告那一节读到「不 pending 且零条」就整节不显示，而且不再问第二次。于是
// 「电池」这个词好端端地在库里，她永远看不到，树上也永远不会有它 —— 而地图上
// 写着「读完之后，报告上会提出可以加进你树里的词」。两次走查都落在这个窗口里，
// 它不是偶发。
//
// 所以：刚盖章不久而且还没有词，就照实说「还在跑」，让她那一页再问一次。过了
// 这段时间还是零条，那才是真的没有词，那一节按原设计不显示。
func harvestStillRunning(stamped pgtype.Timestamptz, found int) bool {
	if !stamped.Valid {
		return true // 还没人开始采
	}
	return found == 0 && time.Since(stamped.Time) < harvestLandingGrace
}

// decideInterestProposal —— POST /api/v1/interest/proposals/{atomId}/{interestId}
//
// body: {"accept": true}
//
// 认下去会真的种一个词，走 plantKeywords 的同一条路（kind 用这个 atom 的类型），
// evidence 就是她刚看着的那一句。不认也落库（accepted=false）。
func (a *API) decideInterestProposal(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	atomID, ok := a.loadOwnedAtomAny(w, r)
	if !ok {
		return
	}
	interestID := r.PathValue("interestId")
	if !interests.Exists(interestID) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("unknown_interest", "没有这个领域。",
			map[string]any{"id": interestID}))
		return
	}
	var req struct {
		Accept bool `json:"accept"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	ctx := r.Context()

	row, err := a.d.Queries.DecideInterestProposal(ctx, sqlc.DecideInterestProposalParams{
		UserID: u.ID, AtomID: atomID, InterestID: interestID,
		Accepted: &req.Accept,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// 没有这一条，或者已经决定过了。两种都不是错误状态里她能做的事，
			// 说清楚哪一种。
			httpx.WriteError(w, r, httpx.ErrNotFound("这个词没有在等你决定"))
			return
		}
		httpx.WriteError(w, r, err)
		return
	}

	if req.Accept {
		at, err := a.d.Queries.GetAtomOwnerAndKind(ctx, atomID)
		kind := "reading"
		if err == nil {
			kind = at.Kind
		}
		// 标题是「相关活动」那一行显示的字，从这个 atom 自己取。
		title, _ := a.gatherHarvestText(ctx, atomID, kind)
		a.plantKeywords(ctx, u.ID, kind, atomID, title, []interest.Harvested{{
			InterestID: interestID,
			Note:       row.Note,
			Evidence:   row.Evidence,
		}})
	}
	w.WriteHeader(http.StatusNoContent)
}
