package main

import "encoding/binary"

// ------------------------------------------------------------------
// CTOS_RESPONSE 编码的权威来源
//
// ocgcore 的响应只有两种形态：
//   - 4 字节 int32 标量（SendResponseI 已覆盖：yes/no、option、position、
//     announce 系、rps、idlecmd/battlecmd 的 (idx<<16)|type 等）
//   - 变长字节串：由玩家先前的选择请求决定解释方式
//
// 下面的 encode* 函数是每种变长响应的唯一编码实现；前端只发语义参数
// （Respond* 方法），字节布局不再出现在 JavaScript 里。
// 字节级断言见 responses_test.go。
// ------------------------------------------------------------------

// encodeSelectCardResponse 编码 MSG_SELECT_CARD/TRIBUTE 的响应：
// count(1) + 选中序号(1) × n。
func encodeSelectCardResponse(indices []int32) []byte {
	out := make([]byte, 1+len(indices))
	out[0] = byte(len(indices))
	for i, v := range indices {
		out[1+i] = byte(v)
	}
	return out
}

// encodeSelectUnselectResponse 编码 MSG_SELECT_UNSELECT_CARD 的响应：
// count(1)=1 + 合并序号(1)（select 列表在前，unselect 列表在后）。
func encodeSelectUnselectResponse(index int32) []byte {
	return []byte{1, byte(index)}
}

// encodeCounterResponse 编码 MSG_SELECT_COUNTER 的响应：
// 每卡一个 uint16 LE（与请求中的卡片顺序一致，合计等于需求量）。
func encodeCounterResponse(counts []int32) []byte {
	out := make([]byte, 2*len(counts))
	for i, v := range counts {
		binary.LittleEndian.PutUint16(out[2*i:], uint16(v))
	}
	return out
}

// encodeSelectSumResponse 编码 MSG_SELECT_SUM 的响应：
// count(1)（must 数 + 选中数）+ 选中序号(1) × n（只含 select 列表序号）。
func encodeSelectSumResponse(count int32, indices []int32) []byte {
	out := make([]byte, 1+len(indices))
	out[0] = byte(count)
	for i, v := range indices {
		out[1+i] = byte(v)
	}
	return out
}

// encodeSelectPlaceResponse 编码 MSG_SELECT_PLACE/DISFIELD 的响应：
// player(1) + location(1) + sequence(1)。
func encodeSelectPlaceResponse(player, loc, seq int32) []byte {
	return []byte{byte(player), byte(loc), byte(seq)}
}

// encodeSortCardResponse 编码 MSG_SORT_CARD 的响应：
// 排列字节 —— 第 i 个字节 = 用户给第 i 张卡指定的顺序位置。
func encodeSortCardResponse(perm []int32) []byte {
	out := make([]byte, len(perm))
	for i, v := range perm {
		out[i] = byte(v)
	}
	return out
}

// encodeSortCardCancelResponse 编码 MSG_SORT_CARD 的取消响应：单个 0xff。
func encodeSortCardCancelResponse() []byte {
	return []byte{0xff}
}

// encodeIdleCmdResponse 编码 MSG_SELECT_IDLECMD 的整型响应：
// (列表内序号 << 16) | 命令类型。类型值即 YGOPro 按钮 ID：
// 0=summon 1=spsummon 2=pos_change 3=mset 4=sset 5=activate
// 6=toBP 7=toEP 8=shuffle（playerop.cpp SelectIdleCmd）。
func encodeIdleCmdResponse(idx, cmdType int32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(idx<<16|cmdType))
	return buf
}

// encodeBattleCmdResponse 编码 MSG_SELECT_BATTLECMD 的整型响应：
// (列表内序号 << 16) | 命令类型。类型值：0=activate 1=attack
// 2=toM2 3=toEP（playerop.cpp select_battle_command：t==0 查 select_chains，
// t==1 查 attackable_cards）。
func encodeBattleCmdResponse(idx, cmdType int32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(idx<<16|cmdType))
	return buf
}

// ------------------------------------------------------------------
// 暴露给前端的语义化响应方法（Wails Call.ByName("App.RespondXxx")）
// ------------------------------------------------------------------

// RespondSelectCard 回答 MSG_SELECT_CARD / MSG_SELECT_TRIBUTE（选中的卡片序号列表）。
func (a *App) RespondSelectCard(indices []int32) {
	_ = a.routeResponseB(encodeSelectCardResponse(indices))
}

// RespondSelectUnselect 回答 MSG_SELECT_UNSELECT_CARD（两列表合并后的 0 基序号）。
func (a *App) RespondSelectUnselect(index int32) {
	_ = a.routeResponseB(encodeSelectUnselectResponse(index))
}

// RespondCounter 回答 MSG_SELECT_COUNTER（每张卡的移除数量）。
func (a *App) RespondCounter(counts []int32) {
	_ = a.routeResponseB(encodeCounterResponse(counts))
}

// RespondSelectSum 回答 MSG_SELECT_SUM（总数 + select 列表中被选序号）。
func (a *App) RespondSelectSum(count int32, indices []int32) {
	_ = a.routeResponseB(encodeSelectSumResponse(count, indices))
}

// RespondSelectPlace 回答 MSG_SELECT_PLACE / MSG_SELECT_DISFIELD。
func (a *App) RespondSelectPlace(player, loc, seq int32) {
	_ = a.routeResponseB(encodeSelectPlaceResponse(player, loc, seq))
}

// RespondSortCard 回答 MSG_SORT_CARD（排列字节）。
func (a *App) RespondSortCard(perm []int32) {
	_ = a.routeResponseB(encodeSortCardResponse(perm))
}

// RespondSortCardCancel 取消 MSG_SORT_CARD。
func (a *App) RespondSortCardCancel() {
	_ = a.routeResponseB(encodeSortCardCancelResponse())
}

// RespondIdleCmd 回答 MSG_SELECT_IDLECMD：卡片/激活列表内序号 + 命令类型
// （0=summon 1=spsummon 2=repos 3=mset 4=sset 5=activate 6=toBP 7=toEP 8=shuffle）。
func (a *App) RespondIdleCmd(idx, cmdType int32) {
	_ = a.routeResponseB(encodeIdleCmdResponse(idx, cmdType))
}

// RespondBattleCmd 回答 MSG_SELECT_BATTLECMD：列表内序号 + 命令类型
// （0=activate 1=attack 2=toM2 3=toEP）。
func (a *App) RespondBattleCmd(idx, cmdType int32) {
	_ = a.routeResponseB(encodeBattleCmdResponse(idx, cmdType))
}
