package api

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/library"
	"mindimprint/api/internal/liteassign"
)

// TestLiteWorkspaceClampsToWhatPublishAccepts — the clamps a tool applies must
// be the caps the publish path enforces, field by field.
//
// The two differ by an order of magnitude (200 for the title, 2000 for the
// instructions), and getting it wrong is invisible until the last step: the
// card shows a title we wrote ourselves and 发布作业 then refuses it with
// 「请填写作业标题，不超过 200 字」. This test feeds the clamped value straight
// into the real validators, so it fails if either cap drifts.
func TestLiteWorkspaceClampsToWhatPublishAccepts(t *testing.T) {
	long := strings.Repeat("气", 5000)

	title := liteWorkspaceClampRunes(long, maxAssignmentTitleRunes)
	if _, err := parseAssignmentTitle(title); err != nil {
		t.Fatalf("a clamped title is rejected at publish: %v", err)
	}

	ins := liteWorkspaceClampRunes(long, liteWorkspaceMaxInstructionsRunes)
	if _, err := liteassign.ValidateInstructions(ins); err != nil {
		t.Fatalf("clamped instructions are rejected at publish: %v", err)
	}

	// The title cap is the smaller of the two. If someone ever raises it to
	// the instructions cap, the assertion above stops meaning anything, so
	// pin the relationship too.
	if maxAssignmentTitleRunes >= liteWorkspaceMaxInstructionsRunes {
		t.Fatalf("title cap %d is no longer smaller than the instructions cap %d",
			maxAssignmentTitleRunes, liteWorkspaceMaxInstructionsRunes)
	}
}

// TestLiteWorkspaceCardStateIsAllChinese pins spec §12.1: the text that goes
// into the model's system prompt must carry only the words the teacher
// already knows from the assignment form, never a wire value. Production once
// put 「种类：reading」「材料来源：library」「文章 slug：biden-creates-climate-corps」
// in front of the model, and it read them back to her.
func TestLiteWorkspaceCardStateIsAllChinese(t *testing.T) {
	tier := 3
	raw, err := json.Marshal(liteWorkspaceArtifact{
		Kind:          "reading",
		Title:         "美国气候队",
		ReadingSource: "library",
		Slug:          "biden-creates-climate-corps",
		Tier:          &tier,
	})
	if err != nil {
		t.Fatalf("marshal artifact: %v", err)
	}
	got := liteWorkspaceCardState(raw, nil)

	for _, wire := range []string{"reading", "library", "biden-creates-climate-corps"} {
		if strings.Contains(got, wire) {
			t.Fatalf("card state still carries the wire value %q: %q", wire, got)
		}
	}
	for _, chinese := range []string{"类型：阅读", "材料来源：分级阅读库", "文章：《美国气候队》", "难度：进阶"} {
		if !strings.Contains(got, chinese) {
			t.Fatalf("card state = %q, want it to contain %q", got, chinese)
		}
	}
}

// TestLiteWorkspaceCardStateUnknownSlug — an article the catalogue does not
// carry must not fall back to showing the slug: the teacher would see the
// same raw value the lookup was added to hide.
func TestLiteWorkspaceCardStateUnknownSlug(t *testing.T) {
	raw, err := json.Marshal(liteWorkspaceArtifact{Kind: "reading", ReadingSource: "library", Slug: "not-a-real-article"})
	if err != nil {
		t.Fatalf("marshal artifact: %v", err)
	}
	got := liteWorkspaceCardState(raw, nil)

	if strings.Contains(got, "not-a-real-article") {
		t.Fatalf("card state leaked an unknown slug: %q", got)
	}
	if !strings.Contains(got, "文章：未找到") {
		t.Fatalf("card state = %q, want 文章：未找到", got)
	}
}

// TestLiteWorkspaceRecommendArticlesLoadsProfilesLazily — most turns never
// call recommend_articles, and building every enrolled student's profile
// (ListLiteWeekClassStudents + three queries per student inside
// libraryProfileIn — roughly ninety round trips for a class of 30) must not
// run for a turn that never reaches for the tool.
func TestLiteWorkspaceRecommendArticlesLoadsProfilesLazily(t *testing.T) {
	calls := 0
	run := &liteWorkspaceRun{
		groupProfilesLoad: func() ([]library.Profile, error) {
			calls++
			return []library.Profile{{Tier: 2}}, nil
		},
	}
	run.execute(gateway.ToolCall{Name: "set_fields", Args: map[string]any{"title": "阅读作业"}})
	if calls != 0 {
		t.Fatalf("loader calls = %d, want 0 — set_fields must never touch the profile loader", calls)
	}
}

// TestLiteWorkspaceRecommendArticlesCachesProfilesWithinATurn — the model may
// call recommend_articles more than once in one turn (retrying a bad
// discipline filter, say); the second call must reuse the first's fetch
// rather than pay for it again.
func TestLiteWorkspaceRecommendArticlesCachesProfilesWithinATurn(t *testing.T) {
	calls := 0
	run := &liteWorkspaceRun{
		groupProfilesLoad: func() ([]library.Profile, error) {
			calls++
			return []library.Profile{{Tier: 2}}, nil
		},
	}
	run.execute(gateway.ToolCall{Name: "recommend_articles", Args: map[string]any{}})
	run.execute(gateway.ToolCall{Name: "recommend_articles", Args: map[string]any{}})
	if calls != 1 {
		t.Fatalf("loader calls = %d, want 1 — the second call must reuse the cached profiles", calls)
	}
}

// TestLiteWorkspaceRecommendArticlesLoadErrorIsAToolError — a loader failure
// (a transient DB error, say) must come back as a tool result the model can
// react to, not fail the turn. execute has no error return path at all: the
// only way a failure here can reach the teacher is through the reply the
// model writes after reading this tool error.
func TestLiteWorkspaceRecommendArticlesLoadErrorIsAToolError(t *testing.T) {
	run := &liteWorkspaceRun{
		groupProfilesLoad: func() ([]library.Profile, error) {
			return nil, errors.New("connection refused")
		},
	}
	got := run.execute(gateway.ToolCall{Name: "recommend_articles", Args: map[string]any{}})
	var decoded struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("decode tool result: %v — %s", err, got)
	}
	if decoded.OK || !strings.Contains(decoded.Error, "connection refused") {
		t.Fatalf("tool result = %+v, want ok=false carrying the underlying error", decoded)
	}
	if len(run.cards) != 0 {
		t.Fatalf("cards = %+v, want none — a failed load must not push an articles card", run.cards)
	}
}
