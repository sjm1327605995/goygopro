package duel

import (
	"encoding/binary"

	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// writeUpdateData 向 qbuf 写入 MSG_UPDATE_DATA 头并查询场地卡片数据，
// 返回查询结果长度。SingleDuel/TagDuel 的原实现逐字节一致，此处合并。
func writeUpdateData(d *DuelMode, player int, location int, flag uint32, qbuf []byte, useCache int) int32 {
	flag |= ocgcore.QUERY_CODE | ocgcore.QUERY_POSITION
	qbuf[0] = ocgcore.MSG_UPDATE_DATA
	qbuf[1] = byte(player)
	qbuf[2] = byte(location)

	return d.Duel.QueryFieldCard(uint8(player), uint8(location), flag, qbuf[3:], useCache != 0)
}

// refreshZone 是 RefreshMzone / RefreshSzone 的公共实现：
// 己方（single 为本人，tag 为队友两人）收到完整数据，
// 里侧表示的卡数据被抹除后发给对方队伍，再广播给观察者。
func refreshZone(m duelRoom, player int, location int, flag uint32, useCache int) {
	base := m.BaseMode()
	queryBuffer := make([]byte, ocgcore.SIZE_QUERY_BUFFER)
	qbuf := utils.NewYGOBuffer(queryBuffer, binary.LittleEndian)
	length := writeUpdateData(base, player, location, flag, qbuf.Bytes(), useCache)
	f1, f2 := m.zoneFullPair(player)
	base.SendPacketDataToPlayer(f1, network.STOC_GAME_MSG, queryBuffer[:length+3])
	base.ReSendToPlayer(f2)

	var qLen int32
	qbuf.Next(3)
	for qLen < length {
		var clen int32
		qbuf.Read(&clen)
		qLen += clen
		if skipCardQuery(base, clen) {
			continue
		}
		data := qbuf.Bytes()
		position := network.GetPosition(data, 8)
		if position&ocgcore.POS_FACEDOWN != 0 {
			copy(data[:clen-4], make([]byte, clen-4))
		}
		qbuf.Next(int(clen) - 4)
	}
	m1, m2 := m.zoneMaskedPair(player)
	base.SendPacketDataToPlayer(m1, network.STOC_GAME_MSG, queryBuffer[:length+3])
	base.ReSendToPlayer(m2)
	for _, v := range base.Observers {
		base.ReSendToPlayer(v)
	}
}

// refreshHand 是 RefreshHand 的公共实现：当前操作者收到完整手牌数据，
// 非公开（里侧）卡码抹除后发给 handMaskedRecipients（single 仅对手，
// tag 为除当前操作者外的所有玩家），再广播给观察者。
func refreshHand(m duelRoom, player int, flag uint32, useCache int) {
	base := m.BaseMode()
	queryBuffer := make([]byte, ocgcore.SIZE_QUERY_BUFFER)
	qbuf := utils.NewYGOBuffer(queryBuffer, binary.LittleEndian)
	length := writeUpdateData(base, player, int(ocgcore.LOCATION_HAND), flag, qbuf.Bytes(), useCache)
	base.SendPacketDataToPlayer(m.currentPlayer(player), network.STOC_GAME_MSG, queryBuffer[:length+3])
	qbuf.Next(3)
	var (
		qLen int32
	)

	for qLen < length {
		var slen int32
		qbuf.Read(&slen)
		qLen += slen
		if skipCardQuery(base, slen) {
			continue
		}
		data := qbuf.Bytes()
		position := network.GetPosition(data, 8)
		if position&ocgcore.POS_FACEUP == 0 {
			copy(data[:slen-4], make([]byte, slen-4))
		}
		qbuf.Next(int(slen) - 4)
	}
	for _, p := range m.handMaskedRecipients(player) {
		base.SendPacketDataToPlayer(p, network.STOC_GAME_MSG, queryBuffer[:length+3])
	}
	for _, v := range base.Observers {
		base.ReSendToPlayer(v)
	}
}

// refreshGrave 是 RefreshGrave 的公共实现：完整墓地数据广播给所有玩家与观察者。
func refreshGrave(m duelRoom, player int, flag uint32, useCache int) {
	base := m.BaseMode()
	queryBuffer := make([]byte, ocgcore.SIZE_QUERY_BUFFER)
	length := writeUpdateData(base, player, int(ocgcore.LOCATION_GRAVE), flag, queryBuffer, useCache)
	broadcastData(m, network.STOC_GAME_MSG, queryBuffer[:length+3])
}

// refreshExtra 是 RefreshExtra 的公共实现：完整额外卡组数据只发给当前操作者。
func refreshExtra(m duelRoom, player int, flag uint32, useCache int) {
	base := m.BaseMode()
	queryBuffer := make([]byte, ocgcore.SIZE_QUERY_BUFFER)
	length := writeUpdateData(base, player, int(ocgcore.LOCATION_EXTRA), flag, queryBuffer, useCache)
	base.SendPacketDataToPlayer(m.currentPlayer(player), network.STOC_GAME_MSG, queryBuffer[:length+3])
}

// skipCardQuery 报告查询块长度 clen 是否应跳过抹除处理。边界语义按模式保留：
// single 为 clen < LEN_HEADER，tag 为 clen <= LEN_HEADER（refreshFlags.skipCardQueryLe），
// 原版两种实现都必须原样保留。
func skipCardQuery(base *DuelMode, clen int32) bool {
	if base.refresh.skipCardQueryLe {
		return clen <= ocgcore.LEN_HEADER
	}
	return clen < ocgcore.LEN_HEADER
}

// 以下为 *Def 默认参数的公共包装，两个模式的取值完全一致。

func refreshMzoneDef(m duelRoom, player int) {
	refreshZone(m, player, int(ocgcore.LOCATION_MZONE), 0x881fff, 1)
}

func refreshSzoneDef(m duelRoom, player int) {
	refreshZone(m, player, int(ocgcore.LOCATION_SZONE), 0x681fff, 1)
}

func refreshHandDef(m duelRoom, player int) {
	refreshHand(m, player, 0x681fff, 1)
}

func refreshGraveDef(m duelRoom, player int) {
	refreshGrave(m, player, 0x81fff, 1)
}

func refreshExtraDef(m duelRoom, player int) {
	refreshExtra(m, player, 0xe81fff, 1)
}

// refreshFlags 汇集两模式仅在查询 flag / 缓存 / 边界语义上不同的刷新参数。
// SingleDuel 的取值即原 *Def 系列的默认参数（带缓存），TagDuel 为原版
// TagDuel 的 0x81fff4 / 0x781fff（无缓存）；由 newSingleDuel / newTagDuel
// 构造时一次性填好，各 refresh 钩子与 skipCardQuery 的边界判断读表实现。
// 场景 flag 字段为 0 表示该场景不刷新此区域（single 的 newPhase 不刷新手牌，
// 与原版一致），因此 0 不能作为合法的查询 flag 使用。
type refreshFlags struct {
	// summon / chain / damageStep / newPhase 是 MSG_SUMMONED/SPSUMMONED/
	// FLIPSUMMONED、MSG_CHAINED/CHAIN_SOLVED/CHAIN_END、MSG_DAMAGE_STEP_START/
	// END、MSG_NEW_PHASE 后按 mzone→szone→hand 顺序整区刷新的查询 flag。
	summonMzone, summonSzone                   uint32
	chainMzone, chainSzone, chainHand          uint32
	damageStepMzone                            uint32
	newPhaseMzone, newPhaseSzone, newPhaseHand uint32
	// singleMoved / singleFlip 是 MSG_MOVE/POS_CHANGE/SWAP 与 MSG_FLIPSUMMONING
	// 前置单卡刷新 RefreshSingle 的查询 flag（single: 0xf81fff；tag: 0x81fff4）。
	singleMoved, singleFlip uint32
	// graveAfterSwap 是 MSG_SWAP_GRAVE_DECK 后墓地刷新的查询 flag
	//（single: 0x81fff 即 RefreshGraveDef；tag: 0x81fff4）。
	graveAfterSwap uint32
	// useCache 是整区刷新的缓存参数（single 的 *Def 系列传 1；tag 传 0）。
	useCache int
	// skipCardQueryLe 是查询块抹除跳过的边界语义：false = clen < LEN_HEADER
	//（single）；true = clen <= LEN_HEADER（tag）——原版两种实现都必须原样保留。
	skipCardQueryLe bool
}

// refreshAfterSummon 是 MSG_SUMMONED/SPSUMMONED/FLIPSUMMONED 后的场地刷新。
// 两模式的差异仅是查询 flag/缓存常量（refreshFlags.summonMzone/summonSzone），
// 此处读表实现一次。
func (d *DuelMode) refreshAfterSummon() {
	f := d.refresh
	d.RefreshMzone(0, f.summonMzone, f.useCache)
	d.RefreshMzone(1, f.summonMzone, f.useCache)
	d.RefreshSzone(0, f.summonSzone, f.useCache)
	d.RefreshSzone(1, f.summonSzone, f.useCache)
}

// refreshAfterChain 是 MSG_CHAINED/CHAIN_SOLVED/CHAIN_END 后的全区域刷新。
// 差异已收进 refreshFlags.chain*。
func (d *DuelMode) refreshAfterChain() {
	f := d.refresh
	d.RefreshMzone(0, f.chainMzone, f.useCache)
	d.RefreshMzone(1, f.chainMzone, f.useCache)
	d.RefreshSzone(0, f.chainSzone, f.useCache)
	d.RefreshSzone(1, f.chainSzone, f.useCache)
	d.RefreshHand(0, f.chainHand, f.useCache)
	d.RefreshHand(1, f.chainHand, f.useCache)
}

// refreshAfterDamageStep 是 MSG_DAMAGE_STEP_START/END 后的怪兽区刷新。
// 差异已收进 refreshFlags.damageStepMzone。
func (d *DuelMode) refreshAfterDamageStep() {
	f := d.refresh
	d.RefreshMzone(0, f.damageStepMzone, f.useCache)
	d.RefreshMzone(1, f.damageStepMzone, f.useCache)
}

// refreshAfterNewPhase 是 MSG_NEW_PHASE 后的区域刷新。flag 0 表示不刷新
//（single 的 newPhaseHand 为 0，即原版 single 不刷新手牌；tag 为 0x781fff）。
func (d *DuelMode) refreshAfterNewPhase() {
	f := d.refresh
	d.RefreshMzone(0, f.newPhaseMzone, f.useCache)
	d.RefreshMzone(1, f.newPhaseMzone, f.useCache)
	d.RefreshSzone(0, f.newPhaseSzone, f.useCache)
	d.RefreshSzone(1, f.newPhaseSzone, f.useCache)
	if f.newPhaseHand != 0 {
		d.RefreshHand(0, f.newPhaseHand, f.useCache)
		d.RefreshHand(1, f.newPhaseHand, f.useCache)
	}
}

// refreshSingleMoved 是 MSG_MOVE/POS_CHANGE/SWAP 后的单卡刷新。flag 差异已收进
// refreshFlags.singleMoved（single 即原 RefreshSingleDef 的 0xf81fff；tag 0x81fff4）；
// RefreshSingle 本身按模式不同（发送对象不同），仍由模式实现。
func (d *DuelMode) refreshSingleMoved(cc, cl, cs uint8) {
	d.room.RefreshSingle(cc, cl, cs, int32(d.refresh.singleMoved))
}

// refreshSingleFlip 是 MSG_FLIPSUMMONING 前置的单卡刷新，flag 差异已收进
// refreshFlags.singleFlip（single: 0xf81fff；tag: 0x81fff4）。
func (d *DuelMode) refreshSingleFlip(cc, cl, cs uint8) {
	d.room.RefreshSingle(cc, cl, cs, int32(d.refresh.singleFlip))
}

// refreshGraveAfterSwap 是 MSG_SWAP_GRAVE_DECK 后的墓地刷新。flag/缓存差异已收进
// refreshFlags.graveAfterSwap/useCache（single 即原 RefreshGraveDef；tag 0x81fff4/无缓存）。
func (d *DuelMode) refreshGraveAfterSwap(player int) {
	d.RefreshGrave(player, d.refresh.graveAfterSwap, d.refresh.useCache)
}
