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

type showcaseGuideContext struct {
	Name             string   `json:"name"`
	Bio              string   `json:"bio"`
	Tagline          string   `json:"tagline"`
	HeroTitle        string   `json:"heroTitle"`
	Interests        []string `json:"interests"`
	Style            string   `json:"style"`
	Layout           string   `json:"layout"`
	Palette          string   `json:"palette"`
	Font             string   `json:"font"`
	AboutLayout      string   `json:"aboutLayout"`
	InterestTreeMode string   `json:"interestTreeMode"`
	PortfolioLayout  string   `json:"portfolioLayout"`
	WritingStyle     string   `json:"writingStyle"`
	ReadingStyle     string   `json:"readingStyle"`
	HomeWorkLimit    int      `json:"homeWorkLimit"`
}

type showcaseGuideRequest struct {
	Stage    string                `json:"stage"`
	Messages []showcaseChatMessage `json:"messages"`
	Context  showcaseGuideContext  `json:"context"`
}

type showcaseGuideProposal struct {
	Style            string   `json:"style,omitempty"`
	Layout           string   `json:"layout,omitempty"`
	Palette          string   `json:"palette,omitempty"`
	Font             string   `json:"font,omitempty"`
	HeroTitle        string   `json:"heroTitle,omitempty"`
	Tagline          string   `json:"tagline,omitempty"`
	HeroImagePrompt  string   `json:"heroImagePrompt,omitempty"`
	Name             string   `json:"name,omitempty"`
	Bio              string   `json:"bio,omitempty"`
	Interests        []string `json:"interests,omitempty"`
	AboutLayout      string   `json:"aboutLayout,omitempty"`
	InterestTreeMode string   `json:"interestTreeMode,omitempty"`
	PortfolioLayout  string   `json:"portfolioLayout,omitempty"`
	WritingStyle     string   `json:"writingStyle,omitempty"`
	ReadingStyle     string   `json:"readingStyle,omitempty"`
	HomeWorkLimit    int      `json:"homeWorkLimit,omitempty"`
	Reason           string   `json:"reason"`
}

type showcaseGuideReply struct {
	Reply    string                 `json:"reply"`
	Proposal *showcaseGuideProposal `json:"proposal,omitempty"`
}

var showcaseGuideFields = map[string]map[string]bool{
	"design":     {"style": true, "layout": true, "palette": true, "font": true},
	"hero":       {"heroTitle": true, "tagline": true, "heroImagePrompt": true},
	"profile":    {"name": true, "bio": true, "interests": true, "aboutLayout": true, "interestTreeMode": true},
	"works":      {"portfolioLayout": true, "writingStyle": true, "readingStyle": true, "homeWorkLimit": true},
	"components": {},
	"finish":     {},
}

func normalizeShowcaseGuideRequest(in showcaseGuideRequest) (showcaseGuideRequest, error) {
	if _, ok := showcaseGuideFields[in.Stage]; !ok || len(in.Messages) == 0 || len(in.Messages) > 20 {
		return in, errors.New("设计向导请求无效")
	}
	total := 0
	for i := range in.Messages {
		m := &in.Messages[i]
		m.Content = strings.TrimSpace(m.Content)
		total += len([]rune(m.Content))
		if !oneOf(m.Role, "user", "assistant") || m.Content == "" || len([]rune(m.Content)) > 2000 || total > 20000 {
			return in, errors.New("设计向导对话无效")
		}
	}
	if in.Messages[len(in.Messages)-1].Role != "user" {
		return in, errors.New("最后一条消息必须来自学生")
	}
	context, _ := json.Marshal(in.Context)
	if len(context) > 5000 || len(in.Context.Interests) > 12 {
		return in, errors.New("主页资料超过长度限制")
	}
	return in, nil
}

func parseShowcaseGuideReply(raw, stage string) (showcaseGuideReply, error) {
	var out showcaseGuideReply
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &out); err != nil {
		return out, errors.New("模型返回内容无法解析")
	}
	out.Reply = strings.TrimSpace(out.Reply)
	if out.Reply == "" || len([]rune(out.Reply)) > 1200 {
		return out, errors.New("模型回复无效")
	}
	if out.Proposal == nil {
		return out, nil
	}
	var envelope struct {
		Proposal map[string]json.RawMessage `json:"proposal"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return out, errors.New("模型建议无效")
	}
	allowed := showcaseGuideFields[stage]
	fields := 0
	for key := range envelope.Proposal {
		if key == "reason" {
			continue
		}
		if !allowed[key] {
			return out, errors.New("模型建议包含当前步骤以外的字段")
		}
		fields++
	}
	p := out.Proposal
	p.Reason = strings.TrimSpace(p.Reason)
	if fields == 0 || p.Reason == "" || len([]rune(p.Reason)) > 300 || len([]rune(p.HeroTitle)) > 200 || len([]rune(p.Tagline)) > 200 || len([]rune(p.HeroImagePrompt)) > 2000 || len([]rune(p.Name)) > 80 || len([]rune(p.Bio)) > 2000 {
		return out, errors.New("模型建议无效")
	}
	if p.Style != "" && !oneOf(p.Style, "classic", "minimal", "cute", "dark", "anime", "mecha") ||
		p.Layout != "" && !oneOf(p.Layout, "folio", "journal", "studio") ||
		p.Palette != "" && !oneOf(p.Palette, "paper", "forest", "ocean", "rose", "night", "sunshine") ||
		p.Font != "" && !oneOf(p.Font, "sans", "serif", "mono") ||
		p.AboutLayout != "" && !oneOf(p.AboutLayout, "classic", "orbit") ||
		p.InterestTreeMode != "" && !oneOf(p.InterestTreeMode, "none", "tree", "keywords") ||
		p.PortfolioLayout != "" && !oneOf(p.PortfolioLayout, "sections", "flow", "timeline", "film", "planets", "cloud", "calendar", "list") ||
		p.WritingStyle != "" && !oneOf(p.WritingStyle, "cards", "list") ||
		p.ReadingStyle != "" && !oneOf(p.ReadingStyle, "shelf", "list") ||
		p.HomeWorkLimit != 0 && p.HomeWorkLimit != 3 && p.HomeWorkLimit != 6 && p.HomeWorkLimit != 9 && p.HomeWorkLimit != 12 {
		return out, errors.New("模型建议无效")
	}
	if p.Interests != nil {
		if _, ok := cleanShowcaseStrings(p.Interests, 12, 60); !ok {
			return out, errors.New("模型兴趣建议无效")
		}
	}
	return out, nil
}

func (a *API) postPblShowcaseGuideChat(w http.ResponseWriter, r *http.Request) {
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
	var input showcaseGuideRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&input); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_json", "请求内容无效", nil))
		return
	}
	input, err = normalizeShowcaseGuideRequest(input)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", err.Error(), nil))
		return
	}
	contextJSON, _ := json.Marshal(input.Context)
	messages := []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: prompts.ShowcaseGuideSystem}, {Role: gateway.RoleUser, Content: "当前步骤：" + input.Stage + "。当前草稿的可讨论资料：" + string(contextJSON)}}
	if input.Stage == "profile" {
		if tree, treeErr := a.showcaseInterestTree(r.Context(), u.ID); treeErr == nil {
			keywords, _ := json.Marshal(tree.Keywords)
			messages = append(messages, gateway.ChatMessage{Role: gateway.RoleUser, Content: "学生兴趣树中的关键词（仅用于建议，公开与否由学生决定）：" + string(keywords)})
		}
	}
	for _, message := range input.Messages {
		role := gateway.RoleUser
		if message.Role == "assistant" {
			role = gateway.RoleAssistant
		}
		messages = append(messages, gateway.ChatMessage{Role: role, Content: message.Content})
	}
	resolved, err := a.routeE(r.Context(), gateway.ClassDialogue)
	if err != nil || a.d.Provider == nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	result, err := gateway.Collect(r.Context(), a.d.Provider, resolved, gateway.ChatRequest{Messages: messages, MaxTokens: 2000, ResponseFormat: gateway.ResponseFormatJSONObject})
	a.recordLiteLLMCall(r.Context(), u.ID, uuid.Nil, "showcase_guide_chat", resolved, result.Usage)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	output, err := parseShowcaseGuideReply(result.Text, input.Stage)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, output)
}

type showcaseComponentRequest struct {
	Prompt  string `json:"prompt"`
	Format  string `json:"format"`
	Style   string `json:"style"`
	Palette string `json:"palette"`
}
type showcaseGeneratedComponent struct {
	Title       string `json:"title"`
	Source      string `json:"source"`
	Height      int    `json:"height"`
	Placement   string `json:"placement"`
	Explanation string `json:"explanation"`
}

func normalizeShowcaseComponentRequest(in showcaseComponentRequest) (showcaseComponentRequest, error) {
	in.Prompt = strings.TrimSpace(in.Prompt)
	if len([]rune(in.Prompt)) < 4 || len([]rune(in.Prompt)) > 1000 || !oneOf(in.Format, "svg", "html") || !oneOf(in.Style, "classic", "minimal", "cute", "dark", "anime", "mecha") || !oneOf(in.Palette, "paper", "forest", "ocean", "rose", "night", "sunshine") {
		return in, errors.New("组件描述或格式无效")
	}
	return in, nil
}

func parseShowcaseGeneratedComponent(raw, format string) (showcaseGeneratedComponent, error) {
	var out showcaseGeneratedComponent
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &out); err != nil {
		return out, errors.New("模型返回内容无法解析")
	}
	out.Title, out.Source, out.Explanation = strings.TrimSpace(out.Title), strings.TrimSpace(out.Source), strings.TrimSpace(out.Explanation)
	if out.Title == "" || len([]rune(out.Title)) > 80 || len(out.Source) == 0 || len(out.Source) > 100<<10 || out.Height < 160 || out.Height > 800 || !oneOf(out.Placement, "after-about", "after-works") || out.Explanation == "" || len([]rune(out.Explanation)) > 300 {
		return out, errors.New("模型生成的组件无效")
	}
	if format == "svg" && !validShowcaseSVG(out.Source) {
		return out, errors.New("模型生成的 SVG 无效")
	}
	if format == "html" && !strings.Contains(strings.ToLower(out.Source), "<canvas") {
		return out, errors.New("模型生成的 Canvas 无效")
	}
	return out, nil
}

func (a *API) postPblShowcaseComponentGenerate(w http.ResponseWriter, r *http.Request) {
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
	var input showcaseComponentRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&input); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_json", "请求内容无效", nil))
		return
	}
	input, err = normalizeShowcaseComponentRequest(input)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", err.Error(), nil))
		return
	}
	resolved, err := a.routeE(r.Context(), gateway.ClassCompose)
	if err != nil || a.d.Provider == nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	inputJSON, _ := json.Marshal(input)
	result, err := gateway.Collect(r.Context(), a.d.Provider, resolved, gateway.ChatRequest{Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: prompts.ShowcaseComponentSystem}, {Role: gateway.RoleUser, Content: string(inputJSON)}}, MaxTokens: 6000, ResponseFormat: gateway.ResponseFormatJSONObject})
	a.recordLiteLLMCall(r.Context(), u.ID, uuid.Nil, "showcase_component_generate", resolved, result.Usage)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	component, err := parseShowcaseGeneratedComponent(result.Text, input.Format)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, component)
}
