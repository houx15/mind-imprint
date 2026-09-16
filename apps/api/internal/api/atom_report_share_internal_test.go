package api

// atom_report_share_internal_test.go — newShareToken is unexported, so this
// test lives in package api (see atom_report_share_test.go for the
// HTTP-level guarantees, which run as package api_test like every other
// test file in this directory).

import (
	"bytes"
	"encoding/json"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// TestShareTokenIsUnguessable — R1's first compensating control: 32 hex
// chars (16 random bytes), never repeating across 200 draws. This does not
// prove cryptographic randomness (no test can), but it does pin the SHAPE
// (length) that guessability depends on and would catch a regression to a
// shorter or non-hex encoding.
func TestShareTokenIsUnguessable(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		tok, err := newShareToken()
		if err != nil {
			t.Fatal(err)
		}
		if len(tok) != 32 {
			t.Fatalf("token %q is %d chars, want 32", tok, len(tok))
		}
		if seen[tok] {
			t.Fatal("newShareToken repeated a token")
		}
		seen[tok] = true
	}
}

// 🚨 判的是**字节**，不是结构体。漏出去的是响应体，不是字段：一个 `omitempty`
// 写错、一个 nil 切片被序列化成 `[]`，在结构体上都看不出来。
func TestPublicReportBodyOmitsTheTranscriptKeyWhenSheDidNotAskForIt(t *testing.T) {
	body, err := publicReportBody([]byte(`{"version":1,"kind":"reading"}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(body, []byte("transcript")) {
		t.Fatalf("a report shared without the transcript must not carry the key: %s", body)
	}
}

// 勾了之后：每一条都带说话人。这是公开页上唯一同时印着她的话和印记的话的地方，
// 不标就是把印记的话记在她名下。
func TestPublicReportBodyLabelsEveryLine(t *testing.T) {
	body, err := publicReportBody([]byte(`{"version":1,"kind":"reading"}`), []publicTranscriptLine{
		{Who: "student", Text: "我觉得人均排放更重要。"},
		{Who: "coach", Text: "为什么？"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Report     json.RawMessage `json:"report"`
		Transcript []struct {
			Who  string `json:"who"`
			Text string `json:"text"`
		} `json:"transcript"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v — body=%s", err, body)
	}
	if len(got.Transcript) != 2 {
		t.Fatalf("transcript has %d lines, want 2: %s", len(got.Transcript), body)
	}
	if got.Transcript[0].Who != "student" || got.Transcript[1].Who != "coach" {
		t.Fatalf("every line must name who said it: %+v", got.Transcript)
	}
	if len(got.Report) == 0 {
		t.Error("the report itself must still be there")
	}
}

// system 那种记账消息不是任何人说的话，空白消息也不是。两者都不出现在她公开
// 出去的那份对话里。
func TestPublicTranscriptDropsBookkeepingAndBlanks(t *testing.T) {
	got := publicTranscriptOf([]sqlc.AtomMessage{
		{Seq: 1, Role: "student", Content: "我先说一句。"},
		{Seq: 2, Role: "system", Content: "工具已了结"},
		{Seq: 3, Role: "ai", Content: "   "},
		{Seq: 4, Role: "ai", Content: "接住了。"},
	})
	if len(got) != 2 {
		t.Fatalf("kept %d lines, want 2: %+v", len(got), got)
	}
	if got[0].Who != "student" || got[1].Who != "coach" {
		t.Errorf("roles: %+v", got)
	}
}
