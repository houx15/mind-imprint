package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

// pbl_tool_gate.go —— 别把她送进一间空屋子。
//
// 🚨 有四件工具的界面本身没有内容，摆的是印记做出来的那份东西（见 pbl.Tool
// 的 Needs）。2026-09-02 线上实测：印记递「理性决策」，她点进去看到的是
//
//	「暂时没有需要决策的内容。印记提出几个方案时，会在这里让你选。」
//
// 结构审查和分工建议一样——两块白板，各配一句「到对话里请印记先给一个」。
// 她刚照着印记说的点进来，被打发回去求印记再做一遍它刚说要做的事。
//
// prompt 那边已经写清楚了「工具和产出同一轮一起给」，但 prompt 是请求，不是
// 保证。这个闸是保证：产出没落上，这件工具这一轮就不递。她少看见一个工具卡，
// 比看见一个点开是空的工具卡好得多——后者教她的是"这里的按钮不作数"。
//
// 时机很要紧：调用点在 applyPblProduce **之后**。印记这一轮做出来的东西那时
// 已经在库里了，所以这里只问库，不用再看模型说了什么——产出落库失败（那边是
// 记日志不报错）同样会被这道闸拦住，而那正是我们想要的。

// pblToolHasContent 说的是：这件工具现在打开，里面有她能动手的东西吗。
//
// 表外的工具一律算有——它们退回到朴素卡片，她自己在上面写结果，不会开白板。
func (a *API) pblToolHasContent(ctx context.Context, atomID uuid.UUID, tool string) bool {
	switch pbl.ToolNeeds(tool) {
	case "decision":
		// 已经拍板的那些留在界面上是记录，不是待办。只有还没定的才算"有东西
		// 可做"，否则印记会对着一屋子做完的决定再递一次理性决策。
		rows, err := a.d.Queries.ListPblDecisions(ctx, atomID)
		if err != nil {
			return false
		}
		for _, d := range rows {
			if !d.SettledAt.Valid {
				return true
			}
		}
		return false

	case "artifact":
		// 同理：判过的成果是记录，没判过的才是要她审的。
		rows, err := a.d.Queries.ListPblArtifacts(ctx, atomID)
		if err != nil {
			return false
		}
		for _, x := range rows {
			if x.Verdict == nil {
				return true
			}
		}
		return false

	case "substeps":
		v, err := a.d.Queries.GetPblLivePlan(ctx, atomID)
		if err != nil {
			return false
		}
		rows, err := a.d.Queries.ListPblSubstepsForPlan(ctx, v.ID)
		return err == nil && len(rows) > 0

	case "structure":
		rows, err := a.d.Queries.ListPblTreeNodes(ctx, sqlc.ListPblTreeNodesParams{
			AtomID: atomID, Tree: pblMainTree,
		})
		return err == nil && len(rows) > 0

	case "course":
		// 同理：上完的课留在界面上是记录，还没上完的才是要她做的事。
		return a.pblHasOpenCourse(ctx, atomID)
	}
	return true
}

// pblMainTree 是结构审查那块界面读的那棵树（前端 getTree 的默认值）。
const pblMainTree = "main"

// pblToolMissing 把 ToolNeeds 说成她读得懂的一句「缺的是什么」。
//
// 撤掉的理由必须说得出具体缺什么，否则那行说明等于「出错了」——她既不知道
// 发生了什么，也不知道下一步该干嘛。
func pblToolMissing(needs string) string {
	switch needs {
	case "decision":
		return "可选的方案"
	case "artifact":
		return "可审的成果"
	case "substeps":
		return "拆好的分工"
	case "structure":
		return "一份结构"
	case "course":
		// 课是从课程库里挑的，不是印记写出来的——所以缺的那样东西是「挑中的
		// 那一门」，不是「一份课」。
		return "挑好的那一门课"
	}
	return "这件工具要摆的那份东西"
}

// recordPblToolDrop 记下这次撤销，并在线程里补一行说明。
//
// 🚨 两件事都要做，缺一件这个问题就只修了一半：
//
//   - 线程里那一行是给**她**的。印记刚说「卡我给你了」，那句话已经落库，改不
//     动了；能做的是紧接着说清楚它没出现，以及为什么。不说，她会去找一张永远
//     不存在的卡——2026-09-04 走查里 Marcus 就这么找了三十多步。
//   - pbl_tool_drop 那一行是给**印记**的。它下一轮的上文里必须出现这件事，
//     否则它会照原样再说一遍。
//
// 两件都失败不让这一轮失败：她该看见的回话已经写进去了。
func (a *API) recordPblToolDrop(r *http.Request, atomID uuid.UUID, scope pgtype.UUID, tool string) {
	ctx := r.Context()
	needs := pbl.ToolNeeds(tool)
	if err := a.d.Queries.RecordPblToolDrop(ctx, sqlc.RecordPblToolDropParams{
		AtomID: atomID, Tool: tool, Needs: needs,
	}); err != nil {
		slog.Warn("pbl turn: could not record the dropped tool",
			"err", err, "atom_id", atomID, "tool", tool,
			"request_id", httpx.RequestIDFromContext(ctx))
	}

	label := tool
	if def, ok := pbl.LookupTool(tool); ok {
		label = def.Label
	}
	seq, err := a.d.Queries.NextAtomMessageSeq(ctx, atomID)
	if err != nil {
		slog.Warn("pbl turn: could not take a seq for the drop notice",
			"err", err, "atom_id", atomID, "request_id", httpx.RequestIDFromContext(ctx))
		return
	}
	if _, err := a.d.Queries.AppendPblSessionMessage(ctx, sqlc.AppendPblSessionMessageParams{
		AtomID: atomID, Seq: seq, Role: "system", SessionID: scope,
		Content: "「" + label + "」未递出：这一轮没有" + pblToolMissing(needs) +
			"，工具打开会是空的。印记已收到这条状态。",
	}); err != nil {
		slog.Warn("pbl turn: could not append the drop notice",
			"err", err, "atom_id", atomID, "request_id", httpx.RequestIDFromContext(ctx))
	}
}

// pblDroppedToolNote 是下一轮要交给印记的那句话，没有就返回空串。
//
// 只报**还没被解决的**那一次：这件工具后来递成了，或者它缺的那份东西现在有了，
// 就当这件事过去了。不这样收口的话，一次撤销会跟着这个项目到底，印记会被一句
// 早就不成立的话反复推着去递同一件工具。
func (a *API) pblDroppedToolNote(r *http.Request, atomID uuid.UUID) string {
	ctx := r.Context()
	d, err := a.d.Queries.LatestPblToolDrop(ctx, atomID)
	if err != nil {
		return ""
	}
	if a.pblToolAlreadyOnHerScreen(r, atomID, d.Tool) {
		return "" // 后来递成了。
	}
	if a.pblToolHasContent(ctx, atomID, d.Tool) {
		return "" // 缺的那份东西现在有了，这一条不再成立。
	}
	label := d.Tool
	if def, ok := pbl.LookupTool(d.Tool); ok {
		label = def.Label
	}
	what := pblToolMissing(d.Needs)
	return "「" + label + "」这件工具**没有**出现在她屏幕上。你说过要给她，但同一轮" +
		"没有做出" + what + "，那个界面打开是一块白板，所以服务端把它撤掉了。" +
		"她现在找不到这张卡。\n" +
		"这一轮只有两条路：把" + what + "和这件工具**一起**给（produce 和 tool 同一轮），" +
		"或者直说这件东西还没有。不要再说你已经把它给她了。"
}
