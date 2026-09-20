// costreport — 一次阅读 / 一次写作到底花了多少钱，花在哪儿。
//
// **这是一件工具，不是服务的一部分。** 它不被 cmd/api 引用，不在任何请求路径上。
//
// 为什么能算历史：`llm_call` 一直在记 prompt_tokens / completion_tokens / model /
// purpose / atom_id —— **token 一直都在，缺的只是价格**。2026-09-20 把百炼的费率
// 填进 models.json 之后，过去每一行都能重新按价目表算一遍，不用等新数据。
//
// 所以线上那一列的 0 不代表便宜，只代表当时没有价目表；这件工具绕过那一列，
// 直接拿 token 乘现在的价格。
//
//	DATABASE_URL=… go run ./cmd/costreport                    # 每种房间的平均一次
//	DATABASE_URL=… go run ./cmd/costreport -by purpose        # 钱花在哪个调用点
//	DATABASE_URL=… go run ./cmd/costreport -by model
//	DATABASE_URL=… go run ./cmd/costreport -atom <uuid>       # 具体某一次
//	DATABASE_URL=… go run ./cmd/costreport -since 2026-09-01
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/gateway"
)

type row struct {
	atomID   string
	kind     string
	purpose  string
	provider string
	model    string
	prompt   int
	compl    int
	cached   int
}

// cny prices one call at the catalog's rate. cached tokens bill at
// gateway.CachedInputRate; a row from before we recorded them reports 0, which
// prices it as a full miss — i.e. an OVER-estimate, never an under-estimate.
func (r row) cny() (float64, bool) {
	usd, ok := gateway.EstimateCostCached(r.provider, r.model, r.prompt, r.cached, r.compl)
	return usd * gateway.CNYPerUSD, ok
}

func main() {
	by := flag.String("by", "kind", "kind | purpose | model | atom")
	atom := flag.String("atom", "", "只看这一个 atom")
	since := flag.String("since", "", "只看这个日期之后（YYYY-MM-DD）")
	hasCached := flag.Bool("cached-col", false, "数据库已有 cached_tokens 列时打开")
	flag.Parse()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL 没设")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "连不上数据库：%v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	cachedCol := "0"
	if *hasCached {
		cachedCol = "coalesce(c.cached_tokens,0)"
	}
	q := `SELECT coalesce(c.atom_id::text,''), coalesce(a.kind,''), c.purpose,
	             c.provider, c.model, c.prompt_tokens, c.completion_tokens, ` + cachedCol + `
	        FROM llm_call c LEFT JOIN atom a ON a.id = c.atom_id
	       WHERE c.surface = 'lite'`
	var args []any
	if *atom != "" {
		args = append(args, *atom)
		q += fmt.Sprintf(" AND c.atom_id = $%d", len(args))
	}
	if *since != "" {
		args = append(args, *since)
		q += fmt.Sprintf(" AND c.created_at >= $%d", len(args))
	}

	rs, err := pool.Query(ctx, q, args...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "查询失败：%v\n", err)
		os.Exit(1)
	}
	defer rs.Close()

	var rows []row
	for rs.Next() {
		var r row
		if err := rs.Scan(&r.atomID, &r.kind, &r.purpose, &r.provider, &r.model,
			&r.prompt, &r.compl, &r.cached); err != nil {
			fmt.Fprintf(os.Stderr, "读行失败：%v\n", err)
			os.Exit(1)
		}
		rows = append(rows, r)
	}
	if err := rs.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "读行失败：%v\n", err)
		os.Exit(1)
	}
	if len(rows) == 0 {
		fmt.Println("没有 surface='lite' 的调用记录。")
		return
	}

	type agg struct {
		calls          int
		prompt, compl  int
		cny            float64
		unpriced       int
		atoms          map[string]bool
	}
	buckets := map[string]*agg{}
	atomTotal := map[string]float64{}
	atomKind := map[string]string{}
	var grand float64
	var unpriced int

	for _, r := range rows {
		var key string
		switch *by {
		case "purpose":
			key = r.purpose
		case "model":
			key = r.provider + "/" + r.model
		case "atom":
			key = r.atomID
		default:
			key = r.kind
			if key == "" {
				key = "(无 atom)"
			}
		}
		a := buckets[key]
		if a == nil {
			a = &agg{atoms: map[string]bool{}}
			buckets[key] = a
		}
		a.calls++
		a.prompt += r.prompt
		a.compl += r.compl
		if r.atomID != "" {
			a.atoms[r.atomID] = true
		}
		c, ok := r.cny()
		if !ok {
			a.unpriced++
			unpriced++
			continue
		}
		a.cny += c
		grand += c
		if r.atomID != "" {
			atomTotal[r.atomID] += c
			atomKind[r.atomID] = r.kind
		}
	}

	keys := make([]string, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return buckets[keys[i]].cny > buckets[keys[j]].cny })

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "按 %s 拆（surface=lite，价目表 %s，汇率 %.1f）\n\n", *by, gateway.PricingVersion, gateway.CNYPerUSD)
	fmt.Fprintln(w, strings.ToUpper(*by)+"\t调用数\t入 tokens\t出 tokens\t¥ 合计\t占比\t¥/次")
	for _, k := range keys {
		a := buckets[k]
		share := 0.0
		if grand > 0 {
			share = a.cny / grand * 100
		}
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%.4f\t%.1f%%\t%.4f\n",
			k, a.calls, a.prompt, a.compl, a.cny, share, a.cny/float64(a.calls))
	}
	fmt.Fprintf(w, "\n合计\t%d\t\t\t¥%.4f\n", len(rows), grand)
	if unpriced > 0 {
		fmt.Fprintf(w, "\n🚨 %d 次调用的模型不在价目表里，没有计入。补 models.json 再跑一遍。\n", unpriced)
	}

	// 「一次阅读 / 一次写作平均多少钱」—— 这才是能拿来做决定的那个数。
	if len(atomTotal) > 0 {
		perKind := map[string][]float64{}
		for id, c := range atomTotal {
			k := atomKind[id]
			if k == "" {
				k = "(未知)"
			}
			perKind[k] = append(perKind[k], c)
		}
		fmt.Fprintf(w, "\n每一次（按 atom 归总）\n\n种类\t次数\t平均 ¥\t中位 ¥\t最贵 ¥\n")
		kinds := make([]string, 0, len(perKind))
		for k := range perKind {
			kinds = append(kinds, k)
		}
		sort.Strings(kinds)
		for _, k := range kinds {
			v := perKind[k]
			sort.Float64s(v)
			sum := 0.0
			for _, x := range v {
				sum += x
			}
			fmt.Fprintf(w, "%s\t%d\t%.4f\t%.4f\t%.4f\n",
				k, len(v), sum/float64(len(v)), v[len(v)/2], v[len(v)-1])
		}
	}
	_ = w.Flush()
}
