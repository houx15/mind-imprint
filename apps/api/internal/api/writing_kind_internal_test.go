package api

import (
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/store/sqlc"
)

func TestWritingKindDepth(t *testing.T) {
	cases := map[string]int32{
		writingKindOpening: 0, writingKindThesis: 0, writingKindClosing: 0,
		writingKindPoint: 1, writingKindCounter: 1,
		writingKindEvidence: 2, writingKindRebuttal: 2, writingKindGap: 2,
	}
	for k, want := range cases {
		if got := writingKindDepth(k); got != want {
			t.Errorf("%s: depth = %d, want %d", k, got, want)
		}
	}
}

func TestWritingKindLabel(t *testing.T) {
	if got := writingKindLabel(writingKindEvidence, ""); got != "论据 · 你见过的事" {
		t.Errorf("evidence without source = %q", got)
	}
	if got := writingKindLabel(writingKindEvidence, "Lu 2008"); got != "论据 · 你找来的材料" {
		t.Errorf("evidence with source = %q", got)
	}
	if got := writingKindLabel(writingKindClosing, ""); got != "结尾" {
		t.Errorf("closing = %q", got)
	}
	if got := writingKindLabel(writingKindGap, ""); got != "待补的材料" {
		t.Errorf("gap = %q", got)
	}
}

// 🚨 意见 3 的回归用例：模型把结尾挂在中心论点底下（深度 1），
// 它仍然必须是一个深度 0 的结尾，而不是「分论点 3」。
func TestWritingKindParentIgnoresModelPlacement(t *testing.T) {
	thesis := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindThesis, Depth: 0, Position: 0}
	point := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindPoint, Depth: 1, Position: 1}
	rows := []sqlc.WritingOutline{thesis, point}

	if p := writingKindParentOf(writingKindClosing, rows); p != nil {
		t.Errorf("closing parent = %v, want nil (top level)", p.ID)
	}
	if p := writingKindParentOf(writingKindPoint, rows); p == nil || p.ID != thesis.ID {
		t.Errorf("point parent = %v, want the thesis", p)
	}
	if p := writingKindParentOf(writingKindEvidence, rows); p == nil || p.ID != point.ID {
		t.Errorf("evidence parent = %v, want the nearest point", p)
	}
	// 一条论据来了，图上还没有分论点 —— 不许挂到中心论点上冒充一条理由。
	if p := writingKindParentOf(writingKindEvidence, []sqlc.WritingOutline{thesis}); p != nil {
		t.Errorf("evidence with no point = %v, want nil", p)
	}
}

// 论据挂在**最后**一条分论点上，不是第一条：她一条一条往下说。
func TestWritingKindParentTakesTheLastPoint(t *testing.T) {
	rows := []sqlc.WritingOutline{
		{ID: uuid.New(), Kind: writingKindThesis, Depth: 0, Position: 0},
		{ID: uuid.New(), Kind: writingKindPoint, Depth: 1, Position: 1},
		{ID: uuid.New(), Kind: writingKindPoint, Depth: 1, Position: 2},
	}
	p := writingKindParentOf(writingKindEvidence, rows)
	if p == nil || p.Position != 2 {
		t.Errorf("evidence parent position = %v, want the last point (2)", p)
	}
}

func TestWritingKindFromRoleBackfill(t *testing.T) {
	cases := []struct {
		role  string
		depth int32
		want  string
	}{
		{"结尾", 1, writingKindClosing},
		{"开头", 0, writingKindOpening},
		{"你见过的事", 2, writingKindEvidence},
		{"一份研究", 1, writingKindEvidence},
		{"反方会说的话", 1, writingKindCounter},
		{"对反方的回应", 1, writingKindRebuttal},
		{"暂时还没找到的材料", 2, writingKindGap},
		{"中心论点", 0, writingKindThesis},
		{"一条理由", 1, writingKindPoint},
		{"完全不认识的东西", 0, writingKindThesis},
		{"完全不认识的东西", 1, writingKindPoint},
		{"完全不认识的东西", 2, writingKindEvidence},
	}
	for _, c := range cases {
		if got := writingKindFromRole(c.role, c.depth); got != c.want {
			t.Errorf("role=%q depth=%d: got %q want %q", c.role, c.depth, got, c.want)
		}
	}
}

// 老行（kind 为空）读出来必须是一个真的 kind，不能是空字符串 ——
// 空字符串会让下游的 switch 一路掉进 default。
func TestWritingKindOfFallsBackForLegacyRows(t *testing.T) {
	legacy := sqlc.WritingOutline{Role: "结尾", Depth: 1}
	if got := writingKindOf(legacy); got != writingKindClosing {
		t.Errorf("legacy row kind = %q, want closing", got)
	}
	fresh := sqlc.WritingOutline{Kind: writingKindPoint, Role: "随便什么", Depth: 0}
	if got := writingKindOf(fresh); got != writingKindPoint {
		t.Errorf("stored kind must win, got %q", got)
	}
}

func TestWritingKindGapIsNotMaterial(t *testing.T) {
	if writingKindIsMaterial(writingKindGap) {
		t.Error("gap must not count as material — it is a hole, not evidence")
	}
	if !writingKindIsMaterial(writingKindEvidence) {
		t.Error("evidence must count as material")
	}
}

func TestWritingKindAppliesTo(t *testing.T) {
	cases := map[string]string{
		writingKindOpening: "opening",
		writingKindClosing: "closing",
		writingKindPoint:   "body",
		writingKindCounter: "body",
		writingKindGap:     "body",
	}
	for k, want := range cases {
		if got := writingKindAppliesTo(k); got != want {
			t.Errorf("%s: appliesTo = %q, want %q", k, got, want)
		}
	}
}
