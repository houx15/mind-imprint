package api

import (
	"strings"
	"testing"
)

// helpShow 那一档不许承诺一件库里做不到的事。
//
// 🚨 2026-09-23 实测：12 格（lang × genre × 位置）里有 **5 格**一条句式都没有
// —— 中文侧只有议论文正文段有（三条分析法 analysis_cause/suppose/induce），
// 开头、结尾、以及整个中文记叙文全是空的。而那段提示词原来无条件写着
// 「本轮提供一句带空格的通用句式」，只有下面的列表是有条件的。
//
// 后果：那 5 格里模型被要求交出一条它没有来源的句式，只能现造，而且现造出来
// 的多半是议论文形状（提示词自己的例子就是「因为……，所以……」）。一篇中文
// 记叙文的结尾因此会收到一条议论句式 —— 正是文体这条轴要消灭的东西。
//
// 这条钉的是**一致性**，不是数量：有句式就可以说「挑一条给她」，没有就不许
// 提句式。将来给中文补了句式，这条照样绿；删掉一批，也照样绿。
func TestHelpShowNeverPromisesFramesItDoesNotHave(t *testing.T) {
	for _, lang := range []string{"zh", langEnglish} {
		for _, genre := range []string{genreArgument, genreNarrative} {
			for _, pos := range []string{"opening", "body", "closing"} {
				frames := writingHelpFrames(pos, lang, genre)
				block := writingHelpModeBlock(helpShow, pos, lang, genre)
				if block == "" {
					t.Errorf("%s/%s/%s: helpShow 这一档整段是空的", lang, genre, pos)
					continue
				}
				promises := strings.Contains(block, "句式")
				if frames == "" && promises {
					t.Errorf("%s/%s/%s: 库里一条句式都没有，提示词却在说句式：\n%s",
						lang, genre, pos, block)
				}
				if frames != "" && !promises {
					t.Errorf("%s/%s/%s: 库里有句式，提示词却没提", lang, genre, pos)
				}
				if frames != "" && !strings.Contains(block, frames) {
					t.Errorf("%s/%s/%s: 句式没被摆进去", lang, genre, pos)
				}
			}
		}
	}
}

// 英文那三条记叙文句式打上 genre 之后，文体这条轴在英文侧才真的起作用。
//
// 🚨 在这之前每一条 en_* 的 genre 都是空串 ⇒ `vocab.For` 对英文来说等于不看
// 文体：一个写英文议论文的学生会拿到 `The turn`（记叙文的转折句式），
// 一个写英文记叙文的学生拿到的则是同一批。这件事**在 2026-09-23 之前看不见**，
// 因为英文记叙文那一格根本到不了（writingGenreOf 的词表全是中文子串）。
//
// 只给 en_story_* 打标，不动 en_concession / en_qualify / en_evidence ——
// 让步与限定在记叙、反思类文章里同样用得上，按窄的那一边收会让记叙文少一块。
func TestEnglishNarrativeFramesDoNotLeakIntoArgument(t *testing.T) {
	for _, pos := range []string{"opening", "body", "closing"} {
		arg := writingHelpFrames(pos, langEnglish, genreArgument)
		nar := writingHelpFrames(pos, langEnglish, genreNarrative)
		if arg == "" || nar == "" {
			t.Fatalf("en/%s: 两种文体都该有句式，arg=%d nar=%d", pos, len(arg), len(nar))
		}
		for _, storyOnly := range []string{"Opening inside a moment", "The turn", "Landing the meaning"} {
			if strings.Contains(arg, storyOnly) {
				t.Errorf("en/argument/%s 拿到了记叙文句式「%s」", pos, storyOnly)
			}
		}
		if countLines(nar) <= countLines(arg) {
			t.Errorf("en/%s: 记叙文应当拿到不少于议论文的条数（记叙 %d，议论 %d）",
				pos, countLines(nar), countLines(arg))
		}
	}
}

func countLines(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	return len(strings.Split(s, "\n"))
}
