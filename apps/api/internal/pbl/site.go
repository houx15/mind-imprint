package pbl

// site.go — 她的主页的内容模型，以及「她自己写的字」和「她真做过的事」怎么合成
// 一个页面。
//
// ## 这个文件存在的理由
//
// 原型里这一层是三个写死的常量：`STUDENT = { name: "林知遥", handle: "zhiyao" }`、
// 一份 READINGS、一份 WRITINGS。跑一遍真实流程之后页面是这样的：页头写着
// 「林知遥 · 初二学生」，紧接着的一行是她自己写的「我是侯知遥，读 IB 的高二学
// 生」；关于那一段是别人的完整自传（台灯、螺丝、213 颗）；文章和作品两栏一条都
// 不是她的。十一个区块里只有两个是她的。
//
// 所以这里有一条硬规则：**页面上的每一个字，要么是她敲进去的，要么是从她真实的
// reading / writing / pbl_project 行里读出来的。这个文件里没有第三种来源。**
// 没有兜底文案，没有示例内容，没有「先放一段让页面好看点」。她没写的地方就是
// 空的，而空的地方由发布门槛（SiteMissing）挡住，不由假字填上。
//
// ## 为什么没有 handle 和 domain
//
// 原型给每个人发了一个 `zhiyao.me` 和 `hi@zhiyao.me`。那是一个不存在的域名和一
// 个不存在的信箱，印在页头和页脚上，是这个页面上最像真的一处假话。真实地址是
// 发布后拿到的那条 /p/<token> 链接，它在她自己的界面里显示，不假装成一个域名。
//
// ## 为什么联系方式默认是空的
//
// 不从账号邮箱自动带出来。spec §15 之所以让这条链接不可索引，就是因为她是未成
// 年人；把她的登录邮箱印在一个公开页面上，比让页面被搜到更糟。要不要留联系方
// 式、留哪个，是她自己决定的一件事。

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

// SiteTheme — 版式带的四个颜色/字体变量。渲染端只认这四个。
type SiteTheme struct {
	Paper  string `json:"paper"`
	Ink    string `json:"ink"`
	Accent string `json:"accent"`
	Font   string `json:"font"`
}

// SiteStat — 站点信息里的一行。
type SiteStat struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// SiteProject — 主页上的一件作品。
type SiteProject struct {
	ID    string `json:"id"`
	Year  string `json:"year"`
	Kind  string `json:"kind"`
	Title string `json:"title"`
	// 她自己写的那一两句。空着就是空着——真实个人站上，介绍作品的那句话就是
	// 作者写的那句话，截取正文前 44 个字再加省略号是爬虫干的事。
	Blurb string `json:"blurb"`
	// 代替照片的双色版。它不假装是一张照片，所以也不需要「这里缺一张照片」的
	// 虚线框——它就是这一页自己的图案。
	Plate [2]string `json:"plate"`
}

// SitePost — 主页上的一篇文章。
type SitePost struct {
	ID    string   `json:"id"`
	Date  string   `json:"date"`
	Title string   `json:"title"`
	Blurb string   `json:"blurb"`
	Kind  string   `json:"kind"`
	Words int      `json:"words"`
	Tags  []string `json:"tags"`
}

// SiteRead — 在读。
type SiteRead struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Source string `json:"source"`
	// 她自己写的一句。我们不替她总结她读过的东西。
	Takeaway string `json:"takeaway"`
}

// SiteContent — 渲染端拿到的全部东西。三个版式都是它的纯函数。
type SiteSection struct {
	ImageKey string `json:"imageKey,omitempty"`
	ImageURL string `json:"imageUrl,omitempty"`
	Key      string `json:"key"`
	Title    string `json:"title"`
	Depth    int    `json:"depth"`
	Body     string `json:"body"`
}

type SiteContent struct {
	Sections []SiteSection `json:"sections,omitempty"`
	Name     string        `json:"name"`
	Role     string        `json:"role"`
	Headline string        `json:"headline"`
	Lead     string        `json:"lead"`
	Now      string        `json:"now"`
	Motto    []string      `json:"motto"`
	Stats    []SiteStat    `json:"stats"`
	Tags     []string      `json:"tags"`
	Projects []SiteProject `json:"projects"`
	Posts    []SitePost    `json:"posts"`
	Reads    []SiteRead    `json:"reads"`
	About    []string      `json:"about"`
	NowList  []string      `json:"nowList"`
	Email    string        `json:"email"`
	Updated  string        `json:"updated"`
	// 画头图用的种子。由她的名字派生，所以每个人的头图都不一样，而且同一个人
	// 每次都一样。见前端 parts.tsx 的 Banner。
	Seed int `json:"seed"`
}

// SiteDraft — 存进 pbl_site.content 的那一份，**只有她写的字**。
//
// 三张列表不在这里：它们每次渲染时从真实的行里现拼。存快照等于存一份会过期的
// 副本，而主页最不该做的事就是与她真做过的事对不上。
type SiteDraft struct {
	Sections []SiteSection `json:"sections,omitempty"`
	Role     string        `json:"role"`
	Headline string        `json:"headline"`
	Lead     string        `json:"lead"`
	Now      string        `json:"now"`
	Motto    []string      `json:"motto"`
	Tags     []string      `json:"tags"`
	About    []string      `json:"about"`
	NowList  []string      `json:"nowList"`
	Contact  string        `json:"contact"`
	// 每条作品 / 文章 / 在读，她自己写的那一句，按稳定 id 存。
	Blurbs map[string]string `json:"blurbs"`
}

// SiteItem — 从真实行里读出来的一条，三种之一。API 层负责把 sqlc 的行转成它，
// 这样这个文件不依赖 sqlc，可以纯逻辑地测。
type SiteItem struct {
	// AtomID 只用来算稳定 id，绝不进入返回给访客的内容。
	AtomID string
	Title  string
	When   time.Time
	// HasWhen 区分「零值时间」和「真的没有时间」——一篇没写完成时间的文章，
	// 日期栏该是空的，不该是 0001-01-01。
	HasWhen bool
	Words   int
	Source  string
	// Kind 是显示用的那个词：作品用 状态，文章用 分类。
	Kind string
}

/* ── 稳定 id ──────────────────────────────────────────────────────────── */

// SiteItemID 把一颗 atom 映射成主页上一个短的、不透明的 id。
//
// 🚨 不能直接用 atom 的 uuid。公开页面是任何人都能打开的，而 uuid 指向的是她
// 名下一行真实的数据，会出现在她其他链接里；把它印在公开 JSON 上，等于把一个
// 可寻址的句柄发给陌生人（atom_report_share.go 的文件头讲的是同一件事）。
//
// 也不能用下标（w1 / w2）。她给每条作品写的那句话是按 id 存的，而下标会在她写
// 完下一篇文章时整体移位——于是她为 A 写的介绍，某天早上出现在 B 底下。
//
// 所以：用户 id 加 atom id 一起哈希。同一个学生看同一条永远是同一个 id，换个
// 学生就完全不同，反推不回 atom id。
func SiteItemID(userID, atomID string) string {
	sum := sha256.Sum256([]byte(userID + ":" + atomID + ":site-item"))
	return hex.EncodeToString(sum[:6])
}

// siteSeed 由她的名字派生一个稳定的数，前端拿它画这一页自己的头图。
//
// 原型的头图是一盏台灯和散在天上的螺丝——那是林知遥的故事，画在每一个学生的
// 页顶上。头图得是这一页自己的图案，所以它由她派生，不由一个写死的场景派生。
func siteSeed(name string) int {
	sum := sha256.Sum256([]byte("banner:" + name))
	n := 0
	for _, b := range sum[:4] {
		n = n<<8 | int(b)
	}
	if n < 0 {
		n = -n
	}
	return n % 100000
}

/* ── 版面色板 ─────────────────────────────────────────────────────────── */

// sitePlates — 代替照片的双色版。刻意不是「app 的封面渐变 + 中间一个图标」：
// 中间带图标的方块是产品家具，也是原型那一版最像产品截图的一处。
var sitePlates = [][2]string{
	{"#6E4F8E", "#33224A"},
	{"#3F7E5C", "#1C4432"},
	{"#4A4E60", "#22242E"},
	{"#A2603C", "#4E2A18"},
	{"#3C6480", "#1B3242"},
	{"#8A5566", "#42222E"},
}

// plateFor 由稳定 id 选一块色版，所以同一件作品的颜色不会因为她新做了一个项目
// 而整排换掉。
func plateFor(id string) [2]string {
	n := 0
	for _, c := range id {
		n = (n*31 + int(c)) % 1000003
	}
	return sitePlates[n%len(sitePlates)]
}

/* ── 合成 ─────────────────────────────────────────────────────────────── */

// SiteInput 是 BuildSite 要的全部东西。
type SiteInput struct {
	// DisplayName 来自 users.display_name。这是她自己的名字，这一层没有别的
	// 名字可用，也不该有。
	DisplayName string
	Draft       SiteDraft
	Projects    []SiteItem
	Posts       []SiteItem
	Reads       []SiteItem
	// Since 建站时间；Updated 最后更新。
	Since   time.Time
	Updated time.Time
}

// BuildSite 把她写的字和她做过的事合成一页。
//
// 规则只有一条：这里不生成任何内容。它只搬运和排列。
func BuildSite(in SiteInput) SiteContent {
	d := in.Draft

	projects := make([]SiteProject, 0, len(in.Projects))
	for _, it := range in.Projects {
		projects = append(projects, SiteProject{
			ID:    it.AtomID,
			Year:  yearOf(it),
			Kind:  it.Kind,
			Title: it.Title,
			Blurb: strings.TrimSpace(d.Blurbs[it.AtomID]),
			Plate: plateFor(it.AtomID),
		})
	}

	posts := make([]SitePost, 0, len(in.Posts))
	for _, it := range in.Posts {
		posts = append(posts, SitePost{
			ID:    it.AtomID,
			Date:  dateOf(it),
			Title: it.Title,
			Blurb: strings.TrimSpace(d.Blurbs[it.AtomID]),
			Kind:  it.Kind,
			Words: it.Words,
			Tags:  nil,
		})
	}
	// 一份不按日期排的文章列表，读者还没读到第一个字就已经觉得这页是坏的。
	sort.SliceStable(posts, func(i, j int) bool { return posts[i].Date > posts[j].Date })

	reads := make([]SiteRead, 0, len(in.Reads))
	for _, it := range in.Reads {
		reads = append(reads, SiteRead{
			ID:       it.AtomID,
			Title:    it.Title,
			Source:   it.Source,
			Takeaway: strings.TrimSpace(d.Blurbs[it.AtomID]),
		})
	}

	stats := []SiteStat{}
	if !in.Since.IsZero() {
		stats = append(stats, SiteStat{Label: "建站", Value: in.Since.Format("2006 年 1 月")})
	}
	// 数字只报真的，而且为 0 的那一行直接不出现——「写了 0 篇」是一句没人会在
	// 自己主页上写的话。
	if n := len(posts); n > 0 {
		stats = append(stats, SiteStat{Label: "文章", Value: fmt.Sprintf("%d 篇", n)})
	}
	if n := len(projects); n > 0 {
		stats = append(stats, SiteStat{Label: "成果", Value: fmt.Sprintf("%d 件", n)})
	}
	if n := len(reads); n > 0 {
		stats = append(stats, SiteStat{Label: "阅读", Value: fmt.Sprintf("%d 篇", n)})
	}

	updated := ""
	if !in.Updated.IsZero() {
		updated = in.Updated.Format("2006-01-02")
	}

	return SiteContent{
		Sections: d.Sections,
		Name:     strings.TrimSpace(in.DisplayName),
		Role:     strings.TrimSpace(d.Role),
		Headline: strings.TrimSpace(d.Headline),
		Lead:     strings.TrimSpace(d.Lead),
		Now:      strings.TrimSpace(d.Now),
		Motto:    nonEmpty(d.Motto),
		Stats:    stats,
		Tags:     nonEmpty(d.Tags),
		Projects: projects,
		Posts:    posts,
		Reads:    reads,
		About:    nonEmpty(d.About),
		NowList:  nonEmpty(d.NowList),
		Email:    strings.TrimSpace(d.Contact),
		Updated:  updated,
		Seed:     siteSeed(in.DisplayName),
	}
}

func yearOf(it SiteItem) string {
	if !it.HasWhen {
		return ""
	}
	return it.When.Format("2006")
}

func dateOf(it SiteItem) string {
	if !it.HasWhen {
		return ""
	}
	return it.When.Format("2006-01-02")
}

func nonEmpty(xs []string) []string {
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if s := strings.TrimSpace(x); s != "" {
			out = append(out, s)
		}
	}
	return out
}

/* ── 发布门槛 ─────────────────────────────────────────────────────────── */

// SiteMissing 报告这一页还缺哪些**她自己的字**，按发布时该先补哪一个排序。
//
// 🚨 这是原型那个 bug 真正的解药。原型能发布一个一个字都不是她的页面，因为缺的
// 地方全被示例内容填满了，看上去是满的。这里没有示例内容，所以缺的地方是空的；
// 空的地方拦住发布，并且明说缺什么。
//
// 门槛只管三样：她是谁（role）、这一页在问什么（headline）、她想说的那段话
// （about）。作品和文章可以一条都没有——一个刚开始的人本来就没有；但一个没有
// 一句她自己的话的页面，发布出去代表不了任何人。
func SiteMissing(c SiteContent) []string {
	var out []string
	if c.Name == "" {
		out = append(out, "你的名字")
	}
	if c.Headline == "" {
		out = append(out, "首屏介绍")
	}
	if c.Role == "" {
		out = append(out, "个人简介")
	}
	if len(c.Sections) > 0 {
		filled := false
		for _, section := range c.Sections {
			if strings.TrimSpace(section.Body) != "" {
				filled = true
			}
		}
		if !filled {
			out = append(out, "主页模块的正文")
		}
	}
	// A confirmed outline holds the student's own introduction/content in
	// sections. Do not require a duplicate paragraph above that outline.
	if len(c.Sections) == 0 && len(c.About) == 0 {
		out = append(out, "关于我的正文")
	}
	return out
}

// SiteLayouts 是三个真正不同的版面（spec §15）。
var SiteLayouts = []string{"essay", "ledger", "magazine"}

// IsSiteLayout 校验她挑的那一个。
func IsSiteLayout(s string) bool {
	for _, l := range SiteLayouts {
		if l == s {
			return true
		}
	}
	return false
}

// SitePublishMissing is shared by the preview and publishing endpoint.
func SitePublishMissing(content SiteContent, palette Palette) []string {
	missing := SiteMissing(content)
	if !ValidPalette(palette) {
		missing = append(missing, "请在视觉基调中确认配色和风格")
	}
	if missing == nil {
		return []string{}
	}
	return missing
}
