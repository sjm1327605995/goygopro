/**
 * Wails → eventBus 转发名单 — 桥层事件名的唯一来源。
 *
 * wails_bridge.js 对名单内每个事件注册 Wails Events.On 并转发到内部
 * eventBus。此前这份名单是桥文件里的裸数组，Go 侧新增 emit 点（如
 * duel:update_data / duel:update_card）时名单不会跟着更新，事件被静默
 * 丢弃；cmd/wails/events_parity_test.go 现在把本文件的事件名与 Go emit
 * 点做名字对齐，漏转会直接挂测试。
 */

/** STOC 协议事件（duel_client.go handleSTOCPacket emit） */
export const STOC_EVENTS = [
  'stoc:join_game',
  'stoc:type_change',
  'stoc:player_enter',
  'stoc:player_change',
  'stoc:watch_change',
  'stoc:duel_start',
  'stoc:deck_count',
  'stoc:select_hand',
  'stoc:hand_result',
  'stoc:select_tp',
  'stoc:time_limit',
  'stoc:chat',
  'stoc:error_msg',
  'stoc:replay',
  'stoc:duel_end',
  'stoc:change_side',
  'stoc:waiting_side',
  'stoc:teammate_surrender',
  'stoc:disconnected',
];

/** 引擎事件（engine_bindings.go engineBindings emit） */
export const DUEL_EVENTS = [
  'duel:start',
  'duel:new_turn',
  'duel:new_phase',
  'duel:move',
  'duel:pos_change',
  'duel:set',
  'duel:summoning',
  'duel:spsummoning',
  'duel:flipsummoning',
  'duel:summoned',
  'duel:spsummoned',
  'duel:flipsummoned',
  'duel:draw',
  'duel:chaining',
  'duel:chained',
  'duel:chain_solving',
  'duel:chain_solved',
  'duel:chain_end',
  'duel:chain_negated',
  'duel:damage',
  'duel:recover',
  'duel:lp_update',
  'duel:pay_lpcost',
  'duel:attack',
  'duel:battle',
  'duel:attack_disabled',
  'duel:become_target',
  'duel:win',
  'duel:select_idlecmd',
  'duel:select_battlecmd',
  'duel:select_effectyn',
  'duel:select_yesno',
  'duel:select_option',
  'duel:select_card',
  'duel:select_position',
  'duel:select_place',
  'duel:select_chain',
  'duel:select_counter',
  'duel:select_sum',
  'duel:sort_card',
  'duel:select_unselect',
  'duel:announce_race',
  'duel:announce_attrib',
  'duel:announce_card',
  'duel:announce_number',
  'duel:rps',
  'duel:hand_res',
  'duel:toss_coin',
  'duel:toss_dice',
  'duel:waiting',
  'duel:confirm_decktop',
  'duel:update_data',
  'duel:update_card',
  // ---- 波 B：决斗过程展示/标记消息 ----
  'duel:hint',
  'duel:confirm_cards',
  'duel:shuffle_hand',
  'duel:shuffle_extra',
  'duel:refresh_deck',
  'duel:shuffle_set_card',
  'duel:deck_top',
  'duel:swap',
  'duel:field_disabled',
  'duel:card_selected',
  'duel:random_selected',
  'duel:equip',
  'duel:unequip',
  'duel:card_target',
  'duel:cancel_target',
  'duel:add_counter',
  'duel:remove_counter',
  'duel:missed_effect',
  'duel:card_hint',
  'duel:player_hint',
  'duel:match_kill',
  // ---- 波 J：单人模式的调试/重载消息（libdebug / query_field_info）----
  'duel:ai_name',
  'duel:show_hint',
  'duel:reload_field',
];

/** 单人模式会话事件（single_mode.go StartSingle 的 goroutine emit） */
export const SINGLE_EVENTS = ['single:ended'];

/** 桥层应转发的全部事件 */
export const FORWARDED_EVENTS = [...STOC_EVENTS, ...DUEL_EVENTS, ...SINGLE_EVENTS];
