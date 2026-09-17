package liteworkspace

import (
	"regexp"
	"sort"
	"strings"
)

// claims.go — checks for replies that say the AI did something no tool can
// do. Each one was seen on production (2026-09-17) and the prompt already
// said not to:
//
//   - the class chat said 「已经打开了本周报告页面」 when open_page had only
//     offered a button;
//   - the report chat offered 「新增一个『阅读』段落」 on a report without
//     that section, and no tool can add a section.
//
// The handler runs these on the reply and the option labels once the model
// has finished, and gives the model one rewrite when one fires.

// openedPagePattern matches a completed opening or navigation: 已打开,
// 已经为您跳转, 帮您打开了. An offer (「要打开本周报告吗」) does not match.
var openedPagePattern = regexp.MustCompile(
	`已(经)?(为您|为你|帮您|帮你|给您|给你)?(打开|跳转|切换到)` +
		`|(为您|为你|帮您|帮你|给您|给你)(打开|跳转|切换)(了|好)`)

// ClaimsOpenedPage reports whether text says a page was opened. No workspace
// tool opens a page: open_page only puts a button under the reply.
func ClaimsOpenedPage(text string) bool {
	return openedPagePattern.MatchString(text)
}

// publishedPattern matches a completed publishing: 已布置好, 已为林知遥布置,
// 布置好了, 已发布, 已发给学生. 「已布置的作业」 and 「布置好以后」 do not match.
var publishedPattern = regexp.MustCompile(
	`已(经)?(为|给|向)[^。！？!?\n]{0,30}?布置` +
		`|已(经)?布置(好|完成|完毕|下去|了)` +
		`|布置(好|完成)了` +
		`|已(经)?(发布|发出|发送|下发)` +
		`|已(经)?发给`)

// ClaimsPublished reports whether text says the homework was published or
// sent. The assignment surface only fills the card; she publishes it with
// 发布作业. Seen on production 2026-09-17: 「已为林知遥、陈思远、赵一诺布置好
// 议论文」 and 「已布置好。项目《手机学习时间调查》发给孙浩然…」 before any
// publish. A negated verb (「还没有发布」) passes.
func ClaimsPublished(text string) bool {
	for _, loc := range publishedPattern.FindAllStringIndex(text, -1) {
		if !negatedBefore(text[:loc[0]]) {
			return true
		}
	}
	return false
}

// changeWordPattern matches a direction for a rewrite: 改短, 写得更具体,
// 加上, 提到, 删掉, 换成, 语气.
var changeWordPattern = regexp.MustCompile(
	`改|写得|写成|加上|加入|补充|提到|写上|写进|删掉|删去|去掉|换成|具体|精简|简短|简洁|详细|短一些|长一些|短一点|长一点|语气|只留`)

// NamesSectionAndChange reports whether the teacher's message names one of
// the report's sections (by its heading) and says how to change it. Such a
// message is a request to rewrite, not a question to answer with options.
// Production 2026-09-17: 「请把总体概述写得更具体一些…」 got 「您想怎么改写？」
// in three tries out of three.
func NamesSectionAndChange(typed string, sectionLabels []string) bool {
	named := false
	for _, l := range sectionLabels {
		if l != "" && strings.Contains(typed, l) {
			named = true
			break
		}
	}
	return named && changeWordPattern.MatchString(typed)
}

var buttonPattern =regexp.MustCompile(`下方(的)?按钮|点击按钮|点下面的按钮|下面的按钮`)

// PointsAtButton reports whether text tells her to use a button under the
// reply.
func PointsAtButton(text string) bool {
	return buttonPattern.MatchString(text)
}

// OnlyAQuestion reports whether text is one bare question: a single clause
// that ends in ？. 「接下来想看什么？」 is; 「名单上的学生本周还没开始。接下来
// 想看什么？」 and 「只有这两名学生还没开始，接下来想做什么？」 are not.
func OnlyAQuestion(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	return !strings.ContainsAny(t, "。！!，,；;") && strings.ContainsAny(t, "？?")
}

// messagePattern matches an offer or claim to contact a student: 提醒该生,
// 通知这些学生, 给她们发消息, 催一下.
var messagePattern = regexp.MustCompile(`提醒|通知|催促|催一下|发消息|发送消息|发个消息|联系(该生|学生|这些学生|家长)`)

// OffersMessage reports whether text offers to remind, notify or message a
// student. No class-chat tool reaches a student; a homework does (it shows in
// her inbox), and that is the page the reply should point to. Seen live on
// 2026-09-17: 「提醒该生开始学习」 as an option, and 「需要提醒她们吗」.
//
// A negated verb is the honest answer: 「无法直接提醒学生」 passes.
func OffersMessage(text string) bool {
	for _, loc := range messagePattern.FindAllStringIndex(text, -1) {
		if !negatedBefore(text[:loc[0]]) {
			return true
		}
	}
	return false
}

// newSectionPattern matches an offer or claim to add or remove a whole
// section: 新增一个『阅读』段落, 添加一个部分, 删除「兴趣」板块. Adding a
// sentence inside a section (「增加一段话」) does not match.
var newSectionPattern = regexp.MustCompile(
	`(新增|增加|添加|加上|增设|删除|删去|删掉|去掉)(一个|个)?(新的)?` +
		`([「『“"][^」』”"]{1,8}[」』”"])?(段落|部分|板块|章节)`)

// OffersSectionChange reports whether text offers to add or remove a report
// section. revise_section can only rewrite a section the report already has.
//
// A negated verb is the honest answer, not an offer: 「不能新增段落」 and
// 「无法新增或删除段落」 pass. Measured: the first version failed the model's
// correct refusal in 3 of 3 live runs.
func OffersSectionChange(text string) bool {
	for _, loc := range newSectionPattern.FindAllStringIndex(text, -1) {
		if !negatedBefore(text[:loc[0]]) {
			return true
		}
	}
	return false
}

// negationPattern matches a negation at the end of the text before a verb,
// allowing up to four runes between them (「不能新增或删除段落」: 不能 sits
// before 新增或 and still negates 删除).
var negationPattern = regexp.MustCompile(`(不|无法|没法|没办法|未)[^，。！？；\n]{0,4}$`)

func negatedBefore(prefix string) bool {
	return negationPattern.MatchString(prefix)
}

// NamesMissingSection returns the first heading in missing that text names as
// a section: quoted (「阅读」) or followed by 段/部分/板块 (阅读部分). ""
// when none is named.
//
// A sentence that says the section is not there is the honest answer:
// 「报告里没有「写作」这个段落，不能新增。」 passes (live, 2026-09-17).
func NamesMissingSection(text string, missing []string) string {
	for _, label := range missing {
		if label == "" {
			continue
		}
		q := regexp.QuoteMeta(label)
		re := regexp.MustCompile(`[「『“"]` + q + `[」』”"]|` + q + `(段|部分|板块)`)
		for _, loc := range re.FindAllStringIndex(text, -1) {
			if !sentenceDenies(text, loc[0], loc[1]) {
				return label
			}
		}
	}
	return ""
}

var sentenceEnd = regexp.MustCompile(`[。！？!?\n]`)

var denial = regexp.MustCompile(`没有|不能|无法|不可以|不支持|不存在`)

// sentenceDenies reports whether the sentence around text[start:end] contains
// a denial.
func sentenceDenies(text string, start, end int) bool {
	from := 0
	if locs := sentenceEnd.FindAllStringIndex(text[:start], -1); len(locs) > 0 {
		from = locs[len(locs)-1][1]
	}
	to := len(text)
	if loc := sentenceEnd.FindStringIndex(text[end:]); loc != nil {
		to = end + loc[0]
	}
	return denial.MatchString(text[from:to])
}

// RedactedName replaces a student's name in a log line.
const RedactedName = "[学生]"

// RedactNames replaces every name in names found in text with RedactedName,
// longest name first, so 王丽华 is not left as [学生]华. Log lines use it:
// a reply the §6 check rejected is logged for debugging, and the names in it
// are students'.
func RedactNames(text string, names []string) string {
	sorted := make([]string, 0, len(names))
	for _, n := range names {
		if strings.TrimSpace(n) != "" {
			sorted = append(sorted, n)
		}
	}
	sort.SliceStable(sorted, func(i, j int) bool { return len([]rune(sorted[i])) > len([]rune(sorted[j])) })
	for _, n := range sorted {
		text = strings.ReplaceAll(text, n, RedactedName)
	}
	return text
}
