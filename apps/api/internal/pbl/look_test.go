package pbl

import (
	"strings"
	"testing"
)

/* ── 配色 ─────────────────────────────────────────────────────────────── */

func TestParsePalettes_ReadsThem(t *testing.T) {
	got, err := ParsePalettes(`{"palettes":[
		{"label":"车间灯","why":"配「动手」和「不怕拆坏」——暖白底加一点机油蓝",
		 "paper":"#F5F2EC","ink":"#1E1C19","accent":"#2F5D8A"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Accent != "#2F5D8A" {
		t.Fatalf("没解析出来：%+v", got)
	}
}

// 🚨 颜色格式在代码里验，不只在 prompt 里要求。
//
// 一个 "warm beige" 或 "rgb(240,235,220)" 会原样流进 CSS 变量，浏览器把整条声明
// 丢掉，她看到的是一个没有变化的预览——而没有任何一层报过错。规矩要能在代码里验
// （memory · prompt-output-must-be-verifiable-2026-09-03）。
func TestParsePalettes_DropsColoursCSSWouldSilentlyIgnore(t *testing.T) {
	got, err := ParsePalettes(`{"palettes":[
		{"label":"坏的","why":"x","paper":"warm beige","ink":"#111111","accent":"#222222"},
		{"label":"也坏的","why":"x","paper":"#FFFFFF","ink":"rgb(0,0,0)","accent":"#222222"},
		{"label":"三位简写","why":"x","paper":"#FFF","ink":"#000","accent":"#123"},
		{"label":"好的","why":"x","paper":"#FFFFFF","ink":"#111111","accent":"#2F5D8A"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Label != "好的" {
		t.Errorf("只该留下颜色合法的那一组，实际 %+v", got)
	}
}

// 一组都合法不了就报错，不给一组默认色。默认色看起来和生成的一模一样，而她会
// 以为那是从她的关键词里来的。
func TestParsePalettes_RefusesWhenNoneAreUsable(t *testing.T) {
	for _, raw := range []string{
		`{"palettes":[]}`,
		`{"palettes":[{"label":"x","paper":"red","ink":"blue","accent":"green"}]}`,
		`nope`,
	} {
		if _, err := ParsePalettes(raw); err == nil {
			t.Errorf("%q 应该被拒", raw)
		}
	}
}

func TestValidPalette(t *testing.T) {
	ok := Palette{Paper: "#FFFFFF", Ink: "#000000", Accent: "#AbCdEf"}
	if !ValidPalette(ok) {
		t.Error("合法的六位十六进制被拒了")
	}
	for _, bad := range []Palette{
		{Paper: "#FFF", Ink: "#000000", Accent: "#000000"},
		{Paper: "#FFFFFF", Ink: "", Accent: "#000000"},
		{Paper: "#FFFFFF", Ink: "#000000", Accent: "#GGGGGG"},
	} {
		if ValidPalette(bad) {
			t.Errorf("%+v 不该算合法", bad)
		}
	}
}

/* ── 头图 ─────────────────────────────────────────────────────────────── */

// 🚨 三条是我们加的，不由模型决定。
//
// 不要文字：生成模型往图里写字几乎必然出错，而头图上一行乱码是整页最显眼的
// 一处坏。不要人脸：头图是她这一页的封面，一张陌生人的脸放在一个未成年人的
// 主页顶上会被读成「这是她」。
func TestHeroPrompt_AlwaysForbidsTextAndFaces(t *testing.T) {
	got := HeroPrompt([]string{"动手", "不怕拆坏"}, "敢动手")
	for _, want := range []string{"不要任何文字", "不要人脸", "动手", "不怕拆坏", "敢动手"} {
		if !strings.Contains(got, want) {
			t.Errorf("头图提示词里少了「%s」：%s", want, got)
		}
	}
}

// 一个关键词都没有时也要给得出一句话——空提示词会被网关拒掉，而她那边看到的
// 会是一个没有解释的失败。
func TestHeroPrompt_SurvivesEmptyKeywords(t *testing.T) {
	got := HeroPrompt(nil, "")
	if strings.TrimSpace(got) == "" || !strings.Contains(got, "不要任何文字") {
		t.Errorf("空关键词时提示词坏了：%q", got)
	}
}
