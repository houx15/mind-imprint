package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
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

// TestLibraryShelfExcludesOpenedArticles pins profileFromRows, the helper
// shared by the shelf and libraryProfileIn: an article she has already opened
// is excluded from her recommendations.
func TestLibraryShelfExcludesOpenedArticles(t *testing.T) {
	h, pool, _, _, studentID := liteTeacherFixture(t)
	all := library.All()
	c := signInAs(t, pool, studentID)
	if code := assignJSON(t, h, c, "POST", "/api/v1/library/"+all[0].Slug+"/levels/2", nil, nil); code != http.StatusCreated {
		t.Fatalf("open an article = %d", code)
	}
	var shelf struct {
		Recommended []struct {
			Slug string `json:"slug"`
		} `json:"recommended"`
	}
	if code := getJSON(t, h, c, "/api/v1/library", &shelf); code != http.StatusOK {
		t.Fatalf("shelf = %d", code)
	}
	for _, rec := range shelf.Recommended {
		if rec.Slug == all[0].Slug {
			t.Fatalf("recommended = %+v, want the opened article %s excluded", shelf.Recommended, all[0].Slug)
		}
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
	// A student with no interest data and no read history recommends
	// articles[0] first (Recommend keeps library order when every score is
	// zero) — that is "base", the unfiltered pick. The filter must name a
	// discipline base does not carry, so the filtered recommendation for s3
	// below is provably a different article, not one that happens to match
	// with or without the filter.
	base, _ := library.PickForStudent(all, library.Profile{Tier: library.SuggestTier(0, 0)}, nil)
	var filter []string
	for _, art := range all {
		shared := false
		for _, d := range art.Disciplines {
			for _, bd := range base.Article.Disciplines {
				if d == bd {
					shared = true
				}
			}
		}
		if !shared {
			filter = []string{art.Disciplines[0]}
			break
		}
	}
	if filter == nil {
		t.Fatal("no article in the library carries a discipline outside base's own — cannot build a discriminating filter")
	}
	want := func() library.StudentPick {
		p, _ := library.PickForStudent(all, library.Profile{Tier: library.SuggestTier(0, 0)}, filter)
		return p
	}()
	if want.Article.Slug == base.Article.Slug {
		t.Fatalf("filter %v did not change the picked article (%s); the filter is not discriminating", filter, base.Article.Slug)
	}

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
	out = startAssignment(t, h, signInAs(t, pool, s3), aid)
	if slug, tier := libraryReadingOf(t, pool, out.AtomID); slug != want.Article.Slug || slug == base.Article.Slug || tier != library.SuggestTier(0, 0) {
		t.Fatalf("s3 started %s tier %d, want the filtered recommendation %s (not the unfiltered %s)", slug, tier, want.Article.Slug, base.Article.Slug)
	}
	// Starting again returns the same item.
	if again := startAssignment(t, h, signInAs(t, pool, s3), aid); again.AtomID != out.AtomID {
		t.Fatalf("repeat start = %s, want %s", again.AtomID, out.AtomID)
	}
}

// TestPersonalizedStartUsesClassTier: a student's pick with no tier of its
// own, and a student with no pick at all, both fall back to the homework's
// class-wide tier before falling back further to her own suggested tier.
func TestPersonalizedStartUsesClassTier(t *testing.T) {
	h, pool, teacher, classID, s1 := liteTeacherFixture(t)
	s2 := createStudent(t, pool, SeedSchoolID, "pt-s2@demo.local")
	enrollStudent(t, pool, s2, classID)
	all := library.All()

	aid := createAssignment(t, h, teacher, classID, readingAssignmentBody("班级难度", map[string]any{
		"source": "personalized",
		"tier":   3,
		"picks": map[string]any{
			s1.String(): map[string]any{"slug": all[0].Slug, "tier": nil},
		},
	}, []string{s1.String(), s2.String()}))

	// s1's pick leaves her tier open: falls back to the class-wide tier (3),
	// not her suggested tier (2, cold start).
	out := startAssignment(t, h, signInAs(t, pool, s1), aid)
	if slug, tier := libraryReadingOf(t, pool, out.AtomID); slug != all[0].Slug || tier != 3 {
		t.Fatalf("s1 started %s tier %d, want %s tier 3", slug, tier, all[0].Slug)
	}

	// s2 has no pick at all: the recommendation also uses the class-wide tier.
	want, _ := library.PickForStudent(all, library.Profile{Tier: 3}, nil)
	out = startAssignment(t, h, signInAs(t, pool, s2), aid)
	if slug, tier := libraryReadingOf(t, pool, out.AtomID); slug != want.Article.Slug || tier != 3 {
		t.Fatalf("s2 started %s tier %d, want the recommendation %s at tier 3", slug, tier, want.Article.Slug)
	}
}

// TestPersonalizedStartFallsBackWhenPickedArticleLeftTheLibrary covers M2:
// the library is embedded and changes only on a deploy, but a student whose
// saved pick names an article that is gone must still get a recommendation
// at start, the same as a student who never had a pick — not a dead end.
// PATCH refuses an unknown slug at save time, so the only way to reach this
// state is to edit the stored payload directly.
func TestPersonalizedStartFallsBackWhenPickedArticleLeftTheLibrary(t *testing.T) {
	h, pool, teacher, classID, s1 := liteTeacherFixture(t)
	all := library.All()

	aid := createAssignment(t, h, teacher, classID, readingAssignmentBody("个性化", personalizedPayload(nil, map[string]any{
		s1.String(): map[string]any{"slug": all[0].Slug},
	}), []string{s1.String()}))

	path := []string{"picks", s1.String(), "slug"}
	if _, err := pool.Exec(context.Background(),
		`UPDATE lite_assignment SET payload = jsonb_set(payload, $2, '"ghost-article"'::jsonb) WHERE id = $1`, aid, path); err != nil {
		t.Fatalf("rewrite stored pick: %v", err)
	}

	want, _ := library.PickForStudent(all, library.Profile{Tier: library.SuggestTier(0, 0)}, nil)
	out := startAssignment(t, h, signInAs(t, pool, s1), aid)
	if slug, _ := libraryReadingOf(t, pool, out.AtomID); slug != want.Article.Slug {
		t.Fatalf("started %s, want the fallback recommendation %s", slug, want.Article.Slug)
	}
}

// TestPersonalizedPatchKeepsRemovedRecipientsPick: a removed recipient's pick
// stays in the stored payload. Only a pick that is new or
// has changed since the stored payload is checked against the recipient list
// a PATCH leaves behind; a pick resent unchanged for a student who was just
// removed is not refused, and does not block a later save either.
func TestPersonalizedPatchKeepsRemovedRecipientsPick(t *testing.T) {
	h, pool, teacher, classID, s1 := liteTeacherFixture(t)
	s2 := createStudent(t, pool, SeedSchoolID, "pk-s2@demo.local")
	s3 := createStudent(t, pool, SeedSchoolID, "pk-s3@demo.local")
	enrollStudent(t, pool, s2, classID)
	enrollStudent(t, pool, s3, classID)
	slug := library.All()[0].Slug

	payload := personalizedPayload(nil, map[string]any{
		s1.String(): map[string]any{"slug": slug},
		s2.String(): map[string]any{"slug": slug},
	})
	aid := createAssignment(t, h, teacher, classID, readingAssignmentBody("个性化", payload, []string{s1.String(), s2.String()}))

	// Removing s2 while resending the identical payload: her pick is
	// unchanged, so it is not re-checked against the new recipient list, and
	// the save succeeds instead of 400-ing and rolling back the removal.
	if code := assignJSON(t, h, teacher, "PATCH", "/api/v1/lite/teacher/assignments/"+aid,
		map[string]any{"payload": payload, "removeUserIds": []string{s2.String()}}, nil); code != http.StatusOK {
		t.Fatalf("patch removing s2 with the same payload = %d", code)
	}
	var got struct {
		Assignment struct {
			Payload json.RawMessage `json:"payload"`
		} `json:"assignment"`
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/assignments/"+aid, &got); code != http.StatusOK {
		t.Fatalf("get assignment = %d", code)
	}
	var storedPayload struct {
		Picks map[string]any `json:"picks"`
	}
	if err := json.Unmarshal(got.Assignment.Payload, &storedPayload); err != nil {
		t.Fatalf("decode stored payload: %v", err)
	}
	if _, ok := storedPayload.Picks[s2.String()]; !ok {
		t.Fatalf("stored picks = %+v, want s2's pick to remain after her removal", storedPayload.Picks)
	}

	// A later save that still resends the same payload (e.g. a title-only
	// edit from the form) also succeeds: s2's stale pick does not keep
	// blocking every future save.
	if code := assignJSON(t, h, teacher, "PATCH", "/api/v1/lite/teacher/assignments/"+aid,
		map[string]any{"title": "新标题", "payload": payload}, nil); code != http.StatusOK {
		t.Fatalf("title-only patch resending the same payload = %d", code)
	}

	// A brand new pick for a student who is not a recipient is still refused.
	withNewPick := map[string]any{"payload": personalizedPayload(nil, map[string]any{
		s1.String(): map[string]any{"slug": slug},
		s3.String(): map[string]any{"slug": slug},
	})}
	if code, errCode := writeErrorCode(t, h, teacher, "PATCH", "/api/v1/lite/teacher/assignments/"+aid, withNewPick); code != http.StatusBadRequest || errCode != "pick_not_recipient" {
		t.Fatalf("patch adding a new pick for a non-recipient = %d %s", code, errCode)
	}
}

func TestPersonalizedDetailShowsEachArticle(t *testing.T) {
	h, pool, teacher, classID, s1 := liteTeacherFixture(t)
	s2 := createStudent(t, pool, SeedSchoolID, "pd-s2@demo.local")
	s3 := createStudent(t, pool, SeedSchoolID, "pd-s3@demo.local")
	enrollStudent(t, pool, s2, classID)
	enrollStudent(t, pool, s3, classID)
	all := library.All()
	aid := createAssignment(t, h, teacher, classID, readingAssignmentBody("个性化", personalizedPayload(nil, map[string]any{
		s1.String(): map[string]any{"slug": all[1].Slug, "tier": 4},
		s2.String(): map[string]any{"slug": all[2].Slug, "tier": nil},
	}), []string{s1.String(), s2.String(), s3.String()}))
	startAssignment(t, h, signInAs(t, pool, s1), aid)

	type reading struct {
		Slug  string `json:"slug"`
		Title string `json:"title"`
		Tier  *int   `json:"tier"`
		State string `json:"state"`
	}
	var detail struct {
		Recipients []struct {
			UserID  string   `json:"userId"`
			Reading *reading `json:"reading"`
		} `json:"recipients"`
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/assignments/"+aid, &detail); code != http.StatusOK {
		t.Fatalf("detail = %d", code)
	}
	got := map[string]*reading{}
	for _, rc := range detail.Recipients {
		got[rc.UserID] = rc.Reading
	}
	if r := got[s1.String()]; r == nil || r.State != "started" || r.Slug != all[1].Slug || r.Title != all[1].ZhTitle || r.Tier == nil || *r.Tier != 4 {
		t.Fatalf("s1 reading = %+v, want started %s tier 4", r, all[1].Slug)
	}
	if r := got[s2.String()]; r == nil || r.State != "picked" || r.Slug != all[2].Slug || r.Title != all[2].ZhTitle || r.Tier != nil {
		t.Fatalf("s2 reading = %+v, want picked %s with an open tier", r, all[2].Slug)
	}
	if r := got[s3.String()]; r == nil || r.State != "pending" || r.Slug != "" || r.Tier != nil {
		t.Fatalf("s3 reading = %+v, want pending", r)
	}

	// Other homework kinds carry reading: null.
	wid := createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{s1.String()}))
	var raw struct {
		Recipients []map[string]any `json:"recipients"`
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/assignments/"+wid, &raw); code != http.StatusOK {
		t.Fatalf("writing detail = %d", code)
	}
	if len(raw.Recipients) == 0 {
		t.Fatalf("writing detail recipients = %+v, want at least one", raw.Recipients)
	}
	if v, ok := raw.Recipients[0]["reading"]; !ok || v != nil {
		t.Fatalf("writing recipient reading = %v (present %v), want null", v, ok)
	}

	other := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "pd-other@demo.local"))
	if code := getJSON(t, h, other, "/api/v1/lite/teacher/assignments/"+aid, nil); code != http.StatusNotFound {
		t.Fatalf("other teacher detail = %d, want 404", code)
	}
}

// TestPersonalizedDetailPickedTierFallsBackToClassTier: a picked recipient's
// open tier (pick.Tier == nil) resolves the same way the detail shows it as
// the way she'll actually start (personalizedTargetIn) — the homework's
// class-wide tier when one is set, else null (her own level).
func TestPersonalizedDetailPickedTierFallsBackToClassTier(t *testing.T) {
	h, pool, teacher, classID, s1 := liteTeacherFixture(t)
	all := library.All()

	// With a class-wide tier: an open pick shows that tier, not null.
	withClassTier := createAssignment(t, h, teacher, classID, readingAssignmentBody("班级难度", map[string]any{
		"source": "personalized",
		"tier":   3,
		"picks": map[string]any{
			s1.String(): map[string]any{"slug": all[0].Slug, "tier": nil},
		},
	}, []string{s1.String()}))

	type reading struct {
		Slug  string `json:"slug"`
		Title string `json:"title"`
		Tier  *int   `json:"tier"`
		State string `json:"state"`
	}
	var detail struct {
		Recipients []struct {
			UserID  string   `json:"userId"`
			Reading *reading `json:"reading"`
		} `json:"recipients"`
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/assignments/"+withClassTier, &detail); code != http.StatusOK {
		t.Fatalf("detail = %d", code)
	}
	r := detail.Recipients[0].Reading
	if r == nil || r.State != "picked" || r.Tier == nil || *r.Tier != 3 {
		t.Fatalf("picked reading = %+v, want tier 3 (the class-wide tier)", r)
	}

	// Without a class-wide tier: the same open pick shows tier null (her own level).
	s2 := createStudent(t, pool, SeedSchoolID, "pd-s4@demo.local")
	enrollStudent(t, pool, s2, classID)
	noClassTier := createAssignment(t, h, teacher, classID, readingAssignmentBody("无班级难度", personalizedPayload(nil, map[string]any{
		s2.String(): map[string]any{"slug": all[0].Slug, "tier": nil},
	}), []string{s2.String()}))
	var detail2 struct {
		Recipients []struct {
			UserID  string   `json:"userId"`
			Reading *reading `json:"reading"`
		} `json:"recipients"`
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/assignments/"+noClassTier, &detail2); code != http.StatusOK {
		t.Fatalf("detail2 = %d", code)
	}
	r2 := detail2.Recipients[0].Reading
	if r2 == nil || r2.State != "picked" || r2.Tier != nil {
		t.Fatalf("picked reading (no class tier) = %+v, want tier null", r2)
	}
}

// patchErr sends a PATCH and returns the status, error code and message.
func patchErr(t *testing.T, h http.Handler, c *http.Cookie, path string, body any) (int, string, string) {
	t.Helper()
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PATCH", path, &buf), c))
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	return rec.Code, env.Error.Code, env.Error.Message
}

// TestPersonalizedPickLocksPerStudent: once A has started, B's pick can still
// change; A's pick cannot, and nothing else in the settings can.
func TestPersonalizedPickLocksPerStudent(t *testing.T) {
	h, pool, teacher, classID, sa := liteTeacherFixture(t)
	sb := createStudent(t, pool, SeedSchoolID, "pl-sb@demo.local")
	enrollStudent(t, pool, sb, classID)
	all := library.All()
	d := all[0].Disciplines[0]
	payload := func(tier any, ds []string, pa, pb string) map[string]any {
		picks := map[string]any{}
		if pa != "" {
			picks[sa.String()] = map[string]any{"slug": pa}
		}
		if pb != "" {
			picks[sb.String()] = map[string]any{"slug": pb}
		}
		p := personalizedPayload(ds, picks)
		if tier != nil {
			p["tier"] = tier
		}
		return p
	}
	start := payload(3, []string{d}, all[0].Slug, all[0].Slug)

	// Nobody started: any settings change is saved.
	aid := createAssignment(t, h, teacher, classID, readingAssignmentBody("个性化", start, []string{sa.String(), sb.String()}))
	path := "/api/v1/lite/teacher/assignments/" + aid
	if code := assignJSON(t, h, teacher, "PATCH", path, map[string]any{"payload": payload(2, nil, all[0].Slug, all[0].Slug)}, nil); code != http.StatusOK {
		t.Fatalf("pre-start settings change = %d, want 200", code)
	}
	if code := assignJSON(t, h, teacher, "PATCH", path, map[string]any{"payload": start}, nil); code != http.StatusOK {
		t.Fatalf("restore settings = %d, want 200", code)
	}

	startAssignment(t, h, signInAs(t, pool, sa), aid)
	var nameA string
	if err := pool.QueryRow(context.Background(), `SELECT display_name FROM users WHERE id = $1`, sa).Scan(&nameA); err != nil {
		t.Fatal(err)
	}

	// B has not started: her pick changes.
	if code := assignJSON(t, h, teacher, "PATCH", path, map[string]any{"payload": payload(3, []string{d}, all[0].Slug, all[1].Slug)}, nil); code != http.StatusOK {
		t.Fatalf("change B's pick = %d, want 200", code)
	}
	var got struct {
		Assignment struct {
			Payload struct {
				Picks map[string]struct {
					Slug string `json:"slug"`
				} `json:"picks"`
			} `json:"payload"`
		} `json:"assignment"`
	}
	getJSON(t, h, teacher, path, &got)
	if p := got.Assignment.Payload.Picks; p[sb.String()].Slug != all[1].Slug || p[sa.String()].Slug != all[0].Slug {
		t.Fatalf("picks after B's change = %+v", p)
	}
	storedPayload := func() json.RawMessage {
		t.Helper()
		var p json.RawMessage
		if err := pool.QueryRow(context.Background(), `SELECT payload FROM lite_assignment WHERE id = $1`, aid).Scan(&p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	before := storedPayload()

	// A has started: changing or removing her pick names her.
	for name, body := range map[string]map[string]any{
		"change A": payload(3, []string{d}, all[1].Slug, all[1].Slug),
		"remove A": payload(3, []string{d}, "", all[1].Slug),
	} {
		code, errCode, msg := patchErr(t, h, teacher, path, map[string]any{"payload": body})
		if code != http.StatusConflict || errCode != "assignment_started" || !strings.HasPrefix(msg, nameA+"已开始") {
			t.Errorf("%s = %d %s %q, want 409 assignment_started starting %q", name, code, errCode, msg, nameA+"已开始")
		}
	}
	// Everything but the picks stays locked.
	for name, body := range map[string]map[string]any{
		"class tier":  {"payload": payload(4, []string{d}, all[0].Slug, all[1].Slug)},
		"disciplines": {"payload": payload(3, nil, all[0].Slug, all[1].Slug)},
		"source":      {"payload": map[string]any{"source": "library", "slug": all[0].Slug}},
		"kind":        {"kind": "writing", "payload": map[string]any{"prompt": "写雨", "targetWords": 600, "lang": "zh"}},
		"B and tier":  {"payload": payload(4, []string{d}, all[0].Slug, all[2].Slug)},
	} {
		code, errCode, _ := patchErr(t, h, teacher, path, body)
		if code != http.StatusConflict || errCode != "assignment_started" {
			t.Errorf("%s = %d %s, want 409 assignment_started", name, code, errCode)
		}
	}
	// No refused request changed the stored payload.
	if now := storedPayload(); !jsonEqual(now, before) {
		t.Fatalf("stored payload = %s, want %s", now, before)
	}

	// A started student with a blank name is still blocked, and the message
	// does not start with an empty name.
	if _, err := pool.Exec(context.Background(), `UPDATE users SET display_name = '' WHERE id = $1`, sa); err != nil {
		t.Fatal(err)
	}
	code, errCode, msg := patchErr(t, h, teacher, path, map[string]any{"payload": payload(3, []string{d}, all[1].Slug, all[1].Slug)})
	if code != http.StatusConflict || errCode != "assignment_started" || !strings.HasPrefix(msg, "有学生已开始") {
		t.Fatalf("blank-name started student = %d %s %q, want 409 starting 有学生已开始", code, errCode, msg)
	}
}

func jsonEqual(a, b []byte) bool {
	var va, vb any
	if json.Unmarshal(a, &va) != nil || json.Unmarshal(b, &vb) != nil {
		return false
	}
	return reflect.DeepEqual(va, vb)
}
