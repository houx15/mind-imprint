package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
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
	ID         string `json:"id"`
	Prompt     string `json:"prompt"`
	AnchorKind string `json:"anchorKind"`
	AnchorRef  string `json:"anchorRef"`
	Answer     string `json:"answer"`
	Ordinal    int32  `json:"ordinal"`
}

func toPblLookbackDTO(p sqlc.PblReview) pblLookbackDTO {
	return pblLookbackDTO{
		ID: p.ID.String(), Prompt: p.Prompt, AnchorKind: p.AnchorKind,
		AnchorRef: p.AnchorRef, Answer: p.Answer, Ordinal: p.Ordinal,
	}
}

type pblLookbackSeed struct {
	prompt     string
	anchorKind string
	anchorRef  string
}

// buildPblLookback —— 把这个项目发生过的事变成问题。
//
// 顺序是有意的：先问她改过主意的地方（问题被重新框定），再问她做过的判断，
// 再问她退回去的东西，最后才问下一次。前面三类都指着一件具体的事；只有最后
// 一问是开放的，而那时候她已经把三件具体的事想过一遍了。
func buildPblLookback(
	reframes []sqlc.PblReframe,
	decisions []sqlc.PblDecision,
	artifacts []sqlc.PblArtifact,
) []pblLookbackSeed {
	out := []pblLookbackSeed{}

	// 她改写过问题——这门课最想让她看见的一件事。
	for _, r := range reframes {
		if !r.Supersedes.Valid || r.ConfirmedAt.Valid == false {
			continue
		}
		out = append(out, pblLookbackSeed{
			prompt: fmt.Sprintf(
				"你后来把问题改成了「%s 需要 %s」。是什么让你改的？", r.Who, r.Needs),
			anchorKind: "reframe", anchorRef: r.ID.String(),
		})
	}

	// 她做过的判断，以及她当时写下的"什么会让我改主意"。
	for _, d := range decisions {
		if !d.SettledAt.Valid {
			continue
		}
		// 她当时写下的「为什么不选别的」，现在原样问回去。做那个决定时想清楚
		// 放掉了什么，到这里才兑现——不然那一句就只是一次填空。
		if strings.TrimSpace(d.WhyNot) != "" {
			out = append(out, pblLookbackSeed{
				prompt: fmt.Sprintf(
					"关于「%s」你选了「%s」，当时放掉别的方案是因为：%s。现在回头看，这个理由还站得住吗？",
					d.Subject, d.Choice, d.WhyNot),
				anchorKind: "decision", anchorRef: d.ID.String(),
			})
			continue
		}
		out = append(out, pblLookbackSeed{
			prompt:     fmt.Sprintf("关于「%s」你选了「%s」。现在还会这么选吗？", d.Subject, d.Choice),
			anchorKind: "decision", anchorRef: d.ID.String(),
		})
	}

	// 她退回去或者不要的东西——她在这里真的行使过判断。
	for _, a := range artifacts {
		if a.Verdict == nil || (*a.Verdict != "revise" && *a.Verdict != "dropped") {
			continue
		}
		title := a.Title
		if strings.TrimSpace(title) == "" {
			title = "印记交的那一份"
		}
		out = append(out, pblLookbackSeed{
			prompt: fmt.Sprintf(
				"你把《%s》退了回去，理由是「%s」。再遇到差不多的东西，你会先看哪里？", title, a.Why),
			anchorKind: "artifact", anchorRef: a.ID.String(),
		})
	}

	// 一个项目可能什么都还没定过。那也要有得问，但仍然是具体的一问。
	if len(out) == 0 {
		out = append(out, pblLookbackSeed{
			prompt:     "这个项目里，哪一步比你想的难？",
			anchorKind: "free",
		})
	}
	out = append(out, pblLookbackSeed{
		prompt:     "下次再做这样一件事，你第一件会做什么？",
		anchorKind: "free",
	})
	return out
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
		reframes, rerr := a.d.Queries.ListPblReframes(r.Context(), atomID)
		if rerr != nil {
			httpx.WriteError(w, r, rerr)
			return
		}
		decisions, derr := a.d.Queries.ListPblDecisions(r.Context(), atomID)
		if derr != nil {
			httpx.WriteError(w, r, derr)
			return
		}
		artifacts, aerr := a.d.Queries.ListPblArtifacts(r.Context(), atomID)
		if aerr != nil {
			httpx.WriteError(w, r, aerr)
			return
		}
		for i, seed := range buildPblLookback(reframes, decisions, artifacts) {
			row, cerr := a.d.Queries.CreatePblReviewPrompt(r.Context(), sqlc.CreatePblReviewPromptParams{
				AtomID: atomID, Prompt: seed.prompt,
				AnchorKind: seed.anchorKind, AnchorRef: seed.anchorRef, Ordinal: int32(i),
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
