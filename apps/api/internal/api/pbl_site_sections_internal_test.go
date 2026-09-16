package api

import (
	"mindimprint/api/internal/pbl"
	"testing"
)

func TestMergeSiteSections_ModelCannotChangeConfirmedStructure(t *testing.T) {
	current := pbl.SiteDraft{Sections: []pbl.SiteSection{{Key: "a", Title: "观察", Depth: 0, Body: "旧内容"}, {Key: "b", Title: "邀请", Depth: 1}}}
	next := pbl.SiteDraft{Sections: []pbl.SiteSection{{Key: "b", Title: "更名", Depth: 0, Body: "一起观察"}, {Key: "unknown", Title: "新增", Body: "外来内容"}}}
	got := mergeSiteDraft(current, next)
	if len(got.Sections) != 2 || got.Sections[0].Body != "旧内容" || got.Sections[1].Title != "邀请" || got.Sections[1].Depth != 1 || got.Sections[1].Body != "一起观察" {
		t.Fatalf("structure changed: %+v", got.Sections)
	}
	if siteDraftEmpty(pbl.SiteDraft{Sections: []pbl.SiteSection{{Key: "a", Body: "有正文"}}}) {
		t.Fatal("section text was treated as empty")
	}
	if !siteDraftEmpty(pbl.SiteDraft{Sections: []pbl.SiteSection{{Key: "a", Title: "只有标题"}}}) {
		t.Fatal("heading mistaken for authored body")
	}
}
