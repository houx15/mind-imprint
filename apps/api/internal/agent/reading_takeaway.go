package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/gateway"
)

// reading_takeaway.go — S2 · the reading sub-agent's "return". Record fields
// (findings/credibility/key_quotes) are the student's ALREADY-confirmed work,
// assembled by the caller (api.readingOutcomesByMaterial) — never re-guessed
// here. This file only SEEDS the two synthesis fields (new_leads/
// proposal_impact) with ONE isolated mid-tier call, constrained to organize
// the student's own material, never to conclude for her (铁律 · 克制). Mirrors
// the assessment/summary templates: pure input → one call → struct.

// Credibility is the student's own credibility verdict on the material
// (e.g. from a completed CRAAP/SIFT card), carried through verbatim.
type Credibility struct {
	Verdict string `json:"verdict"`
	Why     string `json:"why"`
}

// KeyQuote is one of the student's own confirmed evidence quotes.
type KeyQuote struct {
	Quote string `json:"quote"`
	Why   string `json:"why"`
}

// TakeawayRecord is the assembled, deterministic half — the student's
// confirmed work. Never produced or altered by an LLM call.
type TakeawayRecord struct {
	Findings    []string    `json:"findings"`
	Credibility Credibility `json:"credibility"`
	KeyQuotes   []KeyQuote  `json:"key_quotes"`
}

// ReadingTakeaway is the full 5-field object written to reference.takeaway:
// the record half (assembled) plus the synthesis half (seeded by
// ComposeReadingTakeawaySuggestions).
type ReadingTakeaway struct {
	Findings       []string    `json:"findings"`
	Credibility    Credibility `json:"credibility"`
	KeyQuotes      []KeyQuote  `json:"key_quotes"`
	NewLeads       []string    `json:"new_leads"`
	ProposalImpact string      `json:"proposal_impact"`
}

// ReadingTakeawayInput is ComposeReadingTakeawaySuggestions's sole input: the
// reading brief (why she read this) plus her already-assembled record.
type ReadingTakeawayInput struct {
	Brief  ReadingBrief
	Record TakeawayRecord
}

const readingTakeawaySystem = `你是「思维印记」的陪读助手。学生刚读完一篇材料，下面是她自己已经确认的发现、可信度判断和关键引用。请只做两件事，且只能基于她已有的材料，绝不替她下新结论：
1) new_leads：这篇material还遗留、或新引出的、值得继续追的问题（0-3条，每条一句）。
2) proposal_impact：这篇如何影响她的论点/论证（一句话，用她发现里已有的东西，不新增立场）。
只输出 JSON：{"new_leads":[...],"proposal_impact":"..."}。这些是给学生的草稿建议，她会改写。`

// HasRecordContent reports whether there is anything the student actually
// confirmed to organize. An empty record means nothing to seed from — 克制
// requires erroring rather than fabricating a synthesis out of nothing.
// Exported so callers (api.getTakeawayDraft) can gate the resolver/compose/
// meter block on it BEFORE ever resolving a provider — an empty record must
// never touch the network or the llm_call audit trail.
func HasRecordContent(rec TakeawayRecord) bool {
	return len(rec.Findings) > 0 || len(rec.KeyQuotes) > 0 || strings.TrimSpace(rec.Credibility.Verdict) != ""
}

// ComposeReadingTakeawaySuggestions seeds ONLY the two synthesis fields
// (new_leads/proposal_impact) from the student's already-assembled record via
// one isolated mid-tier LLM call. It never emits or re-guesses
// findings/credibility/key_quotes — those are the caller's job
// (readingOutcomesByMaterial). Errors, never fabricates, when the record is
// empty (nothing confirmed to organize).
func ComposeReadingTakeawaySuggestions(ctx context.Context, prov gateway.Provider, r gateway.Resolved, in ReadingTakeawayInput) ([]string, string, gateway.ChatUsage, error) {
	if !HasRecordContent(in.Record) {
		return nil, "", gateway.ChatUsage{}, fmt.Errorf("agent: empty reading record — nothing to organize")
	}
	recJSON, _ := json.Marshal(in.Record)
	user := "阅读目的：" + in.Brief.Reason + "（" + in.Brief.PhaseTag + "）\n她已确认的内容：\n" + string(recJSON)
	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: readingTakeawaySystem},
			{Role: gateway.RoleUser, Content: user},
		},
	})
	if err != nil {
		return nil, "", gateway.ChatUsage{}, err
	}
	var out struct {
		NewLeads       []string `json:"new_leads"`
		ProposalImpact string   `json:"proposal_impact"`
	}
	if err := json.Unmarshal([]byte(stripFences(res.Text)), &out); err != nil {
		return nil, "", res.Usage, fmt.Errorf("agent: takeaway suggestions parse: %w", err)
	}
	return out.NewLeads, strings.TrimSpace(out.ProposalImpact), res.Usage, nil
}
