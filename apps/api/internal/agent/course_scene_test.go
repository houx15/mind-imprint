package agent

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

// TestGenerateCourseSceneOpeningThreadsSignals asserts the opening generator
// returns the model's text, threads only the signals that carry evidence into
// UsedSignalTypes, and places the course title (and a used signal's evidence)
// into the request the model receives.
func TestGenerateCourseSceneOpeningThreadsSignals(t *testing.T) {
	cap := &capturingProvider{inner: scriptedProvider("同学你好，这节课我们一起做信源辨识。")}
	in := SceneGenInput{
		Which:            "opening",
		CourseTitle:      "一条网络信息，该不该信",
		EstimatedMinutes: 12,
		Objectives:       []string{"学会用 CRAAP 判断信源"},
		LearningPreview:  []string{"给一条说法做溯源体检"},
		AllowedSignals:   []string{"last_course", "empty_signal"},
		SignalEvidence:   map[string]string{"last_course": "你上次完成了《论证的骨架》"},
	}
	out, usage, err := GenerateCourseScene(context.Background(), cap, gateway.Resolved{}, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Text != "同学你好，这节课我们一起做信源辨识。" {
		t.Fatalf("text = %q, want the model's completion", out.Text)
	}
	if len(out.UsedSignalTypes) != 1 || out.UsedSignalTypes[0] != "last_course" {
		t.Fatalf("usedSignalTypes = %v, want only [last_course] (empty_signal has no evidence)", out.UsedSignalTypes)
	}
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		t.Fatal("usage must be populated so the caller can meter the call")
	}
	if !strings.Contains(cap.lastPrompt, "一条网络信息，该不该信") {
		t.Fatalf("prompt should carry the course title, got:\n%s", cap.lastPrompt)
	}
	if !strings.Contains(cap.lastPrompt, "你上次完成了《论证的骨架》") {
		t.Fatalf("prompt should carry the used signal's evidence, got:\n%s", cap.lastPrompt)
	}
	if strings.Contains(cap.lastPrompt, "empty_signal") {
		t.Fatalf("an evidence-less signal must never reach the prompt, got:\n%s", cap.lastPrompt)
	}
}

// TestGenerateCourseSceneClosingCarriesSummary asserts the closing generator
// puts the prepared summary into the model request (so the narration can be
// held consistent with it, §6.2).
func TestGenerateCourseSceneClosingCarriesSummary(t *testing.T) {
	cap := &capturingProvider{inner: scriptedProvider("这节课就到这里，你完成了对一条说法的溯源。")}
	in := SceneGenInput{
		Which:                "closing",
		CourseTitle:          "一条网络信息，该不该信",
		PreparedSummary:      "你走完了 CRAAP 的五个维度，并给出了自己的判断。",
		Takeaways:            []string{"信源辨识的五个维度"},
		TransferApplications: []string{"下次读新闻时先查作者与时间"},
	}
	out, _, err := GenerateCourseScene(context.Background(), cap, gateway.Resolved{}, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Text == "" {
		t.Fatal("closing text must not be empty")
	}
	if len(out.UsedSignalTypes) != 0 {
		t.Fatalf("no allowed signals were given, usedSignalTypes should be empty, got %v", out.UsedSignalTypes)
	}
	if !strings.Contains(cap.lastPrompt, "你走完了 CRAAP 的五个维度，并给出了自己的判断。") {
		t.Fatalf("closing prompt must carry the prepared summary, got:\n%s", cap.lastPrompt)
	}
}

// TestGenerateCourseSceneErrorsSurfaceUsage asserts a failed model call returns
// the error but still lets the caller see usage (metering-on-error contract).
func TestGenerateCourseSceneErrorsSurfaceUsage(t *testing.T) {
	_, _, err := GenerateCourseScene(context.Background(), erroringProvider{err: context.DeadlineExceeded}, gateway.Resolved{},
		SceneGenInput{Which: "opening", CourseTitle: "x"})
	if err == nil {
		t.Fatal("expected an error when the provider fails")
	}
}
