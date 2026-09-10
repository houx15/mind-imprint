package news

import (
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/disciplines"
	"mindimprint/api/internal/interests"
)

// select.go —— 从候选池里挑今天的那几条，并给每一条归好类。
//
// **一天一次调用。** 抓取、去重、过滤、排序都在前面做完了，模型只负责它唯一
// 做得比规则好的那件事：判断哪几条对一个中学生**有意思**。
//
// 🚨 这一步**不写任何给学生看的字**。文案在 write.go 里写，写之前先把那一篇的
// 正文抓回来。分开的理由见 write.go 的文件头：合在一起时，模型手上只有 400 字
// 导语却要写出摘要和钩子，于是它把没读到的部分补全了。

// PlanetCount 是一天几颗星。
//
// 五。原型定的数，而它是对的：这一屏的承诺是「五条今天值得知道的事」，不是一个
// 信息流。一个刷不完的列表会把「每天来看一眼」变成「每天在这里待着」，那是另一
// 种产品。
const PlanetCount = 5

// maxCandidates 是最多送几条进 prompt。
//
// 候选池一天大概五六十条。全塞进去既贵又没用 —— 模型在第四十条之后就开始
// 敷衍。按新鲜度排完取前 40 条。
const maxCandidates = 40

// promptExcerptRunes 是每条候选给模型看多少字。
//
// 原来是 220 字，而且只取 description —— 对 Grist 那种源，description 只有
// 120 字符的导语，等于让模型**看着一句话**决定这条值不值得进今天的五颗星，
// 还要据此写出摘要和钩子。现在 Body 里有 feed 自己带的正文（见 parse.go），
// 给它 400 字：够判断这篇到底说了什么，又不至于让四十条候选把 prompt 撑爆
// （220→400 大约让候选那一段长一倍，仍远小于学科表加领域表）。
const promptExcerptRunes = 400

// ExcerptFor 交出这条候选**最值得给模型看的那一段**：feed 带了正文就用正文，
// 没带就用摘要。
//
// 取长的那个而不是永远取 Body：有些源（IEEE Spectrum 实测 8156 字符）把全文
// 放在 description 里，Body 反而是空的。
//
// 🚨 这是**内部处理**，不是转载。这段字只进 prompt，不落库、不显示给学生。
// 正文要显示给她，得先过许可那一关，那件事今天没做。
func ExcerptFor(it Item, runes int) string {
	best := strings.TrimSpace(it.Summary)
	if b := strings.TrimSpace(it.Body); len([]rune(b)) > len([]rune(best)) {
		best = b
	}
	return truncRunes(best, runes)
}

// Planet 是星图上的一颗星。
//
// 选星那一步只填得出下面这几个字段；给学生看的中文（TitleZh / Summary / Hook /
// Evidence）由 write.go 照着正文补上。一个 TitleZh 还空着的 Planet 是半成品，
// 不该落库。
type Planet struct {
	// Index 是它在**送进 prompt 的那批候选**里的下标，由服务端按标题回查得出，
	// 不是模型给的（见 anchorByTitle）。链接、出处、时间都从那一条上取。
	Index int
	// TitleEn 是原标题。
	TitleEn string
	// Written 是照着正文写出来的那几段中文，选星这一步是空的。
	Written
	// Field 是七根主枝之一；DisciplineID 是学科表里的一条。
	Field        string
	DisciplineID string
	// InterestID 是这条新闻落在领域词表里的哪一条。她收藏这颗星时，它会被
	// 种进树 —— 所以它必须出自那张闭表，否则收藏一颗星就把表外的词放回树上。
	// 允许为空：一条挑不出对应领域的新闻仍然是一颗好星，只是收藏它不长词。
	InterestID string
}

const selectSystemPrompt = `你在为一个中学生挑今天值得知道的科学新闻。她 15-18 岁，
读国际课程（IB / A-Level / AP）。

**这一步只挑，不写。** 挑完之后我们会去把每一篇的正文抓回来，照着正文写标题和摘要，
所以你现在一个字的中文文案都不用给 —— 把选择做对就行。

挑的标准，按重要性排：
- **能引出一个她可以自己追问的问题**。一条只能被记住、不能被追问的新闻不要。
- **必须落在至少三根不同的主枝上**（见下面 field 的七选一）。候选里本来就有人文、
  社会、艺术、心理类的条目，请真的用上它们 —— 全是自然科学不合格。
- 具体的发现优于综述，有数字、有方法、有争议的优于「科学家表示」。
- **标题本身就读不懂的不要**：一个术语堆成的论文标题，翻成中文照样读不懂，而她
  看到的就是那个标题。
- 不要政治、战争、灾难报道。不要健康建议类的软文。

每条只填四个字段：
- titleEn：**把候选里那条英文标题原样照抄**，一个字都不要改、不要截短。我们靠它
  回查你挑的是哪一条，对不上这条就作废了。
- field：七选一 —— formal（数学与形式）/ science（科学与自然）/ making（技术与创造）
  / society（社会与世界）/ humanities（人文与写作）/ arts（艺术与表达）/ self（自我与成长）
  按**这条新闻在问什么**判，不是按它发在哪个网站。
- disciplineId：从候选学科 id 里选一个最贴的。
- interestId：从**候选领域**里选一个 id 原样照抄。它是学生收藏这颗星时会加到
  她树上的那个词，所以选「这颗星在讲哪个领域」，不是「这条新闻的话题标签」。
  表里实在没有贴切的就留空字符串 —— 硬凑一个不相干的领域比留空糟得多。

只输出一个 JSON 对象，不要任何解释：
{"planets":[{"titleEn":"","field":"","disciplineId":"","interestId":""}]}`

// isQuestion —— 这句话是不是一个问题。
//
// 判据就是有没有问号（中英文都认）。这是 prompt 里那条「以问号结尾」在代码里
// 的那一半：一条只能在 prompt 里写、在代码里验不了的规矩，实测下来迟早会被
// 悄悄破掉，而破掉的样子是「它想问你」下面摆着一句陈述。
func isQuestion(s string) bool {
	return strings.ContainsAny(s, "？?")
}

// BuildSelectPrompt 拼出选星用的 system 与 user 两段，**并把真正写进 prompt 的
// 那批候选一起返回**。
//
// 🚨 第三个返回值不是为了方便：候选在这里会被截到 maxCandidates，而
// `ParseSelectReply` 要按下标回查链接与出处。调用方如果把**完整的池子**传给解析器，
// 下标的含义就和 prompt 里的不一样了。让这个函数交出它实际描述过的那个切片，
// 这一整类错位就不可能发生。
//
// 候选学科**全表都给**（42 条，只给 id 与中文名，不给方法与考纲）：这一步和
// 关键词路由不同 —— 路由时我们已经知道那个词属于哪根枝，所以只给同枝六门；
// 这里主枝本身就是待判定的，限定候选等于替模型先做了那个判断。
func BuildSelectPrompt(items []Item) (system, user string, candidates []Item) {
	if len(items) > maxCandidates {
		items = items[:maxCandidates]
	}
	var b strings.Builder
	b.WriteString("候选学科（id · 中文名）：\n")
	for _, d := range disciplines.All() {
		fmt.Fprintf(&b, "%s · %s\n", d.ID, d.Zh)
	}
	b.WriteString("\n候选领域（中文名 · id）：\n")
	b.WriteString(interests.PromptList())
	fmt.Fprintf(&b, "\n今天的候选新闻，共 %d 条。请挑 %d 条：\n\n", len(items), SelectCount())
	for i, it := range items {
		fmt.Fprintf(&b, "[%d] (%s) %s\n", i, it.Source, it.Title)
		if s := ExcerptFor(it, promptExcerptRunes); s != "" {
			fmt.Fprintf(&b, "    %s\n", s)
		}
	}
	return selectSystemPrompt, b.String(), items
}

type selectPick struct {
	TitleEn      string `json:"titleEn"`
	Field        string `json:"field"`
	DisciplineID string `json:"disciplineId"`
	InterestID   string `json:"interestId"`
}

type selectReply struct {
	Planets []selectPick `json:"planets"`
}

// ParseSelectReply 读选星的回话。
//
// 丢弃规则，按这个顺序：
//
//  1. 解析不出 JSON → **报错**，今天不出星图。绝不用昨天的冒充今天的，也绝不
//     编几条新闻（memory: ai-errors-must-surface-never-fake）。
//  2. titleEn 在候选里回查不到 → 丢。这条见下面那段 🚨。
//  3. field 不是七根主枝之一 → 丢。
//  4. interestId 落在禁掉的领域上 → 丢。
//  5. disciplineId 不在学科表里 → **不丢这颗星，只清空这条边**。学科连错比
//     没连上糟，但为了一条连错的边扔掉一条好新闻更糟。interestId 同理。
//  6. 同一条候选被挑了两次 → 只留第一个。
//  7. 超过 SelectCount 条 → 截断。
//
// 🚨 这里**不再**验 titleZh / hook：选星这一步压根不写它们。那几道校验搬去了
// write.go，并且严了不少 —— 那边验的是「你写的这句话，原文里有没有」。
func ParseSelectReply(raw string, candidates []Item) ([]Planet, error) {
	var rep selectReply
	body, err := sliceJSONObject(raw)
	switch {
	case err == nil:
		if uerr := json.Unmarshal(body, &rep); uerr != nil {
			return nil, fmt.Errorf("select reply is not the expected object: %w", uerr)
		}
	default:
		// 🚨 回复被截断时，**把写完的那几条捞出来**，不要整天没有星图。
		//
		// 2026-09-07 实测：钩子改成「问这条新闻自己站不站得住」之后，模型写得长
		// 了不少，五颗星那一回正好在收尾的 `]}` 之前断掉 —— 五颗完整的星球都在
		// 回复里，而整张星图因为少两个字符全丢了。一天只生成一次，所以这一下的
		// 代价是**当天没有探索地图**。
		//
		// 拆成两步之后这一段回复短了一个数量级（一条只剩四个短字段），截断已经
		// 很难发生了。留着：它不贵，而它挡的那件事一次的代价是一整天。
		salvaged, ok := salvagePlanets(raw)
		if !ok {
			return nil, err
		}
		rep = salvaged
	}

	out := make([]Planet, 0, SelectCount())
	seen := map[int]bool{}
	for _, p := range rep.Planets {
		if !disciplines.IsField(p.Field) {
			continue
		}
		// 🚨 **不信任模型给的下标 —— 所以干脆不问它要。** 2026-09-03 实测：模型
		// 描述了五条真实存在的新闻，却把它们一律编号成 0,1,2,3,4，下标指向的候选
		// 和它自己写的标题毫不相干。后果是每颗星球挂着一篇**无关文章**的链接与
		// 出处，而学生点「读原文」就落在那篇上。
		//
		// 所以按 titleEn（prompt 要求原样照抄）回查真正的那一条；查不到就丢掉。
		// 四颗真的星，好过五颗里有一颗指向随机文章。
		idx := anchorByTitle(p.TitleEn, candidates)
		if idx < 0 || seen[idx] {
			continue
		}
		if IsBannedNewsInterest(strings.TrimSpace(p.InterestID)) {
			continue
		}
		did := strings.TrimSpace(p.DisciplineID)
		if _, ok := disciplines.ByID(did); !ok {
			did = ""
		}
		seen[idx] = true
		out = append(out, Planet{
			Index:        idx,
			TitleEn:      strings.TrimSpace(candidates[idx].Title),
			Field:        p.Field,
			DisciplineID: did,
			InterestID:   keptInterestID(p.InterestID),
		})
		if len(out) == SelectCount() {
			break
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("select reply 里没有一条可用的新闻")
	}
	return out, nil
}

// sliceJSONObject 从模型的回话里切出那个 JSON 对象。
//
// 🚨 **数括号，不是「第一个 { 到最后一个 }」。**
//
// 原来那种取法在两种很常见的回法上会切出一段坏的：对象后面还跟着一段话而那段
// 话里也有花括号；或者对象前面那段话里就有花括号。2026-09-04 的模拟学生走查上，
// 六次启动撞上两次
//
//	生成失败：select reply is not the expected object: unexpected end of JSON input
//
// 而那一整天的探索地图就没了——星图一天只生成一次，切错一次就是二十个人一整天
// 看同一行英文报错。`pbl/jsonwire.go` 已经因为同一个原因换过一次取法，这里是
// 同一个改动的第二处。
//
// 🚨 光数括号还不够：**第一个配平的对象不一定是那个对象。** 模型写「我按
// {field} 这个字段挑的，结果如下：」，`{field}` 自己就是配平的，取它就等于把
// 整天的星图押在一句开场白上。所以配平之后再过一道 `json.Valid`——第一个真的
// 是合法 JSON 的那段才算数。
//
// 这不是「容错到什么都能过」：取出来之后照样严格 Unmarshal、照样逐条验（钩子
// 不能空、field 必须是七根主枝之一、下标要按标题回查）。放宽的只有「从哪儿到
// 哪儿是那段 JSON」，那本来就不该由模型的排版决定。
func sliceJSONObject(raw string) ([]byte, error) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")

	first := ""
	start, depth, inStr, esc := -1, 0, false, false
	for i, r := range s {
		if esc {
			esc = false
			continue
		}
		switch {
		case inStr && r == '\\':
			esc = true
		case r == '"':
			inStr = !inStr
		case inStr:
			// 字符串里的括号不算数。
		case r == '{':
			if depth == 0 {
				start = i
			}
			depth++
		case r == '}':
			if depth > 0 {
				depth--
				if depth == 0 && start >= 0 {
					span := s[start : i+1]
					if json.Valid([]byte(span)) {
						return []byte(span), nil
					}
					// 留着第一段配平但不合法的，实在找不到合法的时候拿它去
					// Unmarshal——那个报错比「没有 JSON 对象」具体得多。
					if first == "" {
						first = span
					}
					start = -1
				}
			}
		}
	}
	if first != "" {
		return []byte(first), nil
	}
	if start >= 0 {
		// 有开头没配平——模型被截断了。这和「压根没回 JSON」是两回事，说清楚，
		// 否则线上只能看到一句 "unexpected end of JSON input"，看不出是谁截断的。
		return nil, fmt.Errorf("reply 里的 JSON 对象没有写完（被截断了）")
	}
	return nil, fmt.Errorf("reply 里没有 JSON 对象")
}

// salvagePlanets 从一段没写完的回复里，把已经写完整的星球捞出来。
//
// 做法是从 `"planets"` 后面那个 `[` 起，用流式解码一颗一颗地读，读到断掉为止。
// 逐颗解码是关键：整段 Unmarshal 对截断的输入只会给一个错误，而这里每一颗完整
// 的对象都是独立可用的。
func salvagePlanets(raw string) (selectReply, bool) {
	i := strings.Index(raw, `"planets"`)
	if i < 0 {
		return selectReply{}, false
	}
	j := strings.Index(raw[i:], "[")
	if j < 0 {
		return selectReply{}, false
	}
	dec := json.NewDecoder(strings.NewReader(raw[i+j:]))
	if _, err := dec.Token(); err != nil { // 吃掉 '['
		return selectReply{}, false
	}
	var rep selectReply
	for dec.More() {
		var one selectPick
		if err := dec.Decode(&one); err != nil {
			break // 断在这一条上，前面那几条仍然算数。
		}
		rep.Planets = append(rep.Planets, one)
	}
	return rep, len(rep.Planets) > 0
}

func truncRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}


/* ── 按标题回查候选 ─────────────────────────────────────────────────────── */

// titleMatchThreshold 是判定「同一条新闻」所需的词重合度。
//
// 0.5：模型经常把长标题截短或去掉副标题，所以不能要求全等；但低于一半重合就
// 已经是两条不同的新闻了。宁可丢掉一颗星，也不要挂错一个链接。
const titleMatchThreshold = 0.5

// anchorByTitle 在候选里找出模型实际描述的那一条，返回它的下标；找不到返回 -1。
//
// 先试归一化后的完全相等与包含（最常见的情形：模型原样抄了标题，或者截短了
// 它）；都不中时退到词集合的重合度。
func anchorByTitle(titleEn string, candidates []Item) int {
	want := normalizeTitle(titleEn)
	if want == "" {
		return -1
	}
	// 一轮：归一化相等或互相包含。
	for i, c := range candidates {
		got := normalizeTitle(c.Title)
		if got == "" {
			continue
		}
		if got == want || strings.Contains(got, want) || strings.Contains(want, got) {
			return i
		}
	}
	// 二轮：词重合度最高的那条，且要过阈值。
	best, bestScore := -1, 0.0
	wantWords := wordSet(titleEn)
	if len(wantWords) == 0 {
		return -1
	}
	for i, c := range candidates {
		if sc := overlap(wantWords, wordSet(c.Title)); sc > bestScore {
			best, bestScore = i, sc
		}
	}
	if bestScore < titleMatchThreshold {
		return -1
	}
	return best
}

func wordSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(s)) {
		w = strings.Trim(w, ".,:;!?\"'()[]—-")
		// 一两个字母的词（a / of / in）不承载信息，只会把重合度虚抬上去。
		if len([]rune(w)) > 2 {
			out[w] = true
		}
	}
	return out
}

// overlap 是 |A∩B| / |A| —— 以模型给的标题为分母，因为它常常是候选标题的一个
// 截断版本，用并集当分母会把这种正确匹配压到阈值以下。
func overlap(a, b map[string]bool) float64 {
	if len(a) == 0 {
		return 0
	}
	n := 0
	for w := range a {
		if b[w] {
			n++
		}
	}
	return float64(n) / float64(len(a))
}

// keptInterestID 挡掉模型编出来的领域 id。
//
// 和学科那条边一样的姿态（丢弃规则第 6 条）：**不丢这颗星，只清空这个字段**。
// 一条挑不出领域的新闻仍然值得出现在星图上，学生只是收藏它的时候不长词。
// 编一个不存在的 id 进库，换来的是一颗永远种不出东西、也说不出为什么的星。
func keptInterestID(raw string) string {
	id := strings.TrimSpace(raw)
	if id == "" || !interests.Exists(id) {
		return ""
	}
	return id
}
