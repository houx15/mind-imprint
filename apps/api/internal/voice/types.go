package voice

// Transcript is one incremental result from an ASR stream: a partial or
// final recognized text chunk. Task 8's ASRStream implementation produces
// these; Task 7's handler consumes them. Defined now (Task 4) so the
// VoiceService interface can declare ASRStream's return type without a
// forward reference.
type Transcript struct {
	Text  string `json:"text"`
	Final bool   `json:"final"`
}
