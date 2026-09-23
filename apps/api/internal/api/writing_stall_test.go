package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

func stallComment(sid uuid.UUID, sourceText string, symptoms ...string) sqlc.WritingComment {
	pts := make([]CommentPoint, 0, len(symptoms))
	for _, s := range symptoms {
		pts = append(pts, CommentPoint{Kind: "issue", Symptom: s, Text: "…"})
	}
	raw, _ := json.Marshal(pts)
	return sqlc.WritingComment{
		Scope:      "block",
		SnippetID:  pgtype.UUID{Bytes: sid, Valid: true},
		SourceText: sourceText,
		Points:     raw,
	}
}

// general-suggestions.md：「连续两轮无新增信息时，改用选项、句式或简短示范」。
// 验收标准第 5 条：「连续两轮卡住后改变帮助方式」。
//
// 🚨 R4 之前这条只接在**立题**那条路上（writingPlanStalled）。
// 段落这两条路上一个都没有 —— 她卡住，产品用同一句话问她第三遍。
func TestWritingHelpModeLadder(t *testing.T) {
	sid := uuid.New()
	const now = "她这一段的字"

	// prior 按 created_at DESC，最新的在前。
	for _, tc := range []struct {
		name  string
		prior []sqlc.WritingComment
		want  writingHelpMode
	}{
		{
			name:  "还没说过 → 问问题",
			prior: nil,
			want:  helpAsk,
		},
		{
			name:  "说过一轮 → 还是问问题",
			prior: []sqlc.WritingComment{stallComment(sid, now, "claim_no_evidence")},
			want:  helpAsk,
		},
		{
			name: "同一个症状连着两轮、字没改 → 给选项",
			prior: []sqlc.WritingComment{
				stallComment(sid, now, "claim_no_evidence"),
				stallComment(sid, now, "claim_no_evidence"),
			},
			want: helpOffer,
		},
		{
			name: "连着三轮 → 给句式",
			prior: []sqlc.WritingComment{
				stallComment(sid, now, "claim_no_evidence"),
				stallComment(sid, now, "claim_no_evidence"),
				stallComment(sid, now, "claim_no_evidence"),
			},
			want: helpShow,
		},
		{
			// 🚨 这一条是整个判据的重点：她**改过**这一段。
			// 判错的方向不对称 —— 她动了而产品还当她卡着，会继续给她
			// 降级过的帮助，等于告诉她「你刚才做的不算」。
			name: "中间她改过字 → 回到问问题",
			prior: []sqlc.WritingComment{
				stallComment(sid, now, "claim_no_evidence"),
				stallComment(sid, "她改之前的字", "claim_no_evidence"),
				stallComment(sid, "她改之前的字", "claim_no_evidence"),
			},
			want: helpAsk,
		},
		{
			// 说的不是同一件事就不算卡在同一处。
			name: "两轮说的是不同的症状 → 问问题",
			prior: []sqlc.WritingComment{
				stallComment(sid, now, "claim_no_evidence"),
				stallComment(sid, now, "no_transition"),
			},
			want: helpAsk,
		},
		{
			// 别的段的意见不算在这一段头上。
			name: "别的段的两轮不算",
			prior: []sqlc.WritingComment{
				stallComment(uuid.New(), now, "claim_no_evidence"),
				stallComment(uuid.New(), now, "claim_no_evidence"),
			},
			want: helpAsk,
		},
		{
			// 上一轮是 pass（没提毛病）—— 她没卡住。
			name: "中间有一轮通过了",
			prior: []sqlc.WritingComment{
				stallComment(sid, now, "claim_no_evidence"),
				stallComment(sid, now),
				stallComment(sid, now, "claim_no_evidence"),
			},
			want: helpAsk,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := writingHelpModeFor(tc.prior, sid, now); got != tc.want {
				t.Errorf("得到 %v，want %v", got, tc.want)
			}
		})
	}
}

// 陪练那条路数的是「引导重新生成到第几代」。
//
// 🚨 它搭的是 writingGuideWithPrevious 里 `trimmed.Previous = nil` 那句话的
// 便车：存下来的 Previous 永远不带自己的 Previous。那句被改掉，这条判据
// 会悄悄失效 —— 所以这里连它一起验。
func TestGuideHelpModeLadder(t *testing.T) {
	one := &writingGuideDTO{Job: "写这一段", Questions: []string{"这一句凭什么成立？"}}
	two := &writingGuideDTO{Job: "写这一段", Questions: []string{"换个角度呢？"}, Previous: one}

	if got := writingGuideHelpMode(nil); got != helpAsk {
		t.Errorf("第一代该问问题，得到 %v", got)
	}
	if got := writingGuideHelpMode(one); got != helpOffer {
		t.Errorf("第二代该给选项，得到 %v", got)
	}
	if got := writingGuideHelpMode(two); got != helpShow {
		t.Errorf("第三代该给句式，得到 %v", got)
	}
	// 没有问题的那一份不算一代。
	if got := writingGuideHelpMode(&writingGuideDTO{Job: "写这一段"}); got != helpAsk {
		t.Errorf("一个问题都没有的引导不算一代，得到 %v", got)
	}
	// 🚨 只留一层这件事本身。
	nested := writingGuideWithPrevious(writingGuideDTO{Job: "新的", Questions: []string{"？"}}, two)
	if nested.Previous == nil {
		t.Fatal("上一份没挂上去")
	}
	if nested.Previous.Previous != nil {
		t.Error("存下来的 Previous 带上了自己的 Previous —— writingGuideHelpMode 的第三代判据会跟着失效")
	}
}

// 🚨 helpAsk 一个字都不加。
//
// 2026-09-05 的教训：这类提示做成常驻，模型会一直去处理那条提示、
// 把该做的事挤掉（六轮里一直在补一张卡，她的主页三处一直是空的）。
func TestHelpModeBlockIsSilentByDefault(t *testing.T) {
	if got := writingHelpModeBlock(helpAsk, "body", "zh", genreArgument); got != "" {
		t.Errorf("默认那一档不该加任何字：%q", got)
	}

	offer := writingHelpModeBlock(helpOffer, "body", "zh", genreArgument)
	if !strings.Contains(offer, "两个不同的修改方法") {
		t.Errorf("给选项那一档没说清要做什么：%q", offer)
	}

	show := writingHelpModeBlock(helpShow, "body", "zh", genreArgument)
	// 🚨 铁律①：给的是句式，不是替她写好的正文。
	if !strings.Contains(show, "不代写段落") {
		t.Errorf("给句式那一档没划清那条线：%q", show)
	}
	// 讲义三法的句式要真的摆出来 —— 只说「给一句句式」而不给，
	// 模型会自己编一个。
	for _, want := range []string{"假如……，那么……？", "因为……，所以……"} {
		if !strings.Contains(show, want) {
			t.Errorf("句式表里缺 %q：%q", want, show)
		}
	}
	// 英文那一篇不摆中文句式。
	if en := writingHelpModeBlock(helpShow, "body", "en", genreArgument); strings.Contains(en, "假如") {
		t.Errorf("英文那一篇拿到了中文句式：%q", en)
	}
}

// 英文那一篇，helpShow 要给英文句式，而且不许混进中文句式。
func TestHelpShowOffersFramesInThePieceOwnLanguage(t *testing.T) {
	en := writingHelpModeBlock(helpShow, "body", langEnglish, genreArgument)
	if !strings.Contains(en, "While it is true that") {
		t.Error("英文议论文卡住求助，却一条英文句式都没给")
	}
	if strings.Contains(en, "假如") {
		t.Error("英文那一篇里混进了中文句式")
	}

	zh := writingHelpModeBlock(helpShow, "body", "zh", genreArgument)
	if !strings.Contains(zh, "假如") {
		t.Error("中文议论文那条路本来就有句式，不该被这次改动弄丢")
	}
	if strings.Contains(zh, "While it is true that") {
		t.Error("中文那一篇里混进了英文句式")
	}
}

// 🚨 句式按位置挑。
//
// 每一条 en_* 的 genre 都是空串，所以文体那条轴在英文这边筛不掉任何东西；
// 位置是这条路上唯一还在起作用的筛子。少了它，一个卡在议论文分论点上的
// 学生会收到记叙文的开场句式和收尾句式，一共 23 条，而提示词上面那句写的是
// 「这一轮给她一句句式」。
func TestHelpShowFramesFollowThePosition(t *testing.T) {
	body := writingHelpModeBlock(helpShow, writingKindAppliesTo(writingKindPoint), langEnglish, genreArgument)
	for _, notThere := range []string{"Opening inside a moment", "Landing the meaning"} {
		if strings.Contains(body, notThere) {
			t.Errorf("正文一段拿到了 %q —— 那是开头/结尾那一格的句式", notThere)
		}
	}
	if !strings.Contains(body, "While it is true that") {
		t.Error("正文一段该有的让步句式没了")
	}

	opening := writingHelpModeBlock(helpShow, writingKindAppliesTo(writingKindOpening), langEnglish, genreArgument)
	if strings.Contains(opening, "While it is true that") {
		t.Error("开头拿到了正文那一格的让步句式")
	}
	if !strings.Contains(opening, "Opening inside a moment") {
		t.Error("开头一条开场句式都没有")
	}

	// 中文那一路：三条分析句法都在正文那一格，开头一格本来就没有句式。
	zhOpening := writingHelpModeBlock(helpShow, writingKindAppliesTo(writingKindOpening), "zh", genreArgument)
	if strings.Contains(zhOpening, "假如") {
		t.Error("中文开头拿到了正文那一格的分析句法")
	}
}
