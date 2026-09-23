package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 产品负责人 2026-09-23：
//
//	「书信 is a very important format in junior english. but currently we would
//	  guide students to write a letter under the structure of 议论文.」
//
// 🚨 在这之前文体只有两个取值，一封信必然落到最后那句 `return genreArgument`，
// 于是一个初中生被要求给一封信写中心论点、分论点和论据。
//
// 正反两个方向都钉：判漏了她拿不到书信那套骨架，判多了一道议论文题会被当成
// 一封信 —— 后者更糟，所以反方向的用例比正方向还多。

func letterWriting(title string) sqlc.Writing { return sqlc.Writing{Title: title} }

func TestLetterTitlesAreRecognised(t *testing.T) {
	titles := []string{
		"给外婆的一封信",
		"写一封信给三年后的自己",
		"请给校长写一封信，说明你的建议",
		"致全体同学：关于图书角的倡议书",
		"一封感谢信",
		"给笔友的回信",
		// 英文那一套
		"Write a letter to your pen pal about your school life",
		"Suppose you are Li Hua. Write an email to Peter.",
		"Dear Jim, ... (complete the letter)",
		"A letter of application for the summer camp",
		"Write to the headmaster about the school library",
	}
	for _, title := range titles {
		if got := writingGenreOf(letterWriting(title), nil); got != genreLetter {
			t.Errorf("%q 判成了 %s，想要 letter", title, got)
		}
	}
}

// 🚨 反方向：这些都**不是**信。判多了的代价更大 —— 一道议论文题被当成一封信，
// 她拿到的是「这封信写给谁」，而题目根本没有收信人。
func TestNonLetterTitlesStayWhatTheyWere(t *testing.T) {
	cases := []struct {
		title string
		want  string
	}{
		{"读书该快还是该慢", genreArgument},
		{"请给出你的建议：学校该不该取消early bird", genreArgument},
		{"Should schools start the day later?", genreArgument},
		{"Discuss both views and give your own opinion", genreArgument},
		{"邀请大家参加运动会合理吗", genreArgument},
		{"记一次难忘的经历", genreNarrative},
		{"那一天，我终于明白了", genreNarrative},
		{"Write about a time when you failed", genreNarrative},
		{"A narrative essay about my grandmother", genreNarrative},
	}
	for _, c := range cases {
		if got := writingGenreOf(letterWriting(c.title), nil); got != c.want {
			t.Errorf("%q 判成了 %s，想要 %s", c.title, got, c.want)
		}
	}
}

// 板上的东西比题目硬 —— 和另外两种文体同一条规矩。
func TestLetterKindsOnTheBoardDecideTheGenre(t *testing.T) {
	board := []sqlc.WritingOutline{
		{Kind: writingKindPurpose, Text: "想请王老师周六来参加我们的读书会", Depth: 0, Position: 0},
		{Kind: writingKindMatter, Text: "时间是周六下午三点，在图书馆二楼", Depth: 1, Position: 1},
	}
	// 题目长得一点都不像一封信。
	if got := writingGenreOf(letterWriting("读书该快还是该慢"), board); got != genreLetter {
		t.Errorf("板上摆着书信的块，却判成了 %s", got)
	}
}

// 🚨 书信排在记叙文前面：一封信里可以有一段小小的叙事，
// 反过来一篇记叙文里不会出现写信目的和结尾的礼貌话。
func TestALetterAboutAnExperienceIsStillALetter(t *testing.T) {
	if got := writingGenreOf(letterWriting("给外婆写一封信，记一件让你难忘的事"), nil); got != genreLetter {
		t.Errorf("两张表都命中时判成了 %s，想要 letter（称呼、落款一样都少不了）", got)
	}
}

// 书信那三种块的深度、父节点、标题都不能落到议论文那一套上。
func TestLetterKindsHaveTheirOwnShape(t *testing.T) {
	for _, k := range []string{writingKindPurpose, writingKindMatter, writingKindCourtesy} {
		if !writingKindValid(k) {
			t.Errorf("%s 不在闭表里", k)
		}
		if writingKindGenre(k) != genreLetter {
			t.Errorf("%s 的文体是 %q，想要 letter", k, writingKindGenre(k))
		}
		if writingKindLabel(k, "") == "" {
			t.Errorf("%s 没有标题 —— 她会看见一张没名字的卡", k)
		}
	}
	if d := writingKindDepth(writingKindPurpose); d != 0 {
		t.Errorf("写信目的的深度是 %d，想要 0", d)
	}
	if d := writingKindDepth(writingKindMatter); d != 1 {
		t.Errorf("要点的深度是 %d，想要 1", d)
	}
	if d := writingKindDepth(writingKindCourtesy); d != 0 {
		t.Errorf("结尾的话的深度是 %d，想要 0", d)
	}
}

// 🚨 一封信拿到的必须是书信那份节点表，不是议论文那份。
// 这一条是这整档的判据：她报的毛病就是「a letter under the structure of 议论文」。
func TestLetterGetsTheLetterKindsNotTheArgumentOnes(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		s := writingPlanSystemFor(genreLetter, lang, "")
		for _, banned := range []string{"中心论点", "分论点"} {
			// 「没有中心论点，也没有分论点」那句话里会出现这两个词，
			// 所以查的是**节点表**里有没有把它们列成可选的 id。
			if strings.Contains(s, "「thesis」") || strings.Contains(s, "「point」") {
				t.Errorf("%s 的书信提示词里把 %s 列成了可选的节点类型", lang, banned)
			}
		}
		for _, want := range []string{"「purpose」", "「matter」", "「courtesy」"} {
			if !strings.Contains(s, want) {
				t.Errorf("%s 的书信提示词里没有 %s", lang, want)
			}
		}
		// 篇章结构那一节也要是信的，不是总—分—总。
		if !strings.Contains(s, "常见的信件结构") {
			t.Errorf("%s 的书信提示词用的不是信件的篇章结构", lang)
		}
		if strings.Contains(s, "总—分—总") {
			t.Errorf("%s 的书信提示词里混进了议论文的篇章结构", lang)
		}
	}
}
