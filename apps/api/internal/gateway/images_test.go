package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

/* ── 启动即失败 ───────────────────────────────────────────────────────── */

// 🚨 给 draw 绑一个聊天模型，启动就要失败。
//
// 不拦的话，第一次生成头图才会发现——而那一刻错的是学生的那一屏，不是我们的
// 终端。这条和「assess 必须是旗舰」是同一类闸：目录里改错一个字，启动就说话。
func TestCatalog_DrawMustBindAnImageModel(t *testing.T) {
	body := `{"providers":{"p":{"kind":"openai_compatible","baseUrl":"u","apiKeyEnv":"K"}},
		"models":{"p/m":{"provider":"p","model":"m","flagship":true},
		          "p/img":{"provider":"p","model":"img","capabilities":["image"]}},
		"lanes":{"reflex":{"model":"p/m","tier":"chaperone"},
			"dialogue":{"model":"p/m","tier":"chaperone"},
			"compose":{"model":"p/m","tier":"chaperone"},
			"review":{"model":"p/m","tier":"flagship"},
			"assess":{"model":"p/m","tier":"flagship"},
			"digest":{"model":"p/m","tier":"chaperone"},
			"draw":{"model":"p/m","tier":"chaperone"}}}`
	_, err := ParseCatalog([]byte(body))
	if err == nil {
		t.Fatal("给 draw 绑了一个聊天模型，应该启动即失败")
	}
	if !strings.Contains(err.Error(), "not an image model") {
		t.Errorf("报错没说清楚原因：%v", err)
	}
}

// 反过来：给一个聊天档绑图像模型，也要失败。这条本来就有，这里只是确认加了
// draw 之后它没被那个新分支绕过去。
func TestCatalog_AChatClassStillRefusesAnImageModel(t *testing.T) {
	body := `{"providers":{"p":{"kind":"openai_compatible","baseUrl":"u","apiKeyEnv":"K"}},
		"models":{"p/m":{"provider":"p","model":"m","flagship":true},
		          "p/img":{"provider":"p","model":"img","capabilities":["image"]}},
		"lanes":{"reflex":{"model":"p/img","tier":"chaperone"},
			"dialogue":{"model":"p/m","tier":"chaperone"},
			"compose":{"model":"p/m","tier":"chaperone"},
			"review":{"model":"p/m","tier":"flagship"},
			"assess":{"model":"p/m","tier":"flagship"},
			"digest":{"model":"p/m","tier":"chaperone"},
			"draw":{"model":"p/img","tier":"chaperone"}}}`
	if _, err := ParseCatalog([]byte(body)); err == nil {
		t.Fatal("reflex 绑了图像模型，应该启动即失败")
	}
}

// 真实目录必须绑着 draw，而且绑的是一个真能画图的模型。
func TestRealCatalog_BindsDraw(t *testing.T) {
	cat, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	spec, ok := cat.Lanes[ClassDraw]
	if !ok {
		t.Fatal("models.json 没有绑 draw —— 受众画像和头图都没路可走")
	}
	m, ok := cat.Models[spec.Model]
	if !ok {
		t.Fatalf("draw 绑的 %q 不在目录里", spec.Model)
	}
	if !m.Has(CapImage) {
		t.Errorf("draw 绑的 %q 不会画图", spec.Model)
	}
}

/* ── 回包解析 ─────────────────────────────────────────────────────────── */

// 回包形状是 2026-09-04 实测抄下来的，不是照 OpenAI 兼容口想出来的。
// 见 images.go 文件头那张实测表。
func TestParseDrawResponse_ReadsTheNativeShape(t *testing.T) {
	raw := `{"output":{"choices":[{"finish_reason":"stop","message":{"role":"assistant",
		"content":[{"image":"https://dashscope-a717.oss-accelerate.aliyuncs.com/x.png?Expires=1788538749","type":"image"}]}}]},
		"usage":{"output_image_count":1}}`
	got, err := ParseDrawResponse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if got.URL == "" || !strings.Contains(got.URL, "x.png") {
		t.Errorf("没把图的地址读出来：%+v", got)
	}
}

// 200 里也可能带 code —— 上游用它报「内容被拦下了」这类事。当成成功会让一张
// 不存在的图一路走到她的页面上。
func TestParseDrawResponse_TreatsAnInBodyCodeAsFailure(t *testing.T) {
	_, err := ParseDrawResponse([]byte(`{"code":"DataInspectionFailed","message":"blocked"}`))
	if err == nil {
		t.Fatal("正文里带 code 的回包不该算成功")
	}
	if !strings.Contains(err.Error(), "DataInspectionFailed") {
		t.Errorf("报错丢了上游原话：%v", err)
	}
}

// 空回包要报错，不要回一个空 URL。一个空 URL 会一路走到前端，变成一张碎图，
// 而没有任何一层说过出了什么事。
func TestParseDrawResponse_RefusesAnEmptyAnswer(t *testing.T) {
	for _, raw := range []string{`{"output":{"choices":[]}}`, `{"output":{}}`, `{}`, `not json`} {
		if _, err := ParseDrawResponse([]byte(raw)); err == nil {
			t.Errorf("%q 应该被拒", raw)
		}
	}
}

/* ── 请求体 ───────────────────────────────────────────────────────────── */

// 打的是 BaseURL 给的那个原生端点；key 在 Authorization 上。
func TestHTTPDrawer_PostsTheNativeShape(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"output":{"choices":[{"message":{"content":[{"image":"https://x/y.png"}]}}]}}`))
	}))
	defer srv.Close()

	d := &HTTPDrawer{Client: srv.Client()}
	got, err := d.Draw(context.Background(),
		Resolved{BaseURL: srv.URL + "/api/v1/services/aigc/multimodal-generation/generation",
			Model: "qwen-image-3.0", APIKey: "secret"},
		DrawRequest{Prompt: "一个在读工程类专业的大学生"})
	if err != nil {
		t.Fatal(err)
	}
	// 🚨 端点整个来自 BaseURL，不再往后拼 /images/generations —— 那条路实测 404。
	if gotPath != "/api/v1/services/aigc/multimodal-generation/generation" {
		t.Errorf("打错端点了：%s", gotPath)
	}
	if gotAuth != "Bearer secret" {
		t.Errorf("Authorization 不对：%q", gotAuth)
	}
	// 提示词裹在 input.messages[].content[] 这个**数组**里。给字符串会 400。
	if gotBody["model"] != "qwen-image-3.0" {
		t.Errorf("请求体的 model 不对：%v", gotBody)
	}
	if params, _ := gotBody["parameters"].(map[string]any); params == nil || params["size"] != DefaultDrawSize {
		t.Errorf("尺寸没进 parameters：%v", gotBody)
	}
	if _, ok := gotBody["input"].(map[string]any); !ok {
		t.Errorf("请求体没有 input：%v", gotBody)
	}
	if got.URL != "https://x/y.png" {
		t.Errorf("没把 url 读回来：%+v", got)
	}
}

// 🚨 上游报错时，错误里带的是状态码和正文摘要，给服务端日志看。
func TestHTTPDrawer_SurfacesAnUpstreamFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"model not activated"}}`))
	}))
	defer srv.Close()

	d := &HTTPDrawer{Client: srv.Client()}
	_, err := d.Draw(context.Background(),
		Resolved{BaseURL: srv.URL, Model: "m", APIKey: "k"}, DrawRequest{Prompt: "x"})
	if err == nil {
		t.Fatal("上游 400 应该是错误")
	}
	if !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "not activated") {
		t.Errorf("报错丢了可以 debug 的信息：%v", err)
	}
}

func TestHTTPDrawer_RefusesAnEmptyPrompt(t *testing.T) {
	d := &HTTPDrawer{}
	if _, err := d.Draw(context.Background(), Resolved{BaseURL: "u", Model: "m"}, DrawRequest{}); err == nil {
		t.Error("空提示词应该被拒——它只会烧掉一次调用换回一张随机图")
	}
}
