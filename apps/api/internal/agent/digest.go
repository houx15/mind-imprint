package agent

import (
	"context"
	"fmt"
	"strings"

	"mindimprint/api/internal/gateway"
)

// digest.go — S4 · the compaction size-threshold backstop's compose half.
// When the coach's active (non-folded) window still overflows a rune budget
// after fold-on-solidify, the OLDEST overflow turns are merged into a rolling
// per-project "conversation digest" via one isolated mid-tier call, then folded
// out of the window. Mirrors ComposeExplorationGuide: pure input → one call →
// text. Never invents; compresses the student's own reasoning and durable
// facts so the coach never forgets folded history (过程即数据 — nothing lost).

// DigestTurn is one older chat turn to compress into the digest.
type DigestTurn struct {
	Role    string // user|assistant
	Content string
}

const digestSystemPrompt = `你在为一个学生的研究项目维护一份「会话记忆」。把下面这些较早的对话轮次，压进已有的记忆里：
- 保留可复用的事实、学生自己的推理与决定、来源功能与边界；
- 丢掉寒暄与重复；绝不臆造任何内容；
- 输出一段简洁的中文记忆，不要分点堆砌，也不要加任何前后缀说明。`

// ComposeDigestMerge folds the given older turns onto priorDigest via one
// isolated mid-tier call. Empty turns → ("", zero usage, nil) with ZERO
// provider calls (克制/no-spend discipline, cloned from HasGraphContent): the
// caller must never fold turns whose content isn't durable, and must never
// spend when there is nothing to compact.
func ComposeDigestMerge(ctx context.Context, prov gateway.Provider, r gateway.Resolved, priorDigest string, turns []DigestTurn) (string, gateway.ChatUsage, error) {
	if len(turns) == 0 {
		return "", gateway.ChatUsage{}, nil
	}

	var b strings.Builder
	if strings.TrimSpace(priorDigest) != "" {
		fmt.Fprintf(&b, "已有记忆：\n%s\n\n", priorDigest)
	}
	b.WriteString("较早的对话轮次（时间从早到晚）：\n")
	for _, t := range turns {
		fmt.Fprintf(&b, "- [%s] %s\n", t.Role, t.Content)
	}

	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: digestSystemPrompt},
			{Role: gateway.RoleUser, Content: b.String()},
		},
	})
	if err != nil {
		return "", gateway.ChatUsage{}, err
	}
	return strings.TrimSpace(stripFences(res.Text)), res.Usage, nil
}
