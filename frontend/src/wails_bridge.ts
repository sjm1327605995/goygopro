/**
 * YGOPro Wails Bridge
 * Unifies communication between Web Frontend and Go backend.
 * Provides fallback mock data for testing in standalone browser preview.
 */

import { FORWARDED_EVENTS } from './net/events_forward.ts';
import { registerCardName } from './domain/card_names.ts';

// Sample mock card database for standalone preview & testing
const MOCK_CARD_DB = [
  {
    code: 89631139,
    name: "Blue-Eyes White Dragon",
    type: 0x11, // Monster | Normal
    attack: 3000,
    defense: 2500,
    level: 8,
    race: 0x200, // Dragon
    attribute: 0x10, // LIGHT
    desc: "This legendary dragon is a powerful engine of destruction. Virtually invincible, very few have faced this awesome creature and lived to tell the tale."
  },
  {
    code: 46986414,
    name: "Dark Magician",
    type: 0x11, // Monster | Normal
    attack: 2500,
    defense: 2100,
    level: 7,
    race: 0x2, // Spellcaster
    attribute: 0x20, // DARK
    desc: "The ultimate wizard in terms of attack and defense."
  },
  {
    code: 38033121,
    name: "Dark Magician Girl",
    type: 0x21, // Monster | Effect
    attack: 2000,
    defense: 1700,
    level: 6,
    race: 0x2, // Spellcaster
    attribute: 0x20, // DARK
    desc: "Gains 300 ATK for every 'Dark Magician' or 'Magician of Black Chaos' in either player's GY."
  },
  {
    code: 83764718,
    name: "Monster Reborn",
    type: 0x2, // Spell
    attack: 0,
    defense: 0,
    level: 0,
    race: 0,
    attribute: 0,
    desc: "Target 1 monster in either player's GY; Special Summon it."
  },
  {
    code: 55144522,
    name: "Pot of Greed",
    type: 0x2, // Spell
    attack: 0,
    defense: 0,
    level: 0,
    race: 0,
    attribute: 0,
    desc: "Draw 2 cards from your Deck."
  },
  {
    code: 44095762,
    name: "Mirror Force",
    type: 0x4, // Trap
    attack: 0,
    defense: 0,
    level: 0,
    race: 0,
    attribute: 0,
    desc: "When an opponent's monster declares an attack: Destroy all Attack Position monsters your opponent controls."
  },
  {
    code: 12580477,
    name: "Raigeki",
    type: 0x2, // Spell
    attack: 0,
    defense: 0,
    level: 0,
    race: 0,
    attribute: 0,
    desc: "Destroy all monsters your opponent controls."
  },
  {
    code: 5318639,
    name: "Mystical Space Typhoon",
    type: 0x10002, // Spell | Quick-Play
    attack: 0,
    defense: 0,
    level: 0,
    race: 0,
    attribute: 0,
    desc: "Target 1 Spell/Trap on the field; destroy that target."
  },
  {
    code: 41420027,
    name: "Solemn Judgment",
    type: 0x100004, // Trap | Counter
    attack: 0,
    defense: 0,
    level: 0,
    race: 0,
    attribute: 0,
    desc: "When a monster(s) would be Summoned, OR a Spell/Trap Card is activated: Pay half your LP; negate the Summon or activation, and if you do, destroy that card."
  },
  {
    code: 14558127,
    name: "Ash Blossom & Joyous Spring",
    type: 0x21 | 0x800, // Monster | Effect | Tuner
    attack: 0,
    defense: 1800,
    level: 3,
    race: 0x8, // Zombie
    attribute: 0x04, // FIRE
    desc: "When a card or effect is activated that includes any of these effects (Quick Effect): You can discard this card; negate that effect."
  },
  {
    code: 84013237,
    name: "Number 39: Utopia",
    type: 0x800021, // Monster | Effect | Xyz
    attack: 2500,
    defense: 2000,
    level: 4,
    race: 0x1, // Warrior
    attribute: 0x10, // LIGHT
    desc: "2 Level 4 monsters. When any player's monster declares an attack: You can detach 1 material from this card; negate the attack."
  },
  {
    code: 86066372,
    name: "Accesscode Talker",
    type: 0x4000021, // Monster | Effect | Link
    attack: 2300,
    defense: 0,
    level: 4,
    race: 0x1000000, // Cyberse
    attribute: 0x20, // DARK
    desc: "2+ Effect Monsters. If this card is Link Summoned: You can target 1 Link Monster used as material; this card gains ATK equal to that monster's Link Rating x 1000."
  }
];

type BusListener = (data: any) => void;

class EventBus {
  listeners: Record<string, BusListener[]> = {};

  on(event: string, callback: BusListener) {
    if (!this.listeners[event]) this.listeners[event] = [];
    this.listeners[event].push(callback);
  }
  off(event: string, callback: BusListener) {
    const list = this.listeners[event];
    if (!list) return;
    const idx = list.indexOf(callback);
    if (idx > -1) list.splice(idx, 1);
  }
  emit(event: string, data: any) {
    if (this.listeners[event]) {
      this.listeners[event].forEach(cb => cb(data));
    }
  }
}

export const eventBus = new EventBus();

// Check if Wails v3 runtime is injected (window._wails)
const wailsRuntime = typeof window !== 'undefined' && window._wails ? window._wails : null;
const isWails = !!(wailsRuntime && wailsRuntime.Call && wailsRuntime.Call.ByName);

// Call a bound Go service method (v3 uses fully-qualified names "App.Method")
function callWails(method: string, ...args: any[]) {
  return wailsRuntime!.Call!.ByName("App." + method, ...args);
}

const wailsEvents = wailsRuntime && wailsRuntime.Events ? wailsRuntime.Events : null;
if (wailsEvents && wailsEvents.On) {
  // Bridge Wails events to internal eventBus. 名字唯一来源见
  // net/events_forward.ts（与 Go emit 点有名字对齐测试）。
  FORWARDED_EVENTS.forEach(name => {
    wailsEvents.On(name, (ev) => eventBus.emit(name, ev && ev.data !== undefined ? ev.data : ev));
  });
}

export const WailsBridge = {
  isWails,
  _cardCache: new Map<number, any>(),
  _picCache: new Map<number, { url: string } | null>(),

  async connectServer(addr: string, username: string, pass: string) {
    if (isWails) {
      return await callWails("ConnectServer", addr, username, pass);
    }
    console.log("[MockBridge] ConnectServer:", addr, username);
    setTimeout(() => {
      eventBus.emit("stoc:type_change", { type: 0x10, isHost: true, pos: 0 });
      eventBus.emit("stoc:player_enter", { pos: 0, name: username || "Duelist" });
    }, 200);
    return { success: true };
  },

  async startLocalServer(port?: number) {
    if (isWails) {
      return await callWails("StartLocalServer", port);
    }
    return { success: true, port: port || 7911 };
  },

  // 主菜单「退出」（gframe BUTTON_MODE_EXIT）；浏览器/冒烟环境无实际效果
  async quit() {
    if (isWails) {
      await callWails("Quit");
    } else {
      console.log("[MockBridge] Quit (no-op)");
    }
  },

  // ---- 单人模式（wSinglePlay，menu_handler.cpp:400-411）----

  async listSingles(): Promise<{ name: string; message: string }[]> {
    if (isWails) {
      return (await callWails("ListSingles")) || [];
    }
    return [
      { name: '青眼一击.lua', message: '入门残局：用青眼的强大力量一击制胜！' },
      { name: '魔导师的初阵.lua', message: '用连锁完成突破。' },
    ];
  },

  // returnDeckTop = chkSinglePlayReturnDeckTop（不洗切时回卡组改为回顶端）。
  // 成功后事件流与在线对局同构（duel:start → reload_field/update_data →
  // select_* → …… → single:ended）。
  async startSingle(name: string, returnDeckTop = false): Promise<{ success: boolean; name?: string; error?: string }> {
    if (isWails) {
      return await callWails("StartSingle", name, returnDeckTop);
    }
    console.log("[MockBridge] StartSingle:", name, returnDeckTop);
    return { success: true, name };
  },

  stopSingle() {
    if (isWails) callWails("StopSingle");
    else console.log("[MockBridge] StopSingle");
  },

  async createGame(req: any, roomName: string, pass: string) {
    if (isWails) {
      return await callWails("CreateGame", req, roomName, pass);
    }
    return { success: true };
  },

  async joinGame(pass: string) {
    if (isWails) {
      return await callWails("JoinGame", pass);
    }
    return { success: true };
  },

  // 等候区身份切换（CTOS_HS_TOOBSERVER / CTOS_HS_TODUELIST）。观战者只能
  // CTOS_LEAVE_GAME 离开（single_duel.cpp:328-331），由服务端裁决。
  toObserver() {
    if (isWails) callWails("ToObserver");
    // selftype>1 即观战（gframe），type 低 4 位=座位、bit4=宿主
    else eventBus.emit("stoc:type_change", { type: 0x02, isHost: false, pos: 2 });
  },

  toDuelist() {
    if (isWails) callWails("ToDuelist");
    else eventBus.emit("stoc:type_change", { type: 0x01, isHost: false, pos: 1 });
  },

  // 宿主踢人：CTOS_HS_KICK + 座位号（netserver.cpp:355-365 仅准备阶段有效）
  kickPlayer(pos: number) {
    if (isWails) callWails("KickPlayer", pos);
    else console.log("[MockBridge] KickPlayer:", pos);
  },

  leaveGame() {
    if (isWails) callWails("LeaveGame");
  },

  surrender() {
    if (isWails) callWails("Surrender");
    else console.log("[MockBridge] Surrender");
  },

  // STOC_TIME_LIMIT 轮到自己时立即确认（gframe duelclient.cpp:783-784）
  sendTimeConfirm() {
    if (isWails) callWails("SendTimeConfirm");
    else console.log("[MockBridge] SendTimeConfirm");
  },

  // CTOS_UPDATE_DECK：mainList 需包含主卡组+额外（服务端按卡类型自动拆分，
  // 见 core/duel/deck_manager.go LoadDeck）
  updateDeck(mainCards: number[], sideCards: number[]) {
    if (isWails) callWails("UpdateDeck", mainCards, sideCards);
    else console.log("[MockBridge] UpdateDeck:", mainCards.length, sideCards.length);
  },

  setReady(ready: boolean) {
    if (isWails) callWails("SetReady", ready);
    else {
      eventBus.emit("stoc:player_change", { pos: 0, ready, status: ready ? 9 : 0 });
    }
  },

  startDuel() {
    if (isWails) callWails("StartDuel");
  },

  sendChat(msg: string) {
    if (isWails) callWails("SendChat", msg);
    else {
      eventBus.emit("stoc:chat", { player: 0, msg });
    }
  },

  sendHandResult(res: number) {
    if (isWails) callWails("SendHandResult", res);
  },

  sendTPResult(res: number) {
    if (isWails) callWails("SendTPResult", res);
  },

  sendResponseI(val: number) {
    if (isWails) callWails("SendResponseI", val);
    else console.log("[MockBridge] SendResponseI:", val);
  },

  // ---- 语义化响应：字节编码在 Go 侧（responses.go），前端只发语义参数 ----

  respondSelectCard(indices: number[]) {
    if (isWails) callWails("RespondSelectCard", indices);
    else console.log("[MockBridge] RespondSelectCard:", indices);
  },

  respondSelectUnselect(index: number) {
    if (isWails) callWails("RespondSelectUnselect", index);
    else console.log("[MockBridge] RespondSelectUnselect:", index);
  },

  respondCounter(counts: number[]) {
    if (isWails) callWails("RespondCounter", counts);
    else console.log("[MockBridge] RespondCounter:", counts);
  },

  respondSelectSum(count: number, indices: number[]) {
    if (isWails) callWails("RespondSelectSum", count, indices);
    else console.log("[MockBridge] RespondSelectSum:", count, indices);
  },

  respondSelectPlace(player: number, loc: number, seq: number) {
    if (isWails) callWails("RespondSelectPlace", player, loc, seq);
    else console.log("[MockBridge] RespondSelectPlace:", player, loc, seq);
  },

  respondSortCard(perm: number[]) {
    if (isWails) callWails("RespondSortCard", perm);
    else console.log("[MockBridge] RespondSortCard:", perm);
  },

  respondSortCardCancel() {
    if (isWails) callWails("RespondSortCardCancel");
    else console.log("[MockBridge] RespondSortCardCancel");
  },

  // (idx<<16)|cmdType 编码在 Go 侧（responses.go）。idlecmd 类型：
  // 0=summon 1=spsummon 2=repos 3=mset 4=sset 5=activate 6=toBP 7=toEP 8=shuffle
  respondIdleCmd(idx: number, cmdType: number) {
    if (isWails) callWails("RespondIdleCmd", idx, cmdType);
    else console.log("[MockBridge] RespondIdleCmd:", idx, cmdType);
  },

  // battlecmd 类型：0=activate 1=attack 2=toM2 3=toEP
  respondBattleCmd(idx: number, cmdType: number) {
    if (isWails) callWails("RespondBattleCmd", idx, cmdType);
    else console.log("[MockBridge] RespondBattleCmd:", idx, cmdType);
  },

  // Memoized card lookups: replay preload fetches every card code once, so
  // the per-event awaits during playback resolve from cache instantly.
  async getCard(code: number) {
    if (!code) return null;
    if (WailsBridge._cardCache.has(code)) return WailsBridge._cardCache.get(code);
    let info;
    if (isWails) {
      info = await callWails("GetCard", code);
    } else {
      info = MOCK_CARD_DB.find(c => c.code === code) || {
        code,
        name: "Card #" + code,
        type: 0x11,
        attack: 1500,
        defense: 1200,
        level: 4,
        race: 0x1,
        attribute: 0x10,
        desc: "Card details."
      };
    }
    WailsBridge._cardCache.set(code, info);
    // reducer 写日志要用卡名 — 谁解析到卡名谁登记（card_names.ts）
    registerCardName(code, info && info.name);
    return info;
  },

  // Card picture lookup. Returns { url } or null — the URL is a complete card
  // face (pics/<code>.jpg on disk via Go GetCardImage, or the YGOProDeck CDN
  // whenever no local art is installed), drawn full-card by every consumer.
  // Results (including misses) are cached per code.
  async getCardImage(code: number) {
    if (!code) return null;
    if (!WailsBridge._picCache) WailsBridge._picCache = new Map();
    const cached = WailsBridge._picCache.get(code);
    if (cached !== undefined) return cached;
    let result: { url: string } | null = null;
    if (isWails) {
      const data = await callWails("GetCardImage", code);
      if (data) {
        result = { url: data as string };
      }
    }
    if (!result) {
      result = await WailsBridge._fetchCdnCardImage(code);
    }
    WailsBridge._picCache.set(code, result);
    return result;
  },

  async _fetchCdnCardImage(code: number) {
    try {
      const res = await fetch(`https://images.ygoprodeck.com/images/cards_small/${code}.jpg`);
      if (!res.ok) return null;
      const blob = await res.blob();
      const url = await new Promise<string | null>((resolve) => {
        const reader = new FileReader();
        reader.onload = () => resolve(reader.result as string);
        reader.onerror = () => resolve(null);
        reader.readAsDataURL(blob);
      });
      return url ? { url } : null;
    } catch {
      return null;
    }
  },

  // 卡牌检索（cmd/wails/card_db.go CardFilter 的镜像）。Min/Max 字段不传
  // 即不过滤；level 是精确等级。Go 侧关键词语法（deck_con.cpp 翻译）：
  // 空格分隔多元素、`-` 排除、`"..."` 短语、$仅卡名 / @仅系列名；
  // atk/def/level/scaleFilter 收运算符字符串（3000 / =3000 / >=2500 / ?）；
  // effect/linkMarks 是 wCategories/wLinkMarks 汇出的位掩码。
  async searchCards(filter: {
    keyword?: string; type?: number; race?: number; attribute?: number;
    level?: number; minLevel?: number; maxLevel?: number;
    minAttack?: number; maxAttack?: number; minDefense?: number; maxDefense?: number;
    atkFilter?: string; defFilter?: string; levelFilter?: string; scaleFilter?: string;
    effect?: number; linkMarks?: number; multiKeywords?: number;
    minScale?: number; maxScale?: number;
    limit?: number; offset?: number;
  }) {
    if (isWails) {
      return await callWails("SearchCards", filter);
    }
    let res = MOCK_CARD_DB;
    // mock 版关键词：空格拆元素 + `-` 排除 + `$` 仅卡名（不支持的语法忽略）
    if (filter.keyword) {
      const elements = (filter.multiKeywords ?? 0) === 1
        ? filter.keyword.trim().split(/\s+/)
        : [filter.keyword.trim()];
      for (const raw of elements) {
        let el = raw;
        let exclude = false;
        if (el.startsWith('-')) { exclude = true; el = el.slice(1); }
        const nameOnly = el.startsWith('$');
        if (nameOnly) el = el.slice(1);
        if (!el) continue;
        const kw = el.toLowerCase();
        const match = (c: any): boolean => nameOnly
          ? c.name.toLowerCase().includes(kw)
          : c.name.toLowerCase().includes(kw) || c.desc.toLowerCase().includes(kw) || String(c.code).includes(kw);
        res = res.filter((c: any) => (exclude ? !match(c) : match(c)));
      }
    }
    if (filter.type) res = res.filter(c => (c.type & filter.type!) === filter.type);
    if (filter.race) res = res.filter(c => (c.race & filter.race!) !== 0);
    if (filter.attribute) res = res.filter(c => (c.attribute & filter.attribute!) !== 0);
    if (filter.level) res = res.filter(c => c.level === filter.level);
    if (filter.minLevel !== undefined) res = res.filter(c => c.level >= filter.minLevel!);
    if (filter.maxLevel !== undefined) res = res.filter(c => c.level <= filter.maxLevel!);
    if (filter.minAttack !== undefined) res = res.filter(c => c.attack >= filter.minAttack!);
    if (filter.maxAttack !== undefined) res = res.filter(c => c.attack <= filter.maxAttack!);
    if (filter.minDefense !== undefined) res = res.filter(c => c.defense >= filter.minDefense!);
    if (filter.maxDefense !== undefined) res = res.filter(c => c.defense <= filter.maxDefense!);
    // 运算符字符串过滤（与 Go parseStatFilter 同语义）
    const applyOp = (get: (c: any) => number, s?: string): void => {
      if (!s) return;
      const trimmed = s.trim();
      if (trimmed === '?') { res = res.filter(c => get(c) === -2); return; }
      const m = trimmed.match(/^(>=|<=|>|<|=)?\s*(-?\d+)$/);
      if (!m) return;
      const v = Number(m[2]);
      const op = m[1] ?? '=';
      res = res.filter(c => op === '=' ? get(c) === v
        : op === '>=' ? get(c) >= v
        : op === '>' ? get(c) > v
        : op === '<=' ? get(c) <= v && get(c) >= 0
        : get(c) < v && get(c) >= 0);
    };
    applyOp((c: any) => c.attack ?? 0, filter.atkFilter);
    applyOp((c: any) => c.defense ?? 0, filter.defFilter);
    applyOp((c: any) => c.level ?? 0, filter.levelFilter);
    if (filter.scaleFilter) {
      // mock 无刻度数据：非空刻度过滤直接空结果（真机走 Go）
      res = [];
    }
    if (filter.effect) res = res.filter((c: any) => ((c.category ?? 0) & filter.effect!) !== 0);
    if (filter.linkMarks) res = res.filter((c: any) =>
      (c.type & 0x4000000) !== 0 && ((c.defense ?? 0) & filter.linkMarks!) === filter.linkMarks!);
    return res.slice(0, filter.limit || 40);
  },

  // 禁限卡表下拉（app.go ListLFLists → core/duel DeckManger）。HostInfo.LFList
  // 存哈希（deck_manager.cpp 的异或折叠），不是序号。
  async listLFLists(): Promise<{ hash: number; name: string }[]> {
    if (isWails) {
      return await callWails("ListLFLists");
    }
    return [{ hash: 0, name: 'N/A' }, { hash: 0x7dfcee6a, name: 'Test List' }];
  },

  async listDecks() {
    if (isWails) {
      return await callWails("ListDecks");
    }
    // 相对路径名（'/' 分隔分类层级）；根目录 = 未分类
    return ["Blue-Eyes Beatdown", "Dark Magician Control", "Meta/Cyber Dragon OTK"];
  },

  async loadDeck(name: string) {
    if (isWails) {
      return await callWails("LoadDeck", name);
    }
    return {
      name,
      main: [
        89631139, 89631139, 89631139, 46986414, 46986414, 38033121, 38033121,
        83764718, 55144522, 12580477, 5318639, 5318639, 44095762, 44095762,
        41420027, 14558127, 14558127, 14558127
      ],
      extra: [84013237, 86066372],
      side: [41420027, 5318639]
    };
  },

  async listReplays() {
    if (isWails) {
      return await callWails("ListReplays");
    }
    return ["World Championship Finals 2026.yrp"];
  },

  async playReplay(name: string) {
    if (isWails) {
      return await callWails("PlayReplay", name);
    }
    return {
      success: true,
      name,
      players: ["YugiMuto", "SetoKaiba"],
      startLp: 8000,
      duelRule: 5,
      events: [
        { type: 'duel:start', data: { lp0: 8000, lp1: 8000 } },
        { type: 'duel:draw', data: { player: 0, count: 5, cards: [89631139, 46986414, 83764718, 55144522, 44095762] } },
        { type: 'duel:draw', data: { player: 1, count: 5, cards: [0, 0, 0, 0, 0] } },
        { type: 'duel:new_phase', data: { phase: 0x04 } },
        { type: 'duel:summoning', data: { code: 89631139, cc: 0, cl: 4, cs: 2, cp: 0x1 } },
        { type: 'duel:chaining', data: { code: 44095762, cc: 1, cl: 5, cs: 2 } },
        { type: 'duel:chain_solving', data: { count: 1 } },
        { type: 'duel:chain_solved', data: { count: 1 } },
        { type: 'duel:chain_end', data: {} },
        { type: 'duel:attack', data: { attacker: { c: 0, l: 4, s: 2 }, target: { c: 1, l: 0, s: 0 } } },
        { type: 'duel:damage', data: { player: 1, amount: 3000 } },
        { type: 'duel:move', data: { code: 44095762, pc: 1, pl: 0x08, ps: 2, pp: 0xa, cc: 1, cl: 0x10, cs: 0, cp: 0x1, reason: 0x40 } },
        { type: 'duel:win', data: { winner: 0, type: 0 } }
      ]
    };
  },

  async saveDeck(deck: { name: string; [k: string]: any }) {
    if (isWails) {
      return await callWails("SaveDeck", deck);
    }
    console.log("[MockBridge] Saved deck:", deck.name);
    return true;
  },

  async deleteDeck(name: string) {
    if (isWails) {
      return await callWails("DeleteDeck", name);
    }
    console.log("[MockBridge] Deleted deck:", name);
    return { success: true };
  },

  // ---- 回放文件管理 ----

  // 落盘最近一次 STOC_REPLAY（auto_save_replay 或确认弹窗后调用）。
  // 返回 { success, name }，name 是实际写入的文件名。
  async saveLastReplay(name: string): Promise<{ success: boolean; name?: string; error?: string }> {
    if (isWails) {
      return await callWails("SaveLastReplay", name);
    }
    return { success: true, name: (name || "_LastReplay") + ".yrp" };
  },

  async deleteReplay(name: string) {
    if (isWails) {
      return await callWails("DeleteReplay", name);
    }
    console.log("[MockBridge] Deleted replay:", name);
    return { success: true };
  },

  async renameReplay(oldName: string, newName: string) {
    if (isWails) {
      return await callWails("RenameReplay", oldName, newName);
    }
    console.log("[MockBridge] Renamed replay:", oldName, "->", newName);
    return { success: true };
  },

  // 录像头部元信息（原版 LISTBOX_REPLAY_LIST 选中时的 stReplayInfo，
  // menu_handler.cpp:519-559）：version/日期/玩家名/单机脚本名。
  async replayInfo(name: string): Promise<{
    success: boolean; version?: number; date?: string;
    players?: string[]; isTag?: boolean; isSingle?: boolean; script?: string;
    startLp?: number; duelRule?: number; error?: string;
  }> {
    if (isWails) {
      return await callWails("ReplayInfo", name);
    }
    return {
      success: true,
      version: 0x1362,
      date: '2026/01/01 12:00:00',
      players: ['YugiMuto', 'SetoKaiba'],
      isTag: false,
      isSingle: false,
      startLp: 8000,
      duelRule: 5,
    };
  },

  // 提取卡组（原版 BUTTON_EXPORT_DECK，menu_handler.cpp:293-319）：
  // 双方卡组各存一份 .ydk 到 deck 目录，返回生成的文件名。
  async exportReplayDeck(name: string): Promise<{ success: boolean; files?: string[]; error?: string }> {
    if (isWails) {
      return await callWails("ExportReplayDeck", name);
    }
    console.log("[MockBridge] Exported replay decks:", name);
    return { success: true, files: [name + '-1.ydk', name + '-2.ydk'] };
  },

  // 解析引擎 desc 字符串 id（data_manager.cpp GetDesc 语义，Go 侧
  // App.ResolveDesc 实现）：系统字符串/未知卡号返回空串，调用方兜底。
  async resolveDesc(descId: number): Promise<string> {
    if (isWails) {
      const res = await callWails("ResolveDesc", descId);
      return (res && res.text) || "";
    }
    return "";
  },

  // system.conf 读写（原版 game.cpp:1395-1607 LoadConfig/SaveConfig 的
  // App 侧翻译；键名与原版 system.conf 一一对应，可互换）。
  async getConfig(): Promise<Record<string, any>> {
    if (isWails) {
      const res = await callWails("GetConfig");
      return res || {};
    }
    return {};
  },

  async saveConfig(patch: Record<string, any>): Promise<Record<string, any>> {
    if (isWails) {
      const res = await callWails("SaveConfig", patch);
      return res || {};
    }
    return patch;
  }
};
