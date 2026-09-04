package api

// course_audience.go —— 这门课摆给谁看。
//
// 产品负责人 2026-09-04：「the current course, we need to have a level or label
// so that we can present different courses to different kind of students
// (currently pro or lite)」。
//
// 所以 audience 是一个**受众标签**，不是难度分级。今天它只有两个值，因为今天
// 我们只按版本分学生；以后会有别的维度（年级、学科、班级），词表在这里加一行，
// 不动表结构（列是 text[]，见 migration 0133）。
//
// 🚨 空 audience = 不限受众。这条约定是有意的：一门课**没有**被打上标签，正确
// 的默认是"谁都能上"，而不是"谁都上不了"。反过来的默认会让一次漏填把一门
// 课从所有人的目录里拿掉，而没有任何界面会说这件事。

import (
	"context"
	"net/http"

	"mindimprint/api/internal/httpx"
)

// 受众词表。和 category 一样在应用层校验，不写 DB CHECK——词表的真相源是 TS
// 契约（packages/contracts/src/course.ts · CourseAudience），DB 再抄一份就一定
// 会漂移。
const (
	AudienceLite = "lite"
	AudiencePro  = "pro"
)

// courseAudiences 是当前允许的取值，顺序固定（快照测试友好）。
func courseAudiences() []string { return []string{AudienceLite, AudiencePro} }

// isCourseAudience 说的是：这是不是一个我们认得的受众。
func isCourseAudience(v string) bool {
	for _, a := range courseAudiences() {
		if a == v {
			return true
		}
	}
	return false
}

// courseVisibleTo 说的是：一门受众为 audience 的课，摆不摆给 edition 这种学生看。
//
// 空 audience 一律看得见（见文件头）。edition 为空（拿不到学校）时同样一律看得
// 见：一次读不到学校不该表现成"课程没了"，那会把一个基础设施故障伪装成产品行为。
func courseVisibleTo(audience []string, edition string) bool {
	if len(audience) == 0 || edition == "" {
		return true
	}
	for _, a := range audience {
		if a == edition {
			return true
		}
	}
	return false
}

// callerEdition 返回请求者所属学校的版本（"lite" / "pro"），取不到就是空串。
//
// 和 requireEdition 读的是同一个事实：版本是**组织**的属性，不是账号的
// （见 authz.go）。取不到不报错——调用点对空串的处理都是"不过滤"，这比让整个
// 课程目录因为一次学校查询失败而 500 要好。
func (a *API) callerEdition(ctx context.Context) string {
	u, ok := UserFromContext(ctx)
	if !ok {
		return ""
	}
	school, err := a.d.Queries.GetSchool(ctx, u.SchoolID)
	if err != nil {
		return ""
	}
	return school.Edition
}

// requireCourseAudience 是按 slug 读一门课时的受众闸。
//
// 不匹配是 404，不是 403——和 requireVisibleCourse、requireEdition 同一个约定：
// 从一个 lite 学生的角度看，一门只给 pro 的课就是不存在，不该从错误码里泄漏出
// 「有这么一门课，只是不给你」。
//
// 管理员绕过：他们是课程的作者，得能预览自己刚标好受众的那一门。
func (a *API) requireCourseAudience(w http.ResponseWriter, r *http.Request, audience []string) bool {
	if isAdmin(r.Context()) {
		return true
	}
	if courseVisibleTo(audience, a.callerEdition(r.Context())) {
		return true
	}
	httpx.WriteError(w, r, httpx.ErrNotFound("课程不存在"))
	return false
}

// normalizeCourseAudience 清洗一份写进库的受众列表：去掉认不出的值、去重、
// 保持词表顺序。
//
// 返回 nil 表示"这份 body 没说受众"——调用点据此保留库里已有的值（见
// queries/course.sql · UpsertCourseDefinition 的注释：受众是唯一「省略 = 保留」
// 的字段）。传了一个空数组（非 nil）则是明确的「不限受众」，原样落库。
func normalizeCourseAudience(in []string) []string {
	if in == nil {
		return nil
	}
	seen := map[string]bool{}
	for _, v := range in {
		if isCourseAudience(v) {
			seen[v] = true
		}
	}
	out := []string{}
	for _, a := range courseAudiences() {
		if seen[a] {
			out = append(out, a)
		}
	}
	return out
}
