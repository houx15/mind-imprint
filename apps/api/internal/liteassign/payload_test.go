package liteassign

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/library"
)

func TestValidatePayload(t *testing.T) {
	ok := []struct{ kind, raw string }{
		{"reading", `{"source":"library","slug":"coffee","tier":3}`},
		{"reading", `{"source":"library","slug":"coffee"}`},
		{"reading", `{"source":"url","url":"https://example.com/a"}`},
		{"reading", `{"source":"text","text":"一段文章"}`},
		{"writing", `{"prompt":"写一篇关于雨的记叙文","targetWords":800,"lang":"zh"}`},
		{"project", `{"drivingQuestion":"怎样让校园少用一次性杯子？","description":""}`},
	}
	for _, c := range ok {
		if _, err := ValidatePayload(c.kind, json.RawMessage(c.raw)); err != nil {
			t.Errorf("%s %s: unexpected %v", c.kind, c.raw, err)
		}
	}
	bad := []struct{ kind, raw, code string }{
		{"reading", `{"source":"library","slug":""}`, "invalid_slug"},
		{"reading", `{"source":"library","slug":"coffee","tier":6}`, "invalid_tier"},
		{"reading", `{"source":"url","url":"ftp://x"}`, "invalid_url"},
		{"reading", `{"source":"text","text":"   "}`, "empty_text"},
		{"reading", `{"source":"pdf"}`, "invalid_source"},
		{"writing", `{"prompt":"","targetWords":800,"lang":"zh"}`, "empty_prompt"},
		{"writing", `{"prompt":"x","targetWords":0,"lang":"zh"}`, "invalid_target_words"},
		{"writing", `{"prompt":"x","targetWords":800,"lang":"fr"}`, "invalid_lang"},
		{"project", `{"drivingQuestion":"  "}`, "empty_driving_question"},
		{"quiz", `{}`, "invalid_kind"},
		{"writing", `not json`, "invalid_payload"},
	}
	for _, c := range bad {
		_, err := ValidatePayload(c.kind, json.RawMessage(c.raw))
		var pe *PayloadError
		if !errors.As(err, &pe) || pe.Code != c.code {
			t.Errorf("%s %s: got %v, want code %s", c.kind, c.raw, err, c.code)
		}
	}
}

// TestValidateInstructions: the cap counts runes, not bytes. 2000 Chinese
// characters are 6000 bytes and must still pass.
func TestValidateInstructions(t *testing.T) {
	got, err := ValidateInstructions("  " + strings.Repeat("雨", 2000) + "  ")
	if err != nil || got != strings.Repeat("雨", 2000) {
		t.Fatalf("2000 runes: got len %d err %v, want trimmed and accepted", len([]rune(got)), err)
	}
	_, err = ValidateInstructions(strings.Repeat("雨", 2001))
	var pe *PayloadError
	if !errors.As(err, &pe) || pe.Code != "instructions_too_long" || pe.Message != "说明不能超过 2000 字" {
		t.Fatalf("2001 runes: got %v, want instructions_too_long", err)
	}
}

func TestValidatePayloadCanonicalises(t *testing.T) {
	out, err := ValidatePayload("writing", json.RawMessage(`{"prompt":"  写雨  ","targetWords":800,"lang":"zh","extra":1}`))
	if err != nil {
		t.Fatal(err)
	}
	var w WritingPayload
	_ = json.Unmarshal(out, &w)
	if w.Prompt != "写雨" {
		t.Fatalf("prompt not trimmed: %q", w.Prompt)
	}
	if string(out) == `{"prompt":"  写雨  ","targetWords":800,"lang":"zh","extra":1}` {
		t.Fatal("unknown field kept")
	}
}

func TestReadingTextFileName(t *testing.T) {
	out, err := ValidatePayload("reading", json.RawMessage(`{"source":"text","text":"正文","fileName":"  雨水花园.pdf  "}`))
	if err != nil {
		t.Fatal(err)
	}
	var p ReadingPayload
	_ = json.Unmarshal(out, &p)
	if p.FileName != "雨水花园.pdf" {
		t.Fatalf("fileName = %q, want trimmed", p.FileName)
	}
	// 200 runes pass, 201 fail: the cap counts runes, not bytes.
	ok200 := `{"source":"text","text":"x","fileName":"` + strings.Repeat("雨", 200) + `"}`
	if _, err := ValidatePayload("reading", json.RawMessage(ok200)); err != nil {
		t.Fatalf("200-rune file name: %v", err)
	}
	bad := `{"source":"text","text":"x","fileName":"` + strings.Repeat("雨", 201) + `"}`
	var pe *PayloadError
	if _, err := ValidatePayload("reading", json.RawMessage(bad)); !errors.As(err, &pe) || pe.Code != "file_name_too_long" {
		t.Fatalf("201-rune file name: got %v", err)
	}
	// A file name on another source is dropped.
	out, _ = ValidatePayload("reading", json.RawMessage(`{"source":"url","url":"https://example.com","fileName":"a.pdf"}`))
	if strings.Contains(string(out), "fileName") {
		t.Fatalf("url payload kept fileName: %s", out)
	}
}

func TestReadingPersonalized(t *testing.T) {
	art := library.All()[0]
	uid := uuid.New()
	raw := `{"source":"personalized","disciplines":[" ` + art.Disciplines[0] + `","` + art.Disciplines[0] + `",""],"tier":3,` +
		`"picks":{"` + strings.ToUpper(uid.String()) + `":{"slug":" ` + art.Slug + ` ","tier":null}},"slug":"x","text":"y"}`
	out, err := ValidatePayload("reading", json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	var p ReadingPayload
	if err := json.Unmarshal(out, &p); err != nil {
		t.Fatal(err)
	}
	if p.Source != "personalized" || p.Slug != "" || p.Text != "" || p.Tier == nil || *p.Tier != 3 {
		t.Fatalf("payload = %+v", p)
	}
	if len(p.Disciplines) != 1 || p.Disciplines[0] != art.Disciplines[0] {
		t.Fatalf("disciplines = %v, want trimmed and de-duplicated", p.Disciplines)
	}
	pick, ok := p.Picks[uid.String()]
	if !ok || pick.Slug != art.Slug || pick.Tier != nil {
		t.Fatalf("picks = %+v, want canonical key %s", p.Picks, uid)
	}
	if !strings.Contains(string(out), `"tier":null`) {
		t.Fatalf("a pick with no tier must serialise tier:null: %s", out)
	}

	// No picks and no filter is valid: every student is recommended at start.
	if _, err := ValidatePayload("reading", json.RawMessage(`{"source":"personalized"}`)); err != nil {
		t.Fatalf("bare personalized: %v", err)
	}

	bad := []struct{ raw, code string }{
		{`{"source":"personalized","disciplines":["no-such-discipline"]}`, "invalid_discipline"},
		{`{"source":"personalized","tier":6}`, "invalid_tier"},
		{`{"source":"personalized","picks":{"not-a-uuid":{"slug":"` + art.Slug + `"}}}`, "invalid_pick_user"},
		{`{"source":"personalized","picks":{"` + uid.String() + `":{"slug":"no-such-article"}}}`, "invalid_pick_slug"},
		{`{"source":"personalized","picks":{"` + uid.String() + `":{"slug":"` + art.Slug + `","tier":0}}}`, "invalid_tier"},
	}
	for _, c := range bad {
		_, err := ValidatePayload("reading", json.RawMessage(c.raw))
		var pe *PayloadError
		if !errors.As(err, &pe) || pe.Code != c.code {
			t.Errorf("%s: got %v, want %s", c.raw, err, c.code)
		}
	}
}

// TestHasLevel checks the pick-tier gate directly: every article in the
// embedded library has 5 levels today (library.validate enforces it), so a
// real ValidatePayload call can never hit the "missing level" branch — this
// pins hasLevel against a fake article with fewer levels instead.
func TestHasLevel(t *testing.T) {
	art := library.Article{Levels: []library.Level{{Tier: 1}, {Tier: 2}, {Tier: 3}}}
	if !hasLevel(art, 2) {
		t.Fatal("tier 2 exists on this article")
	}
	if hasLevel(art, 4) {
		t.Fatal("tier 4 does not exist on this article")
	}
}

func TestPicksOutside(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	slug := library.All()[0].Slug
	payload, err := ValidatePayload("reading", json.RawMessage(
		`{"source":"personalized","picks":{"`+a.String()+`":{"slug":"`+slug+`"},"`+b.String()+`":{"slug":"`+slug+`"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if PicksOutside(payload, []uuid.UUID{a, b, c}) {
		t.Fatal("every pick user is a recipient, want false")
	}
	if !PicksOutside(payload, []uuid.UUID{a, c}) {
		t.Fatal("b has a pick but is not a recipient, want true")
	}
	if PicksOutside(json.RawMessage(`{"source":"text","text":"x"}`), nil) {
		t.Fatal("a text payload has no picks, want false")
	}
}

func TestPicksOutsideChanged(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	slug := library.All()[0].Slug
	other := library.All()[1].Slug
	old, err := ValidatePayload("reading", json.RawMessage(
		`{"source":"personalized","picks":{"`+a.String()+`":{"slug":"`+slug+`"},"`+b.String()+`":{"slug":"`+slug+`"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	// b was removed from the recipient list, but her pick in the resent
	// payload is byte-for-byte the same as old: not re-checked, so this is
	// not refused even though b is no longer a recipient.
	if PicksOutsideChanged(old, old, []uuid.UUID{a}) {
		t.Fatal("b's pick is unchanged from old, want false even though b is not a recipient")
	}
	// b's pick changed (a different slug): now it is checked, and b is not a
	// recipient, so this is refused.
	changed, err := ValidatePayload("reading", json.RawMessage(
		`{"source":"personalized","picks":{"`+a.String()+`":{"slug":"`+slug+`"},"`+b.String()+`":{"slug":"`+other+`"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if !PicksOutsideChanged(changed, old, []uuid.UUID{a}) {
		t.Fatal("b's pick changed and b is not a recipient, want true")
	}
	// A brand new pick for a non-recipient is always checked.
	c := uuid.New()
	withNew, err := ValidatePayload("reading", json.RawMessage(
		`{"source":"personalized","picks":{"`+a.String()+`":{"slug":"`+slug+`"},"`+c.String()+`":{"slug":"`+slug+`"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if !PicksOutsideChanged(withNew, old, []uuid.UUID{a}) {
		t.Fatal("c is a brand new pick and is not a recipient, want true")
	}
	// Every pick still present and unchanged, and covered by recipients: false.
	if PicksOutsideChanged(old, old, []uuid.UUID{a, b}) {
		t.Fatal("every pick user is a recipient, want false")
	}
}

// TestReadingPersonalizedPickKeyCollision: two pick keys that normalise to
// the same user id (differing only by letter case) must be rejected rather
// than letting Go's map iteration order silently pick a survivor.
func TestReadingPersonalizedPickKeyCollision(t *testing.T) {
	art := library.All()[0]
	uid := uuid.New()
	raw := `{"source":"personalized","picks":{"` + uid.String() + `":{"slug":"` + art.Slug + `"},"` +
		strings.ToUpper(uid.String()) + `":{"slug":"` + art.Slug + `"}}}`
	_, err := ValidatePayload("reading", json.RawMessage(raw))
	var pe *PayloadError
	if !errors.As(err, &pe) || pe.Code != "invalid_pick_user" {
		t.Fatalf("colliding pick keys: got %v, want invalid_pick_user", err)
	}
}

func TestPickOnlyChange(t *testing.T) {
	a, b, c := uuid.New().String(), uuid.New().String(), uuid.New().String()
	all := library.All()
	s0, s1 := all[0].Slug, all[1].Slug
	d := all[0].Disciplines[0]
	v := func(raw string) json.RawMessage {
		t.Helper()
		out, err := ValidatePayload("reading", json.RawMessage(raw))
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		return out
	}
	pa := `"` + a + `":{"slug":"` + s0 + `"}`
	pb := `"` + b + `":{"slug":"` + s0 + `","tier":2}`
	base := v(`{"source":"personalized","tier":3,"disciplines":["` + d + `"],"picks":{` + pa + `,` + pb + `}}`)

	cases := []struct {
		name      string
		kind      string
		payload   json.RawMessage
		wantOnly  bool
		wantUsers []string
	}{
		{"unchanged", "reading", base, true, nil},
		{"b slug", "reading", v(`{"source":"personalized","tier":3,"disciplines":["` + d + `"],"picks":{` + pa + `,"` + b + `":{"slug":"` + s1 + `","tier":2}}}`), true, []string{b}},
		{"b tier", "reading", v(`{"source":"personalized","tier":3,"disciplines":["` + d + `"],"picks":{` + pa + `,"` + b + `":{"slug":"` + s0 + `"}}}`), true, []string{b}},
		{"c added", "reading", v(`{"source":"personalized","tier":3,"disciplines":["` + d + `"],"picks":{` + pa + `,` + pb + `,"` + c + `":{"slug":"` + s1 + `"}}}`), true, []string{c}},
		{"a removed", "reading", v(`{"source":"personalized","tier":3,"disciplines":["` + d + `"],"picks":{` + pb + `}}`), true, []string{a}},
		{"class tier", "reading", v(`{"source":"personalized","tier":4,"disciplines":["` + d + `"],"picks":{` + pa + `,` + pb + `}}`), false, nil},
		{"class tier cleared", "reading", v(`{"source":"personalized","disciplines":["` + d + `"],"picks":{` + pa + `,` + pb + `}}`), false, nil},
		{"disciplines", "reading", v(`{"source":"personalized","tier":3,"picks":{` + pa + `,` + pb + `}}`), false, nil},
		{"source", "reading", v(`{"source":"library","slug":"` + s0 + `"}`), false, nil},
		{"kind", "project", json.RawMessage(`{"drivingQuestion":"问","description":""}`), false, nil},
	}
	for _, tc := range cases {
		got, only := PickOnlyChange(tc.kind, "reading", tc.payload, base)
		if only != tc.wantOnly {
			t.Errorf("%s: onlyPicks = %v, want %v", tc.name, only, tc.wantOnly)
			continue
		}
		if strings.Join(got, ",") != strings.Join(tc.wantUsers, ",") {
			t.Errorf("%s: changed = %v, want %v", tc.name, got, tc.wantUsers)
		}
	}
	// A stored payload that is not personalized never allows a pick-only change.
	lib := v(`{"source":"library","slug":"` + s0 + `"}`)
	if _, only := PickOnlyChange("reading", "reading", base, lib); only {
		t.Error("library -> personalized: onlyPicks = true, want false")
	}
}
