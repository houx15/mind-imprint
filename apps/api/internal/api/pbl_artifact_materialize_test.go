package api_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

func TestEvidenceReviewsInheritedArtifactFieldsBeforeSaving(t *testing.T) {
	provider := gateway.NewSequenceStubProvider(evidenceScript(`{"reply":"测试"}`))
	h, c, _, pool := liteHandlerWithProvider(t, provider)
	pid := newProjectViaAPI(t, h, c)
	aid := newArtifactViaAPI(t, h, c, pid)
	bad := "纸条未经追问，不能原文引用。"
	oldAdmits, _ := json.Marshal([]string{bad})
	if _, err := pool.Exec(context.Background(), `UPDATE pbl_artifact SET admits=$2 WHERE id=$1`, aid, oldAdmits); err != nil {
		t.Fatal(err)
	}
	partial, _ := json.Marshal(map[string]any{
		"kind": "draft", "baseArtifactId": aid,
		"edits": []map[string]string{{"old": "人均排放低于美国。", "new": "比较口径待核实。"}},
	})
	full, _ := json.Marshal(map[string]any{
		"kind": "draft", "replacesArtifactId": aid, "title": "修订稿",
		"body":   "中国的碳排放总量全球第一。比较口径待核实。",
		"admits": []string{"书面原文可准确引用，所述经历尚未核实。"},
	})
	check, _ := json.Marshal(map[string]any{"supported": false, "issues": []map[string]string{{"quote": bad, "reason": "继承的局限混淆引用与真实性，需要完整修订"}}})
	*provider = *gateway.NewSequenceStubProvider(
		evidenceScript(coachProducing("artifact", string(partial))), evidenceScript(string(check)),
		evidenceScript(coachProducing("artifact", string(full))), evidenceScript(`{"supported":true,"issues":[]}`),
	)
	r := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"修正正文并纠正引用局限"}`)
	if r.Code != 200 || provider.Calls != 4 {
		t.Fatalf("repair did not complete: calls=%d response=%s", provider.Calls, r.Body)
	}
	var request struct {
		Candidate struct {
			ArtifactToSave struct {
				Body   string
				Admits []string
			}
		}
	}
	if err := json.Unmarshal([]byte(provider.Requests[1].Messages[1].Content), &request); err != nil {
		t.Fatal(err)
	}
	actual := request.Candidate.ArtifactToSave
	if actual.Body != "中国的碳排放总量全球第一。比较口径待核实。" || len(actual.Admits) != 1 || actual.Admits[0] != bad {
		t.Fatalf("checker did not receive effective document: %+v", actual)
	}
	list := siteReq(t, h, c, "GET", "/api/v1/pbl/projects/"+pid+"/artifacts", "")
	var rows []struct{ Admits []string }
	if err := json.Unmarshal(list.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Admits[0] != bad || len(rows[1].Admits) != 1 || !strings.Contains(rows[1].Admits[0], "可准确引用") {
		t.Fatalf("rejected inherited caveat was saved or source changed: %s", list.Body)
	}
}

func TestInvalidArtifactEditRepairsBeforeEvidenceAndNeverMutatesSource(t *testing.T) {
	for _, repairWorks := range []bool{true, false} {
		t.Run(map[bool]string{true: "repaired", false: "still_invalid"}[repairWorks], func(t *testing.T) {
			provider := gateway.NewSequenceStubProvider(evidenceScript(`{"reply":"测试"}`))
			h, c, _, _ := liteHandlerWithProvider(t, provider)
			pid := newProjectViaAPI(t, h, c)
			aid := newArtifactViaAPI(t, h, c, pid)
			makeEdit := func(old string) string {
				payload, _ := json.Marshal(map[string]any{"kind": "draft", "baseArtifactId": aid, "edits": []map[string]string{{"old": old, "new": "比较口径待核实。"}}})
				return coachProducing("artifact", string(payload))
			}
			repairedOld := "不存在的原文"
			if repairWorks {
				repairedOld = "人均排放低于美国。"
			}
			*provider = *gateway.NewSequenceStubProvider(evidenceScript(makeEdit("不存在的原文")), evidenceScript(makeEdit(repairedOld)), evidenceScript(`{"supported":true,"issues":[]}`))
			result := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"修正比较口径"}`)
			wantCalls, wantRows := 2, 1
			if repairWorks {
				wantCalls, wantRows = 3, 2
			}
			if provider.Calls != wantCalls {
				t.Fatalf("calls=%d response=%s", provider.Calls, result.Body)
			}
			if !strings.Contains(provider.Requests[1].Messages[1].Content, "成果局部修改无法应用") {
				t.Fatal("repair did not receive deterministic failure")
			}
			feedback := provider.Requests[1].Messages[1].Content
			marker := "本次替换必须使用的已保存原文"
			index := strings.LastIndex(feedback, marker)
			if index < 0 || !strings.Contains(feedback[index:], "中国的碳排放总量全球第一。人均排放低于美国。") || strings.Contains(feedback[index:], "不存在的原文") {
				t.Fatal("repair must end with the intact source, not failed edit targets")
			}
			list := siteReq(t, h, c, "GET", "/api/v1/pbl/projects/"+pid+"/artifacts", "")
			var rows []struct{ Payload struct{ Body string } }
			if err := json.Unmarshal(list.Body.Bytes(), &rows); err != nil {
				t.Fatal(err)
			}
			if len(rows) != wantRows || rows[0].Payload.Body != "中国的碳排放总量全球第一。人均排放低于美国。" {
				t.Fatalf("invalid edit persisted or source changed: %s", list.Body)
			}
			if repairWorks && !strings.Contains(rows[1].Payload.Body, "比较口径待核实") {
				t.Fatal("repair not persisted")
			}
		})
	}
}

func TestArtifactRepairDoesNotExposeOtherProjectSource(t *testing.T) {
	provider := gateway.NewSequenceStubProvider(evidenceScript(`{"reply":"测试"}`))
	h, c, _, pool := liteHandlerWithProvider(t, provider)
	other := newProjectViaAPI(t, h, c)
	foreignID := newArtifactViaAPI(t, h, c, other)
	privateBody := "只属于另一个项目的原文，不应进入本轮模型上下文"
	payload, _ := json.Marshal(map[string]string{"body": privateBody})
	if _, err := pool.Exec(context.Background(), `UPDATE pbl_artifact SET payload=$2 WHERE id=$1`, foreignID, payload); err != nil {
		t.Fatal(err)
	}
	pid := newProjectViaAPI(t, h, c)
	newArtifactViaAPI(t, h, c, pid)
	patch, _ := json.Marshal(map[string]any{"kind": "draft", "baseArtifactId": foreignID, "edits": []map[string]string{{"old": "不存在", "new": "修改"}}})
	*provider = *gateway.NewSequenceStubProvider(evidenceScript(coachProducing("artifact", string(patch))))
	r := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"修改当前成果"}`)
	if r.Code == 200 || provider.Calls != 2 {
		t.Fatalf("foreign source accepted: calls=%d response=%s", provider.Calls, r.Body)
	}
	for _, req := range provider.Requests {
		for _, message := range req.Messages {
			if strings.Contains(message.Content, privateBody) {
				t.Fatal("foreign source leaked into repair")
			}
		}
	}
}

func TestArtifactMetadataPatchPreservesBodyAndIsReviewed(t *testing.T) {
	for _, clear := range []bool{false, true} {
		t.Run(map[bool]string{true: "clear", false: "replace"}[clear], func(t *testing.T) {
			provider := gateway.NewSequenceStubProvider(evidenceScript(`{"reply":"测试"}`))
			h, c, _, _ := liteHandlerWithProvider(t, provider)
			pid := newProjectViaAPI(t, h, c)
			aid := newArtifactViaAPI(t, h, c, pid)
			admits := []string{"尚未实际试用"}
			if clear {
				admits = []string{}
			}
			payload, _ := json.Marshal(map[string]any{"kind": "draft", "baseArtifactId": aid, "edits": []any{}, "admits": admits})
			*provider = *gateway.NewSequenceStubProvider(evidenceScript(coachProducing("artifact", string(payload))), evidenceScript(`{"supported":true,"issues":[]}`))
			result := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"只更新局限，正文不变"}`)
			if provider.Calls != 2 {
				t.Fatalf("calls=%d result=%s", provider.Calls, result.Body)
			}
			var request struct {
				Candidate struct {
					ArtifactToSave struct {
						Body    string
						Admits  []string
						Guessed []string
					}
				}
			}
			if err := json.Unmarshal([]byte(provider.Requests[1].Messages[1].Content), &request); err != nil {
				t.Fatal(err)
			}
			final := request.Candidate.ArtifactToSave
			if len(final.Admits) != len(admits) || len(final.Guessed) != 1 || final.Body != "中国的碳排放总量全球第一。人均排放低于美国。" {
				t.Fatalf("wrong projection %+v", final)
			}
			list := siteReq(t, h, c, "GET", "/api/v1/pbl/projects/"+pid+"/artifacts", "")
			var rows []struct {
				Payload struct{ Body string }
				Admits  []string
				Guessed []string
			}
			if err := json.Unmarshal(list.Body.Bytes(), &rows); err != nil {
				t.Fatal(err)
			}
			if len(rows) != 2 || len(rows[1].Admits) != len(admits) || rows[0].Payload.Body != rows[1].Payload.Body || rows[1].Guessed[0] != rows[0].Guessed[0] {
				t.Fatalf("bad saved revision: %s", list.Body)
			}
			if !clear && rows[1].Admits[0] != admits[0] {
				t.Fatal("replacement not saved")
			}
		})
	}
}

func TestSupersededArtifactRemainsReadableButCannotBeSettled(t *testing.T) {
	provider := gateway.NewSequenceStubProvider(evidenceScript(`{"reply":"测试"}`))
	h, c, _, _ := liteHandlerWithProvider(t, provider)
	pid := newProjectViaAPI(t, h, c)
	aid := newArtifactViaAPI(t, h, c, pid)
	payload, _ := json.Marshal(map[string]any{"kind": "draft", "baseArtifactId": aid, "edits": []any{}, "admits": []string{"未试用"}})
	*provider = *gateway.NewSequenceStubProvider(evidenceScript(coachProducing("artifact", string(payload))), evidenceScript(`{"supported":true,"issues":[]}`))
	siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"更新局限"}`)
	list := siteReq(t, h, c, "GET", "/api/v1/pbl/projects/"+pid+"/artifacts", "")
	var rows []struct {
		ID         string
		Superseded bool
		SettledAt  *string
	}
	if err := json.Unmarshal(list.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || !rows[0].Superseded || rows[1].Superseded || rows[0].SettledAt != nil {
		t.Fatalf("history or review state changed: %s", list.Body)
	}
	stale := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/artifacts/"+aid+"/settle", `{"verdict":"kept"}`)
	if stale.Code != 400 || !strings.Contains(stale.Body.String(), "superseded_artifact") {
		t.Fatalf("stale approval accepted: %s", stale.Body)
	}
	latest := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/artifacts/"+rows[1].ID+"/settle", `{"verdict":"kept"}`)
	if latest.Code != 200 {
		t.Fatalf("latest approval failed: %s", latest.Body)
	}
}
