package api

// interest_jobs.go —— 采集从「打开树时惰性补采」改成一个后台任务。
//
// # 为什么不再惰性
//
// 惰性那版把一次三到十秒的旗舰调用挂在 GET /interest/tree 上：她盯着自己的树
// 等它长出来。当时那么写有两个理由，写在旧版 interest_harvest.go 的头上：
// finishReading 是一次瞬时翻转，不能挂三秒调用；而且这个代码库里没有
// fire-and-forget 的先例，开这个先例的代价是无人观测的 goroutine。
//
// **队列把这两条都解决了。** river 的表 0016 就迁移过，只是一直没有 client 和
// worker。现在有了：完成时入队（一条 INSERT，不等待、不起 goroutine），
// worker 在后台采，树变成一次纯读。
//
// # 入队 + 扫尾，两条腿
//
// 只入队不够：入队失败、上线之前积压的、以及迁移 0133 清库之后要重采的那一批，
// 都没有人捞。只扫尾也不够：她读完一篇立刻打开树会看不到新词。所以两条都要。
//
// 重复入队是安全的、也是便宜的：harvestOneAtom 先取 advisory lock，再在锁里复查
// interest_harvested_at。输的那一个醒来看见章，直接返回，一分钱不花。所以这里
// **没有**用 river 的 unique 选项 —— 那是在为一件已经不疼的事再加一层配置。

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"

	"mindimprint/api/internal/store/sqlc"
)

// 扫尾一轮的三道限制。各挡一件事，见 queries/interest.sql 里的注释。
const (
	// sweepWindowDays 只看最近活动过的学生。没有这道门，一次上线会把历史上
	// 每一个完成过任何东西的账号都跑一遍 —— 钱花在早就不来的人身上。
	sweepWindowDays = 30
	// sweepPerUser 每个学生每轮最多几个。一个一口气读了二十篇的学生不能把
	// 这一轮占满，否则其他所有人都要等下一轮。
	sweepPerUser = 3
	// sweepTotal 整轮的上限，也就是一轮最多花多少次调用。
	sweepTotal = 60
	// sweepInterval 扫尾的节奏。两分钟：够快到「读完就去看树」多半已经长好，
	// 又够慢到不会在没有积压的时候空转成噪音。
	sweepInterval = 2 * time.Minute
)

/* ── 任务 ───────────────────────────────────────────────────────────────── */

// HarvestAtomArgs 采集一个已完成的 atom。
type HarvestAtomArgs struct {
	AtomID uuid.UUID `json:"atom_id"`
}

func (HarvestAtomArgs) Kind() string { return "interest_harvest" }

// SweepHarvestArgs 是周期扫尾。它自己不采，只把该采的 atom 入队 —— 一个任务
// 串行采六十个会把队列堵住，而拆开之后 worker 池能并行跑。
type SweepHarvestArgs struct{}

func (SweepHarvestArgs) Kind() string { return "interest_harvest_sweep" }

/* ── enqueue ────────────────────────────────────────────────────────────── */

// JobEnqueuer 是 Deps 上的入队接缝。
//
// 是接口而不是 *river.Client，因为**测试里它就是 nil**：internal/api 的测试不起
// 队列，而 EnqueueHarvest 在那种情况下必须是一次安静的空操作，不是一次 panic。
// 同 Deps.Provider == nil 的那几处 —— 那不是防御性代码，是走得到的分支。
type JobEnqueuer interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// EnqueueHarvest 在她完成一件事之后，请后台去采一次。
//
// **尽力而为**：入队失败只 warn。她刚点的是「完成」，那个动作已经成功了；为了
// 一条排不进去的任务把它变成一次报错，是拿她看得见的东西去赔一件她看不见的事。
// 漏掉的由扫尾捞回来。
func (a *API) EnqueueHarvest(ctx context.Context, atomID uuid.UUID) {
	if a.d.River == nil {
		return
	}
	if _, err := a.d.River.Insert(ctx, HarvestAtomArgs{AtomID: atomID}, nil); err != nil {
		slog.Warn("interest harvest: enqueue failed", "err", err, "atom_id", atomID)
	}
}

/* ── worker ─────────────────────────────────────────────────────────────── */

// HarvestWorker 采一个 atom。
type HarvestWorker struct {
	river.WorkerDefaults[HarvestAtomArgs]
	API *API
}

func (w *HarvestWorker) Work(ctx context.Context, job *river.Job[HarvestAtomArgs]) error {
	a := w.API
	row, err := a.d.Queries.GetAtomOwnerAndKind(ctx, job.Args.AtomID)
	if err != nil {
		// atom 被删了。任务没有意义了，**报成功** —— 重试一个永远失败的任务
		// 只会把它推到重试队列里反复出现。
		slog.Warn("interest harvest: atom gone", "err", err, "atom_id", job.Args.AtomID)
		return nil
	}
	a.harvestOneAtom(ctx, row.UserID, job.Args.AtomID, row.Kind)
	return nil
}

// SweepWorker 把该采还没采的 atom 入队。
type SweepWorker struct {
	river.WorkerDefaults[SweepHarvestArgs]
	API *API
}

func (w *SweepWorker) Work(ctx context.Context, job *river.Job[SweepHarvestArgs]) error {
	a := w.API
	rows, err := a.d.Queries.ListPendingHarvestAtoms(ctx, sqlc.ListPendingHarvestAtomsParams{
		WindowDays: sweepWindowDays, PerUser: sweepPerUser, Total: sweepTotal,
	})
	if err != nil {
		return err // 这一条值得重试：查询失败通常是数据库暂时不可达
	}
	if len(rows) == 0 {
		return nil
	}
	for _, r := range rows {
		if _, err := a.d.River.Insert(ctx, HarvestAtomArgs{AtomID: r.ID}, nil); err != nil {
			slog.Warn("interest harvest: sweep enqueue failed", "err", err, "atom_id", r.ID)
		}
	}
	slog.Info("interest harvest: swept", "queued", len(rows))
	return nil
}

/* ── 装配 ───────────────────────────────────────────────────────────────── */

// AttachRiver 把队列接到 API 上。
//
// 要有这个 setter，是因为构造顺序是个环：river client 要在构造时注册 worker，
// 而 worker 要拿到 *API，而 *API 的 Deps 里又要有 client 才能入队。cmd/api 的
// 顺序是：先建 API（River 为 nil）→ 用它建 worker 与 client → AttachRiver。
func (a *API) AttachRiver(c JobEnqueuer) { a.d.River = c }

// RegisterHarvestWorkers 把两个 worker 注册进 river 的 workers 集合。
func RegisterHarvestWorkers(w *river.Workers, a *API) error {
	if err := river.AddWorkerSafely(w, &HarvestWorker{API: a}); err != nil {
		return err
	}
	return river.AddWorkerSafely(w, &SweepWorker{API: a})
}

// HarvestPeriodicJob 是每两分钟一次的扫尾。
//
// RunOnStart：服务起来立刻扫一次，这样一次部署之后不用等两分钟才开始消化积压 ——
// 迁移 0133 清库之后，第一轮扫尾就是把所有人的树重新长出来的起点。
func HarvestPeriodicJob() *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(sweepInterval),
		func() (river.JobArgs, *river.InsertOpts) { return SweepHarvestArgs{}, nil },
		&river.PeriodicJobOpts{RunOnStart: true},
	)
}

// ErrNoRiver 说明这次调用需要队列而队列没起来。留给以后真的需要区分的调用方；
// 今天所有路径都是「没有就安静跳过」。
var ErrNoRiver = errors.New("river client not attached")

// StartHarvestQueue 建好 river client、注册两个 worker、挂上周期扫尾，并启动它。
//
// 返回的 client 由调用方在关服时 Stop。**启动失败不该拖垮整个服务**：队列是
// 可降级的子系统（采集慢一点），不是正确性不变量；为它拒绝启动会让整个接口
// 下线。所以 cmd/api 拿到 error 之后只 log，继续 serve。
func StartHarvestQueue(ctx context.Context, pool *pgxpool.Pool, a *API) (*river.Client[pgx.Tx], error) {
	workers := river.NewWorkers()
	if err := RegisterHarvestWorkers(workers, a); err != nil {
		return nil, err
	}
	c, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		// 四个并发。每个 worker 里是一次旗舰调用，所以这个数字既是吞吐也是
		// 同时在飞的模型请求数；扫尾一轮最多入队 60 个，四个一批慢慢消化，
		// 好过一次把六十个请求全甩给网关。
		Queues:       map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 4}},
		Workers:      workers,
		PeriodicJobs: []*river.PeriodicJob{HarvestPeriodicJob()},
	})
	if err != nil {
		return nil, err
	}
	if err := c.Start(ctx); err != nil {
		return nil, err
	}
	return c, nil
}
