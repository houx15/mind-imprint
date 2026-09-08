package news

import (
	"context"
	"os"
	"testing"
	"time"
)

// live_test —— 打真源的测试。默认跳过。
//
//	LIVE_FEEDS=1 go test ./internal/news -run TestLive -v
//
// 和 `internal/gateway` 的 `LIVE_LLM` 同一个约定：合成 XML 只能证明解析器读得懂
// **我写的** XML。Nature 的真实标记、arXiv 的 RDF、ScienceDaily 的 `EDT` 时区，
// 都是只有打真源才会暴露的东西 —— Atom 的 `,attr` 那个 bug 就是这一类。
//
// ⚠️ 从北京 ECS 跑和从本机跑结果可能不同（见
// docs/2026-09-03-news-feed-reachability-from-beijing-ecs.md）。判定以 ECS 为准。

func TestLiveFetchAll(t *testing.T) {
	if os.Getenv("LIVE_FEEDS") != "1" {
		t.Skip("set LIVE_FEEDS=1 to hit the real feeds")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	f := NewFetcher()
	pool := f.FetchAll(ctx, 72*time.Hour)

	t.Logf("候选池 %d 条", len(pool))
	if len(pool) < PlanetCount {
		t.Fatalf("候选池只有 %d 条，凑不满 %d 颗星", len(pool), PlanetCount)
	}

	bySource := map[string]int{}
	// 每个源的正文情况。2026-09-08 加：`content:encoded` 之前根本没被读过，
	// 而好几个源的整篇文章就在那里。这一栏让「哪个源真的带正文」是量出来的，
	// 不是靠记忆。
	bodyBySource := map[string]int{}
	withBody := 0
	for _, it := range pool {
		bySource[it.Source]++
		if n := len([]rune(it.Body)); n > 0 {
			withBody++
			if n > bodyBySource[it.Source] {
				bodyBySource[it.Source] = n
			}
		}
		if it.Title == "" {
			t.Error("有条目没有标题")
		}
		// 硬规则：只有标题和链接的条目不该走到这里（WithReadableSummary）。
		if len([]rune(ExcerptFor(it, MinSummaryRunes+1))) < MinSummaryRunes {
			t.Errorf("一条没有可读摘要的条目混进了候选池：%q [%s]", it.Title, it.Source)
		}
		// 🚨 链接为空是 Atom `,attr` 那个 bug 的症状：不报错，只是每颗星球
		// 都点不开。
		if it.Link == "" {
			t.Errorf("条目没有链接（Atom ,attr？）：%q [%s]", it.Title, it.Source)
		}
		if it.Published.IsZero() {
			t.Errorf("条目没有日期（应该已被 FreshWithin 丢掉）：%q", it.Title)
		}
	}
	for src, n := range bySource {
		t.Logf("  %-28s %3d 条   正文最长 %d 字", src, n, bodyBySource[src])
	}
	t.Logf("带正文的条目：%d / %d", withBody, len(pool))
	if len(bySource) < 3 {
		t.Errorf("只有 %d 个源出了东西，太少了", len(bySource))
	}

	t.Log("--- 前 8 条 ---")
	for i, it := range pool {
		if i >= 8 {
			break
		}
		t.Logf("  [%s] %s", it.Published.Format("01-02"), it.Title)
	}
}
