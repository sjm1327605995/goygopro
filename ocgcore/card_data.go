package ocgcore

const (
	CARD_ARTWORK_VERSIONS_OFFSET = 20
	SIZE_SETCODE                 = 16
	CARD_BLACK_LUSTER_SOLDIER2   = 5405695
	CARD_MARINE_DOLPHIN          = 78734254
	CARD_TWINKLE_MOSS            = 13857930
	CARD_TIMAEUS                 = 1784686
	CARD_CRITIAS                 = 11082056
	CARD_HERMOS                  = 46232525
)

var secondCode = map[uint32]uint32{
	CARD_MARINE_DOLPHIN: 17955766,
	CARD_TWINKLE_MOSS:   17732278,
	CARD_TIMAEUS:        10000050,
	CARD_CRITIAS:        10000060,
	CARD_HERMOS:         10000070,
}

// CardData 表示卡片的核心数据
type CardData struct {
	Code       uint32 `db:"id"`
	Alias      uint32
	Setcode    [SIZE_SETCODE]uint16
	Type       uint32 `db:"type"`
	Level      uint32
	Attribute  uint32 `db:"attribute"`
	Race       uint32 `db:"race"`
	Attack     int32  `db:"attack"`
	Defense    int32  `db:"defense"`
	LScale     uint32
	RScale     uint32
	LinkMarker uint32
}

// Clear 初始化所有字段
func (c *CardData) Clear() {
	c.Code = 0
	c.Alias = 0
	for i := range c.Setcode {
		c.Setcode[i] = 0
	}
	c.Type = 0
	c.Level = 0
	c.Attribute = 0
	c.Race = 0
	c.Attack = 0
	c.Defense = 0
	c.LScale = 0
	c.RScale = 0
	c.LinkMarker = 0
}

// IsSetCode 检查给定的value是否匹配卡片的setcode
func (c *CardData) IsSetCode(value uint32) bool {
	setType := uint16(value & 0x0fff)
	setSubtype := uint16(value & 0xf000)

	for _, x := range c.Setcode {
		if x == 0 {
			return false
		}
		if (x&0x0fff) == setType && (x&0xf000&setSubtype) == setSubtype {
			return true
		}
	}
	return false
}

// IsAlternative 检查是否为替代卡
func (c *CardData) IsAlternative() bool {
	if c.Code == CARD_BLACK_LUSTER_SOLDIER2 {
		return false
	}
	return c.Alias != 0 &&
		(c.Alias < c.Code+CARD_ARTWORK_VERSIONS_OFFSET) &&
		(c.Code < c.Alias+CARD_ARTWORK_VERSIONS_OFFSET)
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

// GetOriginalCode 获取原始卡号
func (c *CardData) GetOriginalCode() uint32 {
	if c.IsAlternative() {
		return c.Alias
	}
	return c.Code
}
