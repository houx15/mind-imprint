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

// 只有摘要的那一篇被她粘成全文之后（产品负责人 2026-09-17），旧的读法清单和
// 导读都是按那两段摘要排的 —— b1 已经不是原来那一段。正文换了就清掉；同一份
// 正文再存一次不动。

func putSourceHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, id, body string) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/v1/readings/"+id+"/source", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, withCookie(req, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT source = %d: %s", rec.Code, rec.Body)
	}
}

func TestPastingTheFullTextClearsThePlanBuiltOnTheAbstract(t *testing.T) {
	h, cookie, q, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)
	ctx := context.Background()
	atom := uuid.MustParse(id)

	putSourceHTTP(t, h, cookie, id, `{"title":"栉水母","text":"只有一段导语。\n\nSource"}`)
	// 在摘要上排好的一份清单和一份导读。
	if _, err := q.ReplaceReadingTasks(ctx, sqlc.ReplaceReadingTasksParams{
		AtomID: atom, Positions: []int32{0, 1}, Kinds: []string{"predict", "read"},
		Labels: []string{"先预测", "通读第1–2段·全文"}, Details: []string{"", ""}, BlockIds: []string{"", "b1"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpdateReadingSourceOutline(ctx, sqlc.UpdateReadingSourceOutlineParams{
		AtomID: atom, Outline: []byte(`{"oneLine":"栉水母凭什么是奇迹？","load":{"b1":"core"}}`),
	}); err != nil {
		t.Fatal(err)
	}

	// 同一份正文再存一次：清单、导读都不动。
	putSourceHTTP(t, h, cookie, id, `{"title":"栉水母","text":"只有一段导语。\n\nSource"}`)
	if tasks, _ := q.ListReadingTasks(ctx, atom); len(tasks) != 2 {
		t.Fatalf("正文没变却把清单清了：%d 步", len(tasks))
	}

	// 粘成全文。
	putSourceHTTP(t, h, cookie, id, `{"title":"栉水母","text":"全文第一段。\n\n全文第二段。\n\n全文第三段。"}`)
	tasks, err := q.ListReadingTasks(ctx, atom)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Errorf("粘成全文之后，按摘要排的清单还留着 %d 步 —— 它指的段落已经变了", len(tasks))
	}
	src, err := q.GetReadingSource(ctx, atom)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(src.Outline), "栉水母凭什么") {
		t.Errorf("按摘要写的导读还留着：%s", src.Outline)
	}
	if src.ExcerptOnly {
		t.Error("她粘了全文，还标着只有摘要")
	}
}
