package liteworkspace

import (
	"fmt"

	"mindimprint/api/internal/gateway"
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
  或者设成个性化阅读。不要编造文章标题。
- 你改不了的事不要说你改了。`

// AssignmentSystem renders the system prompt the teacher workspace turn
// loop sends ahead of the transcript.
func AssignmentSystem(c SystemContext) string {
	return fmt.Sprintf(assignmentSystemTemplate, c.TodayBeijing, c.ClassName, c.StudentCount)
}

// AssignmentTools returns the six tool schemas the model may call while
// building one homework card: set_fields, search_library, set_material,
// list_students, set_recipients, ask_choice.
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
					"kind": map[string]any{
						"type":        "string",
						"enum":        []string{"reading", "writing", "project"},
						"description": "作业种类：reading（阅读）、writing（写作）、project（项目）。",
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
			Description: "确定这次作业的阅读材料：库里的一篇文章，或者个性化阅读。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"source": map[string]any{
						"type":        "string",
						"enum":        []string{"library", "personalized"},
						"description": "material 来源：library（库里选定的文章）或 personalized（每个学生各自的个性化阅读）。",
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
						"type":        "string",
						"description": "source 为 library 时必填：本轮 search_library 结果里那篇文章的 slug，原样复制。不要按标题自己拼一个 slug。",
					},
					"tier": map[string]any{
						"type":        "integer",
						"description": "文章的难度档，可留空。",
					},
				},
				"required": []string{"source"},
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
			Name:        "set_recipients",
			Description: "设定这次作业发给谁。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"userIds": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "接收作业的学生 ID 列表。",
					},
				},
				"required": []string{"userIds"},
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
									"type": "string",
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
		},
	}
}
