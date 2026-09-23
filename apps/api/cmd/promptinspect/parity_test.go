package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"mindimprint/api/internal/api"
)

func requestHash(e api.PromptExample) string {
	// Metadata/traces are not wire input. Include model class plus all request
	// options and tools, preserving message/tool order.
	b, err := json.Marshal(struct {
		Class   string
		Request any
	}{e.Class, e.Request})
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

// Baseline requests were reviewed in the 2026-09-22 prompt language audit.
// Before/after evidence is in docs/reviews/2026-09-22-prompt-language. Explicit
// external input is required to deliberately establish a new text baseline.
func TestMainRequestParity(t *testing.T) {
	expected := map[string]string{}
	baseline := os.Getenv("PROMPT_COMPARE_BASELINE")
	if baseline != "" {
		b, err := os.ReadFile(baseline)
		if err != nil {
			t.Fatal(err)
		}
		var old []api.PromptExample
		if err = json.Unmarshal(b, &old); err != nil {
			t.Fatal(err)
		}
		for _, e := range old {
			expected[e.ID] = requestHash(e)
		}
	} else {
		b, err := os.ReadFile("testdata/main_request_hashes.json")
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(b, &expected); err != nil {
			t.Fatal(err)
		}
	}
	if len(expected) == 0 {
		t.Fatal("empty baseline")
	}
	current := map[string]string{}
	for _, e := range examples() {
		current[e.ID] = requestHash(e)
	}
	for id, want := range expected {
		if current[id] != want {
			t.Errorf("request differs from reviewed baseline: %s", id)
		}
	}
	// 🚨 这个循环只走 expected。一条新装配加进 examples() 而没进基线，
	// 上面那段一个字都不会说 —— 它就**不在被比的集合里**。
	//
	// 2026-09-23 之前正是这样：examples() 吐 36 条，基线里 28 条，另外 8 条
	// （含四条 writing/plan/* 装配）从来没被比过。AGENTS.md「提示词怎么写」
	// 第 6 条记的那次事故就是这个形状 —— 基线不等于覆盖。
	//
	// 这一条把「覆盖」本身变成判据：examples() 里每一条都必须在基线里有位置。
	// 新开一条装配就补一条基线，不能只加装配。
	for id := range current {
		if _, ok := expected[id]; !ok {
			t.Errorf("装配 %s 不在基线里 —— 它从来没有被比对过。"+
				"新开一条装配要同时录一条基线（见 README.txt 怎么重录）", id)
		}
	}
	if baseline != "" && !t.Failed() && os.Getenv("PROMPT_RECORD_BASELINE") == "1" {
		if err := os.MkdirAll("testdata", 0755); err != nil {
			t.Fatal(err)
		}
		b, err := json.MarshalIndent(expected, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile("testdata/main_request_hashes.json", append(b, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
