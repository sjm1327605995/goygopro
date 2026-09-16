package main

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// 事件名对齐测试：前端 net/events_forward.ts（桥层转发名单）必须覆盖 Go 侧
// 全部 emit 点（engineBindings 表的 event 字段 + decorate 函数和
// duel_client.go 里的字面量 emit）。此前 duel:update_data / duel:update_card
// 已在桥层被静默丢弃过一次；此测试防复发。
//
// 方向：Go emit 的每个事件名都必须出现在前端转发名单里。前端名单可以包含
// Go 暂不 emit 的名字（如尚未接线的 stoc 事件），不算失败。
// 扫描范围：engineBindings / stocBindings 两张表的 event 字段 + 各 decorate
// 与 handle 闭包里的 c.emit 字面量（duel_client.go 里剩余的字面量 emit）。

// 注意：event: 分支后面不能再跟 \s*"——那样永远匹配不上（只有 c.emit(
// 分支生效），engineBindings 表里 event 字段声明的事件全部漏网。
var goEmitNameRe = regexp.MustCompile(`(?:event:\s*|c\.emit\()\s*"(duel:[a-z_]+|stoc:[a-z_]+)"`)
var tsLiteralRe = regexp.MustCompile(`'(duel:[a-z_]+|stoc:[a-z_]+)'`)

func collectGoEmits(t *testing.T, files ...string) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for _, m := range goEmitNameRe.FindAllStringSubmatch(string(data), -1) {
			names[m[1]] = true
		}
	}
	return names
}

func TestFrontendForwardsAllEmittedEvents(t *testing.T) {
	goEmits := collectGoEmits(t,
		"engine_bindings.go",
		"stoc_bindings.go",
		"duel_client.go",
		"app.go",
	)

	tsData, err := os.ReadFile("../../frontend/src/net/events_forward.ts")
	if err != nil {
		t.Fatalf("read events_forward.ts: %v", err)
	}
	forwarded := map[string]bool{}
	for _, m := range tsLiteralRe.FindAllStringSubmatch(string(tsData), -1) {
		forwarded[m[1]] = true
	}

	var missing []string
	for name := range goEmits {
		if !forwarded[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("Go emits events the bridge does not forward (add them to net/events_forward.ts): %s",
			strings.Join(missing, ", "))
	}
}

func TestForwardedListHasNoDuplicates(t *testing.T) {
	tsData, err := os.ReadFile("../../frontend/src/net/events_forward.ts")
	if err != nil {
		t.Fatalf("read events_forward.ts: %v", err)
	}
	seen := map[string]bool{}
	for _, m := range tsLiteralRe.FindAllStringSubmatch(string(tsData), -1) {
		if seen[m[1]] {
			t.Errorf("duplicate entry in events_forward.ts: %s", m[1])
		}
		seen[m[1]] = true
	}
}
