package api_test

// Scenario 7: the four owner findings of 2026-09-17, against the real model.
//
//   - 他/她 with no gender on record (class summary, class chat, parent report
//     draft and revision);
//   - the class chat saying a page was opened;
//   - the report chat offering to add a section.
//
//	.superpowers/tmp/run-live.sh TestLiveWorkspacePronounsAndClaims <outfile>
//
// LIVE_R3_RUNS overrides the number of runs (default 3). Pronoun use is a
// soft verdict: the server does not check it, and the numbers here are what
// decides whether it should.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/liteweek"
	"mindimprint/api/internal/liteworkspace"
	"mindimprint/api/internal/store/sqlc"
)

// wsLiveNotSingular are words that contain 他 or 她 without being a pronoun
// for one student.
var wsLiveNotSingular = regexp.MustCompile(`其他|其她|他们|她们|他人|吉他`)

// wsLiveSingularPronouns returns every 他 and 她 in s that can refer to one
// person, with a few runes around each.
func wsLiveSingularPronouns(s string) []string {
	r := []rune(wsLiveNotSingular.ReplaceAllString(s, "＿＿"))
	var out []string
	for i, c := range r {
		if c != '他' && c != '她' {
			continue
		}
		lo, hi := max(0, i-6), min(len(r), i+6)
		out = append(out, string(r[lo:hi]))
	}
	return out
}

func TestWsLiveSingularPronouns(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int
	}{
		{"周子涵这周没有登录，她的作业还没开始。", 1},
		{"其他同学都已开始，他们的进度正常。", 0},
		{"他和她", 2},
		{"请点击下方按钮前往。", 0},
	} {
		if got := len(wsLiveSingularPronouns(tc.in)); got != tc.want {
			t.Errorf("wsLiveSingularPronouns(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// wsLiveRewrites counts the calls that were sent the server's false-claim
// rewrite request.
func wsLiveRewrites(calls []wsLiveCall) int {
	n := 0
	for _, c := range calls {
		if strings.Contains(c.Prompt, "上一条回复没有通过检查") {
			n++
		}
	}
	return n
}

func wsLiveR3Runs() int {
	if n, err := strconv.Atoi(os.Getenv("LIVE_R3_RUNS")); err == nil && n > 0 {
		return n
	}
	return 3
}

func wsLiveChoiceText(out wsLiveTurn) string {
	parts := []string{out.Reply}
	for _, c := range out.Choices {
		parts = append(parts, c.Label)
	}
	return strings.Join(parts, "\n")
}

func TestLiveWorkspacePronounsAndClaims(t *testing.T) {
	prov, route, resolved := liveWorkspaceModel(t)
	t.Logf("class %s → provider=%s model=%s", gateway.ClassDialogue, resolved.Provider, resolved.Model)
	for run := 1; run <= wsLiveR3Runs(); run++ {
		t.Run(fmt.Sprintf("home-run-%d", run), func(t *testing.T) { liveR3HomeRun(t, prov, route, run) })
		t.Run(fmt.Sprintf("report-run-%d", run), func(t *testing.T) { liveR3ReportRun(t, prov, route, run) })
	}
}

func liveR3HomeRun(t *testing.T, prov gateway.Provider, route func(string) gateway.KeyResolver, run int) {
	const sc = "7-home"
	rec := &wsLiveRecorder{inner: prov}
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool, Provider: rec, Route: route, SpecByID: cards.ByID,
	}).Handler()
	mustExec(t, pool, `UPDATE schools SET edition = 'lite'`)
	teacher := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "r3-teacher@demo.local"))
	classID := createClassViaAPI(t, h, teacher, "同事试用班")
	today := liteweek.Day(time.Now())
	for _, s := range liveProdClassActive {
		id := createStudent(t, pool, SeedSchoolID, s.email)
		enrollStudent(t, pool, id, classID)
		renameLiteStudent(t, pool, id, s.name)
		for i := 0; i < s.writings; i++ {
			atom := seedLiteWritingForUser(t, pool, id, "finished")
			seedBucket(t, pool, atom, today, 600)
		}
	}
	for _, s := range liveProdClassQuiet {
		id := createStudent(t, pool, SeedSchoolID, s.email)
		enrollStudent(t, pool, id, classID)
		renameLiteStudent(t, pool, id, s.name)
	}
	backdateWeeklyStart(t, pool, classID)

	// The class summary, no gender set.
	before := rec.mark()
	res := doJSON(t, h, teacher, "POST", summaryPath(classID), "")
	sumCalls := rec.since(before)
	var sum classSummaryJSON
	_ = json.Unmarshal(res.Body.Bytes(), &sum)
	wsLiveVerdict(t, sc, run, "summary HTTP 200", res.Code == http.StatusOK, false, res.Body.String())
	pro := wsLiveSingularPronouns(sum.Summary)
	wsLiveVerdict(t, sc, run, "summary uses no 他/她 (gender unset)", len(pro) == 0, true,
		fmt.Sprintf("%v in %q (%d calls)", pro, sum.Summary, len(sumCalls)))

	var turns []liteworkspace.Turn
	for i, text := range []string{"这周谁还没开始学习？", "带我去看周报", "打开周子涵的学习页"} {
		code, body, out, calls := wsLivePost(t, h, teacher, rec, map[string]any{
			"surface": "home", "classId": classID, "text": text, "turns": turns,
		})
		name := fmt.Sprintf("turn %d", i+1)
		wsLiveVerdict(t, sc, run, name+" HTTP 200", code == http.StatusOK, false, body)
		shown := wsLiveChoiceText(out)
		wsLiveVerdict(t, sc, run, name+" no opened-page claim shown", !liteworkspace.ClaimsOpenedPage(shown), false, shown)
		wsLiveVerdict(t, sc, run, name+" no rewrite needed (observation)", wsLiveRewrites(calls) == 0, true,
			fmt.Sprintf("%d rewrites; texts %q", wsLiveRewrites(calls), wsLiveTexts(calls)))
		pro := wsLiveSingularPronouns(shown)
		wsLiveVerdict(t, sc, run, name+" uses no 他/她 (gender unset)", len(pro) == 0, true, fmt.Sprintf("%v in %q", pro, shown))
		if i > 0 {
			wsLiveVerdict(t, sc, run, name+" navigate offered", out.Navigate != nil, false, body)
		}
		turns = append(turns, liteworkspace.Turn{Role: "teacher", Text: text})
		if code == http.StatusOK {
			turns = append(turns, liteworkspace.Turn{Role: "ai", Text: out.Reply})
		}
	}
}

func liveR3ReportRun(t *testing.T, prov gateway.Provider, route func(string) gateway.KeyResolver, run int) {
	const sc = "7-report"
	rec := &wsLiveRecorder{inner: prov}
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool, Provider: rec, Route: route, SpecByID: cards.ByID,
	}).Handler()
	mustExec(t, pool, `UPDATE schools SET edition = 'lite'`)
	teacher := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "r3-pr-teacher@demo.local"))
	classID := createClassViaAPI(t, h, teacher, "高一（2）班 · 阅读写作")
	student := createStudent(t, pool, SeedSchoolID, "r3-pr-student@demo.local")
	enrollStudent(t, pool, student, classID)
	renameLiteStudent(t, pool, student, "陈书宁")
	backdateWeeklyStart(t, pool, classID)
	today := liteweek.Day(time.Now())
	first := seedParentReading(t, pool, student, true)
	seedBucket(t, pool, first, today.AddDate(0, 0, -3), 1500)
	seedBucket(t, pool, first, today.AddDate(0, 0, -4), 900)

	var gen parentReportResp
	code, body := parentDo(t, h, teacher, "POST", parentReportsPath(classID, student), "", &gen)
	if code != http.StatusCreated {
		wsLiveVerdict(t, sc, run, "report generated (fixture)", false, false, body)
		return
	}
	rep := gen.Report
	wsLiveVerdict(t, sc, run, "draft accepted (fixture)", gen.DraftError == nil, true, fmt.Sprintf("%v", gen.DraftError))
	var draft []string
	for _, v := range rep.Body {
		draft = append(draft, v)
	}
	pro := wsLiveSingularPronouns(strings.Join(draft, "\n"))
	wsLiveVerdict(t, sc, run, "draft uses no 他/她 (gender unset)", len(pro) == 0, true, fmt.Sprintf("%v; sections %v", pro, rep.Sections))
	bodyNow := rep.Body
	if bodyNow == nil {
		bodyNow = map[string]string{}
	}

	code2, body2, out, calls := wsLivePost(t, h, teacher, rec, map[string]any{
		"surface": "parentReport", "reportId": rep.ID, "text": "新增一个写作段落",
		"artifact": map[string]any{"body": bodyNow},
	})
	shown := wsLiveChoiceText(out)
	wsLiveVerdict(t, sc, run, "add-section turn HTTP 200", code2 == http.StatusOK, false, body2)
	wsLiveVerdict(t, sc, run, "add-section: no section offer shown", !liteworkspace.OffersSectionChange(shown) &&
		liteworkspace.NamesMissingSection(shown, []string{"写作", "项目", "兴趣"}) == "", false, shown)
	wsLiveVerdict(t, sc, run, "add-section: no rewrite needed (observation)", wsLiveRewrites(calls) == 0, true,
		fmt.Sprintf("%d rewrites; texts %q", wsLiveRewrites(calls), wsLiveTexts(calls)))
	wsLiveVerdict(t, sc, run, "add-section: nothing revised", len(out.Patch) == 0, true, fmt.Sprintf("patch %v", out.Patch))

	code3, body3, out3, _ := wsLivePost(t, h, teacher, rec, map[string]any{
		"surface": "parentReport", "reportId": rep.ID, "text": "把阅读那段写得具体一点",
		"artifact": map[string]any{"body": bodyNow},
	})
	wsLiveVerdict(t, sc, run, "revise turn HTTP 200", code3 == http.StatusOK, false, body3)
	patchBody, _ := out3.Patch["body"].(map[string]any)
	reading, _ := patchBody["reading"].(string)
	pro3 := wsLiveSingularPronouns(reading + "\n" + wsLiveChoiceText(out3))
	wsLiveVerdict(t, sc, run, "revision uses no 他/她 (gender unset)", len(pro3) == 0, true, fmt.Sprintf("%v in %q", pro3, reading))
}
