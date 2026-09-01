package pbl

import (
	"errors"
	"testing"
)

func TestChildDepth(t *testing.T) {
	cases := []struct {
		name         string
		parentExists bool
		parentDepth  int
		want         int
		wantErr      error
	}{
		{"top level ignores parent depth", false, 0, 0, nil},
		{"first nesting", true, 0, 1, nil},
		{"second nesting", true, 1, 2, nil},
		{"a fourth level is refused", true, 2, 0, ErrTooDeep},
		{"a negative parent depth is a missing parent", true, -1, 0, ErrNoParent},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ChildDepth(c.parentExists, c.parentDepth)
			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Fatalf("err = %v, want %v", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Fatalf("depth = %d, want %d", got, c.want)
			}
		})
	}
}

// Go 这边的列表和数据库的 CHECK 必须一字不差。0109 定了五种；0111 又加了两
// 种——审阅里点开一句话开出来的那条线，和上线之后的每一轮维护。
func TestSessionKindsMatchSchema(t *testing.T) {
	want := map[string]bool{
		"free": true, "observation": true, "reframe": true,
		"brainstorm": true, "plan_check": true,
		"review": true, "keeping": true,
	}
	if len(SessionKinds) != len(want) {
		t.Fatalf("SessionKinds = %v", SessionKinds)
	}
	for _, k := range SessionKinds {
		if !want[k] {
			t.Fatalf("%q is in SessionKinds but not in the migration's CHECK", k)
		}
	}
	if IsSessionKind("retro") {
		t.Fatal("IsSessionKind accepted a kind the database will reject")
	}
}

// 🚨 The rule against 方法论表演: finishing the ritual while nothing about the
// project changed or was even recorded.
func TestValidateClose_RefusesAnEmptyWriteBack(t *testing.T) {
	if err := ValidateClose("free", WriteBack{Takeaway: "  "}); !errors.Is(err, ErrNoWriteBack) {
		t.Fatalf("a dig closed with no takeaway: %v", err)
	}
	if err := ValidateClose("free", WriteBack{Takeaway: "剩的主要是米饭，不是菜"}); err != nil {
		t.Fatalf("a dig with a takeaway should close: %v", err)
	}

	// A typed session needs its own contract, not just any prose.
	if err := ValidateClose("brainstorm", WriteBack{Takeaway: "聊得挺好"}); !errors.Is(err, ErrNoWriteBack) {
		t.Fatalf("brainstorm closed without a next_bet: %v", err)
	}
	if err := ValidateClose("brainstorm", WriteBack{
		Fields: map[string]string{"next_bet": "先做一版只有入口提示的"},
	}); err != nil {
		t.Fatalf("brainstorm with a next_bet should close: %v", err)
	}

	if err := ValidateClose("plan_check", WriteBack{
		Fields: map[string]string{"resolution": "kept"},
	}); !errors.Is(err, ErrNoWriteBack) {
		t.Fatal("plan_check closed with a resolution but no reason")
	}
	// 「保留原计划」 is a legitimate outcome — recorded, with her reason.
	if err := ValidateClose("plan_check", WriteBack{
		Fields: map[string]string{"resolution": "kept", "reason": "访谈只有一个人，先补证据"},
	}); err != nil {
		t.Fatalf("keeping the plan with a reason is a valid close: %v", err)
	}

	if err := ValidateClose("retro", WriteBack{Takeaway: "x"}); !errors.Is(err, ErrBadKind) {
		t.Fatalf("unknown kind should be refused: %v", err)
	}
}

// Every kind must declare a contract, even if it is empty — a kind nobody
// listed would silently close on nothing.
func TestEveryKindHasAContract(t *testing.T) {
	for _, k := range SessionKinds {
		if _, ok := requiredFields[k]; !ok {
			t.Fatalf("kind %q has no write-back contract", k)
		}
	}
}
