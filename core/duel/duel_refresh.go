package duel

import (
	"encoding/binary"
	"sync"

	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// MSG_UPDATE_DATA 三字节头 [type, player, location]（writeUpdateData 写入
// qbuf[0..2]），引擎查询数据紧随其后；发送与解析均只看
// length+updateDataHeaderLen。
const updateDataHeaderLen = 3

// 查询块布局（ocgcore card.cpp get_infos 的 "first 8 bytes: data length,
// query flag"）：len(4) + query_flag(4) + code(4) + position(4) + ...
// 偏移以查询块起点计；position 的 uint32 最高字节即表示形式
// （network.GetPosition 的 >>24）。
const (
	queryLenSize     = 4  // 块头长度字段（uint32）自身宽度：遍历时已 Read 消费，advance/clear 需扣除
	queryFlagOff     = 4  // query_flag 在块内偏移
	queryCodeOff     = 8  // 卡码在块内偏移
	queryPositionOff = 12 // 表示形式在块内偏移；也是掩码所需的最小块长
)

// MSG_UPDATE_CARD 四字节头 [type, player, location, sequence]；单卡查询缓冲
// 大小与 C++ RefreshSingle 的 query_buffer 一致（single_duel.cpp:1556）。
const (
	msgUpdateCardHeaderLen = 4
	singleQueryBufferSize  = 0x1000
)

// queryBufferPool 复用整区刷新的 16KB 查询缓冲。refreshAfterChain 一轮
// 6 次整区刷新，此前每次 refresh 都 make([]byte, SIZE_QUERY_BUFFER)，
// 分配量可观。缓冲区只在函数内使用：QueryFieldCard 每次覆盖写入前
// length+updateDataHeaderLen 字节（发送只看这段），gnet Conn.Write 返回前
// 已完成拷贝，归还给池是安全的。
var queryBufferPool = sync.Pool{
	New: func() any { return make([]byte, ocgcore.SIZE_QUERY_BUFFER) },
}

// getQueryBuffer 从池取查询缓冲；归还用 putQueryBuffer（defer 配对）。
func getQueryBuffer() []byte  { return queryBufferPool.Get().([]byte) }
func putQueryBuffer(b []byte) { queryBufferPool.Put(b) }

// writeUpdateData 向 qbuf 写入 MSG_UPDATE_DATA 头并查询场地卡片数据，
// 返回查询结果长度。SingleDuel/TagDuel 的原实现逐字节一致，此处合并。
func writeUpdateData(mode *DuelMode, player int, location int, flag uint32, qbuf []byte, useCache int) int32 {
	flag |= ocgcore.QUERY_CODE | ocgcore.QUERY_POSITION
	qbuf[0] = ocgcore.MSG_UPDATE_DATA
	qbuf[1] = byte(player)
	qbuf[2] = byte(location)

	return mode.Duel.QueryFieldCard(uint8(player), uint8(location), flag, qbuf[3:], useCache != 0)
}

// maskCardQueries 在共享缓冲上逐卡执行「谓词为真则抹除该卡查询块数据」。
// 谓词入参为卡的 position 字节。消费游标与原版 walk 逐字节等价：先跳过
// MSG_UPDATE_DATA 三字节头，再按 while(qLen < length) 逐块读长度字段
// （Read 自身消费 4 字节）；clen <= LEN_HEADER 的短块跳过抹除（skipCardQuery），
// 谓词为真则 clear(data[:clen-queryLenSize])——data 起于 query_flag，对应
// C++ 的 memset(qbuf, 0, clen - 4)；随后游标前进 clen-queryLenSize。
//
// 谓词即两调用点与 C++ 原版的唯一差异：
//   - refreshZone 传 position&POS_FACEDOWN != 0（里侧才抹除）：single_duel.cpp
//     1486-1488 的 RefreshMzone/RefreshSzone、tag_duel.cpp:1602-1604/1627-1629
//     的同名方法；
//   - refreshHand 传 position&POS_FACEUP == 0（非公开即抹除）：single_duel.cpp:
//     1528-1530 的 RefreshHand、tag_duel.cpp:1650-1652 的同名方法。
func maskCardQueries(base *DuelMode, qbuf *utils.YGOBuffer, length int32, shouldMask func(position uint32) bool) {
	qbuf.Next(updateDataHeaderLen)
	var qLen int32
	for qLen < length {
		var clen int32
		qbuf.Read(&clen)
		qLen += clen
		if skipCardQuery(base, clen) {
			continue
		}
		data := qbuf.Bytes()
		// data 起于 query_flag（len 字段已读），position 在 query_flag 之后
		// 第 queryPositionOff-queryFlagOff 字节（块内偏移 12）。
		position := network.GetPosition(data, queryPositionOff-queryFlagOff)
		if shouldMask(position) {
			clear(data[:clen-queryLenSize])
		}
		qbuf.Next(int(clen) - queryLenSize)
	}
}

// refreshZone 是 RefreshMzone / RefreshSzone 的公共实现：
// 己方（single 为本人，tag 为队友两人）收到完整数据，
// 里侧表示的卡数据被抹除后发给对方队伍，再广播给观察者。
func refreshZone(room duelRoom, player int, location int, flag uint32, useCache int) {
	base := room.BaseMode()
	queryBuffer := getQueryBuffer()
	defer putQueryBuffer(queryBuffer)
	qbuf := utils.NewYGOBuffer(queryBuffer, binary.LittleEndian)
	length := writeUpdateData(base, player, location, flag, qbuf.Bytes(), useCache)
	f1, f2 := room.zoneFullPair(player)
	base.SendPacketDataToPlayer(f1, network.STOC_GAME_MSG, queryBuffer[:length+updateDataHeaderLen])
	base.ReSendToPlayer(f2)

	maskCardQueries(base, qbuf, length, func(position uint32) bool {
		return position&ocgcore.POS_FACEDOWN != 0
	})
	m1, m2 := room.zoneMaskedPair(player)
	base.SendPacketDataToPlayer(m1, network.STOC_GAME_MSG, queryBuffer[:length+updateDataHeaderLen])
	base.ReSendToPlayer(m2)
	base.ReSendToObservers()
}

// refreshHand 是 RefreshHand 的公共实现：当前操作者收到完整手牌数据，
// 非公开（里侧）卡码抹除后发给 handMaskedRecipients（single 仅对手，
// tag 为除当前操作者外的所有玩家），再广播给观察者。
func refreshHand(room duelRoom, player int, flag uint32, useCache int) {
	base := room.BaseMode()
	queryBuffer := getQueryBuffer()
	defer putQueryBuffer(queryBuffer)
	qbuf := utils.NewYGOBuffer(queryBuffer, binary.LittleEndian)
	length := writeUpdateData(base, player, int(ocgcore.LOCATION_HAND), flag, qbuf.Bytes(), useCache)
	base.SendPacketDataToPlayer(room.currentPlayer(player), network.STOC_GAME_MSG, queryBuffer[:length+updateDataHeaderLen])

	maskCardQueries(base, qbuf, length, func(position uint32) bool {
		return position&ocgcore.POS_FACEUP == 0
	})
	for _, p := range room.handMaskedRecipients(player) {
		base.SendPacketDataToPlayer(p, network.STOC_GAME_MSG, queryBuffer[:length+updateDataHeaderLen])
	}
	base.ReSendToObservers()
}

// refreshGrave 是 RefreshGrave 的公共实现：完整墓地数据广播给所有玩家与观察者。
func refreshGrave(room duelRoom, player int, flag uint32, useCache int) {
	base := room.BaseMode()
	queryBuffer := getQueryBuffer()
	defer putQueryBuffer(queryBuffer)
	length := writeUpdateData(base, player, int(ocgcore.LOCATION_GRAVE), flag, queryBuffer, useCache)
	broadcastData(room, network.STOC_GAME_MSG, queryBuffer[:length+updateDataHeaderLen])
}

// refreshExtra 是 RefreshExtra 的公共实现：完整额外卡组数据只发给当前操作者。
func refreshExtra(room duelRoom, player int, flag uint32, useCache int) {
	base := room.BaseMode()
	queryBuffer := getQueryBuffer()
	defer putQueryBuffer(queryBuffer)
	length := writeUpdateData(base, player, int(ocgcore.LOCATION_EXTRA), flag, queryBuffer, useCache)
	base.SendPacketDataToPlayer(room.currentPlayer(player), network.STOC_GAME_MSG, queryBuffer[:length+updateDataHeaderLen])
}

// skipCardQuery 报告查询块长度 clen 是否应跳过抹除处理。两模式的边界语义
// 一致：clen <= LEN_HEADER（single_duel.cpp:1484/1505/1526 与 tag 侧同为 <=）。
func skipCardQuery(base *DuelMode, clen int32) bool {
	return clen <= ocgcore.LEN_HEADER
}

// ----- RefreshSingle 公共段（single_duel.cpp / tag_duel.cpp 逐字节一致部分） -----

// querySingleCard 执行 RefreshSingle 的引擎查询：向 qbuf 写 MSG_UPDATE_CARD
// 四字节头 [type, player, location, sequence]，随后 query_card 填入查询块。
// flag 自动并入 QUERY_CODE|QUERY_POSITION（原版 RefreshSingle 入口即或上这
// 两位）。返回消息缓冲、查询块长度与卡的表示形式。
//
// 查询块布局见 queryLenSize 等常量：len(4) + query_flag(4) + code(4) +
// position(4) + ...，position 在块内偏移 queryPositionOff 的 uint32 最高字节。
func querySingleCard(base *DuelMode, player, location, sequence uint8, flag int32) (queryBuffer []byte, length int32, position uint32) {
	queryBuffer = make([]byte, singleQueryBufferSize)
	qbuf := utils.NewYGOBuffer(queryBuffer, binary.LittleEndian)
	qbuf.Write([]byte{ocgcore.MSG_UPDATE_CARD, player, location, sequence})
	length = base.Duel.QueryCard(player, location, sequence,
		flag|int32(ocgcore.QUERY_CODE|ocgcore.QUERY_POSITION), qbuf.Bytes(), false)
	if length <= ocgcore.LEN_HEADER {
		return queryBuffer, length, 0
	}
	// 查询块起于缓冲偏移 msgUpdateCardHeaderLen（四字节头之后）。
	position = network.GetPosition(queryBuffer, msgUpdateCardHeaderLen+queryPositionOff)
	return queryBuffer, length, position
}

// maskSingleQueryCode 把查询块里卡码及其后字段就地抹零，并把块头的查询
// flag 改成 QUERY_CODE——原版 single RefreshSingle 的里侧掩码（对方面前只剩
// 一个 0 卡码，single_duel.cpp:1576-1579 的 Write(QUERY_CODE)/Write(0)/
// memset(qbuf, 0, clen - 12)）。flag 已强制含 QUERY_CODE|QUERY_POSITION，
// 正常查询块 clen >= 16；clen < queryPositionOff 属损坏数据，直接跳过掩码。
func maskSingleQueryCode(queryBuffer []byte) {
	clen := int(binary.LittleEndian.Uint32(queryBuffer[msgUpdateCardHeaderLen:]))
	end := msgUpdateCardHeaderLen + clen
	if clen < queryPositionOff || end > len(queryBuffer) {
		return
	}
	binary.LittleEndian.PutUint32(queryBuffer[msgUpdateCardHeaderLen+queryFlagOff:], ocgcore.QUERY_CODE)
	binary.LittleEndian.PutUint32(queryBuffer[msgUpdateCardHeaderLen+queryCodeOff:], 0)
	clear(queryBuffer[msgUpdateCardHeaderLen+queryPositionOff : end])
}

// 刷新查询 flag 组合（QUERY_* 位掩码之和；C++ 侧是各 Refresh 方法默认参数
// 的字面量，single_duel.h:35-40 与 tag_duel.h:35-40，两模式取值一致）。
const (
	queryFlagMzone      uint32 = 0x881fff // RefreshMzone 默认 flag
	queryFlagSzone      uint32 = 0x681fff // RefreshSzone 默认 flag
	queryFlagHand       uint32 = 0x681fff // RefreshHand 默认 flag
	queryFlagGrave      uint32 = 0x81fff  // RefreshGrave 默认 flag
	queryFlagExtra      uint32 = 0xe81fff // RefreshExtra 默认 flag
	queryFlagSingleCard uint32 = 0xf81fff // RefreshSingle 默认 flag（RefreshSingleDef）
)

// 洗牌类消息后的裸刷新 flag（use_cache=0，区别于上面带缓存的默认参数）。
const (
	queryFlagShuffleHand uint32 = 0x781fff // MSG_SHUFFLE_HAND 后 RefreshHand（single_duel.cpp:851、tag_duel.cpp:820）
	queryFlagShuffleZone uint32 = 0x181fff // MSG_SHUFFLE_SET_CARD 后 RefreshMzone/Szone（single_duel.cpp:907-912、tag_duel.cpp:888-892）
)

// 以下为 *Def 默认参数的公共包装，两个模式的取值完全一致。

func refreshMzoneDef(room duelRoom, player int) {
	refreshZone(room, player, int(ocgcore.LOCATION_MZONE), queryFlagMzone, 1)
}

func refreshSzoneDef(room duelRoom, player int) {
	refreshZone(room, player, int(ocgcore.LOCATION_SZONE), queryFlagSzone, 1)
}

func refreshHandDef(room duelRoom, player int) {
	refreshHand(room, player, queryFlagHand, 1)
}

func refreshGraveDef(room duelRoom, player int) {
	refreshGrave(room, player, queryFlagGrave, 1)
}

func refreshExtraDef(room duelRoom, player int) {
	refreshExtra(room, player, queryFlagExtra, 1)
}

// refreshFlags 汇集各刷新场景的查询 flag / 缓存参数。两模式取值完全一致：
// 原版 tag_duel.h:35-40 与 single_duel.h:35-40 的默认参数相同（0x881fff /
// 0x681fff / 0x81fff / 0xe81fff，use_cache=1，RefreshSingle 0xf81fff），
// 由 newSingleDuel / newTagDuel 经 defaultRefreshFlags 构造时一次性填好，
// 各 refresh 钩子读表实现。
// 场景 flag 字段为 0 表示该场景不刷新此区域，因此 0 不能作为合法的查询 flag 使用。
type refreshFlags struct {
	// summon / chain / damageStep / newPhase 是 MSG_SUMMONED/SPSUMMONED/
	// FLIPSUMMONED、MSG_CHAINED/CHAIN_SOLVED/CHAIN_END、MSG_DAMAGE_STEP_START/
	// END、MSG_NEW_PHASE 后按 mzone→szone→hand 顺序整区刷新的查询 flag
	//（summon 与 damageStep 不含手牌，与原版一致）。
	summonMzone, summonSzone                   uint32
	chainMzone, chainSzone, chainHand          uint32
	damageStepMzone                            uint32
	newPhaseMzone, newPhaseSzone, newPhaseHand uint32
	// singleMoved / singleFlip 是 MSG_MOVE/POS_CHANGE/SWAP 与 MSG_FLIPSUMMONING
	// 前置单卡刷新 RefreshSingle 的查询 flag（两模式均为默认 0xf81fff）。
	singleMoved, singleFlip uint32
	// graveAfterSwap 是 MSG_SWAP_GRAVE_DECK 后墓地刷新的查询 flag
	//（两模式均为默认 0x81fff，即 RefreshGraveDef）。
	graveAfterSwap uint32
	// useCache 是整区刷新的缓存参数（两模式的默认刷新均传 1）。
	useCache int
}

// defaultRefreshFlags 返回两模式共用的刷新参数表（C++ 两模式的默认刷新参数
// 完全一致，见 refreshFlags 文档）。
func defaultRefreshFlags() refreshFlags {
	return refreshFlags{
		summonMzone:     queryFlagMzone,
		summonSzone:     queryFlagSzone,
		chainMzone:      queryFlagMzone,
		chainSzone:      queryFlagSzone,
		chainHand:       queryFlagHand,
		damageStepMzone: queryFlagMzone,
		newPhaseMzone:   queryFlagMzone,
		newPhaseSzone:   queryFlagSzone,
		newPhaseHand:    queryFlagHand,
		singleMoved:     queryFlagSingleCard,
		singleFlip:      queryFlagSingleCard,
		graveAfterSwap:  queryFlagGrave,
		useCache:        1,
	}
}

// refreshAfterSummon 是 MSG_SUMMONED/SPSUMMONED/FLIPSUMMONED 后的场地刷新。
// 两模式的差异仅是查询 flag/缓存常量（refreshFlags.summonMzone/summonSzone），
// 此处读表实现一次。
func (mode *DuelMode) refreshAfterSummon() {
	flags := mode.refresh
	mode.RefreshMzone(0, flags.summonMzone, flags.useCache)
	mode.RefreshMzone(1, flags.summonMzone, flags.useCache)
	mode.RefreshSzone(0, flags.summonSzone, flags.useCache)
	mode.RefreshSzone(1, flags.summonSzone, flags.useCache)
}

// refreshAfterChain 是 MSG_CHAINED/CHAIN_SOLVED/CHAIN_END 后的全区域刷新。
// 差异已收进 refreshFlags.chain*。
func (mode *DuelMode) refreshAfterChain() {
	flags := mode.refresh
	mode.RefreshMzone(0, flags.chainMzone, flags.useCache)
	mode.RefreshMzone(1, flags.chainMzone, flags.useCache)
	mode.RefreshSzone(0, flags.chainSzone, flags.useCache)
	mode.RefreshSzone(1, flags.chainSzone, flags.useCache)
	mode.RefreshHand(0, flags.chainHand, flags.useCache)
	mode.RefreshHand(1, flags.chainHand, flags.useCache)
}

// refreshAfterDamageStep 是 MSG_DAMAGE_STEP_START/END 后的怪兽区刷新。
// 差异已收进 refreshFlags.damageStepMzone。
func (mode *DuelMode) refreshAfterDamageStep() {
	flags := mode.refresh
	mode.RefreshMzone(0, flags.damageStepMzone, flags.useCache)
	mode.RefreshMzone(1, flags.damageStepMzone, flags.useCache)
}

// refreshAfterNewPhase 是 MSG_NEW_PHASE 后的区域刷新。两模式都刷新手牌
// （single_duel.cpp:942-943 / tag_duel.cpp 同）；flag 0 的防御性跳过保留，
// 防止参数表缺字段时发出 flag=0 的非法查询。
func (mode *DuelMode) refreshAfterNewPhase() {
	flags := mode.refresh
	mode.RefreshMzone(0, flags.newPhaseMzone, flags.useCache)
	mode.RefreshMzone(1, flags.newPhaseMzone, flags.useCache)
	mode.RefreshSzone(0, flags.newPhaseSzone, flags.useCache)
	mode.RefreshSzone(1, flags.newPhaseSzone, flags.useCache)
	if flags.newPhaseHand != 0 {
		mode.RefreshHand(0, flags.newPhaseHand, flags.useCache)
		mode.RefreshHand(1, flags.newPhaseHand, flags.useCache)
	}
}

// refreshSingleMoved 是 MSG_MOVE/POS_CHANGE/SWAP 后的单卡刷新。flag 差异已收进
// refreshFlags.singleMoved（两模式均为原 RefreshSingleDef 的 0xf81fff）；
// RefreshSingle 本身按模式不同（发送对象不同），仍由模式实现。
func (mode *DuelMode) refreshSingleMoved(cc, cl, cs uint8) {
	mode.room.RefreshSingle(cc, cl, cs, int32(mode.refresh.singleMoved))
}

// refreshSingleFlip 是 MSG_FLIPSUMMONING 前置的单卡刷新，flag 差异已收进
// refreshFlags.singleFlip（两模式均为 0xf81fff）。
func (mode *DuelMode) refreshSingleFlip(cc, cl, cs uint8) {
	mode.room.RefreshSingle(cc, cl, cs, int32(mode.refresh.singleFlip))
}

// refreshGraveAfterSwap 是 MSG_SWAP_GRAVE_DECK 后的墓地刷新。flag/缓存差异已收进
// refreshFlags.graveAfterSwap/useCache（两模式均为原 RefreshGraveDef 的 0x81fff/缓存）。
func (mode *DuelMode) refreshGraveAfterSwap(player int) {
	mode.RefreshGrave(player, mode.refresh.graveAfterSwap, mode.refresh.useCache)
}
