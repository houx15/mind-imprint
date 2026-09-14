package agent

// compose_lite_parent_live_test.go — the parent report composer against a real
// model. Skipped unless LIVE_LLM=1 and a provider key is set.
//
//	set -a; . .deploy-local/env.prod; set +a
//	LIVE_LLM=1 CGO_ENABLED=0 go test ./internal/agent -run TestLiveLiteParent -v -count=1 -timeout 600s
//
// The stub tests prove the validator reads JSON the tests wrote. Only a real
// model shows whether the prompt gets the sections past CheckProse on the
// first attempt. The run records the risks named in plan 3's final review:
// numbering in next (stripped before the check), Chinese numerals, arithmetic
// on the facts, and a title written in 「」 instead of 《》.
//
// The provider and resolver come from liveLiteWeeklyModel, which builds them
// the way cmd/api does (config.Load, then gateway.NewResolvers). The class is
// ClassAssess, the one the parent report handlers route.

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/liteparent"
)

// liveLiteParentFacts is four realistic weeks, in the shape the loader fills.
// The counts share no digit run with the range dates (2026, 08, 17, 09, 13, 8,
// 9, 28), so a count the model changes shows up as a digit not in the facts.
func liveLiteParentFacts() liteparent.Facts {
	return liteparent.Facts{
		StudentName: "林小雨",
		ClassName:   "高一（3）班 · 阅读写作",
		TeacherName: "李老师",
		RangeStart:  "2026-08-17",
		RangeEnd:    "2026-09-13",
		Days:        28,
		ActiveDays:  11,
		Minutes:     380,
		Turns:       64,
		Readings: []liteparent.Item{
			{Kind: "reading", Title: "咖啡的旅程", FinishedAt: "2026-08-22"},
			{Kind: "reading", Title: "中国是否让地球变得更可持续？", FinishedAt: "2026-09-05"},
		},
		Writings: []liteparent.Item{
			{Kind: "writing", Title: "一场雨", FinishedAt: "2026-09-11"},
		},
		Projects: []liteparent.Item{
			{Kind: "project", Title: "校园雨水调查", FinishedAt: "2026-09-12"},
		},
		Moments: []liteparent.Moment{
			{Quote: "雨落在屋檐上像敲鼓", ItemTitle: "一场雨"},
		},
		AssignmentsTotal:  3,
		AssignmentsOnTime: 2,
		AssignmentsLate:   1,
		AssignmentsMissed: 0,
		Keywords: []liteparent.Keyword{
			{Text: "天气", Field: "science", FieldLabel: "科学与自然"},
			{Text: "金融", Field: "society", FieldLabel: "社会与世界"},
		},
	}
}

var (
	liveDigitRun = regexp.MustCompile(`[0-9]+`)
	// A Chinese numeral followed by a counter word: 三篇, 十一天, 两份.
	liveChineseCount = regexp.MustCompile(`[零一二两三四五六七八九十百千]+(天|篇|份|个|轮|分钟|小时|周|条|次)`)
	liveQuoteSpan    = regexp.MustCompile(`「([^「」]*)」`)
	liveTitleSpan    = regexp.MustCompile(`《[^《》]*》`)
)

// liveParentObservations lists what a reviewer needs to know about one
// section beyond pass/fail. Titles and quotes are removed before looking for
// Chinese numerals, so 《一场雨》 and 「…」 do not count.
func liveParentObservations(f liteparent.Facts, text string) []string {
	var out []string
	if stripListMarkers(text) != text {
		out = append(out, "numbered lines (markers stripped for the check)")
	}
	plain := liveTitleSpan.ReplaceAllString(liveQuoteSpan.ReplaceAllString(text, ""), "")
	if m := liveChineseCount.FindAllString(plain, -1); len(m) > 0 {
		out = append(out, "Chinese numerals: "+strings.Join(m, ", "))
	}
	allowed := map[string]bool{}
	for _, d := range liveDigitRun.FindAllString(liteparent.FactsText(f), -1) {
		allowed[d] = true
	}
	var invented []string
	for _, d := range liveDigitRun.FindAllString(stripListMarkers(text), -1) {
		if !allowed[d] {
			invented = append(invented, d)
		}
	}
	if len(invented) > 0 {
		out = append(out, "digits not in facts (arithmetic or invented): "+strings.Join(invented, ", "))
	}
	if strings.Contains(plain, "小时") {
		out = append(out, "unit conversion: 小时")
	}
	titles := strings.Split(liteparent.TitleCorpus(f), "\n")
	for _, m := range liveQuoteSpan.FindAllStringSubmatch(text, -1) {
		for _, title := range titles {
			if title != "" && m[1] == title {
				out = append(out, "title in 「」: "+m[0])
			}
		}
	}
	return out
}

func TestLiveLiteParentReport(t *testing.T) {
	prov, resolved := liveLiteWeeklyModel(t)
	f := liveLiteParentFacts()
	others := []string{"王小明"}
	t.Logf("model: %s; sections: %v", resolved.Model, liteparent.SectionsWithFacts(f))
	t.Logf("facts text: %s", liteparent.FactsText(f))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	sections, attempts, err := ComposeLiteParentReport(ctx, prov, resolved, f, others)
	logAttempts(t, attempts)
	for i, at := range attempts {
		parsed, perr := parseLiteWeeklyJSON[map[string]string](at.Text)
		if perr != nil {
			t.Logf("attempt %d observations: not parseable (%v)", i+1, perr)
			continue
		}
		for _, k := range liteparent.SectionsWithFacts(f) {
			if obs := liveParentObservations(f, parsed[k]); len(obs) > 0 {
				t.Logf("attempt %d %s: %s", i+1, k, strings.Join(obs, "; "))
			}
		}
	}
	firstPass := len(attempts) > 0 && attempts[0].Err == nil
	t.Logf("first attempt passed: %v", firstPass)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	for _, k := range liteparent.SectionsWithFacts(f) {
		t.Logf("%s (%s):\n%s", liteparent.SectionLabels[k], k, sections[k])
	}
}
