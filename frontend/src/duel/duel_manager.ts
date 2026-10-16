/**
 * Duel Manager & Controller
 * Bridges engine events to the 3D Field (meshes, tweens, chain visualizer)
 * and keeps the imperative side of the duel loop:
 *   - protocol auto-answers that need no UI (time confirm, select_place)
 *   - the pending idlecmd/battlecmd command state behind hand/field actions
 *
 * Everything the user can SEE as state (LP, hand, log, hints, prompts, win)
 * lives in duelStore (domain/reducer.ts is the single event→state translator);
 * the prompt popups live in components/PromptHost.tsx. This class therefore
 * has no HUD dependency and emits no DOM.
 *
 * 回放剧场同样走本类的动画订阅（DuelStage interactive=false）：事件带
 * __instant 标志时跳过 tween 直接落位（seek 重建用）。
 */

import { WailsBridge, eventBus } from '../wails_bridge.ts';
import { duelStore } from './store.ts';
import { settingsStore } from '../domain/settings.ts';
import { seatToDisplay, bottomEngineSeat } from '../domain/reducer.ts';
import { respondCardSelection } from './card_select.ts';
import {fieldSelectionReady,canPickFieldCard,sumValues,type FieldSelection,type FieldSelectionKind} from './field_selection.ts';
import {
  LOC_NAMES,
  LOC_DECK, LOC_HAND, LOC_EXTRA,
  LOC_MZONE, LOC_SZONE, LOC_GRAVE, LOC_BANISH,
  CARD_QUESTION, PHINT_DESC_ADD, REASON_DESTROY,
} from '../domain/constants.ts';
import type { DuelField3D } from './field3d.ts';
import { sysString } from '../domain/sys_strings.ts';
import { cardName } from '../domain/card_names.ts';
import { soundManager } from '../audio/sound_manager.ts';
import { TYPE_TRAP, TYPE_SPELL, TYPE_CONTINUOUS, TYPE_FIELD } from '../domain/constants.ts';

export class DuelManager {
  field3D: DuelField3D;
  /** 非交互模式（回放）不发送任何协议响应、不响应点击。 */
  interactive: boolean;
  playerSlot = 0;
  /** 双座手牌数（引擎 seat 索引；syncHands 把 store 手牌补零码到此数，
   *  驱动 field3D.setHands 的 3D 手牌直排——服务端抹零的对侧只知张数） */
  handCounts: [number, number] = [0, 0];
  idleCmd: any = null;
  battleCmd: any = null;
  fieldSelection:FieldSelection|null=null;
  /** MSG_SELECT_PLACE/DISFIELD 点选状态；null = 不在选位中 */
  placeSelect: {
    player: number;
    count: number;
    cancelable: boolean;
    zones: { player: number; loc: number; seq: number }[];
    selected: string[];
  } | null = null;
  _resyncing = false;
  _subs: [string, (data: any) => void][] = [];
  /** HINT_ZONE 高亮的自动清除定时器（原版 WaitFrameSignal(40) 后清） */
  hintZoneTimer: ReturnType<typeof setTimeout> | null = null;
  /** store 订阅（视角交换 / select_card 选择态联动）的退订函数 */
  _storeUnsub: (() => void) | null = null;
  _lastViewSwapped = false;
  /** 上次布设高亮时的 cardSelect 指纹（kind:count:selected），避免重复重建 */
  _lastCardSelectKey = '';
  /** store.board / piles 的上次引用：移动/表示变化等非 update_data 事件
   * 同样要改变状态浮签与牌堆计数（update_data 路径另有显式同步） */
  _lastBoard: any = null;
  _lastPiles: any = null;
  /** 装备关系：装备卡 key → 目标卡 key（MSG_EQUIP/UNEQUIP 维护，
   *  client_card.cpp 的 equipTarget/equipped） */
  equipPairs: Map<string, string> = new Map();
  /** 对象关系：卡对 "keyA|keyB"（MSG_CARD_TARGET/CANCEL_TARGET 维护，
   *  client_card.cpp 的 cardTarget/ownerTarget） */
  targetPairs: Set<string> = new Set();
  /** 连锁对象：当前连锁段的链卡与其对象集（MSG_BECOME_TARGET 追加、
   *  chain_end 清空，duelclient.cpp:3051-3052） */
  chainTargets: { card: string; targets: Set<string> }[] = [];

  constructor(field3D: DuelField3D, { interactive = true }: { interactive?: boolean } = {}) {
    this.field3D = field3D;
    // 非交互模式（回放）不发送任何协议响应、不响应点击。
    this.interactive = interactive;

    // 3D 场的点选/右键回调挂到本 manager（构造顺序：field 先建、manager 后建，
    // 回调只在事件后触发，无竞态）。
    this.field3D.onPlaceZoneClick = (zone) => this.onPlaceZoneClick(zone);
    this.field3D.onCardSelectPick = (idx) => this.onCardSelectPick(idx);
    this.field3D.onBoardRightClick = (x, y) => this.onBoardRightClick(x, y);
    this.field3D.onPileRightClick = (player, pile) => this.onPileRightClick(player, pile);
    // 左键点堆区 = 查看列表（与右键/F 键同路；堆顶卡的点击在 onFieldCardClick 转发）
    this.field3D.onPileLeftClick = (player, pile) => this.onPileRightClick(player, pile);
    this.field3D.onHoverChange = () => this.refreshRelationMarks();

    this.initNetworkListeners();

    // store 联动：视角交换 → 相机翻转（回放也生效）；cardSelect
    // 选择态 → 场上高亮刷新 + 选满自动应答（仅交互模式应答）。
    this._lastViewSwapped = duelStore.getState().viewSwapped;
    this._storeUnsub = duelStore.subscribe(() => this.onStoreChange());
    const initial = duelStore.getState();
    this.playerSlot = initial.playerSlot;
    this.field3D.setBottomSeat?.(initial.playerSlot, true);
    this.field3D.setDuelRule?.(initial.duelRule);
    this.syncHands();
    // Wails may deliver the initial snapshot before the stage mounts.
    if(this.field3D.cardsOnField)void this.resyncFromStore().then(()=>this.syncLinkData());
  }

  onStoreChange(): void {
    const st = duelStore.getState();
    this.syncHands();
    if (st.viewSwapped !== this._lastViewSwapped) {
      this._lastViewSwapped = st.viewSwapped;
      this.field3D.setViewSwapped(st.viewSwapped);
    }
    const cs = st.cardSelect;
    const key = cs ? `${cs.kind}:${cs.cards.length}:${cs.selected.join(',')}` : '';
    if (key !== this._lastCardSelectKey) {
      this._lastCardSelectKey = key;
      this.refreshCardSelectMarks();
    }
    // 场面/牌堆的任何 store 变化都刷新状态浮签与牌堆计数（幂等）；
    // 覆盖 pos_change/move 等不走 update_data 的路径
    if (st.board !== this._lastBoard || st.piles !== this._lastPiles) {
      this._lastBoard = st.board;
      this._lastPiles = st.piles;
      this.syncLinkData();
    }
    if(this.fieldSelection)this.refreshFieldSelection();
    if (!this.interactive || !cs || cs.kind !== 'card' || !cs.selected.length) return;
    // 选满自动应答（原版 event_handler.cpp:1389-1398：达到 select_max，
    // 或已达 select_min 且可选卡全部被选中时立即 SendResponse）
    if (cs.selected.length >= cs.max
        || (cs.selected.length >= cs.min && cs.selected.length === cs.cards.length)) {
      respondCardSelection();
    }
  }

  syncHands(): void {
    const st = duelStore.getState();
    const bottom = bottomEngineSeat(st);
    const codes: [number[], number[]] = [[],[]];
    codes[bottom] = st.hand.map((c) => c?.code || 0);
    codes[1-bottom] = st.oppHand.map((c) => c?.code || 0);
    for (const seat of [0,1] as const) while (codes[seat].length < this.handCounts[seat]) codes[seat].push(0);
    this.field3D.setHands?.(codes);
    for(const seat of [0,1] as const)this.field3D.handMeshes?.[seat].forEach((mesh:any)=>{
      const code=mesh.userData.cardCode;if(!code)return;
      const cached=WailsBridge._cardCache.get(code);
      if(cached)mesh.userData.cardInfo=cached;
      else WailsBridge.getCard(code).then(info=>{if(this.field3D.handMeshes[seat].includes(mesh))mesh.userData.cardInfo=info;});
    });
  }

  beginFieldSelection(kind:FieldSelectionKind,data:any):void {
    const cards=kind==='sum'?data.select||[]:data.cards||[];
    if(!this.interactive||!document.getElementById('duel-canvas-container')||![...cards,...(data.must||[])].every(c=>[LOC_HAND,LOC_MZONE,LOC_SZONE].includes(c.l)))return;
    this.fieldSelection={kind,data,cards,selected:[],counts:cards.map(()=>0)};
    if(kind==='counter'){
      const session=this.fieldSelection;
      WailsBridge.getCounterName(data.countertype).then(name=>{if(this.fieldSelection===session){session.data={...session.data,counterName:name};this.refreshFieldSelection();}}).catch(()=>{});
    }
    if(fieldSelectionReady(this.fieldSelection)&&!cards.some((_:any,i:number)=>canPickFieldCard(this.fieldSelection!,i)))this.respondFieldSelection();
    else this.refreshFieldSelection();
  }

  refreshFieldSelection():void {
    const s=this.fieldSelection;if(!s)return;
    const entries=s.cards.map((c,idx)=>({...c,idx})).filter((_,idx)=>canPickFieldCard(s,idx));
    this.field3D.setCardSelectMarks(entries,new Set(s.kind==='counter'?s.counts.flatMap((n,i)=>n?[i]:[]):s.selected));
    const ready=fieldSelectionReady(s),remaining=s.kind==='counter'?s.data.count-s.counts.reduce((a,b)=>a+b,0):undefined;
    let text=duelStore.getState().selectHintText||'请选择卡牌';
    if(s.kind==='counter')text=`请取除${remaining}个[${s.data.counterName||'？？？'}]`;
    else if(s.kind==='sum'){
      const all=[...(s.data.must||[]),...s.selected.map(i=>s.cards[i])];
      const low=all.reduce((sum,c)=>sum+Math.min(...sumValues(c.param)),0),high=all.reduce((sum,c)=>sum+Math.max(...sumValues(c.param)),0);
      text=`${text}(${low===high?low:`${low}-${high}`}/${s.data.acc}${s.data.sumMode?'+' : ''})`;
    }else if(!text.endsWith(`(${s.data.min}-${s.data.max})`))text=`${text}(${s.data.min}-${s.data.max})`;
    if(duelStore.getState().hint!==text)duelStore.setHint(text);
    eventBus.emit('ui:field_selection_control',{ready,cancelable:s.kind==='tribute'&&!!s.data.cancelable&&s.selected.length===0,text});
  }

  pickFieldSelection(idx:number):void {
    const s=this.fieldSelection;if(!s||!canPickFieldCard(s,idx))return;
    if(s.kind==='counter')s.counts[idx]++;
    else if(s.selected.includes(idx))s.selected=s.selected.filter(i=>i!==idx);
    else s.selected.push(idx);
    const ready=fieldSelectionReady(s),canExtend=s.cards.some((_,i)=>!s.selected.includes(i)&&canPickFieldCard(s,i));
    if(ready&&(s.kind==='counter'||!canExtend))this.respondFieldSelection();else this.refreshFieldSelection();
  }

  respondFieldSelection():void {
    const s=this.fieldSelection;if(!s||!fieldSelectionReady(s))return;
    this.endFieldSelection();
    if(s.kind==='counter')WailsBridge.respondCounter([...s.counts]);
    else if(s.kind==='sum')WailsBridge.respondSelectSum((s.data.must||[]).length+s.selected.length,[...s.selected]);
    else WailsBridge.respondSelectCard([...s.selected]);
  }

  cancelFieldSelection():void {
    const s=this.fieldSelection;if(!s||s.kind!=='tribute'||!s.data.cancelable||s.selected.length)return;
    this.endFieldSelection();WailsBridge.sendResponseI(-1);
  }

  endFieldSelection():void {
    if(!this.fieldSelection)return;
    this.fieldSelection=null;this.field3D.clearCardSelectMarks();
    duelStore.disarmSelectHint();duelStore.setHint('');
    eventBus.emit('ui:field_selection_control',null);
  }

  /** 关系条目的键 = 引擎座标 "c:l:s"（事件载荷的数值 loc） */
  relKey(e: { c: number; l: number; s: number }): string {
    return `${e.c}:${e.l}:${e.s}`;
  }

  /** 抹掉涉及某张卡的全部关系（ClientCard 析构 ClearTarget/equipTarget 同款） */
  purgeRelations(key: string): void {
    for (const [a, b] of this.equipPairs) {
      if (a === key || b === key) this.equipPairs.delete(a);
    }
    for (const p of [...this.targetPairs]) {
      if (p.split('|').includes(key)) this.targetPairs.delete(p);
    }
    this.chainTargets = this.chainTargets.filter((ch) => {
      if (ch.card === key) return false;
      ch.targets.delete(key);
      return true;
    });
  }

  /**
   * SetShowMark 语义（client_field.cpp:2444-2462）：悬停卡的装备/对象/
   * 连锁对象关联卡布关系图标；同卡优先级 equip > target > chaintarget
   * （drawing.cpp:446-457 的 else-if 链）。无悬停或悬停卡已离场时清空。
   */
  refreshRelationMarks(): void {
    if (typeof this.field3D.setRelationMarks !== 'function') return;
    const mesh = this.field3D.hoveredCard;
    const slot = mesh && mesh.userData.slot;
    const locCode = slot ? ({ mzone: 0x04, szone: 0x08 } as Record<string, number>)[slot.loc] : undefined;
    // 悬停引用可能已离场（mesh 退休时 hoveredCard 未清），以登记槽位为准
    const zone = slot && this.field3D.cardsOnField
      && this.field3D.cardsOnField[slot.player]
      && this.field3D.cardsOnField[slot.player][slot.loc];
    if (!mesh || !slot || locCode === undefined || !zone || zone[slot.seq] !== mesh) {
      this.field3D.setRelationMarks([]);
      return;
    }
    const hkey = `${slot.player}:${locCode}:${slot.seq}`;
    const marks = new Map<string, 'equip' | 'target' | 'chaintarget'>();
    const add = (key: string, kind: 'equip' | 'target' | 'chaintarget') => {
      // Map 保序 + 先入优先 = equip > target > chaintarget
      if (key !== hkey && !marks.has(key)) marks.set(key, kind);
    };
    for (const [a, b] of this.equipPairs) {
      if (a === hkey) add(b, 'equip');
      if (b === hkey) add(a, 'equip');
    }
    for (const p of this.targetPairs) {
      const [a, b] = p.split('|');
      if (a === hkey) add(b, 'target');
      if (b === hkey) add(a, 'target');
    }
    for (const ch of this.chainTargets) {
      if (ch.card === hkey) for (const t of ch.targets) add(t, 'chaintarget');
      if (ch.targets.has(hkey)) add(ch.card, 'chaintarget');
    }
    const out: { mesh: any; kind: 'equip' | 'target' | 'chaintarget' }[] = [];
    for (const [key, kind] of marks) {
      const [c, l, s] = key.split(':').map(Number);
      const m = this.field3D.meshAt(c, l, s);
      if (m) out.push({ mesh: m, kind });
    }
    this.field3D.setRelationMarks(out);
  }

  /** select_card/unselect 的场上候选（mzone/szone）布/撤 3D 高亮 */
  refreshCardSelectMarks(): void {
    const cs = duelStore.getState().cardSelect;
    if (!this.interactive || !cs || !cs.cards.some((c) => c.onField)) {
      this.field3D.clearCardSelectMarks();
      return;
    }
    const entries = cs.cards
      .filter((c) => c.onField)
      .map((c) => ({ idx: c.idx, c: c.c, l: c.l, s: c.s }));
    this.field3D.setCardSelectMarks(entries, new Set(cs.selected));
  }

  /** 3D 场点选回调：unselect 单选即应答；card 点选/取消（选满自动应答
   * 由 onStoreChange 统一触发，弹窗路径共享同一语义） */
  onCardSelectPick(idx: number): void {
    if (!this.interactive) return;
    if(this.fieldSelection){this.pickFieldSelection(idx);return;}
    const cs = duelStore.getState().cardSelect;
    if (!cs) return;
    if (cs.kind === 'unselect') {
      duelStore.toggleCardSelect(idx);
      respondCardSelection();
      return;
    }
    duelStore.toggleCardSelect(idx);
  }

  initNetworkListeners(): void {
    // Every subscription is recorded so dispose() can unsubscribe: the
    // eventBus outlives the duel screen, and a re-entered duel must not stack
    // a second DuelManager that would apply each event twice.
    this._subs = [];
    const sub = (event: string, fn: (data: any) => void) => {
      eventBus.on(event, fn);
      this._subs.push([event, fn]);
    };

    // STOC_TIME_LIMIT：轮到本地玩家计时，立即回 CTOS_TIME_CONFIRM
    // （gframe duelclient.cpp:783-784 的行为；倒计时显示在 PlayerPanel）。
    sub('stoc:time_limit', (data) => {
      if (this.interactive && data.player === this.playerSlot) {
        WailsBridge.sendTimeConfirm();
      }
    });

    // MSG_START's playertype byte is our seat in the engine's player
    // numbering (0 = first player). Every perspective decision below keys
    // off it; the store displays our seat as the "player" slot.
    sub('duel:start', (data) => {
      this.endFieldSelection();
      this.field3D.clearBoard?.();
      this.playerSlot = (data.playerType || 0) & 0x0f;
      this.field3D.setBottomSeat?.(this.playerSlot,true);
      this.field3D.setDuelRule?.(data.duelRule || 5);
      this.handCounts = [0, 0];
      this.field3D.setCantCheckGrave(false);
      this.equipPairs.clear();
      this.targetPairs.clear();
      this.chainTargets = [];
      // 新一局视角复位（reducer 已清 viewSwapped，这里同步 3D 相机）
      this._lastViewSwapped = false;
      this.field3D.setViewSwapped(false);
    });

    // MSG_PLAYER_HINT 的 CARD_QUESTION（duelclient.cpp:3757-3768）：
    // 提示加/删到本方 → 双方墓地上空的禁查「?」图标显/隐。
    sub('duel:player_hint', (data) => {
      if (data.data === CARD_QUESTION && data.player === this.playerSlot) {
        this.field3D.setCantCheckGrave(data.type === PHINT_DESC_ADD);
      }
    });

    // MSG_RELOAD_FIELD（谜题布场）：双座手牌数直接取快照
    sub('duel:reload_field', (data) => {
      for (const seat of [0, 1] as const) {
        const p = data.players && data.players[seat];
        if (p) this.handCounts[seat] = p.hand || 0;
      }
      this.syncHands();
    });

    // MSG_TAG_SWAP（TAG 队友换手）：该座的手牌数直接以本消息为准（原版
    // duelclient.cpp:3782 用 hcount 重排 dField.hand）；本队手牌由 reducer 处理
    sub('duel:tag_swap', (data) => {
      this.handCounts[data.player as 0 | 1] = data.handCount || 0;
      this.syncHands();
    });

    sub('duel:draw', async (data) => {
      // 3D 手牌行是持久表示，计数在 __instant（seek 重建）下也要同步
      this.handCounts[data.player as 0 | 1] = (this.handCounts[data.player as 0 | 1] || 0) + data.count;
      this.syncHands();
      if (data.__instant) return; // 回放 seek 重建：跳过发牌动画
      for (let i = 0; i < data.count; i++) {
        const cardCode = (data.cards && data.cards[i]) ? data.cards[i] : 0;
        const cardInfo = cardCode ? await WailsBridge.getCard(cardCode) : null;
        this.field3D.animateDrawCard(data.player, cardCode, cardInfo);
      }
    });

    sub('duel:shuffle_hand', (data) => {
      this.syncHands();
      if (!data.__instant) this.field3D.shuffleHand?.(data.player);
    });

    // MSG_SHUFFLE_DECK：原版 SOUND_SHUFFLE + 卡组 5 轮抖动（duelclient.cpp:2263-2301）
    sub('duel:shuffle_deck', (data) => {
      if (data.__instant) return;
      soundManager.playShuffle();
      this.field3D.shakeDeckPile?.(data.player as 0 | 1);
    });    // MSG_SHUFFLE_EXTRA：原版同样播 SOUND_SHUFFLE（duelclient.cpp:2345-2372）；
    // 额外堆无实体 mesh，不抖动
    sub('duel:shuffle_extra', (data) => {
      if (data.__instant) return;
      soundManager.playShuffle();
    });

    // 原版 LP 三事件的事件级音效（duelclient.cpp:3182 SOUND_DAMAGE /
    // :3211 SOUND_RECOVER / :3367 SOUND_DAMAGE）：数字滚动本身不再加 tick
    sub('duel:damage', (data) => { if (!data.__instant) soundManager.playDamage(); });
    sub('duel:recover', (data) => { if (!data.__instant) soundManager.playRecover?.(); });
    sub('duel:pay_lpcost', (data) => { if (!data.__instant) soundManager.playDamage(); });

    sub('duel:select_card',data=>{if(data.tribute)this.beginFieldSelection('tribute',data);});
    sub('duel:select_sum',data=>this.beginFieldSelection('sum',data));
    sub('duel:select_counter',data=>this.beginFieldSelection('counter',data));
    sub('ui:field_selection_finish',()=>this.respondFieldSelection());
    sub('ui:field_selection_cancel',()=>this.cancelFieldSelection());

    sub('duel:summoning', async (data) => {
      const cardInfo = await WailsBridge.getCard(data.code);
      if (data.__instant) {
        this.field3D.placeCard(data.cc, 'mzone', data.cs, data.code, cardInfo, data.cp);
        return;
      }
      this.field3D.animateSummon(data.cc, data.code, data.cs, data.cp, cardInfo, false);
    });

    sub('duel:spsummoning', async (data) => {
      const cardInfo = await WailsBridge.getCard(data.code);
      if (data.__instant) {
        this.field3D.placeCard(data.cc, 'mzone', data.cs, data.code, cardInfo, data.cp);
        return;
      }
      this.field3D.animateSummon(data.cc, data.code, data.cs, data.cp, cardInfo, true);
    });

    sub('duel:flipsummoning', async (data) => {
      const cardInfo = await WailsBridge.getCard(data.code);
      if (data.__instant) {
        this.field3D.placeCard(data.cc, 'mzone', data.cs, data.code, cardInfo, data.cp);
        return;
      }
      this.field3D.animateSummon(data.cc, data.code, data.cs, data.cp, cardInfo, false);
    });

    // MSG_SUMMONED/SPSUMMONED/FLIPSUMMONED：召唤落定顿点（小冲击波+压实弹跳）；
    // 日志在 reducer（SysString 1604/1606/1608）
    const onSummoned = (data: any) => {
      if (data && data.__instant) return;
      this.field3D.settleCard(data.cc, data.cs);
    };
    sub('duel:summoned', onSummoned);
    sub('duel:spsummoned', onSummoned);
    sub('duel:flipsummoned', onSummoned);

    sub('duel:set', async (data) => {
      const isMonster = data.cl === 0x4;
      const cardInfo = await WailsBridge.getCard(data.code);
      if (data.__instant) {
        this.field3D.placeCard(data.cc, isMonster ? 'mzone' : 'szone', data.cs, data.code, cardInfo, data.cp);
        return;
      }
      this.field3D.animateSetCard(data.cc, data.code, data.cs, isMonster, cardInfo);
    });

    sub('duel:pos_change', (data) => {
      // MSG_POS_CHANGE carries the zone (cl); mzone flips rotate the mesh,
      // szone flips matter for set field spells activating face-up.
      const locName = LOC_NAMES[data.cl as number] || 'mzone';
      if (data.__instant) {
        const zone = this.field3D.cardsOnField[data.cc] && this.field3D.cardsOnField[data.cc][locName];
        const mesh = zone ? zone[data.cs] : null;
        if (mesh) {
          const rot = this.field3D.cardRotationFor(data.cc, data.cp);
          mesh.rotation.set(rot.x, rot.y, rot.z);
          mesh.userData.positionState = this.field3D.positionStateFor(data.cp);
          this.field3D.refreshFieldSpell();
        }
        return;
      }
      this.field3D.animateReposition(data.cc, locName, data.cs, data.cp);
    });

    // MSG_MOVE covers every relocation the summon/set events do not animate:
    // destroyed → grave, returned → deck/hand/extra, banished, and cards
    // detaching as Xyz material (overlay). Cards entering the field by plain
    // move (no SUMMONING/SET preamble) are placed directly. Hand bookkeeping
    // is the store's job (reducer syncHandOnMove) — no 2D dock here.
    sub('duel:move', async (data) => {
      const fromLoc = LOC_NAMES[data.pl as number];
      const toLoc = LOC_NAMES[data.cl as number];

      // 手牌数：离手 -1、回手 +1（3D 手牌行的持久表示同步）
      if (fromLoc === 'hand') {
        this.handCounts[data.pc as 0 | 1] = Math.max(0, this.handCounts[data.pc as 0 | 1] - 1);
        this.syncHands();
      }
      if (toLoc === 'hand') {
        this.handCounts[data.cc as 0 | 1] += 1;
        this.syncHands();
      }

      // 离场卡的装备/对象/连锁对象关系清理（MSG_MOVE 里 ClientCard 析构的
      // ClearTarget/equipTarget 抹除，duelclient.cpp:2645-2653/2704-2710）
      if (fromLoc === 'mzone' || fromLoc === 'szone') {
        this.purgeRelations(`${data.pc}:${data.pl}:${data.ps}`);
        this.refreshRelationMarks();
      }

      const mesh = (fromLoc && data.pl !== LOC_HAND && data.pl !== LOC_DECK && data.pl !== LOC_EXTRA)
        ? this.field3D.removeFromSlot(data.pc, fromLoc, data.ps)
        : null;

      if (mesh) {
        // Card was visibly on the board: animate it to its destination.
        // duelclient.cpp MSG_MOVE: destroyed sound, unless banish takes priority.
        if (!data.__instant && data.pl!==data.cl && !(data.cl&LOC_BANISH) && (Number(data.reason) & REASON_DESTROY)) {
          this.field3D.animateDestroy?.(mesh);
        }
        if (toLoc === 'hand') {
          this.field3D.retireMesh(mesh, !!data.__instant);
        } else {
          this.field3D.moveCard(mesh, data.cc, toLoc, data.cs, data.cp, { instant: !!data.__instant });
        }
      } else if ((toLoc === 'grave' || toLoc === 'banish' || toLoc === 'mzone' || toLoc === 'szone') && data.code) {
        // 卡从不可见区域直接落到可见区（pl==0 凭空出现：token 出场、手牌/
        // 卡组直入场上等）——原地生成 mesh；场上目标再按原版 FadeCard 20 帧
        // 淡入（client_field.cpp:1107-1111），不再等下一次 update_data 的
        // 漂移重建。grave/banish 目标维持原样的即时堆叠。
        const cardInfo = await WailsBridge.getCard(data.code);
        const mesh = this.field3D.placeCard(data.cc, toLoc, data.cs, data.code, cardInfo, data.cp);
        if (mesh && (toLoc === 'mzone' || toLoc === 'szone')) {
          this.field3D.fadeInMesh(mesh, !!data.__instant);
        }
      }
      // Note: a plain move INTO our hand renders nothing (the store's hand
      // dock shows it); from-board meshes retire above.
    });

    sub('duel:attack', (data) => {
      if (data.__instant) return;
      this.field3D.animateAttack(
        data.attacker.c,
        data.attacker.s,
        data.target.c,
        data.target.s,
        data.target.l === 0
      );
    });

    // Chain Events
    sub('duel:chaining', async (data) => {
      // 连锁段的对象集登记（MSG_BECOME_TARGET 追加到最后一条链）——
      // 必须先于 await，否则紧随的 become_target 会找不到链
      this.chainTargets.push({ card: this.relKey({ c: data.cc, l: data.cl, s: data.cs }), targets: new Set() });
      const cardInfo = await WailsBridge.getCard(data.code);
      // Anchor the badge on the field only for cards that are ON the field;
      // hand/deck/grave activations (e.g. hand traps) have no slot to pin to.
      if (data.cl === 0x4 || data.cl === 0x8) {
        const locName = data.cl === 0x4 ? 'mzone' : 'szone';
        this.field3D.chainVisualizer?.addChainLink(data.code, { player: data.cc, loc: locName, seq: data.cs }, cardInfo);
        // Activating a set field spell turns it face-up; its art becomes the
        // board background as soon as the chain anchor is announced.
        if (locName === 'szone' && data.cs === 5) {
          this.field3D.flipSzoneCard(data.cc, 5);
        }
      }
    });

    sub('duel:chain_solving', (data) => {
      this.field3D.chainVisualizer?.highlightSolvingLink(data.count);
    });

    sub('duel:chain_solved', (data) => {
      this.field3D.chainVisualizer?.removeSolvingLink(data.count);
    });

    // MSG_CHAINED：连锁成立——最新连锁徽章顿点盖戳（原版在 dField.chains
    // 压入正式连锁序号，徽章本身在 chaining 时已弹出）
    sub('duel:chained', (data) => {
      if (data && data.__instant) return;
      this.field3D.chainVisualizer?.stampLatestLink();
    });

    sub('duel:chain_end', () => {
      this.field3D.chainVisualizer?.clearChain();
      // 连锁段结束：链卡/对象的 chaintarget 图标关系一并清除
      this.chainTargets = [];
      this.refreshRelationMarks();
    });

    sub('duel:select_idlecmd', (data) => {
      this.idleCmd = data;
      this.battleCmd = null;
      // 原版 act.png 可发动角标（drawing.cpp:483-522）：idle 询问期间
      // 标出墓地/除外/额外/场上所有可发动点；进入战阶询问后清除
      this.field3D.clearAttackable();
      this.field3D.setActivatable(data.activate || []);
    });

    sub('duel:select_battlecmd', (data) => {
      this.battleCmd = data;
      // The engine awaits a battle-command answer now; a stale idle command
      // would let the hand popup send idle-phase responses mid-battle.
      this.idleCmd = null;
      // 原版战阶询问：可攻击怪兽头顶剑标记 + 速攻等可发动角标
      this.field3D.setActivatable(data.activate || []);
      this.field3D.setAttackable(data.attack || []);
    });

    // 连锁询问期间原版同样显示 act.png 角标（可连锁的卡）
    sub('duel:select_chain', (data) => {
      const chains = (data && data.chains) || [];
      this.field3D.setActivatable(chains.map((c: any) => ({ c: c.c, l: c.l, s: c.s })));
    });

    // 原版应答连锁即 ClearChainSelect（client_field.cpp:404-420）：角标不残留
    // 到 chain_end/下一阶段，引擎接受应答进入 chaining 时即清除
    sub('duel:chaining', () => {
      this.field3D.clearActivatable();
    });

    // 新阶段/指令结束：两类角标都清除；阶段横幅音效（原版 SOUND_PHASE，
    // 在 MSG_NEW_PHASE 时与 showcard=101 横幅一起触发）
    sub('duel:new_phase', () => {
      this.field3D.clearAttackable();
      this.field3D.clearActivatable();
      this.clearHintZones();
      soundManager.playPhaseChange();
    });

    // MSG_SELECT_PLACE / MSG_SELECT_DISFIELD：原版两种模式——
    // 自动落点（duelclient.cpp:1852-1905：automonsterpos 覆盖含怪兽区的
    // 询问、autospellpos 覆盖纯魔陷区询问；SELECT_DISFIELD 从不自动）或
    // 进入点选模式（event_handler.cpp:1316-1375：点格累计选位、再点已选
    // 格取消、选满即应答）。Go decorateSelectPlace 已把禁用位掩码取反、
    // 解码成带区域归属 player 的 zones 列表。
    sub('duel:select_place', (data) => {
      if (!this.interactive) return;
      this.beginPlaceSelect(data);
    });

    // MSG_FIELD_DISABLED：禁用格白叉（drawing.cpp:210-241）
    sub('duel:field_disabled', (data) => {
      this.field3D.setFieldDisabled((data && data.zones) || 0);
    });

    // ---- 波 B：过程展示消息 → field3d 原语（回放同样消费，无协议响应）----

    // MSG_SHUFFLE_SET_CARD：同区盖卡换位。mesh 按卡密码配对重排——自己的
    // 盖卡 mesh 带真实卡图，必须跟着自己的 code 走；对方全是卡背任意配对。
    // 换位是同区排列，槽集合不变，instant 落位（原版也只是快速归位）。
    sub('duel:shuffle_set_card', (data) => {
      if (data.__instant) return; // seek 重建走 resyncFromStore
      const entries = data.cards || [];
      if (!entries.length) return;
      const locName = LOC_NAMES[entries[0].l];
      if (locName !== 'mzone' && locName !== 'szone') return;
      const meshes = entries.map((e: any) => this.field3D.meshAt(e.c, e.l, e.s));
      const used = new Set<number>();
      entries.forEach((e: any) => {
        let idx = meshes.findIndex((m: any, j: number) =>
          m && !used.has(j) && m.userData.cardCode === e.code);
        if (idx < 0) idx = meshes.findIndex((m: any, j: number) => m && !used.has(j));
        used.add(idx);
        if (meshes[idx]) {
          this.field3D.moveCard(meshes[idx], e.c, locName, e.s, e.p || 0x8, { instant: true });
        }
      });
    });

    // ---- 悬停关系图标的数据源（client_card.cpp 的 equipTarget/cardTarget
    // 关系表）：事件只维护关系，图标在悬停时经 refreshRelationMarks 按
    // SetShowMark 语义布设（原版无事件驱动的关系连线） ----

    sub('duel:equip', (data) => {
      // MSG_EQUIP：装备卡原有对象先解除（duelclient.cpp:3249-3254）
      this.equipPairs.delete(this.relKey(data.card));
      this.equipPairs.set(this.relKey(data.card), this.relKey(data.target));
      this.refreshRelationMarks();
    });

    sub('duel:unequip', (data) => {
      this.equipPairs.delete(this.relKey(data.card));
      this.refreshRelationMarks();
    });

    sub('duel:card_target', (data) => {
      this.targetPairs.add(this.relKey(data.card) + '|' + this.relKey(data.target));
      this.refreshRelationMarks();
    });

    sub('duel:cancel_target', (data) => {
      this.targetPairs.delete(this.relKey(data.card) + '|' + this.relKey(data.target));
      this.refreshRelationMarks();
    });

    // MSG_BECOME_TARGET：对象追加进当前（最后一条）连锁的对象集
    sub('duel:become_target', (data) => {
      const chain = this.chainTargets[this.chainTargets.length - 1];
      if (!chain) return;
      for (const t of data.targets || []) chain.targets.add(this.relKey(t));
    });

    // 控制权交换（MSG_SWAP）：涉及卡的关系清掉（原版重绑 equipTarget，
    // 这里近似为清除——swap 后再由新事件重建）
    sub('duel:swap', (data) => {
      this.purgeRelations(`${data.cc1}:${data.cl1}:${data.cs1}`);
      this.purgeRelations(`${data.cc2}:${data.cl2}:${data.cs2}`);
    });

    sub('duel:add_counter', (data) => {
      this.field3D.applyCounterDelta(data.c, data.l, data.s, data.type, data.count);
      if (!data.__instant) soundManager.playCounter(true);
    });

    sub('duel:remove_counter', (data) => {
      this.field3D.applyCounterDelta(data.c, data.l, data.s, data.type, -data.count);
      if (!data.__instant) soundManager.playCounter(false);
    });

    sub('duel:confirm_cards', (data) => {
      if (data.__instant) return;
      this.field3D.flashCards(data.cards || []);
    });

    // MSG_HINT：HINT_EVENT(1)/HINT_MESSAGE(2) 的文本走 ResolveDesc 显示在
    // 提示条（gframe MESG_HINT 的 stHintMsg 分支）；HINT_ZONE(11) 译位掩码
    // 为场上区域高亮；其余类型（select 提示/宣言展示/效果角标）由
    // reducer / PromptHost / SpecOverlay 消费。
    sub('duel:hint', async (data) => {
      if (data.type === 11) {
        this.showHintZones(data.player, data.data);
        return;
      }
      if (data.type !== 1 && data.type !== 2) return;
      const text = await WailsBridge.resolveDesc(data.data);
      if (text) {
        duelStore.setHint(text);
        // HINT_EVENT 存 store（原版 event_string）：询问弹窗文案以它开头
        if (data.type === 1) duelStore.setHintEvent(text);
      }
    });

    // ---- 2D UI 动作接线（HandDock / ActionPopup 发出）----

    sub('ui:hand_click', async (data) => {
      if (!this.interactive) return;
      const opts = await this.getHandOptions(data.code,data.seq);
      if (opts.length) {
        duelStore.openActionPopup(data.x, data.y, opts);
      } else {
        duelStore.closeActionPopup();
      }
    });

    sub('ui:player_action', (data) => {
      if (!this.interactive) return;
      this.handlePlayerAction(data.type, data.data);
    });

    // MSG_UPDATE_DATA：引擎的权威同步点。用它对比网格槽与 store.board，
    // 漂移时清盘重建（ Step E 的 store 重同步路径）；随后把连接怪的
    // type/linkMarker 写进 mesh userData（悬停互连高亮的数据源），并按
    // 新场面重算悬停中的高亮（场面变化清除/刷新语义）。
    sub('duel:update_data', () => {
      this.resyncFromStore().then(() => {
        this.syncLinkData();
        this.field3D.updateLinkedZones();
      });
    });
    // MSG_UPDATE_CARD 不走整板 resync，但 status/linkMarker 可能变化 →
    // 刷新无效化徽章 + 连接数据 + 悬停高亮
    sub('duel:update_card', () => {
      this.syncNegatedBadges();
      this.syncLinkData();
      this.field3D.updateLinkedZones();
    });
  }

  // Unsubscribes every eventBus listener registered in initNetworkListeners.
  // Called by the hosting React component on unmount.
  dispose() {
    this.endFieldSelection();
    if (this._storeUnsub) {
      this._storeUnsub();
      this._storeUnsub = null;
    }
    this.field3D.onHoverChange = null;
    if (!this._subs) return;
    this._subs.forEach(([event, fn]) => eventBus.off(event, fn));
    this._subs = [];
  }

  // ---- MSG_SELECT_PLACE 落点选择（点选模式） ----

  /**
   * HINT_ZONE（duelclient.cpp:1168）：位掩码译成格子在场上画高亮（与
   * select_place 同位域、不同样式），~40 帧（0.8s）后自动清除；对方操作
   * 时高低 16 位互换（低半 = 屏幕下方本方场地）。
   */
  showHintZones(player: number, raw: number): void {
    // 位掩码低 16 位 = 屏幕下侧一方的场地（viewSwapped 时下侧是对面 seat）
    const seat = bottomEngineSeat(duelStore.getState());
    let mask = raw >>> 0;
    if (player !== seat) mask = ((mask >>> 16) | (mask << 16)) >>> 0;
    const opp = 1 - seat;
    const zones: { player: number; loc: number; seq: number }[] = [];
    const scan = (p: number, loc: number, base: number, count: number) => {
      for (let seq = 0; seq < count; seq++) {
        if (mask & (base << seq)) zones.push({ player: p, loc, seq });
      }
    };
    scan(seat, 0x04, 0x1, 7);
    scan(seat, 0x08, 0x100, 8);
    scan(opp, 0x04, 0x10000, 7);
    scan(opp, 0x08, 0x1000000, 8);
    this.field3D.setHintZones(zones);
    if (this.hintZoneTimer) clearTimeout(this.hintZoneTimer);
    this.hintZoneTimer = setTimeout(() => {
      this.hintZoneTimer = null;
      this.field3D.clearHintZones();
    }, 800);
  }

  clearHintZones(): void {
    if (this.hintZoneTimer) {
      clearTimeout(this.hintZoneTimer);
      this.hintZoneTimer = null;
    }
    this.field3D.clearHintZones();
  }

  placeZoneKey(zone: { player: number; loc: number; seq: number }): string {
    return `${zone.player}:${zone.loc}:${zone.seq}`;
  }

  /**
   * select_place 入口：自动落点设置命中时按原版优先级代答（随机位
   * randompos 开时在候选里均匀取），否则进入点选模式并布设高亮。
   */
  beginPlaceSelect(data: any): void {
    this.endPlaceSelect(); // 新的询问到达时旧的高亮/选择态作废
    // 原版消息处理即消费 select_hint（自动代答路径也不例外）
    const hintId = duelStore.getState().selectHint;
    if (hintId) duelStore.consumeSelectHint();
    const zones = (data.zones || []).map((z: any) => ({
      player: z.player ?? data.player,
      loc: z.loc,
      seq: z.seq,
    }));
    if (!zones.length) return;
    // 原版 selectable_field & 0x7f007f：可选集含任一怪兽区（己方或对方）
    const hasMzone = zones.some((z: any) => z.loc === 0x04);
    const autoMonster = Number(settingsStore.get('automonsterpos')) !== 0;
    const autoSpell = Number(settingsStore.get('autospellpos')) !== 0;
    if (!data.disfield && ((autoMonster && hasMzone) || (autoSpell && !hasMzone))) {
      const zone = this.pickAutoPlaceZone(zones, data.player, hasMzone);
      if (zone) WailsBridge.respondSelectPlace(zone.player, zone.loc, zone.seq);
      return;
    }
    const count = Math.max(1, data.count || 1);
    this.placeSelect = {
      player: data.player,
      count,
      cancelable: (data.count || 0) === 0,
      zones,
      selected: [],
    };
    this.field3D.setPlaceSelectZones(zones, new Set());
    // 原版 stHintMsg：SysString 560「请选择」/ 570「请选择要变成不能使用的
    // 卡片区域」（SELECT_DISFIELD）；select_hint 优先——SELECT_PLACE 时
    // hint 是卡号（SysString 569「请选择[%ls]的位置」），DISFIELD 时是
    // desc id（duelclient.cpp:1839-1849）。
    const fallback = data.disfield
      ? (sysString(570) || '请选择要变成不能使用的区域')
      : (sysString(560) || '请选择卡片的位置');
    let text = fallback;
    if (hintId) {
      text = data.disfield
        ? (sysString(hintId) || fallback)
        : (sysString(569) || '请选择[%ls]的位置').replace('[%ls]', cardName(hintId));
    }
    duelStore.setHint(text);
    duelStore.armSelectHint(text);
    if (hintId) {
      // desc id / 卡名未入缓存时异步精化（原版 GetDesc/GetName 是同步全表）
      const sel = this.placeSelect;
      void (async () => {
        let refined = '';
        if (data.disfield) {
          refined = sysString(hintId) || (await WailsBridge.resolveDesc(hintId)) || '';
        } else {
          const info = await WailsBridge.getCard(hintId);
          const name = (info && info.name) || cardName(hintId);
          refined = (sysString(569) || '请选择[%ls]的位置').replace('[%ls]', name);
        }
        if (refined && this.placeSelect === sel) {
          duelStore.setHint(refined);
          duelStore.armSelectHint(refined);
        }
      })();
    }
  }

  // 原版自动落点序（duelclient.cpp:1856-1901）：区域优先级
  // 己 mzone→己 szone→己灵摆→对 mzone→对 szone→对灵摆；区内优先序
  // 怪兽区 6,5,2,1,3,0,4（额外怪区优先），灵摆区 6,7；randompos 开时区内均匀随机。
  pickAutoPlaceZone(
    zones: { player: number; loc: number; seq: number }[],
    player: number,
    hasMzone: boolean,
  ): { player: number; loc: number; seq: number } | null {
    const regions = [
      { player, loc: 0x04, pzone: false },
      { player, loc: 0x08, pzone: false },
      { player, loc: 0x08, pzone: true },
      { player: 1 - player, loc: 0x04, pzone: false },
      { player: 1 - player, loc: 0x08, pzone: false },
      { player: 1 - player, loc: 0x08, pzone: true },
    ];
    const randomPos = Number(settingsStore.get('randompos')) !== 0;
    for (const region of regions) {
      // mzone 全区 seq0-6（5 主怪区 + 2 额外怪区）；szone 拆魔陷 seq0-5 与
      // 灵摆 seq6/7 两个区域（原版 filter 0x3f00 / 0xc000 的拆分）
      const candidates = zones.filter((z) => {
        if (z.player !== region.player || z.loc !== region.loc) return false;
        if (region.loc === 0x04) return true;
        return region.pzone ? z.seq >= 6 : z.seq < 6;
      });
      if (!candidates.length) continue;
      if (region.pzone) {
        return candidates.find((z) => z.seq === 6) || candidates[0];
      }
      if (randomPos) {
        return candidates[Math.floor(Math.random() * candidates.length)];
      }
      const priority = [6, 5, 2, 1, 3, 0, 4];
      for (const seq of priority) {
        const hit = candidates.find((z) => z.seq === seq);
        if (hit) return hit;
      }
      return candidates[0];
    }
    return null;
  }

  /** 3D 场点选回调：点击高亮格 → 选中/再点取消；选满即应答 */
  onPlaceZoneClick(zone: { player: number; loc: number; seq: number }): void {
    if (!this.interactive) return;
    const ps = this.placeSelect;
    if (!ps) return;
    const key = this.placeZoneKey(zone);
    const idx = ps.selected.indexOf(key);
    if (idx >= 0) {
      ps.selected.splice(idx, 1);
    } else {
      if (ps.selected.length >= ps.count) return; // 选满后已应答，防重入
      ps.selected.push(key);
    }
    this.field3D.setPlaceSelectZones(ps.zones, new Set(ps.selected));
    if (ps.selected.length === ps.count) {
      // 应答顺序对齐原版 respbuf 排列：己方 mzone → 己方 szone →
      // 对方 mzone → 对方 szone，各按 seq 升序（event_handler.cpp:1328-1366）
      const selectedZones = ps.selected.map((k) => {
        const [p, l, s] = k.split(':').map(Number);
        return { player: p, loc: l, seq: s };
      });
      const regionRank = (z: { player: number; loc: number }): number =>
        (z.player === ps.player ? 0 : 2) + (z.loc === 0x04 ? 0 : 1);
      selectedZones.sort((a, b) => (regionRank(a) - regionRank(b)) || (a.seq - b.seq));
      const flat: number[] = [];
      for (const z of selectedZones) flat.push(z.player, z.loc, z.seq);
      WailsBridge.respondSelectPlaces(flat);
      this.endPlaceSelect();
    }
  }

  /** 右键点空处：选位模式下撤销最近一次已选；无已选且询问可取消时回
   * [player,0,0]；select_card 点选中撤销最近一次已选（与弹窗选择集同步）；
   * 否则关动作菜单（原版 CancelOrFinish 对非 cancelable 询问不做清除，
   * 这里按需求补"撤销最近一次"的 Web 习惯交互） */
  onBoardRightClick(_x: number, _y: number): void {
    if (!this.interactive) return;
    if(this.fieldSelection){if(fieldSelectionReady(this.fieldSelection))this.respondFieldSelection();else this.cancelFieldSelection();return;}
    if (this.placeSelect) {
      const ps = this.placeSelect;
      if (ps.selected.length) {
        ps.selected.pop(); // 取消最近一次已选
        this.field3D.setPlaceSelectZones(ps.zones, new Set(ps.selected));
      } else if (ps.cancelable) {
        WailsBridge.respondSelectPlace(ps.player, 0, 0);
        this.endPlaceSelect();
      }
      return;
    }
    const cs = duelStore.getState().cardSelect;
    if (cs && cs.kind === 'card' && cs.selected.length) {
      duelStore.toggleCardSelect(cs.selected[cs.selected.length - 1]);
      return;
    }
    // 原版 CancelOrFinish：右键收起已开的堆区列表
    eventBus.emit('ui:close_pile', {});
    duelStore.closeActionPopup();
  }

  /** 右键点在墓地/除外/额外堆区：快速查看列表（F1-F8 同源的 wCardDisplay）。
   * 原版观战/回放同样可浏览堆区（event_handler.cpp:1113-1162），不限制 interactive */
  onPileRightClick(player: number, pile: string): void {
    if (pile === 'deck') return; // 卡组内容本前端不落 board，无列表数据（原版 F 键同样不含卡组）
    if (pile === 'grave' && duelStore.getState().cantCheckGrave) return; // 原版 F1 禁查
    eventBus.emit('ui:show_pile', { player: seatToDisplay(duelStore.getState(), player), pile });
  }

  endPlaceSelect(): void {
    if (!this.placeSelect) return;
    this.placeSelect = null;
    this.field3D.clearPlaceSelectZones();
    duelStore.setHint('');
    duelStore.disarmSelectHint();
  }

  // Builds the action popup options for a hand card from the pending idle
  // command: only actions the engine actually offered are shown. 战阶询问中
  // 手牌只有可发动的二速效果（速攻魔法/手坑，原版 ShowMenu 手卡 cmdFlag
  // 的 ACTIVATE 项）。
  async getHandOptions(code: number, seq?: number) {
    if (!this.idleCmd) {
      const acts = (this.battleCmd && this.battleCmd.activate) || [];
      return this.activateOptions(
        acts.filter((a: any) => a.l === LOC_HAND && a.code === code && (seq === undefined || a.s === seq)),
        { action: 'activate', code, seq },
      );
    }
    const opts = [];
    const has = (list: any) => list && list.some((c: any) => c.code === code && (seq === undefined || c.s === seq) && (c.l === undefined || c.l === LOC_HAND));
    if (has(this.idleCmd.summon)) opts.push({ label: '召唤', action: 'summon', code,seq });
    if (has(this.idleCmd.spsummon)) opts.push({ label: '特殊召唤', action: 'spsummon', code,seq });
    if (has(this.idleCmd.mset)) opts.push({ label: '盖放', action: 'mset', code,seq });
    if (has(this.idleCmd.sset)) opts.push({ label: '盖放', action: 'sset', code,seq });
    opts.push(...await this.activateOptions(
      (this.idleCmd.activate || []).filter((a: any) => a.l === LOC_HAND && a.code === code && (seq === undefined || a.s === seq)),
      { action: 'activate', code, seq },
    ));
    return opts;
  }

  /**
   * 同一卡的可发动效果条目 → 菜单项（带精确 idx 应答）。单条直接标「发动」；
   * 多条按 desc 解析效果文本各列一项（原版 ShowSelectOption 的平铺：
   * event_handler.cpp:375-405 一卡多效果时列效果文本供选择）。
   */
  async activateOptions(matches: any[], base: any): Promise<any[]> {
    if (!matches.length) return [];
    if (matches.length === 1) return [{ ...base, label: '发动', idx: matches[0].idx }];
    return Promise.all(matches.map(async (m: any) => {
      const text = m.desc ? await WailsBridge.resolveDesc(m.desc) : '';
      return { ...base, label: text ? `发动：${text}` : '发动', idx: m.idx };
    }));
  }

  // Click on a 3D board card: offer activate (idle/battle), attack (battle
  // phase) or reposition (idle phase) when the engine's command list includes
  // that slot. 点击墓地/除外/额外堆区的卡 = 打开该堆区列表（原版 MSG_WAITING
  // 点堆区的 COMMAND_LIST 菜单，event_handler.cpp:1218-1237）。
  async onFieldCardClick(x: number, y: number, userData: any) {
    if (!this.interactive) return;
    const slot = userData && userData.slot;
    if (!slot) return;
    if (slot.loc === 'hand') {
      if (slot.player !== this.playerSlot) return;
      const opts = await this.getHandOptions(userData.cardCode,slot.seq);
      if (opts.length) duelStore.openActionPopup(x,y,opts);
      return;
    }
    if (slot.loc === 'grave' || slot.loc === 'banish' || slot.loc === 'extra') {
      this.onPileRightClick(slot.player, slot.loc);
      return;
    }
    const locCode = slot.loc === 'mzone' ? LOC_MZONE : LOC_SZONE;
    const opts: any[] = [];
    // 查看素材（原版点带素材的超量怪 → COMMAND_LIST，event_handler.cpp:1211-1216）：
    // 双方场上的素材怪都可看，排在询问动作之前
    const side = duelStore.getState().board[slot.player];
    const mats = side && side.overlay ? side.overlay[slot.seq] : null;
    if (mats && mats.length) {
      opts.push({ label: '查看素材', action: 'view_overlay', player: slot.player, seq: slot.seq });
    }
    if (slot.player !== this.playerSlot) {
      if (opts.length) duelStore.openActionPopup(x, y, opts);
      return;
    }
    // 发动（原版 BUTTON_CMD_ACTIVATE）：idle/战阶询问的可发动表按格匹配
    const actList = this.idleCmd ? this.idleCmd.activate
      : this.battleCmd ? this.battleCmd.activate : null;
    if (actList) {
      opts.push(...await this.activateOptions(
        actList.filter((a: any) => a.c === slot.player && a.l === locCode && a.s === slot.seq),
        { action: 'activate', code: userData.cardCode },
      ));
    }
    if (this.battleCmd && this.battleCmd.attack &&
        this.battleCmd.attack.some((a: any) => a.c === slot.player && a.s === slot.seq)) {
      opts.push({ label: '攻击', action: 'attack', s: slot.seq });
    }
    if (this.idleCmd && this.idleCmd.repos &&
        this.idleCmd.repos.some((r: any) => r.c === slot.player && r.s === slot.seq)) {
      opts.push({ label: '表示形式变更', action: 'repos', s: slot.seq });
    }
    if (opts.length) duelStore.openActionPopup(x, y, opts);
  }

  // Sends the player's action to the engine as the ocgcore int32 answer
  // (action type in the low 16 bits, list index in the high 16). Also emits
  // duel:player_action so the practice-mode AI simulator (which has no
  // engine to answer) can drive its own state machine.
  handlePlayerAction(actionType: string, data: any) {
    if (actionType === 'view_overlay') {
      eventBus.emit('ui:show_pile', { player: data.player, pile: 'overlay', seq: data.seq });
      return;
    }
    // idlecmd/battlecmd 的整型响应编码 ((idx<<16)|type) 已收回 Go
    // （responses.go RespondIdleCmd/RespondBattleCmd）。
    const respond = (list: any, cmdType: number, predicate?: (c: any) => boolean) => {
      if (!list) return;
      const match = list.find((c: any) => (predicate ? predicate(c) : c.code === data.code && (data.seq === undefined || c.s === data.seq)));
      if (match) {
        if (this.battleCmd) WailsBridge.respondBattleCmd(match.idx, cmdType);
        else WailsBridge.respondIdleCmd(match.idx, cmdType);
      }
    };

    if (actionType === 'phase_change') {
      if (this.battleCmd) {
        // Battle-phase confirms: cmdType 2 = to Main Phase 2, 3 = to End Phase.
        if (data === 'M2' && this.battleCmd.toM2) WailsBridge.respondBattleCmd(0, 2);
        else if (data === 'EP' && this.battleCmd.toEP) WailsBridge.respondBattleCmd(0, 3);
      } else if (this.idleCmd) {
        // Idle-phase confirms: cmdType 6 = to Battle Phase, 7 = to End Phase.
        if (data === 'BP' && this.idleCmd.toBP) WailsBridge.respondIdleCmd(0, 6);
        else if (data === 'EP' && this.idleCmd.toEP) WailsBridge.respondIdleCmd(0, 7);
      }
    } else if (actionType === 'summon') {
      respond(this.idleCmd && this.idleCmd.summon, 0);
    } else if (actionType === 'spsummon') {
      // 带 idx 的精确应答（堆区列表点选，原版 list_command 的 SPSUMMON 分支）
      if (data && typeof data.idx === 'number') WailsBridge.respondIdleCmd(data.idx, 1);
      else respond(this.idleCmd && this.idleCmd.spsummon, 1);
    } else if (actionType === 'repos') {
      // Field clicks carry only the slot (data.s), never a code — match by
      // slot like the attack branch, or the response is never sent.
      respond(this.idleCmd && this.idleCmd.repos, 2, (r) => r.s === data.s);
    } else if (actionType === 'mset') {
      respond(this.idleCmd && this.idleCmd.mset, 3);
    } else if (actionType === 'sset') {
      respond(this.idleCmd && this.idleCmd.sset, 4);
    } else if (actionType === 'activate') {
      // 带 idx 的精确应答（场上多效果条目 / 堆区列表 / 战阶手牌二速）；
      // 无 idx 的旧路径按 code+seq 匹配
      if (data && typeof data.idx === 'number') {
        if (this.battleCmd) WailsBridge.respondBattleCmd(data.idx, 0);
        else WailsBridge.respondIdleCmd(data.idx, 5);
      } else {
        respond(this.idleCmd && this.idleCmd.activate, 5);
      }
    } else if (actionType === 'attack') {
      // battlecmd 解码（playerop.cpp）：0=activate 1=attack 2=toM2 3=toEP
      respond(this.battleCmd && this.battleCmd.attack, 1, (a) => a.s === data.s);
    }

    eventBus.emit('duel:player_action', { type: actionType, data });
  }

  // ---- Step E：field3d 网格 ↔ store.board 重同步 ----

  // Compares the rendered mesh slots against duelStore.board; any drift
  // (missed/mis-decoded event) triggers a full instant rebuild from the
  // store snapshot. Undrifted duels pay only the comparison cost.
  // 场上卡无效化徽章同步（drawing.cpp:458-462）：按 store.board 的 status
  // 给 STATUS_DISABLED/FORBIDDEN 的表侧场卡挂/摘 negated 图标。
  syncNegatedBadges(): void {
    const state = duelStore.getState();
    const bottom = bottomEngineSeat(state);
    for (let disp = 0; disp < 2; disp++) {
      const seat = disp === 0 ? bottom : 1 - bottom;
      const sb = state.board[disp];
      this.field3D.syncNegatedBadges(seat, 'mzone', sb.mzone);
      this.field3D.syncNegatedBadges(seat, 'szone', sb.szone);
    }
  }

  // 连接怪数据同步（drawing.cpp DrawLinkedZones 的数据源）：把 store.board
  // 的 type/linkMarker（QUERY_TYPE/QUERY_LINK，update_data 携带）写到对应
  // mesh 的 userData；只有主怪兽区需要（连接怪只存在于 mzone）。悬停高亮
  // 本身由 field3d.updateLinkedZones 按 hoveredCard 重算。
  // 同时写入攻守/等级字段（DrawStatus 的数据源），末尾刷新状态浮签与牌堆计数。
  syncLinkData(): void {
    const state = duelStore.getState();
    const bottom = bottomEngineSeat(state);
    // 防御：smoke 的桩 field 没有 cardsOnField/syncStatusLabels 等
    const cof = this.field3D.cardsOnField;
    if (cof) {
    for (let disp = 0; disp < 2; disp++) {
      const seat = disp === 0 ? bottom : 1 - bottom;
      const zone = (cof[seat] || {}).mzone || [];
      const cards = state.board[disp].mzone;
      for (let i = 0; i < zone.length; i++) {
        const mesh = zone[i];
        if (!mesh) continue;
        const card = cards[i] || null;
        mesh.userData.cardType = card ? (card.type || 0) : 0;
        mesh.userData.linkMarker = card ? (card.linkMarker || 0) : 0;
        mesh.userData.link = card ? (card.link || 0) : 0;
        mesh.userData.cardPos = card ? card.pos : 0;
        mesh.userData.attack = card ? card.attack : undefined;
        mesh.userData.defense = card ? card.defense : undefined;
        mesh.userData.baseAttack = card ? card.baseAttack : undefined;
        mesh.userData.baseDefense = card ? card.baseDefense : undefined;
        mesh.userData.level = card ? card.level : undefined;
        mesh.userData.rank = card ? card.rank : undefined;
      }
    }
    }
    // DrawStatus（drawing.cpp:771-786）+ 牌堆计数（:724-769）浮签刷新。
    // store.piles 按显示座索引，field3d 的堆区按引擎 seat → 换算后再传。
    if (typeof this.field3D.syncStatusLabels === 'function')
      this.field3D.syncStatusLabels(state.playerSlot);
    const pilesBySeat = bottom === 0
      ? [state.piles[0], state.piles[1]]
      : [state.piles[1], state.piles[0]];
    if (typeof this.field3D.syncPileCounts === 'function')
      this.field3D.syncPileCounts(pilesBySeat);
    this.field3D.syncOverlays?.(bottom===0?[state.board[0].overlay,state.board[1].overlay]:[state.board[1].overlay,state.board[0].overlay]);
  }

  async resyncFromStore(): Promise<void> {
    if (this._resyncing) return;
    const state = duelStore.getState();
    // 显示座 → 引擎 seat（viewSwapped 时下侧是对面 seat）
    const bottom = bottomEngineSeat(state);
    const seatOf = (disp: number) => (disp === 0 ? bottom : 1 - bottom);
    const posState = (pos: number) => this.field3D.positionStateFor(pos);

    this.syncNegatedBadges();

    let drift = false;
    const check = (disp: number) => {
      const seat = seatOf(disp);
      const fc = this.field3D.cardsOnField[seat] || {};
      const sb = state.board[disp];
      for (const loc of ['mzone', 'szone'] as const) {
        const meshes = fc[loc] || [];
        const cards = sb[loc];
        const len = Math.max(cards.length, meshes.length);
        for (let i = 0; i < len; i++) {
          const card = cards[i] || null;
          const mesh = meshes[i] || null;
          if (!card && !mesh) continue;
          if (card && mesh && mesh.userData.cardCode === card.code
              && mesh.userData.positionState === posState(card.pos)) continue;
          drift = true;
        }
      }
      for (const loc of ['grave', 'banish'] as const) {
        const meshes = (fc[loc] || []).filter(Boolean);
        const stack = sb[loc];
        if (meshes.length !== stack.length
            || meshes.some((m: any, i: number) => m.userData.cardCode !== stack[i].code)) drift = true;
      }
    };
    check(0);
    check(1);
    if (!drift) return;

    this._resyncing = true;
    try {
      this.field3D.clearBoard();
      // 整板重建：旧的装备/对象关系随 mesh 一并作废
      this.equipPairs.clear();
      this.targetPairs.clear();
      this.chainTargets = [];
      for (let disp = 0; disp < 2; disp++) {
        const seat = seatOf(disp);
        const sb = state.board[disp];
        const rebuild = async (loc: string, cards: any[]) => {
          for (let i = 0; i < cards.length; i++) {
            const c = cards[i];
            if (!c) continue;
            const info = await WailsBridge.getCard(c.code);
            this.field3D.placeCard(seat, loc, i, c.code, info, c.pos);
          }
        };
        await rebuild('mzone', sb.mzone);
        await rebuild('szone', sb.szone);
        await rebuild('grave', sb.grave);
        await rebuild('banish', sb.banish);
      }
      // 重建换了全部 mesh，徽章需按新 mesh 重挂
      this.syncNegatedBadges();
      this.syncHands();
      this.syncLinkData();
    } finally {
      this._resyncing = false;
      // A second query may arrive while card metadata is being loaded.
      // Reconcile the latest board instead of dropping that authoritative update.
      if(duelStore.getState().board!==state.board)await this.resyncFromStore();
    }
  }
}
