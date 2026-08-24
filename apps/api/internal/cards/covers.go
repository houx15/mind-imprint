package cards

import "fmt"

// Card cover art (v3). Every registered card has a cover in four colorway
// variants, uploaded to the student OSS bucket under a DETERMINISTIC key by
// deploy/upload-card-covers.sh:
//
//	web/cards/v3/<cardId>-<n>.webp     n = 1 white · 2 black · 3 green · 4 blue
//
// v3 supersedes v2 under a NEW prefix rather than overwriting it: covers are
// served with a 24h Cache-Control, so reusing the old keys would leave students
// on stale art for a day. The old v2 objects stay in place as a rollback.
//
// So a cover key is a pure function of (cardId, theme) — no manifest, no uuid
// indirection. The object keys are non-secret; the catalog endpoint resolves
// them to short-lived signed URLs at request time (the bucket is private).

// Themes is the closed set of cover colorways a student may choose. "light" is
// the default (matches users.card_theme default). The keys are historical;
// cyber-warm now points at the blue variant (the art has no warm colorway),
// surfaced to students as 湖蓝 — kept under the old key so no DB/enum migration
// is needed.
var Themes = []string{"light", "cyber-sage", "cyber-slate", "cyber-warm"}

// DefaultTheme is used when a caller supplies no theme or an unknown one.
const DefaultTheme = "light"

// themeVariant maps a theme to its cover-art variant number (see the key shape
// above). light→white, cyber-slate→black, cyber-sage→green, cyber-warm→blue.
var themeVariant = map[string]int{
	"light":       1,
	"cyber-slate": 2,
	"cyber-sage":  3,
	"cyber-warm":  4,
}

// coverless is the set of registered cards that have NO cover art (they render
// a text face instead). Empty since v3 — toulmin was the last holdout and its
// art shipped with that batch. Kept as a seam for any future card whose art
// lands after the card itself.
var coverless = map[string]bool{}

// ValidTheme reports whether theme is one of the known colorways.
func ValidTheme(theme string) bool {
	for _, t := range Themes {
		if t == theme {
			return true
		}
	}
	return false
}

// CoverKey returns the OSS object key for a card's cover in the given theme, and
// whether one exists. An unknown theme falls back to DefaultTheme so a bad query
// param never blanks every cover. Cards in the coverless set (and empty ids)
// return ("", false) so the caller renders a text face.
func CoverKey(cardID, theme string) (string, bool) {
	if cardID == "" || coverless[cardID] {
		return "", false
	}
	n, ok := themeVariant[theme]
	if !ok {
		n = themeVariant[DefaultTheme]
	}
	return fmt.Sprintf("web/cards/v3/%s-%d.webp", cardID, n), true
}
