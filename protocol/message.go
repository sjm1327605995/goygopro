package protocol

import (
	"encoding/binary"

	"github.com/go-restruct/restruct"
)

const posFaceDown = 0x0a

// ------------------------------------------------------------------
// 通用基础类型
// ------------------------------------------------------------------

// CardCode 卡片代码，在 YGO 消息中常见的高字节携带位置信息的 int32
// 实际只用到低 24 位，最高位(0x80000000)有时用于标记
// 在 struct 中直接用 int32 即可，需要时再提取高位
type CardCode = int32

// ------------------------------------------------------------------
// MSG_HINT — 提示消息 (6 bytes)
// C++ 无独立 struct，格式: uint8 type, uint8 player, int32 data
// ------------------------------------------------------------------
type HintMsg struct {
	Type   uint8 `struct:"uint8"`
	Player uint8 `struct:"uint8"`
	Data   int32 `struct:"int32"`
}

// ------------------------------------------------------------------
// MSG_SELECT_BATTLECMD — 选择战斗指令
// 格式: uint8 player, uint8 count_a, [11 bytes] * count_a, uint8 count_b, [8 bytes] * count_b
// ------------------------------------------------------------------
type BattleCmdEntry struct {
	_ [11]byte `struct:"[11]byte"`
}

type IdleCmdEntry struct {
	_ [8]byte `struct:"[8]byte"`
}

type SelectBattleCmdMsg struct {
	Player uint8 `struct:"uint8"`
	CountA uint8 `struct:"uint8,sizeof=CmdsA"`
	CmdsA  []BattleCmdEntry
	CountB uint8 `struct:"uint8,sizeof=CmdsB"`
	CmdsB  []IdleCmdEntry
}

// ------------------------------------------------------------------
// MSG_SELECT_IDLECMD — 选择空闲指令
// 格式: uint8 player, uint8 count_a * 11, uint8 count_b * 8, uint8 count_c * 8,
//
//	uint8 count_d * 8, uint8 count_e * 8
//
// ------------------------------------------------------------------
type SelectIdleCmdMsg struct {
	Player uint8 `struct:"uint8"`
	CountA uint8 `struct:"uint8,sizeof=CmdsA"`
	CmdsA  []BattleCmdEntry
	CountB uint8 `struct:"uint8,sizeof=CmdsB"`
	CmdsB  []IdleCmdEntry
	CountC uint8 `struct:"uint8,sizeof=CmdsC"`
	CmdsC  []IdleCmdEntry
	CountD uint8 `struct:"uint8,sizeof=CmdsD"`
	CmdsD  []IdleCmdEntry
	CountE uint8 `struct:"uint8,sizeof=CmdsE"`
	CmdsE  []IdleCmdEntry
}

// ------------------------------------------------------------------
// MSG_SELECT_EFFECTYN — 选择是否发动效果 (10 bytes)
// 格式: uint8 player, uint8 unk[9]
// ------------------------------------------------------------------
type SelectEffectYNMsg struct {
	Player uint8   `struct:"uint8"`
	_      [9]byte `struct:"[9]byte"`
}

// ------------------------------------------------------------------
// MSG_SELECT_YESNO — 选择是/否 (6 bytes)
// 格式: uint8 player, uint8 unk[5]
// ------------------------------------------------------------------
type SelectYesNoMsg struct {
	Player uint8   `struct:"uint8"`
	_      [5]byte `struct:"[5]byte"`
}

// ------------------------------------------------------------------
// MSG_SELECT_OPTION — 选择选项
// 格式: uint8 player, uint8 count, int32[count] options
// ------------------------------------------------------------------
type SelectOptionMsg struct {
	Player  uint8   `struct:"uint8"`
	Count   uint8   `struct:"uint8,sizeof=Options"`
	Options []int32 `struct:"[]int32"`
}

// ------------------------------------------------------------------
// MSG_SELECT_CARD / MSG_SELECT_TRIBUTE — 选择卡片 / 祭品
// 格式: uint8 player, uint8 unk[3], uint8 count,
//
//	(int32 code, uint8 controller, uint8 unk[3]) * count
//
// 每个 entry 8 bytes，其中 code 需要根据 controller 决定是否隐藏
// ------------------------------------------------------------------
type SelectCardEntry struct {
	Code       int32   `struct:"int32"`
	Controller uint8   `struct:"uint8"`
	_          [3]byte `struct:"[3]byte"`
}

type SelectCardMsg struct {
	Player uint8   `struct:"uint8"`
	_      [3]byte `struct:"[3]byte"`
	Count  uint8   `struct:"uint8,sizeof=Cards"`
	Cards  []SelectCardEntry
}

// HideCodesForOtherPlayers 将非指定玩家的卡片代码归零（用于隐私发送）
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
// 格式: uint8 player, uint8 unk[4], uint8 count1, (int32 code, uint8 c, uint8[3]) * count1,
//
//	uint8 count2, (int32 code, uint8 c, uint8[3]) * count2
//
// ------------------------------------------------------------------
type SelectUnselectCardMsg struct {
	Player uint8   `struct:"uint8"`
	_      [4]byte `struct:"[4]byte"`
	Count1 uint8   `struct:"uint8,sizeof=Cards1"`
	Cards1 []SelectCardEntry
	Count2 uint8 `struct:"uint8,sizeof=Cards2"`
	Cards2 []SelectCardEntry
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
// MSG_SELECT_CHAIN — 选择连锁（双打版，无 player 字段）
// 格式: uint8 count, [14 bytes] * count, uint8[9]
// tag_duel.go 原始代码: pbuf.Next(int(count)*12 + 1) 和 pbuf.Next(9 + int(count)*14)
// 经实际核对，单个 entry 为 14 bytes，尾部固定 9 bytes（不含 player/count）
// 注意：single_duel.go 版本含 player 字段，格式不同
// ------------------------------------------------------------------
type ChainEntry struct {
	_ [14]byte `struct:"[14]byte"`
}

type TagSelectChainMsg struct {
	Count  uint8 `struct:"uint8,sizeof=Chains"`
	Chains []ChainEntry
	_      [9]byte `struct:"[9]byte"`
}

// ------------------------------------------------------------------
// MSG_SELECT_CHAIN — 选择连锁（单人版，含 player 字段）
// single_duel.go 原始代码: player(1) + count(1) + skip(9 + count*14)
// ------------------------------------------------------------------
type SingleSelectChainMsg struct {
	Player uint8 `struct:"uint8"`
	Count  uint8 `struct:"uint8,sizeof=Chains"`
	Chains []ChainEntry
	_      [9]byte `struct:"[9]byte"`
}

// ------------------------------------------------------------------
// MSG_SELECT_PLACE / MSG_SELECT_DISFIELD — 选择位置
// 格式: uint8 player, uint8 count, [4 bytes] * count
// ------------------------------------------------------------------
type SelectPlaceMsg struct {
	Player uint8 `struct:"uint8"`
	Count  uint8 `struct:"uint8,sizeof=Places"`
	Places []PlaceEntry
}

type PlaceEntry struct {
	_ [4]byte `struct:"[4]byte"`
}

// ------------------------------------------------------------------
// MSG_SELECT_POSITION — 选择表示形式
// 格式: uint8 player, int32 code, uint8 positions
// ------------------------------------------------------------------
type SelectPositionMsg struct {
	Player    uint8 `struct:"uint8"`
	Code      int32 `struct:"int32"`
	Positions uint8 `struct:"uint8"`
}

// ------------------------------------------------------------------
// MSG_SELECT_COUNTER — 选择计数器
// 格式: uint8 player, uint8 unk[3], uint8 count, [7 bytes] * count
// ------------------------------------------------------------------
type CounterEntry struct {
	_ [7]byte `struct:"[7]byte"`
}

type SelectCounterMsg struct {
	Player  uint8   `struct:"uint8"`
	_       [3]byte `struct:"[3]byte"`
	Count   uint8   `struct:"uint8,sizeof=Entries"`
	Entries []CounterEntry
}

// ------------------------------------------------------------------
// MSG_SELECT_SUM — 选择合计数值
// 格式: uint8 player, uint8 count_a, [11 bytes] * count_a,
//
//	uint8 count_b, [7 bytes] * count_b, uint8 count_c, [7 bytes] * count_c
//
// ------------------------------------------------------------------
type SumEntryA struct {
	_ [11]byte `struct:"[11]byte"`
}

type SumEntryB struct {
	_ [7]byte `struct:"[7]byte"`
}

type SelectSumMsg struct {
	Player   uint8 `struct:"uint8"`
	CountA   uint8 `struct:"uint8,sizeof=EntriesA"`
	EntriesA []SumEntryA
	CountB   uint8 `struct:"uint8,sizeof=EntriesB"`
	EntriesB []SumEntryB
	CountC   uint8 `struct:"uint8,sizeof=EntriesC"`
	EntriesC []SumEntryB
}

// ------------------------------------------------------------------
// MSG_SORT_CARD — 排序卡片
// 格式: uint8 player, uint8 count, [7 bytes] * count
// ------------------------------------------------------------------
type SortCardEntry struct {
	_ [7]byte `struct:"[7]byte"`
}

type SortCardMsg struct {
	Player uint8 `struct:"uint8"`
	Count  uint8 `struct:"uint8,sizeof=Cards"`
	Cards  []SortCardEntry
}

// ------------------------------------------------------------------
// MSG_CONFIRM_DECKTOP — 确认卡组顶部
// 格式: uint8 player, uint8 count, (int32 code + uint8 c + uint8 loc + uint8 seq + uint8 pos) * count
// 每个 entry 8 bytes
// ------------------------------------------------------------------
type ConfirmCardEntry struct {
	Code       int32 `struct:"int32"`
	Controller uint8 `struct:"uint8"`
	Location   uint8 `struct:"uint8"`
	Sequence   uint8 `struct:"uint8"`
}

type ConfirmDeckTopMsg struct {
	Player uint8 `struct:"uint8"`
	Count  uint8 `struct:"uint8,sizeof=Cards"`
	Cards  []ConfirmCardEntry
}

func (m *ConfirmDeckTopMsg) HideForPlayer(player uint8) {
	for i := range m.Cards {
		// 双打特殊逻辑：player==1 时里侧表示的卡片需要隐藏
		posByte := uint32(m.Cards[i].Code) >> 24
		if player == 1 && posByte&posFaceDown != 0 {
			m.Cards[i].Code = 0
		}
	}
}

func (m *ConfirmDeckTopMsg) Pack() []byte {
	data, _ := restruct.Pack(binary.LittleEndian, m)
	return data
}

// ------------------------------------------------------------------
// MSG_CONFIRM_CARDS — 确认卡片
// 格式与 MSG_CONFIRM_DECKTOP 相同
// ------------------------------------------------------------------
type ConfirmCardsMsg = ConfirmDeckTopMsg

// ------------------------------------------------------------------
// MSG_DRAW — 抽卡
// 格式: uint8 player, uint8 count, int32[count] codes
// 注意：code 的最高位(0x80)在某些版本中用于标记是否已知
// ------------------------------------------------------------------
type DrawMsg struct {
	Player uint8   `struct:"uint8"`
	Count  uint8   `struct:"uint8,sizeof=Cards"`
	Cards  []int32 `struct:"[]int32"`
}

func (m *DrawMsg) IsCardKnown(idx int) bool {
	if idx < 0 || idx >= len(m.Cards) {
		return false
	}
	// 从高位提取位置信息字节: code >> 24
	posByte := uint32(m.Cards[idx]) >> 24
	return posByte&0x80 == 0
}

// HideUnknownCards 将未知卡片代码归零（高位 0x80 标记未知）
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
	Player uint8   `struct:"uint8"`
	Count  uint8   `struct:"uint8,sizeof=Cards"`
	Cards  []int32 `struct:"[]int32"`
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

// ------------------------------------------------------------------
// MSG_SHUFFLE_EXTRA — 额外卡组洗牌
// 格式与 MSG_SHUFFLE_HAND 相同
// ------------------------------------------------------------------
type ShuffleExtraMsg = ShuffleHandMsg

// ------------------------------------------------------------------
// MSG_MOVE — 移动卡片
// 格式: uint32 code, uint8 pc, uint8 pl, uint8 ps, uint8 pp,
//
//	uint8 cc, uint8 cl, uint8 cs, uint8 cp, uint8 reason
//
// = 4 + 9 = 13 bytes
// ------------------------------------------------------------------
type MoveMsg struct {
	Code   uint32 `struct:"uint32"`
	PC     uint8  `struct:"uint8"` // prev controller
	PL     uint8  `struct:"uint8"` // prev location
	PS     uint8  `struct:"uint8"` // prev sequence
	PP     uint8  `struct:"uint8"` // prev position
	CC     uint8  `struct:"uint8"` // curr controller
	CL     uint8  `struct:"uint8"` // curr location
	CS     uint8  `struct:"uint8"` // curr sequence
	CP     uint8  `struct:"uint8"` // curr position
	Reason uint8  `struct:"uint8"`
}

// ------------------------------------------------------------------
// MSG_POS_CHANGE — 位置变更
// 格式: uint32 code, uint8 cc, uint8 cl, uint8 cs, uint8 pp, uint8 cp
// = 4 + 6 = 10 bytes
// ------------------------------------------------------------------
type PosChangeMsg struct {
	Code uint32 `struct:"uint32"`
	CC   uint8  `struct:"uint8"`
	CL   uint8  `struct:"uint8"`
	CS   uint8  `struct:"uint8"`
	PP   uint8  `struct:"uint8"`
	CP   uint8  `struct:"uint8"`
}

// ------------------------------------------------------------------
// MSG_SET — 设置卡片
// 格式: uint32 code, uint8 cc, uint8 cl, uint8 cs, uint8 cp
// = 4 + 5 = 9 bytes
// ------------------------------------------------------------------
type SetMsg struct {
	Code uint32 `struct:"uint32"`
	CC   uint8  `struct:"uint8"`
	CL   uint8  `struct:"uint8"`
	CS   uint8  `struct:"uint8"`
	CP   uint8  `struct:"uint8"`
}

// ------------------------------------------------------------------
// MSG_SWAP — 交换卡片
// 格式: uint32 code1, uint8 cc1, uint8 cl1, uint8 cs1, uint8 cp1,
//
//	uint32 code2, uint8 cc2, uint8 cl2, uint8 cs2, uint8 cp2
//
// = 9 + 9 = 18 bytes？不对，看代码中 pbuf.Next(16)
// 实际应该是 16 bytes：code1(4) + c1(1) + l1(1) + s1(1) + p1(1) + code2(4) + c2(1) + l2(1) + s2(1) + p2(1) = 16
// ------------------------------------------------------------------
type SwapMsg struct {
	Code1 uint32 `struct:"uint32"`
	CC1   uint8  `struct:"uint8"`
	CL1   uint8  `struct:"uint8"`
	CS1   uint8  `struct:"uint8"`
	CP1   uint8  `struct:"uint8"`
	Code2 uint32 `struct:"uint32"`
	CC2   uint8  `struct:"uint8"`
	CL2   uint8  `struct:"uint8"`
	CS2   uint8  `struct:"uint8"`
	CP2   uint8  `struct:"uint8"`
}

// ------------------------------------------------------------------
// MSG_FIELD_DISABLED — 场地禁用
// 格式: uint32 zones, uint32 loc1, uint32 loc2
// 看代码中 pbuf.Next(8) 所以是 8 bytes，可能只有 zones
// ------------------------------------------------------------------
type FieldDisabledMsg struct {
	Zones uint32  `struct:"uint32"`
	_     [4]byte `struct:"[4]byte"`
}

// ------------------------------------------------------------------
// MSG_SPSUMMONING — 特殊召唤中（双打版）
// 格式: uint32 code + uint8[4]（cc 在 offset 4, cp 在 offset 7）
// ------------------------------------------------------------------
type SPSummoningMsg struct {
	Code uint32  `struct:"uint32"`
	CC   uint8   `struct:"uint8"`
	_    [2]byte `struct:"[2]byte"`
	CP   uint8   `struct:"uint8"`
}

// ------------------------------------------------------------------
// MSG_SUMMONING / MSG_FLIPSUMMONING — 召唤中（8 bytes 占位）
// ------------------------------------------------------------------
type SummoningMsg struct {
	Code uint32  `struct:"uint32"`
	_    [4]byte `struct:"[4]byte"`
}

// ------------------------------------------------------------------
// MSG_SUMMONED / MSG_SPSUMMONED / MSG_FLIPSUMMONED / MSG_CHAINED
// MSG_CHAIN_SOLVED / MSG_CHAIN_END / MSG_CHAIN_NEGATED / MSG_CHAIN_DISABLED
// MSG_ATTACK_DISABLED / MSG_DAMAGE_STEP_START / MSG_DAMAGE_STEP_END
// 这些消息大多没有额外数据或固定长度
// ------------------------------------------------------------------

// ------------------------------------------------------------------
// MSG_CHAINING — 连锁发动中（16 bytes）
// 格式: uint32 code + uint8[3] + uint8 cc + uint8[2] + uint8 cp + uint8[5]
// single_duel.go 中 cc 在 offset 4, cp 在 offset 7
// ------------------------------------------------------------------
type ChainingMsg struct {
	Code uint32  `struct:"uint32"`
	_    [3]byte `struct:"[3]byte"`
	CC   uint8   `struct:"uint8"`
	_    [2]byte `struct:"[2]byte"`
	CP   uint8   `struct:"uint8"`
	_    [5]byte `struct:"[5]byte"`
}

// ------------------------------------------------------------------
// MSG_CHAIN_SOLVING — 连锁处理中
// 格式: uint8[13] 或 int32 + uint8[9]？
// tag_duel.go 中 pbuf.Next(13)
// ------------------------------------------------------------------
type ChainSolvingMsg struct {
	_ [13]byte `struct:"[13]byte"`
}

// ------------------------------------------------------------------
// MSG_CARD_SELECTED / MSG_RANDOM_SELECTED
// MSG_CARD_SELECTED: uint8 count, [4 bytes] * count
// MSG_RANDOM_SELECTED: uint8 player, uint8 count, [4 bytes] * count
// ------------------------------------------------------------------
type CardSelectedMsg struct {
	Count uint8   `struct:"uint8,sizeof=Cards"`
	Cards []int32 `struct:"[]int32"`
}

type RandomSelectedMsg struct {
	Player uint8   `struct:"uint8"`
	Count  uint8   `struct:"uint8,sizeof=Cards"`
	Cards  []int32 `struct:"[]int32"`
}

// ------------------------------------------------------------------
// MSG_BECOME_TARGET — 成为对象
// 格式: uint8 count, [4 bytes] * count
// ------------------------------------------------------------------
type BecomeTargetMsg = CardSelectedMsg

// ------------------------------------------------------------------
// MSG_DAMAGE — 伤害
// 格式: uint8 player, int32 value, uint8[0]?
// single_duel.go 中 pbuf.Next(5) -> player(1) + value(4) = 5
// ------------------------------------------------------------------
type DamageMsg struct {
	Player uint8 `struct:"uint8"`
	Value  int32 `struct:"int32"`
}

// ------------------------------------------------------------------
// MSG_RECOVER — 恢复LP
// 格式同 MSG_DAMAGE
// ------------------------------------------------------------------
type RecoverMsg = DamageMsg

// ------------------------------------------------------------------
// MSG_LPUPDATE — LP更新
// 格式: uint8 player, int32 lp
// ------------------------------------------------------------------
type LPUpdateMsg struct {
	Player uint8 `struct:"uint8"`
	LP     int32 `struct:"int32"`
}

// ------------------------------------------------------------------
// MSG_EQUIP — 装备
// 格式: uint8[8]
// ------------------------------------------------------------------
type EquipMsg struct {
	_ [8]byte `struct:"[8]byte"`
}

// ------------------------------------------------------------------
// MSG_UNEQUIP — 解除装备
// 格式: uint8[4]
// ------------------------------------------------------------------
type UnequipMsg struct {
	_ [4]byte `struct:"[4]byte"`
}

// ------------------------------------------------------------------
// MSG_CARD_TARGET / MSG_CANCEL_TARGET
// 格式: uint8[8]
// ------------------------------------------------------------------
type CardTargetMsg struct {
	_ [8]byte `struct:"[8]byte"`
}

// ------------------------------------------------------------------
// MSG_PAY_LPCOST — 支付LP
// 格式: uint8 player, int32 value
// ------------------------------------------------------------------
type PayLPCostMsg struct {
	Player uint8 `struct:"uint8"`
	Value  int32 `struct:"int32"`
}

// ------------------------------------------------------------------
// MSG_ADD_COUNTER / MSG_REMOVE_COUNTER
// 格式: uint8[7]
// ------------------------------------------------------------------
type CounterMsg struct {
	_ [7]byte `struct:"[7]byte"`
}

// ------------------------------------------------------------------
// MSG_ATTACK — 攻击
// 格式: uint8[8]
// ------------------------------------------------------------------
type AttackMsg struct {
	_ [8]byte `struct:"[8]byte"`
}

// ------------------------------------------------------------------
// MSG_BATTLE — 战斗
// 格式: uint8[26]? 或更复杂
// tag_duel.go 中没看到单独处理，可能直接转发
// ------------------------------------------------------------------

// ------------------------------------------------------------------
// MSG_MISSED_EFFECT — 错过效果
// 格式: uint32 code + uint8[7]?
// ------------------------------------------------------------------
type MissedEffectMsg struct {
	Code uint32  `struct:"uint32"`
	_    [7]byte `struct:"[7]byte"`
}

// ------------------------------------------------------------------
// MSG_TOSS_COIN — 抛硬币
// 格式: uint8 player, uint8 count, uint8[count] results
// ------------------------------------------------------------------
type TossCoinMsg struct {
	Player  uint8   `struct:"uint8"`
	Count   uint8   `struct:"uint8,sizeof=Results"`
	Results []uint8 `struct:"[]uint8"`
}

// ------------------------------------------------------------------
// MSG_TOSS_DICE — 掷骰子
// 格式: uint8 player, uint8 count, uint8[count] results
// ------------------------------------------------------------------
type TossDiceMsg struct {
	Player  uint8   `struct:"uint8"`
	Count   uint8   `struct:"uint8,sizeof=Results"`
	Results []uint8 `struct:"[]uint8"`
}

// ------------------------------------------------------------------
// MSG_ANNOUNCE_RACE — 宣言种族
// 格式: uint8 player, uint8 count, uint8[count] races
// ------------------------------------------------------------------
type AnnounceRaceMsg struct {
	Player uint8   `struct:"uint8"`
	Count  uint8   `struct:"uint8,sizeof=Races"`
	Races  []uint8 `struct:"[]uint8"`
}

// ------------------------------------------------------------------
// MSG_ANNOUNCE_ATTRIB — 宣言属性
// 格式同 MSG_ANNOUNCE_RACE
// ------------------------------------------------------------------
type AnnounceAttribMsg = AnnounceRaceMsg

// ------------------------------------------------------------------
// MSG_ANNOUNCE_CARD / MSG_ANNOUNCE_NUMBER
// 格式: uint8 player, uint8 count, int32[count] values
// ------------------------------------------------------------------
type AnnounceCardMsg struct {
	Player uint8   `struct:"uint8"`
	Count  uint8   `struct:"uint8,sizeof=Values"`
	Values []int32 `struct:"[]int32"`
}

type AnnounceNumberMsg = AnnounceCardMsg

// ------------------------------------------------------------------
// MSG_CARD_HINT — 卡片提示
// 格式: uint8[9] 或 int32 code + uint8[5]?
// tag_duel.go 中 pbuf.Next(9)
// ------------------------------------------------------------------
type CardHintMsg struct {
	_ [9]byte `struct:"[9]byte"`
}

// ------------------------------------------------------------------
// MSG_PLAYER_HINT — 玩家提示
// 格式: uint8[6]
// ------------------------------------------------------------------
type PlayerHintMsg struct {
	_ [6]byte `struct:"[6]byte"`
}

// ------------------------------------------------------------------
// MSG_TAG_SWAP — 交换（双打）
// 格式: uint8 player, uint8 main_count, uint8 extra_count, uint8 side_count,
//
//	int32[main_count] main, int32[extra_count] extra, int32[side_count] side
//
// ------------------------------------------------------------------
type TagSwapMsg struct {
	Player     uint8   `struct:"uint8"`
	MainCount  uint8   `struct:"uint8,sizeof=Main"`
	ExtraCount uint8   `struct:"uint8,sizeof=Extra"`
	SideCount  uint8   `struct:"uint8,sizeof=Side"`
	Main       []int32 `struct:"[]int32"`
	Extra      []int32 `struct:"[]int32"`
	Side       []int32 `struct:"[]int32"`
}

// ------------------------------------------------------------------
// MSG_NEW_TURN — 新回合
// 格式: uint8 player
// ------------------------------------------------------------------
type NewTurnMsg struct {
	Player uint8 `struct:"uint8"`
}

// ------------------------------------------------------------------
// MSG_NEW_PHASE — 新阶段
// 格式: uint16 phase
// ------------------------------------------------------------------
type NewPhaseMsg struct {
	Phase uint16 `struct:"uint16"`
}

// ------------------------------------------------------------------
// MSG_WIN — 胜利
// 格式: uint8 player, uint8 type
// ------------------------------------------------------------------
type WinMsg struct {
	Player uint8 `struct:"uint8"`
	Type   uint8 `struct:"uint8"`
}

// ------------------------------------------------------------------
// MSG_DECK_TOP — 卡组顶部
// 格式: uint8 player, uint8 sequence, int32 code, uint8 position
// = 1 + 1 + 4 + 1 + 1 (padding?) = 7? 但代码中 pbuf.Next(6)
// 实际: player(1) + seq(1) + code(4?)... 但代码是 Next(6)，可能 code 是 uint16
// 暂不精确定义，用占位符
// ------------------------------------------------------------------
type DeckTopMsg struct {
	_ [6]byte `struct:"[6]byte"`
}

// ------------------------------------------------------------------
// MSG_SHUFFLE_SET_CARD — 设置卡片洗牌
// 格式: uint8 loc, uint8 count, [8 bytes] * count
// ------------------------------------------------------------------
type ShuffleSetCardMsg struct {
	Loc   uint8 `struct:"uint8"`
	Count uint8 `struct:"uint8,sizeof=Cards"`
	Cards []SetCardEntry
}

type SetCardEntry struct {
	_ [8]byte `struct:"[8]byte"`
}

// ------------------------------------------------------------------
// MSG_REFRESH_DECK — 刷新卡组
// 格式: uint8 player
// ------------------------------------------------------------------
type RefreshDeckMsg struct {
	_ byte `struct:"uint8"`
}

// ------------------------------------------------------------------
// MSG_REVERSE_DECK — 翻转卡组
// 无数据
// ------------------------------------------------------------------
type ReverseDeckMsg struct{}

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
