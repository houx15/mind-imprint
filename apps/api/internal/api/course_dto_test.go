package api

import (
	"encoding/json"
	"strings"
	"testing"

	"mindimprint/api/internal/agent"
)

func TestCourseSummaryDTOEmitsCatalogMetadata(t *testing.T) {
	cat := "source-check"
	rank := int32(2)
	a := &API{} // resolveCoverURL tolerates a nil deps cover signer for an empty cover
	dto := a.toCourseSummaryDTO(agent.CourseSummaryRow{
		Slug: "x", Branch: "A", Title: "T", Blurb: "b", TimeLabel: "10m",
		Category: &cat, Introduction: []byte(`{"hook":"h"}`), FeaturedRank: &rank,
	})
	b, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `"category":"source-check"`) {
		t.Fatalf("category missing: %s", s)
	}
	if !strings.Contains(s, `"introduction":{"hook":"h"}`) {
		t.Fatalf("introduction not verbatim: %s", s)
	}
	if !strings.Contains(s, `"featuredRank":2`) {
		t.Fatalf("featuredRank missing: %s", s)
	}
}

func TestCourseSummaryDTONullsWhenAbsent(t *testing.T) {
	a := &API{}
	dto := a.toCourseSummaryDTO(agent.CourseSummaryRow{Slug: "x", Branch: "A", Title: "T"})
	b, _ := json.Marshal(dto)
	s := string(b)
	if !strings.Contains(s, `"category":null`) || !strings.Contains(s, `"introduction":null`) || !strings.Contains(s, `"featuredRank":null`) {
		t.Fatalf("absent fields must serialize as null: %s", s)
	}
}
