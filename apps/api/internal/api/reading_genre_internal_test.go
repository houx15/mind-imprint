package api

import "testing"

// 同事 2026-09-17 逐字：「我总觉得不是所有的文章都应该按照主张、证据、限制
// 这样的内容来拆分，而且主张、证据、限制很多时候并不知道哪些该在哪里。」
//
// 他看的那一篇是战地新闻报道。那块板上每一个格子都从一个前提上长出来 ——
// 主张是**作者要你接受的那句话** —— 而报道里没有那句话。

func TestGenreClosedTable(t *testing.T) {
	for _, ok := range []string{"argument", "report", "narrative", "explain"} {
		if got := validateGenre(ok); got != ok {
			t.Errorf("validateGenre(%q) = %q", ok, got)
		}
	}
	if got := validateGenre("  ARGUMENT "); got != genreArgument {
		t.Errorf("大小写和空格应该收得住，得到 %q", got)
	}
	// 模型编的第五个体裁进不来 —— 和格子名、学科表是同一条纪律。
	for _, bad := range []string{"editorial", "议论文", "", "opinion piece"} {
		if got := validateGenre(bad); got != "" {
			t.Errorf("validateGenre(%q) = %q，编出来的体裁不该留下", bad, got)
		}
	}
}

func TestRoleBoardOnlyWhereTheAuthorArgues(t *testing.T) {
	if !hasAuthorsArgument(genreArgument) || !hasAuthorsArgument(genreExplain) {
		t.Error("议论和说明上应该照常摆那块板")
	}
	if hasAuthorsArgument(genreReport) || hasAuthorsArgument(genreNarrative) {
		t.Error("报道和记叙里没有「作者的主张」，那块板不该摆")
	}
	// 🚨 认不出体裁就不挡。判错的方向和别处一致：宁可放过，不可误伤 ——
	// 老数据、以及模型漏填 genre 的那几份，都走这一条。
	if !hasAuthorsArgument("") || !hasAuthorsArgument("editorial") {
		t.Error("体裁认不出来的时候不该挡任何东西")
	}
}

// 这一条是 2026-09-17 之前那个 bug 的结构性版本：**两套英文读法都带着标注步**，
// 于是一篇英文报道无论排到哪一套，清单上都会出现「标注论证」。
func TestEnglishHasARoutineWithoutTheRoleBoard(t *testing.T) {
	withoutLabel := 0
	for _, r := range readingRoutines {
		if r.Lang != "en" {
			continue
		}
		has := false
		for _, s := range r.Steps {
			if s.Kind == taskLabel {
				has = true
			}
		}
		if !has {
			withoutLabel++
		}
	}
	if withoutLabel == 0 {
		t.Error("英文没有一套不带「标注论证」的读法 —— 一篇新闻报道只能被按主张/证据/限制拆")
	}
}

// 导读的校验把体裁一起收进闭表。
func TestOutlineKeepsTheGenre(t *testing.T) {
	blocks := outlineBlocks(6)
	load := fullLoad(6, loadSupport)
	load["b2"] = loadCore
	got, ok := validateOutline(readingOutline{
		OneLine: "这篇在问什么", Genre: "report", Load: load,
	}, blocks)
	if !ok {
		t.Fatal("这份导读该留下")
	}
	if got.Genre != genreReport {
		t.Errorf("genre = %q, want report", got.Genre)
	}
}
