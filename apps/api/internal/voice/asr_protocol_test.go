package voice

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestASRFullRequestHeader(t *testing.T) {
	raw := buildFullClientRequest(1, []byte(`{"a":1}`))
	if raw[0] != 0x11 || raw[1] != 0x11 || raw[2] != 0x11 {
		t.Fatalf("header = % x, want 11 11 11 ..", raw[:3])
	}
}

// TestASRAudioOnlyRequestHeader checks the non-last and last framing: the
// message type nibble is CLIENT_AUDIO_ONLY_REQUEST (0b0010), the flag is
// POS_SEQUENCE (0b0001) for a non-last chunk and NEG_WITH_SEQUENCE (0b0011)
// with a negated sequence number for the last chunk.
func TestASRAudioOnlyRequestHeader(t *testing.T) {
	nonLast := buildAudioOnlyRequest(2, []byte("pcm-bytes"), false)
	if nonLast[1] != 0x21 {
		t.Fatalf("non-last header byte1 = %#x, want 0x21 (type=0010,flags=0001)", nonLast[1])
	}
	seq := int32(binary.BigEndian.Uint32(nonLast[4:8]))
	if seq != 2 {
		t.Fatalf("non-last seq = %d, want 2", seq)
	}

	last := buildAudioOnlyRequest(3, nil, true)
	if last[1] != 0x23 {
		t.Fatalf("last header byte1 = %#x, want 0x23 (type=0010,flags=0011)", last[1])
	}
	lastSeq := int32(binary.BigEndian.Uint32(last[4:8]))
	if lastSeq != -3 {
		t.Fatalf("last seq = %d, want -3 (negated)", lastSeq)
	}
}

func TestParseASRServerResponse(t *testing.T) {
	payload := []byte(`{"result":{"text":"你好","is_final":true}}`)

	var gz bytes.Buffer
	gw := gzip.NewWriter(&gz)
	if _, err := gw.Write(payload); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	compressed := gz.Bytes()

	var buf bytes.Buffer
	buf.WriteByte(0x11)                   // version=1, header_size=1
	buf.WriteByte((0b1001 << 4) | 0b0001) // SERVER_FULL_RESPONSE, POS_SEQUENCE
	buf.WriteByte(0x11)                   // JSON serialization, GZIP compression
	buf.WriteByte(0x00)                   // reserved

	seqBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(seqBytes, uint32(int32(7)))
	buf.Write(seqBytes)

	sizeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(sizeBytes, uint32(len(compressed)))
	buf.Write(sizeBytes)

	buf.Write(compressed)

	resp, err := parseASRResponse(buf.Bytes())
	if err != nil {
		t.Fatalf("parseASRResponse: %v", err)
	}
	if resp.Seq != 7 {
		t.Fatalf("Seq = %d, want 7", resp.Seq)
	}
	if resp.IsLast {
		t.Fatalf("IsLast = true, want false")
	}
	result, ok := resp.PayloadMsg["result"].(map[string]any)
	if !ok {
		t.Fatalf("PayloadMsg[\"result\"] not a map: %#v", resp.PayloadMsg["result"])
	}
	if result["text"] != "你好" {
		t.Fatalf("text = %v, want 你好", result["text"])
	}
	if result["is_final"] != true {
		t.Fatalf("is_final = %v, want true", result["is_final"])
	}
}

func TestParseASRServerErrorResponse(t *testing.T) {
	payload := []byte(`{"message":"bad request"}`)

	var gz bytes.Buffer
	gw := gzip.NewWriter(&gz)
	if _, err := gw.Write(payload); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	compressed := gz.Bytes()

	var buf bytes.Buffer
	buf.WriteByte(0x11)                   // version=1, header_size=1
	buf.WriteByte((0b1111 << 4) | 0b0000) // SERVER_ERROR_RESPONSE, no sequence flag
	buf.WriteByte(0x11)                   // JSON serialization, GZIP compression
	buf.WriteByte(0x00)                   // reserved

	codeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(codeBytes, uint32(int32(500)))
	buf.Write(codeBytes)

	sizeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(sizeBytes, uint32(len(compressed)))
	buf.Write(sizeBytes)

	buf.Write(compressed)

	resp, err := parseASRResponse(buf.Bytes())
	if err != nil {
		t.Fatalf("parseASRResponse: %v", err)
	}
	if resp.Code != 500 {
		t.Fatalf("Code = %d, want 500", resp.Code)
	}
	if resp.PayloadMsg["message"] != "bad request" {
		t.Fatalf("message = %v, want 'bad request'", resp.PayloadMsg["message"])
	}
}

func TestParseASRResponseTooShort(t *testing.T) {
	if _, err := parseASRResponse([]byte{0x11, 0x91}); err == nil {
		t.Fatalf("expected error for truncated buffer, got nil")
	}
}

func TestParseASRResponseTruncatedGzip(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteByte(0x11)
	buf.WriteByte((0b1001 << 4) | 0b0000) // SERVER_FULL_RESPONSE, no seq flag
	buf.WriteByte(0x11)
	buf.WriteByte(0x00)

	sizeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(sizeBytes, 4)
	buf.Write(sizeBytes)
	buf.Write([]byte{0x1f, 0x8b, 0x00}) // truncated gzip header (short by declared size)

	if _, err := parseASRResponse(buf.Bytes()); err == nil {
		t.Fatalf("expected error for truncated gzip payload, got nil")
	}
}

func TestBuildFullClientRequestPayloadRoundTrips(t *testing.T) {
	original := map[string]any{"a": float64(1), "b": "x"}
	payloadBytes, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	raw := buildFullClientRequest(5, payloadBytes)

	seq := int32(binary.BigEndian.Uint32(raw[4:8]))
	if seq != 5 {
		t.Fatalf("seq = %d, want 5", seq)
	}
	size := binary.BigEndian.Uint32(raw[8:12])
	compressed := raw[12 : 12+size]

	gr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	var decompressed bytes.Buffer
	if _, err := decompressed.ReadFrom(gr); err != nil {
		t.Fatalf("gzip read: %v", err)
	}

	var roundTripped map[string]any
	if err := json.Unmarshal(decompressed.Bytes(), &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if roundTripped["a"] != float64(1) || roundTripped["b"] != "x" {
		t.Fatalf("roundTripped = %#v, want {a:1 b:x}", roundTripped)
	}
}
