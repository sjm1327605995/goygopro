package main

import (
	"bytes"
	"encoding/json"

	"github.com/sjm1327605995/goygopro/protocol"
)

// ------------------------------------------------------------------
// 前端事件 DTO
//
// engine_bindings.go / stoc_bindings.go 的 decorate/handle 原来手拼
// map[string]interface{}，这里改成带 json tag 的类型化 struct。键名与
// 历史手拼 map 逐字一致（前端 frontend/src 按这些键消费，是硬契约）；
// 布尔化（如 Cancelable != 0）在构造 DTO 时完成。形状与 protocol 消息
// struct 完全一致的事件（如 duel:start ↔ protocol.StartMsg）不经 decorate
// 直接 emit 该 struct（engineBindings/stocBindings 表里的无 decorate 项），
// 不在此重复包一层。
//
// 字段类型刻意与历史 map 里存入的 Go 类型一致：uint8/int32/uint32 等经
// JSON 序列化都是 number；bool 字段对应历史上的 != 0 布尔化。
// ------------------------------------------------------------------

// locRefDTO 打包 info_location 展开后的 {c,l,s}（不含 pos）。
type locRefDTO struct {
	C uint8 `json:"c"`
	L uint8 `json:"l"`
	S uint8 `json:"s"`
}

// locPosRefDTO 打包 info_location 展开后的 {c,l,s,p}。
type locPosRefDTO struct {
	C uint8 `json:"c"`
	L uint8 `json:"l"`
	S uint8 `json:"s"`
	P uint8 `json:"p"`
}

func newLocRef(loc uint32) locRefDTO {
	c, l, s, _ := protocol.UnpackPackedLoc(loc)
	return locRefDTO{C: c, L: l, S: s}
}

func newLocPosRef(loc uint32) locPosRefDTO {
	c, l, s, p := protocol.UnpackPackedLoc(loc)
	return locPosRefDTO{C: c, L: l, S: s, P: p}
}

func newLocRefList(locs []uint32) []locRefDTO {
	out := make([]locRefDTO, len(locs))
	for i, loc := range locs {
		out[i] = newLocRef(loc)
	}
	return out
}

// ---- duel:select_card / duel:select_unselect ----

type selectCardEntryDTO struct {
	Code int32 `json:"code"`
	C    uint8 `json:"c"`
	L    uint8 `json:"l"`
	S    uint8 `json:"s"`
	P    uint8 `json:"p"`
}

type selectUnselectEntryDTO struct {
	Code int32 `json:"code"`
	C    uint8 `json:"c"`
	L    uint8 `json:"l"`
	S    uint8 `json:"s"`
}

type selectCardDTO struct {
	Player     uint8                `json:"player"`
	Cancelable bool                 `json:"cancelable"`
	Min        uint8                `json:"min"`
	Max        uint8                `json:"max"`
	Cards      []selectCardEntryDTO `json:"cards"`
	// Tribute 标记 MSG_SELECT_TRIBUTE 来源：前端祭品选择保持弹窗路径
	// （原版 CheckSelectTribute 求和校验），不并入场上点选选择态
	Tribute bool `json:"tribute,omitempty"`
}

type selectUnselectDTO struct {
	Player       uint8                    `json:"player"`
	Finishable   bool                     `json:"finishable"`
	Cancelable   bool                     `json:"cancelable"`
	Min          uint8                    `json:"min"`
	Max          uint8                    `json:"max"`
	Cards        []selectUnselectEntryDTO `json:"cards"`
	UnselectList []selectUnselectEntryDTO `json:"unselectList"`
}

// ---- duel:select_chain ----

type chainEntryDTO struct {
	Flag   uint32 `json:"flag"`
	Forced bool   `json:"forced"`
	Code   uint32 `json:"code"`
	CC     uint8  `json:"cc"`
	CL     uint8  `json:"cl"`
	CS     uint8  `json:"cs"`
	CP     uint8  `json:"cp"`
	Desc   uint32 `json:"desc"`
}

type selectChainDTO struct {
	Player uint8           `json:"player"`
	Count  uint8           `json:"count"`
	Forced bool            `json:"forced"`
	Chains []chainEntryDTO `json:"chains"`
}

// ---- duel:select_idlecmd / duel:select_battlecmd 条目 ----

type cmdCardEntryDTO struct {
	Code int32 `json:"code"`
	C    uint8 `json:"c"`
	L    uint8 `json:"l"`
	S    uint8 `json:"s"`
	Idx  int   `json:"idx"`
}

type cmdActivateEntryDTO struct {
	Code int32  `json:"code"`
	C    uint8  `json:"c"`
	L    uint8  `json:"l"`
	S    uint8  `json:"s"`
	Desc uint32 `json:"desc"`
	Idx  int    `json:"idx"`
}

type cmdAttackEntryDTO struct {
	Code   int32 `json:"code"`
	C      uint8 `json:"c"`
	L      uint8 `json:"l"`
	S      uint8 `json:"s"`
	DirAtt bool  `json:"diratt"`
	Idx    int   `json:"idx"`
}

type selectIdleCmdDTO struct {
	Player   uint8                 `json:"player"`
	Summon   []cmdCardEntryDTO     `json:"summon"`
	SPSummon []cmdCardEntryDTO     `json:"spsummon"`
	Repos    []cmdCardEntryDTO     `json:"repos"`
	MSet     []cmdCardEntryDTO     `json:"mset"`
	SSet     []cmdCardEntryDTO     `json:"sset"`
	Activate []cmdActivateEntryDTO `json:"activate"`
	ToBP     bool                  `json:"toBP"`
	ToEP     bool                  `json:"toEP"`
	Shuffle  bool                  `json:"shuffle"`
}

type selectBattleCmdDTO struct {
	Player   uint8                 `json:"player"`
	Activate []cmdActivateEntryDTO `json:"activate"`
	Attack   []cmdAttackEntryDTO   `json:"attack"`
	ToM2     bool                  `json:"toM2"`
	ToEP     bool                  `json:"toEP"`
}

// ---- duel:draw ----

type drawDTO struct {
	Player uint8   `json:"player"`
	Count  uint8   `json:"count"`
	Cards  []int32 `json:"cards"`
}

// ---- duel:attack / duel:become_target / duel:battle ----

type attackDTO struct {
	Attacker locRefDTO `json:"attacker"`
	Target   locRefDTO `json:"target"`
}

type becomeTargetDTO struct {
	Targets []locRefDTO `json:"targets"`
}

type battleDTO struct {
	Attacker          locPosRefDTO `json:"attacker"`
	AttackerATK       int32        `json:"attackerATK"`
	AttackerDEF       int32        `json:"attackerDEF"`
	AttackerDestroyed bool         `json:"attackerDestroyed"`
	Target            locPosRefDTO `json:"target"`
	TargetATK         int32        `json:"targetATK"`
	TargetDEF         int32        `json:"targetDEF"`
	TargetDestroyed   bool         `json:"targetDestroyed"`
}

// ---- duel:update_data / duel:update_card（query blob 解码结果）----

// counterDTO 是 QUERY_COUNTERS 条目（type | count<<16 拆开后的两半）。
type counterDTO struct {
	Type  uint32 `json:"type"`
	Count uint32 `json:"count"`
}

// queryCardDTO 是一张卡的 query blob 解码结果。键集由引擎的 query flag
// 动态决定（缓存命中时未变化的字段会被引擎从 flag 剔除），所以全部用
// 指针 + omitempty：flag 置位的字段才出现，与历史手拼 map 的键集一致。
type queryCardDTO struct {
	Code        *uint32         `json:"code,omitempty"`
	Position    *locPosRefDTO   `json:"position,omitempty"`
	Alias       *uint32         `json:"alias,omitempty"`
	Type        *uint32         `json:"type,omitempty"`
	Level       *uint32         `json:"level,omitempty"`
	Rank        *uint32         `json:"rank,omitempty"`
	Attribute   *uint32         `json:"attribute,omitempty"`
	Race        *uint32         `json:"race,omitempty"`
	Attack      *int32          `json:"attack,omitempty"`
	Defense     *int32          `json:"defense,omitempty"`
	BaseAttack  *int32          `json:"baseAttack,omitempty"`
	BaseDefense *int32          `json:"baseDefense,omitempty"`
	Reason      *uint32         `json:"reason,omitempty"`
	ReasonCard  *locPosRefDTO   `json:"reasonCard,omitempty"`
	EquipCard   *locPosRefDTO   `json:"equipCard,omitempty"`
	Targets     []*locPosRefDTO `json:"targets,omitempty"`
	Overlays    []uint32        `json:"overlays,omitempty"`
	Counters    []*counterDTO   `json:"counters,omitempty"`
	Owner       *int32          `json:"owner,omitempty"`
	Status      *uint32         `json:"status,omitempty"`
	LScale      *uint32         `json:"lscale,omitempty"`
	RScale      *uint32         `json:"rscale,omitempty"`
	Link        *uint32         `json:"link,omitempty"`
	LinkMarker  *uint32         `json:"linkMarker,omitempty"`
}

type updateDataDTO struct {
	Player   uint8           `json:"player"`
	Location uint8           `json:"location"`
	Cards    []*queryCardDTO `json:"cards"`
}

// updateCardDTO 是 duel:update_card 事件体。query 字段（Card）历史上
// 平铺在事件顶层（前端 DuelUpdateCardEvent extends CardQuery），
// MarshalJSON 保持这一形状。
type updateCardDTO struct {
	Player   uint8         `json:"player"`
	Location uint8         `json:"location"`
	Sequence uint8         `json:"sequence"`
	Card     *queryCardDTO `json:"-"`
}

func (d updateCardDTO) MarshalJSON() ([]byte, error) {
	type fixed struct {
		Player   uint8 `json:"player"`
		Location uint8 `json:"location"`
		Sequence uint8 `json:"sequence"`
	}
	head, err := json.Marshal(fixed{Player: d.Player, Location: d.Location, Sequence: d.Sequence})
	if err != nil {
		return nil, err
	}
	if d.Card == nil {
		return head, nil
	}
	tail, err := json.Marshal(d.Card)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(tail, []byte("{}")) {
		return head, nil
	}
	head = bytes.TrimSuffix(head, []byte("}"))
	tail = bytes.TrimPrefix(tail, []byte("{"))
	return append(append(head, ','), tail...), nil
}

// ---- duel:announce_card ----

type announceCardDTO struct {
	Player     uint8   `json:"player"`
	Options    []int32 `json:"options"`
	Candidates []int32 `json:"candidates"`
	Decodable  bool    `json:"decodable"`
}

// ---- duel:select_place ----

type zoneRefDTO struct {
	Loc int `json:"loc"`
	Seq int `json:"seq"`
}

type selectPlaceDTO struct {
	Player uint8        `json:"player"`
	Count  uint8        `json:"count"`
	Flag   uint32       `json:"flag"`
	Zones  []zoneRefDTO `json:"zones"`
}

// ---- duel:confirm_cards / duel:refresh_deck ----

type confirmCardsDTO struct {
	Player    uint8                       `json:"player"`
	SkipPanel bool                        `json:"skipPanel"`
	Cards     []protocol.ConfirmCardEntry `json:"cards"`
}

type refreshDeckDTO struct {
	Player uint8 `json:"player"`
}

// ---- duel:shuffle_set_card ----

type shuffleSetCardEntryDTO struct {
	Code int32 `json:"code"`
	C    uint8 `json:"c"`
	L    uint8 `json:"l"`
	S    uint8 `json:"s"`
	P    uint8 `json:"p"`
}

type shuffleSetCardDTO struct {
	Loc   uint8                    `json:"loc"`
	Cards []shuffleSetCardEntryDTO `json:"cards"`
}

// ---- duel:field_disabled / duel:equip 系 ----

type fieldDisabledDTO struct {
	Zones uint32 `json:"zones"`
}

// cardTargetDTO 供 duel:equip / duel:card_target / duel:cancel_target 共用
// （三者布局一致：卡位置 + 对象位置）。
type cardTargetDTO struct {
	Card   locPosRefDTO `json:"card"`
	Target locPosRefDTO `json:"target"`
}

type unequipDTO struct {
	Card locPosRefDTO `json:"card"`
}

type missedEffectDTO struct {
	Card locPosRefDTO `json:"card"`
	Code uint32       `json:"code"`
}

// ---- duel:card_hint / duel:player_hint ----

type cardHintDTO struct {
	Card locPosRefDTO `json:"card"`
	Type uint8        `json:"type"`
	Data int32        `json:"data"`
}

type playerHintDTO struct {
	Player uint8 `json:"player"`
	Type   uint8 `json:"type"`
	Data   int32 `json:"data"`
}

// ---- 单人谜题 Debug 消息 ----

type aiNameDTO struct {
	Name string `json:"name"`
}

type showHintDTO struct {
	Text string `json:"text"`
}

// ---- duel:reload_field（MSG_RELOAD_FIELD 布场快照）----

type reloadFieldDTO struct {
	Rule       uint8                    `json:"rule"`
	Players    [2]*reloadFieldPlayerDTO `json:"players"`
	ChainCount int                      `json:"chainCount"`
}

type reloadFieldPlayerDTO struct {
	LP      int32                 `json:"lp"`
	MZone   []*reloadMZoneSlotDTO `json:"mzone"`
	SZone   []*reloadSZoneSlotDTO `json:"szone"`
	Deck    uint8                 `json:"deck"`
	Hand    uint8                 `json:"hand"`
	Grave   uint8                 `json:"grave"`
	Removed uint8                 `json:"removed"`
	Extra   uint8                 `json:"extra"`
	ExtraP  uint8                 `json:"extraP"`
}

type reloadMZoneSlotDTO struct {
	Pos     uint8 `json:"pos"`
	Overlay uint8 `json:"overlay"`
}

type reloadSZoneSlotDTO struct {
	Pos uint8 `json:"pos"`
}

// ------------------------------------------------------------------
// STOC 事件 DTO
// ------------------------------------------------------------------

type typeChangeDTO struct {
	Type   uint8 `json:"type"`
	IsHost bool  `json:"isHost"`
	Pos    uint8 `json:"pos"`
}

type playerEnterDTO struct {
	Pos  uint8  `json:"pos"`
	Name string `json:"name"`
}

type playerChangeDTO struct {
	Pos    uint8 `json:"pos"`
	Status uint8 `json:"status"`
	Ready  bool  `json:"ready"`
}

type watchChangeDTO struct {
	Count uint16 `json:"count"`
}

type handResultDTO struct {
	Res1 uint8 `json:"res1"`
	Res2 uint8 `json:"res2"`
}

type timeLimitDTO struct {
	Player   uint8  `json:"player"`
	LeftTime uint16 `json:"leftTime"`
}

type errorMsgDTO struct {
	Msg  uint8  `json:"msg"`
	Code uint32 `json:"code"`
}

type chatDTO struct {
	Player uint16 `json:"player"`
	Msg    string `json:"msg"`
}

type replayDTO struct {
	Name string `json:"name"`
	Size int    `json:"size"`
}
