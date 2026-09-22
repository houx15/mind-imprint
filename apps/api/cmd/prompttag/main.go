// Command prompttag 把作文题库那份源数据编译成 internal/promptlib/prompts.json。
//
//	set -a; . .deploy-local/env.prod; set +a
//	go run ./cmd/prompttag -src ../../docs/reference/writing-teaching/essay_prompts_2024_2026.json
//
// # 它做三件事
//
//  1. **抄**：id / 出处 / 年份 / 题面 / 字数要求 / 分值，原样搬过来，一个字不改。
//  2. **推**：语言和难度。确定性的，由考试类型算出来，不问模型 ——
//     「中考卷比 GRE 简单」是一条关于谁在考这张卷子的事实，不需要判断力。
//  3. **标**：话题。源数据里没有这个字段，而它恰恰是最难用规则做的一维
//     （「语言的滋味」「攀登是幸福的」按关键词分不出任何东西），所以跑一次模型。
//
// # 🚨 为什么标在这里，而不是她翻库的时候标
//
// 一次标好、随库提交，之后**她翻库一次模型调用都不发**：翻页、筛选、搜索
// 全是本地内存操作。标签进了 git，改动能在 pull request 里逐条看。
// 这和 internal/library 那份 articles.json 是同一个做法。
//
// # 🚨 默认不重标
//
// 已经有话题的题目会被原样留着，只给**没有话题的**发请求。所以这个命令可以
// 反复跑：加了新题就只标新题，改了 derive 规则重跑一遍一分钱不花。
// 要整库重标加 `-retag`。
//
// 判据照这个仓库的老规矩：闭表之外的标签一律丢掉（promptlib.TopicValid），
// 一道题最多留三个，一个都不认就空着 —— 空着的那一维筛选器里自然不出现，
// 比挂一个点进去为空的标签好。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/promptlib"
)

// 源数据的形状。字段名和 essay_prompts_2024_2026.json 一一对应。
type srcPrompt struct {
	ID           string `json:"id"`
	Category     string `json:"category"`
	Year         int    `json:"year"`
	Type         string `json:"type"`
	Source       string `json:"source"`
	Region       string `json:"region"`
	TaskType     string `json:"task_type"`
	PromptText   string `json:"prompt_text"`
	Requirements string `json:"requirements"`
	WordLimit    string `json:"word_limit"`
	Minutes      *int   `json:"time_limit_minutes"`
	FullScore    *int   `json:"full_score"`
	SourceURL    string `json:"source_url"`
	Notes        string `json:"notes"`
}

// langOf —— 这道题要用哪种语言写。
//
// 🚨 按**考试**判，不按题面判。中考英语的题面常常大半是中文（情景说明写中文、
// 只有最后一句说 write an email），按题面判会把它判成中文题，
// 而她要交的是一篇英文作文。
func langOf(category string) string {
	switch category {
	case "中考语文", "高考语文":
		return promptlib.LangZH
	case "中考英语", "高考英语", "托福写作", "雅思写作", "GRE写作":
		return promptlib.LangEN
	}
	return ""
}

// difficultyOf —— 三档，由考试类型派生。
//
// 它说的是「谁在考这张卷子」，不是「这道题有多难想」：
// 同一道题给初三和给研究生是两件事。
func difficultyOf(category string) int {
	switch category {
	case "中考语文", "中考英语":
		return promptlib.DiffBasic
	case "高考语文", "高考英语", "托福写作", "雅思写作":
		return promptlib.DiffAdvanced
	case "GRE写作":
		return promptlib.DiffExpert
	}
	return 0
}

const tagBatch = 8

func tagSystem() string {
	return `你在给一个写作题库打话题标签。

给你几道作文题，每道一个 id 和题面。为每一道挑 1–3 个话题。

**只能从下面这 16 个里挑，一个字都不能改，也不许自己造：**
` + strings.Join(promptlib.Topics, "\n") + `

挑的依据是**这道题要学生写什么**，不是题面里出现了哪些词。
例：「语言的滋味」写的是人怎么感受和使用语言 → 语言与表达、成长与自我。
例：一道让学生讨论政府该不该资助基础科研的题 → 科技与网络、社会与公共生活。

只输出 JSON，形如：
{"ZKYW-001":["语言与表达","成长与自我"],"GRE-014":["科技与网络"]}
不要解释，不要围栏。`
}

func main() {
	src := flag.String("src", "../../docs/reference/writing-teaching/essay_prompts_2024_2026.json",
		"源数据（gitignore 掉的那份）")
	out := flag.String("out", "internal/promptlib/prompts.json", "编译产物")
	retag := flag.Bool("retag", false, "整库重标话题（默认只标没有话题的）")
	dry := flag.Bool("dry", false, "不发请求，只跑抄和推那两步")
	// -reclean 拿**产物自己**当输入，重跑清洗和拆题那两步。
	//
	// 源数据是 gitignore 掉的，不在每台机器上；而清洗和拆题都是纯函数，
	// 对着已经清洗过的文本再跑一遍结果不变（clean_test.go 里那条
	// TestCleanPromptText_Idempotent 钉着这件事）。所以改了清洗规则之后，
	// 不需要那份源数据也能把整库重新洗一遍。
	reclean := flag.Bool("reclean", false, "拿 -out 当输入，只重跑清洗与拆题（不发请求）")
	flag.Parse()

	if *reclean {
		if err := runReclean(*out); err != nil {
			fmt.Fprintf(os.Stderr, "重洗失败：%v\n", err)
			os.Exit(1)
		}
		return
	}

	raw, err := os.ReadFile(*src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读源数据失败：%v\n", err)
		os.Exit(1)
	}
	var srcs []srcPrompt
	if err := json.Unmarshal(raw, &srcs); err != nil {
		fmt.Fprintf(os.Stderr, "源数据不是一个数组：%v\n", err)
		os.Exit(1)
	}

	// 上一版已经标好的话题，按 id 记下来 —— 默认不重标。
	prev := map[string][]string{}
	if !*retag {
		if b, rerr := os.ReadFile(*out); rerr == nil {
			var old []promptlib.Prompt
			if json.Unmarshal(b, &old) == nil {
				for _, p := range old {
					if len(p.Topics) > 0 {
						prev[p.ID] = p.Topics
					}
				}
			}
		}
	}

	items := make([]promptlib.Prompt, 0, len(srcs))
	for _, s := range srcs {
		lang := langOf(s.Category)
		diff := difficultyOf(s.Category)
		if lang == "" || diff == 0 {
			fmt.Fprintf(os.Stderr, "🚨 认不出的考试类型 %q（%s）—— 闭表要补一条，不要猜\n", s.Category, s.ID)
			os.Exit(1)
		}
		p := promptlib.Prompt{
			ID: s.ID, Category: s.Category, Lang: lang, Year: s.Year,
			Type: s.Type, Source: s.Source, Region: s.Region, TaskType: s.TaskType,
			// 🚨 题面先把卷面脚手架清掉（题号、第X节 书面表达、（20分））——
			// 那几行是卷子的结构，不是题目。见 promptlib.CleanPromptText。
			Text: promptlib.CleanPromptText(s.PromptText), Requirements: strings.TrimSpace(s.Requirements),
			WordLimit: s.WordLimit, SourceURL: s.SourceURL, Notes: s.Notes,
			Difficulty: diff, Topics: prev[s.ID],
		}
		if s.Minutes != nil {
			p.Minutes = *s.Minutes
		}
		if s.FullScore != nil {
			p.FullScore = *s.FullScore
		}
		items = append(items, p)
	}
	items = expandOptions(items)
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })

	todo := make([]int, 0, len(items))
	for i := range items {
		if len(items[i].Topics) == 0 {
			todo = append(todo, i)
		}
	}
	fmt.Fprintf(os.Stderr, "共 %d 道，要标话题的 %d 道\n", len(items), len(todo))

	if len(todo) > 0 && !*dry {
		if err := tagAll(items, todo); err != nil {
			fmt.Fprintf(os.Stderr, "标话题失败：%v\n", err)
			os.Exit(1)
		}
	}

	blank := 0
	for _, p := range items {
		if len(p.Topics) == 0 {
			blank++
		}
	}
	fmt.Fprintf(os.Stderr, "标完：%d 道没有话题\n", blank)

	buf, _ := json.MarshalIndent(items, "", "  ")
	if err := os.WriteFile(*out, append(buf, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "写产物失败：%v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "写好了：%s（%d 道）\n", *out, len(items))
}

func tagAll(items []promptlib.Prompt, todo []int) error {
	cat, err := gateway.DefaultCatalog()
	if err != nil {
		return err
	}
	httpc := &http.Client{Timeout: 5 * time.Minute}
	prov := gateway.NewMuxProvider(map[string]gateway.Provider{
		gateway.KindOpenAICompatible: gateway.NewCatalogProvider(httpc),
		gateway.KindAnthropic:        gateway.NewAnthropicProvider(httpc),
	})
	// digest：长输入短输出。给它一段题面，要回来的是几个标签。
	resolved, err := cat.Resolve(gateway.ClassDigest, "", gateway.OSEnvKeyLookup)
	if err != nil {
		return err
	}
	ctx := context.Background()

	for start := 0; start < len(todo); start += tagBatch {
		end := start + tagBatch
		if end > len(todo) {
			end = len(todo)
		}
		batch := todo[start:end]

		var b strings.Builder
		for _, i := range batch {
			p := items[i]
			fmt.Fprintf(&b, "\n--- id: %s（%s · %s）\n%s\n", p.ID, p.Category, p.TaskType, clip(p.Text, 700))
		}
		res, cerr := gateway.Collect(ctx, prov, resolved, gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: tagSystem()},
				{Role: gateway.RoleUser, Content: b.String()},
			},
		})
		if cerr != nil {
			fmt.Fprintf(os.Stderr, "  第 %d–%d 道调用失败，跳过：%v\n", start, end, cerr)
			continue
		}
		got := parseTags(res.Text)
		for _, i := range batch {
			kept := keepValid(got[items[i].ID])
			items[i].Topics = kept
		}
		fmt.Fprintf(os.Stderr, "  %d/%d\n", end, len(todo))
	}
	return nil
}

// keepValid 只留闭表里的，最多三个。
func keepValid(in []string) []string {
	out := make([]string, 0, 3)
	seen := map[string]bool{}
	for _, t := range in {
		t = strings.TrimSpace(t)
		if !promptlib.TopicValid(t) || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
		if len(out) == 3 {
			break
		}
	}
	return out
}

// parseTags 从回复里挑出那个 JSON 对象。模型偶尔会在前后加话，
// 所以夹第一个 { 到最后一个 }（这一步坏了只会让这一批空着，下次再跑补上）。
func parseTags(s string) map[string][]string {
	i := strings.IndexByte(s, '{')
	j := strings.LastIndexByte(s, '}')
	if i < 0 || j <= i {
		return nil
	}
	var m map[string][]string
	if json.Unmarshal([]byte(s[i:j+1]), &m) != nil {
		return nil
	}
	return m
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// expandOptions 把「从下面两个题目中任选一题」那几道拆开。
//
// 见 promptlib/options.go：库里有 24 道带这句引子，11 道的引子在说谎
//（源数据已经拆成两条，引子留着没删），13 道真的把两三个题目挤在一张卡上。
// 一张卡该是一道能写的题 —— 按下「用这道题写」之后，整段题面进
// `writing.assigned_prompt`，两个题目一起进去，印记就在拿两篇文章陪她想一篇。
func expandOptions(items []promptlib.Prompt) []promptlib.Prompt {
	out := make([]promptlib.Prompt, 0, len(items)+16)
	split := 0
	for _, p := range items {
		parts := promptlib.SplitPromptText(p.Text)
		if len(parts) == 1 {
			p.Text = parts[0].Text
			out = append(out, p)
			continue
		}
		split++
		for _, part := range parts {
			q := p
			q.ID = p.ID + part.Suffix
			q.Text = part.Text
			out = append(out, q)
		}
	}
	if split > 0 {
		fmt.Fprintf(os.Stderr, "拆开了 %d 道多选题 → 共 %d 道\n", split, len(out))
	}
	return out
}

// runReclean 拿产物当输入，重跑清洗与拆题，写回去。
func runReclean(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var items []promptlib.Prompt
	if err := json.Unmarshal(b, &items); err != nil {
		return err
	}
	before := len(items)
	for i := range items {
		items[i].Text = promptlib.CleanPromptText(items[i].Text)
	}
	items = expandOptions(items)
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	buf, _ := json.MarshalIndent(items, "", "  ")
	if err := os.WriteFile(path, append(buf, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "重洗完：%d 道 → %d 道\n", before, len(items))
	return nil
}
