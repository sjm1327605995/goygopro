package duel

import (
	"bytes"
	"encoding/binary"
	"log"
	"math/rand"
	"time"

	"github.com/duke-git/lancet/v2/condition"
	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// duelRoom 抽象 SingleDuel / TagDuel 之间的差异点，供共享助手函数使用。
// 实现为 *SingleDuel / *TagDuel；新增共享逻辑时在此补充钩子。
type duelRoom interface {
	IDuelMode
	// DuelEndProc 处理一局结束后的房间收尾（single 含 match 局数逻辑，tag 直接结束）。
	DuelEndProc()
	// allPlayers 按座位顺序返回全部玩家（长度 2 或 4，可能含 nil）。
	// 返回切片与内部数组共享底层存储，调用方不得修改。
	allPlayers() []*DuelPlayer
	// currentPlayer 返回引擎视角玩家(0/1)当前实际操作的玩家
	// （single: players[player]；tag: curPlayer[player]）。
	currentPlayer(player int) *DuelPlayer
	// zoneFullPair 返回刷新场地时接收完整数据的玩家（至多 2 个，second 可为 nil）
	// （single: 仅 players[player]；tag: 队友两人 players[teamBase], players[teamBase+1]）。
	zoneFullPair(player int) (first, second *DuelPlayer)
	// zoneMaskedPair 返回刷新场地时接收里侧抹除数据的玩家（至多 2 个，second 可为 nil）
	// （single: 仅对手 players[1-player]；tag: 对方队伍两人）。
	zoneMaskedPair(player int) (first, second *DuelPlayer)
	// handMaskedRecipients 返回刷新手牌时接收隐藏数据（非公开卡码抹除）的玩家。
	handMaskedRecipients(player int) []*DuelPlayer
	// waitingNotifyRecipients 返回 WaitForResponse 时需要收到 MSG_WAITING 的玩家
	// （single: 仅对手 players[1-player]；tag: 除当前操作者外的所有玩家）。
	waitingNotifyRecipients(player byte) []*DuelPlayer
	// timeLimitToObservers 报告广播 STOC_TIME_LIMIT 时是否同时重发给观察者
	// （single: 是；tag: 否——原版 TagDuel 不发）。
	timeLimitToObservers() bool
	// isResponder 报告 dp 是否是当前等待响应的玩家（TimeConfirm 用；
	// single: dp.Type == lastResponse；tag: dp == curPlayer[lastResponse]）。
	isResponder(dp *DuelPlayer) bool
	// firstFreeSeat 返回第一个空座位下标（调用前必须确保存在空位）。
	firstFreeSeat() int
	// assignSeat / removeFromSeat 把玩家放入/移出指定座位。
	assignSeat(pos int, dp *DuelPlayer)
	removeFromSeat(seat int)
	// handResultIndex 把座位号映射为 handResult 下标（single: 即座位号；tag: 0 号位 → 0，其余 → 1）。
	handResultIndex(seat uint8) int
	// handSeats 返回猜拳双方座位：a 为先手代表、b 为对方代表，
	// a2/b2 为各自队友（single 为 nil，对应原版不向第二人重发）。
	handSeats() (a, a2, b, b2 *DuelPlayer)
	// recordTpPlayer 在猜拳决出先后攻时记录将要选择先攻的一方
	// （single: 写 tpPlayer，bWins=true 记 1 否则记 0；tag: 无此概念）。
	recordTpPlayer(bWins bool)
	// onJoinPassDenied 是 JoinGame 密码错误后的模式特有处理（single: 断开连接；tag: 不断开）。
	onJoinPassDenied(dp *DuelPlayer)
	// onDuelEnded 在 EndDuel 公共流程结束后执行的模式特有清理
	// （single: 无；tag: 把所有玩家 State 置 0xff）。
	onDuelEnded()
	// leaveAsPlayer 处理决斗者（非房主、非观战者）的离开逻辑（single/tag 差异较大）。
	leaveAsPlayer(dp *DuelPlayer)
	// RefreshHand / RefreshExtra / RefreshMzone / RefreshSzone 由 DuelMode 基类
	// 实现一次（见 duel_refresh.go），接口中保留声明有两个原因：共享 handler 经
	// duelRoom 分发；测试桩（analyzeFakeRoom）需要覆写为空操作（真实刷新依赖引擎）。
	RefreshHand(player int, flag uint32, useCache int)
	RefreshExtra(player int, flag uint32, useCache int)
	RefreshMzone(player int, flag uint32, useCache int)
	RefreshSzone(player int, flag uint32, useCache int)
	// onEngineWin 处理引擎判定的胜负（MSG_WIN；single: matchResult/tpPlayer 记账；tag: 无）。
	onEngineWin(player uint8)
	// RefreshSingle 是单卡刷新（MSG_UPDATE_CARD），发送对象按模式不同
	//（single: 本人+对手；tag: 队友/队伍分发），由模式实现；
	// duelRoom 声明它供基类的 refreshSingleMoved / refreshSingleFlip 读表转发。
	RefreshSingle(player uint8, location uint8, sequence uint8, flag int32)
	// refreshGraveAfterSwap / refreshAfterSummon / refreshAfterChain /
	// refreshAfterDamageStep / refreshAfterNewPhase / refreshSingleMoved /
	// refreshSingleFlip 原本是两模式仅查询 flag/缓存常量不同的钩子，差异已收进
	// DuelMode.refresh 参数表（refreshFlags），现由 DuelMode 基类读表实现一次；
	// 接口中保留声明是为了让测试桩（analyzeFakeRoom）可以继续覆写为空操作。
	refreshGraveAfterSwap(player int)
	// refreshAfterSummon 是 MSG_SUMMONED/SPSUMMONED/FLIPSUMMONED 后的场地刷新。
	refreshAfterSummon()
	// refreshAfterChain 是 MSG_CHAINED/CHAIN_SOLVED/CHAIN_END 后的全区域刷新。
	refreshAfterChain()
	// refreshAfterDamageStep 是 MSG_DAMAGE_STEP_START/END 后的怪兽区刷新。
	refreshAfterDamageStep()
	// refreshAfterNewPhase 是 MSG_NEW_PHASE 后的区域刷新。
	refreshAfterNewPhase()
	// refreshSingleMoved 是 MSG_MOVE/POS_CHANGE/SWAP 后的单卡刷新
	// （single: RefreshSingleDef 即 0xf81fff；tag: RefreshSingle 0x81fff4）。
	refreshSingleMoved(cc, cl, cs uint8)
	// refreshSingleFlip 是 MSG_FLIPSUMMONING 前置的单卡刷新
	// （single: 0xf81fff；tag: 0x81fff4）。
	refreshSingleFlip(cc, cl, cs uint8)
	// seatCount 返回座位总数（single: 2；tag: 4），用于开局就绪检查与洗牌范围。
	seatCount() int
	// replayFlag 返回回放头标志（single: REPLAY_UNIFORM；tag: REPLAY_UNIFORM|REPLAY_TAG，
	// 与原版 single_duel.cpp / tag_duel.cpp 的 rh.flag 一致）。
	replayFlag() uint32
	// extraDuelOpt 返回并入 start_duel opt 的模式特有引擎选项
	// （single: 0；tag: DUEL_TAG_MODE）。
	extraDuelOpt() uint32
	// loadDecksToEngine 把本模式全部座位的卡组按原版顺序装入引擎并写入回放
	// （含 Main 的 Reverse 与 AddCard/AddTagCard 的调用序列，逐字节对应
	// 原版 TPResult 的 load 段；调用时 Duel 与回放已就绪）。
	loadDecksToEngine(d *ocgcore.Duel, rp *Replay)
	// refreshOnDuelStart 是 TPResult 末尾、Duel.Start 之前的开局额外卡组刷新
	// （single: RefreshExtraDef(0)/(1)；tag: RefreshExtra(0/1, 0x81fff4, 0)）。
	refreshOnDuelStart()
	// armDuelTimer 在限时开启时武装决斗秒表（single: 每秒 SingleTimer 且
	// 超时处理自行重新武装；tag: TagTimer，原版非超时路径不重新武装——
	// 两种语义各自保留在模式的 timer 回调里，此处只做首次武装）。
	armDuelTimer()
}

// broadcastData 把 proto+data 发送给所有玩家与观察者：首个玩家用
// SendPacketDataToPlayer 编码进 buff，其余玩家与观察者用 ReSendToPlayer
// 复用同一 buff。与各模式里重复出现的发送三连（Send + ReSend + 观察者循环）
// 字节序完全一致。
func broadcastData(m duelRoom, proto byte, data any) {
	base := m.BaseMode()
	players := m.allPlayers()
	base.SendPacketDataToPlayer(players[0], proto, data)
	for _, p := range players[1:] {
		base.ReSendToPlayer(p)
	}
	for _, v := range base.Observers {
		base.ReSendToPlayer(v)
	}
}

// broadcastProto 是 broadcastData 的无数据版本（仅 proto 字节）。
func broadcastProto(m duelRoom, proto byte) {
	base := m.BaseMode()
	players := m.allPlayers()
	base.SendPacketToPlayer(players[0], proto)
	for _, p := range players[1:] {
		base.ReSendToPlayer(p)
	}
	for _, v := range base.Observers {
		base.ReSendToPlayer(v)
	}
}

// waitForResponse 是 WaitForResponse 的公共实现：记录 lastResponse，
// 向 waitingNotifyRecipients（single 为对手，tag 为除当前操作者外的所有玩家）
// 发送 MSG_WAITING；限时开启时广播 STOC_TIME_LIMIT 并把被等待玩家状态置为
// CTOS_TIME_CONFIRM，否则直接置为 CTOS_RESPONSE。
func waitForResponse(m duelRoom, player byte) {
	base := m.BaseMode()
	base.lastResponse = player
	msgWaiting := []byte{ocgcore.MSG_WAITING}
	for _, p := range m.waitingNotifyRecipients(player) {
		base.SendPacketDataToPlayer(p, network.STOC_GAME_MSG, msgWaiting)
	}
	if base.HostInfo.TimeLimit != 0 {
		base.timeElapsed = 0
		var sctl protocol.STOCTimeLimit
		sctl.Player = player
		sctl.LeftTime = uint16(base.timeLimit[player])
		players := m.allPlayers()
		base.SendPacketDataToPlayer(players[0], network.STOC_TIME_LIMIT, sctl)
		for _, p := range players[1:] {
			base.ReSendToPlayer(p)
		}
		if m.timeLimitToObservers() {
			for _, v := range base.Observers {
				base.ReSendToPlayer(v)
			}
		}
		m.currentPlayer(int(player)).State = network.CTOS_TIME_CONFIRM
	} else {
		m.currentPlayer(int(player)).State = network.CTOS_RESPONSE
	}
}

// timeConfirm 是 TimeConfirm 的公共实现：限时开启且 dp 正是被等待响应的玩家时，
// 把其状态推进到 CTOS_RESPONSE，并清零 10 秒以内的计时时长。
func timeConfirm(m duelRoom, dp *DuelPlayer) {
	base := m.BaseMode()
	if base.HostInfo.TimeLimit == 0 {
		return
	}
	if !m.isResponder(dp) {
		return
	}
	m.currentPlayer(int(base.lastResponse)).State = network.CTOS_RESPONSE
	if base.timeElapsed < 10 {
		base.timeElapsed = 0
	}
}

// recordResponseAndSet 把客户端响应写入回放并提交给引擎：
// 回放记录 [长度字节, 响应字节]，引擎收到定长 SIZE_RETURN_VALUE 缓冲区。
// 两个模式的逐字节行为一致（包括 msgBuffer 超长时的表现），此处合并。
func recordResponseAndSet(d *DuelMode, msgBuffer []byte) {
	resb := make([]byte, ocgcore.SIZE_RETURN_VALUE)
	copy(resb, msgBuffer)
	d.lastReplay.WriteData([]byte{uint8(len(msgBuffer))}, false)
	d.lastReplay.WriteData(resb[:len(msgBuffer)], false)
	d.Duel.SetResponseBytes(resb)
}

// endDuel 是 EndDuel 的公共实现：回放收尾并广播 STOC_REPLAY，结束引擎决斗、
// 停掉计时器；之后执行 onDuelEnded 钩子（tag 借此把所有玩家 State 置 0xff）。
func endDuel(m duelRoom) {
	base := m.BaseMode()
	if base.Duel == nil {
		return
	}
	base.lastReplay.EndRecord()
	replayBuf := make([]byte, 0x2000)
	pBuf := utils.NewYGOBuffer(replayBuf, binary.LittleEndian)
	pBuf.Write(base.lastReplay.pheader)
	pBuf.Write(base.lastReplay.compData[:base.lastReplay.compSize])
	broadcastData(m, network.STOC_REPLAY, replayBuf[:pBuf.Offset()])
	base.Duel.End()
	if base.ETimer != nil {
		base.ETimer.Stop()
		base.ETimer = nil
	}
	base.Duel = nil
	m.onDuelEnded()
}

// tickDuelTimerLocked 推进决斗秒表（调用方必须已持有 base.Mu，且整个超时处理
// 也应在该锁保护下进行——与原版 SingleTimer/TagTimer 的临界区一致）：
// running=false 表示计时器应停止（timeout=true 为超时，false 为决斗已结束
// 或不在决斗阶段），running=true 表示本轮未超时，由调用方决定是否重新武装。
func tickDuelTimerLocked(base *DuelMode) (running bool, timeout bool) {
	if base.Duel == nil || base.DuelStage != network.DUEL_STAGE_DUELING {
		return false, false
	}
	base.timeElapsed++
	if int(base.timeElapsed) >= int(base.timeLimit[base.lastResponse]) || base.timeLimit[base.lastResponse] <= 0 {
		return false, true
	}
	return true, false
}

// processDuel 是 Process 的公共实现：驱动引擎直到出现交互提示（Analyze 返回 1）、
// 处理器结束或游戏结束（Analyze 返回 2，随后走 DuelEndProc）。
func processDuel(m duelRoom) {
	base := m.BaseMode()
	var (
		buff    = make([]byte, ocgcore.SIZE_MESSAGE_BUFFER)
		engFlag uint32
		engLen  int
		stop    int
	)
	for stop == 0 {
		if engFlag == ocgcore.PROCESSOR_END {
			break
		}
		result := base.Duel.Process()
		engLen = int(result & ocgcore.PROCESSOR_BUFFER_LEN)
		engFlag = result & ocgcore.PROCESSOR_FLAG
		if engLen > 0 {
			if engLen > len(buff) {
				buff = make([]byte, engLen)
			}
			base.Duel.GetMessage(buff)
			stop = m.Analyze(buff[:engLen])
		}
	}
	if stop == 2 {
		m.DuelEndProc()
	}
}

// duelEndProcSimple 是非 match 模式的 DuelEndProc：广播 STOC_DUEL_END 并置阶段为 END。
// （tag 的 DuelEndProc 即此实现；single 的 match 分支另有自己的局数逻辑。）
func duelEndProcSimple(m duelRoom) {
	broadcastProto(m, network.STOC_DUEL_END)
	m.BaseMode().DuelStage = network.DUEL_STAGE_END
}

// duelChat 是 Chat 的公共实现：构造聊天包并广播给所有玩家与观察者。
func duelChat(m duelRoom, dp *DuelPlayer, pData []byte) {
	base := m.BaseMode()
	var buff bytes.Buffer
	sccSize := base.CreateChatPacket(pData, &buff, uint16(dp.Type))
	if sccSize == 0 {
		return
	}
	broadcastData(m, network.STOC_CHAT, buff.Bytes())
}

// sendHsWatchChangeAll 广播观战人数变化（STOC_HS_WATCH_CHANGE）给所有玩家与观察者。
func sendHsWatchChangeAll(m duelRoom) {
	base := m.BaseMode()
	var watchChangePkt protocol.STOCHsWatchChange
	watchChangePkt.WatchCount = uint16(len(base.Observers))
	for _, p := range m.allPlayers() {
		base.SendPacketDataToPlayer(p, network.STOC_HS_WATCH_CHANGE, watchChangePkt)
	}
	for _, v := range base.Observers {
		base.ReSendToPlayer(v)
	}
}

// sendHsPlayerChangeAll 向所有玩家与观察者发送大厅玩家状态变化（STOC_HS_PLAYER_CHANGE）。
func sendHsPlayerChangeAll(m duelRoom, scpc protocol.STOCHsPlayerChange) {
	base := m.BaseMode()
	for _, p := range m.allPlayers() {
		base.SendPacketDataToPlayer(p, network.STOC_HS_PLAYER_CHANGE, scpc)
	}
	for _, v := range base.Observers {
		base.ReSendToPlayer(v)
	}
}

// typeChangeType 计算 STOC_TYPE_CHANGE 的类型字节：房主加 0x10 标志，或上当前 Type。
func typeChangeType(m duelRoom, dp *DuelPlayer) uint8 {
	t := condition.Ternary[bool, uint8](dp == m.BaseMode().HostPlayer, 0x10, 0)
	return t | dp.Type
}

// playerReady 是 PlayerReady 的公共实现：校验卡组（含禁卡表与缓存的未知卡错误），
//
//	deckError 非零时告知本人并退回 NOTREADY；否则更新 ready 状态并向全房间广播。
func playerReady(m duelRoom, dp *DuelPlayer, isReady bool) {
	base := m.BaseMode()
	if int(dp.Type) > len(m.allPlayers())-1 || base.ready[dp.Type] == isReady {
		return
	}
	if isReady {
		var deckError uint32
		if base.HostInfo.NoCheckDeck == 0 {
			if base.DeckError[dp.Type] != 0 {
				deckError = network.DECKERROR_UNKNOWNCARD<<28 | base.DeckError[dp.Type]
			} else {
				deckError = DeckManager.CheckDeck(base.pDeck[dp.Type], base.HostInfo.LFList, int(base.HostInfo.Rule))
			}
		}
		if deckError != 0 {
			var scpc protocol.STOCHsPlayerChange
			scpc.Status = dp.Type<<4 | network.PLAYERCHANGE_NOTREADY
			base.SendPacketDataToPlayer(dp, network.STOC_HS_PLAYER_CHANGE, scpc)
			var scem protocol.STOCErrorMsg
			scem.Msg = network.ERRMSG_DECKERROR
			scem.Code = deckError
			base.SendPacketDataToPlayer(dp, network.STOC_ERROR_MSG, scem)
			return
		}
	}
	base.ready[dp.Type] = isReady
	var scpc protocol.STOCHsPlayerChange
	scpc.Status = dp.Type<<4 | condition.Ternary[bool, byte](isReady, network.PLAYERCHANGE_READY, network.PLAYERCHANGE_NOTREADY)
	sendHsPlayerChangeAll(m, scpc)
}

// leaveGame 是 LeaveGame 的公共部分：房主离开直接结束房间；观战者离开更新观战人数；
// 决斗者离开交给 leaveAsPlayer（各模式差异较大，单独实现）。
func leaveGame(m duelRoom, dp *DuelPlayer) {
	base := m.BaseMode()
	if dp == base.HostPlayer {
		m.EndDuel()
		// 先摘除房间再停服务器：StopServer 会阻塞等待 gnet 事件循环退出，
		// 而 leaveGame 正是在事件循环回调（OnClose/CTOS_LEAVE_GAME）里执行的，
		// 顺序颠倒会死锁，导致 RemoveRoom 永远执行不到。
		DefaultManager.RemoveRoom(base.RoomID)
		base.StopServer()
		return
	}
	if dp.Type == network.NETPLAYER_TYPE_OBSERVER {
		delete(base.Observers, dp.ID)
		if base.DuelStage == network.DUEL_STAGE_BEGIN {
			sendHsWatchChangeAll(m)
		}
		_ = base.DisconnectPlayer(dp)
		return
	}
	m.leaveAsPlayer(dp)
}

// toObserver 是 ToObserver 的公共实现：广播状态变化、把玩家从座位移到观战列表。
func toObserver(m duelRoom, dp *DuelPlayer) {
	base := m.BaseMode()
	if int(dp.Type) > len(m.allPlayers())-1 {
		return
	}
	var scpc protocol.STOCHsPlayerChange
	scpc.Status = dp.Type<<4 | network.PLAYERCHANGE_OBSERVE
	sendHsPlayerChangeAll(m, scpc)
	m.removeFromSeat(int(dp.Type))
	base.ready[dp.Type] = false
	dp.Type = network.NETPLAYER_TYPE_OBSERVER
	base.Observers[dp.ID] = dp
	base.SendPacketDataToPlayer(dp, network.STOC_TYPE_CHANGE, protocol.STOCTypeChange{Type: typeChangeType(m, dp)})
}

// toDuelListObserver 把观战者提升为决斗者（single 的 ToDuelList 全部逻辑、tag 的
// 观战者分支）。调用前必须保证 dp 是观战者且尚有空座位。
func toDuelListObserver(m duelRoom, dp *DuelPlayer) {
	base := m.BaseMode()
	delete(base.Observers, dp.ID)
	var playerEnterPkt protocol.STOCHsPlayerEnter
	copy(playerEnterPkt.Name[:], dp.Name[:])
	pos := m.firstFreeSeat()
	dp.Type = uint8(pos)
	m.assignSeat(pos, dp)
	playerEnterPkt.Pos = uint8(pos)
	var watchChangePkt protocol.STOCHsWatchChange
	watchChangePkt.WatchCount = uint16(len(base.Observers))
	for _, p := range m.allPlayers() {
		base.SendPacketDataToPlayer(p, network.STOC_HS_PLAYER_ENTER, playerEnterPkt)
		base.SendPacketDataToPlayer(p, network.STOC_HS_WATCH_CHANGE, watchChangePkt)
	}
	for _, v := range base.Observers {
		base.SendPacketDataToPlayer(v, network.STOC_HS_PLAYER_ENTER, playerEnterPkt)
		base.SendPacketDataToPlayer(v, network.STOC_HS_WATCH_CHANGE, watchChangePkt)
	}
	base.SendPacketDataToPlayer(dp, network.STOC_TYPE_CHANGE, protocol.STOCTypeChange{Type: typeChangeType(m, dp)})
}

// playerKick 是 PlayerKick 的公共实现：仅房主可踢、不能踢自己、目标必须在场。
func playerKick(m duelRoom, dp *DuelPlayer, pos byte) {
	base := m.BaseMode()
	players := m.allPlayers()
	if int(pos) > len(players)-1 || dp != base.HostPlayer || dp == players[pos] || players[pos] == nil {
		return
	}
	m.LeaveGame(players[pos])
}

// handResult 是 HandResult 的公共实现：收集双方猜拳结果，平局重猜，否则向胜者代表
// 发送 STOC_SELECT_TP 并推进到 DUEL_STAGE_FIRSTGO。
func handResult(m duelRoom, dp *DuelPlayer, res byte) {
	base := m.BaseMode()
	if res > 3 {
		return
	}
	if dp.State != network.CTOS_HAND_RESULT {
		return
	}
	base.handResult[m.handResultIndex(dp.Type)] = res
	if base.handResult[0] != 0 && base.handResult[1] != 0 {
		a, a2, b, b2 := m.handSeats()
		var handResultPkt protocol.STOCHandResult
		handResultPkt.Res1 = base.handResult[0]
		handResultPkt.Res2 = base.handResult[1]
		base.SendPacketDataToPlayer(a, network.STOC_HAND_RESULT, handResultPkt)
		base.ReSendToPlayer(a2)
		for _, v := range base.Observers {
			base.ReSendToPlayer(v)
		}
		handResultPkt.Res1 = base.handResult[1]
		handResultPkt.Res2 = base.handResult[0]
		base.SendPacketDataToPlayer(b, network.STOC_HAND_RESULT, handResultPkt)
		base.ReSendToPlayer(b2)
		if base.handResult[0] == base.handResult[1] {
			base.SendPacketToPlayer(a, network.STOC_SELECT_HAND)
			base.ReSendToPlayer(b)
			base.handResult[0], base.handResult[1] = 0, 0
			a.State = network.CTOS_HAND_RESULT
			b.State = network.CTOS_HAND_RESULT
		} else if (base.handResult[0] == 1 && base.handResult[1] == 2) ||
			(base.handResult[0] == 2 && base.handResult[1] == 3) ||
			(base.handResult[0] == 3 && base.handResult[1] == 1) {
			base.SendPacketToPlayer(b, network.STOC_SELECT_TP)
			a.State = 0xff
			b.State = network.CTOS_TP_RESULT
			base.DuelStage = network.DUEL_STAGE_FIRSTGO
			m.recordTpPlayer(true)
		} else {
			base.SendPacketToPlayer(a, network.STOC_SELECT_TP)
			b.State = 0xff
			a.State = network.CTOS_TP_RESULT
			base.DuelStage = network.DUEL_STAGE_FIRSTGO
			m.recordTpPlayer(false)
		}
	}
}

// unpackDeckData 校验并解包 CTOS_UPDATE_DECK 载荷。长度与解包失败静默返回
// （与原版一致，仅记日志）；内容不合法时发送 ERRMSG_DECKERROR 后返回 ok=false。
func unpackDeckData(base *DuelMode, dp *DuelPlayer, pData []byte) (protocol.CTOSDeckData, bool) {
	length := len(pData)
	if length < 8 || length > 2008 {
		return protocol.CTOSDeckData{}, false
	}
	var deckBuf protocol.CTOSDeckData
	// CTOSDeckData 带变长 List，走它自己的 Unpack（restruct 的反射路径处理
	// 不了无 struct tag 的变长切片，会解包失败导致卡组被静默丢弃——随后
	// StartDuel 解引用 nil 的 pDeck 直接 panic）。
	if _, err := deckBuf.Unpack(pData, binary.LittleEndian); err != nil {
		log.Printf("[duel] unpack CTOS_UPDATE_DECK: %v", err)
		return protocol.CTOSDeckData{}, false
	}
	if deckBuf.MainC < 0 || deckBuf.MainC > protocol.MAINC_MAX ||
		deckBuf.SideC < 0 || deckBuf.SideC > protocol.SIDEC_MAX ||
		int32(length) < (2+deckBuf.MainC+deckBuf.SideC)*4 {
		base.SendPacketDataToPlayer(dp, network.STOC_ERROR_MSG, protocol.STOCErrorMsg{Msg: network.ERRMSG_DECKERROR, Code: 0})
		return protocol.CTOSDeckData{}, false
	}
	return deckBuf, true
}

// checkJoinAllowed 执行 JoinGame 的三项进入校验（已在游戏/版本/密码），
// 不通过时发送错误包并按模式特有方式处理连接后返回 false。
func checkJoinAllowed(m duelRoom, dp *DuelPlayer, pkt *protocol.CTOSJoinGame, isCreator bool) bool {
	base := m.BaseMode()
	if isCreator {
		return true
	}
	if dp.Game != nil && dp.Type != 0xff {
		base.SendPacketDataToPlayer(dp, network.STOC_ERROR_MSG, protocol.STOCErrorMsg{Msg: network.ERRMSG_JOINERROR})
		_ = base.DisconnectPlayer(dp)
		return false
	}
	if pkt.Version != PRO_VERSION {
		base.SendPacketDataToPlayer(dp, network.STOC_ERROR_MSG, protocol.STOCErrorMsg{Msg: network.ERRMSG_VERERROR, Code: PRO_VERSION})
		_ = base.DisconnectPlayer(dp)
		return false
	}
	var jpass [20]uint16
	utils.NullTerminate(pkt.Pass[:], 0)
	copy(jpass[:], pkt.Pass[:])
	if utils.Wcscmp(jpass[:], base.Pass[:]) != 0 {
		base.SendPacketDataToPlayer(dp, network.STOC_ERROR_MSG, protocol.STOCErrorMsg{Msg: network.ERRMSG_JOINERROR, Code: 1})
		m.onJoinPassDenied(dp)
		return false
	}
	return true
}

// startDuelCommon 是 StartDuel 的公共实现（对应原版 single_duel.cpp /
// tag_duel.cpp 的 StartDuel 逐字节一致段）：校验房主与全部座位就绪、停止监听、
// 广播 STOC_DUEL_START（观察者同时被置为离开状态）、按猜拳队伍分组发送
// STOC_DECK_COUNT——先手队（handSeats 的 a/a2）收前 6 字节、后手队（b/b2）收
// 交换后的后 6 字节（single 的 a2/b2 为 nil，ReSendToPlayer(nil) 为空操作，
// 与原版 single 只发两人完全一致）——最后广播 STOC_SELECT_HAND 进入猜拳。
func startDuelCommon(m duelRoom, dp *DuelPlayer) {
	base := m.BaseMode()
	if dp != base.HostPlayer {
		return
	}
	for i := 0; i < m.seatCount(); i++ {
		if !base.ready[i] {
			return
		}
	}
	base.StopListen()
	players := m.allPlayers()
	base.SendPacketToPlayer(players[0], network.STOC_DUEL_START)
	for _, p := range players[1:] {
		base.ReSendToPlayer(p)
	}
	for _, v := range base.Observers {
		v.State = network.CTOS_LEAVE_GAME
		base.ReSendToPlayer(v)
	}
	a, a2, b, b2 := m.handSeats()
	deckBuff := make([]byte, 12)
	pBuf := utils.NewYGOBuffer(deckBuff, binary.LittleEndian)
	pBuf.Write(
		int16(len(base.pDeck[a.Type].Main)), int16(len(base.pDeck[a.Type].Extra)), int16(len(base.pDeck[a.Type].Side)),
		int16(len(base.pDeck[b.Type].Main)), int16(len(base.pDeck[b.Type].Extra)), int16(len(base.pDeck[b.Type].Side)))
	base.SendPacketDataToPlayer(a, network.STOC_DECK_COUNT, deckBuff)
	base.ReSendToPlayer(a2)

	// 交换前6字节和后6字节
	temp := make([]byte, 6)
	copy(temp, deckBuff[:6])
	copy(deckBuff[:6], deckBuff[6:])
	copy(deckBuff[6:], temp)
	base.SendPacketDataToPlayer(b, network.STOC_DECK_COUNT, deckBuff)
	base.ReSendToPlayer(b2)
	base.SendPacketToPlayer(a, network.STOC_SELECT_HAND)
	base.ReSendToPlayer(b)
	base.handResult[0] = 0
	base.handResult[1] = 0
	a.State = network.CTOS_HAND_RESULT
	b.State = network.CTOS_HAND_RESULT
	base.DuelStage = network.DUEL_STAGE_FINGER
}

// beginDuel 是 TPResult 的公共实现（对应原版两个 TPResult 逐字节一致段）：
// 置回应状态、生成种子与回放头（标志取 replayFlag）、按座位顺序写玩家名、
// 洗牌（NoShuffleDeck 时跳过）、初始化引擎与开局参数（opt = DuelRule<<16 |
// PSEUDO_SHUFFLE? | extraDuelOpt）、装载卡组（loadDecksToEngine）、广播
// MSG_START（a/a2 收己方视角 0/1，b/b2 收对方视角，观察者收 0x10/0x11
// 取决于 swapped）、开局刷新（refreshOnDuelStart）、Duel.Start、限时武装
// （armDuelTimer），最后驱动引擎直至出现交互提示。
// swapped 由各模式的换位逻辑得出，仅影响观察者收到的 MSG_START 标志字节。
func beginDuel(m duelRoom, dp *DuelPlayer, swapped bool) {
	base := m.BaseMode()
	dp.State = network.CTOS_RESPONSE
	seed := rand.Uint32()

	rnd := rand.New(rand.NewSource(int64(seed)))
	rh := ExtendedReplayHeader{}
	rh.Base.ID = REPLAY_ID_YRP2
	rh.Base.Version = PRO_VERSION
	rh.Base.Flag = m.replayFlag()
	rh.Base.Seed = seed
	for i := 0; i < SEED_COUNT; i++ {
		rh.SeedSequence[i] = rand.Uint32()
	}
	rh.Base.StartTime = uint32(time.Now().Unix())
	base.lastReplay = NewReplay()
	base.lastReplay.BeginRecord()
	base.lastReplay.WriteHeader(rh)
	for _, p := range m.allPlayers() {
		name := make([]byte, 40)
		for j := 0; j < 20; j++ {
			binary.LittleEndian.PutUint16(name[j*2:], p.Name[j])
		}
		base.lastReplay.WriteData(name, false)
	}
	if base.HostInfo.NoShuffleDeck == 0 {
		for seat := 0; seat < m.seatCount(); seat++ {
			main := base.pDeck[seat].Main
			rnd.Shuffle(len(main), func(x, y int) {
				main[x], main[y] = main[y], main[x]
			})
		}
	}
	base.timeLimit[0], base.timeLimit[1] = int16(base.HostInfo.TimeLimit), int16(base.HostInfo.TimeLimit)

	base.Duel = ocgcore.NewDuelV2(rh.SeedSequence)
	base.Duel.InitPlayers(base.HostInfo.StartLp, int32(base.HostInfo.StartHand), int32(base.HostInfo.DrawCount))

	opt := uint32(base.HostInfo.DuelRule) << 16
	if base.HostInfo.NoShuffleDeck != 0 {
		opt |= ocgcore.DUEL_PSEUDO_SHUFFLE
	}
	opt |= m.extraDuelOpt()
	base.lastReplay.WriteInt32(base.HostInfo.StartLp, false)
	base.lastReplay.WriteInt32(int32(base.HostInfo.StartHand), false)
	base.lastReplay.WriteInt32(int32(base.HostInfo.DrawCount), false)
	base.lastReplay.WriteInt32(int32(opt), false)
	base.lastReplay.Flush()

	m.loadDecksToEngine(base.Duel, base.lastReplay)

	startBuf := make([]byte, 32)
	pBuf := utils.NewYGOBuffer(startBuf, binary.LittleEndian)
	pBuf.Write(
		uint8(ocgcore.MSG_START), uint8(0), uint8(base.HostInfo.DuelRule),
		base.HostInfo.StartLp, base.HostInfo.StartLp,
		uint16(base.Duel.QueryFieldCount(0, ocgcore.LOCATION_DECK)),
		uint16(base.Duel.QueryFieldCount(0, ocgcore.LOCATION_EXTRA)),
		uint16(base.Duel.QueryFieldCount(1, ocgcore.LOCATION_DECK)),
		uint16(base.Duel.QueryFieldCount(1, ocgcore.LOCATION_EXTRA)),
	)
	a, a2, b, b2 := m.handSeats()
	base.SendPacketDataToPlayer(a, network.STOC_GAME_MSG, startBuf[:19])
	base.ReSendToPlayer(a2)
	startBuf[1] = 1
	base.SendPacketDataToPlayer(b, network.STOC_GAME_MSG, startBuf[:19])
	base.ReSendToPlayer(b2)
	if !swapped {
		startBuf[1] = 0x10
	} else {
		startBuf[1] = 0x11
	}
	for _, v := range base.Observers {
		base.SendPacketDataToPlayer(v, network.STOC_GAME_MSG, startBuf[:19])
	}
	m.refreshOnDuelStart()
	base.Duel.Start(int32(opt))
	if base.HostInfo.TimeLimit != 0 {
		base.timeElapsed = 0
		m.armDuelTimer()
	}

	m.Process()
}
