/**
 * AI Practice Duel Simulator Engine
 *
 * A lightweight standalone duel rule engine for practice mode (no server, no
 * ocgcore). It emits the same eventBus events the Go engine produces, and it
 * answers the player's actions (duel:player_action, emitted by DuelManager)
 * by mutating its own state — summon, set, activate, reposition, battle and
 * phase changes all drive real state transitions, so the 3D field and HUD
 * behave exactly as they do in a network duel.
 */

import { WailsBridge, eventBus } from '../wails_bridge.js';

const POS_FACEUP_ATTACK = 0x1;
const POS_FACEUP_DEFENSE = 0x4;
const POS_FACEDOWN_DEFENSE = 0x8;

const TYPE_MONSTER = 0x1;
const TYPE_SPELL = 0x2;
const TYPE_TRAP = 0x4;

const LOC_HAND = 0x02;
const LOC_MZONE = 0x04;
const LOC_SZONE = 0x08;
const LOC_GRAVE = 0x10;

export class AISimulator {
  constructor() {
    this.running = false;
    this.turn = 1;
    this.turnPlayer = 0;
    this.DELAY = 600; // pacing between engine steps; smoke tests shrink this

    this._cardInfo = new Map();
    this._onAction = (payload) => this.handleAction(payload);
    eventBus.on('duel:player_action', this._onAction);
  }

  dispose() {
    eventBus.off('duel:player_action', this._onAction);
  }

  // Memoized card info so prompt building stays synchronous after preload.
  async info(code) {
    if (!code) return null;
    if (!this._cardInfo.has(code)) {
      this._cardInfo.set(code, await WailsBridge.getCard(code));
    }
    return this._cardInfo.get(code);
  }

  isMonster(info) { return !!(info && (info.type & TYPE_MONSTER)); }
  isSpell(info) { return !!(info && (info.type & TYPE_SPELL)); }
  isTrap(info) { return !!(info && (info.type & TYPE_TRAP)); }

  state(player) { return player === 0 ? this.playerState : this.aiState; }

  freeSlot(arr) { return arr.findIndex((s) => s === null); }

  delay(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms ?? this.DELAY));
  }

  async start() {
    if (this.running) return;
    this.running = true;
    this.turn = 1;
    this.turnPlayer = 0;
    this.currentPhase = 0x01;

    this.playerState = {
      lp: 8000,
      deck: [89631139, 46986414, 38033121, 83764718, 55144522, 12580477, 5318639, 44095762, 41420027, 14558127, 84013237],
      hand: [],
      mzone: [null, null, null, null, null],
      szone: [null, null, null, null, null],
      grave: []
    };
    this.aiState = {
      lp: 8000,
      deck: [89631139, 46986414, 84013237, 44095762, 12580477, 55144522, 83764718, 41420027],
      hand: [],
      mzone: [null, null, null, null, null],
      szone: [null, null, null, null, null],
      grave: []
    };

    // Preload info for every card either player can ever touch.
    await Promise.all([...this.playerState.deck, ...this.aiState.deck].map((c) => this.info(c)));

    eventBus.emit('duel:start', {
      playerType: 0,
      duelRule: 5,
      lp0: 8000,
      lp1: 8000,
      deck0: this.playerState.deck.length,
      deck1: this.aiState.deck.length
    });

    await this.delay(400);
    this.draw(0, 5);
    this.draw(1, 5);
    await this.delay(400);
    this.startTurn(0);
  }

  // Deals `count` cards to `player`, emitting the draw event (real codes for
  // the local player, face-down zeros for the AI). Deck-out loses the duel.
  draw(player, count) {
    const st = this.state(player);
    const drawn = [];
    for (let i = 0; i < count; i++) {
      if (!st.deck.length) { this.declareWin(1 - player); return; }
      const code = st.deck.shift();
      st.hand.push(code);
      drawn.push(code);
    }
    eventBus.emit('duel:draw', {
      player,
      count,
      cards: player === 0 ? drawn : drawn.map(() => 0)
    });
  }

  setPhase(phase) {
    this.currentPhase = phase;
    eventBus.emit('duel:new_phase', { phase });
  }

  startTurn(player) {
    if (!this.running) return;
    this.turnPlayer = player;
    eventBus.emit('duel:new_turn', { player });
    const st = this.state(player);
    st.mzone.forEach((m) => { if (m) m.attacked = false; });

    this.setPhase(0x01); // DP
    if (this.turn > 1) this.draw(player, 1);

    this.delay().then(() => {
      this.setPhase(0x02); // SP
      return this.delay();
    }).then(() => {
      this.setPhase(0x04); // M1
      if (player === 0) this.promptIdle();
      else this.aiMainPhase();
    });
  }

  // ------- Player interaction (driven by duel:player_action) -------

  handleAction({ type, data }) {
    if (!this.running || this.turnPlayer !== 0) return;
    switch (type) {
      case 'summon': this.playerSummon(data.code, POS_FACEUP_ATTACK, false); break;
      case 'spsummon': this.playerSummon(data.code, POS_FACEUP_ATTACK, true); break;
      case 'mset': this.playerSummon(data.code, POS_FACEDOWN_DEFENSE, false); break;
      case 'sset': this.playerSetSpellTrap(data.code); break;
      case 'activate': this.playerActivate(data.code); break;
      case 'repos': this.playerReposition(data.s); break;
      case 'attack': this.resolveBattle(0, data.s); break;
      case 'phase_change':
        if (data === 'BP' && this.currentPhase === 0x04) {
          this.setPhase(0x08); // BP
          this.promptBattle();
        } else if (data === 'M2' && this.currentPhase === 0x08) {
          this.setPhase(0x10); // M2
          this.promptIdle();
        } else if (data === 'EP') {
          this.endPlayerTurn();
        }
        break;
      default: break;
    }
  }

  playerSummon(code, pos, isSpecial) {
    const st = this.playerState;
    const handIdx = st.hand.indexOf(code);
    if (handIdx === -1) return;
    const slot = this.freeSlot(st.mzone);
    if (slot === -1) return;

    st.hand.splice(handIdx, 1);
    st.mzone[slot] = { code, pos, attacked: false };
    if (isSpecial) {
      eventBus.emit('duel:spsummoning', { code, cc: 0, cl: LOC_MZONE, cs: slot, cp: pos });
    } else if (pos === POS_FACEDOWN_DEFENSE) {
      eventBus.emit('duel:set', { code, cc: 0, cl: LOC_MZONE, cs: slot, cp: pos });
    } else {
      eventBus.emit('duel:summoning', { code, cc: 0, cl: LOC_MZONE, cs: slot, cp: pos });
    }
    this.delay(300).then(() => this.promptIdle());
  }

  playerSetSpellTrap(code) {
    const st = this.playerState;
    const handIdx = st.hand.indexOf(code);
    if (handIdx === -1) return;
    const slot = this.freeSlot(st.szone);
    if (slot === -1) return;

    st.hand.splice(handIdx, 1);
    st.szone[slot] = { code, pos: 0xa, faceDown: true };
    eventBus.emit('duel:set', { code, cc: 0, cl: LOC_SZONE, cs: slot, cp: 0xa });
    this.delay(300).then(() => this.promptIdle());
  }

  async playerActivate(code) {
    const st = this.playerState;
    const handIdx = st.hand.indexOf(code);
    if (handIdx === -1) return;
    const slot = this.freeSlot(st.szone);
    if (slot === -1) return;

    st.hand.splice(handIdx, 1);
    st.szone[slot] = { code, pos: POS_FACEUP_ATTACK, faceDown: false };
    eventBus.emit('duel:chaining', { code, cc: 0, cl: LOC_SZONE, cs: slot, cp: POS_FACEUP_ATTACK, desc: 0 });

    // A handful of well-known simple spells resolve for real; anything else
    // stays on the field face-up without an effect (practice-mode simplification).
    const info = this._cardInfo.get(code);
    if (code === 55144522) { // Pot of Greed
      this.draw(0, 2);
    } else if (code === 12580477) { // Raigeki
      this.aiState.mzone.forEach((m, seq) => {
        if (m) this.destroyCard(1, LOC_MZONE, seq);
      });
    } else if (code === 83764718) { // Monster Reborn
      const graveIdx = this.playerState.grave.findIndex((g) => this.isMonster(this._cardInfo.get(g.code)));
      if (graveIdx !== -1) {
        const entry = this.playerState.grave.splice(graveIdx, 1)[0];
        const spawnSlot = this.freeSlot(this.playerState.mzone);
        if (spawnSlot !== -1) {
          this.playerState.mzone[spawnSlot] = { code: entry.code, pos: POS_FACEUP_ATTACK, attacked: true };
          eventBus.emit('duel:spsummoning', { code: entry.code, cc: 0, cl: LOC_MZONE, cs: spawnSlot, cp: POS_FACEUP_ATTACK });
        }
      }
    }

    await this.delay(500);
    eventBus.emit('duel:chain_solving', { count: 1 });
    await this.delay(300);
    eventBus.emit('duel:chain_solved', { count: 1 });
    eventBus.emit('duel:chain_end', { count: 0 });
    this.promptIdle();
  }

  playerReposition(seq) {
    const m = this.playerState.mzone[seq];
    if (!m || (m.pos & 0xa)) return;
    m.pos = (m.pos & 0xc) ? POS_FACEUP_ATTACK : POS_FACEUP_DEFENSE;
    eventBus.emit('duel:pos_change', { cc: 0, cs: seq, cp: m.pos });
    this.delay(300).then(() => this.promptIdle());
  }

  promptIdle() {
    if (!this.running || this.turnPlayer !== 0) return;
    const st = this.playerState;
    const summon = [];
    const spsummon = [];
    const repos = [];
    const mset = [];
    const sset = [];
    const activate = [];
    const mFree = this.freeSlot(st.mzone) !== -1;
    const sFree = this.freeSlot(st.szone) !== -1;

    st.hand.forEach((code, idx) => {
      const info = this._cardInfo.get(code);
      if (this.isMonster(info)) {
        if (mFree) {
          if ((info.level || 0) <= 4) {
            summon.push({ code, c: 0, l: LOC_HAND, s: idx, idx: summon.length });
          }
          mset.push({ code, c: 0, l: LOC_HAND, s: idx, idx: mset.length });
        }
      } else if (this.isSpell(info) && sFree) {
        activate.push({ code, c: 0, l: LOC_HAND, s: idx, desc: 0, idx: activate.length });
        sset.push({ code, c: 0, l: LOC_HAND, s: idx, idx: sset.length });
      } else if (this.isTrap(info) && sFree) {
        sset.push({ code, c: 0, l: LOC_HAND, s: idx, idx: sset.length });
      }
    });

    st.mzone.forEach((m, seq) => {
      if (m && !(m.pos & 0xa)) {
        repos.push({ code: m.code, c: 0, l: LOC_MZONE, s: seq, idx: repos.length });
      }
    });

    eventBus.emit('duel:select_idlecmd', {
      player: 0,
      summon,
      spsummon,
      repos,
      mset,
      sset,
      activate,
      toBP: this.currentPhase === 0x04 && this.turn > 1,
      toEP: true,
      shuffle: false
    });
  }

  promptBattle() {
    if (!this.running || this.turnPlayer !== 0) return;
    const st = this.playerState;
    const attack = [];
    st.mzone.forEach((m, seq) => {
      if (m && !(m.pos & 0xc) && !(m.pos & 0xa) && !m.attacked) {
        attack.push({ code: m.code, c: 0, l: LOC_MZONE, s: seq, diratt: true, idx: attack.length });
      }
    });
    eventBus.emit('duel:select_battlecmd', {
      player: 0,
      activate: [],
      attack,
      toM2: true,
      toEP: true
    });
  }

  endPlayerTurn() {
    if (!this.running) return;
    this.setPhase(0x20); // EP
    this.turn++;
    this.delay().then(() => this.startTurn(1));
  }

  // ------- Battle resolution (shared by both sides) -------

  // Effective battle value of a field monster: ATK in face-up attack,
  // DEF in face-up defense, and the true DEF for face-down (the practice AI
  // is allowed to peek; the player just sees the flip).
  battleValue(entry) {
    const info = this._cardInfo.get(entry.code);
    if (!info) return { val: 0, isDef: false };
    const isDef = !!(entry.pos & 0xc);
    return { val: isDef ? (info.defense || 0) : (info.attack || 0), isDef };
  }

  destroyCard(player, loc, seq) {
    const st = this.state(player);
    if (loc === LOC_MZONE) {
      const m = st.mzone[seq];
      if (!m) return;
      st.mzone[seq] = null;
      st.grave.push({ code: m.code });
      eventBus.emit('duel:move', {
        code: m.code, pc: player, pl: LOC_MZONE, ps: seq,
        cc: player, cl: LOC_GRAVE, cs: st.grave.length - 1, cp: 0x1, reason: 0x1100
      });
    } else {
      const s = st.szone[seq];
      if (!s) return;
      st.szone[seq] = null;
      st.grave.push({ code: s.code });
      eventBus.emit('duel:move', {
        code: s.code, pc: player, pl: LOC_SZONE, ps: seq,
        cc: player, cl: LOC_GRAVE, cs: st.grave.length - 1, cp: 0x1, reason: 0x1100
      });
    }
  }

  applyDamage(player, amount) {
    if (amount <= 0) return;
    const st = this.state(player);
    st.lp -= amount;
    eventBus.emit('duel:damage', { player, amount: st.lp < 0 ? amount + st.lp : amount });
    if (st.lp <= 0) this.declareWin(1 - player);
  }

  declareWin(winner) {
    if (!this.running) return;
    this.running = false;
    eventBus.emit('duel:win', { winner, type: 0 });
  }

  // Declares and resolves one attack: `attackerPlayer`'s monster in
  // `attackerSeq` attacks the best target (weakest) or directly.
  resolveBattle(attackerPlayer, attackerSeq) {
    if (!this.running) return;
    const atkSt = this.state(attackerPlayer);
    const defPlayer = 1 - attackerPlayer;
    const defSt = this.state(defPlayer);
    const attacker = atkSt.mzone[attackerSeq];
    if (!attacker || (attacker.pos & 0xa) || attacker.attacked) return;
    const atkInfo = this._cardInfo.get(attacker.code);
    if (!atkInfo) return;
    attacker.attacked = true;

    // Target: weakest opposing face-up monster; direct attack if none.
    let targetSeq = -1;
    let bestVal = Infinity;
    defSt.mzone.forEach((m, seq) => {
      if (!m || (m.pos & 0xa)) return; // skip face-down for simplicity
      const bv = this.battleValue(m);
      if (bv.val < bestVal) { bestVal = bv.val; targetSeq = seq; }
    });

    const direct = targetSeq === -1;
    eventBus.emit('duel:attack', {
      attacker: { c: attackerPlayer, l: LOC_MZONE, s: attackerSeq },
      target: { c: defPlayer, l: direct ? 0 : LOC_MZONE, s: direct ? 0 : targetSeq }
    });

    this.delay(900).then(() => {
      if (!this.running) return;
      if (direct) {
        this.applyDamage(defPlayer, atkInfo.attack || 0);
      } else {
        const target = defSt.mzone[targetSeq];
        if (!target) return; // gone (shouldn't happen in practice mode)
        const tv = this.battleValue(target);
        if (tv.isDef) {
          if ((atkInfo.attack || 0) > tv.val) {
            this.destroyCard(defPlayer, LOC_MZONE, targetSeq);
          } else if ((atkInfo.attack || 0) < tv.val) {
            this.applyDamage(attackerPlayer, tv.val - (atkInfo.attack || 0));
          }
        } else {
          if ((atkInfo.attack || 0) > tv.val) {
            this.destroyCard(defPlayer, LOC_MZONE, targetSeq);
            this.applyDamage(defPlayer, (atkInfo.attack || 0) - tv.val);
          } else if ((atkInfo.attack || 0) < tv.val) {
            this.destroyCard(attackerPlayer, LOC_MZONE, attackerSeq);
            this.applyDamage(attackerPlayer, tv.val - (atkInfo.attack || 0));
          } else {
            this.destroyCard(defPlayer, LOC_MZONE, targetSeq);
            this.destroyCard(attackerPlayer, LOC_MZONE, attackerSeq);
          }
        }
      }
      if (!this.running) return;
      // Re-prompt: the attacker's side may attack again or move on.
      if (this.turnPlayer === 0) this.promptBattle();
    });
  }

  // ------- AI turn -------

  async aiMainPhase() {
    if (!this.running || this.turnPlayer !== 1) return;
    const st = this.aiState;

    // Summon the strongest monster face-up, or set the strongest if the board
    // is full — otherwise set a spell/trap.
    const slot = this.freeSlot(st.mzone);
    let bestIdx = -1;
    let bestAtk = -1;
    st.hand.forEach((code, idx) => {
      const info = this._cardInfo.get(code);
      if (this.isMonster(info) && (info.attack || 0) > bestAtk) {
        bestAtk = info.attack || 0;
        bestIdx = idx;
      }
    });
    if (bestIdx !== -1 && slot !== -1) {
      const code = st.hand.splice(bestIdx, 1)[0];
      const lowLevel = ((this._cardInfo.get(code) || {}).level || 0) <= 4;
      if (lowLevel) {
        st.mzone[slot] = { code, pos: POS_FACEUP_ATTACK, attacked: false };
        eventBus.emit('duel:summoning', { code, cc: 1, cl: LOC_MZONE, cs: slot, cp: POS_FACEUP_ATTACK });
      } else {
        st.mzone[slot] = { code, pos: POS_FACEDOWN_DEFENSE, attacked: false };
        eventBus.emit('duel:set', { code, cc: 1, cl: LOC_MZONE, cs: slot, cp: POS_FACEDOWN_DEFENSE });
      }
      await this.delay();
    } else {
      const trapIdx = st.hand.findIndex((c) => {
        const info = this._cardInfo.get(c);
        return this.isSpell(info) || this.isTrap(info);
      });
      const sSlot = this.freeSlot(st.szone);
      if (trapIdx !== -1 && sSlot !== -1) {
        const code = st.hand.splice(trapIdx, 1)[0];
        st.szone[sSlot] = { code, pos: 0xa, faceDown: true };
        eventBus.emit('duel:set', { code, cc: 1, cl: LOC_SZONE, cs: sSlot, cp: 0xa });
        await this.delay();
      }
    }

    // Battle phase.
    this.setPhase(0x08); // BP
    await this.delay();
    for (let seq = 0; seq < st.mzone.length; seq++) {
      if (!this.running || this.turnPlayer !== 1) return;
      const m = st.mzone[seq];
      if (!m || (m.pos & 0xa) || m.attacked) continue;
      this.resolveBattle(1, seq);
      await this.delay(1200);
      if (!this.running) return;
    }

    this.setPhase(0x20); // EP
    this.turn++;
    await this.delay();
    if (this.running) this.startTurn(0);
  }
}

export const aiSimulator = new AISimulator();
