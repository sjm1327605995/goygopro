package main

import (
	"encoding/binary"
	"testing"
)

// 构造一个 query blob 体：flag + 按位序追加字段。
func buildQueryBody(flag uint32, fields ...uint32) []byte {
	body := binary.LittleEndian.AppendUint32(nil, flag)
	for _, f := range fields {
		body = binary.LittleEndian.AppendUint32(body, f)
	}
	return body
}

// TestDecodeQueryBodyFlagBits：flag 驱动的条件字段——每个 bit 独立验证
// 置位/不置位时的键集，迁移到 core/codec 后锁定行为。
func TestDecodeQueryBodyFlagBits(t *testing.T) {
	// bit0 CODE：置位解码出 Code，不置位为 nil
	card := decodeQueryBody(buildQueryBody(0x01, 89631139))
	if card == nil || card.Code == nil || *card.Code != 89631139 {
		t.Fatalf("CODE 置位解码错: %+v", card)
	}
	card = decodeQueryBody(buildQueryBody(0x00))
	if card == nil {
		t.Fatal("flag=0 应得到空 DTO 而非 nil")
	}
	if card.Code != nil || card.Position != nil || card.Targets != nil {
		t.Fatalf("flag=0 应全部字段 nil: %+v", card)
	}
	// bit1 POSITION：打包 info_location 展开成 {c,l,s,p}
	// (c=1, l=4, s=2, p=1) 打包：c | l<<8 | s<<16 | p<<24
	card = decodeQueryBody(buildQueryBody(0x02, 1|4<<8|2<<16|1<<24))
	if card == nil || card.Position == nil {
		t.Fatalf("POSITION 置位应解码: %+v", card)
	}
	if card.Position.C != 1 || card.Position.L != 4 || card.Position.S != 2 || card.Position.P != 1 {
		t.Fatalf("Position 展开错: %+v", *card.Position)
	}
	// bit8/9 ATTACK/DEFENSE 是 int32（负数可往返）
	card = decodeQueryBody(buildQueryBody(0x300, 0xffffff9c, 0x96)) // -100, 150
	if card == nil || card.Attack == nil || *card.Attack != -100 || card.Defense == nil || *card.Defense != 150 {
		t.Fatalf("ATTACK/DEFENSE 解码错: %+v", card)
	}
}

// TestDecodeQueryBodyLists：计数列表（TARGET/OVERLAY/COUNTERS）与 LINK 双子段。
func TestDecodeQueryBodyLists(t *testing.T) {
	// TARGET(0x8000)：count + 两个打包 info_location
	card := decodeQueryBody(buildQueryBody(0x8000, 2, 1|4<<8|0<<16|1<<24, 0|4<<8|1<<16|2<<24))
	if card == nil || card.Targets == nil || len(card.Targets) != 2 {
		t.Fatalf("TARGET 解码错: %+v", card)
	}
	if card.Targets[0].S != 0 || card.Targets[1].S != 1 {
		t.Fatalf("Targets 展开错: %+v", card.Targets)
	}
	// OVERLAY(0x10000)：count + 卡号
	card = decodeQueryBody(buildQueryBody(0x10000, 2, 111, 222))
	if card == nil || card.Overlays == nil || len(card.Overlays) != 2 || card.Overlays[1] != 222 {
		t.Fatalf("OVERLAY 解码错: %+v", card)
	}
	// COUNTERS(0x20000)：每条 type | count<<16
	card = decodeQueryBody(buildQueryBody(0x20000, 1, 0x00030007))
	if card == nil || card.Counters == nil || len(card.Counters) != 1 ||
		card.Counters[0].Type != 7 || card.Counters[0].Count != 3 {
		t.Fatalf("COUNTERS 解码错: %+v", card)
	}
	// LINK(0x800000)：link + link_marker 两个 u32
	card = decodeQueryBody(buildQueryBody(0x800000, 5, 0xbf))
	if card == nil || card.Link == nil || *card.Link != 5 || card.LinkMarker == nil || *card.LinkMarker != 0xbf {
		t.Fatalf("LINK 解码错: %+v", card)
	}
	// LINK 只出一个半段（截断）→ 整体放弃
	if got := decodeQueryBody(buildQueryBody(0x800000, 5)); got != nil {
		t.Fatalf("LINK 截断应返回 nil，得到 %+v", got)
	}
}

// TestDecodeQueryBodyTruncation：截断/越界返回 nil（走 opaque skip），
// len<4 拒绝；负长度计数同样放弃而不是 panic。
func TestDecodeQueryBodyTruncation(t *testing.T) {
	if decodeQueryBody(nil) != nil {
		t.Fatal("空 body 应返回 nil")
	}
	if decodeQueryBody([]byte{1, 2}) != nil {
		t.Fatal("len<4 应返回 nil")
	}
	// flag 置位 CODE 但缺字段
	if got := decodeQueryBody(buildQueryBody(0x01)); got != nil {
		t.Fatalf("字段缺失应返回 nil，得到 %+v", got)
	}
	// TARGET count=3 但只有 2 条
	body := buildQueryBody(0x8000, 3, 1, 2)
	if got := decodeQueryBody(body); got != nil {
		t.Fatalf("列表截断应返回 nil，得到 %+v", got)
	}
	// 负长度计数（0xffffffff）：旧实现会 make 负容量 panic，现在应放弃
	body = buildQueryBody(0x10000, 0xffffffff)
	if got := decodeQueryBody(body); got != nil {
		t.Fatalf("负长度应返回 nil，得到 %+v", got)
	}
	// blob 尾部多带垃圾字节：不影响（LEN_HEADER 推进由 decodeQueryBlobList 保证）
	card := decodeQueryBody(append(buildQueryBody(0x01, 7), 0xde, 0xad))
	if card == nil || card.Code == nil || *card.Code != 7 {
		t.Fatalf("尾部垃圾不应影响解码: %+v", card)
	}
}
