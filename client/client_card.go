package client

import (
	"encoding/binary"
	"sync/atomic"
)

// Command flags (correspond to C++ COMMAND_* in client_card.h)
const (
	CommandSummon   = 0x01
	CommandSPSummon = 0x02
	CommandRepos    = 0x04
	CommandMSet     = 0x08
	CommandSSet     = 0x10
	CommandActivate = 0x20
	CommandAttack   = 0x40
)

// EDESC flags for activatable_descs
const (
	EDESCOperation = 0x01
	EDESCReset     = 0x02
)

// cardUID 给每张卡发一个进程内唯一的号。
//
// 卡在区域之间移动时用的自始至终是同一个 ClientCard 对象（见 event_handler 的 MSG_MOVE：
// 从旧位置 GetCard 出来再 AddCard 回新位置），但它的 Controler/Location/Sequence 都会变，
// 没法拿来当身份。UI 需要一个跟着卡走的稳定标识，才能认出「这还是刚才那张卡，它移动了」
// 并播放移动动画，而不是当成旧的消失、新的出现。
var cardUID atomic.Uint64

// ClientCard corresponds to C++ class ClientCard in client_card.h
// Represents a card on the client side with rendering and animation state.
type ClientCard struct {
	// UID 是这张卡的稳定标识，创建时分配，之后不变。见 cardUID。
	UID uint64

	// 这里曾有一整套逐帧动画状态（CurPosX/TargetX/DPosX/AniFrame/CurAlpha/IsMoving…），
	// 是 C++ ClientCard 的直译。GUI 层换成 tenon 后由它负责动画：位置变化用 FLIP
	// （ui.Animated）自动补间，悬停/浮起用 UseTween，都不需要在模型里存插值中间量。
	// 两套动画并存只会互相打架，故删除。卡摆在哪由 GetCardLocation 现算。

	IsSelectable      bool
	IsSelected        bool
	IsShowEquip       bool
	IsShowTarget      bool
	IsShowChainTarget bool
	IsReversed        bool

	Code        uint32
	ChainCode   uint32
	Alias       uint32
	Type        uint32
	Level       uint32
	Rank        uint32
	Link        uint32
	Attribute   uint32
	Race        uint32
	Attack      int
	Defense     int
	BaseAttack  int
	BaseDefense int
	LScale      uint32
	RScale      uint32
	LinkMarker  uint32
	Reason      uint32
	SelectSeq   uint32

	Owner     uint8
	Controler uint8
	Location  uint8
	Sequence  uint8
	Position  uint8
	Status    uint32
	CHint     uint8
	ChValue   uint32
	OpParam   uint32
	Symbol    uint32
	CmdFlag   uint32

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
		UID:         cardUID.Add(1),
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
	// flag == 0 表示这张卡已经没有可见数据了（离场、被翻回背面等），要清空而不是保留旧值。
	// 漏掉这一步的话，卡面会一直停在上一次看到的攻守与类型上。
	if flag == 0 {
		c.ClearData()
		return
	}
	if flag&0x1 != 0 && len(buf) >= 4 { // QUERY_CODE
		code := binary.LittleEndian.Uint32(buf)
		// C++ 在 code 为 0 时先 ClearData 再 SetCode —— 卡号没了，旧的数值也不该留着。
		if code == 0 {
			c.ClearData()
		}
		c.SetCode(code)
		buf = buf[4:]
	}
	if flag&0x2 != 0 && len(buf) >= 4 { // QUERY_POSITION
		// 引擎发的是 get_info_location()：controler | location<<8 | sequence<<16 | position<<24，
		// 表示形式在**最高**字节。此前读的是 buf[0]（controler，只有 0/1），
		// 于是所有卡的表示形式都是错的 —— 守备与盖伏一律显示成表侧攻击。
		c.Position = buf[3]
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
	// QUERY_EQUIP_CARD (0x4000)：装备关系。此前只跳过字节，于是装备卡与被装备卡
	// 互相不认识，界面上的「装」标记永远不出现。
	if flag&0x4000 != 0 && len(buf) >= 4 {
		ec, el, es := MainGame.LocalPlayer(int(buf[0])), buf[1], int(buf[2])
		buf = buf[4:]
		if ecard := MainGame.DField.GetCard(ec, el, es); ecard != nil {
			c.EquipTarget = ecard
			ecard.Equipped[c] = struct{}{}
		}
	}
	// QUERY_TARGET_CARD (0x8000)：这张卡指示了谁。同样此前只跳字节，
	// 「标」标记因此永远不出现。
	if flag&0x8000 != 0 && len(buf) >= 4 {
		count := int(binary.LittleEndian.Uint32(buf))
		buf = buf[4:]
		for i := 0; i < count && len(buf) >= 4; i++ {
			tc, tl, ts := MainGame.LocalPlayer(int(buf[0])), buf[1], int(buf[2])
			buf = buf[4:]
			if tcard := MainGame.DField.GetCard(tc, tl, ts); tcard != nil {
				c.CardTarget[tcard] = struct{}{}
				tcard.OwnerTarget[c] = struct{}{}
			}
		}
	}
	// QUERY_OVERLAY_CARD (0x10000)：超量素材的卡号。此前跳过，素材翻开后仍是背面。
	if flag&0x10000 != 0 && len(buf) >= 4 {
		count := int(binary.LittleEndian.Uint32(buf))
		buf = buf[4:]
		for i := 0; i < count && len(buf) >= 4; i++ {
			code := binary.LittleEndian.Uint32(buf)
			buf = buf[4:]
			if i < len(c.Overlayed) && c.Overlayed[i] != nil {
				c.Overlayed[i].SetCode(code)
			}
		}
	}
	// QUERY_COUNTERS (0x20000)
	if flag&0x20000 != 0 {
		if len(buf) >= 4 {
			count := int(binary.LittleEndian.Uint32(buf))
			buf = buf[4:]
			// 每条计数器是 uint16 类型 + uint16 数量 = 4 字节。
			// 此前按 8 字节读，多吃一倍，其后的 OWNER/STATUS/LSCALE/RSCALE/LINK 全部错位。
			for i := 0; i < count && len(buf) >= 4; i++ {
				ctype := int(binary.LittleEndian.Uint16(buf))
				cval := int(binary.LittleEndian.Uint16(buf[2:]))
				buf = buf[4:]
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
		// 连接标记（箭头方向）紧随其后，此前没读，LINK 怪的箭头全丢。
		if len(buf) >= 4 {
			c.LinkMarker = binary.LittleEndian.Uint32(buf)
			buf = buf[4:]
		}
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
	if c1.Controler != c2.Controler {
		return c1.Controler < c2.Controler
	}
	if c1.Location != c2.Location {
		return c1.Location < c2.Location
	}
	return c1.Sequence < c2.Sequence
}

// CardRotationName returns a human-readable rotation description.
func (c *ClientCard) CardRotationName() string {
	if c.Location == 0x04 { // MZONE
		if c.Position&0x04 != 0 {
			return "DEF"
		}
		return "ATK"
	}
	return ""
}

// IsFaceDown reports whether the card is face-down.
func (c *ClientCard) IsFaceDown() bool {
	return c.Position&0x08 != 0 || c.Position&0x02 != 0
}
