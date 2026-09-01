// Package pbl is the 项目 room's server side: classifying what she brought
// now, the step loop and artifacts in later slices. It calls the gateway; it
// never holds a key and never reaches a provider directly.
package pbl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"mindimprint/api/internal/gateway"
)

// ProjectKinds is the closed set, mirroring pbl_project's `kind` CHECK
// constraint (migration 0108). Adding one is a migration AND this list; the
// two drifting apart shows up as a 500 at insert time, so keep them together.
var ProjectKinds = []string{"website", "research", "design", "making", "investigation"}

func IsProjectKind(s string) bool {
	for _, k := range ProjectKinds {
		if k == s {
			return true
		}
	}
	return false
}

const classifySystem = `你要判断一个中学生描述的项目属于哪一类。

只返回一个 JSON 对象：{"kind": "..."}

kind 只能是下面五个之一：
- website：做一个网站、主页、展示页
- research：想弄明白一个问题，需要查资料、读文献、分析
- design：做一个设计、方案、作品、活动策划
- making：动手做出一个实物或者一个能用的东西
- investigation：到真实世界里去看、去问、去记录（走访、观察、问卷）

只回 JSON，不要解释，不要代码块以外的话。`

var errNoKind = errors.New("pbl: no usable kind in model output")

// parseKind pulls {"kind": "..."} out of the model's text. It accepts a fenced
// block or prose around the object, because models add both.
//
// It REFUSES anything outside the closed set. A kind the database would reject
// has to fail here, loudly, rather than travel on and surface as a CHECK
// constraint violation from inside a transaction.
func parseKind(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", errNoKind
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return "", errNoKind
	}
	var out struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal([]byte(s[start:end+1]), &out); err != nil {
		return "", fmt.Errorf("pbl: %w", err)
	}
	k := strings.TrimSpace(strings.ToLower(out.Kind))
	if !IsProjectKind(k) {
		return "", fmt.Errorf("pbl: kind %q is not one of %v", k, ProjectKinds)
	}
	return k, nil
}

const maxClassifyAttempts = 2

// DetectKind classifies the idea she typed into the big box.
//
// Usage is returned even on failure so the caller meters: a call that yielded
// nothing still cost money, and an unmetered failure is a hole in the org
// cost aggregate.
//
// It returns an error rather than a default when the output cannot be used.
// Falling back to "research" would hang a wrong, invisible label on her
// project and hide a broken classifier behind a plausible answer — the caller
// decides what the student sees.
func DetectKind(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, idea string) (string, gateway.ChatUsage, error) {
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: classifySystem},
			{Role: gateway.RoleUser, Content: strings.TrimSpace(idea)},
		},
		MaxTokens: 200,
	}

	var lastUsage gateway.ChatUsage
	var lastErr error
	for attempt := 0; attempt < maxClassifyAttempts; attempt++ {
		res, err := gateway.Collect(ctx, prov, resolved, req)
		lastUsage = res.Usage
		if err != nil {
			lastErr = err
			continue
		}
		kind, perr := parseKind(res.Text)
		if perr != nil {
			lastErr = perr
			continue
		}
		return kind, lastUsage, nil
	}
	return "", lastUsage, lastErr
}

// ResolveKind applies spec §4 over whatever the classifier answered.
//
// A student with no projects yet gets the website project, whatever she wrote
// in the box. The website is the one artifact we host, it is where her
// readings, writings and later projects land, and it is a real build with a
// real audience at the end — so it is what the first project IS, rather than
// one option among several.
//
// existingProjects stands in for "does she have a site" until pbl_site exists
// in S5. When it lands, change what feeds this function, not the rule.
func ResolveKind(existingProjects int64, detected string) string {
	if existingProjects == 0 {
		return "website"
	}
	return detected
}
