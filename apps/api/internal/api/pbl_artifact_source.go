package api

import "encoding/json"

// Revisions retain provenance for display, but prior edit targets are not part
// of the current document. Keep them out of the coach's editable source.
func currentArtifactSource(payload, guessed, admits []byte) string {
	var current map[string]json.RawMessage
	if json.Unmarshal(payload, &current) != nil || current == nil {
		return string(payload)
	}
	for _, key := range []string{"edits", "paperEdits", "baseArtifactId", "replacesArtifactId"} {
		delete(current, key)
	}
	if json.Valid(guessed) {
		current["guessed"] = guessed
	}
	if json.Valid(admits) {
		current["admits"] = admits
	}
	encoded, err := json.Marshal(current)
	if err != nil {
		return string(payload)
	}
	return string(encoded)
}
