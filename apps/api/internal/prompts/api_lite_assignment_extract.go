package prompts

// Purpose: api/lite_assignment_extract.go 的固定提示词与条件指令。
// Consumer: internal/api/lite_assignment_extract.go; builders choose conditions and render context.
// Comments are maintenance metadata and are never sent to a model.

// AssignmentExtractSystem retains the production text of assignmentExtractSystem.
// assignmentExtractSystem asks for the three writing-assignment settings the
// form has. The prompt field keeps the teacher's own words: this call is a
// compose step over text she already wrote, not a rewrite.
const AssignmentExtractSystem = `你从老师粘贴的一段作业说明里提取写作作业的三项设置。
只输出 JSON：{"prompt":"","targetWords":null,"lang":"zh"}。
prompt：学生要写的题目或要求，保留老师的原话，不改写、不补充。
targetWords：老师写明的字数；写的是范围取上限；没写就填 null。
lang：作文要用的语言，中文填 zh，英文填 en。`
