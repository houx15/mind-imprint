package voice

// Config holds the Volcano Engine credentials and resource identifiers
// needed to call the TTS (and, later, ASR) WebSocket APIs. It is
// constructed from server-side environment variables only — these values
// must never be logged, returned in an error, or sent to the client.
type Config struct {
	AppID         string
	AccessKey     string
	TTSVoice      string
	TTSResourceID string
	ASRResourceID string
}
