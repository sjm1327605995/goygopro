/**
 * applyEvent — 唯一把 duel:* / stoc:* 事件翻译成 UI 状态的地方。
 *
 * 纯函数：输入 (state, 事件名, 载荷)，输出新的 state（无变化时返回原引用）。
 * 不 import DOM/Three/事件总线；事件总线→store 的接线在 duel/store.js。
 *
 * 座标系约定（本文件的关键不变量）：
 *   - 事件载荷里的 player/c/l 等都是**引擎 seat 编号**（0/1，MSG_START 的
 *     playertype 即本方 seat）。
 *   - store 内部全部使用**显示座标**：0 = 本方（屏幕下侧），1 = 对方。
 *     任何 seat → 显示座的换算只允许经过 localSeat() 这一个函数。
 *   - LP 差值事件（damage/recover/pay_lpcost）按显示座累加，绝对值事件
 *     （lp_update）直接覆盖；同一批次内事件的先后顺序由事件总线保证，
 *     reducer 不做跨事件缓冲。
 */
import type { CardQuery, GameEvent } from './events.ts';
import {
  LOC_DECK, LOC_HAND, LOC_MZONE, LOC_SZONE, LOC_GRAVE, LOC_BANISH, LOC_EXTRA, LOC_OVERLAY,
  LOC_LABELS, CARD_QUESTION, PHINT_DESC_ADD,
} from './constants.ts';
import { cardName } from './card_names.ts';

export interface HandCard {
  code: number;
}

/** 场上一张卡的位置状态（ocgcore POS_* 位） */
export interface BoardCard {
  code: number;
  pos: number;
}

/** 一个显示座的场地区（只追踪有 mesh 的区；deck/extra 走 piles 计数） */
export interface BoardSide {
  mzone: (BoardCard | null)[];
  szone: (BoardCard | null)[];
  grave: BoardCard[];
  banish: BoardCard[];
  /**
   * 波 F 快捷键查看用：额外卡组出现过的卡（move 事件驱动，与原版
   * ClientField.extra 同样只含已揭示的卡）；overlay[s] 是叠在
   * mzone[s] 上的超量素材（LOC_OVERLAY 进出）。
   */
  extra: BoardCard[];
  overlay: number[][];
}

export function emptyBoardSide(): BoardSide {
  return {
    mzone: [null, null, null, null, null, null, null],
    szone: [null, null, null, null, null, null, null, null],
    grave: [],
    banish: [],
    extra: [],
    overlay: [[], [], [], [], [], [], []],
  };
}

export interface PileCounts {
  deck: number;
  grave: number;
  banish: number;
  extra: number;
}

export interface LogEntry {
  id: number;
  text: string;
  cls: string;
}

export interface CardInspection {
  code: number;
  /** 异步 getCard 完成后回填的卡信息； undefined = 仍在加载 */
  info?: Record<string, unknown> | null;
}

export interface PhasePrompt {
  /** 'idle' = MSG_SELECT_IDLECMD（BP/EP），'battle' = MSG_SELECT_BATTLECMD（M2/EP） */
  mode: 'idle' | 'battle';
  canBP: boolean;
  canM2: boolean;
  canEP: boolean;
}

export interface DuelState {
  started: boolean;
  /** 本方在引擎里的 seat（0=先攻方）；所有 seat→显示换算的锚点 */
  playerSlot: number;
  /** [本方, 对方] 名字；练习模式没有 player_enter 时用占位名 */
  names: [string, string];
  lp: [number, number];
  turn: number;
  /** 显示座：0=本方回合 */
  turnPlayer: number;
  /** PHASE_* 位标志（constants.ts） */
  phase: number;
  /** 引擎当前的阶段按钮可用性（select_idlecmd/battlecmd 带来，新阶段清除） */
  phasePrompt: PhasePrompt | null;
  /** 本方手牌（显示座 0）；按序号索引 */
  hand: (HandCard | null)[];
  /** [本方, 对方] 卡组/墓地/除外/额外计数 */
  piles: [PileCounts, PileCounts];
  /** [本方, 对方] 剩余秒数；null = 没有计时 */
  timer: [number | null, number | null];
  /** 每回合计时时限（秒；0 = 未知）。原版 dInfo.time_limit（时限条的归一基准） */
  timeLimit: number;
  /** 初始基本分（LP 条归一基准；原版 dInfo.start_lp） */
  startLP: number;
  /** 观战者视角（MSG_START playertype 高 4 位非 0，duelclient.cpp:1269-1270） */
  isObserver: boolean;
  /** 墓地禁查（CARD_QUESTION 玩家提示，duelclient.cpp:3757-3768） */
  cantCheckGrave: boolean;
  win: { winner: number; type: number } | null;
  /** MSG_MATCH_KILL 展示卡（胜利画面用）；null = 无 */
  matchKill: number | null;
  /** MSG_FIELD_DISABLED 的禁用区位域（原样透传，field 渲染消费） */
  fieldDisabled: number;
  /** 指示物：键 `c:l:s:type` → 当前数量（add/remove_counter 维护） */
  counters: Record<string, number>;
  /** 左侧预览面板当前查看的卡（UI 动作，非引擎事件） */
  inspection: CardInspection | null;
  log: LogEntry[];
  /** [本方, 对方] 场地区快照（field3d 重同步与回放校验的依据） */
  board: [BoardSide, BoardSide];
  /** stHintMsg 提示条文本（duel:waiting 设置，任何提示/新回合清除）；null = 隐藏 */
  hint: string | null;
  /** 2D 动作弹窗（手牌/场地点击菜单）；null = 隐藏 */
  actionPopup: { x: number; y: number; actions: PopupAction[] } | null;
}

export interface PopupAction {
  label: string;
  action: string;
  code?: number;
  s?: number;
}

export function initialDuelState(): DuelState {
  return {
    started: false,
    playerSlot: 0,
    names: ['YugiMuto', 'SetoKaiba'],
    lp: [8000, 8000],
    turn: 0,
    turnPlayer: 0,
    phase: 0,
    phasePrompt: null,
    hand: [],
    piles: [
      { deck: 0, grave: 0, banish: 0, extra: 0 },
      { deck: 0, grave: 0, banish: 0, extra: 0 },
    ],
    timer: [null, null],
    timeLimit: 0,
    startLP: 8000,
    isObserver: false,
    cantCheckGrave: false,
    win: null,
    matchKill: null,
    fieldDisabled: 0,
    counters: {},
    inspection: null,
    log: [],
    board: [emptyBoardSide(), emptyBoardSide()],
    hint: null,
    actionPopup: null,
  };
}

const localSeat = (state: DuelState, seat: number): 0 | 1 =>
  seat === state.playerSlot ? 0 : 1;

const PILE_OF_LOC: Record<number, keyof PileCounts | undefined> = {
  [LOC_DECK]: 'deck',
  [LOC_GRAVE]: 'grave',
  [LOC_BANISH]: 'banish',
  [LOC_EXTRA]: 'extra',
};

let logSeq = 0;
function withLog(state: DuelState, text: string, cls = 'log-action'): DuelState {
  const entry: LogEntry = { id: ++logSeq, text, cls };
  return { ...state, log: [...state.log, entry].slice(-200) };
}

/** 阶段码 → 中文名（原版 phase_labels；hud.js 的表收编于此）。
 *  键值必须是 ocgcore PHASE_*：0x10=战斗步骤、0x20=伤害步骤、
 *  0x40=伤害计算、0x80=主要阶段 2、0x100=结束阶段 */
const PHASE_LABELS: Record<number, string> = {
  0x01: '抽卡阶段',
  0x02: '准备阶段',
  0x04: '主要阶段 1',
  0x08: '战斗阶段',
  0x10: '战斗步骤',
  0x20: '伤害步骤',
  0x40: '伤害计算',
  0x80: '主要阶段 2',
  0x100: '结束阶段',
};

function setLP(state: DuelState, seat: number, lp: number): DuelState {
  const disp = localSeat(state, seat);
  const lp2: [number, number] = [...state.lp];
  lp2[disp] = Math.max(0, lp);
  return { ...state, lp: lp2 };
}

function adjustPile(
  state: DuelState, seat: number, loc: number, delta: number,
): DuelState {
  const key = PILE_OF_LOC[loc];
  if (!key) return state;
  const disp = localSeat(state, seat);
  const piles: [PileCounts, PileCounts] = [...state.piles];
  piles[disp] = { ...piles[disp], [key]: Math.max(0, piles[disp][key] + delta) };
  return { ...state, piles };
}

/** 把手牌区域事件同步进 store（仅本方；对方手牌不可见） */
function syncHandOnMove(state: DuelState, ev: {
  code: number; pc: number; pl: number; ps: number; cc: number; cl: number; cs: number;
}): DuelState {
  const isMine = (seat: number) => seat === state.playerSlot;
  let hand = state.hand;
  if (isMine(ev.pc) && ev.pl === LOC_HAND) {
    // 离开手牌：按序号删；序号对不上时退化为删尾（防止幽灵卡残留）
    hand = hand.filter((_, i) => i !== ev.ps);
    if (hand.length === state.hand.length) hand = hand.slice(0, -1);
  }
  if (isMine(ev.cc) && ev.cl === LOC_HAND) {
    if (hand === state.hand) hand = [...hand];
    hand[ev.cs] = { code: ev.code };
  }
  return hand === state.hand ? state : { ...state, hand };
}

// ---- board 区同步（唯一事实： summoning/set/move/pos_change 四类事件） ----

function cloneBoard(state: DuelState): [BoardSide, BoardSide] {
  const deepSide = (side: BoardSide): BoardSide => ({
    ...side,
    extra: [...side.extra],
    overlay: side.overlay.map((mats) => [...mats]),
  });
  return [deepSide(state.board[0]), deepSide(state.board[1])];
}

function zoneOf(side: BoardSide, loc: number): (BoardCard | null)[] | null {
  if (loc === LOC_MZONE) return side.mzone;
  if (loc === LOC_SZONE) return side.szone;
  return null;
}

/** 场上放置（召唤/盖卡类事件）：cc/ccs/cl 已定，pos 直接记录 */
function boardPlace(state: DuelState, seat: number, loc: number, seq: number, card: BoardCard): DuelState {
  const board = cloneBoard(state);
  const zone = zoneOf(board[localSeat(state, seat)], loc);
  if (!zone || seq < 0 || seq >= zone.length) return state;
  zone[seq] = card;
  return { ...state, board };
}

/** pos_change：只改表示形式 */
function boardReposition(state: DuelState, seat: number, loc: number, seq: number, pos: number): DuelState {
  const board = cloneBoard(state);
  const side = board[localSeat(state, seat)];
  const zone = zoneOf(side, loc);
  if (!zone || !zone[seq]) return state;
  zone[seq] = { ...zone[seq]!, pos };
  return { ...state, board };
}

/** move：源区拿掉、目的区放下（grave/banish/extra 追加；deck/hand 不落 board） */
function boardMove(state: DuelState, ev: {
  code: number; pc: number; pl: number; ps: number; cc: number; cl: number; cs: number; cp: number;
}): DuelState {
  const board = cloneBoard(state);
  const dispOf = (seat: number) => localSeat(state, seat);

  // 堆区的公共进出：grave/banish/extra 同一语义（extra 列表供 F3/F7 查看）
  const stackOf = (side: BoardSide, loc: number): BoardCard[] | null => {
    if (loc === LOC_GRAVE) return side.grave;
    if (loc === LOC_BANISH) return side.banish;
    if (loc === LOC_EXTRA) return side.extra;
    return null;
  };

  const srcSide = board[dispOf(ev.pc)];
  const srcZone = zoneOf(srcSide, ev.pl);
  if (srcZone) {
    if (ev.ps >= 0 && ev.ps < srcZone.length) srcZone[ev.ps] = null;
  } else {
    const srcStack = stackOf(srcSide, ev.pl);
    if (srcStack) {
      // 堆区离开：序号对得上按序号删，否则按卡密码找，再不行删尾兜底
      if (ev.ps >= 0 && ev.ps < srcStack.length && srcStack[ev.ps].code === ev.code) {
        srcStack.splice(ev.ps, 1);
      } else {
        const i = srcStack.findIndex((c) => c.code === ev.code);
        if (i > -1) srcStack.splice(i, 1);
        else if (srcStack.length) srcStack.splice(-1, 1);
      }
    } else if (ev.pl === LOC_OVERLAY) {
      // 超量素材离体：ps 是它吸附的 mzone 槽位
      const mats = srcSide.overlay[ev.ps];
      if (mats) {
        const i = mats.indexOf(ev.code);
        if (i > -1) mats.splice(i, 1);
      }
    }
  }

  const dstSide = board[dispOf(ev.cc)];
  const dstZone = zoneOf(dstSide, ev.cl);
  if (dstZone) {
    if (ev.cs >= 0 && ev.cs < dstZone.length) dstZone[ev.cs] = { code: ev.code, pos: ev.cp };
  } else {
    const dstStack = stackOf(dstSide, ev.cl);
    if (dstStack) {
      dstStack.push({ code: ev.code, pos: ev.cp });
    } else if (ev.cl === LOC_OVERLAY) {
      // 超量素材吸附：cs 是目标 mzone 槽位
      const mats = dstSide.overlay[ev.cs];
      if (mats) mats.push(ev.code);
    }
  }
  return { ...state, board };
}

/** 任意提示事件到达即隐藏 stHintMsg（原版：弹窗打开时提示条让位） */
const HINT_CLEARING = new Set([
  'duel:select_idlecmd', 'duel:select_battlecmd', 'duel:select_effectyn',
  'duel:select_yesno', 'duel:select_option', 'duel:select_card',
  'duel:select_position', 'duel:select_place', 'duel:select_chain',
  'duel:select_counter', 'duel:select_sum', 'duel:sort_card',
  'duel:select_unselect', 'duel:announce_race', 'duel:announce_attrib',
  'duel:announce_card', 'duel:announce_number', 'duel:rps',
  'stoc:select_hand', 'stoc:select_tp', 'duel:new_turn', 'duel:win',
]);

function clearHint(state: DuelState): DuelState {
  return state.hint === null ? state : { ...state, hint: null };
}

/**
 * UI 动作（不是引擎事件）：左面板查看某张卡。
 * 导出为独立函数供 store 的 action 与 reducer 分发使用。
 */
export function applyInspect(state: DuelState, code: number): DuelState {
  return { ...state, inspection: { code, info: undefined } };
}

export function applyInspectInfo(
  state: DuelState, code: number, info: Record<string, unknown> | null,
): DuelState {
  if (!state.inspection || state.inspection.code !== code) return state;
  return { ...state, inspection: { code, info } };
}

// ---- 事件族 handler 表（P5-3：原 applyEvent 的 422 行 switch 按事件族拆表） ----
// 每个 handler 只收 (state, ev)；ev 已拼上 event 字段，字段语义同 events.ts。
// 同族内多个事件名可指向同一 handler（如三种召唤、增删指示物、连锁结束）。

type EventHandler = (state: DuelState, ev: any) => DuelState;

const noop: EventHandler = (state) => state;

function applySummoning(state: DuelState, ev: any): DuelState {
  const label = ev.event === 'duel:spsummoning' ? '特殊召唤'
    : ev.event === 'duel:flipsummoning' ? '反转召唤' : '通常召唤';
  let next = boardPlace(state, ev.cc, LOC_MZONE, ev.cs, { code: ev.code, pos: ev.cp });
  return withLog(next, `${label}：${cardName(ev.code)}`, 'log-summon');
}

function applyCounter(state: DuelState, ev: any): DuelState {
  const key = `${ev.c}:${ev.l}:${ev.s}:${ev.type}`;
  const counters = { ...state.counters };
  const v = (counters[key] || 0) + (ev.event === 'duel:add_counter' ? ev.count : -ev.count);
  if (v > 0) counters[key] = v;
  else delete counters[key];
  return { ...state, counters };
}

/** 决斗会话生命周期与 scope 级状态（开局/进房/终局/计时/猜拳结果） */
const lifecycleHandlers: Record<string, EventHandler> = {
  'duel:start': (state, ev) => {
    const playerSlot = (ev.playerType || 0) & 0x0f;
    let next: DuelState = {
      ...state,
      started: true,
      playerSlot,
      lp: [0, 0],
      turn: 0,
      turnPlayer: 0,
      phase: 0,
      phasePrompt: null,
      hand: [],
      piles: [
        { deck: 0, grave: 0, banish: 0, extra: 0 },
        { deck: 0, grave: 0, banish: 0, extra: 0 },
      ],
      timer: [null, null],
      timeLimit: 0,
      isObserver: ((ev.playerType || 0) & 0xf0) !== 0,
      cantCheckGrave: false,
      win: null,
      matchKill: null,
      fieldDisabled: 0,
      counters: {},
    };
    // MSG_START 的 lp0/lp1、deck0/…按引擎 seat 给值，换到显示座
    next = setLP(next, 0, ev.lp0 || 8000);
    next = setLP(next, 1, ev.lp1 || 8000);
    const seed = (seat: number, deck: number, extra: number) => {
      const disp = localSeat(next, seat);
      const piles: [PileCounts, PileCounts] = [...next.piles];
      piles[disp] = { ...piles[disp], deck: deck || 0, extra: extra || 0 };
      return { ...next, piles };
    };
    next = seed(0, ev.deck0, ev.extra0);
    next = seed(1, ev.deck1, ev.extra1);
    // LP 条的归一基准 = 自己的初始基本分（drawing.cpp:591 maxLP = start_lp）
    next = { ...next, startLP: (playerSlot === 1 ? ev.lp1 : ev.lp0) || 8000 };
    return withLog(next, '决斗开始！', 'log-action');
  },

  'stoc:player_enter': (state, ev) => {
    // STOC_HS_PLAYER_ENTER 的 pos 是引擎 seat（0/1=决斗者，7=观战者）
    if (ev.pos > 1) return state;
    const disp = localSeat(state, ev.pos);
    const names: [string, string] = [...state.names];
    names[disp] = ev.name || names[disp];
    return { ...state, names };
  },

  'stoc:deck_count': (state, ev) => {
    // 服务器已按接收方交换前后半（自己在前）：初始化双方卡组/额外计数。
    // 仅在决斗未真正开始（piles 还是 0）时生效，避免覆盖 MSG_START 的种子值。
    if (state.started) return state;
    const piles: [PileCounts, PileCounts] = [...state.piles];
    piles[0] = { ...piles[0], deck: ev.deck0 || 0, extra: ev.extra0 || 0 };
    piles[1] = { ...piles[1], deck: ev.deck1 || 0, extra: ev.extra1 || 0 };
    return { ...state, piles };
  },

  'stoc:hand_result': (state, ev) => {
    // STOC_HAND_RESULT：双方猜拳结果（0=剪刀 1=石头 2=布）
    const names = ['剪刀', '石头', '布'];
    const mine = names[ev.res1] ?? String(ev.res1);
    const theirs = names[ev.res2] ?? String(ev.res2);
    return withLog(state, `猜拳结果：你出了${mine}，对方出了${theirs}。`);
  },

  // 三局两胜局间，等待对方调整副卡组（sysString 1409）
  'stoc:waiting_side': (state) => ({ ...state, hint: '等待更换副卡组中...' }),

  'stoc:duel_end': (state) => withLog(state, '决斗结束。'),

  'stoc:time_limit': (state, ev) => {
    // 剩余秒 + 时限条归一基准（dInfo.time_limit 来自建房 host_info；
    // 首个 time_limit 包即满额值，取历史最大防御乱序）
    const disp = localSeat(state, ev.player);
    const timer: [number | null, number | null] = [...state.timer];
    timer[disp] = ev.leftTime;
    return { ...state, timer, timeLimit: Math.max(state.timeLimit, ev.leftTime || 0) };
  },

  'duel:win': (state, ev) => withLog(
    { ...state, win: { winner: localSeat(state, ev.winner), type: ev.type } },
    ev.winner === state.playerSlot ? '你赢了！' : '对方获胜。',
    'log-damage',
  ),
};

/** LP 变化（差值事件按显示座累加，绝对值事件覆盖） */
const lpHandlers: Record<string, EventHandler> = {
  'duel:lp_update': (state, ev) => setLP(state, ev.player, ev.lp),

  'duel:damage': (state, ev) => withLog(
    setLP(state, ev.player, state.lp[localSeat(state, ev.player)] - ev.amount),
    `玩家 ${localSeat(state, ev.player) === 0 ? '你' : '对方'} 受到 ${ev.amount} 点伤害！`,
    'log-damage',
  ),

  'duel:recover': (state, ev) => withLog(
    setLP(state, ev.player, state.lp[localSeat(state, ev.player)] + ev.amount),
    `回复了 ${ev.amount} 点 LP！`,
  ),

  'duel:pay_lpcost': (state, ev) => withLog(
    setLP(state, ev.player, state.lp[localSeat(state, ev.player)] - ev.amount),
    `支付了 ${ev.amount} 点 LP 作为代价。`,
    'log-damage',
  ),
};

/** 场地区同步（唯一事实：summoning/set/move/pos_change/update_* 等） */
const boardHandlers: Record<string, EventHandler> = {
  'duel:summoning': applySummoning,
  'duel:spsummoning': applySummoning,
  'duel:flipsummoning': applySummoning,

  'duel:set': (state, ev) => {
    const next = boardPlace(state, ev.cc, ev.cl, ev.cs, { code: ev.code, pos: ev.cp });
    return withLog(next, `玩家 ${ev.cc} 盖下了一张卡。`);
  },

  'duel:pos_change': (state, ev) => withLog(
    boardReposition(state, ev.cc, ev.cl, ev.cs, ev.cp),
    `玩家 ${ev.cc} 改变了槽位 ${ev.cs} 卡片的表示形式。`,
  ),

  'duel:move': (state, ev) => {
    const locLabel = (l: number) => (LOC_LABELS as Record<number, string>)[l] || String(l);
    let next = syncHandOnMove(state, ev);
    next = adjustPile(next, ev.pc, ev.pl, -1);
    next = adjustPile(next, ev.cc, ev.cl, +1);
    next = boardMove(next, ev);
    return withLog(next, `卡牌移动（${locLabel(ev.pl)} → ${locLabel(ev.cl)}）。`);
  },

  'duel:shuffle_set_card': (state, ev) => {
    // 同区盖卡互相换位：按服务端给的新排列重建该区条目
    let next = state;
    for (const e of ev.cards || []) {
      const board = cloneBoard(next);
      const zone = zoneOf(board[localSeat(next, e.c)], e.l);
      if (!zone || e.s < 0 || e.s >= zone.length) continue;
      zone[e.s] = { code: e.code, pos: e.p || 0 };
      next = { ...next, board };
    }
    return next === state ? state : withLog(next, '盖卡互相换位。');
  },

  'duel:swap': (state, ev) => {
    // MSG_SWAP（精神操作类控制权交换）：两张场上卡互换槽位
    const board = cloneBoard(state);
    const zone1 = zoneOf(board[localSeat(state, ev.cc1)], ev.cl1);
    const zone2 = zoneOf(board[localSeat(state, ev.cc2)], ev.cl2);
    if (!zone1 || !zone2 || !zone1[ev.cs1] || !zone2[ev.cs2]) return state;
    const tmp = zone1[ev.cs1];
    zone1[ev.cs1] = { code: ev.code2, pos: zone2[ev.cs2]!.pos };
    zone2[ev.cs2] = { code: ev.code1, pos: tmp!.pos };
    return withLog({ ...state, board }, '卡片控制权被交换了。');
  },

  'duel:field_disabled': (state, ev) => ({ ...state, fieldDisabled: ev.zones }),

  'duel:update_data': (state, ev) => {
    // MSG_UPDATE_DATA：cards[i] 按序对应 location 区的第 i 张卡
    if (!ev.cards || !ev.cards.length) return state;
    // 本方手牌的整表刷新：谜题脚本布的手牌没有 draw 事件，只能从这里来
    // （在线路径服务器同样会对本方发 HAND 的 update_data，覆盖即权威）
    if (ev.location === LOC_HAND && ev.player === state.playerSlot) {
      const hand = ev.cards.map((q: CardQuery | null) => (q && q.code ? { code: q.code } : null));
      return { ...state, hand };
    }
    const board = cloneBoard(state);
    const zone = zoneOf(board[localSeat(state, ev.player)], ev.location);
    if (!zone) return state;
    let changed = false;
    (ev.cards as (CardQuery | null)[]).forEach((q, seq) => {
      if (seq >= zone.length) return;
      const cur = zone[seq];
      if (!q) {
        // LEN_EMPTY 空槽标记：引擎说这个槽没卡
        if (cur) {
          zone[seq] = null;
          changed = true;
        }
        return;
      }
      const card: BoardCard = {
        code: q.code ?? cur?.code ?? 0,
        pos: q.position?.p ?? cur?.pos ?? 0,
      };
      if (!cur || cur.code !== card.code || cur.pos !== card.pos) {
        zone[seq] = card;
        changed = true;
      }
    });
    return changed ? { ...state, board } : state;
  },

  'duel:update_card': (state, ev) => {
    // MSG_UPDATE_CARD：只带变化字段的 query blob；缺的字段沿用旧值
    const board = cloneBoard(state);
    const zone = zoneOf(board[localSeat(state, ev.player)], ev.location);
    if (!zone || ev.sequence < 0 || ev.sequence >= zone.length) return state;
    const cur = zone[ev.sequence];
    const card: BoardCard = {
      code: ev.code ?? cur?.code ?? 0,
      pos: ev.position?.p ?? cur?.pos ?? 0,
    };
    if (cur && cur.code === card.code && cur.pos === card.pos) return state;
    zone[ev.sequence] = card;
    return { ...state, board };
  },
};

/** 指示物计数（add/remove_counter；键 c:l:s:type） */
const counterHandlers: Record<string, EventHandler> = {
  'duel:add_counter': applyCounter,
  'duel:remove_counter': applyCounter,
};

/** 阶段/回合/阶段按钮提示（phasePrompt 生命周期） */
const phaseHandlers: Record<string, EventHandler> = {
  'duel:select_idlecmd': (state, ev) => ({
    ...state,
    phasePrompt: { mode: 'idle', canBP: !!ev.toBP, canM2: false, canEP: !!ev.toEP },
  }),

  'duel:select_battlecmd': (state, ev) => ({
    ...state,
    phasePrompt: { mode: 'battle', canBP: false, canM2: !!ev.toM2, canEP: !!ev.toEP },
  }),

  'duel:new_turn': (state, ev) => withLog({
    ...state,
    turn: state.turn + 1,
    turnPlayer: localSeat(state, ev.player),
    // 新回合意味着上一条阶段按钮提示已作废
    phasePrompt: null,
  }, `—— 玩家 ${localSeat(state, ev.player) === 0 ? '你' : '对方'} 的回合 ——`, 'log-action'),

  'duel:new_phase': (state, ev) => withLog(
    { ...state, phase: ev.phase, phasePrompt: null },
    `进入${PHASE_LABELS[ev.phase] || '未知阶段'}`,
    'log-action',
  ),
};

/** 单人模式（谜题脚本 / AI 与提示类事件） */
const singleModeHandlers: Record<string, EventHandler> = {
  'duel:ai_name': (state, ev) => {
    // MSG_AI_NAME：单机谜题的 AI 对手名（引擎 seat 1，playerSlot 恒 0）
    const names: [string, string] = [...state.names];
    names[localSeat(state, 1)] = ev.name || names[localSeat(state, 1)];
    return { ...state, names };
  },

  // MSG_SHOW_HINT：谜题脚本的提示文本（原版弹提示窗，这里走提示条 + 日志）
  'duel:show_hint': (state, ev) => withLog({ ...state, hint: ev.text }, ev.text),

  'duel:reload_field': (state, ev) => {
    // MSG_RELOAD_FIELD：完整布场快照。先落 LP 与堆计数；场上卡的细节
    // 由紧随其后的 update_data 刷新（SinglePlayRefresh）补齐。
    let next = state;
    for (const seat of [0, 1] as const) {
      const p = ev.players && ev.players[seat];
      if (!p) continue;
      next = setLP(next, seat, p.lp);
      const disp = localSeat(next, seat);
      const piles: [PileCounts, PileCounts] = [...next.piles];
      piles[disp] = { ...piles[disp], deck: p.deck, grave: p.grave, banish: p.removed, extra: p.extra };
      next = { ...next, piles };
    }
    return next;
  },

  // 原版 SysString 1390（MSG_WAITING → stHintMsg）
  'duel:waiting': (state) => ({ ...state, hint: '等待行动中...' }),

  'duel:player_hint': (state, ev) => {
    // MSG_PLAYER_HINT（duelclient.cpp:3757-3768）：CARD_QUESTION 加/删到
    // 本方 → 全场墓地禁查（drawing.cpp:564-575 在双方墓地上叠禁查图标）。
    if (ev.data === CARD_QUESTION && ev.player === state.playerSlot) {
      return { ...state, cantCheckGrave: ev.type === PHINT_DESC_ADD };
    }
    return state;
  },

  // 效果角标是 3D 浮层素材，reducer 不消费
  'duel:card_hint': noop,
};

/** 抽卡与卡信息揭示（draw/confirm/shuffle/deck_top/选中/装备/对象） */
const cardHandlers: Record<string, EventHandler> = {
  'duel:draw': (state, ev) => {
    let next = adjustPile(state, ev.player, LOC_DECK, -ev.count);
    if (ev.player === next.playerSlot && ev.cards && ev.cards.length) {
      const hand = [...next.hand];
      for (const code of ev.cards) {
        if (code) hand.push({ code });
      }
      next = { ...next, hand };
    }
    return { ...next, log: [...next.log, { id: ++logSeq, text: `玩家 ${ev.player} 抽了 ${ev.count} 张卡。`, cls: '' }].slice(-200) };
  },

  'duel:confirm_decktop': (state, ev) => withLog(
    state,
    `玩家 ${ev.player} 卡组顶端：${(ev.cards || []).map((c: { code: number }) => cardName(c.code)).join('、')}`,
  ),

  'duel:confirm_cards': (state, ev) => {
    const names = (ev.cards || []).map((c: { code: number }) => cardName(c.code)).join('、');
    return withLog(state, `翻开确认：${names}`);
  },

  'duel:shuffle_hand': (state, ev) => {
    // 对方收到的 code 已被服务端归零，只有本方能按新序重建手牌
    if (ev.player === state.playerSlot && ev.cards) {
      const hand = ev.cards.filter((code: number) => code).map((code: number) => ({ code }));
      return withLog({ ...state, hand }, '手牌重新洗切。');
    }
    return withLog(state, '对方洗切了手牌。');
  },

  'duel:shuffle_extra': (state) => withLog(state, '额外卡组重新洗切。'),

  'duel:refresh_deck': (state) => withLog(state, '卡组被洗切。'),

  'duel:deck_top': (state, ev) => {
    // 0x80000000 位 = 卡组顶朝向反转标记，卡号取低 31 位
    const code = ev.code & 0x7fffffff;
    const reversed = ev.code >= 0x80000000;
    return withLog(state, `玩家 ${ev.player} 卡组顶端${reversed ? '（反转）' : ''}：${cardName(code)}`);
  },

  'duel:card_selected': (state, ev) => withLog(state, `有 ${(ev.cards || []).length} 张卡被选中。`),
  'duel:random_selected': (state, ev) => withLog(state, `有 ${(ev.cards || []).length} 张卡被选中。`),

  'duel:equip': (state) => withLog(state, '一张卡装备到了对象卡上。'),

  'duel:card_target': (state) => withLog(state, '一张卡成为了对象。'),

  // 目标线清理，3D 层消费，无日志
  'duel:cancel_target': noop,

  'duel:unequip': (state) => withLog(state, '装备关系被解除。'),
};

/** 连锁/战斗/掷币/无效等纯日志与视觉事件 */
const logHandlers: Record<string, EventHandler> = {
  'duel:chaining': (state, ev) => withLog(state, `连锁发动：${cardName(ev.code)}！`, 'log-action'),

  // 连锁环视觉事件，3D 层消费，无日志
  'duel:chain_solving': noop,
  'duel:chain_solved': noop,
  'duel:chain_end': noop,

  'duel:attack': (state, ev) => withLog(state, `宣告攻击！槽位 ${ev.attacker.s} → 槽位 ${ev.target.s}`, 'log-damage'),

  'duel:hand_res': (state, ev) => {
    // 出拳值 1=石头 2=剪刀 3=布（gframe f1/f2/f3 按钮语义）
    const handName = (v: number) => ['石头', '剪刀', '布'][v - 1] || String(v);
    return withLog(state, `猜拳结果已确定（${handName(ev.res & 3)} vs ${handName((ev.res >> 2) & 3)}）。`);
  },

  'duel:toss_coin': (state, ev) => {
    const faces = (ev.results || []).map((r: number) => (r ? '正' : '反')).join('、');
    return withLog(state, `玩家 ${ev.player} 掷硬币：${faces}`);
  },

  'duel:toss_dice': (state, ev) => {
    const faces = (ev.results || []).map((r: number) => r + 1).join('、');
    return withLog(state, `玩家 ${ev.player} 掷骰子：${faces}`);
  },

  'duel:attack_disabled': (state) => withLog(state, '攻击被无效了！', 'log-damage'),

  'duel:chain_negated': (state, ev) => withLog(state, `连锁 ${ev.count} 被无效。`, 'log-damage'),

  'duel:become_target': (state, ev) => withLog(state, `${(ev.targets || []).length} 张卡被选为对象。`),

  'duel:missed_effect': (state, ev) =>
    // 原版 SysString 1622「错过时点」
    withLog(state, `${cardName(ev.code)} 错过了发动时点。`, 'log-damage'),

  'duel:match_kill': (state, ev) => withLog(
    { ...state, matchKill: ev.code },
    `比赛击杀！${cardName(ev.code)}`,
    'log-damage',
  ),

  // 26 字节结算体由 3D 层/攻守浮层直接消费，reducer 不落盘
  'duel:battle': noop,
};

/** 合并所有事件族进一张查表（事件名在 events.ts 中全局唯一） */
const HANDLERS: Record<string, EventHandler> = {
  ...lifecycleHandlers,
  ...lpHandlers,
  ...boardHandlers,
  ...counterHandlers,
  ...phaseHandlers,
  ...singleModeHandlers,
  ...cardHandlers,
  ...logHandlers,
};

export function applyEvent(state: DuelState, name: string, data: any): DuelState {
  const ev = { event: name, ...data } as GameEvent;
  if (HINT_CLEARING.has(ev.event)) state = clearHint(state);
  const handler = HANDLERS[ev.event];
  return handler ? handler(state, ev) : state;
}