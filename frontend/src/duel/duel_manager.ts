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
import {
  LOC_NAMES,
  LOC_DECK, LOC_HAND, LOC_EXTRA,
  CARD_QUESTION, PHINT_DESC_ADD,
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
  /** 双座手牌数（引擎 seat 索引；驱动 field3D 的远侧手背行——显示座 1 一侧） */
  handCounts: [number, number] = [0, 0];
  idleCmd: any = null;
  battleCmd: any = null;
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

    this.initNetworkListeners();

    // store 联动：视角交换 → 相机/手背行翻转（回放也生效）；cardSelect
    // 选择态 → 场上高亮刷新 + 选满自动应答（仅交互模式应答）。
    this._lastViewSwapped = duelStore.getState().viewSwapped;
    this._storeUnsub = duelStore.subscribe(() => this.onStoreChange());
  }

  onStoreChange(): void {
    const st = duelStore.getState();
    if (st.viewSwapped !== this._lastViewSwapped) {
      this._lastViewSwapped = st.viewSwapped;
      this.field3D.setViewSwapped(st.viewSwapped);
      this.refreshFarHandRow();
    }
    const cs = st.cardSelect;
    const key = cs ? `${cs.kind}:${cs.cards.length}:${cs.selected.join(',')}` : '';
    if (key !== this._lastCardSelectKey) {
      this._lastCardSelectKey = key;
      this.refreshCardSelectMarks();
    }
    if (!this.interactive || !cs || cs.kind !== 'card' || !cs.selected.length) return;
    // 选满自动应答（原版 event_handler.cpp:1389-1398：达到 select_max，
    // 或已达 select_min 且可选卡全部被选中时立即 SendResponse）
    if (cs.selected.length >= cs.max
        || (cs.selected.length >= cs.min && cs.selected.length === cs.cards.length)) {
      respondCardSelection();
    }
  }

  /** 远侧（显示座 1）手背行：视角交换后远侧换 seat，重新落数 */
  refreshFarHandRow(): void {
    const st = duelStore.getState();
    const topSeat = 1 - bottomEngineSeat(st);
    this.field3D.setFarHandCount(this.handCounts[topSeat as 0 | 1] || 0);
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
      this.playerSlot = (data.playerType || 0) & 0x0f;
      this.handCounts = [0, 0];
      this.field3D.setFarHandCount(0);
      this.field3D.setCantCheckGrave(false);
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
      this.refreshFarHandRow();
    });

    // MSG_TAG_SWAP（TAG 队友换手）：该座的手牌数直接以本消息为准（原版
    // duelclient.cpp:3782 用 hcount 重排 dField.hand）；本队手牌由 reducer 处理
    sub('duel:tag_swap', (data) => {
      this.handCounts[data.player as 0 | 1] = data.handCount || 0;
      this.refreshFarHandRow();
    });

    sub('duel:draw', async (data) => {
      // 手背行是持久表示，计数在 __instant（seek 重建）下也要同步
      this.handCounts[data.player as 0 | 1] = (this.handCounts[data.player as 0 | 1] || 0) + data.count;
      this.refreshFarHandRow();
      if (data.__instant) return; // 回放 seek 重建：跳过发牌动画
      for (let i = 0; i < data.count; i++) {
        const cardCode = (data.cards && data.cards[i]) ? data.cards[i] : 0;
        const cardInfo = cardCode ? await WailsBridge.getCard(cardCode) : null;
        this.field3D.animateDrawCard(data.player, cardCode, cardInfo);
      }
    });

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

      // 手牌数：离手 -1、回手 +1（手背行的持久表示同步）
      if (fromLoc === 'hand') {
        this.handCounts[data.pc as 0 | 1] = Math.max(0, this.handCounts[data.pc as 0 | 1] - 1);
        this.refreshFarHandRow();
      }
      if (toLoc === 'hand') {
        this.handCounts[data.cc as 0 | 1] += 1;
        this.refreshFarHandRow();
      }

      const mesh = (fromLoc && data.pl !== LOC_HAND && data.pl !== LOC_DECK && data.pl !== LOC_EXTRA)
        ? this.field3D.removeFromSlot(data.pc, fromLoc, data.ps)
        : null;

      if (mesh) {
        // Card was visibly on the board: animate it to its destination.
        if (toLoc === 'hand') {
          this.field3D.retireMesh(mesh, !!data.__instant);
        } else {
          this.field3D.moveCard(mesh, data.cc, toLoc, data.cs, data.cp, { instant: !!data.__instant });
        }
      } else if ((toLoc === 'grave' || toLoc === 'banish') && data.code) {
        // Card left the board without a rendered mesh — materialize it in its
        // pile so grave/banish counts stay honest.
        const cardInfo = await WailsBridge.getCard(data.code);
        this.field3D.placeCard(data.cc, toLoc, data.cs, data.code, cardInfo, data.cp);
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

    sub('duel:damage', (data) => {
      if (data.__instant) return;
      this.field3D.cameraShake(0.3, 250);
    });

    // Chain Events
    sub('duel:chaining', async (data) => {
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

    // Go 载荷的位置条目是 {c,l,s}，field3d 的槽参数是 {player,loc,seq}
    const toSlot = (e: any) => ({ player: e.c, loc: e.l, seq: e.s });

    sub('duel:equip', (data) => {
      this.field3D.addRelationLine(toSlot(data.card), toSlot(data.target), 'equip');
    });

    sub('duel:card_target', (data) => {
      this.field3D.addRelationLine(toSlot(data.card), toSlot(data.target), 'target');
    });

    sub('duel:cancel_target', (data) => {
      this.field3D.removeRelationLine(toSlot(data.card), toSlot(data.target), 'target');
    });

    sub('duel:unequip', (data) => {
      this.field3D.removeRelationLine(toSlot(data.card), undefined, 'equip');
    });

    sub('duel:add_counter', (data) => {
      this.field3D.applyCounterDelta(data.c, data.l, data.s, data.type, data.count);
    });

    sub('duel:remove_counter', (data) => {
      this.field3D.applyCounterDelta(data.c, data.l, data.s, data.type, -data.count);
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
      if (text) duelStore.setHint(text);
    });

    // ---- 2D UI 动作接线（HandDock / ActionPopup 发出）----

    sub('ui:hand_click', (data) => {
      if (!this.interactive) return;
      const opts = this.getHandOptions(data.code);
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
    if (this._storeUnsub) {
      this._storeUnsub();
      this._storeUnsub = null;
    }
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
    duelStore.closeActionPopup();
  }

  /** 右键点在墓地/除外/额外堆区：快速查看列表（F1-F8 同源的 wCardDisplay） */
  onPileRightClick(player: number, pile: string): void {
    if (!this.interactive) return;
    if (pile === 'deck') return; // 卡组无列表数据（原版 F 键同样不含卡组）
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
  // command: only actions the engine actually offered are shown.
  getHandOptions(code: number) {
    if (!this.idleCmd) return [];
    const opts = [];
    const has = (list: any) => list && list.some((c: any) => c.code === code);
    if (has(this.idleCmd.summon)) opts.push({ label: '召唤', action: 'summon', code });
    if (has(this.idleCmd.spsummon)) opts.push({ label: '特殊召唤', action: 'spsummon', code });
    if (has(this.idleCmd.mset)) opts.push({ label: '盖放', action: 'mset', code });
    if (has(this.idleCmd.sset)) opts.push({ label: '盖放', action: 'sset', code });
    if (has(this.idleCmd.activate)) opts.push({ label: '发动', action: 'activate', code });
    return opts;
  }

  // Click on a 3D board card: offer attack (battle phase) or reposition
  // (idle phase) when the engine's command list includes that slot.
  onFieldCardClick(x: number, y: number, userData: any) {
    if (!this.interactive) return;
    const slot = userData && userData.slot;
    if (!slot || slot.player !== this.playerSlot) return;
    const opts = [];
    if (this.battleCmd && this.battleCmd.attack &&
        this.battleCmd.attack.some((a: any) => a.s === slot.seq)) {
      opts.push({ label: '攻击', action: 'attack', s: slot.seq });
    }
    if (this.idleCmd && this.idleCmd.repos &&
        this.idleCmd.repos.some((r: any) => r.s === slot.seq)) {
      opts.push({ label: '表示形式变更', action: 'repos', s: slot.seq });
    }
    if (opts.length) duelStore.openActionPopup(x, y, opts);
  }

  // Sends the player's action to the engine as the ocgcore int32 answer
  // (action type in the low 16 bits, list index in the high 16). Also emits
  // duel:player_action so the practice-mode AI simulator (which has no
  // engine to answer) can drive its own state machine.
  handlePlayerAction(actionType: string, data: any) {
    // idlecmd/battlecmd 的整型响应编码 ((idx<<16)|type) 已收回 Go
    // （responses.go RespondIdleCmd/RespondBattleCmd）。
    const respond = (list: any, cmdType: number, predicate?: (c: any) => boolean) => {
      if (!list) return;
      const match = list.find((c: any) => (predicate ? predicate(c) : c.code === data.code));
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
      respond(this.idleCmd && this.idleCmd.spsummon, 1);
    } else if (actionType === 'repos') {
      // Field clicks carry only the slot (data.s), never a code — match by
      // slot like the attack branch, or the response is never sent.
      respond(this.idleCmd && this.idleCmd.repos, 2, (r) => r.s === data.s);
    } else if (actionType === 'mset') {
      respond(this.idleCmd && this.idleCmd.mset, 3);
    } else if (actionType === 'sset') {
      respond(this.idleCmd && this.idleCmd.sset, 4);
    } else if (actionType === 'activate') {
      respond(this.idleCmd && this.idleCmd.activate, 5);
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
  syncLinkData(): void {
    const state = duelStore.getState();
    const bottom = bottomEngineSeat(state);
    for (let disp = 0; disp < 2; disp++) {
      const seat = disp === 0 ? bottom : 1 - bottom;
      const zone = (this.field3D.cardsOnField[seat] || {}).mzone || [];
      const cards = state.board[disp].mzone;
      for (let i = 0; i < zone.length; i++) {
        const mesh = zone[i];
        if (!mesh) continue;
        const card = cards[i] || null;
        mesh.userData.cardType = card ? (card.type || 0) : 0;
        mesh.userData.linkMarker = card ? (card.linkMarker || 0) : 0;
      }
    }
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
    } finally {
      this._resyncing = false;
    }
  }
}
