package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 产品负责人 2026-09-23：
//
//	「for writing, maybe we need to let the students select/talk with ai about
//	  what genre they are going to write. sometimes they are writing a 记叙文,
//	  sometimes 散文, sometimes 议论文, sometimes 书信.」
//
// 🚨 这几条钉的是**她说的话压过我们的猜测**，以及压过之后**全屋都跟着变**。
// 后者才是这一档的重点：文体在十二处被读到，而它们全都走 writingGenreOf ——
// 少接一处，她改完之后某一屏还在按旧文体教，那比不给她改更糟。

type genreBody struct {
	Genre   string `json:"genre"`
	Chosen  bool   `json:"chosen"`
	Choices []struct {
		ID    string `json:"id"`
		Label string `json:"label"`
		Blurb string `json:"blurb"`
	} `json:"choices"`
}

func getGenre(t *testing.T, h http.Handler, cookie *http.Cookie, id string) genreBody {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/writings/"+id+"/genre", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET genre = %d; body=%s", rec.Code, rec.Body)
	}
	var out genreBody
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, rec.Body)
	}
	return out
}

func putGenre(t *testing.T, h http.Handler, cookie *http.Cookie, id, genre string) genreBody {
	t.Helper()
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"genre":"` + genre + `"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/writings/"+id+"/genre", body), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT genre %q = %d; body=%s", genre, rec.Code, rec.Body)
	}
	var out genreBody
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, rec.Body)
	}
	return out
}

func TestSheCanSayWhatGenreThisIs(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	// 题目长得像一道议论文题 —— 推断会说 argument。
	id := createWritingAtomHTTP(t, h, cookie, "读书该快还是该慢")

	before := getGenre(t, h, cookie, id)
	if before.Genre != "argument" {
		t.Fatalf("推断出来是 %q，这条用例想要 argument", before.Genre)
	}
	// 🚨 推断出来的不算「她定的」。把推断说成是她的选择，
	// 是替她做主之后再赖给她。
	if before.Chosen {
		t.Error("她一个字都没说，chosen 却是 true")
	}
	if len(before.Choices) < 3 {
		t.Errorf("只给了 %d 种可选，想要至少三种", len(before.Choices))
	}
	for _, c := range before.Choices {
		if strings.TrimSpace(c.Blurb) == "" {
			t.Errorf("%s 没有说明 —— 「记叙文」三个字她未必读得懂", c.ID)
		}
	}

	after := putGenre(t, h, cookie, id, "letter")
	if after.Genre != "letter" {
		t.Errorf("她说了是书信，却仍然是 %q", after.Genre)
	}
	if !after.Chosen {
		t.Error("她定过之后 chosen 还是 false")
	}
	// 存下去了，下一次进来还是她说的那个。
	if again := getGenre(t, h, cookie, id); again.Genre != "letter" || !again.Chosen {
		t.Errorf("重新取回来是 %q（chosen=%v）", again.Genre, again.Chosen)
	}
}

// 🚨 收回是可以的：空串回到推断。一条单向的门迟早要靠「再开一篇」绕过去。
func TestSheCanTakeHerChoiceBack(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createWritingAtomHTTP(t, h, cookie, "读书该快还是该慢")
	if got := putGenre(t, h, cookie, id, "narrative"); got.Genre != "narrative" {
		t.Fatalf("没定上：%q", got.Genre)
	}
	back := putGenre(t, h, cookie, id, "")
	if back.Chosen {
		t.Error("收回之后 chosen 还是 true")
	}
	if back.Genre != "argument" {
		t.Errorf("收回之后该回到推断（argument），得到 %q", back.Genre)
	}
}

// 写错的 id 当成「她没说」，不把她锁在某一种文体上，也不报错吓她一跳。
func TestAnUnknownGenreFallsBackToInference(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createWritingAtomHTTP(t, h, cookie, "读书该快还是该慢")
	got := putGenre(t, h, cookie, id, "我编的一种")
	if got.Chosen {
		t.Error("一个编出来的取值被当成了她的选择")
	}
	if got.Genre != "argument" {
		t.Errorf("得到 %q，想要回到推断", got.Genre)
	}
	// 🚨 阅读面那两种也不收：她写不出一篇「新闻报道体」交上去。
	for _, bad := range []string{"report", "explain"} {
		if out := putGenre(t, h, cookie, id, bad); out.Chosen {
			t.Errorf("%q 被当成了一种写作文体", bad)
		}
	}
}

// 🚨 **这一条是整档的重点**：她改完之后，真正发出去的提示词跟着换。
//
// 文体在十二处被读到，它们全都走 writingGenreOf。这条判据走的是其中最要紧的
// 一处（立题那一轮的 system prompt），因为那是她改完之后第一眼会看到差别的地方。
func TestChangingTheGenreChangesWhatTheCoachIsTold(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createWritingAtomHTTP(t, h, cookie, "读书该快还是该慢")

	putGenre(t, h, cookie, id, "letter")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET",
		"/api/v1/writings/"+id+"/flow/structures", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET structures = %d; body=%s", rec.Code, rec.Body)
	}
	var flow struct {
		Structures []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"structures"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &flow); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// 一封信不该再被摆出总分式 / 并列式 / 层进式 / 对照式。
	for _, s := range flow.Structures {
		if strings.HasPrefix(s.ID, "struct_total_part") ||
			strings.HasPrefix(s.ID, "struct_parallel") ||
			strings.HasPrefix(s.ID, "struct_progressive") ||
			strings.HasPrefix(s.ID, "struct_contrast") {
			t.Errorf("她说了这是一封信，行文那一屏还在摆议论文的结构：%s（%s）", s.ID, s.Name)
		}
	}
}
