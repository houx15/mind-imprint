// Package disciplines 是学科表 —— 自上而下的那一张图。
//
// 学生的兴趣关键词是自下而上长出来的（她读过、写过、做过什么）；这个包是另一
// 半：人类已经分好的学科。internal/interest 的路由把两者连起来。
//
// # 学科为骨，课程为投影
//
// Syllabus 是一个**可以为空**的投影层。IB / A-Level / AP / IGCSE 各自的考纲编号
// 挂在学科上，而不是反过来。这样：
//
//   - 考试局 2027 年改一次大纲，改的是这个数组，学生的关键词不受影响 —— 因为
//     它当初连的是 "statistical-inference"，从来不是 "MAA-HL-4.11"。
//   - 读 A-Level 的学生和读 IB 的学生共用同一张图，只是投影不同。
//   - 空的 Syllabus 是合法的，它表示「这门学科高中不教」。那是真话，而且对一个
//     讲探索的产品来说是有吸引力的真话；绑死某一个考试局的分类学根本说不出这句。
//
// # 为什么是 JSON 而不是一张表
//
// 它是内容，不是用户数据。全校共用一份，改它是一次内容编辑加一次评审，不是一次
// 数据迁移。只有「边」（她的词 → 学科）才进 Postgres。
//
// # 为什么有两份一模一样的 disciplines.json
//
// 故意的。可编辑的真相源在 packages/contracts/disciplines/disciplines.json（前端
// 构建期 import 同一份）；go:embed 的模式出不了本包目录，所以这里放一份逐字节
// 副本。TestEmbeddedCopyMatchesSourceOfTruth 守着两者不漂移。不要「简化」掉其中
// 任何一份 —— 那会把这份重复本来要解决的问题放回来。
package disciplines

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed disciplines.json
var disciplinesJSON []byte

// Fields 是树的七根主枝，也是 Discipline.Field 的合法取值。
//
// 第七根 formal「数学与形式」是 2026-09-02 加的：原来的六根里没有数学，而这个
// 产品服务 IB / A-Level / AP —— 统计推断、微积分、逻辑、离散数学没有家是硬缺口。
var Fields = []string{"formal", "science", "making", "society", "humanities", "arts", "self"}

// FieldLabels 是主枝的中文名，用于日志与错误信息；界面上的文案由前端自己拿。
var FieldLabels = map[string]string{
	"formal":     "数学与形式",
	"science":    "科学与自然",
	"making":     "技术与创造",
	"society":    "社会与世界",
	"humanities": "人文与写作",
	"arts":       "艺术与表达",
	"self":       "自我与成长",
}

// IsField 报告 f 是不是七根主枝之一。采集器用它挡掉模型编出来的野生 field。
func IsField(f string) bool {
	_, ok := FieldLabels[f]
	return ok
}

// SyllabusRef 是一门学科在某一个考试体系里的落点。
type SyllabusRef struct {
	// Board: "IB" | "ALevel" | "AP" | "IGCSE" | "University"
	Board string `json:"board"`
	Code  string `json:"code"`
	Label string `json:"label"`
	Level string `json:"level,omitempty"`
}

// Discipline 是一门学科。
//
// Asks / Method / Exemplar 三个字段是这张表值得打开的原因：一个只有名字的学科
// 节点，学生点开只会看到自己已经知道的四个字。
//
//   - Asks     它研究什么 —— 写成一句问题，不是定义。
//   - Method   它的核心方法 —— 学生能拿去用的那几个词。
//   - Exemplar 一个典型问题 —— 具体到能想象出画面。
type Discipline struct {
	ID       string        `json:"id"`
	Field    string        `json:"field"`
	Zh       string        `json:"zh"`
	En       string        `json:"en"`
	Asks     string        `json:"asks"`
	Method   string        `json:"method"`
	Exemplar string        `json:"exemplar"`
	Aliases  []string      `json:"aliases"`
	Syllabus []SyllabusRef `json:"syllabus"`
}

var (
	once    sync.Once
	all     []Discipline
	byID    map[string]Discipline
	byAlias map[string]Discipline
	loadErr error
)

func load() {
	once.Do(func() {
		if err := json.Unmarshal(disciplinesJSON, &all); err != nil {
			loadErr = err
			return
		}
		byID = make(map[string]Discipline, len(all))
		byAlias = make(map[string]Discipline, len(all)*10)
		for _, d := range all {
			byID[d.ID] = d
			// 学科自己的中英文名天然是别名，不必在数据里重复写一遍。
			for _, a := range append([]string{d.Zh, d.En}, d.Aliases...) {
				if n := Normalize(a); n != "" {
					byAlias[n] = d
				}
			}
		}
	})
}

// LoadErr 返回解析 disciplines.json 时的错误（正常情况下是 nil）。测试用它把
// 「数据坏了」和「查不到」区分开，而不是让一个 panic 淹掉真正的原因。
func LoadErr() error { load(); return loadErr }

// All 返回全部学科，按文件里的顺序。
func All() []Discipline { load(); return all }

// ByID 按 id 查一门学科。模型返回的 id 必须先过这一关才允许落库。
func ByID(id string) (Discipline, bool) {
	load()
	d, ok := byID[id]
	return d, ok
}

// ByField 返回一根主枝下的全部学科。路由的 T3 档用它组候选表 —— 只带同枝的
// 六门，prompt 才小，判错也只错在一根枝的范围内。
func ByField(field string) []Discipline {
	load()
	out := make([]Discipline, 0, 6)
	for _, d := range all {
		if d.Field == field {
			out = append(out, d)
		}
	}
	return out
}

// MatchAlias 是路由的 T1 档：归一化后的词直接命中某门学科的别名。免费，且确定。
// 传进来的字符串必须已经过 Normalize。
func MatchAlias(normalized string) (Discipline, bool) {
	load()
	d, ok := byAlias[normalized]
	return d, ok
}

// Normalize 把一个词压成可比较的形式：去空白、去常见标点、转小写。
//
// 路由的 T1 档、关键词去重、别名唯一性测试共用这一个实现，所以三处不可能对
// 「两个词是不是同一个」有不同看法。
func Normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch r {
		case ' ', '\t', '\n', '\r', '·', '、', '，', ',', '。', '.', '「', '」',
			'"', '\'', '(', ')', '（', '）', '-', '_', '/', '：', ':':
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
