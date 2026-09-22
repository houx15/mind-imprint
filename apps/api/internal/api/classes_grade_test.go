package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/auth"
	"mindimprint/api/internal/store/sqlc"
)

// DTO 要把年级带出去，否则老师那边建完就再也看不到自己填了什么。
func TestClassDTOCarriesGrade(t *testing.T) {
	dto := toClassDTO(sqlcClassWithGrade("junior2"))
	if dto.Grade != "junior2" {
		t.Errorf("classDTO.Grade = %q，想要 junior2", dto.Grade)
	}
	if dto.GradeLabel != "初二" {
		t.Errorf("classDTO.GradeLabel = %q，想要「初二」", dto.GradeLabel)
	}
}

// 🚨 没填年级的班（老班、批量导入建的）要原样出去，不要在这儿编一个默认值。
// 编一个出来，她就会按一个没人填过的年级拿教学内容。
func TestClassDTOKeepsUnknownGradeEmpty(t *testing.T) {
	dto := toClassDTO(sqlcClassWithGrade(""))
	if dto.Grade != "" || dto.GradeLabel != "" {
		t.Errorf("没填年级时该是两个空串，拿到 %q / %q", dto.Grade, dto.GradeLabel)
	}
}

func sqlcClassWithGrade(grade string) sqlc.Class {
	return sqlc.Class{Name: "试用班级", JoinCode: "AAAA-BBBB", Grade: grade}
}

// 🚨 闭表里每一个年级都必须有中文说法。
//
// 少一个不会报错 —— map 取不到就是空串，老师的屏幕上那一格直接是空的，
// 而测试全绿。Task 3 的评审指出这条缺口时，原来的测试只抽查了六个里的两个。
func TestEveryClassGradeHasALabel(t *testing.T) {
	for _, g := range classGrades {
		if classGradeLabel(g) == "" {
			t.Errorf("年级 %q 没有中文说法", g)
		}
	}
}

// gradeTestTeacher 和 gradeTestSignIn 是 maintest_test.go 的 createTeacher /
// signInAs 的本地副本——那两个定义在 package api_test 里，这个文件为了能
// 直接摸 toClassDTO / classGradeLabel 这些未导出符号，必须留在 package api，
// 两边互不可见（同一条纪律见 revision_checkpoint_test.go 顶部的注释）。
func gradeTestTeacher(t *testing.T, pool *pgxpool.Pool, schoolID uuid.UUID, email string) uuid.UUID {
	t.Helper()
	u, err := sqlc.New(pool).CreateUser(context.Background(), sqlc.CreateUserParams{
		Email: email, PasswordHash: "x", Role: "teacher", SchoolID: schoolID,
		DisplayName: "T " + email, AvatarColor: "#888888",
		EmailVerifiedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		t.Fatalf("create teacher: %v", err)
	}
	return u.ID
}

func gradeTestSignIn(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) *http.Cookie {
	t.Helper()
	raw, hash, err := auth.NewToken()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if _, err := sqlc.New(pool).CreateSession(context.Background(), sqlc.CreateSessionParams{
		UserID: userID, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	return &http.Cookie{Name: "mk_session", Value: raw}
}

// 🚨 handler 层的校验只在 validateClassGrade 里读代码验证过；没有任何一条
// 测试真正打 HTTP 确认不认识的年级会被拒。前端传错一个字符串，这条测试
// 才是最后一道防线。
func TestCreateClassRejectsUnknownGrade(t *testing.T) {
	pool := NewTestDB(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	teacherID := gradeTestTeacher(t, pool, SeedSchoolID, "reject-grade@demo.local")
	teacher := gradeTestSignIn(t, pool, teacherID)

	body, _ := json.Marshal(map[string]any{"name": "坏年级班", "grade": "junior9"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/classes", bytes.NewReader(body))
	req.AddCookie(teacher)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("不认识的年级该是 400，拿到 %d body=%s", rec.Code, rec.Body)
	}

	rows, err := sqlc.New(pool).ListClassesForTeacher(context.Background(), teacherID)
	if err != nil {
		t.Fatalf("ListClassesForTeacher: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("这次请求不该建出任何班，拿到 %d 个", len(rows))
	}
}

// 建班时传一个真实存在的年级，回包里的 grade 和 grade_label 都要对上 ——
// 她刚建完就要看到自己填了什么，不用再刷新一次。
func TestCreateClassAcceptsAGradeAndReturnsItsLabel(t *testing.T) {
	pool := NewTestDB(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	teacherID := gradeTestTeacher(t, pool, SeedSchoolID, "accept-grade@demo.local")
	teacher := gradeTestSignIn(t, pool, teacherID)

	body, _ := json.Marshal(map[string]any{"name": "初二 3 班", "grade": "junior2"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/classes", bytes.NewReader(body))
	req.AddCookie(teacher)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("createClass 想要 201，拿到 %d body=%s", rec.Code, rec.Body)
	}
	var resp struct {
		Class struct {
			Grade      string `json:"grade"`
			GradeLabel string `json:"grade_label"`
		} `json:"class"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if resp.Class.Grade != "junior2" {
		t.Errorf("class.grade = %q，想要 junior2", resp.Class.Grade)
	}
	if resp.Class.GradeLabel != "初二" {
		t.Errorf("class.grade_label = %q，想要「初二」", resp.Class.GradeLabel)
	}
}

// 🚨 「字段没传」和「字段传了空串」是两件不同的事：前者是不动它，后者是
// 清空它。这条测试钉住后者 —— 老师建班时填错了年级，要能改回「没填」。
func TestPatchClassClearsGradeWithExplicitEmpty(t *testing.T) {
	pool := NewTestDB(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	teacherID := gradeTestTeacher(t, pool, SeedSchoolID, "clear-grade@demo.local")
	teacher := gradeTestSignIn(t, pool, teacherID)

	createBody, _ := json.Marshal(map[string]any{"name": "待清空年级班", "grade": "junior2"})
	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest("POST", "/api/v1/classes", bytes.NewReader(createBody))
	createReq.AddCookie(teacher)
	h.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("createClass 想要 201，拿到 %d body=%s", createRec.Code, createRec.Body)
	}
	var created struct {
		Class struct {
			ID    string `json:"id"`
			Grade string `json:"grade"`
		} `json:"class"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v — body=%s", err, createRec.Body)
	}
	if created.Class.Grade != "junior2" {
		t.Fatalf("建班时该已经存了 junior2，拿到 %q", created.Class.Grade)
	}

	patchBody, _ := json.Marshal(map[string]any{"grade": ""})
	patchRec := httptest.NewRecorder()
	patchReq := httptest.NewRequest("PATCH", "/api/v1/classes/"+created.Class.ID, bytes.NewReader(patchBody))
	patchReq.AddCookie(teacher)
	h.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patchClass 想要 200，拿到 %d body=%s", patchRec.Code, patchRec.Body)
	}
	var patched struct {
		Class struct {
			Grade string `json:"grade"`
		} `json:"class"`
	}
	if err := json.Unmarshal(patchRec.Body.Bytes(), &patched); err != nil {
		t.Fatalf("decode patch: %v — body=%s", err, patchRec.Body)
	}
	if patched.Class.Grade != "" {
		t.Errorf("显式传空串该清掉年级，拿到 %q", patched.Class.Grade)
	}
}
