package agent

import "testing"

func TestBuildAssessmentInputDigestsProcessRecord(t *testing.T) {
	in := BuildAssessmentInput(
		[]EventDigest{
			{Type: "card_surfaced", Order: 1, Text: "CRAAP 卡触发"},
			{Type: "card_completed", Order: 2, Text: "学生自发完成 SIFT"},
		},
		[]CardUse{{CardID: "sift", Dimension: "D3", Spont: "自发"}},
		[]DispositionUse{{Kind: "reject", Reason: "我不同意这条，因为原文语境不同"}},
		[]string{"S3 evaluate: solid", "S5 draft_polish: owed"},
		[]int{1780},
		[]string{"表D 熟练", "表E 发展中"},
		"claims:1 evidence:2 concession:1（钢人由学生撰写）",
		nil,
		true,
		[]string{"从「中国让地球更可持续」改为限定判断版本"},
	)
	if len(in.CardUses) != 1 || in.CardUses[0].Spont != "自发" {
		t.Fatalf("card uses not carried: %+v", in.CardUses)
	}
	if len(in.Timeline) != 2 || in.Timeline[0] != "1. card_surfaced：CRAAP 卡触发" {
		t.Fatalf("timeline order/format wrong: %+v", in.Timeline)
	}
	if in.SnapshotCount != 1 || len(in.WordCounts) != 1 {
		t.Fatalf("snapshot facts wrong: %+v", in)
	}
	if len(in.Dispositions) != 1 || in.Dispositions[0].Kind != "reject" {
		t.Fatalf("dispositions not carried")
	}
	if !in.ProjectProjection {
		t.Fatalf("ProjectProjection not carried: %+v", in)
	}
	if len(in.WorkSamples) != 1 || in.WorkSamples[0] == "" {
		t.Fatalf("WorkSamples not carried: %+v", in.WorkSamples)
	}
}

func TestBuildAssessmentInputEmptyProjectIsMinimal(t *testing.T) {
	in := BuildAssessmentInput(nil, nil, nil, nil, nil, nil, "", nil, false, nil)
	if len(in.Timeline) != 0 || in.SnapshotCount != 0 || in.GraphSummary != "" {
		t.Fatalf("empty project should digest to a minimal input: %+v", in)
	}
	if in.ProjectProjection {
		t.Fatalf("ProjectProjection should default false on a non-project surface")
	}
	if len(in.WorkSamples) != 0 {
		t.Fatalf("WorkSamples should be empty when not supplied: %+v", in.WorkSamples)
	}
}

// The chat/course surfaces are non-project: ProjectProjection stays false and
// WorkSamples stays empty even when other fields are populated.
func TestBuildAssessmentInputNonProjectSurfaceOmitsProjectFields(t *testing.T) {
	in := BuildAssessmentInput(
		[]EventDigest{{Type: "chat_turn", Order: 1, Text: "问 CRAAP"}},
		nil, nil, nil, nil, nil, "", nil,
		false, nil,
	)
	if in.ProjectProjection {
		t.Fatalf("chat/course surface must not set ProjectProjection")
	}
	if len(in.WorkSamples) != 0 {
		t.Fatalf("chat/course surface must not carry WorkSamples: %+v", in.WorkSamples)
	}
}
