// 波 H：主菜单 1:1 对齐 docs/layout_prototype.html 第 1 节 wMainMenu——
// 320×210 窗口（20% 顶部留白）、版本标题、5 个纵排按钮；点联机切 lobby、退出经 bridge。
import { createRoot } from 'react-dom/client';
import React from 'react';
import { WailsBridge } from '../src/wails_bridge.ts';
import '../css/style.css';
import '../css/gframe-window.css';
import MainMenu from '../src/components/MainMenu.tsx';

window.__menuSmoke = { checks: {}, ready: false };
const checks = window.__menuSmoke.checks;
const record = (name, ok, extra) => { checks[name] = extra === undefined ? ok : extra; };

const navs = [];
const quitSends = [];
const root = createRoot(document.getElementById('root'));
root.render(
  <MainMenu
    onDuel={() => navs.push('lobby')}
    onPractice={() => navs.push('practice')}
    onDeck={() => navs.push('deck')}
    onReplay={() => navs.push('replay')}
    onQuit={() => { WailsBridge.quit().then(() => quitSends.push(1)); }}
  />,
);

const $ = (id) => document.getElementById(id);
const waitFor = (predicate, timeoutMs = 5000) => new Promise((resolve, reject) => {
  const started = Date.now();
  const tick = () => {
    if (predicate()) return resolve();
    if (Date.now() - started > timeoutMs) return reject(new Error('waitFor timeout: ' + predicate));
    setTimeout(tick, 25);
  };
  tick();
});

(async () => {
try {
  await waitFor(() => !!$('main-menu-screen'));
  record('menu-screen-renders', true);

  // 原型窗口形态：gfw-mainmenu 尺寸 320×210（docs/layout_prototype.html 第 1 节）、标题为版本串
  const win = $('main-menu-window');
  const cls = win && win.className;
  record('menu-window-shape', !!win && cls.includes('gfw-window') && cls.includes('gfw-mainmenu'));
  record('menu-title-version', win && win.querySelector('.gfw-title')?.textContent === 'YGOPro Version:1.036.2');

  // 5 个按钮（SysString 1200/1201/1202/1204/1210）
  const btnTexts = [...win.querySelectorAll('button')].map((b) => b.textContent);
  record('menu-five-buttons', JSON.stringify(btnTexts) === JSON.stringify(['联机模式', '单人模式', '观看录像', '编辑卡组', '退出']), JSON.stringify(btnTexts));

  // 点击路由
  $('menu-btn-lan').click();
  $('menu-btn-single').click();
  $('menu-btn-replay').click();
  $('menu-btn-deck').click();
  record('menu-buttons-navigate', JSON.stringify(navs) === JSON.stringify(['lobby', 'practice', 'replay', 'deck']), JSON.stringify(navs));

  // 退出经 WailsBridge.quit（mock 环境为 no-op，可正常 resolve）
  $('menu-btn-exit').click();
  await waitFor(() => quitSends.length === 1);
  record('menu-exit-calls-quit', true);

  record('no-fatal', true);
} catch (err) {
  checks.fatalMsg = String(err && err.stack || err);
  record('fatal', false);
} finally {
  window.__menuSmoke.ready = true;
}
})();
