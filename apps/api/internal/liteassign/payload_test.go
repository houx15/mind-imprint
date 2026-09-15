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
