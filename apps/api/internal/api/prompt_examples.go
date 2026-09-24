package api

// Synthetic offline examples, like benchcases.go. No handler calls this file;
// it never reads student data, credentials or a database. Prompt text is built
// by the same functions production uses, not copied into fixture templates.

import (
	"github.com/jackc/pgx/v5/pgtype"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/promptassembly"
	"mindimprint/api/internal/store/sqlc"
	"strings"
)

type PromptExample struct {
	ID      string              `json:"id"`
	Class   string              `json:"class"`
	Request gateway.ChatRequest `json:"request"`
	// Documents maps message index to its section/selection trace.
	Documents        map[int]promptassembly.Document    `json:"documents,omitempty"`
	ContextFragments map[string]promptassembly.Document `json:"contextFragments,omitempty"`
}

func PromptAssemblyExamples() []PromptExample {
	var out []PromptExample
	for _, c := range []struct {
		id, text string
		done     *readingLensDone
		open     string
	}{
		{id: "reading/concept", text: "装机容量和发电量有什么区别？"},
		{id: "reading/open-lens", text: "这个工具怎么用？", open: openLensLine(true, "信源评估")},
		{id: "reading/lens-completed", done: &readingLensDone{CardName: "信源评估", Quote: "新增装机容量增长了。", Finding: "这里说的是装机容量。"}},
	} {
		blocks := SplitBlocks("新增装机容量增长了。\n\n实际发电量还与设备运行时间有关。")
		tasks := []sqlc.ReadingTask{{Kind: "focus_block", Label: "比较两个指标", Status: "pending", BlockID: "b1"}}
		doc := renderReadingCoachPrompt(selectReadingCoachContext("能源指标", blocks, readingOutline{}, tasks, nil, nil, c.text, c.done, c.open))
		out = append(out, PromptExample{ID: c.id, Class: gateway.ClassDialogue, Request: gateway.ChatRequest{Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: buildReadingCoachSystem("zh", "")}, {Role: gateway.RoleUser, Content: doc.Text},
		}}, Documents: map[int]promptassembly.Document{1: doc}})
	}
	// 🚨 2026-09-23 加上 genreLetter。AGENTS.md 第 6 条：新开一条分支不进基线，
	// 就是「基线不等于覆盖」那个形状 —— 占位符在没被覆盖的分支上漏掉，
	// 整套测试照样绿，而线上那一篇收到的是字面写着 @@KINDS@@ 的提示词。
	for _, lang := range []string{"zh", "en"} {
		// 🚨 2026-09-24 加上 genreContinuation，同一条理由。
		for _, genre := range []string{genreArgument, genreNarrative, genreLetter, genreProse, genreContinuation} {
			wr := sqlc.Writing{Lang: lang, Title: "一次图书馆里的经历"}
			doc := renderWritingPlanPrompt(selectWritingPlanContext(wr, nil, nil, "我想记录上周和同学一起找资料的经历。"))
			out = append(out, PromptExample{ID: "writing/plan/" + lang + "/" + genre, Class: gateway.ClassDialogue, Request: gateway.ChatRequest{MaxTokens: 4096, Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: writingPlanSystemFor(genre, lang, "")}, {Role: gateway.RoleUser, Content: doc.Text},
			}}, Documents: map[int]promptassembly.Document{1: doc}})
		}
	}
	// 🚨 读后续写：依据齐备的那一份。
	//
	// 上面那条 writing/plan/*/continuation 的 AssignedPrompt 是空的，走的是
	// 「依据还不齐」那一支；判前输入检查**通过**之后长什么样，它一个字都没覆盖。
	// 同一条理由（基线不等于覆盖），把齐备的那一份也摆进来。
	{
		assigned := continuationExampleAssigned
		wr := sqlc.Writing{Lang: "en", Title: "读后续写", AssignedPrompt: &assigned}
		doc := renderWritingPlanPrompt(selectWritingPlanContext(wr, nil, nil, "第一段我想写他发现家里没人。"))
		out = append(out, PromptExample{ID: "writing/plan/en/continuation/inputs-ready", Class: gateway.ClassDialogue, Request: gateway.ChatRequest{MaxTokens: 4096, Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: writingPlanSystemFor(genreContinuation, "en", "")}, {Role: gateway.RoleUser, Content: doc.Text},
		}}, Documents: map[int]promptassembly.Document{1: doc}})
	}
	// 🚨 读后续写：依据**不齐**的那一份 —— 判前输入检查拦下来的样子。
	//
	// 这一条比上一条更要紧：门槛那句话是这一档唯一一段由服务端算出来、
	// 再写进提示词的文字，它写错了，学生会收到一份对着不存在的前文做出的
	// 情节判断。上面 writing/plan/*/continuation 那两条都走不到它
	// （题面是空的，文体反而推断不成续写）。
	{
		assigned := "读后续写：根据材料续写两段，词数 150 左右。"
		wr := sqlc.Writing{Lang: "en", Title: "读后续写", AssignedPrompt: &assigned}
		doc := renderWritingPlanPrompt(selectWritingPlanContext(wr, nil, nil, "我不知道第一段该怎么起头。"))
		out = append(out, PromptExample{ID: "writing/plan/en/continuation/inputs-missing", Class: gateway.ClassDialogue, Request: gateway.ChatRequest{MaxTokens: 4096, Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: writingPlanSystemFor(genreContinuation, "en", "")}, {Role: gateway.RoleUser, Content: doc.Text},
		}}, Documents: map[int]promptassembly.Document{1: doc}})
	}
	// 🚨 分论点够了、还没有结尾的那一轮。
	//
	// 上面那四条 writing/plan/* 传的 outline 都是 nil，所以【分论点已经够了，
	// 下一件可以谈结尾】那一栏在它们身上**一条都不渲染** —— 新开的分支不进
	// 基线，就是 AGENTS.md「提示词怎么写」第 6 条记的那个形状（基线不等于覆盖）。
	// 这一条专门把它摆进来。
	{
		wr := sqlc.Writing{Lang: "zh", Title: "读书该快还是该慢"}
		rows := []sqlc.WritingOutline{
			{Kind: writingKindThesis, Text: "读书要读慢", Depth: 0, Position: 0},
			{Kind: writingKindPoint, Text: "慢读才能发现问题", Depth: 1, Position: 1},
			{Kind: writingKindPoint, Text: "读得慢才看得出作者的立场", Depth: 1, Position: 2},
			{Kind: writingKindPoint, Text: "慢下来才记得住", Depth: 1, Position: 3},
			{Kind: writingKindEvidence, Text: "我读《城南旧事》那次", Depth: 2, Position: 4},
			{Kind: writingKindReference, Text: "一项关于阅读速度与留存率的研究", Depth: 2, Position: 5},
		}
		doc := renderWritingPlanPrompt(selectWritingPlanContext(wr, rows, nil, "我想最后再说一句，让读者也去试试慢读。"))
		out = append(out, PromptExample{ID: "writing/plan/zh/argument/closing-next", Class: gateway.ClassDialogue, Request: gateway.ChatRequest{MaxTokens: 4096, Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: writingPlanSystemFor(genreArgument, "zh", "")}, {Role: gateway.RoleUser, Content: doc.Text},
		}}, Documents: map[int]promptassembly.Document{1: doc}})
	}

	// 卡住的那一档（helpShow）：说过三轮她还没动，这一轮摆句式给她照着填。
	//
	// 🚨 两种语言都录，录的是两件不同的事。en 那条钉住**新给出来的内容**
	// （在这之前英文那一篇一条句式都拿不到）；zh 那条钉住**它没变**——
	// 在这之前「zh 没变」只靠两句 strings.Contains 撑着，句式重新排序、
	// 或者掉了一个中文读法的括号，那两句一个字都不会说。
	for _, lang := range []string{"zh", "en"} {
		stuck := sqlc.Writing{Lang: lang, Title: "校园观察"}
		body := "我觉得鸟会挑屋檐下筑巢。"
		if lang == "en" {
			body = "I think birds choose the eaves because it is sheltered."
		}
		out = append(out, PromptExample{ID: "writing/help-show/" + lang, Class: gateway.ClassReview, Request: gateway.ChatRequest{Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: buildWritingCommentSystem(lang, writingBlockCommentMaxIssues, writingKindPoint, helpShow, genreArgument)},
			{Role: gateway.RoleUser, Content: buildWritingCommentPrompt(stuck, "筑巢位置的选择", body, "", genreArgument)},
		}}})
	}

	wr := sqlc.Writing{Lang: "zh", Title: "校园观察"}
	node := sqlc.WritingOutline{ID: fixtureTaskID(1), Kind: writingKindPoint, Depth: 1, Text: "筑巢位置的选择"}
	text := strings.Repeat("我观察到鸟把树枝衔进屋檐。", 40)
	snippets := []sqlc.WritingSnippet{{ID: fixtureTaskID(2), OutlineID: pgtype.UUID{Bytes: node.ID, Valid: true}, Text: text}}
	piece := renderWritingPieceContext(selectWritingPieceContext(wr, []sqlc.WritingOutline{node}, snippets, nil, &node))
	out = append(out, PromptExample{ID: "writing/feedback/long-context", Class: gateway.ClassReview, Request: gateway.ChatRequest{Messages: []gateway.ChatMessage{
		{Role: gateway.RoleSystem, Content: buildWritingCommentSystem("zh", writingBlockCommentMaxIssues, writingKindPoint, helpAsk, genreArgument)},
		{Role: gateway.RoleUser, Content: buildWritingCommentPrompt(wr, "筑巢位置的选择", text, piece.Text, genreArgument)},
	}}, ContextFragments: map[string]promptassembly.Document{"piece": piece}})
	return out
}

// continuationExampleAssigned —— 一道读后续写题的题面（前文 + 两个段首句）。
//
// 只给 promptinspect 的样例用：判前输入检查**通过**之后 prompt 长什么样，
// 没有这一份就一个字都没被比过。内容是为这份样例写的，不是真题。
const continuationExampleAssigned = `阅读下面材料，根据其内容和所给段落开头语续写两段。

David had been training for the school marathon for three months. Every evening his
younger brother Toby waited at the gate with a packet of biscuits, and Toby had never
missed a single evening. On the day before the race David twisted his ankle on a loose
stone, and that night he sat on the step without saying anything while Toby held out
the biscuits and did not know what to say.

Paragraph 1: The next morning David woke to find the house completely silent.
Paragraph 2: When he finally reached the starting line, Toby was already there.`

