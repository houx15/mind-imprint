package agent

import (
	"context"
	"fmt"
	"strings"

	"mindimprint/api/internal/gateway"
)

// dig_query.go — Task A8: before /dig ever calls OpenAlex, 印记 turns the
// student's own question (any language, often a long Chinese sentence) into
// a short English keyword query. OpenAlex relevance on raw natural-language
// Chinese text is poor; this one isolated call is the whole value of dig.
// Never decides FOR the student which source matters (铁律①) — it only
// reframes her own question so the search actually finds what she meant.

const digQuerySystem = `你把学生的研究问题（任何语言）转成一个简短的英文关键词查询，用于在 OpenAlex 上检索学术文献。只输出查询词本身，不要引号、不要解释、不要写成完整句子；3-8 个词，抓住核心概念；把非英文的概念译成对应的标准英文学术术语。输出要尽量短。`

// ComposeDigQuery refines a student's research question into a concise
// English keyword query via one isolated LLM call. Best-effort: on an
// empty/whitespace-only result it returns an error so the caller (the /dig
// handler) falls back to the raw question text rather than searching
// OpenAlex with nothing.
func ComposeDigQuery(ctx context.Context, prov gateway.Provider, r gateway.Resolved, question string) (string, gateway.ChatUsage, error) {
	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: digQuerySystem},
			{Role: gateway.RoleUser, Content: "学生的研究问题：" + question},
		},
	})
	if err != nil {
		return "", gateway.ChatUsage{}, err
	}

	query := strings.TrimSpace(stripFences(res.Text))
	if query == "" {
		return "", res.Usage, fmt.Errorf("agent: dig query refine produced empty output")
	}
	return query, res.Usage, nil
}
