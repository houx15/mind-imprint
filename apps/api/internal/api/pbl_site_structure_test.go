package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestSiteStructure_AppliesOrderPreservesTextAndExcludesPlanningNotes(t *testing.T) {
	h, c, _, pool := liteHandler(t)
	id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	first, second, child := uuid.New(), uuid.New(), uuid.New()
	ctx := context.Background()
	for _, n := range []struct {
		id             uuid.UUID
		parent         any
		depth, ordinal int
		title          string
	}{
		{first, nil, 0, 0, "观察问题"}, {second, nil, 0, 1, "拍摄邀请"}, {child, first, 1, 0, "叶片为什么变黄"},
	} {
		_, err := pool.Exec(ctx, `INSERT INTO pbl_tree_node(id,atom_id,tree,parent_id,depth,ordinal,title,body,author) VALUES($1,$2,'main',$3,$4,$5,$6,'PRIVATE planning advice','yinji')`, n.id, id, n.parent, n.depth, n.ordinal, n.title)
		if err != nil {
			t.Fatal(err)
		}
	}
	url := "/api/v1/pbl/projects/" + id + "/site-structure"
	rec := siteReq(t, h, c, "POST", url, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("apply: %d %s", rec.Code, rec.Body)
	}
	var state struct {
		Draft   map[string]any `json:"draft"`
		Content struct {
			Sections []struct {
				Key, Title, Body string
				Depth            int
			} `json:"sections"`
		} `json:"content"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if len(state.Content.Sections) != 3 || state.Content.Sections[1].Title != "叶片为什么变黄" || state.Content.Sections[1].Depth != 1 {
		t.Fatalf("bad order: %s", rec.Body)
	}
	if strings.Contains(rec.Body.String(), "PRIVATE") || strings.Contains(rec.Body.String(), first.String()) {
		t.Fatal("planning note or database id leaked")
	}
	sections := state.Draft["sections"].([]any)
	sections[0].(map[string]any)["body"] = "这是我观察到的变化。"
	blob, _ := json.Marshal(state.Draft)
	if rec = siteReq(t, h, c, "PUT", "/api/v1/pbl/site/content", string(blob)); rec.Code != http.StatusOK {
		t.Fatal(rec.Body)
	}
	if _, err := pool.Exec(ctx, `UPDATE pbl_tree_node SET ordinal=-1,title='参与拍摄' WHERE id=$1`, second); err != nil {
		t.Fatal(err)
	}
	// Remove the child: applying again must remove it from the page, not append.
	if _, err := pool.Exec(ctx, `DELETE FROM pbl_tree_node WHERE id=$1`, child); err != nil {
		t.Fatal(err)
	}
	rec = siteReq(t, h, c, "POST", url, "")
	if err := json.Unmarshal(rec.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if len(state.Content.Sections) != 2 || state.Content.Sections[0].Title != "参与拍摄" || state.Content.Sections[1].Body != "这是我观察到的变化。" {
		t.Fatalf("lost edits/order: %s", rec.Body)
	}
}
