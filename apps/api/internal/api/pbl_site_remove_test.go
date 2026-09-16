package api_test

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"strings"
	"testing"
)

func TestSiteRemoveUpdatesTreeAndDraftAtomically(t *testing.T) {
	prov := pblStub(`{"reply":"测试"}`)
	h, c, _, pool := liteHandlerWithProvider(t, prov)
	pid := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	parent, child, keep := uuid.New(), uuid.New(), uuid.New()
	ctx := context.Background()
	for i, n := range []struct {
		id     uuid.UUID
		parent any
		depth  int
		title  string
	}{{parent, nil, 0, "空照片"}, {child, parent, 1, "照片说明"}, {keep, nil, 0, "观察方法"}} {
		_, err := pool.Exec(ctx, `INSERT INTO pbl_tree_node(id,atom_id,tree,parent_id,depth,ordinal,title,body,author) VALUES($1,$2,'main',$3,$4,$5,$6,'','student')`, n.id, pid, n.parent, n.depth, i, n.title)
		if err != nil {
			t.Fatal(err)
		}
	}
	rec := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/site-structure", "")
	var state struct {
		Draft struct {
			Sections []struct {
				Key string `json:"key"`
			} `json:"sections"`
		} `json:"draft"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if len(state.Draft.Sections) != 3 {
		t.Fatal(rec.Body)
	}
	key := state.Draft.Sections[0].Key
	*prov = *pblStub(coachProducing("site_content", `{"removeSectionKeys":["`+key+`"]}`))
	siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"请移除空照片模块和它的子模块"}`)
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pbl_tree_node WHERE atom_id=$1`, pid).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("tree not updated: %d", count)
	}
	rec = siteReq(t, h, c, "GET", "/api/v1/pbl/site", "")
	if strings.Contains(rec.Body.String(), "空照片") || !strings.Contains(rec.Body.String(), "观察方法") {
		t.Fatal(rec.Body)
	}
	// Invalid key plus a real key must roll back the entire edit.
	key = state.Draft.Sections[2].Key
	*prov = *pblStub(coachProducing("site_content", `{"removeSectionKeys":["`+key+`","unknown"]}`))
	siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"测试无效修改"}`)
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pbl_tree_node WHERE atom_id=$1`, pid).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("invalid edit partially applied")
	}
}
