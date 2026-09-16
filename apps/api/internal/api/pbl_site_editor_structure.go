package api

import (
	"context"

	"github.com/google/uuid"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

// Called with the project and site locked. Copy stays in site content; the
// structure stores identity/title only, never duplicates the student's body.
func syncEditedSiteSections(ctx context.Context, q *sqlc.Queries, user, atom uuid.UUID, before pbl.SiteDraft, after *pbl.SiteDraft) error {
	nodes, err := q.ListPblTreeNodes(ctx, sqlc.ListPblTreeNodesParams{AtomID: atom, Tree: "main"})
	if err != nil {
		return err
	}
	byKey := map[string]sqlc.PblTreeNode{}
	var ordinal int32
	for _, node := range nodes {
		byKey[pbl.SiteItemID(user.String(), node.ID.String())] = node
		if !node.ParentID.Valid && node.Ordinal >= ordinal {
			ordinal = node.Ordinal + 1
		}
	}
	old := map[string]pbl.SiteSection{}
	for _, section := range before.Sections {
		old[section.Key] = section
	}
	count := len(nodes)
	for i, section := range after.Sections {
		if node, ok := byKey[section.Key]; ok {
			if previous, exists := old[section.Key]; exists && section.Title != previous.Title {
				if _, err = q.UpdatePblTreeNode(ctx, sqlc.UpdatePblTreeNodeParams{ID: node.ID, Title: section.Title, Body: node.Body}); err != nil {
					return err
				}
			}
			continue
		}
		if _, exists := old[section.Key]; exists {
			continue
		} // Historical draft: do not invent a duplicate node.
		if count >= 60 {
			return httpx.ErrBadRequest("invalid_outline", "主页结构最多包含60个模块", nil)
		}
		node, err := q.CreatePblTreeNode(ctx, sqlc.CreatePblTreeNodeParams{AtomID: atom, Tree: "main", Ordinal: ordinal, Title: section.Title, Body: "", Author: "student"})
		if err != nil {
			return err
		}
		after.Sections[i].Key = pbl.SiteItemID(user.String(), node.ID.String())
		after.Sections[i].Depth = 0
		ordinal++
		count++
	}
	return nil
}
