/**
 * 事件载荷类型 — 前端事件的唯一 schema。
 *
 * 每个变体的键名必须与 Go 侧 emit 的 map/结构体 json tag 逐字一致：
 *   - duel:* 事件由 cmd/wails/engine_bindings.go 的 engineBindings 表产生
 *     （直接 emit 结构体的取 protocol/message.go 的 json tag；decorate 函数
 *     产的取 map 字面量键）。
 *   - stoc:* 事件由 cmd/wails/duel_client.go 的 handleSTOCPacket 产生。
 * cmd/wails/events_parity_test.go 扫描本文件的事件名字面量与 Go emit 点做
 * 名字对齐，防止两侧行走。
 */

// ---- 公共形状 ----

/** 打包位置条目（UnpackPackedLoc 的产物）。c=控制者 l=区域 s=序号 p=表示形式 */
export interface LocEntry {
  c: number;
  l: number;
  s: number;
  p?: number;
}

export interface CmdCardEntry {
  code: number;
  c: number;
  l: number;
  s: number;
  idx: number;
}

export interface CmdActivateEntry extends CmdCardEntry {
  desc: number;
}

export interface CmdAttackEntry extends Omit<CmdCardEntry, 'idx'> {
  diratt: boolean;
  idx: number;
}

export interface SelectCardEntry {
  code: number;
  c: number;
  l: number;
  s: number;
  p?: number;
}

// ---- duel:* 引擎事件（engine_bindings.go）----

export interface DuelStartEvent {
  event: 'duel:start';
  playerType: number;
  duelRule: number;
  lp0: number;
  lp1: number;
  deck0: number;
  extra0: number;
  deck1: number;
  extra1: number;
}

export interface DuelNewTurnEvent {
  event: 'duel:new_turn';
  player: number;
}

export interface DuelNewPhaseEvent {
  event: 'duel:new_phase';
  phase: number;
}

/** MSG_MOVE — 卡片移动。pc/pl/ps/pp 为原位置，cc/cl/cs/cp 为新位置 */
export interface DuelMoveEvent {
  event: 'duel:move';
  code: number;
  pc: number;
  pl: number;
  ps: number;
  pp: number;
  cc: number;
  cl: number;
  cs: number;
  cp: number;
  reason: number;
}

export interface DuelPosChangeEvent {
  event: 'duel:pos_change';
  code: number;
  cc: number;
  cl: number;
  cs: number;
  pp: number;
  cp: number;
}

export interface DuelSetEvent {
  event: 'duel:set';
  code: number;
  cc: number;
  cl: number;
  cs: number;
  cp: number;
}

/** MSG_SUMMONING / SPSUMMONING / FLIPSUMMONING 共用布局 */
export interface DuelSummoningEvent {
  event: 'duel:summoning' | 'duel:spsummoning' | 'duel:flipsummoning';
  code: number;
  cc: number;
  cl: number;
  cs: number;
  cp: number;
}

export interface DuelDrawEvent {
  event: 'duel:draw';
  player: number;
  count: number;
  /** 未带 0x80000000 公开标记的对手抽卡已被 Go 归零 */
  cards: number[];
}

export interface DuelDamageEvent {
  event: 'duel:damage' | 'duel:recover' | 'duel:pay_lpcost';
  player: number;
  amount: number;
}

export interface DuelLPUpdateEvent {
  event: 'duel:lp_update';
  player: number;
  lp: number;
}

export interface DuelWinEvent {
  event: 'duel:win';
  winner: number;
  type: number;
}

export interface DuelTossCoinEvent {
  event: 'duel:toss_coin' | 'duel:toss_dice';
  player: number;
  results: number[];
}

export interface DuelRPSEvent {
  event: 'duel:rps';
  phase: number;
}

export interface DuelHandResEvent {
  event: 'duel:hand_res';
  res: number; // hand0 | (hand1 << 2)，出拳 1=石头 2=剪刀 3=布（gframe f1/f2/f3）
}

export interface DuelConfirmDeckTopEvent {
  event: 'duel:confirm_decktop';
  player: number;
  count: number;
  cards: SelectCardEntry[];
}

export interface DuelAttackEvent {
  event: 'duel:attack';
  attacker: LocEntry;
  target: LocEntry;
}

/** MSG_BATTLE 26 字节结算体（攻防对撞浮层消费；reducer 不落盘） */
export interface DuelBattleEvent {
  event: 'duel:battle';
  attacker: LocEntry;
  attackerATK: number;
  attackerDEF: number;
  /** ocgcore 战破旗标 bd[0]：攻击方将被战斗破坏 */
  attackerDestroyed: boolean;
  target: LocEntry;
  targetATK: number;
  targetDEF: number;
  /** ocgcore 战破旗标 bd[1]：防守方将被战斗破坏（直接攻击时无目标） */
  targetDestroyed: boolean;
}

export interface DuelBecomeTargetEvent {
  event: 'duel:become_target';
  targets: LocEntry[];
}

export interface DuelSelectCardEvent {
  event: 'duel:select_card';
  player: number;
  cancelable: boolean;
  min: number;
  max: number;
  cards: SelectCardEntry[];
}

export interface DuelSelectUnselectEvent {
  event: 'duel:select_unselect';
  player: number;
  finishable: boolean;
  cancelable: boolean;
  min: number;
  max: number;
  cards: SelectCardEntry[];
  unselectList: SelectCardEntry[];
}

export interface ChainEntry {
  flag: number;
  forced: boolean;
  code: number;
  cc: number;
  cl: number;
  cs: number;
  cp: number;
  desc: number;
}

export interface DuelSelectChainEvent {
  event: 'duel:select_chain';
  player: number;
  count: number;
  forced: boolean;
  chains: ChainEntry[];
}

export interface DuelSelectIdleCmdEvent {
  event: 'duel:select_idlecmd';
  player: number;
  summon: CmdCardEntry[];
  spsummon: CmdCardEntry[];
  repos: CmdCardEntry[];
  mset: CmdCardEntry[];
  sset: CmdCardEntry[];
  activate: CmdActivateEntry[];
  toBP: boolean;
  toEP: boolean;
  shuffle: boolean;
}

export interface DuelSelectBattleCmdEvent {
  event: 'duel:select_battlecmd';
  player: number;
  activate: CmdActivateEntry[];
  attack: CmdAttackEntry[];
  toM2: boolean;
  toEP: boolean;
}

/** query blob 解码出的单卡状态（engine_bindings.go decodeQueryBody） */
export interface CardQuery {
  code?: number;
  position?: LocEntry;
  alias?: number;
  type?: number;
  level?: number;
  rank?: number;
  attribute?: number;
  race?: number;
  attack?: number;
  defense?: number;
  baseAttack?: number;
  baseDefense?: number;
  reason?: number;
  reasonCard?: LocEntry;
  equipCard?: LocEntry;
  targets?: LocEntry[];
  overlays?: number[];
  counters?: { type: number; count: number }[];
  owner?: number;
  status?: number;
  lscale?: number;
  rscale?: number;
  link?: number;
  linkMarker?: number;
}

export interface DuelUpdateDataEvent {
  event: 'duel:update_data';
  player: number;
  location: number;
  /**
   * 每张卡一条 query；解码失败的 blob 被跳过。MZONE/SZONE 的空槽写
   * LEN_EMPTY(4) 标记（ocgapi.cpp query_field_card），以 null 占位保持
   * 条目与槽序号对齐。
   */
  cards?: (CardQuery | null)[];
}

export interface DuelUpdateCardEvent extends CardQuery {
  event: 'duel:update_card';
  player: number;
  location: number;
  sequence: number;
}

// ---- 波 B：决斗过程展示/标记消息（engine_bindings.go）----

/** MSG_CONFIRM_CARDS — 翻开确认（原版有高亮动画；skipPanel 时不开选卡面板） */
export interface DuelConfirmCardsEvent {
  event: 'duel:confirm_cards';
  player: number;
  skipPanel: boolean;
  cards: SelectCardEntry[];
}

/** MSG_SHUFFLE_HAND — 手牌重排。对手收到的 code 已被服务端归零 */
export interface DuelShuffleHandEvent {
  event: 'duel:shuffle_hand';
  player: number;
  cards: number[];
}

/** MSG_SHUFFLE_EXTRA — 额外卡组洗牌（布局同 SHUFFLE_HAND） */
export interface DuelShuffleExtraEvent {
  event: 'duel:shuffle_extra';
  player: number;
  cards: number[];
}

export interface DuelRefreshDeckEvent {
  event: 'duel:refresh_deck';
  player: number;
}

/** MSG_SHUFFLE_SET_CARD — 同区盖卡互相换位 */
export interface DuelShuffleSetCardEvent {
  event: 'duel:shuffle_set_card';
  loc: number;
  cards: (SelectCardEntry & { p?: number })[];
}

/** MSG_DECK_TOP — 卡组顶公开（code 0x80000000 位是反转标记） */
export interface DuelDeckTopEvent {
  event: 'duel:deck_top';
  player: number;
  sequence: number;
  code: number;
}

export interface DuelSwapEvent {
  event: 'duel:swap';
  code1: number;
  cc1: number;
  cl1: number;
  cs1: number;
  cp1: number;
  code2: number;
  cc2: number;
  cl2: number;
  cs2: number;
  cp2: number;
}

/** MSG_TAG_SWAP — TAG 队友换手（手牌/额外码对非当前操作者已被服务端抹零） */
export interface DuelTagSwapEvent {
  event: 'duel:tag_swap';
  player: number;
  deckCount: number;
  extraCount: number;
  extraFaceUpCount: number;
  handCount: number;
  topCode: number;
  hand: number[];
  extra: number[];
}

export interface DuelFieldDisabledEvent {
  event: 'duel:field_disabled';
  zones: number;
}

export interface DuelCardSelectedEvent {
  event: 'duel:card_selected' | 'duel:random_selected';
  player?: number;
  cards: number[];
}

/** MSG_EQUIP — 装备线（原版 equipTarget + is_showequip） */
export interface DuelEquipEvent {
  event: 'duel:equip' | 'duel:card_target' | 'duel:cancel_target';
  card: LocEntry;
  target: LocEntry;
}

export interface DuelUnequipEvent {
  event: 'duel:unequip';
  card: LocEntry;
}

/** MSG_ADD/REMOVE_COUNTER — 指示物增减（count 为本次变动量） */
export interface DuelCounterChangeEvent {
  event: 'duel:add_counter' | 'duel:remove_counter';
  type: number;
  c: number;
  l: number;
  s: number;
  count: number;
}

/** MSG_MISSED_EFFECT — 错过时点（原版日志 SysString 1622） */
export interface DuelMissedEffectEvent {
  event: 'duel:missed_effect';
  card: LocEntry;
  code: number;
}

export interface DuelCardHintEvent {
  event: 'duel:card_hint';
  card: LocEntry;
  type: number;
  data: number;
}

export interface DuelPlayerHintEvent {
  event: 'duel:player_hint';
  player: number;
  type: number;
  data: number;
}

/**
 * MSG_HINT — 提示消息（ocgcore common.go HINT_*）：
 *   1 EVENT / 2 MESSAGE（提示条文本，duel_manager 异步 ResolveDesc）
 *   3 SELECTMSG（后续 select 类询问的标题 id，reducer 存 selectHint）
 *   4 OPSELECTED / 6 RACE / 7 ATTRIB / 8 CODE / 9 NUMBER（宣言展示：
 *     reducer 日志 + SpecOverlay ACMessage 浮条）
 *   5 EFFECT / 10 CARD（showcard 揭示，SpecOverlay）
 *   11 ZONE（区域高亮位掩码，格式同 select_place：每 16bit 一区）
 */
export interface DuelHintEvent {
  event: 'duel:hint';
  type: number;
  player: number;
  data: number;
}

/** MSG_MATCH_KILL — 比赛击杀（胜利画面展示这张卡图） */
export interface DuelMatchKillEvent {
  event: 'duel:match_kill';
  code: number;
}

export interface DuelSelectEffectYNEvent {
  event: 'duel:select_effectyn';
  player: number;
  code: number;
  loc: number;
  desc: number;
}

export interface DuelSelectYesNoEvent {
  event: 'duel:select_yesno';
  player: number;
  desc: number;
}

export interface DuelSelectOptionEvent {
  event: 'duel:select_option';
  player: number;
  options: number[];
}

export interface DuelSelectPositionEvent {
  event: 'duel:select_position';
  player: number;
  code: number;
  positions: number;
}

export interface DuelSelectPlaceEvent {
  event: 'duel:select_place';
  player: number;
  count: number;
  /** 0x7f 主怪兽区 / 0x3f00 魔法陷阱区 / 0xc000 灵摆（MR2020 位域） */
  flag: number;
  /** true = MSG_SELECT_DISFIELD（不应用 automonsterpos/autospellpos 自动落点） */
  disfield?: boolean;
  /** Go decorateSelectPlace 解码好的可选落点（含区域归属 player） */
  zones?: { player?: number; loc: number; seq: number }[];
  /** P1 落点是否交换左右（引擎对对手 seat 的镜像标记） */
  puttingPlayer?: number;
}

export interface CounterEntry {
  code: number;
  c: number;
  l: number;
  s: number;
  cnt: number;
}

export interface DuelSelectCounterEvent {
  event: 'duel:select_counter';
  player: number;
  countertype: number;
  count: number;
  cards: CounterEntry[];
}

export interface SumEntry {
  code: number;
  c: number;
  l: number;
  s: number;
  param: number;
}

export interface DuelSelectSumEvent {
  event: 'duel:select_sum';
  sumMode: number;
  player: number;
  acc: number;
  min: number;
  max: number;
  must: SumEntry[];
  select: SumEntry[];
}

export interface SortCardEntry {
  code: number;
  c: number;
  l: number;
  s: number;
}

export interface DuelSortCardEvent {
  event: 'duel:sort_card';
  player: number;
  cards: SortCardEntry[];
}

export interface DuelAnnounceRaceEvent {
  event: 'duel:announce_race' | 'duel:announce_attrib';
  player: number;
  count: number;
  available: number;
}

export interface DuelAnnounceCardEvent {
  event: 'duel:announce_card';
  player: number;
  /** options 是脚本 announce_filter 的后缀 opcode 表达式 */
  options: number[];
  /** Go decorateAnnounceCard 解码 ISCODE opcode 得到的候选卡 code */
  candidates?: number[];
  /** 整条表达式能否被 Go 侧 opcode 机解码（false 时前端回退自由输入） */
  decodable?: boolean;
}

export interface DuelAnnounceNumberEvent {
  event: 'duel:announce_number';
  player: number;
  options: number[];
}

// ---- 无数据事件 ----
// （duel:chaining / duel:chain_negated 有自己的载荷接口，不在此列——
//   同时出现在两个成员里会让联合收窄失效）
export type DuelEmptyEvent =
  | 'duel:chained'
  | 'duel:chain_solving'
  | 'duel:chain_solved'
  | 'duel:summoned'
  | 'duel:spsummoned'
  | 'duel:flipsummoned'
  | 'duel:chain_end'
  | 'duel:attack_disabled'
  | 'duel:waiting';

export interface DuelChainingEvent {
  event: 'duel:chaining';
  code: number;
  cc: number;
  cl: number;
  cs: number;
  cp: number;
  desc: number;
  count: number;
}

export interface DuelChainCountEvent {
  event: 'duel:chained' | 'duel:chain_solving' | 'duel:chain_solved' | 'duel:chain_negated';
  count: number;
}

// ---- 波 J：单人模式（single_mode.go / libdebug 调试消息）----

/** MSG_AI_NAME — 单机谜题的 AI 对手名（引擎 seat 1） */
export interface DuelAINameEvent {
  event: 'duel:ai_name';
  name: string;
}

/** MSG_SHOW_HINT — 谜题脚本 Debug.ShowHint 的提示文本 */
export interface DuelShowHintEvent {
  event: 'duel:show_hint';
  text: string;
}

/** MSG_RELOAD_FIELD — 完整布场快照（query_field_info；谜题布场/中途加入） */
export interface DuelReloadFieldEvent {
  event: 'duel:reload_field';
  rule: number;
  players: {
    lp: number;
    mzone: ({ pos: number; overlay: number } | null)[];
    szone: ({ pos: number } | null)[];
    deck: number;
    hand: number;
    grave: number;
    removed: number;
    extra: number;
    extraP: number;
  }[];
  chainCount: number;
}

// 所有 duel:* 事件的联合
export type DuelEvent =
  | DuelStartEvent
  | DuelNewTurnEvent
  | DuelNewPhaseEvent
  | DuelMoveEvent
  | DuelPosChangeEvent
  | DuelSetEvent
  | DuelSummoningEvent
  | DuelDrawEvent
  | DuelDamageEvent
  | DuelLPUpdateEvent
  | DuelWinEvent
  | DuelTossCoinEvent
  | DuelRPSEvent
  | DuelHandResEvent
  | DuelConfirmDeckTopEvent
  | DuelAttackEvent
  | DuelBattleEvent
  | DuelBecomeTargetEvent
  | DuelSelectCardEvent
  | DuelSelectUnselectEvent
  | DuelSelectChainEvent
  | DuelSelectIdleCmdEvent
  | DuelSelectBattleCmdEvent
  | DuelUpdateDataEvent
  | DuelUpdateCardEvent
  | DuelSelectEffectYNEvent
  | DuelSelectYesNoEvent
  | DuelSelectOptionEvent
  | DuelSelectPositionEvent
  | DuelSelectPlaceEvent
  | DuelSelectCounterEvent
  | DuelSelectSumEvent
  | DuelSortCardEvent
  | DuelAnnounceRaceEvent
  | DuelAnnounceCardEvent
  | DuelAnnounceNumberEvent
  | DuelChainingEvent
  | DuelChainCountEvent
  | DuelConfirmCardsEvent
  | DuelShuffleHandEvent
  | DuelShuffleExtraEvent
  | DuelRefreshDeckEvent
  | DuelShuffleSetCardEvent
  | DuelDeckTopEvent
  | DuelSwapEvent
  | DuelTagSwapEvent
  | DuelFieldDisabledEvent
  | DuelCardSelectedEvent
  | DuelEquipEvent
  | DuelUnequipEvent
  | DuelCounterChangeEvent
  | DuelMissedEffectEvent
  | DuelCardHintEvent
  | DuelPlayerHintEvent
  | DuelHintEvent
  | DuelMatchKillEvent
  | DuelAINameEvent
  | DuelShowHintEvent
  | DuelReloadFieldEvent
  | { event: DuelEmptyEvent };

// ---- stoc:* 协议事件（duel_client.go handleSTOCPacket）----

/** STOC_JOIN_GAME — Go emit 的是 STOCJoinGame 结构体（协议/types.go，Go 字段名直传） */
export interface StocJoinGameEvent {
  event: 'stoc:join_game';
  Info: {
    LFList: number;
    Rule: number;
    Mode: number;
    DuelRule: number;
    NoCheckDeck: number;
    NoShuffleDeck: number;
    StartLp: number;
    StartHand: number;
    DrawCount: number;
    TimeLimit: number;
  };
}

export interface StocTypeChangeEvent {
  event: 'stoc:type_change';
  type: number;
  isHost: boolean;
  pos: number;
}

export interface StocPlayerEnterEvent {
  event: 'stoc:player_enter';
  pos: number;
  name: string;
}

export interface StocPlayerChangeEvent {
  event: 'stoc:player_change';
  pos: number;
  status: number;
  ready: boolean;
}

export interface StocWatchChangeEvent {
  event: 'stoc:watch_change';
  count: number;
}

export interface StocDeckCountEvent {
  event: 'stoc:deck_count';
  /** 服务器已按接收方玩家交换过前后半，可直接按"自己在前"消费 */
  deck0: number;
  extra0: number;
  side0: number;
  deck1: number;
  extra1: number;
  side1: number;
}

export interface StocHandResultEvent {
  event: 'stoc:hand_result';
  res1: number;
  res2: number;
}

export interface StocTimeLimitEvent {
  event: 'stoc:time_limit';
  player: number;
  leftTime: number;
}

export interface StocChatEvent {
  event: 'stoc:chat';
  player: number;
  msg: string;
}

export interface StocErrorMsgEvent {
  event: 'stoc:error_msg';
  msg: number;
  code: number;
}

export interface StocReplayEvent {
  event: 'stoc:replay';
  /** 按 gframe 语义从 StartTime/Seed 推导的建议文件名 */
  name: string;
  size: number;
}

// 无载荷 STOC 事件
export type StocEmptyEvent =
  | 'stoc:duel_start'
  | 'stoc:select_hand'
  | 'stoc:select_tp'
  | 'stoc:duel_end'
  | 'stoc:change_side'
  | 'stoc:waiting_side'
  /** 组队赛队友请求投降（duelclient.cpp:947-952） */
  | 'stoc:teammate_surrender'
  | 'stoc:disconnected';

export type StocEvent =
  | StocJoinGameEvent
  | StocTypeChangeEvent
  | StocPlayerEnterEvent
  | StocPlayerChangeEvent
  | StocWatchChangeEvent
  | StocDeckCountEvent
  | StocHandResultEvent
  | StocTimeLimitEvent
  | StocChatEvent
  | StocErrorMsgEvent
  | StocReplayEvent
  | { event: StocEmptyEvent };

/** 前端事件总线上的全部事件 */
export type GameEvent = DuelEvent | StocEvent;
