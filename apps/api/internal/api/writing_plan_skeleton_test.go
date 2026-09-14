package api

import (
	"strings"
	"testing"
)

// 篇章骨架必须真的在提示词里。
//
// 产品负责人 2026-09-12 的第三条：「写作-模块增加"总—分""总—分—总"等结构模板，
// 按结构分步指导」。
//
// 她指的是一个真的空洞：writing_plan.go 的文件头注释写着
//
//	两层都写进系统提示词，但**只作为 印记 自己的知识**
//
// 而实际上只有第二层（单个分论点怎么展开，那 25 个方法）进了提示词。
// 篇章骨架那一层只存在于**注释里** —— 也就是说 印记 手上一个整篇结构的词
// 都没有，它没法说出「你这已经是总—分—总了」。
func TestWritingPlanSystem_CarriesTheWholePieceSkeletons(t *testing.T) {
	for _, name := range []string{"总—分", "总—分—总", "立场式", "起承转合"} {
		if !strings.Contains(writingPlanSystem, name) {
			t.Errorf("提示词里没有骨架「%s」—— 注释说两层都在，实际只有一层", name)
		}
	}
}

// 🚨 骨架名不许写成 『…』。
//
// TestWritingPlanSystem_NamesOnlyRealMethods 把提示词里每一个 『…』 都当成方法名，
// 拿 methods.json 逐字比对。骨架不是方法、也不在那个库里，写成 『总—分』 会让
// 那条测试红 —— 而那条测试是对的，不该为了这个放宽。
//
// 这一条把「用哪种引号」钉死，免得下一个人顺手改成 『』 再去改那条测试。
func TestWritingPlanSystem_SkeletonsAreNotQuotedAsMethods(t *testing.T) {
	for _, quoted := range bracketed(writingPlanSystem) {
		for _, skeleton := range []string{"总—分", "总—分—总", "立场式", "起承转合"} {
			if quoted == skeleton {
				t.Errorf("骨架「%s」被写成了 『』—— 那是方法名的引号，methods.json 里没有它", skeleton)
			}
		}
	}
}

// 🚨 骨架是 印记 自己的知识，不是给她挑的菜单。
//
// 2026-08-27 产品裁定（写在 writing_plan.go 开头）：
//
//	结构像 planning，不是一副固定的骨架。先引导学生想，再把结构长出来。
//	前一版让学生从一张写死的表里挑一副骨架……那不是在思考，那是在填表。
//
// 所以这一段加进去的时候必须同时带着那条禁令。没有它，下一轮很容易顺手把
// 这张表做成一个选择题 —— 那就是把那次裁定悄悄推翻。
func TestWritingPlanSystem_ForbidsOfferingTheSkeletonsAsAMenu(t *testing.T) {
	if !strings.Contains(writingPlanSystem, "绝不要把这张表甩给她挑") {
		t.Error("骨架那一段没有带上「不许做成菜单」的禁令")
	}
	if !strings.Contains(writingPlanSystem, "让她填表") {
		t.Error("没有说清为什么不许挑 —— 2026-08-27 裁掉的正是「填表」那种做法")
	}
}
