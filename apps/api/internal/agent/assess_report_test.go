package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/rubric"
)

// reportProvider stubs a flagship reply (mirrors assessProvider in assess_test.go).
func reportProvider(reply string) *gateway.StubProvider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: reply},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 80, OutputTokens: 40}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

// goodReportWire is a full-coverage wire fixture: all 6 depth dims, all 6
// autonomy signals, 3 stats + all 6 lenses, one interaction row, narrative,
// and one next step. No officialProjection/workAndProcess (chat/course shape).
const goodReportWire = `{
  "depthAxis":[
    {"code":"D1","level":"L3","evidence":"把绝对命题改成有限定判断","promptEvidence":"R4"},
    {"code":"D2","level":"L3","evidence":"溯到一手来源","promptEvidence":""},
    {"code":"D3","level":"L3","evidence":"识别缺失 warrant","promptEvidence":""},
    {"code":"D4","level":"L3","evidence":"邀请反方","promptEvidence":""},
    {"code":"D5","level":"L3","evidence":"说明采纳与拒绝理由","promptEvidence":""},
    {"code":"D6","level":"L3","evidence":"原假设如何随证据改变","promptEvidence":""}
  ],
  "autonomyAxis":[
    {"code":"A1","level":3,"opportunity":"given_taken","evidence":"改题收窄变量","promptEvidence":""},
    {"code":"A2","level":3,"opportunity":"given_taken","evidence":"自发补查","promptEvidence":""},
    {"code":"A3","level":3,"opportunity":"given_taken","evidence":"不要代写","promptEvidence":""},
    {"code":"A4","level":3,"opportunity":"given_taken","evidence":"邀请质疑","promptEvidence":""},
    {"code":"A5","level":3,"opportunity":"given_taken","evidence":"自评档位","promptEvidence":""},
    {"code":"A6","level":3,"opportunity":"given_taken","evidence":"收窄结论","promptEvidence":""}
  ],
  "promptLens":{
    "stats":[{"label":"对话轮次","value":"10"},{"label":"边界句","value":"3"},{"label":"对手邀请","value":"0"}],
    "lenses":[
      {"code":"L_decisions","level":3,"evidence":"多数提示词含任务+材料+边界"},
      {"code":"L_maturity","level":3,"evidence":"多要过程"},
      {"code":"L_boundary","level":3,"evidence":"3 条边界句"},
      {"code":"L_adversary","level":0,"evidence":"0 次"},
      {"code":"L_directive","level":3,"evidence":"多轮主动改路线"},
      {"code":"L_acceptance","level":3,"evidence":"给出可回扣 RQ 的验收标准"}
    ]
  },
  "interactionEvidence":[{"round":4,"student":"我想把 thesis 改成…","aiSummary":"帮你把绝对命题改成有限定判断","signal":"D1→L3"}],
  "narrative":"深度侧 L3 结构稳定复现，自主侧多数事件为学生自发。",
  "guidance":{"nextSteps":[{"title":"下一步强化 D2","task":"跑一张 SIFT 记录，追一手出处"}]}
}`

// goodReportWireWithProject is goodReportWire plus a full officialProjection +
// workAndProcess superset (project-surface shape).
const goodReportWireWithProject = `{
  "depthAxis":[
    {"code":"D1","level":"L3","evidence":"把绝对命题改成有限定判断","promptEvidence":"R4"},
    {"code":"D2","level":"L3","evidence":"溯到一手来源","promptEvidence":""},
    {"code":"D3","level":"L3","evidence":"识别缺失 warrant","promptEvidence":""},
    {"code":"D4","level":"L3","evidence":"邀请反方","promptEvidence":""},
    {"code":"D5","level":"L3","evidence":"说明采纳与拒绝理由","promptEvidence":""},
    {"code":"D6","level":"L3","evidence":"原假设如何随证据改变","promptEvidence":""}
  ],
  "autonomyAxis":[
    {"code":"A1","level":3,"opportunity":"given_taken","evidence":"改题收窄变量","promptEvidence":""},
    {"code":"A2","level":3,"opportunity":"given_taken","evidence":"自发补查","promptEvidence":""},
    {"code":"A3","level":3,"opportunity":"given_taken","evidence":"不要代写","promptEvidence":""},
    {"code":"A4","level":3,"opportunity":"given_taken","evidence":"邀请质疑","promptEvidence":""},
    {"code":"A5","level":3,"opportunity":"given_taken","evidence":"自评档位","promptEvidence":""},
    {"code":"A6","level":3,"opportunity":"given_taken","evidence":"收窄结论","promptEvidence":""}
  ],
  "promptLens":{
    "stats":[{"label":"对话轮次","value":"10"},{"label":"边界句","value":"3"},{"label":"对手邀请","value":"0"}],
    "lenses":[
      {"code":"L_decisions","level":3,"evidence":"多数提示词含任务+材料+边界"},
      {"code":"L_maturity","level":3,"evidence":"多要过程"},
      {"code":"L_boundary","level":3,"evidence":"3 条边界句"},
      {"code":"L_adversary","level":0,"evidence":"0 次"},
      {"code":"L_directive","level":3,"evidence":"多轮主动改路线"},
      {"code":"L_acceptance","level":3,"evidence":"给出可回扣 RQ 的验收标准"}
    ]
  },
  "interactionEvidence":[{"round":4,"student":"我想把 thesis 改成…","aiSummary":"帮你把绝对命题改成有限定判断","signal":"D1→L3"}],
  "narrative":"深度侧 L3 结构稳定复现，自主侧多数事件为学生自发。",
  "guidance":{"nextSteps":[{"title":"下一步强化 D2","task":"跑一张 SIFT 记录，追一手出处"}]},
  "officialProjection":{
    "standard":{"id":"ap-research","name":"AP Research"},
    "components":[
      {"name":"Academic Paper","judgement":"Paper 3","reason":"topic focus 贯穿方法与论证，但证据解释仍不充分"},
      {"name":"POD","judgement":"18/24","reason":"Argument 与 Reflect 较强"},
      {"name":"训练用折算","judgement":"72/100","reason":"仅作作品就绪度参考"},
      {"name":"诚信/真实性","judgement":"高","reason":"PREP 记录完整"}
    ],
    "alignment":[{"item":"Through-course inquiry","standard":"围绕自选 RQ 设计并反思一个长期 inquiry","performance":"已完成收窄与初稿","impact":"支持 Paper 3 档位"}],
    "readiness":{"score":72,"note":"仅作作品就绪度参考，不与 D/A 双轴合成"}
  },
  "workAndProcess":{
    "workSamples":[{"title":"Thesis 修订","text":"从「中国让地球更可持续」改为限定判断版本"}],
    "processMaterials":[{"name":"SIFT 记录","status":"完成","diagnosis":"溯到 NASA/Nature 一手来源"}]
  }
}`

func assessChat(t *testing.T, reply string, in AssessmentInput) Report {
	t.Helper()
	rep, _, err := AssessReport(context.Background(), reportProvider(reply),
		gateway.Resolved{Tier: "flagship"}, rubric.Model(), in)
	if err != nil {
		t.Fatalf("AssessReport: %v", err)
	}
	return rep
}

func TestAssessReportDepthSixDimensionCoverage(t *testing.T) {
	rep := assessChat(t, goodReportWire, AssessmentInput{})
	if len(rep.DepthAxis) != 6 {
		t.Fatalf("depth dims = %d, want 6", len(rep.DepthAxis))
	}
	for i, dim := range rubric.DepthDims() {
		if rep.DepthAxis[i].Code != dim.ID {
			t.Fatalf("depth[%d].Code = %q, want %q (rubric order)", i, rep.DepthAxis[i].Code, dim.ID)
		}
		if rep.DepthAxis[i].Name != dim.Name {
			t.Fatalf("depth[%d].Name = %q, want %q from rubric", i, rep.DepthAxis[i].Name, dim.Name)
		}
	}
}

func TestAssessReportDepthMissingDimensionDefaultsNA(t *testing.T) {
	reply := `{"depthAxis":[{"code":"D1","level":"L3","evidence":"x","promptEvidence":""}],
		"autonomyAxis":[],"promptLens":{},"narrative":"n"}`
	rep := assessChat(t, reply, AssessmentInput{})
	if len(rep.DepthAxis) != 6 {
		t.Fatalf("depth dims = %d, want 6 (full coverage even when model supplies 1)", len(rep.DepthAxis))
	}
	for _, d := range rep.DepthAxis {
		if d.Code == "D1" {
			if d.Level != "L3" {
				t.Fatalf("D1 level = %q, want L3 passthrough", d.Level)
			}
			continue
		}
		if d.Level != "NA" {
			t.Fatalf("missing dim %s level = %q, want NA", d.Code, d.Level)
		}
		if d.Evidence != "" || d.PromptEvidence != "" {
			t.Fatalf("missing dim %s should have empty evidence, got %+v", d.Code, d)
		}
	}
}

func TestAssessReportDepthInvalidLevelNormalizesToNA(t *testing.T) {
	reply := `{"depthAxis":[{"code":"D1","level":"L9","evidence":"x","promptEvidence":""}],
		"autonomyAxis":[],"promptLens":{},"narrative":"n"}`
	rep := assessChat(t, reply, AssessmentInput{})
	for _, d := range rep.DepthAxis {
		if d.Code == "D1" && d.Level != "NA" {
			t.Fatalf("out-of-set level %q did not normalize to NA, got %q", "L9", d.Level)
		}
	}
}

func TestAssessReportAutonomySixSignalCoverage(t *testing.T) {
	rep := assessChat(t, goodReportWire, AssessmentInput{})
	if len(rep.AutonomyAxis) != 6 {
		t.Fatalf("autonomy signals = %d, want 6", len(rep.AutonomyAxis))
	}
	for i, sig := range rubric.AutonomySignals() {
		if rep.AutonomyAxis[i].Code != sig.ID || rep.AutonomyAxis[i].Name != sig.Name {
			t.Fatalf("autonomy[%d] = %+v, want code/name from rubric %s/%s", i, rep.AutonomyAxis[i], sig.ID, sig.Name)
		}
	}
}

func TestAssessReportAutonomyMissingSignalDefaultsLevel0NotSupplied(t *testing.T) {
	reply := `{"depthAxis":[],"autonomyAxis":[{"code":"A1","level":3,"opportunity":"given_taken","evidence":"x"}],
		"promptLens":{},"narrative":"n"}`
	rep := assessChat(t, reply, AssessmentInput{})
	for _, a := range rep.AutonomyAxis {
		if a.Code == "A1" {
			continue
		}
		if a.Level != 0 {
			t.Fatalf("missing signal %s level = %d, want 0", a.Code, a.Level)
		}
		if a.Opportunity != "not_supplied" {
			t.Fatalf("missing signal %s opportunity = %q, want not_supplied", a.Code, a.Opportunity)
		}
	}
}

func TestAssessReportAutonomyLevelClamps(t *testing.T) {
	reply := `{"depthAxis":[],"autonomyAxis":[{"code":"A1","level":9,"opportunity":"given_taken","evidence":"x"},
		{"code":"A2","level":-3,"opportunity":"given_taken","evidence":"x"}],
		"promptLens":{},"narrative":"n"}`
	rep := assessChat(t, reply, AssessmentInput{})
	for _, a := range rep.AutonomyAxis {
		if a.Level < 0 || a.Level > 5 {
			t.Fatalf("autonomy %s level %d out of 0..5", a.Code, a.Level)
		}
		if a.Code == "A1" && a.Level != 5 {
			t.Fatalf("A1 level = %d, want clamp to 5", a.Level)
		}
		if a.Code == "A2" && a.Level != 0 {
			t.Fatalf("A2 level = %d, want clamp to 0", a.Level)
		}
	}
}

func TestAssessReportAutonomyInvalidOpportunityDefaultsGivenTaken(t *testing.T) {
	reply := `{"depthAxis":[],"autonomyAxis":[{"code":"A1","level":2,"opportunity":"bogus","evidence":"x"}],
		"promptLens":{},"narrative":"n"}`
	rep := assessChat(t, reply, AssessmentInput{})
	for _, a := range rep.AutonomyAxis {
		if a.Code == "A1" && a.Opportunity != "given_taken" {
			t.Fatalf("invalid opportunity did not default to given_taken, got %q", a.Opportunity)
		}
	}
}

func TestAssessReportLensesSixCoverageAndClamp(t *testing.T) {
	reply := `{"depthAxis":[],"autonomyAxis":[],
		"promptLens":{"lenses":[{"code":"L_decisions","level":9,"evidence":"x"}]},"narrative":"n"}`
	rep := assessChat(t, reply, AssessmentInput{})
	if len(rep.PromptLens.Lenses) != 6 {
		t.Fatalf("lenses = %d, want 6", len(rep.PromptLens.Lenses))
	}
	for i, lensCfg := range rubric.Lenses() {
		got := rep.PromptLens.Lenses[i]
		if got.Code != lensCfg.ID || got.Name != lensCfg.Name {
			t.Fatalf("lens[%d] = %+v, want code/name from rubric %s/%s", i, got, lensCfg.ID, lensCfg.Name)
		}
		if got.Code == "L_decisions" && got.Level != 5 {
			t.Fatalf("L_decisions level = %d, want clamp to 5", got.Level)
		} else if got.Code != "L_decisions" && got.Level != 0 {
			t.Fatalf("missing lens %s level = %d, want 0", got.Code, got.Level)
		}
	}
}

func TestAssessReportLensNoteFromConfig(t *testing.T) {
	rep := assessChat(t, goodReportWire, AssessmentInput{})
	if rep.PromptLens.Note != rubric.Model().LensNote {
		t.Fatalf("promptLens.note = %q, want config lensNote", rep.PromptLens.Note)
	}
}

func TestAssessReportStatsNormalizedToThree(t *testing.T) {
	// under-produce: 1 stat → padded to 3.
	reply := `{"depthAxis":[],"autonomyAxis":[],
		"promptLens":{"stats":[{"label":"对话轮次","value":"10"}]},"narrative":"n"}`
	rep := assessChat(t, reply, AssessmentInput{})
	if len(rep.PromptLens.Stats) != 3 {
		t.Fatalf("stats = %d, want 3 (padded)", len(rep.PromptLens.Stats))
	}
	if rep.PromptLens.Stats[0].Label != "对话轮次" {
		t.Fatalf("stats[0] not passed through: %+v", rep.PromptLens.Stats[0])
	}
	if rep.PromptLens.Stats[1].Label != "" || rep.PromptLens.Stats[2].Value != "" {
		t.Fatalf("padded stats should be blank: %+v", rep.PromptLens.Stats)
	}

	// over-produce: 4 stats → truncated to first 3.
	replyOver := `{"depthAxis":[],"autonomyAxis":[],
		"promptLens":{"stats":[{"label":"a","value":"1"},{"label":"b","value":"2"},
		{"label":"c","value":"3"},{"label":"d","value":"4"}]},"narrative":"n"}`
	repOver := assessChat(t, replyOver, AssessmentInput{})
	if len(repOver.PromptLens.Stats) != 3 {
		t.Fatalf("stats = %d, want 3 (truncated)", len(repOver.PromptLens.Stats))
	}
	if repOver.PromptLens.Stats[2].Label != "c" {
		t.Fatalf("stats not truncated to first 3: %+v", repOver.PromptLens.Stats)
	}
}

func TestAssessReportProjectProjectionFalseYieldsNilSuperset(t *testing.T) {
	rep := assessChat(t, goodReportWireWithProject, AssessmentInput{ProjectProjection: false})
	if rep.OfficialProjection != nil {
		t.Fatalf("OfficialProjection should be nil when ProjectProjection is false, got %+v", rep.OfficialProjection)
	}
	if rep.WorkAndProcess != nil {
		t.Fatalf("WorkAndProcess should be nil when ProjectProjection is false, got %+v", rep.WorkAndProcess)
	}
}

func TestAssessReportProjectProjectionTruePassesThroughAndClampsReadiness(t *testing.T) {
	rep := assessChat(t, goodReportWireWithProject, AssessmentInput{ProjectProjection: true})
	if rep.OfficialProjection == nil {
		t.Fatalf("OfficialProjection should be present when ProjectProjection is true")
	}
	if rep.OfficialProjection.Standard.ID != "ap-research" {
		t.Fatalf("standard = %+v, want ap-research passthrough", rep.OfficialProjection.Standard)
	}
	if len(rep.OfficialProjection.Components) != 4 {
		t.Fatalf("components = %d, want 4", len(rep.OfficialProjection.Components))
	}
	if rep.OfficialProjection.Readiness.Score != 72 {
		t.Fatalf("readiness.score = %d, want 72 passthrough", rep.OfficialProjection.Readiness.Score)
	}
	if rep.WorkAndProcess == nil || len(rep.WorkAndProcess.WorkSamples) != 1 {
		t.Fatalf("WorkAndProcess not passed through: %+v", rep.WorkAndProcess)
	}

	// readiness.score out of range clamps to 0..100.
	replyHigh := `{"depthAxis":[],"autonomyAxis":[],"promptLens":{},"narrative":"n",
		"officialProjection":{"standard":{"id":"ap-research","name":"AP Research"},"readiness":{"score":150,"note":"n"}}}`
	repHigh := assessChat(t, replyHigh, AssessmentInput{ProjectProjection: true})
	if repHigh.OfficialProjection.Readiness.Score != 100 {
		t.Fatalf("readiness.score = %d, want clamp to 100", repHigh.OfficialProjection.Readiness.Score)
	}

	replyLow := `{"depthAxis":[],"autonomyAxis":[],"promptLens":{},"narrative":"n",
		"officialProjection":{"standard":{"id":"ap-research","name":"AP Research"},"readiness":{"score":-10,"note":"n"}}}`
	repLow := assessChat(t, replyLow, AssessmentInput{ProjectProjection: true})
	if repLow.OfficialProjection.Readiness.Score != 0 {
		t.Fatalf("readiness.score = %d, want clamp to 0", repLow.OfficialProjection.Readiness.Score)
	}
}

func TestAssessReportOfficialStandardFilledWhenMissing(t *testing.T) {
	reply := `{"depthAxis":[],"autonomyAxis":[],"promptLens":{},"narrative":"n",
		"officialProjection":{"readiness":{"score":50,"note":"n"}}}`
	rep := assessChat(t, reply, AssessmentInput{ProjectProjection: true})
	std, ok := rubric.Standard("ap-research")
	if !ok {
		t.Fatalf("rubric.Standard(ap-research) not found")
	}
	if rep.OfficialProjection.Standard.ID != std.ID || rep.OfficialProjection.Standard.Name != std.Name {
		t.Fatalf("standard not filled from rubric when missing: %+v, want %+v", rep.OfficialProjection.Standard, std)
	}
}

func TestAssessReportAxiomFromConfig(t *testing.T) {
	rep := assessChat(t, goodReportWire, AssessmentInput{})
	if rep.Axiom != rubric.Model().Axiom {
		t.Fatalf("axiom = %q, want config axiom", rep.Axiom)
	}
	if rep.Axiom != "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定" {
		t.Fatalf("axiom mismatch verbatim: %q", rep.Axiom)
	}
}

func TestAssessReportNilGuardsProduceEmptySlicesNotNull(t *testing.T) {
	reply := `{"depthAxis":[],"autonomyAxis":[],"promptLens":{},"narrative":"n"}`
	rep := assessChat(t, reply, AssessmentInput{})
	if rep.InteractionEvidence == nil {
		t.Fatalf("InteractionEvidence should be [] not nil")
	}
	if rep.Guidance.NextSteps == nil {
		t.Fatalf("Guidance.NextSteps should be [] not nil")
	}
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if strings.Contains(s, "null") {
		t.Fatalf("marshaled report contains a null array: %s", s)
	}
}

func TestAssessReportNilGuardsCoverProjectSupersetToo(t *testing.T) {
	reply := `{"depthAxis":[],"autonomyAxis":[],"promptLens":{},"narrative":"n",
		"officialProjection":{"standard":{"id":"ap-research","name":"AP Research"},"readiness":{"score":50,"note":"n"}},
		"workAndProcess":{}}`
	rep := assessChat(t, reply, AssessmentInput{ProjectProjection: true})
	if rep.OfficialProjection.Components == nil || rep.OfficialProjection.Alignment == nil {
		t.Fatalf("project superset nested arrays should nil-guard to []: %+v", rep.OfficialProjection)
	}
	if rep.WorkAndProcess.WorkSamples == nil || rep.WorkAndProcess.ProcessMaterials == nil {
		t.Fatalf("workAndProcess nested arrays should nil-guard to []: %+v", rep.WorkAndProcess)
	}
}

func TestAssessReportBannedPhrasingRejectsAcrossFreeTextFields(t *testing.T) {
	cases := map[string]string{
		"depth evidence": `{"depthAxis":[{"code":"D1","level":"L2","evidence":"你应该这样写：先摆结论"}],
			"autonomyAxis":[],"promptLens":{},"narrative":"n"}`,
		"autonomy evidence": `{"depthAxis":[],"autonomyAxis":[{"code":"A1","level":2,"opportunity":"given_taken","evidence":"你应该这样写：先摆结论"}],
			"promptLens":{},"narrative":"n"}`,
		"lens evidence": `{"depthAxis":[],"autonomyAxis":[],
			"promptLens":{"lenses":[{"code":"L_decisions","level":2,"evidence":"你应该这样写：先摆结论"}]},"narrative":"n"}`,
		"narrative": `{"depthAxis":[],"autonomyAxis":[],"promptLens":{},"narrative":"你应该这样写：先摆结论"}`,
		"next step": `{"depthAxis":[],"autonomyAxis":[],"promptLens":{},"narrative":"n",
			"guidance":{"nextSteps":[{"title":"你应该这样写：先摆结论","task":"t"}]}}`,
		"interaction evidence": `{"depthAxis":[],"autonomyAxis":[],"promptLens":{},"narrative":"n",
			"interactionEvidence":[{"round":1,"student":"s","aiSummary":"你应该这样写：先摆结论","signal":"sig"}]}`,
	}
	for name, reply := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, err := AssessReport(context.Background(), reportProvider(reply),
				gateway.Resolved{Tier: "flagship"}, rubric.Model(), AssessmentInput{})
			if err == nil {
				t.Fatalf("expected banned-phrasing rejection for %s", name)
			}
		})
	}
}

func TestAssessReportBannedPhrasingRejectsInProjectSupersetFields(t *testing.T) {
	cases := map[string]string{
		"component reason": `{"depthAxis":[],"autonomyAxis":[],"promptLens":{},"narrative":"n",
			"officialProjection":{"standard":{"id":"ap-research","name":"AP Research"},
			"components":[{"name":"Academic Paper","judgement":"Paper 3","reason":"你应该这样写：先摆结论"}],
			"readiness":{"score":50,"note":"n"}}}`,
		"alignment performance": `{"depthAxis":[],"autonomyAxis":[],"promptLens":{},"narrative":"n",
			"officialProjection":{"standard":{"id":"ap-research","name":"AP Research"},
			"alignment":[{"item":"i","standard":"s","performance":"你应该这样写：先摆结论","impact":"i"}],
			"readiness":{"score":50,"note":"n"}}}`,
		"readiness note": `{"depthAxis":[],"autonomyAxis":[],"promptLens":{},"narrative":"n",
			"officialProjection":{"standard":{"id":"ap-research","name":"AP Research"},"readiness":{"score":50,"note":"你应该这样写：先摆结论"}}}`,
		"work sample text": `{"depthAxis":[],"autonomyAxis":[],"promptLens":{},"narrative":"n",
			"officialProjection":{"standard":{"id":"ap-research","name":"AP Research"},"readiness":{"score":50,"note":"n"}},
			"workAndProcess":{"workSamples":[{"title":"t","text":"你应该这样写：先摆结论"}]}}`,
		"process material diagnosis": `{"depthAxis":[],"autonomyAxis":[],"promptLens":{},"narrative":"n",
			"officialProjection":{"standard":{"id":"ap-research","name":"AP Research"},"readiness":{"score":50,"note":"n"}},
			"workAndProcess":{"processMaterials":[{"name":"n","status":"s","diagnosis":"你应该这样写：先摆结论"}]}}`,
	}
	for name, reply := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, err := AssessReport(context.Background(), reportProvider(reply),
				gateway.Resolved{Tier: "flagship"}, rubric.Model(), AssessmentInput{ProjectProjection: true})
			if err == nil {
				t.Fatalf("expected banned-phrasing rejection for %s", name)
			}
		})
	}
}

// ---- prompt tests (Task 4) ----

func TestAssessReportSystemPromptCarriesDualAxisAndAxiom(t *testing.T) {
	sys := assessReportSystemPrompt(rubric.Model(), false)
	for _, want := range []string{"认知深度", "智识自主", "两轴永不合成总分", "不排名", "L4", "计数带"} {
		if !strings.Contains(sys, want) {
			t.Fatalf("system prompt missing %q", want)
		}
	}
}

func TestAssessReportSystemPromptOmitsOfficialProjectionWhenNotProject(t *testing.T) {
	sys := assessReportSystemPrompt(rubric.Model(), false)
	if strings.Contains(sys, "officialProjection") {
		t.Fatalf("non-project system prompt should not mention officialProjection: %q", sys)
	}
	if strings.Contains(sys, "AP Research") {
		t.Fatalf("non-project system prompt should not mention AP Research: %q", sys)
	}
}

func TestAssessReportSystemPromptIncludesOfficialProjectionWhenProject(t *testing.T) {
	sys := assessReportSystemPrompt(rubric.Model(), true)
	for _, want := range []string{"AP Research", "官方投影", "officialProjection", "workAndProcess"} {
		if !strings.Contains(sys, want) {
			t.Fatalf("project system prompt missing %q", want)
		}
	}
}

func TestAssessReportSystemPromptCarriesAntiFabricationGuard(t *testing.T) {
	sys := assessReportSystemPrompt(rubric.Model(), false)
	if !strings.Contains(sys, "禁止杜撰学生提示词") {
		t.Fatalf("system prompt missing anti-fabrication guard: %q", sys)
	}
}

func TestAssessReportUserInputAppendsWorkSamplesOnlyWhenSupplied(t *testing.T) {
	withSamples := assessReportUserInput(AssessmentInput{WorkSamples: []string{"从「中国让地球更可持续」改为限定判断版本"}})
	if !strings.Contains(withSamples, "作品片段") {
		t.Fatalf("user input with WorkSamples should include 作品片段 section: %q", withSamples)
	}
	without := assessReportUserInput(AssessmentInput{})
	if strings.Contains(without, "作品片段") {
		t.Fatalf("user input without WorkSamples should not include 作品片段 section: %q", without)
	}
}
