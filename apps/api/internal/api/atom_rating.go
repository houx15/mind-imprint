package api

import (
	"net/http"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// 她给这次阅读体验打的星，收在报告的底部。
//
// 🚨 方向读反了就成了铁律②禁的东西。**她评的是我们**：这一趟读下来感觉怎么样。
// 它不是对她的评分、不是等第、不进任何评估、也不喂给模型——所以它只在她自己的
// 报告页出现，公开分享的那份 payload 里一个字都没有（见 getPublicReport 的注释
// 与 TestPublicPayloadCarriesNothingExtra，那个测试钉死了公开报告的键集）。
//
// 「还没打星」和「打了一星」必须分得开，所以列可空、DTO 里也是 `null` 而不是 0
// （0107）。把没说过的话读成最低分，是这个功能唯一真正会造成伤害的错法。

type atomRatingRequest struct {
	// 指针：缺字段、null、0 三者要能区分。少写这一个星号，前端漏传就会被当成
	// 0 分，然后被下面的范围检查拒掉——错误信息还指着一个她没做过的动作。
	Rating *int `json:"rating"`
}

// putAtomRatingFor is the shared body behind PUT /api/v1/readings/{id}/rating.
// Generic over kind so the writing report can ask the same question later
// without a second copy of the range check.
func (a *API) putAtomRatingFor(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// loadOwnedAtomRow, NOT loadOwnedAtom: a finished atom is exactly when
		// this is asked, and the finished-write gate would refuse it. Rating
		// the experience is not editing the record of it.
		at, ok := a.loadOwnedAtomRow(w, r, kind)
		if !ok {
			return
		}
		var body atomRatingRequest
		if err := decodeJSON(r, &body); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if body.Rating == nil || *body.Rating < 1 || *body.Rating > 5 {
			httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_rating", "星星只能是 1 到 5 颗。", nil))
			return
		}
		v := int16(*body.Rating)
		row, err := a.d.Queries.SetAtomExperienceRating(r.Context(), sqlc.SetAtomExperienceRatingParams{
			ID: at.ID, ExperienceRating: &v,
		})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"rating": row.ExperienceRating})
	}
}

func (a *API) putReadingRating() http.HandlerFunc { return a.putAtomRatingFor("reading") }
