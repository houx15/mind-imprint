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
	// Named exceptions, for CHINESE entries only — a 语文 method without a
	// curriculum term is rare enough that each one should be argued for by
	// name rather than waved through.
	formalMayBeEmpty := map[string]bool{
		// 「最后提个建议」has no 语文 term of its own.
		"closing_scope": true,
	}
	// The English half is a RULE, not a list: an `en` entry's `name` already
	// IS what an English writer reads ("Naming with an appositive"), and there
	// is no second, more formal English label to reveal on a card. This used
	// to be three ids written out by hand, which meant every new English
	// method failed this test for a reason that was never a defect — and the
	// 2026-09-04 batch (vocabulary / sentence formats / story line) added nine
	// at once. Encoding the reason instead of the instances is what keeps the
	// test about the invariant.
	for _, m := range All() {
		if m.Name == "" {
			t.Errorf("method %q has no student-facing name", m.ID)
		}
		mayBeEmpty := formalMayBeEmpty[m.ID] || m.Lang == "en"
		if m.FormalName == "" && !mayBeEmpty {
			t.Errorf("method %q has an empty formal_name; only lang=en entries and %v are allowed to", m.ID, formalMayBeEmpty)
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

// TestFor_NeverOffersEnglishWordingToAChinesePiece pins the TWO properties the
// language axis exists for — deliberately as properties, not as counts, so that
// re-tagging an entry wrongly fails here instead of quietly passing:
//
//  1. A Chinese piece is never offered an English EXPRESSION. Sentence frames
//     ("While it is true that ___") are the only language-bound thing in the
//     library, and they live in `patterns` — so "no entry carrying patterns
//     reaches lang=zh" is the real guarantee, stronger than naming en_* ids.
//  2. An English piece keeps a full vocabulary at EVERY position. Tagging the
//     structural methods "zh" would have left an English writer with one
//     opening method and no closings — the mirror image of the reported bug,
//     and just as much a bug (2026-08-28 ruling).
func TestFor_NeverOffersEnglishWordingToAChinesePiece(t *testing.T) {
	positions := []string{"opening", "body", "closing"}

	for _, pos := range append(positions, "any") {
		for _, m := range For(pos, "zh") {
			if len(m.Patterns) > 0 {
				t.Errorf("For(%q, \"zh\") offered %q, which carries sentence frames — 中文作文 must never be handed English wording", pos, m.ID)
			}
			if m.Lang == "en" {
				t.Errorf("For(%q, \"zh\") offered English-only method %q", pos, m.ID)
			}
		}
	}
	for _, m := range ForLang("zh") {
		if len(m.Patterns) > 0 || m.Lang == "en" {
			t.Errorf(`ForLang("zh") offered %q, an English-wording entry`, m.ID)
		}
	}

	// The English half: every position must still have something to teach.
	for _, pos := range positions {
		if got := For(pos, "en"); len(got) == 0 {
			t.Errorf("For(%q, \"en\") returned nothing — an English writer has no method to be offered at this position", pos)
		}
	}
	// …and the frames themselves are what an English writer gets that a
	// Chinese one must not.
	var sawFrames bool
	for _, m := range For("body", "en") {
		if len(m.Patterns) > 0 {
			sawFrames = true
		}
	}
	if !sawFrames {
		t.Error(`For("body", "en") offered no sentence frames — English writers lost the entries that are theirs`)
	}

	// The structural methods serve both, so a Chinese piece keeps its own.
	for _, pos := range positions {
		if got := For(pos, "zh"); len(got) == 0 {
			t.Errorf("For(%q, \"zh\") returned nothing", pos)
		}
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

// TestEnglishPieceGetsVocabSentenceAndStoryMethods pins the 2026-09-04 ask,
// verbatim from the product owner:
//
//	> currently english directions for snippets, they write with not enough
//	> guidance, english should have methods about vocab, sentence formats,
//	> and also story line.
//
// Before that batch, every English-only entry was an ARGUMENT frame
// (concession / qualify / evidence). A student writing an English narrative —
// or any student stuck on a sentence rather than on a claim — was offered
// nothing that spoke to what she was actually doing. That is what "not enough
// guidance" meant, and no test could have caught it, because nothing was
// broken: the library simply had a hole shaped like two thirds of English
// writing.
//
// Asserted by id rather than by counting: the point is not "there are more
// methods now", it is that each of the three kinds of help is reachable, at a
// position where it makes sense.
func TestEnglishPieceGetsVocabSentenceAndStoryMethods(t *testing.T) {
	families := map[string][]string{
		"vocabulary":      {"en_word_precision", "en_word_register"},
		"sentence format": {"en_sentence_variety", "en_sentence_opener", "en_sentence_appositive", "en_sentence_parallel"},
		"story line":      {"en_story_scene", "en_story_turn", "en_story_landing"},
	}
	for family, ids := range families {
		for _, id := range ids {
			m, ok := ByID(id)
			if !ok {
				t.Errorf("%s: method %q is gone", family, id)
				continue
			}
			if m.Lang != "en" {
				t.Errorf("%s: %q has lang %q — these carry English wording and must never reach a Chinese piece", family, id, m.Lang)
			}
			if len(m.Examples) == 0 && len(m.Patterns) == 0 {
				t.Errorf("%s: %q teaches nothing", family, id)
			}
		}
	}

	// A story line needs all three of its beats, each where it belongs — an
	// arc with no turn is just events in the order they happened.
	for _, tc := range []struct{ id, pos string }{
		{"en_story_scene", "opening"},
		{"en_story_turn", "body"},
		{"en_story_landing", "closing"},
	} {
		var found bool
		for _, m := range For(tc.pos, "en") {
			if m.ID == tc.id {
				found = true
			}
		}
		if !found {
			t.Errorf("For(%q, \"en\") does not offer %q", tc.pos, tc.id)
		}
	}

	// 🚨 And none of it leaks the other way. TestFor_NeverOffersEnglishWording
	// ToAChinesePiece covers that generally; naming the new families here means
	// a future edit that retags one of them "any" fails with the reason
	// attached, rather than as a count mismatch somewhere else.
	for _, m := range ForLang("zh") {
		for family, ids := range families {
			for _, id := range ids {
				if m.ID == id {
					t.Errorf("%s: %q reached a Chinese piece", family, id)
				}
			}
		}
	}
}
