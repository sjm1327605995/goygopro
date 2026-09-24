// Headless smoke test for practice mode: drives the real interactive stack —
// DuelField3D + React widgets (HandDock/ActionPopup/PromptHost/CardPreviewPanel) +
// DuelManager + AISimulator — through real player actions and checks that
// each action actually drove the corresponding state transition, exactly as
// it would in a network duel. The simulator emits the same eventBus events
// the Go engine produces, so DuelManager needs no practice-mode special
// casing.
//
// P5 path: hand/LP/board live in duelStore (hud.js is gone); the summon is
// driven through the 2D action popup like a real user would.
import { createRoot } from 'react-dom/client';
import React from 'react';
import { DuelField3D } from '../src/duel/field3d.ts';
import { DuelManager } from '../src/duel/duel_manager.ts';
import { aiSimulator } from '../src/duel/ai_simulator.ts';
import { eventBus, WailsBridge } from '../src/wails_bridge.ts';
import { duelStore } from '../src/duel/store.ts';
import chainPrefs from '../src/duel/chain_prefs.ts';
import { soundManager } from '../src/audio/sound_manager.ts';
import PromptHost from '../src/components/PromptHost.tsx';
import HintBar from '../src/components/HintBar.tsx';
import CardPreviewPanel from '../src/components/CardPreviewPanel.tsx';
import HandDock from '../src/components/HandDock.tsx';
import ActionPopup from '../src/components/ActionPopup.tsx';
// 原版样式（动作弹窗/提示条的视觉断言需要真样式表）
import '../css/style.css';
import '../css/duel-original.css';

soundManager.muted = true;

// React mounts the interactive widgets (DuelStage would do this).
const root = createRoot(document.getElementById('root'));
root.render(
  <React.Fragment>
    <PromptHost />
    <HintBar />
    <CardPreviewPanel />
    <HandDock interactive={true} />
    <ActionPopup />
  </React.Fragment>
);

let manager;
const field = new DuelField3D(
  document.getElementById('stage'),
  (code) => duelStore.inspect(code),
  (x, y, userData) => manager.onFieldCardClick(x, y, userData),
);
manager = new DuelManager(field, { interactive: true });

// P4: record protocol sends (chain auto-decline / time-limit confirm / idle
// command responses) while still letting the real bridge methods run.
const protocolSends = [];
const wrap = (name, kind, keyOf) => {
  const orig = WailsBridge[name].bind(WailsBridge);
  WailsBridge[name] = (...args) => {
    protocolSends.push({ kind, ...keyOf(args) });
    return orig(...args);
  };
};
wrap('sendResponseI', 'responseI', (args) => ({ v: args[0] }));
wrap('sendTimeConfirm', 'timeConfirm', () => ({}));
wrap('respondIdleCmd', 'idleCmd', (args) => ({ idx: args[0], cmdType: args[1] }));
wrap('respondBattleCmd', 'battleCmd', (args) => ({ idx: args[0], cmdType: args[1] }));
// 卡图走确定性桩（真实实现会去 CDN 拉，冒烟不依赖外网）
WailsBridge.getCardImage = async (code) => ({ url: `/textures/cover.jpg#${code}`, full: false });

aiSimulator.DELAY = 40;

// Record every duel event so checks can inspect the full transcript.
const transcript = [];
const origEmit = eventBus.emit.bind(eventBus);
eventBus.emit = (event, data) => {
  if (event.startsWith('duel:') || event.startsWith('stoc:')) transcript.push({ event, data });
  return origEmit(event, data);
};
const emitted = (type) => transcript.some((t) => t.event === type);

// Polls until `predicate` holds (draw tweens finish asynchronously in the
// headless renderer before the hand dock updates).
const waitFor = (predicate, timeoutMs = 5000) => new Promise((resolve, reject) => {
  const started = Date.now();
  const tick = () => {
    if (predicate()) return resolve();
    if (Date.now() - started > timeoutMs) return reject(new Error('waitFor timeout'));
    setTimeout(tick, 25);
  };
  tick();
});

// Resolves when `type` fires after the call (subscribe before acting — no races).
const nextEvent = (type, timeoutMs = 5000) => new Promise((resolve, reject) => {
  const timer = setTimeout(() => {
    eventBus.off(type, listener);
    reject(new Error('timeout waiting for ' + type));
  }, timeoutMs);
  const listener = (data) => {
    clearTimeout(timer);
    eventBus.off(type, listener);
    resolve(data);
  };
  eventBus.on(type, listener);
});

// 日志收在左侧预览面板的「消息记录」页签（右侧 LogDrawer 已删，同原版 wInfos）：
// Radix Tabs 触发用 mousedown 激活，需先点开页签内容才挂载。
const openLogTab = async () => {
  const btn = [...document.querySelectorAll('.preview-tab')]
    .find((b) => b.textContent.includes('消息记录'));
  if (btn) {
    btn.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    btn.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }));
    btn.dispatchEvent(new MouseEvent('click', { bubbles: true }));
  }
  await waitFor(() => document.getElementById('duel-log-list'), 'log tab content');
};
const logText = () => document.getElementById('duel-log-list').innerText;
const popup = () => document.getElementById('action-popup');
const state = () => duelStore.getState();

window.__practiceSmoke = { field, manager, aiSimulator, store: duelStore, transcript, checks: {}, ready: false, chainPrefs, protocolSends };

(async () => {
  // React 18 createRoot commits the initial mount asynchronously — wait for
  // the widgets to exist before driving them.
  await waitFor(() => document.getElementById('modal-overlay')
    && document.getElementById('hand-cards-dock')
    && document.getElementById('action-popup'));
  await openLogTab();

  // ---- P4: chain-prefs 'ignore' auto-declines a non-forced select_chain
  // without showing any popup (gframe duelclient.cpp:1776 local semantics —
  // the three chain buttons send no packets). Same key toggles it off. ----
  chainPrefs.set('ignore');
  eventBus.emit('duel:select_chain', { forced: false, chains: [{ code: 89631139, desc: 0 }] });
  await waitFor(() => protocolSends.some((s) => s.kind === 'responseI' && s.v === -1));
  const overlay = document.getElementById('modal-overlay');
  const checksPre = {
    chainIgnoreAutoDeclined: protocolSends.some((s) => s.kind === 'responseI' && s.v === -1)
      && !overlay.className.includes('active'),
  };
  chainPrefs.set('ignore'); // same key again = cancel (push-button semantics)
  checksPre.chainPrefsCleared = chainPrefs.get() === null;

  // With prefs cleared the same prompt shows the normal yes/no popup
  // (PromptHost owns all modals now; the manager never opens one).
  eventBus.emit('duel:select_chain', { forced: false, chains: [{ code: 89631139, desc: 0 }] });
  await waitFor(() => overlay.querySelector('#modal-btn-yes'));
  checksPre.chainPopupShowsYesNo = overlay.className.includes('active');
  overlay.querySelector('#modal-btn-yes').click();
  await waitFor(() => protocolSends.some((s) => s.kind === 'responseI' && s.v === 0)
    && !overlay.className.includes('active'));

  aiSimulator.start();

  // Turn 1: the simulator prompts the player with an idle command built from
  // the actual opening hand.
  const firstIdle = await nextEvent('duel:select_idlecmd');
  await waitFor(() => state().hand.length === 5
    && document.querySelectorAll('#hand-cards-dock .hand-card-item').length === 5);
  const checks = {
    ...checksPre,
    // Blue-Eyes is level 8: rules-correct prompts offer it as a monster set,
    // not a normal summon (no tributes in this simplified simulator).
    idlePromptHasMset: firstIdle.mset.some((c) => c.code === 89631139),
    turn1NoBattlePhase: !firstIdle.toBP,
    handDealt: state().hand.length === 5,
  };

  // ---- P4: STOC_TIME_LIMIT for our seat immediately answers CTOS_TIME_CONFIRM
  // (gframe duelclient.cpp:783-784). playerSlot is known after duel:start. ----
  eventBus.emit('stoc:time_limit', { player: manager.playerSlot, leftTime: 180 });
  await waitFor(() => protocolSends.some((s) => s.kind === 'timeConfirm'));
  checks.timeConfirmSent = protocolSends.some((s) => s.kind === 'timeConfirm');

  // ---- UI-driven action: click Pot of Greed in the hand dock → the 2D
  // action popup offers what the engine's idle command allows (activate +
  // set) → clicking 发动 answers RespondIdleCmd(idx, 5) AND drives the
  // simulator's real activation (draw 2). ----
  document.querySelector('#hand-cards-dock .hand-card-item[data-code="55144522"]').click();
  await waitFor(() => popup().querySelectorAll('.action-btn').length >= 1);
  const popupLabels = [...popup().querySelectorAll('.action-btn')].map((b) => b.innerText);
  checks.popupOffersEngineActions = popupLabels.includes('发动') && popupLabels.includes('盖放')
    && !popupLabels.includes('召唤'); // level-8 monsters get no 召唤（原版 1151）
  [...popup().querySelectorAll('.action-btn')]
    .find((b) => b.innerText === '发动').click();
  const potPrompt = nextEvent('duel:select_idlecmd'); // re-prompt after the chain resolves
  await waitFor(() => state().hand.length === 6); // 5 − Pot itself + two draws
  await potPrompt;
  checks.popupActivateResponded = protocolSends.some((s) => s.kind === 'idleCmd' && s.cmdType === 5);
  checks.potDrewTwo = state().hand.length === 6 && emitted('duel:chaining');

  // ---- P3: MSG_SHUFFLE_HAND 手牌重排——手牌坞播抖动动画并应用新顺序 ----
  const beforeShuffle = state().hand.map((c) => c.code);
  const shuffled = [...beforeShuffle].reverse();
  eventBus.emit('duel:shuffle_hand', { player: manager.playerSlot, cards: shuffled });
  await waitFor(() => document.getElementById('hand-cards-dock').className.includes('shuffling'));
  checks.shuffleAnimatesDock = document.getElementById('hand-cards-dock').className.includes('shuffling');
  await waitFor(() => JSON.stringify(state().hand.map((c) => c.code)) === JSON.stringify(shuffled));
  checks.shuffleReordersHand = JSON.stringify(state().hand.map((c) => c.code)) === JSON.stringify(shuffled);

  // Player summons Blue-Eyes face-up through the same entry point the popup
  // uses (handlePlayerAction). The rules-correct popup only offered 盖放
  // for a level-8 monster, but the practice simulator honors the summon
  // action directly — that keeps the battle-phase path testable.
  const rePrompt = nextEvent('duel:select_idlecmd');
  manager.handlePlayerAction('summon', { code: 89631139 });
  await rePrompt;
  await waitFor(() => !!field.cardsOnField[0].mzone[0]);
  checks.summonPlacedMesh = !!(field.cardsOnField[0].mzone[0]);
  checks.summonLeftHand = state().hand.length === 5;
  checks.summonInStoreBoard = state().board[0].mzone[0].code === 89631139;
  checks.summonLogged = emitted('duel:summoning');

  // ---- P5 Step E: store resync. Simulate a drifted mesh slot (as a
  // missed/mis-decoded move would leave it), then poke the engine's
  // authoritative update_data — the manager must rebuild the mesh from
  // duelStore.board. (The test drives the actual drift, not a fresh board.) ----
  const meshBefore = field.cardsOnField[0].mzone[0];
  field.cardsOnField[0].mzone[0] = null;
  eventBus.emit('duel:update_data', { player: 0 });
  await waitFor(() => field.cardsOnField[0].mzone[0]
    && field.cardsOnField[0].mzone[0] !== meshBefore
    && field.cardsOnField[0].mzone[0].userData.cardCode === 89631139
    && !manager._resyncing);
  checks.resyncRebuiltMesh = field.cardsOnField[0].mzone[0].userData.cardCode === 89631139;

  // End turn → the AI takes its turn (sets a facedown monster, cannot attack)
  // and hands the turn back.
  const aiIdle = nextEvent('duel:select_idlecmd');
  manager.handlePlayerAction('phase_change', 'EP');
  await aiIdle;
  await waitFor(() => !!field.cardsOnField[1].mzone[0]);
  // 盖怪=守备盖卡：顶边指向己方右手边（p1 → +PI/2，见 field3d.cardRotationFor）
  checks.aiSetFacedown = !!(field.cardsOnField[1].mzone[0]) &&
    Math.round(field.cardsOnField[1].mzone[0].rotation.y * 100) / 100 === 1.57;
  checks.aiSetInStoreBoard = !!state().board[1].mzone[0];
  checks.turnReturned = emitted('duel:new_turn');

  // Turn 2: go to battle and attack — the AI's only monster is face-down, so
  // the attack resolves as direct damage for full ATK.
  const battlePrompt = nextEvent('duel:select_battlecmd');
  manager.handlePlayerAction('phase_change', 'BP');
  const battleCmd = await battlePrompt;
  checks.battlePromptHasAttack = battleCmd.attack.some((a) => a.s === 0 && a.code === 89631139);

  const damageEvent = nextEvent('duel:damage');
  const reBattlePrompt = nextEvent('duel:select_battlecmd');
  manager.handlePlayerAction('attack', { s: 0 });
  const dmg = await damageEvent;
  await reBattlePrompt;
  checks.directAttackDamage = dmg.player === 1 && dmg.amount === 3000;
  checks.opponentLPFromStore = state().lp[1] === 5000;
  checks.attackAnimated = emitted('duel:attack');

  // End turn again: the AI draws, sets a second monster, passes back.
  const thirdTurn = nextEvent('duel:select_idlecmd');
  manager.handlePlayerAction('phase_change', 'EP');
  await thirdTurn;
  await waitFor(() => state().hand.length === 7); // 5 + turn-2 + turn-3 draws
  checks.turnThreePrompted = state().hand.length === 7;
  checks.noCrashAfterAiTurn = aiSimulator.running;

  checks.logIsChinese = logText().includes('通常召唤')
    && logText().includes('宣告攻击');

  window.__practiceSmoke.checks = checks;
  window.__practiceSmoke.ready = true;
  document.title = 'SMOKE:' + JSON.stringify(checks);
})().catch((err) => {
  window.__practiceSmoke.checks = { fatal: String(err && err.stack || err) };
  window.__practiceSmoke.ready = true;
  document.title = 'SMOKE:' + JSON.stringify(window.__practiceSmoke.checks);
});
