package gateway

import (
	"context"
	"os"
	"testing"
	"time"
)

// 🚨 这条路必须真的打一次才算数。
//
// AGENTS.md：「换通道后必须跑 LIVE_LLM=1 go test ./internal/gateway -run TestLive
// 验证。」理由 2026-09-02 已经付过学费：同一个模型换条通道，关思考的字段名不一样，
// 而写错**不会报错**，只会安静地给出另一种行为。
//
// 图这条路第一次跑就抓到了：目录里那句「answer on /images/generations」是错的
// ——那条路 404，图要走 DashScope 原生的多模态生成口。整张实测表在 images.go 的
// 文件头。桩测试只能证明我们的解析器读得懂**我们自己写的** JSON，所以换模型、
// 换通道之后都要再跑一次这条。
//
// 跑法：
//
//	DASHSCOPE_API_KEY=... LIVE_LLM=1 go test ./internal/gateway -run TestLiveDraw -v
func TestLiveDraw(t *testing.T) {
	if os.Getenv("LIVE_LLM") == "" {
		t.Skip("LIVE_LLM 没开——这条要真打一次上游")
	}
	cat, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := cat.Resolve(ClassDraw, "", func(env string) string { return os.Getenv(env) })
	if err != nil {
		t.Fatalf("draw 档解析不出来：%v", err)
	}
	if resolved.APIKey == "" {
		t.Skip("没有 key")
	}
	t.Logf("draw → %s (%s) @ %s", resolved.ModelID, resolved.Model, resolved.BaseURL)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	start := time.Now()
	got, err := NewHTTPDrawer().Draw(ctx, resolved, DrawRequest{
		Prompt: "一张简洁的头像插画：一个高中生，正在读一本书，暖色调，扁平插画风格，不要文字",
	})
	if err != nil {
		t.Fatalf("生成失败（这就是这条测试存在的理由）：%v", err)
	}
	t.Logf("耗时 %s；url=%q", time.Since(start).Round(time.Millisecond), got.URL)
	if got.URL == "" {
		t.Fatal("上游回了一个没有图的东西")
	}
}
