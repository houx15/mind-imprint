package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/prompts"
)

const showcaseAboutRequestMaxBytes = 128 << 10

type showcaseAboutProfile struct {
	Name        string   `json:"name"`
	Bio         string   `json:"bio"`
	Interests   []string `json:"interests"`
	AboutLayout string   `json:"aboutLayout"`
}

type showcaseAboutProposal struct {
	Name        string   `json:"name"`
	Bio         string   `json:"bio"`
	Interests   []string `json:"interests"`
	AboutLayout string   `json:"aboutLayout"`
	Reason      string   `json:"reason"`
}

type showcaseAboutReply struct {
	Reply    string                 `json:"reply"`
	Proposal *showcaseAboutProposal `json:"proposal,omitempty"`
}

type showcaseAboutRequest struct {
	Messages []showcaseChatMessage `json:"messages"`
	Profile  showcaseAboutProfile  `json:"profile"`
}

func normalizeShowcaseAboutRequest(in showcaseAboutRequest) (showcaseAboutRequest, error) {
	if len(in.Messages) == 0 || len(in.Messages) > 20 {
		return in, errors.New("个人介绍对话数量无效")
	}
	total := 0
	for i := range in.Messages {
		m := &in.Messages[i]
		m.Content = strings.TrimSpace(m.Content)
		total += len([]rune(m.Content))
		if !oneOf(m.Role, "user", "assistant") || m.Content == "" || len([]rune(m.Content)) > 2000 || total > 20000 {
			return in, errors.New("个人介绍对话无效")
		}
	}
	if in.Messages[len(in.Messages)-1].Role != "user" {
		return in, errors.New("最后一条消息必须来自学生")
	}
	in.Profile.Name = strings.TrimSpace(in.Profile.Name)
	in.Profile.Bio = strings.TrimSpace(in.Profile.Bio)
	if len([]rune(in.Profile.Name)) > 80 || len([]rune(in.Profile.Bio)) > 2000 || !oneOf(in.Profile.AboutLayout, "classic", "orbit") {
		return in, errors.New("个人资料无效")
	}
	clean, ok := cleanShowcaseStrings(in.Profile.Interests, 12, 60)
	if !ok {
		return in, errors.New("兴趣内容无效")
	}
	in.Profile.Interests = clean
	return in, nil
}

func cleanShowcaseStrings(values []string, max, width int) ([]string, bool) {
	if len(values) > max {
		return nil, false
	}
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || len([]rune(value)) > width {
			return nil, false
		}
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out, true
}

func parseShowcaseAboutReply(raw string) (showcaseAboutReply, error) {
	var out showcaseAboutReply
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &out); err != nil {
		return out, errors.New("模型返回内容无法解析")
	}
	out.Reply = strings.TrimSpace(out.Reply)
	if out.Reply == "" || len([]rune(out.Reply)) > 1000 {
		return out, errors.New("模型回复无效")
	}
	if out.Proposal == nil {
		return out, nil
	}
	p := out.Proposal
	p.Name, p.Bio, p.Reason = strings.TrimSpace(p.Name), strings.TrimSpace(p.Bio), strings.TrimSpace(p.Reason)
	interests, ok := cleanShowcaseStrings(p.Interests, 12, 60)
	if !ok || len([]rune(p.Name)) > 80 || len([]rune(p.Bio)) > 2000 || p.Reason == "" || len([]rune(p.Reason)) > 300 || !oneOf(p.AboutLayout, "classic", "orbit") {
		return out, errors.New("模型建议无效")
	}
	p.Interests = interests
	return out, nil
}

func showcaseAboutMessages(in showcaseAboutRequest) []gateway.ChatMessage {
	profile, _ := json.Marshal(in.Profile)
	messages := []gateway.ChatMessage{
		{Role: gateway.RoleSystem, Content: prompts.ShowcaseAboutSystem},
		{Role: gateway.RoleUser, Content: "当前个人资料（仅可依据这些资料和后续对话）：\n" + string(profile)},
	}
	for _, message := range in.Messages {
		role := gateway.RoleUser
		if message.Role == "assistant" {
			role = gateway.RoleAssistant
		}
		messages = append(messages, gateway.ChatMessage{Role: role, Content: message.Content})
	}
	return messages
}

func (a *API) postPblShowcaseAboutChat(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}
	var input showcaseAboutRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, showcaseAboutRequestMaxBytes)).Decode(&input); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_json", "请求内容无效", nil))
		return
	}
	input, err = normalizeShowcaseAboutRequest(input)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", err.Error(), nil))
		return
	}
	resolved, err := a.routeE(r.Context(), gateway.ClassDialogue)
	if err != nil || a.d.Provider == nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	result, err := gateway.Collect(r.Context(), a.d.Provider, resolved, gateway.ChatRequest{
		Messages:       showcaseAboutMessages(input),
		MaxTokens:      1600,
		ResponseFormat: gateway.ResponseFormatJSONObject,
	})
	a.recordLiteLLMCall(r.Context(), u.ID, uuid.Nil, "showcase_about_chat", resolved, result.Usage)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	output, err := parseShowcaseAboutReply(result.Text)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, output)
}
