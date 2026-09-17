package quotematch

import "testing"

func TestLocateReturnsTheVerbatimSpan(t *testing.T) {
	src := "First, working in group can let students learn from each other. For example, last semester my class did a project.\n\n上周五我在食堂门口数了一下，有六个桶是满的。"
	cases := []struct {
		q, want string
		ok      bool
	}{
		// exact
		{"For example, last semester my class did a project.", "For example, last semester my class did a project", true},
		// missing trailing period, different case, doubled space
		{"first,  working in group can let students learn from each other", "First, working in group can let students learn from each other", true},
		// Chinese, comma dropped
		{"上周五我在食堂门口数了一下有六个桶是满的", "上周五我在食堂门口数了一下，有六个桶是满的", true},
		// not there
		{"working in groups is always better", "", false},
		// only punctuation
		{"。。", "", false},
	}
	for _, c := range cases {
		got, ok := Locate(src, c.q)
		if ok != c.ok || got != c.want {
			t.Errorf("Locate(%q) = %q, %v; want %q, %v", c.q, got, ok, c.want, c.ok)
		}
	}
}
