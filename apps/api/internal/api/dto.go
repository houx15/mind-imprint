package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// numericString converts a pgtype.Numeric cost value to a string representation.
// Returns "0" when the value is NULL/invalid.
func numericString(n pgtype.Numeric) string {
	if !n.Valid {
		return "0"
	}
	v, _ := n.Value()
	return fmt.Sprint(v)
}

const tsLayout = time.RFC3339Nano

// nowPlusDays returns the time d days from now (used for invite TTL).
func nowPlusDays(d int) time.Time {
	return time.Now().Add(time.Duration(d) * 24 * time.Hour)
}

// decodeJSON reads a JSON request body into v, mapping any failure to a 400.
func decodeJSON(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return httpx.ErrBadRequest("validation_failed", "请求体格式错误", nil)
	}
	return nil
}

type taskDTO struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	Seed         *string `json:"seed"`
	Status       string  `json:"status"`
	CreatedAt    string  `json:"created_at"`
	LastActiveAt string  `json:"last_active_at"`
}

func toTaskDTO(t sqlc.Task) taskDTO {
	return taskDTO{
		ID:           t.ID.String(),
		Title:        t.Title,
		Seed:         t.Seed,
		Status:       t.Status,
		CreatedAt:    t.CreatedAt.Format(tsLayout),
		LastActiveAt: t.LastActiveAt.Format(tsLayout),
	}
}

type materialDTO struct {
	ID        string          `json:"id"`
	TaskID    string          `json:"task_id"`
	Kind      string          `json:"kind"`
	Source    string          `json:"source"`
	Title     string          `json:"title"`
	SourceURL *string         `json:"source_url"`
	Blocks    json.RawMessage `json:"blocks"`
	Scratch   string          `json:"scratch"`
	CreatedAt string          `json:"created_at"`
}

func toMaterialDTO(m sqlc.Material) materialDTO {
	blocks := json.RawMessage(m.Blocks)
	if len(blocks) == 0 {
		blocks = json.RawMessage("[]")
	}
	return materialDTO{
		ID:        m.ID.String(),
		TaskID:    m.TaskID.String(),
		Kind:      m.Kind,
		Source:    m.Source,
		Title:     m.Title,
		SourceURL: m.SourceUrl,
		Blocks:    blocks,
		Scratch:   m.Scratch,
		CreatedAt: m.CreatedAt.Format(tsLayout),
	}
}

func toMaterialDTOs(rows []sqlc.Material) []materialDTO {
	out := make([]materialDTO, 0, len(rows))
	for _, m := range rows {
		out = append(out, toMaterialDTO(m))
	}
	return out
}

type messageDTO struct {
	ID        string          `json:"id"`
	TaskID    string          `json:"task_id"`
	Role      string          `json:"role"`
	Content   string          `json:"content"`
	ToolCall  json.RawMessage `json:"tool_call,omitempty"`
	Source    *string         `json:"source,omitempty"`
	CreatedAt string          `json:"created_at"`
}

func toMessageDTOs(rows []sqlc.Message) []messageDTO {
	out := make([]messageDTO, 0, len(rows))
	for _, m := range rows {
		var tc json.RawMessage
		if len(m.ToolCall) > 0 {
			tc = json.RawMessage(m.ToolCall)
		}
		out = append(out, messageDTO{
			ID:        m.ID.String(),
			TaskID:    m.TaskID.String(),
			Role:      m.Role,
			Content:   m.Content,
			ToolCall:  tc,
			Source:    m.Source,
			CreatedAt: m.CreatedAt.Format(tsLayout),
		})
	}
	return out
}

type cardDTO struct {
	ID           string          `json:"id"`
	TaskID       string          `json:"task_id"`
	CardID       string          `json:"card_id"`
	Status       string          `json:"status"`
	ParentNodeID *string         `json:"parent_node_id"`
	FieldValues  json.RawMessage `json:"field_values"`
	EventTrace   json.RawMessage `json:"event_trace"`
	Anchors      json.RawMessage `json:"anchors"`
	RubricTags   []string        `json:"rubric_tags"`
	CreatedAt    string          `json:"created_at"`
	CompletedAt  *string         `json:"completed_at"`
}

func toCardDTO(c sqlc.CardInstance) cardDTO {
	d := cardDTO{
		ID:          c.ID.String(),
		TaskID:      c.TaskID.String(),
		CardID:      c.CardID,
		Status:      c.Status,
		FieldValues: json.RawMessage(c.FieldValues),
		EventTrace:  json.RawMessage(c.EventTrace),
		RubricTags:  c.RubricTags,
		CreatedAt:   c.CreatedAt.Format(tsLayout),
	}
	if c.ParentNodeID.Valid {
		// pgtype.UUID.String() returns the canonical hyphenated form when Valid.
		s := c.ParentNodeID.String()
		d.ParentNodeID = &s
	}
	if c.CompletedAt.Valid {
		s := c.CompletedAt.Time.Format(tsLayout)
		d.CompletedAt = &s
	}
	if d.RubricTags == nil {
		d.RubricTags = []string{}
	}
	d.Anchors = json.RawMessage(c.Anchors)
	if len(d.Anchors) == 0 {
		d.Anchors = json.RawMessage("[]")
	}
	return d
}

func toCardDTOs(rows []sqlc.CardInstance) []cardDTO {
	out := make([]cardDTO, 0, len(rows))
	for _, c := range rows {
		out = append(out, toCardDTO(c))
	}
	return out
}

type evaluationDTO struct {
	ID          string          `json:"id"`
	TaskID      string          `json:"task_id"`
	Scores      json.RawMessage `json:"scores"`
	Narrative   string          `json:"narrative"`
	Model       string          `json:"model"`
	Status      string          `json:"status"`
	CreatedAt   string          `json:"created_at"`
	CompletedAt *string         `json:"completed_at"`
}

func toEvaluationDTO(e sqlc.Evaluation) evaluationDTO {
	d := evaluationDTO{
		ID:        e.ID.String(),
		TaskID:    e.TaskID.String(),
		Scores:    json.RawMessage(e.Scores),
		Narrative: e.Narrative,
		Model:     e.Model,
		Status:    e.Status,
		CreatedAt: e.CreatedAt.Format(tsLayout),
	}
	if e.CompletedAt.Valid {
		s := e.CompletedAt.Time.Format(tsLayout)
		d.CompletedAt = &s
	}
	return d
}

type meSchoolDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type meClassDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	RoleInClass string `json:"role_in_class"`
}

type meUserDTO struct {
	ID          string       `json:"id"`
	Email       string       `json:"email"`
	DisplayName string       `json:"display_name"`
	Role        string       `json:"role"`
	AvatarColor string       `json:"avatar_color"`
	School      meSchoolDTO  `json:"school"`
	Classes     []meClassDTO `json:"classes"`
}

// buildMeUser assembles the full /me + signin user payload from the principal.
func (a *API) buildMeUser(ctx context.Context, u User) (meUserDTO, error) {
	full, err := a.d.Queries.GetUserByID(ctx, u.ID)
	if err != nil {
		return meUserDTO{}, err
	}
	school, err := a.d.Queries.GetSchool(ctx, u.SchoolID)
	if err != nil {
		return meUserDTO{}, err
	}
	rows, err := a.d.Queries.ListClassesForUser(ctx, u.ID)
	if err != nil {
		return meUserDTO{}, err
	}
	classes := make([]meClassDTO, 0, len(rows))
	for _, c := range rows {
		classes = append(classes, meClassDTO{ID: c.ID.String(), Name: c.Name, RoleInClass: c.RoleInClass})
	}
	return meUserDTO{
		ID:          full.ID.String(),
		Email:       full.Email,
		DisplayName: full.DisplayName,
		Role:        full.Role,
		AvatarColor: full.AvatarColor,
		School:      meSchoolDTO{ID: school.ID.String(), Name: school.Name},
		Classes:     classes,
	}, nil
}
