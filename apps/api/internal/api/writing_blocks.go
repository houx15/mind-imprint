package api

import (
	"sort"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/store/sqlc"
)

// writingMaterialDepth is the outline depth at and below which a node is her
// material (an example, a number, a source), not a paragraph of its own.
// Mirrors apps/lite-web/src/writings/slots.ts MATERIAL_DEPTH.
const writingMaterialDepth = 2

// writingBlockNumbers numbers the blocks the 段落 screen shows, keyed by
// snippet id, with the rule slots.ts uses (buildSlots):
//
//   - every outline node is a block, in position order, except a material
//     node (depth ≥ 2) with no written snippet that has a block above it —
//     that one folds into the paragraph above;
//   - snippets not linked to a current outline node follow, by position;
//   - numbers are 1-based, in that order.
//
// 🚨 2026-09-18：块原来按 position+1 编号，而 position 是结构图里的位置。
// 材料节点并进上一段之后，屏幕上的号改成连续的顺序号；印记 说「第 N 块」
// 必须和屏幕对得上，所以两边用同一条规则。
func writingBlockNumbers(outline []sqlc.WritingOutline, snippets []sqlc.WritingSnippet) map[uuid.UUID]int {
	sorted := append([]sqlc.WritingOutline(nil), outline...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Position < sorted[j].Position })
	snippetOf := map[uuid.UUID]sqlc.WritingSnippet{}
	for _, s := range snippets {
		if s.OutlineID.Valid {
			id := uuid.UUID(s.OutlineID.Bytes)
			if _, seen := snippetOf[id]; !seen {
				snippetOf[id] = s
			}
		}
	}
	inOutline := map[uuid.UUID]bool{}
	for _, o := range sorted {
		inOutline[o.ID] = true
	}

	out := map[uuid.UUID]int{}
	n := 0
	haveParent := false
	for _, o := range sorted {
		s, has := snippetOf[o.ID]
		written := has && strings.TrimSpace(s.Text) != ""
		material := o.Depth >= writingMaterialDepth
		if material && !written && haveParent {
			continue
		}
		n++
		if has {
			out[s.ID] = n
		}
		if !material {
			haveParent = true
		}
	}

	var free []sqlc.WritingSnippet
	for _, s := range snippets {
		if !s.OutlineID.Valid || !inOutline[uuid.UUID(s.OutlineID.Bytes)] {
			free = append(free, s)
		}
	}
	sort.SliceStable(free, func(i, j int) bool { return free[i].Position < free[j].Position })
	for _, s := range free {
		n++
		out[s.ID] = n
	}
	return out
}
