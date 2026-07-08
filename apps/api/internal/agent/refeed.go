package agent

import (
	"encoding/json"

	"mindimprint/api/internal/cards"
)

// CardInstance is the agent-layer domain view of a card_instance row: its status
// and the nested field_values (step.key → field.key → value). The DB stores
// field_values as jsonb; the turn engine decodes it into this shape.
type CardInstance struct {
	ID            string
	CardID        string
	TaskID        string
	Status        string
	FieldValues   map[string]map[string]any
	Anchors       []Anchor // annotation/keystone cards persist answers here, not in FieldValues
	EventTraceLen int      // populated for eval input (TS card.event_trace.length)
}

// RefeedAnswer pairs a field label with the value the human entered.
type RefeedAnswer struct {
	Label string `json:"label"`
	Value any    `json:"value"`
}

// RefeedStep groups the answers for one card step.
type RefeedStep struct {
	Title   string         `json:"title"`
	Answers []RefeedAnswer `json:"answers"`
}

// RefeedPayload is the serialized card outcome fed back to the model as a
// tool_result. Mirrors the TS RefeedPayload: {card_id, card_name, status, steps?}.
type RefeedPayload struct {
	CardID   string
	CardName string
	Status   string
	// Steps is nil for skipped (the key is omitted) and a (possibly empty)
	// slice for completed (the key is always present). MarshalJSON enforces this.
	Steps []RefeedStep
}

// MarshalJSON reproduces the TS field order and the steps?-omission rule: the
// steps key is present only when Status != "skipped".
func (p RefeedPayload) MarshalJSON() ([]byte, error) {
	m := map[string]any{
		"card_id":   p.CardID,
		"card_name": p.CardName,
		"status":    p.Status,
	}
	if p.Status != "skipped" {
		steps := p.Steps
		if steps == nil {
			steps = []RefeedStep{}
		}
		m["steps"] = steps
	}
	return json.Marshal(m)
}

// isEmpty mirrors the TS isEmpty: undefined/null/""/empty-array are empty.
// Numbers (including 0) and false are NOT empty — they are non-nil non-string
// non-slice values, so the default branch returns false. JSON numbers decode
// to float64 in Go's map[string]any, so isEmpty(float64(0)) == false, matching TS.
func isEmpty(v any) bool {
	if v == nil {
		return true
	}
	switch t := v.(type) {
	case string:
		return t == ""
	case []any:
		return len(t) == 0
	case []map[string]any:
		return len(t) == 0
	}
	return false
}

// SerializeCardForRefeed ports the TS serializeCardForRefeed verbatim:
//   - skipped → identity + status only (no steps key)
//   - completed → per-step answers, empty fields skipped, repeatable_group rows
//     remapped from item.key → item.label, empty rows/cells dropped.
func SerializeCardForRefeed(spec cards.Spec, inst CardInstance) RefeedPayload {
	cardName := spec.Name
	if inst.Status == "skipped" {
		return RefeedPayload{CardID: spec.ID, CardName: cardName, Status: "skipped"}
	}

	steps := []RefeedStep{}
	for _, step := range spec.Steps {
		stepValues := inst.FieldValues[step.Key] // nil if absent → reads as empty
		var answers []RefeedAnswer
		for _, field := range step.Fields {
			raw, present := stepValues[field.Key]
			if !present || isEmpty(raw) {
				continue
			}
			if field.Type == "repeatable_group" {
				rawRows, ok := raw.([]any)
				if !ok {
					continue
				}
				var rows []map[string]any
				for _, r := range rawRows {
					row, ok := r.(map[string]any)
					if !ok {
						continue
					}
					out := map[string]any{}
					for _, item := range field.ItemFields {
						cell, has := row[item.Key]
						if has && !isEmpty(cell) {
							out[item.Label] = cell
						}
					}
					if len(out) > 0 {
						rows = append(rows, out)
					}
				}
				if len(rows) > 0 {
					answers = append(answers, RefeedAnswer{Label: field.Label, Value: rows})
				}
			} else {
				answers = append(answers, RefeedAnswer{Label: field.Label, Value: raw})
			}
		}
		if len(answers) > 0 {
			steps = append(steps, RefeedStep{Title: step.Title, Answers: answers})
		}
	}
	// Annotation/keystone cards leave field_values empty and persist the
	// student's answers on anchors; fold those in so 摘要回灌 sees them.
	steps = append(steps, anchorSteps(inst)...)
	return RefeedPayload{CardID: spec.ID, CardName: cardName, Status: "completed", Steps: steps}
}

// anchorSteps turns answered anchors into refeed steps, one per dimension
// (first-seen order), mapping each anchor to {label: question, value: answer}.
// Unanswered anchors (the AI's question with no student reply) are dropped, so
// form cards — which carry no answered anchors — contribute nothing here.
func anchorSteps(inst CardInstance) []RefeedStep {
	var steps []RefeedStep
	byDim := map[string]int{}
	for _, a := range inst.Anchors {
		if isEmpty(a.Answer) {
			continue
		}
		dim := a.Dimension
		if dim == "" {
			dim = "标注"
		}
		label := a.Question
		if label == "" {
			label = dim
		}
		i, ok := byDim[dim]
		if !ok {
			steps = append(steps, RefeedStep{Title: dim})
			i = len(steps) - 1
			byDim[dim] = i
		}
		steps[i].Answers = append(steps[i].Answers, RefeedAnswer{Label: label, Value: a.Answer})
	}
	return steps
}
