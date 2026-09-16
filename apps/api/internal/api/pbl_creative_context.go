package api

import "encoding/json"

// Preserve the saved choices while keeping unfinished input out of all model contexts.
func creativeContext(document []byte) string {
	var fields map[string]json.RawMessage
	if json.Unmarshal(document, &fields) != nil || fields == nil {
		return ""
	}
	delete(fields, "pendingMotif")
	delete(fields, "responseDrafts")
	raw, err := json.Marshal(fields)
	if err != nil {
		return ""
	}
	return string(raw)
}
