package api

// writing_flow.go —— 行文这一步：写之前先想清楚整篇怎么组织。
//
// # 为什么多这一步（同事 2026-09-20 的意见 4）
//
//	「前期逻辑讨论的部分需要增加一个对于行文方式的思考和梳理部分，
//	  要在开始写之前先想好整个文章组织框架（**并非填充内容**）如何搭建，
//	  现在只有文本内容的引导。」
//
// 在这之前三步是 结构 → 段落 → 成稿：结构那一步长出「有哪些点」，
// 段落那一步直接开始写字。中间少了一问 —— **这些点之间是什么关系，按什么顺序
// 摆，每一条打算怎么证明**。她因此是一段一段攒出一篇文章，而不是先有一篇文章
// 的形状再去填。
//
// # 它不新建表
//
// 行文那一步就是**那张结构图**，只是换一种用法：
//
//   - 顺序 → 已有的 `writing_outline.position`（拖动那套 0182 那一轮建好了）
//   - 每块的论证方法 → `writing_outline.method`（0184）
//   - 整篇的论证结构 → `writing.structure_key`（0184 把这列改作此用；
//     它自 2026-08-27 模板选择器被删之后就没人写过）
//
// # 🚨 这不是 2026-08-27 被否掉的那个「挑一副骨架往里填」
//
// 那次否掉的是**在她想之前**给她一张表去挑，然后照着填空 —— 那是填表，不是思考。
// 这一步发生在她自己的分论点都已经在图上之后，选的是**她已经摆出来的那些点
// 之间是什么关系**。先有东西，再给它命名 —— 和「方法名等她做出来再点」
// 是同一条教学动作（[[middle-school-writing-structure-pedagogy]]）。
//
// 所以这一步**一个字的正文都不写**。同事那句括号里的话是这一步的边界：
// 「并非填充内容」。

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/vocab"
)

// writingFlowStructureValid：这个 id 是不是那四条论证结构之一。
//
// 闭表来自 vocab（`applies_to: "whole"`），不在这里再写一份 —— 两份词表
// 迟早分岔，而分岔的那天她会在板上选一个服务端不认的结构。
func writingFlowStructureValid(id string) bool {
	if strings.TrimSpace(id) == "" {
		return false
	}
	// 🚨 这里**不挑文体**。校验回答的是「这个 id 库里有没有」；
	// 「这篇该看见哪几条」是目录那一条路的事（getWritingFlowStructures）。
	// 两件事混在一起的话，她在议论文里选完结构、接着把板改成记叙文，
	// 那次保存会以一个她读不懂的错误失败。
	for _, m := range vocab.Structures("") {
		if m.ID == id {
			return true
		}
	}
	return false
}

// writingValidStructureKey 收一收客户端给的结构 id：认不出的当「还没选」。
//
// 🚨 清空而不是拒绝整次保存：她在板上点了一下，服务端认不出那个 id，
// 这时候把她别的改动（顺序、方法）一起退回去，代价和收益完全不成比例。
func writingValidStructureKey(id string) string {
	if writingFlowStructureValid(id) {
		return id
	}
	return ""
}

// writingValidMethodID 收一收一块上标的论证方法。
//
// 只接**论证方法**那一层（`category == "method"`）：一块上标「总分式」是句
// 错话 —— 那是整篇的结构，不是某一段的证明方式。开篇和结尾那几种开法收法
// 也不在这里，它们由 kind 决定，不由她挑。
func writingValidMethodID(id string) string {
	t := strings.TrimSpace(id)
	if t == "" {
		return ""
	}
	m, ok := vocab.ByID(t)
	if !ok || m.Category != "method" {
		return ""
	}
	return t
}

// writingFlowStructureDTO 是摆在板上的那四张卡之一。
type writingFlowStructureDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Definition string `json:"definition"`
	// Example 是这种结构摆出来长什么样 —— 一句借来的示范（讲的是别的题目）。
	// 光给定义，「层进式」和「并列式」在一个中学生眼里是同一句话。
	Example string `json:"example"`
}

// writingFlowMethodDTO 是那个下拉里的一项（只有论证方法那一层）。
type writingFlowMethodDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Definition string `json:"definition"`
}

// vocabSpeaksLang 和 vocab 内部那条同一个判据：这篇的语言用不用得上这一条。
func vocabSpeaksLang(m vocab.Method, lang string) bool {
	return m.Lang == lang || m.Lang == "any"
}

// getWritingFlowStructures is GET /api/v1/writings/{id}/flow/structures.
//
// 那四条论证结构。挂在 writing 下面而不是做成一条全局的路：这个房间的每一条
// 读取路径都带着归属校验，一条不带的路会成为唯一的例外。
func (a *API) getWritingFlowStructures(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	wr, err := a.d.Queries.GetWriting(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	outline, err := a.d.Queries.ListWritingOutline(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// 摆给她看的只有这一种文体用得上的几条 —— 一篇记叙文里没有分论点，
	// 「它们之间是并列还是层进」是句问不出口的话。见 writing_genre.go。
	out := make([]writingFlowStructureDTO, 0, 4)
	for _, m := range vocab.Structures(writingGenreOf(wr, outline)) {
		ex := ""
		if len(m.Examples) > 0 {
			ex = m.Examples[0].Text
		}
		out = append(out, writingFlowStructureDTO{
			ID: m.ID, Name: m.Name, Definition: m.Definition, Example: ex,
		})
	}

	// 顺带把论证方法也给出去：那个下拉只在这一屏用得上，单开一条路不值得，
	// 而让前端自己写一份词表就是在等它和 vocab 分岔。
	methods := make([]writingFlowMethodDTO, 0, 8)
	for _, m := range vocab.All() {
		if m.Category != "method" || !vocabSpeaksLang(m, wr.Lang) {
			continue
		}
		name := m.FormalName
		if name == "" {
			name = m.Name
		}
		methods = append(methods, writingFlowMethodDTO{ID: m.ID, Name: name, Definition: m.Definition})
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{"structures": out, "methods": methods})
}

// putWritingFlow is PUT /api/v1/writings/{id}/flow.
//
// 收整篇的论证结构，以及每一块标的论证方法。**一个字的正文都不收** ——
// 同事那句括号里的话就是这条路的边界：「并非填充内容」。
func (a *API) putWritingFlow(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	var body struct {
		StructureKey string            `json:"structureKey"`
		Methods      map[string]string `json:"methods"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// 认不出的结构 id 当「还没选」，不整份拒绝 —— 见 writingValidStructureKey。
	if _, err := a.d.Queries.SetWritingStructure(r.Context(), sqlc.SetWritingStructureParams{
		AtomID: at.ID, StructureKey: writingValidStructureKey(body.StructureKey),
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// 每一块的方法。只认这一篇自己的块 —— 一个别人的 outline id 在这里
	// 什么都改不动，和这个房间别的写入路径是同一条规矩。
	rows, err := a.d.Queries.ListWritingOutline(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	mine := make(map[string]bool, len(rows))
	for _, row := range rows {
		mine[row.ID.String()] = true
	}
	for id, method := range body.Methods {
		if !mine[id] {
			continue
		}
		oid, perr := uuid.Parse(id)
		if perr != nil {
			continue
		}
		if err := a.d.Queries.SetWritingOutlineMethod(r.Context(), sqlc.SetWritingOutlineMethodParams{
			ID: oid, Method: writingValidMethodID(method),
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}

	fresh, err := a.d.Queries.ListWritingOutline(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]writingOutlineItemDTO, 0, len(fresh))
	for _, row := range fresh {
		out = append(out, toWritingOutlineItemDTO(row))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"outline": out})
}
