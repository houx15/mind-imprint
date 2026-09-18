package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"mindimprint/api/internal/gateway"
)

// walkArtifact keeps multi-turn evidence separate from single-turn evidence.
type walkArtifact struct {
	Schema         int               `json:"schema"`
	Suite          string            `json:"suite"`
	CapturedAt     time.Time         `json:"capturedAt"`
	JudgeModel     string            `json:"judgeModel"`
	Turns          int               `json:"turns"`
	Repeats        int               `json:"repeats"`
	ScenarioHashes map[string]string `json:"scenarioHashes"`
	Rows           []walkArtifactRow `json:"rows"`
}

type walkArtifactRow struct {
	ID                 string                  `json:"id"`
	Version            int                     `json:"version"`
	Model              string                  `json:"model"`
	Run                int                     `json:"run"`
	Completed          bool                    `json:"completed"`
	Error              string                  `json:"error,omitempty"`
	Score              float64                 `json:"score"`
	JudgeWhy           string                  `json:"judgeWhy,omitempty"`
	P50MS              int64                   `json:"p50Ms"`
	P50InputTokens     int                     `json:"p50InputTokens"`
	P50OutputTokens    int                     `json:"p50OutputTokens"`
	P50ReasoningTokens int                     `json:"p50ReasoningTokens"`
	Retries            int                     `json:"retries"`
	Violations         map[string]int          `json:"violations"`
	ViolationDetails   []walkArtifactViolation `json:"violationDetails,omitempty"`
	Observations       map[string]int          `json:"observations,omitempty"`
	ObservationDetails []walkArtifactViolation `json:"observationDetails,omitempty"`
	Turns              []walkArtifactTurn      `json:"turnDetails,omitempty"`
}

type walkArtifactViolation struct {
	Turn int    `json:"turn,omitempty"`
	Kind string `json:"kind"`
	Note string `json:"note"`
}

type walkArtifactTurn struct {
	Number          int                   `json:"number"`
	Reply           string                `json:"reply,omitempty"`
	StudentSaid     string                `json:"studentSaid,omitempty"`
	ParseError      string                `json:"parseError,omitempty"`
	SelectedAttempt int                   `json:"selectedAttempt,omitempty"`
	RecoveryReason  string                `json:"recoveryReason,omitempty"`
	InputTokens     int                   `json:"inputTokens"`
	OutputTokens    int                   `json:"outputTokens"`
	ReasoningTokens int                   `json:"reasoningTokens"`
	LatencyMS       int64                 `json:"latencyMs"`
	Attempts        []walkArtifactAttempt `json:"attempts"`
}

type walkArtifactAttempt struct {
	Raw             string `json:"raw,omitempty"`
	Error           string `json:"error,omitempty"`
	InputTokens     int    `json:"inputTokens"`
	OutputTokens    int    `json:"outputTokens"`
	ReasoningTokens int    `json:"reasoningTokens"`
	LatencyMS       int64  `json:"latencyMs"`
}

// hashCoaches records exactly which initial production request and scripted
// student path produced an artifact. The hash is evidence, not a compatibility
// check: changing the production prompt is the experiment a prompt gate exists
// to compare before and after.
func hashCoaches(coaches []coach) (map[string]string, error) {
	out := make(map[string]string, len(coaches))
	for _, c := range coaches {
		id := c.id
		if id == "" {
			id = c.name
		}
		payload := struct {
			Request gateway.ChatRequest `json:"request"`
			Script  []string            `json:"script,omitempty"`
		}{Request: c.make().Request(), Script: c.script}
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("hash scenario %s: %w", id, err)
		}
		sum := sha256.Sum256(b)
		out[id] = hex.EncodeToString(sum[:])
	}
	return out, nil
}

func writeWalkArtifact(path string, a walkArtifact) error {
	b, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func readWalkArtifact(path string) (walkArtifact, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return walkArtifact{}, err
	}
	var a walkArtifact
	if err := json.Unmarshal(b, &a); err != nil {
		return walkArtifact{}, err
	}
	if a.Schema != 3 {
		return walkArtifact{}, fmt.Errorf("unsupported coachwalk artifact schema %d", a.Schema)
	}
	return a, nil
}

func walkTechnicalFailures(a walkArtifact) []string {
	if len(a.Rows) == 0 {
		return []string{"没有运行任何多轮 scenario"}
	}
	var out []string
	for _, r := range a.Rows {
		if r.Error != "" {
			out = append(out, fmt.Sprintf("%s 第 %d 条：%s", r.ID, r.Run, r.Error))
		}
		if r.Violations["parse"] > 0 {
			out = append(out, fmt.Sprintf("%s 第 %d 条：最终回复解析失败", r.ID, r.Run))
		}
	}
	return out
}

func compareWalkArtifacts(before, after walkArtifact) string {
	var b strings.Builder
	b.WriteString("\n## 与改前运行对比（供人工审阅）\n\n")
	if before.Suite == "" || before.Suite != after.Suite {
		fmt.Fprintf(&b, "不可比：suite 不同（%q / %q）。\n", before.Suite, after.Suite)
		return b.String()
	}
	if before.JudgeModel != after.JudgeModel {
		b.WriteString("判官模型不同：分数不可直接比较。\n\n")
	}
	if before.Repeats != after.Repeats {
		b.WriteString("重复次数不同：汇总数字仅供参考。\n\n")
	}
	b.WriteString("| 路径 | 入 tokens p50 | 出 tokens p50 | 推理 tokens p50 | 延迟 p50 | 到达终态 | 判官 |\n")
	b.WriteString("|---|---:|---:|---:|---:|---|---:|\n")
	key := func(r walkArtifactRow) string { return fmt.Sprintf("%s\x00%s\x00%d", r.ID, r.Model, r.Run) }
	old := map[string]walkArtifactRow{}
	for _, r := range before.Rows {
		old[key(r)] = r
	}
	seen := map[string]bool{}
	for _, r := range after.Rows {
		k := key(r)
		seen[k] = true
		p, ok := old[k]
		if !ok {
			fmt.Fprintf(&b, "| `%s` 第 %d 条 | 模型或路径不同，不可比 | | | | | |\n", r.ID, r.Run)
			continue
		}
		if p.Version != r.Version {
			fmt.Fprintf(&b, "| `%s` 第 %d 条 | 路径已更新，不可比 | | | | | |\n", r.ID, r.Run)
			continue
		}
		score := "—"
		if before.JudgeModel == after.JudgeModel && p.Score > 0 && r.Score > 0 {
			score = fmt.Sprintf("%.0f → %.0f", p.Score, r.Score)
		}
		fmt.Fprintf(&b, "| `%s` 第 %d 条 | %d → %d | %d → %d | %d → %d | %.1fs → %.1fs | %t → %t | %s |\n",
			r.ID, r.Run, p.P50InputTokens, r.P50InputTokens, p.P50OutputTokens, r.P50OutputTokens,
			p.P50ReasoningTokens, r.P50ReasoningTokens, float64(p.P50MS)/1000, float64(r.P50MS)/1000,
			p.Completed, r.Completed, score)
	}
	var missing []string
	for k, r := range old {
		if !seen[k] {
			missing = append(missing, fmt.Sprintf("%s 第 %d 条", r.ID, r.Run))
		}
	}
	sort.Strings(missing)
	for _, id := range missing {
		fmt.Fprintf(&b, "| `%s` | 改后未运行，不可比 | | | | | |\n", id)
	}
	b.WriteString("\n终态与判官变化只提示审阅，不决定命令退出码。\n")
	return b.String()
}
