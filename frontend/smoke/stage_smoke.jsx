// 波 7 HUD 几何冒烟：整块 DuelStage（3D 场地 + 全部 2D 面板）按原版 gframe
// 坐标落位（drawing.cpp / game.cpp 的 1024×640 布局，按窗口比例断言）。
// 断言读 getBoundingClientRect 的几何，不依赖像素截图。
import { createRoot } from 'react-dom/client';
import React from 'react';
import { eventBus, WailsBridge } from '../src/wails_bridge.ts';
import '../css/style.css';
import '../css/duel-original.css';
import DuelStage from '../src/components/DuelStage.tsx';

// 卡面必须能和卡背（cover.jpg 漩涡）区分开，且自带方向标记：
// 顶部▲箭头 + 卡号，正面朝上时▲应指向屏幕上方。
const FRONT_SVG = (code) => 'data:image/svg+xml;utf8,' + encodeURIComponent(
  `<svg xmlns='http://www.w3.org/2000/svg' width='512' height='744'>` +
  `<rect width='512' height='744' fill='#c28544'/>` +
  `<text x='256' y='130' font-size='72' font-weight='bold' fill='#000' text-anchor='middle'>▲</text>` +
  `<text x='256' y='400' font-size='90' font-weight='bold' fill='#000' text-anchor='middle'>FACE</text>` +
  `<text x='256' y='700' font-size='60' fill='#000' text-anchor='middle'>${code}</text></svg>`
);
WailsBridge.getCardImage = async (code) => ({ url: FRONT_SVG(code), full: true });
WailsBridge.getCard = async (code) => ({
  code, name: 'Blue-Eyes White Dragon', type: 0x11, attack: 3000, defense: 2500, desc: '', strings: [],
});

const root = createRoot(document.getElementById('root'));
root.render(
  <div style={{ position: 'relative', width: '100vw', height: '100vh', overflow: 'hidden' }}>
    <DuelStage
      interactive={false}
      showSurrender={false}
      onReady={(h) => { window.__stage = h; }}
    />
  </div>
);

window.__stageSmoke = { checks: {}, ready: false };
const checks = window.__stageSmoke.checks;
const record = (name, ok) => { checks[name] = ok; };

const $ = (sel) => document.querySelector(sel);
const rectOf = (sel) => {
  const el = typeof sel === 'string' ? $(sel) : sel;
  return el ? el.getBoundingClientRect() : null;
};
const waitFor = (predicate, label, timeoutMs = 8000) => new Promise((resolve, reject) => {
  const started = Date.now();
  const tick = () => {
    if (predicate()) return resolve();
    if (Date.now() - started > timeoutMs) return reject(new Error('waitFor timeout: ' + label));
    setTimeout(tick, 25);
  };
  tick();
});

(async () => {
  try {
    const W = window.innerWidth;
    const H = window.innerHeight;

    // ---- 种一份可见的对局状态 ----
    eventBus.emit('duel:start', {
      playerType: 1, duelRule: 5, lp0: 8000, lp1: 4000,
      deck0: 40, extra0: 15, deck1: 37, extra1: 12,
    });
    eventBus.emit('stoc:player_enter', { pos: 0, name: 'HostPlayer' });
    eventBus.emit('stoc:player_enter', { pos: 1, name: 'Challenger' });
    eventBus.emit('duel:draw', { player: 1, count: 5, cards: [89631139, 46986414, 55144522, 44095762, 83764718] });
    eventBus.emit('duel:new_turn', { player: 1 });
    eventBus.emit('duel:new_phase', { phase: 0x10 });
    eventBus.emit('duel:move', {
      code: 89631139, pc: 1, pl: 0x02, ps: 2, pp: 0x1,
      cc: 1, cl: 0x10, cs: 2, cp: 0x1, reason: 0,
    });
    eventBus.emit('stoc:chat', { player: 0, msg: '决斗吧！' });

    await waitFor(() => $('#player-panel') && $('#opponent-panel') && $('#hand-cards-dock')
      && $('#hand-cards-dock').children.length >= 4, 'panels+hand mount');
    await waitFor(() => rectOf('#phase-strip') && rectOf('.card-preview-panel'), 'panels render');
    // 聊天监听器随组件挂载，首条在挂载前发出会丢；挂载后再发一条保证可见
    eventBus.emit('stoc:chat', { player: 0, msg: '决斗吧！' });

    // ---- 左侧预览面板：原版 wCardImg(1,1)-(198,273) + wInfos(1,275)-(301,639)
    // 合并列，固定左上、宽约 300，1024 宽度下必须可见 ----
    const pv = rectOf('.card-preview-panel');
    record('preview-visible-at-1024', pv.width > 0 && pv.height > 100);
    record('preview-anchored-topleft', pv.left <= 3 && pv.top <= 3, JSON.stringify(pv));
    record('preview-original-width', pv.width >= 290 && pv.width <= 310, String(pv.width));

    // ---- LP 条：两条都在顶部（p0 左中 32%、p1 右侧 3.1%），LP 数字在框上 ----
    const pp = rectOf('#player-panel');
    const op = rectOf('#opponent-panel');
    record('lp-top-anchored', pp.top <= 20 && op.top <= 20, `pp.top=${pp.top} op.top=${op.top}`);
    record('lp-player-left-band', pp.left >= W * 0.28 && pp.left <= W * 0.36, `pp.left=${pp.left} W=${W}`);
    record('lp-opponent-right-band', op.right >= W * 0.95, `op.right=${op.right}`);
    record('lp-values-on-frame', $('#player-panel-lp').innerText === '4000'
      && $('#opponent-panel-lp').innerText === '8000');
    // lpf.png 被 vite 内联成 data URL，只断言背景图确实解析成功
    record('lp-frame-uses-texture', getComputedStyle($('.lp-frame')).backgroundImage.startsWith('url('));

    // ---- 回合数菱形块：顶部居中 ----
    const tb = rectOf('#turn-block');
    record('turn-block-centered', tb && Math.abs(tb.left + tb.width / 2 - W / 2) < 30
      && tb.top <= 20, JSON.stringify(tb));

    // ---- 阶段条：原版 wPhase (480,310)-(855,330) → 右中横向 ----
    const ps = rectOf('#phase-strip');
    record('phase-strip-mid-height', ps.top >= H * 0.42 && ps.top <= H * 0.58, `ps.top=${ps.top} H=${H}`);
    record('phase-strip-right-band', ps.right >= W * 0.75 && ps.right <= W * 0.92, `ps.right=${ps.right}`);
    record('phase-strip-horizontal', ps.width > ps.height * 2, `${ps.width}x${ps.height}`);

    // ---- 手牌：底部居中（原版手牌画在场地底部） ----
    const hd = rectOf('#hand-cards-dock');
    record('hand-bottom-centered', hd.bottom >= H - 70 && hd.bottom <= H
      && Math.abs(hd.left + hd.width / 2 - W / 2) < W * 0.06, JSON.stringify(hd));
    record('hand-after-summon', $('#hand-cards-dock').children.length === 4);

    // ---- 聊天：底部输入条 + 着色消息（原版 wChat 全宽条） ----
    const chatInput = rectOf('.chat-overlay input');
    record('chat-input-bottom-bar', chatInput && chatInput.bottom >= H - 12
      && chatInput.left >= W * 0.28, JSON.stringify(chatInput));
    await waitFor(() => $('.chat-line.chat-p0'), 'chat line shows');
    record('chat-line-colored', !!$('.chat-line.chat-p0'));

    // ---- 提示条：顶部偏右（原版 hint 中心 ~64%） ----
    eventBus.emit('duel:waiting', {});
    await waitFor(() => rectOf('.hint-bar') && $('.hint-bar').style.display === 'block', 'hint bar shows');
    const hb = rectOf('.hint-bar');
    record('hint-top-right-of-center', Math.abs(hb.top - 60) <= 12
      && hb.left + hb.width / 2 > W * 0.55, JSON.stringify(hb));

    // ---- 朝向陈列：双方各放 ATK / 守备表 / 盖卡 / 墓地卡，肉眼对照 ----
    await waitFor(() => window.__stage && window.__stage.field3D, 'field3d ready');
    const f = window.__stage.field3D;
    f.placeCard(0, 'mzone', 0, 89631139, null, 0x1); // 己方 ATK
    f.placeCard(0, 'mzone', 2, 89631139, null, 0x4); // 己方守备表侧（0x4=FACEUP_DEFENSE，别用掩码 0xc）
    f.placeCard(0, 'szone', 2, 89631139, null, 0xa); // 己方盖卡
    f.placeCard(1, 'mzone', 0, 89631139, null, 0x1); // 对方 ATK
    f.placeCard(1, 'mzone', 2, 89631139, null, 0x4); // 对方守备表侧
    f.placeCard(1, 'szone', 2, 89631139, null, 0xa); // 对方盖卡
    f.placeCard(0, 'grave', 0, 83764718, null, 0x1);
    f.placeCard(1, 'grave', 0, 44095762, null, 0x1);
    record('orientation-gallery-placed', f.cardsOnField[0].mzone[0] && f.cardsOnField[1].mzone[0]);

    // ---- 3D 朝向语义：表侧攻击正面朝上（p1 平面转 180°）、守备顶边指向
    // 己方右手边（原版 client_field.cpp Z=∓PI/2）、盖卡 x=PI 背面朝上 ----
    const rotOf = (p, loc, s) => {
      const m = f.cardsOnField[p][loc][s];
      return m ? { x: m.rotation.x, y: m.rotation.y } : null;
    };
    const near = (a, b) => Math.abs(a - b) < 0.01;
    const p0atk = rotOf(0, 'mzone', 0), p1atk = rotOf(1, 'mzone', 0);
    record('atk-faceup-p0-upright', p0atk && near(p0atk.x, 0) && near(p0atk.y, 0), JSON.stringify(p0atk));
    record('atk-faceup-p1-flipped', p1atk && near(p1atk.x, 0) && near(p1atk.y, Math.PI), JSON.stringify(p1atk));
    const p0def = rotOf(0, 'mzone', 2), p1def = rotOf(1, 'mzone', 2);
    record('def-top-toward-owner-right', p0def && p1def
      && near(p0def.y, -Math.PI / 2) && near(p1def.y, Math.PI / 2),
      `p0=${p0def && p0def.y} p1=${p1def && p1def.y}`);
    const p0set = rotOf(0, 'szone', 2), p1set = rotOf(1, 'szone', 2);
    record('set-facedown-back-up', p0set && p1set
      && near(p0set.x, Math.PI) && near(p1set.x, Math.PI),
      JSON.stringify([p0set, p1set]));

    record('no-fatal', true);
  } catch (err) {
    checks.fatalMsg = String(err && err.stack || err);
    record('fatal', false);
  } finally {
    window.__stageSmoke.ready = true;
  }
})();
