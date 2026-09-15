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
import {
  LOC_NAMES,
  LOC_DECK, LOC_HAND, LOC_EXTRA,
} from '../domain/constants.ts';

export class DuelManager {
  // field3d 将在棘轮最后一步 strict 化；在那之前以 any 桥接（字段面很宽）。
  field3D: any;
  /** 非交互模式（回放）不发送任何协议响应、不响应点击。 */
  interactive: boolean;
  playerSlot = 0;
  /** 对手手牌数（驱动 field3D 的对手手背行；自己的手牌在 store.hand） */
  opponentHandCount = 0;
  idleCmd: any = null;
  battleCmd: any = null;
  _resyncing = false;
  _subs: [string, (data: any) => void][] = [];

  constructor(field3D: any, { interactive = true }: { interactive?: boolean } = {}) {
    this.field3D = field3D;
    // 非交互模式（回放）不发送任何协议响应、不响应点击。
    this.interactive = interactive;

    this.initNetworkListeners();
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
      this.opponentHandCount = 0;
      this.field3D.setOpponentHandCount(0);
    });

    // MSG_RELOAD_FIELD（谜题布场）：对手手背行直接取快照里的手牌数
    sub('duel:reload_field', (data) => {
      const opp = data.players && data.players[1 - this.playerSlot];
      if (opp) {
        this.opponentHandCount = opp.hand || 0;
        this.field3D.setOpponentHandCount(this.opponentHandCount);
      }
    });

    sub('duel:draw', async (data) => {
      // 对手手背行是持久表示，计数在 __instant（seek 重建）下也要同步
      if (data.player !== this.playerSlot) {
        this.opponentHandCount += data.count;
        this.field3D.setOpponentHandCount(this.opponentHandCount);
      }
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

      // 对手手牌数：离手 -1、回手 +1（背面行的持久表示同步）
      if (fromLoc === 'hand' && data.pc !== this.playerSlot) {
        this.opponentHandCount -= 1;
        this.field3D.setOpponentHandCount(this.opponentHandCount);
      }
      if (toLoc === 'hand' && data.cc !== this.playerSlot) {
        this.opponentHandCount += 1;
        this.field3D.setOpponentHandCount(this.opponentHandCount);
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
        this.field3D.chainVisualizer.addChainLink(data.code, { player: data.cc, loc: locName, seq: data.cs }, cardInfo);
        // Activating a set field spell turns it face-up; its art becomes the
        // board background as soon as the chain anchor is announced.
        if (locName === 'szone' && data.cs === 5) {
          this.field3D.flipSzoneCard(data.cc, 5);
        }
      }
    });

    sub('duel:chain_solving', (data) => {
      this.field3D.chainVisualizer.highlightSolvingLink(data.count);
    });

    sub('duel:chain_solved', (data) => {
      this.field3D.chainVisualizer.removeSolvingLink(data.count);
    });

    sub('duel:chain_end', () => {
      this.field3D.chainVisualizer.clearChain();
    });

    sub('duel:select_idlecmd', (data) => {
      this.idleCmd = data;
      this.battleCmd = null;
    });

    sub('duel:select_battlecmd', (data) => {
      this.battleCmd = data;
      // The engine awaits a battle-command answer now; a stale idle command
      // would let the hand popup send idle-phase responses mid-battle.
      this.idleCmd = null;
    });

    // MSG_SELECT_PLACE：无 UI 的自动落点（gframe 自动选择语义：优先空区）。
    // Go 已把 flag 位域解码成语义化 zones:[{loc,seq}]（engine_bindings.go
    // decorateSelectPlace）。
    sub('duel:select_place', (data) => {
      if (!this.interactive) return;
      const owner = data.player;
      const zones = data.zones || [];
      const target = this.field3D.cardsOnField[owner];
      const locName: Record<number, string> = { 0x04: 'mzone', 0x08: 'szone' };
      const occupied = (z: { loc: number; seq: number }) => {
        const arr = target && target[locName[z.loc]];
        return !!(arr && arr[z.seq]);
      };
      const zone = zones.find((z: any) => !occupied(z)) || zones[0];
      if (!zone) return;
      WailsBridge.respondSelectPlace(owner, zone.loc, zone.seq);
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
    // 提示条（gframe MESG_HINT 的 stHintMsg 分支）；其余类型是选择提示/
    // 效果角标，不落 2D。
    sub('duel:hint', async (data) => {
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
    // 漂移时清盘重建（ Step E 的 store 重同步路径）。
    sub('duel:update_data', () => {
      this.resyncFromStore();
    });
  }

  // Unsubscribes every eventBus listener registered in initNetworkListeners.
  // Called by the hosting React component on unmount.
  dispose() {
    if (!this._subs) return;
    this._subs.forEach(([event, fn]) => eventBus.off(event, fn));
    this._subs = [];
  }

  // Builds the action popup options for a hand card from the pending idle
  // command: only actions the engine actually offered are shown.
  getHandOptions(code: number) {
    if (!this.idleCmd) return [];
    const opts = [];
    const has = (list: any) => list && list.some((c: any) => c.code === code);
    if (has(this.idleCmd.summon)) opts.push({ label: '通常召唤', action: 'summon', code });
    if (has(this.idleCmd.spsummon)) opts.push({ label: '特殊召唤', action: 'spsummon', code });
    if (has(this.idleCmd.mset)) opts.push({ label: '盖放怪兽', action: 'mset', code });
    if (has(this.idleCmd.sset)) opts.push({ label: '盖卡', action: 'sset', code });
    if (has(this.idleCmd.activate)) opts.push({ label: '发动效果', action: 'activate', code });
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
  async resyncFromStore(): Promise<void> {
    if (this._resyncing) return;
    const state = duelStore.getState();
    const seatOf = (disp: number) => (disp === 0 ? this.playerSlot : 1 - this.playerSlot);
    const posState = (pos: number) => this.field3D.positionStateFor(pos);

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
    } finally {
      this._resyncing = false;
    }
  }
}
