package vocab

import (
	"bytes"
	"os"
	"testing"
)

// The library is the ONLY place 印记 may get a method name from, so a malformed
// entry is a product defect, not a runtime inconvenience — it must fail at load.
func TestLoad_EveryMethodIsUsable(t *testing.T) {
	all := All()
	if len(all) < 10 {
		t.Fatalf("library has %d methods, want at least 10", len(all))
	}
	seen := map[string]bool{}
	for _, m := range all {
		if m.ID == "" || m.Name == "" || m.Definition == "" {
			t.Errorf("method %+v is missing id, name or definition", m)
		}
		if seen[m.ID] {
			t.Errorf("duplicate method id %q", m.ID)
		}
		seen[m.ID] = true
		switch m.AppliesTo {
		case "opening", "body", "closing", "any":
		default:
			t.Errorf("method %q has applies_to %q, want opening|body|closing|any", m.ID, m.AppliesTo)
		}
		switch m.Lang {
		case "zh", "en", "any":
		default:
			t.Errorf("method %q has lang %q, want zh|en|any", m.ID, m.Lang)
		}
		// Borrowed material by construction: an example with no topic cannot be
		// checked for being about someone else's subject.
		for _, ex := range m.Examples {
			if ex.Topic == "" || ex.Text == "" {
				t.Errorf("method %q has an example missing topic or text", m.ID)
			}
		}
		for _, p := range m.Patterns {
			if p.Frame == "" {
				t.Errorf("method %q has a pattern with no frame", m.ID)
			}
		}
		if len(m.Examples) == 0 && len(m.Patterns) == 0 {
			t.Errorf("method %q teaches nothing: no examples and no patterns", m.ID)
		}
	}
}

// TestEveryMethodHasAStudentFacingName pins the 2026-08-28 ruling: 论证 is
// itself jargon, so `name` is what she reads (plain, an action she already
// performs) and `formal_name` is the curriculum term a card reveals. A missing
// `name` would leave her reading nothing; a missing `formal_name` is legitimate
// ONLY where no curriculum term exists — which is a short, deliberate list, not
// "whatever someone forgot to fill in".
func TestEveryMethodHasAStudentFacingName(t *testing.T) {
	formalMayBeEmpty := map[string]bool{
		// 「最后提个建议」has no 语文 term of its own.
		"closing_scope": true,
		// The English entries' name IS what an English writer reads; there is
		// no second, more formal English label to reveal.
		"en_concession": true,
		"en_qualify":    true,
		"en_evidence":   true,
	}
	for _, m := range All() {
		if m.Name == "" {
			t.Errorf("method %q has no student-facing name", m.ID)
		}
		if m.FormalName == "" && !formalMayBeEmpty[m.ID] {
			t.Errorf("method %q has an empty formal_name; only %v are allowed to", m.ID, formalMayBeEmpty)
		}
		if formalMayBeEmpty[m.ID] && m.FormalName != "" {
			t.Errorf("method %q now has formal_name %q — update the exception list rather than leaving it stale", m.ID, m.FormalName)
		}
	}
	// Label is what a prompt prints: the plain name, with the curriculum term
	// alongside it so 印记 can say one and know the other.
	pee, ok := ByID("point_pee")
	if !ok {
		t.Fatal(`ByID("point_pee") not found`)
	}
	if pee.Name != "举个例子" || pee.FormalName != "举例论证" {
		t.Errorf("point_pee = %q / %q, want 举个例子 / 举例论证", pee.Name, pee.FormalName)
	}
	if got := pee.Label(); got != "举个例子（正式名称：举例论证）" {
		t.Errorf("point_pee.Label() = %q", got)
	}
	// Where the two names coincide, saying it twice teaches nothing.
	direct, _ := ByID("opening_direct")
	if got := direct.Label(); got != "开门见山" {
		t.Errorf("opening_direct.Label() = %q, want the bare name (both names are identical)", got)
	}
	scope, _ := ByID("closing_scope")
	if got := scope.Label(); got != "最后提个建议" {
		t.Errorf("closing_scope.Label() = %q, want the bare name (no formal term exists)", got)
	}
}

func TestByID_AndFor(t *testing.T) {
	if _, ok := ByID("point_concession"); !ok {
		t.Fatal(`ByID("point_concession") not found`)
	}
	if _, ok := ByID("no_such_method"); ok {
		t.Fatal("ByID returned ok for an unknown id")
	}
	openings := For("opening", "zh")
	if len(openings) == 0 {
		t.Fatal(`For("opening", "zh") returned nothing`)
	}
	for _, m := range openings {
		if m.AppliesTo != "opening" && m.AppliesTo != "any" {
			t.Errorf("For(\"opening\") returned %q with applies_to %q", m.ID, m.AppliesTo)
		}
	}
}

// TestFor_NeverCrossesLanguages is the regression test for the bug the product
// owner called serious: a student writing a Chinese essay was being offered
// "While it is true that ___". Position filtering alone could not stop that, so
// the guarantee has to live in the selector — which is what this pins.
func TestFor_NeverCrossesLanguages(t *testing.T) {
	for _, pos := range []string{"opening", "body", "closing", "any"} {
		for _, m := range For(pos, "zh") {
			if m.Lang == "en" {
				t.Errorf("For(%q, \"zh\") offered English method %q — 中文作文 must never be handed an English frame", pos, m.ID)
			}
		}
		for _, m := range For(pos, "en") {
			if m.Lang == "zh" {
				t.Errorf("For(%q, \"en\") offered Chinese-only method %q", pos, m.ID)
			}
		}
	}
	for _, m := range ForLang("zh") {
		if m.Lang == "en" {
			t.Errorf(`ForLang("zh") offered English method %q`, m.ID)
		}
	}
	for _, m := range ForLang("en") {
		if m.Lang == "zh" {
			t.Errorf(`ForLang("en") offered Chinese-only method %q`, m.ID)
		}
	}
	// Concretely: the concession frames are the entry that used to leak.
	for _, m := range For("body", "zh") {
		if m.ID == "en_concession" {
			t.Fatal(`For("body", "zh") still returns en_concession — the reported bug is back`)
		}
	}
	var sawEnConcession bool
	for _, m := range For("body", "en") {
		if m.ID == "en_concession" {
			sawEnConcession = true
		}
	}
	if !sawEnConcession {
		t.Error(`For("body", "en") no longer returns en_concession — English writers lost their frames`)
	}
	// And the Chinese methods are still there for a Chinese piece.
	if len(For("body", "zh")) < 5 {
		t.Errorf(`For("body", "zh") returned only %d methods`, len(For("body", "zh")))
	}
}

// TestEmbeddedCopyMatchesSourceOfTruth guards the fork forced by go:embed's
// inability to escape the package directory: packages/contracts/vocab/methods.json
// is the editable source of truth, apps/api/internal/vocab/methods.json is the
// embedded copy. Nothing enforces they stay identical except this test, so if
// someone edits one and forgets the other, this must fail loudly rather than
// let the two ends of the product quietly disagree about what a method is called.
func TestEmbeddedCopyMatchesSourceOfTruth(t *testing.T) {
	sourceOfTruth, err := os.ReadFile("../../../../packages/contracts/vocab/methods.json")
	if err != nil {
		t.Fatalf("could not read source of truth: %v", err)
	}
	if !bytes.Equal(sourceOfTruth, methodsJSON) {
		t.Fatal("apps/api/internal/vocab/methods.json has drifted from packages/contracts/vocab/methods.json — copy the source of truth over the embedded file and rerun")
	}
}
