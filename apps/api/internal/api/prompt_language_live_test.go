package api_test

import (
	"fmt"
	"mindimprint/api/internal/awakening"
	"mindimprint/api/internal/claritytest"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/interest"
	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/litegrade"
	"mindimprint/api/internal/news"
	"strings"
	"testing"
)

// Real production builders/parsers for domains outside the original clarity
// suite. Synthetic replies still need human review after parser validation.
func TestClarityPromptLanguageDomains(t *testing.T) {
	run := func(name, class, system, user string, check func(string) error) {
		t.Run(name, func(t *testing.T) {
			claritytest.Run(t, class, gateway.ChatRequest{MaxTokens: 4096, Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: system}, {Role: gateway.RoleUser, Content: user}}}, check)
		})
	}
	answers := []string{"我喜欢观察校园里的鸟，昨天看见一只鸟衔着树枝。", "我想了解鸟怎样选择筑巢的地方。", "我还想比较树枝和屋檐下的巢有什么不同。"}
	s, u := awakening.BuildSelectionPrompt(answers, "鸟怎样选择筑巢的地方？")
	run("selection", gateway.ClassCompose, s, u, func(raw string) error { _, e := interest.ParseHarvestReply(raw); return e })
	s, u = awakening.BuildReportPrompt(answers)
	run("awakening-report", gateway.ClassCompose, s, u, func(raw string) error { _, _, e := awakening.ParseReportReply(raw); return e })
	s, u = awakening.BuildTitlePrompt(answers)
	run("title", gateway.ClassReflex, s, u, func(raw string) error {
		if len(awakening.ParseTitles(raw)) == 0 {
			return fmt.Errorf("no usable title")
		}
		return nil
	})
	for _, tc := range []struct{ name, topic, answer string }{
		{"title-tidal", "潮汐发电", "我对潮汐发电感兴趣，想了解潮水怎样用来发电。"},
		{"title-boardgame", "桌游", "我很喜欢桌游，周末和朋友玩，也会试着修改规则。"},
	} {
		system, user := awakening.BuildTitlePrompt([]string{tc.answer})
		run(tc.name, gateway.ClassReflex, system, user, func(raw string) error {
			titles := awakening.ParseTitles(raw)
			if len(titles) != 3 {
				return fmt.Errorf("expected three topic-name candidates")
			}
			for _, title := range titles {
				if !strings.Contains(title, tc.topic) && !(tc.name == "title-tidal" && strings.Contains(title, "潮水") && strings.Contains(title, "发电")) {
					return fmt.Errorf("interest name expanded beyond the stated topic: %s", title)
				}
				if strings.ContainsAny(title, "？?") || strings.Contains(title, "为什么") || strings.Contains(title, "怎么") {
					return fmt.Errorf("question instead of interest name: %s", title)
				}
			}
			if !strings.Contains(strings.Join(titles, " "), tc.topic) {
				return fmt.Errorf("lost the student's explicit interest: %s", tc.topic)
			}
			return nil
		})
	}
	s, u = interest.BuildHarvestPrompt("reading", "校园观鸟", strings.Join(answers, "\n"))
	run("harvest", gateway.ClassDigest, s, u, func(raw string) error { _, e := interest.ParseHarvestReply(raw); return e })
	s, u = interest.BuildDigPrompt("鸟类", "校园观察", answers, nil)
	run("dig", gateway.ClassCompose, s, u, func(raw string) error {
		seeds, e := interest.ParseDigReply(raw, nil)
		if e != nil {
			return e
		}
		for _, seed := range seeds {
			if seed.Kind == interest.DigRead {
				return fmt.Errorf("recommended a library item without a candidate")
			}
		}
		return nil
	})
	candidatesForDig := []interest.LibraryCandidate{
		{Slug: "bird-nests", Title: "鸟类如何选择筑巢地点", Reason: "比较树上与建筑附近的巢址，介绍遮蔽、天敌及筑巢材料。"},
		{Slug: "school-garden", Title: "学校花园的植物分布", Reason: "记录花园中树木和花草的分布。"},
		{Slug: "wind-power", Title: "风力发电机怎样工作", Reason: "介绍风能转化成电能的过程。"},
	}
	s, u = interest.BuildDigPrompt("鸟类", "校园观察", answers, candidatesForDig)
	run("dig-shortlist", gateway.ClassCompose, s, u, func(raw string) error {
		seeds, e := interest.ParseDigReply(raw, candidatesForDig)
		if e != nil {
			return e
		}
		for _, seed := range seeds {
			if seed.Kind == interest.DigRead && seed.LibrarySlug != "bird-nests" {
				return fmt.Errorf("recommended candidate does not address the stated interest: %s", seed.LibrarySlug)
			}
		}
		return nil
	})
	ground := "Researchers observed birds in a school garden for two weeks. They recorded where each bird carried nesting material. Some birds took twigs to trees, while others took them under a roof. The team did not measure nesting success. The records describe this garden during the observation period and cannot establish why the birds chose each location."
	item := news.Item{Title: "Birds choose different nesting sites in a school garden", Body: ground, Summary: ground, Source: "Synthetic observation", Link: "https://example.org/birds"}
	s, u, candidates := news.BuildSelectPrompt([]news.Item{item})
	run("news-select", gateway.ClassDigest, s, u, func(raw string) error { _, e := news.ParseSelectReply(raw, candidates); return e })
	s, u = news.BuildWritePrompt(item, ground)
	run("news-write", gateway.ClassDigest, s, u, func(raw string) error { _, e := news.ParseWriteReply(raw, item, ground); return e })
}

// A fully satisfied task must not acquire an issue to meet an opinion quota.
func TestClarityGradingNoInventedIssue(t *testing.T) {
	in := litegrade.Input{Lang: "zh", Title: "观察记录", AssignedPrompt: "用两句话记录一次观察。写明时间、地点和直接看到的行为即可，不需要解释原因或提出建议。", Body: "今天中午十二点，我在学校花园看到一只鸟衔着树枝。它飞到屋檐下，把树枝放进巢里。", VersionNumber: 1, Rubric: liteassign.Rubric{Scale: liteassign.ScalePoints, Max: 2, Dimensions: []liteassign.RubricDimension{{Name: "记录内容", Note: "写明时间、地点与直接看到的行为即满足本次要求，不评价分析、原因或建议。"}}}}
	claritytest.Run(t, gateway.ClassReview, gateway.ChatRequest{MaxTokens: 4096, Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: litegrade.SystemPrompt(in)}, {Role: gateway.RoleUser, Content: litegrade.UserPrompt(in)}}}, func(raw string) error {
		c, e := litegrade.Parse(raw)
		if e != nil {
			return e
		}
		if rs := litegrade.Check(c, in); len(rs) > 0 {
			return fmt.Errorf("grading contract: %v", rs)
		}
		for _, p := range c.Points {
			if p.Kind == litegrade.KindIssue {
				return fmt.Errorf("invented issue for satisfied observation task: %s", p.Text)
			}
		}
		return nil
	})
}
