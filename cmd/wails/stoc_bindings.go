package main

import (
	"encoding/binary"
	"log"
	"time"
	"unicode/utf16"

	"github.com/sjm1327605995/goygopro/core/duel"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// stocBinding 描述一个 STOC 网络包如何解析并转发给前端（仿 engineBinding）：
//   - newMsg:   返回新的包结构体实例；nil 表示该包没有包体
//     （handleSTOCPacket 只 emit 空事件）；
//   - event:    要 emit 的前端事件名；
//   - decorate: 可选，把解析出的结构体转成前端事件的 map 并 emit。
//     nil 时直接把结构体 emit 出去（结构体 json tag 即前端事件键名）；
//   - handle:   完全自定义的分支（包体不是定长结构体，或需要读写
//     WailsDuelClient 状态）。非 nil 时优先于 newMsg/decorate。
//
// 前端事件契约（字段名/重命名/派生布尔）是硬约束：frontend/src 按这些
// 键名消费，改动需同步前端。
type stocBinding struct {
	name     string
	newMsg   func() any
	event    string
	decorate func(c *WailsDuelClient, msg any)
	handle   func(c *WailsDuelClient, payload []byte)
}

// decorateTypeChange 派生身份位：高位是宿主标志，低位是座位号。
func decorateTypeChange(c *WailsDuelClient, msg any) {
	m := msg.(*protocol.STOCTypeChange)
	c.emit("stoc:type_change", map[string]interface{}{
		"type":   m.Type,
		"isHost": (m.Type & 0x10) != 0,
		"pos":    m.Type & 0x0f,
	})
}

// decorateHsPlayerEnter 把 UTF-16 定长名字解码成 Go 字符串。
func decorateHsPlayerEnter(c *WailsDuelClient, msg any) {
	m := msg.(*protocol.STOCHsPlayerEnter)
	name := string(utf16.Decode(m.Name[:]))
	c.emit("stoc:player_enter", map[string]interface{}{
		"pos":  m.Pos,
		"name": name,
	})
}

// decorateHsPlayerChange 拆开 status 的高低半字节：pos=高半，state=低半。
func decorateHsPlayerChange(c *WailsDuelClient, msg any) {
	m := msg.(*protocol.STOCHsPlayerChange)
	pos := (m.Status >> 4) & 0x0f
	state := m.Status & 0x0f
	c.emit("stoc:player_change", map[string]interface{}{
		"pos":    pos,
		"status": state,
		"ready":  state == network.PLAYERCHANGE_READY,
	})
}

func decorateHsWatchChange(c *WailsDuelClient, msg any) {
	m := msg.(*protocol.STOCHsWatchChange)
	c.emit("stoc:watch_change", map[string]interface{}{
		"count": m.WatchCount,
	})
}

func decorateHandResult(c *WailsDuelClient, msg any) {
	m := msg.(*protocol.STOCHandResult)
	c.emit("stoc:hand_result", map[string]interface{}{
		"res1": m.Res1,
		"res2": m.Res2,
	})
}

func decorateTimeLimit(c *WailsDuelClient, msg any) {
	m := msg.(*protocol.STOCTimeLimit)
	c.emit("stoc:time_limit", map[string]interface{}{
		"player":   m.Player,
		"leftTime": m.LeftTime,
	})
}

func decorateErrorMsg(c *WailsDuelClient, msg any) {
	m := msg.(*protocol.STOCErrorMsg)
	c.emit("stoc:error_msg", map[string]interface{}{
		"msg":  m.Msg,
		"code": m.Code,
	})
}

// handleChat 布局 = player_type(2) + NUL 结尾的 UTF-16 变长串
// （netserver.cpp:380-385：只写入 player + 原始消息字节，非定长 256），
// restruct 的定长数组表达不了 NUL 结尾，保留手写解码。
func handleChat(c *WailsDuelClient, payload []byte) {
	if len(payload) < 2 {
		return
	}
	player := binary.LittleEndian.Uint16(payload[0:2])
	msgRunes := make([]uint16, (len(payload)-2)/2)
	for i := range msgRunes {
		msgRunes[i] = binary.LittleEndian.Uint16(payload[2+i*2 : 4+i*2])
	}
	msg := string(utf16.Decode(msgRunes))
	c.emit("stoc:chat", map[string]interface{}{
		"player": player,
		"msg":    msg,
	})
}

// handleReplay 缓存决斗结束的完整录像（duelclient.cpp:727-774）：
// header + 压缩数据。gframe 把 Base.StartTime（REPLAY_UNIFORM）或 Seed 当
// time_t 格式化成文件名；这里同样推导建议文件名，落盘时机交给前端
// （auto_save_replay 或确认弹窗）调 SaveReplay。
func handleReplay(c *WailsDuelClient, payload []byte) {
	if len(payload) < 32 {
		return
	}
	c.mu.Lock()
	c.lastReplay = append([]byte(nil), payload...)
	c.mu.Unlock()
	flag := binary.LittleEndian.Uint32(payload[8:12])
	startTime := binary.LittleEndian.Uint32(payload[12:16]) // Seed
	if flag&duel.REPLAY_UNIFORM != 0 {
		startTime = binary.LittleEndian.Uint32(payload[20:24]) // StartTime
	}
	suggested := time.Unix(int64(startTime), 0).Format("2006-01-02 15-04-05")
	c.emit("stoc:replay", map[string]interface{}{
		"name": suggested,
		"size": len(payload),
	})
}

func handleGameMsgPacket(c *WailsDuelClient, payload []byte) {
	if err := c.handleGameMessage(payload); err != nil {
		log.Printf("[WailsDuelClient] game message parse error: %v", err)
	}
}

// stocBindings 是 STOC 包类型 → 解析/转发行为的总表。
// 表外的 STOC 包由 handleSTOCPacket 记日志后丢弃。
var stocBindings = map[byte]stocBinding{
	// ---- 结构体直接 emit（json tag 即前端键名）----
	network.STOC_JOIN_GAME: {name: "JOIN_GAME", newMsg: func() any { return &protocol.STOCJoinGame{} }, event: "stoc:join_game"},
	// DECK_COUNT payload = int16_t[6]，且服务器已按接收方玩家交换过前后半
	// （single_duel.cpp:340-345），直接按相对序透传。
	network.STOC_DECK_COUNT: {name: "DECK_COUNT", newMsg: func() any { return &protocol.STOCDeckCount{} }, event: "stoc:deck_count"},

	// ---- 需要派生字段/重命名的包 ----
	network.STOC_TYPE_CHANGE:      {name: "TYPE_CHANGE", newMsg: func() any { return &protocol.STOCTypeChange{} }, event: "stoc:type_change", decorate: decorateTypeChange},
	network.STOC_HS_PLAYER_ENTER:  {name: "HS_PLAYER_ENTER", newMsg: func() any { return &protocol.STOCHsPlayerEnter{} }, event: "stoc:player_enter", decorate: decorateHsPlayerEnter},
	network.STOC_HS_PLAYER_CHANGE: {name: "HS_PLAYER_CHANGE", newMsg: func() any { return &protocol.STOCHsPlayerChange{} }, event: "stoc:player_change", decorate: decorateHsPlayerChange},
	network.STOC_HS_WATCH_CHANGE:  {name: "HS_WATCH_CHANGE", newMsg: func() any { return &protocol.STOCHsWatchChange{} }, event: "stoc:watch_change", decorate: decorateHsWatchChange},
	network.STOC_HAND_RESULT:      {name: "HAND_RESULT", newMsg: func() any { return &protocol.STOCHandResult{} }, event: "stoc:hand_result", decorate: decorateHandResult},
	network.STOC_TIME_LIMIT:       {name: "TIME_LIMIT", newMsg: func() any { return &protocol.STOCTimeLimit{} }, event: "stoc:time_limit", decorate: decorateTimeLimit},
	network.STOC_ERROR_MSG:        {name: "ERROR_MSG", newMsg: func() any { return &protocol.STOCErrorMsg{} }, event: "stoc:error_msg", decorate: decorateErrorMsg},

	// ---- 完全自定义（变长包体或需要读写客户端状态）----
	network.STOC_CHAT:     {name: "CHAT", event: "stoc:chat", handle: handleChat},
	network.STOC_REPLAY:   {name: "REPLAY", event: "stoc:replay", handle: handleReplay},
	network.STOC_GAME_MSG: {name: "GAME_MSG", event: "", handle: handleGameMsgPacket},

	// ---- 无包体事件 ----
	network.STOC_DUEL_START:   {name: "DUEL_START", event: "stoc:duel_start"},
	network.STOC_SELECT_HAND:  {name: "SELECT_HAND", event: "stoc:select_hand"},
	network.STOC_SELECT_TP:    {name: "SELECT_TP", event: "stoc:select_tp"},
	network.STOC_DUEL_END:     {name: "DUEL_END", event: "stoc:duel_end"},
	network.STOC_CHANGE_SIDE:  {name: "CHANGE_SIDE", event: "stoc:change_side"},
	network.STOC_WAITING_SIDE: {name: "WAITING_SIDE", event: "stoc:waiting_side"},
	// 组队赛队友请求投降（duelclient.cpp:947-952）：无包体；原版把
	// btnLeaveGame 文案换成 SysString 1355「投降(1/2)」，这里交给
	// 右侧控制组同语义处理。
	network.STOC_TEAMMATE_SURRENDER: {name: "TEAMMATE_SURRENDER", event: "stoc:teammate_surrender"},
}
