package duel

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
	"unicode/utf16"

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

	// HostName 写进回放头的玩家昵称（原版 dInfo.hostname = ebNickName）。
	// 驱动方在 Prepare 前设置；为空则回放头里该字段恒为零。
	HostName string

	// Diagnostics, filled by Prepare/Run.
	ResponsesConsumed int
	Started           bool
	Completed         bool

	// 回放录制的内部状态（原版 SingleMode::last_replay + SinglePlayThread 的
	// rh.seed_sequence / opt / open_file_name）。
	seedSequence [8]uint32
	opt          int32
	scriptFile   string
	lastReplay   *Replay
}

// NewSingleSession creates the engine duel with a random seed sequence
// (SinglePlayThread 用 std::seed_seq 派生 duel_seed 后 create_duel_v2).
func NewSingleSession(seedSequence [8]uint32) *SingleSession {
	return &SingleSession{
		Duel:         ocgcore.NewDuelV2(seedSequence),
		seedSequence: seedSequence,
	}
}

// Prepare loads the puzzle and starts the engine duel. scriptName is the file
// name inside ./single/ (the "./single/" prefix matches the C++
// preload_script call). SinglePlayThread 固定 8000 基本分 / 5 张起手 / 每回合
// 抽 1；谜题脚本里的 Debug.SetPlayerInfo 会在脚本加载后覆盖这些默认值，
// ReloadFieldBegin 的规则与选项（DUEL_ATTACK_FIRST_TURN 等）由引擎内部保存。
func (ss *SingleSession) Prepare(scriptName string) error {
	return ss.PrepareWithOpt(scriptName, 0)
}

// PrepareWithOpt 同 Prepare，但把额外选项并进 start_duel 的 opt（原版
// SinglePlayThread：勾选 chkSinglePlayReturnDeckTop 时 opt |= DUEL_RETURN_DECK_TOP，
// 「不洗切时回卡组改为回顶端」）。
func (ss *SingleSession) PrepareWithOpt(scriptName string, opt int32) error {
	d := ss.Duel
	if d == nil {
		return fmt.Errorf("single session: no engine duel")
	}
	d.InitPlayers(8000, 5, 1)
	filename := "./single/" + scriptName
	if ocgcore.API.PreloadScript(d.GetNativePtr(), filename, int32(len(filename))) == 0 {
		return fmt.Errorf("preload script %s: failed", filename)
	}
	ss.opt = opt
	ss.scriptFile = filename
	ss.beginRecord()
	d.Start(opt)
	ss.Started = true
	return nil
}

// beginRecord 建立回放文件并写头部（原版 single_mode.cpp:114-128）：
// BeginRecord → WriteHeader(YRP2, REPLAY_UNIFORM|REPLAY_SINGLE_MODE, seed_sequence)
// → host_name / client_name 各 40 字节 → start_lp/start_hand/draw_count/opt
// → uint16 长度前缀 + 脚本文件名字节。文件布局与 core/duel/replay.go 的
// ReadInfo 中 REPLAY_SINGLE_MODE 分支一一对应，SaveReplay 时还能原样读回。
//
// 注意：原版 `ExtendedReplayHeader rh;` 只显式填 id/version/flag/start_time/
// seed_sequence，其余字段（seed/datasize/props/header_version/value*）是未初始化
// 栈垃圾；Go 侧统一置零，写出更干净的头部（这些字段在读取端本就被忽略）。
func (ss *SingleSession) beginRecord() {
	rh := ExtendedReplayHeader{}
	rh.Base.ID = REPLAY_ID_YRP2
	rh.Base.Version = PRO_VERSION
	rh.Base.Flag = REPLAY_UNIFORM | REPLAY_SINGLE_MODE
	rh.SeedSequence = ss.seedSequence
	rh.Base.StartTime = uint32(time.Now().Unix())

	ss.lastReplay = NewReplay()
	ss.lastReplay.BeginRecord()
	ss.lastReplay.WriteHeader(rh)
	ss.lastReplay.WriteData(encodeReplayName(ss.HostName), false)
	ss.lastReplay.WriteData(make([]byte, 40), false) // client_name 恒为空
	ss.lastReplay.WriteInt32(8000, false)            // start_lp
	ss.lastReplay.WriteInt32(5, false)               // start_hand
	ss.lastReplay.WriteInt32(1, false)               // draw_count
	ss.lastReplay.WriteInt32(ss.opt, false)          // DUEL_RETURN_DECK_TOP 等
	var lenBuf [2]byte
	binary.LittleEndian.PutUint16(lenBuf[:], uint16(len(ss.scriptFile)))
	ss.lastReplay.WriteData(lenBuf[:], false)
	ss.lastReplay.WriteData([]byte(ss.scriptFile), false)
	ss.lastReplay.Flush()
}

// encodeReplayName 把字符串昵称编成回放头里的 40 字节（20×uint16 UTF-16LE，
// 超出 20 字截断、不足补零），等价 BufferIO::CopyCharArray。
func encodeReplayName(name string) []byte {
	buf := make([]byte, 40)
	runes := utf16.Encode([]rune(name))
	for i := 0; i < 20 && i < len(runes); i++ {
		binary.LittleEndian.PutUint16(buf[i*2:], runes[i])
	}
	return buf
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

// recordResponse 把玩家的一次响应写进回放（原版 SingleMode::SetResponse：
// 先写 uint8 长度前缀再写响应体，replay.cpp ReadNextResponse 反读的同一格式）。
func (ss *SingleSession) recordResponse(resp []byte) {
	if ss.lastReplay == nil {
		return
	}
	ss.lastReplay.WriteData([]byte{uint8(len(resp))}, false)
	ss.lastReplay.WriteData(resp, false)
}

// endRecord 收尾回放（LZMA 压缩 + 关闭 _LastReplay.yrp）。Run 的正常完结与
// 中途退出路径都经 defer 调用；未开始录制（Prepare 失败）时 lastReplay 为 nil。
func (ss *SingleSession) endRecord() {
	if ss.lastReplay != nil {
		ss.lastReplay.EndRecord()
	}
}

// SaveReplay 落盘已完结的单机录像（原版 end_duel 前 last_replay.SaveReplay）。
// 仅在 Run 正常返回（Completed）后有完整录像；中途退出只 EndRecord 不保存。
func (ss *SingleSession) SaveReplay(baseName string) bool {
	if ss.lastReplay == nil {
		return false
	}
	return ss.lastReplay.SaveReplay(baseName)
}

// Replay 暴露已录制的回放句柄（供驱动方在会话结束后保存，等价
// SingleMode::last_replay）。
func (ss *SingleSession) Replay() *Replay { return ss.lastReplay }

// Run executes the puzzle. handler receives every engine message batch;
// respCh delivers the player's raw response bytes; closing stopCh aborts the
// duel. Returns ErrSingleStopped on abort.
func (ss *SingleSession) Run(handler func(msg []byte), respCh <-chan []byte, stopCh <-chan struct{}) error {
	d := ss.Duel
	if d == nil || !ss.Started {
		return fmt.Errorf("single session: duel not prepared")
	}
	buf := make([]byte, ocgcore.SIZE_MESSAGE_BUFFER)
	defer d.End()        // 无论胜负或退出，都释放引擎决斗（end_duel + Dispose）
	defer ss.endRecord() // 无论正常完结还是中途退出都收尾回放（原版 line 140）
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
				ss.recordResponse(resp)
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
