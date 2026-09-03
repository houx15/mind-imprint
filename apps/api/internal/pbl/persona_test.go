package pbl

import (
	"strings"
	"testing"
)

func TestParsePersonas_ReadsTheCandidates(t *testing.T) {
	got, err := ParsePersonas(`{"personas":[
		{"label":"想学修东西的初中生","whyKnows":"在维修论坛的回帖里看到她的链接",
		 "wants":"她拆过的东西和翻车的地方","feeling":"敢动手","keywords":["动手","实拍","不怕拆坏"],
		 "portrait":"一个初中生蹲在地上拆一个电吹风"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Label != "想学修东西的初中生" {
		t.Fatalf("没解析出来：%+v", got)
	}
	if len(got[0].Keywords) != 3 {
		t.Errorf("关键词丢了：%+v", got[0].Keywords)
	}
}

// 一张残卡在界面上是一张空卡，而她要在三张卡之间做判断。宁可少一张。
func TestParsePersonas_DropsAHalfCandidate(t *testing.T) {
	got, err := ParsePersonas(`{"personas":[
		{"label":"","whyKnows":"x","keywords":["a"]},
		{"label":"有称呼的","whyKnows":"","keywords":["a"]},
		{"label":"没关键词的","whyKnows":"x","keywords":[]},
		{"label":"完整的","whyKnows":"x","keywords":["a"]}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Label != "完整的" {
		t.Errorf("应该只留下完整的那一个，实际 %+v", got)
	}
}

// 一个也解析不出来要报错，不给一组模板人。三个模板受众看起来和真的一模一样，
// 而她照着一个编出来的读者定整页的调子，错在哪儿永远看不出来。
func TestParsePersonas_RefusesWhenNothingIsUsable(t *testing.T) {
	for _, raw := range []string{`{"personas":[]}`, `{"personas":[{"label":""}]}`, `nope`} {
		if _, err := ParsePersonas(raw); err == nil {
			t.Errorf("%q 应该被拒", raw)
		}
	}
}

// 关键词最多六个：再多，第三关派生配色时它们会互相矛盾，而她没办法判断是哪几个
// 在打架。
func TestParsePersonas_CapsKeywords(t *testing.T) {
	got, err := ParsePersonas(`{"personas":[{"label":"a","whyKnows":"b",
		"keywords":["1","2","3","4","5","6","7","8"]}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got[0].Keywords) != maxPersonaKeywords {
		t.Errorf("关键词没截到 %d 个：%v", maxPersonaKeywords, got[0].Keywords)
	}
}

// 最多三个候选：两个是一道二选一，四个开始变成一份清单，而清单是用来扫的。
func TestParsePersonas_CapsCandidates(t *testing.T) {
	one := `{"label":"a","whyKnows":"b","keywords":["k"]},`
	got, err := ParsePersonas(`{"personas":[` + strings.Repeat(one, 5) + `{"label":"z","whyKnows":"b","keywords":["k"]}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != PersonaWanted {
		t.Errorf("候选没截到 %d 个：%d", PersonaWanted, len(got))
	}
}

// 🚨 画像提示词里那两条是我们加的，不由模型决定：不要文字（生成模型写中文几乎
// 必然出错，一张写着乱码的画像会让整块板看起来是坏的），不要照片写实（这是一个
// 虚构的读者，画成真人脸她会以为那是个真人）。
func TestPortraitPrompt_AlwaysForbidsTextAndPhotorealism(t *testing.T) {
	got := PortraitPrompt(PersonaCandidate{Label: "想学修东西的初中生", Portrait: "一个初中生在拆电吹风"})
	for _, want := range []string{"不要任何文字", "不要照片写实"} {
		if !strings.Contains(got, want) {
			t.Errorf("提示词里少了「%s」：%s", want, got)
		}
	}
	if !strings.Contains(got, "一个初中生在拆电吹风") {
		t.Errorf("印记自己写的那句被丢了：%s", got)
	}
}

// 模型没给 portrait 时用称呼兜底——空提示词会被网关拒掉，那一整张卡就永远没有图。
func TestPortraitPrompt_FallsBackToTheLabel(t *testing.T) {
	got := PortraitPrompt(PersonaCandidate{Label: "想学修东西的初中生"})
	if !strings.Contains(got, "想学修东西的初中生") {
		t.Errorf("没有用称呼兜底：%s", got)
	}
}
