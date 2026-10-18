// Headless smoke for the P2 React widgets (CardPreviewPanel + PlayerPanel)
// driven purely through the duelStore event path. No Three.js / HUD shim —
// this is the "new path" the store-based UI consumes. The store itself is
// wired to the real event bus via duel/store.js, so dispatching through
// eventBus exercises the exact production wiring.
import UiRoot from '../src/ui/UiRoot.tsx';
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
import ReplaySavePrompt from '../src/components/ReplaySavePrompt.tsx';
import { registerCardName } from '../src/domain/card_names.ts';
import CardListOverlay from '../src/components/CardListOverlay.tsx';
import { installHotkeys } from '../src/duel/hotkeys.ts';
import { settingsStore } from '../src/domain/settings.ts';
import chainPrefs from '../src/duel/chain_prefs.ts';
import { soundManager } from '../src/audio/sound_manager.ts';
import { duelStore } from '../src/duel/store.ts';

soundManager.muted = true; // VictoryOverlay plays sounds on duel:win

const root = createRoot(document.getElementById('root'));
root.render(
  <UiRoot>
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
    <ReplaySavePrompt />
    {/* 波 F: F1-F8 卡片列表浮窗（A/S/D 热键全局监听在下方 installHotkeys） */}
    <CardListOverlay />
  </React.Fragment>
  </UiRoot>
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
    if (Date.now() - started > timeoutMs) return reject(new Error('waitFor timeout: '+String(predicate)));
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
  await waitFor(() => $('preview-card-name').innerText === 'Blue-Eyes White Dragon[89631139]');
  assert('previewInfo', $('preview-card-name').innerText === 'Blue-Eyes White Dragon[89631139]',
    `got ${$('preview-card-name').innerText}`);
  assert('previewDesc', $('preview-card-desc').innerText.includes('legendary dragon'),
    `desc=${$('preview-card-desc').innerText.slice(0, 40)}`);

  // ---- 互连箭头：LINK 怪兽预览显示 LINK-N + 箭头（替代无意义的 DEF 0）。
  // 对齐 gframe game.cpp:1684-1690（adBuffer = "atk/-   <墓碑 FormatLinkMarker>"
  // ）与 data_manager.cpp:410-430 FormatLinkMarker 顺序。Decode Talker
  // def=133=0x85 → [↑][↙][↘]（CDB 里 def 存 link_marker 位掩码）。 ----
  const origGetCard = WailsBridge.getCard;
  const LINK_MONSTER = {
    code: 1861629, name: 'Decode Talker',
    type: 0x4000000 | 0x1 | 0x20, // TYPE_LINK | TYPE_MONSTER | TYPE_EFFECT
    attack: 2300, defense: 0, level: 3,
    race: 0x1000000, attribute: 0x10, linkMarker: 133,
    desc: '2+ Effect Monsters',
  };
  const PENDULUM_MONSTER = {
    code: 10000001, name: 'Pendulum Test',
    type: 0x1000000 | 0x1 | 0x20,
    attack: 1800, defense: 1200, level: 4, lscale: 1, rscale: 1,
    race: 0x2000, attribute: 0x10,
    desc: '←1 【灵摆】 1→\n【灵摆效果】灵摆区效果文本\n【怪兽效果】怪兽区效果文本',
  };
  WailsBridge.getCard = async (code) => {
    if (code === 1861629) return LINK_MONSTER;
    if (code === 10000001) return PENDULUM_MONSTER;
    return origGetCard(code);
  };
  duelStore.inspect(1861629);
  await waitFor(() => $('preview-card-name').innerText === 'Decode Talker[01861629]');
  const statsEl = document.querySelector('.preview-stats');
  // textContent 保留多空格（innerText 会折叠），核对与格式串的空格一致
  assert('previewLinkArrows', statsEl && statsEl.textContent === '[LINK-3] 2300/-   [↑][↙][↘]',
    `got ${statsEl && statsEl.textContent}`);
  duelStore.inspect(10000001);
  await waitFor(() => $('preview-card-name').innerText === 'Pendulum Test[10000001]');
  const effectHeadings = [...document.querySelectorAll('#preview-card-desc .preview-effect-heading')].map((el) => el.innerText);
  assert('previewPendulumSections', effectHeadings.join('|') === '灵摆效果|怪兽效果'
    && $('preview-card-desc').textContent.includes('灵摆区效果文本')
    && $('preview-card-desc').textContent.includes('怪兽区效果文本')
    && !$('preview-card-desc').textContent.includes('【怪兽效果】')
    && !$('preview-card-desc').textContent.includes('←1'), $('preview-card-desc').textContent);
  WailsBridge.getCard = origGetCard;

  // ---- turn/phase ----
  eventBus.emit('duel:new_turn', { player: 1 });
  eventBus.emit('duel:new_phase', { phase: 0x04 });
  await waitFor(() => $('player-panel').className.includes('is-turn'));
  assert('turnHighlight', $('player-panel').className.includes('is-turn'));
  assert('playerNameTitle', $('player-panel-name').title === $('player-panel-name').textContent);
  assert('lpMeterAria', document.querySelector('#player-panel .lp-frame').getAttribute('role') === 'meter'
    && document.querySelector('#player-panel .lp-frame').getAttribute('aria-valuenow') === String(duelStore.getState().lp[0]));

  // ---- timer ----
  eventBus.emit('stoc:time_limit', { player: 1, leftTime: 125 });
  await waitFor(() => $('player-panel').innerText.includes('02:05'));
  assert('timerShown', $('player-panel').innerText.includes('02:05'));
  assert('timerAria', $('player-time-bar').getAttribute('role') === 'progressbar'
    && $('player-time-bar').getAttribute('aria-valuenow') === '125'
    && $('player-time-bar').getAttribute('aria-valuetext') === '02:05');
  eventBus.emit('stoc:time_limit', { player: 1, leftTime: 30 });
  await waitFor(() => $('player-time-bar').dataset.lowTime === 'true');
  assert('timerLowTimeState', $('player-time-bar').classList.contains('is-low')
    && $('player-time-bar').getAttribute('aria-valuenow') === '30');

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
  assert('phaseBanner', document.querySelector('.phase-banner').innerText === 'Main Phase 1',
    `got ${document.querySelector('.phase-banner').innerText}`);
  eventBus.emit('duel:win', { winner: 1, type: 1 });
  await waitFor(() => document.querySelector('.phase-banner')?.innerText === 'You Win!');
  assert('winBanner', true);

  // Original DrawSpec: transient title and losing player's reason, no modal.
  await waitFor(() => $('victory-reason'));
  assert('victoryReasonShown', $('victory-reason').innerText === '[HostPlayer] 基本分变成0');
  assert('victoryHasNoModal', !$('victory-overlay') && !$('modal-duel-exit'));
  registerCardName(89631139, '青眼白龙');
  eventBus.emit('duel:match_kill', { code: 89631139 });
  await waitFor(() => duelStore.getState().matchKill === 89631139);
  eventBus.emit('duel:win', { winner: 1, type: 1 });
  await waitFor(() => $('victory-reason')?.innerText === '由于「青眼白龙」获得比赛胜利');
  assert('matchKillTextShown', !$('match-kill-card'));
  eventBus.emit('single:replay', { name: 'parity-replay' });
  await new Promise(r => setTimeout(r, 700));
  assert('winBannerLastsBeyondPhase', document.querySelector('.phase-banner')?.innerText === 'You Win!');
  assert('replayWaitsForWinBanner', !$('replay-save-prompt'));
  await waitFor(() => $('replay-save-prompt'));
  const replayRect = $('replay-save-prompt').getBoundingClientRect();
  assert('replaySaveUsesSourceWindowSize', replayRect.width === 310 && replayRect.height === 120);
  assert('winBannerExpiresBeforeReplay', !document.querySelector('.phase-banner') && !$('victory-reason'));
  $('replay-save-cancel').click();
  eventBus.emit('duel:win', { winner: 2, type: 1 });
  await waitFor(() => document.querySelector('.phase-banner')?.innerText === 'Draw Game');
  assert('drawResultPreserved', duelStore.getState().win.winner === 2 && !$('victory-reason'));
  duelStore.toggleViewSwap();
  await waitFor(() => duelStore.getState().viewSwapped);
  assert('drawSurvivesViewSwap', duelStore.getState().win.winner === 2);
  duelStore.toggleViewSwap();

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
  // 不发包只改本地 chain_prefs。原版 event_handler.cpp:1740-1765：
  // A=always_chain / S=ignore_chain / D=chain_when_avail） ----
  const keydown = (k, init = {}) => window.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true, ...init }));
  const keyup = (k, init = {}) => window.dispatchEvent(new KeyboardEvent('keyup', { key: k, bubbles: true, ...init }));
  keydown('A');
  await waitFor(() => chainPrefs.get() === 'always');
  assert('hotkeyAHold', chainPrefs.get() === 'always'
    && $('chain-btn-always').className.includes('pressed'),
    `mode=${chainPrefs.get()}`);
  keyup('A');
  await waitFor(() => chainPrefs.get() === null);
  assert('hotkeyARelease', chainPrefs.get() === null);

  keydown('s');
  await waitFor(() => chainPrefs.get() === 'ignore');
  assert('hotkeySHold', chainPrefs.get() === 'ignore'
    && $('chain-btn-ignore').className.includes('pressed'),
    `mode=${chainPrefs.get()}`);
  keyup('s');
  await waitFor(() => chainPrefs.get() === null);

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

  // ---- 鼠标模式（control_mode==1）：场地内按住左键=全部连锁、右键=忽略
  // 连锁、松开即清（event_handler.cpp:1709-1730 EMIE_LMOUSE/RMOUSE_PRESSED_DOWN） ----
  const duelCanvas = document.createElement('div');
  duelCanvas.id = 'duel-canvas-container';
  document.body.appendChild(duelCanvas);
  duelCanvas.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, button: 0 }));
  await waitFor(() => chainPrefs.get() === 'always');
  assert('mouseHoldLeftAlwaysChain', chainPrefs.get() === 'always', `mode=${chainPrefs.get()}`);
  window.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, button: 0 }));
  await waitFor(() => chainPrefs.get() === null);
  duelCanvas.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, button: 2 }));
  await waitFor(() => chainPrefs.get() === 'ignore');
  assert('mouseHoldRightIgnoreChain', chainPrefs.get() === 'ignore', `mode=${chainPrefs.get()}`);
  window.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, button: 2 }));
  await waitFor(() => chainPrefs.get() === null);
  // control_mode==0 时鼠标按住不影响连锁偏好（仅键盘三键）
  settingsStore.set('control_mode', 0);
  duelCanvas.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, button: 0 }));
  await new Promise((r) => setTimeout(r, 30));
  assert('mouseHoldRequiresMouseMode', chainPrefs.get() === null, `mode=${chainPrefs.get()}`);
  window.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, button: 0 }));
  duelCanvas.remove();
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

  // 堆区可发动卡（cmdActions = 原版 ClientCard::cmdFlag 的 ACTIVATE 位）：
  // 列表内该卡高亮可点，点击弹 ActionPopup 带精确 idx
  // （本页 playerType=1：本方显示座 0 = 引擎 seat 1，条目的 c 域对引擎座标）
  eventBus.emit('duel:select_idlecmd', {
    player: 1, summon: [], spsummon: [], repos: [], mset: [], sset: [],
    activate: [{ code: 89631139, c: 1, l: 0x10, s: 0, desc: 0, idx: 3 }],
    toBP: 0, toEP: 1, shuffle: 0
  });
  await waitFor(() => document.querySelector('.card-display-item.card-display-actionable'));
  assert('pileActionableHighlighted', !!document.querySelector('.card-display-item.card-display-actionable'));
  document.querySelector('.card-display-item.card-display-actionable').click();
  await waitFor(() => duelStore.getState().actionPopup);
  const pileAct = duelStore.getState().actionPopup.actions[0];
  assert('pileActivateOffered', pileAct.action === 'activate' && pileAct.idx === 3, JSON.stringify(pileAct));
  duelStore.closeActionPopup();
  // 新阶段清除 cmdActions → 高亮消失（原版 ClearCommandFlag 时机）
  eventBus.emit('duel:new_phase', { player: 0, phase: 0x2 });
  await waitFor(() => !document.querySelector('.card-display-actionable'));
  assert('pileActionableClearedOnNewPhase', !document.querySelector('.card-display-actionable'));

  // 额外卡组：怪兽从 mzone 回到 extra（LOC_EXTRA=0x40 进堆区）
  eventBus.emit('duel:move', {
    code: 46986414, pc: 1, pl: 0x04, ps: 2, pp: 0x1,
    cc: 1, cl: 0x40, cs: 0, cp: 0x1, reason: 0,
  });
  await waitFor(() => $('player-pile-extra').innerText === '13'); // 12+1
  keyup('F2');
  assert('pileHotkeysIgnoredWhileOpen', $('card-display-title').innerText === '墓地(1)');
  $('card-display-close').click();
  await waitFor(() => !$('card-display-overlay'));
  keyup('F2');
  await new Promise(r=>setTimeout(r,30));
  assert('emptyBanishDoesNotOpen', !$('card-display-overlay'));
  keyup('F3');
  await waitFor(() => $('card-display-title')?.innerText === '额外(1)');
  assert('f3ExtraList', $('card-display-title').innerText === '额外(1)');
  $('card-display-close').click();
  await waitFor(() => !$('card-display-overlay'));

  // 超量素材：extra 卡吸附到 mzone[2]（LOC_OVERLAY=0x80，cs=槽位）
  eventBus.emit('duel:move', {
    code: 46986414, pc: 1, pl: 0x40, ps: 0, pp: 0x1,
    cc: 1, cl: 0x80, cs: 2, cp: 0x1, reason: 0,
  });
  await waitFor(() => duelStore.getState().board[0].overlay[2].length === 1);
  assert('overlayMaterialTracked', duelStore.getState().board[0].overlay[2][0] === 46986414);
  keyup('F4');
  await waitFor(() => $('card-display-title')?.innerText === '叠放(1)');
  assert('f4OverlayList', $('card-display-title').innerText === '叠放(1)');
  assert('extraDrainedByOverlayMove', $('player-pile-extra').innerText === '12');

  $('card-display-close').click();
  await waitFor(() => !$('card-display-overlay'));
  // C++ does not open empty piles.
  keyup('F5');
  await new Promise(r=>setTimeout(r,30));
  assert('emptyOpponentGraveDoesNotOpen', !$('card-display-overlay'));
  eventBus.emit('duel:move',{code:1001,pc:0,pl:1,ps:0,pp:1,cc:0,cl:16,cs:0,cp:1});
  keyup('F5');
  await waitFor(() => $('card-display-title')?.innerText === '对方墓地(1)');
  assert('f5OpponentPile',true);
  keyup('F5');
  await new Promise(r=>setTimeout(r,30));
  assert('sameKeyKeepsOpen',!!$('card-display-overlay'));
  $('card-display-close').click();
  await waitFor(() => !$('card-display-overlay'));

  // ESC minimises the native window instead of closing the card list.
  keyup('F1');
  await waitFor(() => !!$('card-display-overlay'));
  let minimises=0;WailsBridge.minimise=()=>{minimises++;};
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  assert('escMinimisesWindow',minimises===1&&!!$('card-display-overlay'));
  $('card-display-close').click();
  await waitFor(() => !$('card-display-overlay'));

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
