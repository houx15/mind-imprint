package api

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

// Called inside the same transaction as the draft update. Module keys are
// scoped to the owner's actual tree; unknown keys abort the entire change.
func removeSiteSections(ctx context.Context, q *sqlc.Queries, owner, atom uuid.UUID, draft pbl.SiteDraft, keys []string) (pbl.SiteDraft, error) {
	nodes, err := q.ListPblTreeNodes(ctx, sqlc.ListPblTreeNodesParams{AtomID: atom, Tree: "main"})
	if err != nil {
		return draft, err
	}
	byKey := map[string]uuid.UUID{}
	for _, n := range nodes {
		byKey[pbl.SiteItemID(owner.String(), n.ID.String())] = n.ID
	}
	remove := map[uuid.UUID]bool{}
	for _, key := range keys {
		id, ok := byKey[key]
		if !ok {
			return draft, fmt.Errorf("移除模块失败：模块不存在，请重新读取结构")
		}
		remove[id] = true
	}
	for changed := true; changed; {
		changed = false
		for _, n := range nodes {
			if n.ParentID.Valid && remove[uuid.UUID(n.ParentID.Bytes)] && !remove[n.ID] {
				remove[n.ID] = true
				changed = true
			}
		}
	}
	if len(remove) == len(nodes) {
		return draft, fmt.Errorf("移除模块失败：请保留至少一个模块")
	}
	keep := make([]pbl.SiteSection, 0, len(draft.Sections))
	for _, s := range draft.Sections {
		if !remove[byKey[s.Key]] {
			keep = append(keep, s)
		}
	}
	// Children first supports either CASCADE or RESTRICT parent constraints.
	for depth := 60; depth >= 0; depth-- {
		for _, n := range nodes {
			if remove[n.ID] && int(n.Depth) == depth {
				if err := q.DeletePblTreeNode(ctx, n.ID); err != nil {
					return draft, err
				}
			}
		}
	}
	draft.Sections = keep
	return draft, nil
}
