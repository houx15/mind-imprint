package store_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/store/sqlc"
)

// TestAtomMessagePayload_RoundTrips — 0106 的 payload 真的能一路走到 Postgres
// 再走回来。这个测试存在的理由不是覆盖率：go build 只检查 Go，不检查 SQL，上
// 一期就有一条编译得干干净净、跑起来 SQLSTATE 42P08 的查询，直到很久以后才被
// 一个真的执行它的调用方撞出来。所以每一条新增/改动的查询都得在真库上跑一次。
//
// 一起断言的三件事：带 payload 的消息读回来内容不变；不带 payload 的消息读回来
// 是 nil（而不是 []byte("null") 这种假空）；段落子对话的两条查询——它们的
// SELECT 列表也被这一列改动了——同样被真的执行一次。
func TestAtomMessagePayload_RoundTrips(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	var uid uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM users LIMIT 1`).Scan(&uid); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	a, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "reading", UserID: uid})
	if err != nil {
		t.Fatalf("CreateAtom: %v", err)
	}

	card := []byte(`{"kind":"pick_one","prompt":"哪一句在给出证据？","options":["> 2023 年的排放量比上一年高 2.4%。","> 这让人担心。"]}`)
	answer := []byte(`{"picked":0}`)

	// 绝大多数消息什么也不带：payload 这一列压根没被写过。
	if _, err := q.AppendAtomMessage(ctx, sqlc.AppendAtomMessageParams{
		AtomID: a.ID, Seq: 1, Role: "student", Content: "这段在讲什么？",
	}); err != nil {
		t.Fatalf("AppendAtomMessage (no payload): %v", err)
	}
	// AI 侧：这条消息随身带着一张卡片。
	withCard, err := q.AppendAtomMessage(ctx, sqlc.AppendAtomMessageParams{
		AtomID: a.ID, Seq: 2, Role: "ai", Content: "先挑一句。", Payload: card,
	})
	if err != nil {
		t.Fatalf("AppendAtomMessage (payload): %v", err)
	}
	if !sameJSONPayload(t, withCard.Payload, card) {
		t.Fatalf("RETURNING payload = %s, want %s", withCard.Payload, card)
	}
	// 学生侧：她在卡片上的回答也是结构化的。
	if _, err := q.AppendAtomMessage(ctx, sqlc.AppendAtomMessageParams{
		AtomID: a.ID, Seq: 3, Role: "student", Content: "第一句", Payload: answer,
	}); err != nil {
		t.Fatalf("AppendAtomMessage (answer): %v", err)
	}

	msgs, err := q.ListAtomMessages(ctx, a.ID)
	if err != nil {
		t.Fatalf("ListAtomMessages: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("messages = %d, want 3", len(msgs))
	}
	if msgs[0].Payload != nil {
		t.Fatalf("a message written without a payload read back as %s, want nil", msgs[0].Payload)
	}
	if !sameJSONPayload(t, msgs[1].Payload, card) {
		t.Fatalf("card payload = %s, want %s", msgs[1].Payload, card)
	}
	if !sameJSONPayload(t, msgs[2].Payload, answer) {
		t.Fatalf("answer payload = %s, want %s", msgs[2].Payload, answer)
	}

	// 段落子对话没有 payload 参数（卡片长在房间自己那条主线程上），但它的两条
	// 查询现在都 SELECT 了这一列，所以也得真的执行一次，且读回来必须是 nil。
	blk, err := q.AppendAtomBlockMessage(ctx, sqlc.AppendAtomBlockMessageParams{
		AtomID: a.ID, Seq: 4, Role: "student", Content: "这一段呢？", BlockID: ptr("b1"),
	})
	if err != nil {
		t.Fatalf("AppendAtomBlockMessage: %v", err)
	}
	if blk.Payload != nil {
		t.Fatalf("block message payload = %s, want nil", blk.Payload)
	}
	blocks, err := q.ListAtomBlockMessages(ctx, sqlc.ListAtomBlockMessagesParams{AtomID: a.ID, BlockID: ptr("b1")})
	if err != nil {
		t.Fatalf("ListAtomBlockMessages: %v", err)
	}
	if len(blocks) != 1 || blocks[0].Payload != nil {
		t.Fatalf("block messages = %+v, want one row carrying a nil payload", blocks)
	}

	// 主线程的 block_id IS NULL 过滤没被这次改动碰过：第四条没漏进来。
	after, err := q.ListAtomMessages(ctx, a.ID)
	if err != nil {
		t.Fatalf("ListAtomMessages after the block append: %v", err)
	}
	if len(after) != 3 {
		t.Fatalf("the block message leaked into the room's own thread: %d rows", len(after))
	}
}

// sameJSONPayload compares semantically — jsonb normalises whitespace and key
// order on the way in, so a byte-for-byte comparison would fail on a payload
// that round-tripped perfectly well.
func sameJSONPayload(t *testing.T, got, want []byte) bool {
	t.Helper()
	if got == nil {
		return false
	}
	var a, b any
	if err := json.Unmarshal(got, &a); err != nil {
		t.Fatalf("payload read back is not JSON (%v): %s", err, got)
	}
	if err := json.Unmarshal(want, &b); err != nil {
		t.Fatalf("fixture is not JSON: %v", err)
	}
	return reflect.DeepEqual(a, b)
}
