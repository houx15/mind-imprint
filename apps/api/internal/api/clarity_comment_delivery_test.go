package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"mindimprint/api/internal/claritytest"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
	"testing"
)

// Only billing persistence is stubbed. Prompts, model routing, parsing,
// delivery filtering and the bounded retry are the production implementation.
// No student data or production database is accessed.
type clarityUsageDB struct{ sqlc.DBTX }
type clarityUsageRow struct{}

func (clarityUsageDB) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	return clarityUsageRow{}
}
func (clarityUsageRow) Scan(...interface{}) error { return nil }

type clarityCommentProvider struct {
	real  gateway.Provider
	usage gateway.ChatUsage
	t     *testing.T
	calls int
}

func (p *clarityCommentProvider) Stream(ctx context.Context, r gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	res, err := gateway.Collect(ctx, p.real, r, req)
	p.calls++
	p.usage.InputTokens += res.Usage.InputTokens
	p.usage.OutputTokens += res.Usage.OutputTokens
	p.usage.CachedInputTokens += res.Usage.CachedInputTokens
	p.t.Logf("review attempt=%d raw=%s", p.calls, res.Text)
	if err != nil {
		return nil, err
	}
	return gateway.NewStubProvider([]gateway.StreamEvent{{Kind: gateway.EventTextDelta, TextDelta: res.Text}, {Kind: gateway.EventUsage, Usage: &res.Usage}, {Kind: gateway.EventDone, StopReason: res.StopReason}}).Stream(ctx, r, req)
}
func clarityCommentDelivery(t *testing.T, source, lang string) claritytest.Collector {
	return func(ctx context.Context, prov gateway.Provider, r gateway.Resolved, req gateway.ChatRequest) (gateway.ChatResult, error) {
		tapped := &clarityCommentProvider{real: prov, t: t}
		a := New(Deps{Provider: tapped, Queries: sqlc.New(clarityUsageDB{})})
		deliver := func(points []CommentPoint) []CommentPoint {
			return validateCommentPoints(points, source, lang, writingBlockCommentMaxIssues)
		}
		out, ok := a.collectWritingComment(ctx, uuid.Nil, uuid.Nil, "copy-review", r, req.Messages[0].Content, req.Messages[1].Content, deliver)
		out.Points = deliver(out.Points)
		b, _ := json.Marshal(out)
		res := gateway.ChatResult{Text: string(b), Usage: tapped.usage}
		if !ok {
			return res, fmt.Errorf("production comment collection failed")
		}
		return res, nil
	}
}
