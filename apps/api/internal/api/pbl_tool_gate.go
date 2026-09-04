package api

import (
	"context"

	"github.com/google/uuid"

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
