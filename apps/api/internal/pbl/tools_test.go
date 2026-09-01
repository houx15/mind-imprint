package pbl_test

import (
	"testing"

	"mindimprint/api/internal/pbl"
)

// 已知工具的类别由表说了算。同一件工具在不同项目里属于不同类别是说不通的，
// 而请求里的 kind 是客户端传的——听它的，等于把「她在做还是放弃了」交给
// 客户端决定。
func TestResolveToolKind_RegistryWinsOverRequest(t *testing.T) {
	if got := pbl.ResolveToolKind("observe", pbl.KindThinking); got != pbl.KindWorld {
		t.Fatalf("observe = %q, want world — 出门看东西不会因为请求这么说就变成当场做完", got)
	}
	if got := pbl.ResolveToolKind("decide", pbl.KindWorld); got != pbl.KindThinking {
		t.Fatalf("decide = %q, want thinking", got)
	}
}

// 工具箱是开放的：没见过的名字照样能用。
func TestResolveToolKind_UnknownToolIsAllowed(t *testing.T) {
	if got := pbl.ResolveToolKind("还没设计出来的工具", ""); got != pbl.KindThinking {
		t.Fatalf("unknown default = %q, want thinking", got)
	}
	if got := pbl.ResolveToolKind("去街上数一数", pbl.KindWorld); got != pbl.KindWorld {
		t.Fatalf("unknown declared world = %q, want world", got)
	}
}

// 只有 world 会让印记停下来等她。
func TestWaitsForStudent(t *testing.T) {
	if !pbl.WaitsForStudent(pbl.KindWorld) {
		t.Fatal("出门在外的时候印记该停下来")
	}
	if pbl.WaitsForStudent(pbl.KindThinking) {
		t.Fatal("当场做完的工具不该让对话停住")
	}
}

// 界面上出现的字里不该有方法论名字（产品负责人 2026-09-01：plain language,
// no jargon）。这条人眼很难一直盯住，加一件新工具时最容易破。
func TestToolLabels_NoJargon(t *testing.T) {
	jargon := []string{"HMW", "Reframe", "reframe", "矩阵", "范式", "架构",
		"发散", "收敛", "复用", "赋能", "闭环", "抓手"}
	for _, name := range pbl.ToolNames() {
		tool, ok := pbl.LookupTool(name)
		if !ok {
			t.Fatalf("ToolNames 列了 %q，registry 里却没有", name)
		}
		if tool.Label == "" {
			t.Fatalf("%s 没有给她看的名字", name)
		}
		for _, bad := range jargon {
			if tool.Label == bad || contains(tool.Label, bad) {
				t.Fatalf("%s 的名字 %q 里有行话 %q", name, tool.Label, bad)
			}
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// ToolNames 和 registry 不该各说各的——目录从表派生是这个项目的一条硬约束。
func TestToolNames_MatchesRegistry(t *testing.T) {
	names := pbl.ToolNames()
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			t.Fatalf("%q 列了两次", n)
		}
		seen[n] = true
		if _, ok := pbl.LookupTool(n); !ok {
			t.Fatalf("%q 不在 registry 里", n)
		}
	}
	for _, n := range []string{"observe", "board", "reframe", "ideas", "review",
		"decide", "structure", "split", "lookback", "keep"} {
		if !seen[n] {
			t.Fatalf("registry 有 %q，ToolNames 漏了", n)
		}
	}
}
