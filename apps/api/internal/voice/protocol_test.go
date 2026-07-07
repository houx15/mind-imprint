package voice

import (
	"bytes"
	"testing"
)

func TestFullClientRequestRoundTrip(t *testing.T) {
	payload := []byte(`{"hello":"world"}`)
	raw := FullClientRequest(payload).Marshal()
	// Header: 0x11, (FullClientRequest<<4)|NoSeq = 0x10, 0x10, 0x00
	if !bytes.Equal(raw[:4], []byte{0x11, 0x10, 0x10, 0x00}) {
		t.Fatalf("header = % x, want 11 10 10 00", raw[:4])
	}
	msg, err := ParseMessage(raw)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != FullClientRequestType || !bytes.Equal(msg.Payload, payload) {
		t.Fatalf("round trip lost data: type=%v payload=%q", msg.Type, msg.Payload)
	}
}

func TestParseAudioOnlyServerFrame(t *testing.T) {
	// AudioOnlyServer (0b1011) + NoSeq, JSON/gzip irrelevant, 3-byte audio payload.
	audio := []byte{0xAA, 0xBB, 0xCC}
	m := Message{Type: AudioOnlyServer, Flag: NoSeq, Payload: audio}
	msg, err := ParseMessage(m.Marshal())
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != AudioOnlyServer || !bytes.Equal(msg.Payload, audio) {
		t.Fatalf("got type=%v payload=% x", msg.Type, msg.Payload)
	}
}

func TestParseServerEventFrame(t *testing.T) {
	m := Message{Type: FullServerResponse, Flag: WithEvent, Event: SessionFinished, Payload: []byte(`{}`)}
	msg, err := ParseMessage(m.Marshal())
	if err != nil {
		t.Fatal(err)
	}
	if msg.Event != SessionFinished {
		t.Fatalf("event = %d, want %d", msg.Event, SessionFinished)
	}
}
