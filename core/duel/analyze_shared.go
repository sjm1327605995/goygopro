package duel

import (
	"encoding/binary"
	"fmt"
	"log"

	"github.com/sjm1327605995/goygopro/core/codec"
	"github.com/sjm1327605995/goygopro/core/enginemsg"
	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// 本文件把 SingleDuel.Analyze / TagDuel.Analyze 的 MSG_* switch 表驱动化：
// 逐消息的字段跳过以 engineMsgLayouts（core/enginemsg/engine_msg_layout.go）为唯一事实源，
// 玩家路由差异通过 duelRoom 钩子吸收（currentPlayer / handMaskedRecipients /
// zoneFullPair / zoneMaskedPair / refresh* 系列），模式特有的消息
// （MSG_NEW_TURN、MSG_TAG_SWAP、MSG_MATCH_KILL 等）留在各模式的表里。
//
// 每个 handler 的返回值与原版 switch 的 case 语义一致：0 继续处理批次，
// 1 表示批次以交互提示结束（等待响应），2 表示游戏结束（随后走 DuelEndProc）。

type analyzeHandler func(room duelRoom, engType uint8, r, offset *utils.YGOBuffer) int

// runAnalyze 是 Analyze 的公共驱动循环。
func runAnalyze(room duelRoom, table map[uint8]analyzeHandler, msgBuffer []byte) int {
	r := utils.NewYGOBuffer(msgBuffer, binary.LittleEndian)

	var offset *utils.YGOBuffer
	for r.Len() > 0 {
		// 记录当前消息的起始位置，用于后续提取完整消息数据
		offset = r.Clone()

		var engType uint8
		if err := r.Read(&engType); err != nil {
			panic(err)
		}

		handler, ok := table[engType]
		if !ok {
			// 显式降级。原版 switch 无匹配 case 时只消费类型字节——若该
			// 消息实际带消息体，体字节会被误读成下一条消息的类型，批次静默
			// 错位。这里改为：
			//  - 布局表认识的消息（原版无 case 的保留号码，如 MSG_HAND_RES、
			//    MSG_REQUEST_DECK 等）：按布局安全跳过整条消息体并记日志；
			//  - 布局表也不认识的消息：无从得知消息体长度，无法安全跳过，
			//    记日志后 EndDuel 兜底（返回 2 走 DuelEndProc）。
			//  win 型消息（MSG_WIN）是终局消息，无 handler 时同样不能跳过继续。
			layout, known := enginemsg.EngineMsgLayouts[engType]
			if !known || layout.Win {
				log.Printf("[duel] analyze: unknown engine message 0x%02x (offset %d, batch %d bytes): no layout registered, cannot skip safely, duel aborted",
					engType, r.Offset()-1, len(msgBuffer))
				room.EndDuel()
				return 2
			}
			bodyStart := r.Offset()
			skipLayout(r, layout.Steps)
			if r.Overflowed() {
				log.Printf("[duel] analyze: corrupt engine message 0x%02x (offset %d, batch %d bytes): buffer overflow, duel aborted", engType, r.Offset(), len(msgBuffer))
				room.EndDuel()
				return 2
			}
			log.Printf("[duel] analyze: engine message 0x%02x not in this mode's analyze table: skipped %d body bytes per layout", engType, r.Offset()-bodyStart)
			continue
		}
		// 解析越界（截断/损坏的引擎消息批次）：handler 内被吞掉的读取失败
		// 会留在这里被一次性拦下——记日志（消息类型、越界偏移、批次长度）
		// 并终止本局（EndDuel 收尾回放/引擎，返回 2 走既有 DuelEndProc 通道），
		// 避免基于错位缓冲区继续广播损坏数据。
		if r.Overflowed() {
			log.Printf("[duel] analyze: corrupt engine message 0x%02x (offset %d, batch %d bytes): buffer overflow, duel aborted", engType, r.Offset(), len(msgBuffer))
			room.EndDuel()
			return 2
		}
		if ret := handler(room, engType, r, offset); ret != 0 {
			return ret
		}
		if r.Overflowed() {
			log.Printf("[duel] analyze: corrupt engine message 0x%02x (offset %d, batch %d bytes): buffer overflow, duel aborted", engType, r.Offset(), len(msgBuffer))
			room.EndDuel()
			return 2
		}
	}
	return 0
}

// analyzeSelectWait 是 select 系消息的公共收尾：等待玩家响应并把整条消息
// （offset.SubSlices(r)）发给当前操作者（single: players[player]；
// tag: curPlayer[player]）。
func analyzeSelectWait(room duelRoom, r, offset *utils.YGOBuffer, player uint8) int {
	waitForResponse(room, player)
	room.BaseMode().SendPacketDataToPlayer(room.currentPlayer(int(player)), network.STOC_GAME_MSG, offset.SubSlices(r))
	return 1
}

// mustReadMsg 从批次游标解析一个 codec 结构体并推进游标消费。解析失败
// panic（与既有 codec 化 handler 的 r.Unpack 失败处理一致）；截断/错位的
// 批次由 runAnalyze 的溢出检查兜底。消费字节数与 enginemsg.EngineMsgLayouts
// 布局表的一致性由 TestAnalyzeLayoutMatchesBatchWalker 锁定。
func mustReadMsg[T any](r *utils.YGOBuffer) *T {
	var msg T
	if err := r.Unpack(&msg); err != nil {
		panic(fmt.Sprintf("analyze 解码 %T 失败: %v", &msg, err))
	}
	return &msg
}

// skipLayout 按布局表跳过消息字段（与 handler 里逐字段 Read/Next 等价，
// 消费的字节数以 enginemsg.EngineMsgLayouts 为准）。仍用于：
//   - runAnalyze 对「布局表认识但本模式无 handler」消息的显式降级跳过；
//   - analyzeSkipBroadcast* 系列（原始字节整段转发，不做结构化解码）。
func skipLayout(r *utils.YGOBuffer, steps []enginemsg.LayStep) {
	for _, step := range steps {
		switch step.Kind {
		case enginemsg.LayFixed:
			r.Next(step.N)
		case enginemsg.LayCountList:
			var count uint8
			_ = r.Read(&count)
			r.Next(int(count) * step.N)
		case enginemsg.LayToss:
			var player, count uint8
			_ = r.Read(&player, &count)
			r.Next(int(count))
		case enginemsg.LayChain:
			var player, count uint8
			_ = r.Read(&player, &count)
			r.Next(9 + int(count)*14)
		default:
			// 其余布局仅出现于 Analyze 不会收到的消息（UPDATE_DATA 等），
			// 与原版 switch 未覆盖这些消息的行为一致：不消费。
		}
	}
}

// layoutOf 取消息的布局；Analyze 表中的每条消息都必须有布局登记
// （TestAnalyzeLayoutMatchesBatchWalker 强制）。
func layoutOf(engType uint8) enginemsg.EngineMsgLayout {
	return enginemsg.EngineMsgLayouts[engType]
}

// analyzeSkipBroadcast 返回「跳过布局字段 → 整段广播给所有玩家与观察者」的 handler。
// 保留 skipLayout 游标走查而非 codec 解码：转发的是 offset.SubSlices(r) 的
// 原始字节，handler 本身不消费字段语义，布局表（enginemsg.EngineMsgLayouts）
// 已是消费事实源；解码再重编码只会引入无收益的字节重排风险。
func analyzeSkipBroadcast(engType uint8) analyzeHandler {
	steps := layoutOf(engType).Steps
	return func(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
		skipLayout(r, steps)
		broadcastData(room, network.STOC_GAME_MSG, offset.SubSlices(r))
		return 0
	}
}

// analyzeSkipBroadcastRefresh 在广播后追加一次模式特有的区域刷新。
func analyzeSkipBroadcastRefresh(engType uint8, refresh func(duelRoom)) analyzeHandler {
	steps := layoutOf(engType).Steps
	return func(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
		skipLayout(r, steps)
		broadcastData(room, network.STOC_GAME_MSG, offset.SubSlices(r))
		refresh(room)
		return 0
	}
}

// analyzeBroadcastRefresh 是无字段跳过的广播+刷新（MSG_SUMMONED 等无体消息）。
func analyzeBroadcastRefresh(refresh func(duelRoom)) analyzeHandler {
	return func(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
		broadcastData(room, network.STOC_GAME_MSG, offset.SubSlices(r))
		refresh(room)
		return 0
	}
}

// ----- 共享 handler：路由复杂或含隐藏规则的消息 -----

// analyzeRetry 重试消息：让上次响应的玩家重新选择。
func analyzeRetry(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
	waitForResponse(room, room.BaseMode().lastResponse)
	room.BaseMode().SendPacketDataToPlayer(room.currentPlayer(int(room.BaseMode().lastResponse)), network.STOC_GAME_MSG, offset.SubSlices(r))
	return 1
}

// analyzeHint 提示消息：按提示类型路由（HINT_EVENT/MESSAGE/SELECTMSG/EFFECT
// 只发指定玩家；HINT_OPSELECTED/RACE/ATTRIB/CODE/NUMBER/ZONE 发其余玩家
// （single 为对手，tag 为除当前操作者外的所有玩家）+观察者；HINT_CARD 全员广播）。
func analyzeHint(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
	base := room.BaseMode()
	msg := mustReadMsg[protocol.HintMsg](r)
	switch msg.Type {
	case ocgcore.HINT_EVENT, ocgcore.HINT_MESSAGE, ocgcore.HINT_SELECTMSG, ocgcore.HINT_EFFECT:
		base.SendPacketDataToPlayer(room.currentPlayer(int(msg.Player)), network.STOC_GAME_MSG, offset.SubSlices(r))
	case ocgcore.HINT_OPSELECTED, ocgcore.HINT_RACE, ocgcore.HINT_ATTRIB, ocgcore.HINT_CODE, ocgcore.HINT_NUMBER, ocgcore.HINT_ZONE:
		for _, recipient := range room.waitingNotifyRecipients(msg.Player) {
			base.SendPacketDataToPlayer(recipient, network.STOC_GAME_MSG, offset.SubSlices(r))
		}
		base.ReSendToObservers()
	case ocgcore.HINT_CARD:
		broadcastData(room, network.STOC_GAME_MSG, offset.SubSlices(r))
	}
	return 0
}

// analyzeWin 胜利消息：广播结果、记账（single 的 match 逻辑）、结束决斗。
func analyzeWin(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
	msg := mustReadMsg[protocol.WinMsg](r)
	broadcastData(room, network.STOC_GAME_MSG, offset.SubSlices(r))
	room.onEngineWin(msg.Player)
	room.EndDuel()
	return 2
}

// ----- 共享 handler：select 系（读字段 → 等待响应 → 发给当前操作者）-----
// 全部经 mustReadMsg 一次 codec 解码，消费与 enginemsg.EngineMsgLayouts
// 逐消息一致（TestAnalyzeLayoutMatchesBatchWalker 锁定）；bookmark 转发
// 保持 offset.SubSlices(r) 原始字节不变。

func analyzeSelectBattlecmd(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
	msg := mustReadMsg[protocol.SelectBattleCmdMsg](r)
	return analyzeSelectWait(room, r, offset, msg.Player)
}

func analyzeSelectIdlecmd(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
	msg := mustReadMsg[protocol.SelectIdleCmdMsg](r)
	return analyzeSelectWait(room, r, offset, msg.Player)
}

func analyzeSelectEffectyn(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
	msg := mustReadMsg[protocol.SelectEffectYNMsg](r)
	return analyzeSelectWait(room, r, offset, msg.Player)
}

// packEngineMsg 组装 [类型字节|结构体负载] 的引擎消息，一次分配
// （WireSize 预算 + make(1+n) + MarshalInto）。内部走 codec 计划链（零反射），
// 替代旧的 append + restruct 反射打包。打包失败 panic：这些路径在
// RecoverMiddleware 链内，且属于静态布局 bug（由 codec_wire_test.go 锁定）。
func packEngineMsg(engType uint8, msg any) []byte {
	size, err := codec.WireSize(msg)
	if err != nil {
		panic(fmt.Sprintf("packEngineMsg(%T): %v", msg, err))
	}
	out := make([]byte, 1+size)
	out[0] = engType
	if _, err := codec.MarshalInto(out[1:], msg); err != nil {
		panic(fmt.Sprintf("packEngineMsg(%T): %v", msg, err))
	}
	return out
}

func analyzeSelectYesno(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
	msg := mustReadMsg[protocol.SelectYesNoMsg](r)
	return analyzeSelectWait(room, r, offset, msg.Player)
}

func analyzeSelectOption(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
	msg := mustReadMsg[protocol.SelectOptionMsg](r)
	return analyzeSelectWait(room, r, offset, msg.Player)
}

func analyzeSelectCard(room duelRoom, engType uint8, r *utils.YGOBuffer) int {
	sel := mustReadMsg[protocol.SelectCardMsg](r)
	sel.HideCodesForPlayer(sel.Player)
	data := packEngineMsg(engType, sel)
	waitForResponse(room, sel.Player)
	room.BaseMode().SendPacketDataToPlayer(room.currentPlayer(int(sel.Player)), network.STOC_GAME_MSG, data)
	return 1
}

func analyzeSelectUnselectCard(room duelRoom, engType uint8, r *utils.YGOBuffer) int {
	sel := mustReadMsg[protocol.SelectUnselectCardMsg](r)
	sel.HideCodesForPlayer(sel.Player)
	data := packEngineMsg(engType, sel)
	waitForResponse(room, sel.Player)
	room.BaseMode().SendPacketDataToPlayer(room.currentPlayer(int(sel.Player)), network.STOC_GAME_MSG, data)
	return 1
}

func analyzeSelectChain(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
	msg := mustReadMsg[protocol.SelectChainMsg](r)
	return analyzeSelectWait(room, r, offset, msg.Player)
}

// analyzeSelectPlace 服务 MSG_SELECT_PLACE / MSG_SELECT_DISFIELD /
// MSG_SELECT_POSITION：三者线宽同为 6 字节，按消息类型选对应结构体解码。
func analyzeSelectPlace(room duelRoom, engType uint8, r, offset *utils.YGOBuffer) int {
	var player uint8
	if engType == ocgcore.MSG_SELECT_POSITION {
		player = mustReadMsg[protocol.SelectPositionMsg](r).Player
	} else {
		player = mustReadMsg[protocol.SelectPlaceMsg](r).Player
	}
	return analyzeSelectWait(room, r, offset, player)
}

func analyzeSelectCounter(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
	msg := mustReadMsg[protocol.SelectCounterMsg](r)
	return analyzeSelectWait(room, r, offset, msg.Player)
}

func analyzeSelectSum(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
	// 布局以 single_duel 的解析为准（sumMode, player, acc+min+max 6 字节，
	// 两组 count+count*11），与 enginemsg.EngineMsgLayouts / message_scan
	// 一致；tag 旧实现把 player 读在偏移 0，此处统一修正。
	msg := mustReadMsg[protocol.SelectSumMsg](r)
	return analyzeSelectWait(room, r, offset, msg.Player)
}

func analyzeSortCard(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
	msg := mustReadMsg[protocol.SortCardMsg](r)
	return analyzeSelectWait(room, r, offset, msg.Player)
}

func analyzeRockPaperScissors(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
	// C++ 把该消息首字节直接当 player 路由（single_duel.cpp:1343、
	// tag_duel.cpp:1416），protocol 结构体中此字节命名为 Phase，按原版语义取用。
	msg := mustReadMsg[protocol.RockPaperScissorsMsg](r)
	return analyzeSelectWait(room, r, offset, msg.Phase)
}

func analyzeAnnounceRace(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
	msg := mustReadMsg[protocol.AnnounceRaceMsg](r)
	return analyzeSelectWait(room, r, offset, msg.Player)
}

func analyzeAnnounceCard(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
	msg := mustReadMsg[protocol.AnnounceCardMsg](r)
	return analyzeSelectWait(room, r, offset, msg.Player)
}

// ----- 共享 handler：带隐藏规则的路由 -----

// analyzeConfirmCards 卡片确认：卡组位置只发给当前操作者，其余位置全员广播。
// 判定位点是首个条目的 location 字节（体偏移 5），与原版 single/tag 一致；
// 先按原版偏移预读再做 codec 解码，count=0 时预读落到后续字节的行为也与
// 原版相同。
func analyzeConfirmCards(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
	base := room.BaseMode()
	firstLoc := r.At(5)
	msg := mustReadMsg[protocol.ConfirmCardsMsg](r)
	if firstLoc != ocgcore.LOCATION_DECK {
		broadcastData(room, network.STOC_GAME_MSG, offset.SubSlices(r))
		return 0
	}
	base.SendPacketDataToPlayer(room.currentPlayer(int(msg.Player)), network.STOC_GAME_MSG, offset.SubSlices(r))
	return 0
}

// analyzeShuffleHand 手牌洗牌：当前操作者收到完整 codes，其余玩家收到抹零版本
// （single 仅对手；tag 为除当前操作者外的所有玩家），然后整区刷新。
func analyzeShuffleHand(room duelRoom, engType uint8, r *utils.YGOBuffer) int {
	base := room.BaseMode()
	shuffle := mustReadMsg[protocol.ShuffleHandMsg](r)
	data := packEngineMsg(engType, shuffle)
	base.SendPacketDataToPlayer(room.currentPlayer(int(shuffle.Player)), network.STOC_GAME_MSG, data)
	shuffle.HideAllCodes()
	data = packEngineMsg(engType, shuffle)
	for _, recipient := range room.handMaskedRecipients(int(shuffle.Player)) {
		base.SendPacketDataToPlayer(recipient, network.STOC_GAME_MSG, data)
	}
	base.ReSendToObservers()
	room.RefreshHand(int(shuffle.Player), queryFlagShuffleHand, 0)
	return 0
}

// analyzeShuffleExtra 额外卡组洗牌：与 analyzeShuffleHand 同构。
func analyzeShuffleExtra(room duelRoom, engType uint8, r *utils.YGOBuffer) int {
	base := room.BaseMode()
	shuffle := mustReadMsg[protocol.ShuffleExtraMsg](r)
	data := packEngineMsg(engType, shuffle)
	base.SendPacketDataToPlayer(room.currentPlayer(int(shuffle.Player)), network.STOC_GAME_MSG, data)
	shuffle.HideAllCodes()
	data = packEngineMsg(engType, shuffle)
	for _, recipient := range room.handMaskedRecipients(int(shuffle.Player)) {
		base.SendPacketDataToPlayer(recipient, network.STOC_GAME_MSG, data)
	}
	base.ReSendToObservers()
	// 两模式的 MSG_SHUFFLE_EXTRA 都调裸 RefreshExtra(player) 默认参数
	// queryFlagExtra / use_cache=1（single_duel.cpp:863、tag_duel.cpp:834）。
	room.RefreshExtra(int(shuffle.Player), queryFlagExtra, 1)
	return 0
}

// analyzeSwapGraveDeck 墓地卡组交换：广播后按模式规则刷新墓地。
func analyzeSwapGraveDeck(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
	msg := mustReadMsg[protocol.SwapGraveDeckMsg](r)
	broadcastData(room, network.STOC_GAME_MSG, offset.SubSlices(r))
	room.refreshGraveAfterSwap(int(msg.Player))
	return 0
}

// analyzeShuffleSetCard 场上设置卡洗牌：按位置类型刷新相应区域。
func analyzeShuffleSetCard(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
	msg := mustReadMsg[protocol.ShuffleSetCardMsg](r)
	broadcastData(room, network.STOC_GAME_MSG, offset.SubSlices(r))
	refreshZone := room.RefreshSzone
	if msg.Loc == ocgcore.LOCATION_MZONE {
		refreshZone = room.RefreshMzone
	}
	refreshZone(0, queryFlagShuffleZone, 0)
	refreshZone(1, queryFlagShuffleZone, 0)
	return 0
}

// analyzeMove 卡片移动：控制者收到完整信息，隐藏位置的卡对对手抹码，随后单卡刷新。
func analyzeMove(room duelRoom, engType uint8, r *utils.YGOBuffer) int {
	base := room.BaseMode()
	mv := mustReadMsg[protocol.MoveMsg](r)
	data := packEngineMsg(engType, mv)
	base.SendPacketDataToPlayer(room.currentPlayer(int(mv.CC)), network.STOC_GAME_MSG, data)
	if (mv.CL&(ocgcore.LOCATION_GRAVE+ocgcore.LOCATION_OVERLAY)) == 0 &&
		((mv.CL&(ocgcore.LOCATION_DECK+ocgcore.LOCATION_HAND)) != 0 || mv.CP&ocgcore.POS_FACEDOWN != 0) {
		mv.Code = 0
	}
	data = packEngineMsg(engType, mv)
	for _, recipient := range room.handMaskedRecipients(int(mv.CC)) {
		base.SendPacketDataToPlayer(recipient, network.STOC_GAME_MSG, data)
	}
	base.ReSendToObservers()
	if mv.CL != 0 && (mv.CL&ocgcore.LOCATION_OVERLAY) == 0 && (mv.CL != mv.PL || mv.PC != mv.CC) {
		room.refreshSingleMoved(mv.CC, mv.CL, mv.CS)
	}
	return 0
}

// analyzePosChange 表示形式变更：里侧翻表侧时刷新该卡。
func analyzePosChange(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
	msg := mustReadMsg[protocol.PosChangeMsg](r)
	broadcastData(room, network.STOC_GAME_MSG, offset.SubSlices(r))
	if (msg.PP&ocgcore.POS_FACEDOWN != 0) && (msg.CP&ocgcore.POS_FACEUP != 0) {
		room.refreshSingleMoved(msg.CC, msg.CL, msg.CS)
	}
	return 0
}

// analyzeSet 设置卡片：卡码抹零后全员广播。
func analyzeSet(room duelRoom, engType uint8, r *utils.YGOBuffer) int {
	setMsg := mustReadMsg[protocol.SetMsg](r)
	setMsg.Code = 0
	data := packEngineMsg(engType, setMsg)
	broadcastData(room, network.STOC_GAME_MSG, data)
	return 0
}

// analyzeSwap 两张卡交换位置：广播后分别刷新两张卡。
func analyzeSwap(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
	msg := mustReadMsg[protocol.SwapMsg](r)
	broadcastData(room, network.STOC_GAME_MSG, offset.SubSlices(r))
	room.refreshSingleMoved(msg.CC1, msg.CL1, msg.CS1)
	room.refreshSingleMoved(msg.CC2, msg.CL2, msg.CS2)
	return 0
}

// analyzeSpsummoning 特殊召唤中：己方队伍（single 为本人）收完整信息，
// 里侧表示的卡在发给对方队伍前抹码。
func analyzeSpsummoning(room duelRoom, engType uint8, r *utils.YGOBuffer) int {
	base := room.BaseMode()
	sp := mustReadMsg[protocol.SPSummoningMsg](r)
	data := packEngineMsg(engType, sp)
	f1, f2 := room.zoneFullPair(int(sp.CC))
	base.SendPacketDataToPlayer(f1, network.STOC_GAME_MSG, data)
	base.ReSendToPlayer(f2)
	if sp.CP&ocgcore.POS_FACEDOWN != 0 {
		sp.Code = 0
	}
	data = packEngineMsg(engType, sp)
	h1, h2 := room.zoneMaskedPair(int(sp.CC))
	base.SendPacketDataToPlayer(h1, network.STOC_GAME_MSG, data)
	base.ReSendToPlayer(h2)
	base.ReSendToObservers()
	return 0
}

// analyzeFlipSummoning 反转召唤中：先刷新该卡（此时已知表示形式），再广播。
func analyzeFlipSummoning(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
	msg := mustReadMsg[protocol.SummoningMsg](r)
	room.refreshSingleFlip(msg.CC, msg.CL, msg.CS)
	broadcastData(room, network.STOC_GAME_MSG, offset.SubSlices(r))
	return 0
}

// analyzeCardSelected 卡片选择结果：只消费字段，不转发。
func analyzeCardSelected(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
	_ = mustReadMsg[protocol.CardSelectedMsg](r)
	return 0
}

// analyzeRandomSelected 随机选择结果：发给 players[player] 并重发给 1 号起的
// 所有座位（与原版 single/tag 一致：含 player 本人所在队伍的非代表座位）。
func analyzeRandomSelected(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
	base := room.BaseMode()
	msg := mustReadMsg[protocol.RandomSelectedMsg](r)
	players := room.allPlayers()
	base.SendPacketDataToPlayer(players[msg.Player], network.STOC_GAME_MSG, offset.SubSlices(r))
	base.ReSendToPlayers(players[1:]...)
	base.ReSendToObservers()
	return 0
}

// analyzeDraw 抽卡：当前操作者收到完整 codes，未带公开标记的卡对其他人抹零。
func analyzeDraw(room duelRoom, engType uint8, r *utils.YGOBuffer) int {
	base := room.BaseMode()
	draw := mustReadMsg[protocol.DrawMsg](r)
	data := packEngineMsg(engType, draw)
	base.SendPacketDataToPlayer(room.currentPlayer(int(draw.Player)), network.STOC_GAME_MSG, data)
	draw.HideUnknownCards()
	data = packEngineMsg(engType, draw)
	for _, recipient := range room.handMaskedRecipients(int(draw.Player)) {
		base.SendPacketDataToPlayer(recipient, network.STOC_GAME_MSG, data)
	}
	base.ReSendToObservers()
	return 0
}

// analyzeMissedEffect 错过时点：只发给当前操作者。原版的 player 取
// InfoLocation 低字节（get_info_location 的 controller 字段）。
func analyzeMissedEffect(room duelRoom, _ uint8, r, offset *utils.YGOBuffer) int {
	msg := mustReadMsg[protocol.MissedEffectMsg](r)
	player := uint8(msg.InfoLocation)
	room.BaseMode().SendPacketDataToPlayer(room.currentPlayer(int(player)), network.STOC_GAME_MSG, offset.SubSlices(r))
	return 0
}

// ----- 模式特有的 handler（留在各模式文件中实现） -----

// sharedAnalyzeHandlers 是两个模式共用的消息处理表。只收录两边行为一致
// （或由钩子吸收差异）的消息；模式特有消息在各模式的表构建函数里追加。
var sharedAnalyzeHandlers = map[uint8]analyzeHandler{
	ocgcore.MSG_RETRY:     analyzeRetry,
	ocgcore.MSG_HINT:      analyzeHint,
	ocgcore.MSG_WIN:       analyzeWin,
	ocgcore.MSG_NEW_PHASE: analyzeSkipBroadcastRefresh(ocgcore.MSG_NEW_PHASE, func(room duelRoom) { room.refreshAfterNewPhase() }),

	ocgcore.MSG_SELECT_BATTLECMD: analyzeSelectBattlecmd,
	ocgcore.MSG_SELECT_IDLECMD:   analyzeSelectIdlecmd,
	ocgcore.MSG_SELECT_EFFECTYN:  analyzeSelectEffectyn,
	ocgcore.MSG_SELECT_YESNO:     analyzeSelectYesno,
	ocgcore.MSG_SELECT_OPTION:    analyzeSelectOption,
	ocgcore.MSG_SELECT_CARD: func(room duelRoom, engType uint8, r, _ *utils.YGOBuffer) int {
		return analyzeSelectCard(room, engType, r)
	},
	ocgcore.MSG_SELECT_TRIBUTE: func(room duelRoom, engType uint8, r, _ *utils.YGOBuffer) int {
		return analyzeSelectCard(room, engType, r)
	},
	ocgcore.MSG_SELECT_UNSELECT_CARD: func(room duelRoom, engType uint8, r, _ *utils.YGOBuffer) int {
		return analyzeSelectUnselectCard(room, engType, r)
	},
	ocgcore.MSG_SELECT_CHAIN:        analyzeSelectChain,
	ocgcore.MSG_SELECT_PLACE:        analyzeSelectPlace,
	ocgcore.MSG_SELECT_DISFIELD:     analyzeSelectPlace,
	ocgcore.MSG_SELECT_POSITION:     analyzeSelectPlace,
	ocgcore.MSG_SELECT_COUNTER:      analyzeSelectCounter,
	ocgcore.MSG_SELECT_SUM:          analyzeSelectSum,
	ocgcore.MSG_SORT_CARD:           analyzeSortCard,
	ocgcore.MSG_ROCK_PAPER_SCISSORS: analyzeRockPaperScissors,
	ocgcore.MSG_ANNOUNCE_RACE:       analyzeAnnounceRace,
	ocgcore.MSG_ANNOUNCE_ATTRIB:     analyzeAnnounceRace,
	ocgcore.MSG_ANNOUNCE_CARD:       analyzeAnnounceCard,
	ocgcore.MSG_ANNOUNCE_NUMBER:     analyzeAnnounceCard,

	ocgcore.MSG_CONFIRM_DECKTOP:  analyzeSkipBroadcast(ocgcore.MSG_CONFIRM_DECKTOP),
	ocgcore.MSG_CONFIRM_EXTRATOP: analyzeSkipBroadcast(ocgcore.MSG_CONFIRM_EXTRATOP),
	ocgcore.MSG_CONFIRM_CARDS:    analyzeConfirmCards,
	ocgcore.MSG_SHUFFLE_DECK:     analyzeSkipBroadcast(ocgcore.MSG_SHUFFLE_DECK),
	ocgcore.MSG_SHUFFLE_HAND: func(room duelRoom, engType uint8, r, _ *utils.YGOBuffer) int {
		return analyzeShuffleHand(room, engType, r)
	},
	ocgcore.MSG_SHUFFLE_EXTRA: func(room duelRoom, engType uint8, r, _ *utils.YGOBuffer) int {
		return analyzeShuffleExtra(room, engType, r)
	},
	ocgcore.MSG_REFRESH_DECK:     analyzeSkipBroadcast(ocgcore.MSG_REFRESH_DECK),
	ocgcore.MSG_SWAP_GRAVE_DECK:  analyzeSwapGraveDeck,
	ocgcore.MSG_REVERSE_DECK:     analyzeSkipBroadcast(ocgcore.MSG_REVERSE_DECK),
	ocgcore.MSG_DECK_TOP:         analyzeSkipBroadcast(ocgcore.MSG_DECK_TOP),
	ocgcore.MSG_SHUFFLE_SET_CARD: analyzeShuffleSetCard,
	ocgcore.MSG_MOVE:             func(room duelRoom, engType uint8, r, _ *utils.YGOBuffer) int { return analyzeMove(room, engType, r) },
	ocgcore.MSG_POS_CHANGE:       analyzePosChange,
	ocgcore.MSG_SET:              func(room duelRoom, engType uint8, r, _ *utils.YGOBuffer) int { return analyzeSet(room, engType, r) },
	ocgcore.MSG_SWAP:             analyzeSwap,
	ocgcore.MSG_FIELD_DISABLED:   analyzeSkipBroadcast(ocgcore.MSG_FIELD_DISABLED),
	ocgcore.MSG_SUMMONING:        analyzeSkipBroadcast(ocgcore.MSG_SUMMONING),
	ocgcore.MSG_SUMMONED:         analyzeBroadcastRefresh(func(room duelRoom) { room.refreshAfterSummon() }),
	ocgcore.MSG_SPSUMMONING: func(room duelRoom, engType uint8, r, _ *utils.YGOBuffer) int {
		return analyzeSpsummoning(room, engType, r)
	},
	ocgcore.MSG_SPSUMMONED:      analyzeBroadcastRefresh(func(room duelRoom) { room.refreshAfterSummon() }),
	ocgcore.MSG_FLIPSUMMONING:   analyzeFlipSummoning,
	ocgcore.MSG_FLIPSUMMONED:    analyzeBroadcastRefresh(func(room duelRoom) { room.refreshAfterSummon() }),
	ocgcore.MSG_CHAINING:        analyzeSkipBroadcast(ocgcore.MSG_CHAINING),
	ocgcore.MSG_CHAINED:         analyzeSkipBroadcastRefresh(ocgcore.MSG_CHAINED, func(room duelRoom) { room.refreshAfterChain() }),
	ocgcore.MSG_CHAIN_SOLVING:   analyzeSkipBroadcast(ocgcore.MSG_CHAIN_SOLVING),
	ocgcore.MSG_CHAIN_SOLVED:    analyzeSkipBroadcastRefresh(ocgcore.MSG_CHAIN_SOLVED, func(room duelRoom) { room.refreshAfterChain() }),
	ocgcore.MSG_CHAIN_END:       analyzeBroadcastRefresh(func(room duelRoom) { room.refreshAfterChain() }),
	ocgcore.MSG_CHAIN_NEGATED:   analyzeSkipBroadcast(ocgcore.MSG_CHAIN_NEGATED),
	ocgcore.MSG_CHAIN_DISABLED:  analyzeSkipBroadcast(ocgcore.MSG_CHAIN_DISABLED),
	ocgcore.MSG_CARD_SELECTED:   analyzeCardSelected,
	ocgcore.MSG_RANDOM_SELECTED: analyzeRandomSelected,
	ocgcore.MSG_BECOME_TARGET:   analyzeSkipBroadcast(ocgcore.MSG_BECOME_TARGET),
	ocgcore.MSG_DRAW:            func(room duelRoom, engType uint8, r, _ *utils.YGOBuffer) int { return analyzeDraw(room, engType, r) },
	ocgcore.MSG_DAMAGE:          analyzeSkipBroadcast(ocgcore.MSG_DAMAGE),
	ocgcore.MSG_RECOVER:         analyzeSkipBroadcast(ocgcore.MSG_RECOVER),
	ocgcore.MSG_EQUIP:           analyzeSkipBroadcast(ocgcore.MSG_EQUIP),
	ocgcore.MSG_LPUPDATE:        analyzeSkipBroadcast(ocgcore.MSG_LPUPDATE),
	ocgcore.MSG_UNEQUIP:         analyzeSkipBroadcast(ocgcore.MSG_UNEQUIP),
	ocgcore.MSG_CARD_TARGET:     analyzeSkipBroadcast(ocgcore.MSG_CARD_TARGET),
	ocgcore.MSG_CANCEL_TARGET:   analyzeSkipBroadcast(ocgcore.MSG_CANCEL_TARGET),
	ocgcore.MSG_PAY_LPCOST:      analyzeSkipBroadcast(ocgcore.MSG_PAY_LPCOST),
	ocgcore.MSG_ADD_COUNTER:     analyzeSkipBroadcast(ocgcore.MSG_ADD_COUNTER),
	ocgcore.MSG_REMOVE_COUNTER:  analyzeSkipBroadcast(ocgcore.MSG_REMOVE_COUNTER),
	ocgcore.MSG_ATTACK:          analyzeSkipBroadcast(ocgcore.MSG_ATTACK),
	ocgcore.MSG_BATTLE:          analyzeSkipBroadcast(ocgcore.MSG_BATTLE),
	ocgcore.MSG_ATTACK_DISABLED: analyzeSkipBroadcast(ocgcore.MSG_ATTACK_DISABLED),
	ocgcore.MSG_DAMAGE_STEP_START: analyzeSkipBroadcastRefresh(ocgcore.MSG_DAMAGE_STEP_START, func(room duelRoom) {
		room.refreshAfterDamageStep()
	}),
	ocgcore.MSG_DAMAGE_STEP_END: analyzeSkipBroadcastRefresh(ocgcore.MSG_DAMAGE_STEP_END, func(room duelRoom) {
		room.refreshAfterDamageStep()
	}),
	ocgcore.MSG_MISSED_EFFECT: analyzeMissedEffect,
	ocgcore.MSG_TOSS_COIN:     analyzeSkipBroadcast(ocgcore.MSG_TOSS_COIN),
	ocgcore.MSG_TOSS_DICE:     analyzeSkipBroadcast(ocgcore.MSG_TOSS_DICE),
	ocgcore.MSG_CARD_HINT:     analyzeSkipBroadcast(ocgcore.MSG_CARD_HINT),
	ocgcore.MSG_PLAYER_HINT:   analyzeSkipBroadcast(ocgcore.MSG_PLAYER_HINT),
}

// copyAnalyzeTable 复制共享表并追加/覆盖模式特有条目。
func copyAnalyzeTable(extra map[uint8]analyzeHandler) map[uint8]analyzeHandler {
	merged := make(map[uint8]analyzeHandler, len(sharedAnalyzeHandlers)+len(extra))
	for msgID, handler := range sharedAnalyzeHandlers {
		merged[msgID] = handler
	}
	for msgID, handler := range extra {
		merged[msgID] = handler
	}
	return merged
}
