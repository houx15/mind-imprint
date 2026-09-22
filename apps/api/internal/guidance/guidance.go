// Package guidance 回答一件事：这一次该用哪份教学内容。
//
// # 为什么要有它
//
// 2026-09-22 之前，这个问题由五处各自回答（读法表、带读说明、写作立题的
// switch、vocab 三轴、毛病表三个切片），各有各的兜底、各有各的缺口。
// 这里放的是**唯一那条匹配规则**，五处共用。
//
// 规则本身不认识任何一段教学内容 —— 内容由调用方登记进来，所以阅读的
// readingRoutine 和写作的毛病表可以用同一条规则，不必都变成字符串。
package guidance

// Key 是「这一次是什么情况」。
type Key struct {
	Surface string // SurfaceRead / SurfaceWrite / SurfaceComment
	Lang    string // "zh" | "en"
	Genre   string // argument | narrative | report | explain
	Grade   string // "" 不限 | junior1..3 | senior1..3
}

const (
	SurfaceRead    = "read"
	SurfaceWrite   = "write"
	SurfaceComment = "comment"
)

// Scope 是一行登记：它在什么情况下适用。
// Genres / Grades 为空表示不限；Surface / Lang 为空同理。
type Scope struct {
	Surface string
	Lang    string
	Genres  []string
	Grades  []string // 可以写年级（junior2），也可以写学段（junior）
}

// Row 是注册表里的一行。T 是这一行携带的东西 —— 一段正文、一套读法、
// 一张毛病表，都行。
type Row[T any] struct {
	Scope Scope
	Value T
}

// GradeBand 把年级折成学段。
//
// 「整个初中通用」的内容因此只写一份，登记成 Grades: []string{"junior"}，
// 初一初二初三都取得到。
func GradeBand(grade string) string {
	switch grade {
	case "junior1", "junior2", "junior3":
		return "junior"
	case "senior1", "senior2", "senior3":
		return "senior"
	}
	return ""
}

// Matches —— 这一行服务这一次吗。
func (s Scope) Matches(k Key) bool {
	if s.Surface != "" && s.Surface != k.Surface {
		return false
	}
	if s.Lang != "" && s.Lang != k.Lang {
		return false
	}
	if len(s.Genres) > 0 && !contains(s.Genres, k.Genre) {
		return false
	}
	if len(s.Grades) > 0 &&
		!contains(s.Grades, k.Grade) && !contains(s.Grades, GradeBand(k.Grade)) {
		return false
	}
	return true
}

// specificity —— 这一行有多具体。大的赢。
//
// 权重拉开到 2 的幂，算出来的不是「轴的条数」，是一条严格的优先顺序：
// Surface(16) > Lang(8) > Genre(4) > Grade(最多 3)。上一根轴单独一分，
// 永远压过下面所有轴加在一起 —— 8 > 4+3，定了语言的一行一定赢只在文体、
// 学段上更细的一行，不管那一行凑了几根轴。
//
// 生产里真的靠这条顺序活着的只有 Lang(8) > Genre(4) 这一处：英文记叙文
// 能拿到英文那张毛病表（writing_symptoms.go），而不是掉到中文记叙文的
// 兜底行，靠的就是「先比语言、语言一样才比文体」。倒过来定优先级 ——
// 文体比语言更重 —— 会让这批学生悄悄换到中文教学内容上，她看不出来，
// 因为印记仍然在用中文跟她说话。
func (s Scope) specificity(k Key) int {
	n := 0
	if s.Surface != "" {
		n += 16
	}
	if s.Lang != "" {
		n += 8
	}
	if len(s.Genres) > 0 {
		n += 4
	}
	switch {
	case len(s.Grades) == 0:
		// 不限学段，不加分。
	case contains(s.Grades, k.Grade):
		n += 2 // 年级逐字命中
	default:
		n += 1 // 只命中学段
	}
	return n
}

func contains(xs []string, s string) bool {
	if s == "" {
		return false
	}
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
