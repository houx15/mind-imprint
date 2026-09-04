package interests

import (
	"bytes"
	"os"
	"testing"

	"mindimprint/api/internal/disciplines"
)

func TestJSONParses(t *testing.T) {
	if err := LoadErr(); err != nil {
		t.Fatalf("interests.json 解析失败: %v", err)
	}
	if len(All()) < 200 {
		t.Fatalf("词表只有 %d 条，太薄了 —— 一张两百条以下的表挑不中她真正在说的东西", len(All()))
	}
}

// 打错一个学科 id 不会编译报错，也不会在运行时报错：那片叶子只是连不到任何根，
// 而「连不到根」和「这个学生还没做过相关的事」在界面上长得一模一样。所以这条
// 必须由测试守着。
func TestEveryDisciplineIDIsReal(t *testing.T) {
	for _, it := range All() {
		for _, d := range it.Disciplines {
			if _, ok := disciplines.ByID(d); !ok {
				t.Errorf("领域 %s(%s) 指向了不存在的学科 %q", it.ID, it.Zh, d)
			}
		}
	}
}

func TestEveryEntryHasAtLeastTwoDisciplines(t *testing.T) {
	// 一条边的领域说不出「它是几门学问的组合」，而那正是这张表存在的理由。
	for _, it := range All() {
		if len(it.Disciplines) < 2 {
			t.Errorf("领域 %s(%s) 只有 %d 条边", it.ID, it.Zh, len(it.Disciplines))
		}
	}
}

func TestEveryFieldIsReal(t *testing.T) {
	for _, it := range All() {
		if !disciplines.IsField(it.Field) {
			t.Errorf("领域 %s 的 field %q 不是七根主枝之一", it.ID, it.Field)
		}
	}
}

func TestIDsAreUnique(t *testing.T) {
	seen := map[string]string{}
	for _, it := range All() {
		if prev, dup := seen[it.ID]; dup {
			t.Errorf("id %q 重复（%s 与 %s）", it.ID, prev, it.Zh)
		}
		seen[it.ID] = it.Zh
	}
}

// 中文名重复会让两个不同的 id 在树上长成两片看不出区别的叶子。
func TestChineseNamesAreUnique(t *testing.T) {
	seen := map[string]string{}
	for _, it := range All() {
		if prev, dup := seen[it.Zh]; dup {
			t.Errorf("中文名 %q 重复（%s 与 %s）", it.Zh, prev, it.ID)
		}
		seen[it.Zh] = it.ID
	}
}

func TestEveryEntryIsComplete(t *testing.T) {
	for _, it := range All() {
		if it.ID == "" || it.Zh == "" || it.En == "" || it.Field == "" {
			t.Errorf("领域 %+v 有空字段", it)
		}
	}
}

// 入表标准里的「2-4 字为主」。放到 8 是给「3D 打印」「冲突与调解」这类留余地，
// 但一个十几个字的条目说明它其实是一句话，不是一个领域。
func TestNamesAreShort(t *testing.T) {
	for _, it := range All() {
		if n := len([]rune(it.Zh)); n > 8 {
			t.Errorf("领域 %s 的中文名 %q 有 %d 个字，太长了", it.ID, it.Zh, n)
		}
	}
}

func TestExistsRejectsInventedIDs(t *testing.T) {
	if !Exists("games") {
		t.Fatal("games 应该在表里")
	}
	// 模型很容易造出这种看起来合理的 id。闭表那条不变量就是挡它。
	for _, invented := range []string{"esports", "coral-reefs", "environmentalism", ""} {
		if Exists(invented) {
			t.Errorf("Exists(%q) 应该是 false", invented)
		}
	}
}

func TestPromptListCoversEveryEntry(t *testing.T) {
	list := PromptList()
	for _, it := range All() {
		if !bytes.Contains([]byte(list), []byte("("+it.ID+")")) {
			t.Errorf("候选清单里没有 %s(%s) —— 模型选不到它", it.Zh, it.ID)
		}
	}
}

// 清单进 system prompt，每次采集都付这个钱。涨到几万字就该重新想办法了。
func TestPromptListStaysSmall(t *testing.T) {
	if n := len([]rune(PromptList())); n > 6000 {
		t.Fatalf("候选清单 %d 字，太大了", n)
	}
}

func TestEveryFieldHasEntries(t *testing.T) {
	// 分布不均是设计的一部分（formal 在世俗词里天然薄），但**空的**主枝会让
	// 那根枝上永远长不出叶子。
	for _, f := range disciplines.Fields {
		if len(ByField(f)) == 0 {
			t.Errorf("主枝 %s 一个领域都没有", f)
		}
	}
}

func TestEmbeddedCopyMatchesSourceOfTruth(t *testing.T) {
	src, err := os.ReadFile("../../../../packages/contracts/interests/interests.json")
	if err != nil {
		t.Fatalf("读不到真相源: %v", err)
	}
	if !bytes.Equal(src, interestsJSON) {
		t.Fatal("apps/api/internal/interests/interests.json 已与 packages/contracts/interests/interests.json 漂移 —— 跑 packages/contracts/interests/build.py")
	}
}
