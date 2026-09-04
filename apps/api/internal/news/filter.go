package news

import (
	"sort"
	"strings"
	"time"
	"unicode"
)

// filter.go —— 从一堆抓回来的条目里，挑出**值得放进候选池**的那些。
//
// 三道，顺序固定：太旧的丢 → 政治的丢 → 重复的丢。放在模型调用之前，因为每
// 一条送进 prompt 的候选都在花钱，而这三件事都不需要智力。

/* ── 政治过滤 ───────────────────────────────────────────────────────────── */

// politicalTerms 是「这条新闻的主语是政治」的信号词。
//
// # 为什么这道过滤存在
//
// 这是给中学生的每日探索地图，不是新闻客户端。一条关于选举、战争、制裁的新闻，
// 无论多重要，都不是这个产品该替一个学生挑起的话题 —— 我们没有资格，也没有那个
// 语境去接住它引发的东西。
//
// # 🚨 为什么它必须**宁可漏掉也不要误伤**
//
// 科学新闻里合法地出现政治词汇是常态：「气候政策」「疫苗接种率」「科研经费」。
// 一个宁枉勿纵的过滤器会把气候科学整根枝砍掉 —— 而气候恰恰是这个产品最想给
// 学生的东西之一。所以这里只列**主语就是政治**的词，不列「和政治沾边」的词：
// 没有 policy、没有 government、没有 climate policy。
//
// 命中一个就丢。丢错一条的代价是今天少一个候选（池子里有几十条），而放过一条
// 的代价是一个十五岁的学生在一个学习产品里被推了一条战争新闻。
var politicalTerms = []string{
	// 英文
	"election", "elections", "electoral", "ballot", "referendum",
	"parliament", "congress", "senate", "presidential", "prime minister",
	"impeach", "coup", "sanction", "sanctions", "tariff", "tariffs",
	"war", "warfare", "invasion", "airstrike", "missile strike", "ceasefire",
	"troops", "military offensive", "insurgent", "terrorist", "terrorism",
	"protest", "protests", "riot", "unrest", "crackdown",
	"geopolitic", "diplomatic row", "espionage",
	// 中文
	"选举", "大选", "议会", "国会", "参议院", "众议院", "总统", "首相",
	"弹劾", "政变", "制裁", "关税", "战争", "战事", "空袭",
	"停火", "军事入侵", "武装冲突", "恐怖袭击", "示威", "骚乱", "镇压", "地缘政治",
}

// newsBannedInterests 是**不允许出现在每日星图上**的领域 id。
//
// 这不是说这些领域不好 —— 它们在词表里，学生完全可以因为自己读了、写了什么而
// 让「外交」长到自己的树上。这里挡的是另一件事：**一条落在这几个领域上的新闻，
// 主语就是当下的政治。**
//
// 🚨 它替掉的是一道真的救过场的闸。2026-09-03 漏过一条 "Venice Biennale
// President Defends Russia Inclusion"：英文原标题里一个信号词都没有，是模型
// 自己写的中文关键词「文化制裁边界」里的「制裁」把它拦下来的。2026-09-04 关键词
// 改成了闭表 id，那句中文没有了，词表匹配再也看不见这条。领域 id 是同一条信息
// 的另一种形态，而且比词表匹配更准 —— 它是模型对「这条新闻在讲什么」的判断，
// 不是一次字符串巧合。
//
// 名单保持很短，理由和词表一样：**宁可漏也不要误伤**。这里没有 climate-policy
// （气候政策正是这个产品最想给学生的东西），没有 law（一桩基因专利的判决是好
// 新闻），没有 inequality。只有主语就是政治的那几个。
var newsBannedInterests = map[string]bool{
	"elections":   true, // 选举
	"diplomacy":   true, // 外交
	"war-history": true, // 战争史
	"refugees":    true, // 难民
	"punishment":  true, // 惩罚与监狱
}

// IsBannedNewsInterest 报告这个领域 id 是否不该出现在每日星图上。
func IsBannedNewsInterest(id string) bool { return newsBannedInterests[id] }

// 🚨 从中文表里**拿掉**过的词，以及为什么 —— 别再加回来：
//
//   - 「入侵」：中文里它最常出现在**生物入侵**（入侵物种）里，那是生态学的核心
//     话题，也正是这个产品最想给学生的那类新闻。实测被它误杀过一条「蓝蟹入侵
//     亚得里亚海」。军事那层意思由「军事入侵」「武装冲突」「战争」覆盖。
//   - 「军事」：单独用会命中「军事医学」这类。改用「军事入侵」。
//   - 「抗议」：科学界的联署与抗议是科学政策新闻，不是政治新闻。
//
// 中文没有词边界可用，所以这里只能靠**词组够长、够具体**来避免误伤 —— 这和
// 英文那半边用词边界解决的是同一个问题。

// IsPolitical 说这条的主语是不是政治。
//
// 大小写不敏感；英文按**词边界**匹配，中文按子串匹配。词边界这件事不是洁癖：
// 不加的话 "war" 会命中 "warming"（全球变暖）、"warn"（预警）、"forward"，
// 而气候与预警正是我们最想留下的两类新闻。
func IsPolitical(title, summary string) bool {
	hay := strings.ToLower(title + " " + summary)
	for _, term := range politicalTerms {
		if !containsASCII(term) {
			if strings.Contains(hay, term) {
				return true
			}
			continue
		}
		if containsWord(hay, term) {
			return true
		}
	}
	return false
}

func containsASCII(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}

// containsWord 找一个以非字母数字为边界的出现。term 自身可以带空格
// （"prime minister"）。
func containsWord(hay, term string) bool {
	from := 0
	for {
		i := strings.Index(hay[from:], term)
		if i < 0 {
			return false
		}
		i += from
		beforeOK := i == 0 || !isWordByte(hay[i-1])
		end := i + len(term)
		afterOK := end >= len(hay) || !isWordByte(hay[end])
		if beforeOK && afterOK {
			return true
		}
		from = i + 1
	}
}

func isWordByte(b byte) bool {
	return b == '_' ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}

/* ── 去重 ───────────────────────────────────────────────────────────────── */

// normalizeTitle 把标题压成一个用来比对的键：小写、去掉所有非字母数字。
//
// 同一项研究会被 Phys.org、ScienceDaily、EurekAlert 同一天发三遍，标题只差
// 标点和一个副标题。不去重的话，五颗星里可能有三颗是同一件事。
func normalizeTitle(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Dedupe 按**链接**和**归一化标题**去重，保留先出现的那一条。
//
// 调用方按源的可信度顺序传进来（Sources 的顺序），于是 Nature 的版本会赢过
// 转载它的聚合站。
func Dedupe(items []Item) []Item {
	seenLink := map[string]bool{}
	seenTitle := map[string]bool{}
	out := make([]Item, 0, len(items))
	for _, it := range items {
		link := strings.TrimSpace(it.Link)
		title := normalizeTitle(it.Title)
		if title == "" {
			continue
		}
		if link != "" && seenLink[link] {
			continue
		}
		if seenTitle[title] {
			continue
		}
		if link != "" {
			seenLink[link] = true
		}
		seenTitle[title] = true
		out = append(out, it)
	}
	return out
}

/* ── 新鲜度 ─────────────────────────────────────────────────────────────── */

// FreshWithin 保留 `now` 之前 `window` 之内的条目。
//
// 🚨 **时间为零值的条目会被丢掉**，而不是当成新的。日期解析不出来的条目没有
// 办法判断新鲜度，把它当成今天的，就是让一条来路不明的旧闻混进「今日」。
//
// 未来时间保留：源的时区写错、或者一篇文章标着明天的日期，都不该被丢。
func FreshWithin(items []Item, now time.Time, window time.Duration) []Item {
	out := make([]Item, 0, len(items))
	cutoff := now.Add(-window)
	for _, it := range items {
		if it.Published.IsZero() || it.Published.Before(cutoff) {
			continue
		}
		out = append(out, it)
	}
	return out
}

// SourceIsAlive 说这个源还活着 —— 它最新的一条在 StaleAfter 之内。
//
// 见 sources.go：WHO 和 ESA 的一条 feed 都返回 200 但半年没更新过。只看状态码
// 的健康检查会一直报绿。
func SourceIsAlive(items []Item, now time.Time) bool {
	for _, it := range items {
		if !it.Published.IsZero() && it.Published.After(now.Add(-StaleAfter)) {
			return true
		}
	}
	return false
}

// SortByPublished 把新的排前面。零值排最后（它们本来也会被 FreshWithin 丢掉，
// 这里只是让顺序稳定）。
func SortByPublished(items []Item) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i].Published, items[j].Published
		if a.IsZero() != b.IsZero() {
			return b.IsZero()
		}
		return a.After(b)
	})
}

/* ── 铺开来源 ───────────────────────────────────────────────────────────── */

// InterleaveBySource 按来源轮转重排：每个源出一条，再每个源出第二条，依此类推。
// 每个源内部保持原有顺序（调用方已按新鲜度排过）。
//
// # 为什么需要它（2026-09-03 实测发现）
//
// 单纯按发布时间排序，会把当天发得最勤的那个源整块顶到前面。第一次真模型实测
// 里，五颗星**全部来自 Phys.org**，四颗落在同一根主枝上 —— 因为送进 prompt 的
// 前 40 条候选里，Phys.org 占了一大片。
//
// prompt 里已经写了「五条尽量分布在不同领域」，但那是在**求模型**做一件我们
// 本来就可以保证的事。一个「今天值得知道的五条」如果五条来自同一个网站，它就
// 不是今日星图，是那个网站的日报。所以铺开这件事在数据层做掉。
func InterleaveBySource(items []Item) []Item {
	if len(items) == 0 {
		return items
	}
	// 保持来源第一次出现的顺序 —— 调用方按可信度排过 Sources。
	order := make([]string, 0, 8)
	buckets := map[string][]Item{}
	for _, it := range items {
		if _, ok := buckets[it.Source]; !ok {
			order = append(order, it.Source)
		}
		buckets[it.Source] = append(buckets[it.Source], it)
	}

	out := make([]Item, 0, len(items))
	for round := 0; len(out) < len(items); round++ {
		progressed := false
		for _, src := range order {
			b := buckets[src]
			if round < len(b) {
				out = append(out, b[round])
				progressed = true
			}
		}
		if !progressed {
			break // 防御：不该发生，但绝不允许这里转成死循环
		}
	}
	return out
}
