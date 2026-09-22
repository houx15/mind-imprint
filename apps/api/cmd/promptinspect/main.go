// promptinspect is an offline developer tool. It renders synthetic contexts
// through production builders. It never resolves model credentials or calls an
// API/database. Inspecting a prompt does not execute its instructions or tools.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/api"
	"mindimprint/api/internal/awakening"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/interest"
	"mindimprint/api/internal/liteworkspace"
	"mindimprint/api/internal/news"
	"mindimprint/api/internal/prompts"
)

func examples() []api.PromptExample {
	out := api.PromptAssemblyExamples()
	for _, c := range api.BenchCases() {
		out = append(out, api.PromptExample{ID: "bench/" + c.ID, Class: c.Class, Request: c.Request})
	}
	for _, c := range agent.BenchCases() {
		if c.ID == "dialogue/lite-writing-coach" || c.ID == "compose/reading-router" {
			out = append(out, api.PromptExample{ID: "bench/" + c.ID, Class: c.Class, Request: c.Request})
		}
	}
	add := func(id, class, system, user string, tools []gateway.ChatTool) {
		out = append(out, api.PromptExample{ID: id, Class: class, Request: gateway.ChatRequest{Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: system}, {Role: gateway.RoleUser, Content: user}}, Tools: tools}})
	}
	answers := []string{"我喜欢观察校园里的鸟。昨天看到一只鸟衔树枝。", "我想了解鸟怎样选择筑巢的地方。"}
	for _, mode := range []string{"opening", "retry", "last"} {
		in := awakening.DialogueInput{Guide: awakening.Guides[0], Latest: answers[0], Retry: mode == "retry", Last: mode == "last"}
		if mode == "last" {
			in.NodeIndex = 7
		}
		if mode == "retry" {
			in.NodeIndex = 4
		}
		s, u := awakening.BuildDialoguePrompt(in)
		add("awakening/"+mode, gateway.ClassDialogue, s, u, nil)
	}
	s, u := awakening.BuildSelectionPrompt(answers, "鸟怎样选择筑巢的地方？")
	add("awakening/selection", gateway.ClassCompose, s, u, nil)
	s, u = awakening.BuildReportPrompt(answers)
	add("awakening/report", gateway.ClassCompose, s, u, nil)
	s, u = awakening.BuildTitlePrompt(answers)
	add("awakening/title", gateway.ClassReflex, s, u, nil)
	s, u = interest.BuildHarvestPrompt("reading", "校园观鸟", strings.Join(answers, "\n"))
	add("interest/harvest", gateway.ClassDigest, s, u, nil)
	s, u = interest.BuildDigPrompt("鸟类", "校园观察", answers, nil)
	add("interest/dig", gateway.ClassCompose, s, u, nil)
	c := liteworkspace.SystemContext{ClassName: "示例班级", TodayBeijing: "2026-09-22", StudentCount: 2}
	add("teacher/assignment", gateway.ClassDialogue, liteworkspace.AssignmentSystem(c), "布置一份中文阅读作业。", liteworkspace.AssignmentTools())
	add("teacher/home", gateway.ClassDialogue, liteworkspace.HomeSystem(c), "哪些学生本周还没有写作？", liteworkspace.HomeTools())
	rc := liteworkspace.ReportSystemContext{TodayBeijing: c.TodayBeijing, ClassName: c.ClassName, StudentName: "示例学生", Pronoun: "她", Sections: "「学习概况」", Canvas: "本周完成一次阅读。"}
	add("teacher/report", gateway.ClassDialogue, liteworkspace.ReportSystem(rc), "请根据现有事实简化概况。", liteworkspace.ReportTools())
	add("course/ask", gateway.ClassDialogue, agent.BuildCourseAskPrompt("来源核查", "比较证据", "区分信息与证据", "核对来源与发布时间。"), "什么是原始来源？", nil)
	item := news.Item{Title: "Birds choose nesting sites", Body: "Researchers observed birds choosing sheltered nesting sites."}
	s, u, _ = news.BuildSelectPrompt([]news.Item{item})
	add("news/select", gateway.ClassDigest, s, u, nil)
	s, u = news.BuildWritePrompt(item, item.Body)
	add("news/write", gateway.ClassDigest, s, u, nil)
	return out
}

func run(args []string, w io.Writer) error {
	f := flag.NewFlagSet("promptinspect", flag.ContinueOnError)
	f.SetOutput(w)
	list := f.Bool("list", false, "list feature assemblies and source paths")
	templates := f.Bool("templates", false, "list static template IDs, source and consumer paths")
	template := f.String("template", "", "print one static definition including its text")
	cases := f.Bool("cases", false, "list available synthetic assembled examples")
	id := f.String("case", "", "print one assembled example (messages, tools and available section traces)")
	all := f.Bool("all", false, "print all assembled examples")
	if err := f.Parse(args); err != nil {
		return err
	}
	modes := 0
	for _, on := range []bool{*list, *templates, *template != "", *cases, *id != "", *all} {
		if on {
			modes++
		}
	}
	if modes != 1 || f.NArg() != 0 {
		return fmt.Errorf("choose exactly one of -list, -templates, -template ID, -cases, -case ID, -all")
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if *list {
		return enc.Encode(prompts.Assemblies())
	}
	if *templates {
		defs := prompts.Catalog()
		for i := range defs {
			defs[i].Text = ""
		}
		return enc.Encode(defs)
	}
	if *template != "" {
		for _, d := range prompts.Catalog() {
			if d.ID == *template {
				return enc.Encode(d)
			}
		}
		return fmt.Errorf("unknown template %q", *template)
	}
	ex := examples()
	if *all {
		return enc.Encode(ex)
	}
	if *cases {
		type label struct{ ID, Class string }
		ls := []label{}
		for _, e := range ex {
			ls = append(ls, label{e.ID, e.Class})
		}
		return enc.Encode(ls)
	}
	for _, e := range ex {
		if e.ID == *id {
			return enc.Encode(e)
		}
	}
	return fmt.Errorf("unknown case %q", *id)
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
