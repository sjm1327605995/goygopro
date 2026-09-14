package protocol

import (
	"encoding/binary"

	"github.com/go-restruct/restruct"
)

// 本文件是引擎游戏消息（STOC_GAME_MSG 载荷，不含开头 1 字节消息类型）的
// restruct 结构体定义。所有布局的权威依据是本仓库自带的引擎源码
// source/ygopro/ocgcore/（编译进 ocgcore.dll 的那一套）：
//
//   - processor.cpp / operations.cpp / playerop.cpp / field.cpp / libduel.cpp
//     中的 write_buffer 序列；
//   - card.cpp: card::get_info_location() = c | (l << 8) | (s << 16) | (pos << 24)，
//     小端写出后即 4 个独立字节 c, l, s, pos。
//
// json tag 对齐 cmd/wails 客户端发给前端的事件键名（c/l/s/cc/cl/cs/cp 等），
// 前端事件消费代码依赖这些键名，改动前先核对 frontend/src/duel/duel_manager.js。

// ------------------------------------------------------------------
// MSG_START — 对局开始
// 布局见 gframe/single_duel.cpp: playertype(1) duelrule(1) lp0(4) lp1(4)
// deck0(2) extra0(2) deck1(2) extra1(2) = 18 bytes
// （该消息由服务端而非引擎拼装，双打时 playertype 另有 0x9/0x1a 等值）
// ------------------------------------------------------------------
type StartMsg struct {
	PlayerType uint8  `struct:"uint8" json:"playerType"`
	DuelRule   uint8  `struct:"uint8" json:"duelRule"`
	LP0        int32  `struct:"int32" json:"lp0"`
	LP1        int32  `struct:"int32" json:"lp1"`
	Deck0      uint16 `struct:"uint16" json:"deck0"`
	Extra0     uint16 `struct:"uint16" json:"extra0"`
	Deck1      uint16 `struct:"uint16" json:"deck1"`
	Extra1     uint16 `struct:"uint16" json:"extra1"`
}

// ------------------------------------------------------------------
// 通用基础类型
// ------------------------------------------------------------------

// CardCode 卡片代码，在 YGO 消息中常见的高字节携带位置信息的 int32
// 实际只用到低 24 位，最高位(0x80000000)有时用于标记
// 在 struct 中直接用 int32 即可，需要时再提取高位
type CardCode = int32

// PackedLoc 引擎打包的位置字段（get_info_location / get_select_info_location），
// 小端展开后 4 字节依次是 controller, location, sequence, position。
func UnpackPackedLoc(v uint32) (c, l, s, pos uint8) {
	return uint8(v & 0xff), uint8((v >> 8) & 0xff), uint8((v >> 16) & 0xff), uint8((v >> 24) & 0xff)
}

// PackPackedLoc 是 UnpackPackedLoc 的逆操作。
func PackPackedLoc(c, l, s, pos uint8) uint32 {
	return uint32(c) | uint32(l)<<8 | uint32(s)<<16 | uint32(pos)<<24
}

// ------------------------------------------------------------------
// MSG_SELECT_BATTLECMD — 选择战斗指令
// per processor.cpp: player(1),
// 激活数(1) + 激活条目 × n [code(4) + c(1) + l(1) + s(1) + description(4) = 11B]，
// 攻击数(1) + 攻击条目 × n [code(4) + c(1) + l(1) + s(1) + direct(1) = 8B]，
// toM2(1) + toEP(1)
// ------------------------------------------------------------------

// CmdActivateEntry 可发动效果的条目（BATTLECMD/IDLECMD 通用）
type CmdActivateEntry struct {
	Code        uint32 `struct:"uint32" json:"code"`
	CC          uint8  `struct:"uint8" json:"cc"`
	CL          uint8  `struct:"uint8" json:"cl"`
	CS          uint8  `struct:"uint8" json:"cs"`
	Description uint32 `struct:"uint32" json:"desc"`
}

// CmdCardEntry 卡片操作条目（召唤/特殊召唤/改变表示形式/盖放等）
type CmdCardEntry struct {
	Code uint32 `struct:"uint32" json:"code"`
	CC   uint8  `struct:"uint8" json:"cc"`
	CL   uint8  `struct:"uint8" json:"cl"`
	CS   uint8  `struct:"uint8" json:"cs"`
}

// CmdAttackEntry 可攻击宣言的条目
type CmdAttackEntry struct {
	Code             uint32 `struct:"uint32" json:"code"`
	CC               uint8  `struct:"uint8" json:"cc"`
	CL               uint8  `struct:"uint8" json:"cl"`
	CS               uint8  `struct:"uint8" json:"cs"`
	DirectAttackable uint8  `struct:"uint8" json:"direct"`
}

type SelectBattleCmdMsg struct {
	Player uint8              `struct:"uint8" json:"player"`
	CountA uint8              `struct:"uint8,sizeof=CmdsA" json:"-"`
	CmdsA  []CmdActivateEntry `json:"activate"`
	CountB uint8              `struct:"uint8,sizeof=CmdsB" json:"-"`
	CmdsB  []CmdAttackEntry   `json:"attack"`
	ToM2   uint8              `struct:"uint8" json:"toM2"`
	ToEP   uint8              `struct:"uint8" json:"toEP"`
}

// ------------------------------------------------------------------
// MSG_SELECT_IDLECMD — 选择空闲指令
// per playerop.cpp: player(1),
// 召唤/特殊召唤/表示形式/怪兽盖放/魔法盖放各一组 [count(1) + 条目(7B: code4+c+l+s)]，
// 激活组 [count(1) + 激活条目(11B: code4+c+l+s+desc4)]，toBP(1) + toEP(1) + shuffle(1)
// ------------------------------------------------------------------
type SelectIdleCmdMsg struct {
	Player     uint8              `struct:"uint8" json:"player"`
	CountA     uint8              `struct:"uint8,sizeof=CmdsA" json:"-"`
	CmdsA      []CmdCardEntry     `json:"summon"`
	CountB     uint8              `struct:"uint8,sizeof=CmdsB" json:"-"`
	CmdsB      []CmdCardEntry     `json:"spsummon"`
	CountC     uint8              `struct:"uint8,sizeof=CmdsC" json:"-"`
	CmdsC      []CmdCardEntry     `json:"repos"`
	CountD     uint8              `struct:"uint8,sizeof=CmdsD" json:"-"`
	CmdsD      []CmdCardEntry     `json:"mset"`
	CountE     uint8              `struct:"uint8,sizeof=CmdsE" json:"-"`
	CmdsE      []CmdCardEntry     `json:"sset"`
	CountF     uint8              `struct:"uint8,sizeof=CmdsF" json:"-"`
	CmdsF      []CmdActivateEntry `json:"activate"`
	ToBP       uint8              `struct:"uint8" json:"toBP"`
	ToEP       uint8              `struct:"uint8" json:"toEP"`
	CanShuffle uint8              `struct:"uint8" json:"shuffle"`
}

// ------------------------------------------------------------------
// MSG_SELECT_EFFECTYN — 选择是否发动效果 (13 bytes)
// per playerop.cpp: player(1), code(4), get_info_location(4), description(4)
// ------------------------------------------------------------------
type SelectEffectYNMsg struct {
	Player       uint8  `struct:"uint8" json:"player"`
	Code         uint32 `struct:"uint32" json:"code"`
	InfoLocation uint32 `struct:"uint32" json:"loc"`
	Description  uint32 `struct:"uint32" json:"desc"`
}

// ------------------------------------------------------------------
// MSG_SELECT_YESNO — 选择是/否 (5 bytes)
// per playerop.cpp: player(1), description(4)
// ------------------------------------------------------------------
type SelectYesNoMsg struct {
	Player      uint8  `struct:"uint8" json:"player"`
	Description uint32 `struct:"uint32" json:"desc"`
}

// ------------------------------------------------------------------
// MSG_SELECT_OPTION — 选择选项
// per playerop.cpp: player(1), count(1), int32[count] options
// ------------------------------------------------------------------
type SelectOptionMsg struct {
	Player  uint8   `struct:"uint8" json:"player"`
	Count   uint8   `struct:"uint8,sizeof=Options" json:"-"`
	Options []int32 `struct:"[]int32" json:"options"`
}

// ------------------------------------------------------------------
// MSG_SELECT_CARD / MSG_SELECT_TRIBUTE — 选择卡片 / 祭品
// per playerop.cpp: player(1) cancelable(1) min(1) max(1) count(1)，
// 条目 = code(4) + get_select_info_location(4) = 8 bytes
// （位置 4 字节依次为 c, l, s, 选择序号/pos）
// ------------------------------------------------------------------
type SelectCardEntry struct {
	Code       int32 `struct:"int32" json:"code"`
	Controller uint8 `struct:"uint8" json:"c"`
	Location   uint8 `struct:"uint8" json:"l"`
	Sequence   uint8 `struct:"uint8" json:"s"`
	Position   uint8 `struct:"uint8" json:"p"`
}

type SelectCardMsg struct {
	Player     uint8             `struct:"uint8" json:"player"`
	Cancelable uint8             `struct:"uint8" json:"cancelable"`
	Min        uint8             `struct:"uint8" json:"min"`
	Max        uint8             `struct:"uint8" json:"max"`
	Count      uint8             `struct:"uint8,sizeof=Cards" json:"-"`
	Cards      []SelectCardEntry `json:"cards"`
}

// HideCodesForPlayer 将非指定玩家的卡片代码归零（用于隐私发送）
func (m *SelectCardMsg) HideCodesForPlayer(player uint8) {
	for i := range m.Cards {
		if m.Cards[i].Controller != player {
			m.Cards[i].Code = 0
		}
	}
}

// Pack 将消息打包为字节数组（注意不包含开头的消息类型字节）
func (m *SelectCardMsg) Pack() []byte {
	data, _ := restruct.Pack(binary.LittleEndian, m)
	return data
}

// ------------------------------------------------------------------
// MSG_SELECT_UNSELECT_CARD — 选择/取消选择卡片
// per playerop.cpp: player(1) finishable(1) cancelable(1) min(1) max(1)，
// countA(1) + 条目(8B)，countB(1) + 条目(8B)；条目与 SELECT_CARD 相同
// ------------------------------------------------------------------
type SelectUnselectCardMsg struct {
	Player     uint8             `struct:"uint8" json:"player"`
	Finishable uint8             `struct:"uint8" json:"finishable"`
	Cancelable uint8             `struct:"uint8" json:"cancelable"`
	Min        uint8             `struct:"uint8" json:"min"`
	Max        uint8             `struct:"uint8" json:"max"`
	Count1     uint8             `struct:"uint8,sizeof=Cards1" json:"-"`
	Cards1     []SelectCardEntry `json:"cards"`
	Count2     uint8             `struct:"uint8,sizeof=Cards2" json:"-"`
	Cards2     []SelectCardEntry `json:"unselectList"`
}

func (m *SelectUnselectCardMsg) HideCodesForPlayer(player uint8) {
	for i := range m.Cards1 {
		if m.Cards1[i].Controller != player {
			m.Cards1[i].Code = 0
		}
	}
	for i := range m.Cards2 {
		if m.Cards2[i].Controller != player {
			m.Cards2[i].Code = 0
		}
	}
}

func (m *SelectUnselectCardMsg) Pack() []byte {
	data, _ := restruct.Pack(binary.LittleEndian, m)
	return data
}

// ------------------------------------------------------------------
// MSG_SELECT_CHAIN — 选择连锁
// per playerop.cpp: player(1) count(1) spe_count(1) hint0(4) hint1(4)，
// 条目 × count = desc_flag(1) + forced(1) + code(4) + info_location(4) + description(4) = 14B
// forced 是每个条目内的字节，不在头部
// ------------------------------------------------------------------
type ChainEntry struct {
	DescFlag     uint8  `struct:"uint8" json:"flag"`
	Forced       uint8  `struct:"uint8" json:"forced"`
	Code         uint32 `struct:"uint32" json:"code"`
	InfoLocation uint32 `struct:"uint32" json:"-"`
	Description  uint32 `struct:"uint32" json:"desc"`
}

type SelectChainMsg struct {
	Player   uint8        `struct:"uint8" json:"player"`
	Count    uint8        `struct:"uint8,sizeof=Chains" json:"-"`
	SpeCount uint8        `struct:"uint8" json:"-"`
	Hint0    uint32       `struct:"uint32" json:"-"`
	Hint1    uint32       `struct:"uint32" json:"-"`
	Chains   []ChainEntry `json:"chains"`
}

// ------------------------------------------------------------------
// MSG_SELECT_PLACE / MSG_SELECT_DISFIELD — 选择位置
// per playerop.cpp: player(1) count(1) flag(4)，无条目
// ------------------------------------------------------------------
type SelectPlaceMsg struct {
	Player uint8  `struct:"uint8" json:"player"`
	Count  uint8  `struct:"uint8" json:"count"`
	Flag   uint32 `struct:"uint32" json:"flag"`
}

// ------------------------------------------------------------------
// MSG_SELECT_POSITION — 选择表示形式
// per playerop.cpp: player(1), code(4), positions(1)
// ------------------------------------------------------------------
type SelectPositionMsg struct {
	Player    uint8  `struct:"uint8" json:"player"`
	Code      uint32 `struct:"uint32" json:"code"`
	Positions uint8  `struct:"uint8" json:"positions"`
}

// ------------------------------------------------------------------
// MSG_SELECT_COUNTER — 选择计数器
// per playerop.cpp: player(1) countertype(2) count(2) cardcount(1)，
// 条目 × cardcount = code(4) + c(1) + l(1) + s(1) + 计数(2) = 9 bytes
// ------------------------------------------------------------------
type CounterEntry struct {
	Code       int32  `struct:"int32" json:"code"`
	Controller uint8  `struct:"uint8" json:"c"`
	Location   uint8  `struct:"uint8" json:"l"`
	Sequence   uint8  `struct:"uint8" json:"s"`
	Count      uint16 `struct:"uint16" json:"cnt"`
}

type SelectCounterMsg struct {
	Player      uint8          `struct:"uint8" json:"player"`
	CounterType uint16         `struct:"uint16" json:"countertype"`
	Count       uint16         `struct:"uint16" json:"count"`
	CardCount   uint8          `struct:"uint8,sizeof=Entries" json:"-"`
	Entries     []CounterEntry `json:"cards"`
}

// ------------------------------------------------------------------
// MSG_SELECT_SUM — 选择合计数值
// per playerop.cpp: sumMode(1: max!=0 时为 0) player(1) acc(4) min(1) max(1)，
// must 数量(1) + 条目(11B)，select 数量(1) + 条目(11B)；
// 条目 = code(4) + c(1) + l(1) + s(1) + sum_param(4) = 11 bytes
// ------------------------------------------------------------------
type SumEntry struct {
	Code       int32  `struct:"int32" json:"code"`
	Controller uint8  `struct:"uint8" json:"c"`
	Location   uint8  `struct:"uint8" json:"l"`
	Sequence   uint8  `struct:"uint8" json:"s"`
	Param      uint32 `struct:"uint32" json:"param"`
}

type SelectSumMsg struct {
	SumMode     uint8      `struct:"uint8" json:"sumMode"`
	Player      uint8      `struct:"uint8" json:"player"`
	Acc         uint32     `struct:"uint32" json:"acc"`
	Min         uint8      `struct:"uint8" json:"min"`
	Max         uint8      `struct:"uint8" json:"max"`
	MustCount   uint8      `struct:"uint8,sizeof=Must" json:"-"`
	Must        []SumEntry `json:"must"`
	SelectCount uint8      `struct:"uint8,sizeof=Select" json:"-"`
	Select      []SumEntry `json:"select"`
}

// ------------------------------------------------------------------
// MSG_SORT_CARD — 排序卡片
// per playerop.cpp: player(1) count(1)，
// 条目 × count = code(4) + c(1) + l(1) + s(1) = 7 bytes
// ------------------------------------------------------------------
type SortCardEntry struct {
	Code       int32 `struct:"int32" json:"code"`
	Controller uint8 `struct:"uint8" json:"c"`
	Location   uint8 `struct:"uint8" json:"l"`
	Sequence   uint8 `struct:"uint8" json:"s"`
}

type SortCardMsg struct {
	Player uint8           `struct:"uint8" json:"player"`
	Count  uint8           `struct:"uint8,sizeof=Cards" json:"-"`
	Cards  []SortCardEntry `json:"cards"`
}

// ------------------------------------------------------------------
// MSG_CONFIRM_DECKTOP / MSG_CONFIRM_EXTRATOP — 确认卡组/额外卡组顶端
// per libduel.cpp: player(1) count(1)，
// 条目 × count = code(4) + c(1) + l(1) + s(1) = 7 bytes（无 pos 字节）
// ------------------------------------------------------------------
type ConfirmCardEntry struct {
	Code       int32 `struct:"int32" json:"code"`
	Controller uint8 `struct:"uint8" json:"c"`
	Location   uint8 `struct:"uint8" json:"l"`
	Sequence   uint8 `struct:"uint8" json:"s"`
}

type ConfirmDeckTopMsg struct {
	Player uint8              `struct:"uint8" json:"player"`
	Count  uint8              `struct:"uint8,sizeof=Cards" json:"-"`
	Cards  []ConfirmCardEntry `json:"cards"`
}

func (m *ConfirmDeckTopMsg) Pack() []byte {
	data, _ := restruct.Pack(binary.LittleEndian, m)
	return data
}

// MSG_CONFIRM_EXTRATOP 与 MSG_CONFIRM_DECKTOP 布局完全相同（libduel.cpp:1030）
type ConfirmExtraTopMsg = ConfirmDeckTopMsg

// ------------------------------------------------------------------
// MSG_CONFIRM_CARDS — 确认卡片
// per operations.cpp: player(1) skip_panel(1) count(1)，
// 条目与 ConfirmCardEntry 相同（7 bytes）
// ------------------------------------------------------------------
type ConfirmCardsMsg struct {
	Player    uint8              `struct:"uint8" json:"player"`
	SkipPanel uint8              `struct:"uint8" json:"-"`
	Count     uint8              `struct:"uint8,sizeof=Cards" json:"-"`
	Cards     []ConfirmCardEntry `json:"cards"`
}

// ------------------------------------------------------------------
// MSG_DRAW — 抽卡
// per operations.cpp: player(1) count(1)，
// code | (表侧公开 ? 0x80000000 : 0)
// ------------------------------------------------------------------
type DrawMsg struct {
	Player uint8   `struct:"uint8" json:"player"`
	Count  uint8   `struct:"uint8,sizeof=Cards" json:"-"`
	Cards  []int32 `struct:"[]int32" json:"cards"`
}

func (m *DrawMsg) IsCardKnown(idx int) bool {
	if idx < 0 || idx >= len(m.Cards) {
		return false
	}
	// 引擎对表侧(cards is_position(POS_FACEUP))的抽卡把 code 最高位置 1
	// （operations.cpp: code | 0x80000000），服务端只应把未标记的 code 对对手归零
	// （gframe/single_duel.cpp: if(!(pbufw[3] & 0x80)) Write<int32_t>(pbufw, 0)）。
	posByte := uint32(m.Cards[idx]) >> 24
	return posByte&0x80 != 0
}

// HideUnknownCards 将未带 0x80000000 公开标记的卡片代码归零
func (m *DrawMsg) HideUnknownCards() {
	for i := range m.Cards {
		if !m.IsCardKnown(i) {
			m.Cards[i] = 0
		}
	}
}

func (m *DrawMsg) Pack() []byte {
	data, _ := restruct.Pack(binary.LittleEndian, m)
	return data
}

// ------------------------------------------------------------------
// MSG_SHUFFLE_HAND — 手牌洗牌
// 格式: uint8 player, uint8 count, int32[count] codes
// 对非当前玩家发送时需要把 code 归零
// ------------------------------------------------------------------
type ShuffleHandMsg struct {
	Player uint8   `struct:"uint8" json:"player"`
	Count  uint8   `struct:"uint8,sizeof=Cards" json:"-"`
	Cards  []int32 `struct:"[]int32" json:"cards"`
}

func (m *ShuffleHandMsg) HideAllCodes() {
	for i := range m.Cards {
		m.Cards[i] = 0
	}
}

func (m *ShuffleHandMsg) Pack() []byte {
	data, _ := restruct.Pack(binary.LittleEndian, m)
	return data
}

// MSG_SHUFFLE_EXTRA 与 MSG_SHUFFLE_HAND 布局相同
type ShuffleExtraMsg = ShuffleHandMsg

// ------------------------------------------------------------------
// MSG_MOVE — 移动卡片
// per field.cpp: code(4) + 原位置 get_info_location(4) + 新位置(4) + reason(4) = 16 bytes
// （打包位置的 4 字节依次为 c, l, s, pos，与 8 个独立字节等价）
// ------------------------------------------------------------------
type MoveMsg struct {
	Code   uint32 `struct:"uint32" json:"code"`
	PC     uint8  `struct:"uint8" json:"pc"` // prev controller
	PL     uint8  `struct:"uint8" json:"pl"` // prev location
	PS     uint8  `struct:"uint8" json:"ps"` // prev sequence
	PP     uint8  `struct:"uint8" json:"pp"` // prev position
	CC     uint8  `struct:"uint8" json:"cc"` // curr controller
	CL     uint8  `struct:"uint8" json:"cl"` // curr location
	CS     uint8  `struct:"uint8" json:"cs"` // curr sequence
	CP     uint8  `struct:"uint8" json:"cp"` // curr position
	Reason uint32 `struct:"uint32" json:"reason"`
}

// ------------------------------------------------------------------
// MSG_POS_CHANGE — 位置变更
// per operations.cpp: code(4) + c(1) + l(1) + s(1) + pp(1) + cp(1) = 9 bytes
// （pp=previous.position，cp=current.position）
// ------------------------------------------------------------------
type PosChangeMsg struct {
	Code uint32 `struct:"uint32" json:"code"`
	CC   uint8  `struct:"uint8" json:"cc"`
	CL   uint8  `struct:"uint8" json:"cl"`
	CS   uint8  `struct:"uint8" json:"cs"`
	PP   uint8  `struct:"uint8" json:"pp"`
	CP   uint8  `struct:"uint8" json:"cp"`
}

// ------------------------------------------------------------------
// MSG_SET — 设置卡片
// per operations.cpp: code(4) + get_info_location(4) = 8 bytes
// ------------------------------------------------------------------
type SetMsg struct {
	Code uint32 `struct:"uint32" json:"code"`
	CC   uint8  `struct:"uint8" json:"cc"`
	CL   uint8  `struct:"uint8" json:"cl"`
	CS   uint8  `struct:"uint8" json:"cs"`
	CP   uint8  `struct:"uint8" json:"cp"`
}

// ------------------------------------------------------------------
// MSG_SUMMONING / MSG_FLIPSUMMONING / MSG_SPSUMMONING — 召唤中
// per operations.cpp: code(4) + get_info_location(4) = 8 bytes，布局相同
// ------------------------------------------------------------------
type SummoningMsg struct {
	Code uint32 `struct:"uint32" json:"code"`
	CC   uint8  `struct:"uint8" json:"cc"`
	CL   uint8  `struct:"uint8" json:"cl"`
	CS   uint8  `struct:"uint8" json:"cs"`
	CP   uint8  `struct:"uint8" json:"cp"`
}

type FlipSummoningMsg = SummoningMsg
type SPSummoningMsg = SummoningMsg

// ------------------------------------------------------------------
// MSG_SWAP — 交换卡片
// per operations.cpp: code1(4)+info1(4) + code2(4)+info2(4) = 16 bytes
// ------------------------------------------------------------------
type SwapMsg struct {
	Code1 uint32 `struct:"uint32" json:"code1"`
	CC1   uint8  `struct:"uint8" json:"cc1"`
	CL1   uint8  `struct:"uint8" json:"cl1"`
	CS1   uint8  `struct:"uint8" json:"cs1"`
	CP1   uint8  `struct:"uint8" json:"cp1"`
	Code2 uint32 `struct:"uint32" json:"code2"`
	CC2   uint8  `struct:"uint8" json:"cc2"`
	CL2   uint8  `struct:"uint8" json:"cl2"`
	CS2   uint8  `struct:"uint8" json:"cs2"`
	CP2   uint8  `struct:"uint8" json:"cp2"`
}

// ------------------------------------------------------------------
// MSG_CHAINING — 连锁发动中（16 bytes）
// per processor.cpp: code(4) + get_info_location(4)（发动卡的位置 c,l,s,pos）+
// 触发 controler(1) + location(1) + sequence(1) + description(4) + chain_count(1)
// 注意：一般发动时触发位置 == 发动卡位置，但字节上是两组独立数据
// ------------------------------------------------------------------
type ChainingMsg struct {
	Code                 uint32 `struct:"uint32" json:"code"`
	CardController       uint8  `struct:"uint8" json:"cc"`
	CardLocation         uint8  `struct:"uint8" json:"cl"`
	CardSequence         uint8  `struct:"uint8" json:"cs"`
	CardPosition         uint8  `struct:"uint8" json:"cp"`
	TriggeringController uint8  `struct:"uint8" json:"-"`
	TriggeringLocation   uint8  `struct:"uint8" json:"-"`
	TriggeringSequence   uint8  `struct:"uint8" json:"-"`
	Description          uint32 `struct:"uint32" json:"desc"`
	ChainCount           uint8  `struct:"uint8" json:"count"`
}

// ------------------------------------------------------------------
// MSG_CHAIN_SOLVING / MSG_CHAIN_SOLVED / MSG_CHAINED / MSG_CHAIN_NEGATED /
// MSG_CHAIN_DISABLED — 连锁处理，各 1 字节 chain number
// ------------------------------------------------------------------
type ChainSolvingMsg struct {
	Count uint8 `struct:"uint8" json:"count"`
}

type ChainedMsg = ChainSolvingMsg
type ChainSolvedMsg = ChainSolvingMsg
type ChainNegatedMsg = ChainSolvingMsg

// MSG_ATTACK_DISABLED / MSG_CHAIN_END — 无数据
type AttackDisabledMsg struct{}

// ------------------------------------------------------------------
// MSG_ATTACK — 攻击宣言
// per processor.cpp: 攻击方 get_info_location(4) + 对象方 get_info_location(4) = 8 bytes
// （无对象时第二组为 0）
// ------------------------------------------------------------------
type AttackMsg struct {
	AttackerInfo uint32 `struct:"uint32" json:"-"`
	TargetInfo   uint32 `struct:"uint32" json:"-"`
}

// ------------------------------------------------------------------
// MSG_BATTLE — 战斗结算
// per processor.cpp: 攻击方位置(4) + aa(4) + ad(4) + bd0(1) +
// 对象方位置(4) + da(4) + dd(4) + bd1(1) = 26 bytes
// ------------------------------------------------------------------
type BattleMsg struct {
	AttackerInfo   uint32 `struct:"uint32" json:"-"`
	AttackerATK    int32  `struct:"int32" json:"-"`
	AttackerDEF    int32  `struct:"int32" json:"-"`
	AttackerDirect uint8  `struct:"uint8" json:"-"`
	TargetInfo     uint32 `struct:"uint32" json:"-"`
	TargetATK      int32  `struct:"int32" json:"-"`
	TargetDEF      int32  `struct:"int32" json:"-"`
	TargetDirect   uint8  `struct:"uint8" json:"-"`
}

// ------------------------------------------------------------------
// MSG_CARD_SELECTED / MSG_RANDOM_SELECTED
// per playerop.cpp: count(1) + int32[count]；RANDOM 版本前面多 1 字节 player
// ------------------------------------------------------------------
type CardSelectedMsg struct {
	Count uint8   `struct:"uint8,sizeof=Cards" json:"-"`
	Cards []int32 `struct:"[]int32" json:"cards"`
}

type RandomSelectedMsg struct {
	Player uint8   `struct:"uint8" json:"player"`
	Count  uint8   `struct:"uint8,sizeof=Cards" json:"-"`
	Cards  []int32 `struct:"[]int32" json:"cards"`
}

// ------------------------------------------------------------------
// MSG_BECOME_TARGET — 成为对象
// per operations.cpp: count(1) + get_info_location(4) × count
// ------------------------------------------------------------------
type BecomeTargetMsg struct {
	Count   uint8    `struct:"uint8,sizeof=Targets" json:"-"`
	Targets []uint32 `struct:"[]uint32" json:"targets"`
}

// ------------------------------------------------------------------
// MSG_DAMAGE / MSG_RECOVER / MSG_LPUPDATE / MSG_PAY_LPCOST — LP 变动
// per operations.cpp: player(1) + value(4)
// ------------------------------------------------------------------
type DamageMsg struct {
	Player uint8 `struct:"uint8" json:"player"`
	Value  int32 `struct:"int32" json:"amount"`
}

type RecoverMsg = DamageMsg

type PayLPCostMsg struct {
	Player uint8 `struct:"uint8" json:"player"`
	Value  int32 `struct:"int32" json:"amount"`
}

type LPUpdateMsg struct {
	Player uint8 `struct:"uint8" json:"player"`
	LP     int32 `struct:"int32" json:"lp"`
}

// ------------------------------------------------------------------
// MSG_EQUIP — 装备
// per card.cpp: 装备卡 get_info_location(4) + 对象卡 get_info_location(4) = 8 bytes
// ------------------------------------------------------------------
type EquipMsg struct {
	CardInfo   uint32 `struct:"uint32" json:"-"`
	TargetInfo uint32 `struct:"uint32" json:"-"`
}

// ------------------------------------------------------------------
// MSG_UNEQUIP — 解除装备（4 bytes：get_info_location）
// ------------------------------------------------------------------
type UnequipMsg struct {
	Info uint32 `struct:"uint32" json:"-"`
}

// ------------------------------------------------------------------
// MSG_CARD_TARGET / MSG_CANCEL_TARGET（8 bytes，同 MSG_EQUIP）
// ------------------------------------------------------------------
type CardTargetMsg = EquipMsg

// ------------------------------------------------------------------
// MSG_ADD_COUNTER / MSG_REMOVE_COUNTER
// per card.cpp:2357-2361 / 2379-2383: type(2) + c(1) + l(1) + s(1) + count(2)
// = 7 bytes（注意不是 get_info_location——没有 pos 字节）
// ------------------------------------------------------------------
type CounterMsg struct {
	Type  uint16 `struct:"uint16" json:"type"`
	CC    uint8  `struct:"uint8" json:"c"`
	CL    uint8  `struct:"uint8" json:"l"`
	CS    uint8  `struct:"uint8" json:"s"`
	Count uint16 `struct:"uint16" json:"count"`
}

// ------------------------------------------------------------------
// MSG_MISSED_EFFECT — 错过效果
// per processor.cpp: get_info_location(4) + code(4)
// ------------------------------------------------------------------
type MissedEffectMsg struct {
	InfoLocation uint32 `struct:"uint32" json:"-"`
	Code         uint32 `struct:"uint32" json:"code"`
}

// ------------------------------------------------------------------
// MSG_MATCH_KILL — 比赛击杀（战斗伤害致死且攻击者带 EFFECT_MATCH_KILL）
// per operations.cpp:535-536: code(4)（gframe 胜利时展示这张卡图）
// ------------------------------------------------------------------
type MatchKillMsg struct {
	Code uint32 `struct:"uint32" json:"code"`
}

// ------------------------------------------------------------------
// MSG_TOSS_COIN / MSG_TOSS_DICE — 抛硬币/掷骰子
// per operations.cpp: player(1) count(1) + result(1) × count
// ------------------------------------------------------------------
type TossCoinMsg struct {
	Player  uint8   `struct:"uint8" json:"player"`
	Count   uint8   `struct:"uint8,sizeof=Results" json:"-"`
	Results []uint8 `struct:"[]uint8" json:"results"`
}

type TossDiceMsg = TossCoinMsg

// ------------------------------------------------------------------
// MSG_ROCK_PAPER_SCISSORS — 猜拳
// per operations.cpp: phase(1)（0=请求出拳，1=请求出拳（再猜））
// ------------------------------------------------------------------
type RockPaperScissorsMsg struct {
	Phase uint8 `struct:"uint8" json:"phase"`
}

// MSG_HAND_RES — 猜拳结果：res = hand0 | (hand1 << 2)
// 出拳值：1=石头 2=剪刀 3=布（gframe 按钮图 f1/f2/f3 依次发 1/2/3，
// event_handler.cpp:33；引擎判定 1 胜 2、2 胜 3、3 胜 1，operations.cpp:6538）
type HandResMsg struct {
	Res uint8 `struct:"uint8" json:"res"`
}

// ------------------------------------------------------------------
// MSG_ANNOUNCE_RACE / MSG_ANNOUNCE_ATTRIB — 宣言种族/属性
// per playerop.cpp: player(1) count(1) available(4)（位掩码）
// ------------------------------------------------------------------
type AnnounceRaceMsg struct {
	Player    uint8  `struct:"uint8" json:"player"`
	Count     uint8  `struct:"uint8" json:"count"`
	Available uint32 `struct:"uint32" json:"available"`
}

type AnnounceAttribMsg = AnnounceRaceMsg

// ------------------------------------------------------------------
// MSG_ANNOUNCE_CARD / MSG_ANNOUNCE_NUMBER
// per playerop.cpp: player(1) count(1) + int32[count]
// ANNOUNCE_CARD 的 values 是脚本 announce_filter 的后缀表达式（OPCODE_*），
// 字面量 + OPCODE_ISCODE 的组合即为可选卡号
// ------------------------------------------------------------------
type AnnounceCardMsg struct {
	Player uint8   `struct:"uint8" json:"player"`
	Count  uint8   `struct:"uint8,sizeof=Values" json:"-"`
	Values []int32 `struct:"[]int32" json:"options"`
}

type AnnounceNumberMsg = AnnounceCardMsg

// ------------------------------------------------------------------
// MSG_HINT — 提示消息 (6 bytes)
// per processor.cpp: uint8 type, uint8 player, int32 data
// ------------------------------------------------------------------
type HintMsg struct {
	Type   uint8 `struct:"uint8" json:"type"`
	Player uint8 `struct:"uint8" json:"player"`
	Data   int32 `struct:"int32" json:"data"`
}

// ------------------------------------------------------------------
// MSG_CARD_HINT — 卡片提示
// per card.cpp: get_info_location(4) + type(1) + data(4) = 9 bytes
// ------------------------------------------------------------------
type CardHintMsg struct {
	Info uint32 `struct:"uint32" json:"-"`
	Type uint8  `struct:"uint8" json:"-"`
	Data int32  `struct:"int32" json:"-"`
}

// MSG_PLAYER_HINT — 玩家提示 (6 bytes: player(1) + type(1) + data(4))
type PlayerHintMsg struct {
	Player uint8 `struct:"uint8" json:"-"`
	Type   uint8 `struct:"uint8" json:"-"`
	Data   int32 `struct:"int32" json:"-"`
}

// ------------------------------------------------------------------
// MSG_NEW_TURN / MSG_NEW_PHASE / MSG_WIN — 回合/阶段/胜利
// ------------------------------------------------------------------
type NewTurnMsg struct {
	Player uint8 `struct:"uint8" json:"player"`
}

// per processor.cpp: phase(2)（uint16）
type NewPhaseMsg struct {
	Phase uint16 `struct:"uint16" json:"phase"`
}

type WinMsg struct {
	Player uint8 `struct:"uint8" json:"winner"`
	Type   uint8 `struct:"uint8" json:"type"`
}

// ------------------------------------------------------------------
// MSG_DECK_TOP — 卡组顶端公开（deck reversed 时）
// per libduel.cpp: player(1) sequence(1) code(4) = 6 bytes
// ------------------------------------------------------------------
type DeckTopMsg struct {
	Player   uint8  `struct:"uint8" json:"player"`
	Sequence uint8  `struct:"uint8" json:"sequence"`
	Code     uint32 `struct:"uint32" json:"code"`
}

// ------------------------------------------------------------------
// MSG_TAG_SWAP — 交换（双打）
// per tag_duel.cpp: player(1) main数(1) extra数(1) side数(1) + 各 int32 列表
// ------------------------------------------------------------------
type TagSwapMsg struct {
	Player     uint8   `struct:"uint8" json:"player"`
	MainCount  uint8   `struct:"uint8,sizeof=Main" json:"-"`
	ExtraCount uint8   `struct:"uint8,sizeof=Extra" json:"-"`
	SideCount  uint8   `struct:"uint8,sizeof=Side" json:"-"`
	Main       []int32 `struct:"[]int32" json:"main"`
	Extra      []int32 `struct:"[]int32" json:"extra"`
	Side       []int32 `struct:"[]int32" json:"side"`
}

// ------------------------------------------------------------------
// MSG_SHUFFLE_SET_CARD — 盖放卡片洗牌
// per operations.cpp: loc(1) count(1) + [code(4) + get_info_location(4)] × count
// ------------------------------------------------------------------
type ShuffleSetCardMsg struct {
	Loc   uint8          `struct:"uint8" json:"-"`
	Count uint8          `struct:"uint8,sizeof=Cards" json:"-"`
	Cards []SetCardEntry `json:"-"`
}

type SetCardEntry struct {
	Code int32  `struct:"int32" json:"-"`
	Info uint32 `struct:"uint32" json:"-"`
}

// ------------------------------------------------------------------
// MSG_REFRESH_DECK — 刷新卡组（player(1)）
// ------------------------------------------------------------------
type RefreshDeckMsg struct {
	Player uint8 `struct:"uint8" json:"-"`
}

// MSG_REVERSE_DECK — 翻转卡组，无数据
type ReverseDeckMsg struct{}

// ------------------------------------------------------------------
// MSG_FIELD_DISABLED — 场地禁用（zones(4)）
// ------------------------------------------------------------------
type FieldDisabledMsg struct {
	Zones uint32 `struct:"uint32" json:"-"`
}

// ------------------------------------------------------------------
// 辅助函数
// ------------------------------------------------------------------

// UnpackGameMsg 用 restruct 从字节切片解析特定类型的消息
// 注意：data 不应包含开头的消息类型字节
func UnpackGameMsg(data []byte, msg interface{}) error {
	return restruct.Unpack(data, binary.LittleEndian, msg)
}

// PackGameMsg 用 restruct 将消息打包为字节切片
func PackGameMsg(msg interface{}) []byte {
	data, _ := restruct.Pack(binary.LittleEndian, msg)
	return data
}
