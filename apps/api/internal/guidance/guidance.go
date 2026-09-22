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
	Stage   string // "" 不限 | junior1..3 | senior1..3
}

const (
	SurfaceRead    = "read"
	SurfaceWrite   = "write"
	SurfaceComment = "comment"
)

// Scope 是一行登记：它在什么情况下适用。
// Genres / Stages 为空表示不限；Surface / Lang 为空同理。
type Scope struct {
	Surface string
	Lang    string
	Genres  []string
	Stages  []string // 可以写年级（junior2），也可以写学段（junior）
}

// Row 是注册表里的一行。T 是这一行携带的东西 —— 一段正文、一套读法、
// 一张毛病表，都行。
type Row[T any] struct {
	Scope Scope
	Value T
}

// StageBand 把年级折成学段。
//
// 「整个初中通用」的内容因此只写一份，登记成 Stages: []string{"junior"}，
// 初一初二初三都取得到。
func StageBand(stage string) string {
	switch stage {
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
	if len(s.Stages) > 0 &&
		!contains(s.Stages, k.Stage) && !contains(s.Stages, StageBand(k.Stage)) {
		return false
	}
	return true
}

// specificity —— 这一行有多具体。大的赢。
//
// 权重拉开到 2 的幂，是为了让「轴的条数」永远压过「同一条轴上更细」：
// 定了文体的一行，一定比只在学段上更细的一行更该用。
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
	case len(s.Stages) == 0:
		// 不限学段，不加分。
	case contains(s.Stages, k.Stage):
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
