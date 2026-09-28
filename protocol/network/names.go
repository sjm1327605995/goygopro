package network

import "fmt"

// CTOS/STOC 包类型名表，仅供诊断日志（core/utils.NetLogf）与调试输出使用，
// 不参与协议编解码。

var ctosNames = map[uint8]string{
	CTOS_RESPONSE:         "RESPONSE",
	CTOS_UPDATE_DECK:      "UPDATE_DECK",
	CTOS_HAND_RESULT:      "HAND_RESULT",
	CTOS_TP_RESULT:        "TP_RESULT",
	CTOS_PLAYER_INFO:      "PLAYER_INFO",
	CTOS_CREATE_GAME:      "CREATE_GAME",
	CTOS_JOIN_GAME:        "JOIN_GAME",
	CTOS_LEAVE_GAME:       "LEAVE_GAME",
	CTOS_SURRENDER:        "SURRENDER",
	CTOS_TIME_CONFIRM:     "TIME_CONFIRM",
	CTOS_CHAT:             "CHAT",
	CTOS_EXTERNAL_ADDRESS: "EXTERNAL_ADDRESS",
	CTOS_HS_TODUELIST:     "HS_TODUELIST",
	CTOS_HS_TOOBSERVER:    "HS_TOOBSERVER",
	CTOS_HS_READY:         "HS_READY",
	CTOS_HS_NOTREADY:      "HS_NOTREADY",
	CTOS_HS_KICK:          "HS_KICK",
	CTOS_HS_START:         "HS_START",
	CTOS_REQUEST_FIELD:    "REQUEST_FIELD",
}

var stocNames = map[uint8]string{
	STOC_GAME_MSG:           "GAME_MSG",
	STOC_ERROR_MSG:          "ERROR_MSG",
	STOC_SELECT_HAND:        "SELECT_HAND",
	STOC_SELECT_TP:          "SELECT_TP",
	STOC_HAND_RESULT:        "HAND_RESULT",
	STOC_TP_RESULT:          "TP_RESULT",
	STOC_CHANGE_SIDE:        "CHANGE_SIDE",
	STOC_WAITING_SIDE:       "WAITING_SIDE",
	STOC_DECK_COUNT:         "DECK_COUNT",
	STOC_CREATE_GAME:        "CREATE_GAME",
	STOC_JOIN_GAME:          "JOIN_GAME",
	STOC_TYPE_CHANGE:        "TYPE_CHANGE",
	STOC_LEAVE_GAME:         "LEAVE_GAME",
	STOC_DUEL_START:         "DUEL_START",
	STOC_DUEL_END:           "DUEL_END",
	STOC_REPLAY:             "REPLAY",
	STOC_TIME_LIMIT:         "TIME_LIMIT",
	STOC_CHAT:               "CHAT",
	STOC_HS_PLAYER_ENTER:    "HS_PLAYER_ENTER",
	STOC_HS_PLAYER_CHANGE:   "HS_PLAYER_CHANGE",
	STOC_HS_WATCH_CHANGE:    "HS_WATCH_CHANGE",
	STOC_TEAMMATE_SURRENDER: "TEAMMATE_SURRENDER",
	STOC_FIELD_FINISH:       "FIELD_FINISH",
}

// CTOSName 返回 CTOS 包类型的可读名，未知类型返回 0xXX。
func CTOSName(t uint8) string {
	if n, ok := ctosNames[t]; ok {
		return n
	}
	return fmt.Sprintf("0x%02x", t)
}

// STOCName 返回 STOC 包类型的可读名，未知类型返回 0xXX。
func STOCName(t uint8) string {
	if n, ok := stocNames[t]; ok {
		return n
	}
	return fmt.Sprintf("0x%02x", t)
}
