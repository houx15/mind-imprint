// Package voice implements the Volcano Engine TTS WebSocket binary frame
// protocol. It is a faithful, network-free port of the Python reference at
// backend/app/services/volc_tts_protocol.py in the learning-lamp-v2 repo:
// same bit layout, same field ordering, same enum values. This package does
// no I/O; it only marshals/parses in-memory byte slices.
package voice

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// MsgType is the 4-bit message-type nibble carried in header byte 1
// (high nibble). Values match the Python MsgType IntEnum exactly.
type MsgType uint8

const (
	Invalid MsgType = 0
	// FullClientRequestType is the MsgType enum value for a full client
	// request (Python: MsgType.FullClientRequest = 0b1). It is named
	// "...Type" rather than the bare "FullClientRequest" used in the
	// Python enum because the bare name is already used in this package
	// for the FullClientRequest(payload) constructor function below —
	// Go does not allow a function and a constant to share one
	// identifier in the same package. See the task report for details.
	FullClientRequestType MsgType = 0b1
	AudioOnlyClient       MsgType = 0b10
	FullServerResponse    MsgType = 0b1001
	AudioOnlyServer       MsgType = 0b1011
	FrontEndResultServer  MsgType = 0b1100
	Error                 MsgType = 0b1111
)

// MsgTypeFlag is the 4-bit flag nibble carried in header byte 1 (low
// nibble). Values match the Python MsgTypeFlagBits IntEnum exactly.
type MsgTypeFlag uint8

const (
	NoSeq       MsgTypeFlag = 0
	PositiveSeq MsgTypeFlag = 0b1
	LastNoSeq   MsgTypeFlag = 0b10
	NegativeSeq MsgTypeFlag = 0b11
	WithEvent   MsgTypeFlag = 0b100
)

// EventType is the optional 4-byte signed event code present when Flag ==
// WithEvent. Values match the Python EventType IntEnum exactly (copied in
// full, not just the three named in the task brief).
type EventType int32

const (
	EventNone EventType = 0

	StartConnection  EventType = 1
	FinishConnection EventType = 2

	ConnectionStarted  EventType = 50
	ConnectionFailed   EventType = 51
	ConnectionFinished EventType = 52

	StartSession  EventType = 100
	CancelSession EventType = 101
	FinishSession EventType = 102

	SessionStarted  EventType = 150
	SessionCanceled EventType = 151
	SessionFinished EventType = 152
	SessionFailed   EventType = 153
	UsageResponse   EventType = 154

	TaskRequest  EventType = 200
	UpdateConfig EventType = 201

	TTSSentenceStart EventType = 350
	TTSSentenceEnd   EventType = 351
	TTSResponse      EventType = 352
	TTSEnded         EventType = 359
)

// Serialization / compression nibbles for header byte 2. Only JSON/None are
// used by this codebase today; the other Python enum members (Thrift,
// Custom, Gzip) are omitted since nothing in this port produces or expects
// them, but the field is a plain uint8 so any value round-trips fine.
const (
	serializationJSON = 1
	compressionNone   = 0
)

// connectionEvents are the EventType values for which the wire format
// carries a connect_id instead of a session_id. Mirrors the Python
// _read_session_id / _read_connect_id skip-lists exactly.
var sessionIDSkipEvents = map[EventType]bool{
	StartConnection:    true,
	FinishConnection:   true,
	ConnectionStarted:  true,
	ConnectionFailed:   true,
	ConnectionFinished: true,
}

var connectIDReadEvents = map[EventType]bool{
	ConnectionStarted:  true,
	ConnectionFailed:   true,
	ConnectionFinished: true,
}

// sequencedTypes are the MsgTypes for which a PositiveSeq/NegativeSeq flag
// causes a 4-byte signed sequence number to be written/read.
func hasSequenceField(t MsgType) bool {
	switch t {
	case FullClientRequestType, FullServerResponse, FrontEndResultServer, AudioOnlyClient, AudioOnlyServer:
		return true
	default:
		return false
	}
}

// knownType reports whether t is one of the MsgType values this protocol
// understands (the same set the Python reference's _get_readers/_get_writers
// branch on, i.e. every non-Invalid MsgType).
func knownType(t MsgType) bool {
	switch t {
	case FullClientRequestType, FullServerResponse, FrontEndResultServer, AudioOnlyClient, AudioOnlyServer, Error:
		return true
	default:
		return false
	}
}

// Message is a single Volcano Engine TTS WebSocket protocol frame. Zero
// values for Version/HeaderSize are treated as "use the default" (1/1) by
// Marshal, since 0 is not a valid protocol value for either field — this
// lets Message{Type: ..., ...} literals omit them, as the fixed test cases
// do.
type Message struct {
	Version       uint8
	HeaderSize    uint8
	Type          MsgType
	Flag          MsgTypeFlag
	Serialization uint8
	Compression   uint8

	Event     EventType
	SessionID string
	ConnectID string
	Sequence  int32
	ErrorCode uint32

	Payload []byte
}

// FullClientRequest builds a FullClientRequest/NoSeq frame with a JSON
// payload, mirroring the Python reference's full_client_request() helper
// (which always sends MsgType.FullClientRequest with MsgTypeFlagBits.NoSeq
// and JSON serialization).
func FullClientRequest(payload []byte) Message {
	return Message{
		Type:          FullClientRequestType,
		Flag:          NoSeq,
		Serialization: serializationJSON,
		Compression:   compressionNone,
		Payload:       payload,
	}
}

// Marshal encodes the message into its wire representation. It never
// returns an error: Version/HeaderSize default to 1 when zero, and an
// unrecognized Type simply omits the sequence/error-code field (there is
// nothing else to validate that would justify a panic or a silent wire
// corruption).
func (m Message) Marshal() []byte {
	var buf bytes.Buffer

	version := m.Version
	if version == 0 {
		version = 1
	}
	headerSize := m.HeaderSize
	if headerSize == 0 {
		headerSize = 1
	}

	header := []byte{
		(version << 4) | headerSize,
		(byte(m.Type) << 4) | byte(m.Flag),
		(m.Serialization << 4) | m.Compression,
	}
	buf.Write(header)

	if padding := int(headerSize)*4 - len(header); padding > 0 {
		buf.Write(make([]byte, padding))
	}

	// Order mirrors the Python reference's Message._get_writers(): when
	// flag == WithEvent, event+session_id are written first; then, if the
	// type carries a sequence number or is an Error frame, that field is
	// written; payload always comes last.
	if m.Flag == WithEvent {
		writeEvent(&buf, m.Event)
		writeSessionID(&buf, m.SessionID)
	}

	switch {
	case hasSequenceField(m.Type) && (m.Flag == PositiveSeq || m.Flag == NegativeSeq):
		writeSequence(&buf, m.Sequence)
	case m.Type == Error:
		writeErrorCode(&buf, m.ErrorCode)
	}

	writePayload(&buf, m.Payload)

	return buf.Bytes()
}

// ParseMessage decodes a wire frame produced by Marshal. It returns an
// error (never panics) on short, truncated, or malformed input.
func ParseMessage(data []byte) (Message, error) {
	if len(data) < 3 {
		return Message{}, fmt.Errorf("voice: data too short: expected at least 3 bytes, got %d", len(data))
	}

	typeAndFlag := data[1]
	msg := Message{
		Type: MsgType(typeAndFlag >> 4),
		Flag: MsgTypeFlag(typeAndFlag & 0x0F),
	}

	if err := msg.unmarshal(data); err != nil {
		return Message{}, err
	}
	return msg, nil
}

func (m *Message) unmarshal(data []byte) error {
	r := bytes.NewReader(data)

	b0, err := r.ReadByte()
	if err != nil {
		return fmt.Errorf("voice: read version/header_size: %w", err)
	}
	m.Version = b0 >> 4
	m.HeaderSize = b0 & 0x0F

	if _, err := r.ReadByte(); err != nil {
		return fmt.Errorf("voice: read type/flag: %w", err)
	}

	b2, err := r.ReadByte()
	if err != nil {
		return fmt.Errorf("voice: read serialization/compression: %w", err)
	}
	m.Serialization = b2 >> 4
	m.Compression = b2 & 0x0F

	headerBytes := int(m.HeaderSize) * 4
	if headerBytes < 3 {
		return fmt.Errorf("voice: invalid header_size %d", m.HeaderSize)
	}
	if padding := headerBytes - 3; padding > 0 {
		if _, err := r.Seek(int64(padding), io.SeekCurrent); err != nil {
			return fmt.Errorf("voice: skip header padding: %w", err)
		}
	}

	// Order mirrors the Python reference's Message._get_readers(): the
	// sequence/error-code field (if any) is read first, then, if
	// flag == WithEvent, event+session_id+connect_id are read; payload
	// always comes last.
	switch {
	case hasSequenceField(m.Type) && (m.Flag == PositiveSeq || m.Flag == NegativeSeq):
		if err := readSequence(r, &m.Sequence); err != nil {
			return err
		}
	case m.Type == Error:
		if err := readErrorCode(r, &m.ErrorCode); err != nil {
			return err
		}
	case !knownType(m.Type):
		return fmt.Errorf("voice: unsupported message type: %d", m.Type)
	}

	if m.Flag == WithEvent {
		if err := readEvent(r, &m.Event); err != nil {
			return err
		}
		if err := readSessionID(r, m.Event, &m.SessionID); err != nil {
			return err
		}
		if err := readConnectID(r, m.Event, &m.ConnectID); err != nil {
			return err
		}
	}

	if err := readPayload(r, &m.Payload); err != nil {
		return err
	}

	if r.Len() > 0 {
		remaining := make([]byte, r.Len())
		_, _ = r.Read(remaining)
		return fmt.Errorf("voice: unexpected trailing data: % x", remaining)
	}

	return nil
}

// readExactOrEOF reads exactly n bytes. Hitting EOF with zero bytes read
// means the optional trailing field is simply absent (returns ok=false, no
// error) — mirroring the Python reference's `if size_bytes:` / `if
// event_bytes:` truthiness checks on a possibly-empty buffer.Read(n) result.
// Hitting EOF after reading 1..n-1 bytes means the frame was truncated
// mid-field, which is always an error.
func readExactOrEOF(r *bytes.Reader, n int) (out []byte, ok bool, err error) {
	buf := make([]byte, n)
	read, readErr := io.ReadFull(r, buf)
	if read == 0 {
		return nil, false, nil
	}
	if readErr != nil {
		return nil, false, fmt.Errorf("voice: truncated field: read %d of %d bytes: %w", read, n, readErr)
	}
	return buf, true, nil
}

func writeEvent(buf *bytes.Buffer, event EventType) {
	var tmp [4]byte
	binary.BigEndian.PutUint32(tmp[:], uint32(int32(event)))
	buf.Write(tmp[:])
}

func readEvent(r *bytes.Reader, out *EventType) error {
	b, ok, err := readExactOrEOF(r, 4)
	if err != nil {
		return fmt.Errorf("voice: read event: %w", err)
	}
	if !ok {
		return nil
	}
	*out = EventType(int32(binary.BigEndian.Uint32(b)))
	return nil
}

func writeSessionID(buf *bytes.Buffer, sessionID string) {
	if sessionID == "" {
		var tmp [4]byte
		binary.BigEndian.PutUint32(tmp[:], 0)
		buf.Write(tmp[:])
		return
	}
	sid := []byte(sessionID)
	var tmp [4]byte
	binary.BigEndian.PutUint32(tmp[:], uint32(len(sid)))
	buf.Write(tmp[:])
	buf.Write(sid)
}

func readSessionID(r *bytes.Reader, event EventType, out *string) error {
	if sessionIDSkipEvents[event] {
		return nil
	}
	return readLengthPrefixedString(r, "session_id", out)
}

func readConnectID(r *bytes.Reader, event EventType, out *string) error {
	if !connectIDReadEvents[event] {
		return nil
	}
	return readLengthPrefixedString(r, "connect_id", out)
}

func readLengthPrefixedString(r *bytes.Reader, field string, out *string) error {
	sizeBytes, ok, err := readExactOrEOF(r, 4)
	if err != nil {
		return fmt.Errorf("voice: read %s length: %w", field, err)
	}
	if !ok {
		return nil
	}
	size := binary.BigEndian.Uint32(sizeBytes)
	if size == 0 {
		return nil
	}
	data := make([]byte, size)
	n, err := io.ReadFull(r, data)
	if err != nil {
		return fmt.Errorf("voice: read %s: read %d of %d bytes: %w", field, n, size, err)
	}
	*out = string(data)
	return nil
}

func writeSequence(buf *bytes.Buffer, sequence int32) {
	var tmp [4]byte
	binary.BigEndian.PutUint32(tmp[:], uint32(sequence))
	buf.Write(tmp[:])
}

func readSequence(r *bytes.Reader, out *int32) error {
	b, ok, err := readExactOrEOF(r, 4)
	if err != nil {
		return fmt.Errorf("voice: read sequence: %w", err)
	}
	if !ok {
		return nil
	}
	*out = int32(binary.BigEndian.Uint32(b))
	return nil
}

func writeErrorCode(buf *bytes.Buffer, errorCode uint32) {
	var tmp [4]byte
	binary.BigEndian.PutUint32(tmp[:], errorCode)
	buf.Write(tmp[:])
}

func readErrorCode(r *bytes.Reader, out *uint32) error {
	b, ok, err := readExactOrEOF(r, 4)
	if err != nil {
		return fmt.Errorf("voice: read error_code: %w", err)
	}
	if !ok {
		return nil
	}
	*out = binary.BigEndian.Uint32(b)
	return nil
}

func writePayload(buf *bytes.Buffer, payload []byte) {
	var tmp [4]byte
	binary.BigEndian.PutUint32(tmp[:], uint32(len(payload)))
	buf.Write(tmp[:])
	buf.Write(payload)
}

func readPayload(r *bytes.Reader, out *[]byte) error {
	sizeBytes, ok, err := readExactOrEOF(r, 4)
	if err != nil {
		return fmt.Errorf("voice: read payload length: %w", err)
	}
	if !ok {
		return nil
	}
	size := binary.BigEndian.Uint32(sizeBytes)
	if size == 0 {
		return nil
	}
	data := make([]byte, size)
	n, err := io.ReadFull(r, data)
	if err != nil {
		return fmt.Errorf("voice: read payload: read %d of %d bytes: %w", n, size, err)
	}
	*out = data
	return nil
}
