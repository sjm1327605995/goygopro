/**
 * Duel Manager & Controller
 * Coordinates duel flow, bridges network packets to 3D Field animations, Chain Visualizer, and 2D HUD.
 */

import { WailsBridge, eventBus } from '../wails_bridge.js';

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
    eventBus.on('stoc:select_hand', () => {
      this.hud.showRPSModal((choice) => {
        WailsBridge.sendHandResult(choice);
      });
    });

    eventBus.on('duel:start', (data) => {
      this.hud.updateLP(0, data.lp0 || 8000);
      this.hud.updateLP(1, data.lp1 || 8000);
      this.hud.appendLog('Duel Started!', 'log-action');
    });

    eventBus.on('duel:draw', async (data) => {
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
      this.hud.appendLog(`Player ${data.player} drew ${data.count} card(s).`);
    });

    eventBus.on('duel:new_turn', (data) => {
      this.hud.appendLog(`--- Turn of Player ${data.player} ---`, 'log-action');
    });

    eventBus.on('duel:new_phase', (data) => {
      this.hud.updatePhase(data.phase);
    });

    eventBus.on('duel:summoning', async (data) => {
      const cardInfo = await WailsBridge.getCard(data.code);
      this.field3D.animateSummon(data.cc, data.code, data.cs, data.cp, cardInfo, false);
      this.hud.appendLog(`Normal Summon: ${cardInfo ? cardInfo.name : data.code}`, 'log-summon');
    });

    eventBus.on('duel:spsummoning', async (data) => {
      const cardInfo = await WailsBridge.getCard(data.code);
      this.field3D.animateSummon(data.cc, data.code, data.cs, data.cp, cardInfo, true);
      this.hud.appendLog(`Special Summon: ${cardInfo ? cardInfo.name : data.code}`, 'log-summon');
    });

    eventBus.on('duel:set', async (data) => {
      const isMonster = data.cl === 0x4;
      const cardInfo = await WailsBridge.getCard(data.code);
      this.field3D.animateSetCard(data.cc, data.code, data.cs, isMonster, cardInfo);
      this.hud.appendLog(`Player ${data.cc} Set a card.`);
    });

    eventBus.on('duel:pos_change', (data) => {
      this.field3D.animateReposition(data.cc, data.cs, data.cp);
      this.hud.appendLog(`Player ${data.cc} changed position of card in slot ${data.cs}.`);
    });

    eventBus.on('duel:attack', (data) => {
      this.field3D.animateAttack(
        data.attacker.c,
        data.attacker.s,
        data.target.c,
        data.target.s,
        data.target.l === 0
      );
      this.hud.appendLog(`Attack declared! Slot ${data.attacker.s} -> Slot ${data.target.s}`, 'log-damage');
    });

    eventBus.on('duel:damage', (data) => {
      const currentLP = data.player === 0 ? this.hud.playerLP : this.hud.opponentLP;
      const newLP = currentLP - data.amount;
      this.hud.updateLP(data.player, newLP);
      this.field3D.cameraShake(0.3, 250);
      this.hud.appendLog(`Player ${data.player} took ${data.amount} damage!`, 'log-damage');
    });

    eventBus.on('duel:recover', (data) => {
      const currentLP = data.player === 0 ? this.hud.playerLP : this.hud.opponentLP;
      const newLP = currentLP + data.amount;
      this.hud.updateLP(data.player, newLP);
      this.hud.appendLog(`Player ${data.player} gained ${data.amount} LP!`, 'log-action');
    });

    // Chain Events
    eventBus.on('duel:chaining', async (data) => {
      const cardInfo = await WailsBridge.getCard(data.code);
      const locName = data.cl === 0x4 ? 'mzone' : 'szone';
      this.field3D.chainVisualizer.addChainLink(data.code, { player: data.cc, loc: locName, seq: data.cs }, cardInfo);
      this.hud.appendLog(`Chain Link: ${cardInfo ? cardInfo.name : data.code} activated!`, 'log-action');
    });

    eventBus.on('duel:chain_solving', (data) => {
      this.field3D.chainVisualizer.highlightSolvingLink(data.count);
    });

    eventBus.on('duel:chain_solved', (data) => {
      this.field3D.chainVisualizer.removeSolvingLink(data.count);
    });

    eventBus.on('duel:chain_end', () => {
      this.field3D.chainVisualizer.clearChain();
    });

    eventBus.on('duel:select_idlecmd', (data) => {
      this.idleCmd = data;
      this.hud.setPhaseButtonsActionable(data.toBP, false, data.toEP);
    });

    eventBus.on('duel:select_battlecmd', (data) => {
      this.battleCmd = data;
      this.hud.setPhaseButtonsActionable(false, data.toM2, data.toEP);
    });

    eventBus.on('duel:select_effectyn', async (data) => {
      const cardInfo = await WailsBridge.getCard(data.code);
      const title = 'Activate Card Effect?';
      const msg = `Do you wish to activate the effect of "${cardInfo ? cardInfo.name : data.code}"?`;
      this.hud.showYesNoModal(title, msg, () => {
        WailsBridge.sendResponseI(1);
      }, () => {
        WailsBridge.sendResponseI(0);
      });
    });

    eventBus.on('duel:select_card', async (data) => {
      const cardsWithInfo = await Promise.all(data.cards.map(async (c) => {
        const info = await WailsBridge.getCard(c.code);
        return { ...c, name: info ? info.name : `Card #${c.code}` };
      }));

      this.hud.showCardSelectModal('Select Card(s)', cardsWithInfo, data.min, data.max, (selectedIndices) => {
        const resp = new Uint8Array(1 + selectedIndices.length);
        resp[0] = selectedIndices.length;
        for (let i = 0; i < selectedIndices.length; i++) {
          resp[1 + i] = selectedIndices[i];
        }
        WailsBridge.sendResponseB(Array.from(resp));
      });
    });

    eventBus.on('duel:select_position', (data) => {
      this.hud.showPositionPickerModal(data.positions, (pos) => {
        WailsBridge.sendResponseI(pos);
      });
    });

    eventBus.on('duel:win', (data) => {
      const isWinner = data.winner === this.playerSlot;
      this.hud.showVictoryModal(isWinner);
    });
  }

  handlePlayerAction(actionType, data) {
    if (actionType === 'phase_change') {
      if (data === 'BP') WailsBridge.sendResponseI(6);
      else if (data === 'M2') WailsBridge.sendResponseI(2);
      else if (data === 'EP') WailsBridge.sendResponseI(7);
      return;
    }

    if (actionType === 'summon' && this.idleCmd && this.idleCmd.summon) {
      const match = this.idleCmd.summon.find(s => s.code === data.code);
      if (match) {
        WailsBridge.sendResponseI((match.idx << 16) | 0);
      } else {
        this.field3D.animateSummon(0, data.code, 2, 0x1, null, false);
      }
    } else if (actionType === 'set') {
      this.field3D.animateSetCard(0, data.code, 2, false, null);
    }
  }
}
