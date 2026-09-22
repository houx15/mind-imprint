package agent

import (
	"context"
	"fmt"
	"strings"

	"mindimprint/api/internal/gateway"
)

// project_summary.go — S1 · summary-on-return. When a student re-opens an
// in-progress project, the coach composes ONE compact, warm paragraph from the
// spine projection so she can pick the thread back up (same first-open-wins
// stored-prose pattern as the 你的思维印记 mirror). Composed once per project,
// via flagship; the caller stores it.

const returnSummarySystem = `你是「思维印记」的陪练。学生重新打开一个进行中的研究项目。根据下面的「项目当前状态」，用一段温和、简短的话（两到四句）直接对学生说，用“你”称呼学生，说明：学生在做的是什么、进行到哪、下一步大概可以想什么。只依据提供的项目状态，不编造此前的对话或进步，不替学生下结论，不布置命令式任务。只输出这段话本身，不要任何前缀、标签或格式。`

// ComposeReturnSummary asks the flagship model for the re-entry paragraph from
// the spine projection. Usage is populated whenever Collect succeeded. An empty
// projection or an empty reply is an error (the caller then does not persist,
// so a later open retries).
func ComposeReturnSummary(ctx context.Context, prov gateway.Provider, r gateway.Resolved, projection string) (string, gateway.ChatUsage, error) {
	if strings.TrimSpace(projection) == "" {
		return "", gateway.ChatUsage{}, fmt.Errorf("agent: empty spine projection for return summary")
	}
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: returnSummarySystem},
			{Role: gateway.RoleUser, Content: "项目当前状态：\n" + projection},
		},
	}
	res, err := gateway.Collect(ctx, prov, r, req)
	if err != nil {
		return "", gateway.ChatUsage{}, err
	}
	text := strings.TrimSpace(res.Text)
	if text == "" {
		return "", res.Usage, fmt.Errorf("agent: empty return summary")
	}
	return text, res.Usage, nil
}
