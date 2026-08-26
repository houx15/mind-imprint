package evalbench

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/evalreport"
	"mindimprint/api/internal/gateway"
)

func TestSinglePromptEvaluatorBuildsOneStrictReport(t *testing.T) {
	in := singlePromptTestInput()
	reply := validSinglePromptReply()
	reply.Depth[0].Evidence[0].ID = "chat:valid"
	reply.Depth[1].Evidence[0].ID = "not-a-real-id"
	reply.PromptLens.Prompts[0].Ref.ID = "chat:valid"
	reply.Risks[0].Ref.ID = "not-a-real-id"
	text := marshalSinglePromptReply(t, reply)

	stub := gateway.NewStubProvider(streamText(text))
	var purposes []string
	result, err := singlePromptEvalReportV1{}.Run(context.Background(), RunDeps{
		Resolved: gateway.Resolved{Provider: "deepseek", Model: "test", Tier: FlagshipTier},
		ProviderForPurpose: func(purpose string) gateway.Provider {
			purposes = append(purposes, purpose)
			return stub
		},
	}, in, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Complete || !reflect.DeepEqual(result.Report.Basics, in.Basics) || !reflect.DeepEqual(result.Report.Events, in.Events) || !reflect.DeepEqual(result.Report.Materials, in.Materials) || !reflect.DeepEqual(result.Report.ToolUsage, in.ToolUsage) {
		t.Fatalf("FACT fields changed: %#v", result.Report)
	}
	if !reflect.DeepEqual(purposes, []string{singlePromptPurpose}) {
		t.Fatalf("purposes = %#v", purposes)
	}
	if stub.LastRequest.MaxTokens != singlePromptMaxTokens || stub.LastRequest.DisableThinking {
		t.Fatalf("request changed evaluator model settings: %#v", stub.LastRequest)
	}
	if len(stub.LastRequest.Messages) != 2 || !strings.Contains(stub.LastRequest.Messages[0].Content, "D1") || !strings.Contains(stub.LastRequest.Messages[1].Content, in.Context.Title) || !strings.Contains(stub.LastRequest.Messages[1].Content, in.Context.Candidates) || !strings.Contains(stub.LastRequest.Messages[1].Content, in.Context.Trajectory) || !strings.Contains(stub.LastRequest.Messages[1].Content, in.Context.Prompts) || !strings.Contains(stub.LastRequest.Messages[1].Content, in.Context.RiskSignals) {
		t.Fatalf("request is missing required input: %#v", stub.LastRequest.Messages)
	}
	if result.Report.Depth[0].ID != "D1" || result.Report.Depth[0].Evidence[0].ID != "chat:valid" || result.Report.Depth[1].Evidence[0].ID != "" || result.Report.Risks[0].Ref.ID != "" {
		t.Fatalf("axes/ref validation failed: %#v", result.Report)
	}
}

func TestSinglePromptEvaluatorRejectsOutOfOrderAxes(t *testing.T) {
	in := singlePromptTestInput()
	reply := validSinglePromptReply()
	reverseDepth(reply.Depth)
	reverseAutonomy(reply.Autonomy)
	stub := gateway.NewStubProvider(streamText("```json\n" + marshalSinglePromptReply(t, reply) + "\n```"))
	_, err := singlePromptEvalReportV1{}.Run(context.Background(), RunDeps{Resolved: gateway.Resolved{}, ProviderForPurpose: func(string) gateway.Provider { return stub }}, in, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("out-of-order tuple unexpectedly accepted")
	}
}

func TestSinglePromptEvaluatorRejectsMalformedOrIncompleteReplyWithoutRetry(t *testing.T) {
	base := validSinglePromptReply()
	unknownField := marshalSinglePromptReply(t, base)
	unknownField = strings.TrimSuffix(unknownField, "}") + `,"unexpected":true}`
	cases := []struct {
		name string
		text string
	}{
		{name: "not JSON", text: "not JSON"},
		{name: "trailing JSON", text: marshalSinglePromptReply(t, base) + " true"},
		{name: "unknown field", text: unknownField},
		{name: "missing depth", text: marshalSinglePromptReply(t, withMissingDepth(copySinglePromptReply(t, base)))},
		{name: "duplicate depth", text: marshalSinglePromptReply(t, withDuplicateDepth(copySinglePromptReply(t, base)))},
		{name: "invalid level", text: marshalSinglePromptReply(t, withInvalidDepthLevel(copySinglePromptReply(t, base)))},
		{name: "invalid risk", text: marshalSinglePromptReply(t, withInvalidRisk(copySinglePromptReply(t, base)))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := gateway.NewSequenceStubProvider(streamText(tc.text), streamText(marshalSinglePromptReply(t, base)))
			_, err := singlePromptEvalReportV1{}.Run(context.Background(), RunDeps{ProviderForPurpose: func(string) gateway.Provider { return stub }}, singlePromptTestInput(), json.RawMessage(`{}`))
			if err == nil {
				t.Fatal("Run unexpectedly succeeded")
			}
			if stub.Calls != 1 {
				t.Fatalf("calls = %d, want exactly one", stub.Calls)
			}
		})
	}
}

func TestSinglePromptEvaluatorRejectsNullRequiredArrayWithoutNormalizing(t *testing.T) {
	reply := validSinglePromptReply()
	text := strings.Replace(marshalSinglePromptReply(t, reply), `"suggestionSentences":["下一步"]`, `"suggestionSentences":null`, 1)
	stub := gateway.NewStubProvider(streamText(text))
	_, err := singlePromptEvalReportV1{}.Run(context.Background(), RunDeps{ProviderForPurpose: func(string) gateway.Provider { return stub }}, singlePromptTestInput(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("null required array unexpectedly accepted")
	}
}

func TestSinglePromptDescriptorAndParams(t *testing.T) {
	e := singlePromptEvalReportV1{}
	d := e.Descriptor()
	if d.ID != singlePromptEvaluatorID || d.ImplementationVersion != singlePromptImplementationV1 || d.PromptVersion != singlePromptTemplateVersion || len(d.PromptSHA256) != 64 || d.PromptSHA256 != e.Descriptor().PromptSHA256 {
		t.Fatalf("descriptor = %#v", d)
	}
	if err := e.ValidateParams(json.RawMessage(`{}`)); err != nil {
		t.Fatalf("empty params rejected: %v", err)
	}
	if err := e.ValidateParams(json.RawMessage(`{"prompt":"override"}`)); err == nil {
		t.Fatal("prompt override unexpectedly accepted")
	}
	descriptors := evaluatorDescriptors([]VariantConfig{{ID: "production", Evaluator: "production-evalreport-v1"}, {ID: "single", Evaluator: singlePromptEvaluatorID}})
	if descriptors["single"].PromptSHA256 != d.PromptSHA256 || descriptors["production"].ImplementationVersion != "evalbench-production-baseline-v1" {
		t.Fatalf("descriptors = %#v", descriptors)
	}
}

func TestSinglePromptV1EvaluatorUsesCompleteSchemaAndExample(t *testing.T) {
	stub := gateway.NewStubProvider(streamText(singlePromptExample))
	var purposes []string
	result, err := singlePromptEvalReportV1{}.Run(context.Background(), RunDeps{
		Resolved: gateway.Resolved{Provider: "deepseek", Model: "test", Tier: FlagshipTier},
		ProviderForPurpose: func(purpose string) gateway.Provider {
			purposes = append(purposes, purpose)
			return stub
		},
	}, singlePromptTestInput(), json.RawMessage(`{}`))
	if err != nil || !result.Complete {
		t.Fatalf("Run: result=%#v err=%v", result, err)
	}
	if !reflect.DeepEqual(purposes, []string{singlePromptPurpose}) {
		t.Fatalf("purposes=%#v", purposes)
	}
	if stub.LastRequest.MaxTokens != singlePromptMaxTokens || stub.LastRequest.DisableThinking || stub.LastRequest.ResponseFormat != gateway.ResponseFormatJSONObject {
		t.Fatalf("request = %#v", stub.LastRequest)
	}
	system := stub.LastRequest.Messages[0].Content
	for _, want := range []string{`"$ref": "#/definitions/EvaluationReportModelOutput"`, `"courseId"`, `"attention"`, `"type": "boolean"`, `"ai-ghostwrite"`, `"id": "D6"`, `"id": "A6"`, "不能复制尖括号占位值"} {
		if !strings.Contains(system, want) {
			t.Fatalf("complete schema prompt is missing %q", want)
		}
	}
}

func singlePromptTestInput() Input {
	return Input{
		ReportID: "report", ProjectID: "project", StudentID: "student", StudentName: "Student", GeneratedAt: "2026-08-18T00:00:00Z",
		Basics:     evalreport.Basics{Title: "测试题目", StartDate: "2026-08-01T00:00:00Z", Counters: evalreport.Counters{AITurns: 2, MaterialsRead: 1, WordsWritten: 100, AICommentCount: 1, EditCount: 1}},
		Events:     []evalreport.EventEntry{{TS: "2026-08-01T00:00:00Z", Kind: "chat", Summary: "学生发起问题"}},
		Materials:  []evalreport.MaterialEntry{{MaterialID: "material:1", AddedAt: "2026-08-01T00:00:00Z", Source: "来源", FinalStatus: "使用"}},
		ToolUsage:  []evalreport.ToolUsageEntry{{ToolID: "craap", Name: "CRAAP", Purpose: "completed", Summary: "完成"}},
		Candidates: []evalreport.Candidate{{ID: "chat:valid", Kind: evalreport.KindChat, Label: "学生问题"}},
		Context:    agent.ReportGenContext{Title: "测试题目", Counters: "AI轮次2", Candidates: "[chat:valid] chat: 学生问题", Trajectory: "轨迹文本", Prompts: "提示词文本", RiskSignals: "风险文本"},
	}
}

func validSinglePromptReply() singlePromptReply {
	reply := singlePromptReply{
		Abstract:   evalreport.Abstract{Overview: "综述", MaterialSentence: "材料", WritingSentence: "写作", AISentence: "AI", SuggestionParagraph: "建议", SuggestionSentences: []string{"下一步"}, RecommendedCourses: []evalreport.RecommendedCourse{}},
		PromptLens: evalreport.PromptLens{Summary: "提问透镜", Prompts: []evalreport.PromptItem{{Stage: "研究", Quote: "学生问题", Ref: evalreport.Ref{ID: "chat:valid", Label: "学生问题"}, Observation: "观察", RelatedDomains: []string{}, Attention: false}}},
		Risks:      []evalreport.RiskEntry{{Type: "missing-source", Behaviour: "风险", Ref: &evalreport.Ref{ID: "chat:valid", Label: "学生问题"}, Suggestion: "补充来源"}},
	}
	for _, id := range []string{"D1", "D2", "D3", "D4", "D5", "D6"} {
		reply.Depth = append(reply.Depth, evalreport.DepthDimResult{ID: id, Level: 2, Summary: id, Evidence: []evalreport.EvidenceItem{{ID: "chat:valid", Quote: "学生问题", Observation: "观察"}}, Suggestion: "建议"})
	}
	for _, id := range []string{"A1", "A2", "A3", "A4", "A5", "A6"} {
		reply.Autonomy = append(reply.Autonomy, evalreport.AutonomyDimResult{ID: id, Band: 2, Summary: id, Evidence: []evalreport.EvidenceItem{{ID: "chat:valid", Quote: "学生问题", Observation: "观察"}}, Suggestion: "建议"})
	}
	return reply
}

func marshalSinglePromptReply(t *testing.T, reply singlePromptReply) string {
	t.Helper()
	b, err := json.Marshal(reply)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func copySinglePromptReply(t *testing.T, reply singlePromptReply) singlePromptReply {
	t.Helper()
	var copy singlePromptReply
	if err := json.Unmarshal([]byte(marshalSinglePromptReply(t, reply)), &copy); err != nil {
		t.Fatal(err)
	}
	return copy
}

func streamText(text string) []gateway.StreamEvent {
	return []gateway.StreamEvent{{Kind: gateway.EventTextDelta, TextDelta: text}, {Kind: gateway.EventDone, StopReason: gateway.StopStop}}
}

func reverseDepth(items []evalreport.DepthDimResult) {
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
}

func reverseAutonomy(items []evalreport.AutonomyDimResult) {
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
}

func withMissingDepth(reply singlePromptReply) singlePromptReply {
	reply.Depth = reply.Depth[1:]
	return reply
}

func withDuplicateDepth(reply singlePromptReply) singlePromptReply {
	reply.Depth[1].ID = "D1"
	return reply
}

func withInvalidDepthLevel(reply singlePromptReply) singlePromptReply {
	reply.Depth[0].Level = 0
	return reply
}

func withInvalidRisk(reply singlePromptReply) singlePromptReply {
	reply.Risks[0].Type = "invalid"
	return reply
}
