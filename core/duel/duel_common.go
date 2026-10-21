package duel

import (
	"bytes"
	"encoding/binary"
	"log"
	"math/rand"
	"time"

	"github.com/duke-git/lancet/v2/condition"
	"github.com/sjm1327605995/goygopro/core/codec"
	"github.com/sjm1327605995/goygopro/core/data"
	"github.com/sjm1327605995/goygopro/core/replay"
	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// 线协议魔数的具名常量（行为与原字面量完全一致，出处为原版 ygopro）。
const (
	// playerChangePlayerShift 是 STOC_HS_PLAYER_CHANGE Status 高半字节（玩家
	// 下标）的位移：原版 (dp->type << 4) | PLAYERCHANGE_*
	// （single_duel.cpp PlayerReady/ToObserver，tag_duel.cpp:109/140/209/237/248）。
	playerChangePlayerShift = 4
	// deckErrorKindShift 是 ERRMSG_DECKERROR Code 高 4 位（错误类型）的位移：
	// 原版 (DECKERROR_UNKNOWNCARD << 28) | deck_error[dp->type]
	// （tag_duel.cpp:230，single_duel.cpp 同式）。
	deckErrorKindShift = 28
	// typeChangeHostFlag 是 STOC_TYPE_CHANGE 类型字节的房主标志：
	// 原版 (host_player == dp) ? 0x10 : 0
	// （single_duel.cpp:63，tag_duel.cpp:67/182/198/220）。
	typeChangeHostFlag = 0x10
	// playerStateIdle 是「不参与 CTOS 状态机」的玩家状态占位：
	// 原版猜拳决出先后攻后把等待方 state 置 0xff
	// （single_duel.cpp HandResult，tag_duel.cpp:350/355/1525）。
	playerStateIdle = 0xff
	// playerTypeUnset 是「不在任何座位」的玩家类型占位：
	// 原版 JOIN_GAME 校验 dp->type != 0xff 判断是否已在游戏中
	// （netserver.cpp / tag_duel.cpp:31 的 JoinGame 预检）。
	playerTypeUnset = 0xff
	// duelRuleOptShift 是引擎 opt 中 DuelRule 字段的位移：
	// 原版 opt = (unsigned)host_info.duel_rule << 16
	// （single_duel.cpp / tag_duel.cpp:414 TPResult）。
	duelRuleOptShift = 16
	// msgStartObserverPlayerBase 是旁观者收到的 MSG_START 负载视角字节基准：
	// 原版 swapped ? 0x11 : 0x10（tag_duel.cpp:463-464 TPResult；
	// single_duel.cpp TPResult 同式）。
	msgStartObserverPlayerBase    = 0x10
	msgStartObserverPlayerSwapped = 0x11
	// deckDataHeaderLen 是 CTOS_UPDATE_DECK 载荷头部（mainc/sidec 两个 int32）
	// 的字节数，对应 C++ struct CTOS_DeckData 前两个字段（protocol/types.go）。
	deckDataHeaderLen = 8
	// deckDataMaxPayload 是 CTOS_UPDATE_DECK 载荷上限：
	// deckDataHeaderLen + (MAINC_MAX+SIDEC_MAX)*4 = 2008，
	// 与原版 UpdateDeck 的 len > (int)sizeof(CTOS_DeckData) 一致
	// （MAINC_MAX = SIDEC_MAX = 250）。
	deckDataMaxPayload = deckDataHeaderLen + (protocol.MAINC_MAX+protocol.SIDEC_MAX)*4
	// replayPlayerNameBytes 是回放头中单个玩家名的字节数：
	// 20 个宽字符 × 2 字节（C++ replay.h 的 NAME_SIZE 20，TPResult 逐名写入）。
	replayPlayerNameBytes = 40
)

// deckCountTeam 是一支队伍的卡组三区张数（STOC_DECK_COUNT 的半包，6 字节）。
//go:generate codecgen
type deckCountTeam struct {
	Main  int16
	Extra int16
	Side  int16
}

// deckCountBody 是 STOC_DECK_COUNT 负载（12 字节）：host 队 + guest 队计数。
// 对应原版 StartDuel 的 12 字节 pbuf 缓冲与半包 memcpy 交换。
type deckCountBody struct {
	Host  deckCountTeam
	Guest deckCountTeam
}

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
	// （single/tag 均为否——原版 single_duel.cpp:1451-1452 只发 players[0]/[1]，
	// tag_duel 同样不发观战者）。
	timeLimitToObservers() bool
	// isResponder 报告 player 是否是当前等待响应的玩家（TimeConfirm 用；
	// single: player.Type == lastResponse；tag: player == curPlayer[lastResponse]）。
	isResponder(player *DuelPlayer) bool
	// firstFreeSeat 返回第一个空座位下标（调用前必须确保存在空位）。
	firstFreeSeat() int
	// assignSeat / removeFromSeat 把玩家放入/移出指定座位。
	assignSeat(pos int, player *DuelPlayer)
	removeFromSeat(seat int)
	// handResultIndex 把座位号映射为 handResult 下标（single: 即座位号；tag: 0 号位 → 0，其余 → 1）。
	handResultIndex(seat uint8) int
	// handSeats 返回猜拳双方座位：host 为先手代表、guest 为对方代表，
	// hostPartner/guestPartner 为各自队友（single 为 nil，对应原版不向第二人重发）。
	handSeats() (host, hostPartner, guest, guestPartner *DuelPlayer)
	// recordTpPlayer 在猜拳决出先后攻时记录将要选择先攻的一方
	// （single: 写 tpPlayer，bWins=true 记 1 否则记 0；tag: 无此概念）。
	recordTpPlayer(bWins bool)
	// onJoinPassDenied 是 JoinGame 密码错误后的模式特有处理（single: 断开连接；tag: 不断开）。
	onJoinPassDenied(player *DuelPlayer)
	// onDuelEnded 在 EndDuel 公共流程结束后执行的模式特有清理
	// （single: 无；tag: 把所有玩家 State 置 0xff）。
	onDuelEnded()
	// leaveAsPlayer 处理决斗者（非房主、非观战者）的离开逻辑（single/tag 差异较大）。
	leaveAsPlayer(player *DuelPlayer)
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
	// （两模式均为原 RefreshSingle 默认 0xf81fff）。
	refreshSingleMoved(cc, cl, cs uint8)
	// refreshSingleFlip 是 MSG_FLIPSUMMONING 前置的单卡刷新
	// （两模式均为 0xf81fff）。
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
	loadDecksToEngine(duel *ocgcore.Duel, rp *replay.Replay)
	// refreshOnDuelStart 是 TPResult 末尾、Duel.Start 之前的开局额外卡组刷新
	// （single: RefreshExtraDef(0)/(1)；tag: RefreshExtra(0/1, 0x81fff4, 0)）。
	refreshOnDuelStart()
	// armDuelTimer 在限时开启时武装决斗秒表（single: 每秒 SingleTimer；
	// tag: 每秒 TagTimer——两种模式的回调都在非超时路径每秒自行重新武装，
	// 与原版 SingleTimer/TagTimer 一致，此处只做首次武装）。
	armDuelTimer()
}

// broadcastData 把 proto+data 发送给所有玩家与观察者：首个玩家用
// SendPacketDataToPlayer 编码进 buff，其余玩家与观察者用 ReSendToPlayer
// 复用同一 buff。与各模式里重复出现的发送三连（Send + ReSend + 观察者循环）
// 字节序完全一致。
func broadcastData(room duelRoom, proto byte, data any) {
	base := room.BaseMode()
	players := room.allPlayers()
	base.SendPacketDataToPlayer(players[0], proto, data)
	for _, otherPlayer := range players[1:] {
		base.ReSendToPlayer(otherPlayer)
	}
	base.ReSendToObservers()
}

// broadcastProto 是 broadcastData 的无数据版本（仅 proto 字节）。
func broadcastProto(room duelRoom, proto byte) {
	base := room.BaseMode()
	players := room.allPlayers()
	base.SendPacketToPlayer(players[0], proto)
	for _, otherPlayer := range players[1:] {
		base.ReSendToPlayer(otherPlayer)
	}
	base.ReSendToObservers()
}

// waitForResponse 是 WaitForResponse 的公共实现：记录 lastResponse，
// 向 waitingNotifyRecipients（single 为对手，tag 为除当前操作者外的所有玩家）
// 发送 MSG_WAITING；限时开启时广播 STOC_TIME_LIMIT 并把被等待玩家状态置为
// CTOS_TIME_CONFIRM，否则直接置为 CTOS_RESPONSE。
func waitForResponse(room duelRoom, player byte) {
	base := room.BaseMode()
	base.lastResponse = player
	msgWaiting := []byte{ocgcore.MSG_WAITING}
	for _, recipient := range room.waitingNotifyRecipients(player) {
		base.SendPacketDataToPlayer(recipient, network.STOC_GAME_MSG, msgWaiting)
	}
	if base.HostInfo.TimeLimit != 0 {
		var timeLimitPkt protocol.STOCTimeLimit
		timeLimitPkt.Player = player
		timeLimitPkt.LeftTime = uint16(base.timeLimit[player])
		players := room.allPlayers()
		base.SendPacketDataToPlayer(players[0], network.STOC_TIME_LIMIT, timeLimitPkt)
		for _, otherPlayer := range players[1:] {
			base.ReSendToPlayer(otherPlayer)
		}
		if room.timeLimitToObservers() {
			base.ReSendToObservers()
		}
		room.currentPlayer(int(player)).State = network.CTOS_TIME_CONFIRM
	} else {
		room.currentPlayer(int(player)).State = network.CTOS_RESPONSE
	}
}

// timeConfirm 是 TimeConfirm 的公共实现：限时开启且 player 正是被等待响应的玩家时，
// 把其状态推进到 CTOS_RESPONSE，并清零 10 秒以内的计时时长。
func timeConfirm(room duelRoom, player *DuelPlayer) {
	base := room.BaseMode()
	if base.HostInfo.TimeLimit == 0 {
		return
	}
	if !room.isResponder(player) {
		return
	}
	room.currentPlayer(int(base.lastResponse)).State = network.CTOS_RESPONSE
	if base.timeElapsed < 10 {
		base.timeElapsed = 0
	}
}

// recordResponseAndSet 把客户端响应写入回放并提交给引擎：
// 回放记录 [长度字节, 响应字节]，引擎收到定长 SIZE_RETURN_VALUE 缓冲区。
// 两个模式的逐字节行为一致（包括 msgBuffer 超长时的表现），此处合并。
func recordResponseAndSet(mode *DuelMode, msgBuffer []byte) {
	engineResp := make([]byte, ocgcore.SIZE_RETURN_VALUE)
	copy(engineResp, msgBuffer)
	mode.lastReplay.WriteData([]byte{uint8(len(msgBuffer))}, false)
	mode.lastReplay.WriteData(engineResp[:len(msgBuffer)], false)
	mode.Duel.SetResponseBytes(engineResp)
}

// endDuel 是 EndDuel 的公共实现：回放收尾并广播 STOC_REPLAY，结束引擎决斗、
// 停掉计时器；之后执行 onDuelEnded 钩子（tag 借此把所有玩家 State 置 0xff）。
func endDuel(room duelRoom) {
	base := room.BaseMode()
	if base.Duel == nil {
		return
	}
	base.lastReplay.EndRecord()
	// 回放包 = 扩展头（80 字节）+ 压缩数据（compSize 上限 MAX_COMP_SIZE）。
	// 旧缓冲只有 0x2000，长对局的回放会被 YGOBuffer.Write 静默截断（错误
	// 被忽略），客户端拿到的 STOC_REPLAY 不完整。
	replayBuf := base.lastReplay.BroadcastPayload()
	broadcastData(room, network.STOC_REPLAY, replayBuf)
	base.Duel.End()
	if base.ETimer != nil {
		base.ETimer.Stop()
		base.ETimer = nil
	}
	base.Duel = nil
	room.onDuelEnded()
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
func processDuel(room duelRoom) {
	base := room.BaseMode()
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
			stop = room.Analyze(buff[:engLen])
		}
	}
	if stop == 2 {
		room.DuelEndProc()
	}
}

// duelEndProcSimple 是非 match 模式的 DuelEndProc：广播 STOC_DUEL_END 并置阶段为 END。
// （tag 的 DuelEndProc 即此实现；single 的 match 分支另有自己的局数逻辑。）
func duelEndProcSimple(room duelRoom) {
	broadcastProto(room, network.STOC_DUEL_END)
	room.BaseMode().DuelStage = network.DUEL_STAGE_END
}

// duelChat 是 Chat 的公共实现：构造聊天包并广播给所有玩家与观察者。
func duelChat(room duelRoom, player *DuelPlayer, payload []byte) {
	base := room.BaseMode()
	var buff bytes.Buffer
	chatSize := base.CreateChatPacket(payload, &buff, uint16(player.Type))
	if chatSize == 0 {
		return
	}
	broadcastData(room, network.STOC_CHAT, buff.Bytes())
}

// sendDataToAll 把 data 序列化一次进房间共享缓冲，扇出给全部玩家与观察者
// （编码一次 + ReSend 复用，替代按接收者循环重复序列化）。空座位（nil）由
// sendFrame 天然跳过。对同一批接收者连发多个不同 proto 的包时，保持「一个
// proto 发完再发下一个」——各连接上包序不变（跨连接交错顺序无协议语义）。
func sendDataToAll(room duelRoom, proto byte, data any) {
	base := room.BaseMode()
	players := room.allPlayers()
	if len(players) > 0 {
		base.SendPacketDataToPlayer(players[0], proto, data)
		for _, otherPlayer := range players[1:] {
			base.ReSendToPlayer(otherPlayer)
		}
	}
	base.ReSendToObservers()
}

// sendHsWatchChangeAll 广播观战人数变化（STOC_HS_WATCH_CHANGE）给所有玩家与观察者。
func sendHsWatchChangeAll(room duelRoom) {
	base := room.BaseMode()
	var watchChangePkt protocol.STOCHsWatchChange
	watchChangePkt.WatchCount = uint16(len(base.Observers))
	sendDataToAll(room, network.STOC_HS_WATCH_CHANGE, watchChangePkt)
}

// sendHsPlayerChangeAll 向所有玩家与观察者发送大厅玩家状态变化（STOC_HS_PLAYER_CHANGE）。
func sendHsPlayerChangeAll(room duelRoom, playerChangePkt protocol.STOCHsPlayerChange) {
	sendDataToAll(room, network.STOC_HS_PLAYER_CHANGE, playerChangePkt)
}

// typeChangeType 计算 STOC_TYPE_CHANGE 的类型字节：房主加 0x10 标志，或上当前 Type。
func typeChangeType(room duelRoom, player *DuelPlayer) uint8 {
	typeByte := condition.Ternary[bool, uint8](player == room.BaseMode().HostPlayer, typeChangeHostFlag, 0)
	return typeByte | player.Type
}

// playerReady 是 PlayerReady 的公共实现：校验卡组（含禁卡表与缓存的未知卡错误），
//
//	deckError 非零时告知本人并退回 NOTREADY；否则更新 ready 状态并向全房间广播。
func playerReady(room duelRoom, player *DuelPlayer, isReady bool) {
	base := room.BaseMode()
	if int(player.Type) > len(room.allPlayers())-1 || base.ready[player.Type] == isReady {
		return
	}
	if isReady {
		var deckError uint32
		if base.HostInfo.NoCheckDeck == 0 {
			if base.DeckError[player.Type] != 0 {
				deckError = network.DECKERROR_UNKNOWNCARD<<deckErrorKindShift | base.DeckError[player.Type]
			} else {
				// HS_READY 可能先于 UPDATE_DECK 到达（IPC 乱序），pDeck 尚未填充。
				// 原版 pDeck 是值类型、空主卡组走 DECKERROR_MAINCOUNT 拒绝；
				// 这里 nil 指针必须按空卡组处理，不能直接传给 CheckDeck（nil 解引用 panic）。
				deck := base.pDeck[player.Type]
				if deck == nil {
					deck = &data.Deck{}
				}
				deckError = data.DeckManager.CheckDeck(deck, base.HostInfo.LFList, int(base.HostInfo.Rule))
			}
		}
		if deckError != 0 {
			var playerChangePkt protocol.STOCHsPlayerChange
			playerChangePkt.Status = player.Type<<playerChangePlayerShift | network.PLAYERCHANGE_NOTREADY
			base.SendPacketDataToPlayer(player, network.STOC_HS_PLAYER_CHANGE, &playerChangePkt)
			var errMsgPkt protocol.STOCErrorMsg
			errMsgPkt.Msg = network.ERRMSG_DECKERROR
			errMsgPkt.Code = deckError
			base.SendPacketDataToPlayer(player, network.STOC_ERROR_MSG, &errMsgPkt)
			return
		}
	}
	base.ready[player.Type] = isReady
	var playerChangePkt protocol.STOCHsPlayerChange
	playerChangePkt.Status = player.Type<<playerChangePlayerShift | condition.Ternary[bool, byte](isReady, network.PLAYERCHANGE_READY, network.PLAYERCHANGE_NOTREADY)
	sendHsPlayerChangeAll(room, playerChangePkt)
}

// leaveGame 是 LeaveGame 的公共部分：房主离开直接结束房间；观战者离开更新观战人数；
// 决斗者离开交给 leaveAsPlayer（各模式差异较大，单独实现）。
func leaveGame(room duelRoom, player *DuelPlayer) {
	base := room.BaseMode()
	if player == base.HostPlayer {
		room.EndDuel()
		// 先摘除房间再停服务器：StopServer 会阻塞等待 gnet 事件循环退出，
		// 而 leaveGame 正是在事件循环回调（OnClose/CTOS_LEAVE_GAME）里执行的，
		// 同步调用必然死锁（服务器此后不再处理任何包，其他玩家看到僵尸房间）。
		// 异步停服：本回调先返回，事件循环随即由 Stop 安全收起。
		DefaultManager.RemoveRoom(base.RoomID)
		go base.StopServer()
		return
	}
	if player.Type == network.NETPLAYER_TYPE_OBSERVER {
		delete(base.Observers, player.ID)
		if base.DuelStage == network.DUEL_STAGE_BEGIN {
			sendHsWatchChangeAll(room)
		}
		_ = base.DisconnectPlayer(player)
		return
	}
	room.leaveAsPlayer(player)
}

// toObserver 是 ToObserver 的公共实现：广播状态变化、把玩家从座位移到观战列表。
func toObserver(room duelRoom, player *DuelPlayer) {
	base := room.BaseMode()
	if int(player.Type) > len(room.allPlayers())-1 {
		return
	}
	var playerChangePkt protocol.STOCHsPlayerChange
	playerChangePkt.Status = player.Type<<playerChangePlayerShift | network.PLAYERCHANGE_OBSERVE
	sendHsPlayerChangeAll(room, playerChangePkt)
	room.removeFromSeat(int(player.Type))
	base.ready[player.Type] = false
	player.Type = network.NETPLAYER_TYPE_OBSERVER
	base.Observers[player.ID] = player
	base.SendPacketDataToPlayer(player, network.STOC_TYPE_CHANGE, protocol.STOCTypeChange{Type: typeChangeType(room, player)})
}

// toDuelListObserver 把观战者提升为决斗者（single 的 ToDuelList 全部逻辑、tag 的
// 观战者分支）。调用前必须保证 player 是观战者且尚有空座位。
func toDuelListObserver(room duelRoom, player *DuelPlayer) {
	base := room.BaseMode()
	delete(base.Observers, player.ID)
	var playerEnterPkt protocol.STOCHsPlayerEnter
	copy(playerEnterPkt.Name[:], player.Name[:])
	pos := room.firstFreeSeat()
	player.Type = uint8(pos)
	room.assignSeat(pos, player)
	playerEnterPkt.Pos = uint8(pos)
	var watchChangePkt protocol.STOCHsWatchChange
	watchChangePkt.WatchCount = uint16(len(base.Observers))
	sendDataToAll(room, network.STOC_HS_PLAYER_ENTER, playerEnterPkt)
	sendDataToAll(room, network.STOC_HS_WATCH_CHANGE, watchChangePkt)
	base.SendPacketDataToPlayer(player, network.STOC_TYPE_CHANGE, protocol.STOCTypeChange{Type: typeChangeType(room, player)})
}

// playerKick 是 PlayerKick 的公共实现：仅房主可踢、不能踢自己、目标必须在场。
// 调用方（CTOS_HS_KICK 路由）已持房间锁。
func playerKick(room duelRoom, player *DuelPlayer, pos byte) {
	base := room.BaseMode()
	players := room.allPlayers()
	if int(pos) > len(players)-1 || player != base.HostPlayer || player == players[pos] || players[pos] == nil {
		return
	}
	victim := players[pos]
	// 与 leaveGameOnce 同语义：先清 Game 引用再执行离开。被踢者的连接随即
	// 被 DisconnectPlayer 关闭，gnet 触发 OnClose → leaveGameOnce，后者在
	// 锁内二次确认 d.Game 时会发现已被清而直接返回——否则 LeaveGame 会
	// 执行两遍（leaveGame 只经由 leaveGameOnce 清 Game，本路径是唯一例外
	// 入口），重复广播 PLAYERCHANGE_LEAVE。
	victim.Game = nil
	room.LeaveGame(victim)
}

// rpsBeats 报告猜拳手势 hostGesture 是否克 guestGesture（1=石头 2=剪刀 3=布）。
// 循环克制关系用取模表达：(host-guest+3)%3==2 恰覆盖 石头克剪刀/剪刀克布/布克石头
// 三种组合，与原版 single_duel.cpp HandResult 的三重或式逐情况等价。
func rpsBeats(hostGesture, guestGesture uint8) bool {
	return (int(hostGesture)-int(guestGesture)+3)%3 == 2
}

// enterSelectTp 把先攻选择权交给 chooser（发 STOC_SELECT_TP），另一方
// 置 0xff 等待，房间推进到 DUEL_STAGE_FIRSTGO。
func enterSelectTp(base *DuelMode, chooser, waiter *DuelPlayer) {
	base.SendPacketToPlayer(chooser, network.STOC_SELECT_TP)
	waiter.State = playerStateIdle
	chooser.State = network.CTOS_TP_RESULT
	base.DuelStage = network.DUEL_STAGE_FIRSTGO
}

// handResult 是 HandResult 的公共实现：收集双方猜拳结果，平局重猜，否则
// 进入先攻选择（原版把选择权发给 RPS 负方一侧，见 single_duel.cpp:380-390）。
// 并发敏感（持房间锁期间按序广播），此处只做纯改名，语句顺序不可调整。
func handResult(room duelRoom, player *DuelPlayer, gesture byte) {
	base := room.BaseMode()
	if gesture > 3 || player.State != network.CTOS_HAND_RESULT {
		return
	}
	base.handResult[room.handResultIndex(player.Type)] = gesture
	if base.handResult[0] == 0 || base.handResult[1] == 0 {
		return
	}

	host, hostPartner, guest, guestPartner := room.handSeats()
	hostGesture, guestGesture := base.handResult[0], base.handResult[1]

	// 双方按各自视角收结果（己方手势在前）；旁观者跟随 host 侧看一份。
	sendResult := func(recipient, partner *DuelPlayer, mine, theirs uint8) {
		pkt := protocol.STOCHandResult{Res1: mine, Res2: theirs}
		base.SendPacketDataToPlayer(recipient, network.STOC_HAND_RESULT, pkt)
		base.ReSendToPlayer(partner)
	}
	sendResult(host, hostPartner, hostGesture, guestGesture)
	base.ReSendToObservers()
	sendResult(guest, guestPartner, guestGesture, hostGesture)

	switch {
	case hostGesture == guestGesture:
		// 平局：清零重猜，双方回到等待手势状态。
		base.SendPacketToPlayer(host, network.STOC_SELECT_HAND)
		base.ReSendToPlayer(guest)
		base.handResult[0], base.handResult[1] = 0, 0
		host.State = network.CTOS_HAND_RESULT
		guest.State = network.CTOS_HAND_RESULT
	case rpsBeats(hostGesture, guestGesture):
		enterSelectTp(base, guest, host)
		room.recordTpPlayer(true)
	default:
		enterSelectTp(base, host, guest)
		room.recordTpPlayer(false)
	}
}

// unpackDeckData 校验并解包 CTOS_UPDATE_DECK 载荷。长度与解包失败静默返回
// （与原版一致，仅记日志）；内容不合法时发送 ERRMSG_DECKERROR 后返回 ok=false。
func unpackDeckData(base *DuelMode, player *DuelPlayer, payload []byte) (protocol.CTOSDeckData, bool) {
	length := len(payload)
	if length < deckDataHeaderLen || length > deckDataMaxPayload {
		return protocol.CTOSDeckData{}, false
	}
	var deckBuf protocol.CTOSDeckData
	// CTOSDeckData 的 List 长度 = MainC + SideC（两个计数之和），codec 的
	// Count 标记表达不了，走它自己的手写的 Unpack（解包失败会导致卡组被
	// 静默丢弃——随后 StartDuel 解引用 nil 的 pDeck 直接 panic）。
	if _, err := deckBuf.Unpack(payload, binary.LittleEndian); err != nil {
		log.Printf("[duel] unpack CTOS_UPDATE_DECK: %v", err)
		return protocol.CTOSDeckData{}, false
	}
	// (2+mainc+sidec)*4：2 个 int32 计数 + 每张卡一个 uint32 卡码。
	if deckBuf.MainC < 0 || deckBuf.MainC > protocol.MAINC_MAX ||
		deckBuf.SideC < 0 || deckBuf.SideC > protocol.SIDEC_MAX ||
		int32(length) < (2+deckBuf.MainC+deckBuf.SideC)*4 {
		base.SendPacketDataToPlayer(player, network.STOC_ERROR_MSG, &protocol.STOCErrorMsg{Msg: network.ERRMSG_DECKERROR, Code: 0})
		return protocol.CTOSDeckData{}, false
	}
	return deckBuf, true
}

// checkJoinAllowed 执行 JoinGame 的三项进入校验（已在游戏/版本/密码），
// 不通过时发送错误包并按模式特有方式处理连接后返回 false。
func checkJoinAllowed(room duelRoom, player *DuelPlayer, pkt *protocol.CTOSJoinGame, isCreator bool) bool {
	base := room.BaseMode()
	if isCreator {
		return true
	}
	if player.Game != nil && player.Type != playerTypeUnset {
		utils.NetLogf("server", "JOIN_GAME rejected: player=%q already in game (type=%d)", utils.WideString(player.Name[:]), player.Type)
		base.SendPacketDataToPlayer(player, network.STOC_ERROR_MSG, &protocol.STOCErrorMsg{Msg: network.ERRMSG_JOINERROR})
		_ = base.DisconnectPlayer(player)
		return false
	}
	if pkt.Version != PRO_VERSION {
		utils.NetLogf("server", "JOIN_GAME rejected: player=%q version mismatch (got 0x%04x, want 0x%04x)",
			utils.WideString(player.Name[:]), pkt.Version, PRO_VERSION)
		base.SendPacketDataToPlayer(player, network.STOC_ERROR_MSG, &protocol.STOCErrorMsg{Msg: network.ERRMSG_VERERROR, Code: PRO_VERSION})
		_ = base.DisconnectPlayer(player)
		return false
	}
	var joinPass [20]uint16
	utils.NullTerminate(pkt.Pass[:], 0)
	copy(joinPass[:], pkt.Pass[:])
	if utils.Wcscmp(joinPass[:], base.Pass[:]) != 0 {
		utils.NetLogf("server", "JOIN_GAME rejected: player=%q wrong password (got %q, room %q)",
			utils.WideString(player.Name[:]), utils.WideString(joinPass[:]), utils.WideString(base.Pass[:]))
		base.SendPacketDataToPlayer(player, network.STOC_ERROR_MSG, &protocol.STOCErrorMsg{Msg: network.ERRMSG_JOINERROR, Code: 1})
		room.onJoinPassDenied(player)
		return false
	}
	return true
}

// startDuelCommon 是 StartDuel 的公共实现（对应原版 single_duel.cpp /
// tag_duel.cpp 的 StartDuel 逐字节一致段）：校验房主与全部座位就绪、停止监听、
// 广播 STOC_DUEL_START（观察者同时被置为离开状态）、按猜拳队伍分组发送
// STOC_DECK_COUNT——先手队（handSeats 的 host/hostPartner）收前 6 字节、后手队
// （guest/guestPartner）收交换后的后 6 字节（single 的 hostPartner/guestPartner
// 为 nil，ReSendToPlayer(nil) 为空操作，与原版 single 只发两人完全一致）——
// 最后广播 STOC_SELECT_HAND 进入猜拳。
func startDuelCommon(room duelRoom, player *DuelPlayer) {
	base := room.BaseMode()
	if player != base.HostPlayer {
		return
	}
	for i := 0; i < room.seatCount(); i++ {
		if !base.ready[i] {
			return
		}
		// Go 侧 pDeck 是指针数组、初值 nil；NoCheckDeck 开启时玩家可以不
		// 传卡组直接 ready（原版 pdeck 是值成员，空卡组天然可用），这里
		// 兜底为空卡组，避免下方 DECK_COUNT / beginDuel 解引用 nil。
		if base.pDeck[i] == nil {
			base.pDeck[i] = &data.Deck{}
		}
	}
	base.StopListen()
	players := room.allPlayers()
	base.SendPacketToPlayer(players[0], network.STOC_DUEL_START)
	for _, otherPlayer := range players[1:] {
		base.ReSendToPlayer(otherPlayer)
	}
	for _, observer := range base.Observers {
		observer.State = network.CTOS_LEAVE_GAME
	}
	base.ReSendToObservers()
	host, hostPartner, guest, guestPartner := room.handSeats()
	// 双方卡组张数（main/extra/side 各 int16）。guest 视角即两队计数互换，
	// 直接在结构体层交换，省去半包字节搬移。
	hostTeam := deckCountTeam{
		int16(len(base.pDeck[host.Type].Main)), int16(len(base.pDeck[host.Type].Extra)), int16(len(base.pDeck[host.Type].Side)),
	}
	guestTeam := deckCountTeam{
		int16(len(base.pDeck[guest.Type].Main)), int16(len(base.pDeck[guest.Type].Extra)), int16(len(base.pDeck[guest.Type].Side)),
	}
	base.SendPacketDataToPlayer(host, network.STOC_DECK_COUNT, &deckCountBody{Host: hostTeam, Guest: guestTeam})
	base.ReSendToPlayer(hostPartner)
	base.SendPacketDataToPlayer(guest, network.STOC_DECK_COUNT, &deckCountBody{Host: guestTeam, Guest: hostTeam})
	base.ReSendToPlayer(guestPartner)
	base.SendPacketToPlayer(host, network.STOC_SELECT_HAND)
	base.ReSendToPlayer(guest)
	base.handResult[0] = 0
	base.handResult[1] = 0
	host.State = network.CTOS_HAND_RESULT
	guest.State = network.CTOS_HAND_RESULT
	base.DuelStage = network.DUEL_STAGE_FINGER
}

// msgStartSide 是 MSG_START 中一位玩家的区域张数（卡组 + 额外）。
type msgStartSide struct {
	Deck  uint16
	Extra uint16
}

// msgStartBody 是引擎 MSG_START 消息的负载（定长 19 字节，对应原版
// StartDuel 的手写缓冲）。三个接收组（host 侧 / guest 侧 / 旁观者）只差 Player
// 视角字节：对战双方各收 0/1，旁观者收 0x10/0x11（取决于 swapped）。
type msgStartBody struct {
	Type     uint8 // 恒为 ocgcore.MSG_START
	Player   uint8 // 视角玩家
	DuelRule uint8
	LP       [2]int32
	Side     [2]msgStartSide
}

// msgStartPlayerOffset 是 MSG_START 负载的 Player 字节在房间共享帧缓冲中的
// 偏移：3 字节帧头 + 1 字节消息类型。
const msgStartPlayerOffset = 4

// buildReplayHeader 组装回放扩展头（对应原版 TPResult 的 rh 填充段）：
// 标志取模式 replayFlag，HeaderVersion 对齐 C++ 类内初始化 header_version{1}
// （replay.h:39），种子序列与时间戳按原版顺序取自全局 rand（回放字节序敏感，
// 调用次序不可调整）。
func buildReplayHeader(room duelRoom, seed uint32) replay.ExtendedReplayHeader {
	rh := replay.ExtendedReplayHeader{}
	rh.Base.ID = replay.REPLAY_ID_YRP2
	rh.Base.Version = PRO_VERSION
	rh.Base.Flag = room.replayFlag()
	rh.Base.Seed = seed
	rh.HeaderVersion = 1
	for i := 0; i < replay.SEED_COUNT; i++ {
		rh.SeedSequence[i] = rand.Uint32()
	}
	rh.Base.StartTime = uint32(time.Now().Unix())
	return rh
}

// writeReplayPlayerNames 按座位顺序把全部玩家名写入回放（每名字节数见
// replayPlayerNameBytes，对应原版 TPResult 的逐名 WriteData）。
func writeReplayPlayerNames(base *DuelMode, players []*DuelPlayer) {
	for _, player := range players {
		name := make([]byte, replayPlayerNameBytes)
		for j := 0; j < 20; j++ {
			binary.LittleEndian.PutUint16(name[j*2:], player.Name[j])
		}
		base.lastReplay.WriteData(name, false)
	}
}

// shuffleMainDecksIfEnabled 在 NoShuffleDeck 未开启时按座位顺序洗牌各主卡组
// （原版 TPResult 的 shuffle 段；rnd 的种子与回放头同源）。
func shuffleMainDecksIfEnabled(base *DuelMode, room duelRoom, rnd *rand.Rand) {
	if base.HostInfo.NoShuffleDeck != 0 {
		return
	}
	for seat := 0; seat < room.seatCount(); seat++ {
		main := base.pDeck[seat].Main
		rnd.Shuffle(len(main), func(x, y int) {
			main[x], main[y] = main[y], main[x]
		})
	}
}

// beginDuel 是 TPResult 的公共实现（对应原版两个 TPResult 逐字节一致段）：
// 置回应状态、生成种子与回放头（标志取 replayFlag）、按座位顺序写玩家名、
// 洗牌（NoShuffleDeck 时跳过）、初始化引擎与开局参数（opt = DuelRule<<16 |
// PSEUDO_SHUFFLE? | extraDuelOpt）、装载卡组（loadDecksToEngine）、广播
// MSG_START（host/hostPartner 收己方视角 0/1，guest/guestPartner 收对方视角，
// 观察者收 0x10/0x11 取决于 swapped）、开局刷新（refreshOnDuelStart）、
// Duel.Start、限时武装（armDuelTimer），最后驱动引擎直至出现交互提示。
// swapped 由各模式的换位逻辑得出，仅影响观察者收到的 MSG_START 标志字节。
func beginDuel(room duelRoom, player *DuelPlayer, swapped bool) {
	base := room.BaseMode()
	player.State = network.CTOS_RESPONSE
	seed := rand.Uint32()

	rnd := rand.New(rand.NewSource(int64(seed)))
	rh := buildReplayHeader(room, seed)
	base.lastReplay = replay.NewReplay()
	base.lastReplay.BeginRecord()
	base.lastReplay.WriteHeader(rh)
	writeReplayPlayerNames(base, room.allPlayers())
	shuffleMainDecksIfEnabled(base, room, rnd)
	base.timeLimit[0], base.timeLimit[1] = int16(base.HostInfo.TimeLimit), int16(base.HostInfo.TimeLimit)

	base.Duel = ocgcore.NewDuelV2(rh.SeedSequence)
	base.Duel.InitPlayers(base.HostInfo.StartLp, int32(base.HostInfo.StartHand), int32(base.HostInfo.DrawCount))

	opt := uint32(base.HostInfo.DuelRule) << duelRuleOptShift
	if base.HostInfo.NoShuffleDeck != 0 {
		opt |= ocgcore.DUEL_PSEUDO_SHUFFLE
	}
	opt |= room.extraDuelOpt()
	// 录制参数块与回放读取端同源（replay.DuelParameters，16 字节 LE），
	// 布局由 TestDuelParametersGolden / TestReplaySaveAndOpen 锁定
	base.lastReplay.WriteData(codec.MustFreeze(&replay.DuelParameters{
		StartLP:   base.HostInfo.StartLp,
		StartHand: int32(base.HostInfo.StartHand),
		DrawCount: int32(base.HostInfo.DrawCount),
		DuelFlag:  opt,
	}).Bytes(), false)
	base.lastReplay.Flush()

	room.loadDecksToEngine(base.Duel, base.lastReplay)

	body := msgStartBody{
		Type:     ocgcore.MSG_START,
		DuelRule: base.HostInfo.DuelRule,
		LP:       [2]int32{base.HostInfo.StartLp, base.HostInfo.StartLp},
		Side: [2]msgStartSide{
			{
				uint16(base.Duel.QueryFieldCount(0, ocgcore.LOCATION_DECK)),
				uint16(base.Duel.QueryFieldCount(0, ocgcore.LOCATION_EXTRA)),
			},
			{
				uint16(base.Duel.QueryFieldCount(1, ocgcore.LOCATION_DECK)),
				uint16(base.Duel.QueryFieldCount(1, ocgcore.LOCATION_EXTRA)),
			},
		},
	}
	host, hostPartner, guest, guestPartner := room.handSeats()
	// 三个接收组仅视角字节不同：编码一次进共享缓冲（body.Player=0），
	// 之后只改写该字节再按组扇出。
	base.SendPacketDataToPlayer(host, network.STOC_GAME_MSG, &body)
	base.ReSendToPlayers(hostPartner)
	base.buff[msgStartPlayerOffset] = 1
	base.ReSendToPlayers(guest, guestPartner)
	base.buff[msgStartPlayerOffset] = condition.Ternary[bool, uint8](swapped, msgStartObserverPlayerSwapped, msgStartObserverPlayerBase)
	base.ReSendToObservers()
	room.refreshOnDuelStart()
	base.Duel.Start(int32(opt))
	if base.HostInfo.TimeLimit != 0 {
		base.timeElapsed = 0
		room.armDuelTimer()
	}

	room.Process()
}
