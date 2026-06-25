package auth

import "testing"

func TestNewTokenRawDiffersFromHashAndIsDeterministic(t *testing.T) {
	raw, hash, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	if raw == "" || hash == "" || raw == hash {
		t.Fatalf("raw=%q hash=%q", raw, hash)
	}
	if HashToken(raw) != hash {
		t.Fatal("HashToken(raw) must equal NewToken's hash")
	}
	raw2, _, _ := NewToken()
	if raw2 == raw {
		t.Fatal("two tokens collided — not random")
	}
}
