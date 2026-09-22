// Package vocab is the single source of truth for the method names 印记 is
// allowed to use, their definitions, and the examples it may show.
//
// Two properties this package exists to guarantee:
//
//   - TERMINOLOGY IS OURS. A teacher is rigorous about terms; a model improvising
//     「递进式论证法」 on one turn and 「层进」 on the next teaches nothing. 印记
//     selects from this file and tunes; it never invents a term. An id the model
//     returns that is not in here is dropped by the caller.
//   - EXAMPLES ARE BORROWED MATERIAL. Every example here is pre-authored about a
//     topic no student is writing on. An "example 留悬念 opening" generated about
//     her own thesis IS her opening, authored by AI — 铁律① through the back door.
//     Shipping the examples as data closes that door structurally.
//
// The same JSON is imported at build time by apps/lite-web, so the two ends can
// never disagree about what a method is called.
//
// methods.json is duplicated on purpose. The editable source of truth lives at
// packages/contracts/vocab/methods.json; go:embed patterns cannot escape this
// package's own directory, so apps/api/internal/vocab/methods.json is a
// byte-identical copy embedded locally. TestEmbeddedCopyMatchesSourceOfTruth in
// vocab_test.go enforces the two never drift apart. Do not "simplify" this by
// deleting one of the two files — that reintroduces the escape-the-module
// problem this duplication exists to solve.
package vocab

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed methods.json
var methodsJSON []byte

type Example struct {
	Topic string `json:"topic"`
	Text  string `json:"text"`
}

type Pattern struct {
	Label string `json:"label"`
	Frame string `json:"frame"`
}

// Method is one entry in the library. It carries TWO names on purpose
// (2026-08-28 product ruling): even 论证 is jargon to a 中学生, but the
// curriculum term must stay reachable rather than be hidden.
//
//   - Name is plain language — what the student reads. It names the ACTION she
//     already performs（「举个例子」「比一比」）.
//   - FormalName is the 语文 curriculum term（「举例论证」「对比论证」）, revealed
//     by a card she can click. Offered, never imposed. It is empty where no
//     curriculum term exists (closing_scope) and on the English entries, whose
//     Name is already what an English writer reads.
type Method struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	FormalName string `json:"formal_name"`
	AppliesTo  string `json:"applies_to"`
	// Lang is the writing language this method may be offered for: "zh", "en"
	// or "any". It is a STRUCTURAL guard, not a hint to the model — handing an
	// English sentence frame ("While it is true that ___") to a student writing
	// a Chinese essay is a bug, and a prompt line asking the model to please
	// avoid that is not a fix. See For.
	//
	// 🔑 What is language-bound is the WORDING, not the rhetoric. The en_*
	// entries carry literal English sentence frames, so they are "en": useless
	// inside a Chinese essay. Every structural method — hooks, parallel
	// reasons, concession, cause-and-effect — is "any", because an English
	// essay has all of those too, and 印记 discusses an English piece IN
	// CHINESE anyway (buildWritingPlanPrompt: 「这篇用英文写（但你和她用中文
	// 讨论）」), so a Chinese method name in front of an English writer is
	// exactly what the rest of the room already does. Tagging the structural
	// entries "zh" would fix the reported bug and create its mirror image: an
	// English writer left with one opening method and no closings at all.
	Lang string `json:"lang"`
	// Category 把 body 那一层再分成两类，照语文课的分法（同事 2026-09-20 给的
	// 两页教辅）：
	//
	//	"structure" 论证结构 —— 整篇的几条分论点之间是什么关系（总分式 /
	//	            并列式 / 层进式 / 对照式）。它是**行文那一步**的东西，
	//	            不是某一段的；`applies_to` 是 "whole"。
	//	"method"    论证方法 —— 一个看法怎么被证明（举例 / 引用 / 对比 /
	//	            比喻 / 因果 / 类比 / 归谬）。
	//	"analysis"  分析句三法 —— **例子摆完之后那一句**怎么写（因果 / 假设 /
	//	            归纳）。它和 method 不同层：method 回答「这一段怎么证」，
	//	            analysis 回答「材料和主张之间那座桥怎么搭」。
	//	            这是讲义（五）里最值钱的一条，三条都带 `patterns`。
	//	"detail"    记叙文的细节描写 —— 人物 / 环境 / 物件。
	//	"closing"   结尾四技法 —— 首尾呼应 / 名言警句 / 总结归纳 / 修辞收束。
	//	""          开篇的那几种开法，以及英文的句式 —— 不在这条轴上。
	//
	// 🚨 加这个字段**没有动任何一个现有的 name / formal_name**：提示词散文里
	// 引着它们，而 api 包的 TestWritingPlanSystem_NamesOnlyRealMethods 逐字
	// 钉着（它还显式要求「并列论证」存在）。要加就只加新条目。
	//
	// 🚨 `point_parallel`（并列论证）和 `struct_parallel`（并列式）不是重复：
	// 前者说的是**一段**里几条理由并排摆，后者说的是**整篇**的分论点之间
	// 地位相同。教辅里这两个词也确实分属两层。
	Category string `json:"category"`
	// Genre 是这一条属于哪一种文体："argument" / "narrative" / ""（两种都用）。
	//
	// R4 把一位语文老师的讲义装进来之后，库里第一次同时有了两种文体的东西：
	// 分论点之间的关系（总分式……）在记叙文里无处安放，细节描写和抑扬转情
	// 在议论文里也一样。把这件事写成**数据**而不是调用点上的一张 id 名单 ——
	// 名单会漂，加一条新词条的人不会知道去改它。
	//
	// 🚨 空串是「两种都用」，不是「都不用」。开篇的几种开法、英文的那一套
	// 都是空串：一篇记叙文也要开头。
	Genre      string    `json:"genre"`
	Definition string    `json:"definition"`
	Examples   []Example `json:"examples"`
	Patterns   []Pattern `json:"patterns"`
}

// 文体的两个取值。词表在这里和 api 包的 writing_genre.go 各一份
// （同 course.audience 的理由：不写 DB CHECK，两处词表各自成立）。
const (
	GenreArgument  = "argument"
	GenreNarrative = "narrative"
)

// suits 说这一条在这种文体里用不用得上。genre 传空串 = 不挑文体，全要。
func suits(m Method, genre string) bool {
	return genre == "" || m.Genre == "" || m.Genre == genre
}

// Label is how a method is named INSIDE A PROMPT: the plain name, plus the
// curriculum term in parentheses when there is a distinct one. 印记 has to be
// able to say the plain name to the student and still know what the method is
// really called when she asks.
func (m Method) Label() string {
	if m.FormalName == "" || m.FormalName == m.Name {
		return m.Name
	}
	return m.Name + "（正式名称：" + m.FormalName + "）"
}

var loaded []Method
var byID map[string]Method

func init() {
	var doc struct {
		Methods []Method `json:"methods"`
	}
	if err := json.Unmarshal(methodsJSON, &doc); err != nil {
		panic(fmt.Sprintf("vocab: methods.json is not valid: %v", err))
	}
	loaded = doc.Methods
	byID = make(map[string]Method, len(loaded))
	for _, m := range loaded {
		byID[m.ID] = m
	}
}

func All() []Method { return loaded }

func ByID(id string) (Method, bool) {
	m, ok := byID[id]
	return m, ok
}

// For returns the methods usable at a position in the piece, IN THE LANGUAGE
// the piece is written in. "any" is always included on both axes, because a
// method that fits everywhere fits here too.
//
// The lang axis is not a refinement, it is a bug fix (2026-08-28 ruling):
// filtering on position alone handed en_concession's "While it is true that
// ___" to a student writing a Chinese essay. Language belongs in the selector,
// not in a prompt sentence asking the model to be careful.
//
// It filters EXPRESSIONS, not rhetoric — see Method.Lang. In practice: a
// Chinese piece never sees an English sentence frame, and an English piece
// keeps the whole method vocabulary plus the frames.
// genre 是第三条轴（R4 起），取值见 GenreArgument / GenreNarrative；空串 =
// 不挑文体。它和 lang 是同一个道理的两次应用：把「这一条用不用得上」写进
// 选择器，而不是写进一句请模型当心的提示。
func For(appliesTo string, lang string, genre string) []Method {
	out := make([]Method, 0, len(loaded))
	for _, m := range loaded {
		// 🚨 整篇层的论证结构不属于任何一个位置。漏进来，某一段的引导里就会
		// 出现「这一段可以用总分式」—— 那是句错话。见 WholePiece。
		if m.AppliesTo == WholePiece {
			continue
		}
		if (m.AppliesTo == appliesTo || m.AppliesTo == "any") && speaks(m, lang) && suits(m, genre) {
			out = append(out, m)
		}
	}
	return out
}

// 🚨 ForLang / Structures 是**过滤器**，不接入 internal/guidance 的 Pick。
//
// guidance.Pick 回答的是「这一档信息该给哪一条」，赢家只有一条；ForLang 和
// Structures 回答的是「这个体裁 × 语言下有哪些方法都用得上」，答案是一整批 ——
// 写作面一页上要同时摆四条论证结构给她挑，不是先替她挑好其中最贴的一条。
// 把这两个函数改造成走 Pick，会把「全都给」悄悄变成「只给一条」，那是行为
// 改变，不是把选取逻辑挪个地方而已。
//
// 两边真正共用的是体裁这条轴的取值（argument / narrative / report / explain），
// 不是选取的算法；这条轴由 TestVocabGenreFieldStaysInTheClosedSet 和
// TestVocabAxesAgreeWithGuidanceClosedSets（apps/api/internal/api） 钉住，
// 防止 vocab 这边的 genre 和 guidance/api 那边的体裁闭表各自漂开。

// ForLang returns every method usable in a language, at ANY position. It
// serves the two prompts that must cover the whole piece at once (the planning
// turn, and the batch guide that guides every block in one call): they annotate
// each entry with its applies_to instead of filtering by it, but they must
// still never offer the wrong language.
func ForLang(lang string, genre string) []Method {
	out := make([]Method, 0, len(loaded))
	for _, m := range loaded {
		// 同 For：整篇层的不进按块取方法的那几条路。
		if m.AppliesTo == WholePiece {
			continue
		}
		if speaks(m, lang) && suits(m, genre) {
			out = append(out, m)
		}
	}
	return out
}

// WholePiece 是论证结构那一层的 applies_to —— 它不是一个位置，
// 是「整篇」。行文那一步用它，别的地方一律把它挡在外面。
const WholePiece = "whole"

// Structures 返回整篇那一层的结构，**按语言也按文体挑**。
//
// 中文议论文是那四条论证结构（总分式 / 并列式 / 层进式 / 对照式），中文记叙文
// 是抑扬转情法；英文议论文是 thesis-body-conclusion / claim-counterargument-
// refutation / point-by-point / block，英文记叙文是 narrative arc。
//
// 只有行文那一步用得上：她的分论点都摆出来之后，选它们之间是什么关系。
//
// 🚨 这**不是**2026-08-27 被否掉的那个「从库里挑一副骨架往里填」。
// 那次否的是**在她想之前**让她挑；这一步发生在她自己的分论点已经在图上之后，
// 选的是她已经摆出来的那些点之间的关系 —— 先有东西，再给它命名。
//
// # 🚨 lang 这条轴是 2026-09-22 补的，补的是一个真缺陷
//
// 在这之前 Structures 根本不看语言，而那四条论证结构当时标的是 "any" ——
// 于是一个写英文议论文的学生看到的是四张语文课的卡，名字是语文课的词，
// 「比如」后面那句示范讲的是「读书要读慢」。同事 2026-09-22 的意见 7：
// 「english writing is quite different from chinese. but now we use the same
// guidance. strange」。
//
// 和 For / ForLang 的 lang 轴是同一个道理的又一次应用（见 Method.Lang）：
// 「这一条用不用得上」写进选择器，不写进一句请模型当心的提示。
//
// 两个参数都传空串就是全都要（校验那条路和 --print 之类的地方）。
func Structures(genre string, lang string) []Method {
	out := make([]Method, 0, 4)
	for _, m := range loaded {
		if m.AppliesTo != WholePiece || !suits(m, genre) {
			continue
		}
		// lang 传空串 = 不挑语言。挑的时候 "any" 照收（今天整篇这一层没有
		// "any"，但这条规则和 speaks 保持一致，免得以后加一条时行为变了）。
		if lang != "" && !speaks(m, lang) {
			continue
		}
		out = append(out, m)
	}
	return out
}

// Analyses 返回分析句三法（因果 / 假设 / 归纳）。
//
// 讲义（五）：「①只摆现象材料 → 追原因；②已有结论材料 → 问假如；
// ③列举事实材料 → 找共性。」学生最常缺的就是材料和主张之间的这一句，
// 而我们在 R4 之前只会说「这件事凭什么证明上面那句话」—— 说的是缺什么，
// 没说怎么补。这三条各带一句可套的句式（Patterns）。
func Analyses(lang string) []Method { return byCategory("analysis", lang) }

// Details 返回记叙文的细节描写（人物 / 环境 / 物件）。
func Details(lang string) []Method { return byCategory("detail", lang) }

func byCategory(cat, lang string) []Method {
	out := make([]Method, 0, 4)
	for _, m := range loaded {
		if m.Category == cat && speaks(m, lang) {
			out = append(out, m)
		}
	}
	return out
}

func speaks(m Method, lang string) bool { return m.Lang == lang || m.Lang == "any" }
