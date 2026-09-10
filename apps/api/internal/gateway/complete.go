package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// complete.go —— 一次要一整份回答，而不是一个流。
//
// # 为什么这个文件存在（2026-09-10 实测）
//
// **DashScope 那条聚合口的流式回复会丢掉最后一块内容。** 同一个 prompt、同一个
// 模型、同一秒，用 curl 各打一次：
//
//	非流式：{"a":"…","b":"…prolonging global warming."}   finish=stop  out=55
//	流式：  {"a":"…","b":"…prolonging global             finish=stop  out=55
//
// 两边的 `completion_tokens` 都是 55 —— 服务端认为它生成了 55 个 token，而流里
// 只送到了其中一部分。最后一个内容分片从来没到过，紧接着就是一个 content 为空、
// `finish_reason:"stop"` 的分片。**没有任何一个字段说这份回答不完整。**
//
// 后果按调用的形状不同：
//
//   - 给学生看的对话，末尾少几个字，容易被当成模型话没说完。
//   - **要解析的 JSON，整份作废。** 少的那几个字符里必然有收尾的 `"}`，于是
//     一次完整、正确的回答变成一句 "reply 里的 JSON 对象没有写完"。
//
// 这正是这个 repo 里反复出现的那一类事故（2026-09-08：一天的星图没了、卡片断在
// 一半、"finish_reason:stop 不等于写完了"）。它一直被当成模型的毛病，而它是**传输
// 层丢数据**：非流式同一个请求一次都没丢过。
//
// # 所以 Collect 不再走流
//
// `Collect` 的语义本来就是「我不要流，给我整份」。对这样的调用，流式没有任何好处
// ——多一层解析，外加这个丢数据的毛病。所以 `Collect` 优先走这里的非流式请求，
// 只有在通道没实现 `Completer` 时才退回流式。
//
// 真的要流的地方（对话逐字吐给学生的 SSE）照旧走 `Stream`。那条路上这个上游毛病
// 还在，我们挡不住 —— 但那半边丢的是几个字，不是一整份产物。

// Completer 是「能一次给出整份回答」的通道。
//
// 单独一个接口而不是加进 Provider：不是每种通道都需要它，而 Provider 只有一个
// 方法这件事让 mock 一直很便宜。没实现的通道，Collect 自动退回流式。
type Completer interface {
	Complete(ctx context.Context, r Resolved, req ChatRequest) (ChatResult, error)
}

// errNotCompletable 是 mux 在「底下那个通道不支持非流式」时给出的信号。它只在
// gateway 内部流转：Collect 认出它就退回流式，调用方永远看不到。
var errNotCompletable = errors.New("provider cannot complete without streaming")

// Complete 发一次非流式请求，把整份回答读回来。
func (p *CatalogProvider) Complete(ctx context.Context, r Resolved, req ChatRequest) (ChatResult, error) {
	body, err := p.buildBody(r, req)
	if err != nil {
		return ChatResult{}, err
	}
	name := r.Provider
	if name == "" {
		name = KindOpenAICompatible
	}
	res, _, cerr := completeOpenAICompatible(ctx, p.http, r, body, name)
	return res, cerr
}

// openAICompletion 是非流式回复的形状。字段与流式那边一一对应，好让两条路产出
// 同一个 ChatResult。
type openAICompletion struct {
	Choices []struct {
		Message struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
			ToolCalls        []struct {
				ID       string `json:"id"`
				Function struct {
					Name string `json:"name"`
					// 🚨 参数要读出来。非流式这一路如果只取 name，
					// streamViaComplete 发出去的 EventToolUse 就没有参数，而真
					// 流式是有的 —— 一个「工具被调用了但没带参数」的事件，调用方
					// 多半会当成一次空调用默默跳过。
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// maxCompletionBytes 挡住一个坏掉的上游返回几百兆。16000 token 的回答远在这个数
// 以内，所以它只在出事的时候起作用。
const maxCompletionBytes = 8 << 20

// completeOpenAICompatible 发一次非流式请求。
//
// 第二个返回值是**带参数的**工具调用，只有 streamViaComplete 用得上：ChatResult
// 里的 ToolCall 不带参数（`Collect` 从流式那一路收上来的也不带，见 collect.go），
// 两条路必须给调用方一模一样的东西，所以这里不去「顺手补上」它。
func completeOpenAICompatible(ctx context.Context, client *http.Client, r Resolved, body map[string]any, provider string) (ChatResult, []StreamToolUse, error) {
	// 🚨 显式把 stream 关掉。buildBody 是两条路共用的，而流式那边靠它带上
	// `stream:true`；漏掉这一行会让上游按 SSE 回，而这里按 JSON 读 —— 症状是
	// 「解析不出任何东西」，看上去又像模型的错。
	body["stream"] = false
	delete(body, "stream_options")

	payload, _ := json.Marshal(body)
	endpoint := strings.TrimRight(r.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return ChatResult{}, nil, errStreamFailed
	}
	httpReq.Header.Set("Authorization", "Bearer "+r.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		// 带上连接层的错因，理由同 streamOpenAICompatible：少了它，日志里分不清
		// 是连不上、被限流、还是请求被取消。密钥在 header 里，不会进这句话。
		return ChatResult{}, nil, fmt.Errorf("%w: %s transport: %v", errStreamFailed, provider, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		detail := readErrorBody(resp.StatusCode, resp.Body)
		return ChatResult{}, nil, fmt.Errorf("%w: %s http %d%s", errStreamFailed, provider, resp.StatusCode, detail)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxCompletionBytes))
	if err != nil {
		return ChatResult{}, nil, fmt.Errorf("%w: %s read: %v", errStreamFailed, provider, err)
	}
	var out openAICompletion
	if err := json.Unmarshal(raw, &out); err != nil {
		return ChatResult{}, nil, fmt.Errorf("%w: %s reply is not JSON: %v", errStreamFailed, provider, err)
	}
	res := ChatResult{Usage: ChatUsage{
		InputTokens:  out.Usage.PromptTokens,
		OutputTokens: out.Usage.CompletionTokens,
	}}
	var uses []StreamToolUse
	if len(out.Choices) > 0 {
		c := out.Choices[0]
		res.Text = c.Message.Content
		res.Reasoning = c.Message.ReasoningContent
		res.StopReason = mapOpenAIFinish(c.FinishReason)
		for _, tc := range c.Message.ToolCalls {
			res.ToolCalls = append(res.ToolCalls, ToolCall{ID: tc.ID, Name: tc.Function.Name})
			uses = append(uses, StreamToolUse{ID: tc.ID, Name: tc.Function.Name, ArgsJSON: tc.Function.Arguments})
		}
	}
	return res, uses, nil
}

// streamViaComplete 把一次非流式请求包装成一个「流」：整份回答作为一个 delta
// 发出去，随后是用量与收尾。
//
// 给的是 Policy.StreamDropsTail 那种通道用的（见 catalog.go 上那个字段）。调用方
// 拿到的事件序列和真流式一模一样，只是内容一次到齐 —— 它换来的是「不再每次都
// 少掉最后几个字符」。
func (p *CatalogProvider) streamViaComplete(ctx context.Context, r Resolved, req ChatRequest) (<-chan StreamEvent, error) {
	body, berr := p.buildBody(r, req)
	if berr != nil {
		return nil, berr
	}
	name := r.Provider
	if name == "" {
		name = KindOpenAICompatible
	}
	res, uses, err := completeOpenAICompatible(ctx, p.http, r, body, name)
	if err != nil {
		return nil, err
	}
	// 🚨 缓冲开够 + 先填满再 close，所以这里不需要 goroutine，也就不存在
	// 「调用方没把 channel 读干净就泄漏一个 goroutine」那种事。
	out := make(chan StreamEvent, len(uses)+4)
	if res.Reasoning != "" {
		out <- StreamEvent{Kind: EventReasoningDelta, TextDelta: res.Reasoning}
	}
	if res.Text != "" {
		out <- StreamEvent{Kind: EventTextDelta, TextDelta: res.Text}
	}
	for i := range uses {
		u := uses[i]
		out <- StreamEvent{Kind: EventToolUse, ToolUse: &u}
	}
	usage := res.Usage
	out <- StreamEvent{Kind: EventUsage, Usage: &usage}
	out <- StreamEvent{Kind: EventDone, StopReason: res.StopReason}
	close(out)
	return out, nil
}
