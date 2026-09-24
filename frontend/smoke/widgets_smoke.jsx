// Headless smoke for the P2 React widgets (CardPreviewPanel + PlayerPanel)
// driven purely through the duelStore event path. No Three.js / HUD shim —
// this is the "new path" the store-based UI consumes. The store itself is
// wired to the real event bus via duel/store.js, so dispatching through
// eventBus exercises the exact production wiring.
import { createRoot } from 'react-dom/client';
import React from 'react';
import { eventBus, WailsBridge } from '../src/wails_bridge.ts';
import '../css/duel-original.css';
import '../src/duel/store.ts'; // creates + wires duelStore as a side effect
import CardPreviewPanel from '../src/components/CardPreviewPanel.tsx';
import PlayerPanel from '../src/components/PlayerPanel.tsx';
import PhaseStrip from '../src/components/PhaseStrip.tsx';
import RightControls from '../src/components/RightControls.tsx';
import ChatOverlay from '../src/components/ChatOverlay.tsx';
import SideDecking from '../src/components/SideDecking.tsx';
import VictoryOverlay from '../src/components/VictoryOverlay.tsx';
import CardListOverlay from '../src/components/CardListOverlay.tsx';
import { installHotkeys } from '../src/duel/hotkeys.ts';
import { settingsStore } from '../src/domain/settings.ts';
import chainPrefs from '../src/duel/chain_prefs.ts';
import { soundManager } from '../src/audio/sound_manager.ts';
import { duelStore } from '../src/duel/store.ts';

soundManager.muted = true; // VictoryOverlay plays sounds on duel:win

const root = createRoot(document.getElementById('root'));
root.render(
  <React.Fragment>
    <CardPreviewPanel />
    <PlayerPanel side="opponent" />
    <PlayerPanel side="player" />
    <PhaseStrip />
    {/* P4: right controls / chat overlay / side-decking (self-hiding) */}
    <RightControls />
    <ChatOverlay />
    <SideDecking />
    {/* P5: victory modal on duel:win (replay theater would pass showVictory=false) */}
    <VictoryOverlay showVictory={true} />
    {/* 波 F: F1-F8 卡片列表浮窗（A/S/D 热键全局监听在下方 installHotkeys） */}
    <CardListOverlay />
  </React.Fragment>
);

// 波 F：全局快捷键（决斗屏挂载；冒烟里手动装/卸）
const uninstallHotkeys = installHotkeys();

// Record semantic phase responses from PhaseStrip (byte encodings are locked
// down by Go-side responses_test.go).
const phaseResponses = [];
WailsBridge.respondIdleCmd = (idx, cmdType) => phaseResponses.push({ kind: 'idle', idx, cmdType });
WailsBridge.respondBattleCmd = (idx, cmdType) => phaseResponses.push({ kind: 'battle', idx, cmdType });

// P4: record side-deck handshake sends (CTOS_UPDATE_DECK payload).
const deckSends = [];
WailsBridge.updateDeck = (mainCards, sideCards) => deckSends.push({ main: mainCards, side: sideCards });

// P3: VictoryOverlay 的 match kill 卡图走 getCardImage，mock 成 data URL
// （避免冒烟环境发 CDN 请求）
const FAKE_PIC = 'data:image/gif;base64,R0lGODlhAQABAIAAAAUEBAAAACwAAAAAAQABAAACAkQBADs=';
WailsBridge.getCardImage = async () => ({ url: FAKE_PIC, full: false });

const waitFor = (predicate, timeoutMs = 5000) => new Promise((resolve, reject) => {
  const started = Date.now();
  const tick = () => {
    if (predicate()) return resolve();
    if (Date.now() - started > timeoutMs) return reject(new Error('waitFor timeout'));
    setTimeout(tick, 25);
  };
  tick();
});

const $ = (id) => document.getElementById(id);

// Radix Select（GfwSelect）驱动：trigger 靠 pointerdown 打开；
// 这里只需要数 option（确认换副卡组下拉已填充）
const openSelect = async (id) => {
  const t = $(id);
  t.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, cancelable: true, button: 0, pointerId: 1, pointerType: 'mouse' }));
  t.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true, button: 0 }));
  t.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, cancelable: true, button: 0 }));
  t.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, button: 0 }));
  await waitFor(() => !!document.querySelector('[role="option"]'));
};
const optionCount = async (id) => {
  await openSelect(id);
  const n = document.querySelectorAll('[role="option"]').length;
  document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }));
  await new Promise((r) => setTimeout(r, 100));
  return n;
};

window.__widgetsSmoke = { checks: {}, ready: false };

const run = async () => {
  const checks = {};
  const assert = (name, cond, detail = '') => {
    checks[name] = cond ? true : `FAIL: ${detail}`;
  };

  // ---- duel:start seeds LP and pile counts (seat 1 is the local player here:
  // playerType=1 covers the perspective mapping) ----
  eventBus.emit('duel:start', {
    playerType: 1, duelRule: 5, lp0: 8000, lp1: 4000,
    deck0: 40, extra0: 15, deck1: 37, extra1: 12,
  });
  await waitFor(() => $('player-panel'));
  assert('playerLPShown', $('player-panel-lp').innerText === '4000',
    `got ${$('player-panel-lp').innerText}`);
  assert('opponentLPShown', $('opponent-panel-lp').innerText === '8000',
    `got ${$('opponent-panel-lp').innerText}`);
  assert('pilesSeeded', $('player-pile-deck').innerText === '37'
    && $('player-pile-extra').innerText === '12'
    && $('opponent-pile-deck').innerText === '40',
    `deck=${$('player-pile-deck').innerText} extra=${$('player-pile-extra').innerText} opp=${$('opponent-pile-deck').innerText}`);
  assert('namesPlaceholder', $('player-panel-name').innerText.length > 0);

  // ---- stoc:player_enter replaces the placeholder name (engine seat 0 →
  // opponent display slot because playerSlot === 1) ----
  eventBus.emit('stoc:player_enter', { pos: 0, name: 'HostPlayer' });
  await waitFor(() => $('opponent-panel-name').innerText === 'HostPlayer');
  assert('playerEnterName', $('opponent-panel-name').innerText === 'HostPlayer',
    `got ${$('opponent-panel-name').innerText}`);

  // ---- LP deltas and absolute updates, in one sequence (order-sensitive) ----
  eventBus.emit('duel:damage', { player: 1, amount: 1500 });
  await waitFor(() => $('player-panel-lp').innerText === '2500');
  eventBus.emit('duel:recover', { player: 1, amount: 300 });
  await waitFor(() => $('player-panel-lp').innerText === '2800');
  eventBus.emit('duel:pay_lpcost', { player: 1, amount: 800 });
  await waitFor(() => $('player-panel-lp').innerText === '2000');
  eventBus.emit('duel:lp_update', { player: 1, lp: 7000 });
  await waitFor(() => $('player-panel-lp').innerText === '7000');
  assert('lpSequence', $('player-panel-lp').innerText === '7000',
    `got ${$('player-panel-lp').innerText}`);

  // ---- draw decrements deck; move deck→grave moves counts between piles ----
  eventBus.emit('duel:draw', { player: 1, count: 1, cards: [89631139] });
  await waitFor(() => $('player-pile-deck').innerText === '36');
  eventBus.emit('duel:move', {
    code: 89631139, pc: 1, pl: 0x02, ps: 5, pp: 0x1,
    cc: 1, cl: 0x10, cs: 0, cp: 0x1, reason: 0x41,
  });
  await waitFor(() => $('player-pile-grave').innerText === '1');
  assert('movePiles', $('player-pile-deck').innerText === '36'
    && $('player-pile-grave').innerText === '1',
    `deck=${$('player-pile-deck').innerText} grave=${$('player-pile-grave').innerText}`);

  // ---- P5 board tracking: the same move lands in store.board (display seat
  // 0 = the local player here, since playerType=1) ----
  const boardNow = () => duelStore.getState().board;
  assert('boardGraveTracked', boardNow()[0].grave.length === 1
    && boardNow()[0].grave[0].code === 89631139,
    `grave=${JSON.stringify(boardNow()[0].grave)}`);
  // summon-style move hand→mzone places the card in the board snapshot…
  eventBus.emit('duel:move', {
    code: 46986414, pc: 1, pl: 0x02, ps: 0, pp: 0x1,
    cc: 1, cl: 0x04, cs: 2, cp: 0x1, reason: 0,
  });
  await waitFor(() => boardNow()[0].mzone[2] && boardNow()[0].mzone[2].code === 46986414);
  assert('boardMzoneTracked', boardNow()[0].mzone[2].code === 46986414
    && boardNow()[0].mzone[2].pos === 0x1,
    `mzone[2]=${JSON.stringify(boardNow()[0].mzone[2])}`);
  // …and pos_change flips its recorded position
  eventBus.emit('duel:pos_change', { cc: 1, cl: 0x04, cs: 2, cp: 0x8 });
  await waitFor(() => boardNow()[0].mzone[2].pos === 0x8);
  assert('boardRepositionTracked', boardNow()[0].mzone[2].pos === 0x8,
    `pos=${boardNow()[0].mzone[2].pos}`);

  // ---- hand tracking: draw added to store hand, move to grave removed it ----
  assert('handTracked', duelStore.getState().hand.length === 0,
    `hand=${JSON.stringify(duelStore.getState().hand)}`);
  duelStore.inspect(89631139);

  // ---- card preview resolves name/desc via the (mock) bridge ----
  await waitFor(() => $('preview-card-name').innerText === 'Blue-Eyes White Dragon');
  assert('previewInfo', $('preview-card-name').innerText === 'Blue-Eyes White Dragon',
    `got ${$('preview-card-name').innerText}`);
  assert('previewDesc', $('preview-card-desc').innerText.includes('legendary dragon'),
    `desc=${$('preview-card-desc').innerText.slice(0, 40)}`);

  // ---- turn/phase ----
  eventBus.emit('duel:new_turn', { player: 1 });
  eventBus.emit('duel:new_phase', { phase: 0x04 });
  await waitFor(() => $('player-panel').className.includes('is-turn'));
  assert('turnHighlight', $('player-panel').className.includes('is-turn'));

  // ---- timer ----
  eventBus.emit('stoc:time_limit', { player: 1, leftTime: 125 });
  await waitFor(() => $('player-panel').innerText.includes('02:05'));
  assert('timerShown', $('player-panel').innerText.includes('02:05'));

  // ---- PhaseStrip: idlecmd prompt enables BP/EP with the right responses ----
  const bpBtn = () => $('phase-btn-bp');
  const epBtn = () => $('phase-btn-ep');
  const m2Btn = () => $('phase-btn-m2');
  eventBus.emit('duel:select_idlecmd', {
    player: 1, summon: [], spsummon: [], repos: [], mset: [], sset: [],
    activate: [], toBP: 1, toEP: 1, shuffle: 0,
  });
  await waitFor(() => !bpBtn().disabled);
  assert('bpEnabled', !bpBtn().disabled && !epBtn().disabled && m2Btn().disabled);
  bpBtn().click();
  await waitFor(() => phaseResponses.some((r) => r.kind === 'idle' && r.idx === 0 && r.cmdType === 6));
  assert('stripBP', true, JSON.stringify(phaseResponses));

  epBtn().click();
  await waitFor(() => phaseResponses.some((r) => r.kind === 'idle' && r.idx === 0 && r.cmdType === 7));
  assert('stripEP', true, JSON.stringify(phaseResponses));

  // ---- PhaseStrip: battlecmd prompt enables M2/EP, EP answers battle 3 ----
  eventBus.emit('duel:select_battlecmd', {
    player: 1, activate: [], attack: [], toM2: 1, toEP: 1,
  });
  await waitFor(() => !m2Btn().disabled);
  assert('m2Enabled', !m2Btn().disabled && bpBtn().disabled);
  m2Btn().click();
  await waitFor(() => phaseResponses.some((r) => r.kind === 'battle' && r.idx === 0 && r.cmdType === 2));
  epBtn().click();
  await waitFor(() => phaseResponses.some((r) => r.kind === 'battle' && r.idx === 0 && r.cmdType === 3));
  assert('stripM2AndEP', true, JSON.stringify(phaseResponses));

  // ---- phase banner pops on new_phase and again on win ----
  eventBus.emit('duel:new_phase', { phase: 0x04 });
  await waitFor(() => document.querySelector('.phase-banner'));
  assert('phaseBanner', document.querySelector('.phase-banner').innerText === '主要阶段 1',
    `got ${document.querySelector('.phase-banner').innerText}`);
  eventBus.emit('duel:win', { winner: 1, type: 1 });
  await waitFor(() => document.querySelector('.phase-banner').innerText === '胜利！');
  assert('winBanner', true);

  // ---- P5 VictoryOverlay: duel:win opens the victory modal (winner 1 is the
  // local seat here → 胜利/🏆), and its exit button emits nav 'menu'. ----
  // P3 补完：MSG_MATCH_KILL 击杀卡图 + 胜负原因（strings.conf !victory 子集）
  eventBus.emit('duel:match_kill', { code: 89631139 });
  await waitFor(() => duelStore.getState().matchKill === 89631139);
  eventBus.emit('duel:win', { winner: 1, type: 1 });
  await waitFor(() => $('victory-overlay'));
  assert('victoryOverlayShown', $('victory-overlay').className.includes('active')
    && $('victory-overlay').innerText.includes('胜利')
    && $('victory-overlay').innerText.includes('🏆'),
    $('victory-overlay').innerText.slice(0, 40));
  assert('victoryReasonShown', $('victory-overlay').innerText.includes('原因：基本分变成0'),
    $('victory-overlay').innerText.slice(0, 60));
  await waitFor(() => !!document.querySelector('#match-kill-card img'));
  const killImg = document.querySelector('#match-kill-card img');
  assert('matchKillCardShown', killImg.src === FAKE_PIC, killImg.src.slice(0, 40));
  const navEvents = [];
  eventBus.on('nav', (dest) => navEvents.push(dest));
  $('modal-duel-exit').click();
  await waitFor(() => navEvents.includes('menu'));
  assert('victoryExitNavigates', navEvents.includes('menu'));

  // ---- P4 RightControls: chain mode buttons are single-select toggles
  // (same key again cancels — gframe isPushButton semantics, no packets). ----
  const chainIgnore = $('chain-btn-ignore');
  assert('chainButtonsExist', !!chainIgnore && !!$('chain-btn-always') && !!$('chain-btn-whenAvail'));
  chainIgnore.click();
  await waitFor(() => chainIgnore.className.includes('pressed'));
  assert('chainPressed', chainIgnore.className.includes('pressed'));
  chainIgnore.click();
  await waitFor(() => !chainIgnore.className.includes('pressed'));
  assert('chainToggleOff', !chainIgnore.className.includes('pressed'));

  // STOC_TEAMMATE_SURRENDER：组队赛队友请求投降 → 按钮文案换「投降(1/2)」
  eventBus.emit('stoc:teammate_surrender', {});
  await waitFor(() => $('surrender-btn').innerText.includes('投降(1/2)'));
  assert('teammateSurrenderLabel', $('surrender-btn').innerText.includes('投降(1/2)'));

  // ---- P4 ChatOverlay: chat lines colored by seat (p0 blue-ish host class,
  // local player labeled 你 since playerSlot === 1 in this fixture). ----
  eventBus.emit('stoc:chat', { player: 0, msg: '决斗吧！' });
  await waitFor(() => document.querySelector('.chat-line.chat-p0'));
  assert('chatHostLine', !!document.querySelector('.chat-line.chat-p0')
    && document.querySelector('.chat-line.chat-p0').innerText.includes('决斗吧！'));
  eventBus.emit('stoc:chat', { player: 1, msg: '轮到我了' });
  await waitFor(() => document.querySelector('.chat-line.chat-p1'));
  assert('chatLocalLine', document.querySelector('.chat-line.chat-p1').innerText.includes('你'));

  // ---- P4 SideDecking: stoc:change_side shows the overlay; confirm sends
  // CTOS_UPDATE_DECK with main+extra merged (server splits by card type);
  // stoc:duel_start hides it again. ----
  assert('sideDeckingHiddenInitially', !$('side-decking'));
  eventBus.emit('stoc:change_side', {});
  await waitFor(() => $('side-deck-select'));
  assert('sideDeckingShown', !!$('side-deck-select')
    && (await optionCount('side-deck-select')) > 0);
  $('side-deck-confirm').click();
  await waitFor(() => deckSends.length === 1);
  assert('sideDeckUpdateMerged', deckSends[0].main.length === 20 && deckSends[0].side.length === 2,
    `main=${deckSends[0].main.length} side=${deckSends[0].side.length}`);
  eventBus.emit('stoc:duel_start', {});
  await waitFor(() => !$('side-decking'));
  assert('sideDeckingHiddenAfterStart', !$('side-decking'));

  // ---- 波 F: A/S/D 按住连锁键（control_mode==0，按住生效、松开清空；
  // 不发包只改本地 chain_prefs，与 gframe event_handler.cpp:1734 一致） ----
  const keydown = (k, init = {}) => window.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true, ...init }));
  const keyup = (k, init = {}) => window.dispatchEvent(new KeyboardEvent('keyup', { key: k, bubbles: true, ...init }));
  keydown('A');
  await waitFor(() => chainPrefs.get() === 'ignore');
  assert('hotkeyAHold', chainPrefs.get() === 'ignore'
    && $('chain-btn-ignore').className.includes('pressed'),
    `mode=${chainPrefs.get()}`);
  keyup('A');
  await waitFor(() => chainPrefs.get() === null);
  assert('hotkeyARelease', chainPrefs.get() === null);

  keydown('d');
  await waitFor(() => chainPrefs.get() === 'whenAvail');
  assert('hotkeyDHold', chainPrefs.get() === 'whenAvail');
  keyup('d');
  await waitFor(() => chainPrefs.get() === null);

  // control_mode!=0 时三键不生效（原版仅键鼠模式启用）
  settingsStore.set('control_mode', 1);
  keydown('s');
  await new Promise((r) => setTimeout(r, 30));
  assert('hotkeySRequiresKeyMouse', chainPrefs.get() === null, `mode=${chainPrefs.get()}`);
  keyup('s');
  settingsStore.set('control_mode', 0);

  // 输入框焦点时全部跳过（聊天输入不被劫持）
  const input = document.createElement('input');
  document.body.appendChild(input);
  input.focus();
  keydown('a');
  await new Promise((r) => setTimeout(r, 30));
  assert('hotkeySkipsTextField', chainPrefs.get() === null, `mode=${chainPrefs.get()}`);
  keyup('a');
  input.blur();
  input.remove();

  // ---- 波 F: F1-F8 卡片列表浮窗 ----
  // 当前 board：本方墓地已有 1 张（89631139，上文 movePiles 步骤）
  keyup('F1');
  await waitFor(() => $('card-display-title') && $('card-display-title').innerText === '墓地(1)');
  assert('f1GraveList', $('card-display-title').innerText === '墓地(1)'
    && document.querySelectorAll('.card-display-item').length === 1,
    `title=${$('card-display-title') && $('card-display-title').innerText}`);

  // 额外卡组：怪兽从 mzone 回到 extra（LOC_EXTRA=0x40 进堆区）
  eventBus.emit('duel:move', {
    code: 46986414, pc: 1, pl: 0x04, ps: 2, pp: 0x1,
    cc: 1, cl: 0x40, cs: 0, cp: 0x1, reason: 0,
  });
  await waitFor(() => $('player-pile-extra').innerText === '13'); // 12+1
  keyup('F2');
  await waitFor(() => $('card-display-title') && $('card-display-title').innerText === '除外(0)');
  assert('f2BanishSwitch', $('card-display-title').innerText === '除外(0)');
  keyup('F3');
  await waitFor(() => $('card-display-title').innerText === '额外(1)');
  assert('f3ExtraList', $('card-display-title').innerText === '额外(1)');

  // 超量素材：extra 卡吸附到 mzone[2]（LOC_OVERLAY=0x80，cs=槽位）
  eventBus.emit('duel:move', {
    code: 46986414, pc: 1, pl: 0x40, ps: 0, pp: 0x1,
    cc: 1, cl: 0x80, cs: 2, cp: 0x1, reason: 0,
  });
  await waitFor(() => duelStore.getState().board[0].overlay[2].length === 1);
  assert('overlayMaterialTracked', duelStore.getState().board[0].overlay[2][0] === 46986414);
  keyup('F4');
  await waitFor(() => $('card-display-title').innerText === '叠放(1)');
  assert('f4OverlayList', $('card-display-title').innerText === '叠放(1)');
  assert('extraDrainedByOverlayMove', $('player-pile-extra').innerText === '12');

  // 对方视角：F5 = 对方墓地（空 → 标题带「对方」+（空）占位）
  keyup('F5');
  await waitFor(() => $('card-display-title').innerText === '对方墓地(0)');
  assert('f5OpponentPile', $('card-display-title').innerText === '对方墓地(0)'
    && !!document.querySelector('.card-display-empty'));

  // 再按同一键关闭（浮层 toggle）
  keyup('F5');
  await waitFor(() => !$('card-display-overlay'));
  assert('sameKeyCloses', !$('card-display-overlay'));

  // ESC 关闭
  keyup('F1');
  await waitFor(() => !!$('card-display-overlay'));
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  await waitFor(() => !$('card-display-overlay'));
  assert('escClosesOverlay', !$('card-display-overlay'));

  // 决斗未开始（reset 后）F 键不弹窗
  duelStore.reset();
  keyup('F1');
  await new Promise((r) => setTimeout(r, 30));
  assert('hotkeysRequireStarted', !$('card-display-overlay'));
  uninstallHotkeys();

  window.__widgetsSmoke.checks = checks;
  window.__widgetsSmoke.ready = true;
};

run().catch((err) => {
  window.__widgetsSmoke.checks = { fatal: String((err && err.stack) || err) };
  window.__widgetsSmoke.ready = true;
});
