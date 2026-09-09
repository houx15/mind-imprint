package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mindimprint/api/internal/gateway"
)

func scriptedProvider(text string) *gateway.StubProvider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: text},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 120, OutputTokens: 80}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

func testResolved() gateway.Resolved {
	return gateway.Resolved{Provider: "sub2api", Model: "test-model", APIKey: "test-key"}
}

func TestRecommendUsesGatewayAndReturnsValidatedDirections(t *testing.T) {
	reply := `{"brief":{"kind":"课程页","style":"清晰现代","topic":"社区调研 PBL","goal":"推动预约","audience":["学生","家长"],"requiredContent":["路径","成果"],"avoid":["普通官网"]},"directions":[{"key":"course","name":"探索路径图","score":96,"reason":"突出过程","headline":"让学习被看见","description":"展示路径和成果","cta":"预约体验"},{"key":"editorial","name":"独立编辑部","score":91,"reason":"强调内容证据","headline":"真实问题，真实研究","description":"像专题报道一样展开","cta":"查看项目"},{"key":"gallery","name":"成果展厅","score":87,"reason":"集中呈现作品","headline":"从调查走向行动","description":"展示学生成果","cta":"浏览成果"}]}`
	provider := scriptedProvider(reply)
	h := newHandler(provider, testResolved())
	body := bytes.NewBufferString(`{"requirement":"面向初中生和家长的社区调研课程页"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/web-design/recommend", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got recommendResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got.Directions) != 3 || got.Provider != "sub2api" || got.Usage.InputTokens != 120 {
		t.Fatalf("unexpected response: %+v", got)
	}
	if provider.LastRequest.ResponseFormat != gateway.ResponseFormatJSONObject || len(provider.LastRequest.Messages) != 2 {
		t.Fatalf("gateway request was not JSON constrained: %+v", provider.LastRequest)
	}
}

func TestProposePatchRejectsChangesOutsideScope(t *testing.T) {
	reply := `{"summary":"调整标题","before":"标题较短","after":"标题更清晰","changes":[{"componentId":"outside","action":"replace_text","text":"新标题"}]}`
	h := newHandler(scriptedProvider(reply), testResolved())
	body := bytes.NewBufferString(`{"instruction":"突出标题","scope":{"type":"component","id":"c1","label":"组件：heading"},"page":{"blocks":[{"id":"b1","items":[{"id":"c1","type":"heading","span":8}]}]}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/web-design/propose-patch", body)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body = %s", rec.Code, rec.Body.String())
	}
}

func TestLocalCORSAllowsDemoOrigin(t *testing.T) {
	h := newHandler(scriptedProvider(`{}`), testResolved())
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/web-design/recommend", nil)
	req.Header.Set("Origin", "http://127.0.0.1:8765")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "http://127.0.0.1:8765" {
		t.Fatalf("unexpected CORS response: status=%d origin=%q", rec.Code, rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestGenerateDesignCompilesVisualPlanIntoExecutableSpec(t *testing.T) {
	reply := `{"title":"社区研究作品页","brief":{"topic":"社区调研","audience":["学生","家长"],"goal":"展示研究过程","constraints":["不编造数据"]},"rationale":"采用分段叙事和暖色强调研究过程。","theme":{"background":"#F7F3EC","surfaces":["#FFFFFF","#E8EFEA"],"text":["#1F2933","#59636E"],"accents":["#C65D3B"],"headingFont":"system-ui","bodyFont":"system-ui","baseFontSize":16,"spacingScale":[8,16,24,32,48],"radius":14},"page":{"name":"主页","width":1440,"height":1400,"background":"#F7F3EC","sections":[{"id":"hero","type":"header","name":"首屏","x":0,"y":0,"width":1440,"height":420,"background":"#F7F3EC","layout":"free","children":[{"id":"hero-title","type":"text","name":"主标题","text":"观察社区，提出真实问题","x":80,"y":90,"width":760,"height":90,"color":"#1F2933","fontSize":56,"fontWeight":700,"lineHeight":1.2},{"id":"hero-copy","type":"text","name":"简介","text":"记录调查、证据与行动。","x":80,"y":210,"width":620,"height":60,"color":"#59636E","fontSize":20,"fontWeight":400,"lineHeight":1.5},{"id":"hero-button","type":"button","name":"查看过程","text":"查看研究过程","x":80,"y":300,"width":180,"height":52,"background":"#C65D3B","color":"#FFFFFF","fontSize":16,"fontWeight":600,"lineHeight":1.2,"radius":12,"action":"scroll-to","targetId":"process"}]},{"id":"process","type":"section","name":"研究过程","x":0,"y":420,"width":1440,"height":520,"background":"#FFFFFF","layout":"grid","columns":3,"children":[{"id":"question","type":"card","name":"研究问题","text":"从一个值得追问的问题开始","x":80,"y":500,"width":380,"height":220,"background":"#E8EFEA","color":"#1F2933"},{"id":"evidence","type":"card","name":"证据","text":"整理访谈、观察与资料","x":530,"y":500,"width":380,"height":220,"background":"#F7F3EC","color":"#1F2933"},{"id":"action","type":"card","name":"行动","text":"把发现转化为具体行动","x":980,"y":500,"width":380,"height":220,"background":"#E8EFEA","color":"#1F2933"}]},{"id":"contact","type":"footer","name":"联系区","x":0,"y":940,"width":1440,"height":360,"background":"#1F2933","layout":"free","children":[{"id":"contact-title","type":"text","name":"联系标题","text":"一起讨论下一次研究","x":80,"y":1020,"width":600,"height":70,"color":"#FFFFFF","fontSize":38,"fontWeight":700,"lineHeight":1.2},{"id":"contact-copy","type":"text","name":"联系说明","text":"保留问题，继续观察与交流。","x":80,"y":1110,"width":600,"height":60,"color":"#FFFFFF","fontSize":18,"fontWeight":400,"lineHeight":1.5},{"id":"contact-shape","type":"shape","name":"装饰色块","x":980,"y":1020,"width":280,"height":160,"background":"#C65D3B","radius":20}] }]},"referenceUses":[{"referenceId":"reference.test","borrowedPatterns":["暖色强调","三段式叙事"],"rejectedPatterns":[],"affectedSectionIds":["hero","process"]}],"assumptions":["联系信息由用户后续补充"]}`
	provider := scriptedProvider(reply)
	h := newHandler(provider, testResolved())
	body := bytes.NewBufferString(`{"requirement":"制作一个展示社区调研过程的个人页面","references":[{"referenceId":"reference.test"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/web-design/generate-design", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var result map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result["schema"] != "visual-pbl-design-output" {
		t.Fatalf("schema = %v", result["schema"])
	}
	if _, ok := result["brief"].(map[string]any); !ok {
		t.Fatalf("brief was not compiled as an object: %#v", result["brief"])
	}
	if err := validateVisualDesignQuality(result); err != nil {
		t.Fatalf("compiled design quality: %v", err)
	}
	pages := result["pages"].([]any)
	nodes := pages[0].(map[string]any)["nodes"].([]any)
	if len(nodes) != 12 {
		t.Fatalf("compiled nodes = %d, want exactly the 12 planned nodes", len(nodes))
	}
}

func TestFlatVisualPlanLinksElementsWithoutRecursiveChildren(t *testing.T) {
	plan := visualPlan{
		Title: "扁平计划",
		Brief: visualPlanBrief{Topic: "作品集", Goal: "展示作品"},
		Theme: visualPlanTheme{Background: "#F7F3EC", Surfaces: []string{"#FFFFFF"}, Text: []string{"#1F2933"}, Accents: []string{"#C65D3B"}},
		Page: visualPlanPage{Name: "主页", Width: 1440, Height: 1200, Sections: []visualPlanSection{
			{ID: "hero", Type: "header", Name: "首屏", Bounds: []float64{0, 0, 1440, 400}},
			{ID: "work", Type: "section", Name: "作品", Bounds: []float64{0, 400, 1440, 500}},
			{ID: "footer", Type: "footer", Name: "页脚", Bounds: []float64{0, 900, 1440, 300}},
		}},
	}
	for i := 0; i < 9; i++ {
		sectionID := []string{"hero", "work", "footer"}[i/3]
		typeName := "text"
		action := ""
		if i == 2 {
			typeName, action = "button", "custom"
		}
		plan.Page.Elements = append(plan.Page.Elements, visualPlanNode{
			ID: "element-" + string(rune('a'+i)), SectionID: sectionID, Type: typeName,
			Name: "元素", Text: "可见内容", Bounds: []float64{80 + float64(i%3)*360, float64(i/3)*400 + 80, 300, 60}, Action: action,
		})
	}
	if err := normalizeFlatVisualPlan(&plan); err != nil {
		t.Fatal(err)
	}
	if len(plan.Page.Sections[0].Children) != 3 || len(plan.Page.Sections[1].Children) != 3 || len(plan.Page.Sections[2].Children) != 3 {
		t.Fatalf("flat elements were not linked to sections: %#v", plan.Page.Sections)
	}
	if err := validateVisualPlan(plan, nil); err != nil {
		t.Fatal(err)
	}
	result := compileVisualPlan(plan, nil)
	if err := validateVisualDesignQuality(result); err != nil {
		t.Fatalf("compiled flat plan: %v", err)
	}
}

