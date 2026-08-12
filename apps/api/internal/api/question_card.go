package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// question_card.go — slice 3a · the 提问卡 (question card) adaptive sub-agent
// endpoints (all-statuses.md §2). The turn loop is stateless server-side: the
// modal holds the conversation and posts the whole history each turn. On commit
// the student's articulated research question fills proposal.objective and a
// card-used event lands in the process tree.

// questionCardAISummonable reports whether the coach may PROPOSE the 提问卡 for
// this project — true ONLY while proposal.objective is empty (a start-of-
// framework aid). Once the research question is formed the card is gone.
func (a *API) questionCardAISummonable(ctx context.Context, projectID uuid.UUID) bool {
	objectiveEmpty := true
	if prop, err := a.d.Queries.GetProjectProposal(ctx, projectID); err == nil {
		objectiveEmpty = strings.TrimSpace(prop.Objective) == ""
	}
	return agent.QuestionCardAISummonable(objectiveEmpty)
}

// POST /projects/{id}/cards/question-card/turn {messages:[{role,text}]}
func (a *API) postQuestionCardTurn(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Messages []struct {
			Role string `json:"role"`
			Text string `json:"text"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "请求格式不对", nil))
		return
	}
	history := make([]agent.ChatTurn, 0, len(body.Messages))
	for _, m := range body.Messages {
		role := "user"
		if m.Role == "ai" || m.Role == "assistant" {
			role = "assistant"
		}
		history = append(history, agent.ChatTurn{Role: role, Content: m.Text})
	}

	title, objective := "", ""
	if p, err := a.d.Queries.GetProject(r.Context(), projectID); err == nil {
		title = p.Title
	}
	if prop, err := a.d.Queries.GetProjectProposal(r.Context(), projectID); err == nil {
		objective = prop.Objective
	}

	// Fallback reply keeps the modal alive if the model call degrades.
	reply := questionCardReplyDTO{Narrate: "我们先从你自己的直觉开始——用你自己的话说说，你对这个题目的理解是？", SuggestedObjective: nil, Done: false}
	if a.d.Provider != nil {
		if resolved, rok := a.resolveFast(r.Context()); rok {
			out, usage, err := agent.QuestionCardTurn(r.Context(), a.d.Provider, resolved, agent.QuestionCardInput{
				Title: title, Objective: objective, History: history,
			})
			a.meterCall(r.Context(), projectID, resolved, "question_card", usage)
			if err == nil {
				reply.Narrate = out.Narrate
				reply.Done = out.Done
				if s := strings.TrimSpace(out.SuggestedObjective); s != "" {
					reply.SuggestedObjective = &s
				}
			} else {
				slog.Warn("question card: turn failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
			}
		}
	}
	httpx.WriteJSON(w, http.StatusOK, reply)
}

type questionCardReplyDTO struct {
	Narrate            string  `json:"narrate"`
	SuggestedObjective *string `json:"suggestedObjective"`
	Done               bool    `json:"done"`
}

// POST /projects/{id}/cards/question-card/commit {objective}
func (a *API) postQuestionCardCommit(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Objective string `json:"objective"`
		// The modal's whole conversation — the design (§2) treats the chat
		// history AS the detailed content of the 提问卡, so we persist it in the
		// process tree alongside the formed research question (过程即数据).
		Messages []struct {
			Role string `json:"role"`
			Text string `json:"text"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "请求格式不对", nil))
		return
	}
	objective := strings.TrimSpace(body.Objective)
	if objective == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "研究问题不能为空", nil))
		return
	}
	transcript := make([]map[string]string, 0, len(body.Messages))
	for _, m := range body.Messages {
		if strings.TrimSpace(m.Text) == "" {
			continue
		}
		transcript = append(transcript, map[string]string{"role": m.Role, "text": m.Text})
	}

	// Preserve the other dims; only the objective is filled by the card (§2).
	prev, _ := a.d.Queries.GetProjectProposal(r.Context(), projectID)
	if _, err := a.d.Queries.UpsertProjectProposal(r.Context(), sqlc.UpsertProjectProposalParams{
		ProjectID:     projectID,
		Objective:     objective,
		Reason:        prev.Reason,
		Activities:    prev.Activities,
		Resources:     prev.Resources,
		Counterpoints: prev.Counterpoints,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Record the card usage so it lands in the process tree (a card-used marker).
	// The chat history is the card's detailed content (§2) — stored as transcript.
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "card_completed",
		Payload: mustJSON(map[string]any{"cardId": "question-card", "objective": objective, "transcript": transcript}),
	}); err != nil {
		slog.Warn("question card: append card_completed event failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{"objective": objective})
}
