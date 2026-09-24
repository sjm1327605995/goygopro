package duel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/panjf2000/gnet/v2"
	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// stubConn 满足 gnet.Conn：嵌入空接口以补齐全部方法，仅覆盖 Write 为吞掉写。
// 本测试只断言 handler 消费的字节数，写路径一律 no-op，避免真实连接依赖。
type stubConn struct{ gnet.Conn }

func (stubConn) Write(p []byte) (int, error) { return len(p), nil }

// analyzeFakeRoom 用带桩玩家的 SingleDuel 满足 duelRoom 接口，
// 刷新/胜负/结束钩子全部置空 —— 本测试只断言「handler 消费的字节数
// 与 engineMsgLayouts 布局一致」，不断言路由内容。
type analyzeFakeRoom struct {
	*SingleDuel
}

func (f *analyzeFakeRoom) EndDuel()                                           {}
func (f *analyzeFakeRoom) onEngineWin(player uint8)                           {}
func (f *analyzeFakeRoom) refreshGraveAfterSwap(int)                          {}
func (f *analyzeFakeRoom) refreshAfterSummon()                                {}
func (f *analyzeFakeRoom) refreshAfterChain()                                 {}
func (f *analyzeFakeRoom) refreshAfterDamageStep()                            {}
func (f *analyzeFakeRoom) refreshAfterNewPhase()                              {}
func (f *analyzeFakeRoom) refreshSingleMoved(uint8, uint8, uint8)             {}
func (f *analyzeFakeRoom) refreshSingleFlip(uint8, uint8, uint8)              {}
func (f *analyzeFakeRoom) RefreshHand(player int, flag uint32, useCache int)  {}
func (f *analyzeFakeRoom) RefreshExtra(player int, flag uint32, useCache int) {}
func (f *analyzeFakeRoom) RefreshMzone(player int, flag uint32, useCache int) {}
func (f *analyzeFakeRoom) RefreshSzone(player int, flag uint32, useCache int) {}

// TestAnalyzeLayoutMatchesBatchWalker 强制 Analyze 的每个 handler 与
// batchResponseOffset 走查同一份布局（engineMsgLayouts）：对全零合成负载，
// handler 消费的字节数必须等于布局表走查的结果。这消除了历史上
// message_scan 与 Analyze 各写一份布局导致的事实分叉（如 MSG_SELECT_SUM
// 的 player 偏移）。
//
// 不覆盖：MSG_NEW_TURN（single 具体方法，刷新前置于跳过，布局 trivially 1 字节）、
// MSG_TAG_SWAP（tag 具体方法，其显式解析与 layTagSwap 步同源）。
func TestAnalyzeLayoutMatchesBatchWalker(t *testing.T) {
	// 共享表与走字节跳过路线的 single 特有消息只要满足 duelRoom 接口即可；
	// 带 m.(*SingleDuel) 类型断言的 MSG_MATCH_KILL 必须给真实 *SingleDuel。
	fake := &analyzeFakeRoom{SingleDuel: &SingleDuel{}}
	fake.players[0] = &DuelPlayer{Conn: stubConn{}}
	fake.players[1] = &DuelPlayer{Conn: stubConn{}}
	realSingle := &SingleDuel{}
	realSingle.players[0] = &DuelPlayer{Conn: stubConn{}}
	realSingle.players[1] = &DuelPlayer{Conn: stubConn{}}

	tables := []struct {
		name  string
		table map[uint8]analyzeHandler
		room  duelRoom
	}{
		{"shared", sharedAnalyzeHandlers, fake},
		{"single-hand-res", map[uint8]analyzeHandler{ocgcore.MSG_HAND_RES: singleAnalyzeTable[ocgcore.MSG_HAND_RES]}, fake},
		{"single-match-kill", map[uint8]analyzeHandler{ocgcore.MSG_MATCH_KILL: singleAnalyzeTable[ocgcore.MSG_MATCH_KILL]}, realSingle},
	}

	for _, grp := range tables {
		for msgType, h := range grp.table {
			t.Run(fmt.Sprintf("%s/0x%02x", grp.name, msgType), func(t *testing.T) {
				payload := make([]byte, 64)
				payload[0] = msgType
				end, _, win, ok := walkEngineMessage(payload, 1)
				if !ok {
					t.Fatal("walker 不识别该消息（engineMsgLayouts 缺少条目）")
				}
				want := end - 1
				if win {
					// MSG_WIN 为终局消息，walker 不走查字段；handler 读 player+typ 共 2 字节。
					want = 2
				}

				pbuf := utils.NewYGOBuffer(payload, binary.LittleEndian)
				offset := pbuf.Clone()
				var engType uint8
				if err := pbuf.Read(&engType); err != nil {
					t.Fatal(err)
				}
				h(grp.room, engType, pbuf, offset)
				if got := pbuf.Offset() - 1; got != want {
					t.Fatalf("handler 消费 %d 字节，布局表为 %d 字节（布局漂移？）", got, want)
				}
			})
		}
	}
}

// TestAnalyzeTableModeSpecifics 守住各模式表的特有/排除条目，
// 防止模式特有的消息被误加进共享表（改变该模式的原版 fall-through 语义）。
func TestAnalyzeTableModeSpecifics(t *testing.T) {
	sharedMust := []uint8{
		ocgcore.MSG_RETRY, ocgcore.MSG_WIN, ocgcore.MSG_HINT,
		ocgcore.MSG_SELECT_SUM, ocgcore.MSG_SELECT_UNSELECT_CARD,
		ocgcore.MSG_MOVE, ocgcore.MSG_DRAW, ocgcore.MSG_SHUFFLE_HAND,
		ocgcore.MSG_CHAIN_END, ocgcore.MSG_NEW_PHASE,
	}
	for _, mt := range sharedMust {
		if _, ok := sharedAnalyzeHandlers[mt]; !ok {
			t.Errorf("共享表缺少 0x%02x", mt)
		}
		if _, ok := singleAnalyzeTable[mt]; !ok {
			t.Errorf("single 表缺少 0x%02x", mt)
		}
		if _, ok := tagAnalyzeTable[mt]; !ok {
			t.Errorf("tag 表缺少 0x%02x", mt)
		}
	}

	// 原版 tag 的 switch 没有 MSG_HAND_RES：落入无 case 分支（只消费类型字节），
	// 绝不能注册 handler（否则会吃掉后续字节，改变批次解析）。
	if _, ok := tagAnalyzeTable[ocgcore.MSG_HAND_RES]; ok {
		t.Error("tag 表不应包含 MSG_HAND_RES（原版 switch 无此 case）")
	}
	if _, ok := singleAnalyzeTable[ocgcore.MSG_TAG_SWAP]; ok {
		t.Error("single 表不应包含 MSG_TAG_SWAP")
	}
	for _, mt := range []uint8{ocgcore.MSG_NEW_TURN, ocgcore.MSG_MATCH_KILL} {
		if _, ok := singleAnalyzeTable[mt]; !ok {
			t.Errorf("single 表缺少特有消息 0x%02x", mt)
		}
		if _, ok := tagAnalyzeTable[mt]; !ok {
			t.Errorf("tag 表缺少特有消息 0x%02x", mt)
		}
	}
	if _, ok := tagAnalyzeTable[ocgcore.MSG_TAG_SWAP]; !ok {
		t.Error("tag 表缺少 MSG_TAG_SWAP")
	}
}

// TestNewlyRegisteredMessageLayouts 覆盖显式降级登记的五条消息：
// MSG_SORT_CHAIN(21)、MSG_ANNOUNCE_CARD_FILTER(144)、MSG_REQUEST_DECK(8)、
// MSG_DUEL_WINNER(200)、MSG_CUSTOM_MSG(180)。
// 断言两件事：
//  1. 布局表（walker）认识它们，且走查消费与登记的体长一致；
//  2. 模式表无 handler 时 runAnalyze 按布局安全跳过、后续消息保持对齐：
//     消息体后紧跟一条完整的 MSG_NEW_PHASE，players[0] 收到的广播字节
//     必须恰好只有这一条（一个数据包）。旧实现只消费类型字节，体里等于
//     表内类型的字节（SORT_CHAIN 的 player 字节故意取 0x29）会被当成
//     MSG_NEW_PHASE 命中，多广播一条错位数据。
func TestNewlyRegisteredMessageLayouts(t *testing.T) {
	// SORT_CHAIN 体：player(0x29) + count(1) + 1×(code4+cc+cl+cs 全 0xaa)。
	// 0x29 是 MSG_NEW_PHASE 的类型字节，0xaa 不在任何 analyze 表里。
	sortChainBody := append([]byte{0x29, 0x01}, bytes.Repeat([]byte{0xaa}, 7)...)
	cases := []struct {
		name string
		typ  uint8
		body []byte
	}{
		{"sort-chain", ocgcore.MSG_SORT_CHAIN, sortChainBody},
		{"announce-card-filter", ocgcore.MSG_ANNOUNCE_CARD_FILTER, nil},
		{"request-deck", ocgcore.MSG_REQUEST_DECK, nil},
		{"duel-winner", ocgcore.MSG_DUEL_WINNER, nil},
		{"custom-msg", ocgcore.MSG_CUSTOM_MSG, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := append([]byte{tc.typ}, tc.body...)
			end, _, _, ok := walkEngineMessage(payload, 1)
			if !ok {
				t.Fatal("walker 不识别该消息（engineMsgLayouts 缺少条目）")
			}
			if got := end - 1; got != len(tc.body) {
				t.Fatalf("walker 消费 %d 字节，期望 %d（布局登记漂移？）", got, len(tc.body))
			}

			fake := &analyzeFakeRoom{SingleDuel: &SingleDuel{}}
			fake.players[0] = &DuelPlayer{Conn: &recordConn{}}
			fake.players[1] = &DuelPlayer{Conn: &recordConn{}}

			// 完整尾随消息：MSG_NEW_PHASE + 2 字节体，runAnalyze 应干净走完（0）。
			batch := append(append([]byte{tc.typ}, tc.body...), ocgcore.MSG_NEW_PHASE, 0, 0)
			if r := runAnalyze(fake, sharedAnalyzeHandlers, batch); r != 0 {
				t.Fatalf("runAnalyze 返回 %d，期望 0", r)
			}

			// players[0] 只收到 NEW_PHASE 这一条广播（u16 长=4 + proto + 3 字节消息）。
			want := []byte{4, 0, network.STOC_GAME_MSG, ocgcore.MSG_NEW_PHASE, 0, 0}
			if got := fake.players[0].Conn.(*recordConn).buf; !bytes.Equal(got, want) {
				t.Fatalf("players[0] 收到 %v，期望仅一条 NEW_PHASE 广播 %v（安全跳过失准？）", got, want)
			}
		})
	}
}

// abortRecordingRoom 在 analyzeFakeRoom 之上记录 EndDuel 是否被调用。
type abortRecordingRoom struct {
	*analyzeFakeRoom
	aborted bool
}

func (f *abortRecordingRoom) EndDuel() { f.aborted = true }

// TestAnalyzeUnknownMessageAbortsDuel 守住显式降级的另一半：布局表也
// 不认识的消息（如 0xfe）无从得知消息体长度，必须记日志并 EndDuel 兜底
//（返回 2），禁止像旧实现那样只消费类型字节后静默错位继续广播。
func TestAnalyzeUnknownMessageAbortsDuel(t *testing.T) {
	fake := &abortRecordingRoom{analyzeFakeRoom: &analyzeFakeRoom{SingleDuel: &SingleDuel{}}}
	fake.players[0] = &DuelPlayer{Conn: stubConn{}}
	fake.players[1] = &DuelPlayer{Conn: stubConn{}}

	batch := []byte{0xfe, 0, 0, 0}
	if r := runAnalyze(fake, sharedAnalyzeHandlers, batch); r != 2 {
		t.Fatalf("runAnalyze 返回 %d，期望 2（EndDuel 兜底）", r)
	}
	if !fake.aborted {
		t.Fatal("未知消息未触发 EndDuel 兜底")
	}
}
