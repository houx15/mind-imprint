package api

import (
	"encoding/json"
	"net/http"
	"time"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

const tsLayout = time.RFC3339Nano

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

type messageDTO struct {
	ID        string          `json:"id"`
	Role      string          `json:"role"`
	Content   string          `json:"content"`
	ToolCall  json.RawMessage `json:"tool_call,omitempty"`
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
			Role:      m.Role,
			Content:   m.Content,
			ToolCall:  tc,
			CreatedAt: m.CreatedAt.Format(tsLayout),
		})
	}
	return out
}

type cardDTO struct {
	ID           string          `json:"id"`
	CardID       string          `json:"card_id"`
	Status       string          `json:"status"`
	ParentNodeID *string         `json:"parent_node_id"`
	FieldValues  json.RawMessage `json:"field_values"`
	EventTrace   json.RawMessage `json:"event_trace"`
	RubricTags   []string        `json:"rubric_tags"`
	CreatedAt    string          `json:"created_at"`
	CompletedAt  *string         `json:"completed_at"`
}

func toCardDTO(c sqlc.CardInstance) cardDTO {
	d := cardDTO{
		ID:          c.ID.String(),
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
