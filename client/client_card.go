package client

import (
	"encoding/binary"
	"image"
)

// Command flags (correspond to C++ COMMAND_* in client_card.h)
const (
	CommandSummon    = 0x01
	CommandSPSummon  = 0x02
	CommandRepos     = 0x04
	CommandMSet      = 0x08
	CommandSSet      = 0x10
	CommandActivate  = 0x20
	CommandAttack    = 0x40
)

// EDESC flags for activatable_descs
const (
	EDESCOperation = 0x01
	EDESCReset     = 0x02
)

// ClientCard corresponds to C++ class ClientCard in client_card.h
// Represents a card on the client side with rendering and animation state.
type ClientCard struct {
	// Transform / animation (mapped from Irrlicht 3D to 2D UI)
	CurPos image.Point
	CurRot image.Point
	DPos   image.Point
	DRot   image.Point

	CurAlpha uint32
	DAlpha   uint32
	AniFrame uint32

	IsMoving          bool
	IsFading          bool
	IsHovered         bool
	IsSelectable      bool
	IsSelected        bool
	IsShowEquip       bool
	IsShowTarget      bool
	IsShowChainTarget bool
	IsHighlighting    bool
	IsReversed        bool

	Code      uint32
	ChainCode uint32
	Alias     uint32
	Type      uint32
	Level     uint32
	Rank      uint32
	Link      uint32
	Attribute uint32
	Race      uint32
	Attack    int
	Defense   int
	BaseAttack int
	BaseDefense int
	LScale     uint32
	RScale     uint32
	LinkMarker uint32
	Reason     uint32
	SelectSeq  uint32

	Owner      uint8
	Controler  uint8
	Location   uint8
	Sequence   uint8
	Position   uint8
	Status     uint32
	CHint      uint8
	ChValue    uint32
	OpParam    uint32
	Symbol     uint32
	CmdFlag    uint32

	OverlayTarget *ClientCard
	Overlayed     []*ClientCard
	EquipTarget   *ClientCard
	Equipped      map[*ClientCard]struct{}
	CardTarget    map[*ClientCard]struct{}
	OwnerTarget   map[*ClientCard]struct{}
	Counters      map[int]int
	DescHints     map[int]int

	AtkString  string
	DefString  string
	LvString   string
	LinkString string
	LscString  string
	RscString  string
}

func NewClientCard() *ClientCard {
	return &ClientCard{
		Equipped:    make(map[*ClientCard]struct{}),
		CardTarget:  make(map[*ClientCard]struct{}),
		OwnerTarget: make(map[*ClientCard]struct{}),
		Counters:    make(map[int]int),
		DescHints:   make(map[int]int),
	}
}

func (c *ClientCard) SetCode(x uint32) {
	c.Code = x
}

func (c *ClientCard) UpdateInfo(buf []byte) {
	if len(buf) < 4 {
		return
	}
	flag := binary.LittleEndian.Uint32(buf)
	buf = buf[4:]
	if flag&0x1 != 0 && len(buf) >= 4 { // QUERY_CODE
		c.Code = binary.LittleEndian.Uint32(buf)
		buf = buf[4:]
	}
	if flag&0x2 != 0 && len(buf) >= 4 { // QUERY_POSITION
		c.Position = buf[0]
		buf = buf[4:]
	}
	if flag&0x4 != 0 && len(buf) >= 4 { // QUERY_ALIAS
		c.Alias = binary.LittleEndian.Uint32(buf)
		buf = buf[4:]
	}
	if flag&0x8 != 0 && len(buf) >= 4 { // QUERY_TYPE
		c.Type = binary.LittleEndian.Uint32(buf)
		buf = buf[4:]
	}
	if flag&0x10 != 0 && len(buf) >= 4 { // QUERY_LEVEL
		c.Level = binary.LittleEndian.Uint32(buf)
		buf = buf[4:]
	}
	if flag&0x20 != 0 && len(buf) >= 4 { // QUERY_RANK
		c.Rank = binary.LittleEndian.Uint32(buf)
		buf = buf[4:]
	}
	if flag&0x40 != 0 && len(buf) >= 4 { // QUERY_ATTRIBUTE
		c.Attribute = binary.LittleEndian.Uint32(buf)
		buf = buf[4:]
	}
	if flag&0x80 != 0 && len(buf) >= 4 { // QUERY_RACE
		c.Race = binary.LittleEndian.Uint32(buf)
		buf = buf[4:]
	}
	if flag&0x100 != 0 && len(buf) >= 4 { // QUERY_ATTACK
		c.Attack = int(int32(binary.LittleEndian.Uint32(buf)))
		buf = buf[4:]
	}
	if flag&0x200 != 0 && len(buf) >= 4 { // QUERY_DEFENSE
		c.Defense = int(int32(binary.LittleEndian.Uint32(buf)))
		buf = buf[4:]
	}
	if flag&0x400 != 0 && len(buf) >= 4 { // QUERY_BASE_ATTACK
		c.BaseAttack = int(int32(binary.LittleEndian.Uint32(buf)))
		buf = buf[4:]
	}
	if flag&0x800 != 0 && len(buf) >= 4 { // QUERY_BASE_DEFENSE
		c.BaseDefense = int(int32(binary.LittleEndian.Uint32(buf)))
		buf = buf[4:]
	}
	if flag&0x1000 != 0 && len(buf) >= 4 { // QUERY_REASON
		c.Reason = binary.LittleEndian.Uint32(buf)
		buf = buf[4:]
	}
	// QUERY_REASON_CARD (0x2000) skip
	if flag&0x2000 != 0 && len(buf) >= 4 {
		buf = buf[4:]
	}
	// QUERY_EQUIP_CARD (0x4000) skip
	if flag&0x4000 != 0 && len(buf) >= 4 {
		buf = buf[4:]
	}
	// QUERY_TARGET_CARD (0x8000) skip
	if flag&0x8000 != 0 {
		if len(buf) >= 4 {
			count := int(binary.LittleEndian.Uint32(buf))
			buf = buf[4:]
			for i := 0; i < count && len(buf) >= 4; i++ {
				buf = buf[4:]
			}
		}
	}
	// QUERY_OVERLAY_CARD (0x10000) skip
	if flag&0x10000 != 0 {
		if len(buf) >= 4 {
			count := int(binary.LittleEndian.Uint32(buf))
			buf = buf[4:]
			for i := 0; i < count && len(buf) >= 4; i++ {
				buf = buf[4:]
			}
		}
	}
	// QUERY_COUNTERS (0x20000)
	if flag&0x20000 != 0 {
		if len(buf) >= 4 {
			count := int(binary.LittleEndian.Uint32(buf))
			buf = buf[4:]
			for i := 0; i < count && len(buf) >= 8; i++ {
				ctype := int(binary.LittleEndian.Uint16(buf))
				cval := int(binary.LittleEndian.Uint16(buf[2:]))
				buf = buf[8:]
				c.Counters[ctype] = cval
			}
		}
	}
	if flag&0x40000 != 0 && len(buf) >= 4 { // QUERY_OWNER
		c.Owner = uint8(binary.LittleEndian.Uint32(buf))
		buf = buf[4:]
	}
	if flag&0x80000 != 0 && len(buf) >= 4 { // QUERY_STATUS
		c.Status = binary.LittleEndian.Uint32(buf)
		buf = buf[4:]
	}
	if flag&0x100000 != 0 && len(buf) >= 4 { // QUERY_LSCALE
		c.LScale = binary.LittleEndian.Uint32(buf)
		buf = buf[4:]
	}
	if flag&0x200000 != 0 && len(buf) >= 4 { // QUERY_RSCALE
		c.RScale = binary.LittleEndian.Uint32(buf)
		buf = buf[4:]
	}
	if flag&0x400000 != 0 && len(buf) >= 4 { // QUERY_LINK
		c.Link = binary.LittleEndian.Uint32(buf)
		buf = buf[4:]
	}
}

func (c *ClientCard) ClearTarget() {
	c.CardTarget = make(map[*ClientCard]struct{})
	c.OwnerTarget = make(map[*ClientCard]struct{})
}

func (c *ClientCard) ClearData() {
	c.Type = 0
	c.Level = 0
	c.Rank = 0
	c.Link = 0
	c.Attribute = 0
	c.Race = 0
	c.Attack = 0
	c.Defense = 0
	c.BaseAttack = 0
	c.BaseDefense = 0
	c.LScale = 0
	c.RScale = 0
	c.LinkMarker = 0
	c.Status = 0
	c.Counters = make(map[int]int)
	c.DescHints = make(map[int]int)
	c.AtkString = ""
	c.DefString = ""
	c.LvString = ""
	c.LinkString = ""
	c.LscString = ""
	c.RscString = ""
}

func ClientCardSort(c1, c2 *ClientCard) bool {
	// C++ sorts by controler then location then sequence
	if c1.Controler != c2.Controler {
		return c1.Controler < c2.Controler
	}
	if c1.Location != c2.Location {
		return c1.Location < c2.Location
	}
	return c1.Sequence < c2.Sequence
}
