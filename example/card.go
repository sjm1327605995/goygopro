package main

const (
	SIZE_SETCODE = 16
	TYPE_LINK    = 0x4000000
)

type CardString struct {
	Name string   `db:"name"`
	Text string   `db:"text"`
	Desc []string `db:"desc"`
}

type CardData struct {
	Code       uint32 `db:"id"`
	OT         int    `db:"ot"`
	Alias      uint32 `db:"alias"`
	Setcode    [SIZE_SETCODE]uint16
	Type       uint32 `db:"type"`
	Attack     int32  `db:"atk"`
	Defense    int32  `db:"def"`
	Level      uint8
	LScale     uint8
	RScale     uint8
	Race       uint32 `db:"race"`
	Attribute  uint32 `db:"attribute"`
	Category   int64  `db:"category"`
	LinkMarker uint32
}

// SetSetCode 设置setcode数组
func (c *CardData) SetSetCode(value uint64) {
	ctr := 0
	for value != 0 {
		if value&0xffff != 0 {
			c.Setcode[ctr] = uint16(value & 0xffff)
			ctr++
		}
		value >>= 16
	}
	for i := ctr; i < SIZE_SETCODE; i++ {
		c.Setcode[i] = 0
	}
}
