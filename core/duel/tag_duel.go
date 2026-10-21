package duel

import (
	"encoding/binary"
	"slices"
	"time"

	"github.com/sjm1327605995/goygopro/core/data"
	"github.com/sjm1327605995/goygopro/core/replay"
	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// 卡查询数据第 4 字节的「公开」标记：原版 tag_duel.cpp:1484/1490 的
// pbufw[3] & 0x80，未公开的卡码就地抹零后才转发给其他玩家。
const cardCodePublicFlag = 0x80

// 需要向其余座位重发刷新数据的区域掩码：原版 tag_duel.cpp:1707 的字面量
// 0x90，即 LOCATION_GRAVE|LOCATION_OVERLAY（ocgcore common.h:51/54）。
const locationPublicResendMask = 0x90

func newTagDuel() *TagDuel {
	td := &TagDuel{DuelMode: DuelMode{
		Observers: make(map[string]*DuelPlayer),
		// 刷新参数表与 single 完全一致（C++ tag_duel.h:35-40 与
		// single_duel.h:35-40 的默认参数相同），见 defaultRefreshFlags。
		refresh: defaultRefreshFlags(),
	}}
	td.room = td
	return td
}

type TagDuel struct {
	DuelMode
	players   [4]*DuelPlayer
	pplayer   [4]*DuelPlayer
	curPlayer [2]*DuelPlayer
	surrender [4]bool
	turnCount uint8
}

// JoinGame 的全部逻辑在 joinGameCommon：tag 与 single 的 JoinGame 除座位数
// 外逐字节一致（对照原版 tag_duel.cpp:38-112 / single_duel.cpp:25-100），
// 已用 duelRoom 钩子（firstFreeSeat/assignSeat/seatCount/allPlayers）泛化。
func (tag *TagDuel) JoinGame(player *DuelPlayer, pkt *protocol.CTOSJoinGame, isCreator bool) {
	joinGameCommon(tag, player, pkt, isCreator, "tag")
}

func (tag *TagDuel) leaveAsPlayer(player *DuelPlayer) {
	if tag.DuelStage == network.DUEL_STAGE_BEGIN {
		var playerChangePkt protocol.STOCHsPlayerChange
		tag.players[player.Type] = nil
		tag.ready[player.Type] = false
		playerChangePkt.Status = uint8(player.Type<<4) | network.PLAYERCHANGE_LEAVE
		sendHsPlayerChangeAll(tag, playerChangePkt)
	} else if tag.DuelStage != network.DUEL_STAGE_END {
		tag.EndDuel()
		tag.DuelEndProc()
	}
	tag.DisconnectPlayer(player)
}

func (tag *TagDuel) ToDuelList(player *DuelPlayer) {
	for _, seat := range tag.players {
		if seat != nil {
			return
		}
	}
	if player.Type == network.NETPLAYER_TYPE_OBSERVER {
		toDuelListObserver(tag, player)
		return
	}
	// 以下为本模式特有的换座逻辑（观战者之外的大厅玩家移到下一个空位）。
	if tag.ready[player.Type] {
		return
	}
	nextSeat := (player.Type + 1) % uint8(tag.seatCount())
	for tag.players[nextSeat] != nil {
		nextSeat = (nextSeat + 1) % uint8(tag.seatCount())
	}
	var playerChangePkt protocol.STOCHsPlayerChange
	playerChangePkt.Status = uint8(player.Type<<4) | uint8(nextSeat)
	for i := 0; i < tag.seatCount(); i++ {
		if tag.players[i] != nil {
			tag.SendPacketDataToPlayer(tag.players[i], network.STOC_HS_PLAYER_CHANGE, playerChangePkt)
		}
	}
	for _, observer := range tag.Observers {
		tag.SendPacketDataToPlayer(observer, network.STOC_HS_PLAYER_CHANGE, playerChangePkt)
	}
	var typeChangePkt protocol.STOCTypeChange
	typeChangePkt.Type = 0
	if player == tag.HostPlayer {
		typeChangePkt.Type = 0x10
	}
	typeChangePkt.Type |= uint8(nextSeat)
	tag.SendPacketDataToPlayer(player, network.STOC_TYPE_CHANGE, typeChangePkt)
	tag.players[player.Type] = nil
	tag.players[nextSeat] = player
	player.Type = nextSeat
}

func (tag *TagDuel) UpdateDeck(player *DuelPlayer, pData []byte) {
	deckBuf, ok := updateDeckSlot(tag, player, pData)
	if !ok {
		return
	}
	tag.DeckError[player.Type] = data.DeckManager.LoadDeck(tag.pDeck[player.Type], deckBuf.List[:], deckBuf.MainC, deckBuf.SideC, false)
}

func (tag *TagDuel) StartDuel(player *DuelPlayer) {
	startDuelCommon(tag, player)
}

func (tag *TagDuel) TPResult(player *DuelPlayer, tp byte) {
	if player.State != network.CTOS_TP_RESULT {
		return
	}
	tag.DuelStage = network.DUEL_STAGE_DUELING
	var swapped bool
	tag.pplayer[0] = tag.players[0]
	tag.pplayer[1] = tag.players[1]
	tag.pplayer[2] = tag.players[2]
	tag.pplayer[3] = tag.players[3]
	if (tp != 0 && player.Type == 2) || (tp == 0 && player.Type == 0) {
		tag.players[0], tag.players[2] = tag.players[2], tag.players[0]
		tag.players[1], tag.players[3] = tag.players[3], tag.players[1]
		tag.players[0].Type, tag.players[1].Type, tag.players[2].Type, tag.players[3].Type = 0, 1, 2, 3
		tag.pDeck[0], tag.pDeck[2] = tag.pDeck[2], tag.pDeck[0]
		tag.pDeck[1], tag.pDeck[3] = tag.pDeck[3], tag.pDeck[1]
		swapped = true
	}
	tag.turnCount = 0
	tag.curPlayer[0] = tag.players[0]
	tag.curPlayer[1] = tag.players[3]
	beginDuel(tag, player, swapped)
}

func (tag *TagDuel) DuelEndProc() {
	duelEndProcSimple(tag)
}

func (tag *TagDuel) Surrender(surrenderer *DuelPlayer) {
	if surrenderer.Type > 3 || tag.Duel == nil {
		return
	}
	player := surrenderer.Type
	if tag.surrender[player] {
		return
	}
	teammateMap := [4]uint8{1, 0, 3, 2}
	teammate := teammateMap[player]
	if !tag.surrender[teammate] {
		tag.surrender[player] = true
		tag.SendPacketToPlayer(tag.players[player], network.STOC_TEAMMATE_SURRENDER)
		tag.SendPacketToPlayer(tag.players[teammate], network.STOC_TEAMMATE_SURRENDER)
		return
	}
	winPlayerMap := [4]uint8{1, 1, 0, 0}
	var wbuf [3]byte
	wbuf[0] = ocgcore.MSG_WIN
	wbuf[1] = winPlayerMap[player]
	wbuf[2] = 0
	broadcastData(tag, network.STOC_GAME_MSG, wbuf[:])
	tag.EndDuel()
	tag.DuelEndProc()
	if tag.ETimer != nil {
		tag.ETimer.Stop()
	}
}

// tagAnalyzeTable 在共享表之上追加 tag 特有的消息
// （MSG_TAG_SWAP 为 tag 独有；MSG_NEW_TURN / MSG_MATCH_KILL 两模式行为不同）。
var tagAnalyzeTable = copyAnalyzeTable(map[uint8]analyzeHandler{
	ocgcore.MSG_NEW_TURN: func(room duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
		return room.(*TagDuel).analyzeNewTurn(pbuf, offset)
	},
	ocgcore.MSG_TAG_SWAP: func(room duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
		return room.(*TagDuel).analyzeTagSwap(pbuf, offset)
	},
	ocgcore.MSG_MATCH_KILL: func(_ duelRoom, _ uint8, pbuf, _ *utils.YGOBuffer) int {
		_ = mustReadMsg[protocol.MatchKillMsg](pbuf)
		return 0
	},
})

func (tag *TagDuel) Analyze(msgBuffer []byte) int {
	return runAnalyze(tag, tagAnalyzeTable, msgBuffer)
}

// analyzeNewTurn 新回合开始：广播、重置时间限制，并按回合数奇偶轮换
// 双方的当前操作者（curPlayer）。
func (tag *TagDuel) analyzeNewTurn(pbuf, offset *utils.YGOBuffer) int {
	_ = mustReadMsg[protocol.NewTurnMsg](pbuf)
	tag.timeLimit[0] = int16(tag.HostInfo.TimeLimit)
	tag.timeLimit[1] = int16(tag.HostInfo.TimeLimit)
	broadcastData(tag, network.STOC_GAME_MSG, offset.SubSlices(pbuf))
	if tag.turnCount > 0 {
		tag.rotateCurPlayers()
	}
	tag.turnCount++
	// 原版 tag_duel.cpp:920-922：每次 MSG_NEW_TURN 清空全部投降标记，
	// 「队友两人投降判负」只在同一回合内累积，跨回合重新计数。
	for i := range tag.surrender {
		tag.surrender[i] = false
	}
	return 0
}

// rotateCurPlayers 按回合数奇偶轮换双方的当前操作者：偶数回合轮换
// 0 号队的两人，奇数回合轮换 1 号队的两人（原版 tag_duel.cpp:908-917）。
func (tag *TagDuel) rotateCurPlayers() {
	if tag.turnCount%2 == 0 {
		if tag.curPlayer[0] == tag.players[0] {
			tag.curPlayer[0] = tag.players[1]
		} else {
			tag.curPlayer[0] = tag.players[0]
		}
	} else {
		if tag.curPlayer[1] == tag.players[2] {
			tag.curPlayer[1] = tag.players[3]
		} else {
			tag.curPlayer[1] = tag.players[2]
		}
	}
}

// analyzeTagSwap 队友交换（TAG_SWAP）：当前操作者收完整手牌/额外信息，
// 未公开（cardCodePublicFlag 标记）的卡码对其他人就地抹零后转发，然后整区刷新。
// 保留游标走查：消息体是五计数字节 + topcode + 两段变长码表（ LayTagSwap
// 布局），且抹码直接改写书签区间内的原始字节（maskHiddenCodes 就地写），
// 走「解码 → 掩码 → 重打包」会让转发字节经 codec 重编码，无对应定长
// protocol 结构体可一次解码。
func (tag *TagDuel) analyzeTagSwap(pbuf, offset *utils.YGOBuffer) int {
	var player uint8
	_ = pbuf.Read(&player)
	pbuf.Next(1) // skip main_size
	var ecount uint8
	_ = pbuf.Read(&ecount) // extra_size
	pbuf.Next(1)           // skip extra_p_count
	var hcount uint8
	_ = pbuf.Read(&hcount) // hand_size
	pbufw := pbuf.Clone()
	pbufw.Next(4)
	pbuf.Next(int(hcount)*4 + int(ecount)*4 + 4)
	tag.SendPacketDataToPlayer(tag.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
	maskHiddenCodes(pbufw, hcount)
	maskHiddenCodes(pbufw, ecount)
	for i := 0; i < tag.seatCount(); i++ {
		if tag.players[i] != tag.curPlayer[player] {
			tag.SendPacketDataToPlayer(tag.players[i], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
		}
	}
	tag.ReSendToObservers()
	tag.RefreshExtra(int(player), 0xe81fff, 1)
	tag.RefreshMzone(0, 0x81fff, 0)
	tag.RefreshMzone(1, 0x81fff, 0)
	tag.RefreshSzone(0, 0x681fff, 0)
	tag.RefreshSzone(1, 0x681fff, 0)
	tag.RefreshHand(0, 0x781fff, 0)
	tag.RefreshHand(1, 0x781fff, 0)
	return 0
}

// maskHiddenCodes 就地抹除 count 张卡查询数据里未公开的卡码（每张 4 字节，
// 第 4 字节的 cardCodePublicFlag 标记是否公开），与原版 tag_duel.cpp:
// 1483-1494 的手牌/额外两段循环逐字节一致。
func maskHiddenCodes(pbufw *utils.YGOBuffer, count uint8) {
	for i := uint8(0); i < count; i++ {
		if pbufw.At(3)&cardCodePublicFlag == 0 {
			binary.LittleEndian.PutUint32(pbufw.ReadNext(4), 0)
		} else {
			pbufw.Next(4)
		}
	}
}

// GetResponse 保留 tag 独立实现、不与 single 合并：原版两端逻辑不同——
// tag_duel.cpp:1517-1533 在写回引擎后多一步 players[type].state = 0xff，
// 计时银行按下标 resp_type = type/2（按队伍）而非按座位扣减，守卫条件也
// 不同（tag 依赖路由层的响应者校验，Go 侧补了 Duel/阶段校验）。合并需要
// 新增 duelRoom 钩子且收益不抵风险，故保留两份；仅截断段共用
// truncateResponse。
func (tag *TagDuel) GetResponse(player *DuelPlayer, msgBuffer []byte) {
	if player.State != network.CTOS_RESPONSE {
		return
	}
	if tag.Duel == nil {
		return
	}
	if tag.DuelStage != network.DUEL_STAGE_DUELING {
		return
	}
	if player != tag.curPlayer[tag.lastResponse] {
		return
	}
	// 原版 tag_duel.cpp:1519-1520：响应长度先截断到 SIZE_RETURN_VALUE，
	// 超过定长缓冲的响应直接写回放缓冲区会越界。
	recordResponseAndSet(&tag.DuelMode, truncateResponse(msgBuffer))
	tag.players[player.Type].State = 0xff
	if tag.HostInfo.TimeLimit != 0 {
		respType := 0
		if player.Type >= 2 {
			respType = 1
		}
		if tag.timeLimit[respType] >= tag.timeElapsed {
			tag.timeLimit[respType] -= tag.timeElapsed
		} else {
			tag.timeLimit[respType] = 0
		}
		tag.timeElapsed = 0
	}
	tag.Process()
}

// duelRoom 接口实现（差异点钩子）。

func (tag *TagDuel) allPlayers() []*DuelPlayer { return tag.players[:] }

func (tag *TagDuel) currentPlayer(player int) *DuelPlayer { return tag.curPlayer[player] }

func (tag *TagDuel) zonePair(player int) (int, int) {
	if player == 0 {
		return 0, 2
	}
	return 2, 0
}

func (tag *TagDuel) zoneFullPair(player int) (*DuelPlayer, *DuelPlayer) {
	baseSeat, _ := tag.zonePair(player)
	return tag.players[baseSeat], tag.players[baseSeat+1]
}

func (tag *TagDuel) zoneMaskedPair(player int) (*DuelPlayer, *DuelPlayer) {
	_, baseSeat := tag.zonePair(player)
	return tag.players[baseSeat], tag.players[baseSeat+1]
}

// otherPlayersThanCurPlayer 返回除当前操作者外的全部在席玩家
// （handMaskedRecipients 与 waitingNotifyRecipients 的共同实现）。
func (tag *TagDuel) otherPlayersThanCurPlayer(player int) []*DuelPlayer {
	recipients := make([]*DuelPlayer, 0, 3)
	for i := 0; i < tag.seatCount(); i++ {
		if tag.players[i] != tag.curPlayer[player] {
			recipients = append(recipients, tag.players[i])
		}
	}
	return recipients
}

func (tag *TagDuel) handMaskedRecipients(player int) []*DuelPlayer {
	return tag.otherPlayersThanCurPlayer(player)
}

func (tag *TagDuel) waitingNotifyRecipients(player byte) []*DuelPlayer {
	return tag.otherPlayersThanCurPlayer(int(player))
}

func (tag *TagDuel) timeLimitToObservers() bool { return false }

func (tag *TagDuel) isResponder(player *DuelPlayer) bool {
	return player == tag.curPlayer[tag.lastResponse]
}

func (tag *TagDuel) firstFreeSeat() int {
	for i := 0; i < 4; i++ {
		if tag.players[i] == nil {
			return i
		}
	}
	return 3
}

func (tag *TagDuel) assignSeat(pos int, player *DuelPlayer) { tag.players[pos] = player }

func (tag *TagDuel) removeFromSeat(seat int) { tag.players[seat] = nil }

func (tag *TagDuel) handResultIndex(seat uint8) int {
	if seat == 0 {
		return 0
	}
	return 1
}

// handSeats 返回猜拳双方的代表座位与各自队友：host 为先手队代表、
// hostPartner 为其队友，guest 为后手队代表、guestPartner 为其队友。
// tag：host=players[0]、hostPartner=players[1]（0 号队），
// guest=players[2]、guestPartner=players[3]（1 号队）；
// single 无队友概念，hostPartner/guestPartner 为 nil。
func (tag *TagDuel) handSeats() (host, hostPartner, guest, guestPartner *DuelPlayer) {
	return tag.players[0], tag.players[1], tag.players[2], tag.players[3]
}

func (tag *TagDuel) recordTpPlayer(bWins bool) {}

func (tag *TagDuel) onJoinPassDenied(player *DuelPlayer) {}

func (tag *TagDuel) onEngineWin(player uint8) {}

func (tag *TagDuel) seatCount() int { return 4 }

func (tag *TagDuel) replayFlag() uint32 { return replay.REPLAY_UNIFORM | replay.REPLAY_TAG }

func (tag *TagDuel) extraDuelOpt() uint32 { return ocgcore.DUEL_TAG_MODE }

// loadDecksToEngine 把四个座位的卡组按原版顺序装入引擎并写入回放：
// 各座位 Main 先整体 Reverse，再按「0 号主队 AddCard、1 号队tag AddTagCard、
// 3 号主队 AddCard、2 号队tag AddTagCard」的固定序列装载
// （tag_duel.cpp TPResult 的 loadSingle/loadTag 调用序列；curPlayer 已在
// TPResult 换位后归位，0 号队即 players[0]/players[1]，1 号队即 players[2]/players[3]）。
func (tag *TagDuel) loadDecksToEngine(engine *ocgcore.Duel, rec *replay.Replay) {
	loadSingle := func(deckContainer []*data.CardDataC, player uint8, location uint8) {
		rec.WriteInt32(int32(len(deckContainer)), false)
		for _, card := range deckContainer {
			engine.AddCard(card.Code, int(player), location)
			rec.WriteInt32(int32(card.Code), false)
		}
	}
	loadTag := func(deckContainer []*data.CardDataC, player uint8, location uint8) {
		rec.WriteInt32(int32(len(deckContainer)), false)
		for _, card := range deckContainer {
			engine.AddTagCard(card.Code, player, location)
			rec.WriteInt32(int32(card.Code), false)
		}
	}
	slices.Reverse(tag.pDeck[0].Main)
	slices.Reverse(tag.pDeck[1].Main)
	slices.Reverse(tag.pDeck[2].Main)
	slices.Reverse(tag.pDeck[3].Main)
	loadSingle(tag.pDeck[0].Main, 0, ocgcore.LOCATION_DECK)
	loadSingle(tag.pDeck[0].Extra, 0, ocgcore.LOCATION_EXTRA)
	loadTag(tag.pDeck[1].Main, 0, ocgcore.LOCATION_DECK)
	loadTag(tag.pDeck[1].Extra, 0, ocgcore.LOCATION_EXTRA)
	loadSingle(tag.pDeck[3].Main, 1, ocgcore.LOCATION_DECK)
	loadSingle(tag.pDeck[3].Extra, 1, ocgcore.LOCATION_EXTRA)
	loadTag(tag.pDeck[2].Main, 1, ocgcore.LOCATION_DECK)
	loadTag(tag.pDeck[2].Extra, 1, ocgcore.LOCATION_EXTRA)
}

// refreshOnDuelStart 是开局额外卡组刷新（原版 tag_duel.cpp:467-468 的
// 裸 RefreshExtra(0)/(1)，即默认参数 0xe81fff/缓存，与 single 相同）。
func (tag *TagDuel) refreshOnDuelStart() {
	tag.RefreshExtraDef(0)
	tag.RefreshExtraDef(1)
}

// armDuelTimer 武装决斗秒表：TagTimer 非超时路径每秒自行重新武装
// （与原版 TagTimer 一致，见 TagTimer 方法注释）。
func (tag *TagDuel) armDuelTimer() {
	tag.ETimer = timerWheel.AfterFunc(time.Second, tag.TagTimer)
}

// refreshGraveAfterSwap / refreshAfterSummon / refreshAfterChain /
// refreshAfterDamageStep / refreshAfterNewPhase / refreshSingleMoved /
// refreshSingleFlip 原本在本文件以显式常量实现，差异已收进 DuelMode.refresh
// 参数表（构造时填写，两模式取值一致），统一由 DuelMode 基类读表实现
//（见 duel_refresh.go）。

func (tag *TagDuel) onDuelEnded() {
	for i := 0; i < tag.seatCount(); i++ {
		if tag.players[i] != nil {
			tag.players[i].State = 0xff
		}
	}
}

// tagTeamBaseSeat 返回引擎视角玩家(0/1)所属队伍的基座位
// （0 号队 players[0]/[1]，1 号队 players[2]/[3]；原版 tag_duel.cpp:1691
// 的 pid = (player == 0) ? 0 : 2）。
func tagTeamBaseSeat(player uint8) int {
	if player == 0 {
		return 0
	}
	return 2
}

func (tag *TagDuel) RefreshSingle(player uint8, location uint8, sequence uint8, flag int32) {
	queryBuffer, length, position := querySingleCard(&tag.DuelMode, player, location, sequence, flag)
	baseSeat := tagTeamBaseSeat(player)
	tag.SendPacketDataToPlayer(tag.players[baseSeat], network.STOC_GAME_MSG, queryBuffer[:int(length)+4])
	tag.ReSendToPlayer(tag.players[baseSeat+1])
	if location&uint8(ocgcore.LOCATION_ONFIELD) != 0 {
		if position&ocgcore.POS_FACEUP != 0 {
			baseSeat = 2 - baseSeat
			tag.SendPacketDataToPlayer(tag.players[baseSeat], network.STOC_GAME_MSG, queryBuffer[:int(length)+4])
			tag.ReSendToPlayer(tag.players[baseSeat+1])
			tag.ReSendToObservers()
		}
		return
	}
	if location == uint8(ocgcore.LOCATION_REMOVED) && (position&ocgcore.POS_FACEDOWN) != 0 {
		return
	}
	if location&locationPublicResendMask != 0 {
		for i := 0; i < tag.seatCount(); i++ {
			if tag.players[i] != tag.curPlayer[int(player)] {
				tag.ReSendToPlayer(tag.players[i])
			}
		}
		tag.ReSendToObservers()
	}
}

func (tag *TagDuel) TagTimer() {
	tag.Mu.Lock()
	defer tag.Mu.Unlock()
	running, timeout := tickDuelTimerLocked(&tag.DuelMode)
	if !running {
		if !timeout {
			return
		}
		var wbuf [3]byte
		player := tag.lastResponse
		wbuf[0] = ocgcore.MSG_WIN
		wbuf[1] = 1 - player
		wbuf[2] = 0x3
		tag.SendPacketDataToPlayer(tag.players[0], network.STOC_GAME_MSG, wbuf[:])
		tag.ReSendToPlayers(tag.players[1:]...)
		tag.EndDuel()
		tag.DuelEndProc()
		if tag.ETimer != nil {
			tag.ETimer.Stop()
		}
		return
	}
	// 原版 TagTimer 非超时路径每秒 event_add 重新武装定时器
	//（source/ygopro/gframe/tag_duel.cpp:1740-1741）；漏掉这行会让
	// tag 计时器走一秒后停走，超时判负永不触发。
	tag.ETimer = timerWheel.AfterFunc(time.Second, tag.TagTimer)
}
