package api

// writing_english_live_test.go — the two 2026-09-04 writing-room asks, sent to
// a REAL model with production's prompts and production's parsers.
//
//	set -a; . .deploy-local/env.prod; set +a
//	LIVE_LLM=1 go test ./internal/api -run TestLiveWriting -v -count=1
//
// Skipped by default, so the normal suite stays offline and deterministic.
//
// # 🚨 Why these two need a live test and not only unit tests
//
// [[prompt-output-must-be-verifiable-2026-09-03]], and the lens-completion
// turn proved it again the same week: production's own prompt told the model
// 「lens、card 都留空」 and the model attached a card 2 times in 3 anyway.
// A unit test on the prompt text would have been green throughout.
//
// Both things checked here are of exactly that kind — a rule stated in prose
// that only the model can honour:
//
//  1. **The English method families.** The library now carries vocabulary,
//     sentence-format and story-line entries (2026-09-04), but the model picks
//     1–3 ids out of 25 and the twelve it had before were ALL about argument.
//     Adding entries does nothing if it keeps choosing what it always chose.
//  2. **`ready`.** 印记 can finally say 「这份计划够写了」 — but a field the
//     model never sets is the same as no field, and the bug being fixed is
//     literally 「ai never auto triggers」.

import (
	"context"
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

func liveCompose(t *testing.T) (gateway.Provider, gateway.Resolved) {
	t.Helper()
	return liveClass(t, gateway.ClassCompose)
}

// TestLiveWritingGuideReachesTheNewEnglishFamilies — a student writing an
// English NARRATIVE block must be offered something about narrative or
// sentences, not a concession frame.
//
// The assertion is deliberately loose about WHICH id: picking the exact best
// method is judgement, and pinning one would be scoring the model rather than
// checking the product. What must not happen is the whole English half of the
// library staying invisible.
func TestLiveWritingGuideReachesTheNewEnglishFamilies(t *testing.T) {
	p, r := liveCompose(t)

	wr := sqlc.Writing{
		Title: "A small moment that changed how I see something",
		Lang:  "en",
	}
	block := sqlc.WritingOutline{Text: "The morning on the mountain steps", Role: "开头"}
	msgs := []sqlc.AtomMessage{
		{Role: "student", Content: "I want to write about watching a pilgrim climb the steps at Wutai Shan at four in the morning."},
		{Role: "student", Content: "My sentences all come out the same length and they sound flat."},
	}

	// The narrative + sentence-craft families, i.e. everything this batch
	// added that is not about argument.
	wanted := map[string]bool{
		"en_story_scene": true, "en_story_turn": true, "en_story_landing": true,
		"en_sentence_variety": true, "en_sentence_opener": true,
		"en_sentence_appositive": true, "en_sentence_parallel": true,
		"en_word_precision": true, "en_word_register": true,
	}

	hits := 0
	const samples = 3
	for i := 0; i < samples; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		res, err := gateway.Collect(ctx, p, r, gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: writingGuideSystem},
				{Role: gateway.RoleUser, Content: buildWritingGuidePrompt(wr, block, nil, "", msgs, "")},
			},
		})
		cancel()
		if err != nil {
			t.Fatalf("sample %d: live call failed: %v", i, err)
		}
		got, ok := parseWritingGuide(res.Text)
		if !ok {
			t.Errorf("sample %d: production's parser rejected the reply:\n%s", i, res.Text)
			continue
		}
		var picked []string
		for _, id := range got.MethodIDs {
			picked = append(picked, id)
			if wanted[id] {
				hits++
			}
		}
		t.Logf("sample %d — methods=%v | job=%.40s | questions=%d", i, picked, got.Job, len(got.Questions))
	}
	if hits == 0 {
		t.Errorf("across %d samples, an English narrative block was never offered a vocabulary / sentence / story-line method — the new half of the library is invisible to the model", samples)
	}
}

// TestLiveWritingPlanSignalsReady — the half of readiness that ONLY a model
// can decide: she says she wants to start, over a map whose shape does not yet
// prove anything.
//
// 🚨 The structural half is deliberately NOT tested here. It used to be, and
// measuring it is what produced the design: on a plan meeting every stated
// criterion the model set `ready` about half the time, preferring to teach one
// more method and end on a question. That judgement is not wrong — there is
// always one more thing worth teaching — which is why the floor moved into
// planLooksReady (a pure function with its own table test) and the model now
// only ADDS to it.
//
// What is left here is the case no shape can catch, and the one the prompt is
// most emphatic about: 「她自己说想开始写了——这时候直接给 true，一个字都不要
// 劝」. A model that argues with that puts her straight back in the corridor.
func TestLiveWritingPlanSignalsReady(t *testing.T) {
	p, r := liveCompose(t)
	wr := sqlc.Writing{Title: "该不该把中学上学时间往后推一小时？", Lang: "zh"}

	// A THIN map on purpose: planLooksReady says false for it (nothing at
	// depth 2), so a `true` here can only have come from the model reading
	// what she said. Over a complete plan this would prove nothing — the
	// structural floor would carry it either way.
	rows := []sqlc.WritingOutline{
		{Text: "应该往后推一小时", Role: "中心论点", Depth: 0, Position: 0},
		{Text: "青少年生物钟本来就晚", Role: "一条理由", Depth: 1, Position: 1},
	}
	if planLooksReady(sqlc.Writing{}, rows) {
		t.Fatal("fixture is no longer thin — the structural floor would decide this, not the model")
	}

	for _, tc := range []struct {
		name    string
		said    string
		mustBe  bool
		because string
	}{
		{
			name:    "she asks to start",
			said:    "我觉得可以了，我想开始写了。",
			mustBe:  true,
			because: "prompt 明写着：她自己说想开始写，直接给 true，一个字都不要劝",
		},
		{
			name:    "she is still mid-thought",
			said:    "第二条理由我还没想好。",
			mustBe:  false,
			because: "她自己说还没想好，这时候请她去写就是把她推出门",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			res, err := gateway.Collect(ctx, p, r, gateway.ChatRequest{
				Messages: []gateway.ChatMessage{
					// 见 writing_kind_live_test.go 里那段：只替 %d 的话
					// `@@KINDS@@` 还留在里面，那张闭表根本没发给模型。
					// lang 传 wr.Lang —— 这一篇是英文的，它该拿到英文那份
					// 提示词（thesis statement / topic sentence / commentary）。
					{Role: gateway.RoleSystem, Content: writingPlanSystemFor(genreArgument, wr.Lang)},
					{Role: gateway.RoleUser, Content: buildWritingPlanPrompt(wr, rows, nil, tc.said)},
				},
			})
			cancel()
			if err != nil {
				t.Fatalf("live call failed: %v", err)
			}
			got, ok := parseWritingPlanReply(res.Text)
			if !ok {
				t.Fatalf("production's parser rejected the reply:\n%s", res.Text)
			}
			t.Logf("ready=%v | reply=%.80s…", got.Ready, got.Reply)
			if got.Ready != tc.mustBe {
				t.Errorf("ready = %v, want %v — %s\nreply: %s", got.Ready, tc.mustBe, tc.because, got.Reply)
			}
			// The ready turn is an invitation, not another question.
			if got.Ready && strings.Count(got.Reply, "？")+strings.Count(got.Reply, "?") > 1 {
				t.Errorf("the ready turn should invite her to write, not keep interrogating: %s", got.Reply)
			}
		})
	}
}
