// Headless smoke for the P4 wave-6 completion: DrawSpec overlay (flip / zoom /
// mask reveal / negated + coin/dice ACMessage) and the stTip hover tooltip.
// All spec cards are mocked to a data URL so no network is involved.
import { createRoot } from 'react-dom/client';
import React from 'react';
import { WailsBridge, eventBus } from '../src/wails_bridge.ts';
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

    // --- 猜硬币/骰子 ACMessage 文本弹条 ---
    eventBus.emit('duel:toss_coin', { player: 0, results: [1, 0] });
    await waitFor(() => $('#ac-message') && $('#ac-message').style.display === 'block');
    record('coin-popup-text', $('#ac-message').textContent.includes('正面') && $('#ac-message').textContent.includes('反面'));
    await waitFor(() => $('#ac-message').style.display === 'none', 3000);

    eventBus.emit('duel:toss_dice', { player: 0, results: [3, 5] });
    await waitFor(() => $('#ac-message').style.display === 'block');
    record('dice-popup-text', $('#ac-message').textContent.includes('3') && $('#ac-message').textContent.includes('5'));
    await waitFor(() => $('#ac-message').style.display === 'none', 3000);

    // --- stTip：悬停出现、移开消失、跟随坐标 ---
    const canvas = $('#duel-canvas-container');
    canvas.dispatchEvent(new CustomEvent('ygo:cardhover', { detail: { code: 89631139, info: { name: '青眼白龙', type: 0x11 }, x: 400, y: 300 } }));
    await waitFor(() => !!$('#st-tip'));
    record('tooltip-shows-on-hover', $('#st-tip').textContent === '青眼白龙');
    record('tooltip-monster-border', /248, 113/.test($('#st-tip').style.borderLeft));
    canvas.dispatchEvent(new CustomEvent('ygo:cardhoverend'));
    await waitFor(() => !$('#st-tip'));
    record('tooltip-hides-on-hover-end', true);

    record('no-fatal', true);
  } catch (err) {
    checks.fatalMsg = String(err && err.stack || err);
    record('fatal', false);
  } finally {
    window.__specSmoke.ready = true;
  }
})();
