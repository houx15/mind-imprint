package api

// coach_cardref_test.go — the card-turn reload path (pure, no DB). A card turn
// stores {"card":{"cardId","fieldValues"}} in the attachments jsonb;
// cardRefFromAttachments must recover it for a reloaded thread, and must treat a
// plain turn ('[]', empty, malformed, or card-with-no-id) as "no card" so a
// normal message never renders as a chip.

import (
	"encoding/json"
	"testing"
)

func TestCardRefFromAttachments(t *testing.T) {
	cases := []struct {
		name       string
		raw        string
		wantCard   bool
		wantCardID string
	}{
		{"plain turn empty array", `[]`, false, ""},
		{"empty bytes", ``, false, ""},
		{"malformed", `{`, false, ""},
		{"object without card", `{"other":1}`, false, ""},
		{"card with no id", `{"card":{"cardId":"  ","fieldValues":{}}}`, false, ""},
		{"card turn", `{"card":{"cardId":"pee","fieldValues":{"point":"中国碳排放全球第一"}}}`, true, "pee"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cardRefFromAttachments([]byte(tc.raw))
			if tc.wantCard != (got != nil) {
				t.Fatalf("cardRefFromAttachments(%q) card presence = %v, want %v", tc.raw, got != nil, tc.wantCard)
			}
			if got != nil && got.CardID != tc.wantCardID {
				t.Fatalf("cardId = %q, want %q", got.CardID, tc.wantCardID)
			}
		})
	}
}

// The recovered fieldValues must be the student's raw answers verbatim — the
// record the read-only viewer renders (铁律: never rewritten).
func TestCardRefFromAttachments_PreservesFieldValues(t *testing.T) {
	got := cardRefFromAttachments([]byte(`{"card":{"cardId":"pee","fieldValues":{"point":"中国碳排放全球第一","evidence":"IEA 2023"}}}`))
	if got == nil {
		t.Fatal("expected a card ref")
	}
	var fv map[string]string
	if err := json.Unmarshal(got.FieldValues, &fv); err != nil {
		t.Fatalf("fieldValues not decodable: %v", err)
	}
	if fv["point"] != "中国碳排放全球第一" || fv["evidence"] != "IEA 2023" {
		t.Fatalf("fieldValues not preserved verbatim: %+v", fv)
	}
}
