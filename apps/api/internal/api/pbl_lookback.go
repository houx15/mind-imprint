package api

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

// pbl_lookback.go —— 复盘。
//
// 产品负责人 2026-09-01 只说了一句：「it can be a form.」
//
// 做成表单可以，但**不能是一张空格**。2026-09-01 被否掉的正是"没人喜欢那种
// 表单式的东西"，而一张问「你学到了什么」的空表是同一件事的另一个样子。
//
// 所以这些问题是**从这个项目真的发生过的事里长出来的**：她定过的那些决定、
// 她改写过的问题、她退回去的成果。每一问都指着一件具体的事，她回答的时候
// 面对的是自己当时写下的那句话，不是一个抽象的提示。
//
// 🚨 生成只做一次。第二次打开还重新生成，她答过的东西就会被冲掉——而复盘这
// 件事本来就是隔几天回来慢慢写的。

// pblStances —— 她对自己当初那句话现在的看法。
//
// 🚨 「当时没想清楚」必须是其中一个：在空白框里承认这件事要写一段话，成本太高，
// 她于是写「挺好的」。一个可点的态度只要一下，诚实因此变便宜（铁律④）。
var pblStances = map[string]bool{
	"still":   true, // 现在仍这么想
	"changed": true, // 现在会改
	"unclear": true, // 当时没想清楚
}

type pblLookbackDTO struct {
	Revision int32  `json:"revision"`
	ID       string `json:"id"`
	// 六段之一：what / how / moment / praise / improve / with_ai。
	Section string `json:"section"`
	Prompt  string `json:"prompt"`
	Answer  string `json:"answer"`
	// Evidence 是这一问冲着的那件事——她当初写下的原话。空 = 这一问是冲着她
	// 本人问的（「感受如何」那一段）。
	Evidence string `json:"evidence"`
	// 她现在怎么看当初那句话：still / changed / unclear，空 = 没表态。
	Stance  string `json:"stance"`
	Ordinal int32  `json:"ordinal"`
}

func toPblLookbackDTO(p sqlc.PblReview) pblLookbackDTO {
	return pblLookbackDTO{
		Revision: p.Revision,
		ID:       p.ID.String(), Section: p.Section, Prompt: p.Prompt,
		Answer: p.Answer, Evidence: p.AnchorRef, Stance: p.Stance, Ordinal: p.Ordinal,
	}
}

// gatherPblLookback 把这个项目真发生过的事收集起来，交给印记去写问题。
//
// 收集在这里做、写在 internal/pbl/lookback.go 做：能拿到什么是数据库的事，
// 问什么是印记的事。
func (a *API) gatherPblLookback(r *http.Request, atomID uuid.UUID) (pbl.LookbackInput, error) {
	in := pbl.LookbackInput{}
	p, err := a.d.Queries.GetPblProject(r.Context(), atomID)
	if err != nil {
		return in, err
	}
	in.Idea, in.Name, in.Kind = p.Idea, p.Name, p.Kind
	in.Assigned, in.AssignedBrief = p.Assigned, derefOr(p.AssignedBrief, "")
	if in.Assigned {
		in.AssignedBrief, err = a.pblAssignmentBrief(r, atomID, in.AssignedBrief)
		if err != nil {
			return in, err
		}
	}

	personas, err := a.d.Queries.ListPblPersonas(r.Context(), atomID)
	if err != nil {
		return in, err
	}
	for _, persona := range personas {
		if persona.Chosen {
			in.Process = append(in.Process, "已选择的受众："+persona.Label+"；展示目标："+persona.Wants)
		}
	}
	refs, err := a.d.Queries.ListPblSiteRefs(r.Context(), atomID)
	if err != nil {
		return in, err
	}
	for _, ref := range refs {
		if strings.TrimSpace(ref.SheSaid) != "" {
			in.Process = append(in.Process, "学生对参考网站的取舍："+ref.SheSaid)
		}
	}
	messages, err := a.d.Queries.ListPblMainThread(r.Context(), atomID)
	if err != nil {
		return in, err
	}
	// Keep recent authored evidence bounded; never treat AI success prose as an
	// executed action. Tool/system receipts and student words remain labelled.
	var recent []string
	for _, message := range messages {
		if message.Role == "student" || message.Role == "system" {
			recent = append(recent, message.Role+"原始记录："+message.Content)
		}
	}
	if len(recent) > 30 {
		recent = recent[len(recent)-30:]
	}
	in.Process = append(in.Process, recent...)

	if v, verr := a.d.Queries.GetPblLivePlan(r.Context(), atomID); verr == nil {
		if steps, serr := a.d.Queries.ListPblPlanSteps(r.Context(), v.ID); serr == nil {
			plan := pblPlanDTO{}
			for _, st := range steps {
				plan.Steps = append(plan.Steps, toPblStepDTO(st))
			}
			a.attachHomepageProgress(r, atomID, &plan)
			for _, st := range plan.Steps {
				status := map[string]string{"todo": "待开始", "doing": "进行中", "done": "已完成"}[st.Progress]
				if status != "" {
					in.Steps = append(in.Steps, st.Title+"（"+status+"）")
				} else {
					in.Steps = append(in.Steps, st.Title)
				}
			}
		}
	}
	if rs, rerr := a.d.Queries.ListPblReframes(r.Context(), atomID); rerr == nil {
		for _, x := range rs {
			if x.ConfirmedAt.Valid {
				in.Reframes = append(in.Reframes,
					x.Who+" 需要 "+x.Needs+"，因为 "+x.Why)
			}
		}
	}
	if ds, derr := a.d.Queries.ListPblDecisions(r.Context(), atomID); derr == nil {
		for _, d := range ds {
			if !d.SettledAt.Valid {
				continue
			}
			line := "关于「" + d.Subject + "」选了「" + d.Choice + "」，因为" + d.Why
			if strings.TrimSpace(d.WhyNot) != "" {
				line += "；没选别的是因为" + d.WhyNot
			}
			in.Decisions = append(in.Decisions, line)
		}
	}
	if as, aerr := a.d.Queries.ListPblArtifacts(r.Context(), atomID); aerr == nil {
		for _, x := range as {
			if x.Verdict == nil {
				continue
			}
			word := map[string]string{"kept": "通过了", "revise": "要求修改", "dropped": "打回重做"}[*x.Verdict]
			title := x.Title
			if strings.TrimSpace(title) == "" {
				title = "印记交的一份东西"
			}
			line := "《" + title + "》" + word
			if strings.TrimSpace(x.Why) != "" {
				line += "，理由是「" + x.Why + "」"
			}
			in.Artifacts = append(in.Artifacts, line)
		}
	}
	if ks, kerr := a.d.Queries.ListPblKeepEntries(r.Context(), atomID); kerr == nil {
		for _, k := range ks {
			in.Keeps = append(in.Keeps, k.Body)
		}
	}
	return in, nil
}

func (a *API) getPblLookback(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	existing, err := a.d.Queries.ListPblReviewPrompts(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// 🚨 只生成一次。再生成一遍会把她答过的冲掉，而复盘本来就是隔几天回来
	// 慢慢写的。
	regenerate := r.Method == http.MethodPost
	if len(existing) == 0 || regenerate {
		var baseRevision int32
		for _, p := range existing {
			if p.Revision > baseRevision {
				baseRevision = p.Revision
			}
		}
		u, _ := UserFromContext(r.Context())
		in, gerr := a.gatherPblLookback(r, atomID)
		if gerr != nil {
			httpx.WriteError(w, r, gerr)
			return
		}
		resolved, rok := a.route(r.Context(), gateway.ClassAssess)
		if !rok {
			httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
			return
		}
		qs, usage, qerr := pbl.GenerateLookback(r.Context(), a.d.Provider, resolved, in)
		a.recordLiteLLMCall(r.Context(), u.ID, atomID, "pbl_lookback", resolved, usage)
		if qerr != nil {
			// 🚨 不兜底成一份通用问卷。她会照着答完，然后以为自己复盘过了——
			// 那比没有复盘更糟。
			slog.Warn("pbl lookback: could not write the questions; surfacing",
				"err", qerr, "atom_id", atomID,
				"request_id", httpx.RequestIDFromContext(r.Context()))
			httpx.WriteError(w, r, httpx.ErrAIDialogueFailed(qerr.Error()))
			return
		}
		// 🚨 落库前在事务里加锁复查一遍。
		//
		// 两个请求可能同时看到零行（StrictMode 的二次挂载就会），于是各自生成
		// 一套。锁在这里而不是在模型调用外面：一次生成要几十秒，攥着数据库连接
		// 和行锁等模型，比多花一次 token 糟得多。所以让它们都生成，但只有一个
		// 插得进去，另一个把自己那套丢掉——数据永远只有一套。
		tx, terr := a.d.Pool.Begin(r.Context())
		if terr != nil {
			httpx.WriteError(w, r, terr)
			return
		}
		defer func() { _ = tx.Rollback(r.Context()) }()
		qtx := a.d.Queries.WithTx(tx)
		if _, lerr := qtx.LockAtom(r.Context(), atomID); lerr != nil {
			httpx.WriteError(w, r, lerr)
			return
		}
		again, aerr := qtx.ListPblReviewPrompts(r.Context(), atomID)
		if aerr != nil {
			httpx.WriteError(w, r, aerr)
			return
		}
		var latestRevision int32
		for _, p := range again {
			if p.Revision > latestRevision {
				latestRevision = p.Revision
			}
		}
		if latestRevision > baseRevision || (!regenerate && len(again) > 0) {
			existing = again
		} else {
			existing = again
			for i, q := range qs {
				row, cerr := qtx.CreatePblReviewPrompt(r.Context(), sqlc.CreatePblReviewPromptParams{
					Revision: baseRevision + 1,
					AtomID:   atomID, Prompt: q.Prompt, Section: q.Section,
					// 🚨 这一问是冲着哪件事去的。原来这里恒是 free/""——两列白摆着，
					// 而复盘因此变回了一张放到任何项目上都成立的感想表。
					AnchorKind: anchorKindOf(q.Evidence), AnchorRef: q.Evidence,
					Ordinal: int32(i),
				})
				if cerr != nil {
					httpx.WriteError(w, r, cerr)
					return
				}
				existing = append(existing, row)
			}
			if cerr := tx.Commit(r.Context()); cerr != nil {
				httpx.WriteError(w, r, cerr)
				return
			}
		}
	}
	out := make([]pblLookbackDTO, 0, len(existing))
	for _, p := range existing {
		out = append(out, toPblLookbackDTO(p))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (a *API) answerPblLookback(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	lid, err := uuid.Parse(r.PathValue("lid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一问不存在"))
		return
	}
	row, err := a.d.Queries.GetPblReviewPrompt(r.Context(), lid)
	if err != nil || row.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一问不存在"))
		return
	}
	var req struct {
		Answer *string `json:"answer"`
		// 现在还这么想吗：still / changed / unclear。空 = 她没表态。
		Stance *string `json:"stance"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	// 只在这一轮带了 stance 时才动它——她先答文字、后点态度（或反过来）都不该
	// 把另一样清掉。
	stance := row.Stance
	if req.Stance != nil {
		s := strings.TrimSpace(*req.Stance)
		if s != "" && !pblStances[s] {
			httpx.WriteError(w, r, httpx.ErrBadRequest("bad_stance", "不认识这种表态", nil))
			return
		}
		stance = s
	}
	answer := row.Answer
	if req.Answer != nil {
		answer = strings.TrimSpace(*req.Answer)
	}
	out, err := a.d.Queries.AnswerPblReviewPrompt(r.Context(), sqlc.AnswerPblReviewPromptParams{
		ID: lid, Answer: answer, Stance: stance,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPblLookbackDTO(out))
}

// anchorKindOf 说这一问有没有落在一件真事上。
//
// evidence 空 = 这一问是冲着她本人问的（「感受如何」那一段就该这样），不是
// 冲着某件事——那仍然是合法的一问，只是没有可摆出来的出处。
func anchorKindOf(evidence string) string {
	if strings.TrimSpace(evidence) == "" {
		return "free"
	}
	return "evidence"
}
