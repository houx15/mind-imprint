package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 产品负责人 2026-09-18：「reading notes is also: it should not only be lens.
// it should be the grammar/words/my thoughts etc. it's my real reading 成果」
//
// 她摆过的板在这之前只活在对话记录里。这里测的是把它认回来那一段 —— 认错了，
// 报告上就少一节她真做过的事。

func boardAnswerMsg(typ, choice string) sqlc.AtomMessage {
	return sqlc.AtomMessage{
		Role:    "student",
		Payload: coachCardAnswerPayload(&coachCardAnswer{Type: typ, Prompt: "p", Choice: choice}),
	}
}

func TestReportBoardsReadHerPlacements(t *testing.T) {
	msgs := []sqlc.AtomMessage{
		boardAnswerMsg(coachCardLabelRoles,
			"事实：\nThe bridge closed on Monday.\n引述：\n“We will not leave,” said the mayor.\n事实：\nPower returned on Sunday."),
		boardAnswerMsg(coachCardOrderEvents,
			"第1：\nOfficials ordered an evacuation.\n第2：\nThe storm reached the coast."),
	}
	got := buildReportBoards(msgs, genreReport)
	if len(got) != 2 {
		t.Fatalf("板的数量 = %d：%+v", len(got), got)
	}
	if got[0].Title != "事实与来源对照" || got[0].Kind != "label" {
		t.Errorf("报道上的标注板 = %+v", got[0])
	}
	// 同一格里的两句并成一格，格子按第一次出现的顺序。
	if len(got[0].Groups) != 2 || got[0].Groups[0].Bin != "事实" || len(got[0].Groups[0].Quotes) != 2 {
		t.Errorf("格子分组不对：%+v", got[0].Groups)
	}
	if got[1].Kind != "order" || got[1].Title != reportOrderBoardTitle ||
		strings.Join(got[1].Order, "|") != "Officials ordered an evacuation.|The storm reached the coast." {
		t.Errorf("排序板 = %+v", got[1])
	}
}

func TestReportBoardsTitleFollowsGenre(t *testing.T) {
	msg := boardAnswerMsg(coachCardLabelRoles, "关键主张：\n屋顶光伏在南方划算。")
	for genre, want := range map[string]string{
		genreArgument:  "论证图",
		genreExplain:   "知识结构",
		genreNarrative: "人物与描写",
		"":             "你摆的板",
	} {
		got := buildReportBoards([]sqlc.AtomMessage{msg}, genre)
		if len(got) != 1 || got[0].Title != want {
			t.Errorf("genre %q 的标题 = %+v，想要 %q", genre, got, want)
		}
	}
}

func TestReportBoardsIgnoreEverythingElse(t *testing.T) {
	// 她打字说的话、生词板、以及格子名认不出来的行，都不是一块板。
	msgs := []sqlc.AtomMessage{
		{Role: "student", Content: "我摆好了"},
		boardAnswerMsg(coachCardWordBank, "scramble — 不认识"),
		boardAnswerMsg(coachCardLabelRoles, "随便编的格子：\n某一句"),
		{Role: "ai", Content: "这块板摆得不错。"},
	}
	if got := buildReportBoards(msgs, genreReport); got != nil {
		t.Errorf("认出了不该认的东西：%+v", got)
	}
}
