package api

// interest_harvest_text_test.go —— 一次正常走完的阅读，采集拿得到她的话吗。
//
// 2026-09-08 的全链路走查（`apps/lite-web/e2e/full-loop-walk.spec.ts`）卡在这里，
// 而且卡得很安静：她从探索地图进来、粘了正文、跟印记聊了两轮、按了完成，报告
// 出来了 —— 报告上那一节「可以加进你的兴趣树」**整个不显示**。
//
// 库里是这样的：`atom_message` 有她两条，`reading_takeaway` 零行，
// `interest_keyword` 零行，`interest_proposal` 零行，而 `llm_call` 里连一次
// `interest_harvest` 都没有。任务跑完了、章也盖上了，模型根本没被调用 ——
// `gatherHarvestText` 交回来的是空字符串，`harvestOneAtom` 就在调用前返回了。
//
// 原因是它只读两样东西，而这两样今天**都是空的**：
//
//   - 「我的收获」那张归纳表已经删掉了（ReadingRoom.tsx 里产品负责人的原话：
//     阅读这条路上的步骤已经够多，不必再让她填一次表）；
//   - 阅读室至今没有写批注的控件，批注是只读的。
//
// 于是「阅读」这条路永远长不出词，而探索地图上明明写着「读完之后，报告上会提出
// 可以加进你树里的词」。这条测试守的就是这句话：**一次只留下对话的阅读，也必须
// 是可采的。**
//
// 🚨 这类错读代码看不出来 —— `gatherHarvestText` 每一行都对，错在它没读的那张表。

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/store/sqlc"
)

// mkReadingWithMessages 建一篇属于种子学生的阅读，只往主线程里放几条消息 ——
// 没有收获，没有批注，正是她走完一次阅读之后库里真实的样子。
func mkReadingWithMessages(t *testing.T, pool *pgxpool.Pool, msgs [][2]string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(t.Context(),
		`INSERT INTO atom (kind, user_id) VALUES ('reading', $1) RETURNING id`,
		SeedUserID).Scan(&id); err != nil {
		t.Fatalf("insert atom: %v", err)
	}
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO reading (atom_id, title, lang, status)
		 VALUES ($1, '太阳能的十年', 'zh', 'finished')`, id); err != nil {
		t.Fatalf("insert reading: %v", err)
	}
	for i, m := range msgs {
		if _, err := pool.Exec(t.Context(),
			`INSERT INTO atom_message (atom_id, seq, role, content) VALUES ($1, $2, $3, $4)`,
			id, i+1, m[0], m[1]); err != nil {
			t.Fatalf("insert atom_message: %v", err)
		}
	}
	return id
}

func TestGatherHarvestText_ReadsWhatSheSaidToTheCoach(t *testing.T) {
	pool := NewTestDB(t)
	a := New(Deps{Queries: sqlc.New(pool), Pool: pool, CookieSecure: false})

	hers := "我最在意的是储能：如果白天多出来的电存不下来，那装机再多是不是就没意义了？"
	his := "这是一个很好的切入点，我们先看看第三段是怎么说错位的。"
	id := mkReadingWithMessages(t, pool, [][2]string{
		{"ai", "让我来带你详细读一遍这篇文章。"},
		{"student", hers},
		{"ai", his},
	})

	title, body := a.gatherHarvestText(t.Context(), id, "reading")
	if title != "太阳能的十年" {
		t.Errorf("title = %q", title)
	}
	// 这一条就是那个 bug：body 空着，采集在调用模型之前就返回，章却已经盖了。
	if strings.TrimSpace(body) == "" {
		t.Fatal("一次只留下对话的阅读采不到任何东西 —— 阅读这条路长不出词，报告上那一节整个不显示")
	}
	if !strings.Contains(body, hers) {
		t.Errorf("她说的话没进去：\n%s", body)
	}
	// 印记说的话不是她的话。喂回去采出来的会是印记的用词，而整棵树的说法是
	// 「这就是你的模型」。
	if strings.Contains(body, his) {
		t.Errorf("印记说的话被当成她的话喂进采集了：\n%s", body)
	}
}

// mkProjectWithIdea 建一个属于种子学生的项目。assigned=true 时 idea 是老师布置的
// 驱动问题。
func mkProjectWithIdea(t *testing.T, pool *pgxpool.Pool, idea string, assigned bool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(t.Context(),
		`INSERT INTO atom (kind, user_id) VALUES ('project', $1) RETURNING id`,
		SeedUserID).Scan(&id); err != nil {
		t.Fatalf("insert atom: %v", err)
	}
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO pbl_project (atom_id, idea, kind, name, assigned) VALUES ($1, $2, '', '杯子', $3)`,
		id, idea, assigned); err != nil {
		t.Fatalf("insert pbl_project: %v", err)
	}
	return id
}

// 老师布置的驱动问题不是她的立题。采进去，树上长出来的就是老师的词。
func TestGatherHarvestText_SkipsAssignedProjectIdea(t *testing.T) {
	pool := NewTestDB(t)
	a := New(Deps{Queries: sqlc.New(pool), Pool: pool, CookieSecure: false})

	teachers := "怎样让校园少用一次性杯子？"
	if _, body := a.gatherHarvestText(t.Context(), mkProjectWithIdea(t, pool, teachers, true), "project"); strings.Contains(body, teachers) {
		t.Errorf("老师布置的驱动问题被当成她的立题采进去了：\n%s", body)
	}

	hers := "我想让食堂的剩饭少一半"
	if _, body := a.gatherHarvestText(t.Context(), mkProjectWithIdea(t, pool, hers, false), "project"); !strings.Contains(body, hers) {
		t.Errorf("她自己的立题没进去：\n%s", body)
	}
}

// 她一个字都没说过的阅读仍然采不到东西 —— 这是对的，不要为了让上面那条过去
// 就把正文喂进来。喂正文采出来的是文章的词，不是她的词。
func TestGatherHarvestText_SilentReadingStaysEmpty(t *testing.T) {
	pool := NewTestDB(t)
	a := New(Deps{Queries: sqlc.New(pool), Pool: pool, CookieSecure: false})

	id := mkReadingWithMessages(t, pool, [][2]string{
		{"ai", "让我来带你详细读一遍这篇文章。"},
	})
	if _, body := a.gatherHarvestText(t.Context(), id, "reading"); strings.TrimSpace(body) != "" {
		t.Fatalf("她什么都没说，却采到了东西：\n%s", body)
	}
}
