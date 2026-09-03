package news

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// fetch.go —— 把 Sources 抓成一个候选池。
//
// # 并行，每个源自己的超时
//
// 串行抓十二个源，光 Quanta 一个就要 11 秒（实测），加起来会让第一个打开星图的
// 学生等半分钟。所以并行，并且**每个源用自己的超时**（Source.Timeout）——
// 一个统一的短超时会把 Quanta 永久踢掉，而它的内容恰恰最适合中学生。
//
// # 一个源挂掉不影响别人
//
// 每个源的失败都只是 warn + 少几条候选。一天里有一半源挂掉，星图照样出，只是
// 候选池小一点。**唯一不能做的是拿昨天的冒充今天的。**

// userAgent 说明我们是谁，并留一个能找到我们的地址。
//
// 不伪装成浏览器。三个源（Science、EurekAlert、NASA）明确用反爬和限流拒绝了
// 我们，那是对方表达的意愿 —— 它们已经从 Sources 里拿掉了，不绕过。
const userAgent = "MindImprintBot/0.1 (+https://mind.uni-robot.cn; 每日五条科学新闻，给中学生)"

// maxFeedBytes 是单个 feed 最多读多少。arXiv 实测 414 KB；2 MB 足够宽松，
// 又挡得住一个源突然返回一个几百兆的东西。
const maxFeedBytes = 2 << 20

// Fetcher 抓一组源。client 可注入，测试因此不需要网络。
type Fetcher struct {
	Client *http.Client
	// Now 可注入，好让新鲜度判断在测试里是确定的。
	Now func() time.Time
}

func NewFetcher() *Fetcher {
	return &Fetcher{
		Client: &http.Client{
			// 单条请求的兜底。真正生效的是每个源自己的 Timeout（用 ctx 施加）。
			Timeout: 30 * time.Second,
		},
		Now: time.Now,
	}
}

// FetchAll 并行抓所有源，返回一个**已经去重、已按新鲜度排序**的候选池。
//
// window 是「多新算今天的」。给 48 小时而不是 24：源的时区五花八门，而一条
// 昨天下午发的 Nature 论文今天依然是新闻。
func (f *Fetcher) FetchAll(ctx context.Context, window time.Duration) []Item {
	now := f.Now()

	type result struct {
		idx   int
		items []Item
	}
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		res  = make([]result, 0, len(Sources))
	)

	for i, src := range Sources {
		wg.Add(1)
		go func(i int, src Source) {
			defer wg.Done()
			items, err := f.fetchOne(ctx, src)
			if err != nil {
				slog.Warn("news: source failed", "source", src.Name, "err", err)
				return
			}
			// 🚨 状态码 200 不等于这个源还活着。见 sources.go 的 StaleAfter：
			// WHO 返回 200 / 140 KB，最新一条是六个月前。
			if !SourceIsAlive(items, now) {
				slog.Warn("news: source is stale — newest item is too old",
					"source", src.Name, "items", len(items))
				return
			}
			SortByPublished(items)
			if len(items) > src.Cap {
				items = items[:src.Cap]
			}
			for j := range items {
				items[j].Source = src.Name
				items[j].Field = src.Field
			}
			mu.Lock()
			res = append(res, result{idx: i, items: items})
			mu.Unlock()
		}(i, src)
	}
	wg.Wait()

	// 按 Sources 的顺序拼回去，让去重时**先出现的赢** —— Nature 该赢过转载
	// 它的聚合站，而 goroutine 的完成顺序是随机的。
	pool := make([]Item, 0, 64)
	for i := range Sources {
		for _, r := range res {
			if r.idx == i {
				pool = append(pool, r.items...)
			}
		}
	}

	pool = Dedupe(pool)
	pool = FreshWithin(pool, now, window)
	pool = dropPolitical(pool)
	// 先按新鲜度排，**再按来源轮转铺开**。只排新鲜度的话，当天发得最勤的那个源
	// 会整块占住送进 prompt 的前 40 条 —— 实测过一次：五颗星全部来自 Phys.org。
	SortByPublished(pool)
	return InterleaveBySource(pool)
}

func dropPolitical(items []Item) []Item {
	out := make([]Item, 0, len(items))
	for _, it := range items {
		if IsPolitical(it.Title, it.Summary) {
			continue
		}
		out = append(out, it)
	}
	return out
}

func (f *Fetcher) fetchOne(ctx context.Context, src Source) ([]Item, error) {
	ctx, cancel := context.WithTimeout(ctx, src.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml;q=0.9, */*;q=0.8")

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedBytes))
	if err != nil {
		return nil, err
	}
	return Parse(body)
}
