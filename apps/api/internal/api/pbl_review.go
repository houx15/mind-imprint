package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// pbl_review.go —— 审一遍印记交出来的东西。
//
// 产品负责人 2026-09-01：「A good review, the first requirement is a easy-to-read
// thing needs my review... highlighted sentences with questions to answer,
// explanations for each part so that we know what we should care about in each
// part. Click on any word to have a new chat line with AI to know why and
// what.」
//
// 两件事：
//
//   - **划出来的句子**（mark）：一句话 + 一个要她回答的问题。印记交东西的时候
//     先划一批；她自己在文里选中任何一段，也能当场划一条出来问。
//   - **该看的几个方面**（dimension）：图片和网站没有句子可划，但每种材料都有
//     几个必须看的方面。
//
// 🚨「点开任何一个词就能问」必须是**一次**动作。拆成"建 mark → 建会话 → 关联"
// 三个请求，她选中一段话要等三次网络往返，这个动作就没人用了。所以 ask 在服务
// 端一步做完。

// 她自己在文里选中一段问出来的那些，排在印记划的后面。
//
// 用序号而不是新开一列：这一刀不再动 schema，而"谁划的"只影响一件事——收工
// 时要求答完的是印记划的那些，她自己问出来的不算作业。
const pblAdHocMarkOrdinal = 9999

type pblMarkDTO struct {
	ID        string  `json:"id"`
	Part      string  `json:"part"`
	PartNote  string  `json:"partNote"`
	Quote     string  `json:"quote"`
	Question  string  `json:"question"`
	Answer    string  `json:"answer"`
	SessionID *string `json:"sessionId"`
	Ordinal   int32   `json:"ordinal"`
	/** 她自己问出来的（不是印记划的）。 */
	Mine bool `json:"mine"`
}

func toPblMarkDTO(m sqlc.PblReviewMark) pblMarkDTO {
	out := pblMarkDTO{
		ID: m.ID.String(), Part: m.Part, PartNote: m.PartNote, Quote: m.Quote,
		Question: m.Question, Answer: m.Answer, Ordinal: m.Ordinal,
		Mine: m.Ordinal >= pblAdHocMarkOrdinal,
	}
	if m.SessionID.Valid {
		s := uuid.UUID(m.SessionID.Bytes).String()
		out.SessionID = &s
	}
	return out
}

type pblDimensionDTO struct {
	ID      string `json:"id"`
	Prompt  string `json:"prompt"`
	Why     string `json:"why"`
	Answer  string `json:"answer"`
	Ordinal int32  `json:"ordinal"`
}

func toPblDimensionDTO(d sqlc.PblReviewDimension) pblDimensionDTO {
	return pblDimensionDTO{
		ID: d.ID.String(), Prompt: d.Prompt, Why: d.Why,
		Answer: d.Answer, Ordinal: d.Ordinal,
	}
}

// loadOwnedPblArtifact —— 成果挂在项目上，项目挂在人身上。
func (a *API) loadOwnedPblArtifact(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	aid, err := uuid.Parse(r.PathValue("aid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这件成果不存在"))
		return uuid.Nil, uuid.Nil, false
	}
	row, err := a.d.Queries.GetPblArtifact(r.Context(), aid)
	if err != nil || row.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这件成果不存在"))
		return uuid.Nil, uuid.Nil, false
	}
	return atomID, aid, true
}

// getPblReview —— 这件成果要审的东西：划出来的句子，和该看的几个方面。
func (a *API) getPblReview(w http.ResponseWriter, r *http.Request) {
	_, aid, ok := a.loadOwnedPblArtifact(w, r)
	if !ok {
		return
	}
	marks, err := a.d.Queries.ListPblReviewMarks(r.Context(), aid)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	dims, err := a.d.Queries.ListPblReviewDimensions(r.Context(), aid)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	outMarks := make([]pblMarkDTO, 0, len(marks))
	for _, m := range marks {
		outMarks = append(outMarks, toPblMarkDTO(m))
	}
	outDims := make([]pblDimensionDTO, 0, len(dims))
	for _, d := range dims {
		outDims = append(outDims, toPblDimensionDTO(d))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"marks": outMarks, "dimensions": outDims})
}

// createPblReviewPlan —— 印记交东西时，连着说清楚每一部分该看什么。
func (a *API) createPblReviewPlan(w http.ResponseWriter, r *http.Request) {
	_, aid, ok := a.loadOwnedPblArtifact(w, r)
	if !ok {
		return
	}
	var req struct {
		Marks []struct {
			Part     string `json:"part"`
			PartNote string `json:"partNote"`
			Quote    string `json:"quote"`
			Question string `json:"question"`
		} `json:"marks"`
		Dimensions []struct {
			Prompt string `json:"prompt"`
			Why    string `json:"why"`
		} `json:"dimensions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	if len(req.Marks) == 0 && len(req.Dimensions) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty", "没有要审的东西", nil))
		return
	}
	for i, m := range req.Marks {
		// 🚨 划一句话出来却不问什么，只是在把字标黄。
		if strings.TrimSpace(m.Question) == "" {
			httpx.WriteError(w, r, httpx.ErrBadRequest("no_question",
				"划出来的句子要带一个问题", nil))
			return
		}
		if _, err := a.d.Queries.CreatePblReviewMark(r.Context(), sqlc.CreatePblReviewMarkParams{
			ArtifactID: aid, Part: strings.TrimSpace(m.Part),
			PartNote: strings.TrimSpace(m.PartNote), Quote: strings.TrimSpace(m.Quote),
			Question: strings.TrimSpace(m.Question), Ordinal: int32(i),
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	for i, d := range req.Dimensions {
		if strings.TrimSpace(d.Prompt) == "" {
			httpx.WriteError(w, r, httpx.ErrBadRequest("no_prompt", "这一条没说要看什么", nil))
			return
		}
		if _, err := a.d.Queries.CreatePblReviewDimension(r.Context(), sqlc.CreatePblReviewDimensionParams{
			ArtifactID: aid, Prompt: strings.TrimSpace(d.Prompt),
			Why: strings.TrimSpace(d.Why), Ordinal: int32(i),
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	a.getPblReview(w, r)
}

// askPblReviewMark —— 她在文里选中一段，就地问。
//
// 一次请求做完：划一条出来、开一条会话线、把两者接上。这是「click on any word」
// 这个动作能不能被真的用起来的关键。
func (a *API) askPblReviewMark(w http.ResponseWriter, r *http.Request) {
	atomID, aid, ok := a.loadOwnedPblArtifact(w, r)
	if !ok {
		return
	}
	var req struct {
		Quote    string `json:"quote"`
		Question string `json:"question"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	quote := strings.TrimSpace(req.Quote)
	if quote == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_quote", "没选中哪一段", nil))
		return
	}
	question := strings.TrimSpace(req.Question)
	if question == "" {
		question = "这一句是什么意思，为什么这么写？"
	}

	mark, err := a.d.Queries.CreatePblReviewMark(r.Context(), sqlc.CreatePblReviewMarkParams{
		ArtifactID: aid, Quote: quote, Question: question, Ordinal: pblAdHocMarkOrdinal,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// 审阅里问出来的这条线挂在主线上（depth 0）：它是关于这份东西的，不是
	// 关于她当时正在挖的那一层的。
	sess, err := a.d.Queries.CreatePblSession(r.Context(), sqlc.CreatePblSessionParams{
		AtomID: atomID, Kind: "review", ParentID: pgtype.UUID{}, Depth: 0,
		AnchorKind: "artifact", AnchorRef: aid.String(), Question: question,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	linked, err := a.d.Queries.LinkPblReviewMarkSession(r.Context(), sqlc.LinkPblReviewMarkSessionParams{
		ID: mark.ID, SessionID: pgtype.UUID{Bytes: sess.ID, Valid: true},
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"mark":      toPblMarkDTO(linked),
		"sessionId": sess.ID.String(),
	})
}

// answerPblReviewMark —— 她对这一句的回答。
func (a *API) answerPblReviewMark(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	mid, err := uuid.Parse(r.PathValue("mid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一条不存在"))
		return
	}
	row, err := a.d.Queries.GetPblReviewMark(r.Context(), mid)
	if err != nil || row.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一条不存在"))
		return
	}
	var req struct {
		Answer string `json:"answer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	out, err := a.d.Queries.AnswerPblReviewMark(r.Context(), sqlc.AnswerPblReviewMarkParams{
		ID: mid, Answer: strings.TrimSpace(req.Answer),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPblMarkDTO(out))
}

// answerPblReviewDimension —— 她对"该看的这个方面"的回答。
func (a *API) answerPblReviewDimension(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	did, err := uuid.Parse(r.PathValue("did"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一条不存在"))
		return
	}
	row, err := a.d.Queries.GetPblReviewDimension(r.Context(), did)
	if err != nil || row.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一条不存在"))
		return
	}
	var req struct {
		Answer string `json:"answer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	out, err := a.d.Queries.AnswerPblReviewDimension(r.Context(), sqlc.AnswerPblReviewDimensionParams{
		ID: did, Answer: strings.TrimSpace(req.Answer),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPblDimensionDTO(out))
}
