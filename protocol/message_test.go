package protocol

// 黄金往返测试：按引擎源码（source/ygopro/ocgcore/*.cpp）的 write_buffer 序列
// 手工构造线上字节 → Unpack → 字段断言 → Pack → 与原始字节全等。
// "一切以结构体为准"由此变成可执行事实：布局漂移会让任何一条用例立刻变红。

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/go-restruct/restruct"
)

func le16(v uint16) []byte { b := make([]byte, 2); binary.LittleEndian.PutUint16(b, v); return b }
func le32(v uint32) []byte { b := make([]byte, 4); binary.LittleEndian.PutUint32(b, v); return b }

// locBytes 是 PackPackedLoc 的字节形式：c, l, s, pos 依次各 1 字节
func locBytes(c, l, s, pos uint8) []byte { return []byte{c, l, s, pos} }

type roundTripCase struct {
	name  string
	wire  []byte
	new   func() any
	check func(t *testing.T, msg any)
}

func TestMessageGoldenRoundTrip(t *testing.T) {
	cases := []roundTripCase{
		{
			name: "SelectCardMsg",
			// player, cancelable, min, max, count, 条目×2 (code4 + c,l,s,p)
			wire: concat([]byte{0, 1, 1, 2, 2},
				le32(12345), []byte{0, 4, 1, 1},
				le32(678), []byte{1, 2, 0, 2}),
			new: func() any { return &SelectCardMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*SelectCardMsg)
				if msg.Player != 0 || msg.Cancelable != 1 || msg.Min != 1 || msg.Max != 2 || len(msg.Cards) != 2 {
					t.Fatalf("header: %+v", msg)
				}
				if msg.Cards[0].Code != 12345 || msg.Cards[0].Controller != 0 ||
					msg.Cards[0].Location != 4 || msg.Cards[0].Sequence != 1 || msg.Cards[0].Position != 1 {
					t.Fatalf("card0: %+v", msg.Cards[0])
				}
				if msg.Cards[1].Code != 678 || msg.Cards[1].Controller != 1 || msg.Cards[1].Sequence != 0 {
					t.Fatalf("card1: %+v", msg.Cards[1])
				}
			},
		},
		{
			name: "SelectUnselectCardMsg",
			// player, finishable, cancelable, min, max, countA, 条目, countB, 条目
			wire: concat([]byte{1, 1, 0, 1, 2, 1},
				le32(21), []byte{0, 4, 0, 1},
				[]byte{1},
				le32(22), []byte{1, 4, 1, 2}),
			new: func() any { return &SelectUnselectCardMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*SelectUnselectCardMsg)
				if msg.Finishable != 1 || msg.Cancelable != 0 || msg.Min != 1 || msg.Max != 2 {
					t.Fatalf("header: %+v", msg)
				}
				if len(msg.Cards1) != 1 || msg.Cards1[0].Code != 21 || msg.Cards1[0].Controller != 0 {
					t.Fatalf("cards1: %+v", msg.Cards1)
				}
				if len(msg.Cards2) != 1 || msg.Cards2[0].Code != 22 || msg.Cards2[0].Controller != 1 || msg.Cards2[0].Sequence != 1 {
					t.Fatalf("cards2: %+v", msg.Cards2)
				}
			},
		},
		{
			name: "CounterMsg",
			// card.cpp:2357-2361 的 7 字节布局：type(2)+c(1)+l(1)+s(1)+count(2)
			// ——注意没有 get_info_location 的 pos 字节
			wire: concat(le16(0x1051), []byte{0, 4, 2}, le16(3)),
			new:  func() any { return &CounterMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*CounterMsg)
				if msg.Type != 0x1051 || msg.CC != 0 || msg.CL != 4 || msg.CS != 2 || msg.Count != 3 {
					t.Fatalf("counter: %+v", msg)
				}
			},
		},
		{
			name: "MatchKillMsg",
			// operations.cpp:535：write_buffer32(code)
			wire: le32(7086090),
			new:  func() any { return &MatchKillMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*MatchKillMsg)
				if msg.Code != 7086090 {
					t.Fatalf("match kill code: %+v", msg)
				}
			},
		},
		{
			name: "SelectCounterMsg",
			// player, countertype(2), count(2), cardcount, 条目×2 (code4+c+l+s+cnt2)
			wire: concat([]byte{0}, le16(1), le16(3), []byte{2},
				le32(12345), []byte{0, 4, 1}, le16(2),
				le32(678), []byte{1, 4, 2}, le16(1)),
			new: func() any { return &SelectCounterMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*SelectCounterMsg)
				if msg.CounterType != 1 || msg.Count != 3 || len(msg.Entries) != 2 {
					t.Fatalf("header: %+v", msg)
				}
				if msg.Entries[0].Code != 12345 || msg.Entries[0].Count != 2 ||
					msg.Entries[1].Controller != 1 || msg.Entries[1].Count != 1 {
					t.Fatalf("entries: %+v", msg.Entries)
				}
			},
		},
		{
			name: "SelectSumMsg",
			// sumMode, player, acc(4), min, max, mustCount, 条目(11), selCount, 条目(11)
			wire: concat([]byte{0, 0}, le32(7), []byte{1, 3, 1},
				le32(9), []byte{0, 4, 0}, le32(2),
				[]byte{2},
				le32(1), []byte{0, 4, 0}, le32(5),
				le32(2), []byte{0, 4, 1}, le32(4)),
			new: func() any { return &SelectSumMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*SelectSumMsg)
				if msg.SumMode != 0 || msg.Acc != 7 || msg.Min != 1 || msg.Max != 3 {
					t.Fatalf("header: %+v", msg)
				}
				if len(msg.Must) != 1 || msg.Must[0].Param != 2 {
					t.Fatalf("must: %+v", msg.Must)
				}
				if len(msg.Select) != 2 || msg.Select[1].Sequence != 1 || msg.Select[1].Param != 4 {
					t.Fatalf("select: %+v", msg.Select)
				}
			},
		},
		{
			name: "SortCardMsg",
			// player, count, 条目×3 (code4 + c,l,s)
			wire: concat([]byte{0, 3},
				le32(11), []byte{0, 1, 0},
				le32(12), []byte{0, 1, 1},
				le32(13), []byte{0, 1, 2}),
			new: func() any { return &SortCardMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*SortCardMsg)
				if len(msg.Cards) != 3 || msg.Cards[2].Code != 13 || msg.Cards[2].Sequence != 2 {
					t.Fatalf("cards: %+v", msg.Cards)
				}
			},
		},
		{
			name: "SelectPlaceMsg",
			// player, count, flag(4)，无条目
			wire: concat([]byte{0, 1}, le32(0x1234)),
			new:  func() any { return &SelectPlaceMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*SelectPlaceMsg)
				if msg.Count != 1 || msg.Flag != 0x1234 {
					t.Fatalf("msg: %+v", msg)
				}
			},
		},
		{
			name: "SelectChainMsg",
			// player, count, spe_count, hint0(4), hint1(4), 条目(14B = flag+forced+code4+info4+desc4)
			wire: concat([]byte{0, 1, 0}, le32(0x10), le32(0x20),
				[]byte{0, 1}, le32(99), locBytes(0, 4, 2, 1), le32(7)),
			new: func() any { return &SelectChainMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*SelectChainMsg)
				if len(msg.Chains) != 1 {
					t.Fatalf("chains: %+v", msg.Chains)
				}
				ch := msg.Chains[0]
				if ch.DescFlag != 0 || ch.Forced != 1 || ch.Code != 99 || ch.Description != 7 {
					t.Fatalf("chain: %+v", ch)
				}
				c, l, s, pos := UnpackPackedLoc(ch.InfoLocation)
				if c != 0 || l != 4 || s != 2 || pos != 1 {
					t.Fatalf("info_location: %d -> %d,%d,%d,%d", ch.InfoLocation, c, l, s, pos)
				}
			},
		},
		{
			name: "SelectBattleCmdMsg",
			// player, 激活数, 激活条目(12B), 攻击数, 攻击条目(8B)×2, toM2, toEP
			wire: concat([]byte{0, 1},
				le32(86039056), []byte{0, 4, 1}, le32(1234),
				[]byte{2},
				le32(70902743), []byte{0, 4, 0, 0},
				le32(70902743), []byte{0, 4, 1, 1},
				[]byte{1, 0}),
			new: func() any { return &SelectBattleCmdMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*SelectBattleCmdMsg)
				if len(msg.CmdsA) != 1 || msg.CmdsA[0].Code != 86039056 || msg.CmdsA[0].Description != 1234 {
					t.Fatalf("cmdsA: %+v", msg.CmdsA)
				}
				if len(msg.CmdsB) != 2 || msg.CmdsB[1].DirectAttackable != 1 || msg.CmdsB[1].CS != 1 {
					t.Fatalf("cmdsB: %+v", msg.CmdsB)
				}
				if msg.ToM2 != 1 || msg.ToEP != 0 {
					t.Fatalf("toM2/toEP: %+v", msg)
				}
			},
		},
		{
			name: "SelectIdleCmdMsg",
			// player, 5×[count(1)+条目(8B)], 激活数, 激活条目(12B), toBP, toEP, shuffle
			wire: concat([]byte{0},
				concat([]byte{1}, le32(1), []byte{0, 4, 0}), // summon
				concat([]byte{1}, le32(2), []byte{0, 4, 1}), // spsummon
				concat([]byte{1}, le32(3), []byte{0, 4, 2}), // repos
				concat([]byte{1}, le32(4), []byte{0, 4, 3}), // mset
				concat([]byte{1}, le32(5), []byte{0, 4, 4}), // sset
				[]byte{1}, le32(6), []byte{0, 4, 5}, le32(66), // activate
				[]byte{1, 1, 0}),
			new: func() any { return &SelectIdleCmdMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*SelectIdleCmdMsg)
				if len(msg.CmdsA) != 1 || msg.CmdsA[0].Code != 1 ||
					len(msg.CmdsE) != 1 || msg.CmdsE[0].Code != 5 {
					t.Fatalf("card lists: %+v", msg)
				}
				if len(msg.CmdsF) != 1 || msg.CmdsF[0].Code != 6 || msg.CmdsF[0].Description != 66 {
					t.Fatalf("activate: %+v", msg.CmdsF)
				}
				if msg.ToBP != 1 || msg.ToEP != 1 || msg.CanShuffle != 0 {
					t.Fatalf("flags: %+v", msg)
				}
			},
		},
		{
			name: "ChainingMsg",
			// code(4), 发动卡位置 c,l,s,pos, 触发 c,l,s, description(4), chain_count
			wire: concat(le32(86039056), []byte{0, 2, 1, 5}, []byte{0, 2, 1}, le32(0x123456), []byte{1}),
			new:  func() any { return &ChainingMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*ChainingMsg)
				if msg.Code != 86039056 || msg.CardController != 0 || msg.CardLocation != 2 ||
					msg.CardSequence != 1 || msg.CardPosition != 5 {
					t.Fatalf("card info: %+v", msg)
				}
				if msg.TriggeringController != 0 || msg.TriggeringLocation != 2 ||
					msg.TriggeringSequence != 1 || msg.Description != 0x123456 || msg.ChainCount != 1 {
					t.Fatalf("triggering info: %+v", msg)
				}
			},
		},
		{
			name: "ConfirmDeckTopMsg",
			// player, count, 条目×2 (code4 + c,l,s) = 7B each
			wire: concat([]byte{0, 2},
				le32(111), []byte{0, 1, 39},
				le32(222), []byte{0, 1, 40}),
			new: func() any { return &ConfirmDeckTopMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*ConfirmDeckTopMsg)
				if len(msg.Cards) != 2 || msg.Cards[1].Code != 222 || msg.Cards[1].Sequence != 40 {
					t.Fatalf("cards: %+v", msg.Cards)
				}
			},
		},
		{
			name: "BecomeTargetMsg",
			// count, get_info_location × count
			wire: concat([]byte{2}, le32(PackPackedLoc(0, 4, 0, 1)), le32(PackPackedLoc(1, 4, 2, 5))),
			new:  func() any { return &BecomeTargetMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*BecomeTargetMsg)
				if len(msg.Targets) != 2 {
					t.Fatalf("targets: %+v", msg.Targets)
				}
				c, l, s, pos := UnpackPackedLoc(msg.Targets[1])
				if c != 1 || l != 4 || s != 2 || pos != 5 {
					t.Fatalf("target1 unpack: %d,%d,%d,%d", c, l, s, pos)
				}
			},
		},
		{
			name: "AttackMsg",
			wire: concat(le32(PackPackedLoc(0, 4, 1, 1)), le32(PackPackedLoc(1, 4, 2, 1))),
			new:  func() any { return &AttackMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*AttackMsg)
				c, _, _, _ := UnpackPackedLoc(msg.AttackerInfo)
				if c != 0 {
					t.Fatalf("attacker: %d", msg.AttackerInfo)
				}
				c, _, _, _ = UnpackPackedLoc(msg.TargetInfo)
				if c != 1 {
					t.Fatalf("target: %d", msg.TargetInfo)
				}
			},
		},
		{
			name: "BattleMsg",
			// 攻击方位置(4) + aa(4) + ad(4) + bd0(1) + 对象方位置(4) + da(4) + dd(4) + bd1(1)
			wire: concat(le32(PackPackedLoc(0, 4, 1, 1)), le32(1900), le32(100), []byte{0},
				le32(PackPackedLoc(1, 4, 2, 1)), le32(800), le32(1500), []byte{0}),
			new: func() any { return &BattleMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*BattleMsg)
				if msg.AttackerATK != 1900 || msg.AttackerDEF != 100 || msg.AttackerDirect != 0 ||
					msg.TargetATK != 800 || msg.TargetDEF != 1500 || msg.TargetDirect != 0 {
					t.Fatalf("battle: %+v", msg)
				}
			},
		},
		{
			name: "DrawMsg",
			// player, count, code|公开标记
			wire: concat([]byte{0, 2}, le32(123|0x80000000), le32(456)),
			new:  func() any { return &DrawMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*DrawMsg)
				if len(msg.Cards) != 2 || !msg.IsCardKnown(0) || msg.IsCardKnown(1) {
					t.Fatalf("cards: %+v", msg.Cards)
				}
			},
		},
		{
			name: "MoveMsg",
			// code(4) + prev c,l,s,pos + curr c,l,s,pos + reason(4) = 16B
			wire: concat(le32(123), []byte{0, 2, 0, 5}, []byte{0, 4, 1, 1}, le32(0x2000)),
			new:  func() any { return &MoveMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*MoveMsg)
				if msg.PL != 2 || msg.PS != 0 || msg.CL != 4 || msg.CS != 1 || msg.Reason != 0x2000 {
					t.Fatalf("move: %+v", msg)
				}
			},
		},
		{
			name: "PosChangeMsg",
			// code(4) + c,l,s + previous.position + current.position = 9B
			wire: concat(le32(123), []byte{0, 4, 1, 2, 5}),
			new:  func() any { return &PosChangeMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*PosChangeMsg)
				if msg.PP != 2 || msg.CP != 5 || msg.CS != 1 {
					t.Fatalf("pos change: %+v", msg)
				}
			},
		},
		{
			name: "SetMsg/SummoningMsg",
			wire: concat(le32(999), []byte{0, 4, 2, 1}),
			new:  func() any { return &SetMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*SetMsg)
				if msg.Code != 999 || msg.CC != 0 || msg.CL != 4 || msg.CS != 2 || msg.CP != 1 {
					t.Fatalf("set: %+v", msg)
				}
			},
		},
		{
			name: "SummoningMsg",
			wire: concat(le32(888), []byte{1, 4, 0, 1}),
			new:  func() any { return &SummoningMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*SummoningMsg)
				if msg.CC != 1 || msg.CL != 4 || msg.CS != 0 || msg.CP != 1 {
					t.Fatalf("summoning: %+v", msg)
				}
			},
		},
		{
			name: "SelectEffectYNMsg",
			wire: concat([]byte{0}, le32(123), le32(PackPackedLoc(0, 2, 0, 1)), le32(456)),
			new:  func() any { return &SelectEffectYNMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*SelectEffectYNMsg)
				if msg.Code != 123 || msg.InfoLocation != PackPackedLoc(0, 2, 0, 1) || msg.Description != 456 {
					t.Fatalf("effectyn: %+v", msg)
				}
			},
		},
		{
			name: "SelectYesNoMsg",
			wire: concat([]byte{0}, le32(789)),
			new:  func() any { return &SelectYesNoMsg{} },
			check: func(t *testing.T, m any) {
				if m.(*SelectYesNoMsg).Description != 789 {
					t.Fatalf("yesno: %+v", m)
				}
			},
		},
		{
			name: "SelectOptionMsg",
			wire: concat([]byte{0, 2}, le32(1), le32(2)),
			new:  func() any { return &SelectOptionMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*SelectOptionMsg)
				if len(msg.Options) != 2 || msg.Options[1] != 2 {
					t.Fatalf("options: %+v", msg.Options)
				}
			},
		},
		{
			name: "SelectPositionMsg",
			wire: concat([]byte{0}, le32(123), []byte{3}),
			new:  func() any { return &SelectPositionMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*SelectPositionMsg)
				if msg.Code != 123 || msg.Positions != 3 {
					t.Fatalf("position: %+v", msg)
				}
			},
		},
		{
			name: "TossCoinMsg",
			wire: concat([]byte{0, 3, 1, 0, 1}),
			new:  func() any { return &TossCoinMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*TossCoinMsg)
				if len(msg.Results) != 3 || msg.Results[0] != 1 || msg.Results[1] != 0 {
					t.Fatalf("toss: %+v", msg)
				}
			},
		},
		{
			name: "AnnounceRaceMsg",
			wire: concat([]byte{0, 2}, le32(0x2003)),
			new:  func() any { return &AnnounceRaceMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*AnnounceRaceMsg)
				if msg.Count != 2 || msg.Available != 0x2003 {
					t.Fatalf("announce race: %+v", msg)
				}
			},
		},
		{
			name: "AnnounceCardMsg",
			wire: concat([]byte{0, 2}, le32(86039056), le32(0x40000100)),
			new:  func() any { return &AnnounceCardMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*AnnounceCardMsg)
				if len(msg.Values) != 2 || msg.Values[0] != 86039056 {
					t.Fatalf("announce card: %+v", msg.Values)
				}
			},
		},
		{
			name: "RockPaperScissorsMsg",
			wire: []byte{0},
			new:  func() any { return &RockPaperScissorsMsg{} },
			check: func(t *testing.T, m any) {
				if m.(*RockPaperScissorsMsg).Phase != 0 {
					t.Fatalf("rps: %+v", m)
				}
			},
		},
		{
			name: "HandResMsg",
			wire: []byte{0x25},
			new:  func() any { return &HandResMsg{} },
			check: func(t *testing.T, m any) {
				if m.(*HandResMsg).Res != 0x25 {
					t.Fatalf("hand res: %+v", m)
				}
			},
		},
		{
			name: "ChainSolvingMsg",
			wire: []byte{3},
			new:  func() any { return &ChainSolvingMsg{} },
			check: func(t *testing.T, m any) {
				if m.(*ChainSolvingMsg).Count != 3 {
					t.Fatalf("chain solving: %+v", m)
				}
			},
		},
		{
			name: "WinMsg",
			wire: []byte{1, 2},
			new:  func() any { return &WinMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*WinMsg)
				if msg.Player != 1 || msg.Type != 2 {
					t.Fatalf("win: %+v", msg)
				}
			},
		},
		{
			name: "NewPhaseMsg",
			wire: le16(0x19),
			new:  func() any { return &NewPhaseMsg{} },
			check: func(t *testing.T, m any) {
				if m.(*NewPhaseMsg).Phase != 0x19 {
					t.Fatalf("new phase: %+v", m)
				}
			},
		},
		{
			name: "DamageMsg",
			wire: concat([]byte{0}, le32(1000)),
			new:  func() any { return &DamageMsg{} },
			check: func(t *testing.T, m any) {
				if m.(*DamageMsg).Value != 1000 {
					t.Fatalf("damage: %+v", m)
				}
			},
		},
		{
			name: "LPUpdateMsg",
			wire: concat([]byte{1}, le32(4000)),
			new:  func() any { return &LPUpdateMsg{} },
			check: func(t *testing.T, m any) {
				if m.(*LPUpdateMsg).LP != 4000 {
					t.Fatalf("lp update: %+v", m)
				}
			},
		},
		{
			name: "PayLPCostMsg",
			wire: concat([]byte{0}, le32(800)),
			new:  func() any { return &PayLPCostMsg{} },
			check: func(t *testing.T, m any) {
				if m.(*PayLPCostMsg).Value != 800 {
					t.Fatalf("pay cost: %+v", m)
				}
			},
		},
		{
			name: "ShuffleHandMsg",
			wire: concat([]byte{0, 2}, le32(10), le32(20)),
			new:  func() any { return &ShuffleHandMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*ShuffleHandMsg)
				if len(msg.Cards) != 2 || msg.Cards[1] != 20 {
					t.Fatalf("shuffle hand: %+v", msg)
				}
			},
		},
		{
			name: "DeckTopMsg",
			wire: concat([]byte{0, 0}, le32(123)),
			new:  func() any { return &DeckTopMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*DeckTopMsg)
				if msg.Player != 0 || msg.Sequence != 0 || msg.Code != 123 {
					t.Fatalf("deck top: %+v", msg)
				}
			},
		},
		{
			name: "TagSwapMsg",
			wire: concat([]byte{1, 2, 1, 0},
				le32(1), le32(2),
				le32(3)),
			new: func() any { return &TagSwapMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*TagSwapMsg)
				if msg.Player != 1 || len(msg.Main) != 2 || len(msg.Extra) != 1 || len(msg.Side) != 0 {
					t.Fatalf("tag swap: %+v", msg)
				}
			},
		},
		{
			name: "CardHintMsg",
			wire: concat(le32(PackPackedLoc(0, 5, 0, 1)), []byte{3}, le32(456)),
			new:  func() any { return &CardHintMsg{} },
			check: func(t *testing.T, m any) {
				msg := m.(*CardHintMsg)
				if msg.Type != 3 || msg.Data != 456 {
					t.Fatalf("card hint: %+v", msg)
				}
			},
		},
		{
			name: "MissedEffectMsg",
			wire: concat(le32(PackPackedLoc(0, 2, 0, 1)), le32(123)),
			new:  func() any { return &MissedEffectMsg{} },
			check: func(t *testing.T, m any) {
				if m.(*MissedEffectMsg).Code != 123 {
					t.Fatalf("missed effect: %+v", m)
				}
			},
		},
		{
			name: "FieldDisabledMsg",
			wire: le32(0xffff),
			new:  func() any { return &FieldDisabledMsg{} },
			check: func(t *testing.T, m any) {
				if m.(*FieldDisabledMsg).Zones != 0xffff {
					t.Fatalf("field disabled: %+v", m)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			msg := tc.new()
			if err := UnpackGameMsg(tc.wire, msg); err != nil {
				t.Fatalf("Unpack: %v", err)
			}
			tc.check(t, msg)
			packed := PackGameMsg(msg)
			if !bytes.Equal(packed, tc.wire) {
				t.Fatalf("Pack != wire:\n wire  = %v\n packed= %v", tc.wire, packed)
			}
		})
	}
}

// TestMessageStaticSizes 锁定条目结构的固定字节数，布局漂移第一时间暴露
func TestMessageStaticSizes(t *testing.T) {
	sizes := []struct {
		name string
		v    any
		want int
	}{
		{"SelectCardEntry", SelectCardEntry{}, 8},
		{"CounterEntry", CounterEntry{}, 9},
		{"SumEntry", SumEntry{}, 11},
		{"SortCardEntry", SortCardEntry{}, 7},
		{"ConfirmCardEntry", ConfirmCardEntry{}, 7},
		{"ChainEntry", ChainEntry{}, 14},
		{"CmdCardEntry", CmdCardEntry{}, 7},
		{"CmdActivateEntry", CmdActivateEntry{}, 11},
		{"CmdAttackEntry", CmdAttackEntry{}, 8},
		{"ChainingMsg", ChainingMsg{}, 16},
		{"MoveMsg", MoveMsg{}, 16},
		{"BattleMsg", BattleMsg{}, 26},
		{"SetMsg", SetMsg{}, 8},
		{"PosChangeMsg", PosChangeMsg{}, 9},
		{"AttackMsg", AttackMsg{}, 8},
		{"SelectPlaceMsg", SelectPlaceMsg{}, 6},
		{"SelectEffectYNMsg", SelectEffectYNMsg{}, 13},
		{"SelectYesNoMsg", SelectYesNoMsg{}, 5},
		{"SelectPositionMsg", SelectPositionMsg{}, 6},
		{"HintMsg", HintMsg{}, 6},
		{"CardHintMsg", CardHintMsg{}, 9},
		{"PlayerHintMsg", PlayerHintMsg{}, 6},
		{"HandResMsg", HandResMsg{}, 1},
		{"RockPaperScissorsMsg", RockPaperScissorsMsg{}, 1},
		{"DeckTopMsg", DeckTopMsg{}, 6},
		{"MissedEffectMsg", MissedEffectMsg{}, 8},
		{"CounterMsg", CounterMsg{}, 7},
		{"MatchKillMsg", MatchKillMsg{}, 4},
	}
	for _, s := range sizes {
		got, err := restruct.SizeOf(s.v)
		if err != nil {
			t.Errorf("%s: SizeOf error: %v", s.name, err)
			continue
		}
		if int(got) != s.want {
			t.Errorf("%s: SizeOf = %d, want %d", s.name, got, s.want)
		}
	}
}

// concat 是测试里的字节拼接辅助
func concat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// TestHideCodesForPlayer 驱动服务端 Analyze 的隐私掩码行为：
// 发给某玩家前，非该玩家的卡片代码必须归零，且字节布局不变
func TestHideCodesForPlayer(t *testing.T) {
	sel := &SelectCardMsg{
		Player:     1,
		Cancelable: 1,
		Min:        1,
		Max:        1,
		Cards: []SelectCardEntry{
			{Code: 111, Controller: 0, Location: 4, Sequence: 0, Position: 1},
			{Code: 222, Controller: 1, Location: 4, Sequence: 1, Position: 1},
		},
	}
	sel.HideCodesForPlayer(1)
	if sel.Cards[0].Code != 0 {
		t.Fatalf("opponent card should be hidden: %+v", sel.Cards[0])
	}
	if sel.Cards[1].Code != 222 {
		t.Fatalf("own card should stay: %+v", sel.Cards[1])
	}
	// 归零后重新打包，字节长度必须与原始布局一致（8B/条目）
	packed := sel.Pack()
	if len(packed) != 5+2*8 {
		t.Fatalf("packed size = %d, want %d", len(packed), 5+2*8)
	}

	uns := &SelectUnselectCardMsg{
		Player: 0,
		Cards1: []SelectCardEntry{{Code: 333, Controller: 1, Location: 4, Sequence: 2}},
		Cards2: []SelectCardEntry{{Code: 444, Controller: 1, Location: 4, Sequence: 3}},
	}
	uns.HideCodesForPlayer(0)
	if uns.Cards1[0].Code != 0 || uns.Cards2[0].Code != 0 {
		t.Fatalf("both lists should be masked: %+v %+v", uns.Cards1, uns.Cards2)
	}
	if len(uns.Pack()) != 6+8+1+8 {
		t.Fatalf("packed size mismatch: %d", len(uns.Pack()))
	}
}

// TestDrawHideUnknownCards 抽卡隐私：无 0x80000000 公开标记的代码归零
func TestDrawHideUnknownCards(t *testing.T) {
	pub := uint32(123) | uint32(0x80000000)
	draw := &DrawMsg{Player: 1, Cards: []int32{int32(pub), 456}}
	draw.HideUnknownCards()
	if draw.Cards[0] != int32(pub) {
		t.Fatalf("public card should stay: %d", draw.Cards[0])
	}
	if draw.Cards[1] != 0 {
		t.Fatalf("hidden card should be zeroed: %d", draw.Cards[1])
	}
}
