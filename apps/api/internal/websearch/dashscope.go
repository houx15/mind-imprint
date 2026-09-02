// Package websearch is a minimal client for Aliyun Bailian's WebSearch MCP
// service — the one tool in the product that can reach outside the model's
// training data.
//
// It speaks MCP's streamable-HTTP transport directly rather than pulling in an
// MCP SDK. The reason is proportion: this server exposes ONE tool with two
// arguments, and the three JSON-RPC calls it takes to use it (initialize,
// tools/call, plus the initialized notification) fit in this file. A dependency
// that speaks the whole protocol would be more code to audit than the protocol
// we actually use.
//
// 🚨 The key is DASHSCOPE_API_KEY, server-side only. It goes in an
// Authorization header and never into a log, an error, a stored row, or an
// evaluation payload (AGENTS.md 硬约束).
//
// Billing: 2000 free calls, then ¥29 per 1000. Rate limit 15 QPS, SHARED across
// the Aliyun main account and every RAM sub-account — so a burst here is a
// burst against everything else on the account.
package websearch

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultEndpoint is Bailian's WebSearch MCP server. Note this is the PUBLIC
// dashscope host, not the private workspace endpoint the chat models use —
// the MCP service is account-scoped, not workspace-scoped.
const DefaultEndpoint = "https://dashscope.aliyuncs.com/api/v1/mcps/WebSearch/mcp"

// ToolName is the single tool this server exposes.
const ToolName = "bailian_web_search"

// Result is one search hit, flattened out of whatever the tool returns.
type Result struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// Client calls the WebSearch MCP server.
type Client struct {
	Endpoint string
	APIKey   string
	HTTP     *http.Client
}

// New builds a client. An empty key is allowed so the caller can construct one
// unconditionally and let Search report the misconfiguration — the same shape
// as the gateway's resolver seam.
func New(apiKey string) *Client {
	return &Client{
		Endpoint: DefaultEndpoint,
		APIKey:   apiKey,
		HTTP:     &http.Client{Timeout: 30 * time.Second},
	}
}

// ErrNotConfigured is returned when no key is present. Callers degrade to "no
// search" rather than failing the turn: search is an enrichment, and a reading
// room that dies because a search key is missing is worse than one that quietly
// has no results.
var ErrNotConfigured = errors.New("websearch: DASHSCOPE_API_KEY not configured")

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// toolCallResult is MCP's tools/call reply: a list of content blocks. The
// search server returns its hits as a JSON document inside a text block, which
// is why this unwraps twice.
type toolCallResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError"`
}

// Search runs one query and returns at most count results.
//
// The whole exchange is one HTTP round trip per JSON-RPC call. MCP's
// streamable-HTTP transport answers either with a JSON body or with an SSE
// stream depending on the server's mood, so the reader below accepts both —
// a server that upgrades to SSE on a slow query would otherwise look like a
// parse failure.
func (c *Client) Search(ctx context.Context, query string, count int) ([]Result, error) {
	if c == nil || strings.TrimSpace(c.APIKey) == "" {
		return nil, ErrNotConfigured
	}
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("websearch: empty query")
	}
	if count <= 0 {
		count = 5
	}

	sessionID, err := c.initialize(ctx)
	if err != nil {
		return nil, err
	}

	raw, err := c.call(ctx, sessionID, rpcRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: map[string]any{
			"name":      ToolName,
			"arguments": map[string]any{"query": query, "count": count},
		},
	})
	if err != nil {
		return nil, err
	}
	var out toolCallResult
	if uerr := json.Unmarshal(raw, &out); uerr != nil {
		return nil, fmt.Errorf("websearch: tools/call reply unparseable: %w", uerr)
	}
	if out.IsError {
		return nil, fmt.Errorf("websearch: tool reported an error: %s", firstText(out))
	}
	return parseHits(firstText(out), count), nil
}

func firstText(r toolCallResult) string {
	var b strings.Builder
	for _, c := range r.Content {
		if c.Text != "" {
			b.WriteString(c.Text)
		}
	}
	return b.String()
}

// initialize performs the MCP handshake and returns the session id the server
// wants echoed back, if it issued one.
func (c *Client) initialize(ctx context.Context) (string, error) {
	body, _ := json.Marshal(rpcRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "mind-imprint", "version": "1"},
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	c.setHeaders(req, "")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("websearch: initialize: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", statusError("initialize", resp)
	}
	// Drain so the connection can be reused; the handshake's payload is not
	// needed beyond the session header.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return resp.Header.Get("Mcp-Session-Id"), nil
}

func (c *Client) call(ctx context.Context, sessionID string, rpc rpcRequest) (json.RawMessage, error) {
	body, _ := json.Marshal(rpc)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	c.setHeaders(req, sessionID)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("websearch: %s: %w", rpc.Method, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, statusError(rpc.Method, resp)
	}
	payload, err := readJSONOrSSE(resp)
	if err != nil {
		return nil, err
	}
	var out rpcResponse
	if uerr := json.Unmarshal(payload, &out); uerr != nil {
		return nil, fmt.Errorf("websearch: %s: reply unparseable: %w", rpc.Method, uerr)
	}
	if out.Error != nil {
		return nil, fmt.Errorf("websearch: %s: server error %d: %s", rpc.Method, out.Error.Code, out.Error.Message)
	}
	return out.Result, nil
}

func (c *Client) setHeaders(req *http.Request, sessionID string) {
	req.Header.Set("Content-Type", "application/json")
	// Both, because the streamable-HTTP transport picks its response format
	// from Accept and either is fine here.
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
}

// statusError reports the upstream status WITHOUT echoing the request, so a key
// can never reach a log or a rendered error through this path.
func statusError(method string, resp *http.Response) error {
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("websearch: %s: http %d: %s", method, resp.StatusCode, strings.TrimSpace(string(snippet)))
}

// readJSONOrSSE returns the JSON-RPC payload whether the server answered with a
// plain JSON body or with an SSE stream carrying it in a data: frame.
func readJSONOrSSE(resp *http.Response) ([]byte, error) {
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 8<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if data, ok := strings.CutPrefix(line, "data:"); ok {
			if d := strings.TrimSpace(data); d != "" && d != "[DONE]" {
				return []byte(d), nil
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return nil, errors.New("websearch: SSE stream carried no data frame")
}

// parseHits pulls results out of the tool's text payload.
//
// The shape is not contractual — it is whatever this vendor's tool returns
// today — so this accepts the three shapes it has been seen to use and gives up
// gracefully rather than erroring. An empty slice is a real answer ("nothing
// found"); making the caller distinguish that from "we could not read the
// reply" would be a distinction it cannot act on either way.
func parseHits(text string, count int) []Result {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	// 1. a bare array
	var arr []Result
	if json.Unmarshal([]byte(text), &arr) == nil && len(arr) > 0 {
		return cap_(arr, count)
	}
	// 2. an object with results/data/items
	var obj map[string]json.RawMessage
	if json.Unmarshal([]byte(text), &obj) == nil {
		for _, key := range []string{"results", "data", "items", "pages", "webPages"} {
			raw, ok := obj[key]
			if !ok {
				continue
			}
			var hits []Result
			if json.Unmarshal(raw, &hits) == nil && len(hits) > 0 {
				return cap_(hits, count)
			}
			// nested one level (e.g. {"webPages":{"value":[…]}})
			var nested map[string]json.RawMessage
			if json.Unmarshal(raw, &nested) == nil {
				for _, inner := range nested {
					if json.Unmarshal(inner, &hits) == nil && len(hits) > 0 {
						return cap_(hits, count)
					}
				}
			}
		}
	}
	// 3. plain prose — hand it back as one result so the caller still has
	// something to show rather than silently nothing.
	return []Result{{Snippet: text}}
}

func cap_(rs []Result, n int) []Result {
	if n > 0 && len(rs) > n {
		return rs[:n]
	}
	return rs
}
