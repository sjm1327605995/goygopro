package main

import "github.com/sjm1327605995/goygopro/protocol"

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

// updateCardDTO 是 duel:update_card 事件体。query 字段（匿名内嵌）平铺在
// 事件顶层（前端 DuelUpdateCardEvent extends CardQuery），编码/json 自动
// 提升内嵌指针的字段；nil 时整体省略。
type updateCardDTO struct {
	Player   uint8 `json:"player"`
	Location uint8 `json:"location"`
	Sequence uint8 `json:"sequence"`
	*queryCardDTO
}
