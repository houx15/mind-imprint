package routebench

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"mindimprint/api/internal/benchcase"
)

// Artifact is a machine-readable snapshot for an optimization cycle.
// It contains synthetic fixtures only; no student data belongs here.
type Artifact struct {
	Schema     int               `json:"schema"`
	Suite      string            `json:"suite"`
	CapturedAt time.Time         `json:"capturedAt"`
	JudgeModel string            `json:"judgeModel"`
	Samples    int               `json:"samples"`
	CaseHashes map[string]string `json:"caseHashes"`
	Results    []Result          `json:"results"`
}

func NewArtifact(cases []benchcase.Case, cfg Config, results []Result, captured time.Time) Artifact {
	hashes := make(map[string]string)
	for _, c := range cases {
		if cfg.Suite != "" && c.Suite != cfg.Suite {
			continue
		}
		// Marshal the full request rather than only message text so response
		// format, token budget, tools and reasoning controls are traceable too.
		b, _ := json.Marshal(c.Request)
		h := sha256.Sum256(b)
		hashes[c.ID] = hex.EncodeToString(h[:])
	}
	return Artifact{Schema: 4, Suite: cfg.Suite, CapturedAt: captured, JudgeModel: cfg.JudgeModel,
		Samples: cfg.Samples, CaseHashes: hashes, Results: results}
}

func WriteArtifact(path string, a Artifact) error {
	b, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func ReadArtifact(path string) (Artifact, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Artifact{}, err
	}
	var a Artifact
	if err := json.Unmarshal(b, &a); err != nil {
		return Artifact{}, err
	}
	if a.Schema != 4 {
		return Artifact{}, fmt.Errorf("unsupported routebench artifact schema %d", a.Schema)
	}
	return a, nil
}

// TechnicalFailures alone determines -check's exit code. Fixture expectations
// and judge scores are deliberately review notes, not release policy.
func TechnicalFailures(a Artifact) []string {
	if len(a.Results) == 0 {
		return []string{"没有运行任何 case"}
	}
	var out []string
	for _, r := range a.Results {
		if r.Skipped != "" || len(r.Samples) == 0 {
			out = append(out, r.CaseID+"：没有可用的模型回复")
		}
		for i, s := range r.Samples {
			if s.Err != "" || (s.ParseErr != "" && s.ParseErr != "n/a") {
				out = append(out, fmt.Sprintf("%s 第 %d 次：调用或最终解析失败：%s %s", r.CaseID, i+1, s.Err, s.ParseErr))
			}
		}
	}
	return out
}

// CompareArtifactMarkdown gives directional evidence, never a pass/fail grade.
// A prompt hash is expected to change in a prompt experiment; case versions
// identify fixture changes that make a row incomparable.
func CompareArtifactMarkdown(before, after Artifact) string {
	var b strings.Builder
	b.WriteString("\n## 与改前运行对比（供人工审阅）\n\n")
	if before.Suite != after.Suite || before.Suite == "" {
		fmt.Fprintf(&b, "不可比：suite 不同（%q / %q）。\n", before.Suite, after.Suite)
		return b.String()
	}
	if before.JudgeModel != after.JudgeModel {
		b.WriteString("判官模型改变：质量分数不可直接比较。\n\n")
	}
	if before.Samples != after.Samples {
		b.WriteString("采样次数改变：百分比和中位数仅供参考。\n\n")
	}
	b.WriteString("| 用例 | 入 tokens p50 | 出 tokens p50 | 推理 tokens p50 | 延迟 p50 | 解析 | 预期 | 判官 |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|---:|---:|\n")
	old := map[string]Result{}
	for _, r := range before.Results {
		old[r.CaseID+"\x00"+r.ModelID] = r
	}
	seen := map[string]bool{}
	for _, r := range after.Results {
		k := r.CaseID + "\x00" + r.ModelID
		seen[k] = true
		p, ok := old[k]
		if !ok {
			fmt.Fprintf(&b, "| `%s` | 模型或用例不同，不可比 | | | | | | |\n", r.CaseID)
			continue
		}
		if p.Version != r.Version {
			fmt.Fprintf(&b, "| `%s` | 用例已更新，不可比 | | | | | | |\n", r.CaseID)
			continue
		}
		judge := "—"
		if p.JudgeMedian() > 0 && r.JudgeMedian() > 0 && before.JudgeModel == after.JudgeModel {
			judge = fmt.Sprintf("%.1f → %.1f", p.JudgeMedian(), r.JudgeMedian())
		}
		fmt.Fprintf(&b, "| `%s` | %d → %d | %d → %d | %d → %d | %.1fs → %.1fs | %.0f%% → %.0f%% | %.0f%% → %.0f%% | %s |\n",
			r.CaseID, p.MedIn(), r.MedIn(), p.MedOut(), r.MedOut(), p.MedReasoning(), r.MedReasoning(),
			p.P50Total().Seconds(), r.P50Total().Seconds(), p.ParseRate()*100, r.ParseRate()*100,
			p.ExpectedRate()*100, r.ExpectedRate()*100, judge)
	}
	var missing []string
	for k, r := range old {
		if !seen[k] {
			missing = append(missing, r.CaseID)
		}
	}
	sort.Strings(missing)
	for _, id := range missing {
		fmt.Fprintf(&b, "| `%s` | 改后未运行，不可比 | | | | | | |\n", id)
	}
	b.WriteString("\n预期和判官变化只提示审阅，不决定命令退出码。\n")
	return b.String()
}
