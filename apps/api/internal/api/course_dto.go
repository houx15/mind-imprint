package api

import (
	"encoding/json"

	"mindimprint/api/internal/store/sqlc"
)

type courseSummaryDTO struct {
	ID         string `json:"id"`
	Branch     string `json:"branch"`
	Title      string `json:"title"`
	Blurb      string `json:"blurb"`
	TasksCount int32  `json:"tasks_count"`
	ToolsCount int32  `json:"tools_count"`
	TimeLabel  string `json:"time_label"`
	StepCount  int    `json:"step_count"`
}

func toCourseSummaryDTO(r sqlc.ListCoursesRow) courseSummaryDTO {
	return courseSummaryDTO{
		ID: r.ID.String(), Branch: r.Branch, Title: r.Title, Blurb: r.Blurb,
		TasksCount: r.TasksCount, ToolsCount: r.ToolsCount, TimeLabel: r.TimeLabel,
		StepCount: int(r.StepCount),
	}
}

type courseStepDTO struct {
	ID              string          `json:"id"`
	CourseID        string          `json:"course_id"`
	Ordinal         int32           `json:"ordinal"`
	Kind            string          `json:"kind"`
	Purpose         string          `json:"purpose"`
	Assets          json.RawMessage `json:"assets"`
	ChallengeType   *string         `json:"challenge_type"`
	AuthoredContent json.RawMessage `json:"authored_content"`
}

func toCourseStepDTO(s sqlc.CourseStep) courseStepDTO {
	assets := json.RawMessage(s.Assets)
	if len(assets) == 0 {
		assets = json.RawMessage("[]")
	}
	ac := json.RawMessage(s.AuthoredContent)
	if len(ac) == 0 {
		ac = json.RawMessage("{}")
	}
	return courseStepDTO{
		ID: s.ID.String(), CourseID: s.CourseID.String(), Ordinal: s.Ordinal,
		Kind: s.Kind, Purpose: s.Purpose, Assets: assets,
		ChallengeType: s.ChallengeType, AuthoredContent: ac,
	}
}

type courseDTO struct {
	courseSummaryDTO
	Steps []courseStepDTO `json:"steps"`
}

type courseProgressDTO struct {
	CourseID          string  `json:"course_id"`
	CurrentOrdinal    int32   `json:"current_ordinal"`
	CompletedOrdinals []int32 `json:"completed_ordinals"`
	UpdatedAt         string  `json:"updated_at"`
}

func toCourseProgressDTO(p sqlc.CourseProgress) courseProgressDTO {
	co := p.CompletedOrdinals
	if co == nil {
		co = []int32{}
	}
	return courseProgressDTO{
		CourseID: p.CourseID.String(), CurrentOrdinal: p.CurrentOrdinal,
		CompletedOrdinals: co, UpdatedAt: p.UpdatedAt.Format(tsLayout),
	}
}
