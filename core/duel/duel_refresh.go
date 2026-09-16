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
		if m.skipCardQuery(clen) {
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
		if m.skipCardQuery(slen) {
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
