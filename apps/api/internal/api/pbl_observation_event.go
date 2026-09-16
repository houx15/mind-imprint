package api

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"net/http"
)

type observationSubmission struct {
	ToolID   string `json:"toolId"`
	Revision int32  `json:"revision"`
}

func (a *API) observationSubmissionEvent(r *http.Request, atomID uuid.UUID, scope pgtype.UUID, event *observationSubmission) (string, error) {
	if event == nil {
		return "", nil
	}
	id, err := uuid.Parse(event.ToolID)
	if err != nil {
		return "", fmt.Errorf("观察记录目标无效")
	}
	tool, err := a.d.Queries.GetPblTool(r.Context(), id)
	if err != nil || tool.AtomID != atomID || tool.Tool != "observe" || tool.Status != "accepted" || tool.SessionID != scope {
		return "", fmt.Errorf("观察任务不属于当前讨论或已结束")
	}
	draft, err := a.d.Queries.GetPblObservationDraft(r.Context(), id)
	if err != nil || draft.SubmittedRevision == nil || *draft.SubmittedRevision != event.Revision {
		return "", fmt.Errorf("观察提交回执已变化或不存在，请查看最新记录")
	}
	var notes []pblNoteDTO
	if err = json.Unmarshal(draft.SubmittedNotes, &notes); err != nil || len(notes) == 0 {
		return "", fmt.Errorf("观察提交回执为空")
	}
	line := "学生刚提交一批观察记录，观察任务仍在进行。提交不代表已外出或已完成调查；先回应本批记录中的问题，不自动恢复历史制作请求。"
	for _, note := range notes {
		line += "；本次提交（类型：" + note.Kind + "）：" + note.Body
	}
	return line, nil
}
