// 回放剧场冒烟：真实 ReplayTheater 组件 + mock playReplay 事件流。
// 目标：复现/回归「回放一直闪烁、看不到卡牌」——
// 插桩 clearBoard / duelStore.reset 计数，播放期间若持续增长即为重建循环；
// 并断言播放推进后场上真的有卡（3D mesh + store 面板）。
// 波 I：列表元信息面板 / 播放起始于回合（skip 语义）/ 提取卡组。
import { createRoot } from 'react-dom/client';
import React from 'react';
import { WailsBridge, eventBus } from '../src/wails_bridge.ts';
import { duelStore } from '../src/duel/store.ts';
import { settingsStore } from '../src/domain/settings.ts';
import { DuelField3D } from '../src/duel/field3d.ts';
import ReplayTheater from '../src/components/ReplayTheater.tsx';
import ReplaySavePrompt from '../src/components/ReplaySavePrompt.tsx';
import '../css/style.css';
import '../css/duel-original.css';

// ---- mock 卡图/卡信息（带 ▲ 方向标记，方便肉眼核对朝向） ----
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

// ---- mock 回放库与事件流（回放走与实况同一套 duel:* 事件） ----
const EVENTS = [
  { type: 'duel:start', data: { playerType: 1, duelRule: 5, lp0: 8000, lp1: 8000, deck0: 40, extra0: 15, deck1: 40, extra1: 15 } },
  { type: 'stoc:player_enter', data: { pos: 0, name: 'HostPlayer' } },
  { type: 'stoc:player_enter', data: { pos: 1, name: 'Challenger' } },
  { type: 'duel:draw', data: { player: 0, count: 5, cards: [89631139, 46986414, 55144522, 44095762, 83764718] } },
  { type: 'duel:draw', data: { player: 1, count: 5, cards: [89631139, 46986414, 55144522, 44095762, 83764718] } },
  { type: 'duel:new_turn', data: { player: 0 } },
  { type: 'duel:new_phase', data: { phase: 0x10 } },
  // 通常召唤青眼到 p0 mzone[2]（LOC_HAND=0x02 → LOC_MZONE=0x04）
  { type: 'duel:move', data: { code: 89631139, pc: 0, pl: 0x02, ps: 0, pp: 0x1, cc: 0, cl: 0x04, cs: 2, cp: 0x1, reason: 0 } },
  { type: 'duel:lp_update', data: { player: 0, lp: 4000, reason: 0 } },
  // 青眼被送去墓地（LOC_GRAVE=0x10）
  { type: 'duel:move', data: { code: 89631139, pc: 0, pl: 0x04, ps: 2, pp: 0x1, cc: 0, cl: 0x10, cs: 0, cp: 0x1, reason: 0 } },
  { type: 'duel:new_turn', data: { player: 1 } },
];
WailsBridge.listReplays = async () => replayList;
let replayList = ['flicker_repro.yrp'];

// ---- 波 E：录像落盘/改名/删除的 mock 捕获 ----
const replayMutations = { deleted: [], renamed: [], saved: [] };
WailsBridge.saveLastReplay = async (n) => { replayMutations.saved.push(n); return { success: true, name: (n || '_LastReplay') + '.yrp' }; };
WailsBridge.deleteReplay = async (n) => { replayMutations.deleted.push(n); replayList = replayList.filter((x) => x !== n); return { success: true }; };
WailsBridge.renameReplay = async (o, n) => { replayMutations.renamed.push([o, n]); replayList = replayList.map((x) => (x === o + '.yrp' || x === o ? n + '.yrp' : x)); return { success: true }; };
WailsBridge.saveConfig = async (p) => p;
window.prompt = () => 'renamed_replay';
window.confirm = () => true;
window.alert = () => {};
WailsBridge.playReplay = async (name) => ({
  success: true, name, players: ['HostPlayer', 'Challenger'], startLp: 8000, duelRule: 5,
  events: EVENTS,
});

// ---- 波 I：录像信息 / 提取卡组的 mock 捕获 ----
const infoCalls = [];
WailsBridge.replayInfo = async (name) => { infoCalls.push(name); return { success: true, date: '2026/09/14 12:00:00', players: ['HostPlayer', 'Challenger'], isTag: false, isSingle: false }; };
const exportCalls = [];
WailsBridge.exportReplayDeck = async (name) => { exportCalls.push(name); return { success: true, files: [name + '-1 Player1.ydk'] }; };

// ---- 插桩：抓「重建循环」 ----
window.__theaterProbe = { clearBoardCalls: 0, resetCalls: 0 };
const origClear = DuelField3D.prototype.clearBoard;
DuelField3D.prototype.clearBoard = function (...args) {
  window.__theaterProbe.clearBoardCalls += 1;
  window.__theaterProbe.field = this;
  return origClear.apply(this, args);
};
const origReset = duelStore.reset.bind(duelStore);
duelStore.reset = (...args) => {
  window.__theaterProbe.resetCalls += 1;
  return origReset(...args);
};

const root = createRoot(document.getElementById('root'));
root.render(
  <div style={{ position: 'relative', width: '100vw', height: '100vh', overflow: 'hidden' }}>
    <ReplayTheater onNavigate={() => {}} />
    <ReplaySavePrompt />
  </div>
);

window.__theaterSmoke = { checks: {}, ready: false };
const checks = window.__theaterSmoke.checks;
const $ = (sel) => document.querySelector(sel);

const clickListItem = (name) => {
  const item = [...document.querySelectorAll('#replay-list .gfw-list-item')].find((el) => el.textContent === name);
  if (!item) throw new Error('list item not found: ' + name);
  item.click();
};
// 受控数字输入：原生 value setter + input 事件
const setInputVal = (el, v) => {
  const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
  setter.call(el, v);
  el.dispatchEvent(new Event('input', { bubbles: true }));
};

(async () => {
  try {
    const waitFor = (predicate, label, timeoutMs = 12000) => new Promise((resolve, reject) => {
      const started = Date.now();
      const tick = () => {
        if (predicate()) return resolve();
        if (Date.now() - started > timeoutMs) return reject(new Error('waitFor timeout: ' + label));
        setTimeout(tick, 25);
      };
      tick();
    });
    const findPlayBtn = () => [...document.querySelectorAll('button')].find((b) => b.textContent.includes('播放'));

    // ---------- 波 I：列表挂载 + 元信息面板 ----------
    // listReplays 是异步 mock：等真条目出现（占位「没有录像文件」也是 .gfw-list-item）
    await waitFor(() => [...document.querySelectorAll('#replay-list .gfw-list-item')]
      .some((el) => el.textContent === 'flicker_repro.yrp'), 'theater list mounts');
    checks.listRenders = document.querySelector('#replay-list .gfw-list-item').textContent === 'flicker_repro.yrp';

    // 未选中时信息面板为占位、按钮禁用
    checks.infoPlaceholder = document.getElementById('replay-info').textContent.includes('未选择');
    checks.buttonsDisabledBeforeSelect = document.getElementById('replay-export-btn').disabled
      && document.getElementById('replay-load-btn').disabled;

    // 选中 → 信息面板（原版 stReplayInfo：日期 + ===VS=== 布局）
    clickListItem('flicker_repro.yrp');
    await waitFor(() => document.getElementById('replay-info').textContent.includes('===VS==='), 'info panel filled');
    checks.infoPanelContent = document.getElementById('replay-info').textContent.includes('2026/09/14 12:00:00')
      && document.getElementById('replay-info').textContent.includes('HostPlayer')
      && document.getElementById('replay-info').textContent.includes('Challenger');
    checks.listItemSelected = document.querySelector('#replay-list .gfw-list-item').className.includes('gfw-selected');
    checks.buttonsEnabledAfterSelect = !document.getElementById('replay-export-btn').disabled;

    // 起始回合输入框存在，选中动作把它重置回 1（原版 ebRepStartTurn→setText(L"1")）
    const turnInput = document.getElementById('replay-start-turn');
    checks.startTurnInputExists = !!turnInput;
    setInputVal(turnInput, '5');
    clickListItem('flicker_repro.yrp');
    await waitFor(() => turnInput.value === '1', 'start turn reset on select');
    checks.startTurnResetOnSelect = true;

    // ---------- 波 I：提取卡组 ----------
    document.getElementById('replay-export-btn').click();
    await waitFor(() => exportCalls.length === 1, 'export called');
    checks.exportWired = exportCalls[0] === 'flicker_repro.yrp';
    await waitFor(() => document.getElementById('replay-toast'), 'toast shown');
    checks.exportToast = document.getElementById('replay-toast').textContent === '保存成功';

    // ---------- 载入（起始于回合 2：第 1 回合事件瞬时应用） ----------
    setInputVal(turnInput, '2');
    const loadBtn = document.getElementById('replay-load-btn');
    await waitFor(() => loadBtn && !loadBtn.disabled, 'load enabled');
    loadBtn.click();
    await waitFor(() => { const b = findPlayBtn(); return b && !b.disabled; }, 'play button enabled', 15000);

    // skip 语义：回合 2 从第 10 个事件（第 2 个 new_turn）开始 → 进度 10/11
    checks.startTurnSeeked = /10\s*\/\s*11\s*步/.test(document.body.textContent);
    // 第 1 回合已应用：青眼已进墓
    const stSeek = duelStore.getState();
    checks.startTurnBoardApplied = stSeek.board[0].grave.length + stSeek.board[1].grave.length === 1;

    // 记录基线：加载完成的重建次数（reset effect 触发一次 → 各 1 次）
    const baseline = { clear: window.__theaterProbe.clearBoardCalls, reset: window.__theaterProbe.resetCalls };
    checks.baselineRecorded = Number.isFinite(baseline.clear);

    // 播放到终态（speed 1 → 11 步约 11s），别掐 6 秒——后面断言的是终态
    const playBtn = findPlayBtn();
    checks.playButtonFound = !!playBtn;
    await waitFor(() => playBtn && !playBtn.disabled, 'play enabled', 15000);
    playBtn.click();
    await waitFor(() => /11\s*\/\s*11\s*步/.test(document.body.textContent), 'playback finished', 20000);

    const probe = window.__theaterProbe;
    checks.noRebuildLoopDuringPlayback =
      probe.clearBoardCalls - baseline.clear <= 2 && probe.resetCalls - baseline.reset <= 2;
    window.__theaterSmoke.probe = {
      ...probe, baseline,
      // 抖动取样：1 秒内重建计数若持续变化即为闪烁实锤
    };
    const snap1 = { c: probe.clearBoardCalls, r: probe.resetCalls };
    await new Promise((r) => setTimeout(r, 1000));
    const snap2 = { c: probe.clearBoardCalls, r: probe.resetCalls };
    checks.noRebuildWhileIdleish = snap1.c === snap2.c && snap1.r === snap2.r;

    // 播放推进断言：完整跑完 11 步（注意只折叠连续空白，不能全删——regex 里带空格）
    checks.progressAdvanced = /11\s*\/\s*11\s*步/.test(document.body.textContent);

    // compact 模式：全屏 HUD 面板不得出现在剧场（否则遮死 626px 的 3D 容器，
    // 阶段条 actionable 闪光也会在回放里常亮 = 「一直闪烁」）。
    // 例外：手牌坞保留——回放里自己的手牌从这里看。
    checks.compactHudHidden = ['#preview-info', '#duel-log-list', '#phase-strip', '#hint-bar',
      '.chat-overlay'].every((sel) => !$(sel));
    checks.lpPanelsKept = !!$('.player-state-panel');
    checks.handDockKept = !!$('#hand-cards-dock');

    // compact 模式仍保留中央大字阶段横幅（原版回放的 DrawSpec showcard 文本）：
    // 手动推一个 new_phase，横幅应在 1.4s 自动消退前出现。
    eventBus.emit('duel:new_phase', { phase: 0x04 });
    await waitFor(() => !!document.querySelector('.phase-banner'), 'compact phase banner', 3000);
    checks.compactPhaseBanner = document.querySelector('.phase-banner').textContent === '主要阶段 1';
    await waitFor(() => !document.querySelector('.phase-banner'), 'banner auto-dismiss', 5000);
    checks.compactPhaseBannerExpires = !document.querySelector('.phase-banner');
    // 自己的手牌：playerType=1 → 视角玩家是 seat1，开局抽 5 → 手牌坞 5 张
    checks.ownHandCards = document.querySelectorAll('#hand-cards-dock .hand-card-item').length === 5;

    // 场上真的有卡：召唤事件（第 8 步）被播放后，store 面板应有青眼
    recordStore();

    // ---------- 波 E：回放列表改名/删除 ----------
    const renameBtn = document.getElementById('replay-rename-btn');
    checks.renameDeleteButtonsExist = !!renameBtn && !!document.getElementById('replay-delete-btn');
    renameBtn.click();
    await waitFor(() => replayMutations.renamed.length === 1, 'renamed');
    checks.renameWired = replayMutations.renamed[0][0] === 'flicker_repro.yrp'
      && replayMutations.renamed[0][1] === 'renamed_replay';
    await waitFor(() => [...document.querySelectorAll('#replay-list .gfw-list-item')]
      .some((el) => el.textContent === 'renamed_replay.yrp'), 'list refreshed');
    checks.listRefreshedAfterRename = true;

    // 删除：组件内的列表状态仍是改名后那一项，重新选中并删除
    clickListItem('renamed_replay.yrp');
    const deleteBtn = document.getElementById('replay-delete-btn');
    await waitFor(() => deleteBtn && !deleteBtn.disabled, 'delete enabled');
    deleteBtn.click();
    await waitFor(() => replayMutations.deleted.length === 1, 'deleted');
    checks.deleteWired = replayMutations.deleted[0] === 'renamed_replay.yrp';

    // ---------- 波 E：STOC_REPLAY 保存确认（弹窗路径） ----------
    eventBus.emit('stoc:replay', { name: '2026-09-11 12-30-00', size: 1024 });
    await waitFor(() => document.getElementById('replay-save-prompt'), 'prompt shown');
    const nameInput = document.getElementById('replay-save-name');
    checks.promptPrefilled = nameInput.value === '2026-09-11 12-30-00';
    const inpSetter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
    inpSetter.call(nameInput, 'my_replay');
    nameInput.dispatchEvent(new Event('input', { bubbles: true }));
    document.getElementById('replay-save-confirm').click();
    await waitFor(() => replayMutations.saved.length === 1, 'saved');
    checks.saveWired = replayMutations.saved[0] === 'my_replay';
    await waitFor(() => !document.getElementById('replay-save-prompt'), 'prompt closed');
    checks.promptClosesAfterSave = true;

    // 取消路径不落盘
    eventBus.emit('stoc:replay', { name: 'discard_me', size: 1 });
    await waitFor(() => document.getElementById('replay-save-prompt'), 'prompt again');
    document.getElementById('replay-save-cancel').click();
    await waitFor(() => !document.getElementById('replay-save-prompt'), 'cancelled');
    checks.cancelDoesNotSave = replayMutations.saved.length === 1;

    // auto_save_replay=1 → 不弹窗直接按建议名落盘
    settingsStore.set('auto_save_replay', 1);
    await new Promise((r) => setTimeout(r, 50)); // 等 useSyncExternalStore 重渲染、ref 更新
    eventBus.emit('stoc:replay', { name: 'auto_one', size: 2 });
    await waitFor(() => replayMutations.saved.length === 2, 'auto saved');
    checks.autoSavePath = replayMutations.saved[1] === 'auto_one'
      && !document.getElementById('replay-save-prompt');

    // ---- btnReplaySwap（game.cpp:901 → ReplayMode::SwapField）：交换视角
    // —— store 显示座翻转 + 相机绕到另一侧，再换一次复原 ----
    const swapBtn = document.getElementById('replay-swap-btn');
    record('replay-swap-btn-shown', !!swapBtn && !swapBtn.disabled);
    const lpBeforeSwap = [duelStore.getState().lp[0], duelStore.getState().lp[1]];
    swapBtn.click();
    await waitFor(() => duelStore.getState().viewSwapped === true, 'view swapped');
    record('replay-swap-flips-store', duelStore.getState().lp[0] === lpBeforeSwap[1]
      && duelStore.getState().lp[1] === lpBeforeSwap[0], JSON.stringify(duelStore.getState().lp));
    const fSwap = window.__theaterProbe.field;
    await waitFor(() => fSwap.camera.position.z < 0, 'camera flips');
    record('replay-swap-flips-camera', true);
    swapBtn.click();
    await waitFor(() => duelStore.getState().viewSwapped === false
      && fSwap.camera.position.z > 0, 'view restored');
    record('replay-swap-back', duelStore.getState().lp[0] === lpBeforeSwap[0]);

    record('no-fatal', true);
  } catch (err) {
    checks.fatalMsg = String(err && err.stack || err);
    checks['fatal'] = false;
  } finally {
    window.__theaterSmoke.ready = true;
  }
})();

function record(name, ok, detail) { checks[name] = ok; if (detail !== undefined) checks[name + '-detail'] = detail; }
function recordStore() {
  // 终态（11/11 步）：青眼召唤后又送墓 → mzone 空、grave 1 张。
  // 注意 playerType=1 时 store 做了座位映射，事件落在 board[1]，两个座位合计断言。
  const st = duelStore.getState();
  const graveTotal = st.board[0].grave.length + st.board[1].grave.length;
  const m2Any = !!st.board[0].mzone[2] || !!st.board[1].mzone[2];
  record('store-grave-has-card', graveTotal === 1 && !m2Any,
    JSON.stringify({ graveTotal, m20: st.board[0].mzone[2] || null, m21: st.board[1].mzone[2] || null }));
  record('store-lp', st.lp[0] === 4000 || st.lp[0] === 8000);
  // 3D mesh 层面：卡真的渲染过并被收进墓地堆
  const f = window.__theaterProbe.field;
  record('mesh-grave-has-card', !!f && Array.isArray(f.cardsOnField[0].grave) && f.cardsOnField[0].grave.length === 1,
    JSON.stringify(f && f.cardsOnField && f.cardsOnField[0] && {
      m2: !!f.cardsOnField[0].mzone[2],
      grave: f.cardsOnField[0].grave && f.cardsOnField[0].grave.length,
    }));
  // 对手手背行：seat0 开局 5 张，召唤青眼离手 -1 → 终态 4 张背面
  record('opp-hand-backs', !!f && Array.isArray(f.opponentHandMeshes) && f.opponentHandMeshes.length === 4,
    f ? f.opponentHandMeshes.length : 'no-field');
}
