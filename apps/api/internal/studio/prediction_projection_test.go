package studio

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
)

func TestProjectPrediction_PreReviewNotRevealed(t *testing.T) {
	sk := testSkill(t)
	// S0 predicted 表D + 表E weakest (indices 0,1); no review yet.
	d := ProjectData{Nodes: []sqlc.GraphNode{
		{Type: "task_restatement", Body: []byte(`{"restate":"...","weak_picks":[0,1]}`)},
	}}
	p := projectPrediction(sk, d)
	if len(p.Predicted) != 2 || p.Predicted[0].Code != "表D" || p.Predicted[1].Code != "表E" {
		t.Fatalf("predicted = %+v, want 表D+表E", p.Predicted)
	}
	if p.Revealed {
		t.Error("revealed = true pre-review, want false")
	}
	if len(p.Actual) != 0 {
		t.Errorf("actual = %+v, want [] pre-review", p.Actual)
	}
}

func TestProjectPrediction_RevealedOverlap(t *testing.T) {
	sk := testSkill(t)
	snapID := uuid.New()
	mkReviewIV := func(code string, points int) sqlc.Intervention {
		body, _ := json.Marshal(agent.ReviewItem{CriterionCode: code, Points: points})
		anchor, _ := json.Marshal(map[string]string{
			"kind":  "draft_snapshot",
			"id":    snapID.String(),
			"voice": "board",
		})
		return sqlc.Intervention{ID: uuid.New(), Type: "review_item", Anchor: anchor, Body: string(body)}
	}
	// S0 predicted 表D + 表E (indices 0,1). Board review: 表D partial (2/4),
	// 表E full (4/4). 表F/表H untouched (empty). Actual = non-full = 表D,表F,表H.
	// Overlap with predicted = 表D only (表E was predicted but came back full).
	d := ProjectData{
		Nodes: []sqlc.GraphNode{
			{Type: "task_restatement", Body: []byte(`{"restate":"...","weak_picks":[0,1]}`)},
		},
		LatestSnapshot: &sqlc.DraftSnapshot{ID: snapID},
		Interventions: []sqlc.Intervention{
			mkReviewIV("表D", 2),
			mkReviewIV("表E", 4),
		},
	}
	p := projectPrediction(sk, d)
	if !p.Revealed {
		t.Fatal("revealed = false, want true once a board review has landed")
	}
	if len(p.Actual) != 3 {
		t.Fatalf("actual = %+v, want 3 non-full criteria (表D,表F,表H)", p.Actual)
	}
	if p.Overlap != 1 {
		t.Errorf("overlap = %d, want 1 (表D only — 表E predicted but came back full)", p.Overlap)
	}
}
