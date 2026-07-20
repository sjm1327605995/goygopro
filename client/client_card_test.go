package client

import (
	"encoding/binary"
	"testing"
)

// UpdateInfo 是决斗中卡片数据的唯一入口：攻守、表示形式、计数器、装备与指示关系
// 全靠它从引擎的查询结果里解出来。字段是按 flag 位依次紧挨着排的，
// **任何一处宽度读错，其后所有字段一起错位**，而且不会报错 —— 只会显示成一堆怪数值。

// queryBuilder 按引擎的格式拼一段查询结果。
type queryBuilder struct {
	flag uint32
	body []byte
}

func (b *queryBuilder) add(bit uint32, data ...byte) *queryBuilder {
	b.flag |= bit
	b.body = append(b.body, data...)
	return b
}

func (b *queryBuilder) u32(bit uint32, v uint32) *queryBuilder {
	var tmp [4]byte
	binary.LittleEndian.PutUint32(tmp[:], v)
	return b.add(bit, tmp[:]...)
}

func (b *queryBuilder) bytes() []byte {
	var head [4]byte
	binary.LittleEndian.PutUint32(head[:], b.flag)
	return append(head[:], b.body...)
}

const (
	qCode     = 0x1
	qPosition = 0x2
	qType     = 0x8
	qAttack   = 0x100
	qDefense  = 0x200
	qCounters = 0x20000
	qOwner    = 0x40000
	qStatus   = 0x80000
	qLink     = 0x400000
)

// 表示形式在引擎给的 4 字节里位于**最高**字节：
// get_info_location() = controler | location<<8 | sequence<<16 | position<<24。
// 曾经读的是最低字节（controler，只有 0/1），于是守备与盖伏一律显示成表侧攻击。
func TestUpdateInfoReadsPositionFromHighByte(t *testing.T) {
	const (
		controler = 1
		location  = 0x04
		sequence  = 3
		position  = 0x08 // 里侧守备
	)
	infoLoc := uint32(controler) | uint32(location)<<8 | uint32(sequence)<<16 | uint32(position)<<24

	c := NewClientCard()
	b := (&queryBuilder{}).u32(qPosition, infoLoc)
	c.UpdateInfo(b.bytes())

	if c.Position != position {
		t.Errorf("Position = %#x, want %#x —— 读错字节了（%#x 是 controler）", c.Position, position, controler)
	}
}

// 每条计数器是 uint16 类型 + uint16 数量 = 4 字节。曾经按 8 字节读，多吃一倍，
// 其后的 OWNER/STATUS/LSCALE/RSCALE/LINK 全部错位 —— 这个测试正是冲着错位来的：
// 计数器后面还跟着两个字段，它们的值必须完好。
func TestUpdateInfoCountersDoNotMisalignFollowingFields(t *testing.T) {
	c := NewClientCard()
	b := &queryBuilder{}
	// 两条计数器：类型 5 数量 2，类型 9 数量 1
	b.flag |= qCounters
	var cnt [4]byte
	binary.LittleEndian.PutUint32(cnt[:], 2)
	b.body = append(b.body, cnt[:]...)
	b.body = append(b.body, 5, 0, 2, 0) // ctype=5 ccount=2
	b.body = append(b.body, 9, 0, 1, 0) // ctype=9 ccount=1
	// 紧跟着两个字段，用来暴露错位
	b.u32(qOwner, 1)
	b.u32(qStatus, 0xABCD)

	c.UpdateInfo(b.bytes())

	if c.Counters[5] != 2 || c.Counters[9] != 1 {
		t.Errorf("计数器 = %v, want {5:2, 9:1}", c.Counters)
	}
	if c.Owner != 1 {
		t.Errorf("Owner = %d, want 1 —— 计数器之后错位了", c.Owner)
	}
	if c.Status != 0xABCD {
		t.Errorf("Status = %#x, want 0xABCD —— 计数器之后错位了", c.Status)
	}
}

// LINK 后面紧跟着连接标记（箭头方向），此前没读，LINK 怪的箭头全丢。
func TestUpdateInfoReadsLinkMarker(t *testing.T) {
	c := NewClientCard()
	b := (&queryBuilder{}).u32(qLink, 3)
	var marker [4]byte
	binary.LittleEndian.PutUint32(marker[:], 0x144) // 左下+右下之类的箭头组合
	b.body = append(b.body, marker[:]...)

	c.UpdateInfo(b.bytes())

	if c.Link != 3 {
		t.Errorf("Link = %d, want 3", c.Link)
	}
	if c.LinkMarker != 0x144 {
		t.Errorf("LinkMarker = %#x, want 0x144 —— 连接箭头没有被读出来", c.LinkMarker)
	}
}

// 常规字段的往返：攻守是有符号的，负值表示「？」，不能读成巨大的无符号数。
func TestUpdateInfoBasicFields(t *testing.T) {
	c := NewClientCard()
	b := (&queryBuilder{}).
		u32(qCode, 12345).
		u32(qType, 0x21).
		u32(qAttack, 2500).
		u32(qDefense, uint32(0xfffffffe)) // -2，即「?」守

	c.UpdateInfo(b.bytes())

	if c.Code != 12345 {
		t.Errorf("Code = %d, want 12345", c.Code)
	}
	if c.Type != 0x21 {
		t.Errorf("Type = %#x, want 0x21", c.Type)
	}
	if c.Attack != 2500 {
		t.Errorf("Attack = %d, want 2500", c.Attack)
	}
	if c.Defense != -2 {
		t.Errorf("Defense = %d, want -2（负值是「?」，不能当无符号读）", c.Defense)
	}
}

// flag == 0 表示这张卡的数据被清空。
func TestUpdateInfoZeroFlagClearsData(t *testing.T) {
	c := NewClientCard()
	c.Attack, c.Type = 3000, 0x21

	c.UpdateInfo(make([]byte, 4)) // flag = 0

	if c.Attack != 0 || c.Type != 0 {
		t.Errorf("flag=0 应当清空数据，现在 Attack=%d Type=%#x", c.Attack, c.Type)
	}
}

// 截断的包不能让客户端崩掉。
func TestUpdateInfoHandlesTruncatedBuffers(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("截断的包导致 panic: %v", r)
		}
	}()
	full := (&queryBuilder{}).u32(qCode, 1).u32(qAttack, 100).bytes()
	for i := 0; i <= len(full); i++ {
		NewClientCard().UpdateInfo(full[:i])
	}
}
