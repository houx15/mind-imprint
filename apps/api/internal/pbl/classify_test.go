package pbl

import "testing"

// parseKind is the boundary between a model's free text and a column with a
// CHECK constraint on it. Everything it lets through reaches the database, so
// what matters is what it REFUSES.
func TestParseKind(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		bad  bool
	}{
		{"plain json", `{"kind":"research"}`, "research", false},
		{"fenced", "```json\n{\"kind\":\"design\"}\n```", "design", false},
		{"prose around it", "好的。{\"kind\":\"making\"} 就这样", "making", false},
		{"uppercase", `{"kind":"WEBSITE"}`, "website", false},
		{"padded", `{"kind":"  investigation  "}`, "investigation", false},
		{"unknown kind", `{"kind":"podcast"}`, "", true},
		{"empty kind", `{"kind":""}`, "", true},
		{"empty input", ``, "", true},
		{"not json", `research`, "", true},
		{"truncated json", `{"kind":"res`, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseKind(c.in)
			if c.bad {
				if err == nil {
					t.Fatalf("parseKind(%q) = %q, want an error", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseKind(%q): %v", c.in, err)
			}
			if got != c.want {
				t.Fatalf("parseKind(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// The Go list and the CHECK constraint in 0108 must name the same five things.
func TestProjectKindsMatchesSchema(t *testing.T) {
	want := map[string]bool{
		"website": true, "research": true, "design": true,
		"making": true, "investigation": true,
	}
	if len(ProjectKinds) != len(want) {
		t.Fatalf("ProjectKinds = %v, want %d kinds", ProjectKinds, len(want))
	}
	for _, k := range ProjectKinds {
		if !want[k] {
			t.Fatalf("%q is in ProjectKinds but not in 0108's CHECK constraint", k)
		}
	}
	if IsProjectKind("podcast") {
		t.Fatal("IsProjectKind accepted a kind the database will reject")
	}
}

// spec §4 — 她还没有项目的时候，第一个项目就是做自己的主页。不是推荐，是它就是。
func TestResolveKindForcesWebsiteOnFirstProject(t *testing.T) {
	if got := ResolveKind(0, "research"); got != "website" {
		t.Fatalf("first project = %q, want website even though she described research", got)
	}
	if got := ResolveKind(0, "website"); got != "website" {
		t.Fatalf("first project = %q, want website", got)
	}
	if got := ResolveKind(1, "making"); got != "making" {
		t.Fatalf("second project = %q, want the detected kind", got)
	}
	if got := ResolveKind(3, "research"); got != "research" {
		t.Fatalf("fourth project = %q, want the detected kind", got)
	}
}
