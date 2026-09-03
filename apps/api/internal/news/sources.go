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

// Sources 是每天去抓的源。
//
// 🚨 **不收的三个，以及为什么不收：**
//   - Science (AAAS) 403、EurekAlert 403 —— 站点反爬返回 JS 挑战页。
//   - NASA 429 —— 对这个 IP 限流，三条 URL 都试过。
//
// 反爬是对方明确表达的意愿。**不做绕过反爬的事**，而下面这些源已经足够撑起
// 一天五颗星。
var Sources = []Source{
	{Name: "Nature", URL: "https://www.nature.com/nature.rss", Field: "science", Cap: 8, Timeout: 12 * time.Second},
	{Name: "Nature Climate Change", URL: "https://www.nature.com/nclimate.rss", Field: "science", Cap: 4, Timeout: 12 * time.Second},
	{Name: "Nature Ecology & Evolution", URL: "https://www.nature.com/natecolevol.rss", Field: "science", Cap: 4, Timeout: 12 * time.Second},
	{Name: "Quanta Magazine", URL: "https://api.quantamagazine.org/feed/", Field: "formal", Cap: 6, Timeout: 20 * time.Second},
	{Name: "arXiv · 天体物理", URL: "http://export.arxiv.org/rss/astro-ph", Field: "science", Cap: 5, Timeout: 15 * time.Second},
	{Name: "Phys.org", URL: "https://phys.org/rss-feed/", Field: "science", Cap: 8, Timeout: 12 * time.Second},
	{Name: "ScienceDaily", URL: "https://www.sciencedaily.com/rss/all.xml", Field: "science", Cap: 8, Timeout: 12 * time.Second},
	{Name: "NOAA", URL: "https://www.noaa.gov/rss.xml", Field: "science", Cap: 4, Timeout: 12 * time.Second},
	{Name: "ESA", URL: "https://www.esa.int/rssfeed/science", Field: "science", Cap: 4, Timeout: 12 * time.Second},
	{Name: "MIT Technology Review", URL: "https://www.technologyreview.com/feed/", Field: "making", Cap: 6, Timeout: 12 * time.Second},
	{Name: "Ars Technica · 科学", URL: "https://feeds.arstechnica.com/arstechnica/science", Field: "making", Cap: 6, Timeout: 12 * time.Second},
	{Name: "New Scientist", URL: "https://www.newscientist.com/feed/home/", Field: "science", Cap: 6, Timeout: 12 * time.Second},
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
