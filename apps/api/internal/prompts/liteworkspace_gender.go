package prompts

// TeacherPronounRule controls references to students in teacher-facing output.
const TeacherPronounRule = "- **代词按「称谓」写。** 学生的称谓是「她」或「他」时，只用这个代词；" +
	"称谓是「未设置」时，不用「他」「她」指这名学生，重复姓名，或者写「该生」。" +
	"指多名学生时写「这些学生」「这两名学生」，不用「他们」「她们」。" +
	"称呼老师时用「您」。"
