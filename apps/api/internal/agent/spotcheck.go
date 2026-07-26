package agent

// spotcheck.go — N3f Task 2. The S3/S4 station spot-checks: 信源体检 and
// 论证体检. Structurally the same move as 整稿体检 (review.go) one and two
// stations earlier, which is what makes the two `human` gate items
// source_quality_spot_check / warrant_quality_spot_check satisfiable at all.
//
// A human gate item is satisfied by an adjudicating voice the student summons
// — the reading whole_draft_review/orderReview already established
// (api/writing.go:363-394). When a teacher-facing surface exists, these are
// where a real teacher routes in; the gate item names do not change.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/gateway"
)

// The two contracts that have a spot-check. Any other contract id is a 404 at
// the endpoint, so this route can never mint interventions against a station
// with no check defined.
const (
	SpotCheckSources  = "evaluate_sources"
	SpotCheckArgument = "build_argument"
)

// SpotCheckTarget is one thing the check examines: a source (S3) or a Toulmin
// slot (S4). Detail is the already-projected description the prompt carries —
// building it is the caller's job, so this file stays free of storage shapes.
type SpotCheckTarget struct {
	ID     string
	Name   string
	Detail string
}

// SpotCheckItem is one work-order row from a station spot-check.
//
// Deliberately has NO band and NO points. Points is 0457 readiness-gauge data
// (Slice 9), specific to the whole-draft review. A band would be a quality
// verdict on her sources, and the dossier deliberately carries no credibility
// field because such a judgment "has no honest producer and would have to be
// fabricated" (studio/dto.go:200-205) — a band here would smuggle it back in.
type SpotCheckItem struct {
	TargetID   string `json:"target_id"`
	TargetName string `json:"target_name"`
	Evidence   string `json:"evidence"`
	Missing    string `json:"missing"`
	Fix        string `json:"fix"` // advice only, never a rewritten sentence (RL-1)
}

// spotCheckItemWire is the model's per-item contract. target_name is absent by
// design: the name is resolved server-side from the id, the same discipline
// ProposeReview uses for CriterionName — the model never invents labels.
type spotCheckItemWire struct {
	TargetID string     `json:"target_id"`
	Evidence flexString `json:"evidence"`
	Missing  flexString `json:"missing"`
	Fix      flexString `json:"fix"`
}

const spotCheckPostureSources = `你是 IB/国际课程研究过程的「信源体检」考官。学生已经把她评估过的来源摆在这里。
只做一件事：逐条指出这条来源的「作用与风险」写到了什么程度、还缺什么——是没说清它能回答什么，
还是没说清它不能回答什么，还是根本没有交代它在论证里承担的角色。
铁律：绝不替学生改写句子、绝不给示范句、绝不续写。你的「建议」只能是"要补什么/要想清楚什么"的方向，
不能是可直接粘贴的成品句子。绝不给这条来源下「可信 / 存疑」这类结论——那是学生自己的判断。
一次只输出 JSON 数组，每条来源一个对象。`

const spotCheckPostureArgument = `你是 IB/国际课程论证结构的「论证体检」考官。学生已经把她的论证骨架摆在这里。
只做一件事：逐个位置指出这一步写到了什么程度、还缺什么——理据有没有把证据到主张之间的推理写出来，
反方是不是被写成了最强的版本（还是打了稻草人），让步有没有真的转折回来。
铁律：绝不替学生改写句子、绝不给示范句、绝不续写。你的「建议」只能是"要补哪一步/要想清楚什么"的方向，
不能是可直接粘贴的成品句子。一次只输出 JSON 数组，每个位置一个对象。`

// spotCheckFieldSpec pins the per-object JSON shape onto every posture. Without
// it the model picks its own keys (deepseek-v4-pro emits {"id","comment"}), so
// TargetID comes back empty, every item is dropped as an unknown target, and the
// whole order is rejected "no usable items" — the same failure class review.go
// was hardened against. target_id MUST echo the bracketed id from the user
// message verbatim so the server can resolve the name back (spotCheckItemWire).
const spotCheckFieldSpec = `

每个对象必须且只包含这四个字段（键名一字不差）：
- "target_id"：原样回填题面里每条前面方括号 [] 中的 id，一个字都不能改；
- "evidence"：这一条目前已经写到什么程度；
- "missing"：还缺什么、还没说清什么；
- "fix"：要补什么、要想清楚什么方向的建议（绝不是可直接粘贴的成品句子）。
只输出这个 JSON 数组本身，不要任何额外说明文字或代码块围栏。`

// spotCheckSystemPrompt selects the station's posture and appends the shared
// field spec. Pure — no I/O — so posture selection is unit-testable without a
// model.
func spotCheckSystemPrompt(station string) string {
	if station == SpotCheckArgument {
		return spotCheckPostureArgument + spotCheckFieldSpec
	}
	return spotCheckPostureSources + spotCheckFieldSpec
}

// ProposeSpotCheck asks the flagship model for one station's work order, then
// runs the enforcement stack on every field before returning. A single
// banned-phrasing violation rejects the WHOLE order (nothing returned, nothing
// persisted) — the same all-or-nothing discipline as ProposeReview and the
// coach. Usage is populated whenever Collect succeeded, even on a later
// rejection, so the caller can still record 档位+token+成本: a rejected call
// still cost money.
func ProposeSpotCheck(ctx context.Context, prov gateway.Provider, r gateway.Resolved, station string, targets []SpotCheckTarget) ([]SpotCheckItem, gateway.ChatUsage, error) {
	name := make(map[string]string, len(targets))
	lines := make([]string, 0, len(targets))
	for _, t := range targets {
		name[t.ID] = t.Name
		lines = append(lines, fmt.Sprintf("[%s] %s —— %s", t.ID, t.Name, t.Detail))
	}
	user := strings.Join(lines, "\n")

	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: spotCheckSystemPrompt(station)},
			{Role: gateway.RoleUser, Content: user},
		},
	})
	if err != nil {
		return nil, gateway.ChatUsage{}, err
	}
	usage := res.Usage

	var wires []spotCheckItemWire
	if err := json.Unmarshal([]byte(strings.TrimSpace(res.Text)), &wires); err != nil {
		return nil, usage, fmt.Errorf("agent: spot-check output not a JSON array: %w", err)
	}
	items := make([]SpotCheckItem, 0, len(wires))
	for _, wv := range wires {
		nm, known := name[wv.TargetID]
		if !known {
			continue // ignore targets the caller didn't ask about
		}
		for _, field := range []string{wv.Evidence.String(), wv.Missing.String(), wv.Fix.String()} {
			if field == "" {
				continue
			}
			if rule := enforcement.BannedPhrasing(field); rule != nil {
				return nil, usage, fmt.Errorf("agent: spot-check output rejected by banned-phrasing rule %q", rule.Name)
			}
		}
		items = append(items, SpotCheckItem{
			TargetID: wv.TargetID, TargetName: nm,
			Evidence: wv.Evidence.String(), Missing: wv.Missing.String(), Fix: wv.Fix.String(),
		})
	}
	if len(items) == 0 {
		return nil, usage, fmt.Errorf("agent: spot-check produced no usable items")
	}
	return items, usage, nil
}

// SpotCheckFingerprint hashes exactly what a spot-check reads, so an unchanged
// order can be answered from storage with zero model calls. This is
// orderReview's one-snapshot-one-review guard with the snapshot id generalized
// to a content hash, because S3/S4 have no commit action to anchor on.
//
// The serialization is length-prefixed, not delimiter-joined: a student's
// risk_note may contain any character, so no separator byte is safe, and a
// naive join would let two different dossiers collide.
func SpotCheckFingerprint(targets []SpotCheckTarget) string {
	h := sha256.New()
	write := func(s string) {
		fmt.Fprintf(h, "%d:", len(s))
		h.Write([]byte(s))
	}
	for _, t := range targets {
		write(t.ID)
		write(t.Name)
		write(t.Detail)
	}
	return hex.EncodeToString(h.Sum(nil))
}
