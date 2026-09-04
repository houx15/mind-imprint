package pbl

import "testing"

// 这几条都是**真模型真回过**的形状，不是想象出来的边界。
//
// 2026-09-04 的真浏览器走查上，第三关以
// `palettes are not JSON: invalid character 'ç' after object key:value pair`
// 失败——她那一侧看到的是「生成失败」。原来的取法是「第一个 { 到最后一个 }」，
// 模型在对象前后多写一句话就会切出一段坏的。
func TestFirstJSONObject(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"干净的一个对象", `{"a":1}`, `{"a":1}`},
		{"前面有话", "好的，这是三组配色：\n{\"a\":1}", `{"a":1}`},
		{"后面还有话", "{\"a\":1}\n\n你挑一个，挑完我们继续。", `{"a":1}`},
		{"围栏包着", "```json\n{\"a\":1}\n```", `{"a":1}`},
		{"嵌套", `{"a":{"b":2}}`, `{"a":{"b":2}}`},
		{
			// 🚨 这一条是「最后一个 }」那种取法真正会栽的地方：后面那段话里
			// 也有花括号，切到最后一个就把两段粘成了一段坏 JSON。
			"后面那段话里也有花括号",
			`{"a":1} 说明：占位符写成 {name} 就行`,
			`{"a":1}`,
		},
		{
			// 字符串里的括号不能参与配平。
			"字符串里有括号",
			`{"why":"像 { 这样的符号"} 后面随便写`,
			`{"why":"像 { 这样的符号"}`,
		},
		{
			"字符串里有转义引号",
			`{"why":"他说\"好\"{"} 尾巴`,
			`{"why":"他说\"好\"{"}`,
		},
		{"一个对象都没有", "我这次不想给 JSON", "我这次不想给 JSON"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := firstJSONObject(c.in); got != c.want {
				t.Errorf("firstJSONObject(%q)\n = %q\n想要 %q", c.in, got, c.want)
			}
		})
	}
}

// 报错要带上模型原样回的东西，否则线上只剩一句「不是 JSON」，
// 到底是它写跑题了还是我们切错了范围，无从判断。
func TestParsePalettes_ErrorCarriesWhatTheModelSaid(t *testing.T) {
	_, err := ParsePalettes("我不想给 JSON，我给你讲讲配色的道理")
	if err == nil {
		t.Fatal("不是 JSON 却解析成功了")
	}
	if got := err.Error(); !contains(got, "我不想给 JSON") {
		t.Errorf("报错里没有模型原话：%s", got)
	}
}

// 三组配色里有一组颜色是坏的，只丢那一组，别把整批扔掉。
func TestParsePalettes_KeepsTheGoodOnes(t *testing.T) {
	raw := `好的：{"palettes":[
	  {"label":"旧纸","why":"安静","paper":"#F6F1E7","ink":"#2B2A26","accent":"#B4553A"},
	  {"label":"暖","why":"暖","paper":"warm beige","ink":"#2B2A26","accent":"#B4553A"}
	]} 你挑一个。`
	out, err := ParsePalettes(raw)
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if len(out) != 1 || out[0].Label != "旧纸" {
		t.Fatalf("想要只留下「旧纸」，得到 %+v", out)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
