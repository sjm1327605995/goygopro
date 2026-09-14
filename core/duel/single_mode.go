package duel

import (
	"errors"
	"fmt"

	"github.com/sjm1327605995/goygopro/ocgcore"
)

// ErrSingleStopped 表示玩家在解谜过程中退出了单人模式（原版 StopPlay）。
var ErrSingleStopped = errors.New("single mode stopped by user")

// SingleSession drives one puzzle duel entirely in-process: the engine runs on
// the caller's goroutine, every message batch is handed to a handler (the
// frontend parser), and whenever the batch ends with an interaction prompt the
// human's answer is read from a response channel — the Go equivalent of
// source/ygopro/gframe/single_mode.cpp (SinglePlayThread / SinglePlayAnalyze,
// singleSignal.Wait / SingleMode::SetResponse).
//
// Unlike the C++ client, prompts are NOT answered mid-batch: the engine only
// stops a Process() unit at a prompt, so the whole batch up to (and including)
// the select message is forwarded before waiting, exactly like the online
// server path and ReplayMode.
type SingleSession struct {
	Duel *ocgcore.Duel

	// OnPrompt（可选）在 Run 阻塞等待玩家作答前调用 —— 原版
	// SinglePlayAnalyze 每个 select 提示后调 SinglePlayRefresh 刷 mzone/
	// szone/hand 的时机（single_mode.cpp:788-843）。驱动方在这里做 query_field_card
	// → MSG_UPDATE_DATA 的场地同步；调用发生在引擎 goroutine 上，与 Run 同步。
	OnPrompt func()

	// Diagnostics, filled by Prepare/Run.
	ResponsesConsumed int
	Started           bool
	Completed         bool
}

// NewSingleSession creates the engine duel with a random seed sequence
// (SinglePlayThread 用 std::seed_seq 派生 duel_seed 后 create_duel_v2).
func NewSingleSession(seedSequence [8]uint32) *SingleSession {
	return &SingleSession{Duel: ocgcore.NewDuelV2(seedSequence)}
}

// Prepare loads the puzzle and starts the engine duel. scriptName is the file
// name inside ./single/ (the "./single/" prefix matches the C++
// preload_script call). SinglePlayThread 固定 8000 基本分 / 5 张起手 / 每回合
// 抽 1；谜题脚本里的 Debug.SetPlayerInfo 会在脚本加载后覆盖这些默认值，
// ReloadFieldBegin 的规则与选项（DUEL_ATTACK_FIRST_TURN 等）由引擎内部保存，
// start_duel 的 opt 保持 0（chkSinglePlayReturnDeckTop 未勾选）。
func (ss *SingleSession) Prepare(scriptName string) error {
	d := ss.Duel
	if d == nil {
		return fmt.Errorf("single session: no engine duel")
	}
	d.InitPlayers(8000, 5, 1)
	filename := "./single/" + scriptName
	if ocgcore.API.PreloadScript(d.GetNativePtr(), filename, int32(len(filename))) == 0 {
		return fmt.Errorf("preload script %s: failed", filename)
	}
	d.Start(0)
	ss.Started = true
	return nil
}

// DeckCount reports the number of cards in a player's main/extra deck after
// the puzzle script laid out the field (供 MSG_START 合成用，等价服务器的
// QueryFieldCount）。
func (ss *SingleSession) DeckCount(player int, location uint8) int {
	if ss.Duel == nil {
		return 0
	}
	return int(ss.Duel.QueryFieldCount(uint8(player), location))
}

// Run executes the puzzle. handler receives every engine message batch;
// respCh delivers the player's raw response bytes; closing stopCh aborts the
// duel. Returns ErrSingleStopped on abort.
func (ss *SingleSession) Run(handler func(msg []byte), respCh <-chan []byte, stopCh <-chan struct{}) error {
	d := ss.Duel
	if d == nil || !ss.Started {
		return fmt.Errorf("single session: duel not prepared")
	}
	buf := make([]byte, ocgcore.SIZE_MESSAGE_BUFFER)
	defer d.End() // 无论胜负或退出，都释放引擎决斗（end_duel + Dispose）
	for {
		// 协作式中止：引擎单元之间检查退出标记（原版 StopPlay 置
		// is_closing 后 Set 唤醒等待中的信号量）。
		select {
		case <-stopCh:
			return ErrSingleStopped
		default:
		}

		result := d.Process()
		engLen := int(result & ocgcore.PROCESSOR_BUFFER_LEN)
		engFlag := result & ocgcore.PROCESSOR_FLAG
		if engFlag == ocgcore.PROCESSOR_END {
			break
		}
		if engLen > 0 {
			n := int(d.GetMessage(buf))
			if handler != nil {
				handler(buf[:n])
			}
			// MSG_WIN：引擎的 adjust_step 自身不会终止对局（后续每个 adjust
			// 都会重发 MSG_WIN），由驱动方停止 —— 等价 C++ 客户端
			// SinglePlayAnalyze 对 MSG_WIN 返回 false 结束线程。
			respOffset, ok := batchResponseOffset(buf[:n])
			if ok && respOffset == batchWinOffset {
				break
			}
			// 批次以交互提示结尾（或无法解析流布局时引擎报告 WAITING）→
			// 等玩家作答。MSG_RETRY 批次同样落在这里：引擎驳回了上一响应，
			// 前端会重新弹出提示等待再次作答。
			if (ok && respOffset >= 0) || (!ok && engFlag == ocgcore.PROCESSOR_WAITING) {
				if ss.OnPrompt != nil {
					ss.OnPrompt()
				}
				var resp []byte
				select {
				case resp = <-respCh:
				case <-stopCh:
					return ErrSingleStopped
				}
				_ = d.SetResponseBytes(resp)
				ss.ResponsesConsumed++
			}
		}
		// engLen == 0 的空转单元（引擎内部时序步进）直接进入下一轮，
		// 与 C++ SinglePlayThread 的 while 循环一致。
	}
	ss.Completed = true
	return nil
}
