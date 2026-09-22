package api

import (
	"errors"
	"mindimprint/api/internal/claritytest"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
	"strings"
	"testing"
)

func TestClarityMergeWriting(t *testing.T) {
	for _, tc := range []struct {
		name, lang, title, said string
		rows                    []sqlc.WritingOutline
		noAdd, judgment, terms  bool
	}{
		{name: "process", lang: "zh", title: "成功", said: "我得先选择一个有意思的词。", noAdd: true},
		{name: "judgment", lang: "zh", title: "成功", said: "我还想说，黑心商家哪怕赚很多钱，也是失败。", judgment: true, rows: []sqlc.WritingOutline{planRow("a", 0, "平凡尽责也是成功", writingKindThesis), planRow("b", 1, "人人都有自己的贡献", writingKindPoint)}},
		{name: "english-analysis", lang: "en", title: "Should schools start later?", said: "My classmate falls asleep in first period every day. He gets up at five thirty to take two buses.", terms: true, rows: []sqlc.WritingOutline{planRow("a", 0, "Schools should start later", writingKindThesis), planRow("b", 1, "Students do not get enough sleep", writingKindPoint)}},
		{name: "english-narrative", lang: "en", title: "A moment that changed how I see something", said: "I saw my grandfather repair a broken chair instead of throwing it away. I realized that I had never noticed how carefully he worked.", rows: []sqlc.WritingOutline{planRow("a", 0, "In my grandfather's workshop", writingKindOpening), planRow("b", 1, "He repaired a broken chair", writingKindScene)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wr := sqlc.Writing{Lang: tc.lang, Title: tc.title}
			if tc.noAdd {
				p := liveWordChoicePrompt
				wr.AssignedPrompt = &p
			}
			genre := genreArgument
			if tc.name == "english-narrative" {
				genre = genreNarrative
			}
			claritytest.Run(t, gateway.ClassDialogue, gateway.ChatRequest{MaxTokens: 4096, Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: writingPlanSystemFor(genre, wr.Lang, "")}, {Role: gateway.RoleUser, Content: buildWritingPlanPrompt(wr, tc.rows, nil, tc.said)}}}, func(raw string) error {
				out, ok := parseWritingPlanReply(raw)
				if !ok {
					return errors.New("plan parse failed")
				}
				if tc.noAdd && len(out.Add) > 0 {
					return errors.New("process talk added as content")
				}
				for _, n := range out.Add {
					if !writingKindValid(n.Kind) {
						return errors.New("invalid node kind")
					}
					if tc.judgment && (n.Kind == writingKindEvidence || n.Kind == writingKindReference) {
						return errors.New("judgment treated as evidence")
					}
					if genre == genreNarrative && (n.Kind == writingKindThesis || n.Kind == writingKindPoint) {
						return errors.New("argument node added to narrative")
					}
				}
				if tc.terms {
					if strings.Contains(out.Reply, "第二条 topic sentence") || strings.Contains(out.Reply, "挂在半空") {
						return errors.New("existing reason ignored or metaphor introduced")
					}
					found := false
					for _, term := range []string{"thesis statement", "topic sentence", "commentary", "analysis", "counterargument"} {
						found = found || strings.Contains(strings.ToLower(out.Reply), term)
					}
					if !found {
						return errors.New("English teaching terminology missing")
					}
				}
				return checkClarityTeachingLanguage(out.Reply)
			})
		})
	}
	t.Run("choose-word-opening", func(t *testing.T) {
		wr := liveWordChoiceWriting()
		claritytest.Run(t, gateway.ClassDialogue, gateway.ChatRequest{MaxTokens: 1024, Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: writingOpeningSystemFor(wr)}, {Role: gateway.RoleUser, Content: buildWritingOpeningPrompt(wr, nil)}}}, func(raw string) error {
			if !strings.Contains(raw, "词") || strings.Contains(raw, "中心论点") {
				return errors.New("opening skipped word choice")
			}
			return checkClarityTeachingLanguage(raw)
		})
	})
}

// Two concrete personal experiences are sufficient material for the automatic
// planning check. They must not trigger an external-source quota in dialogue.
func TestClarityPersonalPlanReady(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		t.Run(lang, func(t *testing.T) {
			words := int32(800)
			wr := sqlc.Writing{Title: "学校图书馆是否应延长开放时间", Lang: lang, TargetWords: &words}
			if lang == "en" {
				words = 500
				wr.Title = "Should the school library stay open later?"
			}
			rows := []sqlc.WritingOutline{
				planRow("a", 0, "图书馆应该延长开放时间", writingKindThesis),
				planRow("b", 1, "晚自习后需要安静的学习场所", writingKindPoint),
				planRow("c", 2, "上周我在走廊复习，一直被路过的人打断", writingKindEvidence),
				planRow("d", 1, "晚间开放便于借阅参考书", writingKindPoint),
				planRow("e", 2, "昨天我下课后去借书，图书馆已经关门", writingKindEvidence),
			}
			if lang == "en" {
				for i, text := range []string{"The school library should stay open later", "Students need a quiet place after evening classes", "Last week people passing through the corridor interrupted my revision", "Later hours make reference books accessible after class", "Yesterday I tried to borrow a book after class but the library was closed"} {
					rows[i].Text = text
				}
			}
			if !planLooksReady(wr, rows) {
				t.Fatal("personal material incorrectly blocks readiness")
			}
			claritytest.Run(t, gateway.ClassDialogue, gateway.ChatRequest{MaxTokens: 2048, Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: writingPlanSystemFor(genreArgument, lang, "")}, {Role: gateway.RoleUser, Content: buildWritingPlanPrompt(wr, rows, nil, "这些就是我的计划。我想开始写了。")}}}, func(raw string) error {
				out, ok := parseWritingPlanReply(raw)
				if !ok {
					return errors.New("plan parse failed")
				}
				if !out.Ready || writingPlanReplyAsks(out.Reply) || strings.Contains(out.Reply, "先改") || strings.Contains(out.Reply, "改完就") {
					return errors.New("personal material prevents invitation to write")
				}
				return checkClarityTeachingLanguage(out.Reply)
			})
		})
	}
}
