package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// flexString decodes a JSON value the live model may emit as EITHER a string or
// an array of strings for the same field. deepseek-v4-pro does both — e.g. a
// review/spot-check item's `missing` comes back as "…" on one call and ["…","…"]
// on the next. With a plain `string` field the whole-array unmarshal fails and
// the entire work-order is rejected ("output not a JSON array"), which made
// 整稿体检 (and, latently, 信源/论证体检) reject ~40% of live calls. flexString
// joins an array so a shape the model chose never sinks the review; any other
// JSON scalar is stringified; null/empty becomes "".
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*f = ""
		return nil
	}
	switch b[0] {
	case '"':
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = flexString(s)
	case '[':
		var arr []any
		if err := json.Unmarshal(b, &arr); err != nil {
			return err
		}
		parts := make([]string, 0, len(arr))
		for _, v := range arr {
			if v == nil {
				continue
			}
			parts = append(parts, strings.TrimSpace(fmt.Sprint(v)))
		}
		*f = flexString(strings.Join(parts, "；"))
	default:
		// number / bool / object: keep the raw token so nothing is lost and the
		// unmarshal never fails on a scalar the model chose.
		*f = flexString(strings.Trim(string(b), `"`))
	}
	return nil
}

func (f flexString) String() string { return string(f) }
