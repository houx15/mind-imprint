package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
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
func (a *API) gatherPblToolWork(r *http.Request, atomID uuid.UUID, courseTitles map[string]string) []string {
	ctx := r.Context()
	var out []string
	add := func(s string) {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}

	// 🚨 主页项目：这一页现在长什么样，还差哪几处。
	//
	// 不给这一段的话，印记是**闭着眼睛**在往页面上摆字：它不知道哪几格已经填了、
	// 哪几格还空着，也不知道自己上一轮摆的那一句根本没上页面。
	//
	// 而它上一轮那句话很可能真的没上去：页面上每一句都必须能在她说过的话里逐字
	// 找到（`pbl.GroundSiteDraft`），对不上的**静默丢掉**——不报错、不提示。印记
	// 一润色就被丢，然后它以为放好了，下一轮接着聊别的。2026-09-04 的真浏览器
	// 走查上，「名字底下那行你是谁」就是这么一直空着的：她原话说的是「我是一个
	// 在读 IB 的高二学生」，印记摆上去的是改写过的说法，逐字对不上。
	//
	// 只在**这个项目就是她的主页项目**时给（site.AtomID == atomID），否则别的
	// 项目的上下文里会莫名其妙多出一段她主页的状态。
	if u, ok := UserFromContext(ctx); ok {
		if row, err := a.ensureSite(r, u.ID); err == nil &&
			row.AtomID.Valid && uuid.UUID(row.AtomID.Bytes) == atomID {
			if content, cerr := a.loadSiteContent(r, u.ID, u.DisplayName, row); cerr == nil {
				if missing := pbl.SiteMissing(content); len(missing) > 0 {
					add("她的主页上还差这几处：" + strings.Join(missing, "、") +
						"。放上去的每一句必须是她**说过的原话**，逐字照抄——" +
						"改写过的句子会被丢掉，页面上不会有任何变化。")
				} else {
					add("她的主页该有的几处都填上了，可以让她看一遍再决定要不要上线。")
				}
			}
		}
	}

	// 主页项目第一关：她留下的那个读者，和她留下的关键词。
	//
	// 这一关的产出是后面每一关的输入（结构对着关键词检查、配色从关键词派生），
	// 所以它必须回到印记那儿——不然第三关它会重新问一遍「你想给谁看」。
	if ps, err := a.d.Queries.ListPblPersonas(ctx, atomID); err == nil {
		for _, p := range ps {
			if !p.Chosen {
				continue
			}
			line := "她定下的读者是：" + p.Label
			if strings.TrimSpace(p.WhyKnows) != "" {
				line += "（" + p.WhyKnows + "）"
			}
			if strings.TrimSpace(p.Wants) != "" {
				line += "；他想看到：" + p.Wants
			}
			if strings.TrimSpace(p.Feeling) != "" {
				line += "；这一页该给他的感觉：" + p.Feeling
			}
			add(line)
			var kws []string
			if len(p.Keywords) > 0 {
				_ = json.Unmarshal(p.Keywords, &kws)
			}
			if len(kws) > 0 {
				add("她留下的关键词：" + strings.Join(kws, "、"))
			}
		}
	}

	// 🚨 主页项目第二关：她自己找到、贴进来的那几个个人网站。
	//
	// 不回灌等于这一关白做：她花时间去搜、去挑、去贴，回到对话，而印记收到的
	// prompt 和上一轮一模一样——它没看见任何一站，只能接着聊上一轮那件事。
	// 闭环（产品负责人 2026-09-03）：有终点信号、但做出来的东西没回到印记那儿，
	// 就不算闭环。
	//
	// 带上她自己补的那一句，并且和印记读出来的那几句分开说——「她说这一站
	// ……」和「这一站的结构是……」是两个人说的话，混在一起，印记就会把自己
	// 读出来的东西当成她的判断复述给她听。
	if refs, err := a.d.Queries.ListPblSiteRefs(ctx, atomID); err == nil {
		for _, s := range refs {
			line := "她贴了一个她喜欢的站：" + s.Title + "（" + s.Url + "）"
			if strings.TrimSpace(s.Structure) != "" {
				line += "；它的结构是：" + s.Structure
			}
			if strings.TrimSpace(s.Best) != "" {
				line += "；最值得学的一处：" + s.Best
			}
			add(line)
			if t := strings.TrimSpace(s.SheSaid); t != "" {
				add("她自己说这一站：" + t)
			}
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
			//
			// 🚨 只说她**自己拖过**的那几张（dragged，migration 0128）。观察日记
			// 带回来的便签，位置是 boardSpot() 按座位号算的；点子默认 (0,0)，
			// 而 (0,0) 在坐标视图里恰好是「我确定 + 很要紧」那一角。不看这一位，
			// 印记就会当着她的面把代码排的座位说成是她的判断——她要么以为自己
			// 做过这个判断，要么发现印记在编。两种都比不说更糟。
			if axes && n.Dragged {
				if q := boardQuadrant(n.X, n.Y); q != "" {
					body += "（她摆在「" + q + "」那一角）"
				}
			}
			// 她挑出来先试的那一条，和为什么先试它。
			//
			// 🚨 不带这一句，印记只能照着列表顺序猜——线上就猜错过：她挑的是第
			// 三条，印记说的是第一条。
			if n.PickedAt.Valid {
				body += "【她挑了这条先试"
				if w := strings.TrimSpace(n.PickWhy); w != "" {
					body += "，因为" + w
				}
				body += "】"
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

	// 🚨 她连出来的关系，尤其是矛盾。
	//
	// 板上的意义不在单张纸上，在两张纸之间。而**矛盾**那几条最要紧：两条都是她
	// 亲眼看到的，却互相打架——真正的问题几乎都是从那儿长出来的。印记读到这一句
	// 才接得上「你这两条对不上，先弄清楚哪一条错了」。
	if ls, err := a.d.Queries.ListPblNoteLinksWithBodies(ctx, atomID); err == nil {
		word := map[string]string{
			"causes": "导致", "contradicts": "和这一条矛盾",
			"same": "和这一条说的是同一件事", "supports": "撑着这一条",
		}
		for _, l := range ls {
			w := word[l.Relation]
			if w == "" {
				continue
			}
			line := "她把「" + strings.TrimSpace(l.FromBody) + "」和「" +
				strings.TrimSpace(l.ToBody) + "」连了起来：" + w
			if l.Relation == "contradicts" {
				line += "（这一对是她自己标出来的矛盾）"
			}
			add(line)
		}
	}

	// 🚨 结构盖没盖全，答案在「放不进去的那几条」里。
	//
	// 「这个分法盖全了吗」以前是个没法回答的问题：她只能盯着提纲想「大概全了吧」。
	// 她把材料一条一条拖进节点之后，剩下的那几条就是没盖到的地方——而那几条是
	// 她亲手收集的，比任何自评都硬。
	if ns, err := a.d.Queries.ListPblNotes(ctx, atomID); err == nil {
		var placed, loose int
		var looseBodies []string
		for _, n := range ns {
			if n.Author != "student" {
				continue
			}
			if n.TreeNodeID.Valid {
				placed++
				continue
			}
			loose++
			if len(looseBodies) < 5 {
				looseBodies = append(looseBodies, strings.TrimSpace(n.Body))
			}
		}
		if placed > 0 && loose > 0 {
			add("她把材料往结构里放，还有 " + strconv.Itoa(loose) +
				" 条放不进去：" + strings.Join(looseBodies, "；"))
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

	// 🚨 她在这个项目里上完的课，和她自己写下的「这一课对我的项目有什么用」。
	//
	// 产品负责人 2026-09-04：「if trigger course inside pbl parts, it should
	// also be end-looped, namely the course interaction and finished results go
	// back to the running project and AI makes responses.」
	//
	// 少了这一段，上课就是项目外面的一件事：她学完四十分钟回来，印记还在问上一
	// 轮那个问题，而她刚补上的那件本事在对话里等于没发生过。见 pbl_course.go。
	out = append(out, a.gatherPblCourseWork(ctx, atomID, courseTitles)...)

	// 上线之后她记下来的事。
	if ks, err := a.d.Queries.ListPblKeepEntries(ctx, atomID); err == nil {
		for _, k := range ks {
			line := "上线之后她记下：" + strings.TrimSpace(k.Body)
			// 🚨 带上变化，不只带数值。一个数字本身不说明任何事——「23」是多
			// 还是少，只有和上一次比才知道。
			if v := numericToFloat(k.Value); v != nil && strings.TrimSpace(k.Metric) != "" {
				line += "（" + strings.TrimSpace(k.Metric) + "：" +
					strconv.FormatFloat(*v, 'f', -1, 64) + strings.TrimSpace(k.Unit)
				if pv := numericToFloat(k.Prev); pv != nil {
					line += "，上一次 " + strconv.FormatFloat(*pv, 'f', -1, 64) +
						strings.TrimSpace(k.Unit)
				} else {
					line += "，首次记录"
				}
				line += "）"
			}
			// 她改一件事时的预期，以及后来兑现没有。没兑现最值钱：那说明她原来
			// 想错了，而印记该接着问的正是那一句。
			if e := strings.TrimSpace(k.Expect); e != "" {
				line += "；她当时预期：" + e
				switch k.Verdict {
				case "met":
					line += "（后来兑现了）"
				case "missed":
					line += "（后来没兑现）"
				}
			}
			add(line)
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
	// 课程库取一次，三处共用（挑课目录、已上过的课名、回灌里那几句）。
	// 🚨 空着 in.Courses 印记就挑不出课，只会编一个 slug 出来被服务端丢掉。
	courses, _ := a.listCoursesForCaller(r.Context())
	titles := courseTitlesBySlug(courses)

	in.ToolWork = a.gatherPblToolWork(r, atomID, titles)
	in.ToolsUsed, in.ToolsOffered = a.pblToolState(r, atomID)
	in.Courses = pblCourseOptions(courses)
	in.CoursesTaken = a.pblCoursesTaken(r.Context(), atomID, titles)
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
