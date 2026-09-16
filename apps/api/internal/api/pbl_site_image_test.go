package api_test

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"strings"
	"testing"
)

func TestSiteImageOwnershipAndStructureRetention(t *testing.T) {
	h, c, _, pool := liteHandler(t)
	pid := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	var owner string
	if err := pool.QueryRow(context.Background(), `SELECT user_id::text FROM atom WHERE id=$1`, pid).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(context.Background(), `INSERT INTO pbl_tree_node(atom_id,tree,depth,ordinal,title,body,author) VALUES($1,'main',0,0,'树叶照片','','student')`, pid)
	if err != nil {
		t.Fatal(err)
	}
	rec := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/site-structure", "")
	var result struct {
		Draft struct{ Sections []struct{ Key string } }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Draft.Sections) != 1 {
		t.Fatal(rec.Body)
	}
	endpoint := "/api/v1/pbl/site/sections/" + result.Draft.Sections[0].Key + "/image"
	own := "users/" + owner + "/images/test.png"
	for _, key := range []string{"users/" + uuid.NewString() + "/images/test.png", "users/" + owner + "/images/../docs/private.pdf", "https://example.com/picture.png"} {
		body, _ := json.Marshal(map[string]string{"objectKey": key})
		if rec := siteReq(t, h, c, "PUT", endpoint, string(body)); rec.Code != 400 {
			t.Fatalf("accepted foreign/invalid key: %d %s", rec.Code, rec.Body)
		}
	}
	body, _ := json.Marshal(map[string]string{"objectKey": own})
	rec = siteReq(t, h, c, "PUT", endpoint, string(body))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), own) {
		t.Fatalf("own image: %d %s", rec.Code, rec.Body)
	}
	rec = siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/site-structure", "")
	if !strings.Contains(rec.Body.String(), own) {
		t.Fatal("structure confirmation lost image")
	}
	rec = siteReq(t, h, c, "PUT", endpoint, `{"objectKey":""}`)
	if rec.Code != 200 || strings.Contains(rec.Body.String(), own) {
		t.Fatal("image removal failed")
	}
}
