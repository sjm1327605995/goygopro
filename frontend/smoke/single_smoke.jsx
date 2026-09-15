// 波 J 冒烟：单人模式窗口（wSinglePlay 1:1）+ 谜题事件流进 store/manager。
// 窗口侧：gfw-single 580×420、列表装载、选中显示 message、复选框随动、
// 确定带 returnDeckTop 调 startSingle 并导航、失败显示错误、退出回主菜单。
// 数据侧：直接在 eventBus 上重放谜题事件（start/reload_field/ai_name/
// update_data 含空槽标记与本方手牌整表），断言 reducer/manager 的行为 ——
// 真实引擎那条线由 Go 侧 TestStartSingleDrivesPuzzleToWin 覆盖。
import { createRoot } from 'react-dom/client';
import React from 'react';
import { WailsBridge, eventBus } from '../src/wails_bridge.ts';
import { duelStore } from '../src/duel/store.ts';
import { DuelManager } from '../src/duel/duel_manager.ts';
import '../css/style.css';
import '../css/gframe-window.css';
import SingleModeWindow from '../src/components/SingleModeWindow.tsx';

window.__singleSmoke = { checks: {}, ready: false };
const checks = window.__singleSmoke.checks;
const record = (name, ok) => { checks[name] = ok; };

// ---- mock 桥（覆盖 mock 分支，捕获调用参数） ----
const SINGLES = [
  { name: '青眼一击.lua', message: '入门残局：用青眼的强大力量一击制胜！' },
  { name: '魔导师的初阵.lua', message: '用连锁完成突破。' },
];
let startCalls = [];
WailsBridge.listSingles = async () => SINGLES;
WailsBridge.startSingle = async (name, returnDeckTop) => {
  startCalls.push({ name, returnDeckTop: !!returnDeckTop });
  if (startCalls.length === 1) return { success: false, error: 'engine busy' };
  return { success: true, name };
};

const navs = [];
const starts = [];
let screen = null;
const root = createRoot(document.getElementById('root'));
screen = (
  <SingleModeWindow
    onNavigate={(s) => navs.push(s)}
    onStart={() => starts.push(1)}
  />
);
root.render(screen);

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
  await waitFor(() => !!$('single-screen'));
  record('single-screen-renders', true);

  // 原版窗口形态：gfw-single 580×420、标题「单人模式」
  const win = $('single-screen').querySelector('.gfw-window');
  record('single-window-shape', !!win && win.className.includes('gfw-single'));
  record('single-title', win && win.querySelector('.gfw-title')?.textContent === '单人模式');

  // 列表装载 + 选中（首项默认选中，信息面板显示 message）
  await waitFor(() => $('single-list') && $('single-list').children.length === 2);
  record('single-list-populated', true);
  record('single-info-first-selected',
    $('single-info') && $('single-info').textContent === SINGLES[0].message);

  // 换选第二项：信息面板跟随
  $('single-list').children[1].click();
  await waitFor(() => $('single-info').textContent === SINGLES[1].message);
  record('single-select-shows-message', true);

  // 复选框：勾上后确定要把 returnDeckTop 传给 startSingle
  $('single-return-decktop').click();
  await waitFor(() => $('single-return-decktop').checked === true);
  record('single-checkbox-toggles', true);

  // 确定（第一次 mock 返回失败）：显示错误、不导航
  $('single-start').click();
  await waitFor(() => !!$('single-error'));
  record('single-error-shown', starts.length === 0);
  record('single-start-args', startCalls[0].name === SINGLES[1].name && startCalls[0].returnDeckTop === true);

  // 确定（第二次成功）：导航到决斗画面
  $('single-start').click();
  await waitFor(() => starts.length === 1);
  record('single-start-navigates', true);

  // 退出回主菜单
  $('single-exit').click();
  record('single-exit-navigates', JSON.stringify(navs) === JSON.stringify(['menu']));

  // ---- 数据侧：谜题事件流 ----
  duelStore.reset();
  eventBus.emit('duel:start', {
    playerType: 0, duelRule: 5,
    lp0: 8000, lp1: 500, deck0: 0, extra0: 0, deck1: 0, extra1: 0,
  });
  let st = duelStore.getState();
  record('puzzle-start-lp', st.lp[0] === 8000 && st.lp[1] === 500);

  // MSG_AI_NAME：对手名（引擎 seat 1）
  eventBus.emit('duel:ai_name', { name: '残局AI' });
  record('puzzle-ai-name', duelStore.getState().names[1] === '残局AI');

  // MSG_RELOAD_FIELD：快照落 LP 与堆计数
  eventBus.emit('duel:reload_field', {
    rule: 1, chainCount: 0,
    players: [
      { lp: 8000, mzone: [], szone: [], deck: 5, hand: 2, grave: 3, removed: 1, extra: 0, extraP: 0 },
      { lp: 500, mzone: [], szone: [], deck: 0, hand: 1, grave: 0, removed: 0, extra: 0, extraP: 0 },
    ],
  });
  st = duelStore.getState();
  record('puzzle-reload-piles',
    st.piles[0].deck === 5 && st.piles[0].grave === 3 && st.piles[0].banish === 1
    && st.piles[1].deck === 0);

  // MSG_UPDATE_DATA：对手 mzone（seq2 恶魔的召唤，前后是空槽 null 标记）
  eventBus.emit('duel:update_data', {
    player: 1, location: 0x04,
    cards: [null, null, { code: 70781052, position: { p: 1 } }, null, null, null, null],
  });
  const board = duelStore.getState().board;
  record('puzzle-update-data-monster',
    board[1].mzone[2] && board[1].mzone[2].code === 70781052 && board[1].mzone[2].pos === 1
    && board[1].mzone[0] === null && board[1].mzone[3] === null);

  // 空槽标记要把之前占位的卡清掉（引擎权威：卡已离场）
  eventBus.emit('duel:update_data', {
    player: 1, location: 0x04,
    cards: [null, null, null, null, null, null, null],
  });
  record('puzzle-empty-marker-clears', duelStore.getState().board[1].mzone[2] === null);

  // MSG_UPDATE_DATA：本方手牌整表（谜题脚本布的手牌，没有 draw 事件）
  eventBus.emit('duel:update_data', {
    player: 0, location: 0x02,
    cards: [{ code: 89631139 }, { code: 46986414 }, null],
  });
  const hand = duelStore.getState().hand;
  record('puzzle-hand-rebuilt',
    hand.length === 3 && hand[0].code === 89631139 && hand[1].code === 46986414 && hand[2] === null);

  // DuelManager：reload_field 直接定对手手背行数
  const handCounts = [];
  const stubField = { setOpponentHandCount: (c) => handCounts.push(c) };
  const manager = new DuelManager(stubField, { interactive: false });
  eventBus.emit('duel:reload_field', {
    rule: 1, chainCount: 0,
    players: [
      { lp: 8000, mzone: [], szone: [], deck: 5, hand: 2, grave: 0, removed: 0, extra: 0, extraP: 0 },
      { lp: 500, mzone: [], szone: [], deck: 0, hand: 4, grave: 0, removed: 0, extra: 0, extraP: 0 },
    ],
  });
  record('puzzle-manager-opponent-hand', handCounts.length === 1 && handCounts[0] === 4);
  manager.dispose();

  record('no-fatal', true);
} catch (err) {
  checks.fatalMsg = String(err && err.stack || err);
  record('fatal', false);
} finally {
  window.__singleSmoke.ready = true;
}
})();
