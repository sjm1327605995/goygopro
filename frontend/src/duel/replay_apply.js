/**
 * Replay Applier
 * Applies recorded duel events directly to the 3D field + HUD, synchronously.
 *
 * The ReplayTheater drives this class instead of emitting events onto the
 * eventBus: playback (especially seek/step-back, which re-applies hundreds of
 * events in a tight loop) must not depend on async card-info fetches or on
 * whichever listener happens to be registered. Card info is preloaded up
 * front, so every apply() is fully synchronous — a seek is just
 * clearBoard() + N instant apply() calls.
 */

import { WailsBridge } from '../wails_bridge.js';

// ocgcore LOCATION_* bits as they arrive in MSG_MOVE's pl/cl fields.
const LOC_DECK = 0x01;
const LOC_HAND = 0x02;
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

// Collects every card code the event stream references so the caller can
// preload card info before synchronous playback.
export function collectCardCodes(events) {
  const codes = new Set();
  for (const evt of events || []) {
    const d = evt.data || {};
    if (d.code) codes.add(d.code);
    if (Array.isArray(d.cards)) {
      d.cards.forEach((c) => {
        if (!c) return;
        if (typeof c === 'object' && c.code) codes.add(c.code);
        else if (typeof c === 'number' && c) codes.add(c);
      });
    }
  }
  return [...codes];
}

export class ReplayApplier {
  constructor(field3D, hud, events) {
    this.field = field3D;
    this.hud = hud;
    this.events = events || [];
    this.cardInfo = new Map();
    this.step = 0;
    this.lp = [8000, 8000];
  }

  // Fetches info for every referenced card code into a local map so apply()
  // never awaits. WailsBridge memoizes, so repeated loads are cheap.
  async preload() {
    const codes = collectCardCodes(this.events);
    await Promise.all(codes.map(async (code) => {
      const info = code ? await WailsBridge.getCard(code) : null;
      this.cardInfo.set(code, info);
    }));
  }

  info(code) {
    return this.cardInfo.get(code) || null;
  }

  // Rebuilds the board from event 0 up to (not including) targetStep,
  // instantly. Backs step-back and clicking the progress bar.
  seek(targetStep) {
    const target = Math.max(0, Math.min(targetStep, this.events.length));
    this.reset();
    for (let i = 0; i < target; i++) {
      this.apply(this.events[i], { instant: true });
    }
    this.step = target;
    return target;
  }

  reset() {
    this.field.clearBoard();
    this.hud.clearHand();
    this.hud.clearLog();
    this.hud.hideModals();
    const startLp = (this.events[0] && this.events[0].type === 'duel:start' && this.events[0].data.lp0) || 8000;
    this.lp = [startLp, startLp];
    this.hud.updateLP(0, startLp, true);
    this.hud.updateLP(1, startLp, true);
    this.step = 0;
  }

  // Applies one event. `instant` snaps placements instead of tweening; tweens
  // still run for animated playback because apply() returns synchronously and
  // the field's animation loop drives them.
  apply(evt, { instant = false } = {}) {
    if (!evt) return;
    const d = evt.data || {};

    switch (evt.type) {
      case 'duel:start':
        this.lp = [d.lp0 || 8000, d.lp1 || 8000];
        this.hud.updateLP(0, this.lp[0], true);
        this.hud.updateLP(1, this.lp[1], true);
        this.hud.appendLog('决斗开始！', 'log-action');
        break;

      case 'duel:draw':
        for (let i = 0; i < (d.count || 0); i++) {
          const code = (d.cards && d.cards[i]) || 0;
          if (d.player === 0 && code) {
            this.hud.addHandCard(code, this.info(code));
          }
        }
        this.hud.appendLog(`玩家 ${d.player} 抽了 ${d.count} 张卡。`);
        break;

      case 'duel:new_turn':
        this.hud.appendLog(`—— 玩家 ${d.player} 的回合 ——`, 'log-action');
        break;

      case 'duel:new_phase':
        this.hud.updatePhase(d.phase);
        break;

      case 'duel:summoning':
      case 'duel:spsummoning':
      case 'duel:flipsummoning': {
        const info = this.info(d.code);
        const isSpecial = evt.type === 'duel:spsummoning';
        // The engine announces a summon without a hand→field move; keep the
        // 2D hand dock in sync ourselves.
        if (d.cc === 0 && evt.type !== 'duel:flipsummoning' && d.code) {
          this.hud.removeHandCardByCode(d.code);
        }
        if (instant) {
          this.field.placeCard(d.cc, 'mzone', d.cs, d.code, info, d.cp);
        } else {
          this.field.animateSummon(d.cc, d.code, d.cs, d.cp, info, isSpecial);
        }
        this.hud.appendLog(
          `${isSpecial ? '特殊召唤' : '通常召唤'}：${info ? info.name : d.code}`,
          'log-summon'
        );
        break;
      }

      case 'duel:set': {
        const isMonster = d.cl === 0x4;
        const info = this.info(d.code);
        if (d.cc === 0 && d.code) this.hud.removeHandCardByCode(d.code);
        if (instant) {
          this.field.placeCard(d.cc, isMonster ? 'mzone' : 'szone', d.cs, d.code, info, d.cp);
        } else {
          this.field.animateSetCard(d.cc, d.code, d.cs, isMonster, info);
        }
        this.hud.appendLog(`玩家 ${d.cc} 盖下了一张卡。`);
        break;
      }

      case 'duel:pos_change': {
        const mesh = this.field.cardsOnField[d.cc] && this.field.cardsOnField[d.cc].mzone[d.cs];
        if (mesh) {
          if (instant) {
            const rot = this.field.cardRotationFor(d.cc, d.cp);
            mesh.rotation.set(rot.x, rot.y, rot.z);
          } else {
            this.field.animateReposition(d.cc, d.cs, d.cp);
          }
        }
        this.hud.appendLog(`玩家 ${d.cc} 改变了槽位 ${d.cs} 卡片的表示形式。`);
        break;
      }

      case 'duel:move': {
        const fromLoc = LOC_NAMES[d.pl];
        const toLoc = LOC_NAMES[d.cl];
        // Hand/deck/extra sources have no board mesh; the summon/set events
        // that precede field entries have already placed the target mesh.
        let mesh = null;
        if (fromLoc && d.pl !== LOC_DECK && d.pl !== LOC_HAND && fromLoc !== 'extra') {
          mesh = this.field.removeFromSlot(d.pc, fromLoc, d.ps);
        }

        // Anything leaving player 0's hand (summon, set, discard, return)
        // leaves the 2D hand dock too.
        if (d.pl === LOC_HAND && d.pc === 0 && d.code) {
          this.hud.removeHandCardByCode(d.code);
        }

        if (mesh) {
          if (toLoc === 'hand') {
            this.field.retireMesh(mesh, instant);
            if (d.cc === 0 && d.code) {
              this.hud.addHandCard(d.code, this.info(d.code));
            }
          } else {
            this.field.moveCard(mesh, d.cc, toLoc, d.cs, d.cp, { instant });
          }
        } else if ((toLoc === 'grave' || toLoc === 'banish') && d.code) {
          // Card left the board without a mesh (moved by the engine while its
          // mesh was never rendered) — materialize it in its pile.
          this.field.placeCard(d.cc, toLoc, d.cs, d.code, this.info(d.code), d.cp);
        } else if (toLoc === 'hand' && d.cc === 0 && d.code && d.pl !== LOC_HAND) {
          // Hand reshuffles (hand → hand) stay untouched; other hand entries
          // (e.g. from deck) add a card to the dock.
          this.hud.addHandCard(d.code, this.info(d.code));
        }
        this.hud.appendLog(`卡牌移动（${LOC_LABELS[d.pl] || d.pl} → ${LOC_LABELS[d.cl] || d.cl}）。`);
        break;
      }

      case 'duel:chaining': {
        // Hand activations are announced as chaining without a move; keep the
        // 2D hand dock in sync.
        if (d.cc === 0 && d.code) this.hud.removeHandCardByCode(d.code);
        const slot = { player: d.cc, loc: d.cl === 0x4 ? 'mzone' : 'szone', seq: d.cs };
        this.field.chainVisualizer.addChainLink(d.code, slot, this.info(d.code));
        this.hud.appendLog(`连锁发动：${this.info(d.code) ? this.info(d.code).name : d.code}！`, 'log-action');
        break;
      }

      case 'duel:chain_solving':
        this.field.chainVisualizer.highlightSolvingLink(d.count);
        break;

      case 'duel:chain_solved':
        this.field.chainVisualizer.removeSolvingLink(d.count);
        break;

      case 'duel:chain_end':
        this.field.chainVisualizer.clearChain();
        break;

      case 'duel:attack':
        if (!instant) {
          this.field.animateAttack(
            d.attacker.c, d.attacker.s,
            d.target.c, d.target.s,
            d.target.l === 0
          );
        }
        this.hud.appendLog(`宣告攻击！槽位 ${d.attacker.s} → 槽位 ${d.target.s}`, 'log-damage');
        break;

      case 'duel:damage':
        this.setLP(d.player, (this.lp[d.player] || 0) - (d.amount || 0), instant);
        if (!instant) this.field.cameraShake(0.3, 250);
        this.hud.appendLog(`玩家 ${d.player} 受到 ${d.amount} 点伤害！`, 'log-damage');
        break;

      case 'duel:recover':
        this.setLP(d.player, (this.lp[d.player] || 0) + (d.amount || 0), instant);
        this.hud.appendLog(`玩家 ${d.player} 回复了 ${d.amount} 点 LP！`, 'log-action');
        break;

      case 'duel:lp_update':
        this.setLP(d.player, d.lp, instant);
        break;

      case 'duel:win':
        this.hud.appendLog(`玩家 ${d.winner} 获胜！`, 'log-action');
        if (!instant) this.hud.showVictoryModal(d.winner === 0);
        break;

      default:
        // summoning bookkeeping (summoned/spsummoned/chained/battle/waiting…)
        // and anything the viewer does not render have no visual effect.
        break;
    }
  }

  setLP(player, value, instant) {
    const clamped = Math.max(0, value);
    this.lp[player] = clamped;
    this.hud.updateLP(player, clamped, instant);
  }
}
