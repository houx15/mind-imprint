package api_test

import (
	"encoding/json"
	"mindimprint/api/internal/gateway"
	"strings"
	"testing"
)

func TestSiteCopyCombinesChineseSavedProcessRecords(t *testing.T) {
	feedback := "温室的玻璃外框现在几乎看不见，花朵也太小了。请把六朵花放大，不增加虚构作品。"
	observation := "温室的穹顶和玻璃分格已经能看清。用键盘 Enter 可以打开作品入口并返回；鼠标与手机操作还需要测试。"
	body := "## 修改意见\n\n" + feedback + "\n\n## 保留这一版的理由\n\n" + observation
	for _, unsupported := range []bool{false, true} {
		t.Run(map[bool]string{false: "saved originals", true: "invented result"}[unsupported], func(t *testing.T) {
			candidate := body
			if unsupported {
				candidate += "\n手机操作已经测试通过。"
			}
			payload, _ := json.Marshal(map[string]any{"sections": []map[string]string{{"key": "process", "body": candidate}}})
			provider := gateway.NewSequenceStubProvider(evidenceScript(coachProducing("site_content", string(payload))), evidenceScript(`{"supported":true,"issues":[]}`))
			h, c, _, pool := liteHandlerWithProvider(t, provider)
			id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
			if r := siteReq(t, h, c, "PUT", "/api/v1/pbl/site/content", `{"sections":[{"key":"process","title":"制作过程","depth":0,"body":""}]}`); r.Code != 200 {
				t.Fatal(r.Body)
			}
			if _, err := pool.Exec(t.Context(), `INSERT INTO pbl_code_version(atom_id,brief_revision,brief,html,feedback) VALUES($1,1,'{}','<html><body></body></html>',$2)`, id, feedback); err != nil {
				t.Fatal(err)
			}
			document, _ := json.Marshal(map[string]any{"stage": "trial", "trial": map[string]string{"versionId": "retained-test-version", "observation": observation}})
			if _, err := pool.Exec(t.Context(), `INSERT INTO pbl_creative_direction(atom_id,document,revision) VALUES($1,$2,1)`, id, document); err != nil {
				t.Fatal(err)
			}
			if r := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+id+"/turn", `{"text":"请使用已保存的修改意见和保留理由作为制作过程，不要补写测试结果。"}`); r.Code != 200 {
				t.Fatal(r.Body)
			}
			result := siteReq(t, h, c, "GET", "/api/v1/pbl/site", "")
			encoded, _ := json.Marshal(body)
			if strings.Contains(result.Body.String(), string(encoded)) == unsupported {
				t.Fatal("saved source assembly mismatch", result.Body)
			}
			if strings.Contains(result.Body.String(), "手机操作已经测试通过") {
				t.Fatal("invented outcome saved", result.Body)
			}
		})
	}
}

func TestSiteCopyReusesCreativeFeedbackButRejectsGeneratedPrompt(t *testing.T) {
	for _, candidate := range []string{"Please enlarge the flowers.", "AI_GENERATED_PROMPT_ONLY"} {
		t.Run(candidate, func(t *testing.T) {
			provider := gateway.NewSequenceStubProvider(evidenceScript(coachProducing("site_content", `{"sections":[{"key":"process","body":"`+candidate+`"}]}`)), evidenceScript(`{"supported":true,"issues":[]}`))
			h, c, _, pool := liteHandlerWithProvider(t, provider)
			id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
			if r := siteReq(t, h, c, "PUT", "/api/v1/pbl/site/content", `{"sections":[{"key":"process","title":"Process","depth":0,"body":""}]}`); r.Code != 200 {
				t.Fatal(r.Body)
			}
			if _, err := pool.Exec(t.Context(), `INSERT INTO pbl_code_version(atom_id,brief_revision,brief,html,feedback) VALUES($1,1,'{}','<html><body></body></html>','Please enlarge the flowers.')`, id); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(t.Context(), `INSERT INTO pbl_creative_direction(atom_id,document,revision) VALUES($1,'{"stage":"hero","hero":{"mode":"code","scene":"garden","prompt":"AI_GENERATED_PROMPT_ONLY"}}',1)`, id); err != nil {
				t.Fatal(err)
			}
			if r := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+id+"/turn", `{"text":"Use my saved feedback in the process section."}`); r.Code != 200 {
				t.Fatal(r.Body)
			}
			result := siteReq(t, h, c, "GET", "/api/v1/pbl/site", "")
			found := strings.Contains(result.Body.String(), candidate)
			if found != (candidate == "Please enlarge the flowers.") {
				t.Fatal("incorrect authorship grounding", result.Body)
			}
		})
	}
}
