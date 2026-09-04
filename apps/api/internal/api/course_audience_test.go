package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

// course_audience_test.go —— 受众标签（migration 0133）。
//
// 测的是「谁看得见哪门课」。这条规则读代码看不出对错：它是三处判断的合取
// （已发布 / 受众 / 是不是 admin），而且分布在目录和按 slug 读两条不同的路径上。
// 只测其中一条，另一条就会漏（目录里没有、手敲 URL 却打得开）。

// setCourseAudience 直接写 course.audience —— 作者端的 PUT 要求一份完整的
// definition body，而这几条测试关心的只有这一列。
func setCourseAudience(t *testing.T, pool *pgxpool.Pool, slug string, audience []string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE course SET audience = $2 WHERE slug = $1`, slug, audience); err != nil {
		t.Fatalf("set audience for %s: %v", slug, err)
	}
}

func listCourseSlugs(t *testing.T, h http.Handler, c *http.Cookie) []string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses", nil), c))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /courses = %d; body=%s", rec.Code, rec.Body)
	}
	var resp struct {
		Courses []struct {
			Slug     string   `json:"slug"`
			Audience []string `json:"audience"`
		} `json:"courses"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode /courses: %v — body=%s", err, rec.Body)
	}
	out := make([]string, 0, len(resp.Courses))
	for _, c := range resp.Courses {
		out = append(out, c.Slug)
	}
	return out
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// 默认受众是 {lite,pro}，所以既有的课两个版本都看得见。产品负责人 2026-09-04:
// 「currently all courses available in lite version and pro version」。
func TestCourseAudience_DefaultsToBothEditions(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	seedCourseWithDefinition(t, pool, "both-course", testCourseDefinitionJSON)

	if got := listCourseSlugs(t, h, cookie); !contains(got, "both-course") {
		t.Fatalf("lite student cannot see a default-audience course: %v", got)
	}
}

// 标了只给 pro 的课，lite 学生的目录里没有。
func TestCourseAudience_ProOnlyCourseHiddenFromLite(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	seedCourseWithDefinition(t, pool, "pro-course", testCourseDefinitionJSON)
	setCourseAudience(t, pool, "pro-course", []string{"pro"})

	if got := listCourseSlugs(t, h, cookie); contains(got, "pro-course") {
		t.Fatalf("pro-only course leaked into the lite catalog: %v", got)
	}
}

// 🚨 目录里藏起来还不够：按 slug 读的那几条路（详情 / 定义 / session / 报告）
// 全都得 404，否则一条手敲的 URL 就绕过了整个受众规则。
func TestCourseAudience_ProOnlyCourseIs404BySlugForLite(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	seedCourseWithDefinition(t, pool, "pro-course", testCourseDefinitionJSON)
	setCourseAudience(t, pool, "pro-course", []string{"pro"})

	for _, path := range []string{
		"/api/v1/courses/pro-course",
		"/api/v1/courses/pro-course/definition",
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", path, nil), cookie))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET %s = %d, want 404; body=%s", path, rec.Code, rec.Body)
		}
	}
}

// 空 audience = 不限受众。一次漏填不该把一门课从所有人的目录里拿掉。
func TestCourseAudience_EmptyMeansEveryone(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	seedCourseWithDefinition(t, pool, "open-course", testCourseDefinitionJSON)
	setCourseAudience(t, pool, "open-course", []string{})

	if got := listCourseSlugs(t, h, cookie); !contains(got, "open-course") {
		t.Fatalf("an untagged course must stay visible: %v", got)
	}
}

// 一个 pro 学校的学生看得见标了 pro 的课 —— 反向也要成立，否则上面三条都可能
// 是「所有课都不见了」这一个 bug 的假阳性。
func TestCourseAudience_ProStudentSeesProCourse(t *testing.T) {
	pool := newAPITestPool(t)
	seedCourseWithDefinition(t, pool, "pro-course", testCourseDefinitionJSON)
	setCourseAudience(t, pool, "pro-course", []string{"pro"})
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	if got := listCourseSlugs(t, h, cookie); !contains(got, "pro-course") {
		t.Fatalf("pro student cannot see a pro course: %v", got)
	}
}
