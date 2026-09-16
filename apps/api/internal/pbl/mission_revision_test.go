package pbl

import "testing"

func TestParseMissionRevisionWithoutDuplicateTool(t *testing.T) {
	out, err := parseCoachOutput(`{"reply":"请审核","mission_target":"target","mission":[{"prompt":"记录可见状态","want_kind":"observation"}],"produce":{"kind":"plan","payload":{"summary":"新版"}}}`)
	if err != nil || out.MissionTarget != "target" || len(out.Mission) != 1 || out.Produce == nil {
		t.Fatalf("revision lost: %+v %v", out, err)
	}
	for _, raw := range []string{
		`{"reply":"修订","mission_target":"target","mission":[]}`,
		`{"reply":"修订","mission_target":"target","tool":"observe","tool_reason":"重复","mission":[{"prompt":"记录"}]}`,
	} {
		if _, err := parseCoachOutput(raw); err == nil {
			t.Fatal("ambiguous revision accepted", raw)
		}
	}
}
