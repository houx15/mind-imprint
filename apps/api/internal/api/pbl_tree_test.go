package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
)

type treeNodeOut struct {
	ID       string  `json:"id"`
	ParentID *string `json:"parentId"`
	Depth    int16   `json:"depth"`
	Title    string  `json:"title"`
}

func decodeNode(t *testing.T, rec *httptest.ResponseRecorder) treeNodeOut {
	t.Helper()
	var out treeNodeOut
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode node: %v — body=%s", err, rec.Body)
	}
	return out
}

func addNode(t *testing.T, h http.Handler, c *http.Cookie, pid, body string) treeNodeOut {
	t.Helper()
	rec := pblPost(t, h, c, "/api/v1/pbl/projects/"+pid+"/tree", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create node = %d; body=%s", rec.Code, rec.Body)
	}
	return decodeNode(t, rec)
}

// 🚨 深度由服务端从父节点算出来。客户端说自己在第 0 层却挂在第 2 层下面，
// CHECK 拦得住越界，拦不住"深度和实际位置对不上"——之后整棵树的缩进就全错了。
func TestPblTree_DepthComesFromTheParent(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)

	root := addNode(t, h, cookie, pid, `{"title":"开头"}`)
	if root.Depth != 0 {
		t.Fatalf("root depth = %d, want 0", root.Depth)
	}
	// 请求里根本没有 depth 字段可以撒谎——但父节点决定一切，这里验证它。
	child := addNode(t, h, cookie, pid, `{"title":"我看到的","parentId":"`+root.ID+`"}`)
	if child.Depth != 1 {
		t.Fatalf("child depth = %d, want 1", child.Depth)
	}
	grand := addNode(t, h, cookie, pid, `{"title":"那天中午","parentId":"`+child.ID+`"}`)
	if grand.Depth != 2 {
		t.Fatalf("grandchild depth = %d, want 2", grand.Depth)
	}
}

// 分得太细就没人读得下去了。直说，比悄悄压平好。
func TestPblTree_RefusesToNestTooDeep(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)

	cur := addNode(t, h, cookie, pid, `{"title":"第0层"}`)
	for i := 1; i <= 3; i++ {
		cur = addNode(t, h, cookie, pid, `{"title":"再深一层","parentId":"`+cur.ID+`"}`)
	}
	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/tree",
		`{"title":"太深了","parentId":"`+cur.ID+`"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("fifth level = %d, want 400", rec.Code)
	}
}

// 🚨 挂到自己的子孙下面，会把这一支从树上切下来——它再也走不到根，界面上
// 就直接消失了。
func TestPblTree_RefusesToMoveANodeUnderItsOwnChild(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)

	parent := addNode(t, h, cookie, pid, `{"title":"开头"}`)
	child := addNode(t, h, cookie, pid, `{"title":"里面这块","parentId":"`+parent.ID+`"}`)
	move := "/api/v1/pbl/projects/" + pid + "/tree/" + parent.ID + "/move"

	if rec := pblPost(t, h, cookie, move, `{"parentId":"`+child.ID+`"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("move under own child = %d, want 400", rec.Code)
	}
	if rec := pblPost(t, h, cookie, move, `{"parentId":"`+parent.ID+`"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("move under itself = %d, want 400", rec.Code)
	}
	// 正当的移动照样可以：提到最外层。
	if rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/tree/"+child.ID+"/move",
		`{"parentId":""}`); rec.Code != http.StatusOK {
		t.Fatalf("outdent = %d; body=%s", rec.Code, rec.Body)
	}
}

func TestPblTree_ChecksAreAnsweredOncePerQuestion(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	url := "/api/v1/pbl/projects/" + pid + "/tree-checks"

	if rec := pblPost(t, h, cookie, url, `{"question":"随便什么","answer":"x"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown question = %d, want 400", rec.Code)
	}
	if rec := pblPost(t, h, cookie, url, `{"question":"covers","answer":"都在"}`); rec.Code != http.StatusOK {
		t.Fatalf("answer = %d; body=%s", rec.Code, rec.Body)
	}
	// 再答一次是改写，不是又来一条。
	if rec := pblPost(t, h, cookie, url, `{"question":"covers","answer":"还差结尾"}`); rec.Code != http.StatusOK {
		t.Fatalf("re-answer = %d", rec.Code)
	}

	rec := pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/tree", "")
	var got struct {
		Checks []struct {
			Question string `json:"question"`
			Answer   string `json:"answer"`
		} `json:"checks"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if len(got.Checks) != 1 || got.Checks[0].Answer != "还差结尾" {
		t.Fatalf("checks = %+v —— 同一个问题应该只有一条", got.Checks)
	}
}

func TestPblTree_OtherStudentGets404(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	root := addNode(t, h, cookie, pid, `{"title":"开头"}`)

	otherID := createStudent(t, pool, SeedSchoolID, "other-tree@demo.local")
	other := signInAs(t, pool, otherID)

	if rec := pblReq(t, h, other, "PATCH", "/api/v1/pbl/projects/"+pid+"/tree/"+root.ID,
		`{"title":"改一下"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("foreign patch = %d, want 404", rec.Code)
	}
}
