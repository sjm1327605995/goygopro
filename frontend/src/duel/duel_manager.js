/**
 * Duel Manager & Controller
 * Coordinates duel flow, bridges network packets to 3D Field animations, Chain Visualizer, and 2D HUD.
 */

import { WailsBridge, eventBus } from '../wails_bridge.js';

// ocgcore LOCATION_* bits, as they arrive in MSG_MOVE's pl/cl fields.
const LOC_DECK = 0x01;
const LOC_HAND = 0x02;
const LOC_MZONE = 0x04;
const LOC_SZONE = 0x08;
const LOC_EXTRA = 0x40;
const LOC_NAMES = {
  0x01: 'deck',
  0x02: 'hand',
  0x04: 'mzone',
  0x08: 'szone',
  0x10: 'grave',
  0x20: 'banish',
  0x40: 'extra',
  0x80: 'overlay'
};
const LOC_LABELS = {
  0x01: '卡组',
  0x02: '手牌',
  0x04: '场上',
  0x08: '场上',
  0x10: '墓地',
  0x20: '除外区',
  0x40: '额外卡组',
  0x80: '超量素材'
};

export class DuelManager {
  constructor(field3D, hud) {
    this.field3D = field3D;
    this.hud = hud;

    this.playerSlot = 0;
    this.idleCmd = null;
    this.battleCmd = null;

    this.initNetworkListeners();
  }

  initNetworkListeners() {
    // Every subscription is recorded so dispose() can unsubscribe: the
    // eventBus outlives the duel screen, and a re-entered duel must not stack
    // a second DuelManager that would apply each event twice.
    this._subs = [];
    const sub = (event, fn) => {
      eventBus.on(event, fn);
      this._subs.push([event, fn]);
    };

    sub('stoc:select_hand', () => {
      this.hud.showRPSModal((choice) => {
        WailsBridge.sendHandResult(choice);
      });
    });

    sub('duel:start', (data) => {
      this.hud.updateLP(0, data.lp0 || 8000);
      this.hud.updateLP(1, data.lp1 || 8000);
      this.hud.appendLog('决斗开始！', 'log-action');
    });

    sub('duel:draw', async (data) => {
      const isPlayer = data.player === this.playerSlot;
      for (let i = 0; i < data.count; i++) {
        const cardCode = (data.cards && data.cards[i]) ? data.cards[i] : 0;
        const cardInfo = cardCode ? await WailsBridge.getCard(cardCode) : null;

        this.field3D.animateDrawCard(data.player, cardCode, cardInfo, () => {
          if (isPlayer) {
            this.hud.addHandCard(cardCode, cardInfo);
          }
        });
      }
      this.hud.appendLog(`玩家 ${data.player} 抽了 ${data.count} 张卡。`);
    });

    sub('duel:new_turn', (data) => {
      this.hud.appendLog(`—— 玩家 ${data.player} 的回合 ——`, 'log-action');
    });

    sub('duel:new_phase', (data) => {
      this.hud.updatePhase(data.phase);
    });

    sub('duel:summoning', async (data) => {
      // The engine announces a summon without a hand→field move; drop the card
      // from the 2D hand dock ourselves (matches the reference client).
      if (data.cc === this.playerSlot) this.hud.removeHandCardByCode(data.code);
      const cardInfo = await WailsBridge.getCard(data.code);
      this.field3D.animateSummon(data.cc, data.code, data.cs, data.cp, cardInfo, false);
      this.hud.appendLog(`通常召唤：${cardInfo ? cardInfo.name : data.code}`, 'log-summon');
    });

    sub('duel:spsummoning', async (data) => {
      if (data.cc === this.playerSlot) this.hud.removeHandCardByCode(data.code);
      const cardInfo = await WailsBridge.getCard(data.code);
      this.field3D.animateSummon(data.cc, data.code, data.cs, data.cp, cardInfo, true);
      this.hud.appendLog(`特殊召唤：${cardInfo ? cardInfo.name : data.code}`, 'log-summon');
    });

    sub('duel:flipsummoning', async (data) => {
      const cardInfo = await WailsBridge.getCard(data.code);
      this.field3D.animateSummon(data.cc, data.code, data.cs, data.cp, cardInfo, false);
      this.hud.appendLog(`反转召唤：${cardInfo ? cardInfo.name : data.code}`, 'log-summon');
    });

    sub('duel:set', async (data) => {
      if (data.cc === this.playerSlot) this.hud.removeHandCardByCode(data.code);
      const isMonster = data.cl === 0x4;
      const cardInfo = await WailsBridge.getCard(data.code);
      this.field3D.animateSetCard(data.cc, data.code, data.cs, isMonster, cardInfo);
      this.hud.appendLog(`玩家 ${data.cc} 盖下了一张卡。`);
    });

    sub('duel:pos_change', (data) => {
      this.field3D.animateReposition(data.cc, data.cs, data.cp);
      this.hud.appendLog(`玩家 ${data.cc} 改变了槽位 ${data.cs} 卡片的表示形式。`);
    });

    // MSG_MOVE covers every relocation the summon/set events do not animate:
    // destroyed → grave, returned → deck/hand/extra, banished, and cards
    // detaching as Xyz material (overlay). Cards entering the field by plain
    // move (no SUMMONING/SET preamble) are placed directly.
    sub('duel:move', async (data) => {
      const fromLoc = LOC_NAMES[data.pl];
      const toLoc = LOC_NAMES[data.cl];

      const mesh = (fromLoc && data.pl !== LOC_HAND && data.pl !== LOC_DECK && data.pl !== LOC_EXTRA)
        ? this.field3D.removeFromSlot(data.pc, fromLoc, data.ps)
        : null;

      // Anything leaving our hand (summon, set, discard, return) leaves the
      // 2D hand dock too.
      if (data.pl === LOC_HAND && data.pc === this.playerSlot && data.code) {
        this.hud.removeHandCardByCode(data.code);
      }

      if (mesh) {
        // Card was visibly on the board: animate it to its destination.
        if (toLoc === 'hand') {
          this.field3D.retireMesh(mesh);
          if (data.cc === this.playerSlot && data.code) {
            const cardInfo = await WailsBridge.getCard(data.code);
            this.hud.addHandCard(data.code, cardInfo);
          }
        } else {
          this.field3D.moveCard(mesh, data.cc, toLoc, data.cs, data.cp);
        }
      } else if ((toLoc === 'grave' || toLoc === 'banish') && data.code) {
        // Card left the board without a rendered mesh — materialize it in its
        // pile so grave/banish counts stay honest.
        const cardInfo = await WailsBridge.getCard(data.code);
        this.field3D.placeCard(data.cc, toLoc, data.cs, data.code, cardInfo, data.cp);
      } else if (toLoc === 'hand' && data.cc === this.playerSlot && data.code) {
        // Hand reshuffles (hand → hand) are skipped by the LOC_HAND source
        // guard on the mesh branch; other entries (e.g. from deck) add a card.
        if (data.pl !== LOC_HAND) {
          const cardInfo = await WailsBridge.getCard(data.code);
          this.hud.addHandCard(data.code, cardInfo);
        }
      }
      this.hud.appendLog(`卡牌移动（${LOC_LABELS[data.pl] || data.pl} → ${LOC_LABELS[data.cl] || data.cl}）。`);
    });

    sub('duel:lp_update', (data) => {
      // Absolute LP value, unlike the delta-based damage/recover events.
      this.hud.updateLP(data.player, data.lp);
    });

    sub('duel:attack', (data) => {
      this.field3D.animateAttack(
        data.attacker.c,
        data.attacker.s,
        data.target.c,
        data.target.s,
        data.target.l === 0
      );
      this.hud.appendLog(`宣告攻击！槽位 ${data.attacker.s} → 槽位 ${data.target.s}`, 'log-damage');
    });

    sub('duel:damage', (data) => {
      const currentLP = data.player === 0 ? this.hud.playerLP : this.hud.opponentLP;
      const newLP = currentLP - data.amount;
      this.hud.updateLP(data.player, newLP);
      this.field3D.cameraShake(0.3, 250);
      this.hud.appendLog(`玩家 ${data.player} 受到 ${data.amount} 点伤害！`, 'log-damage');
    });

    sub('duel:recover', (data) => {
      const currentLP = data.player === 0 ? this.hud.playerLP : this.hud.opponentLP;
      const newLP = currentLP + data.amount;
      this.hud.updateLP(data.player, newLP);
      this.hud.appendLog(`玩家 ${data.player} 回复了 ${data.amount} 点 LP！`, 'log-action');
    });

    // Chain Events
    sub('duel:chaining', async (data) => {
      // An activation from the hand is announced as chaining without a move;
      // keep the 2D hand dock in sync (matches the reference client).
      if (data.cc === this.playerSlot) this.hud.removeHandCardByCode(data.code);
      const cardInfo = await WailsBridge.getCard(data.code);
      const locName = data.cl === 0x4 ? 'mzone' : 'szone';
      this.field3D.chainVisualizer.addChainLink(data.code, { player: data.cc, loc: locName, seq: data.cs }, cardInfo);
      this.hud.appendLog(`连锁发动：${cardInfo ? cardInfo.name : data.code}！`, 'log-action');
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
      this.hud.setPhaseButtonsActionable(data.toBP, false, data.toEP);
    });

    sub('duel:select_battlecmd', (data) => {
      this.battleCmd = data;
      this.hud.setPhaseButtonsActionable(false, data.toM2, data.toEP);
    });

    sub('stoc:select_tp', () => {
      this.hud.showYesNoModal('先攻选择', '你先攻吗？（先攻的第一个回合不能攻击）', () => {
        WailsBridge.sendTPResult(1);
      }, () => {
        WailsBridge.sendTPResult(0);
      });
    });

    sub('duel:select_yesno', (data) => {
      // desc is an engine string id we cannot resolve without strings.conf;
      // show a generic confirmation instead of hanging the duel.
      this.hud.showYesNoModal('确认', '是否执行该操作？', () => {
        WailsBridge.sendResponseI(1);
      }, () => {
        WailsBridge.sendResponseI(0);
      });
    });

    sub('duel:select_option', (data) => {
      // Options carry unresolved desc ids; present them as numbered choices.
      const options = data.options || [];
      const cards = options.map((_, i) => ({ code: i, name: `选项 ${i + 1}` }));
      this.hud.showCardSelectModal('选择一个效果', cards, 1, 1, (selected) => {
        if (selected.length) WailsBridge.sendResponseI(selected[0]);
      });
    });

    sub('duel:select_chain', async (data) => {
      const chains = data.chains || [];
      // Empty chain list (or a forced-chain prompt the UI cannot resolve yet):
      // decline immediately so the duel never waits on us.
      if (!chains.length) {
        WailsBridge.sendResponseI(-1);
        return;
      }
      const cards = await Promise.all(chains.map(async (ch) => {
        const info = await WailsBridge.getCard(ch.code);
        return { ...ch, name: info ? info.name : `卡牌 #${ch.code}` };
      }));
      // A forced chain cannot be declined: answer with the first candidate.
      if (data.forced && cards.length === 1) {
        WailsBridge.sendResponseI(0);
        return;
      }
      if (cards.length === 1) {
        const name = cards[0].name;
        this.hud.showYesNoModal('连锁', `是否发动「${name}」的连锁效果？`, () => {
          WailsBridge.sendResponseI(0);
        }, () => {
          WailsBridge.sendResponseI(-1);
        });
      } else {
        this.hud.showCardSelectModal('选择要发动的效果', cards, 1, 1, (selected) => {
          if (selected.length) WailsBridge.sendResponseI(selected[0]);
        }, () => {
          WailsBridge.sendResponseI(-1);
        });
      }
    });

    sub('duel:select_place', (data) => {
      // Response is 3 bytes [player, location, seq]. The flag bitfield marks
      // selectable zones from the selecting player's perspective: bits 0x7f
      // monster zone (seq 0-4 + two EMZ), 0x3f00 spell/trap zone, 0xc000 the
      // two pendulum zones (szone seq 6/7 in the MR2020 layout).
      const flag = data.flag >>> 0;
      const owner = data.player;
      const target = this.field3D.cardsOnField[owner];
      const pickFrom = (bits, locName, count) => {
        const prefer = [];
        const fallback = [];
        for (let seq = 0; seq < count; seq++) {
          if (!(bits & (1 << seq))) continue;
          if (target && target[locName] && target[locName][seq]) fallback.push(seq);
          else prefer.push(seq);
        }
        return prefer.length ? prefer[0] : (fallback.length ? fallback[0] : -1);
      };

      let loc = LOC_MZONE;
      let seq = pickFrom(flag & 0x7f, 'mzone', 7);
      if (seq === -1) { loc = LOC_SZONE; seq = pickFrom((flag >>> 8) & 0x3f, 'szone', 6); }
      const pFlag = (flag >>> 14) & 0x3;
      if (seq === -1 && pFlag) { loc = LOC_SZONE; seq = (pFlag & 1) ? 6 : 7; }
      if (seq === -1) return;
      WailsBridge.sendResponseB([owner, loc, seq]);
    });

    sub('duel:select_effectyn', async (data) => {
      const cardInfo = await WailsBridge.getCard(data.code);
      const title = '发动卡牌效果？';
      const msg = `是否发动「${cardInfo ? cardInfo.name : data.code}」的效果？`;
      this.hud.showYesNoModal(title, msg, () => {
        WailsBridge.sendResponseI(1);
      }, () => {
        WailsBridge.sendResponseI(0);
      });
    });

    sub('duel:select_card', async (data) => {
      const cardsWithInfo = await Promise.all(data.cards.map(async (c) => {
        const info = await WailsBridge.getCard(c.code);
        return { ...c, name: info ? info.name : `Card #${c.code}` };
      }));

      this.hud.showCardSelectModal('选择卡牌', cardsWithInfo, data.min, data.max, (selectedIndices) => {
        const resp = new Uint8Array(1 + selectedIndices.length);
        resp[0] = selectedIndices.length;
        for (let i = 0; i < selectedIndices.length; i++) {
          resp[1 + i] = selectedIndices[i];
        }
        WailsBridge.sendResponseB(Array.from(resp));
      });
    });

    sub('duel:select_position', (data) => {
      this.hud.showPositionPickerModal(data.positions, (pos) => {
        WailsBridge.sendResponseI(pos);
      });
    });

    sub('duel:win', (data) => {
      const isWinner = data.winner === this.playerSlot;
      this.hud.showVictoryModal(isWinner);
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
  getHandOptions(code) {
    if (!this.idleCmd) return [];
    const opts = [];
    const has = (list) => list && list.some((c) => c.code === code);
    if (has(this.idleCmd.summon)) opts.push({ label: '通常召唤', action: 'summon', code });
    if (has(this.idleCmd.spsummon)) opts.push({ label: '特殊召唤', action: 'spsummon', code });
    if (has(this.idleCmd.mset)) opts.push({ label: '盖放怪兽', action: 'mset', code });
    if (has(this.idleCmd.sset)) opts.push({ label: '盖卡', action: 'sset', code });
    if (has(this.idleCmd.activate)) opts.push({ label: '发动效果', action: 'activate', code });
    return opts;
  }

  // Click on a 3D board card: offer attack (battle phase) or reposition
  // (idle phase) when the engine's command list includes that slot.
  onFieldCardClick(x, y, userData) {
    const slot = userData && userData.slot;
    if (!slot || slot.player !== this.playerSlot) return;
    const opts = [];
    if (this.battleCmd && this.battleCmd.attack &&
        this.battleCmd.attack.some((a) => a.s === slot.seq)) {
      opts.push({ label: '攻击', action: 'attack', s: slot.seq });
    }
    if (this.idleCmd && this.idleCmd.repos &&
        this.idleCmd.repos.some((r) => r.s === slot.seq)) {
      opts.push({ label: '表示形式变更', action: 'repos', s: slot.seq });
    }
    if (opts.length) this.hud.showActionPopup(x, y, opts);
  }

  // Sends the player's action to the engine as the ocgcore int32 answer
  // (action type in the low 16 bits, list index in the high 16). Also emits
  // duel:player_action so the practice-mode AI simulator (which has no
  // engine to answer) can drive its own state machine.
  handlePlayerAction(actionType, data) {
    const respond = (list, type, predicate) => {
      if (!list) return;
      const match = list.find((c) => (predicate ? predicate(c) : c.code === data.code));
      if (match) WailsBridge.sendResponseI((match.idx << 16) | type);
    };

    if (actionType === 'phase_change') {
      if (this.battleCmd) {
        // Battle-phase confirms: 2 = to Main Phase 2, 3 = to End Phase.
        if (data === 'M2' && this.battleCmd.toM2) WailsBridge.sendResponseI(2);
        else if (data === 'EP' && this.battleCmd.toEP) WailsBridge.sendResponseI(3);
      } else if (this.idleCmd) {
        // Idle-phase confirms: 6 = to Battle Phase, 7 = to End Phase.
        if (data === 'BP' && this.idleCmd.toBP) WailsBridge.sendResponseI(6);
        else if (data === 'EP' && this.idleCmd.toEP) WailsBridge.sendResponseI(7);
      }
    } else if (actionType === 'summon') {
      respond(this.idleCmd && this.idleCmd.summon, 0);
    } else if (actionType === 'spsummon') {
      respond(this.idleCmd && this.idleCmd.spsummon, 1);
    } else if (actionType === 'repos') {
      respond(this.idleCmd && this.idleCmd.repos, 2);
    } else if (actionType === 'mset') {
      respond(this.idleCmd && this.idleCmd.mset, 3);
    } else if (actionType === 'sset') {
      respond(this.idleCmd && this.idleCmd.sset, 4);
    } else if (actionType === 'activate') {
      respond(this.idleCmd && this.idleCmd.activate, 5);
    } else if (actionType === 'attack') {
      respond(this.battleCmd && this.battleCmd.attack, 1, (a) => a.s === data.s);
    }

    eventBus.emit('duel:player_action', { type: actionType, data });
  }
}
