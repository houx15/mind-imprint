package api

// reading_plan_genre_live_test.go —— 体裁认得准不准，以及它有没有真的换掉读法。
//
//	set -a; . .deploy-local/env.prod; set +a
//	LIVE_LLM=1 go test ./internal/api -run TestLiveReportGenre -v -count=1
//
// # 为什么必须是实测
//
// 体裁（readingOutline.Genre）**不摆在屏幕上**。它管的是给她什么工具：那块
// 「主张 / 证据 / 限制」的板只在 argument 上成立，而报道上它没有指称对象。
// 所以模型把一篇报道判成 argument 的样子，和一切正常一模一样 —— 板照常摆出来，
// 她照常猜，日志里一个字都没有。单元测试只能证明 validateGenre 认得出这四个词。
//
// 用的那篇就是同事 2026-09-17 截图里的那一篇（Israel-Hamas 的援助报道）。

import (
	"context"
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/gateway"
)

func liveReportBlocks() []Block {
	return SplitBlocks(strings.Join([]string{
		"Aid groups are scrambling to help people caught in the war between Israel and Hamas.",
		"Hamas is a group that governs Gaza. On October 7, Hamas fighters crossed into Israel and killed hundreds of people.",
		"Israel fought back. The country's soldiers sent planes to strike targets in Gaza. Israel also stopped food and other supplies from going into Gaza.",
		"The United Nations (U.N.) and aid groups are worried about working in the area. Hundreds of people have been killed. Thousands have been wounded. Aid groups say there are needs both in Gaza and Israel. They are begging to be allowed into Gaza to help Palestinians stuck in the middle of the fighting.",
		"The Egyptian Red Crescent is one aid group. It sent more than 2 tons (2,000 pounds) of supplies to Gaza. Efforts to organize food and other deliveries are underway.",
		"The U.N. and other aid agencies were talking with Egypt about how to send aid into Gaza. The area is surrounded by armed forces.",
		"Doctors Without Borders said its teams in Gaza were running short of supplies. A spokesperson said the hospitals had no space left.",
		"Israel's government said the supplies would not go in until the hostages taken on October 7 were released.",
		"Families on both sides are waiting for news. Some have not heard from relatives in days.",
		"Aid workers say the next few weeks will decide how bad the shortages become.",
	}, "\n\n"))
}

// TestLiveReportGenreIsNotAnArgument —— 一篇报道，六次，体裁必须不是 argument，
// 而且排出来的读法里不该有「标注论证」那一步。
func TestLiveReportGenreIsNotAnArgument(t *testing.T) {
	prov, r := liveClass(t, gateway.ClassCompose)
	blocks := liveReportBlocks()
	prompt := buildReadingPlanPrompt("en", "Aid groups scramble to help as Israel-Hamas war intensifies", blocks)

	wrong, labelled := 0, 0
	for i := 0; i < 6; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: readingPlanSystem},
				{Role: gateway.RoleUser, Content: prompt},
			},
		})
		cancel()
		if err != nil {
			t.Fatalf("sample %d: live call failed (%s): %v", i, r.ModelID, err)
		}
		plan, routine, reject := parseReadingPlan(res.Text, "en")
		if reject != planOK {
			t.Errorf("sample %d: REJECTED (%s)", i, reject)
			continue
		}
		out, _ := validateOutline(plan.outline(), blocks)
		hasLabel := false
		for _, s := range routine.Steps {
			if s.Kind == taskLabel {
				hasLabel = true
			}
		}
		t.Logf("sample %d — routine=%s 体裁=%q 标注步=%v", i, routine.Key, out.Genre, hasLabel)
		if out.Genre == genreArgument {
			wrong++
			t.Logf("sample %d: 一篇报道被判成 %q —— 那块「主张/证据/限制」的板会照常摆给她",
				i, out.Genre)
		}
		if hasLabel {
			labelled++
		}
	}
	t.Logf("RESULT: %d/6 判成了「作者在说服你」· %d/6 的清单上有标注论证", wrong, labelled)
	// 半数以上认错就是 prompt 没写对。判错的方向是**宁可放过**（见
	// hasAuthorsArgument），所以这里量的是它错得有多频繁，不是一次都不许错。
	if wrong*2 > 6 {
		t.Errorf("%d/6 把一篇战地报道当成了议论文", wrong)
	}
}
