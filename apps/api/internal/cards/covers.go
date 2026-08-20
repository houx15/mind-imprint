package cards

import "fmt"

// Card cover art (v2). Every card except toulmin has a cover in four colorway
// variants, uploaded to the student OSS bucket under a DETERMINISTIC key by
// deploy/upload-card-covers.sh:
//
//	web/cards/v2/<cardId>-<n>.webp     n = 1 white · 2 black · 3 green · 4 blue
//
// So a cover key is a pure function of (cardId, theme) — no manifest, no uuid
// indirection. The object keys are non-secret; the catalog endpoint resolves
// them to short-lived signed URLs at request time (the bucket is private).

// Themes is the closed set of cover colorways a student may choose. "light" is
// the default (matches users.card_theme default). The keys are historical;
// cyber-warm now points at the blue variant (the v2 art has no warm colorway),
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

// coverless is the set of registered cards that have NO v2 cover art (they
// render a text face instead). Only toulmin, deliberately (it is never summoned
// in the live writing flow, so no art was produced for it).
var coverless = map[string]bool{"toulmin": true}

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
	return fmt.Sprintf("web/cards/v2/%s-%d.webp", cardID, n), true
}
