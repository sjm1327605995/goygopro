package enginemsg

import (
	"github.com/sjm1327605995/goygopro/ocgcore"
)

// 本文件是引擎消息（ocgcore MSG_*）字节布局的唯一事实源：
// BatchResponseOffset（message_scan.go 的批次走查，ReplayMode.Run 与单人
// 模式共用）与 core/duel Analyze 的处理器（analyze_shared.go）都以这里的
// EngineMsgLayouts 为准，测试 TestAnalyzeLayoutMatchesBatchWalker 强制
// 两者逐消息消费一致，防止再次出现 message_scan 与 Analyze 各写一份导致的
// 事实分叉（历史分叉例：MSG_SELECT_SUM 的 player 偏移）。
// 本文件随 ReplayMode 一起从 core/duel 移入：布局表的语义来源就是 C++ 客户端
// ReplayMode::ReplayAnalyze 的消息走查。
//
// 布局描述的是消息 TYPE 字节之后的 BODY。字段读取语义
// （玩家路由、隐藏规则等）在 core/duel/analyze_shared.go 的各 handler 中。

type LayKind uint8

const (
	// LayFixed: 固定 N 字节（LayStep.N）
	LayFixed LayKind = iota
	// LayCountList: count(1) + count*n
	LayCountList
	// LayToss: player(1) + count(1) + count 个 1 字节结果（MSG_TOSS_COIN/DICE）
	LayToss
	// LayChain: player(1) + count(1) + spec/forced/hint(9) + count*14（MSG_SELECT_CHAIN）
	LayChain
	// LayQueryBlobs: MSG_UPDATE_DATA 的查询块序列
	LayQueryBlobs
	// LayOneQueryBlob: MSG_UPDATE_CARD 的单个查询块
	LayOneQueryBlob
	// LayTagSwap: MSG_TAG_SWAP 的可变尺寸块
	LayTagSwap
	// LayReloadField: MSG_RELOAD_FIELD
	LayReloadField
	// LaySizedString: len(2) + len 字节 + 结尾 1 字节（MSG_AI_NAME/MSG_SHOW_HINT）
	LaySizedString
)

type LayStep struct {
	Kind LayKind
	N    int
}

type EngineMsgLayout struct {
	Steps    []LayStep // 消息体的布局（win 消息无需走查）
	response bool      // 该消息消费一个已记录的客户端响应
	Win      bool      // MSG_WIN：终局消息，批次即告结束
}

func fixed(size int) LayStep     { return LayStep{Kind: LayFixed, N: size} }
func countList(size int) LayStep { return LayStep{Kind: LayCountList, N: size} }

// EngineMsgLayouts 覆盖引擎可能发出的全部消息布局。
// 数值与 Fluorohydride/ygopro 的 single_duel.cpp / tag_duel.cpp 解析逐字节一致。
var EngineMsgLayouts = map[uint8]EngineMsgLayout{
	// ----- 消费一个已记录响应的消息（交互提示） -----
	ocgcore.MSG_SELECT_BATTLECMD: {response: true, Steps: []LayStep{fixed(1), countList(11), countList(8), fixed(2)}},
	// 5x(count+count*7), count+count*11, +3
	ocgcore.MSG_SELECT_IDLECMD: {response: true, Steps: []LayStep{
		fixed(1), countList(7), countList(7), countList(7), countList(7), countList(7), countList(11), fixed(3),
	}},
	ocgcore.MSG_SELECT_EFFECTYN: {response: true, Steps: []LayStep{fixed(1 + 12)}},
	ocgcore.MSG_SELECT_YESNO:    {response: true, Steps: []LayStep{fixed(1 + 4)}},
	ocgcore.MSG_SELECT_OPTION:   {response: true, Steps: []LayStep{fixed(1), countList(4)}},
	// player, cancelable, min, max, count+count*8
	ocgcore.MSG_SELECT_CARD:    {response: true, Steps: []LayStep{fixed(1 + 3), countList(8)}},
	ocgcore.MSG_SELECT_TRIBUTE: {response: true, Steps: []LayStep{fixed(1 + 3), countList(8)}},
	// player, count, spec, forced, hint0, hint1, count*14
	ocgcore.MSG_SELECT_CHAIN:    {response: true, Steps: []LayStep{{Kind: LayChain}}},
	ocgcore.MSG_SELECT_PLACE:    {response: true, Steps: []LayStep{fixed(1 + 5)}},
	ocgcore.MSG_SELECT_DISFIELD: {response: true, Steps: []LayStep{fixed(1 + 5)}},
	ocgcore.MSG_SELECT_POSITION: {response: true, Steps: []LayStep{fixed(1 + 5)}},
	ocgcore.MSG_SELECT_COUNTER:  {response: true, Steps: []LayStep{fixed(1 + 4), countList(9)}},
	// skip1, player, skip6, count+count*11, count+count*11
	ocgcore.MSG_SELECT_SUM: {response: true, Steps: []LayStep{
		fixed(1), fixed(1), fixed(6), countList(11), countList(11),
	}},
	ocgcore.MSG_SORT_CARD: {response: true, Steps: []LayStep{fixed(1), countList(7)}},
	// MSG_SORT_CHAIN（上游 edo9300 核心的 SortCard 处理器 is_chain 变体）：
	// 经典线格式与 MSG_SORT_CARD 相同，且同样等待客户端排序响应。
	// 本项目所用引擎（Fluorohydride 主线）不发出，原版 Analyze 无 case。
	ocgcore.MSG_SORT_CHAIN: {response: true, Steps: []LayStep{fixed(1), countList(7)}},
	ocgcore.MSG_SELECT_UNSELECT_CARD: {response: true, Steps: []LayStep{
		fixed(1 + 4), countList(8), countList(8),
	}},
	ocgcore.MSG_ROCK_PAPER_SCISSORS: {response: true, Steps: []LayStep{fixed(1)}},
	ocgcore.MSG_ANNOUNCE_RACE:       {response: true, Steps: []LayStep{fixed(1 + 5)}},
	ocgcore.MSG_ANNOUNCE_ATTRIB:     {response: true, Steps: []LayStep{fixed(1 + 5)}},
	ocgcore.MSG_ANNOUNCE_CARD:       {response: true, Steps: []LayStep{fixed(1), countList(4)}},
	ocgcore.MSG_ANNOUNCE_NUMBER:     {response: true, Steps: []LayStep{fixed(1), countList(4)}},

	// ----- 无体消息 -----
	ocgcore.MSG_RETRY:             {},
	ocgcore.MSG_REVERSE_DECK:      {},
	ocgcore.MSG_ATTACK_DISABLED:   {},
	ocgcore.MSG_DAMAGE_STEP_START: {},
	ocgcore.MSG_DAMAGE_STEP_END:   {},
	ocgcore.MSG_SUMMONED:          {},
	ocgcore.MSG_SPSUMMONED:        {},
	ocgcore.MSG_FLIPSUMMONED:      {},
	ocgcore.MSG_CHAIN_END:         {},
	ocgcore.MSG_WAITING:           {},
	// 以下四个号码在上游头文件有定义（MSG_REQUEST_DECK 见 edo9300
	// ocgapi_constants.h；MSG_CUSTOM_MSG 见新旧版 common.h），但主线与
	// edo9300 引擎均无写入方——原版 Analyze 无 case 且实际不可达。
	// 登记为空体，把「布局表不认识的字节」挡在显式跳过路径之外，
	// 防止它们被当作未知消息触发 EndDuel 兜底。
	ocgcore.MSG_REQUEST_DECK:         {},
	ocgcore.MSG_ANNOUNCE_CARD_FILTER: {},
	ocgcore.MSG_CUSTOM_MSG:           {},
	ocgcore.MSG_DUEL_WINNER:          {},

	// ----- 信息消息 -----
	ocgcore.MSG_START:            {Steps: []LayStep{fixed(18)}},
	ocgcore.MSG_WIN:              {Win: true},
	ocgcore.MSG_HINT:             {Steps: []LayStep{fixed(6)}},
	ocgcore.MSG_UPDATE_DATA:      {Steps: []LayStep{fixed(2), {Kind: LayQueryBlobs}}},
	ocgcore.MSG_UPDATE_CARD:      {Steps: []LayStep{fixed(3), {Kind: LayOneQueryBlob}}},
	ocgcore.MSG_CONFIRM_DECKTOP:  {Steps: []LayStep{fixed(1), countList(7)}},
	ocgcore.MSG_CONFIRM_EXTRATOP: {Steps: []LayStep{fixed(1), countList(7)}},
	ocgcore.MSG_CONFIRM_CARDS:    {Steps: []LayStep{fixed(1 + 1), countList(7)}},
	ocgcore.MSG_SHUFFLE_DECK:     {Steps: []LayStep{fixed(1)}},
	ocgcore.MSG_REFRESH_DECK:     {Steps: []LayStep{fixed(1)}},
	ocgcore.MSG_SWAP_GRAVE_DECK:  {Steps: []LayStep{fixed(1)}},
	ocgcore.MSG_SHUFFLE_HAND:     {Steps: []LayStep{fixed(1), countList(4)}},
	ocgcore.MSG_SHUFFLE_EXTRA:    {Steps: []LayStep{fixed(1), countList(4)}},
	ocgcore.MSG_SHUFFLE_SET_CARD: {Steps: []LayStep{fixed(1), countList(8)}},
	ocgcore.MSG_DECK_TOP:         {Steps: []LayStep{fixed(6)}},
	ocgcore.MSG_NEW_TURN:         {Steps: []LayStep{fixed(1)}},
	ocgcore.MSG_NEW_PHASE:        {Steps: []LayStep{fixed(2)}},
	ocgcore.MSG_MOVE:             {Steps: []LayStep{fixed(16)}},
	ocgcore.MSG_POS_CHANGE:       {Steps: []LayStep{fixed(9)}},
	ocgcore.MSG_SET:              {Steps: []LayStep{fixed(8)}},
	ocgcore.MSG_SWAP:             {Steps: []LayStep{fixed(16)}},
	ocgcore.MSG_FIELD_DISABLED:   {Steps: []LayStep{fixed(4)}},
	ocgcore.MSG_SUMMONING:        {Steps: []LayStep{fixed(8)}},
	ocgcore.MSG_SPSUMMONING:      {Steps: []LayStep{fixed(8)}},
	ocgcore.MSG_FLIPSUMMONING:    {Steps: []LayStep{fixed(8)}},
	ocgcore.MSG_CHAINING:         {Steps: []LayStep{fixed(16)}},
	ocgcore.MSG_CHAINED:          {Steps: []LayStep{fixed(1)}},
	ocgcore.MSG_CHAIN_SOLVING:    {Steps: []LayStep{fixed(1)}},
	ocgcore.MSG_CHAIN_SOLVED:     {Steps: []LayStep{fixed(1)}},
	ocgcore.MSG_CHAIN_NEGATED:    {Steps: []LayStep{fixed(1)}},
	ocgcore.MSG_CHAIN_DISABLED:   {Steps: []LayStep{fixed(1)}},
	ocgcore.MSG_CARD_SELECTED:    {Steps: []LayStep{fixed(1), countList(4)}},
	ocgcore.MSG_RANDOM_SELECTED:  {Steps: []LayStep{fixed(1), countList(4)}},
	ocgcore.MSG_BECOME_TARGET:    {Steps: []LayStep{countList(4)}},
	ocgcore.MSG_DRAW:             {Steps: []LayStep{fixed(1), countList(4)}},
	ocgcore.MSG_DAMAGE:           {Steps: []LayStep{fixed(5)}},
	ocgcore.MSG_RECOVER:          {Steps: []LayStep{fixed(5)}},
	ocgcore.MSG_LPUPDATE:         {Steps: []LayStep{fixed(5)}},
	ocgcore.MSG_PAY_LPCOST:       {Steps: []LayStep{fixed(5)}},
	ocgcore.MSG_EQUIP:            {Steps: []LayStep{fixed(8)}},
	ocgcore.MSG_CARD_TARGET:      {Steps: []LayStep{fixed(8)}},
	ocgcore.MSG_CANCEL_TARGET:    {Steps: []LayStep{fixed(8)}},
	ocgcore.MSG_UNEQUIP:          {Steps: []LayStep{fixed(4)}},
	ocgcore.MSG_ADD_COUNTER:      {Steps: []LayStep{fixed(7)}},
	ocgcore.MSG_REMOVE_COUNTER:   {Steps: []LayStep{fixed(7)}},
	ocgcore.MSG_ATTACK:           {Steps: []LayStep{fixed(8)}},
	ocgcore.MSG_BATTLE:           {Steps: []LayStep{fixed(26)}},
	ocgcore.MSG_MISSED_EFFECT:    {Steps: []LayStep{fixed(8)}},
	ocgcore.MSG_BE_CHAIN_TARGET:  {Steps: []LayStep{fixed(1)}},
	ocgcore.MSG_CREATE_RELATION:  {Steps: []LayStep{fixed(16)}},
	ocgcore.MSG_RELEASE_RELATION: {Steps: []LayStep{fixed(16)}},
	ocgcore.MSG_TOSS_COIN:        {Steps: []LayStep{{Kind: LayToss}}},
	ocgcore.MSG_TOSS_DICE:        {Steps: []LayStep{{Kind: LayToss}}},
	ocgcore.MSG_HAND_RES:         {Steps: []LayStep{fixed(1)}},
	ocgcore.MSG_CARD_HINT:        {Steps: []LayStep{fixed(9)}},
	ocgcore.MSG_PLAYER_HINT:      {Steps: []LayStep{fixed(6)}},
	ocgcore.MSG_TAG_SWAP:         {Steps: []LayStep{{Kind: LayTagSwap}}},
	ocgcore.MSG_RELOAD_FIELD:     {Steps: []LayStep{{Kind: LayReloadField}}},
	ocgcore.MSG_AI_NAME:          {Steps: []LayStep{{Kind: LaySizedString}}},
	ocgcore.MSG_SHOW_HINT:        {Steps: []LayStep{{Kind: LaySizedString}}},
	ocgcore.MSG_MATCH_KILL:       {Steps: []LayStep{fixed(4)}},
}

// WalkEngineMessage 走查 msg 中 body（type 字节之后的位置）开始的单条消息，
// 返回消息结束偏移、是否需要响应、是否终局。布局未知或越界时 ok=false。
func WalkEngineMessage(msg []byte, body int) (end int, response bool, win bool, ok bool) {
	op := msg[body-1]
	layout, exists := EngineMsgLayouts[op]
	if !exists {
		return 0, false, false, false
	}
	if layout.Win {
		return body, false, true, true
	}
	if !walkLayoutSteps(&body, msg, layout.Steps) {
		return 0, false, false, false
	}
	return body, layout.response, false, true
}

// walkLayoutSteps 按布局步骤推进 *pos（与旧 switch 版逐字节等价）。
func walkLayoutSteps(pos *int, msg []byte, steps []LayStep) bool {
	for _, step := range steps {
		switch step.Kind {
		case LayFixed:
			if !adv(pos, msg, step.N) {
				return false
			}
		case LayCountList:
			if !walkCountList(pos, msg, step.N) {
				return false
			}
		case LayToss:
			if !adv(pos, msg, 2) {
				return false
			}
			if !adv(pos, msg, int(msg[*pos-1])) {
				return false
			}
		case LayChain:
			if !adv(pos, msg, 1+1) {
				return false
			}
			count := int(msg[*pos-1])
			if !adv(pos, msg, 9+count*14) {
				return false
			}
		case LayQueryBlobs:
			if !walkQueryBlobs(pos, msg) {
				return false
			}
		case LayOneQueryBlob:
			if !walkOneQueryBlob(pos, msg) {
				return false
			}
		case LayTagSwap:
			// body[2]=extra count, body[4]=hand count（相对消息体）
			if *pos+5 > len(msg) {
				return false
			}
			size := 9 + int(msg[*pos+2])*4 + int(msg[*pos+4])*4
			if !adv(pos, msg, size) {
				return false
			}
		case LayReloadField:
			if !walkReloadField(pos, msg) {
				return false
			}
		case LaySizedString:
			if *pos+2 > len(msg) {
				return false
			}
			strLen := int(msg[*pos]) | int(msg[*pos+1])<<8
			if !adv(pos, msg, 2+strLen+1) {
				return false
			}
		}
	}
	return true
}
