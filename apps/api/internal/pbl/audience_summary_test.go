package pbl

import "testing"

func TestAudienceSummaryRequiresEveryBoardAndExistingSource(t *testing.T) {
	doc := AudienceDocument{Boards: []AudienceBoard{{ID: "teacher", Interests: []string{"制作过程"}, Offerings: []string{"校园摄影作品"}}, {ID: "peer", Interests: []string{"活动地点"}, Offerings: []string{"校园拍摄路线"}}}}
	valid := `{"boards":[{"boardId":"teacher","keywords":[{"text":"摄影","sourceField":"offerings","sourceIndex":0}]},{"boardId":"peer","keywords":[{"text":"校园路线","sourceField":"offerings","sourceIndex":0}]}]}`
	if _, err := ParseAudienceSummary(valid, doc); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		`{"boards":[]}`,
		`{"boards":[{"boardId":"teacher","keywords":[]},{"boardId":"peer","keywords":[]}]}`,
		`{"boards":[{"boardId":"teacher","keywords":[{"text":"摄影","sourceField":"ageRange","sourceIndex":0}]},{"boardId":"peer","keywords":[{"text":"路线","sourceField":"offerings","sourceIndex":0}]}]}`,
		`{"boards":[{"boardId":"teacher","keywords":[{"text":"摄影","sourceField":"offerings","sourceIndex":9}]},{"boardId":"peer","keywords":[{"text":"路线","sourceField":"offerings","sourceIndex":0}]}]}`,
		`{"boards":[{"boardId":"teacher","keywords":[{"text":"摄影","sourceField":"offerings","sourceIndex":0}]},{"boardId":"teacher","keywords":[{"text":"路线","sourceField":"offerings","sourceIndex":0}]}]}`,
	} {
		if _, err := ParseAudienceSummary(raw, doc); err == nil {
			t.Fatalf("invalid summary accepted: %s", raw)
		}
	}
}
