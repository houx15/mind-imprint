package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestSiteReviewUsesSavedDraftBeforePublication(t *testing.T) {
	h, c, _, _ := liteHandlerWithProvider(t, pblCoachSaying(coachProducing("artifact", `{"kind":"site","title":"主页审核","body":"模型编造的摄影展","url":"https://example.com/fake"}`)))
	id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	rec := siteReq(t, h, c, "PUT", "/api/v1/pbl/site/content", `{"headline":"观察校园树木","role":"摄影社同学","about":["这是虚构测试"],"sections":[{"key":"a","title":"观察","body":"叶片为什么变黄"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Body)
	}
	rec = siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+id+"/turn", `{"text":"请审核已保存的主页"}`)
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Body)
	}
	rec = siteReq(t, h, c, "GET", "/api/v1/pbl/projects/"+id+"/artifacts", "")
	var arts []struct{ Payload struct{ Body, URL string } }
	if err := json.Unmarshal(rec.Body.Bytes(), &arts); err != nil {
		t.Fatal(err)
	}
	if len(arts) != 1 {
		t.Fatalf("draft must be reviewable before publication: %s", rec.Body)
	}
	if !strings.Contains(arts[0].Payload.Body, "叶片为什么变黄") || strings.Contains(arts[0].Payload.Body, "摄影展") || !strings.HasSuffix(arts[0].Payload.URL, "/site") {
		t.Fatalf("review not grounded in saved draft: %+v", arts)
	}
	state := siteReq(t, h, c, "GET", "/api/v1/pbl/site", "")
	var site struct{ Published bool }
	if err := json.Unmarshal(state.Body.Bytes(), &site); err != nil {
		t.Fatal(err)
	}
	if site.Published {
		t.Fatal("review silently published site")
	}
}

func TestSiteReviewSavesNewStudentCopyBeforeSnapshot(t *testing.T) {
	for _, invented := range []bool{false, true} {
		t.Run(map[bool]string{false: "student copy", true: "invented copy"}[invented], func(t *testing.T) {
			body := "两人轮流说观点和理由。这是虚构练习。"
			if invented {
				body = "已经举办了三场成功比赛。"
			}
			raw, _ := json.Marshal(map[string]any{"kind": "site", "title": "新版主页审核", "siteContent": map[string]any{"headline": "练习辩论", "sections": []map[string]string{{"key": "a", "body": body}}}})
			h, c, _, _ := liteHandlerWithProvider(t, pblCoachSaying(coachProducing("artifact", string(raw))))
			id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
			rec := siteReq(t, h, c, "PUT", "/api/v1/pbl/site/content", `{"headline":"旧标题","role":"测试学生","about":["虚构演练"],"sections":[{"key":"a","title":"练习","body":"旧正文"}]}`)
			if rec.Code != http.StatusOK {
				t.Fatal(rec.Body)
			}
			rec = siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+id+"/turn", `{"text":"首屏标题：练习辩论。练习正文：两人轮流说观点和理由。这是虚构练习。请保存并审核。"}`)
			if rec.Code != http.StatusOK {
				t.Fatal(rec.Body)
			}
			if invented {
				var reply struct{ Reply string }
				if err := json.Unmarshal(rec.Body.Bytes(), &reply); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(reply.Reply, "本轮操作未完成") {
					t.Fatalf("false success returned: %s", rec.Body)
				}
				thread := siteReq(t, h, c, "GET", "/api/v1/pbl/projects/"+id+"/thread", "")
				if !strings.Contains(thread.Body.String(), "produce_failure") || !strings.Contains(thread.Body.String(), "originalReply") || !strings.Contains(thread.Body.String(), body) {
					t.Fatalf("failure audit missing: %s", thread.Body)
				}
			}
			rec = siteReq(t, h, c, "GET", "/api/v1/pbl/projects/"+id+"/artifacts", "")
			var arts []struct{ Payload struct{ Body string } }
			if err := json.Unmarshal(rec.Body.Bytes(), &arts); err != nil {
				t.Fatal(err)
			}
			state := siteReq(t, h, c, "GET", "/api/v1/pbl/site", "")
			if invented {
				if len(arts) != 0 || !strings.Contains(state.Body.String(), "旧正文") || strings.Contains(state.Body.String(), body) {
					t.Fatalf("ungrounded change saved: %s %s", rec.Body, state.Body)
				}
			} else {
				if len(arts) != 1 || !strings.Contains(arts[0].Payload.Body, body) || strings.Contains(arts[0].Payload.Body, "旧正文") || !strings.Contains(state.Body.String(), body) {
					t.Fatalf("snapshot missed new copy: %s %s", rec.Body, state.Body)
				}
			}
		})
	}
}

func TestSiteReviewRejectsStaleApprovalAndRefreshesCurrentCopy(t *testing.T) {
	h, c, _, _ := liteHandlerWithProvider(t, pblCoachSaying(coachProducing("artifact", `{"kind":"site","title":"主页审核"}`)))
	id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	base := "/api/v1/pbl/projects/" + id + "/artifacts"
	write := func(body string) {
		t.Helper()
		rec := siteReq(t, h, c, "PUT", "/api/v1/pbl/site/content", `{"headline":"观察校园","role":"学生","about":["虚构测试"],"sections":[{"key":"a","title":"介绍","body":"`+body+`"}]}`)
		if rec.Code != 200 {
			t.Fatal(rec.Body)
		}
	}
	list := func() []struct {
		ID      string
		Stale   bool
		Payload struct{ Body string }
	} {
		t.Helper()
		var rows []struct {
			ID      string
			Stale   bool
			Payload struct{ Body string }
		}
		rec := siteReq(t, h, c, "GET", base, "")
		if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
			t.Fatal(rec.Body)
		}
		return rows
	}
	write("旧正文")
	rec := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+id+"/turn", `{"text":"请审核主页"}`)
	if rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	rows := list()
	if len(rows) != 1 || rows[0].Stale {
		t.Fatalf("new snapshot stale: %+v", rows)
	}
	old := rows[0].ID
	write("新版正文")
	rows = list()
	if !rows[0].Stale {
		t.Fatal("changed page review not stale")
	}
	rec = siteReq(t, h, c, "POST", base+"/"+old+"/settle", `{"verdict":"kept"}`)
	if rec.Code == 200 || !strings.Contains(rec.Body.String(), "stale_site_review") {
		t.Fatalf("stale approval accepted: %s", rec.Body)
	}
	rec = siteReq(t, h, c, "POST", base+"/"+old+"/refresh-site", "")
	if rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	rows = list()
	if len(rows) != 2 || rows[0].Payload.Body == rows[1].Payload.Body {
		t.Fatalf("snapshot overwritten: %+v", rows)
	}
	var fresh string
	for _, row := range rows {
		if !row.Stale {
			fresh = row.ID
			if !strings.Contains(row.Payload.Body, "新版正文") {
				t.Fatal("new copy absent")
			}
		}
	}
	if fresh == "" {
		t.Fatal("no fresh review")
	}
	rec = siteReq(t, h, c, "POST", base+"/"+fresh+"/settle", `{"verdict":"kept"}`)
	if rec.Code != 200 {
		t.Fatal(rec.Body)
	}
}
