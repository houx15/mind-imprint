package teacher

import (
	"fmt"
	"sort"
	"strings"

	"mindimprint/api/internal/agent"
)

// StudentWeek is one student's week as the rules see it: usage facts for this
// window and the same elapsed offset last week, plus their two most recent
// canonical reports. Latest/Previous are nil when the student has no report —
// no report, no judgment (敢于空白).
type StudentWeek struct {
	UserID, DisplayName, AvatarColor string
	ActiveDays, Turns                int
	PrevActiveDays, PrevTurns        int
	ReportsThisWeek                  int
	Latest, Previous                 *agent.Report
	LatestSurface, LatestScopeID     string
}

// Card is one 值得表扬 / 需要建议 card. Evidence is ALWAYS deterministic —
// excerpted verbatim from the report or a bare statement of fact. Lead and
// action (the wording) are added later by the composer and are not fields here:
// the rule layer never produces prose.
type Card struct {
	UserID, DisplayName, AvatarColor string
	TagCode, TagLabel, Kind          string
	Evidence                         string
	HasReport                        bool
	ReportSurface, ReportScopeID     string
}

type Bucket struct {
	Code, Label string
	Count       int
}

type BucketChange struct{ Name, From, To string }

type DepthDist struct {
	Buckets    []Bucket
	RatedCount int
}

type AutonomyAgg struct {
	Mean, Delta, DeltaDir string
	RatedCount            int
}

type Weekly struct {
	Praise, Watch []Card
	Depth         DepthDist
	Autonomy      AutonomyAgg
	BucketChanges []BucketChange
}

var bucketLabels = []Bucket{
	{Code: "L1", Label: "起步 L1"},
	{Code: "L2", Label: "发展 L2"},
	{Code: "L3", Label: "熟练 L3"},
	{Code: "L4", Label: "优秀 L4"},
}

// Detect is the whole judgment layer. Rules decide who appears, with which tag,
// and which evidence is quoted; the model that writes the wording never sees
// anything this function did not produce.
func Detect(students []StudentWeek) Weekly {
	var w Weekly
	counts := map[string]int{}

	for _, s := range students {
		if c, ok := watchCard(s); ok {
			w.Watch = append(w.Watch, c)
		} else if c, ok := praiseCard(s); ok {
			w.Praise = append(w.Praise, c)
		}
		if s.Latest == nil {
			continue
		}
		if _, max, ok := DLevels(*s.Latest); ok {
			w.Depth.RatedCount++
			counts[fmt.Sprintf("L%d", max)]++
		}
		if s.Previous != nil {
			if from, to, ok := bucketChange(*s.Previous, *s.Latest); ok {
				w.BucketChanges = append(w.BucketChanges, BucketChange{Name: s.DisplayName, From: from, To: to})
			}
		}
	}

	w.Depth.Buckets = make([]Bucket, 0, len(bucketLabels))
	for _, b := range bucketLabels {
		b.Count = counts[b.Code]
		w.Depth.Buckets = append(w.Depth.Buckets, b)
	}
	w.Autonomy = autonomyAgg(students)
	return w
}

// watchCard applies the watch rules in severity order; the first match wins and
// a student never carries more than one card.
func watchCard(s StudentWeek) (Card, bool) {
	mk := func(code, label, evidence string) (Card, bool) {
		return Card{
			UserID: s.UserID, DisplayName: s.DisplayName, AvatarColor: s.AvatarColor,
			TagCode: code, TagLabel: label, Kind: "watch", Evidence: evidence,
			HasReport: s.Latest != nil, ReportSurface: s.LatestSurface, ReportScopeID: s.LatestScopeID,
		}, true
	}
	if s.ActiveDays == 0 {
		return mk("never_used", "本周未使用",
			fmt.Sprintf("本周 0 天活动记录；上周 %d 天。", s.PrevActiveDays))
	}
	if s.ActiveDays <= s.PrevActiveDays-2 && s.ReportsThisWeek == 0 {
		return mk("dropped_off", "本周掉线",
			fmt.Sprintf("活跃天数 上周 %d 天 → 本周 %d 天，本周无新生成报告。", s.PrevActiveDays, s.ActiveDays))
	}
	if s.Latest == nil {
		return Card{}, false
	}
	if mean, ok := AMean(*s.Latest); ok && mean <= 1.0 {
		return mk("outsourced_judgment", "判断在外包",
			join(fmt.Sprintf("A 轴 %.1f/5。", mean), lowestSuppliedEvidence(*s.Latest)))
	}
	if sig, ok := signal(*s.Latest, "A3"); ok && sig.Level == 0 && sig.Opportunity != "not_supplied" {
		return mk("no_boundaries", "从不设界", sig.Evidence)
	}
	if stuckAtStart(*s.Latest) {
		return mk("stuck_at_start", "停在起步档", lowestDepthEvidence(*s.Latest))
	}
	return Card{}, false
}

func praiseCard(s StudentWeek) (Card, bool) {
	if s.Latest == nil || s.Previous == nil {
		return Card{}, false
	}
	mk := func(code, label, evidence string) (Card, bool) {
		return Card{
			UserID: s.UserID, DisplayName: s.DisplayName, AvatarColor: s.AvatarColor,
			TagCode: code, TagLabel: label, Kind: "praise", Evidence: evidence,
			HasReport: true, ReportSurface: s.LatestSurface, ReportScopeID: s.LatestScopeID,
		}, true
	}
	_, prevMax, prevOK := DLevels(*s.Previous)
	_, curMax, curOK := DLevels(*s.Latest)
	if prevOK && curOK && curMax > prevMax {
		return mk("depth_up", "深度升档",
			join(fmt.Sprintf("%s → %s。", DBadge(*s.Previous), DBadge(*s.Latest)), risenDepthEvidence(*s.Previous, *s.Latest)))
	}
	prevMean, pOK := AMean(*s.Previous)
	curMean, cOK := AMean(*s.Latest)
	if pOK && cOK && curMean-prevMean >= 0.5 {
		return mk("more_autonomous", "更愿意自己想",
			join(fmt.Sprintf("A 轴 %.1f → %.1f。", prevMean, curMean), risenSignalEvidence(*s.Previous, *s.Latest)))
	}
	return Card{}, false
}

// join appends the quoted evidence when there is one. An empty source evidence
// leaves the factual half standing alone — never a fabricated substitute.
func join(fact, quoted string) string {
	if quoted == "" {
		return fact
	}
	return fact + quoted
}

func signal(r agent.Report, code string) (agent.AutonomySignal, bool) {
	for _, a := range r.AutonomyAxis {
		if a.Code == code {
			return a, true
		}
	}
	return agent.AutonomySignal{}, false
}

func lowestSuppliedEvidence(r agent.Report) string {
	best, found := agent.AutonomySignal{}, false
	for _, a := range r.AutonomyAxis {
		if a.Opportunity == "not_supplied" {
			continue
		}
		if !found || a.Level < best.Level {
			best, found = a, true
		}
	}
	if !found {
		return ""
	}
	return best.Evidence
}

func lowestDepthEvidence(r agent.Report) string {
	best, bestRank, found := agent.DepthDim{}, 0, false
	for _, d := range r.DepthAxis {
		rank, ok := depthRank[d.Level]
		if !ok {
			continue
		}
		if !found || rank < bestRank {
			best, bestRank, found = d, rank, true
		}
	}
	if !found {
		return ""
	}
	return best.Evidence
}

// stuckAtStart: every rated dim at L2 or below, and at least half of them at L1.
func stuckAtStart(r agent.Report) bool {
	rated, ones := 0, 0
	for _, d := range r.DepthAxis {
		rank, ok := depthRank[d.Level]
		if !ok {
			continue
		}
		rated++
		if rank > 2 {
			return false
		}
		if rank == 1 {
			ones++
		}
	}
	return rated > 0 && ones*2 >= rated
}

func risenDepthEvidence(prev, cur agent.Report) string {
	prevByCode := map[string]int{}
	for _, d := range prev.DepthAxis {
		if rank, ok := depthRank[d.Level]; ok {
			prevByCode[d.Code] = rank
		}
	}
	for _, d := range cur.DepthAxis {
		rank, ok := depthRank[d.Level]
		if ok && rank > prevByCode[d.Code] {
			return d.Evidence
		}
	}
	return ""
}

func risenSignalEvidence(prev, cur agent.Report) string {
	prevByCode := map[string]int{}
	for _, a := range prev.AutonomyAxis {
		prevByCode[a.Code] = a.Level
	}
	best, bestGain, found := agent.AutonomySignal{}, 0, false
	for _, a := range cur.AutonomyAxis {
		if a.Opportunity == "not_supplied" {
			continue
		}
		if gain := a.Level - prevByCode[a.Code]; gain > bestGain {
			best, bestGain, found = a, gain, true
		}
	}
	if !found {
		return ""
	}
	return best.Evidence
}

func bucketOf(r agent.Report) (string, bool) {
	_, max, ok := DLevels(r)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("L%d", max), true
}

func bucketChange(prev, cur agent.Report) (from, to string, ok bool) {
	f, fOK := bucketOf(prev)
	t, tOK := bucketOf(cur)
	if !fOK || !tOK || f == t {
		return "", "", false
	}
	return f, t, true
}

// autonomyAgg averages each rated student's own A mean, and averages the
// per-student change for students holding a second report. Derived in Go and
// ONLY in Go — the web renders these strings, it never recomputes them.
func autonomyAgg(students []StudentWeek) AutonomyAgg {
	sum, n := 0.0, 0
	deltaSum, deltaN := 0.0, 0
	for _, s := range students {
		if s.Latest == nil {
			continue
		}
		cur, ok := AMean(*s.Latest)
		if !ok {
			continue
		}
		sum += cur
		n++
		if s.Previous == nil {
			continue
		}
		if prev, pOK := AMean(*s.Previous); pOK {
			deltaSum += cur - prev
			deltaN++
		}
	}
	agg := AutonomyAgg{Mean: "—", Delta: "—", DeltaDir: "flat", RatedCount: n} // U+2014
	if n > 0 {
		agg.Mean = fmt.Sprintf("%.1f", sum/float64(n))
	}
	if deltaN > 0 {
		agg.Delta = fmt.Sprintf("%+.1f", deltaSum/float64(deltaN))
		agg.DeltaDir = autonomyDeltaDir(agg.Delta)
	}
	return agg
}

// autonomyDeltaDir derives the pill direction from the very string the
// teacher reads (the %+.1f-formatted Delta), never from the raw unrounded
// change — a raw +0.04 renders as "+0.0" and must read "flat", not "up",
// so the pill and the number can never disagree.
func autonomyDeltaDir(rendered string) string {
	if strings.TrimLeft(rendered, "+-") == "0.0" {
		return "flat"
	}
	if strings.HasPrefix(rendered, "-") {
		return "down"
	}
	return "up"
}

// ClassWeekCounts is one window's class-wide counters.
type ClassWeekCounts struct{ ActiveStudents, Reports, Turns, CourseSteps int }

// Stat is one of the four cards at the top of the weekly report.
type Stat struct {
	Key, Label string
	Value      int
	Unit, Foot string
	Delta      string
	DeltaDir   string
}

// Stats renders the four cards in the binding design's order. An unchanged
// metric renders ±0 as flat — deliberately NOT the design prototype's green
// up-arrow, which would be a false signal on a screen whose whole value is that
// a teacher can trust the arrows.
func Stats(cur, prev ClassWeekCounts, classSize int) []Stat {
	mk := func(key, label string, value, prior int, unit, foot string) Stat {
		d := value - prior
		s := Stat{Key: key, Label: label, Value: value, Unit: unit, Foot: foot}
		switch {
		case d > 0:
			s.Delta, s.DeltaDir = fmt.Sprintf("+%d", d), "up"
		case d < 0:
			s.Delta, s.DeltaDir = fmt.Sprintf("%d", d), "down"
		default:
			s.Delta, s.DeltaDir = "±0", "flat"
		}
		return s
	}
	return []Stat{
		mk("active_students", "本周活跃学生", cur.ActiveStudents, prev.ActiveStudents, fmt.Sprintf("/ %d 人", classSize), "登录并有活动的学生"),
		mk("reports", "生成能力报告", cur.Reports, prev.Reports, "份", "来自项目、对话与课程"),
		mk("turns", "AI 对话轮次", cur.Turns, prev.Turns, "轮", "反映本周使用强度"),
		mk("course_steps", "完成课程节", cur.CourseSteps, prev.CourseSteps, "节", "平台内自学课程"),
	}
}

// BuildWeeklyFacts is the ONLY thing the composer sees. It deliberately omits
// the class-level usage counters: those tick continuously, the prose is written
// once and never refreshed (DEC-2), so prose citing them would be wrong by
// Wednesday. A card's own evidence line may contain its own numbers.
func BuildWeeklyFacts(className string, classSize int, weekLabel string, w Weekly) agent.WeeklyFacts {
	facts := agent.WeeklyFacts{
		ClassName: className, ClassSize: classSize, WeekLabel: weekLabel,
		DepthBuckets: map[string]int{}, RatedCount: w.Depth.RatedCount,
		AutonomyMean: w.Autonomy.Mean, AutonomyDelta: w.Autonomy.Delta,
	}
	for _, b := range w.Depth.Buckets {
		facts.DepthBuckets[b.Label] = b.Count
	}
	for _, c := range w.BucketChanges {
		facts.BucketChanges = append(facts.BucketChanges, agent.WeeklyBucketChange{Name: c.Name, From: c.From, To: c.To})
	}
	for _, c := range append(append([]Card{}, w.Praise...), w.Watch...) {
		facts.Cards = append(facts.Cards, agent.WeeklyFactCard{
			UserID: c.UserID, Name: c.DisplayName, Kind: c.Kind,
			TagCode: c.TagCode, TagLabel: c.TagLabel, Evidence: c.Evidence,
		})
	}
	sort.SliceStable(facts.Cards, func(i, j int) bool { return facts.Cards[i].Kind < facts.Cards[j].Kind })
	return facts
}
