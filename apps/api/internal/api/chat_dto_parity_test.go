package api_test

// chat_dto_parity_test.go — Task 5 (Slice 11): locks the Chat surface's wire
// key sets, mirroring studio/dto_parity_test.go's DTO key-set assertions
// (marshal a literal, assert the exact sorted key set). Must match the T6 web
// client's Zod contracts once those land (packages/contracts).
import (
	"encoding/json"
	"sort"
	"testing"

	. "mindimprint/api/internal/api"
)

func TestChatDTOJSONKeys(t *testing.T) {
	thread := ChatThreadDTO{ID: "t1", Title: "对话", CreatedAt: "2026-07-13T00:00:00Z"}
	raw, err := json.Marshal(thread)
	if err != nil {
		t.Fatal(err)
	}
	assertChatDTOKeys(t, raw, []string{"createdAt", "id", "title"})

	msg := ChatMessageDTO{ID: "m1", Role: "user", Content: "c", Modality: "text", CreatedAt: "2026-07-13T00:00:00Z"}
	raw, err = json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	assertChatDTOKeys(t, raw, []string{"content", "createdAt", "id", "modality", "role"})

	offer := ChatCardOfferDTO{CardInstanceID: "ci1", CardID: "craap", MaterialID: "m1"}
	raw, err = json.Marshal(offer)
	if err != nil {
		t.Fatal(err)
	}
	assertChatDTOKeys(t, raw, []string{"cardId", "cardInstanceId", "materialId"})
}

// assertChatDTOKeys unmarshals raw JSON into a map and asserts its sorted key
// set equals want — a self-contained twin of studio/dto_parity_test.go's
// assertKeys, kept local since this file lives in the black-box api_test
// package and can't reach studio's unexported test helpers.
func assertChatDTOKeys(t *testing.T, raw []byte, want []string) {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal into map failed: %v (raw=%s)", err, raw)
	}
	got := make([]string, 0, len(m))
	for k := range m {
		got = append(got, k)
	}
	sort.Strings(got)
	wantSorted := append([]string(nil), want...)
	sort.Strings(wantSorted)
	if len(got) != len(wantSorted) {
		t.Fatalf("keys = %v, want %v (raw=%s)", got, wantSorted, raw)
	}
	for i := range got {
		if got[i] != wantSorted[i] {
			t.Fatalf("keys = %v, want %v (raw=%s)", got, wantSorted, raw)
		}
	}
}
