package protocol

// restruct 独立对照基线。protocol 结构体的 struct tag 已移除（运行时序列化
// 由 core/codec 接管），而 restruct 只认 `struct` tag 键、无法自定义，因此
// 黄金测试的 restruct 基线改用本文件的影子结构体：字段名/类型/布局与
// protocol 结构体一一对应，count 关系用 restruct 的 sizeof tag 表达。
// 影子与 protocol 之间的漂移由两层护栏抓住：
//   - copyBaseline 只按同名字段拷贝：protocol 新增/改名字段在影子侧保持
//     零值，与 codec 解码结果 DeepEqual 时立刻变红；
//   - codec 侧另有多条「codec.Marshal 回到黄金 wire」的断言（
//     message_codec_test.go），protocol 布局漂移直接暴露。

import "reflect"

// ---------------------------------------------------------------------------
// 影子结构体（restruct sizeof 布局，仅测试用）
// ---------------------------------------------------------------------------

type rtSelectCardMsg struct {
	Player     uint8
	Cancelable uint8
	Min        uint8
	Max        uint8
	Count      uint8 `struct:"uint8,sizeof=Cards"`
	Cards      []SelectCardEntry
}

type rtSelectUnselectCardMsg struct {
	Player     uint8
	Finishable uint8
	Cancelable uint8
	Min        uint8
	Max        uint8
	Count1     uint8 `struct:"uint8,sizeof=Cards1"`
	Cards1     []SelectCardEntry
	Count2     uint8 `struct:"uint8,sizeof=Cards2"`
	Cards2     []SelectCardEntry
}

type rtSelectCounterMsg struct {
	Player      uint8
	CounterType uint16
	Count       uint16
	CardCount   uint8 `struct:"uint8,sizeof=Entries"`
	Entries     []CounterEntry
}

type rtSelectSumMsg struct {
	SumMode     uint8
	Player      uint8
	Acc         uint32
	Min         uint8
	Max         uint8
	MustCount   uint8 `struct:"uint8,sizeof=Must"`
	Must        []SumEntry
	SelectCount uint8 `struct:"uint8,sizeof=Select"`
	Select      []SumEntry
}

type rtSortCardMsg struct {
	Player uint8
	Count  uint8 `struct:"uint8,sizeof=Cards"`
	Cards  []SortCardEntry
}

type rtSelectChainMsg struct {
	Player   uint8
	Count    uint8 `struct:"uint8,sizeof=Chains"`
	SpeCount uint8
	Hint0    uint32
	Hint1    uint32
	Chains   []ChainEntry
}

type rtSelectBattleCmdMsg struct {
	Player uint8
	CountA uint8 `struct:"uint8,sizeof=CmdsA"`
	CmdsA  []CmdActivateEntry
	CountB uint8 `struct:"uint8,sizeof=CmdsB"`
	CmdsB  []CmdAttackEntry
	ToM2   uint8
	ToEP   uint8
}

type rtSelectIdleCmdMsg struct {
	Player     uint8
	CountA     uint8 `struct:"uint8,sizeof=CmdsA"`
	CmdsA      []CmdCardEntry
	CountB     uint8 `struct:"uint8,sizeof=CmdsB"`
	CmdsB      []CmdCardEntry
	CountC     uint8 `struct:"uint8,sizeof=CmdsC"`
	CmdsC      []CmdCardEntry
	CountD     uint8 `struct:"uint8,sizeof=CmdsD"`
	CmdsD      []CmdCardEntry
	CountE     uint8 `struct:"uint8,sizeof=CmdsE"`
	CmdsE      []CmdCardEntry
	CountF     uint8 `struct:"uint8,sizeof=CmdsF"`
	CmdsF      []CmdActivateEntry
	ToBP       uint8
	ToEP       uint8
	CanShuffle uint8
}

type rtConfirmDeckTopMsg struct {
	Player uint8
	Count  uint8 `struct:"uint8,sizeof=Cards"`
	Cards  []ConfirmCardEntry
}

type rtBecomeTargetMsg struct {
	Count   uint8 `struct:"uint8,sizeof=Targets"`
	Targets []uint32
}

type rtDrawMsg struct {
	Player uint8
	Count  uint8 `struct:"uint8,sizeof=Cards"`
	Cards  []int32
}

type rtSelectOptionMsg struct {
	Player  uint8
	Count   uint8 `struct:"uint8,sizeof=Options"`
	Options []int32
}

type rtTossCoinMsg struct {
	Player  uint8
	Count   uint8 `struct:"uint8,sizeof=Results"`
	Results []uint8
}

type rtAnnounceCardMsg struct {
	Player uint8
	Count  uint8 `struct:"uint8,sizeof=Values"`
	Values []int32
}

type rtShuffleHandMsg struct {
	Player uint8
	Count  uint8 `struct:"uint8,sizeof=Cards"`
	Cards  []int32
}

type rtTagSwapMsg struct {
	Player     uint8
	MainCount  uint8 `struct:"uint8,sizeof=Main"`
	ExtraCount uint8 `struct:"uint8,sizeof=Extra"`
	SideCount  uint8 `struct:"uint8,sizeof=Side"`
	Main       []int32
	Extra      []int32
	Side       []int32
}

// restructShadowFactories 按黄金用例名提供 restruct 影子样本。只有带
// count 变长列表的消息需要影子（restruct 无 sizeof 解不了裸 slice）；
// 定长消息直接喂 protocol 结构体（无 tag 的 restruct 按原生类型定长读写，
// 行为与带 tag 时一致）。
var restructShadowFactories = map[string]func() any{
	"SelectCardMsg":         func() any { return &rtSelectCardMsg{} },
	"SelectUnselectCardMsg": func() any { return &rtSelectUnselectCardMsg{} },
	"SelectCounterMsg":      func() any { return &rtSelectCounterMsg{} },
	"SelectSumMsg":          func() any { return &rtSelectSumMsg{} },
	"SortCardMsg":           func() any { return &rtSortCardMsg{} },
	"SelectChainMsg":        func() any { return &rtSelectChainMsg{} },
	"SelectBattleCmdMsg":    func() any { return &rtSelectBattleCmdMsg{} },
	"SelectIdleCmdMsg":      func() any { return &rtSelectIdleCmdMsg{} },
	"ConfirmDeckTopMsg":     func() any { return &rtConfirmDeckTopMsg{} },
	"BecomeTargetMsg":       func() any { return &rtBecomeTargetMsg{} },
	"DrawMsg":               func() any { return &rtDrawMsg{} },
	"SelectOptionMsg":       func() any { return &rtSelectOptionMsg{} },
	"TossCoinMsg":           func() any { return &rtTossCoinMsg{} },
	"AnnounceCardMsg":       func() any { return &rtAnnounceCardMsg{} },
	"ShuffleHandMsg":        func() any { return &rtShuffleHandMsg{} },
	"TagSwapMsg":            func() any { return &rtTagSwapMsg{} },
}

// restructBaseline 返回用例的 restruct 解码目标：有影子用影子，否则直接用
// protocol 结构体（msg 本身）。
func restructBaseline(name string, msg any) any {
	if sf, ok := restructShadowFactories[name]; ok {
		return sf()
	}
	return msg
}

// copyBaseline 把 restruct 影子样本的字段按同名拷贝进 protocol 结构体
// （Count8 等标记类型与 uint8 数值互转）。影子侧缺失的字段保持零值——
// 这正是抓住「protocol 改布局忘改影子」漂移的机制。
func copyBaseline(dst, src any) {
	dv := reflect.ValueOf(dst).Elem()
	sv := reflect.ValueOf(src).Elem()
	st := sv.Type()
	for i := 0; i < sv.NumField(); i++ {
		sf := st.Field(i)
		if sf.PkgPath != "" { // 未导出字段不拷
			continue
		}
		df := dv.FieldByName(sf.Name)
		if !df.IsValid() || !df.CanSet() {
			continue
		}
		svf := sv.Field(i)
		switch {
		case svf.Type() == df.Type():
			df.Set(svf)
		case svf.Type().ConvertibleTo(df.Type()):
			df.Set(svf.Convert(df.Type()))
		default:
			panic("copyBaseline: 字段 " + sf.Name + " 类型不兼容: " +
				svf.Type().String() + " -> " + df.Type().String())
		}
	}
}
