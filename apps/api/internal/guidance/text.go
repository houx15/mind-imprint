package guidance

import "fmt"

// Slot 是提示词模板上的一个洞。
type Slot string

const (
	SlotKinds    Slot = "kinds"    // 节点类型闭表
	SlotMaterial Slot = "material" // 材料怎么选
	SlotSkeleton Slot = "skeleton" // 常见文章结构
	SlotCoach    Slot = "coach"    // 这一篇怎么带
	// SlotCeiling 是「她学到哪儿了」—— 只按**年级**登记，不按文体。
	//
	// 🚨 它是第一个真正用上 Grades 的槽。Key.Grade 这条轴 2026-09-22 就
	// 建好了（classes.grade → writingPlanSystemFor 的第三个参数），
	// 但在这之前**没有任何一行按年级登记过内容**，于是它跑了三天空车：
	// 印记照样会建议一个初一学生用虚拟语气。
	SlotCeiling Slot = "ceiling"
)

// Registry 是「哪个槽在什么情况下用哪段正文」。
type Registry struct {
	slots map[Slot][]Row[string]
}

func NewRegistry() *Registry {
	return &Registry{slots: map[Slot][]Row[string]{}}
}

// Add 登记一行。同一个槽登记多行时，先登记的在平局时胜出（见 Pick）。
func (r *Registry) Add(slot Slot, scope Scope, text string) {
	r.slots[slot] = append(r.slots[slot], Row[string]{Scope: scope, Value: text})
}

// Resolve 取这一次要用的每个槽。
//
// 🚨 取不到返回 error，不返回空串。见 TestResolveErrorsRatherThanReturningEmpty
// 上面那段注释：空串在线上的样子是「印记话变少了」。
func (r *Registry) Resolve(k Key, slots ...Slot) (map[Slot]string, error) {
	out := make(map[Slot]string, len(slots))
	for _, slot := range slots {
		text, ok := Pick(k, r.slots[slot])
		if !ok || text == "" {
			return nil, fmt.Errorf(
				"guidance: 槽 %q 在 surface=%q lang=%q genre=%q grade=%q 上没有内容",
				slot, k.Surface, k.Lang, k.Genre, k.Grade)
		}
		out[slot] = text
	}
	return out, nil
}
