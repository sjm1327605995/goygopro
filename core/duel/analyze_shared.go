package duel

import (
	"encoding/binary"
	"log"

	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// 本文件把 SingleDuel.Analyze / TagDuel.Analyze 的 MSG_* switch 表驱动化：
// 逐消息的字段跳过以 engineMsgLayouts（engine_msg_layout.go）为唯一事实源，
// 玩家路由差异通过 duelRoom 钩子吸收（currentPlayer / handMaskedRecipients /
// zoneFullPair / zoneMaskedPair / refresh* 系列），模式特有的消息
// （MSG_NEW_TURN、MSG_TAG_SWAP、MSG_MATCH_KILL 等）留在各模式的表里。
//
// 每个 handler 的返回值与原版 switch 的 case 语义一致：0 继续处理批次，
// 1 表示批次以交互提示结束（等待响应），2 表示游戏结束（随后走 DuelEndProc）。

type analyzeHandler func(m duelRoom, engType uint8, pbuf, offset *utils.YGOBuffer) int

// runAnalyze 是 Analyze 的公共驱动循环。
func runAnalyze(m duelRoom, table map[uint8]analyzeHandler, msgBuffer []byte) int {
	pbuf := utils.NewYGOBuffer(msgBuffer, binary.LittleEndian)

	var offset *utils.YGOBuffer
	for pbuf.Len() > 0 {
		// 记录当前消息的起始位置，用于后续提取完整消息数据
		offset = pbuf.Clone()

		var engType uint8
		if err := pbuf.Read(&engType); err != nil {
			panic(err)
		}

		h, ok := table[engType]
		if !ok {
			// 与原版 switch 无匹配 case 一致：仅消费类型字节后继续。
			continue
		}
		// 解析越界（截断/损坏的引擎消息批次）：handler 内被吞掉的读取失败
		// 会留在这里被一次性拦下——记日志（消息类型、越界偏移、批次长度）
		// 并终止本局（EndDuel 收尾回放/引擎，返回 2 走既有 DuelEndProc 通道），
		// 避免基于错位缓冲区继续广播损坏数据。
		if pbuf.Overflowed() {
			log.Printf("[duel] analyze: corrupt engine message 0x%02x (offset %d, batch %d bytes): buffer overflow, duel aborted", engType, pbuf.Offset(), len(msgBuffer))
			m.EndDuel()
			return 2
		}
		if r := h(m, engType, pbuf, offset); r != 0 {
			return r
		}
		if pbuf.Overflowed() {
			log.Printf("[duel] analyze: corrupt engine message 0x%02x (offset %d, batch %d bytes): buffer overflow, duel aborted", engType, pbuf.Offset(), len(msgBuffer))
			m.EndDuel()
			return 2
		}
	}
	return 0
}

// analyzeSelectWait 是 select 系消息的公共收尾：等待玩家响应并把整条消息
// （offset.SubSlices(pbuf)）发给当前操作者（single: players[player]；
// tag: curPlayer[player]）。
func analyzeSelectWait(m duelRoom, pbuf, offset *utils.YGOBuffer, player uint8) int {
	waitForResponse(m, player)
	m.BaseMode().SendPacketDataToPlayer(m.currentPlayer(int(player)), network.STOC_GAME_MSG, offset.SubSlices(pbuf))
	return 1
}

// skipLayout 按布局表跳过消息字段（与 handler 里逐字段 Read/Next 等价，
// 消费的字节数以 engineMsgLayouts 为准）。
func skipLayout(pbuf *utils.YGOBuffer, steps []layStep) {
	for _, s := range steps {
		switch s.kind {
		case layFixed:
			pbuf.Next(s.n)
		case layCountList:
			var count uint8
			_ = pbuf.Read(&count)
			pbuf.Next(int(count) * s.n)
		case layToss:
			var player, count uint8
			_ = pbuf.Read(&player, &count)
			pbuf.Next(int(count))
		case layChain:
			var player, count uint8
			_ = pbuf.Read(&player, &count)
			pbuf.Next(9 + int(count)*14)
		default:
			// 其余布局仅出现于 Analyze 不会收到的消息（UPDATE_DATA 等），
			// 与原版 switch 未覆盖这些消息的行为一致：不消费。
		}
	}
}

// layoutOf 取消息的布局；Analyze 表中的每条消息都必须有布局登记
// （TestAnalyzeLayoutMatchesBatchWalker 强制）。
func layoutOf(engType uint8) engineMsgLayout {
	return engineMsgLayouts[engType]
}

// analyzeSkipBroadcast 返回「跳过布局字段 → 整段广播给所有玩家与观察者」的 handler。
func analyzeSkipBroadcast(engType uint8) analyzeHandler {
	steps := layoutOf(engType).steps
	return func(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
		skipLayout(pbuf, steps)
		broadcastData(m, network.STOC_GAME_MSG, offset.SubSlices(pbuf))
		return 0
	}
}

// analyzeSkipBroadcastRefresh 在广播后追加一次模式特有的区域刷新。
func analyzeSkipBroadcastRefresh(engType uint8, refresh func(duelRoom)) analyzeHandler {
	steps := layoutOf(engType).steps
	return func(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
		skipLayout(pbuf, steps)
		broadcastData(m, network.STOC_GAME_MSG, offset.SubSlices(pbuf))
		refresh(m)
		return 0
	}
}

// analyzeBroadcastRefresh 是无字段跳过的广播+刷新（MSG_SUMMONED 等无体消息）。
func analyzeBroadcastRefresh(refresh func(duelRoom)) analyzeHandler {
	return func(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
		broadcastData(m, network.STOC_GAME_MSG, offset.SubSlices(pbuf))
		refresh(m)
		return 0
	}
}

// ----- 共享 handler：路由复杂或含隐藏规则的消息 -----

// analyzeRetry 重试消息：让上次响应的玩家重新选择。
func analyzeRetry(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	waitForResponse(m, m.BaseMode().lastResponse)
	m.BaseMode().SendPacketDataToPlayer(m.currentPlayer(int(m.BaseMode().lastResponse)), network.STOC_GAME_MSG, offset.SubSlices(pbuf))
	return 1
}

// analyzeHint 提示消息：按提示类型路由（1,2,3,5 只发指定玩家；4,6,7,8,9,11 发
// 其余玩家（single 为对手，tag 为除当前操作者外的所有玩家）+观察者；10 全员广播）。
func analyzeHint(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	base := m.BaseMode()
	var (
		typ    uint8
		player uint8
		data   int32
	)
	_ = pbuf.Read(&typ, &player, &data)
	switch typ {
	case 1, 2, 3, 5:
		base.SendPacketDataToPlayer(m.currentPlayer(int(player)), network.STOC_GAME_MSG, offset.SubSlices(pbuf))
	case 4, 6, 7, 8, 9, 11:
		for _, p := range m.waitingNotifyRecipients(player) {
			base.SendPacketDataToPlayer(p, network.STOC_GAME_MSG, offset.SubSlices(pbuf))
		}
		for _, v := range base.Observers {
			base.ReSendToPlayer(v)
		}
	case 10:
		broadcastData(m, network.STOC_GAME_MSG, offset.SubSlices(pbuf))
	}
	return 0
}

// analyzeWin 胜利消息：广播结果、记账（single 的 match 逻辑）、结束决斗。
func analyzeWin(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	var (
		player uint8
		typ    uint8
	)
	_ = pbuf.Read(&player, &typ)
	broadcastData(m, network.STOC_GAME_MSG, offset.SubSlices(pbuf))
	m.onEngineWin(player)
	m.EndDuel()
	return 2
}

// ----- 共享 handler：select 系（读字段 → 等待响应 → 发给当前操作者）-----

func analyzeSelectBattlecmd(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	var (
		player uint8
		count  uint8
	)
	_ = pbuf.Read(&player, &count)
	pbuf.Next(int(count) * 11)
	_ = pbuf.Read(&count)
	pbuf.Next(int(count)*8 + 2)
	return analyzeSelectWait(m, pbuf, offset, player)
}

func analyzeSelectIdlecmd(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	var (
		player uint8
		count  uint8
	)
	_ = pbuf.Read(&player, &count)
	pbuf.Next(int(count * 7))
	_ = pbuf.Read(&count)
	pbuf.Next(int(count * 7))
	_ = pbuf.Read(&count)
	pbuf.Next(int(count * 7))
	_ = pbuf.Read(&count)
	pbuf.Next(int(count * 7))
	_ = pbuf.Read(&count)
	pbuf.Next(int(count * 7))
	_ = pbuf.Read(&count)
	pbuf.Next(int(count*11 + 3))
	return analyzeSelectWait(m, pbuf, offset, player)
}

func analyzeSelectEffectyn(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	var player uint8
	_ = pbuf.Read(&player)
	pbuf.Next(12)
	return analyzeSelectWait(m, pbuf, offset, player)
}

func analyzeSelectYesno(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	var player uint8
	_ = pbuf.Read(&player)
	pbuf.Next(4)
	return analyzeSelectWait(m, pbuf, offset, player)
}

func analyzeSelectOption(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	var (
		player uint8
		count  uint8
	)
	_ = pbuf.Read(&player, &count)
	pbuf.Next(int(count * 4))
	return analyzeSelectWait(m, pbuf, offset, player)
}

func analyzeSelectCard(m duelRoom, engType uint8, pbuf *utils.YGOBuffer) int {
	var msg protocol.SelectCardMsg
	if err := pbuf.Unpack(&msg); err != nil {
		panic(err)
	}
	msg.HideCodesForPlayer(msg.Player)
	data := append([]byte{engType}, utils.PackGameMsg(&msg)...)
	waitForResponse(m, msg.Player)
	m.BaseMode().SendPacketDataToPlayer(m.currentPlayer(int(msg.Player)), network.STOC_GAME_MSG, data)
	return 1
}

func analyzeSelectUnselectCard(m duelRoom, engType uint8, pbuf *utils.YGOBuffer) int {
	var msg protocol.SelectUnselectCardMsg
	if err := pbuf.Unpack(&msg); err != nil {
		panic(err)
	}
	msg.HideCodesForPlayer(msg.Player)
	data := append([]byte{engType}, utils.PackGameMsg(&msg)...)
	waitForResponse(m, msg.Player)
	m.BaseMode().SendPacketDataToPlayer(m.currentPlayer(int(msg.Player)), network.STOC_GAME_MSG, data)
	return 1
}

func analyzeSelectChain(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	var (
		player uint8
		count  uint8
	)
	_ = pbuf.Read(&player, &count)
	pbuf.Next(int(9 + count*14))
	return analyzeSelectWait(m, pbuf, offset, player)
}

func analyzeSelectPlace(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	var player uint8
	_ = pbuf.Read(&player)
	pbuf.Next(5)
	return analyzeSelectWait(m, pbuf, offset, player)
}

func analyzeSelectCounter(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	var player uint8
	_ = pbuf.Read(&player)
	pbuf.Next(4)
	var count uint8
	_ = pbuf.Read(&count)
	pbuf.Next(int(count * 9))
	return analyzeSelectWait(m, pbuf, offset, player)
}

func analyzeSelectSum(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	// 布局以 single_duel 的解析为准（skip1, player, skip6, 两组 count+count*11），
	// 与 engineMsgLayouts / message_scan 一致；tag 旧实现把 player 读在偏移 0，
	// 此处统一修正。
	pbuf.Next(1)
	var player uint8
	_ = pbuf.Read(&player)
	pbuf.Next(6)
	var count uint8
	_ = pbuf.Read(&count)
	pbuf.Next(int(count) * 11)
	_ = pbuf.Read(&count)
	pbuf.Next(int(count) * 11)
	return analyzeSelectWait(m, pbuf, offset, player)
}

func analyzeSortCard(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	var (
		player uint8
		count  uint8
	)
	_ = pbuf.Read(&player, &count)
	pbuf.Next(int(count) * 7)
	return analyzeSelectWait(m, pbuf, offset, player)
}

func analyzeRockPaperScissors(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	var player uint8
	_ = pbuf.Read(&player)
	return analyzeSelectWait(m, pbuf, offset, player)
}

func analyzeAnnounceRace(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	var player uint8
	_ = pbuf.Read(&player)
	pbuf.Next(5)
	return analyzeSelectWait(m, pbuf, offset, player)
}

func analyzeAnnounceCard(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	var (
		player uint8
		count  uint8
	)
	_ = pbuf.Read(&player, &count)
	pbuf.Next(int(count) * 4)
	return analyzeSelectWait(m, pbuf, offset, player)
}

// ----- 共享 handler：带隐藏规则的路由 -----

// analyzeConfirmCards 卡片确认：卡组位置只发给当前操作者，其余位置全员广播。
func analyzeConfirmCards(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	base := m.BaseMode()
	var (
		player uint8
		n      uint8
		count  uint8
	)
	_ = pbuf.Read(&player, &n, &count)
	if pbuf.At(5) != ocgcore.LOCATION_DECK {
		pbuf.Next(int(count) * 7)
		broadcastData(m, network.STOC_GAME_MSG, offset.SubSlices(pbuf))
	} else {
		pbuf.Next(int(count * 7))
		base.SendPacketDataToPlayer(m.currentPlayer(int(player)), network.STOC_GAME_MSG, offset.SubSlices(pbuf))
	}
	return 0
}

// analyzeShuffleHand 手牌洗牌：当前操作者收到完整 codes，其余玩家收到抹零版本
// （single 仅对手；tag 为除当前操作者外的所有玩家），然后整区刷新。
func analyzeShuffleHand(m duelRoom, engType uint8, pbuf *utils.YGOBuffer) int {
	base := m.BaseMode()
	var msg protocol.ShuffleHandMsg
	if err := pbuf.Unpack(&msg); err != nil {
		panic(err)
	}
	data := append([]byte{engType}, utils.PackGameMsg(&msg)...)
	base.SendPacketDataToPlayer(m.currentPlayer(int(msg.Player)), network.STOC_GAME_MSG, data)
	msg.HideAllCodes()
	data = append([]byte{engType}, utils.PackGameMsg(&msg)...)
	for _, p := range m.handMaskedRecipients(int(msg.Player)) {
		base.SendPacketDataToPlayer(p, network.STOC_GAME_MSG, data)
	}
	for _, v := range base.Observers {
		base.ReSendToPlayer(v)
	}
	m.RefreshHand(int(msg.Player), 0x781fff, 0)
	return 0
}

// analyzeShuffleExtra 额外卡组洗牌：与 analyzeShuffleHand 同构。
func analyzeShuffleExtra(m duelRoom, engType uint8, pbuf *utils.YGOBuffer) int {
	base := m.BaseMode()
	var msg protocol.ShuffleExtraMsg
	if err := pbuf.Unpack(&msg); err != nil {
		panic(err)
	}
	data := append([]byte{engType}, utils.PackGameMsg(&msg)...)
	base.SendPacketDataToPlayer(m.currentPlayer(int(msg.Player)), network.STOC_GAME_MSG, data)
	msg.HideAllCodes()
	data = append([]byte{engType}, utils.PackGameMsg(&msg)...)
	for _, p := range m.handMaskedRecipients(int(msg.Player)) {
		base.SendPacketDataToPlayer(p, network.STOC_GAME_MSG, data)
	}
	for _, v := range base.Observers {
		base.ReSendToPlayer(v)
	}
	m.RefreshExtra(int(msg.Player), 0x81fff4, 0)
	return 0
}

// analyzeSwapGraveDeck 墓地卡组交换：广播后按模式规则刷新墓地。
func analyzeSwapGraveDeck(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	var player uint8
	_ = pbuf.Read(&player)
	broadcastData(m, network.STOC_GAME_MSG, offset.SubSlices(pbuf))
	m.refreshGraveAfterSwap(int(player))
	return 0
}

// analyzeShuffleSetCard 场上设置卡洗牌：按位置类型刷新相应区域。
func analyzeShuffleSetCard(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	var loc, count uint8
	_ = pbuf.Read(&loc, &count)
	pbuf.Next(int(count) * 8)
	broadcastData(m, network.STOC_GAME_MSG, offset.SubSlices(pbuf))
	if ocgcore.LOCATION_MZONE == loc {
		m.RefreshMzone(0, 0x181fff, 0)
		m.RefreshMzone(1, 0x181fff, 0)
	} else {
		m.RefreshSzone(0, 0x181fff, 0)
		m.RefreshSzone(1, 0x181fff, 0)
	}
	return 0
}

// analyzeMove 卡片移动：控制者收到完整信息，隐藏位置的卡对对手抹码，随后单卡刷新。
func analyzeMove(m duelRoom, engType uint8, pbuf *utils.YGOBuffer) int {
	base := m.BaseMode()
	var msg protocol.MoveMsg
	if err := pbuf.Unpack(&msg); err != nil {
		panic(err)
	}
	data := append([]byte{engType}, utils.PackGameMsg(&msg)...)
	base.SendPacketDataToPlayer(m.currentPlayer(int(msg.CC)), network.STOC_GAME_MSG, data)
	if (msg.CL&(ocgcore.LOCATION_GRAVE+ocgcore.LOCATION_OVERLAY)) == 0 &&
		((msg.CL&(ocgcore.LOCATION_DECK+ocgcore.LOCATION_HAND)) != 0 || msg.CP&ocgcore.POS_FACEDOWN != 0) {
		msg.Code = 0
	}
	data = append([]byte{engType}, utils.PackGameMsg(&msg)...)
	for _, p := range m.handMaskedRecipients(int(msg.CC)) {
		base.SendPacketDataToPlayer(p, network.STOC_GAME_MSG, data)
	}
	for _, v := range base.Observers {
		base.ReSendToPlayer(v)
	}
	if msg.CL != 0 && (msg.CL&ocgcore.LOCATION_OVERLAY) == 0 && (msg.CL != msg.PL || msg.PC != msg.CC) {
		m.refreshSingleMoved(msg.CC, msg.CL, msg.CS)
	}
	return 0
}

// analyzePosChange 表示形式变更：里侧翻表侧时刷新该卡。
func analyzePosChange(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	var (
		cc = pbuf.At(4)
		cl = pbuf.At(5)
		cs = pbuf.At(6)
		pp = pbuf.At(7)
		cp = pbuf.At(8)
	)
	pbuf.Next(9)
	broadcastData(m, network.STOC_GAME_MSG, offset.SubSlices(pbuf))
	if (pp&ocgcore.POS_FACEDOWN != 0) && (cp&ocgcore.POS_FACEUP != 0) {
		m.refreshSingleMoved(cc, cl, cs)
	}
	return 0
}

// analyzeSet 设置卡片：卡码抹零后全员广播。
func analyzeSet(m duelRoom, engType uint8, pbuf *utils.YGOBuffer) int {
	var msg protocol.SetMsg
	if err := pbuf.Unpack(&msg); err != nil {
		panic(err)
	}
	msg.Code = 0
	data := append([]byte{engType}, utils.PackGameMsg(&msg)...)
	broadcastData(m, network.STOC_GAME_MSG, data)
	return 0
}

// analyzeSwap 两张卡交换位置：广播后分别刷新两张卡。
func analyzeSwap(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	var (
		c1 = pbuf.At(4)
		l1 = pbuf.At(5)
		s1 = pbuf.At(6)
		c2 = pbuf.At(12)
		l2 = pbuf.At(13)
		s2 = pbuf.At(14)
	)
	pbuf.Next(16)
	broadcastData(m, network.STOC_GAME_MSG, offset.SubSlices(pbuf))
	m.refreshSingleMoved(c1, l1, s1)
	m.refreshSingleMoved(c2, l2, s2)
	return 0
}

// analyzeSpsummoning 特殊召唤中：己方队伍（single 为本人）收完整信息，
// 里侧表示的卡在发给对方队伍前抹码。
func analyzeSpsummoning(m duelRoom, engType uint8, pbuf *utils.YGOBuffer) int {
	base := m.BaseMode()
	var msg protocol.SPSummoningMsg
	if err := pbuf.Unpack(&msg); err != nil {
		panic(err)
	}
	data := append([]byte{engType}, utils.PackGameMsg(&msg)...)
	f1, f2 := m.zoneFullPair(int(msg.CC))
	base.SendPacketDataToPlayer(f1, network.STOC_GAME_MSG, data)
	base.ReSendToPlayer(f2)
	if msg.CP&ocgcore.POS_FACEDOWN != 0 {
		msg.Code = 0
	}
	data = append([]byte{engType}, utils.PackGameMsg(&msg)...)
	h1, h2 := m.zoneMaskedPair(int(msg.CC))
	base.SendPacketDataToPlayer(h1, network.STOC_GAME_MSG, data)
	base.ReSendToPlayer(h2)
	for _, v := range base.Observers {
		base.ReSendToPlayer(v)
	}
	return 0
}

// analyzeFlipSummoning 反转召唤中：先刷新该卡（此时已知表示形式），再广播。
func analyzeFlipSummoning(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	m.refreshSingleFlip(pbuf.At(4), pbuf.At(5), pbuf.At(6))
	pbuf.Next(8)
	broadcastData(m, network.STOC_GAME_MSG, offset.SubSlices(pbuf))
	return 0
}

// analyzeCardSelected 卡片选择结果：只消费字段，不转发。
func analyzeCardSelected(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	var (
		player uint8
		count  uint8
	)
	_ = pbuf.Read(&player, &count)
	pbuf.Next(int(count) * 4)
	return 0
}

// analyzeRandomSelected 随机选择结果：发给 players[player] 并重发给 1 号起的
// 所有座位（与原版 single/tag 一致：含 player 本人所在队伍的非代表座位）。
func analyzeRandomSelected(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	base := m.BaseMode()
	var (
		player uint8
		count  uint8
	)
	_ = pbuf.Read(&player, &count)
	pbuf.Next(int(count) * 4)
	players := m.allPlayers()
	base.SendPacketDataToPlayer(players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
	for _, p := range players[1:] {
		base.ReSendToPlayer(p)
	}
	for _, v := range base.Observers {
		base.ReSendToPlayer(v)
	}
	return 0
}

// analyzeDraw 抽卡：当前操作者收到完整 codes，未带公开标记的卡对其他人抹零。
func analyzeDraw(m duelRoom, engType uint8, pbuf *utils.YGOBuffer) int {
	base := m.BaseMode()
	var msg protocol.DrawMsg
	if err := pbuf.Unpack(&msg); err != nil {
		panic(err)
	}
	data := append([]byte{engType}, utils.PackGameMsg(&msg)...)
	base.SendPacketDataToPlayer(m.currentPlayer(int(msg.Player)), network.STOC_GAME_MSG, data)
	msg.HideUnknownCards()
	data = append([]byte{engType}, utils.PackGameMsg(&msg)...)
	for _, p := range m.handMaskedRecipients(int(msg.Player)) {
		base.SendPacketDataToPlayer(p, network.STOC_GAME_MSG, data)
	}
	for _, v := range base.Observers {
		base.ReSendToPlayer(v)
	}
	return 0
}

// analyzeMissedEffect 错过时点：只发给当前操作者。
func analyzeMissedEffect(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
	player := pbuf.At(0)
	pbuf.Next(8)
	m.BaseMode().SendPacketDataToPlayer(m.currentPlayer(int(player)), network.STOC_GAME_MSG, offset.SubSlices(pbuf))
	return 0
}

// ----- 模式特有的 handler（留在各模式文件中实现） -----

// sharedAnalyzeHandlers 是两个模式共用的消息处理表。只收录两边行为一致
// （或由钩子吸收差异）的消息；模式特有消息在各模式的表构建函数里追加。
var sharedAnalyzeHandlers = map[uint8]analyzeHandler{
	ocgcore.MSG_RETRY:     analyzeRetry,
	ocgcore.MSG_HINT:      analyzeHint,
	ocgcore.MSG_WIN:       analyzeWin,
	ocgcore.MSG_NEW_PHASE: analyzeSkipBroadcastRefresh(ocgcore.MSG_NEW_PHASE, func(m duelRoom) { m.refreshAfterNewPhase() }),

	ocgcore.MSG_SELECT_BATTLECMD:     analyzeSelectBattlecmd,
	ocgcore.MSG_SELECT_IDLECMD:       analyzeSelectIdlecmd,
	ocgcore.MSG_SELECT_EFFECTYN:      analyzeSelectEffectyn,
	ocgcore.MSG_SELECT_YESNO:         analyzeSelectYesno,
	ocgcore.MSG_SELECT_OPTION:        analyzeSelectOption,
	ocgcore.MSG_SELECT_CARD:          func(m duelRoom, t uint8, p, _ *utils.YGOBuffer) int { return analyzeSelectCard(m, t, p) },
	ocgcore.MSG_SELECT_TRIBUTE:       func(m duelRoom, t uint8, p, _ *utils.YGOBuffer) int { return analyzeSelectCard(m, t, p) },
	ocgcore.MSG_SELECT_UNSELECT_CARD: func(m duelRoom, t uint8, p, _ *utils.YGOBuffer) int { return analyzeSelectUnselectCard(m, t, p) },
	ocgcore.MSG_SELECT_CHAIN:         analyzeSelectChain,
	ocgcore.MSG_SELECT_PLACE:         analyzeSelectPlace,
	ocgcore.MSG_SELECT_DISFIELD:      analyzeSelectPlace,
	ocgcore.MSG_SELECT_POSITION:      analyzeSelectPlace,
	ocgcore.MSG_SELECT_COUNTER:       analyzeSelectCounter,
	ocgcore.MSG_SELECT_SUM:           analyzeSelectSum,
	ocgcore.MSG_SORT_CARD:            analyzeSortCard,
	ocgcore.MSG_ROCK_PAPER_SCISSORS:  analyzeRockPaperScissors,
	ocgcore.MSG_ANNOUNCE_RACE:        analyzeAnnounceRace,
	ocgcore.MSG_ANNOUNCE_ATTRIB:      analyzeAnnounceRace,
	ocgcore.MSG_ANNOUNCE_CARD:        analyzeAnnounceCard,
	ocgcore.MSG_ANNOUNCE_NUMBER:      analyzeAnnounceCard,

	ocgcore.MSG_CONFIRM_DECKTOP:  analyzeSkipBroadcast(ocgcore.MSG_CONFIRM_DECKTOP),
	ocgcore.MSG_CONFIRM_EXTRATOP: analyzeSkipBroadcast(ocgcore.MSG_CONFIRM_EXTRATOP),
	ocgcore.MSG_CONFIRM_CARDS:    analyzeConfirmCards,
	ocgcore.MSG_SHUFFLE_DECK:     analyzeSkipBroadcast(ocgcore.MSG_SHUFFLE_DECK),
	ocgcore.MSG_SHUFFLE_HAND:     func(m duelRoom, t uint8, p, _ *utils.YGOBuffer) int { return analyzeShuffleHand(m, t, p) },
	ocgcore.MSG_SHUFFLE_EXTRA:    func(m duelRoom, t uint8, p, _ *utils.YGOBuffer) int { return analyzeShuffleExtra(m, t, p) },
	ocgcore.MSG_REFRESH_DECK:     analyzeSkipBroadcast(ocgcore.MSG_REFRESH_DECK),
	ocgcore.MSG_SWAP_GRAVE_DECK:  analyzeSwapGraveDeck,
	ocgcore.MSG_REVERSE_DECK:     analyzeSkipBroadcast(ocgcore.MSG_REVERSE_DECK),
	ocgcore.MSG_DECK_TOP:         analyzeSkipBroadcast(ocgcore.MSG_DECK_TOP),
	ocgcore.MSG_SHUFFLE_SET_CARD: analyzeShuffleSetCard,
	ocgcore.MSG_MOVE:             func(m duelRoom, t uint8, p, _ *utils.YGOBuffer) int { return analyzeMove(m, t, p) },
	ocgcore.MSG_POS_CHANGE:       analyzePosChange,
	ocgcore.MSG_SET:              func(m duelRoom, t uint8, p, _ *utils.YGOBuffer) int { return analyzeSet(m, t, p) },
	ocgcore.MSG_SWAP:             analyzeSwap,
	ocgcore.MSG_FIELD_DISABLED:   analyzeSkipBroadcast(ocgcore.MSG_FIELD_DISABLED),
	ocgcore.MSG_SUMMONING:        analyzeSkipBroadcast(ocgcore.MSG_SUMMONING),
	ocgcore.MSG_SUMMONED:         analyzeBroadcastRefresh(func(m duelRoom) { m.refreshAfterSummon() }),
	ocgcore.MSG_SPSUMMONING:      func(m duelRoom, t uint8, p, _ *utils.YGOBuffer) int { return analyzeSpsummoning(m, t, p) },
	ocgcore.MSG_SPSUMMONED:       analyzeBroadcastRefresh(func(m duelRoom) { m.refreshAfterSummon() }),
	ocgcore.MSG_FLIPSUMMONING:    analyzeFlipSummoning,
	ocgcore.MSG_FLIPSUMMONED:     analyzeBroadcastRefresh(func(m duelRoom) { m.refreshAfterSummon() }),
	ocgcore.MSG_CHAINING:         analyzeSkipBroadcast(ocgcore.MSG_CHAINING),
	ocgcore.MSG_CHAINED:          analyzeSkipBroadcastRefresh(ocgcore.MSG_CHAINED, func(m duelRoom) { m.refreshAfterChain() }),
	ocgcore.MSG_CHAIN_SOLVING:    analyzeSkipBroadcast(ocgcore.MSG_CHAIN_SOLVING),
	ocgcore.MSG_CHAIN_SOLVED:     analyzeSkipBroadcastRefresh(ocgcore.MSG_CHAIN_SOLVED, func(m duelRoom) { m.refreshAfterChain() }),
	ocgcore.MSG_CHAIN_END:        analyzeBroadcastRefresh(func(m duelRoom) { m.refreshAfterChain() }),
	ocgcore.MSG_CHAIN_NEGATED:    analyzeSkipBroadcast(ocgcore.MSG_CHAIN_NEGATED),
	ocgcore.MSG_CHAIN_DISABLED:   analyzeSkipBroadcast(ocgcore.MSG_CHAIN_DISABLED),
	ocgcore.MSG_CARD_SELECTED:    analyzeCardSelected,
	ocgcore.MSG_RANDOM_SELECTED:  analyzeRandomSelected,
	ocgcore.MSG_BECOME_TARGET:    analyzeSkipBroadcast(ocgcore.MSG_BECOME_TARGET),
	ocgcore.MSG_DRAW:             func(m duelRoom, t uint8, p, _ *utils.YGOBuffer) int { return analyzeDraw(m, t, p) },
	ocgcore.MSG_DAMAGE:           analyzeSkipBroadcast(ocgcore.MSG_DAMAGE),
	ocgcore.MSG_RECOVER:          analyzeSkipBroadcast(ocgcore.MSG_RECOVER),
	ocgcore.MSG_EQUIP:            analyzeSkipBroadcast(ocgcore.MSG_EQUIP),
	ocgcore.MSG_LPUPDATE:         analyzeSkipBroadcast(ocgcore.MSG_LPUPDATE),
	ocgcore.MSG_UNEQUIP:          analyzeSkipBroadcast(ocgcore.MSG_UNEQUIP),
	ocgcore.MSG_CARD_TARGET:      analyzeSkipBroadcast(ocgcore.MSG_CARD_TARGET),
	ocgcore.MSG_CANCEL_TARGET:    analyzeSkipBroadcast(ocgcore.MSG_CANCEL_TARGET),
	ocgcore.MSG_PAY_LPCOST:       analyzeSkipBroadcast(ocgcore.MSG_PAY_LPCOST),
	ocgcore.MSG_ADD_COUNTER:      analyzeSkipBroadcast(ocgcore.MSG_ADD_COUNTER),
	ocgcore.MSG_REMOVE_COUNTER:   analyzeSkipBroadcast(ocgcore.MSG_REMOVE_COUNTER),
	ocgcore.MSG_ATTACK:           analyzeSkipBroadcast(ocgcore.MSG_ATTACK),
	ocgcore.MSG_BATTLE:           analyzeSkipBroadcast(ocgcore.MSG_BATTLE),
	ocgcore.MSG_ATTACK_DISABLED:  analyzeSkipBroadcast(ocgcore.MSG_ATTACK_DISABLED),
	ocgcore.MSG_DAMAGE_STEP_START: analyzeSkipBroadcastRefresh(ocgcore.MSG_DAMAGE_STEP_START, func(m duelRoom) {
		m.refreshAfterDamageStep()
	}),
	ocgcore.MSG_DAMAGE_STEP_END: analyzeSkipBroadcastRefresh(ocgcore.MSG_DAMAGE_STEP_END, func(m duelRoom) {
		m.refreshAfterDamageStep()
	}),
	ocgcore.MSG_MISSED_EFFECT: analyzeMissedEffect,
	ocgcore.MSG_TOSS_COIN:     analyzeSkipBroadcast(ocgcore.MSG_TOSS_COIN),
	ocgcore.MSG_TOSS_DICE:     analyzeSkipBroadcast(ocgcore.MSG_TOSS_DICE),
	ocgcore.MSG_CARD_HINT:     analyzeSkipBroadcast(ocgcore.MSG_CARD_HINT),
	ocgcore.MSG_PLAYER_HINT:   analyzeSkipBroadcast(ocgcore.MSG_PLAYER_HINT),
}

// copyAnalyzeTable 复制共享表并追加/覆盖模式特有条目。
func copyAnalyzeTable(extra map[uint8]analyzeHandler) map[uint8]analyzeHandler {
	t := make(map[uint8]analyzeHandler, len(sharedAnalyzeHandlers)+len(extra))
	for k, v := range sharedAnalyzeHandlers {
		t[k] = v
	}
	for k, v := range extra {
		t[k] = v
	}
	return t
}
