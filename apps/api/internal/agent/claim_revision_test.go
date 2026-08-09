package agent

import "testing"

func TestParseClaimRevisionVerdict(t *testing.T) {
	t.Run("rephrase", func(t *testing.T) {
		v, err := parseClaimRevisionVerdict(`{"kind":"rephrase","why":"只是说法更清楚"}`)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if v.Kind != "rephrase" {
			t.Fatalf("kind = %q", v.Kind)
		}
	})

	t.Run("total_change with fences", func(t *testing.T) {
		v, err := parseClaimRevisionVerdict("```json\n{\"kind\":\"total_change\",\"why\":\"换了一个对象\"}\n```")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if v.Kind != "total_change" {
			t.Fatalf("kind = %q", v.Kind)
		}
	})

	t.Run("bad kind rejected", func(t *testing.T) {
		if _, err := parseClaimRevisionVerdict(`{"kind":"tweak","why":"x"}`); err == nil {
			t.Fatal("expected error for bad kind")
		}
	})

	t.Run("empty why rejected", func(t *testing.T) {
		if _, err := parseClaimRevisionVerdict(`{"kind":"rephrase","why":"  "}`); err == nil {
			t.Fatal("expected error for empty why")
		}
	})

	t.Run("no json rejected", func(t *testing.T) {
		if _, err := parseClaimRevisionVerdict("sorry I cannot"); err == nil {
			t.Fatal("expected error for no json")
		}
	})
}
