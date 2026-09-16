package api_test

import (
	"encoding/json"
	"mindimprint/api/internal/gateway"
	"net/http"
	"strings"
	"testing"
)

func TestPlanReceiptReflectsSaveResult(t *testing.T) {
	for _, tc := range []struct{ name, payload, want string }{
		{"saved", `{"summary":"观察计划","steps":[{"title":"现场观察","decide":"选择观察时段"}]}`, "计划已保存"},
		{"empty", `{"summary":"观察计划","steps":[]}`, "计划保存失败：pbl: plan has no steps"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := gateway.NewSequenceStubProvider(evidenceScript(coachProducing("plan", tc.payload)), evidenceScript(`{"supported":true,"issues":[]}`))
			h, c, _, _ := liteHandlerWithProvider(t, provider)
			id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
			r := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+id+"/turn", `{"text":"请生成观察计划"}`)
			if r.Code != http.StatusOK {
				t.Fatal(r.Body)
			}
			r = siteReq(t, h, c, "GET", "/api/v1/pbl/projects/"+id+"/thread", "")
			if !strings.Contains(r.Body.String(), tc.want) {
				t.Fatalf("missing save receipt: %s", r.Body)
			}
		})
	}
}

func TestSiteContent_ReceiptReflectsPersistedModules(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"student text", "我想观察树叶的变化。", "1 个模块已有正文"},
		{"unspoken text", "我已经举办三次真实摄影展。", "主页更新失败：模块正文无法核对到学生原文"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, c, _, _ := liteHandlerWithProvider(t, pblCoachSaying(coachProducing("site_content", `{"sections":[{"key":"observation","body":"`+tc.body+`"}]}`)))
			id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
			rec := siteReq(t, h, c, "PUT", "/api/v1/pbl/site/content", `{"headline":"功能测试","sections":[{"key":"observation","title":"观察","depth":0,"body":""}]}`)
			if rec.Code != http.StatusOK {
				t.Fatal(rec.Body)
			}
			rec = siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+id+"/turn", `{"text":"我想观察树叶的变化。"}`)
			if rec.Code != http.StatusOK {
				t.Fatal(rec.Body)
			}
			rec = siteReq(t, h, c, "GET", "/api/v1/pbl/projects/"+id+"/thread", "")
			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("missing actual receipt: %s", rec.Body)
			}
			rec = siteReq(t, h, c, "GET", "/api/v1/pbl/site", "")
			if tc.name == "student text" && !strings.Contains(rec.Body.String(), tc.body) {
				t.Fatal("student text not saved")
			}
			if tc.name == "unspoken text" && strings.Contains(rec.Body.String(), tc.body) {
				t.Fatal("unspoken text saved")
			}
		})
	}
}

func TestSiteContentAcceptsReviewAnswersButNotAIQuestions(t *testing.T) {
	for _, source := range []string{"mark", "dimension", "question"} {
		t.Run(source, func(t *testing.T) {
			text := "请先记录叶片边缘，再比较不同光线下的颜色。"
			prov := gateway.NewSequenceStubProvider(evidenceScript(coachProducing("site_content", `{"sections":[{"key":"a","body":"`+text+`"}]}`)), evidenceScript(`{"supported":true,"issues":[]}`))
			h, c, _, _ := liteHandlerWithProvider(t, prov)
			pid := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
			rec := siteReq(t, h, c, "PUT", "/api/v1/pbl/site/content", `{"headline":"测试","sections":[{"key":"a","title":"观察","body":"旧正文"}]}`)
			if rec.Code != http.StatusOK {
				t.Fatal(rec.Body)
			}
			aid := newArtifactViaAPI(t, h, c, pid)
			rec = pblPost(t, h, c, "/api/v1/pbl/projects/"+pid+"/artifacts/"+aid+"/review", `{"marks":[{"quote":"人均排放低于美国。","question":"`+text+`"}],"dimensions":[{"prompt":"清晰","why":"读者需要知道如何开始"}]}`)
			if rec.Code != http.StatusOK {
				t.Fatal(rec.Body)
			}
			var plan reviewPlanOut
			if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
				t.Fatal(err)
			}
			if source != "question" {
				path := "/marks/" + plan.Marks[0].ID
				if source == "dimension" {
					path = "/dimensions/" + plan.Dimensions[0].ID
				}
				rec = pblReq(t, h, c, "PATCH", "/api/v1/pbl/projects/"+pid+path, `{"answer":"`+text+`"}`)
				if rec.Code != http.StatusOK {
					t.Fatal(rec.Body)
				}
			}
			rec = siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"执行审核中的修改"}`)
			if rec.Code != http.StatusOK {
				t.Fatal(rec.Body)
			}
			rec = siteReq(t, h, c, "GET", "/api/v1/pbl/site", "")
			if strings.Contains(rec.Body.String(), text) != (source != "question") {
				t.Fatalf("wrong source accepted/rejected: %s", rec.Body)
			}
			if source == "question" && !strings.Contains(rec.Body.String(), "旧正文") {
				t.Fatal("failed update changed existing page")
			}
		})
	}
}
