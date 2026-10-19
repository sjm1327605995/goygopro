// Headless smoke for the P4 wave-6 completion: DrawSpec overlay (flip / zoom /
// mask reveal / negated + coin/dice ACMessage) and the stTip hover tooltip.
// All spec cards are mocked to a data URL so no network is involved.
import UiRoot from '../src/ui/UiRoot.tsx';
import { createRoot } from 'react-dom/client';
import React from 'react';
import { WailsBridge, eventBus } from '../src/wails_bridge.ts';
import { duelStore } from '../src/duel/store.ts';
import '../css/style.css';
import '../css/duel-original.css';
import '../css/duel-neos.css';
import SpecOverlay from '../src/components/SpecOverlay.tsx';
import CardTooltip from '../src/components/CardTooltip.tsx';
import { drawSpecFrame, cardHintRect, rpsOffset } from '../src/duel/draw_spec.ts';

const FAKE_PIC = 'data:image/gif;base64,R0lGODlhAQABAIAAAAUEBAAAACwAAAAAAQABAAACAkQBADs=';
WailsBridge.getCardImage = async () => ({ url: FAKE_PIC, full: false });
WailsBridge.getCard = async (code) => ({ code, name: 'Blue-Eyes White Dragon', type: 0x11, attack: 3000, defense: 2500, desc: '' });

const root = createRoot(document.getElementById('root'));
root.render(
  <UiRoot>
  <div className="duel-stage" style={{ position: 'relative', width: '100vw', height: '100vh' }}>
    <div id="duel-canvas-container"></div>
    <SpecOverlay />
    <CardTooltip />
  </div>
  </UiRoot>
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
    // Check the visible output against the source rectangles and sprite indices.
    const pixelCanvas = document.createElement('canvas');
    pixelCanvas.width = 1024; pixelCanvas.height = 640;
    const ctx = pixelCanvas.getContext('2d');
    const card = document.createElement('canvas'); card.width = 177; card.height = 254;
    card.getContext('2d').fillStyle = '#123456'; card.getContext('2d').fillRect(0, 0, 177, 254);
    const number = document.createElement('canvas'); number.width = 320; number.height = 256;
    number.getContext('2d').fillStyle = '#ff0000'; number.getContext('2d').fillRect(128, 0, 64, 64);
    const images = { card, number, mask: null, negated: null };
    const pixel = (x, y) => [...ctx.getImageData(x, y, 1, 1).data].join(',');
    drawSpecFrame(ctx, 'flip', 0, images, { sx: 1, sy: 1 });
    record('flip-starts-edge-on', pixel(662, 277) === '0,0,0,0');
    drawSpecFrame(ctx, 'flip', 10, images, { sx: 1, sy: 1 });
    record('flip-opens-upright-at-source-rectangle', pixel(600, 200) === '18,52,86,255' && pixel(573, 200) === '0,0,0,0');
    drawSpecFrame(ctx, 'number', 16, images, { sx: 1, sy: 1 }, 3);
    record('turn-counter-uses-number-sprite-cell', pixel(664, 269) === '255,0,0,255' && pixel(600, 200) === '18,52,86,255');
    record('card-hint-keeps-aspect-in-wide-window', cardHintRect(574, 150, 751, 404, { sx: 1.5, sy: 1 }).join(',') === '905,150,177,254');
    record('rps-source-frame-offsets', rpsOffset(0) === 50 && rpsOffset(1) === 60 && rpsOffset(29) === rpsOffset(59));

    // --- 翻卡（duel:summoning → showcard=7） ---
    eventBus.emit('duel:summoning', { code: 89631139, cc: 0, cl: 4, cs: 2, cp: 1 });
    await waitFor(() => overlay.style.display === 'block' && !!$('.spec-flip'));
    record('summoning-plays-flip', true);

    // --- 排队：立刻再来一个 zoom，flip 播完后接续 ---
    eventBus.emit('duel:spsummoning', { code: 84013237, cc: 0, cl: 4, cs: 1, cp: 1 });
    await sleep(650); // 原版 flip 30 帧，随后 zoom 30 帧。
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

    // MSG_CONFIRM_CARDS uses the field/list confirmation path, no DrawSpec card.
    eventBus.emit('duel:confirm_cards', {
      player: 0, skipPanel: 0,
      cards: [{ code: 89631139, c: 0, l: 4, s: 0 }],
    });
    await sleep(100);
    record('confirm-cards-no-extra-overlay', overlay.style.display === 'none');

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

    // --- showcard=100：猜拳双拳下落对碰（res = 我 | 对方<<2；1=剪刀 2=石头 3=布） ---
    eventBus.emit('duel:hand_res', { res: 1 | (3 << 2) }); // 我剪刀 vs 对方布
    await waitFor(() => !!$('#rps-arena'));
    record('hand-res-plays-rps', $('#rps-arena .rps-hand-bottom').src.endsWith('/f1.jpg')
      && $('#rps-arena .rps-hand-top').src.endsWith('/f3.jpg'));
    await waitFor(() => !$('#rps-arena'), 3000);
    eventBus.emit('duel:start', { playerType: 1, lp0: 8000, lp1: 8000 });
    eventBus.emit('duel:hand_res', { res: 1 | (3 << 2) });
    await waitFor(() => !!$('#rps-arena'));
    record('hand-res-maps-second-player', $('#rps-arena .rps-hand-bottom').src.endsWith('/f3.jpg')
      && $('#rps-arena .rps-hand-top').src.endsWith('/f1.jpg'));
    await waitFor(() => !$('#rps-arena'), 3000);

    // --- 猜硬币/骰子：图标动画 + ACMessage 文本弹条 ---
    eventBus.emit('duel:toss_coin', { player: 0, results: [1, 0] });
    record('coin-original-text-only', !$('#toss-arena'));
    await waitFor(() => $('#ac-message') && $('#ac-message').style.display === 'block');
    record('coin-popup-text', $('#ac-message').textContent === '投掷硬币结果：[正面][反面]');
    await waitFor(() => $('#ac-message').style.display === 'none' && !$('#toss-arena'), 3000);

    eventBus.emit('duel:toss_dice', { player: 0, results: [3, 5] });
    record('dice-original-text-only', !$('#toss-arena'));
    await waitFor(() => $('#ac-message').style.display === 'block');
    record('dice-popup-text', $('#ac-message').textContent === '投掷骰子结果：[3][5]');
    await waitFor(() => $('#ac-message').style.display === 'none', 3000);

    // --- HINT 宣言展示（duelclient.cpp:1108-1145）：OPSELECTED/RACE/ATTRIB/
    // CODE/NUMBER → stACMessage 浮条（SysString 1510/1511/1512） ---
    eventBus.emit('duel:hint', { type: 9, player: 1, data: 5 });
    await waitFor(() => $('#ac-message').style.display === 'block');
    record('hint-number-popup', $('#ac-message').textContent === '玩家选择了：[5]');
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

    // A delayed image from an old duel must never restart an obsolete animation.
    const loadCardImage = WailsBridge.getCardImage;
    let finishOldImage;
    WailsBridge.getCardImage = () => new Promise(resolve => { finishOldImage = resolve; });
    eventBus.emit('duel:hint', { type: 10, player: 0, data: 89631139 });
    await waitFor(() => !!finishOldImage);
    eventBus.emit('duel:start', { playerType: 0, lp0: 8000, lp1: 8000 });
    finishOldImage({ url: FAKE_PIC });
    WailsBridge.getCardImage = loadCardImage;
    await sleep(100);
    record('new-duel-discards-old-image-result', overlay.style.display === 'none');

    eventBus.emit('duel:hint', { type: 9, player: 0, data: 1 });
    await sleep(200);
    eventBus.emit('duel:hint', { type: 9, player: 0, data: 2 });
    await sleep(500);
    record('new-message-keeps-own-duration', $('#ac-message').style.display === 'block'
      && $('#ac-message').textContent === '玩家选择了：[2]');
    await waitFor(() => $('#ac-message').style.display === 'none');

    // --- stTip：悬停出现、移开消失、跟随坐标；多行信息（原版 stTip 的
    // 攻守/星阶/种族属性行）---
    const blueEyes = { name: '青眼白龙', type: 0x11, attack: 3000, defense: 2500, level: 8, race: 0x2000, attribute: 0x10 };
    const canvas = $('#duel-canvas-container');
    canvas.dispatchEvent(new CustomEvent('ygo:cardhover', { detail: { code: 89631139, info: blueEyes, x: 400, y: 300 } }));
    await waitFor(() => !!$('#st-tip'));
    record('tooltip-shows-on-hover', $('#st-tip').textContent.includes('青眼白龙'));
    record('tooltip-neos-dark-block', getComputedStyle($('#st-tip')).backgroundColor==='rgba(13, 21, 34, 0.94)' && !$('#st-tip').style.background);
    const tipRect=$('#st-tip').getBoundingClientRect();
    record('tooltip-original-upper-left-position',Math.abs(tipRect.right-390)<1&&Math.abs(tipRect.bottom-290)<1);
    record('tooltip-stat-lines', $('#st-tip').textContent.includes('3000/2500')
      && $('#st-tip').textContent.includes('★8')
      && $('#st-tip').textContent.includes('龙/光'));
    // 实时值（live）优先于静态卡数据（同步后的攻守变化）
    canvas.dispatchEvent(new CustomEvent('ygo:cardhoverend'));
    await waitFor(() => !$('#st-tip'));
    canvas.dispatchEvent(new CustomEvent('ygo:cardhover', { detail: { code: 89631139, info: blueEyes, live: { attack: 3800, level: 9 }, x: 400, y: 300 } }));
    await waitFor(() => !!$('#st-tip'));
    record('tooltip-live-stats-override', $('#st-tip').textContent.includes('3800/2500')
      && $('#st-tip').textContent.includes('★9'));
    canvas.dispatchEvent(new CustomEvent('ygo:cardhoverend'));
    await waitFor(() => !$('#st-tip'));
    record('tooltip-hides-on-hover-end', true);

    canvas.dispatchEvent(new CustomEvent('ygo:cardhover', { detail: { code: 89631139, info: blueEyes, x: 0, y: 0 } }));
    await waitFor(() => !!$('#st-tip'));
    const edgeTipRect = $('#st-tip').getBoundingClientRect();
    record('tooltip-stays-inside-viewport', edgeTipRect.left >= 0 && edgeTipRect.top >= 0
      && edgeTipRect.right <= window.innerWidth && edgeTipRect.bottom <= window.innerHeight,
      JSON.stringify({ left: edgeTipRect.left, top: edgeTipRect.top, right: edgeTipRect.right, bottom: edgeTipRect.bottom }));
    record('tooltip-semantics', $('#st-tip').getAttribute('role') === 'tooltip');
    canvas.dispatchEvent(new CustomEvent('ygo:cardhoverend'));
    await waitFor(() => !$('#st-tip'));

    // --- 选择进行中（store.selectHintText）：悬停 tip 首行附带选择提示 ---
    duelStore.armSelectHint('请选择要破坏的卡(1-2)');
    canvas.dispatchEvent(new CustomEvent('ygo:cardhover', { detail: { code: 89631139, info: { name: '青眼白龙', type: 0x11 }, x: 400, y: 300 } }));
    await waitFor(() => !!$('#st-tip'));
    record('tooltip-selection-hint-remains-separate',!$('#st-tip-select-hint')&&!$('#st-tip').textContent.includes('请选择要破坏的卡(1-2)')
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
