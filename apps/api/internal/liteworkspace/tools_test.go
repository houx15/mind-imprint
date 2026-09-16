package liteworkspace

import (
	"strings"
	"testing"
)

// 工具名与闭集必须和 Task 1 的解析器对得上。工具 schema 里写了一个
// ParseStudentFilter 不认的值，模型就会照着 schema 交出一个我们会拒绝的调用。
func TestAssignmentToolsDeclareOnlyParsableFilters(t *testing.T) {
	for _, tool := range AssignmentTools() {
		if tool.Name != "list_students" {
			continue
		}
		props, _ := tool.Parameters["properties"].(map[string]any)
		filter, _ := props["filter"].(map[string]any)
		enum, _ := filter["enum"].([]any)
		if len(enum) == 0 {
			t.Fatal("list_students must constrain filter with an enum")
		}
		for _, v := range enum {
			s, _ := v.(string)
			if _, ok := ParseStudentFilter(s); !ok {
				t.Fatalf("schema offers filter %q that ParseStudentFilter rejects", s)
			}
		}
		return
	}
	t.Fatal("list_students tool is missing")
}

func TestAssignmentToolsAreNamedOnce(t *testing.T) {
	seen := map[string]bool{}
	for _, tool := range AssignmentTools() {
		if seen[tool.Name] {
			t.Fatalf("tool %q declared twice", tool.Name)
		}
		seen[tool.Name] = true
	}
	for _, want := range []string{"set_fields", "search_library", "set_material", "list_students", "set_recipients", "ask_choice"} {
		if !seen[want] {
			t.Fatalf("tool %q is missing", want)
		}
	}
}

// 系统提示词必须带上今天的北京日期——模型要靠它把「周五」算成绝对时刻，
// 而 BeijingWallToUTC 只收绝对时刻。
func TestAssignmentSystemCarriesToday(t *testing.T) {
	s := AssignmentSystem(SystemContext{ClassName: "初三二班", TodayBeijing: "2026-09-16", StudentCount: 12})
	if !strings.Contains(s, "2026-09-16") {
		t.Fatal("system prompt must state today's Beijing date")
	}
	if !strings.Contains(s, "初三二班") {
		t.Fatal("system prompt must name the class")
	}
}
