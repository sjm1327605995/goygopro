package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// GetCounterName reads the same !counter entries as DataManager::GetCounterName.
func (a *App) GetCounterName(counter uint32) string {
	f, err := os.Open("strings.conf")
	if err != nil {
		return "？？？"
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
		if err == nil && uint32(id) == counter {
			rest := strings.TrimSpace(strings.TrimPrefix(line, "!counter"))
			return strings.TrimSpace(strings.TrimPrefix(rest, fields[1]))
		}
	}
	return "？？？"
}
