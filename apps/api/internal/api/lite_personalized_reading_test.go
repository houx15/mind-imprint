package api_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/disciplines"
	"mindimprint/api/internal/library"
)

// seedInterestFor writes one keyword routed to disciplineID for userID, at
// strength 4 and confidence 1.0, so her profile reads {disciplineID: 4}.
func seedInterestFor(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, disciplineID string) {
	t.Helper()
	d, ok := disciplines.ByID(disciplineID)
	if !ok {
		t.Fatalf("unknown discipline %q", disciplineID)
	}
	ctx := context.Background()
	var kid uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO interest_keyword (user_id, text_zh, text_en, norm, field, strength, note)
		VALUES ($1, $2, $2, $2, $3, 4, '') RETURNING id`, userID, disciplineID, d.Field).Scan(&kid); err != nil {
		t.Fatalf("seed keyword: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO keyword_discipline (keyword_id, discipline_id, confidence, how, rationale)
		VALUES ($1, $2, 1.0, 'alias', '')`, kid, disciplineID); err != nil {
		t.Fatalf("seed edge: %v", err)
	}
}

type previewRow struct {
	UserID        string `json:"userId"`
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	Title         string `json:"title"`
	Tier          int    `json:"tier"`
	SuggestedTier int    `json:"suggestedTier"`
	Reason        string `json:"reason"`
}

func postPreview(t *testing.T, h http.Handler, c *http.Cookie, classID string, body map[string]any) map[string]previewRow {
	t.Helper()
	var out struct {
		Rows []previewRow `json:"rows"`
	}
	if code := assignJSON(t, h, c, "POST", "/api/v1/lite/teacher/classes/"+classID+"/personalized-reading/preview", body, &out); code != http.StatusOK {
		t.Fatalf("preview = %d", code)
	}
	byUser := make(map[string]previewRow, len(out.Rows))
	for _, r := range out.Rows {
		byUser[r.UserID] = r
	}
	return byUser
}

// TestLibraryShelfStillRecommendsFromHerInterests pins the shelf through the
// profile refactor: a student with interest in one discipline gets it as the
// first recommendation's reason.
func TestLibraryShelfStillRecommendsFromHerInterests(t *testing.T) {
	h, pool, _, _, studentID := liteTeacherFixture(t)
	d := library.All()[len(library.All())-1].Disciplines[0]
	seedInterestFor(t, pool, studentID, d)
	var shelf struct {
		Recommended []struct {
			Why []string `json:"why"`
		} `json:"recommended"`
		Tier int `json:"tier"`
	}
	if code := getJSON(t, h, signInAs(t, pool, studentID), "/api/v1/library", &shelf); code != http.StatusOK {
		t.Fatalf("shelf = %d", code)
	}
	want, _ := disciplines.ByID(d)
	if len(shelf.Recommended) == 0 || len(shelf.Recommended[0].Why) == 0 || shelf.Recommended[0].Why[0] != want.Zh || shelf.Tier != 2 {
		t.Fatalf("shelf = %+v, want first recommendation for %s at tier 2", shelf, want.Zh)
	}
}

func TestPersonalizedPreview(t *testing.T) {
	h, pool, teacher, classID, s1 := liteTeacherFixture(t)
	s2 := createStudent(t, pool, SeedSchoolID, "pp-s2@demo.local")
	enrollStudent(t, pool, s2, classID)
	all := library.All()

	// s1 opens the first article at tier 4 and leaves it: excluded, suggested tier 3.
	if code := assignJSON(t, h, signInAs(t, pool, s1), "POST", "/api/v1/library/"+all[0].Slug+"/levels/"+strconv.Itoa(4), nil, nil); code != http.StatusCreated {
		t.Fatalf("s1 opens an article = %d", code)
	}
	d := all[len(all)-1].Disciplines[0]
	seedInterestFor(t, pool, s2, d)
	p1 := library.Profile{ReadSlugs: map[string]bool{all[0].Slug: true}, Tier: 3}
	p2 := library.Profile{Disciplines: map[string]float64{d: 4}, ReadSlugs: map[string]bool{}, Tier: 2}

	rows := postPreview(t, h, teacher, classID, map[string]any{})
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want one per enrolled student", rows)
	}
	want1, _ := library.PickForStudent(all, p1, nil)
	r1 := rows[s1.String()]
	if r1.Slug == all[0].Slug || r1.Slug != want1.Article.Slug || r1.Title != want1.Article.ZhTitle ||
		r1.Tier != 3 || r1.SuggestedTier != 3 || r1.Reason != "暂无兴趣数据，按难度推荐" || r1.Name == "" {
		t.Fatalf("s1 row = %+v, want %s at tier 3", r1, want1.Article.Slug)
	}
	zh, _ := disciplines.ByID(d)
	r2 := rows[s2.String()]
	want2, _ := library.PickForStudent(all, p2, nil)
	if r2.Slug != want2.Article.Slug || r2.Reason != "兴趣相关："+zh.Zh || r2.Tier != 2 {
		t.Fatalf("s2 row = %+v, want %s for her interest", r2, want2.Article.Slug)
	}

	// A tier and a filter: tier overrides, suggestedTier stays hers.
	filter := []string{all[0].Disciplines[0]}
	rows = postPreview(t, h, teacher, classID, map[string]any{"tier": 5, "disciplines": filter})
	p1.Tier, p2.Tier = 5, 5
	want1, _ = library.PickForStudent(all, p1, filter)
	want2, _ = library.PickForStudent(all, p2, filter)
	if r := rows[s1.String()]; r.Slug != want1.Article.Slug || r.Reason != want1.Reason() || r.Tier != 5 || r.SuggestedTier != 3 {
		t.Fatalf("filtered s1 row = %+v, want %s %q", r, want1.Article.Slug, want1.Reason())
	}
	if r := rows[s2.String()]; r.Slug != want2.Article.Slug || r.Reason != want2.Reason() || r.SuggestedTier != 2 {
		t.Fatalf("filtered s2 row = %+v, want %s %q", r, want2.Article.Slug, want2.Reason())
	}

	path := "/api/v1/lite/teacher/classes/" + classID + "/personalized-reading/preview"
	if code, errCode := writeErrorCode(t, h, teacher, "POST", path, map[string]any{"disciplines": []string{"no-such"}}); code != http.StatusBadRequest || errCode != "invalid_discipline" {
		t.Fatalf("bad discipline = %d %s", code, errCode)
	}
	if code, errCode := writeErrorCode(t, h, teacher, "POST", path, map[string]any{"tier": 9}); code != http.StatusBadRequest || errCode != "invalid_tier" {
		t.Fatalf("bad tier = %d %s", code, errCode)
	}
	other := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "pp-other@demo.local"))
	if code := assignJSON(t, h, other, "POST", path, map[string]any{}, nil); code != http.StatusNotFound {
		t.Fatalf("other teacher preview = %d, want 404", code)
	}
	if code := assignJSON(t, h, signInAs(t, pool, s1), "POST", path, map[string]any{}, nil); code == http.StatusOK {
		t.Fatalf("student preview = %d, want refused", code)
	}
}

func libraryReadingOf(t *testing.T, pool *pgxpool.Pool, atomID string) (string, int) {
	t.Helper()
	var slug string
	var tier int
	if err := pool.QueryRow(context.Background(),
		`SELECT library_slug, library_tier FROM reading WHERE atom_id = $1`, atomID).Scan(&slug, &tier); err != nil {
		t.Fatalf("reading %s: %v", atomID, err)
	}
	return slug, tier
}

func personalizedPayload(filter []string, picks map[string]any) map[string]any {
	p := map[string]any{"source": "personalized", "picks": picks}
	if filter != nil {
		p["disciplines"] = filter
	}
	return p
}

func TestPersonalizedPicksMustBeRecipients(t *testing.T) {
	h, pool, teacher, classID, s1 := liteTeacherFixture(t)
	s2 := createStudent(t, pool, SeedSchoolID, "pr-s2@demo.local")
	enrollStudent(t, pool, s2, classID)
	slug := library.All()[0].Slug
	path := "/api/v1/lite/teacher/classes/" + classID + "/assignments"

	// s2 is enrolled but not a recipient.
	body := readingAssignmentBody("个性化", personalizedPayload(nil, map[string]any{
		s2.String(): map[string]any{"slug": slug},
	}), []string{s1.String()})
	if code, errCode := writeErrorCode(t, h, teacher, "POST", path, body); code != http.StatusBadRequest || errCode != "pick_not_recipient" {
		t.Fatalf("create with a non-recipient pick = %d %s", code, errCode)
	}

	aid := createAssignment(t, h, teacher, classID, readingAssignmentBody("个性化", personalizedPayload(nil, map[string]any{
		s1.String(): map[string]any{"slug": slug},
	}), []string{s1.String()}))
	withS2 := map[string]any{"payload": personalizedPayload(nil, map[string]any{
		s1.String(): map[string]any{"slug": slug}, s2.String(): map[string]any{"slug": slug},
	})}
	if code, errCode := writeErrorCode(t, h, teacher, "PATCH", "/api/v1/lite/teacher/assignments/"+aid, withS2); code != http.StatusBadRequest || errCode != "pick_not_recipient" {
		t.Fatalf("patch with a non-recipient pick = %d %s", code, errCode)
	}
	// Adding s2 in the same request makes the pick valid.
	withS2["addUserIds"] = []string{s2.String()}
	if code := assignJSON(t, h, teacher, "PATCH", "/api/v1/lite/teacher/assignments/"+aid, withS2, nil); code != http.StatusOK {
		t.Fatalf("patch adding s2 with her pick = %d", code)
	}
	// An unknown article is refused by payload validation.
	bad := readingAssignmentBody("个性化", personalizedPayload(nil, map[string]any{
		s1.String(): map[string]any{"slug": "no-such-article"},
	}), []string{s1.String()})
	if code, errCode := writeErrorCode(t, h, teacher, "POST", path, bad); code != http.StatusBadRequest || errCode != "invalid_pick_slug" {
		t.Fatalf("unknown pick slug = %d %s", code, errCode)
	}
}

func TestPersonalizedStart(t *testing.T) {
	h, pool, teacher, classID, s1 := liteTeacherFixture(t)
	s2 := createStudent(t, pool, SeedSchoolID, "ps-s2@demo.local")
	s3 := createStudent(t, pool, SeedSchoolID, "ps-s3@demo.local")
	enrollStudent(t, pool, s2, classID)
	enrollStudent(t, pool, s3, classID)
	all := library.All()
	filter := []string{all[len(all)-1].Disciplines[0]}

	aid := createAssignment(t, h, teacher, classID, readingAssignmentBody("个性化", personalizedPayload(filter, map[string]any{
		s1.String(): map[string]any{"slug": all[1].Slug, "tier": 4},
		s2.String(): map[string]any{"slug": all[2].Slug, "tier": nil},
	}), []string{s1.String(), s2.String()}))
	// s3 joins after the picks were made: no pick.
	if code := assignJSON(t, h, teacher, "PATCH", "/api/v1/lite/teacher/assignments/"+aid,
		map[string]any{"addUserIds": []string{s3.String()}}, nil); code != http.StatusOK {
		t.Fatalf("add s3 = %d", code)
	}

	out := startAssignment(t, h, signInAs(t, pool, s1), aid)
	if slug, tier := libraryReadingOf(t, pool, out.AtomID); out.Kind != "reading" || slug != all[1].Slug || tier != 4 {
		t.Fatalf("s1 started %s tier %d, want %s tier 4", slug, tier, all[1].Slug)
	}
	out = startAssignment(t, h, signInAs(t, pool, s2), aid)
	if slug, tier := libraryReadingOf(t, pool, out.AtomID); slug != all[2].Slug || tier != library.SuggestTier(0, 0) {
		t.Fatalf("s2 started %s tier %d, want %s at her suggested tier", slug, tier, all[2].Slug)
	}
	want, _ := library.PickForStudent(all, library.Profile{Tier: library.SuggestTier(0, 0)}, filter)
	out = startAssignment(t, h, signInAs(t, pool, s3), aid)
	if slug, tier := libraryReadingOf(t, pool, out.AtomID); slug != want.Article.Slug || tier != library.SuggestTier(0, 0) {
		t.Fatalf("s3 started %s tier %d, want the recommendation %s", slug, tier, want.Article.Slug)
	}
	// Starting again returns the same item.
	if again := startAssignment(t, h, signInAs(t, pool, s3), aid); again.AtomID != out.AtomID {
		t.Fatalf("repeat start = %s, want %s", again.AtomID, out.AtomID)
	}
}
