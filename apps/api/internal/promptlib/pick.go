package promptlib

import (
	"hash/fnv"
	"sort"
)

// 推荐：从 728 道里挑几道摆在她面前。
//
// # 🚨 这里**不猜她的兴趣**
//
// 分级阅读库那边能按她兴趣树上的学科推荐，因为文章本来就有学科标签，
// 两头对得上。作文题这边没有那座桥：话题是「成长与自我」「社会与公共生活」，
// 和兴趣树上的「电池」「太阳能」不是一回事，硬接一条映射就是在编。
//
// 所以推荐只用**真的知道的三件事**：
//
//   - 她最近在用哪种语言写（没有就两种都给）
//   - 她这个难度（默认进阶；调用方可以按年级给）
//   - 她已经开过的那些题，不再推
//
// 剩下的按「真题优先、年份新的优先」排，再**按话题去重**铺开 ——
// 一屏四道题全是「成长与自我」，看起来就像库里只有这一类。
//
// Why 只在真的有理由时才填。把「我随便挑的」说成「按你的兴趣挑的」是在骗人
// （同 library.Recommend 的那条注释）。

// Pick 是一条推荐。
type Pick struct {
	Prompt Prompt
	// Why 空着就表示这一条不是按她什么特征挑的，界面据此换一种说法。
	Why []string
}

// Profile 是推荐要用到的、关于她的那点事实。
type Profile struct {
	// Lang 空表示不限。
	Lang string
	// Difficulty 0 表示不限。
	Difficulty int
	// Started 是她已经从库里开过的题 id，不再推。
	Started map[string]bool
	// Seed 让同一个人拿到稳定的一组，不同的人拿到不同的一组 ——
	// 每次刷新都换一批，她会觉得刚才看到的那道题找不回来了。
	Seed string
}

// Recommend 挑 n 道。
func Recommend(p Profile, n int) []Pick {
	load()
	if loadErr != nil || n <= 0 {
		return nil
	}

	type scored struct {
		p     Prompt
		score int
		why   []string
	}
	var pool []scored
	for _, it := range all {
		if p.Started[it.ID] {
			continue
		}
		if p.Lang != "" && it.Lang != p.Lang {
			continue
		}
		if p.Difficulty != 0 && it.Difficulty != p.Difficulty {
			continue
		}
		s := 0
		var why []string
		// 真题比模拟题值得先看：它真的被考过。
		switch it.Type {
		case "真题":
			s += 30
		case "官方题库", "官方样题":
			s += 25
		case "回忆版":
			s += 15
		}
		s += it.Year // 年份新的靠前
		if p.Lang != "" {
			why = append(why, langLabel(it.Lang))
		}
		if p.Difficulty != 0 {
			why = append(why, diffLabel(it.Difficulty))
		}
		// 同一个人稳定、不同的人不同：拿 seed+id 做一个小抖动，
		// 免得所有人的首页推的是同样那四道。
		s += int(hash(p.Seed+it.ID) % 17)
		pool = append(pool, scored{it, s, why})
	}
	sort.Slice(pool, func(i, j int) bool {
		if pool[i].score != pool[j].score {
			return pool[i].score > pool[j].score
		}
		return pool[i].p.ID < pool[j].p.ID
	})

	// 按话题铺开：先每个话题各取一道，不够再放宽。
	out := make([]Pick, 0, n)
	used := map[string]bool{}
	for _, s := range pool {
		if len(out) == n {
			break
		}
		t := ""
		if len(s.p.Topics) > 0 {
			t = s.p.Topics[0]
		}
		if t != "" && used[t] {
			continue
		}
		used[t] = true
		out = append(out, Pick{Prompt: s.p, Why: s.why})
	}
	for _, s := range pool {
		if len(out) == n {
			break
		}
		already := false
		for _, o := range out {
			if o.Prompt.ID == s.p.ID {
				already = true
				break
			}
		}
		if !already {
			out = append(out, Pick{Prompt: s.p, Why: s.why})
		}
	}
	return out
}

func langLabel(l string) string {
	if l == LangZH {
		return "中文"
	}
	return "英文"
}

// DiffLabel 是难度的中文名。界面和这里共用一份，免得两处各写各的。
func DiffLabel(d int) string { return diffLabel(d) }

func diffLabel(d int) string {
	switch d {
	case DiffBasic:
		return "基础"
	case DiffAdvanced:
		return "进阶"
	case DiffExpert:
		return "高阶"
	}
	return ""
}

func hash(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}
