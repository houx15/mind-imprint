package onboarding

import "testing"

func TestLoad0457(t *testing.T) {
	f, ok := Load("0457")
	if !ok {
		t.Fatal("Load(\"0457\") = false, want a fixture")
	}
	if len(f.RestatePrompt) < 10 {
		t.Errorf("RestatePrompt too short: %q", f.RestatePrompt)
	}
	if len(f.Rows) != 4 {
		t.Fatalf("Rows = %d, want 4", len(f.Rows))
	}
	if f.Rows[0].Official == "" || f.Rows[0].Plain == "" {
		t.Errorf("row 0 has empty official/plain: %+v", f.Rows[0])
	}
	if len(f.Steps) == 0 {
		t.Error("Steps empty")
	}
	wantOfficial := []string{"来源与证据（表D）", "分析（表E）", "评估（表F）", "表达与组织（表H）"}
	for i, w := range wantOfficial {
		if f.Rows[i].Official != w {
			t.Errorf("row %d official = %q, want %q (review_criteria order)", i, f.Rows[i].Official, w)
		}
	}
}

func TestLoadUnknownQualification(t *testing.T) {
	if _, ok := Load("9999"); ok {
		t.Error("Load(\"9999\") = true, want false (only 0457 exists)")
	}
}
