package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

func missionSnapshot(rows []sqlc.PblMissionItem) string {
	raw, _ := json.Marshal(rows)
	return string(raw)
}

func (a *API) attachPblMissionVersions(r *http.Request, atomID uuid.UUID, in *pbl.CoachInput) error {
	rows, err := a.d.Queries.ListPblTools(r.Context(), atomID)
	if err != nil {
		return err
	}
	in.MissionVersions = map[string]string{}
	for _, tool := range rows {
		if tool.Tool != "observe" || (tool.Status != "summoned" && tool.Status != "accepted") {
			continue
		}
		items, err := a.d.Queries.ListPblMissionItems(r.Context(), tool.ID)
		if err != nil {
			return err
		}
		in.MissionVersions[tool.ID.String()] = missionSnapshot(items)
		var lines []string
		for _, item := range items {
			if item.SupersededAt.Valid {
				continue
			}
			status := "待完成"
			if item.DoneAt.Valid {
				status = "已勾选"
			}
			if item.EditedByStudent {
				status += "（学生修改）"
			}
			lines = append(lines, status+"："+item.Prompt)
		}
		in.ToolWork = append(in.ToolWork, "可修订观察清单，mission_target="+tool.ID.String()+"。当前任务："+strings.Join(lines, "；")+"。仅在学生要求修改或明确改变条件时修订；勾选记录不会自动转移到新版，历史保留。")
	}
	return nil
}

func (a *API) revisePblMission(ctx context.Context, atomID uuid.UUID, target string, items []pbl.MissionItem, versions map[string]string) error {
	tid, err := uuid.Parse(target)
	if err != nil {
		return fmt.Errorf("观察清单目标无效")
	}
	expected, ok := versions[tid.String()]
	if !ok {
		return fmt.Errorf("观察清单不在本轮可修订范围内")
	}
	if len(items) == 0 || len(items) > 8 {
		return fmt.Errorf("观察清单须包含1至8项任务")
	}
	for _, item := range items {
		if strings.TrimSpace(item.Prompt) == "" || len([]rune(item.Prompt)) > 2000 {
			return fmt.Errorf("观察任务内容为空或过长")
		}
		if item.WantKind != "" && !pblWantKinds[item.WantKind] {
			return fmt.Errorf("观察任务记录类型无效")
		}
	}
	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := a.d.Queries.WithTx(tx)
	tool, err := q.LockPblMissionTool(ctx, tid)
	if err != nil {
		return err
	}
	if tool.AtomID != atomID || tool.Tool != "observe" || (tool.Status != "summoned" && tool.Status != "accepted") {
		return fmt.Errorf("观察任务已结束或不属于此项目")
	}
	rows, err := q.ListPblMissionItems(ctx, tid)
	if err != nil {
		return err
	}
	if missionSnapshot(rows) != expected {
		return fmt.Errorf("观察清单已发生变化，请根据最新记录重新修改")
	}
	if err = q.SupersedePblMissionItems(ctx, tid); err != nil {
		return err
	}
	for i, item := range items {
		if _, err = q.CreatePblMissionItem(ctx, sqlc.CreatePblMissionItemParams{ToolID: tid, Prompt: strings.TrimSpace(item.Prompt), WantKind: item.WantKind, Ordinal: int32(i)}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
