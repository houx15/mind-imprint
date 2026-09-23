package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// writing_genre_route.go —— 她自己说这一篇是什么文体。
//
// # 为什么存在（2026-09-23，产品负责人）
//
//	「for writing, maybe we need to let the students select/talk with ai about
//	  what genre they are going to write. sometimes they are writing a 记叙文,
//	  sometimes 散文, sometimes 议论文, sometimes 书信.」
//
// # 🚨 这**不是**把进门那一步改回去
//
// writing_setup.go 的文件头写着「没有文体单选——这是产品的明确要求：学生未必
// 知道「文体」是什么意思」。那条理由今天照样成立，所以设定弹窗一个字都没动。
//
// 变的是**房间里**：印记本来就在按某一种文体教（推断出来的），只是从来没说出
// 口，也没给她一个说「不对，我写的是一封信」的地方。现在：
//
//   - GET  /genre —— 这一篇现在按什么在教、是推断的还是她定的、可以换成哪几种。
//   - PUT  /genre —— 她定下一种（空串 = 收回，回到推断）。
//
// 每一种后面跟的是「什么时候选它」而不是定义：「记叙文」三个字她未必读得懂，
// 「写一件真实发生过的事」读得懂。这是对那条老理由的正面回答 ——
// 问题从来不是「不该让她选」，是「不该拿一个她读不懂的词让她选」。
//
// # 换文体不动她任何一个字
//
// 这条路只写一列。图上的节点、写过的段落一个都不碰 —— 她把一篇判错成议论文
// 的信改成书信之后，那些「分论点」节点还在，由她自己在图上改种类
// （outlineRekind，书信那一档给的是要点 / 写信目的 / 结尾的话）。
// 和 writing_plan.go 那条「只加，不改不删」同一条纪律：她的东西归她。

func (a *API) getWritingGenre(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtomRow(w, r)
	if !ok {
		return
	}
	wr, rows, ok := a.writingAndOutline(w, r, at.ID)
	if !ok {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"genre": writingGenreOf(wr, rows),
		// chosen：这一篇的文体是她定的，还是我们推断的。界面据此决定说
		// 「印记按议论文在教」还是「你定的是议论文」—— 两句话的分量不一样，
		// 而把推断说成是她的选择，是在替她做主之后再赖给她。
		"chosen":  validateWritingGenre(wr.Genre) != "",
		"choices": writingGenreChoices(),
	})
}

func (a *API) putWritingGenre(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	var req struct {
		Genre string `json:"genre"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	// 空串是**合法的**：收回自己的选择，回到推断。
	// 认不出来的取值也当成空 —— 一个写错的 id 不该把她锁在某一种文体上。
	genre := validateWritingGenre(req.Genre)
	if err := a.d.Queries.SetWritingGenre(r.Context(), sqlc.SetWritingGenreParams{
		AtomID: at.ID, Genre: genre,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	wr, rows, ok := a.writingAndOutline(w, r, at.ID)
	if !ok {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"genre":   writingGenreOf(wr, rows),
		"chosen":  validateWritingGenre(wr.Genre) != "",
		"choices": writingGenreChoices(),
	})
}

// writingAndOutline 取这一篇和它的图 —— 判文体两样都要
// （板上的东西压过题目，见 writingGenreOf）。
func (a *API) writingAndOutline(w http.ResponseWriter, r *http.Request, atomID uuid.UUID) (sqlc.Writing, []sqlc.WritingOutline, bool) {
	wr, err := a.d.Queries.GetWriting(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return sqlc.Writing{}, nil, false
	}
	rows, err := a.d.Queries.ListWritingOutline(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return sqlc.Writing{}, nil, false
	}
	return wr, rows, true
}
