package pbl

import (
	"strings"
	"testing"
)

// stuck_test.go —— 「她连着几轮答不上来」。
//
// 这个数是【怎么问】那条禁令唯一的出口的触发条件，两种判错都有代价：
// 判不出来，她会被一直追问到不敢再进这个项目（走查里真的发生了）；
// 误判，印记会对着一个正在好好回答的学生开始举例子。

func TestStuckRun_CountsTheTrailingRunOnly(t *testing.T) {
	cases := []struct {
		name   string
		recent []Turn
		want   int
	}{
		{
			name:   "没人卡住",
			recent: []Turn{{Role: "student", Content: "中午十二点半剩得最多"}},
			want:   0,
		},
		{
			name: "连着两次说不上来",
			recent: []Turn{
				{Role: "student", Content: "我不知道"},
				{Role: "ai", Content: "那换个问法……"},
				{Role: "student", Content: "没想过"},
			},
			want: 2,
		},
		{
			name: "她用自己的话求例子，也算答不上来",
			recent: []Turn{
				{Role: "student", Content: "不清楚"},
				{Role: "ai", Content: "……"},
				{Role: "student", Content: "能给我几个选一下吗"},
			},
			want: 2,
		},
		{
			name: "中间卡过、后来聊开了，就归零",
			recent: []Turn{
				{Role: "student", Content: "不知道"},
				{Role: "student", Content: "不知道"},
				{Role: "ai", Content: "……"},
				{Role: "student", Content: "给一起打球的人看"},
			},
			want: 0,
		},
		{
			name: "只应了一声也算没答",
			recent: []Turn{
				{Role: "student", Content: "嗯"},
				{Role: "student", Content: "？"},
			},
			want: 2,
		},
		{
			// 🚨 误判的那一半：一句带保留的真答案不是卡住。判错了，印记会打断一个
			// 正在认真答题的学生。
			name: "「不知道，可能是……」是一个真答案",
			recent: []Turn{
				{Role: "student", Content: "我也不太知道，可能是因为中午那会儿走廊里人特别多，大家都挤在一起"},
			},
			want: 0,
		},
		{
			name: "开头一个「对」不算卡住",
			recent: []Turn{
				{Role: "student", Content: "对，十二点半那会儿最多"},
			},
			want: 0,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := StuckRun(c.recent); got != c.want {
				t.Errorf("StuckRun = %d, want %d", got, c.want)
			}
		})
	}
}

// 🚨 她自己开口要例子，一次就该算数。
//
// 走查里那个学生用自己的话求了三次「给我几个选项选一下」，一次都没得到。让她
// 先卡够两轮才给，是把一条本来就该听见的话当成噪音。
func TestAskedForHelp(t *testing.T) {
	cases := []struct {
		name   string
		recent []Turn
		want   bool
	}{
		{
			name:   "她明说要选项",
			recent: []Turn{{Role: "student", Content: "能给我几个选项选一下吗"}},
			want:   true,
		},
		{
			name:   "她明说要例子",
			recent: []Turn{{Role: "student", Content: "举个例子呗，我不太懂这一格填什么"}},
			want:   true,
		},
		{
			name: "只看最后一句：她后来自己答了",
			recent: []Turn{
				{Role: "student", Content: "举个例子"},
				{Role: "ai", Content: "……"},
				{Role: "student", Content: "给一起打球的人看"},
			},
			want: false,
		},
		{
			name:   "普通的一句答话不算",
			recent: []Turn{{Role: "student", Content: "中午十二点半剩得最多"}},
			want:   false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := AskedForHelp(c.recent); got != c.want {
				t.Errorf("AskedForHelp = %v, want %v", got, c.want)
			}
		})
	}
}

// 她开口要了，这一轮的上下文就该改打法，不必等她卡满两轮。
func TestBuildCoachContext_AnswersAnExplicitAskAtOnce(t *testing.T) {
	in := CoachInput{
		Idea:         "做一个我自己的主页",
		Recent:       []Turn{{Role: "student", Content: "能给我几个选项吗"}},
		Stuck:        1,
		AskedForHelp: true,
	}
	got := buildCoachContext(in)
	if !strings.Contains(got, "自己开口要例子或选项") {
		t.Fatalf("她明说要例子，上下文里却没提这件事：\n%s", got)
	}
	if !strings.Contains(got, "不要再抛敞开的问题") {
		t.Error("说了她开口要例子，却没说这一轮该改成什么打法")
	}
}

// 数出来了还得用上。这条测的是那个数真的变成了 prompt 里的一段话——
// 没有这一段，服务端数得再准，模型那边什么也不会发生。
func TestBuildCoachContext_TellsTheModelSheIsStuck(t *testing.T) {
	in := CoachInput{
		Idea:   "做一个我自己的主页",
		Recent: []Turn{{Role: "student", Content: "不知道"}},
	}
	if got := buildCoachContext(in); strings.Contains(got, "连着答不上来") {
		t.Error("只卡了一次就报了——第一次换个问法再问一遍是对的")
	}

	in.Stuck = 2
	got := buildCoachContext(in)
	if !strings.Contains(got, "连着答不上来") {
		t.Fatal("她连着两轮答不上来，上下文里却一个字都没说")
	}
	if !strings.Contains(got, "例子") {
		t.Error("说了她卡住，却没说这一轮该改成什么打法")
	}
}

// 🚨 被闸撤掉的那件工具必须出现在上下文里。
//
// 撤掉是对的（那个界面打开是一块白板），但撤完没人告诉印记，它下一轮照样说
// 「卡我给你了」，学生满屏幕找一张永远不会出现的卡。这条守的就是那条回路：
// api 那边把话算出来，这里必须把它印进 prompt。
func TestBuildCoachContext_CarriesTheDroppedTool(t *testing.T) {
	in := CoachInput{Idea: "做一个我自己的主页"}
	if strings.Contains(buildCoachContext(in), "没递出去") {
		t.Error("什么都没撤，却报了一件撤掉的工具")
	}

	in.ToolDropped = "「审核助手」这件工具**没有**出现在她屏幕上。"
	got := buildCoachContext(in)
	if !strings.Contains(got, "【上一轮有一件工具没递出去】") {
		t.Fatal("撤掉的工具没有进上下文——印记会把同一句话再说一遍")
	}
	if !strings.Contains(got, "审核助手") {
		t.Error("说了有工具被撤，却没说是哪一件")
	}
}

// 出口本身要在系统提示里写着，不然上下文那句「走那一条」指向的是空气。
func TestCoachSystem_HasAWayOutWhenSheCannotAnswer(t *testing.T) {
	p := sprintCoachSystem("", "  observe（观察日记）—— 他要离开屏幕去做\n", "", "  plan —— 一份计划\n")
	for _, want := range []string{
		"【他答不上来的时候】",
		// 🚨 「挑一个」和「都不是」必须写在同一个问句里。这一句是 2026-09-05
		// 真模型实测逼出来的：上一版把出口写成「哪个更接近？都不是的话是什么？」，
		// 模型照着写了两个问号，破铁律③。
		"哪类更接近你的用途，也可以提出其他读者？",
		"服务端只认原话", // 🚨 例子不能变成替她填的答案：铁律① + GroundSiteDraft。
	} {
		if !strings.Contains(p, want) {
			t.Errorf("系统提示里没有 %q", want)
		}
	}
}
