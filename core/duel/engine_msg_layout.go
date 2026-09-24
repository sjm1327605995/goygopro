package duel

import (
	"github.com/sjm1327605995/goygopro/ocgcore"
)

// 本文件是引擎消息（ocgcore MSG_*）字节布局的唯一事实源：
// batchResponseOffset（message_scan.go 的批次走查）与 Analyze 的处理器
// （analyze_shared.go）都以这里的 engineMsgLayouts 为准，测试
// TestAnalyzeLayoutMatchesBatchWalker 强制两者逐消息消费一致，
// 防止再次出现 message_scan 与 Analyze 各写一份导致的事实分叉
// （历史分叉例：MSG_SELECT_SUM 的 player 偏移）。
//
// 布局描述的是消息 TYPE 字节之后的 BODY。字段读取语义
// （玩家路由、隐藏规则等）在 analyze_shared.go 的各 handler 中。

type layKind uint8

const (
	// layFixed: 固定 n 字节
	layFixed layKind = iota
	// layCountList: count(1) + count*n
	layCountList
	// layToss: player(1) + count(1) + count 个 1 字节结果（MSG_TOSS_COIN/DICE）
	layToss
	// layChain: player(1) + count(1) + spec/forced/hint(9) + count*14（MSG_SELECT_CHAIN）
	layChain
	// layQueryBlobs: MSG_UPDATE_DATA 的查询块序列
	layQueryBlobs
	// layOneQueryBlob: MSG_UPDATE_CARD 的单个查询块
	layOneQueryBlob
	// layTagSwap: MSG_TAG_SWAP 的可变尺寸块
	layTagSwap
	// layReloadField: MSG_RELOAD_FIELD
	layReloadField
	// laySizedString: len(2) + len 字节 + 结尾 1 字节（MSG_AI_NAME/MSG_SHOW_HINT）
	laySizedString
)

type layStep struct {
	kind layKind
	n    int
}

type engineMsgLayout struct {
	steps    []layStep // 消息体的布局（win 消息无需走查）
	response bool      // 该消息消费一个已记录的客户端响应
	win      bool      // MSG_WIN：终局消息，批次即告结束
}

func fixed(n int) layStep     { return layStep{kind: layFixed, n: n} }
func countList(n int) layStep { return layStep{kind: layCountList, n: n} }

// engineMsgLayouts 覆盖引擎可能发出的全部消息布局。
// 数值与 source/ygopro 的 single_duel.cpp / tag_duel.cpp 解析逐字节一致。
var engineMsgLayouts = map[uint8]engineMsgLayout{
	// ----- 消费一个已记录响应的消息（交互提示） -----
	ocgcore.MSG_SELECT_BATTLECMD: {response: true, steps: []layStep{fixed(1), countList(11), countList(8), fixed(2)}},
	// 5x(count+count*7), count+count*11, +3
	ocgcore.MSG_SELECT_IDLECMD: {response: true, steps: []layStep{
		fixed(1), countList(7), countList(7), countList(7), countList(7), countList(7), countList(11), fixed(3),
	}},
	ocgcore.MSG_SELECT_EFFECTYN: {response: true, steps: []layStep{fixed(1 + 12)}},
	ocgcore.MSG_SELECT_YESNO:    {response: true, steps: []layStep{fixed(1 + 4)}},
	ocgcore.MSG_SELECT_OPTION:   {response: true, steps: []layStep{fixed(1), countList(4)}},
	// player, cancelable, min, max, count+count*8
	ocgcore.MSG_SELECT_CARD:    {response: true, steps: []layStep{fixed(1 + 3), countList(8)}},
	ocgcore.MSG_SELECT_TRIBUTE: {response: true, steps: []layStep{fixed(1 + 3), countList(8)}},
	// player, count, spec, forced, hint0, hint1, count*14
	ocgcore.MSG_SELECT_CHAIN:    {response: true, steps: []layStep{{kind: layChain}}},
	ocgcore.MSG_SELECT_PLACE:    {response: true, steps: []layStep{fixed(1 + 5)}},
	ocgcore.MSG_SELECT_DISFIELD: {response: true, steps: []layStep{fixed(1 + 5)}},
	ocgcore.MSG_SELECT_POSITION: {response: true, steps: []layStep{fixed(1 + 5)}},
	ocgcore.MSG_SELECT_COUNTER:  {response: true, steps: []layStep{fixed(1 + 4), countList(9)}},
	// skip1, player, skip6, count+count*11, count+count*11
	ocgcore.MSG_SELECT_SUM: {response: true, steps: []layStep{
		fixed(1), fixed(1), fixed(6), countList(11), countList(11),
	}},
	ocgcore.MSG_SORT_CARD: {response: true, steps: []layStep{fixed(1), countList(7)}},
	// MSG_SORT_CHAIN（上游 edo9300 核心的 SortCard 处理器 is_chain 变体）：
	// 经典线格式与 MSG_SORT_CARD 相同，且同样等待客户端排序响应。
	// 本项目所用引擎（Fluorohydride 主线）不发出，原版 Analyze 无 case。
	ocgcore.MSG_SORT_CHAIN: {response: true, steps: []layStep{fixed(1), countList(7)}},
	ocgcore.MSG_SELECT_UNSELECT_CARD: {response: true, steps: []layStep{
		fixed(1 + 4), countList(8), countList(8),
	}},
	ocgcore.MSG_ROCK_PAPER_SCISSORS: {response: true, steps: []layStep{fixed(1)}},
	ocgcore.MSG_ANNOUNCE_RACE:       {response: true, steps: []layStep{fixed(1 + 5)}},
	ocgcore.MSG_ANNOUNCE_ATTRIB:     {response: true, steps: []layStep{fixed(1 + 5)}},
	ocgcore.MSG_ANNOUNCE_CARD:       {response: true, steps: []layStep{fixed(1), countList(4)}},
	ocgcore.MSG_ANNOUNCE_NUMBER:     {response: true, steps: []layStep{fixed(1), countList(4)}},

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
	ocgcore.MSG_START:            {steps: []layStep{fixed(18)}},
	ocgcore.MSG_WIN:              {win: true},
	ocgcore.MSG_HINT:             {steps: []layStep{fixed(6)}},
	ocgcore.MSG_UPDATE_DATA:      {steps: []layStep{fixed(2), {kind: layQueryBlobs}}},
	ocgcore.MSG_UPDATE_CARD:      {steps: []layStep{fixed(3), {kind: layOneQueryBlob}}},
	ocgcore.MSG_CONFIRM_DECKTOP:  {steps: []layStep{fixed(1), countList(7)}},
	ocgcore.MSG_CONFIRM_EXTRATOP: {steps: []layStep{fixed(1), countList(7)}},
	ocgcore.MSG_CONFIRM_CARDS:    {steps: []layStep{fixed(1 + 1), countList(7)}},
	ocgcore.MSG_SHUFFLE_DECK:     {steps: []layStep{fixed(1)}},
	ocgcore.MSG_REFRESH_DECK:     {steps: []layStep{fixed(1)}},
	ocgcore.MSG_SWAP_GRAVE_DECK:  {steps: []layStep{fixed(1)}},
	ocgcore.MSG_SHUFFLE_HAND:     {steps: []layStep{fixed(1), countList(4)}},
	ocgcore.MSG_SHUFFLE_EXTRA:    {steps: []layStep{fixed(1), countList(4)}},
	ocgcore.MSG_SHUFFLE_SET_CARD: {steps: []layStep{fixed(1), countList(8)}},
	ocgcore.MSG_DECK_TOP:         {steps: []layStep{fixed(6)}},
	ocgcore.MSG_NEW_TURN:         {steps: []layStep{fixed(1)}},
	ocgcore.MSG_NEW_PHASE:        {steps: []layStep{fixed(2)}},
	ocgcore.MSG_MOVE:             {steps: []layStep{fixed(16)}},
	ocgcore.MSG_POS_CHANGE:       {steps: []layStep{fixed(9)}},
	ocgcore.MSG_SET:              {steps: []layStep{fixed(8)}},
	ocgcore.MSG_SWAP:             {steps: []layStep{fixed(16)}},
	ocgcore.MSG_FIELD_DISABLED:   {steps: []layStep{fixed(4)}},
	ocgcore.MSG_SUMMONING:        {steps: []layStep{fixed(8)}},
	ocgcore.MSG_SPSUMMONING:      {steps: []layStep{fixed(8)}},
	ocgcore.MSG_FLIPSUMMONING:    {steps: []layStep{fixed(8)}},
	ocgcore.MSG_CHAINING:         {steps: []layStep{fixed(16)}},
	ocgcore.MSG_CHAINED:          {steps: []layStep{fixed(1)}},
	ocgcore.MSG_CHAIN_SOLVING:    {steps: []layStep{fixed(1)}},
	ocgcore.MSG_CHAIN_SOLVED:     {steps: []layStep{fixed(1)}},
	ocgcore.MSG_CHAIN_NEGATED:    {steps: []layStep{fixed(1)}},
	ocgcore.MSG_CHAIN_DISABLED:   {steps: []layStep{fixed(1)}},
	ocgcore.MSG_CARD_SELECTED:    {steps: []layStep{fixed(1), countList(4)}},
	ocgcore.MSG_RANDOM_SELECTED:  {steps: []layStep{fixed(1), countList(4)}},
	ocgcore.MSG_BECOME_TARGET:    {steps: []layStep{countList(4)}},
	ocgcore.MSG_DRAW:             {steps: []layStep{fixed(1), countList(4)}},
	ocgcore.MSG_DAMAGE:           {steps: []layStep{fixed(5)}},
	ocgcore.MSG_RECOVER:          {steps: []layStep{fixed(5)}},
	ocgcore.MSG_LPUPDATE:         {steps: []layStep{fixed(5)}},
	ocgcore.MSG_PAY_LPCOST:       {steps: []layStep{fixed(5)}},
	ocgcore.MSG_EQUIP:            {steps: []layStep{fixed(8)}},
	ocgcore.MSG_CARD_TARGET:      {steps: []layStep{fixed(8)}},
	ocgcore.MSG_CANCEL_TARGET:    {steps: []layStep{fixed(8)}},
	ocgcore.MSG_UNEQUIP:          {steps: []layStep{fixed(4)}},
	ocgcore.MSG_ADD_COUNTER:      {steps: []layStep{fixed(7)}},
	ocgcore.MSG_REMOVE_COUNTER:   {steps: []layStep{fixed(7)}},
	ocgcore.MSG_ATTACK:           {steps: []layStep{fixed(8)}},
	ocgcore.MSG_BATTLE:           {steps: []layStep{fixed(26)}},
	ocgcore.MSG_MISSED_EFFECT:    {steps: []layStep{fixed(8)}},
	ocgcore.MSG_BE_CHAIN_TARGET:  {steps: []layStep{fixed(1)}},
	ocgcore.MSG_CREATE_RELATION:  {steps: []layStep{fixed(16)}},
	ocgcore.MSG_RELEASE_RELATION: {steps: []layStep{fixed(16)}},
	ocgcore.MSG_TOSS_COIN:        {steps: []layStep{{kind: layToss}}},
	ocgcore.MSG_TOSS_DICE:        {steps: []layStep{{kind: layToss}}},
	ocgcore.MSG_HAND_RES:         {steps: []layStep{fixed(1)}},
	ocgcore.MSG_CARD_HINT:        {steps: []layStep{fixed(9)}},
	ocgcore.MSG_PLAYER_HINT:      {steps: []layStep{fixed(6)}},
	ocgcore.MSG_TAG_SWAP:         {steps: []layStep{{kind: layTagSwap}}},
	ocgcore.MSG_RELOAD_FIELD:     {steps: []layStep{{kind: layReloadField}}},
	ocgcore.MSG_AI_NAME:          {steps: []layStep{{kind: laySizedString}}},
	ocgcore.MSG_SHOW_HINT:        {steps: []layStep{{kind: laySizedString}}},
	ocgcore.MSG_MATCH_KILL:       {steps: []layStep{fixed(4)}},
}

// walkEngineMessage 走查 msg 中 body（type 字节之后的位置）开始的单条消息，
// 返回消息结束偏移、是否需要响应、是否终局。布局未知或越界时 ok=false。
func walkEngineMessage(msg []byte, body int) (end int, response bool, win bool, ok bool) {
	op := msg[body-1]
	layout, exists := engineMsgLayouts[op]
	if !exists {
		return 0, false, false, false
	}
	if layout.win {
		return body, false, true, true
	}
	if !walkLayoutSteps(&body, msg, layout.steps) {
		return 0, false, false, false
	}
	return body, layout.response, false, true
}

// walkLayoutSteps 按布局步骤推进 *p（与旧 switch 版逐字节等价）。
func walkLayoutSteps(p *int, msg []byte, steps []layStep) bool {
	for _, s := range steps {
		switch s.kind {
		case layFixed:
			if !adv(p, msg, s.n) {
				return false
			}
		case layCountList:
			if !walkCountList(p, msg, s.n) {
				return false
			}
		case layToss:
			if !adv(p, msg, 2) {
				return false
			}
			if !adv(p, msg, int(msg[*p-1])) {
				return false
			}
		case layChain:
			if !adv(p, msg, 1+1) {
				return false
			}
			count := int(msg[*p-1])
			if !adv(p, msg, 9+count*14) {
				return false
			}
		case layQueryBlobs:
			if !walkQueryBlobs(p, msg) {
				return false
			}
		case layOneQueryBlob:
			if !walkOneQueryBlob(p, msg) {
				return false
			}
		case layTagSwap:
			// body[2]=extra count, body[4]=hand count（相对消息体）
			if *p+5 > len(msg) {
				return false
			}
			size := 9 + int(msg[*p+2])*4 + int(msg[*p+4])*4
			if !adv(p, msg, size) {
				return false
			}
		case layReloadField:
			if !walkReloadField(p, msg) {
				return false
			}
		case laySizedString:
			if *p+2 > len(msg) {
				return false
			}
			l := int(msg[*p]) | int(msg[*p+1])<<8
			if !adv(p, msg, 2+l+1) {
				return false
			}
		}
	}
	return true
}
