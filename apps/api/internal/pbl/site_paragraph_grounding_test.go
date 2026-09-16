package pbl

import (
	"strings"
	"testing"
)

func TestSectionCanCombineSeparateGroundedRecords(t *testing.T) {
	corpus := "Revision reason\nThe flowers are too small.\nOTHER RECORD\nTrial result\nKeyboard opens the dialog."
	body := "## Revision reason\n\nThe flowers are too small.\n\n## Trial result\n\nKeyboard opens the dialog."
	got, dropped := GroundSiteDraft(SiteDraft{Sections: []SiteSection{{Key: "process", Body: body}}}, corpus)
	if len(dropped) > 0 || len(got.Sections) != 1 || got.Sections[0].Body != body {
		t.Fatal(got, dropped)
	}
	bad := strings.Replace(body, "Trial result", "I won an award", 1)
	got, dropped = GroundSiteDraft(SiteDraft{Sections: []SiteSection{{Key: "process", Body: bad}}}, corpus)
	if len(got.Sections) != 0 || len(dropped) == 0 {
		t.Fatal("unsupported heading passed", got, dropped)
	}
}
