package api_test

import (
	"encoding/json"
	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/gateway"
	"net/http"
	"strings"
	"testing"
)

func TestArtifactLocalEditsPreserveSourceAndCheckOwnership(t *testing.T) {
	for _, mode := range []string{"valid", "missing", "foreign", "full_body"} {
		t.Run(mode, func(t *testing.T) {
			prov := gateway.NewSequenceStubProvider(evidenceScript(`{"reply":"测试"}`))
			h, c, _, pool := liteHandlerWithProvider(t, prov)
			pid := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
			owner, sourceProject := c, pid
			if mode == "foreign" {
				otherID := createStudent(t, pool, SeedSchoolID, "foreign-artifact@demo.local")
				owner = signInAs(t, pool, otherID)
				sourceProject = decodePblProject(t, siteReq(t, h, owner, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
			}
			aid := newArtifactViaAPI(t, h, owner, sourceProject)
			old := "人均排放低于美国。"
			if mode == "missing" {
				old = "不存在的原文"
			}
			payload := map[string]any{"kind": "draft", "baseArtifactId": aid, "edits": []map[string]string{{"old": old, "new": "该比较需要核对口径。"}}, "title": "模型不应更改标题"}
			if mode == "full_body" {
				payload["body"] = "不允许同时重写整稿"
			}
			b, _ := json.Marshal(payload)
			*prov = *gateway.NewSequenceStubProvider(evidenceScript(coachProducing("artifact", string(b))), evidenceScript(`{"supported":true,"issues":[]}`))
			if mode == "missing" {
				*prov = *gateway.NewSequenceStubProvider(evidenceScript(coachProducing("artifact", string(b))), evidenceScript(coachProducing("artifact", string(b))))
			}
			rec := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"只修改这一句"}`)
			if mode == "missing" {
				if rec.Code == http.StatusOK || prov.Calls != 2 || !strings.Contains(rec.Body.String(), "唯一匹配") {
					t.Fatalf("invalid edit was not rejected after bounded repair: %s", rec.Body)
				}
			} else if rec.Code != http.StatusOK {
				t.Fatal(rec.Body)
			}
			rec = siteReq(t, h, c, "GET", "/api/v1/pbl/projects/"+pid+"/artifacts", "")
			var rows []struct {
				ID, Title string
				Payload   struct{ Body, BaseArtifactID string }
				Verdict   *string
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
				t.Fatal(err)
			}
			want := 1
			if mode == "valid" {
				want = 2
			}
			if mode == "foreign" {
				want = 0
			}
			if len(rows) != want {
				t.Fatalf("unexpected artifacts: %s", rec.Body)
			}
			if mode == "valid" {
				if rows[0].Payload.Body != "中国的碳排放总量全球第一。人均排放低于美国。" || rows[1].Payload.Body != "中国的碳排放总量全球第一。该比较需要核对口径。" || rows[1].Title != rows[0].Title || rows[1].Payload.BaseArtifactID != aid || rows[1].Verdict != nil {
					t.Fatalf("local edit changed source or metadata: %s", rec.Body)
				}
				coachInput, _ := json.Marshal(prov.Requests[0])
				if prov.Calls != 2 || !strings.Contains(string(coachInput), aid) || !strings.Contains(string(coachInput), "人均排放低于美国") {
					t.Fatal("model did not receive actual source")
				}
			} else if mode != "missing" {
				thread := siteReq(t, h, c, "GET", "/api/v1/pbl/projects/"+pid+"/thread", "")
				if !strings.Contains(thread.Body.String(), "成果保存失败") {
					t.Fatal(thread.Body)
				}
			}
		})
	}
}

func TestArtifactFullRevisionSnapshotsOwnedSource(t *testing.T) {
	for _, foreign := range []bool{false, true} {
		name := "owned"
		if foreign {
			name = "other-project"
		}
		t.Run(name, func(t *testing.T) {
			prov := gateway.NewSequenceStubProvider(evidenceScript(`{"reply":"测试"}`))
			h, c, _, _ := liteHandlerWithProvider(t, prov)
			pid := newProjectViaAPI(t, h, c)
			source := pid
			if foreign {
				source = newProjectViaAPI(t, h, c)
			}
			aid := newArtifactViaAPI(t, h, c, source)
			raw, _ := json.Marshal(map[string]any{"kind": "draft", "title": "新版提纲", "body": "请先确认有无相关经历。", "replacesArtifactId": aid, "previousBody": "模型伪造的原文", "guessed": []string{}, "admits": []string{"所述经历尚未独立核实；书面原件可以准确引用。"}})
			*prov = *gateway.NewSequenceStubProvider(evidenceScript(coachProducing("artifact", string(raw))), evidenceScript(`{"supported":true,"issues":[]}`))
			rec := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"整体重做提纲"}`)
			if rec.Code != http.StatusOK {
				t.Fatal(rec.Body)
			}
			list := siteReq(t, h, c, "GET", "/api/v1/pbl/projects/"+pid+"/artifacts", "")
			var rows []struct {
				ID      string
				Guessed []string
				Admits  []string
				Payload struct {
					Body               string
					PreviousBody       string
					ReplacesArtifactID string
				}
			}
			if err := json.Unmarshal(list.Body.Bytes(), &rows); err != nil {
				t.Fatal(err)
			}
			if foreign {
				if len(rows) != 0 {
					t.Fatal("cross-project source accepted")
				}
			} else {
				if len(rows) != 2 || rows[0].ID != aid || rows[1].Payload.ReplacesArtifactID != aid || rows[1].Payload.PreviousBody != rows[0].Payload.Body || rows[0].Payload.Body != "中国的碳排放总量全球第一。人均排放低于美国。" || rows[1].Payload.Body != "请先确认有无相关经历。" {
					t.Fatalf("source snapshot mismatch: %s", list.Body)
				}
				if len(rows[1].Guessed) != 0 || len(rows[1].Admits) != 1 || rows[1].Admits[0] != "所述经历尚未独立核实；书面原件可以准确引用。" {
					t.Fatalf("full revision did not replace caveats: %s", list.Body)
				}
			}
		})
	}
}

func TestReviewCompletionBindsFullRewriteWithoutModelSource(t *testing.T) {
	for _, verdict := range []string{"revise", "dropped"} {
		t.Run(verdict, func(t *testing.T) {
			provider := checkedPlanCoach(coachProducing("artifact", `{"kind":"draft","title":"新提纲","body":"你最近是否有相关经历？"}`))
			h, c, _, _ := liteHandlerWithProvider(t, provider)
			pid := newProjectViaAPI(t, h, c)
			base := "/api/v1/pbl/projects/" + pid
			aid := newArtifactViaAPI(t, h, c, pid)
			r := pblPost(t, h, c, base+"/artifacts/"+aid+"/settle", `{"verdict":"`+verdict+`","why":"改成中性入口"}`)
			if r.Code != http.StatusOK {
				t.Fatal(r.Body)
			}
			tool := pblPost(t, h, c, base+"/tools", `{"tool":"review","reason":"审阅提纲"}`)
			tid := artifactID(t, tool.Body.String())
			result, _ := json.Marshal(map[string]any{"status": "done", "result": map[string]string{"artifactId": aid}})
			if r = pblPost(t, h, c, base+"/tools/"+tid+"/resolve", string(result)); r.Code != http.StatusOK {
				t.Fatal(r.Body)
			}
			body, _ := json.Marshal(map[string]string{"completedToolId": tid})
			r = pblPost(t, h, c, base+"/turn", string(body))
			if r.Code != http.StatusOK {
				t.Fatal(r.Body)
			}
			list := pblReq(t, h, c, "GET", base+"/artifacts", "")
			var rows []struct {
				Payload struct {
					ReplacesArtifactID string
					PreviousBody       string
					RevisionRequest    string
				}
			}
			if err := json.Unmarshal(list.Body.Bytes(), &rows); err != nil {
				t.Fatal(err)
			}
			if len(rows) != 2 || rows[1].Payload.ReplacesArtifactID != aid || rows[1].Payload.PreviousBody == "" || rows[1].Payload.RevisionRequest != "改成中性入口" {
				t.Fatalf("missing deterministic source: %s; turn=%s", list.Body, r.Body)
			}
			other := newProjectViaAPI(t, h, c)
			denied := pblPost(t, h, c, "/api/v1/pbl/projects/"+other+"/turn", string(body))
			if denied.Code != http.StatusBadRequest {
				t.Fatalf("foreign completion accepted: %s", denied.Body)
			}
		})
	}
}
