// This file implements the Volcano Engine ASR (speech recognition)
// WebSocket binary frame protocol. It is a faithful, network-free port of
// the Python reference at backend/app/services/asr.py in the
// learning-lamp-v2 repo (AsrRequestHeader / _build_full_client_request /
// _build_audio_only_request / _parse_response): same bit layout, same field
// ordering, same enum values.
//
// This is a separate, simpler protocol from the TTS one in protocol.go —
// ASR always uses JSON serialization + GZIP compression, a fixed 1-byte
// header (no padding, no session/connect IDs, no event field), and gzips
// its payload. It intentionally does not reuse the TTS Message type.
package voice

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// ASR message-type nibble (header byte 1, high nibble). Matches the Python
// MessageType IntEnum exactly.
const (
	asrClientFullRequest  = 0b0001
	asrClientAudioOnly    = 0b0010
	asrServerFullResponse = 0b1001
	asrServerErrorResp    = 0b1111
)

// ASR message-type-specific-flags nibble (header byte 1, low nibble).
// Matches the Python MessageTypeSpecificFlags IntEnum exactly.
const (
	asrFlagPosSequence = 0b0001
	asrFlagLastNoSeq   = 0b0010
	asrFlagNegWithSeq  = 0b0011
)

// ASR serialization/compression nibbles (header byte 2). This protocol only
// ever uses JSON + GZIP, unlike the TTS protocol.
const (
	asrSerializationJSON = 0b0001
	asrCompressionGZIP   = 0b0001
)

// asrHeaderByte0 is fixed for every ASR frame: protocol version 1, header
// size 1 (in 4-byte units). Python: (ProtocolVersion.V1 << 4) | 1 == 0x11.
const asrHeaderByte0 = (0b0001 << 4) | 1

// asrResponse is a parsed ASR server frame. Consumed by Task 7's WebSocket
// handler / Task 8's ASRStream implementation.
type asrResponse struct {
	Code       int32
	IsLast     bool
	Seq        int32
	PayloadMsg map[string]any
}

// gzipCompress compresses payload with gzip, matching Python's
// gzip.compress(bytes).
func gzipCompress(payload []byte) []byte {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	// gzip.Writer.Write on a bytes.Buffer target never errors; Close only
	// flushes/writes the trailer, which likewise cannot fail here.
	_, _ = gw.Write(payload)
	_ = gw.Close()
	return buf.Bytes()
}

// buildFullClientRequest builds the initial ASR full-client-request frame:
// header (CLIENT_FULL_REQUEST, POS_SEQUENCE) + int32 seq + uint32
// compressed-length + gzip(payload). Mirrors
// AsrRequestHeader/_build_full_client_request in the Python reference.
func buildFullClientRequest(seq int32, payload []byte) []byte {
	compressed := gzipCompress(payload)

	var buf bytes.Buffer
	buf.WriteByte(asrHeaderByte0)
	buf.WriteByte((asrClientFullRequest << 4) | asrFlagPosSequence)
	buf.WriteByte((asrSerializationJSON << 4) | asrCompressionGZIP)
	buf.WriteByte(0x00) // reserved

	var seqBytes [4]byte
	binary.BigEndian.PutUint32(seqBytes[:], uint32(seq))
	buf.Write(seqBytes[:])

	var sizeBytes [4]byte
	binary.BigEndian.PutUint32(sizeBytes[:], uint32(len(compressed)))
	buf.Write(sizeBytes[:])

	buf.Write(compressed)

	return buf.Bytes()
}

// buildAudioOnlyRequest builds an ASR audio-only-request frame carrying one
// segment of PCM audio. When last is true, the frame uses
// NEG_WITH_SEQUENCE and the sequence number is negated — mirroring the
// Python reference's _build_audio_only_request, which flips
// message_type_specific_flags and negates seq for the final chunk.
func buildAudioOnlyRequest(seq int32, segment []byte, last bool) []byte {
	flags := byte(asrFlagPosSequence)
	if last {
		flags = asrFlagNegWithSeq
		seq = -seq
	}

	compressed := gzipCompress(segment)

	var buf bytes.Buffer
	buf.WriteByte(asrHeaderByte0)
	buf.WriteByte((asrClientAudioOnly << 4) | flags)
	buf.WriteByte((asrSerializationJSON << 4) | asrCompressionGZIP)
	buf.WriteByte(0x00) // reserved

	var seqBytes [4]byte
	binary.BigEndian.PutUint32(seqBytes[:], uint32(seq))
	buf.Write(seqBytes[:])

	var sizeBytes [4]byte
	binary.BigEndian.PutUint32(sizeBytes[:], uint32(len(compressed)))
	buf.Write(sizeBytes[:])

	buf.Write(compressed)

	return buf.Bytes()
}

// parseASRResponse decodes a server frame received over the ASR WebSocket.
// It mirrors the Python reference's _parse_response field-for-field, but
// returns an error (never panics, never silently drops data) on any short,
// truncated, or malformed input — where the Python version logs and
// returns a partially-populated AsrResponse.
func parseASRResponse(msg []byte) (asrResponse, error) {
	var resp asrResponse

	if len(msg) < 4 {
		return resp, fmt.Errorf("voice: asr response too short: expected at least 4 header bytes, got %d", len(msg))
	}

	headerSize := int(msg[0] & 0x0F)
	messageType := msg[1] >> 4
	flags := msg[1] & 0x0F
	compression := msg[2] & 0x0F

	headerBytes := headerSize * 4
	if headerBytes < 4 || headerBytes > len(msg) {
		return resp, fmt.Errorf("voice: asr response invalid header_size %d for message of length %d", headerSize, len(msg))
	}
	payload := msg[headerBytes:]

	if flags&0x01 != 0 {
		seq, rest, err := readASRInt32(payload, "sequence")
		if err != nil {
			return resp, err
		}
		resp.Seq = seq
		payload = rest
	}
	if flags&0x02 != 0 {
		resp.IsLast = true
	}
	if flags&0x04 != 0 {
		// Event field: present in the wire format but not part of the
		// asrResponse struct this task defines (Task 7's brief only asks
		// for Code/IsLast/Seq/PayloadMsg). Still must be consumed so the
		// payload offset stays correct.
		if _, rest, err := readASRInt32(payload, "event"); err != nil {
			return resp, err
		} else {
			payload = rest
		}
	}

	switch messageType {
	case asrServerFullResponse:
		size, rest, err := readASRUint32(payload, "payload_size")
		if err != nil {
			return resp, err
		}
		payload = rest
		if uint64(len(payload)) < uint64(size) {
			return resp, fmt.Errorf("voice: asr response payload_size %d exceeds remaining %d bytes", size, len(payload))
		}
	case asrServerErrorResp:
		code, rest, err := readASRInt32(payload, "error code")
		if err != nil {
			return resp, err
		}
		resp.Code = code
		payload = rest

		size, rest2, err := readASRUint32(payload, "payload_size")
		if err != nil {
			return resp, err
		}
		payload = rest2
		if uint64(len(payload)) < uint64(size) {
			return resp, fmt.Errorf("voice: asr response payload_size %d exceeds remaining %d bytes", size, len(payload))
		}
	}

	if len(payload) == 0 {
		return resp, nil
	}

	if compression == asrCompressionGZIP {
		decompressed, err := gzipDecompress(payload)
		if err != nil {
			return resp, fmt.Errorf("voice: asr response gzip decompress: %w", err)
		}
		payload = decompressed
	}

	var payloadMsg map[string]any
	if err := json.Unmarshal(payload, &payloadMsg); err != nil {
		return resp, fmt.Errorf("voice: asr response json unmarshal: %w", err)
	}
	resp.PayloadMsg = payloadMsg

	return resp, nil
}

func gzipDecompress(data []byte) ([]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() { _ = gr.Close() }()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, gr); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func readASRInt32(payload []byte, field string) (int32, []byte, error) {
	if len(payload) < 4 {
		return 0, nil, fmt.Errorf("voice: asr response truncated reading %s: need 4 bytes, have %d", field, len(payload))
	}
	return int32(binary.BigEndian.Uint32(payload[:4])), payload[4:], nil
}

func readASRUint32(payload []byte, field string) (uint32, []byte, error) {
	if len(payload) < 4 {
		return 0, nil, fmt.Errorf("voice: asr response truncated reading %s: need 4 bytes, have %d", field, len(payload))
	}
	return binary.BigEndian.Uint32(payload[:4]), payload[4:], nil
}
