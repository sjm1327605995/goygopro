// Headless smoke for the P4 wave-6 completion: DrawSpec overlay (flip / zoom /
// mask reveal / negated + coin/dice ACMessage) and the stTip hover tooltip.
// All spec cards are mocked to a data URL so no network is involved.
import { createRoot } from 'react-dom/client';
import React from 'react';
import { WailsBridge, eventBus } from '../src/wails_bridge.ts';
import { duelStore } from '../src/duel/store.ts';
import '../css/style.css';
import '../css/duel-original.css';
import SpecOverlay from '../src/components/SpecOverlay.tsx';
import CardTooltip from '../src/components/CardTooltip.tsx';

const FAKE_PIC = 'data:image/gif;base64,R0lGODlhAQABAIAAAAUEBAAAACwAAAAAAQABAAACAkQBADs=';
WailsBridge.getCardImage = async () => ({ url: FAKE_PIC, full: false });
WailsBridge.getCard = async (code) => ({ code, name: 'Blue-Eyes White Dragon', type: 0x11, attack: 3000, defense: 2500, desc: '' });

const root = createRoot(document.getElementById('root'));
root.render(
  <div className="duel-stage" style={{ position: 'relative', width: '100vw', height: '100vh' }}>
    <div id="duel-canvas-container"></div>
    <SpecOverlay />
    <CardTooltip />
  </div>
);

window.__specSmoke = { checks: {}, ready: false };
const checks = window.__specSmoke.checks;
const record = (name, ok) => { checks[name] = ok; };

const $ = (sel) => document.querySelector(sel);
const waitFor = (predicate, timeoutMs = 5000) => new Promise((resolve, reject) => {
  const started = Date.now();
  const tick = () => {
    if (predicate()) return resolve();
    if (Date.now() - started > timeoutMs) return reject(new Error('waitFor timeout: ' + (predicate.toString().slice(0, 80))));
    setTimeout(tick, 25);
  };
  tick();
});
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

(async () => {
  try {
    await waitFor(() => !!$('#draw-spec-overlay'));
    const overlay = $('#draw-spec-overlay');

    // --- 翻卡（duel:summoning → showcard=7） ---
    eventBus.emit('duel:summoning', { code: 89631139, cc: 0, cl: 4, cs: 2, cp: 1 });
    await waitFor(() => overlay.style.display === 'block' && !!$('.spec-flip'));
    record('summoning-plays-flip', true);

    // --- 排队：立刻再来一个 zoom，flip 播完后接续 ---
    eventBus.emit('duel:spsummoning', { code: 84013237, cc: 0, cl: 4, cs: 1, cp: 1 });
    await sleep(1000); // flip(800ms) 已结束、zoom(500ms) 应正在播
    record('queue-advances-to-zoom', !!$('.spec-zoom'));
    await waitFor(() => overlay.style.display === 'none', 3000);
    record('overlay-hides-after-queue', true);

    // --- mask 揭示（duel:chaining → showcard=1+2） ---
    eventBus.emit('duel:chaining', { code: 55144522, cc: 0, cl: 5, cs: 0, cp: 1, desc: 0 });
    await waitFor(() => !!$('.spec-reveal') && !!$('.spec-mask'));
    record('chaining-plays-mask-reveal', true);
    await waitFor(() => overlay.style.display === 'none', 3000);

    // --- 无效化（duel:chain_negated → showcard=3，盖在最后一张连锁卡上） ---
    eventBus.emit('duel:chain_negated', { count: 1 });
    await waitFor(() => !!$('.spec-negated'));
    record('chain-negated-plays', true);
    await waitFor(() => overlay.style.display === 'none', 3000);

    // --- showcard=4：confirm_cards 单卡淡入揭示 ---
    eventBus.emit('duel:confirm_cards', {
      player: 0, skipPanel: 0,
      cards: [{ code: 89631139, c: 0, l: 4, s: 0 }],
    });
    await waitFor(() => !!$('.spec-fade'));
    record('confirm-cards-plays-fade', true);
    await waitFor(() => overlay.style.display === 'none', 3000);

    // --- duel:hint HINT_EFFECT/HINT_CARD → mask 揭示（showcard=1） ---
    eventBus.emit('duel:hint', { type: 5, player: 0, data: 55144522 });
    await waitFor(() => !!$('.spec-reveal'));
    record('hint-effect-plays-reveal', true);
    await waitFor(() => overlay.style.display === 'none', 3000);

    // --- showcard=6：CHINT_TURN 回合数大字盖戳（卡图从 store.board 取） ---
    eventBus.emit('duel:start', {
      playerType: 0, duelRule: 5, lp0: 8000, lp1: 8000,
      deck0: 40, extra0: 15, deck1: 40, extra1: 15,
    });
    eventBus.emit('duel:summoning', { code: 89631139, cc: 0, cl: 4, cs: 2, cp: 1 });
    await waitFor(() => (duelStore.getState().board[0].mzone[2] || {}).code === 89631139);
    eventBus.emit('duel:card_hint', { card: { c: 0, l: 4, s: 2 }, type: 1, data: 3 });
    await waitFor(() => !!$('.spec-number') && $('.spec-number').textContent === '3');
    record('card-hint-turn-plays-number', true);
    await waitFor(() => overlay.style.display === 'none', 3000);

    // --- showcard=100：猜拳双拳下落对碰（res = 我 | 对方<<2） ---
    eventBus.emit('duel:hand_res', { res: 1 | (3 << 2) }); // 我石头 vs 对方布
    await waitFor(() => !!$('#rps-arena'));
    record('hand-res-plays-rps', $('#rps-arena').textContent.includes('✊')
      && $('#rps-arena').textContent.includes('✋'));
    await waitFor(() => !$('#rps-arena'), 3000);

    // --- 猜硬币/骰子：图标动画 + ACMessage 文本弹条 ---
    eventBus.emit('duel:toss_coin', { player: 0, results: [1, 0] });
    await waitFor(() => !!$('#toss-arena') && document.querySelectorAll('.toss-coin').length === 2);
    record('coin-icon-flips', !!$('.toss-coin'));
    await waitFor(() => $('#ac-message') && $('#ac-message').style.display === 'block');
    record('coin-popup-text', $('#ac-message').textContent.includes('正面') && $('#ac-message').textContent.includes('反面'));
    await waitFor(() => $('#ac-message').style.display === 'none' && !$('#toss-arena'), 3000);

    eventBus.emit('duel:toss_dice', { player: 0, results: [3, 5] });
    await waitFor(() => !!$('#toss-arena') && document.querySelectorAll('.toss-die').length === 2);
    record('dice-icon-bounces', $('.toss-die') && $('.toss-die').textContent === '3');
    await waitFor(() => $('#ac-message').style.display === 'block');
    record('dice-popup-text', $('#ac-message').textContent.includes('3') && $('#ac-message').textContent.includes('5'));
    await waitFor(() => $('#ac-message').style.display === 'none', 3000);

    // --- HINT 宣言展示（duelclient.cpp:1108-1145）：OPSELECTED/RACE/ATTRIB/
    // CODE/NUMBER → stACMessage 浮条（SysString 1510/1511/1512） ---
    eventBus.emit('duel:hint', { type: 9, player: 1, data: 5 });
    await waitFor(() => $('#ac-message').style.display === 'block');
    record('hint-number-popup', $('#ac-message').textContent.includes('玩家选择了：5'));
    await waitFor(() => $('#ac-message').style.display === 'none', 3000);

    eventBus.emit('duel:hint', { type: 6, player: 1, data: 0x2001 }); // 战士/龙
    await waitFor(() => $('#ac-message').style.display === 'block');
    record('hint-race-popup', $('#ac-message').textContent.includes('宣言')
      && $('#ac-message').textContent.includes('战士/龙'), $('#ac-message').textContent);
    await waitFor(() => $('#ac-message').style.display === 'none', 3000);

    eventBus.emit('duel:hint', { type: 7, player: 1, data: 0x10 }); // 光属性
    await waitFor(() => $('#ac-message').style.display === 'block');
    record('hint-attrib-popup', $('#ac-message').textContent.includes('宣言')
      && $('#ac-message').textContent.includes('光'));
    await waitFor(() => $('#ac-message').style.display === 'none', 3000);

    eventBus.emit('duel:hint', { type: 8, player: 1, data: 89631139 });
    await waitFor(() => $('#ac-message').style.display === 'block'
      && $('#ac-message').textContent.includes('Blue-Eyes'));
    record('hint-code-popup', $('#ac-message').textContent.includes('宣言'));
    await waitFor(() => $('#ac-message').style.display === 'none', 3000);

    // OPSELECTED：desc id 经 ResolveDesc 解析
    WailsBridge.resolveDesc = async (id) => (id === 0x896311391 ? '把「青眼白龙」解放' : '');
    eventBus.emit('duel:hint', { type: 4, player: 1, data: 0x896311391 });
    await waitFor(() => $('#ac-message').style.display === 'block'
      && $('#ac-message').textContent.includes('把「青眼白龙」解放'));
    record('hint-opselected-popup', $('#ac-message').textContent.includes('选择'));
    await waitFor(() => $('#ac-message').style.display === 'none', 3000);
    WailsBridge.resolveDesc = async () => '';

    // --- stTip：悬停出现、移开消失、跟随坐标 ---
    const canvas = $('#duel-canvas-container');
    canvas.dispatchEvent(new CustomEvent('ygo:cardhover', { detail: { code: 89631139, info: { name: '青眼白龙', type: 0x11 }, x: 400, y: 300 } }));
    await waitFor(() => !!$('#st-tip'));
    record('tooltip-shows-on-hover', $('#st-tip').textContent === '青眼白龙');
    record('tooltip-monster-border', /248, 113/.test($('#st-tip').style.borderLeft));
    canvas.dispatchEvent(new CustomEvent('ygo:cardhoverend'));
    await waitFor(() => !$('#st-tip'));
    record('tooltip-hides-on-hover-end', true);

    // --- 选择进行中（store.selectHintText）：悬停 tip 首行附带选择提示 ---
    duelStore.armSelectHint('请选择要破坏的卡(1-2)');
    canvas.dispatchEvent(new CustomEvent('ygo:cardhover', { detail: { code: 89631139, info: { name: '青眼白龙', type: 0x11 }, x: 400, y: 300 } }));
    await waitFor(() => !!$('#st-tip-select-hint'));
    record('tooltip-shows-select-hint', $('#st-tip').textContent.includes('请选择要破坏的卡(1-2)')
      && $('#st-tip').textContent.includes('青眼白龙'));
    duelStore.disarmSelectHint();
    canvas.dispatchEvent(new CustomEvent('ygo:cardhoverend'));
    await waitFor(() => !$('#st-tip'));

    record('no-fatal', true);
  } catch (err) {
    checks.fatalMsg = String(err && err.stack || err);
    record('fatal', false);
  } finally {
    window.__specSmoke.ready = true;
  }
})();
