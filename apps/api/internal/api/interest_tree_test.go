package api_test

// interest_tree_test.go — GET /api/v1/interest/tree 的库级测试。
//
// 这里测的是「读代码看不出对错」的那几件事（AGENTS.md 的测试原则）：迁移 0116
// 真的能建起来、三张表 join 回来的形状对得上前端要用的字段、学科表的静态内容
// 真的被补进了响应，以及两条容易在演示当天才被发现的不变量：
//
//   - 一个词都没有的主枝也必须回来。空枝不是缺数据，它是这张图上最有用的一条
//     信息（她还没走过的方向）；漏掉它，前端就画不出未点亮的枝。
//   - 指向一个已经从学科表里删掉的 id 的边，要整条跳过，而不是发一个只有 id 的
//     空壳让前端去渲染一张没有名字的学科卡。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
)

type treeResp struct {
	Fields []struct {
		ID           string `json:"id"`
		Label        string `json:"label"`
		KeywordCount int    `json:"keywordCount"`
	} `json:"fields"`
	Keywords []struct {
		ID       string `json:"id"`
		TextZh   string `json:"textZh"`
		TextEn   string `json:"textEn"`
		Field    string `json:"field"`
		Strength int    `json:"strength"`
		Note     string `json:"note"`
		Sources  []struct {
			Kind     string `json:"kind"`
			Label    string `json:"label"`
			Evidence string `json:"evidence"`
		} `json:"sources"`
		Disciplines []struct {
			ID         string  `json:"id"`
			Zh         string  `json:"zh"`
			Asks       string  `json:"asks"`
			Method     string  `json:"method"`
			Confidence float32 `json:"confidence"`
			How        string  `json:"how"`
			Syllabus   []struct {
				Board string `json:"board"`
				Label string `json:"label"`
			} `json:"syllabus"`
		} `json:"disciplines"`
	} `json:"keywords"`
}

func getTree(t *testing.T, h http.Handler, cookie *http.Cookie) treeResp {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/interest/tree", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /interest/tree = %d; body=%s", rec.Code, rec.Body)
	}
	var out treeResp
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rec.Body)
	}
	return out
}

// seedKeyword 直接写库，绕开采集器 —— 这个测试要验的是读的那一半。
func seedKeyword(t *testing.T, pool *pgxpool.Pool, disciplineID string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(t.Context(), `
		INSERT INTO interest_keyword (user_id, text_zh, text_en, norm, field, strength, note)
		VALUES ($1, '例外与代表性', 'Exception vs representative', '例外与代表性', 'formal', 4,
		        '你现在会先问这个例子能代表多少。')
		RETURNING id::text`, SeedUserID).Scan(&id)
	if err != nil {
		t.Fatalf("seed keyword: %v", err)
	}
	for _, s := range []struct{ kind, label, evidence string }{
		{"reading", "红海北端那片不白化的珊瑚", "我读到面积那一段才反应过来它有多小。"},
		{"writing", "一片珊瑚不能代表一片海", "一个避难所不是一个计划。"},
	} {
		if _, err := pool.Exec(t.Context(), `
			INSERT INTO keyword_source (keyword_id, kind, ref_id, label, evidence)
			VALUES ($1, $2, gen_random_uuid(), $3, $4)`,
			mustUUID(id), s.kind, s.label, s.evidence); err != nil {
			t.Fatalf("seed source: %v", err)
		}
	}
	if _, err := pool.Exec(t.Context(), `
		INSERT INTO keyword_discipline (keyword_id, discipline_id, confidence, how, rationale)
		VALUES ($1, $2, 1.0, 'alias', '「代表性」是统计推断的说法之一。')`,
		mustUUID(id), disciplineID); err != nil {
		t.Fatalf("seed edge: %v", err)
	}
	return id
}

// 一棵新账号的树是空的，但七根主枝一根都不能少 —— 否则第一次打开的学生看到的
// 是一张什么都没有的白纸，而不是一张画好了枝、等着长词的图。
func TestInterestTree_EmptyTreeStillCarriesAllSevenBranches(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	got := getTree(t, h, cookie)

	if len(got.Fields) != 7 {
		t.Fatalf("空树只回了 %d 根主枝，want 7", len(got.Fields))
	}
	if len(got.Keywords) != 0 {
		t.Fatalf("空树上有 %d 个词", len(got.Keywords))
	}
	for _, f := range got.Fields {
		if f.Label == "" {
			t.Errorf("主枝 %s 没有名字", f.ID)
		}
		if f.KeywordCount != 0 {
			t.Errorf("主枝 %s 在空树上有 %d 个词", f.ID, f.KeywordCount)
		}
	}
}

func TestInterestTree_HydratesTheDisciplineFromTheStaticTable(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	seedKeyword(t, pool, "statistical-inference")

	got := getTree(t, h, cookie)
	if len(got.Keywords) != 1 {
		t.Fatalf("want 1 keyword, got %d", len(got.Keywords))
	}
	k := got.Keywords[0]

	if k.TextZh != "例外与代表性" || k.Field != "formal" || k.Strength != 4 {
		t.Errorf("关键词字段不对：%+v", k)
	}
	if len(k.Sources) != 2 {
		t.Fatalf("want 2 sources, got %d", len(k.Sources))
	}
	// 树的全部说服力在这一句上：来源里带的是她自己的原话。
	for _, s := range k.Sources {
		if s.Evidence == "" {
			t.Errorf("来源 %s 没有带她的原话", s.Label)
		}
	}

	if len(k.Disciplines) != 1 {
		t.Fatalf("want 1 discipline, got %d", len(k.Disciplines))
	}
	d := k.Disciplines[0]
	if d.ID != "statistical-inference" || d.Zh != "统计推断" {
		t.Errorf("学科没对上：%+v", d)
	}
	// 库里只存了一个 id；asks / method / syllabus 全部来自静态学科表。
	// 它们空了，就说明补全那一步没跑，抽屉会是一张只有名字的卡。
	if d.Asks == "" || d.Method == "" {
		t.Errorf("学科的静态内容没补上：asks=%q method=%q", d.Asks, d.Method)
	}
	if len(d.Syllabus) == 0 {
		t.Error("课程投影没补上——她的课程体系那一行就没东西可显示了")
	}
	if d.How != "alias" || d.Confidence != 1.0 {
		t.Errorf("边的来路没带出来：how=%q conf=%v", d.How, d.Confidence)
	}

	// 有词的那根枝计数为 1，其余六根仍然在，且为 0。
	byID := map[string]int{}
	for _, f := range got.Fields {
		byID[f.ID] = f.KeywordCount
	}
	if byID["formal"] != 1 {
		t.Errorf("formal 计数 = %d, want 1", byID["formal"])
	}
	if len(byID) != 7 {
		t.Errorf("主枝少了：%v", byID)
	}
}

// 学科表里删掉过的 id 不该变成一张没有名字的学科卡。
func TestInterestTree_DropsEdgesToDisciplinesThatNoLongerExist(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	seedKeyword(t, pool, "a-discipline-we-deleted")

	got := getTree(t, h, cookie)
	if len(got.Keywords) != 1 {
		t.Fatalf("want 1 keyword, got %d", len(got.Keywords))
	}
	// 词本身还在（它是她的），只是那条边不见了。
	if n := len(got.Keywords[0].Disciplines); n != 0 {
		t.Fatalf("指向已删除学科的边被发出去了：%+v", got.Keywords[0].Disciplines)
	}
}

// 另一个学生的词不该出现在她的树上。归属校验值得有一条自己的测试。
func TestInterestTree_IsScopedToTheSignedInStudent(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	if _, err := pool.Exec(t.Context(), `
		INSERT INTO interest_keyword (user_id, text_zh, norm, field)
		SELECT id, '别人的词', '别人的词', 'arts' FROM users WHERE id <> $1 LIMIT 1`,
		SeedUserID); err != nil {
		t.Fatalf("seed other user keyword: %v", err)
	}
	got := getTree(t, h, cookie)
	for _, k := range got.Keywords {
		if k.TextZh == "别人的词" {
			t.Fatal("别的学生的词出现在了她的树上")
		}
	}
}
