package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"mindimprint/api/internal/gateway"
)

const (
	defaultPort    = "8787"
	defaultBaseURL = "https://sub2api.zbrain.cn/v1"
	dashScopeURL   = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	dashScopeModel = "qwen-plus"
)

func main() {
	_ = godotenv.Load(".env.local")

	dashKey := strings.TrimSpace(os.Getenv("DASHSCOPE_API_KEY"))
	key := dashKey
	providerName := "dashscope"
	baseURL := envOr("DASHSCOPE_BASE_URL", dashScopeURL)
	model := envOr("DASHSCOPE_MODEL", dashScopeModel)
	if key == "" {
		key = strings.TrimSpace(os.Getenv("SUB2API_API_KEY"))
		providerName = "sub2api"
		baseURL = envOr("SUB2API_BASE_URL", defaultBaseURL)
		model = strings.TrimSpace(os.Getenv("SUB2API_MODEL"))
	}
	if key == "" {
		slog.Error("web design demo: DASHSCOPE_API_KEY or SUB2API_API_KEY is required")
		os.Exit(1)
	}
	if model == "" {
		slog.Error("web design demo: model is required")
		os.Exit(1)
	}
	port := envOr("WEB_DESIGN_DEMO_PORT", defaultPort)
	resolved := gateway.Resolved{
		Provider: providerName,
		BaseURL:  baseURL,
		Model:    model,
		APIKey:   key,
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           newHandler(gateway.NewCatalogProvider(&http.Client{}), resolved),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	slog.Info("web design AI gateway ready", "address", "http://127.0.0.1:"+port, "provider", resolved.Provider, "model", resolved.Model)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("web design demo server failed", "error", err)
		os.Exit(1)
	}
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

type demoServer struct {
	provider gateway.Provider
	resolved gateway.Resolved
}

func newHandler(provider gateway.Provider, resolved gateway.Resolved) http.Handler {
	s := &demoServer{provider: provider, resolved: resolved}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("POST /api/v1/web-design/recommend", s.recommend)
	mux.HandleFunc("POST /api/v1/web-design/propose-patch", s.proposePatch)
	mux.HandleFunc("POST /api/v1/web-design/generate-design", s.generateDesign)
	mux.HandleFunc("POST /api/v1/web-design/propose-design-patch", s.proposeDesignPatch)
	mux.HandleFunc("POST /api/v1/web-design/analyze-reference-image", s.analyzeReferenceImage)
	return localCORS(mux)
}

func localCORS(next http.Handler) http.Handler {
	allowed := map[string]bool{
		"http://127.0.0.1:8765": true,
		"http://localhost:8765": true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			if origin == "" || !allowed[origin] {
				writeError(w, http.StatusForbidden, "origin_not_allowed", "本地页面来源未获允许")
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *demoServer) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"provider": s.resolved.Provider,
		"model":    s.resolved.Model,
	})
}

type recommendRequest struct {
	Requirement string `json:"requirement"`
}

type webBrief struct {
	Kind            string   `json:"kind"`
	Style           string   `json:"style"`
	Topic           string   `json:"topic"`
	Goal            string   `json:"goal"`
	Audience        []string `json:"audience"`
	RequiredContent []string `json:"requiredContent"`
	Avoid           []string `json:"avoid"`
}

type designDirection struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Score       int    `json:"score"`
	Reason      string `json:"reason"`
	Headline    string `json:"headline"`
	Description string `json:"description"`
	CTA         string `json:"cta"`
}

type recommendResponse struct {
	Brief      webBrief          `json:"brief"`
	Directions []designDirection `json:"directions"`
	Provider   string            `json:"provider,omitempty"`
	Model      string            `json:"model,omitempty"`
	Usage      gateway.ChatUsage `json:"usage"`
}

func (s *demoServer) recommend(w http.ResponseWriter, r *http.Request) {
	var in recommendRequest
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	in.Requirement = strings.TrimSpace(in.Requirement)
	if len([]rune(in.Requirement)) < 8 || len([]rune(in.Requirement)) > 4000 {
		writeError(w, http.StatusBadRequest, "validation_failed", "网页需求应为 8–4000 个字符")
		return
	}

	system := `你是面向中学生项目成果的网页设计教练。请理解用户需求并推荐三个结构和视觉语言明显不同的网页方向。不要编造用户未提供的项目事实。只输出一个 JSON 对象，不要 Markdown。JSON 必须严格符合：
{"brief":{"kind":"短类型名","style":"风格概括","topic":"不超过35字的主题","goal":"主要行动目标","audience":["受众"],"requiredContent":["必要内容"],"avoid":["应避免内容"]},"directions":[{"key":"course|poster|gallery|editorial|future 五选一，三个方向不得重复","name":"方向名称","score":0到100的整数,"reason":"具体匹配理由","headline":"首屏标题","description":"一行辅助说明","cta":"行动按钮"}]}
directions 必须正好三项。名称、标题和理由要与用户需求相关，不能只换颜色。`

	out, err := s.callJSON(r.Context(), system, in.Requirement, 4200)
	if err != nil {
		slog.Warn("web design recommend failed", "error", err, "provider", s.resolved.Provider, "model", s.resolved.Model)
		writeError(w, http.StatusBadGateway, "model_unavailable", "AI 需求分析失败，请稍后重试")
		return
	}
	var response recommendResponse
	if err := json.Unmarshal([]byte(out.Text), &response); err != nil {
		slog.Warn("web design recommend returned invalid JSON", "error", err)
		writeError(w, http.StatusBadGateway, "invalid_model_output", "AI 返回的设计方案无法解析，请重新生成")
		return
	}
	if err := validateRecommendation(&response); err != nil {
		slog.Warn("web design recommend failed validation", "error", err)
		writeError(w, http.StatusBadGateway, "invalid_model_output", "AI 返回的设计方案不完整，请重新生成")
		return
	}
	response.Provider = s.resolved.Provider
	response.Model = s.resolved.Model
	response.Usage = out.Usage
	writeJSON(w, http.StatusOK, response)
}

type patchScope struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Label string `json:"label"`
}

type patchRequest struct {
	Instruction string          `json:"instruction"`
	Scope       patchScope      `json:"scope"`
	Page        json.RawMessage `json:"page"`
}

type patchChange struct {
	ComponentID string `json:"componentId"`
	Action      string `json:"action"`
	Text        string `json:"text,omitempty"`
	Span        int    `json:"span,omitempty"`
}

type patchResponse struct {
	Summary  string            `json:"summary"`
	Before   string            `json:"before"`
	After    string            `json:"after"`
	Changes  []patchChange     `json:"changes"`
	Provider string            `json:"provider,omitempty"`
	Model    string            `json:"model,omitempty"`
	Usage    gateway.ChatUsage `json:"usage"`
}

type pageShape struct {
	Blocks []struct {
		ID    string `json:"id"`
		Items []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
			Span int    `json:"span"`
		} `json:"items"`
	} `json:"blocks"`
}

func (s *demoServer) proposePatch(w http.ResponseWriter, r *http.Request) {
	var in patchRequest
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	in.Instruction = strings.TrimSpace(in.Instruction)
	if len([]rune(in.Instruction)) < 2 || len([]rune(in.Instruction)) > 1000 {
		writeError(w, http.StatusBadRequest, "validation_failed", "局部修改要求应为 2–1000 个字符")
		return
	}
	if in.Scope.Type != "block" && in.Scope.Type != "component" {
		writeError(w, http.StatusBadRequest, "validation_failed", "局部修改只支持 Block 或组件作用域")
		return
	}
	var page pageShape
	if len(in.Page) == 0 || json.Unmarshal(in.Page, &page) != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "页面结构无效")
		return
	}
	allowed := allowedComponents(page, in.Scope)
	if len(allowed) == 0 {
		writeError(w, http.StatusBadRequest, "validation_failed", "没有找到批注所指向的页面对象")
		return
	}

	pageJSON, _ := json.Marshal(json.RawMessage(in.Page))
	user := fmt.Sprintf("修改要求：%s\n作用域：%s（%s）\n页面 JSON：%s", in.Instruction, in.Scope.Label, in.Scope.Type, pageJSON)
	system := `你是网页设计局部优化器。只能修改给定作用域内已有组件，不能改动其他区域，不能添加用户没有提供的事实。只输出一个 JSON 对象，不要 Markdown：
{"summary":"修改概述","before":"修改前的具体状态","after":"修改后的具体状态","changes":[{"componentId":"已有组件ID","action":"replace_text|resize","text":"replace_text 时的新文字","span":2到12的整数}]}
changes 必须为 1–6 项，禁止为空。用户提出任何修改要求时，至少返回 1 项可执行修改；只在确实需要调整文字时使用 replace_text，只在布局需要变化时使用 resize。componentId 必须来自给定页面且位于当前作用域。cards 和 image 组件不能 replace_text。不要输出 HTML、CSS 或 JavaScript。`

	out, err := s.callJSON(r.Context(), system, user, 2600)
	if err != nil {
		slog.Warn("web design patch failed", "error", err, "provider", s.resolved.Provider, "model", s.resolved.Model)
		writeError(w, http.StatusBadGateway, "model_unavailable", "AI 局部优化失败，请稍后重试")
		return
	}
	var response patchResponse
	parsePatch := func(text string) error {
		response = patchResponse{}
		if err := json.Unmarshal([]byte(text), &response); err != nil {
			return err
		}
		return validatePatch(&response, allowed)
	}
	if err := parsePatch(out.Text); err != nil {
		// Some compatible models occasionally omit changes on the first JSON response.
		// Give the same real model one focused repair attempt before falling back.
		repairSystem := system + `
上一版输出未通过校验。必须返回 changes 数组，且至少包含 1 项修改；用户要求修改文字时使用 replace_text，componentId 必须来自页面 JSON 和当前作用域。只输出修正后的 JSON。`
		retry, retryErr := s.callJSON(r.Context(), repairSystem, user, 2600)
		if retryErr != nil {
			slog.Warn("web design patch retry failed", "error", retryErr)
			writeError(w, http.StatusBadGateway, "model_unavailable", "AI 局部优化失败，请稍后重试")
			return
		}
		if retryErr = parsePatch(retry.Text); retryErr != nil {
			slog.Warn("web design patch failed validation after retry", "error", retryErr)
			writeError(w, http.StatusBadGateway, "invalid_model_output", "AI 返回了无法应用的局部修改，请重新生成")
			return
		}
	}
	response.Provider = s.resolved.Provider
	response.Model = s.resolved.Model
	response.Usage = out.Usage
	writeJSON(w, http.StatusOK, response)
}

type visualDesignRequest struct {
	Requirement string            `json:"requirement"`
	References  []json.RawMessage `json:"references"`
}

type visualDesignAttemptDiagnostic struct {
	Attempt      int    `json:"attempt"`
	RawOutput    string `json:"rawOutput,omitempty"`
	ParseError   string `json:"parseError,omitempty"`
	QualityError string `json:"qualityError,omitempty"`
	SchemaError  string `json:"schemaError,omitempty"`
	CallError    string `json:"callError,omitempty"`
}

type visualDesignFailureDiagnostic struct {
	Stage    string                          `json:"stage"`
	Attempts []visualDesignAttemptDiagnostic `json:"attempts"`
}

func visualDesignAttempt(attempt int, raw string, result map[string]any, parseErr, qualityErr error) visualDesignAttemptDiagnostic {
	diagnostic := visualDesignAttemptDiagnostic{Attempt: attempt, RawOutput: raw}
	if parseErr != nil {
		diagnostic.ParseError = parseErr.Error()
	}
	if qualityErr != nil {
		diagnostic.QualityError = qualityErr.Error()
	}
	if parseErr == nil && result["schema"] != "visual-pbl-design-output" {
		diagnostic.SchemaError = fmt.Sprintf("schema is %q, want visual-pbl-design-output", result["schema"])
	}
	return diagnostic
}

func visualPlanAttempt(attempt int, raw string, parseErr, validationErr error) visualDesignAttemptDiagnostic {
	diagnostic := visualDesignAttemptDiagnostic{Attempt: attempt, RawOutput: raw}
	if parseErr != nil {
		diagnostic.ParseError = parseErr.Error()
	}
	if validationErr != nil {
		diagnostic.QualityError = validationErr.Error()
	}
	return diagnostic
}

func (s *demoServer) generateDesign(w http.ResponseWriter, r *http.Request) {
	var in visualDesignRequest
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	in.Requirement = strings.TrimSpace(in.Requirement)
	if len([]rune(in.Requirement)) < 8 || len([]rune(in.Requirement)) > 8000 {
		writeError(w, http.StatusBadRequest, "validation_failed", "设计要求应为 8–8000 个字符")
		return
	}
	referencesJSON, _ := json.Marshal(in.References)
	user := fmt.Sprintf("用户要求：\n%s\n\nReferenceSpecV1 参考资料：\n%s", in.Requirement, referencesJSON)
	referenceIDs := referenceIDsFromRaw(in.References)
	system := `你是视觉 PBL 设计规划引擎。根据用户要求和 ReferenceSpecV1 创作一个紧凑的 VisualPlan，不得选择或套用固定模板。你只负责真实的设计决策；Gateway 会把计划编译为严格的 VisualDesignSpecV1。
必须实际读取每项参考资料的 palette、typography、spacing、pages.nodes、interactions 和 reusablePatterns。白板节点代表信息层级、相对位置和尺寸意图；HTML/截图节点代表可借鉴的配色、结构、组件排列和交互。Prompt 或参考资料改变时，配色、区块顺序、组件组合、坐标和字体层级必须有可观察变化。
不得编造用户未提供的研究成果、数字、姓名、邮箱、网址或机构。事实缺失时使用简短可替换内容并写入 assumptions，最多两处“在此填写”。
只输出一个紧凑 JSON 对象，不要 Markdown、解释或代码围栏。必须使用扁平结构，section 内禁止出现 children，element 内也禁止嵌套任何节点。严格使用以下结构，不得添加字段：
{"title":"页面标题","brief":{"topic":"主题","audience":["受众"],"goal":"目标","constraints":["约束"]},"rationale":"设计理由","theme":{"background":"#HEX","surfaces":["#HEX"],"text":["#HEX"],"accents":["#HEX"],"headingFont":"字体","bodyFont":"字体","baseFontSize":16,"spacingScale":[8,16,24,32,48],"radius":12},"page":{"name":"主页","width":1440,"height":1800,"background":"#HEX","sections":[{"id":"英文稳定ID","type":"header|nav|section|grid|footer","name":"区块名","bounds":[0,0,1440,360],"background":"#HEX","layout":"free|flow|flex|grid","columns":2,"gap":24,"padding":48}],"elements":[{"id":"英文稳定ID","sectionId":"所属section的id","type":"text|shape|card|image|button|link|input|divider|custom","name":"组件名","text":"可见文字","bounds":[48,48,480,80],"background":"#HEX","color":"#HEX","fontSize":48,"fontWeight":700,"lineHeight":1.2,"radius":12,"action":"custom|scroll-to|toggle|show|hide|open-modal|submit|navigate|open-url","targetId":"目标section或element的id","href":"仅真实网址","interactionDescription":"交互说明"}]},"referenceUses":[{"referenceId":"必须原样使用输入referenceId","borrowedPatterns":["实际借鉴内容，最多3项"],"rejectedPatterns":[],"affectedSectionIds":["本计划中的section id"]}],"assumptions":[]}
页面使用 3–10 个语义区块。sections 与 elements 的总数由内容复杂度决定：简单页面通常12–20个，普通页面20–35个，复杂页面才可增加，硬上限60个；绝不为凑数量添加空节点。至少5个有文字的 element 和1个按钮/链接。bounds=[x,y,width,height]，坐标一律是相对页面左上角的全局px，element 必须落在所属section范围内。相同字体与颜色依靠 theme 继承，element 只填写确实不同的视觉字段，以缩短输出。使用2–5个协调颜色并建立标题、正文、辅助文字层级。
referenceUses 必须逐项覆盖输入中的每个 referenceId，明确说明具体借用了什么以及影响哪个区块；不能只写“参考了设计”。没有参考资料时返回空数组。`
	out, err := s.callJSON(r.Context(), system, user, 6000)
	if err != nil {
		slog.Warn("visual design generation failed", "error", err)
		writeErrorWithDetails(w, http.StatusBadGateway, "model_unavailable", "AI 设计生成失败，请稍后重试", visualDesignFailureDiagnostic{
			Stage:    "initial-call",
			Attempts: []visualDesignAttemptDiagnostic{{Attempt: 1, CallError: err.Error()}},
		})
		return
	}
	var plan visualPlan
	var parseErr error
	if out.StopReason == gateway.StopLength {
		parseErr = fmt.Errorf("AI output was truncated at %d bytes (stopReason=length)", len(out.Text))
	} else {
		plan, parseErr = parseVisualPlan(out.Text)
	}
	var planErr error
	if parseErr == nil {
		planErr = validateVisualPlan(plan, referenceIDs)
	}
	attempts := []visualDesignAttemptDiagnostic{visualPlanAttempt(1, out.Text, parseErr, planErr)}
	if parseErr != nil || planErr != nil {
		slog.Warn("visual plan invalid; requesting targeted repair", "parseError", parseErr, "validationError", planErr, "length", len(out.Text))
		issue := parseErr
		if issue == nil {
			issue = planErr
		}
		var repairSystem, repairUser string
		if parseErr != nil {
			repairSystem = `你是 JSON 语法修复器。把输入修复为一个完整、可解析、紧凑的扁平 VisualPlan JSON。保留已有设计意图和文案；禁止 Markdown；禁止递归 children。顶层只能有 title、brief、rationale、theme、page、referenceUses、assumptions。page 只能有 name、width、height、background、sections、elements。section 使用 bounds:[x,y,width,height]；element 使用 sectionId 和 bounds，不得嵌套节点。如果原文被截断，用已有内容完成最小闭合结构，总节点保持12–30个，不要扩写。只输出 JSON。`
			repairUser = fmt.Sprintf("解析错误：%s\n\n待修复的原始输出：\n%s", issue, out.Text)
		} else {
			repairSystem = system + `\n你现在是 VisualPlan 内容定点修复器。保留原设计意图、颜色、布局和文案，只修复指出的问题。仍然只输出扁平、紧凑的完整 VisualPlan JSON。`
			repairUser = fmt.Sprintf("必须修复的问题：%s\n\n原始 VisualPlan：\n%s\n\n原始用户与参考输入：\n%s", issue, out.Text, user)
		}
		retry, retryErr := s.callJSON(r.Context(), repairSystem, repairUser, 5000)
		if retryErr == nil {
			if retry.StopReason == gateway.StopLength {
				parseErr = fmt.Errorf("AI repair output was truncated at %d bytes (stopReason=length)", len(retry.Text))
				planErr = nil
			} else {
				plan, parseErr = parseVisualPlan(retry.Text)
				planErr = nil
				if parseErr == nil {
					planErr = validateVisualPlan(plan, referenceIDs)
				}
			}
			attempts = append(attempts, visualPlanAttempt(2, retry.Text, parseErr, planErr))
		} else {
			attempts = append(attempts, visualDesignAttemptDiagnostic{Attempt: 2, CallError: retryErr.Error()})
		}
		if retryErr != nil || parseErr != nil || planErr != nil {
			slog.Warn("visual plan repair invalid", "callError", retryErr, "parseError", parseErr, "validationError", planErr)
			writeErrorWithDetails(w, http.StatusBadGateway, "invalid_model_output", "AI 返回的 VisualPlan 无法编译，请查看生成诊断", visualDesignFailureDiagnostic{
				Stage:    "plan-validation",
				Attempts: attempts,
			})
			return
		}
	}
	result := compileVisualPlan(plan, referenceIDs)
	qualityErr := validateVisualDesignQuality(result)
	if qualityErr != nil {
		attempts = append(attempts, visualDesignAttemptDiagnostic{Attempt: len(attempts) + 1, QualityError: "Gateway compiler: " + qualityErr.Error()})
		writeErrorWithDetails(w, http.StatusBadGateway, "invalid_compiled_design", "Gateway 编译后的 VisualDesignSpecV1 未通过质量检查", visualDesignFailureDiagnostic{Stage: "gateway-compile", Attempts: attempts})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func validateVisualDesignQuality(result map[string]any) error {
	if result == nil {
		return errors.New("design is empty")
	}
	pages, ok := result["pages"].([]any)
	if !ok || len(pages) == 0 {
		return errors.New("design has no page")
	}
	page, ok := pages[0].(map[string]any)
	if !ok {
		return errors.New("page is invalid")
	}
	nodes, ok := page["nodes"].([]any)
	if !ok || len(nodes) < 12 || len(nodes) > 60 {
		return fmt.Errorf("got %d nodes, want 12-60", len(nodes))
	}

	containerCount := 0
	textCount := 0
	interactiveNodeCount := 0
	hierarchicalCount := 0
	placeholderCount := 0
	backgrounds := make(map[string]bool)
	for _, raw := range nodes {
		node, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		typeName, _ := node["type"].(string)
		switch typeName {
		case "section", "header", "nav", "footer", "group", "grid":
			containerCount++
		case "button", "link", "input":
			interactiveNodeCount++
		}
		if parentID, exists := node["parentId"]; exists && parentID != nil && strings.TrimSpace(fmt.Sprint(parentID)) != "" {
			hierarchicalCount++
		}
		if content, ok := node["content"].(map[string]any); ok {
			if text, _ := content["text"].(string); strings.TrimSpace(text) != "" {
				textCount++
				if strings.Contains(text, "在此填写") {
					placeholderCount++
				}
			}
		}
		if style, ok := node["style"].(map[string]any); ok {
			if background, _ := style["background"].(string); strings.TrimSpace(background) != "" && !strings.EqualFold(background, "transparent") {
				backgrounds[strings.ToUpper(strings.TrimSpace(background))] = true
			}
		}
	}
	interactions, _ := result["interactions"].([]any)
	if containerCount < 3 {
		return errors.New("design lacks semantic sections")
	}
	if hierarchicalCount < 6 {
		return errors.New("design lacks child components")
	}
	if textCount < 5 {
		return errors.New("design lacks visible content")
	}
	if len(backgrounds) < 3 {
		return errors.New("design lacks visual color hierarchy")
	}
	if interactiveNodeCount < 1 || len(interactions) < 1 {
		return errors.New("design lacks an executable interaction")
	}
	if placeholderCount > 2 {
		return errors.New("design contains too many placeholders")
	}
	return nil
}

type visualPatchRequest struct {
	Instruction string          `json:"instruction"`
	Scope       json.RawMessage `json:"scope"`
	Design      json.RawMessage `json:"design"`
}

func (s *demoServer) proposeDesignPatch(w http.ResponseWriter, r *http.Request) {
	var in visualPatchRequest
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	in.Instruction = strings.TrimSpace(in.Instruction)
	if len([]rune(in.Instruction)) < 2 || len([]rune(in.Instruction)) > 4000 || len(in.Design) == 0 {
		writeError(w, http.StatusBadRequest, "validation_failed", "修改要求或设计稿无效")
		return
	}
	var identity struct {
		DesignID string `json:"designId"`
		Revision int    `json:"revision"`
	}
	if json.Unmarshal(in.Design, &identity) != nil || identity.DesignID == "" || identity.Revision < 1 {
		writeError(w, http.StatusBadRequest, "validation_failed", "VisualDesignSpecV1 无效")
		return
	}
	user := fmt.Sprintf("修改要求：%s\n作用域：%s\n当前 VisualDesignSpecV1：%s", in.Instruction, in.Scope, in.Design)
	system := fmt.Sprintf(`你是视觉设计局部修改引擎。只返回可执行 PatchSpecV1 JSON，不要返回解释、Markdown、完整设计稿或未知字段。designId 必须是 %q，baseRevision 必须是 %d。作用域为 node 时只能修改该节点或其子节点；document 时可修改整页。
严格结构：{"schema":"visual-pbl-local-patch","version":"1.0","patchId":"patch.ai.稳定ID","designId":%q,"baseRevision":%d,"scope":{"type":"document|page|section|node","id":"给定ID"},"instruction":"原要求","summary":"具体变化","operations":[],"referenceIds":[],"author":"ai"}。
operations 必须 1–12 项。每项选择一种：
{"operationId":"op.ID","action":"move-node","nodeId":"已有ID","targetPageId":"已有页面ID","targetParentId":null或已有ID,"index":0,"position":{"x":0,"y":0}}
{"operationId":"op.ID","action":"resize-node","nodeId":"已有ID","bounds":{"x":0,"y":0,"width":100,"height":100}}
{"operationId":"op.ID","action":"replace-content","nodeId":"已有ID","content":{"text":"新文本"}}
{"operationId":"op.ID","action":"set-style","nodeId":"已有ID","style":{"background":"#HEX","color":"#HEX","borderRadius":12}}
{"operationId":"op.ID","action":"set-layout","nodeId":"已有ID","layout":{"mode":"flex","direction":"column","gap":24}}
{"operationId":"op.ID","action":"remove-node","nodeId":"已有ID","expectedParentId":null或已有ID}
{"operationId":"op.ID","action":"set-theme","theme":{"palette":{"background":["#HEX"],"surface":["#HEX"],"text":["#HEX"],"accent":["#HEX"]}}}
坐标修改不得让节点越过页面 viewport。不要修改未要求的内容。`, identity.DesignID, identity.Revision, identity.DesignID, identity.Revision)
	out, err := s.callJSON(r.Context(), system, user, 5000)
	if err != nil {
		slog.Warn("visual patch failed", "error", err)
		writeError(w, http.StatusBadGateway, "model_unavailable", "AI Patch 生成失败，请稍后重试")
		return
	}
	result, parseErr := parseModelObject(out.Text)
	if parseErr != nil || result["schema"] != "visual-pbl-local-patch" {
		writeError(w, http.StatusBadGateway, "invalid_model_output", "AI 返回的 PatchSpecV1 无法解析，请重试")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type visionRequest struct {
	Name     string `json:"name"`
	MimeType string `json:"mimeType"`
	DataURL  string `json:"dataUrl"`
}

func (s *demoServer) analyzeReferenceImage(w http.ResponseWriter, r *http.Request) {
	var in visionRequest
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	if !strings.HasPrefix(in.DataURL, "data:image/") || len(in.DataURL) > 7<<20 {
		writeError(w, http.StatusBadRequest, "validation_failed", "截图数据无效或超过 7MB")
		return
	}
	model := envOr("DASHSCOPE_VISION_MODEL", "qwen3-vl-flash")
	system := `你是网页视觉分析器。分析截图中的整体配色、字体层级、间距、区块结构、组件排列和可推断交互。坐标使用0到1000归一化值。只输出紧凑JSON：{"summary":"总结","pageType":"类型","palette":["#HEX"],"fonts":["推断字体类别"],"fontSizes":[数字],"spacing":[数字],"patterns":["布局模式"],"interactions":["可推断交互"],"components":[{"id":"稳定英文ID","type":"hero|section|nav|header|footer|grid|card|text|image|button|shape","name":"名称","text":"可见短文字或语义","x":0,"y":0,"width":1000,"height":200,"background":"#HEX","color":"#HEX"}]}。components 4–16项，禁止编造看不见的文字或链接。`
	payload := map[string]any{
		"model": model, "max_tokens": 1800, "temperature": 0.15, "response_format": map[string]string{"type": "json_object"},
		"messages": []any{
			map[string]any{"role": "system", "content": system},
			map[string]any{"role": "user", "content": []any{map[string]any{"type": "text", "text": "分析这张网页或视觉参考截图：" + in.Name}, map[string]any{"type": "image_url", "image_url": map[string]string{"url": in.DataURL}}}},
		},
	}
	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	endpoint := strings.TrimRight(s.resolved.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusBadGateway, "vision_unavailable", "视觉分析请求创建失败")
		return
	}
	req.Header.Set("Authorization", "Bearer "+s.resolved.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Warn("vision request failed", "error", err)
		writeError(w, http.StatusBadGateway, "vision_unavailable", "视觉模型暂不可用")
		return
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Warn("vision provider rejected request", "status", resp.StatusCode)
		writeError(w, http.StatusBadGateway, "vision_unavailable", "视觉模型未开放或暂不可用")
		return
	}
	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(responseBody, &completion) != nil || len(completion.Choices) == 0 {
		writeError(w, http.StatusBadGateway, "invalid_model_output", "视觉模型返回无法解析")
		return
	}
	result, parseErr := parseModelObject(completion.Choices[0].Message.Content)
	if parseErr != nil {
		repairSystem := `你是 JSON 修复器。把输入的截图分析修复为完整紧凑 JSON，只保留 summary、pageType、palette、fonts、fontSizes、spacing、patterns、interactions、components。components 最多10项，每项只保留 id,type,name,text,x,y,width,height,background,color。只输出 JSON。`
		repaired, repairErr := s.callJSON(r.Context(), repairSystem, completion.Choices[0].Message.Content, 3000)
		if repairErr == nil {
			result, parseErr = parseModelObject(repaired.Text)
		}
		if repairErr != nil || parseErr != nil {
			slog.Warn("vision JSON repair failed", "callError", repairErr, "parseError", parseErr)
			writeError(w, http.StatusBadGateway, "invalid_model_output", "视觉分析 JSON 无法解析")
			return
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func parseModelObject(text string) (map[string]any, error) {
	value := stripJSONFence(strings.TrimSpace(text))
	var quoted string
	if strings.HasPrefix(value, `"`) && json.Unmarshal([]byte(value), &quoted) == nil {
		value = quoted
	}
	start, end := strings.Index(value, "{"), strings.LastIndex(value, "}")
	if start >= 0 && end > start {
		value = value[start : end+1]
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *demoServer) callJSON(parent context.Context, system, user string, maxTokens int) (gateway.ChatResult, error) {
	if s.provider == nil || strings.TrimSpace(s.resolved.APIKey) == "" {
		return gateway.ChatResult{}, errors.New("provider is not configured")
	}
	ctx, cancel := context.WithTimeout(parent, 180*time.Second)
	defer cancel()
	temperature := 0.35
	responseFormat := gateway.ResponseFormatJSONObject
	// Some DashScope-compatible models reject response_format=json_object.
	// The design endpoint validates and repairs JSON itself, so omit only this
	// optional wire hint for DashScope while retaining it for other providers.
	if s.resolved.Provider == "dashscope" {
		responseFormat = ""
	}
	out, err := gateway.Collect(ctx, s.provider, s.resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: system},
			{Role: gateway.RoleUser, Content: user},
		},
		MaxTokens:      maxTokens,
		Temperature:    &temperature,
		ResponseFormat: responseFormat,
	})
	if err != nil {
		return gateway.ChatResult{}, err
	}
	out.Text = stripJSONFence(strings.TrimSpace(out.Text))
	if out.Text == "" {
		return gateway.ChatResult{}, errors.New("provider returned empty text")
	}
	return out, nil
}

func stripJSONFence(value string) string {
	if !strings.HasPrefix(value, "```") {
		return value
	}
	lines := strings.Split(value, "\n")
	if len(lines) >= 3 && strings.HasPrefix(lines[0], "```") && strings.TrimSpace(lines[len(lines)-1]) == "```" {
		return strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
	}
	return value
}

func validateRecommendation(response *recommendResponse) error {
	if strings.TrimSpace(response.Brief.Topic) == "" || strings.TrimSpace(response.Brief.Goal) == "" {
		return errors.New("brief topic and goal are required")
	}
	if len(response.Directions) != 3 {
		return fmt.Errorf("got %d directions, want 3", len(response.Directions))
	}
	validKeys := map[string]bool{"course": true, "poster": true, "gallery": true, "editorial": true, "future": true}
	seen := map[string]bool{}
	for i := range response.Directions {
		d := &response.Directions[i]
		d.Key = strings.TrimSpace(d.Key)
		if !validKeys[d.Key] || seen[d.Key] {
			return fmt.Errorf("invalid or duplicate direction key %q", d.Key)
		}
		seen[d.Key] = true
		if d.Score < 0 || d.Score > 100 || strings.TrimSpace(d.Name) == "" || strings.TrimSpace(d.Reason) == "" || strings.TrimSpace(d.Headline) == "" {
			return fmt.Errorf("direction %d is incomplete", i)
		}
	}
	return nil
}

type componentRule struct {
	Type string
}

func allowedComponents(page pageShape, scope patchScope) map[string]componentRule {
	allowed := make(map[string]componentRule)
	for _, block := range page.Blocks {
		if scope.Type == "block" && block.ID != scope.ID {
			continue
		}
		for _, item := range block.Items {
			if scope.Type == "component" && item.ID != scope.ID {
				continue
			}
			allowed[item.ID] = componentRule{Type: item.Type}
		}
	}
	return allowed
}

func validatePatch(response *patchResponse, allowed map[string]componentRule) error {
	if strings.TrimSpace(response.Summary) == "" || strings.TrimSpace(response.Before) == "" || strings.TrimSpace(response.After) == "" {
		return errors.New("patch descriptions are required")
	}
	if len(response.Changes) == 0 || len(response.Changes) > 6 {
		return fmt.Errorf("got %d changes, want 1-6", len(response.Changes))
	}
	for _, change := range response.Changes {
		rule, ok := allowed[change.ComponentID]
		if !ok {
			return fmt.Errorf("component %q is outside the scope", change.ComponentID)
		}
		switch change.Action {
		case "replace_text":
			if rule.Type == "cards" || rule.Type == "image" || strings.TrimSpace(change.Text) == "" {
				return fmt.Errorf("component %q cannot receive replacement text", change.ComponentID)
			}
		case "resize":
			if change.Span < 2 || change.Span > 12 {
				return fmt.Errorf("component %q span is outside 2-12", change.ComponentID)
			}
		default:
			return fmt.Errorf("unsupported action %q", change.Action)
		}
	}
	return nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "请求 JSON 无效")
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message}})
}

func writeErrorWithDetails(w http.ResponseWriter, status int, code, message string, details any) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "details": details}})
}
