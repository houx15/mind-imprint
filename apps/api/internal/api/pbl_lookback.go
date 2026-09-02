package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

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

type pblLookbackDTO struct {
	ID string `json:"id"`
	// 六段之一：what / how / moment / praise / improve / with_ai。
	Section string `json:"section"`
	Prompt  string `json:"prompt"`
	Answer  string `json:"answer"`
	Ordinal int32  `json:"ordinal"`
}

func toPblLookbackDTO(p sqlc.PblReview) pblLookbackDTO {
	return pblLookbackDTO{
		ID: p.ID.String(), Section: p.Section, Prompt: p.Prompt,
		Answer: p.Answer, Ordinal: p.Ordinal,
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
	in.Idea, in.Name = p.Idea, p.Name

	if v, verr := a.d.Queries.GetPblLivePlan(r.Context(), atomID); verr == nil {
		if steps, serr := a.d.Queries.ListPblPlanSteps(r.Context(), v.ID); serr == nil {
			for _, st := range steps {
				in.Steps = append(in.Steps, st.Title+"（"+st.Status+"）")
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
	if len(existing) == 0 {
		u, _ := UserFromContext(r.Context())
		in, gerr := a.gatherPblLookback(r, atomID)
		if gerr != nil {
			httpx.WriteError(w, r, gerr)
			return
		}
		resolved, rok := a.resolveEval(r.Context())
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
			httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("lookback_failed"))
			return
		}
		for i, q := range qs {
			row, cerr := a.d.Queries.CreatePblReviewPrompt(r.Context(), sqlc.CreatePblReviewPromptParams{
				AtomID: atomID, Prompt: q.Prompt, Section: q.Section,
				AnchorKind: "free", AnchorRef: "", Ordinal: int32(i),
			})
			if cerr != nil {
				httpx.WriteError(w, r, cerr)
				return
			}
			existing = append(existing, row)
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
		Answer string `json:"answer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	out, err := a.d.Queries.AnswerPblReviewPrompt(r.Context(), sqlc.AnswerPblReviewPromptParams{
		ID: lid, Answer: strings.TrimSpace(req.Answer),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPblLookbackDTO(out))
}
