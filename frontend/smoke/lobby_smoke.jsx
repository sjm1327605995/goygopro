// Headless smoke for the P7 lobby wiring: the rule/mode dropdowns must drive
// the CreateGame hostInfo request (no more hardcoded duelRule:5), and the
// ready toggle must run the CTOS_UPDATE_DECK handshake before setReady.
import { createRoot } from 'react-dom/client';
import React from 'react';
import { WailsBridge, eventBus } from '../src/wails_bridge.ts';
import '../css/style.css';
import '../css/gframe-window.css';
import Lobby from '../src/components/Lobby.tsx';

const root = createRoot(document.getElementById('root'));
root.render(<Lobby onNavigate={() => {}} onDuelStart={() => {}} />);

window.__lobbySmoke = { checks: {}, ready: false };
const checks = window.__lobbySmoke.checks;
const record = (name, ok) => { checks[name] = ok; };

// Capture CreateGame hostInfo + deck handshake sends.
const createGameReqs = [];
WailsBridge.createGame = async (req, roomName, pass) => {
  createGameReqs.push({ req, roomName, pass });
  return { success: true };
};
// createRoom 现在未连接时会先自动连接（生产语义）；这里换成静默 mock，
// 默认 mock 会在 200ms 后伪造 host 身份的 type_change/player_enter，
// 落到流程中途会干扰后续身份断言。
WailsBridge.connectServer = async () => ({ success: true });
const updateDeckSends = [];
WailsBridge.updateDeck = (mainCards, sideCards) => updateDeckSends.push({ main: mainCards, side: sideCards });
const readySends = [];
WailsBridge.setReady = (v) => readySends.push(v);
WailsBridge.startDuel = () => {};

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
const fireChange = (el, value) => {
  const setter = Object.getOwnPropertyDescriptor(window.HTMLSelectElement.prototype, 'value').set;
  setter.call(el, value);
  el.dispatchEvent(new Event('change', { bubbles: true }));
};
const setInputVal = (el, value) => {
  Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set.call(el, value);
  el.dispatchEvent(new Event('input', { bubbles: true }));
};
// 软等待：到时返回 predicate 当前值而不是抛错（用于判定式断言）
const tryWaitFor = async (predicate, ms = 1500) => {
  const started = Date.now();
  while (!predicate() && Date.now() - started < ms) await new Promise((r) => setTimeout(r, 25));
  return predicate();
};

(async () => {
try {
  // React 18 createRoot.render 是异步提交，先等 Lobby 真正挂载。
  await waitFor(() => !!document.getElementById('lobby-screen'));
  await waitFor(() => !!$('lobby-duel-rule-select') && !!$('lobby-duel-mode-select'));
  record('lobby-renders-rule-select', true);

  // Find the 确定 button inside the room-create-panel（docs 原型 wCreateHost 底部按钮）.
  const createBtn = [...document.querySelectorAll('.room-create-panel .btn')].find((b) => b.textContent.includes('确定'));
  record('lobby-has-create-btn', !!createBtn);
  createBtn.click();
  await waitFor(() => createGameReqs.length === 1);
  const defReq = createGameReqs[0].req;
  record('default-duelrule-5', defReq.duelRule === 5);
  record('default-mode-0', defReq.mode === 0);
  record('default-lp-hand-draw', defReq.startLp === 8000 && defReq.startHand === 5 && defReq.drawCount === 1);
  record('create-room-passes-name-pass', createGameReqs[0].roomName === 'Championship Match' && createGameReqs[0].pass === '');

  // --- 换规则/模式后再次建房：下拉驱动 hostInfo ---
  fireChange($('lobby-duel-rule-select'), '4');
  fireChange($('lobby-duel-mode-select'), '1');
  createBtn.click();
  await waitFor(() => createGameReqs.length === 2);
  record('dropdown-drives-duelrule-4', createGameReqs[1].req.duelRule === 4);
  record('dropdown-drives-mode-1', createGameReqs[1].req.mode === 1);

  // --- 波 G-3：建房高级参数（gframe wCreateHost 控件） ---
  record('default-lflist-na', defReq.lflist === 0); // mock 首项 N/A 哈希 0
  record('advanced-controls-exist', !!$('lobby-lflist-select') && !!$('lobby-rule-select')
    && !!$('lobby-startlp') && !!$('lobby-starthand') && !!$('lobby-drawcount') && !!$('lobby-timelimit')
    && !!$('lobby-nocheck') && !!$('lobby-noshuffle'));
  // LFList 下拉由挂载后异步 listLFLists 填充
  await waitFor(() => $('lobby-lflist-select').options.length === 2);
  record('lflist-options-from-bridge', true);
  // 改满参数再建房：全量透传到 HostInfoReq
  fireChange($('lobby-lflist-select'), String(0x7dfcee6a));
  fireChange($('lobby-rule-select'), '1');
  setInputVal($('lobby-startlp'), '16000');
  setInputVal($('lobby-starthand'), '6');
  setInputVal($('lobby-drawcount'), '2');
  setInputVal($('lobby-timelimit'), '300');
  $('lobby-nocheck').click();
  $('lobby-noshuffle').click();
  createBtn.click();
  await waitFor(() => createGameReqs.length === 3);
  const req3 = createGameReqs[2].req;
  record('advanced-passthrough', req3.lflist === 0x7dfcee6a && req3.rule === 1
    && req3.startLp === 16000 && req3.startHand === 6 && req3.drawCount === 2 && req3.timeLimit === 300
    && req3.noCheckDeck === true && req3.noShuffleDeck === true,
    JSON.stringify(req3));

  // --- 波 G-3：规则信息面板（stHostPrepRule，duelclient.cpp:453-490） ---
  eventBus.emit('stoc:join_game', { Info: {
    LFList: 0x7dfcee6a, Rule: 1, Mode: 1, DuelRule: 4,
    NoCheckDeck: 1, NoShuffleDeck: 0,
    StartLp: 16000, StartHand: 6, DrawCount: 2, TimeLimit: 300,
  } });
  await waitFor(() => !!$('lobby-room-rule'));
  const ruleText = $('lobby-room-rule').textContent;
  record('rule-info-panel', ruleText.includes('禁限卡表：Test List') && ruleText.includes('卡片允许：ＴＣＧ')
    && ruleText.includes('决斗模式：比赛模式') && ruleText.includes('初始基本分：16000')
    && ruleText.includes('初始手卡数：6') && ruleText.includes('每回合抽卡：2')
    && ruleText.includes('每回合时间：300') && ruleText.includes('*不检查卡组')
    && ruleText.includes('*新大师规则（2017）') && !ruleText.includes('*不洗切卡组'), ruleText);
  // 默认规则（CURRENT_RULE=5）不显示规则名行
  record('default-rule-name-hidden', !ruleText.includes('*大师规则（2020）'));

  // --- CTOS_UPDATE_DECK 握手：启动本地服务器 → 选卡组 → 准备 ---
  WailsBridge.startLocalServer = async () => ({ success: true, port: 7911 });
  const startSrvBtn = [...document.querySelectorAll('#server-connect-panel .btn')].find((b) => b.textContent.includes('启动服务器'));
  startSrvBtn.click();
  await waitFor(() => !!$('lobby-deck-select') && $('lobby-deck-select').options.length > 0);
  // docs 原型两下拉：卡组分类 + 分类内卡组（mock 卡组 "Meta/Cyber Dragon OTK" 归 Meta 分类，
  // 未分类下 2 个）
  record('deck-select-populated', $('lobby-deck-select').options.length === 2
    && $('lobby-deck-category') && $('lobby-deck-category').options.length === 2);
  fireChange($('lobby-deck-category'), 'Meta');
  await waitFor(() => $('lobby-deck-select').options.length === 1
    && $('lobby-deck-select').textContent.includes('Cyber Dragon OTK'));
  record('deck-category-filters', true);

  const readyBtn = [...document.querySelectorAll('#room-lobby-panel .btn')].find((b) => b.textContent.includes('准备'));
  record('ready-btn-exists', !!readyBtn);
  readyBtn.click();
  await waitFor(() => readySends.length === 1 && readySends[0] === true);
  record('update-deck-handshake-ran', updateDeckSends.length === 1
    && Array.isArray(updateDeckSends[0].main)
    && updateDeckSends[0].main.length === 20 // mock deck main 18 + extra 2
    && updateDeckSends[0].side.length === 2);

  // --- 波 C：等候区补全 ---
  // 前面两次"创建房间"已置 isHost/inRoom：先点"离开房间"复位身份，
  // 验证加入行重新可见后再走加入流程
  WailsBridge.leaveGame = () => {};
  const leaveBtns = [...document.querySelectorAll('#room-lobby-panel .btn')].filter((b) => b.textContent.includes('退出'));
  leaveBtns[0].click();
  const joinSends = [];
  WailsBridge.joinGame = async (pass) => { joinSends.push(pass); return { success: true }; };
  await waitFor(() => $('lobby-join-btn') && $('lobby-join-btn').offsetParent !== null);
  record('leave-room-resets-host-and-room', true);
  const joinBtn = $('lobby-join-btn');
  joinBtn.click();
  await waitFor(() => joinSends.length === 1);
  record('join-game-sent-empty-pass', joinSends[0] === '');
  record('join-hides-after-success', await tryWaitFor(() => $('lobby-join-btn').offsetParent === null));

  // 座位事件：对方入场 → 准备 → 离开清座
  eventBus.emit('stoc:player_enter', { pos: 1, name: 'Opponent' });
  await waitFor(() => document.body.textContent.includes('Opponent'));
  record('opponent-seat-filled', true);
  eventBus.emit('stoc:player_change', { pos: 1, status: 0x9, ready: true });
  await waitFor(() => document.body.textContent.includes('已准备'));
  record('opponent-ready-shown', true);
  eventBus.emit('stoc:player_change', { pos: 1, status: 0xb, ready: false });
  record('leave-clears-seat', await tryWaitFor(() => document.body.textContent.includes('（空位）')));

  // 观战计数
  eventBus.emit('stoc:watch_change', { count: 3 });
  await waitFor(() => $('lobby-watch-count').textContent === '3');
  record('watch-count-updates', true);

  // 观战身份：转观战 → 等候区显示观战者按钮组 → 转回决斗者
  const toObserverSends = [];
  WailsBridge.toObserver = () => toObserverSends.push(1);
  const watchBtn = $('lobby-watch-btn');
  record('watch-btn-visible', !!watchBtn && watchBtn.offsetParent !== null);
  watchBtn.click();
  await waitFor(() => toObserverSends.length === 1);
  eventBus.emit('stoc:type_change', { type: 0x02, isHost: false, pos: 2 });
  await waitFor(() => !!$('lobby-to-duelist'));
  record('observer-hides-ready-panel', !$('lobby-deck-select'));
  record('observer-shows-watch-count', $('lobby-watch-count').offsetParent !== null);
  const toDuelistSends = [];
  WailsBridge.toDuelist = () => toDuelistSends.push(1);
  $('lobby-to-duelist').click();
  await waitFor(() => toDuelistSends.length === 1);
  record('to-duelist-sent', true);

  // 宿主踢人：房主身份 + 座位有人 → 踢出按钮发 CTOS_HS_KICK
  eventBus.emit('stoc:type_change', { type: 0x10, isHost: true, pos: 0 });
  eventBus.emit('stoc:player_enter', { pos: 1, name: 'KickMe' });
  const kickSends = [];
  WailsBridge.kickPlayer = (pos) => kickSends.push(pos);
  await waitFor(() => !!document.getElementById('lobby-kick-p2'));
  document.getElementById('lobby-kick-p2').click();
  await waitFor(() => kickSends.length === 1 && kickSends[0] === 1);
  record('kick-sends-seat-1', true);

  // error_msg 弹窗表（duelclient.cpp:261-368）：
  // JOINERROR code 1 → 密码错误；DECKERROR LFLIST 卡名 → 禁限卡表文案
  eventBus.emit('stoc:error_msg', { msg: 1, code: 1 });
  await waitFor(() => document.body.textContent.includes('房间密码错误'));
  record('joinerror-1-password', true);
  // 关闭弹窗（错误窗的确定按钮有专属 id，避免误点建房窗的「确定」）
  $('lobby-errmsg-ok').click();
  await waitFor(() => !document.body.textContent.includes('房间密码错误'));
  eventBus.emit('stoc:error_msg', { msg: 2, code: (1 << 28) | 89631139 }); // LFLIST | Blue-Eyes
  await waitFor(() => document.body.textContent.includes('Blue-Eyes White Dragon'));
  record('deckerror-lflist-cardname', true);
  $('lobby-errmsg-ok').click();

  record('no-fatal', true);
} catch (err) {
  checks.fatalMsg = String(err && err.stack || err); // 仅展示，判真
  record('fatal', false);
} finally {
  window.__lobbySmoke.ready = true;
  // 无头运行时经 document.title 外露结果（chrome --headless --dump-dom 可读）
  document.title = 'SMOKE:' + JSON.stringify(window.__lobbySmoke.checks);
}
})();
