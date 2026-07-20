package client

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/ulikunitz/xz/lzma"
)

// 录像文件（.yrp）的只读侧：枚举、读出「谁跟谁、什么时候打的」用于列表展示。
//
// 完整的读写实现在 core/duel/replay.go（服务端要录制、也要能回放校验）。这里不 import 它 ——
// 那是服务端包，会把 gnet、ocgcore 之类一并拖进客户端。客户端只需要读头部和玩家名，
// 用不着 responses、卡组、写入那一整套。
//
// 两边解析同一种格式，会不会漂？replay_test.go 里有一条交叉测试：拿 core/duel 存出来的
// 文件让这边读，字段对不上就红。

// 与 core/duel/replay.go 保持一致的常量。数值由文件格式定死，不会变。
const (
	replayIDYRP1 = 0x31707279 // "yrp1"
	replayIDYRP2 = 0x32707279 // "yrp2"

	replayCompressed = 0x1
	replayTag        = 0x2
	replaySingleMode = 0x8
	replayUniform    = 0x10

	replayDir = "replay"

	// 头部布局：基础头 32 字节；YRP2 还带 seed_sequence[8] + header_version + value1..3。
	replayBaseHeaderSize = 32
	replayFullHeaderSize = 80
)

// ReplayInfo 是列表要显示的东西。
type ReplayInfo struct {
	Name      string    // 文件名（不含 .yrp）
	Path      string    // 完整路径
	StartTime time.Time // 对局开始时间
	Players   []string  // 参战玩家
	IsTag     bool      // 双打
	Single    bool      // 单人模式（跑本地脚本，没有卡组段）
}

// Title 是列表里显示的一行：「红方 vs 蓝方」。
func (r ReplayInfo) Title() string {
	switch len(r.Players) {
	case 0:
		return r.Name
	case 1:
		return r.Players[0]
	default:
		return strings.Join(r.Players[:len(r.Players)/2], "+") + " vs " +
			strings.Join(r.Players[len(r.Players)/2:], "+")
	}
}

// ListReplays 枚举 replay 目录下的录像，最近的排在前面。
// 读不动的文件跳过而不是整个失败 —— 目录里混进半截文件是常事。
func ListReplays() []ReplayInfo {
	entries, err := os.ReadDir(replayDir)
	if err != nil {
		return nil
	}
	var out []ReplayInfo
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".yrp") {
			continue
		}
		info, err := ReadReplayInfo(filepath.Join(replayDir, e.Name()))
		if err != nil {
			continue
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartTime.After(out[j].StartTime) })
	return out
}

// ReadReplayInfo 读出一个录像的头部与玩家名。
func ReadReplayInfo(path string) (ReplayInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ReplayInfo{}, err
	}
	if len(data) < replayBaseHeaderSize {
		return ReplayInfo{}, fmt.Errorf("文件太短，不是录像")
	}

	id := binary.LittleEndian.Uint32(data[0:4])
	if id != replayIDYRP1 && id != replayIDYRP2 {
		return ReplayInfo{}, fmt.Errorf("不是 yrp 录像")
	}
	flag := binary.LittleEndian.Uint32(data[8:12])
	seed := binary.LittleEndian.Uint32(data[12:16])
	dataSize := binary.LittleEndian.Uint32(data[16:20])
	startTime := binary.LittleEndian.Uint32(data[20:24])
	props := data[24:29]

	// REPLAY_UNIFORM 之前的版本没有独立的开始时间字段，那时 seed 就是时间戳。
	ts := startTime
	if flag&replayUniform == 0 {
		ts = seed
	}

	headerSize := replayBaseHeaderSize
	if id == replayIDYRP2 {
		headerSize = replayFullHeaderSize
	}
	if len(data) < headerSize {
		return ReplayInfo{}, fmt.Errorf("头部不完整")
	}

	body, err := replayBody(data[headerSize:], flag, props, dataSize)
	if err != nil {
		return ReplayInfo{}, err
	}

	info := ReplayInfo{
		Name:      strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		Path:      path,
		StartTime: time.Unix(int64(ts), 0),
		IsTag:     flag&replayTag != 0,
		Single:    flag&replaySingleMode != 0,
	}
	info.Players = readReplayNames(body, info.IsTag)
	return info, nil
}

// replayBody 取出数据段。压缩时按原版布局解：props 在头里，文件里的压缩段是纯压缩数据。
func replayBody(seg []byte, flag uint32, props []byte, dataSize uint32) ([]byte, error) {
	if flag&replayCompressed == 0 {
		return seg, nil
	}
	// ulikunitz 的 Reader 要「13 字节文件头 + 压缩数据」：前 5 字节是 props，
	// 后 8 字节是原始长度 —— 这里填进真实长度，解出来正好。
	var head [13]byte
	copy(head[0:5], props)
	binary.LittleEndian.PutUint64(head[5:13], uint64(dataSize))

	reader, err := lzma.NewReader(bytes.NewReader(append(head[:], seg...)))
	if err != nil {
		return nil, fmt.Errorf("解压失败: %w", err)
	}
	var out bytes.Buffer
	if _, err := out.ReadFrom(reader); err != nil {
		return nil, fmt.Errorf("解压出错: %w", err)
	}
	return out.Bytes(), nil
}

// readReplayNames 读数据段开头的玩家名。每个名字固定 20 个 uint16（UTF-16LE），
// 双打是 4 个人，否则 2 个。
func readReplayNames(body []byte, isTag bool) []string {
	count := 2
	if isTag {
		count = 4
	}
	const nameBytes = 40 // 20 * uint16
	if len(body) < count*nameBytes {
		return nil
	}
	names := make([]string, 0, count)
	for i := 0; i < count; i++ {
		seg := body[i*nameBytes : (i+1)*nameBytes]
		u16 := make([]uint16, 20)
		for j := range u16 {
			u16[j] = binary.LittleEndian.Uint16(seg[j*2:])
		}
		// 名字以 0 结尾，后面是填充
		end := len(u16)
		for j, v := range u16 {
			if v == 0 {
				end = j
				break
			}
		}
		names = append(names, string(utf16.Decode(u16[:end])))
	}
	return names
}

// SaveReplayFile 把服务器发来的录像存进 replay 目录。
//
// 文件名用录像自带的开始时间（对应 C++ duelclient.cpp 处理 STOC_REPLAY 时的
// "%Y-%m-%d %H-%M-%S"）—— 比进程时间戳可读，也和原版对得上。
// 此前是存到**当前目录**、名字是 Unix 时间戳，录像列表根本找不到它们。
func SaveReplayFile(payload []byte) (string, error) {
	if len(payload) < replayBaseHeaderSize {
		return "", fmt.Errorf("录像数据太短")
	}
	if err := os.MkdirAll(replayDir, 0755); err != nil {
		return "", err
	}

	flag := binary.LittleEndian.Uint32(payload[8:12])
	ts := binary.LittleEndian.Uint32(payload[20:24])
	if flag&replayUniform == 0 {
		ts = binary.LittleEndian.Uint32(payload[12:16])
	}
	name := time.Unix(int64(ts), 0).Format("2006-01-02 15-04-05")

	path := filepath.Join(replayDir, name+".yrp")
	if err := os.WriteFile(path, payload, 0644); err != nil {
		return "", err
	}
	return path, nil
}
