package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/gateway"
)

// publicParentJSON is the parent page payload. It is decoded loosely so a
// stray key shows up in the raw-body assertions, not here.
type publicParentJSON struct {
	ID          string            `json:"id"`
	StudentName string            `json:"studentName"`
	ClassName   string            `json:"className"`
	TeacherName string            `json:"teacherName"`
	RangeStart  string            `json:"rangeStart"`
	RangeEnd    string            `json:"rangeEnd"`
	PublishedAt string            `json:"publishedAt"`
	Facts       json.RawMessage   `json:"facts"`
	Sections    []string          `json:"sections"`
	Body        map[string]string `json:"body"`
}

type publicParentResp struct {
	Report publicParentJSON `json:"report"`
}

func publicParentPath(token string) string { return "/api/v1/public/parent-reports/" + token }

func studentParentPath(id string) string { return "/api/v1/lite/parent-reports/" + id }

// getPublicParent sends the request with no cookie at all: the parent page has
// no session.
func getPublicParent(t *testing.T, h http.Handler, token string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", publicParentPath(token), nil))
	return rec
}

// publishParent publishes a report and returns its share token.
func publishParent(t *testing.T, h http.Handler, teacher *http.Cookie, id string) string {
	t.Helper()
	var pub parentReportResp
	if code, body := parentDo(t, h, teacher, "POST", parentReportPath(id)+"/publish", "", &pub); code != http.StatusOK || pub.Report.ShareToken == nil {
		t.Fatalf("publish = %d %s", code, body)
	}
	return *pub.Report.ShareToken
}

func mdOf(t *testing.T, date string) string {
	t.Helper()
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%d月%d日", int(d.Month()), d.Day())
}

// inboxRaw returns the inbox's unread count and each item as a raw key map, so
// tests can check key sets as well as values.
func inboxRaw(t *testing.T, h http.Handler, c *http.Cookie) (int, []map[string]any) {
	t.Helper()
	var out struct {
		Items  []map[string]any `json:"items"`
		Unread int              `json:"unread"`
	}
	if code := getJSON(t, h, c, "/api/v1/lite/inbox", &out); code != http.StatusOK {
		t.Fatalf("inbox = %d", code)
	}
	return out.Unread, out.Items
}

func inboxOrder(items []map[string]any) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, fmt.Sprintf("%v:%v", it["type"], it["unread"]))
	}
	return out
}

// TestLiteParentPublicPage: the parent page by share token, with no session.
// The payload has the noindex header and no id or token anywhere; a revoked
// token is 404, and re-publishing mints a new token while the old one stays
// 404.
func TestLiteParentPublicPage(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(parentValidReply))
	h, _, teacher, classID, studentID := parentFixture(t, prov)
	rep := generateParentReport(t, h, teacher, classID, studentID).Report
	token := publishParent(t, h, teacher, rep.ID)

	rec := getPublicParent(t, h, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("public GET = %d %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("X-Robots-Tag"); got != "noindex, nofollow, noarchive" {
		t.Fatalf("X-Robots-Tag = %q", got)
	}
	raw := rec.Body.String()
	for _, banned := range []string{`"id"`, `"shareToken"`, token, rep.ID, studentID.String(), `"draft"`, `"status"`} {
		if strings.Contains(raw, banned) {
			t.Fatalf("public payload contains %q: %s", banned, raw)
		}
	}
	var got publicParentResp
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	r := got.Report
	want := parentSectionsOf(t, parentValidReply)
	if r.StudentName != "林知遥" || r.ClassName != "Lite Class" || r.TeacherName != "T lt-teacher@demo.local" ||
		r.RangeStart != rep.RangeStart || r.RangeEnd != rep.RangeEnd || r.PublishedAt == "" ||
		!reflect.DeepEqual(r.Sections, []string{"overview", "reading", "next"}) || !reflect.DeepEqual(r.Body, want) {
		t.Fatalf("public report = %+v", r)
	}
	if !strings.Contains(string(r.Facts), "城市里的雨水花园") || !strings.Contains(string(r.Facts), "雨水不是废水") {
		t.Fatalf("public facts = %s", r.Facts)
	}

	if rec := getPublicParent(t, h, "0123456789abcdef0123456789abcdef"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown token = %d", rec.Code)
	}

	if code, body := parentDo(t, h, teacher, "DELETE", parentReportPath(rep.ID)+"/share", "", nil); code != http.StatusOK {
		t.Fatalf("revoke = %d %s", code, body)
	}
	if rec := getPublicParent(t, h, token); rec.Code != http.StatusNotFound {
		t.Fatalf("revoked token = %d %s, want 404", rec.Code, rec.Body)
	}

	fresh := publishParent(t, h, teacher, rep.ID)
	if fresh == token {
		t.Fatalf("re-publish after revoke kept the old token")
	}
	if rec := getPublicParent(t, h, fresh); rec.Code != http.StatusOK {
		t.Fatalf("new token = %d %s", rec.Code, rec.Body)
	}
	if rec := getPublicParent(t, h, token); rec.Code != http.StatusNotFound {
		t.Fatalf("old token after re-publish = %d, want 404", rec.Code)
	}
}

// TestLiteParentStudentRead: she reads her own published report (with its id,
// without the token or the draft); another student, a teacher and her own
// draft are 404; after the link is revoked she can still read it.
func TestLiteParentStudentRead(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(parentValidReply))
	h, pool, teacher, classID, studentID := parentFixture(t, prov)
	student := signInAs(t, pool, studentID)
	other := createStudent(t, pool, SeedSchoolID, "pr-other@demo.local")
	enrollStudent(t, pool, other, classID)
	otherCookie := signInAs(t, pool, other)

	rep := generateParentReport(t, h, teacher, classID, studentID).Report
	path := studentParentPath(rep.ID)

	if code, body := parentDo(t, h, student, "GET", path, "", nil); code != http.StatusNotFound {
		t.Fatalf("own draft = %d %s, want 404", code, body)
	}
	if code, body := parentDo(t, h, student, "POST", path+"/seen", "", nil); code != http.StatusNotFound {
		t.Fatalf("seen on own draft = %d %s, want 404", code, body)
	}

	token := publishParent(t, h, teacher, rep.ID)

	var got publicParentResp
	code, raw := parentDo(t, h, student, "GET", path, "", &got)
	if code != http.StatusOK {
		t.Fatalf("own published = %d %s", code, raw)
	}
	for _, banned := range []string{`"shareToken"`, token, `"draft"`} {
		if strings.Contains(raw, banned) {
			t.Fatalf("student payload contains %q: %s", banned, raw)
		}
	}
	if got.Report.ID != rep.ID || got.Report.StudentName != "林知遥" || got.Report.TeacherName != "T lt-teacher@demo.local" ||
		!reflect.DeepEqual(got.Report.Body, parentSectionsOf(t, parentValidReply)) {
		t.Fatalf("student report = %+v", got.Report)
	}

	for _, c := range []struct {
		what   string
		cookie *http.Cookie
		path   string
	}{
		{"another student", otherCookie, path},
		{"the teacher", teacher, path},
		{"a malformed id", student, studentParentPath("not-a-uuid")},
		{"an unknown id", student, studentParentPath(uuid.NewString())},
	} {
		if code, body := parentDo(t, h, c.cookie, "GET", c.path, "", nil); code != http.StatusNotFound {
			t.Fatalf("GET by %s = %d %s, want 404", c.what, code, body)
		}
	}
	if code, body := parentDo(t, h, otherCookie, "POST", path+"/seen", "", nil); code != http.StatusNotFound {
		t.Fatalf("seen by another student = %d %s, want 404", code, body)
	}
	if n := weeklyCount(t, pool, `SELECT count(*) FROM lite_parent_report WHERE id = $1 AND student_seen_at IS NULL`, uuid.MustParse(rep.ID)); n != 1 {
		t.Fatalf("another student's seen must not mark the report")
	}

	if code, body := parentDo(t, h, teacher, "DELETE", parentReportPath(rep.ID)+"/share", "", nil); code != http.StatusOK {
		t.Fatalf("revoke = %d %s", code, body)
	}
	if code, body := parentDo(t, h, student, "GET", path, "", nil); code != http.StatusOK {
		t.Fatalf("own report after revoke = %d %s, want 200", code, body)
	}
}

// TestLiteParentInbox: a published report joins the inbox next to an
// assignment. Unread first, then assignments by due date, then reports by
// published_at DESC; unread counts both kinds; seen clears the report. The
// assignment item keeps plan 2's key set exactly.
func TestLiteParentInbox(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(parentValidReply))
	h, pool, teacher, classID, studentID := parentFixture(t, prov)
	student := signInAs(t, pool, studentID)
	aid := createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))

	older := generateParentReport(t, h, teacher, classID, studentID).Report
	newer := generateParentReport(t, h, teacher, classID, studentID).Report

	// A draft is not in her inbox.
	if unread, items := inboxRaw(t, h, student); unread != 1 || len(items) != 1 || items[0]["type"] != "assignment" {
		t.Fatalf("inbox before publish = %d %+v", unread, items)
	}

	publishParent(t, h, teacher, older.ID)
	mustExec(t, pool, `UPDATE lite_parent_report SET published_at = now() - interval '1 hour' WHERE id = $1`, uuid.MustParse(older.ID))
	publishParent(t, h, teacher, newer.ID)

	unread, items := inboxRaw(t, h, student)
	if unread != 3 || len(items) != 3 {
		t.Fatalf("inbox = %d %+v", unread, items)
	}
	if items[0]["id"] != aid || items[1]["id"] != newer.ID || items[2]["id"] != older.ID {
		t.Fatalf("inbox order = %v %v %v, want assignment, newer, older", items[0]["id"], items[1]["id"], items[2]["id"])
	}

	// Plan 2's assignment key set, unchanged: an unstarted assignment with no
	// instructions still carries "instructions" and "atomId".
	wantAssignKeys := []string{"atomId", "className", "dueAt", "id", "instructions", "kind", "status", "statusLabel", "title", "type", "unread"}
	if got := keysOf(items[0]); !reflect.DeepEqual(got, wantAssignKeys) {
		t.Fatalf("assignment keys = %v, want %v", got, wantAssignKeys)
	}
	if items[0]["instructions"] != "" || items[0]["atomId"] != nil || items[0]["kind"] != "writing" || items[0]["status"] != "not_started" {
		t.Fatalf("assignment item = %+v", items[0])
	}

	wantReportKeys := []string{"className", "id", "publishedAt", "title", "type", "unread"}
	rep := items[1]
	if got := keysOf(rep); !reflect.DeepEqual(got, wantReportKeys) {
		t.Fatalf("report keys = %v, want %v", got, wantReportKeys)
	}
	wantTitle := "家长报告（" + mdOf(t, newer.RangeStart) + "–" + mdOf(t, newer.RangeEnd) + "）"
	if rep["type"] != "parent_report" || rep["title"] != wantTitle || rep["className"] != "Lite Class" || rep["unread"] != true || rep["publishedAt"] == "" {
		t.Fatalf("report item = %+v, want title %s", rep, wantTitle)
	}

	// Unread first: once the assignment is read, both reports come before it.
	if code := assignJSON(t, h, student, "POST", "/api/v1/lite/assignments/"+aid+"/seen", nil, nil); code != http.StatusNoContent {
		t.Fatalf("assignment seen = %d", code)
	}
	unread, items = inboxRaw(t, h, student)
	if unread != 2 || !reflect.DeepEqual(inboxOrder(items), []string{"parent_report:true", "parent_report:true", "assignment:false"}) {
		t.Fatalf("inbox after assignment seen = %d %v", unread, inboxOrder(items))
	}

	if code, body := parentDo(t, h, student, "POST", studentParentPath(newer.ID)+"/seen", "", nil); code != http.StatusNoContent {
		t.Fatalf("report seen = %d %s", code, body)
	}
	// Seen twice is still 204 and keeps the first time.
	if code, body := parentDo(t, h, student, "POST", studentParentPath(newer.ID)+"/seen", "", nil); code != http.StatusNoContent {
		t.Fatalf("report seen again = %d %s", code, body)
	}
	unread, items = inboxRaw(t, h, student)
	if unread != 1 || items[0]["id"] != older.ID || items[0]["unread"] != true ||
		items[1]["id"] != aid || items[2]["id"] != newer.ID || items[2]["unread"] != false {
		t.Fatalf("inbox after report seen = %d %v", unread, inboxOrder(items))
	}

	parentDo(t, h, student, "POST", studentParentPath(older.ID)+"/seen", "", nil)
	unread, items = inboxRaw(t, h, student)
	if unread != 0 || items[0]["id"] != aid || items[1]["id"] != newer.ID || items[2]["id"] != older.ID {
		t.Fatalf("inbox all read = %d %v", unread, inboxOrder(items))
	}
}

// TestLiteParentReportAfterLeavingClass (plan 4 Ruling 4): after she leaves
// the class, its assignments leave her inbox but her published report stays,
// and she can still open it and mark it seen.
func TestLiteParentReportAfterLeavingClass(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(weeklyReply(parentValidReply))
	h, pool, teacher, classID, studentID := parentFixture(t, prov)
	student := signInAs(t, pool, studentID)
	createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	rep := generateParentReport(t, h, teacher, classID, studentID).Report
	publishParent(t, h, teacher, rep.ID)

	mustExec(t, pool, `DELETE FROM enrollments WHERE user_id = $1 AND class_id = $2`, studentID, classID)

	unread, items := inboxRaw(t, h, student)
	if unread != 1 || len(items) != 1 || items[0]["type"] != "parent_report" || items[0]["id"] != rep.ID {
		t.Fatalf("inbox after leaving = %d %+v, want only her report", unread, items)
	}
	if code, body := parentDo(t, h, student, "GET", studentParentPath(rep.ID), "", nil); code != http.StatusOK {
		t.Fatalf("GET after leaving = %d %s", code, body)
	}
	if code, body := parentDo(t, h, student, "POST", studentParentPath(rep.ID)+"/seen", "", nil); code != http.StatusNoContent {
		t.Fatalf("seen after leaving = %d %s", code, body)
	}
	if unread, _ := inboxRaw(t, h, student); unread != 0 {
		t.Fatalf("unread after seen = %d", unread)
	}
}
