package gateway

// images.go — 画一张图。
//
// ## 为什么这是和 Provider 并排的第二条路，而不是 Provider 上的一个方法
//
// `Provider` 只有 `Stream`：一次对话是流式的，token 一个一个到。生成一张图不是
// 流式的——请求发出去，等十几秒，回来一个网址。硬塞进 `Stream` 只会让每一个聊天
// 适配器都要处理一种它永远不会遇到的事件。
//
// 目录早就把这件事写在纸上了（models.json · _capabilityComment）：
//
//   「The image / embedding / rerank rows are catalog metadata, not yet callable:
//     they answer on other endpoints (/images/generations, /embeddings, /rerank)
//     and each needs its own provider kind when a feature calls for it.」
//
// 这个文件就是那句话里的 adapter。同一把 DashScope key，另一个**主机**、另一种
// 请求形状。
//
// ## 🚨 端点不是 /images/generations —— 这一条是实测出来的，不是推出来的
//
// 2026-09-04 实测（LIVE_LLM=1，见 images_live_test.go）：
//
//   POST {maas}/compatible-mode/v1/images/generations       → 404（空正文）
//   POST dashscope.aliyuncs.com/compatible-mode/v1/images/…  → 404（空正文）
//   POST {maas}/compatible-mode/v1/chat/completions
//        model=qwen-image-3.0                                → 400 InvalidParameter
//   POST dashscope.aliyuncs.com/api/v1/services/aigc/
//        multimodal-generation/generation                    → 200，回一张图 ✅
//
// 也就是说：聊天走的那个 maas 聚合口**画不了图**，尽管它的 GET /models 里就列着
// qwen-image-3.0。图要走 DashScope 原生的多模态生成口，请求体是 input/parameters
// 那一套，不是 OpenAI 的 prompt/size 那一套。
//
// 这正是 AGENTS.md 那条规矩要挡的事：照 OpenAI 兼容口写完、桩测试全绿、上线之后
// 每一次生成都是 404。桩测试只能证明解析器读得懂**我们自己写的** JSON。
// 换模型或换通道之后，必须再跑一次 `LIVE_LLM=1 go test ./internal/gateway -run TestLiveDraw`。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DrawRequest 是一次生成请求。
//
// 故意窄：一个提示词、一个尺寸、一张图。生成多张让学生挑是**界面**的事（连着
// 请求三次），不是这一层的事——把 n 放进来，调用点就会开始为「拿到几张」写分支。
type DrawRequest struct {
	Prompt string
	// Size 是 "宽x高"。空 = 用 DefaultDrawSize。
	Size string
}

// DrawResult 是一次生成的结果。只有一个地址：实测这条口回的就是一个地址
// （不是 base64），多留一格没人填的字段只会让下一个读代码的人多想一遍。
type DrawResult struct {
	// URL 是上游临时可取的那个地址。**它会过期**，所以调用点必须马上把图取下来
	// 存进我们自己的对象存储，不能把这个地址存进数据库。
	URL string
}

// DefaultDrawSize 是默认尺寸。
//
// 1024x1024：头图和受众画像都用得上，而且是所有通道都支持的那一档。真正的头图
// 是宽幅的，但裁剪在前端做——一个不被支持的尺寸会让整次调用 400，而裁剪不会。
const DefaultDrawSize = "1024*1024"

// drawTimeout 是一次生成的上限。图比对话慢得多。
const drawTimeout = 90 * time.Second

// Drawer 画图。和 Provider 并排。
type Drawer interface {
	Draw(ctx context.Context, r Resolved, req DrawRequest) (DrawResult, error)
}

// HTTPDrawer 打 DashScope 原生的多模态生成口。端点整个写在 Resolved.BaseURL 上，
// 由 models.json · providers.dashscope_image 给。
type HTTPDrawer struct {
	// Client 可注入，测试用假服务器。
	Client *http.Client
}

// NewHTTPDrawer 造一个带超时的画图器。
func NewHTTPDrawer() *HTTPDrawer {
	return &HTTPDrawer{Client: &http.Client{Timeout: drawTimeout}}
}

// errDrawFailed 是唯一会外泄的错误文本。真实原因（状态码、上游正文）只包给
// 服务端日志——和 provider.go 的 errStreamFailed 同一条规矩：上游正文里可能有
// key 的回显。
type DrawError struct {
	Status int
	Body   string
}

func (e *DrawError) Error() string {
	body := e.Body
	if len(body) > 500 {
		body = body[:500]
	}
	return fmt.Sprintf("image generation failed: status %d: %s", e.Status, body)
}

func (d *HTTPDrawer) Draw(ctx context.Context, r Resolved, req DrawRequest) (DrawResult, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return DrawResult{}, fmt.Errorf("gateway: draw with an empty prompt")
	}
	size := req.Size
	if size == "" {
		size = DefaultDrawSize
	}
	// DashScope 原生多模态生成口的形状。提示词裹在 input.messages[].content[].text
	// 里——和聊天口的 messages 长得像，但 content 是一个**数组**，装的是带类型的
	// 块。直接给字符串会 400（实测：Input should be a valid list）。
	body, err := json.Marshal(map[string]any{
		"model": r.Model,
		"input": map[string]any{
			"messages": []map[string]any{{
				"role":    "user",
				"content": []map[string]any{{"text": req.Prompt}},
			}},
		},
		"parameters": map[string]any{"size": size, "n": 1},
	})
	if err != nil {
		return DrawResult{}, err
	}

	// baseUrl 在这条路上就是整个端点，不再往后拼路径：它不是一个 /v1 前缀，
	// 而是一个具体的服务地址（models.json · providers.dashscope_image）。
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.BaseURL, bytes.NewReader(body))
	if err != nil {
		return DrawResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+r.APIKey)

	client := d.Client
	if client == nil {
		client = &http.Client{Timeout: drawTimeout}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return DrawResult{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return DrawResult{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return DrawResult{}, &DrawError{Status: resp.StatusCode, Body: string(raw)}
	}
	return ParseDrawResponse(raw)
}

// ParseDrawResponse 读原生多模态生成口的回包。
//
// 形状（2026-09-04 实测）：
//
//	{"output":{"choices":[{"message":{"content":[{"image":"https://…png?Expires=…"}]}}]},
//	 "usage":{"output_image_count":1,…}}
//
// 🚨 那个 image 地址是**带签名、会过期**的（回包里就写着 Expires）。所以调用点
// 必须马上把图取下来存进我们自己的对象存储；把这个地址存进数据库，等于给她的
// 主页放一张几天后变成碎图的头图。
func ParseDrawResponse(raw []byte) (DrawResult, error) {
	var out struct {
		Output struct {
			Choices []struct {
				Message struct {
					Content []struct {
						Image string `json:"image"`
						Text  string `json:"text"`
					} `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		} `json:"output"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return DrawResult{}, fmt.Errorf("gateway: image response is not JSON: %w", err)
	}
	// 200 里也可能带 code —— 上游用它报「内容被拦下了」这类事。
	if out.Code != "" {
		return DrawResult{}, fmt.Errorf("gateway: image upstream said %s: %s", out.Code, out.Message)
	}
	for _, ch := range out.Output.Choices {
		for _, part := range ch.Message.Content {
			if strings.TrimSpace(part.Image) != "" {
				return DrawResult{URL: part.Image}, nil
			}
		}
	}
	return DrawResult{}, fmt.Errorf("gateway: image response carried no image")
}
