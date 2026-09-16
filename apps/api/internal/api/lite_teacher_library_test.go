package api_test

import (
	"net/http"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/disciplines"
	"mindimprint/api/internal/library"
)

type libraryGroupRecommendedResp struct {
	Articles []struct {
		Slug      string   `json:"slug"`
		ZhTitle   string   `json:"zhTitle"`
		Why       []string `json:"why"`
		ReadCount int      `json:"readCount"`
	} `json:"articles"`
	Tier int `json:"tier"`
}

// TestLiteTeacherLibraryRecommendedAuthz — same posture as every other lite
// teacher route: a teacher who does not own the class gets 404, a student
// gets 403.
func TestLiteTeacherLibraryRecommendedAuthz(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	_ = teacher

	other := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "lt-lib-other@demo.local"))
	if code := getJSON(t, h, other, "/api/v1/lite/teacher/classes/"+classID+"/library/recommended", nil); code != http.StatusNotFound {
		t.Fatalf("other teacher = %d, want 404", code)
	}

	student := signInAs(t, pool, studentID)
	if code := getJSON(t, h, student, "/api/v1/lite/teacher/classes/"+classID+"/library/recommended", nil); code != http.StatusForbidden {
		t.Fatalf("student = %d, want 403", code)
	}
}

// TestLiteTeacherLibraryRecommendedCarriesWhyAndTier pins the shape the brief
// asks for: an article shaped like GET /library's own article, plus why it
// was picked, how many of the class read it, and the class's shared tier.
func TestLiteTeacherLibraryRecommendedCarriesWhyAndTier(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	d := library.All()[len(library.All())-1].Disciplines[0]
	seedInterestFor(t, pool, studentID, d)

	var resp libraryGroupRecommendedResp
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/library/recommended", &resp); code != http.StatusOK {
		t.Fatalf("recommended = %d", code)
	}
	if len(resp.Articles) == 0 {
		t.Fatalf("recommended = %+v, want at least one article", resp)
	}
	want, _ := disciplines.ByID(d)
	first := resp.Articles[0]
	if first.ZhTitle == "" || len(first.Why) == 0 || first.Why[0] != want.Zh {
		t.Fatalf("first article = %+v, want why to name %s", first, want.Zh)
	}
	if resp.Tier < 1 || resp.Tier > 5 {
		t.Fatalf("tier = %d, want 1..5", resp.Tier)
	}
	if first.ReadCount != 0 {
		t.Fatalf("readCount = %d, want 0 — nobody has opened it yet", first.ReadCount)
	}
}

// TestLiteTeacherLibraryRecommendedReadCount — once a student in the class
// opens an article, it still appears (a class-wide recommendation does not
// drop an article the moment one member reads it) and ReadCount says so.
func TestLiteTeacherLibraryRecommendedReadCount(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	all := library.All()
	slug := all[0].Slug
	if code := assignJSON(t, h, signInAs(t, pool, studentID), "POST", "/api/v1/library/"+slug+"/levels/2", nil, nil); code != http.StatusCreated {
		t.Fatalf("open an article = %d", code)
	}

	var resp libraryGroupRecommendedResp
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/library/recommended?limit=20", &resp); code != http.StatusOK {
		t.Fatalf("recommended = %d", code)
	}
	var found bool
	for _, a := range resp.Articles {
		if a.Slug == slug {
			found = true
			if a.ReadCount != 1 {
				t.Fatalf("readCount = %d, want 1", a.ReadCount)
			}
		}
	}
	if !found {
		t.Fatalf("recommended = %+v, want the opened article %s still present", resp.Articles, slug)
	}
}

// TestLiteTeacherLibraryRecommendedLimit — ?limit= bounds the row count.
func TestLiteTeacherLibraryRecommendedLimit(t *testing.T) {
	h, _, teacher, classID, _ := liteTeacherFixture(t)
	var resp libraryGroupRecommendedResp
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/library/recommended?limit=3", &resp); code != http.StatusOK {
		t.Fatalf("recommended = %d", code)
	}
	if len(resp.Articles) > 3 {
		t.Fatalf("articles = %d, want <= 3", len(resp.Articles))
	}
}

// TestLiteTeacherLibraryRecommendedLimitIsCapped — an oversized ?limit= is
// clamped, not passed straight through to RecommendForGroup.
func TestLiteTeacherLibraryRecommendedLimitIsCapped(t *testing.T) {
	h, _, teacher, classID, _ := liteTeacherFixture(t)
	var resp libraryGroupRecommendedResp
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/library/recommended?limit=9999", &resp); code != http.StatusOK {
		t.Fatalf("recommended = %d", code)
	}
	if len(resp.Articles) > 24 {
		t.Fatalf("articles = %d, want <= 24 (the cap), even for an oversized limit", len(resp.Articles))
	}
}
