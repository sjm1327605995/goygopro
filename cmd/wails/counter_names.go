package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// GetCounterName 返回指示物名（strings.conf 的 !counter 行，与原版
// DataManager::GetCounterName 同源）。首次调用时把整个文件解析进
// CardDBManager 的惰性缓存——指示物名随 ADD_COUNTER/REMOVE_COUNTER 事件
// 高频请求，不能每次调用都重新打开并线性扫描文件。未收录的 id 返回「？？？」。
func (a *App) GetCounterName(counter uint32) string {
	if a.cardDB != nil {
		if name, ok := a.cardDB.CounterName(counter); ok {
			return name
		}
	}
	return "？？？"
}

// CounterName 从缓存取指示物名；缓存为空（首次调用）时解析 strings.conf，
// 缺文件按空表处理（调用方兜底「？？？」）。第二个返回值报告是否命中。
func (m *CardDBManager) CounterName(counter uint32) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.counters == nil {
		m.counters = loadCounterNames("strings.conf")
	}
	name, ok := m.counters[counter]
	return name, ok
}

// loadCounterNames 解析 strings.conf 的全部 !counter 行；文件缺失或读失败
// 返回空表。
func loadCounterNames(path string) map[uint32]string {
	out := map[uint32]string{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != "!counter" {
			continue
		}
		id, err := strconv.ParseUint(fields[1], 0, 32)
		if err != nil {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, "!counter"))
		name := strings.TrimSpace(strings.TrimPrefix(rest, fields[1]))
		if name != "" {
			out[uint32(id)] = name
		}
	}
	return out
}
