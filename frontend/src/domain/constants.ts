/**
 * 领域常量 — ocgcore/协议位标志的唯一来源。
 *
 * 此前这些表在 duel_manager.js、replay_apply.js、ai_simulator.js、hud.js、
 * field3d.ts、DeckBuilder.jsx 里各抄了一份；任何一份改了其它份不会跟着变。
 * 全部取值依据 ocgcore（common.go / constant.go）与 vendored YGOPro 引擎源码，
 * 改动前先核对 source/ygopro 或 ocgcore 的对应定义。
 */

// ---- 卡片位置（ocgcore POS_*，见 ocgcore/constant.go）----
export const POS_FACEUP_ATTACK = 0x1;
export const POS_FACEDOWN_ATTACK = 0x2;
export const POS_FACEUP_DEFENSE = 0x4;
export const POS_FACEDOWN_DEFENSE = 0x8;
export const POS_FACEUP = 0x5; // FACEUP_ATTACK | FACEUP_DEFENSE
export const POS_FACEDOWN = 0xa; // FACEDOWN_ATTACK | FACEDOWN_DEFENSE

// ---- 区域（ocgcore LOCATION_*，MSG_MOVE 的 pl/cl 字段取值）----
export const LOC_DECK = 0x01;
export const LOC_HAND = 0x02;
export const LOC_MZONE = 0x04;
export const LOC_SZONE = 0x08;
export const LOC_GRAVE = 0x10;
export const LOC_BANISH = 0x20;
export const LOC_EXTRA = 0x40;
export const LOC_OVERLAY = 0x80;

export const LOC_NAMES: Record<number, string> = {
  [LOC_DECK]: 'deck',
  [LOC_HAND]: 'hand',
  [LOC_MZONE]: 'mzone',
  [LOC_SZONE]: 'szone',
  [LOC_GRAVE]: 'grave',
  [LOC_BANISH]: 'banish',
  [LOC_EXTRA]: 'extra',
  [LOC_OVERLAY]: 'overlay',
};

export const LOC_LABELS = {
  [LOC_DECK]: '卡组',
  [LOC_HAND]: '手牌',
  [LOC_MZONE]: '场上',
  [LOC_SZONE]: '场上',
  [LOC_GRAVE]: '墓地',
  [LOC_BANISH]: '除外区',
  [LOC_EXTRA]: '额外卡组',
  [LOC_OVERLAY]: '超量素材',
};

// ---- 卡片类型（ocgcore TYPE_*）----
export const TYPE_MONSTER = 0x1;
export const TYPE_SPELL = 0x2;
export const TYPE_TRAP = 0x4;
export const TYPE_NORMAL = 0x10;
export const TYPE_EFFECT = 0x20;
export const TYPE_FUSION = 0x40;
export const TYPE_RITUAL = 0x80;
export const TYPE_TRAPMONSTER = 0x100;
export const TYPE_SPIRIT = 0x200;
export const TYPE_UNION = 0x400;
export const TYPE_DUAL = 0x800;
export const TYPE_TUNER = 0x1000;
export const TYPE_SYNCHRO = 0x2000;
export const TYPE_TOKEN = 0x4000;
export const TYPE_QUICKPLAY = 0x10000;
export const TYPE_CONTINUOUS = 0x20000;
export const TYPE_EQUIP = 0x40000;
export const TYPE_FIELD = 0x80000;
export const TYPE_COUNTER = 0x100000;
export const TYPE_FLIP = 0x200000;
export const TYPE_TOON = 0x400000;
export const TYPE_XYZ = 0x800000;
export const TYPE_PENDULUM = 0x1000000;
export const TYPE_LINK = 0x4000000;

// 额外卡组类型合集（DeckBuilder 用于归类 main/extra）
export const EXTRA_TYPES =
  TYPE_FUSION | TYPE_SYNCHRO | TYPE_XYZ | TYPE_LINK;

// ---- 宣言种族（MSG_ANNOUNCE_RACE 的 available 位掩码）----
export const RACES = [
  [0x1, '战士'], [0x2, '魔法师'], [0x4, '天使'], [0x8, '恶魔'],
  [0x10, '不死'], [0x20, '机械'], [0x40, '水'], [0x80, '炎'],
  [0x100, '岩石'], [0x200, '鸟兽'], [0x400, '植物'], [0x800, '昆虫'],
  [0x1000, '雷'], [0x2000, '龙'], [0x4000, '兽'], [0x8000, '兽战士'],
  [0x10000, '恐龙'], [0x20000, '鱼'], [0x40000, '海龙'], [0x80000, '爬虫'],
  [0x100000, '念动力'], [0x200000, '幻神'], [0x400000, '创造神'],
  [0x800000, '幻龙'], [0x1000000, '电子界'], [0x2000000, '幻影'],
];

// ---- 宣言属性（MSG_ANNOUNCE_ATTRIB 的 available 位掩码）----
export const ATTRS = [
  [0x1, '地'], [0x2, '水'], [0x4, '炎'], [0x8, '风'],
  [0x10, '光'], [0x20, '暗'], [0x40, '神'],
];

// ---- 引擎 opcode（ocgcore/common.go OPCODE_*）----
// 值低于操作符区间的都是压栈字面量。
export const OPCODE_OPERATORS = new Set([
  0x40000000, 0x40000001, 0x40000002, 0x40000003, 0x40000004, 0x40000005,
  0x40000006, 0x40000007, 0x40000100, 0x40000101, 0x40000102, 0x40000103,
  0x40000104,
]);
export const OPCODE_ISCODE = 0x40000100;

// ---- 引擎阶段码（ocgcore PHASE_*，MSG_NEW_PHASE 的 phase 字段）----
export const PHASE_DRAW = 0x01;
export const PHASE_STANDBY = 0x02;
export const PHASE_MAIN1 = 0x04;
export const PHASE_BATTLE_START = 0x08;
export const PHASE_BATTLE_STEP = 0x10;
export const PHASE_DAMAGE = 0x20;
export const PHASE_DAMAGE_GLY = 0x40;
export const PHASE_BATTLE = 0x38; // START|STEP|DAMAGE|DAMAGE_GLY
export const PHASE_MAIN2 = 0x80;
export const PHASE_END = 0x100;
