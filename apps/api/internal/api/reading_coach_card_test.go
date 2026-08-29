package api_test

// reading_coach_card_test.go — 印记 can hand her a card inside a coach turn.
//
// Black-box, because the thing worth pinning is the JSON that actually leaves
// the process. Two failures live here that reading the Go will not reveal:
//
//  1. The response map ALREADY has a "card" key — the lens card, a row in
//     atom_card with its own lifetime. The chat card must arrive under
//     "coachCard" or the two silently overwrite each other.
//  2. atom_message.payload is []byte, and encoding/json marshals []byte as a
//     BASE64 STRING. So the transcript's payload is asserted to be a JSON
//     OBJECT — a string there is the bug, and it would surface far from here.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A card built out of real sentences from zhArticle's third paragraph (b3)
// and fifth (b5). The question asks for her judgement and has no right
// answer — 铁律②: this is a ladder, not an exam.
const coachTurnWithCard = `{"routineKey":"zh-scan-focus-lens","focusBlocks":["b3"],
  "steps":[{"kind":"read","detail":"先整体读一遍。"}],
  "reply":"你说的这句我接住了。","advance":"","focusBlock":"b3",
  "card":{"type":"choose_span","prompt":"哪一句你读着最不服气？","options":[
    {"blockId":"b3","quote":"城市里的柏油路和水泥墙白天大量吸热"},
    {"blockId":"b5","quote":"把灰色的屋顶改成绿色的"}]}}`

// The same turn with two sentences the model invented. They read exactly like
// the real ones — which is the entire reason the validator exists.
const coachTurnWithFabricatedCard = `{"routineKey":"zh-scan-focus-lens","focusBlocks":["b3"],
  "steps":[{"kind":"read","detail":"先整体读一遍。"}],
  "reply":"你说的这句我接住了。","advance":"","focusBlock":"b3",
  "card":{"type":"choose_span","prompt":"哪一句你读着最不服气？","options":[
    {"blockId":"b3","quote":"城市的夜晚其实比郊区凉快得多"},
    {"blockId":"b5","quote":"这句话文章里根本不存在"}]}}`

func coachTurnRaw(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("coach turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode coach turn: %v — body=%s", err, rec.Body)
	}
	return out
}

// TestReadingCoach_CardReachesHerUnderItsOwnKey — a valid card travels from
// the model's JSON to the response under "coachCard", never colliding with
// the lens card's "card", and lands on the AI message's payload so a refresh
// still shows it.
func TestReadingCoach_CardReachesHerUnderItsOwnKey(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(coachTurnWithCard))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)

	out := coachTurnRaw(t, coachTurn(t, h, cookie, id, ""))

	if _, collided := out["card"]; collided {
		t.Fatalf(`the chat card must not use "card" — that key is the lens card: %v`, out["card"])
	}
	card, isObject := out["coachCard"].(map[string]any)
	if !isObject {
		t.Fatalf("coachCard is %T, want a JSON object: %s", out["coachCard"], rec2s(out))
	}
	if card["type"] != "choose_span" {
		t.Errorf("type = %v", card["type"])
	}
	if card["prompt"] != "哪一句你读着最不服气？" {
		t.Errorf("prompt = %v", card["prompt"])
	}
	opts, ok := card["options"].([]any)
	if !ok || len(opts) != 2 {
		t.Fatalf("options = %v, want 2 surviving options", card["options"])
	}
	first, _ := opts[0].(map[string]any)
	if first["quote"] != "城市里的柏油路和水泥墙白天大量吸热" {
		t.Errorf("option 0 quote = %v — options must be verbatim article text", first["quote"])
	}
	// 铁律②: the card carries no answer key. Nothing on it says which is right.
	for _, k := range []string{"answer", "correct", "answerKey", "score"} {
		if _, present := card[k]; present {
			t.Errorf("the card carries %q — it must have no answer key", k)
		}
	}

	// The transcript keeps it. 🚨 payload must be a JSON OBJECT here: sqlc
	// gives []byte, and encoding/json would hand the client base64 instead.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(
		httptest.NewRequest("GET", "/api/v1/readings/"+id+"/messages", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("messages = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var listed struct {
		Messages []struct {
			Role    string          `json:"role"`
			Payload json.RawMessage `json:"payload"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode messages: %v — body=%s", err, rec.Body)
	}
	var stored map[string]any
	for _, m := range listed.Messages {
		if m.Role != "ai" || len(m.Payload) == 0 {
			continue
		}
		var env map[string]any
		if err := json.Unmarshal(m.Payload, &env); err != nil {
			t.Fatalf("payload is not JSON — base64 []byte? %v: %s", err, m.Payload)
		}
		if c, isObj := env["card"].(map[string]any); isObj {
			stored = c
		}
	}
	if stored == nil {
		t.Fatalf("the card did not survive into the transcript: %s", rec.Body)
	}
	if stored["prompt"] != "哪一句你读着最不服气？" {
		t.Errorf("stored prompt = %v", stored["prompt"])
	}
}

// TestReadingCoach_FabricatedCardIsDroppedAndTheTurnStands — the failure that
// matters. Sentences the model invented look right to her, so the card is
// dropped whole; the coach's words still arrive and the turn is a 200.
func TestReadingCoach_FabricatedCardIsDroppedAndTheTurnStands(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(coachTurnWithFabricatedCard))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)

	out := coachTurnRaw(t, coachTurn(t, h, cookie, id, ""))
	if _, present := out["coachCard"]; present {
		t.Fatalf("a card built on invented sentences reached her: %v", out["coachCard"])
	}
	if reply, _ := out["reply"].(string); strings.TrimSpace(reply) == "" {
		t.Fatalf("a dropped card took the reply down with it: %s", rec2s(out))
	}
}

func rec2s(m map[string]any) string {
	b, _ := json.Marshal(m)
	return string(b)
}
