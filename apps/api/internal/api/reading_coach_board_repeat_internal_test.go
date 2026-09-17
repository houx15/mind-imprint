package api

import (
	"encoding/json"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 产品负责人 2026-09-17 逐字：
//
//	only one such practice in one paper is enough. (in my just finished paper,
//	I repeated at least four times. although three of them are the same one)
//
// 四次里有三次是同一块板：她摆完，印记 觉得有一两张放错，就再发一块让她**从头
// 摆一遍**。改正是好的，从头摆不是 —— 那三次她做的是同一件事。
//
// prompt 里也写了这条，但散文跨不过判据（[[reading-room-rulings-2026-09-17]]
// 第一条），所以这里数出来。

func boardMsg(t *testing.T, cardType string) sqlc.AtomMessage {
	t.Helper()
	payload, err := json.Marshal(coachMessagePayload{Card: &coachCard{
		Type:   cardType,
		Prompt: coachLabelBoardPrompt,
		Options: []coachCardOption{
			{BlockID: "b1", Quote: "第一段那句话。"},
			{BlockID: "b2", Quote: "第二段那句话。"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return sqlc.AtomMessage{Role: "ai", Payload: payload}
}

func TestCountLabelBoards(t *testing.T) {
	msgs := []sqlc.AtomMessage{
		boardMsg(t, coachCardLabelRoles),
		{Role: "student", Content: "摆好了"},
		boardMsg(t, coachCardWordBank),   // 生词板不算
		boardMsg(t, coachCardChooseSpan), // 选一句也不算
		boardMsg(t, coachCardLabelRoles),
	}
	if got := countLabelBoards(msgs); got != 2 {
		t.Errorf("countLabelBoards = %d, want 2（只数标注板）", got)
	}
}

// 数的是**发出去过几块**，不是她答过几块：一块发出去她没答的板，对她来说
// 照样是一次「又来一块」。
func TestCountLabelBoardsCountsWhatWasHandedToHer(t *testing.T) {
	msgs := []sqlc.AtomMessage{boardMsg(t, coachCardLabelRoles), boardMsg(t, coachCardLabelRoles)}
	if got := countLabelBoards(msgs); got != maxLabelBoards {
		t.Errorf("countLabelBoards = %d, want %d", got, maxLabelBoards)
	}
	// 上限就是这个数：到了它，下一块就该被丢掉。
	if maxLabelBoards > 2 {
		t.Errorf("maxLabelBoards = %d —— 产品负责人报的是一篇里摆了四块，上限不该再放宽", maxLabelBoards)
	}
}

// 她自己说的话里带着「标注板」三个字，不算一块板。
func TestCountLabelBoardsIgnoresHerOwnWords(t *testing.T) {
	msgs := []sqlc.AtomMessage{{Role: "student", Content: "那块标注板我摆好了"}}
	if got := countLabelBoards(msgs); got != 0 {
		t.Errorf("countLabelBoards = %d, want 0", got)
	}
}
