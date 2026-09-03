package api

import (
	"net/http"
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

	// 便签板上她自己写的东西。原样带上——印记要能指着其中某一张说话，
	// 「你写的那条『中午十二点半剩得最多』」比「你贴了 7 张便签」有用一万倍。
	if ns, err := a.d.Queries.ListPblNotes(ctx, atomID); err == nil {
		byKind := map[string][]string{}
		for _, n := range ns {
			if n.Author != "student" {
				continue
			}
			byKind[n.Kind] = append(byKind[n.Kind], n.Body)
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
