/**
 * HUD & 2D UI Overlay System
 * Manages player status, LP animations, Hand dock, Phase bar,
 * Card Inspector sidebar, Action popups, Sound integration, and Selection Modals.
 */

import { WailsBridge, eventBus } from '../wails_bridge.js';
import { soundManager } from '../audio/sound_manager.js';

export class DuelHUD {
  constructor(elements, onActionSelected) {
    this.elements = elements;
    this.onActionSelected = onActionSelected;
    // Optional provider wired by DuelManager: given a hand card code, returns
    // the action options the engine actually offered (usually summon /
    // spsummon / set / activate). Falls back to a generic popup when unset.
    this.handActionProvider = null;

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

  updateLP(player, newLP, instant = false) {
    const isPlayer = player === 0;
    const targetElem = isPlayer ? this.elements.playerLP : this.elements.opponentLP;
    const fillElem = isPlayer ? this.elements.playerLPFill : this.elements.opponentLPFill;
    const currentVal = isPlayer ? this.playerLP : this.opponentLP;

    let start = currentVal;
    let end = Math.max(0, newLP);

    if (instant) {
      targetElem.innerText = end;
      fillElem.style.width = `${Math.max(0, Math.min(100, (end / 8000) * 100))}%`;
      if (isPlayer) this.playerLP = newLP;
      else this.opponentLP = newLP;
      return;
    }

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
    const phaseLabels = {
      DP: '抽卡阶段',
      SP: '准备阶段',
      M1: '主要阶段 1',
      BP: '战斗阶段',
      M2: '主要阶段 2',
      EP: '结束阶段'
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

    this.appendLog(`进入${phaseLabels[name] || name}`, 'log-action');
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
      this.elements.inspectorBadges.innerHTML += `<span class="badge badge-type">怪兽</span>`;
      if (cardInfo.type & 0x20) this.elements.inspectorBadges.innerHTML += `<span class="badge badge-type">效果</span>`;
      if (cardInfo.type & 0x40) this.elements.inspectorBadges.innerHTML += `<span class="badge badge-type">融合</span>`;
      if (cardInfo.type & 0x2000) this.elements.inspectorBadges.innerHTML += `<span class="badge badge-type">同调</span>`;
      if (cardInfo.type & 0x800000) this.elements.inspectorBadges.innerHTML += `<span class="badge badge-type">超量</span>`;
      if (cardInfo.type & 0x4000000) this.elements.inspectorBadges.innerHTML += `<span class="badge badge-type">连接</span>`;

      this.elements.inspectorStats.style.display = 'flex';
      this.elements.inspectorStats.innerHTML = `
        <span>等级 ${cardInfo.level || 0}</span>
        <span>攻击 ${cardInfo.attack}</span>
        <span>守备 ${cardInfo.defense}</span>
      `;
    } else if (cardInfo.type & 0x2) {
      this.elements.inspectorBadges.innerHTML += `<span class="badge badge-attr">魔法</span>`;
      this.elements.inspectorStats.style.display = 'none';
    } else if (cardInfo.type & 0x4) {
      this.elements.inspectorBadges.innerHTML += `<span class="badge badge-attr">陷阱</span>`;
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

      // Real card art over the placeholder (async; ignored if another card is
      // being inspected by the time the picture arrives).
      const token = cardCode;
      this._inspectedCode = cardCode;
      WailsBridge.getCardImage(cardCode).then((url) => {
        if (!url || token !== this._inspectedCode) return;
        const img = new Image();
        img.onload = () => {
          if (token !== this._inspectedCode) return;
          const c2 = canvas.getContext('2d');
          const scale = Math.min(canvas.width / img.width, canvas.height / img.height);
          const dw = img.width * scale, dh = img.height * scale;
          c2.fillStyle = '#1e293b';
          c2.fillRect(0, 0, canvas.width, canvas.height);
          c2.drawImage(img, (canvas.width - dw) / 2, (canvas.height - dh) / 2, dw, dh);
        };
        img.src = url;
      }).catch(() => {});
    }
  }

  addHandCard(cardCode, cardInfo = null) {
    const cardItem = document.createElement('div');
    cardItem.className = 'hand-card-item';
    cardItem.dataset.code = cardCode;

    const img = document.createElement('div');
    img.className = 'hand-card-img';
    img.style.background = `linear-gradient(135deg, #1e293b, #0f172a)`;
    img.innerHTML = `<div style="padding:6px;font-size:11px;font-weight:700;color:#38bdf8;">${cardInfo ? cardInfo.name : '卡牌 #' + cardCode}</div>`;
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

  // Removes the first hand card matching a code (used by move-to-hand events);
  // returns true when a card was removed.
  removeHandCardByCode(cardCode) {
    const idx = this.handCards.findIndex(c => c.code === cardCode);
    if (idx === -1) return false;
    this.removeHandCard(idx);
    return true;
  }

  // Empties the hand dock and clears the duel log; used when the replay
  // applier rebuilds the board for a seek / step-back.
  clearHand() {
    this.handCards.forEach(item => {
      if (item.elem && item.elem.parentNode) {
        item.elem.parentNode.removeChild(item.elem);
      }
    });
    this.handCards = [];
    this.selectedHandCard = null;
  }

  clearLog() {
    this.elements.logList.innerHTML = '';
  }

  // Closes any open modal / action popup; the replay applier calls this when
  // rebuilding the board so a VICTORY modal from an earlier step does not
  // linger across a seek.
  hideModals() {
    if (this.elements.modalOverlay) {
      this.elements.modalOverlay.classList.remove('active');
      this.elements.modalOverlay.innerHTML = '';
    }
    this.hideActionPopup();
  }

  selectHandCard(elem, cardCode, cardInfo) {
    document.querySelectorAll('.hand-card-item').forEach(el => el.classList.remove('selected'));
    elem.classList.add('selected');
    this.selectedHandCard = { code: cardCode, info: cardInfo, elem };

    const rect = elem.getBoundingClientRect();
    const options = this.handActionProvider
      ? this.handActionProvider(cardCode)
      : [
          { label: '通常召唤', action: 'summon', code: cardCode },
          { label: '特殊召唤', action: 'spsummon', code: cardCode },
          { label: '盖卡', action: 'set', code: cardCode },
          { label: '发动效果', action: 'activate', code: cardCode }
        ];
    if (options.length) {
      this.showActionPopup(rect.left + rect.width / 2, rect.top - 10, options);
    } else {
      this.hideActionPopup();
    }
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
          <button id="modal-btn-no" class="btn btn-secondary">否</button>
          <button id="modal-btn-yes" class="btn btn-primary">是</button>
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

  showCardSelectModal(title, cards, min = 1, max = 1, onConfirm = null, onCancel = null) {
    const overlay = this.elements.modalOverlay;
    let selectedIndices = [];

    const cardsHtml = cards.map((c, idx) => `
      <div class="select-card-item" data-idx="${idx}" style="background:#1e293b; padding:6px;">
        <div style="font-size:11px; color:#38bdf8; font-weight:700;">${c.name || '卡牌 #' + c.code}</div>
      </div>
    `).join('');

    overlay.innerHTML = `
      <div class="modal-box" style="min-width:550px;">
        <div class="modal-title">${title}（选择 ${min}-${max} 张）</div>
        <div class="modal-cards-grid">${cardsHtml}</div>
        <div style="display:flex; justify-content:space-between; align-items:center; margin-top:12px;">
          <span id="select-count-text" style="font-size:13px; color:var(--text-muted);">已选 0 / ${max} 张</span>
          <div style="display:flex; gap:8px;">
            ${onCancel ? '<button id="modal-select-cancel" class="btn btn-secondary">取消</button>' : ''}
            <button id="modal-select-confirm" class="btn btn-gold" disabled>确认</button>
          </div>
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
        countText.innerText = `已选 ${selectedIndices.length} / ${max} 张`;
        confirmBtn.disabled = selectedIndices.length < min;
      };
    });

    confirmBtn.onclick = () => {
      overlay.classList.remove('active');
      if (onConfirm) onConfirm(selectedIndices);
    };

    const cancelBtn = document.getElementById('modal-select-cancel');
    if (cancelBtn) {
      cancelBtn.onclick = () => {
        overlay.classList.remove('active');
        if (onCancel) onCancel();
      };
    }
  }

  showPositionPickerModal(positions, onSelect) {
    const overlay = this.elements.modalOverlay;
    overlay.innerHTML = `
      <div class="modal-box" style="min-width:320px; align-items:center;">
        <div class="modal-title">选择表示形式</div>
        <div style="display:flex; gap:16px; margin:16px 0;">
          <button id="pos-atk-btn" class="btn btn-primary">攻击表示</button>
          <button id="pos-def-btn" class="btn btn-secondary">守备表示</button>
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
        <div class="modal-title">猜拳</div>
        <p style="color:var(--text-muted); font-size:13px;">出拳决定先攻顺序</p>
        <div style="display:flex; justify-content:center; gap:20px; margin:20px 0;">
          <button id="rps-rock" class="btn btn-primary" style="padding:16px 24px; font-size:24px;">✊ 石头</button>
          <button id="rps-scissors" class="btn btn-gold" style="padding:16px 24px; font-size:24px;">✌️ 剪刀</button>
          <button id="rps-paper" class="btn btn-secondary" style="padding:16px 24px; font-size:24px;">✋ 布</button>
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
          ${isWin ? '胜利' : '败北'}
        </div>
        <p style="color:var(--text-muted); margin-bottom:24px;">
          ${isWin ? '恭喜！你赢得了这场决斗！' : '虽败犹荣，再接再厉！'}
        </p>
        <button id="modal-duel-exit" class="btn btn-primary" style="padding:12px 32px;">返回主菜单</button>
      </div>
    `;
    overlay.classList.add('active');

    document.getElementById('modal-duel-exit').onclick = () => {
      overlay.classList.remove('active');
      eventBus.emit('nav', 'menu');
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
