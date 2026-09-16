package pbl

import (
	"strings"
	"testing"
)

func TestArtifactTrialEvidenceScope(t *testing.T) {
	trial := ArtifactTrial{Mode: "self", Version: "本地第2版", Task: "打开作品", Expected: "看到说明", Actual: "按钮无响应"}
	if err := trial.Validate(true); err != nil {
		t.Fatal(err)
	}
	body := trial.Body("source-id", "作品页")
	for _, text := range []string{"作品页（source-id）", "不代表其他用户试用", "实际结果：按钮无响应"} {
		if !strings.Contains(body, text) {
			t.Fatal(body)
		}
	}
	trial.Mode = "other"
	if !strings.Contains(trial.Body("a", "b"), "尚未经系统独立验证") {
		t.Fatal("peer report claimed independent verification")
	}
	trial.Mode = "not_tested"
	if strings.Contains(trial.Body("a", "b"), trial.Actual) {
		t.Fatal("untested plan reused an old actual result")
	}
	trial.Mode = "self"
	trial.Actual = " "
	if trial.Validate(true) == nil {
		t.Fatal("tested result accepted without observation")
	}
	if err := trial.Validate(false); err != nil {
		t.Fatal("unfinished draft rejected", err)
	}
	trial.Mode = "not_tested"
	if err := trial.Validate(true); err != nil {
		t.Fatal("untested plan rejected", err)
	}
}
