package agent

import (
	"testing"

	"mindimprint/api/internal/cards"
)

func mustMoney(t *testing.T) cards.Spec {
	t.Helper()
	s, ok := cards.ByID("money-trail")
	if !ok {
		t.Fatal("money-trail not in catalog")
	}
	return s
}

func TestRefeedSkippedIsMinimal(t *testing.T) {
	spec := mustMoney(t)
	p := SerializeCardForRefeed(spec, CardInstance{CardID: "money-trail", Status: "skipped"})
	if p.CardID != "money-trail" || p.CardName != spec.Name || p.Status != "skipped" {
		t.Fatalf("skipped payload wrong: %+v", p)
	}
	if len(p.Steps) != 0 {
		t.Fatalf("skipped payload must carry no steps, got %d", len(p.Steps))
	}
}

func TestRefeedCompletedPairsLabelsWithValues(t *testing.T) {
	spec := mustMoney(t)
	inst := CardInstance{
		CardID: "money-trail",
		Status: "completed",
		FieldValues: map[string]map[string]any{
			"main": {
				"claim": "糖对健康无害",
				"chain": []any{
					map[string]any{"who": "糖业协会", "role": "资助者", "source": "会员企业会费"},
					map[string]any{"who": "某大学实验室", "role": "发布者", "source": "协会资助的课题"},
				},
				"alignment": "一致",
				"meaning":   "出资方利益与结论一致，需找独立来源交叉验证。",
			},
		},
	}
	p := SerializeCardForRefeed(spec, inst)
	if p.Status != "completed" || p.CardID != "money-trail" {
		t.Fatalf("payload identity/status wrong: %+v", p)
	}
	if len(p.Steps) != 1 || p.Steps[0].Title != "资金链溯源" {
		t.Fatalf("steps wrong: %+v", p.Steps)
	}
	answers := p.Steps[0].Answers
	// the claim text field is paired with its label
	if answers[0].Label != "要溯源的说法是什么？" || answers[0].Value != "糖对健康无害" {
		t.Fatalf("claim answer wrong: %+v", answers[0])
	}
	// the repeatable_group is an array of {item-label: value}
	rows, ok := answers[1].Value.([]map[string]any)
	if !ok {
		t.Fatalf("chain value not []map: %T", answers[1].Value)
	}
	if rows[0]["节点（谁）"] != "糖业协会" || rows[0]["这是哪一环"] != "资助者" || rows[0]["它的钱 / 利益从哪来？"] != "会员企业会费" {
		t.Fatalf("remap wrong: %v", rows[0])
	}
}

func TestRefeedOmitsEmptyFields(t *testing.T) {
	spec := mustMoney(t)
	inst := CardInstance{
		CardID: "money-trail",
		Status: "completed",
		FieldValues: map[string]map[string]any{
			"main": {"claim": "x"},
		},
	}
	p := SerializeCardForRefeed(spec, inst)
	if len(p.Steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(p.Steps))
	}
	if len(p.Steps[0].Answers) != 1 {
		t.Fatalf("answers = %d, want 1 (only filled field)", len(p.Steps[0].Answers))
	}
	if p.Steps[0].Answers[0].Value != "x" {
		t.Fatalf("answer value = %v", p.Steps[0].Answers[0].Value)
	}
}

func TestRefeedRepeatableGroupRemap(t *testing.T) {
	spec := mustMoney(t)
	inst := CardInstance{
		CardID: "money-trail",
		Status: "completed",
		FieldValues: map[string]map[string]any{
			"main": {
				"chain": []any{
					map[string]any{"who": "糖业协会", "role": "资助者", "source": "会员企业会费"},
				},
			},
		},
	}
	p := SerializeCardForRefeed(spec, inst)
	rows, ok := p.Steps[0].Answers[0].Value.([]map[string]any)
	if !ok {
		t.Fatalf("repeatable value not []map: %T", p.Steps[0].Answers[0].Value)
	}
	if rows[0]["节点（谁）"] != "糖业协会" || rows[0]["这是哪一环"] != "资助者" || rows[0]["它的钱 / 利益从哪来？"] != "会员企业会费" {
		t.Fatalf("remap wrong: %v", rows[0])
	}
}

// TestRefeedFoldsAnnotationAnchors covers the flagship keystone/annotation card:
// its answers live on `anchors` (field_values stays empty), and they must reach
// the refeed payload — otherwise "摘要回灌" drops the student's actual thinking.
func TestRefeedFoldsAnnotationAnchors(t *testing.T) {
	spec := mustMoney(t)
	inst := CardInstance{
		CardID: "money-trail",
		Status: "completed",
		// annotation cards leave field_values empty; the student's answers ride anchors.
		Anchors: []Anchor{
			{Dimension: "溯源 (SIFT)", Question: "这条信息最初来自哪里？", Answer: "原始研究来自 NASA / Nature Sustainability", Author: "ai"},
			{Dimension: "可信度 (CRAAP)", Question: "这个来源可信吗？", Answer: "NASA 是官方机构，可信", Author: "ai"},
			{Dimension: "空", Question: "没回答", Answer: "", Author: "ai"}, // unanswered → must not leak
		},
	}
	p := SerializeCardForRefeed(spec, inst)
	if p.Status != "completed" {
		t.Fatalf("status = %q, want completed", p.Status)
	}
	if len(p.Steps) != 2 {
		t.Fatalf("steps = %d, want 2 (one per answered anchor dimension)", len(p.Steps))
	}
	if p.Steps[0].Title != "溯源 (SIFT)" {
		t.Fatalf("step[0] title = %q", p.Steps[0].Title)
	}
	if p.Steps[0].Answers[0].Label != "这条信息最初来自哪里？" ||
		p.Steps[0].Answers[0].Value != "原始研究来自 NASA / Nature Sustainability" {
		t.Fatalf("step[0] answer wrong: %+v", p.Steps[0].Answers[0])
	}
}

func TestAnchorStepsLabelsFallBackToQuoteBeforeDimension(t *testing.T) {
	inst := CardInstance{Anchors: []Anchor{
		// no Question — a sort/scale/matrix anchor. The sentence (quote) is
		// the only useful label; the dimension is already the step title.
		{Quote: "中国碳排放全球第一", Dimension: "事实", Answer: "可以去核查"},
		// no Question and no Quote — falls all the way back to the dimension.
		{Dimension: "观点", Answer: "需要给理由"},
	}}
	steps := anchorSteps(inst)
	if len(steps) != 2 {
		t.Fatalf("want 2 steps, got %d", len(steps))
	}
	if steps[0].Title != "事实" || steps[0].Answers[0].Label != "中国碳排放全球第一" {
		t.Fatalf("step0 = %+v", steps[0])
	}
	if steps[1].Answers[0].Label != "观点" {
		t.Fatalf("step1 label = %q, want 观点", steps[1].Answers[0].Label)
	}
}
