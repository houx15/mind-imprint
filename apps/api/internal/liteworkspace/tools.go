package liteworkspace

import (
	"fmt"
	"strings"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/liteparent"
)

// SystemContext is what the assignment prompt needs to know that the tools
// cannot tell it.
type SystemContext struct {
	ClassName    string
	TodayBeijing string // "2006-01-02"
	StudentCount int
}

const assignmentSystemTemplate = `你在帮一位老师布置作业。你的输出会填进右边的作业卡，老师看一眼就发布。

现在是北京时间 %s。班级是%s，共 %d 名学生。

## 你怎么问

- **一轮只问一个问题。** 问的时候尽量给选项，让老师点，而不是让她打字。
  给选项就调 ask_choice，一次 2 到 4 个。选项是名词或短动宾，不要写成句子。
- 老师已经说清楚的事不要再问。她说「这周读气候变化写议论文周五交」，
  你就直接把这些填进去，只问她还没说的那一件。
- 说话要短。不超过 120 个字。

## 硬规矩

- **不要在回复里写学生人数和学生姓名。** 它们会显示在卡片上，由系统查出来。
  你要指代的时候就说「这些学生」「名单上的学生」。
- **截止时间必须是绝对时刻**，格式 2006-01-02T15:04，按北京时间写。
  老师说「周五」，你按上面的今天算出是哪一天，自己写成绝对时刻。
- 材料只能从分级阅读库里选（先 search_library 再 set_material），
  或者设成个性化阅读，或者老师这一轮消息里贴了正文——这时候用
  set_material{source:"text", text:"..."}，text 必须逐字照抄她这一轮贴的内容，
  一个字都不能改、不能自己写或概括，也不能用她更早几轮贴过的文章。不要编造文章标题。
- 老师没有指定文章时，先调用 recommend_articles 给出推荐，不要凭空推荐。
- **提到文章标题或作业标题时，把标题放进《》里**（比如《美国气候队》），不要不加符号地写出来。
- 你改不了的事不要说你改了。`

// AssignmentSystem renders the system prompt the teacher workspace turn
// loop sends ahead of the transcript.
func AssignmentSystem(c SystemContext) string {
	return fmt.Sprintf(assignmentSystemTemplate, c.TodayBeijing, c.ClassName, c.StudentCount)
}

// AssignmentTools returns the seven tool schemas the model may call while
// building one homework card: set_fields, search_library, set_material,
// recommend_articles, list_students, set_recipients, ask_choice.
//
// list_students' filter enum must stay identical to the values
// ParseStudentFilter accepts. The schema is the only thing telling the model
// which filters exist; a value listed here that the parser rejects produces a
// tool call the server discards, which costs a round of the tool loop and
// tells the model nothing about why.
func AssignmentTools() []gateway.ChatTool {
	return []gateway.ChatTool{
		{
			Name:        "set_fields",
			Description: "写入作业卡的种类、标题、说明或截止时间。只填这一轮新确定的字段。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					// 🚨 The read-then-write case has to resolve one way, in the
					// contract, not by the model's mood. 「读一篇报道，写一篇
					// 议论文」 names two activities and the card holds one kind,
					// and only reading has a material row — so a homework that
					// carries an article is a reading homework and the writing
					// requirement lives in instructions. Without this sentence
					// the model picked writing, attached the article anyway,
					// and told her it was chosen while the card showed nothing.
					"kind": map[string]any{
						"type": "string",
						"enum": []string{"reading", "writing", "project"},
						"description": "作业种类：reading（阅读，作业带一篇阅读材料）、writing（写作，没有阅读材料这一栏）、project（项目）。" +
							"老师说「读一篇…再写一篇」这种读写结合的作业，选 reading，把写的要求写进 instructions。" +
							"只有 reading 能设材料；改成 writing 或 project 会把已经选好的文章清掉。" +
							"这个英文值只给系统识别用，不要写进给老师的回复——回复里说「阅读」「写作」「项目」。",
					},
					"title": map[string]any{
						"type":        "string",
						"description": "作业标题。",
					},
					"instructions": map[string]any{
						"type":        "string",
						"description": "给学生看的作业说明。",
					},
					"dueAt": map[string]any{
						"type":        "string",
						"description": "截止时间，必须是绝对时刻，格式 2006-01-02T15:04（如 2026-09-18T18:00），按北京时间写。不要写「周五」这类相对说法。",
					},
				},
			},
		},
		{
			Name:        "search_library",
			Description: "在分级阅读库里查文章，供 set_material 选用。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "标题关键词。",
					},
					"disciplines": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "限定学科，留空不限。",
					},
					"tier": map[string]any{
						"type":        "integer",
						"description": "限定难度档（1 到 5），0 表示不限。",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "set_material",
			Description: "确定这次作业的阅读材料：库里的一篇文章、个性化阅读，或者老师这一轮贴的正文。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"source": map[string]any{
						"type": "string",
						"enum": []string{"library", "personalized", "text"},
						"description": "material 来源：library（库里选定的文章）、personalized（每个学生各自的个性化阅读）" +
							"或 text（老师这一轮消息里贴的正文）。" +
							"这个英文值只给系统识别用，不要写进给老师的回复——回复里说「阅读库」「个性化阅读」或「她贴的正文」。",
					},
					// 🚨 The model guesses this value. In the 2026-09-16 live run
					// it minted american-climate-corps and
					// asian-games-offer-a-few-sports-you-may-not-recognize-like-kabaddi-and-wushu,
					// both derived from the title rather than copied from the
					// search result, and each wrong guess cost two model calls
					// to recover from. The description says 原样复制 because
					// that is the whole contract: the slug is an id we handed
					// back, not a name derivable from the title.
					"slug": map[string]any{
						"type": "string",
						"description": "source 为 library 时必填：本轮 search_library 或 recommend_articles 结果里那篇文章的 slug，原样复制。" +
							"不要按标题自己拼一个 slug。这个值只给系统识别用，不是给老师看的内容——" +
							"回复里提到这篇文章要说书名号里的中文标题，不要写 slug。",
					},
					"tier": map[string]any{
						"type":        "integer",
						"description": "文章的难度档，可留空。",
					},
					// 铁律①：这里绝不是让模型写文章。text 必须是老师这一轮消息里
					// 已经出现过的原文，服务端会逐字核对——不是这一轮贴的、或者
					// 被改写过、概括过、自己续写过，这次调用都会被拒绝。
					"text": map[string]any{
						"type": "string",
						"description": "source 为 text 时必填：老师这一轮消息里贴的文章正文，逐字照抄，" +
							"一个字都不能改、不能概括、不能自己写。不是这一轮贴的内容就不要用这个来源。",
					},
				},
				"required": []string{"source"},
			},
		},
		{
			// recommend_articles gives the model something to call BEFORE the
			// teacher has named an article — a class-wide reading pick, not
			// one student's. It is pure computation (every enrolled student's
			// interest strengths summed into one profile, scored the same way
			// Recommend scores one student), so there is nothing to search for
			// with search_library first.
			Name: "recommend_articles",
			Description: "为这个班推荐几篇分级阅读库文章，按全班学生的兴趣画像聚合打分，老师没有指定文章时先用这个，不要凭空推荐。" +
				"每条结果带 slug——那是给 set_material 用的系统参数，不要出现在给老师的回复里；" +
				"介绍文章时用书名号里的中文标题（zhTitle）。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"disciplines": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "限定学科，留空按全班兴趣不限学科地推荐。",
					},
				},
			},
		},
		{
			Name:        "list_students",
			Description: "按一个闭集条件查班级名单，结果显示在卡片上，不要在回复里复述。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"filter": map[string]any{
						"type":        "string",
						"enum":        []any{"all", "inactive_this_week", "has_overdue", "no_writing_yet"},
						"description": "all（全部）、inactive_this_week（本周未活跃）、has_overdue（有逾期作业）、no_writing_yet（还没写过作文）。",
					},
				},
				"required": []string{"filter"},
			},
		},
		{
			// filter is here so 「发给全班」 costs one tool call instead of
			// three. The model used to answer that intent with
			// set_recipients{userIds:["all"]} — a filter name in the id field —
			// and spend two more model calls recovering via list_students. One
			// turn reached 6 of 6 doing exactly that.
			//
			// The enum must stay identical to ParseStudentFilter's, which is
			// also list_students': one vocabulary for 「这批学生是谁」, named the
			// same way in both places.
			Name:        "set_recipients",
			Description: "设定这次作业发给谁：给一个闭集条件，或者给明确的学生 ID 列表。只给其中一个。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"filter": map[string]any{
						"type":        "string",
						"enum":        []any{"all", "inactive_this_week", "has_overdue", "no_writing_yet"},
						"description": "按条件发：all（全班）、inactive_this_week（本周未活跃）、has_overdue（有逾期作业）、no_writing_yet（还没写过作文）。用这个就不用先 list_students。",
					},
					"userIds": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "接收作业的学生 ID 列表，必须是 list_students 返回的 id，原样复制。要发给一整类学生就用 filter。",
					},
				},
			},
		},
		{
			Name:        "ask_choice",
			Description: "结束这一轮，给老师 2 到 4 个按钮选，而不是问一个开放式问题。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"question": map[string]any{
						"type":        "string",
						"description": "要问的问题，一句话。",
					},
					"options": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"id": map[string]any{
									"type":        "string",
									"description": "这个选项的标识，你自己取。不要把文章 slug 塞进来——那是 slug 字段的事。",
								},
								"label": map[string]any{
									"type": "string",
								},
								// The field that stops the model from smuggling
								// a slug into the id. See liteworkspace.Choice.
								"slug": map[string]any{
									"type":        "string",
									"description": "选项的意思是「用这篇文章」时填：search_library 结果里的 slug，原样复制。老师点了它，材料就直接定成这篇，你不用再查一次。",
								},
							},
							"required": []string{"id", "label"},
						},
						"description": "2 到 4 个选项，超过 4 个会被截断。",
					},
				},
				"required": []string{"question", "options"},
			},
		},
	}
}

const homeSystemTemplate = `你在帮一位老师了解她的一个班：回答关于这个班和学生的问题，需要时给她一个前往某个页面的入口。

现在是北京时间 %s。班级是%s，共 %d 名学生。

## 你怎么答

- 只根据 class_snapshot、list_students、list_assignments 的结果说事实，不要编。
- 她想去某个页面看时，用 open_page 给她一个入口；你自己不会跳转，老师点了才跳。
- 一轮只问一个问题。需要她选的时候用 ask_choice，一次 2 到 4 个选项。
- 说话要短。不超过 120 个字。

## 硬规矩

- **不要在回复里写学生人数、学生姓名，也不要复述作业各状态的人数。** 它们会显示在
  卡片上，由系统查出来。你要指代的时候就说「这些学生」「名单上的学生」「这份作业」。
- **提到作业标题时，把标题放进《》或「」里**（比如《3 人小组汇报》），不要不加符号地写出来。
- 你改不了的事不要说你改了。`

// HomeSystem renders the system prompt the home workspace turn loop sends
// ahead of the transcript (§12.5, D2's class chat).
func HomeSystem(c SystemContext) string {
	return fmt.Sprintf(homeSystemTemplate, c.TodayBeijing, c.ClassName, c.StudentCount)
}

// HomeTools returns the five tool schemas the model may call while helping a
// teacher understand one class: class_snapshot, list_students,
// list_assignments, open_page, ask_choice.
//
// list_students' filter enum must stay identical to ParseStudentFilter's —
// same reason AssignmentTools' comment gives: the schema is the only thing
// telling the model which filters exist.
func HomeTools() []gateway.ChatTool {
	return []gateway.ChatTool{
		{
			Name: "class_snapshot",
			Description: "查这个班这周（进行中）的整体情况：活跃学生数、完成项数、作业按时完成率，" +
				"以及哪些学生值得表扬、哪些需要关注。结果显示在卡片上，不要在回复里复述具体人数或姓名。",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "list_students",
			Description: "按一个闭集条件查班级名单，结果显示在卡片上，不要在回复里复述。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"filter": map[string]any{
						"type":        "string",
						"enum":        []any{"all", "inactive_this_week", "has_overdue", "no_writing_yet"},
						"description": "all（全部）、inactive_this_week（本周未活跃）、has_overdue（有逾期作业）、no_writing_yet（还没写过作文）。",
					},
				},
				"required": []string{"filter"},
			},
		},
		{
			Name: "list_assignments",
			Description: "查这个班布置过的作业：类型、标题、截止时间，以及各状态的人数。" +
				"结果显示在卡片上，不要在回复里复述具体人数。",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "open_page",
			Description: "给老师一个「前往某个页面」的入口。你自己不会跳转，这只是给她点的一个按钮。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"target": map[string]any{
						"type": "string",
						"enum": []any{"classWeekly", "student", "assignmentNew", "assignment", "parentReports"},
						"description": "classWeekly（本周报告）、student（某个学生的学习页，要给 userId）、" +
							"assignmentNew（布置作业）、assignment（某份作业详情，要给 assignmentId）、" +
							"parentReports（家长报告）。这个英文值只给系统识别用，不要写进给老师的回复。",
					},
					"userId": map[string]any{
						"type": "string",
						"description": "target 为 student 时必填：list_students 返回的学生 id，原样复制。" +
							"这个值只给系统识别用，不是给老师看的内容。",
					},
					"assignmentId": map[string]any{
						"type": "string",
						"description": "target 为 assignment 时必填：list_assignments 返回的作业 id，原样复制。" +
							"这个值只给系统识别用，不是给老师看的内容。",
					},
				},
				"required": []string{"target"},
			},
		},
		plainAskChoiceTool(),
	}
}

// plainAskChoiceTool is ask_choice without the article slug field: the home
// and parent report surfaces offer options that are never an article.
func plainAskChoiceTool() gateway.ChatTool {
	return gateway.ChatTool{
		Name:        "ask_choice",
		Description: "结束这一轮，给老师 2 到 4 个按钮选，而不是问一个开放式问题。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"question": map[string]any{
					"type":        "string",
					"description": "要问的问题，一句话。",
				},
				"options": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id": map[string]any{
								"type":        "string",
								"description": "这个选项的标识，你自己取。",
							},
							"label": map[string]any{
								"type": "string",
							},
						},
						"required": []string{"id", "label"},
					},
					"description": "2 到 4 个选项，超过 4 个会被截断。",
				},
			},
			"required": []string{"question", "options"},
		},
	}
}

// ReportSystemContext is what the parent report prompt needs to know that the
// tools cannot tell it. Canvas is the report as it stands (section headings in
// Chinese, current text, visible facts), rendered by the caller.
type ReportSystemContext struct {
	TodayBeijing string // "2006-01-02"
	ClassName    string
	StudentName  string
	Canvas       string
}

// ReviseSectionMaxRunes is revise_section's cap on one section. It equals the
// report editor's own cap (liteParentBodySectionMax in
// apps/api/internal/api/lite_parent_report.go), which the PATCH that saves the
// section enforces; a longer text would pass the tool and fail at save.
const ReviseSectionMaxRunes = 2000

const reportSystemTemplate = `你在帮一位老师修改一份给家长的学习报告。报告由系统根据学生的学习记录生成，老师审阅后发给家长。

现在是北京时间 %s。班级是%s。这份报告写的是%s。

## 你怎么做

- 老师说要改哪一段、怎么改，你就调用 revise_section，写出改好的整段。只改老师要改的段落。
- 改好的段落会显示在右边的报告里，不要在回复里整段复述，说明改了哪一段即可。
- 老师没说清楚改哪一段或怎么改时，用 ask_choice 给 2 到 4 个选项。一轮只问一个问题。
- 说话要短。不超过 120 个字。

## 硬规矩（revise_section 会逐条检查，不通过会返回错误，按错误重写后再调用）

- 只使用下面「可用的事实」里的内容，不补充事实，不评价学生的人格。
- 引用学生原话时用「」，逐字照抄事实里的原话。作品标题用《》，只有学生原话用「」。
- 数字一律用阿拉伯数字，只用事实里出现的数字，照抄，不做加减和单位换算。
- 不写其他学生的名字。
- 说明文，不用比喻、抒情和套话。列举多条时不编号，每条单独一行。
- 提到段落时用段落标题（如「阅读」「下一步建议」）。
- 你改不了的事不要说你改了。

%s`

// ReportSystem renders the system prompt the parent report workspace turn
// loop sends ahead of the transcript (§5.3, §12.6).
func ReportSystem(c ReportSystemContext) string {
	return fmt.Sprintf(reportSystemTemplate, c.TodayBeijing, c.ClassName, c.StudentName, c.Canvas)
}

// reportSectionEnumText lists every section key with its heading, in
// liteparent.SectionKeys order. It reads liteparent.SectionLabels, so the
// tool description holds no second copy of the table.
func reportSectionEnumText() string {
	parts := make([]string, 0, len(liteparent.SectionKeys))
	for _, k := range liteparent.SectionKeys {
		parts = append(parts, k+"（"+liteparent.SectionLabels[k]+"）")
	}
	return strings.Join(parts, "、")
}

// ReportTools returns the two tool schemas the model may call while revising
// a parent report: revise_section and ask_choice.
//
// section's enum is every key; which of them this report shows is checked
// when the tool runs, because it depends on the report's visible facts.
func ReportTools() []gateway.ChatTool {
	enum := make([]any, 0, len(liteparent.SectionKeys))
	for _, k := range liteparent.SectionKeys {
		enum = append(enum, k)
	}
	return []gateway.ChatTool{
		{
			Name: "revise_section",
			Description: "把报告的一个段落替换成改写后的整段文字。改写按生成报告时的规则检查：" +
				"引文和作品标题必须出自事实，数字必须出自事实，不能出现其他学生的名字。" +
				"不通过时返回错误，按错误改好后再调用。写入的是右边编辑器里的内容，老师还可以再改。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"section": map[string]any{
						"type": "string",
						"enum": enum,
						"description": "要改写的段落：" + reportSectionEnumText() +
							"。只能是「报告现在的内容」里列出的段落。" +
							"也可以直接填段落标题。这个英文值只给系统识别用，不要写进给老师的回复，回复里说段落标题。",
					},
					"text": map[string]any{
						"type":        "string",
						"description": fmt.Sprintf("改写后的整段正文，不超过 %d 字，不带段落标题。", ReviseSectionMaxRunes),
					},
				},
				"required": []string{"section", "text"},
			},
		},
		plainAskChoiceTool(),
	}
}
