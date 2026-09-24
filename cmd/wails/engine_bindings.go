package main

import (
	"encoding/binary"
	"fmt"

	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol"
)

// engineBinding 描述一个引擎消息如何解析并转发给前端：
//   - newMsg:   返回新的消息结构体实例；nil 表示该消息没有消息体
//     （handleGameMessage 只 emit 空事件）；
//   - event:    要 emit 的前端事件名；空串表示仅按结构体对齐字节流、不 emit
//     （UI 不关心但必须消费字节的消息）；
//   - decorate: 可选的后处理。nil 时直接把解析出的结构体 emit 出去
//     （结构体的 json tag 即前端事件键名）；需要派生字段（布尔化、
//     解包位置、多列表拆分、query blob 跳过）时给出该函数。
//
// 所有布局权威依据见 protocol/message.go 头部注释（source/ygopro 引擎源码）。
type engineBinding struct {
	name     string
	newMsg   func() any
	event    string
	decorate func(c *WailsDuelClient, engType byte, pbuf *utils.YGOBuffer, msg any) error
}

// packedLocEntry 把单个打包 info_location 解成 {c,l,s,p}。
func packedLocEntry(loc uint32) map[string]interface{} {
	cc, cl, cs, cp := protocol.UnpackPackedLoc(loc)
	return map[string]interface{}{"c": cc, "l": cl, "s": cs, "p": cp}
}

// decorateSelectCard 转换 MSG_SELECT_CARD / MSG_SELECT_TRIBUTE：cancelable
// 布尔化，条目展开；engType 区分祭品来源（tribute 标志）。
func decorateSelectCard(c *WailsDuelClient, engType byte, _ *utils.YGOBuffer, msg any) error {
	m := msg.(*protocol.SelectCardMsg)
	cards := make([]selectCardEntryDTO, len(m.Cards))
	for i, cd := range m.Cards {
		cards[i] = selectCardEntryDTO{
			Code: cd.Code,
			C:    cd.Controller,
			L:    cd.Location,
			S:    cd.Sequence,
			P:    cd.Position,
		}
	}
	c.emit("duel:select_card", selectCardDTO{
		Player:     m.Player,
		Cancelable: m.Cancelable != 0,
		Min:        m.Min,
		Max:        m.Max,
		Cards:      cards,
		Tribute:    engType == ocgcore.MSG_SELECT_TRIBUTE,
	})
	return nil
}

// decorateSelectUnselect 转换 MSG_SELECT_UNSELECT_CARD：布尔化 + 双列表展开。
func decorateSelectUnselect(c *WailsDuelClient, _ byte, _ *utils.YGOBuffer, msg any) error {
	m := msg.(*protocol.SelectUnselectCardMsg)
	cards := make([]selectUnselectEntryDTO, len(m.Cards1))
	for i, cd := range m.Cards1 {
		cards[i] = selectUnselectEntryDTO{Code: cd.Code, C: cd.Controller, L: cd.Location, S: cd.Sequence}
	}
	unselect := make([]selectUnselectEntryDTO, len(m.Cards2))
	for i, cd := range m.Cards2 {
		unselect[i] = selectUnselectEntryDTO{Code: cd.Code, C: cd.Controller, L: cd.Location, S: cd.Sequence}
	}
	c.emit("duel:select_unselect", selectUnselectDTO{
		Player:       m.Player,
		Finishable:   m.Finishable != 0,
		Cancelable:   m.Cancelable != 0,
		Min:          m.Min,
		Max:          m.Max,
		Cards:        cards,
		UnselectList: unselect,
	})
	return nil
}

// decorateSelectChain 转换 MSG_SELECT_CHAIN：forced 取所有条目的或，
// 条目位置从打包 info_location 解出。
func decorateSelectChain(c *WailsDuelClient, _ byte, _ *utils.YGOBuffer, msg any) error {
	m := msg.(*protocol.SelectChainMsg)
	chains := make([]chainEntryDTO, len(m.Chains))
	anyForced := false
	for i, ch := range m.Chains {
		if ch.Forced != 0 {
			anyForced = true
		}
		cc, cl, cs, cp := protocol.UnpackPackedLoc(ch.InfoLocation)
		chains[i] = chainEntryDTO{
			Flag:   uint32(ch.DescFlag),
			Forced: ch.Forced != 0,
			Code:   ch.Code,
			CC:     cc,
			CL:     cl,
			CS:     cs,
			CP:     cp,
			Desc:   ch.Description,
		}
	}
	c.emit("duel:select_chain", selectChainDTO{
		Player: m.Player,
		Count:  m.Count,
		Forced: anyForced,
		Chains: chains,
	})
	return nil
}

// decorateIdleCmd 转换 MSG_SELECT_IDLECMD：五张卡片列表 + 激活列表。
func decorateIdleCmd(c *WailsDuelClient, _ byte, _ *utils.YGOBuffer, msg any) error {
	m := msg.(*protocol.SelectIdleCmdMsg)
	cardEntries := func(entries []protocol.CmdCardEntry) []cmdCardEntryDTO {
		out := make([]cmdCardEntryDTO, len(entries))
		for i, e := range entries {
			out[i] = cmdCardEntryDTO{Code: int32(e.Code), C: e.CC, L: e.CL, S: e.CS, Idx: i}
		}
		return out
	}
	activateEntries := func(entries []protocol.CmdActivateEntry) []cmdActivateEntryDTO {
		out := make([]cmdActivateEntryDTO, len(entries))
		for i, e := range entries {
			out[i] = cmdActivateEntryDTO{Code: int32(e.Code), C: e.CC, L: e.CL, S: e.CS, Desc: e.Description, Idx: i}
		}
		return out
	}
	c.emit("duel:select_idlecmd", selectIdleCmdDTO{
		Player:   m.Player,
		Summon:   cardEntries(m.CmdsA),
		SPSummon: cardEntries(m.CmdsB),
		Repos:    cardEntries(m.CmdsC),
		MSet:     cardEntries(m.CmdsD),
		SSet:     cardEntries(m.CmdsE),
		Activate: activateEntries(m.CmdsF),
		ToBP:     m.ToBP != 0,
		ToEP:     m.ToEP != 0,
		Shuffle:  m.CanShuffle != 0,
	})
	return nil
}

// decorateBattleCmd 转换 MSG_SELECT_BATTLECMD：激活列表 + 攻击列表。
func decorateBattleCmd(c *WailsDuelClient, _ byte, _ *utils.YGOBuffer, msg any) error {
	m := msg.(*protocol.SelectBattleCmdMsg)
	activate := make([]cmdActivateEntryDTO, len(m.CmdsA))
	for i, e := range m.CmdsA {
		activate[i] = cmdActivateEntryDTO{Code: int32(e.Code), C: e.CC, L: e.CL, S: e.CS, Desc: e.Description, Idx: i}
	}
	attack := make([]cmdAttackEntryDTO, len(m.CmdsB))
	for i, e := range m.CmdsB {
		attack[i] = cmdAttackEntryDTO{Code: int32(e.Code), C: e.CC, L: e.CL, S: e.CS, DirAtt: e.DirectAttackable != 0, Idx: i}
	}
	c.emit("duel:select_battlecmd", selectBattleCmdDTO{
		Player:   m.Player,
		Activate: activate,
		Attack:   attack,
		ToM2:     m.ToM2 != 0,
		ToEP:     m.ToEP != 0,
	})
	return nil
}

// decorateDraw 转换 MSG_DRAW：剥掉 0x80000000 公开标记。
func decorateDraw(c *WailsDuelClient, _ byte, _ *utils.YGOBuffer, msg any) error {
	m := msg.(*protocol.DrawMsg)
	cards := make([]int32, len(m.Cards))
	for i, code := range m.Cards {
		// 最高位 0x80000000 是引擎的公开标记（表侧抽卡），不是卡号的一部分
		cards[i] = code & 0x7fffffff
	}
	c.emit("duel:draw", drawDTO{
		Player: m.Player,
		Count:  m.Count,
		Cards:  cards,
	})
	return nil
}

// decorateAttack 转换 MSG_ATTACK：两处打包位置解成 {c,l,s}。
func decorateAttack(c *WailsDuelClient, _ byte, _ *utils.YGOBuffer, msg any) error {
	m := msg.(*protocol.AttackMsg)
	c.emit("duel:attack", attackDTO{
		Attacker: newLocRef(m.AttackerInfo),
		Target:   newLocRef(m.TargetInfo),
	})
	return nil
}

// decorateBecomeTarget 转换 MSG_BECOME_TARGET：打包位置解成 {c,l,s}。
func decorateBecomeTarget(c *WailsDuelClient, _ byte, _ *utils.YGOBuffer, msg any) error {
	m := msg.(*protocol.BecomeTargetMsg)
	c.emit("duel:become_target", becomeTargetDTO{Targets: newLocRefList(m.Targets)})
	return nil
}

// decorateBattle 转换 MSG_BATTLE：26 字节结算体完整透传。末位的两个单
// 字节是 ocgcore 的战破旗标 bd[0]/bd[1]（processor.cpp:2982-2987：
// calculate_battle_damage 的输出，true = 该方将被战斗破坏），payload 里
// 译为 attackerDestroyed/targetDestroyed；双方 ATK/DEF 数值与位置供
// 前端攻防对撞浮层消费（原版 duelclient.cpp:3480-3519 刷卡片攻守显示）。
func decorateBattle(c *WailsDuelClient, _ byte, _ *utils.YGOBuffer, msg any) error {
	m := msg.(*protocol.BattleMsg)
	c.emit("duel:battle", battleDTO{
		Attacker:          newLocPosRef(m.AttackerInfo),
		AttackerATK:       m.AttackerATK,
		AttackerDEF:       m.AttackerDEF,
		AttackerDestroyed: m.AttackerDirect != 0,
		Target:            newLocPosRef(m.TargetInfo),
		TargetATK:         m.TargetATK,
		TargetDEF:         m.TargetDEF,
		TargetDestroyed:   m.TargetDirect != 0,
	})
	return nil
}

// updateDataMsg / updateCardMsg 的消息体后面跟 ocgcore 的变长 query blob，
// restruct 表达不了，decorate 里用 decodeQueryBlobList 消费。
type updateDataMsg struct {
	Player   uint8 `struct:"uint8"`
	Location uint8 `struct:"uint8"`
}

type updateCardMsg struct {
	Player   uint8 `struct:"uint8"`
	Location uint8 `struct:"uint8"`
	Sequence uint8 `struct:"uint8"`
}

func decorateUpdateData(c *WailsDuelClient, _ byte, pbuf *utils.YGOBuffer, msg any) error {
	m := msg.(*updateDataMsg)
	cards, err := decodeQueryBlobList(pbuf, 0)
	if err != nil {
		return err
	}
	c.emit("duel:update_data", updateDataDTO{
		Player:   m.Player,
		Location: m.Location,
		Cards:    cards,
	})
	return nil
}

func decorateUpdateCard(c *WailsDuelClient, _ byte, pbuf *utils.YGOBuffer, msg any) error {
	m := msg.(*updateCardMsg)
	cards, err := decodeQueryBlobList(pbuf, 1)
	if err != nil {
		return err
	}
	var card *queryCardDTO
	if len(cards) > 0 {
		card = cards[0]
	}
	c.emit("duel:update_card", updateCardDTO{
		Player:   m.Player,
		Location: m.Location,
		Sequence: m.Sequence,
		Card:     card,
	})
	return nil
}

// tagSwapMsg 的消息体 = player/mcount/ecount/pcount/hcount 五字节 +
// topcode(4) + hcount*4 手牌码 + ecount*4 额外码（变长，restruct 表达不了，
// decorate 里用 pbuf 消费）。布局依据原版 duelclient.cpp:3782-3788 与
// tag_duel.cpp 的 RefreshExtra 段。
type tagSwapMsg struct {
	Player  uint8 `struct:"uint8"`
	MCount  uint8 `struct:"uint8"`
	ECount  uint8 `struct:"uint8"`
	PCount  uint8 `struct:"uint8"`
	HCount  uint8 `struct:"uint8"`
	TopCode int32 `struct:"int32"`
}

// decorateTagSwap 转换 MSG_TAG_SWAP（TAG 队友换手）。手牌码/额外码对非当前
// 操作者已被服务端就地抹零（core/duel/tag_duel.go analyzeTagSwap），这里
// 原样透传；前端据 deck/extra/hand 计数刷新牌堆与手牌（原版用同一消息重排
// dField.deck/extra/hand 并播 5 帧移动动画）。额外码按原版 & 0x7fffffff
// 去公开标记位。
func decorateTagSwap(c *WailsDuelClient, _ byte, pbuf *utils.YGOBuffer, msg any) error {
	m := msg.(*tagSwapMsg)
	readCodes := func(n int) ([]int32, error) {
		codes := make([]int32, 0, n)
		for i := 0; i < n; i++ {
			var code int32
			if err := pbuf.Read(&code); err != nil {
				return nil, err
			}
			codes = append(codes, code&0x7fffffff)
		}
		return codes, nil
	}
	hand, err := readCodes(int(m.HCount))
	if err != nil {
		return err
	}
	extra, err := readCodes(int(m.ECount))
	if err != nil {
		return err
	}
	c.emit("duel:tag_swap", map[string]interface{}{
		"player":           m.Player,
		"deckCount":        m.MCount,
		"extraCount":       m.ECount,
		"extraFaceUpCount": m.PCount,
		"handCount":        m.HCount,
		"topCode":          m.TopCode,
		"hand":             hand,
		"extra":            extra,
	})
	return nil
}

// ------------------------------------------------------------------
// query blob 解码（ocgcore card.cpp get_infos 的线上格式）
//
// 每个 blob：int32 总长（含本头部 8 字节）+ uint32 最终 query flag，
// 之后按 flag 位序（固定顺序）写入各字段，全部 uint32/int32，唯三例外：
// TARGET_CARD/OVERLAY_CARD/COUNTERS 是 count(4)+条目(4×count)，LINK 是两个
// u32（link + link_marker）。头部 flag 是「实际写出的字段」的最终集合
// （引擎缓存命中时会把未变化的字段从 flag 中剔除）。
// ------------------------------------------------------------------

// query 布局：{flag 位, 消费动作}。顺序即 get_infos 的 if 顺序，不可变。
// 返回 *queryCardDTO：字段是否出现由 flag 决定（指针 nil → json 省略），
// 解码越过 blob 边界即整体放弃（返回 nil 走 opaque skip）。
func decodeQueryBody(body []byte) *queryCardDTO {
	if len(body) < 4 {
		return nil
	}
	flag := binary.LittleEndian.Uint32(body[0:4])
	rest := body[4:]
	pos := 0
	card := &queryCardDTO{}

	u32 := func() uint32 {
		v := binary.LittleEndian.Uint32(rest[pos:])
		pos += 4
		return v
	}
	u32p := func() *uint32 {
		v := u32()
		return &v
	}
	i32p := func() *int32 {
		v := int32(binary.LittleEndian.Uint32(rest[pos:]))
		pos += 4
		return &v
	}
	count := func() int {
		n := int(int32(binary.LittleEndian.Uint32(rest[pos:])))
		pos += 4
		return n
	}
	// guard：字段消费越过 blob 边界即整体放弃（返回 nil 走 opaque skip）
	failed := false
	ensure := func(n int) {
		if pos+n > len(rest) {
			failed = true
		}
	}
	locPosRefp := func() *locPosRefDTO {
		v := newLocPosRef(u32())
		return &v
	}

	if flag&0x01 != 0 { // QUERY_CODE
		ensure(4)
		if !failed {
			card.Code = u32p()
		}
	}
	if !failed && flag&0x02 != 0 { // QUERY_POSITION（get_info_location，含灵摆覆盖形态）
		ensure(4)
		if !failed {
			card.Position = locPosRefp()
		}
	}
	if !failed && flag&0x04 != 0 { // QUERY_ALIAS
		ensure(4)
		if !failed {
			card.Alias = u32p()
		}
	}
	if !failed && flag&0x08 != 0 { // QUERY_TYPE
		ensure(4)
		if !failed {
			card.Type = u32p()
		}
	}
	if !failed && flag&0x10 != 0 { // QUERY_LEVEL（低 16 位等级，高位含灵摆刻度）
		ensure(4)
		if !failed {
			card.Level = u32p()
		}
	}
	if !failed && flag&0x20 != 0 { // QUERY_RANK
		ensure(4)
		if !failed {
			card.Rank = u32p()
		}
	}
	if !failed && flag&0x40 != 0 { // QUERY_ATTRIBUTE
		ensure(4)
		if !failed {
			card.Attribute = u32p()
		}
	}
	if !failed && flag&0x80 != 0 { // QUERY_RACE
		ensure(4)
		if !failed {
			card.Race = u32p()
		}
	}
	if !failed && flag&0x100 != 0 { // QUERY_ATTACK
		ensure(4)
		if !failed {
			card.Attack = i32p()
		}
	}
	if !failed && flag&0x200 != 0 { // QUERY_DEFENSE
		ensure(4)
		if !failed {
			card.Defense = i32p()
		}
	}
	if !failed && flag&0x400 != 0 { // QUERY_BASE_ATTACK
		ensure(4)
		if !failed {
			card.BaseAttack = i32p()
		}
	}
	if !failed && flag&0x800 != 0 { // QUERY_BASE_DEFENSE
		ensure(4)
		if !failed {
			card.BaseDefense = i32p()
		}
	}
	if !failed && flag&0x1000 != 0 { // QUERY_REASON
		ensure(4)
		if !failed {
			card.Reason = u32p()
		}
	}
	if !failed && flag&0x2000 != 0 { // QUERY_REASON_CARD
		ensure(4)
		if !failed {
			card.ReasonCard = locPosRefp()
		}
	}
	if !failed && flag&0x4000 != 0 { // QUERY_EQUIP_CARD（无装备时引擎剔除该位）
		ensure(4)
		if !failed {
			card.EquipCard = locPosRefp()
		}
	}
	if !failed && flag&0x8000 != 0 { // QUERY_TARGET_CARD
		ensure(4)
		if !failed {
			n := count()
			entries := make([]*locPosRefDTO, 0, n)
			for i := 0; i < n; i++ {
				ensure(4)
				if failed {
					break
				}
				entries = append(entries, locPosRefp())
			}
			if !failed {
				card.Targets = entries
			}
		}
	}
	if !failed && flag&0x10000 != 0 { // QUERY_OVERLAY_CARD
		ensure(4)
		if !failed {
			n := count()
			codes := make([]uint32, 0, n)
			for i := 0; i < n; i++ {
				ensure(4)
				if failed {
					break
				}
				codes = append(codes, u32())
			}
			if !failed {
				card.Overlays = codes
			}
		}
	}
	if !failed && flag&0x20000 != 0 { // QUERY_COUNTERS（每条 type | count<<16）
		ensure(4)
		if !failed {
			n := count()
			counters := make([]*counterDTO, 0, n)
			for i := 0; i < n; i++ {
				ensure(4)
				if failed {
					break
				}
				raw := u32()
				counters = append(counters, &counterDTO{Type: raw & 0xffff, Count: raw >> 16})
			}
			if !failed {
				card.Counters = counters
			}
		}
	}
	if !failed && flag&0x40000 != 0 { // QUERY_OWNER
		ensure(4)
		if !failed {
			card.Owner = i32p()
		}
	}
	if !failed && flag&0x80000 != 0 { // QUERY_STATUS
		ensure(4)
		if !failed {
			card.Status = u32p()
		}
	}
	if !failed && flag&0x200000 != 0 { // QUERY_LSCALE
		ensure(4)
		if !failed {
			card.LScale = u32p()
		}
	}
	if !failed && flag&0x400000 != 0 { // QUERY_RSCALE
		ensure(4)
		if !failed {
			card.RScale = u32p()
		}
	}
	if !failed && flag&0x800000 != 0 { // QUERY_LINK（link + link_marker 各 4 字节）
		ensure(8)
		if !failed {
			card.Link = u32p()
			card.LinkMarker = u32p()
		}
	}
	if failed {
		return nil
	}
	return card
}

// decodeQueryBlobList 解码 ocgcore query blob 列表；maxBlobs 限制最大解码
// 数量（0 = 直到缓冲区尾）。blob 的 8 字节头（长度+flag）保证
// 无论解码成败缓冲区都按整 blob 推进；解码失败的 blob 被跳过不计入结果。
// MZONE/SZONE 的空槽写 LEN_EMPTY(4) 标记（ocgapi.cpp query_field_card），
// 以 nil 占位保持输出条目与槽序号的对齐（client_field.cpp:353-359 同语义）。
// 截断（len 头大于剩余字节）视为列表结束。
func decodeQueryBlobList(pbuf *utils.YGOBuffer, maxBlobs int) ([]*queryCardDTO, error) {
	var out []*queryCardDTO
	for skipped := 0; maxBlobs == 0 || skipped < maxBlobs; skipped++ {
		if pbuf.Len() < 4 {
			return out, nil
		}
		var clen int32
		if err := pbuf.Read(&clen); err != nil {
			return nil, err
		}
		if clen == ocgcore.LEN_EMPTY {
			out = append(out, nil)
			continue
		}
		if clen < ocgcore.LEN_HEADER {
			return out, nil
		}
		body := pbuf.ReadNext(int(clen) - 4)
		if body == nil {
			// 截断：剩余字节不足一个完整 blob，按列表结束处理
			return out, nil
		}
		if card := decodeQueryBody(body); card != nil {
			out = append(out, card)
		}
	}
	return out, nil
}

// ocgcore 的 opcode 操作符（ocgcore/common.go OPCODE_*）。低于该区间的值都是
// 压栈字面量。
var opcodeOperators = map[uint32]bool{
	0x40000000: true, 0x40000001: true, 0x40000002: true, 0x40000003: true,
	0x40000004: true, 0x40000005: true, 0x40000006: true, 0x40000007: true,
	0x40000100: true, 0x40000101: true, 0x40000102: true, 0x40000103: true,
	0x40000104: true,
}

const opcodeIsCode = 0x40000100

// decodeAnnounceCandidates 把 MSG_ANNOUNCE_CARD 的后缀 opcode 表达式还原成
// 显式候选卡号列表（脚本 announce_filter 的常见形态只检查固定卡号）。
// 含种族/类型/算术过滤的表达式无法枚举 —— 返回 decodable=false，前端回退到
// 自由输入卡号。
func decodeAnnounceCandidates(values []int32) (candidates []int32, decodable bool) {
	stack := make([]int32, 0, len(values))
	for _, op := range values {
		switch {
		case !opcodeOperators[uint32(op)]:
			stack = append(stack, op)
		case uint32(op) == opcodeIsCode:
			// 后缀序（脚本先压操作数再压谓词）：<code>, OPCODE_ISCODE>
			if len(stack) == 0 {
				return nil, false
			}
			candidates = append(candidates, stack[len(stack)-1])
			stack = stack[:len(stack)-1]
		default:
			return nil, false
		}
	}
	out := append(candidates, stack...)
	filtered := out[:0]
	for _, v := range out {
		if v > 0 {
			filtered = append(filtered, v)
		}
	}
	return filtered, true
}

// decorateAnnounceCard 在透传原始 opcode 列表（options）之外，附上解码好的
// 候选卡号（candidates）与可解码标记（decodable）——JS 不再需要 opcode 机。
func decorateAnnounceCard(c *WailsDuelClient, _ byte, _ *utils.YGOBuffer, msg any) error {
	m := msg.(*protocol.AnnounceCardMsg)
	candidates, decodable := decodeAnnounceCandidates(m.Values)
	c.emit("duel:announce_card", map[string]interface{}{
		"player":     m.Player,
		"options":    m.Values,
		"candidates": candidates,
		"decodable":  decodable,
	})
	return nil
}

// decorateSelectPlace 在透传原始 flag 位域之外，附上解码好的语义化可选区域
// 列表 zones:[{player,loc,seq}]。注意引擎的 flag 是「禁用区域」位掩码
// （gframe duelclient.cpp:1832 selectable_field = ~flag），可选区域要取反：
// 低 16 位是当前操作玩家的 0x7f 主怪兽区 seq0-6、0x3f00 魔法陷阱区 seq0-5、
// 0xc000 灵摆区 seq6/7；高 16 位（0x7f0000/0x3f000000/0xc0000000）是对手的
// 同构区域（SELECT_DISFIELD 会用到），响应时 player 字节须带区域归属方。
// disfield= true 表示 MSG_SELECT_DISFIELD：前端据此禁用自动落点
// （gframe 只在 MSG_SELECT_PLACE 时查 automonsterpos/autospellpos）。
func decorateSelectPlace(c *WailsDuelClient, _ byte, _ *utils.YGOBuffer, msg any) error {
	return emitSelectPlace(c, msg, false)
}

func decorateSelectDisfield(c *WailsDuelClient, _ byte, _ *utils.YGOBuffer, msg any) error {
	return emitSelectPlace(c, msg, true)
}

func emitSelectPlace(c *WailsDuelClient, msg any, disfield bool) error {
	m := msg.(*protocol.SelectPlaceMsg)
	avail := ^uint32(m.Flag)
	type zoneRef struct {
		player int
		loc    int
		seq    int
	}
	var zones []zoneRef
	own, opp := int(m.Player), 1-int(m.Player)
	scan := func(player, loc int, base uint32, seqs []int) {
		for _, seq := range seqs {
			if avail&(base<<uint(seq)) != 0 {
				zones = append(zones, zoneRef{player: player, loc: loc, seq: seq})
			}
		}
	}
	mzoneSeqs := []int{0, 1, 2, 3, 4, 5, 6}
	szoneSeqs := []int{0, 1, 2, 3, 4, 5}
	scan(own, 0x04, 0x1, mzoneSeqs)   // 己方主怪兽区
	scan(own, 0x08, 0x100, szoneSeqs) // 己方魔法陷阱区
	if avail&0x4000 != 0 {
		zones = append(zones, zoneRef{player: own, loc: 0x08, seq: 6})
	}
	if avail&0x8000 != 0 {
		zones = append(zones, zoneRef{player: own, loc: 0x08, seq: 7})
	}
	scan(opp, 0x04, 0x10000, mzoneSeqs)   // 对手主怪兽区
	scan(opp, 0x08, 0x1000000, szoneSeqs) // 对手魔法陷阱区
	if avail&0x40000000 != 0 {
		zones = append(zones, zoneRef{player: opp, loc: 0x08, seq: 6})
	}
	if avail&0x80000000 != 0 {
		zones = append(zones, zoneRef{player: opp, loc: 0x08, seq: 7})
	}
	zoneMaps := make([]map[string]interface{}, len(zones))
	for i, z := range zones {
		zoneMaps[i] = map[string]interface{}{"player": z.player, "loc": z.loc, "seq": z.seq}
	}
	c.emit("duel:select_place", map[string]interface{}{
		"player":   m.Player,
		"count":    m.Count,
		"flag":     m.Flag,
		"disfield": disfield,
		"zones":    zoneMaps,
	})
	return nil
}

// decorateConfirmCards 转换 MSG_CONFIRM_CARDS：skip_panel 补进 payload，
// 卡片条目按 ConfirmCardEntry 的 json tag 直接对齐前端。
func decorateConfirmCards(c *WailsDuelClient, _ byte, _ *utils.YGOBuffer, msg any) error {
	m := msg.(*protocol.ConfirmCardsMsg)
	c.emit("duel:confirm_cards", map[string]interface{}{
		"player":    m.Player,
		"skipPanel": m.SkipPanel != 0,
		"cards":     m.Cards,
	})
	return nil
}

// decorateRefreshDeck 转换 MSG_REFRESH_DECK（player 字段结构体里是 json:"-"）。
func decorateRefreshDeck(c *WailsDuelClient, _ byte, _ *utils.YGOBuffer, msg any) error {
	m := msg.(*protocol.RefreshDeckMsg)
	c.emit("duel:refresh_deck", map[string]interface{}{"player": m.Player})
	return nil
}

// decorateShuffleSetCard 转换 MSG_SHUFFLE_SET_CARD：把盖卡条目的打包位置
// 解成 {c,l,s}（原版 gframe 用它重排同区盖卡 mesh 的画位）。
func decorateShuffleSetCard(c *WailsDuelClient, _ byte, _ *utils.YGOBuffer, msg any) error {
	m := msg.(*protocol.ShuffleSetCardMsg)
	cards := make([]map[string]interface{}, len(m.Cards))
	for i, e := range m.Cards {
		cc, cl, cs, cp := protocol.UnpackPackedLoc(e.Info)
		cards[i] = map[string]interface{}{"code": e.Code, "c": cc, "l": cl, "s": cs, "p": cp}
	}
	c.emit("duel:shuffle_set_card", map[string]interface{}{
		"loc":   m.Loc,
		"cards": cards,
	})
	return nil
}

// decorateFieldDisabled 转换 MSG_FIELD_DISABLED。
func decorateFieldDisabled(c *WailsDuelClient, _ byte, _ *utils.YGOBuffer, msg any) error {
	m := msg.(*protocol.FieldDisabledMsg)
	c.emit("duel:field_disabled", map[string]interface{}{"zones": m.Zones})
	return nil
}

// decorateEquip 转换 MSG_EQUIP / MSG_CARD_TARGET / MSG_CANCEL_TARGET
// （三者布局一致：装备(目标)卡 info + 对象卡 info）。
func decorateEquip(c *WailsDuelClient, _ byte, _ *utils.YGOBuffer, msg any) error {
	m := msg.(*protocol.EquipMsg)
	c.emit("duel:equip", map[string]interface{}{
		"card":   packedLocEntry(m.CardInfo),
		"target": packedLocEntry(m.TargetInfo),
	})
	return nil
}

func decorateCardTarget(c *WailsDuelClient, _ byte, _ *utils.YGOBuffer, msg any) error {
	m := msg.(*protocol.CardTargetMsg)
	c.emit("duel:card_target", map[string]interface{}{
		"card":   packedLocEntry(m.CardInfo),
		"target": packedLocEntry(m.TargetInfo),
	})
	return nil
}

func decorateCancelTarget(c *WailsDuelClient, _ byte, _ *utils.YGOBuffer, msg any) error {
	m := msg.(*protocol.CardTargetMsg)
	c.emit("duel:cancel_target", map[string]interface{}{
		"card":   packedLocEntry(m.CardInfo),
		"target": packedLocEntry(m.TargetInfo),
	})
	return nil
}

// decorateUnequip 转换 MSG_UNEQUIP（4 字节 info_location）。
func decorateUnequip(c *WailsDuelClient, _ byte, _ *utils.YGOBuffer, msg any) error {
	m := msg.(*protocol.UnequipMsg)
	c.emit("duel:unequip", map[string]interface{}{"card": packedLocEntry(m.Info)})
	return nil
}

// decorateMissedEffect 转换 MSG_MISSED_EFFECT。
func decorateMissedEffect(c *WailsDuelClient, _ byte, _ *utils.YGOBuffer, msg any) error {
	m := msg.(*protocol.MissedEffectMsg)
	c.emit("duel:missed_effect", map[string]interface{}{
		"card": packedLocEntry(m.InfoLocation),
		"code": m.Code,
	})
	return nil
}

// decorateCardHint 转换 MSG_CARD_HINT（CHINT_DESC_ADD/REMOVE 效果角标、
// CHINT_TURN 回合数大字）。
func decorateCardHint(c *WailsDuelClient, _ byte, _ *utils.YGOBuffer, msg any) error {
	m := msg.(*protocol.CardHintMsg)
	c.emit("duel:card_hint", map[string]interface{}{
		"card": packedLocEntry(m.Info),
		"type": m.Type,
		"data": m.Data,
	})
	return nil
}

// decoratePlayerHint 转换 MSG_PLAYER_HINT（player_desc_hints / 禁查墓地）。
func decoratePlayerHint(c *WailsDuelClient, _ byte, _ *utils.YGOBuffer, msg any) error {
	m := msg.(*protocol.PlayerHintMsg)
	c.emit("duel:player_hint", map[string]interface{}{
		"player": m.Player,
		"type":   m.Type,
		"data":   m.Data,
	})
	return nil
}

// ------------------------------------------------------------------
// 单人谜题的 Debug 消息（source/ygopro/ocgcore/libdebug.cpp）
//
//   - MSG_AI_NAME / MSG_SHOW_HINT（libdebug.cpp:168-190）：u16 长度 + UTF-8
//     字节串 + NUL。原版 gframe 把 AI_NAME 写进 clientname、SHOW_HINT 弹
//     wMessage 消息框（客户端侧阻塞，引擎继续走到下一个真正的 select）。
//   - MSG_RELOAD_FIELD（field.cpp reload_field_info）：rule(1) + 每玩家
//     [lp(4), mzone 7×(0 | 1+pos+overlay), szone 8×(0 | 1+pos),
//     6 计数] + 链数(1) + 15 字节/链。
// ------------------------------------------------------------------

// rawEngineMsg 是布局完全由 decorate 手工解析的消息的占位结构体（Unpack
// 消费 0 字节，实际字节流在 decorate 里从 pbuf 读取）。
type rawEngineMsg struct{}

// decorateAIName 转换 MSG_AI_NAME。
func decorateAIName(c *WailsDuelClient, _ byte, pbuf *utils.YGOBuffer, _ any) error {
	name, err := readNullTerminated16(pbuf)
	if err != nil {
		return err
	}
	c.emit("duel:ai_name", map[string]interface{}{"name": name})
	return nil
}

// decorateShowHint 转换 MSG_SHOW_HINT。
func decorateShowHint(c *WailsDuelClient, _ byte, pbuf *utils.YGOBuffer, _ any) error {
	text, err := readNullTerminated16(pbuf)
	if err != nil {
		return err
	}
	c.emit("duel:show_hint", map[string]interface{}{"text": text})
	return nil
}

// readNullTerminated16 读取 libdebug 的字符串布局：u16 长度 + 字节串 + NUL。
func readNullTerminated16(pbuf *utils.YGOBuffer) (string, error) {
	var length uint16
	if err := pbuf.Read(&length); err != nil {
		return "", err
	}
	body := pbuf.ReadNext(int(length))
	if body == nil {
		return "", fmt.Errorf("truncated string body (%d bytes)", length)
	}
	pbuf.Next(1) // NUL 终止符
	return string(body), nil
}

// parseReloadFieldBody walks a MSG_RELOAD_FIELD body（rule 起，opcode 之后；
// 布局见 field.cpp reload_field_info / ocgapi.cpp query_field_info 的输出）
// and returns the decoded fields plus the body bytes consumed — 单机模式拿
// query_field_info 的固定 256 字节缓冲时，consumed+1 用来截掉尾部残留。
func parseReloadFieldBody(body []byte) (map[string]interface{}, int, bool) {
	p := 0
	u32 := func() (uint32, bool) {
		if p+4 > len(body) {
			return 0, false
		}
		v := binary.LittleEndian.Uint32(body[p:])
		p += 4
		return v, true
	}
	if p+1 > len(body) {
		return nil, 0, false
	}
	rule := body[p]
	p++
	players := make([]map[string]interface{}, 2)
	for pl := 0; pl < 2; pl++ {
		lp, ok := u32()
		if !ok {
			return nil, 0, false
		}
		mzone := make([]map[string]interface{}, 7)
		for i := 0; i < 7; i++ {
			if p+1 > len(body) {
				return nil, 0, false
			}
			if body[p] == 0 {
				p++
				continue
			}
			if p+3 > len(body) {
				return nil, 0, false
			}
			mzone[i] = map[string]interface{}{"pos": body[p+1], "overlay": body[p+2]}
			p += 3
		}
		szone := make([]map[string]interface{}, 8)
		for i := 0; i < 8; i++ {
			if p+1 > len(body) {
				return nil, 0, false
			}
			if body[p] == 0 {
				p++
				continue
			}
			if p+2 > len(body) {
				return nil, 0, false
			}
			szone[i] = map[string]interface{}{"pos": body[p+1]}
			p += 2
		}
		if p+6 > len(body) {
			return nil, 0, false
		}
		counts := body[p : p+6]
		p += 6
		players[pl] = map[string]interface{}{
			"lp":      int32(lp),
			"mzone":   mzone,
			"szone":   szone,
			"deck":    counts[0],
			"hand":    counts[1],
			"grave":   counts[2],
			"removed": counts[3],
			"extra":   counts[4],
			"extraP":  counts[5],
		}
	}
	if p+1 > len(body) {
		return nil, 0, false
	}
	chainCount := int(body[p])
	p++
	if p+chainCount*15 > len(body) {
		return nil, 0, false
	}
	p += chainCount * 15
	return map[string]interface{}{
		"rule":       rule,
		"players":    players,
		"chainCount": chainCount,
	}, p, true
}

// decorateReloadField 转换 MSG_RELOAD_FIELD：完整布场快照（谜题重载、
// 单机/观战中途加入都会出现）。
func decorateReloadField(c *WailsDuelClient, _ byte, pbuf *utils.YGOBuffer, _ any) error {
	// pbuf 的游标在 opcode 之后，body 从 rule 字节开始
	fields, consumed, ok := parseReloadFieldBody(pbuf.Bytes())
	if !ok {
		return fmt.Errorf("reload field: truncated body")
	}
	pbuf.Next(consumed)
	c.emit("duel:reload_field", fields)
	return nil
}

// engineBindings 是引擎消息类型 → 解析/转发行为的总表。
// 不在表内的消息落入 game_msg_skip.go 的残余 skip 分支。
var engineBindings = map[byte]engineBinding{
	// ---- 状态/展示消息（结构体 json tag 直接对齐前端键名）----
	ocgcore.MSG_START:               {name: "START", newMsg: func() any { return &protocol.StartMsg{} }, event: "duel:start"},
	ocgcore.MSG_NEW_TURN:            {name: "NEW_TURN", newMsg: func() any { return &protocol.NewTurnMsg{} }, event: "duel:new_turn"},
	ocgcore.MSG_NEW_PHASE:           {name: "NEW_PHASE", newMsg: func() any { return &protocol.NewPhaseMsg{} }, event: "duel:new_phase"},
	ocgcore.MSG_MOVE:                {name: "MOVE", newMsg: func() any { return &protocol.MoveMsg{} }, event: "duel:move"},
	ocgcore.MSG_POS_CHANGE:          {name: "POS_CHANGE", newMsg: func() any { return &protocol.PosChangeMsg{} }, event: "duel:pos_change"},
	ocgcore.MSG_SET:                 {name: "SET", newMsg: func() any { return &protocol.SetMsg{} }, event: "duel:set"},
	ocgcore.MSG_SUMMONING:           {name: "SUMMONING", newMsg: func() any { return &protocol.SummoningMsg{} }, event: "duel:summoning"},
	ocgcore.MSG_SPSUMMONING:         {name: "SPSUMMONING", newMsg: func() any { return &protocol.SPSummoningMsg{} }, event: "duel:spsummoning"},
	ocgcore.MSG_FLIPSUMMONING:       {name: "FLIPSUMMONING", newMsg: func() any { return &protocol.FlipSummoningMsg{} }, event: "duel:flipsummoning"},
	ocgcore.MSG_CHAINING:            {name: "CHAINING", newMsg: func() any { return &protocol.ChainingMsg{} }, event: "duel:chaining"},
	ocgcore.MSG_CHAINED:             {name: "CHAINED", newMsg: func() any { return &protocol.ChainedMsg{} }, event: "duel:chained"},
	ocgcore.MSG_CHAIN_SOLVING:       {name: "CHAIN_SOLVING", newMsg: func() any { return &protocol.ChainSolvingMsg{} }, event: "duel:chain_solving"},
	ocgcore.MSG_CHAIN_SOLVED:        {name: "CHAIN_SOLVED", newMsg: func() any { return &protocol.ChainSolvedMsg{} }, event: "duel:chain_solved"},
	ocgcore.MSG_CHAIN_NEGATED:       {name: "CHAIN_NEGATED", newMsg: func() any { return &protocol.ChainNegatedMsg{} }, event: "duel:chain_negated"},
	ocgcore.MSG_CHAIN_DISABLED:      {name: "CHAIN_DISABLED", newMsg: func() any { return &protocol.ChainNegatedMsg{} }, event: "duel:chain_negated"},
	ocgcore.MSG_DAMAGE:              {name: "DAMAGE", newMsg: func() any { return &protocol.DamageMsg{} }, event: "duel:damage"},
	ocgcore.MSG_RECOVER:             {name: "RECOVER", newMsg: func() any { return &protocol.RecoverMsg{} }, event: "duel:recover"},
	ocgcore.MSG_LPUPDATE:            {name: "LPUPDATE", newMsg: func() any { return &protocol.LPUpdateMsg{} }, event: "duel:lp_update"},
	ocgcore.MSG_PAY_LPCOST:          {name: "PAY_LPCOST", newMsg: func() any { return &protocol.PayLPCostMsg{} }, event: "duel:pay_lpcost"},
	ocgcore.MSG_WIN:                 {name: "WIN", newMsg: func() any { return &protocol.WinMsg{} }, event: "duel:win"},
	ocgcore.MSG_TOSS_COIN:           {name: "TOSS_COIN", newMsg: func() any { return &protocol.TossCoinMsg{} }, event: "duel:toss_coin"},
	ocgcore.MSG_TOSS_DICE:           {name: "TOSS_DICE", newMsg: func() any { return &protocol.TossDiceMsg{} }, event: "duel:toss_dice"},
	ocgcore.MSG_ROCK_PAPER_SCISSORS: {name: "RPS", newMsg: func() any { return &protocol.RockPaperScissorsMsg{} }, event: "duel:rps"},
	ocgcore.MSG_HAND_RES:            {name: "HAND_RES", newMsg: func() any { return &protocol.HandResMsg{} }, event: "duel:hand_res"},
	ocgcore.MSG_CONFIRM_DECKTOP:     {name: "CONFIRM_DECKTOP", newMsg: func() any { return &protocol.ConfirmDeckTopMsg{} }, event: "duel:confirm_decktop"},
	ocgcore.MSG_CONFIRM_EXTRATOP:    {name: "CONFIRM_EXTRATOP", newMsg: func() any { return &protocol.ConfirmExtraTopMsg{} }, event: "duel:confirm_decktop"},

	// ---- 需要派生字段的消息 ----
	ocgcore.MSG_DRAW:                 {name: "DRAW", newMsg: func() any { return &protocol.DrawMsg{} }, event: "duel:draw", decorate: decorateDraw},
	ocgcore.MSG_ATTACK:               {name: "ATTACK", newMsg: func() any { return &protocol.AttackMsg{} }, event: "duel:attack", decorate: decorateAttack},
	ocgcore.MSG_BATTLE:               {name: "BATTLE", newMsg: func() any { return &protocol.BattleMsg{} }, event: "duel:battle", decorate: decorateBattle},
	ocgcore.MSG_BECOME_TARGET:        {name: "BECOME_TARGET", newMsg: func() any { return &protocol.BecomeTargetMsg{} }, event: "duel:become_target", decorate: decorateBecomeTarget},
	ocgcore.MSG_SELECT_CARD:          {name: "SELECT_CARD", newMsg: func() any { return &protocol.SelectCardMsg{} }, event: "duel:select_card", decorate: decorateSelectCard},
	ocgcore.MSG_SELECT_TRIBUTE:       {name: "SELECT_TRIBUTE", newMsg: func() any { return &protocol.SelectCardMsg{} }, event: "duel:select_card", decorate: decorateSelectCard},
	ocgcore.MSG_SELECT_UNSELECT_CARD: {name: "SELECT_UNSELECT_CARD", newMsg: func() any { return &protocol.SelectUnselectCardMsg{} }, event: "duel:select_unselect", decorate: decorateSelectUnselect},
	ocgcore.MSG_SELECT_CHAIN:         {name: "SELECT_CHAIN", newMsg: func() any { return &protocol.SelectChainMsg{} }, event: "duel:select_chain", decorate: decorateSelectChain},
	ocgcore.MSG_SELECT_IDLECMD:       {name: "SELECT_IDLECMD", newMsg: func() any { return &protocol.SelectIdleCmdMsg{} }, event: "duel:select_idlecmd", decorate: decorateIdleCmd},
	ocgcore.MSG_SELECT_BATTLECMD:     {name: "SELECT_BATTLECMD", newMsg: func() any { return &protocol.SelectBattleCmdMsg{} }, event: "duel:select_battlecmd", decorate: decorateBattleCmd},
	ocgcore.MSG_UPDATE_DATA:          {name: "UPDATE_DATA", newMsg: func() any { return &updateDataMsg{} }, event: "duel:update_data", decorate: decorateUpdateData},
	ocgcore.MSG_UPDATE_CARD:          {name: "UPDATE_CARD", newMsg: func() any { return &updateCardMsg{} }, event: "duel:update_card", decorate: decorateUpdateCard},
	// TAG 队友换手（变长体：5 字节计数 + topcode + hcount*4 手牌码 + ecount*4 额外码）
	ocgcore.MSG_TAG_SWAP:             {name: "TAG_SWAP", newMsg: func() any { return &tagSwapMsg{} }, event: "duel:tag_swap", decorate: decorateTagSwap},

	// ---- 交互提示（结构体直接 emit）----
	ocgcore.MSG_SELECT_EFFECTYN: {name: "SELECT_EFFECTYN", newMsg: func() any { return &protocol.SelectEffectYNMsg{} }, event: "duel:select_effectyn"},
	ocgcore.MSG_SELECT_YESNO:    {name: "SELECT_YESNO", newMsg: func() any { return &protocol.SelectYesNoMsg{} }, event: "duel:select_yesno"},
	ocgcore.MSG_SELECT_OPTION:   {name: "SELECT_OPTION", newMsg: func() any { return &protocol.SelectOptionMsg{} }, event: "duel:select_option"},
	ocgcore.MSG_SELECT_POSITION: {name: "SELECT_POSITION", newMsg: func() any { return &protocol.SelectPositionMsg{} }, event: "duel:select_position"},
	ocgcore.MSG_SELECT_PLACE:    {name: "SELECT_PLACE", newMsg: func() any { return &protocol.SelectPlaceMsg{} }, event: "duel:select_place", decorate: decorateSelectPlace},
	ocgcore.MSG_SELECT_DISFIELD: {name: "SELECT_DISFIELD", newMsg: func() any { return &protocol.SelectPlaceMsg{} }, event: "duel:select_place", decorate: decorateSelectDisfield},
	ocgcore.MSG_SELECT_COUNTER:  {name: "SELECT_COUNTER", newMsg: func() any { return &protocol.SelectCounterMsg{} }, event: "duel:select_counter"},
	ocgcore.MSG_SELECT_SUM:      {name: "SELECT_SUM", newMsg: func() any { return &protocol.SelectSumMsg{} }, event: "duel:select_sum"},
	ocgcore.MSG_SORT_CARD:       {name: "SORT_CARD", newMsg: func() any { return &protocol.SortCardMsg{} }, event: "duel:sort_card"},
	// MSG_SORT_CHAIN（上游 edo9300 核心的 SortCard is_chain 变体）：体布局与
	// SORT_CARD 相同（player(1)+count(1)+count×7），响应同为排列字节，
	// 故复用同一消息结构与 duel:sort_card 事件（frontend PromptHost 同一 UI）。
	ocgcore.MSG_SORT_CHAIN:      {name: "SORT_CHAIN", newMsg: func() any { return &protocol.SortCardMsg{} }, event: "duel:sort_card"},
	ocgcore.MSG_ANNOUNCE_RACE:   {name: "ANNOUNCE_RACE", newMsg: func() any { return &protocol.AnnounceRaceMsg{} }, event: "duel:announce_race"},
	ocgcore.MSG_ANNOUNCE_ATTRIB: {name: "ANNOUNCE_ATTRIB", newMsg: func() any { return &protocol.AnnounceAttribMsg{} }, event: "duel:announce_attrib"},
	ocgcore.MSG_ANNOUNCE_CARD:   {name: "ANNOUNCE_CARD", newMsg: func() any { return &protocol.AnnounceCardMsg{} }, event: "duel:announce_card", decorate: decorateAnnounceCard},
	ocgcore.MSG_ANNOUNCE_NUMBER: {name: "ANNOUNCE_NUMBER", newMsg: func() any { return &protocol.AnnounceNumberMsg{} }, event: "duel:announce_number"},

	// ---- 无数据事件 ----
	ocgcore.MSG_SUMMONED:        {name: "SUMMONED", event: "duel:summoned"},
	ocgcore.MSG_SPSUMMONED:      {name: "SPSUMMONED", event: "duel:spsummoned"},
	ocgcore.MSG_FLIPSUMMONED:    {name: "FLIPSUMMONED", event: "duel:flipsummoned"},
	ocgcore.MSG_CHAIN_END:       {name: "CHAIN_END", event: "duel:chain_end"},
	ocgcore.MSG_ATTACK_DISABLED: {name: "ATTACK_DISABLED", event: "duel:attack_disabled"},
	ocgcore.MSG_WAITING:         {name: "WAITING", event: "duel:waiting"},

	// ---- 展示/标记消息（波 B：原先只对齐字节流，现在完整 emit）----
	ocgcore.MSG_HINT:             {name: "HINT", newMsg: func() any { return &protocol.HintMsg{} }, event: "duel:hint"},
	ocgcore.MSG_CONFIRM_CARDS:    {name: "CONFIRM_CARDS", newMsg: func() any { return &protocol.ConfirmCardsMsg{} }, event: "duel:confirm_cards", decorate: decorateConfirmCards},
	ocgcore.MSG_SHUFFLE_HAND:     {name: "SHUFFLE_HAND", newMsg: func() any { return &protocol.ShuffleHandMsg{} }, event: "duel:shuffle_hand"},
	ocgcore.MSG_SHUFFLE_EXTRA:    {name: "SHUFFLE_EXTRA", newMsg: func() any { return &protocol.ShuffleExtraMsg{} }, event: "duel:shuffle_extra"},
	ocgcore.MSG_REFRESH_DECK:     {name: "REFRESH_DECK", newMsg: func() any { return &protocol.RefreshDeckMsg{} }, event: "duel:refresh_deck", decorate: decorateRefreshDeck},
	ocgcore.MSG_SHUFFLE_SET_CARD: {name: "SHUFFLE_SET_CARD", newMsg: func() any { return &protocol.ShuffleSetCardMsg{} }, event: "duel:shuffle_set_card", decorate: decorateShuffleSetCard},
	ocgcore.MSG_DECK_TOP:         {name: "DECK_TOP", newMsg: func() any { return &protocol.DeckTopMsg{} }, event: "duel:deck_top"},
	ocgcore.MSG_SWAP:             {name: "SWAP", newMsg: func() any { return &protocol.SwapMsg{} }, event: "duel:swap"},
	ocgcore.MSG_FIELD_DISABLED:   {name: "FIELD_DISABLED", newMsg: func() any { return &protocol.FieldDisabledMsg{} }, event: "duel:field_disabled", decorate: decorateFieldDisabled},
	ocgcore.MSG_CARD_SELECTED:    {name: "CARD_SELECTED", newMsg: func() any { return &protocol.CardSelectedMsg{} }, event: "duel:card_selected"},
	ocgcore.MSG_RANDOM_SELECTED:  {name: "RANDOM_SELECTED", newMsg: func() any { return &protocol.RandomSelectedMsg{} }, event: "duel:random_selected"},
	ocgcore.MSG_EQUIP:            {name: "EQUIP", newMsg: func() any { return &protocol.EquipMsg{} }, event: "duel:equip", decorate: decorateEquip},
	ocgcore.MSG_CARD_TARGET:      {name: "CARD_TARGET", newMsg: func() any { return &protocol.CardTargetMsg{} }, event: "duel:card_target", decorate: decorateCardTarget},
	ocgcore.MSG_CANCEL_TARGET:    {name: "CANCEL_TARGET", newMsg: func() any { return &protocol.CardTargetMsg{} }, event: "duel:cancel_target", decorate: decorateCancelTarget},
	ocgcore.MSG_UNEQUIP:          {name: "UNEQUIP", newMsg: func() any { return &protocol.UnequipMsg{} }, event: "duel:unequip", decorate: decorateUnequip},
	ocgcore.MSG_ADD_COUNTER:      {name: "ADD_COUNTER", newMsg: func() any { return &protocol.CounterMsg{} }, event: "duel:add_counter"},
	ocgcore.MSG_REMOVE_COUNTER:   {name: "REMOVE_COUNTER", newMsg: func() any { return &protocol.CounterMsg{} }, event: "duel:remove_counter"},
	ocgcore.MSG_MISSED_EFFECT:    {name: "MISSED_EFFECT", newMsg: func() any { return &protocol.MissedEffectMsg{} }, event: "duel:missed_effect", decorate: decorateMissedEffect},
	ocgcore.MSG_CARD_HINT:        {name: "CARD_HINT", newMsg: func() any { return &protocol.CardHintMsg{} }, event: "duel:card_hint", decorate: decorateCardHint},
	ocgcore.MSG_PLAYER_HINT:      {name: "PLAYER_HINT", newMsg: func() any { return &protocol.PlayerHintMsg{} }, event: "duel:player_hint", decorate: decoratePlayerHint},
	ocgcore.MSG_MATCH_KILL:       {name: "MATCH_KILL", newMsg: func() any { return &protocol.MatchKillMsg{} }, event: "duel:match_kill"},

	// ---- 单人谜题的 Debug 消息（libdebug.cpp；布局由 decorate 手工解析）----
	ocgcore.MSG_AI_NAME:      {name: "AI_NAME", newMsg: func() any { return &rawEngineMsg{} }, event: "duel:ai_name", decorate: decorateAIName},
	ocgcore.MSG_SHOW_HINT:    {name: "SHOW_HINT", newMsg: func() any { return &rawEngineMsg{} }, event: "duel:show_hint", decorate: decorateShowHint},
	ocgcore.MSG_RELOAD_FIELD: {name: "RELOAD_FIELD", newMsg: func() any { return &rawEngineMsg{} }, event: "duel:reload_field", decorate: decorateReloadField},
}
