package api

import (
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

// pbl_refeed.go —— 把她在工具里做出来的东西，交回给印记。
//
// 🚨 这是 AGENTS.md 那条主线里一直缺的一环：
//
//	决策层 → summon_card → Card Runtime → **回灌陪练** → 过程树
//
// 「回灌」这个箭头以前不存在。她在便签板上摆了十五分钟、把问题改写了一遍、
// 挑了一个方案并写下理由，然后回到对话——而印记收到的 prompt 和上一轮**一模
// 一样**。它没看见任何一张便签。
//
// 于是它只能接着聊上一轮那件事，或者把她刚做完的事再问一遍。学生那边的感受
// 很直接：这个东西没在听我说话，那我为什么要认真填。
//
// 工具卡是「由人来执行的工具」，那它就得像工具一样有返回值。这个文件就是那个
// 返回值：印记看得见的，是她真写下的那些句子，不是"她用过某个工具"这种元信息。

// gatherPblToolWork 收集这个项目里她已经做出来的东西。
//
// 只收**定下来的**：确认过的改写、挑定的方案、判过的成果、settle 过的决定。
// 半路上的草稿不进——那些还在变，喂给印记只会让它对着一个她自己都还没想好的
// 说法发挥。
func (a *API) gatherPblToolWork(r *http.Request, atomID uuid.UUID) []string {
	ctx := r.Context()
	var out []string
	add := func(s string) {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}

	// 她改写过的问题。这是阶段一的落点，也是整个项目后面所有事的锚。
	if rs, err := a.d.Queries.ListPblReframes(ctx, atomID); err == nil {
		for _, x := range rs {
			if x.ConfirmedAt.Valid {
				add("她把问题定成了：" + x.Who + "需要" + x.Needs + "，因为" + trimBecause(x.Why))
				if strings.TrimSpace(x.Hmw) != "" {
					add("她的「我们可以怎样」：" + x.Hmw)
				}
			}
		}
	}

	// 🚨 板子按坐标摆过，位置就是一句判断。
	//
	// 没开过坐标视图的板子，x/y 是 boardSpot() 派的座位，把它读成「她认为这条
	// 不要紧」是在编造——所以先看这一位（migration 0122）。
	axes := false
	if p, err := a.d.Queries.GetPblProject(ctx, atomID); err == nil {
		axes = p.BoardAxes
	}

	// 便签板上她自己写的东西。原样带上——印记要能指着其中某一张说话，
	// 「你写的那条『中午十二点半剩得最多』」比「你贴了 7 张便签」有用一万倍。
	if ns, err := a.d.Queries.ListPblNotes(ctx, atomID); err == nil {
		byKind := map[string][]string{}
		for _, n := range ns {
			if n.Author != "student" {
				continue
			}
			body := n.Body
			// 🚨 她拍了照片就说一句。印记看不见那张图（模型这一路没有视觉），
			// 但"她带回来一张照片"本身就是要接的话——不说，她辛辛苦苦拍的东西
			// 在对话里等于没发生过。闭环（产品负责人 2026-09-03）。
			if strings.TrimSpace(n.ImageKey) != "" {
				body += "（她还拍了一张照片）"
			}
			// 她摆在哪个角上。轴见 Board.tsx：横 = 有多确定，纵 = 有多要紧。
			if axes {
				if q := boardQuadrant(n.X, n.Y); q != "" {
					body += "（她摆在「" + q + "」那一角）"
				}
			}
			byKind[n.Kind] = append(byKind[n.Kind], body)
		}
		for _, k := range []struct{ kind, label string }{
			{"observation", "她在板上记的实际观察"},
			{"quote", "她记下的别人的原话"},
			{"assumption", "她自己标出来的推论"},
			{"question", "她提出的问题"},
			{"idea", "她想到的点子"},
		} {
			if xs := byKind[k.kind]; len(xs) > 0 {
				add(k.label + "：" + strings.Join(xs, "；"))
			}
		}
	}

	// 她挑定的方案和理由。
	if ds, err := a.d.Queries.ListPblDecisions(ctx, atomID); err == nil {
		for _, d := range ds {
			if !d.SettledAt.Valid {
				continue
			}
			line := "关于「" + d.Subject + "」，她选了「" + d.Choice + "」，因为" + d.Why
			if strings.TrimSpace(d.WhyNot) != "" {
				line += "；没选别的是因为" + d.WhyNot
			}
			// 🚨 她当初写下的翻盘条件。这一句是复盘阶段唯一能回头对照的东西：
			// 「你当时说出现 X 就改主意，现在 X 发生了吗」。不带上，那句话就白写了。
			// 🚨 她把几条路排出来的顺序。只挑一个不需要比较，排成一列才需要——
			// 这个序本身就是她比过的证据。
			if os, oerr := a.d.Queries.ListPblDecisionOptions(ctx, d.ID); oerr == nil {
				ranked := make([]sqlc.PblDecisionOption, 0, len(os))
				for _, o := range os {
					if o.StudentRank > 0 {
						ranked = append(ranked, o)
					}
				}
				if len(ranked) > 1 {
					sort.Slice(ranked, func(i, j int) bool {
						return ranked[i].StudentRank < ranked[j].StudentRank
					})
					labels := make([]string, 0, len(ranked))
					for _, o := range ranked {
						labels = append(labels, strings.TrimSpace(o.Label))
					}
					line += "；她把这几条排成：" + strings.Join(labels, " > ")
				}
			}
			// 🚨 她自己加的那条路。「这些都不对，我要的是另一样」和「在给定的
			// 选项里挑一个」是两件事，后者印记看不出区别，前者是她判断力的证据。
			if os, oerr := a.d.Queries.ListPblDecisionOptions(ctx, d.ID); oerr == nil {
				var mine []string
				for _, o := range os {
					if o.Author == "student" {
						mine = append(mine, strings.TrimSpace(o.Label))
					}
				}
				if len(mine) > 0 {
					line += "；这几条是她自己加进去的：" + strings.Join(mine, "、")
				}
			}
			if f := strings.TrimSpace(d.Flip); f != "" {
				line += "；她说会让她改主意的情况是：" + f
			}
			add(line)
		}
	}

	// 她判过的成果。退回去的那些尤其重要——那是她自己的判断力在起作用。
	if as, err := a.d.Queries.ListPblArtifacts(ctx, atomID); err == nil {
		word := map[string]string{"kept": "通过了", "revise": "要求修改", "dropped": "打回重做"}
		for _, x := range as {
			if x.Verdict == nil {
				continue
			}
			title := strings.TrimSpace(x.Title)
			if title == "" {
				title = "我交的一份东西"
			}
			line := "她对《" + title + "》的判断：" + word[*x.Verdict]
			if strings.TrimSpace(x.Why) != "" {
				line += "，理由是「" + x.Why + "」"
			}
			add(line)
		}
	}

	// 🚨 她审成果时答的那些问题。
	//
	// 产品负责人 2026-09-03 的闭环原则：「we invoke one interactive tool, it must
	// have a finish signal and the finished content have to be sent back to AI to
	// push forward the flow」。
	//
	// 原来这里只收了「通过/打回」和一句理由——而审核这件事真正的产出是**她对
	// 每一处的判断**：印记划出来的那几句她怎么答的、几个方面她怎么看的。少了
	// 它们，她认认真真审了十分钟，印记只知道"她点了通过"。
	if as, err := a.d.Queries.ListPblArtifacts(ctx, atomID); err == nil {
		for _, x := range as {
			title := strings.TrimSpace(x.Title)
			if title == "" {
				title = "我交的一份东西"
			}
			if ms, merr := a.d.Queries.ListPblReviewMarks(ctx, x.ID); merr == nil {
				for _, m := range ms {
					if ans := strings.TrimSpace(m.Answer); ans != "" {
						add("审《" + title + "》时，对「" + strings.TrimSpace(m.Question) +
							"」她答：" + ans)
					}
				}
			}
			if ds, derr := a.d.Queries.ListPblReviewDimensions(ctx, x.ID); derr == nil {
				for _, d := range ds {
					if ans := strings.TrimSpace(d.Answer); ans != "" {
						add("审《" + title + "》时，关于「" + strings.TrimSpace(d.Prompt) +
							"」她答：" + ans)
					}
				}
			}
		}
	}

	// 🚨 她在结构审查里定下来的形状。
	//
	// 这一整棵树以前从来没回到过印记那里：她可以花十分钟把提纲重排一遍，而印记
	// 下一轮完全不知道这个项目现在长什么样。
	if ns, err := a.d.Queries.ListPblTreeNodes(ctx, sqlc.ListPblTreeNodesParams{
		AtomID: atomID, Tree: pblMainTree,
	}); err == nil && len(ns) > 0 {
		var b strings.Builder
		for _, n := range ns {
			b.WriteString("\n  " + strings.Repeat("\u3000", int(n.Depth)) + strings.TrimSpace(n.Title))
			if body := strings.TrimSpace(n.Body); body != "" {
				b.WriteString("：" + body)
			}
		}
		add("她定下来的结构：" + b.String())
	}
	// 她对结构那三个问题的回答。
	if cs, err := a.d.Queries.ListPblTreeChecks(ctx, sqlc.ListPblTreeChecksParams{
		AtomID: atomID, Tree: pblMainTree,
	}); err == nil {
		for _, c := range cs {
			if ans := strings.TrimSpace(c.Answer); ans != "" {
				add("看结构时她对「" + strings.TrimSpace(c.Question) + "」的判断：" + ans)
			}
		}
	}

	// 🚨 某一步的分工，她确认过的那一版。
	if v, err := a.d.Queries.GetPblLivePlan(ctx, atomID); err == nil {
		if ss, serr := a.d.Queries.ListPblSubstepsForPlan(ctx, v.ID); serr == nil && len(ss) > 0 {
			who := map[string]string{"yinji": "印记", "student": "她自己", "both": "两个人一起"}
			var lines []string
			for _, x := range ss {
				// 🚨 她改过的那一版才算数，而且"她改过"本身就是信号（铁律④）：
				// 印记本来派给自己的一件事被她要了过去，那是她的判断在起作用。
				raw, why, moved := x.Owner, strings.TrimSpace(x.Reason), false
				if x.StudentOwner != nil && strings.TrimSpace(*x.StudentOwner) != "" {
					if *x.StudentOwner != x.Owner {
						moved = true
					}
					raw = *x.StudentOwner
					if sr := strings.TrimSpace(x.StudentReason); sr != "" {
						why = sr
					}
				}
				owner := who[raw]
				if owner == "" {
					owner = raw
				}
				line := strings.TrimSpace(x.Title) + "（" + owner
				if moved {
					line += "，她改的"
				}
				// 🚨 她发现方案里少了一件事——审一份方案不等于逐格同意。
				if x.AddedByStudent {
					line += "，她补的"
				}
				line += "）"
				if why != "" {
					line += "，因为" + why
				}
				lines = append(lines, line)
			}
			add("这一步的分工：" + strings.Join(lines, "；"))
		}
	}

	// 🚨 复盘里她写下的答案。整件项目最后的那层意思就在这儿，不回灌等于白写。
	if ps, err := a.d.Queries.ListPblReviewPrompts(ctx, atomID); err == nil {
		// 🚨 「现在还这么想吗」比答案本身更要紧：她说「当时没想清楚」，印记
		// 下一轮就该问那一处到底哪儿没想清楚。
		stance := map[string]string{
			"still":   "她说现在仍这么想",
			"changed": "她说现在会改",
			"unclear": "她说当时没想清楚",
		}
		for _, x := range ps {
			ans := strings.TrimSpace(x.Answer)
			st := stance[strings.TrimSpace(x.Stance)]
			if ans == "" && st == "" {
				continue
			}
			line := "复盘时她对「" + strings.TrimSpace(x.Prompt) + "」"
			if st != "" {
				line += "：" + st
			}
			if ans != "" {
				line += "，她写的是：" + ans
			}
			add(line)
		}
	}

	// 🚨 出门清单：做到了哪几条，**没做到哪几条**。
	//
	// 后半句才是这一段存在的理由。她答应去看三件事、回来只做到一件，这件事今天
	// 在系统里毫无痕迹——而它恰恰是印记下一轮最该接的话（铁律④：跳过也是信号）。
	// 不是拿来责备她的：没做到常常说明那一条本来就不现实，那也值得说出来。
	if ms, err := a.d.Queries.ListPblMissionItemsByAtom(ctx, atomID); err == nil && len(ms) > 0 {
		var missed []string
		for _, m := range ms {
			if !m.DoneAt.Valid {
				missed = append(missed, strings.TrimSpace(m.Prompt))
			}
		}
		if len(missed) > 0 {
			add("出门清单上她没做到的：" + strings.Join(missed, "；"))
		}
	}

	// 上线之后她记下来的事。
	if ks, err := a.d.Queries.ListPblKeepEntries(ctx, atomID); err == nil {
		for _, k := range ks {
			add("上线之后她记下：" + k.Body)
		}
	}

	return out
}

// trimBecause 去掉她答案开头自带的「因为」。
//
// 问的是「为什么这对他重要？」，中文里几乎必然答成「因为…」，模板再补一个
// 就成了「因为 因为课间只有十分钟」——界面上和喂给印记的那句都是这样。
func trimBecause(s string) string {
	t := strings.TrimLeft(strings.TrimSpace(s), "，,、 \t")
	t = strings.TrimPrefix(t, "因为")
	return strings.TrimLeft(t, "，,：: \t")
}

// attachPblToolWork 把上面收集到的东西挂进这一轮的 CoachInput。
func (a *API) attachPblToolWork(r *http.Request, atomID uuid.UUID, in *pbl.CoachInput) {
	in.ToolWork = a.gatherPblToolWork(r, atomID)
	in.ToolsUsed, in.ToolsOffered = a.pblToolState(r, atomID)
}

// pblToolState 列出她已经做完的工具，和已经递过、她还没做的那些。
//
// 🚨 prompt 里的工具目录不带状态，所以印记看不出哪件已经在她桌上了。两种都会
// 出事，而且是同一种出事：
//
//   - 做完的又递一遍——2026-09-02 实测，她做完「观察日记」，下一轮印记又递了
//     「观察日记」。
//   - 递过还没做的又递一遍——2026-09-03 实测，屏幕上并排两张「头脑风暴」，
//     理由还各写各的。
//
// 已经 done 的优先：一件既做过又有新一张挂着的工具，对印记来说"做过了"是更
// 要紧的那条信息。
func (a *API) pblToolState(r *http.Request, atomID uuid.UUID) (used, offered []string) {
	rows, err := a.d.Queries.ListPblTools(r.Context(), atomID)
	if err != nil {
		return nil, nil
	}
	label := func(name string) string {
		if def, ok := pbl.LookupTool(name); ok {
			return def.Label
		}
		return name
	}
	done := map[string]bool{}
	for _, t := range rows {
		if t.Status == "done" && !done[t.Tool] {
			done[t.Tool] = true
			used = append(used, label(t.Tool))
		}
	}
	open := map[string]bool{}
	for _, t := range rows {
		if t.Status == "done" || t.Status == "skipped" || done[t.Tool] || open[t.Tool] {
			continue
		}
		open[t.Tool] = true
		offered = append(offered, label(t.Tool))
	}
	return used, offered
}

// pblToolAlreadyOnHerScreen 说的是：这件工具已经递过、她还没做完吗。
//
// prompt 里说了不要重复递，但 prompt 是请求。这是保证：同一件工具在她屏幕上
// 只会有一张卡。
func (a *API) pblToolAlreadyOnHerScreen(r *http.Request, atomID uuid.UUID, tool string) bool {
	rows, err := a.d.Queries.ListPblTools(r.Context(), atomID)
	if err != nil {
		return false
	}
	for _, t := range rows {
		if t.Tool == tool && t.Status != "done" && t.Status != "skipped" {
			return true
		}
	}
	return false
}

// lastPblToolEvent 描述她刚做完的那件工具——这一轮她没打字，就靠这一句。
//
// 只说工具名和**她自己写下的那句话**。她在工具里产出的完整内容已经由
// gatherPblToolWork 送进去了，这里不重复。
func (a *API) lastPblToolEvent(r *http.Request, atomID uuid.UUID) string {
	rows, err := a.d.Queries.ListPblTools(r.Context(), atomID)
	if err != nil {
		return ""
	}
	var last *sqlc.PblToolInstance
	for i := range rows {
		t := rows[i]
		if t.Status != "done" || !t.ResolvedAt.Valid {
			continue
		}
		if last == nil || t.ResolvedAt.Time.After(last.ResolvedAt.Time) {
			last = &rows[i]
		}
	}
	if last == nil {
		return ""
	}
	label := last.Tool
	if def, ok := pbl.LookupTool(last.Tool); ok {
		label = def.Label
	}
	line := "她做完了「" + label + "」"
	if note := strings.TrimSpace(last.StudentNote); note != "" {
		line += "，她写下的是：" + note
	}
	return line + "。"
}

// boardQuadrant 把便签在坐标板上的位置读成一句话。
//
// 板子是 0–1 的相对坐标（Board.tsx 在坐标视图下按比例存）：
// 横轴左「我确定」→ 右「我在猜」，纵轴上「很要紧」→ 下「关系不大」。
//
// 🚨 「又要紧、又没把握」那一角是这块板真正的产出——那几条正是她接下来该去
// 弄清楚的。所以四个角都要说得出名字，不能只报坐标。
func boardQuadrant(x, y float32) string {
	sure, big := x < 0.5, y < 0.5
	switch {
	case !sure && big:
		return "很要紧，但我在猜"
	case sure && big:
		return "很要紧，而且我确定"
	case !sure && !big:
		return "关系不大，也只是猜的"
	default:
		return "关系不大，但我确定"
	}
}
