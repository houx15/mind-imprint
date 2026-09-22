package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"mindimprint/api/internal/store/sqlc"
)

// 摘抄是正文定下来之后才有的事。产品负责人 2026-09-22：
//
//	I think 摘抄 is a feature that after the text is decided.
//
// 只有摘要的那一篇（excerpt_only，从探索地图点开、三条取正文的路都没走通）
// 上不许摘抄。理由不是洁癖：摘抄存的是**字偏移**，而她随时会把全文粘进来
// 换掉这份摘要。在那之前摘下的一句，换完正文之后偏移指的是另一段话 —— 要么
// 她的摘抄被静默改写，要么（按 refuseIfAnchored 的现行规则）她因为摘过一句
// 而再也换不成正文，被锁在一篇两句话的摘要上。

func postAnnotation(t *testing.T, h http.Handler, cookie *http.Cookie, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/readings/"+id+"/annotations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, withCookie(req, cookie))
	return rec
}

const oneExcerpt = `{"blockId":"b1","span":{"start":0,"end":4},"quote":"只有一段","note":""}`

func TestExcerptRefusedWhileTheArticleIsOnlyAnAbstract(t *testing.T) {
	h, cookie, q, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)
	atom := uuid.MustParse(id)
	ctx := context.Background()

	// 从地图点开的那一篇：正文里放的只是导语。
	if _, err := q.UpsertReadingSource(ctx, sqlc.UpsertReadingSourceParams{
		AtomID: atom, Title: "栉水母", Body: "只有一段导语。", ExcerptOnly: true,
	}); err != nil {
		t.Fatal(err)
	}

	rec := postAnnotation(t, h, cookie, id, oneExcerpt)
	if rec.Code == http.StatusCreated {
		t.Fatalf("摘抄在只有摘要的那一篇上成功了 —— 她随后粘全文时会被自己这一句锁住")
	}
	if !strings.Contains(rec.Body.String(), "excerpt_only_source") {
		t.Fatalf("拒绝的理由要说得出口，得到：%d %s", rec.Code, rec.Body)
	}

	// 🚨 这条闸真正要保住的东西：她**还换得成正文**。
	// 摘抄没落库，所以 refuseIfAnchored 数不到证据，PUT /source 照常放行。
	putSourceHTTP(t, h, cookie, id, `{"title":"栉水母","text":"全文第一段。\n\n全文第二段。\n\n全文第三段。"}`)

	src, err := q.GetReadingSource(ctx, atom)
	if err != nil {
		t.Fatal(err)
	}
	if src.ExcerptOnly {
		t.Error("她粘了全文，还标着只有摘要")
	}

	// 正文定下来了 —— 现在摘抄成立。
	rec = postAnnotation(t, h, cookie, id, `{"blockId":"b1","span":{"start":0,"end":5},"quote":"全文第一段","note":""}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("正文定下来之后摘抄该成立，得到 %d：%s", rec.Code, rec.Body)
	}
}

// 普通的一篇（她自己粘进来的正文）不受这条闸影响 —— 写窄一点，别把正常那条
// 路也一起拦了。
func TestExcerptAllowedOnAPastedArticle(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)
	putSourceHTTP(t, h, cookie, id, `{"title":"太阳能","text":"第一段。\n\n第二段。"}`)

	rec := postAnnotation(t, h, cookie, id, `{"blockId":"b1","span":{"start":0,"end":3},"quote":"第一段","note":""}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST annotation = %d, want 201; body=%s", rec.Code, rec.Body)
	}
}
