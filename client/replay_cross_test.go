package client

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"

	"github.com/sjm1327605995/goygopro/core/duel"
)

// 客户端只读一小段（头 + 玩家名），服务端有完整的读写实现。同一种格式解析两遍就会漂 ——
// 这条测试拿**服务端存出来的真文件**让客户端读，字段对不上立刻红。
//
// 之所以敢在测试里 import core/duel：测试的依赖不进生产二进制，
// 客户端本身仍然只依赖标准库和 lzma。
func TestClientReadsServerWrittenReplay(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)

	// 玩家名在录像里是固定 20 个 uint16（UTF-16LE），与服务端 single_duel.go 的写法一致
	nameBytes := func(s string) []byte {
		out := make([]byte, 40)
		for i, r := range utf16.Encode([]rune(s)) {
			if i >= 20 {
				break
			}
			binary.LittleEndian.PutUint16(out[i*2:], r)
		}
		return out
	}

	var rh duel.ExtendedReplayHeader
	rh.Base.ID = duel.REPLAY_ID_YRP2
	rh.Base.Version = 0x1360
	rh.Base.Flag = duel.REPLAY_UNIFORM
	rh.Base.StartTime = 1700000000

	r := duel.NewReplay()
	r.BeginRecord()
	r.WriteHeader(rh)
	r.WriteData(nameBytes("服务端红"), false)
	r.WriteData(nameBytes("服务端蓝"), false)
	r.WriteInt32(8000, false) // start_lp
	r.WriteInt32(5, false)    // start_hand
	r.WriteInt32(1, false)    // draw_count
	r.WriteInt32(0, false)    // duel_flag
	r.Flush()
	r.EndRecord()
	if !r.SaveReplay("cross") {
		t.Fatal("服务端保存录像失败")
	}

	// 客户端侧读同一个文件
	info, err := ReadReplayInfo(filepath.Join("replay", "cross.yrp"))
	if err != nil {
		t.Fatalf("客户端读不了服务端写的录像: %v", err)
	}
	if len(info.Players) != 2 ||
		info.Players[0] != "服务端红" || info.Players[1] != "服务端蓝" {
		t.Errorf("玩家 = %v, want [服务端红 服务端蓝] —— 两侧解析漂了", info.Players)
	}
	if info.StartTime.Unix() != 1700000000 {
		t.Errorf("开始时间 = %d, want 1700000000", info.StartTime.Unix())
	}
}
