// Headless smoke for the two-client lobby seat rendering: mounts the REAL
// Lobby.tsx twice (host A, then guest B), feeds each the exact event stream
// the server sends (same sequences the Go netplay tests pin down), and
// asserts the seat rows / ready flags / start-button enablement each client
// must see. Guards the「两个玩家都显示在位置1、开不了游戏」regression.
import { createRoot } from 'react-dom/client';
import React from 'react';
import { WailsBridge, eventBus } from '../src/wails_bridge.ts';
import '../css/style.css';
import '../css/gframe-window.css';
import Lobby from '../src/components/Lobby.tsx';

window.__netplaySmoke = { checks: {}, ready: false };
const checks = window.__netplaySmoke.checks;
const record = (name, ok) => { checks[name] = !!ok; };

const $ = (id) => document.getElementById(id);
const waitFor = (predicate, timeoutMs = 5000) => new Promise((resolve, reject) => {
  const started = Date.now();
  const tick = () => {
    if (predicate()) return resolve();
    if (Date.now() - started > timeoutMs) return reject(new Error('waitFor timeout'));
    setTimeout(tick, 25);
  };
  tick();
});

// Lobby 渲染的座位行没有独立 id：按 #room-lobby-panel 内「决斗者」列的
// 昵称 span 取文本（结构见 Lobby.tsx seatRow）。
const seatTexts = () => {
  const panel = $('room-lobby-panel');
  if (!panel) return [];
  // seatRow = X/span(昵称)/勾选框 三件套，昵称 span 带 20px 行高内联样式
  return [...panel.querySelectorAll('div div span')].filter((s) => s.style.height === '20px').map((s) => s.textContent);
};
const startBtn = () => [...document.querySelectorAll('#room-lobby-panel .btn')].find((b) => b.textContent === '开始');

const resetBridge = () => {
  WailsBridge.connectServer = async () => ({ success: true });
  WailsBridge.createGame = async () => ({ success: true });
  WailsBridge.joinGame = async () => ({ success: true });
  WailsBridge.listLFLists = async () => [{ hash: 0, name: 'N/A' }];
  WailsBridge.listDecks = async () => ['TestDeck'];
  WailsBridge.loadDeck = async () => ({ main: [89631139], extra: [], side: [] });
  WailsBridge.updateDeck = () => {};
  WailsBridge.setReady = () => {};
  WailsBridge.startDuel = () => {};
};

(async () => {
  const container = document.getElementById('root');
  try {
    // ============ 客户端 A（房主）============
    resetBridge();
    let root = createRoot(container);
    root.render(<Lobby onNavigate={() => {}} onDuelStart={() => {}} />);
    await waitFor(() => !!$('lobby-screen'));
    // 建立主机：未连接 → 自动连接本地 + 建房
    const createBtn = [...document.querySelectorAll('.room-create-panel .btn')].find((b) => b.textContent.includes('确定'));
    createBtn.click();
    await waitFor(() => $('room-lobby-panel') && $('room-lobby-panel').offsetParent !== null);
    record('A-in-room-after-create', true);

    // 服务器下发给 A 的事件序列（与 netplay 测试钉死的一致）
    eventBus.emit('stoc:type_change', { type: 0x10, isHost: true, pos: 0 });
    eventBus.emit('stoc:player_enter', { pos: 0, name: 'HostA' });
    await waitFor(() => seatTexts().length >= 2 && seatTexts()[0].includes('HostA'));
    record('A-sees-self-seat0', seatTexts()[0].includes('HostA'));

    // B 加入：A 收到 pos=1 的 player_enter
    eventBus.emit('stoc:player_enter', { pos: 1, name: 'GuestB' });
    await waitFor(() => seatTexts()[1].includes('GuestB'));
    record('A-sees-guest-seat1', seatTexts()[1].includes('GuestB'));
    record('A-seat0-unchanged', seatTexts()[0].includes('HostA'));

    // 双方准备 → 开始按钮亮起
    eventBus.emit('stoc:player_change', { pos: 0, status: 0x9, ready: true });
    eventBus.emit('stoc:player_change', { pos: 1, status: 0x9, ready: true });
    await waitFor(() => startBtn() && !startBtn().disabled);
    record('A-start-enabled-when-both-ready', true);
    root.unmount();

    // ============ 客户端 B（加入者）============
    resetBridge();
    container.innerHTML = '';
    root = createRoot(container);
    root.render(<Lobby onNavigate={() => {}} onDuelStart={() => {}} />);
    await waitFor(() => !!$('lobby-screen'));
    // 「加入游戏」一键语义：连接 + 立即 JOIN；GUI 会预填自己到 0 号位，
    // 必须由服务器 player_enter 事件纠正
    const joinGameBtn = [...document.querySelectorAll('#server-connect-panel .btn')].find((b) => b.textContent.includes('加入游戏'));
    joinGameBtn.click();
    await waitFor(() => $('room-lobby-panel') && $('room-lobby-panel').offsetParent !== null);
    record('B-in-room-after-join', true);

    // 服务器下发给 B 的事件序列
    eventBus.emit('stoc:type_change', { type: 0x01, isHost: false, pos: 1 });
    eventBus.emit('stoc:player_enter', { pos: 0, name: 'HostA' });
    eventBus.emit('stoc:player_enter', { pos: 1, name: 'GuestB' });
    await waitFor(() => seatTexts().length >= 2 && seatTexts()[1].includes('GuestB'));
    record('B-sees-self-seat1', seatTexts()[1].includes('GuestB'));
    // 关键回归点：B 预填的 0 号位必须被 HostA 覆盖，不能两个都挤在位置1
    record('B-sees-host-seat0', seatTexts()[0].includes('HostA'));

    eventBus.emit('stoc:player_change', { pos: 0, status: 0x9, ready: true });
    eventBus.emit('stoc:player_change', { pos: 1, status: 0x9, ready: true });
    await waitFor(() => seatTexts()[0].includes('已准备') && seatTexts()[1].includes('已准备'));
    record('B-sees-both-ready', true);
    // B 不是房主：没有开始按钮，但有准备按钮
    record('B-no-start-button', !startBtn());

    record('no-fatal', true);
  } catch (err) {
    checks.fatalMsg = String((err && err.stack) || err);
    record('fatal', false);
  } finally {
    window.__netplaySmoke.ready = true;
    document.title = 'SMOKE:' + JSON.stringify(window.__netplaySmoke.checks);
  }
})();
