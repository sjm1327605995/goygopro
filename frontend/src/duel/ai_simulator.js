/**
 * AI Practice Duel Simulator Engine
 * Implements an interactive standalone duel rule engine against an AI opponent (Seto Kaiba).
 */

import { eventBus } from '../wails_bridge.js';

export class AISimulator {
  constructor() {
    this.running = false;
    this.turn = 1;
    this.currentPhase = 0x01; // DP
    this.turnPlayer = 0; // 0: Player, 1: AI

    this.playerState = {
      lp: 8000,
      deck: [89631139, 46986414, 38033121, 83764718, 55144522, 12580477, 5318639, 44095762, 41420027, 14558127, 84013237],
      hand: [],
      mzone: [null, null, null, null, null],
      szone: [null, null, null, null, null],
      grave: []
    };

    this.aiState = {
      name: 'SetoKaiba (AI)',
      lp: 8000,
      deck: [89631139, 89631139, 89631139, 44095762, 12580477, 55144522, 83764718, 41420027],
      hand: [],
      mzone: [null, null, null, null, null],
      szone: [null, null, null, null, null],
      grave: []
    };
  }

  start() {
    this.running = true;
    this.turn = 1;
    this.turnPlayer = 0;

    // 1. Duel Start
    eventBus.emit('duel:start', {
      playerType: 0,
      duelRule: 5,
      lp0: 8000,
      lp1: 8000,
      deck0: this.playerState.deck.length,
      deck1: this.aiState.deck.length
    });

    // 2. Draw 5 opening cards for both players
    setTimeout(() => {
      const pHand = this.drawCards(0, 5);
      const aiHand = this.drawCards(1, 5);

      eventBus.emit('duel:draw', { player: 0, count: 5, cards: pHand });
      eventBus.emit('duel:draw', { player: 1, count: 5, cards: aiHand.map(() => 0) });

      // Start Turn 1
      setTimeout(() => this.startTurn(0), 1000);
    }, 600);
  }

  drawCards(player, count) {
    const state = player === 0 ? this.playerState : this.aiState;
    const drawn = [];
    for (let i = 0; i < count; i++) {
      if (state.deck.length > 0) {
        const code = state.deck.shift();
        state.hand.push(code);
        drawn.push(code);
      }
    }
    return drawn;
  }

  startTurn(player) {
    if (!this.running) return;
    this.turnPlayer = player;
    eventBus.emit('duel:new_turn', { player });

    // Draw Phase
    this.setPhase(0x01); // DP
    if (this.turn > 1) {
      const drawn = this.drawCards(player, 1);
      eventBus.emit('duel:draw', { player, count: 1, cards: player === 0 ? drawn : [0] });
    }

    // Standby Phase -> Main Phase 1
    setTimeout(() => {
      this.setPhase(0x02); // SP
      setTimeout(() => {
        this.setPhase(0x04); // M1
        if (player === 0) {
          this.promptPlayerIdle();
        } else {
          this.executeAITurn();
        }
      }, 500);
    }, 500);
  }

  setPhase(phase) {
    this.currentPhase = phase;
    eventBus.emit('duel:new_phase', { phase });
  }

  promptPlayerIdle() {
    // Generate summonable & spell activatable lists from hand
    const summonList = [];
    const activateList = [];

    this.playerState.hand.forEach((code, idx) => {
      if ([89631139, 46986414, 38033121, 14558127].includes(code)) {
        summonList.push({ code, c: 0, l: 2, s: idx, idx });
      } else if ([55144522, 12580477, 83764718, 5318639].includes(code)) {
        activateList.push({ code, c: 0, l: 2, s: idx, desc: 0, idx });
      }
    });

    eventBus.emit('duel:select_idlecmd', {
      player: 0,
      summon: summonList,
      spsummon: [],
      repos: [],
      mset: [],
      sset: [],
      activate: activateList,
      toBP: true,
      toEP: true,
      shuffle: false
    });
  }

  executeAITurn() {
    // AI summons a monster if it has one in hand
    setTimeout(() => {
      const monsterIdx = this.aiState.hand.findIndex(c => [89631139, 46986414].includes(c));
      if (monsterIdx !== -1) {
        const code = this.aiState.hand.splice(monsterIdx, 1)[0];
        const emptySlot = this.aiState.mzone.findIndex(s => s === null);
        if (emptySlot !== -1) {
          this.aiState.mzone[emptySlot] = { code, atk: 3000, def: 2500, pos: 0x1 };
          eventBus.emit('duel:summoning', { code, cc: 1, cl: 4, cs: emptySlot, cp: 0x1 });
        }
      }

      // Enter Battle Phase
      setTimeout(() => {
        this.setPhase(0x08); // BP
        // AI declares attack if has monster
        const attackerSlot = this.aiState.mzone.findIndex(s => s !== null);
        if (attackerSlot !== -1) {
          const pDefenderSlot = this.playerState.mzone.findIndex(s => s !== null);
          setTimeout(() => {
            eventBus.emit('duel:attack', {
              attacker: { c: 1, l: 4, s: attackerSlot },
              target: { c: 0, l: pDefenderSlot !== -1 ? 4 : 0, s: pDefenderSlot !== -1 ? pDefenderSlot : 0 }
            });

            setTimeout(() => {
              // Deal battle damage to player
              const dmg = 3000;
              this.playerState.lp -= dmg;
              eventBus.emit('duel:damage', { player: 0, amount: dmg });

              // End Turn
              setTimeout(() => {
                this.setPhase(0x20); // EP
                this.turn++;
                setTimeout(() => this.startTurn(0), 800);
              }, 1000);
            }, 800);
          }, 800);
        } else {
          this.setPhase(0x20); // EP
          this.turn++;
          setTimeout(() => this.startTurn(0), 800);
        }
      }, 1000);
    }, 1200);
  }

  playerSummon(code, slotIndex = 2) {
    const handIdx = this.playerState.hand.indexOf(code);
    if (handIdx !== -1) {
      this.playerState.hand.splice(handIdx, 1);
      this.playerState.mzone[slotIndex] = { code, atk: 3000, def: 2500, pos: 0x1 };
      eventBus.emit('duel:summoning', { code, cc: 0, cl: 4, cs: slotIndex, cp: 0x1 });
    }
  }

  playerAttack(attackerSlot = 2, targetSlot = 0) {
    eventBus.emit('duel:attack', {
      attacker: { c: 0, l: 4, s: attackerSlot },
      target: { c: 1, l: this.aiState.mzone[targetSlot] ? 4 : 0, s: targetSlot }
    });

    setTimeout(() => {
      const dmg = 3000;
      this.aiState.lp -= dmg;
      eventBus.emit('duel:damage', { player: 1, amount: dmg });

      if (this.aiState.lp <= 0) {
        eventBus.emit('duel:win', { winner: 0, type: 0 });
      }
    }, 800);
  }
}

export const aiSimulator = new AISimulator();
