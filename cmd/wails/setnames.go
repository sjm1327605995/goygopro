package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"sync"
)

// SetNameTable 存「系列码 → 系列名」映射，来源是 strings.conf 的
// `!setname <hex> <文本>` 行（data_manager.cpp:203-211；tab 之后是注释，
// 一个系列多段名用 | 分隔）。仓库原本没有 strings.conf，卡详情因此
// 一直没有系列名行；数据文件取自 ygopro-database locales/zh-CN。
type SetNameTable struct {
	mu    sync.RWMutex
	names map[uint32]string
}

func NewSetNameTable() *SetNameTable {
	return &SetNameTable{names: make(map[uint32]string)}
}

// Load 解析一个 strings.conf。文件不存在不算错误（返回 nil）——应用在
// 缺系列名数据时照常工作，只是详情面板少一行。
func (t *SetNameTable) Load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	names := make(map[uint32]string)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if !strings.HasPrefix(line, "!setname ") {
			continue
		}
		rest := strings.TrimSpace(line[len("!setname "):])
		sp := strings.IndexByte(rest, ' ')
		if sp <= 0 {
			continue
		}
		code, err := strconv.ParseUint(rest[:sp], 0, 32)
		if err != nil {
			continue
		}
		text := rest[sp+1:]
		// 原版 sscanf 的 %240[^\t\n]：tab 及之后的内容是注释
		if idx := strings.IndexByte(text, '\t'); idx >= 0 {
			text = text[:idx]
		}
		name := strings.TrimSpace(text)
		if name != "" {
			names[uint32(code)] = name
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}

	t.mu.Lock()
	t.names = names
	t.mu.Unlock()
	return nil
}

// Get 精确匹配 16 位系列码（data_manager.cpp:287-288 GetSetName）。
func (t *SetNameTable) Get(code uint32) string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.names[code]
}

// CodesByName 按名称反查系列码：短关键字（<2 字符）token 精确匹配、
// 长关键字 token 子串匹配，token 按 | 拆（data_manager.cpp:289-322
// GetSetCodes）。
func (t *SetNameTable) CodesByName(name string) []uint32 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var out []uint32
	short := len([]rune(name)) < 2
	for code, sn := range t.names {
		for _, token := range strings.Split(sn, "|") {
			if short {
				if token == name {
					out = append(out, code)
					break
				}
			} else if strings.Contains(token, name) {
				out = append(out, code)
				break
			}
		}
	}
	return out
}

// FormatSetNames 把卡的 setcode（4×16 位子码打包，cdb datas.setcode）展开
// 成系列名列表（data_manager.cpp:396-409 FormatSetName 的 | 连接语义）。
func (t *SetNameTable) FormatSetNames(setcode uint64) []string {
	var out []string
	for i := 0; i < 4; i++ {
		sub := uint32((setcode >> (16 * i)) & 0xffff)
		if sub == 0 {
			continue
		}
		if n := t.Get(sub); n != "" {
			out = append(out, n)
		}
	}
	return out
}
