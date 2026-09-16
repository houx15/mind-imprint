package api_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

// A malformed print layout must be repaired before persistence, even in a
// fresh project without evidence notes. Repair must preserve the single-card
// request instead of forcing it into a six-panel foldout.
func TestInvalidPrintLayoutRepairsBeforeSaving(t *testing.T) {
	invalid := `{"reply":"指引卡已准备","produce":{"kind":"artifact","payload":{"kind":"draft","title":"找书指引卡","printLayout":{"format":"a4-card","panels":[{"title":"找书","body":"先看标签"}]}}}}`
	repaired := `{"reply":"请查看单张指引卡","produce":{"kind":"artifact","payload":{"kind":"draft","title":"找书指引卡","body":"先看标签，再看书脊，最后翻简介。找不到时记录卡住的位置。尚未现场验证。","marks":[],"dimensions":[]}}}`
	for _, repairSucceeds := range []bool{true, false} {
		name := "repaired"
		next := repaired
		if !repairSucceeds {
			name = "rejected"
			next = invalid
		}
		t.Run(name, func(t *testing.T) {
			provider := gateway.NewSequenceStubProvider(evidenceScript(invalid), evidenceScript(next), evidenceScript(`{"supported":true,"issues":[]}`))
			h, c, _, pool := liteHandlerWithProvider(t, provider)
			pid := newProjectViaAPI(t, h, c)
			rec := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"请做单张指引卡，不要折页。"}`)
			wantCode, wantCount, wantCalls := 200, 1, 3
			if !repairSucceeds {
				wantCode, wantCount, wantCalls = 502, 0, 2
			}
			if rec.Code != wantCode {
				t.Fatalf("status %d: %s", rec.Code, rec.Body)
			}
			if len(provider.Requests) != wantCalls {
				t.Fatalf("calls %d", len(provider.Requests))
			}
			raw, _ := json.Marshal(provider.Requests[1])
			if !strings.Contains(string(raw), "打印格式无效") || !strings.Contains(string(raw), "不要为通过格式校验改成折页") {
				t.Fatalf("missing repair context: %s", raw)
			}
			var count int
			if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM pbl_artifact WHERE atom_id=$1`, pid).Scan(&count); err != nil || count != wantCount {
				t.Fatalf("artifacts %d: %v", count, err)
			}
			if repairSucceeds {
				var payload []byte
				if err := pool.QueryRow(context.Background(), `SELECT payload FROM pbl_artifact WHERE atom_id=$1`, pid).Scan(&payload); err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(payload), "printLayout") || !strings.Contains(string(payload), "尚未现场验证") {
					t.Fatalf("wrong saved output: %s", payload)
				}
			}
		})
	}
}
