package agent

import (
	"encoding/json"
	"testing"
)

func TestFlexStringDecodesStringAndArray(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`"just a string"`, "just a string"},
		{`["a","b","c"]`, "a；b；c"},
		{`[]`, ""},
		{`null`, ""},
		{`""`, ""},
		{`["one"]`, "one"},
		{`[1,2]`, "1；2"}, // array of non-strings is tolerated, not rejected
	}
	for _, c := range cases {
		var f flexString
		if err := json.Unmarshal([]byte(c.in), &f); err != nil {
			t.Fatalf("Unmarshal(%s) errored: %v", c.in, err)
		}
		if f.String() != c.want {
			t.Errorf("Unmarshal(%s) = %q, want %q", c.in, f.String(), c.want)
		}
	}
}

// Regression: the review wire must survive a field arriving as a JSON array —
// deepseek-v4-pro returns e.g. `missing` as ["…","…"] on ~40% of live calls, and
// a plain-string field made the whole-array unmarshal fail → 整稿体检 rejected.
func TestReviewItemWireToleratesArrayField(t *testing.T) {
	raw := `[{"criterion_code":"表D","band":"良","evidence":"写了论点","missing":["缺口径","缺年份"],"fix":"补来源","points":2}]`
	var wires []reviewItemWire
	if err := json.Unmarshal([]byte(raw), &wires); err != nil {
		t.Fatalf("array field must not fail the unmarshal: %v", err)
	}
	if len(wires) != 1 || wires[0].Missing.String() != "缺口径；缺年份" {
		t.Fatalf("got %+v", wires)
	}
}

func TestSpotCheckItemWireToleratesArrayField(t *testing.T) {
	raw := `[{"target_id":"m1","evidence":["e1","e2"],"missing":"缺角色","fix":"补交代"}]`
	var wires []spotCheckItemWire
	if err := json.Unmarshal([]byte(raw), &wires); err != nil {
		t.Fatalf("array field must not fail the unmarshal: %v", err)
	}
	if len(wires) != 1 || wires[0].Evidence.String() != "e1；e2" {
		t.Fatalf("got %+v", wires)
	}
}
