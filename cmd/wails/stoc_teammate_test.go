package main

import (
	"testing"

	"github.com/sjm1327605995/goygopro/protocol/network"
)

// TestSTOCTeammateSurrenderEmitted：组队赛队友请求投降（STOC_TEAMMATE_SURRENDER，
// 无包体，tag_duel.go:645 发出）应转发为 stoc:teammate_surrender，供右侧
// 控制组把投降按钮换成 SysString 1355「投降(1/2)」。
func TestSTOCTeammateSurrenderEmitted(t *testing.T) {
	var emitted map[string]interface{}
	c := NewWailsDuelClient(func(eventName string, optionalData ...interface{}) {
		if eventName == "stoc:teammate_surrender" {
			emitted = optionalData[0].(map[string]interface{})
		}
	})

	c.handleSTOCPacket(byte(network.STOC_TEAMMATE_SURRENDER), nil)

	if emitted == nil {
		t.Fatalf("STOC_TEAMMATE_SURRENDER 应 emit stoc:teammate_surrender")
	}
}
