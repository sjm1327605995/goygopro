package client

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

// strings.conf 的读取 —— 界面上一切给人看的文字都从这里来：阶段提示、胜负原因、
// 指示物名、系列名。文件来自原版 ygopro（仓库根目录），格式是每行 `!段名 值 文本`。
//
// 在此之前这些文本只能靠猜。猜出来的东西是错的：victoryReason 里 0x0 被写成
// 「LP 归零」，实际是「投降」；0x1 才是基本分变成 0 —— 26 条胜利原因全部错位一格。
// 玩家看到的败因根本对不上。
//
// 对应 C++ data_manager.cpp 的 LoadStrings / ReadStringConfLine。

// StringTable 是 strings.conf 的四张表。
type StringTable struct {
	mu      sync.RWMutex
	system  map[int]string // 阶段、操作提示（十进制编号）
	victory map[int]string // 胜负原因（十六进制）
	counter map[int]string // 指示物名（十六进制）
	setName map[int]string // 系列名（十六进制）
	loaded  bool
}

// Strings 是全局的文本表。没加载时各 Get 返回占位串，不会崩。
var Strings = &StringTable{
	system:  map[int]string{},
	victory: map[int]string{},
	counter: map[int]string{},
	setName: map[int]string{},
}

// LoadStrings 读入 strings.conf。文件缺失时返回错误，界面退化成显示编号。
func (t *StringTable) LoadStrings(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	t.mu.Lock()
	defer t.mu.Unlock()

	sc := bufio.NewScanner(f)
	// 系列名那一行可能很长，默认 64KB 缓冲不一定够
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		t.readLine(sc.Text())
	}
	t.loaded = true
	return sc.Err()
}

// readLine 解析一行。不以 '!' 开头的都是注释。
func (t *StringTable) readLine(line string) {
	if !strings.HasPrefix(line, "!") {
		return
	}
	rest := line[1:]
	section, rest, ok := strings.Cut(rest, " ")
	if !ok {
		return
	}
	rest = strings.TrimLeft(rest, " ")
	valStr, text, ok := strings.Cut(rest, " ")
	if !ok {
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	switch section {
	case "system":
		// 只有 system 段是十进制，其余是十六进制
		if v, err := strconv.Atoi(valStr); err == nil {
			t.system[v] = text
		}
	case "victory":
		if v, err := strconv.ParseInt(strings.TrimPrefix(valStr, "0x"), 16, 32); err == nil {
			t.victory[int(v)] = text
		}
	case "counter":
		if v, err := strconv.ParseInt(strings.TrimPrefix(valStr, "0x"), 16, 32); err == nil {
			t.counter[int(v)] = text
		}
	case "setname":
		// 系列名后面用制表符跟注释，要截掉（C++ 的 %240[^\t\n]）
		if i := strings.IndexByte(text, '\t'); i >= 0 {
			text = strings.TrimSpace(text[:i])
		}
		if v, err := strconv.ParseInt(strings.TrimPrefix(valStr, "0x"), 16, 32); err == nil {
			t.setName[int(v)] = text
		}
	}
}

// Loaded 表示 strings.conf 是否读进来了。
func (t *StringTable) Loaded() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.loaded
}

// System 取一条系统提示。查不到时回显编号 —— 空字符串会让界面莫名其妙地少一块。
func (t *StringTable) System(code int) string {
	return t.lookup(t.system, code, "系统提示")
}

// Victory 取一条胜负原因。
func (t *StringTable) Victory(code int) string {
	return t.lookup(t.victory, code, "胜利条件")
}

// Counter 取指示物名。
func (t *StringTable) Counter(code int) string {
	return t.lookup(t.counter, code, "指示物")
}

// SetName 取系列名。
func (t *StringTable) SetName(code int) string {
	return t.lookup(t.setName, code, "系列")
}

func (t *StringTable) lookup(m map[int]string, code int, kind string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if s, ok := m[code]; ok {
		return s
	}
	return fmt.Sprintf("%s %d", kind, code)
}
