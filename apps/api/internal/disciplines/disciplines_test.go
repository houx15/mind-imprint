package disciplines

import (
	"bytes"
	"os"
	"testing"
)

func TestJSONParses(t *testing.T) {
	if err := LoadErr(); err != nil {
		t.Fatalf("disciplines.json 解析失败: %v", err)
	}
}

// 七根主枝各六门。这条不是审美要求：ByField 是 T3 档的候选表，六门左右 prompt
// 才小；一根枝上堆到二十门，模型的判断就退化成挑最眼熟的那个。
func TestEveryFieldHasSix(t *testing.T) {
	count := map[string]int{}
	for _, d := range All() {
		if !IsField(d.Field) {
			t.Fatalf("%s: 未知主枝 %q", d.ID, d.Field)
		}
		count[d.Field]++
	}
	if len(count) != len(Fields) {
		t.Fatalf("want %d fields, got %d", len(Fields), len(count))
	}
	for _, f := range Fields {
		if count[f] != 6 {
			t.Errorf("主枝 %s 有 %d 门学科，want 6", f, count[f])
		}
	}
}

func TestIDsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range All() {
		if seen[d.ID] {
			t.Errorf("重复的 id: %s", d.ID)
		}
		seen[d.ID] = true
	}
}

// 别名必须全局唯一。两门学科抢同一个别名，T1 会按文件顺序静默选一个 —— 而那个
// 顺序对读代码的人完全不可见，是一类查起来极贵的 bug。
func TestAliasesAreGloballyUnique(t *testing.T) {
	owner := map[string]string{}
	for _, d := range All() {
		for _, a := range append([]string{d.Zh, d.En}, d.Aliases...) {
			n := Normalize(a)
			if n == "" {
				t.Errorf("%s: 空别名", d.ID)
				continue
			}
			if prev, dup := owner[n]; dup && prev != d.ID {
				t.Errorf("别名 %q 被 %s 和 %s 同时认领", a, prev, d.ID)
			}
			owner[n] = d.ID
		}
	}
}

func TestEveryDisciplineIsComplete(t *testing.T) {
	for _, d := range All() {
		if d.Zh == "" || d.En == "" || d.Asks == "" || d.Method == "" || d.Exemplar == "" {
			t.Errorf("%s: 字段不全 (zh=%q en=%q asks=%q method=%q exemplar=%q)",
				d.ID, d.Zh, d.En, d.Asks, d.Method, d.Exemplar)
		}
		// T1 是免费那一档。别名少，命中率就低，所有词都掉到花钱的 T3 上。
		if len(d.Aliases) < 4 {
			t.Errorf("%s: 只有 %d 个别名，want >= 4", d.ID, len(d.Aliases))
		}
	}
}

// Asks 是「它研究什么」，写成一句问题。这条测试守的是内容风格：一个写成定义的
// asks（「统计推断是一门研究…的学科」）会让学科卡退回成词典条目。
func TestAsksIsAQuestion(t *testing.T) {
	for _, d := range All() {
		runes := []rune(d.Asks)
		if last := runes[len(runes)-1]; last != '？' && last != '?' {
			t.Errorf("%s: asks 不是一句问题: %q", d.ID, d.Asks)
		}
	}
}

// 一门学科至少落在一个考试体系或大学方向上，否则学生无从判断它和自己的关系。
// 允许 syllabus 里只有 University —— 那正是「这门学科高中不教」的表达方式。
func TestEveryDisciplineHasSomewhereToLand(t *testing.T) {
	boards := map[string]bool{"IB": true, "ALevel": true, "AP": true, "IGCSE": true, "University": true}
	for _, d := range All() {
		if len(d.Syllabus) == 0 {
			t.Errorf("%s: syllabus 为空 —— 至少要有一个大学方向", d.ID)
		}
		for _, s := range d.Syllabus {
			if !boards[s.Board] {
				t.Errorf("%s: 未知考试体系 %q", d.ID, s.Board)
			}
			if s.Code == "" || s.Label == "" {
				t.Errorf("%s: syllabus 条目不全 %+v", d.ID, s)
			}
		}
	}
}

// 四大体系至少各被覆盖到一定数量，否则「兼容 IB / A-Level / AP」是句空话。
func TestAllBoardsAreActuallyCovered(t *testing.T) {
	count := map[string]int{}
	for _, d := range All() {
		for _, s := range d.Syllabus {
			count[s.Board]++
		}
	}
	for _, b := range []string{"IB", "ALevel", "AP", "University"} {
		if count[b] < 10 {
			t.Errorf("体系 %s 只覆盖了 %d 门学科，want >= 10", b, count[b])
		}
	}
}

func TestMatchAliasFindsDiscipline(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"代表性", "statistical-inference"},
		{"统计推断", "statistical-inference"}, // 学科自己的中文名
		{"Statistical inference", "statistical-inference"},
		{"机器学习", "ai-ml"},
	} {
		d, ok := MatchAlias(Normalize(c.in))
		if !ok || d.ID != c.want {
			t.Errorf("MatchAlias(%q) = %q/%v, want %q", c.in, d.ID, ok, c.want)
		}
	}
}

func TestMatchAliasMissesUnknownWords(t *testing.T) {
	if d, ok := MatchAlias(Normalize("一个谁也不认识的词")); ok {
		t.Errorf("凭空命中了 %s", d.ID)
	}
}

func TestByFieldReturnsSix(t *testing.T) {
	got := ByField("formal")
	if len(got) != 6 {
		t.Fatalf("ByField(formal) 返回 %d 门，want 6", len(got))
	}
	for _, d := range got {
		if d.Field != "formal" {
			t.Errorf("%s 不属于 formal", d.ID)
		}
	}
}

func TestNormalizeCollapsesPunctuationAndCase(t *testing.T) {
	for _, c := range []struct{ a, b string }{
		{"例外与代表性", "例外 与 代表性"},
		{"Sampling", "sampling"},
		{"人工智能 / 机器学习", "人工智能机器学习"},
		{"「气候」", "气候"},
	} {
		if Normalize(c.a) != Normalize(c.b) {
			t.Errorf("Normalize(%q)=%q != Normalize(%q)=%q", c.a, Normalize(c.a), c.b, Normalize(c.b))
		}
	}
	if Normalize("气候与海洋") == Normalize("记忆怎么形成") {
		t.Error("两个不同的词被压成了同一个")
	}
}

// go:embed 出不了包目录，所以真相源与副本靠这条测试守住。见 vocab_test.go 的同名测试。
func TestEmbeddedCopyMatchesSourceOfTruth(t *testing.T) {
	src, err := os.ReadFile("../../../../packages/contracts/disciplines/disciplines.json")
	if err != nil {
		t.Fatalf("读不到真相源: %v", err)
	}
	if !bytes.Equal(src, disciplinesJSON) {
		t.Fatal("apps/api/internal/disciplines/disciplines.json 已与 packages/contracts/disciplines/disciplines.json 漂移 —— 把真相源复制过来重跑")
	}
}
