package news

import "time"

// sources.go —— 今日新闻星图的源清单。
//
// 写在代码里而不是建一张表，理由和 `disciplines.json` 一样：**这是内容，不是
// 用户数据。** 加一个源是一次发布，不是一次运营动作，而它需要被 review。
//
// 每条源都实测过（2026-09-03，从生产 ECS `47.93.151.131` 跑 curl），记录见
// `docs/2026-09-03-news-feed-reachability-from-beijing-ecs.md`。三个源明确
// **不收**，理由写在下面。

// Source 是一个 RSS / Atom 源。
type Source struct {
	// Name 是学生在星球上看见的出处。
	Name string
	URL  string
	// Field 是这个源的主枝倾向，用来让一天的五颗星尽量铺开而不是全挤在科学上。
	// 它是**倾向不是断言** —— 单条新闻最终归哪根枝由模型按内容判断。
	Field string
	// Cap 是从这个源最多取几条。arXiv 一天全量 414 KB，不限量会把候选池淹掉。
	Cap int
	// Timeout 是这一个源的超时。Quanta 实测 11 秒，比别人慢一个数量级，
	// 但内容最适合中学生，所以给它更长的时间而不是把它踢掉。
	Timeout time.Duration
}

// Sources 是每天去抓的源，按主枝分组。
//
// # 🚨 为什么它必须覆盖七根主枝，而不只是科学
//
// 第一版只有科学源，于是第一次真模型实测里**五颗星全落在科学上**。prompt 里
// 写着「五条尽量分布在不同领域」，但候选池里根本没有别的领域 —— 那是在求模型
// 做一件它做不到的事。一屏「今天值得知道的五条」如果永远是五条科学新闻，它就
// 不是探索地图，是科学日报。所以 2026-09-03 补进了人文 / 社会 / 艺术 / 自我
// 四组源，并把科学源的 Cap 调低。
//
// # 不收的这些，以及为什么
//
//   - Science (AAAS) 403、EurekAlert 403 —— 站点反爬返回 JS 挑战页。
//   - NASA 429 —— 对这个 IP 限流，三条 URL 都试过。
//   - The Conversation —— 25 秒超时、零字节（2026-09-03 与 2026-09-08 两次实测）。
//   - Public Domain Review —— 最新一条五周前，过不了 StaleAfter。
//   - Yale E360 / Carbon Brief / Mongabay / 知识分子 —— HTTP 错误（2026-09-08）。
//   - Pew Research / 澎湃 / Hugging Face / BAIR —— 北京出口 25 秒超时（2026-09-08）。
//
// 反爬是对方明确表达的意愿。**不做绕过反爬的事。**
//
// # arXiv 拿掉了（2026-09-08）
//
// `export.arxiv.org/rss/astro-ph` 返回 **HTTP 200、892 字节、零条 item** ——
// 一个格式完全正确、但一条内容都没有的 feed。换 `rss.arxiv.org`、换子分类
// （astro-ph.GA）、换 cs.AI，四个组合全都是零条；当天 08:31 到 12:00 UTC
// 之间反复量过，一次都没有过内容。它每天占一次 HTTP 请求、在日志里留一行
// 「解析不出任何条目」，而那行错误久了就会教我们忽略所有错误。
//
// 这不是解析器的毛病（`Parse` 对它报错是对的），是这个源对我们没有产出。
// 要让 arXiv 回来，先量出一个真的有条目的时段和 URL，再连着量几天。
var Sources = []Source{
	// ── 科学与自然 ───────────────────────────────────────────────────────
	// 🚨 Cap 比第一版调低了（Nature 8→6、Phys.org 8→5、ScienceDaily 8→5）。
	// 不是因为它们不好，是因为**十二个源里九个偏科学**时，候选池会被科学淹没，
	// 而这一屏该给的是七根主枝上的五颗星，不是五条科学日报。
	{Name: "Nature", URL: "https://www.nature.com/nature.rss", Field: "science", Cap: 6, Timeout: 12 * time.Second},
	{Name: "Nature Climate Change", URL: "https://www.nature.com/nclimate.rss", Field: "science", Cap: 3, Timeout: 12 * time.Second},
	{Name: "Nature Ecology & Evolution", URL: "https://www.nature.com/natecolevol.rss", Field: "science", Cap: 3, Timeout: 12 * time.Second},
	{Name: "Phys.org", URL: "https://phys.org/rss-feed/", Field: "science", Cap: 5, Timeout: 12 * time.Second},
	{Name: "ScienceDaily", URL: "https://www.sciencedaily.com/rss/all.xml", Field: "science", Cap: 5, Timeout: 12 * time.Second},
	{Name: "NOAA", URL: "https://www.noaa.gov/rss.xml", Field: "science", Cap: 3, Timeout: 12 * time.Second},
	{Name: "ESA", URL: "https://www.esa.int/rssfeed/science", Field: "science", Cap: 3, Timeout: 12 * time.Second},
	{Name: "New Scientist", URL: "https://www.newscientist.com/feed/home/", Field: "science", Cap: 4, Timeout: 12 * time.Second},

	// ── 数学与形式 ───────────────────────────────────────────────────────
	{Name: "Quanta Magazine", URL: "https://api.quantamagazine.org/feed/", Field: "formal", Cap: 5, Timeout: 20 * time.Second},

	// ── 技术与创造 ───────────────────────────────────────────────────────
	{Name: "MIT Technology Review", URL: "https://www.technologyreview.com/feed/", Field: "making", Cap: 5, Timeout: 12 * time.Second},
	{Name: "Ars Technica · 科学", URL: "https://feeds.arstechnica.com/arstechnica/science", Field: "making", Cap: 4, Timeout: 12 * time.Second},

	// ── 人文与写作 ───────────────────────────────────────────────────────
	// 2026-09-03 加的一批。第一次真模型实测里五颗星全落在科学上，因为源清单
	// 本身就只有科学 —— prompt 里求模型「分布在不同领域」，而候选池里根本没有
	// 别的领域，那是在求它做一件做不到的事。
	{Name: "Aeon", URL: "https://aeon.co/feed.rss", Field: "humanities", Cap: 4, Timeout: 12 * time.Second},
	{Name: "JSTOR Daily", URL: "https://daily.jstor.org/feed/", Field: "humanities", Cap: 4, Timeout: 15 * time.Second},
	{Name: "Smithsonian", URL: "https://www.smithsonianmag.com/rss/latest_articles/", Field: "humanities", Cap: 4, Timeout: 12 * time.Second},

	// ── 社会与世界 ───────────────────────────────────────────────────────
	{Name: "Nature Human Behaviour", URL: "https://www.nature.com/nathumbehav.rss", Field: "society", Cap: 4, Timeout: 12 * time.Second},
	{Name: "Our World in Data", URL: "https://ourworldindata.org/atom.xml", Field: "society", Cap: 3, Timeout: 12 * time.Second},
	{Name: "Nature Sustainability", URL: "https://www.nature.com/natsustain.rss", Field: "society", Cap: 3, Timeout: 12 * time.Second},

	// ── 艺术与表达 ───────────────────────────────────────────────────────
	{Name: "Colossal", URL: "https://www.thisiscolossal.com/feed/", Field: "arts", Cap: 4, Timeout: 12 * time.Second},
	{Name: "Hyperallergic", URL: "https://hyperallergic.com/feed/", Field: "arts", Cap: 4, Timeout: 15 * time.Second},

	// ── 自我与成长 ───────────────────────────────────────────────────────
	{Name: "Psyche", URL: "https://psyche.co/feed", Field: "self", Cap: 4, Timeout: 12 * time.Second},
}

// StaleAfter 是一个源多久没有新条目就当它死了。
//
// 🚨 这条判据是这次实测里最值钱的发现：**HTTP 200 不等于这个源还活着。**
// WHO 的 feed 返回 200、140 KB，最新一条是六个月前；ESA 的
// `Our_Activities/Space_News` 返回 200，最新一条是八个月前。一个只看状态码的
// 健康检查会一直报绿，而星图上会出现半年前的「今日新闻」。
//
// 十四天：够宽松，容得下一个源过节停更几天；够严格，挡得住一个已经死掉的源。
const StaleAfter = 14 * 24 * time.Hour
