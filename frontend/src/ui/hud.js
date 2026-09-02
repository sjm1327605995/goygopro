/**
 * HUD & 2D UI Overlay System
 * Manages player status, LP animations, Hand dock, Phase bar,
 * Card Inspector sidebar, Action popups, Sound integration, and Selection Modals.
 */

import { WailsBridge } from '../wails_bridge.js';
import { soundManager } from '../audio/sound_manager.js';

export class DuelHUD {
  constructor(elements, onActionSelected) {
    this.elements = elements;
    this.onActionSelected = onActionSelected;

    this.playerLP = 8000;
    this.opponentLP = 8000;
    this.handCards = [];
    this.currentPhase = 'DP';
    this.selectedHandCard = null;

    this.initEventListeners();
  }

  initEventListeners() {
    document.addEventListener('click', (e) => {
      if (!e.target.closest('.action-popup') && !e.target.closest('.hand-card-item')) {
        this.hideActionPopup();
      }
    });

    const phaseItems = document.querySelectorAll('.phase-item');
    phaseItems.forEach(item => {
      item.addEventListener('click', () => {
        const phaseName = item.dataset.phase;
        if (item.classList.contains('actionable')) {
          this.onActionSelected('phase_change', phaseName);
        }
      });
    });
  }

  updateLP(player, newLP) {
    const isPlayer = player === 0;
    const targetElem = isPlayer ? this.elements.playerLP : this.elements.opponentLP;
    const fillElem = isPlayer ? this.elements.playerLPFill : this.elements.opponentLPFill;
    const currentVal = isPlayer ? this.playerLP : this.opponentLP;

    let start = currentVal;
    let end = Math.max(0, newLP);
    let duration = 600;
    let startTime = performance.now();
    soundManager.playLPTick();

    const animateNumber = (time) => {
      let progress = Math.min((time - startTime) / duration, 1);
      let val = Math.round(start + (end - start) * progress);
      targetElem.innerText = val;

      const pct = Math.max(0, Math.min(100, (val / 8000) * 100));
      fillElem.style.width = `${pct}%`;

      if (pct < 25) {
        fillElem.style.background = 'linear-gradient(90deg, #ef4444, #dc2626)';
      } else if (pct < 50) {
        fillElem.style.background = 'linear-gradient(90deg, #f59e0b, #d97706)';
      } else {
        fillElem.style.background = 'linear-gradient(90deg, #10b981, #00d2ff)';
      }

      if (progress < 1) {
        requestAnimationFrame(animateNumber);
      }
    };
    requestAnimationFrame(animateNumber);

    if (isPlayer) this.playerLP = newLP;
    else this.opponentLP = newLP;
  }

  updatePhase(phaseCode) {
    const phaseNames = {
      0x01: 'DP',
      0x02: 'SP',
      0x04: 'M1',
      0x08: 'BP',
      0x10: 'M2',
      0x20: 'EP'
    };
    const name = phaseNames[phaseCode] || 'M1';
    this.currentPhase = name;
    soundManager.playPhaseChange();

    document.querySelectorAll('.phase-item').forEach(item => {
      item.classList.remove('active');
      if (item.dataset.phase === name) {
        item.classList.add('active');
      }
    });

    this.appendLog(`Phase changed to ${name}`, 'log-action');
  }

  setPhaseButtonsActionable(canBP, canM2, canEP) {
    document.querySelectorAll('.phase-item').forEach(item => {
      const p = item.dataset.phase;
      item.classList.remove('actionable');
      if (p === 'BP' && canBP) item.classList.add('actionable');
      if (p === 'M2' && canM2) item.classList.add('actionable');
      if (p === 'EP' && canEP) item.classList.add('actionable');
    });
  }

  async inspectCard(cardCode, cardInfo = null) {
    if (!cardInfo) {
      cardInfo = await WailsBridge.getCard(cardCode);
    }
    if (!cardInfo) return;

    this.elements.inspectorName.innerText = cardInfo.name;
    this.elements.inspectorDesc.innerText = cardInfo.desc;

    this.elements.inspectorBadges.innerHTML = '';
    if (cardInfo.type & 0x1) {
      this.elements.inspectorBadges.innerHTML += `<span class="badge badge-type">Monster</span>`;
      if (cardInfo.type & 0x20) this.elements.inspectorBadges.innerHTML += `<span class="badge badge-type">Effect</span>`;
      if (cardInfo.type & 0x40) this.elements.inspectorBadges.innerHTML += `<span class="badge badge-type">Fusion</span>`;
      if (cardInfo.type & 0x2000) this.elements.inspectorBadges.innerHTML += `<span class="badge badge-type">Synchro</span>`;
      if (cardInfo.type & 0x800000) this.elements.inspectorBadges.innerHTML += `<span class="badge badge-type">Xyz</span>`;
      if (cardInfo.type & 0x4000000) this.elements.inspectorBadges.innerHTML += `<span class="badge badge-type">Link</span>`;

      this.elements.inspectorStats.style.display = 'flex';
      this.elements.inspectorStats.innerHTML = `
        <span>LV ${cardInfo.level || 0}</span>
        <span>ATK / ${cardInfo.attack}</span>
        <span>DEF / ${cardInfo.defense}</span>
      `;
    } else if (cardInfo.type & 0x2) {
      this.elements.inspectorBadges.innerHTML += `<span class="badge badge-attr">SPELL</span>`;
      this.elements.inspectorStats.style.display = 'none';
    } else if (cardInfo.type & 0x4) {
      this.elements.inspectorBadges.innerHTML += `<span class="badge badge-attr">TRAP</span>`;
      this.elements.inspectorStats.style.display = 'none';
    }

    const canvas = this.elements.inspectorPicCanvas;
    if (canvas) {
      const ctx = canvas.getContext('2d');
      ctx.fillStyle = '#1e293b';
      ctx.fillRect(0, 0, canvas.width, canvas.height);
      ctx.fillStyle = '#00d2ff';
      ctx.font = 'bold 15px sans-serif';
      ctx.textAlign = 'center';
      ctx.fillText(cardInfo.name.substring(0, 20), canvas.width / 2, canvas.height / 2);
    }
  }

  addHandCard(cardCode, cardInfo = null) {
    const cardItem = document.createElement('div');
    cardItem.className = 'hand-card-item';
    cardItem.dataset.code = cardCode;

    const img = document.createElement('div');
    img.className = 'hand-card-img';
    img.style.background = `linear-gradient(135deg, #1e293b, #0f172a)`;
    img.innerHTML = `<div style="padding:6px;font-size:11px;font-weight:700;color:#38bdf8;">${cardInfo ? cardInfo.name : 'Card #' + cardCode}</div>`;
    cardItem.appendChild(img);

    cardItem.addEventListener('mouseenter', () => {
      this.inspectCard(cardCode, cardInfo);
    });

    cardItem.addEventListener('click', (e) => {
      e.stopPropagation();
      this.selectHandCard(cardItem, cardCode, cardInfo);
    });

    this.elements.handContainer.appendChild(cardItem);
    this.handCards.push({ code: cardCode, info: cardInfo, elem: cardItem });
  }

  removeHandCard(index) {
    if (index >= 0 && index < this.handCards.length) {
      const item = this.handCards[index];
      if (item.elem && item.elem.parentNode) {
        item.elem.parentNode.removeChild(item.elem);
      }
      this.handCards.splice(index, 1);
    }
  }

  selectHandCard(elem, cardCode, cardInfo) {
    document.querySelectorAll('.hand-card-item').forEach(el => el.classList.remove('selected'));
    elem.classList.add('selected');
    this.selectedHandCard = { code: cardCode, info: cardInfo, elem };

    const rect = elem.getBoundingClientRect();
    this.showActionPopup(rect.left + rect.width / 2, rect.top - 10, [
      { label: 'Normal Summon', action: 'summon', code: cardCode },
      { label: 'Special Summon', action: 'spsummon', code: cardCode },
      { label: 'Set Card', action: 'set', code: cardCode },
      { label: 'Activate Effect', action: 'activate', code: cardCode }
    ]);
  }

  showActionPopup(x, y, actions) {
    const popup = this.elements.actionPopup;
    popup.innerHTML = '';

    actions.forEach(act => {
      const btn = document.createElement('button');
      btn.className = 'action-btn';
      btn.innerText = act.label;
      btn.addEventListener('click', () => {
        this.hideActionPopup();
        if (this.onActionSelected) {
          this.onActionSelected(act.action, act);
        }
      });
      popup.appendChild(btn);
    });

    popup.style.left = `${Math.max(20, Math.min(window.innerWidth - 180, x - 80))}px`;
    popup.style.top = `${Math.max(20, y - actions.length * 35)}px`;
    popup.style.display = 'flex';
  }

  hideActionPopup() {
    this.elements.actionPopup.style.display = 'none';
  }

  showYesNoModal(title, message, onYes, onNo) {
    const overlay = this.elements.modalOverlay;
    overlay.innerHTML = `
      <div class="modal-box">
        <div class="modal-title">${title}</div>
        <p style="color:var(--text-main); font-size:14px; line-height:1.5;">${message}</p>
        <div style="display:flex; justify-content:flex-end; gap:12px; margin-top:10px;">
          <button id="modal-btn-no" class="btn btn-secondary">No</button>
          <button id="modal-btn-yes" class="btn btn-primary">Yes</button>
        </div>
      </div>
    `;
    overlay.classList.add('active');

    document.getElementById('modal-btn-yes').onclick = () => {
      overlay.classList.remove('active');
      if (onYes) onYes();
    };
    document.getElementById('modal-btn-no').onclick = () => {
      overlay.classList.remove('active');
      if (onNo) onNo();
    };
  }

  showCardSelectModal(title, cards, min = 1, max = 1, onConfirm = null) {
    const overlay = this.elements.modalOverlay;
    let selectedIndices = [];

    const cardsHtml = cards.map((c, idx) => `
      <div class="select-card-item" data-idx="${idx}" style="background:#1e293b; padding:6px;">
        <div style="font-size:11px; color:#38bdf8; font-weight:700;">${c.name || 'Card #' + c.code}</div>
      </div>
    `).join('');

    overlay.innerHTML = `
      <div class="modal-box" style="min-width:550px;">
        <div class="modal-title">${title} (Select ${min}-${max})</div>
        <div class="modal-cards-grid">${cardsHtml}</div>
        <div style="display:flex; justify-content:space-between; align-items:center; margin-top:12px;">
          <span id="select-count-text" style="font-size:13px; color:var(--text-muted);">0 / ${max} selected</span>
          <button id="modal-select-confirm" class="btn btn-gold" disabled>Confirm</button>
        </div>
      </div>
    `;
    overlay.classList.add('active');

    const cardElems = overlay.querySelectorAll('.select-card-item');
    const confirmBtn = document.getElementById('modal-select-confirm');
    const countText = document.getElementById('select-count-text');

    cardElems.forEach(el => {
      el.onclick = () => {
        const idx = parseInt(el.dataset.idx, 10);
        const selPos = selectedIndices.indexOf(idx);
        if (selPos > -1) {
          selectedIndices.splice(selPos, 1);
          el.classList.remove('selected');
        } else if (selectedIndices.length < max) {
          selectedIndices.push(idx);
          el.classList.add('selected');
        }
        countText.innerText = `${selectedIndices.length} / ${max} selected`;
        confirmBtn.disabled = selectedIndices.length < min;
      };
    });

    confirmBtn.onclick = () => {
      overlay.classList.remove('active');
      if (onConfirm) onConfirm(selectedIndices);
    };
  }

  showPositionPickerModal(positions, onSelect) {
    const overlay = this.elements.modalOverlay;
    overlay.innerHTML = `
      <div class="modal-box" style="min-width:320px; align-items:center;">
        <div class="modal-title">Select Battle Position</div>
        <div style="display:flex; gap:16px; margin:16px 0;">
          <button id="pos-atk-btn" class="btn btn-primary">Attack Position</button>
          <button id="pos-def-btn" class="btn btn-secondary">Defense Position</button>
        </div>
      </div>
    `;
    overlay.classList.add('active');

    document.getElementById('pos-atk-btn').onclick = () => {
      overlay.classList.remove('active');
      if (onSelect) onSelect(0x1);
    };
    document.getElementById('pos-def-btn').onclick = () => {
      overlay.classList.remove('active');
      if (onSelect) onSelect(0x4);
    };
  }

  showRPSModal(onSelect) {
    const overlay = this.elements.modalOverlay;
    overlay.innerHTML = `
      <div class="modal-box" style="min-width:400px; text-align:center;">
        <div class="modal-title">Rock - Paper - Scissors</div>
        <p style="color:var(--text-muted); font-size:13px;">Choose your hand to decide turn order</p>
        <div style="display:flex; justify-content:center; gap:20px; margin:20px 0;">
          <button id="rps-rock" class="btn btn-primary" style="padding:16px 24px; font-size:24px;">✊ Rock</button>
          <button id="rps-scissors" class="btn btn-gold" style="padding:16px 24px; font-size:24px;">✌️ Scissors</button>
          <button id="rps-paper" class="btn btn-secondary" style="padding:16px 24px; font-size:24px;">✋ Paper</button>
        </div>
      </div>
    `;
    overlay.classList.add('active');

    document.getElementById('rps-rock').onclick = () => {
      overlay.classList.remove('active');
      if (onSelect) onSelect(1);
    };
    document.getElementById('rps-scissors').onclick = () => {
      overlay.classList.remove('active');
      if (onSelect) onSelect(2);
    };
    document.getElementById('rps-paper').onclick = () => {
      overlay.classList.remove('active');
      if (onSelect) onSelect(3);
    };
  }

  showVictoryModal(isWin) {
    if (isWin) soundManager.playVictory();
    else soundManager.playDefeat();

    const overlay = this.elements.modalOverlay;
    overlay.innerHTML = `
      <div class="modal-box" style="text-align:center; min-width:400px; padding:36px 24px;">
        <div style="font-size:64px; margin-bottom:12px;">${isWin ? '🏆' : '💀'}</div>
        <div style="font-size:32px; font-weight:900; color:${isWin ? '#f59e0b' : '#ef4444'}; margin-bottom:8px;">
          ${isWin ? 'VICTORY' : 'DEFEAT'}
        </div>
        <p style="color:var(--text-muted); margin-bottom:24px;">
          ${isWin ? 'Congratulations! You emerged victorious!' : 'You fought valiantly. Try again!'}
        </p>
        <button id="modal-duel-exit" class="btn btn-primary" style="padding:12px 32px;">Return to Menu</button>
      </div>
    `;
    overlay.classList.add('active');

    document.getElementById('modal-duel-exit').onclick = () => {
      overlay.classList.remove('active');
      if (window.app) window.app.showScreen('main-menu-screen');
    };
  }

  appendLog(msg, typeClass = '') {
    const entry = document.createElement('div');
    entry.className = `log-entry ${typeClass}`;
    entry.innerText = `> ${msg}`;
    this.elements.logList.appendChild(entry);
    this.elements.logList.scrollTop = this.elements.logList.scrollHeight;
  }
}
