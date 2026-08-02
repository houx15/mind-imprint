package agent

// compile_card.go — turn a completed card's raw field_values into a readable
// student-turn text the coach can respond to. It mirrors the web's
// apps/web/src/studio/compileCard.ts `compileCardForCoach`, so the text the
// model sees (persisted as the student's chat_message) and the text the student
// sees locally use the SAME algorithm and read identically on reload.
//
// It is a plain PROJECTION, not authorship (铁律①): it only reformats what the
// STUDENT typed into the card — it never adds sentences of its own.

import (
	"sort"
	"strconv"
	"strings"

	"mindimprint/api/internal/cards"
)

// compileValueToText renders one field value as plain text, mirroring
// valueToText in compileCard.ts: strings trimmed; numbers/bools stringified;
// arrays joined with "、"; objects flattened one level (deterministic key
// order) joined with " · ". Anything else → "".
func compileValueToText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(x)
	case bool:
		return strconv.FormatBool(x)
	case float64: // JSON numbers decode to float64
		return strconv.FormatFloat(x, 'f', -1, 64)
	case []any:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			if s := compileValueToText(e); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "、")
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			if s := compileValueToText(x[k]); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, " · ")
	default:
		return ""
	}
}

// CompileCardForCoach walks the card's declared fields (label + the student's
// value) into a short titled block: `我刚填完《<name>》：` then one `- <label>：
// <value>` line per non-empty field. Custom-renderer cards whose field_values
// keys don't line up with declared field keys fall back to a generic dump (by
// key, deterministic order) so no answer is silently dropped. Returns "" when
// the student filled nothing — the caller treats an empty result as a no-op
// (empty card = no persist, no spend).
func CompileCardForCoach(spec cards.Spec, fieldValues map[string]any) string {
	lines := make([]string, 0, len(fieldValues))
	seen := make(map[string]bool)
	for _, step := range spec.Steps {
		for _, f := range step.Fields {
			txt := compileValueToText(fieldValues[f.Key])
			if txt == "" {
				continue
			}
			seen[f.Key] = true
			lines = append(lines, "- "+f.Label+"："+txt)
		}
	}
	// Fallback: surface any non-empty values a custom renderer stored under keys
	// the declarative walk didn't cover (deterministic order).
	extra := make([]string, 0)
	for k := range fieldValues {
		if seen[k] {
			continue
		}
		if compileValueToText(fieldValues[k]) != "" {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	for _, k := range extra {
		lines = append(lines, "- "+k+"："+compileValueToText(fieldValues[k]))
	}
	if len(lines) == 0 {
		return ""
	}
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		name = spec.ID
	}
	return "我刚填完《" + name + "》：\n" + strings.Join(lines, "\n")
}
