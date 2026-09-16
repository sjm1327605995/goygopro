// Headless smoke test for the engine prompt handlers: emits the same
// eventBus events the Go client produces for the newly covered
// MSG_SELECT_* / MSG_ANNOUNCE_* / display messages, drives the React
// PromptHost modals exactly like a user would, and asserts the semantic
// response parameters sent through WailsBridge (byte encodings are locked
// down by Go-side responses_test.go).
//
// P5 path: the modals live in components/PromptHost.tsx (hud.js is gone);
// state assertions read duelStore + the React-rendered DOM.
import { createRoot } from 'react-dom/client';
import React from 'react';
import { DuelField3D } from '../src/duel/field3d.ts';
import { DuelManager } from '../src/duel/duel_manager.ts';
import { eventBus, WailsBridge } from '../src/wails_bridge.ts';
import { soundManager } from '../src/audio/sound_manager.ts';
import { duelStore } from '../src/duel/store.ts';
import PromptHost from '../src/components/PromptHost.tsx';
import HintBar from '../src/components/HintBar.tsx';
import LogDrawer from '../src/components/LogDrawer.tsx';
import RightControls from '../src/components/RightControls.tsx';
// 弹窗原版样式（视觉断言需要真样式表）
import '../css/style.css';
import '../css/duel-original.css';

soundManager.muted = true;

// Records every response the handlers produce. Byte-level responses are gone;
// the handlers call the semantic Respond* bridge methods, so the stubs record
// the semantic parameters they receive.
const responses = [];
WailsBridge.sendResponseB = (data) => { responses.push({ kind: 'B', data: Array.from(data) }); };
WailsBridge.sendResponseI = (v) => { responses.push({ kind: 'I', v }); };
WailsBridge.respondSelectCard = (indices) => { responses.push({ kind: 'card', indices: [...indices] }); };
WailsBridge.respondSelectUnselect = (index) => { responses.push({ kind: 'unselect', index }); };
WailsBridge.respondCounter = (counts) => { responses.push({ kind: 'counter', counts: [...counts] }); };
WailsBridge.respondSelectSum = (count, indices) => { responses.push({ kind: 'sum', count, indices: [...indices] }); };
WailsBridge.respondSelectPlace = (player, loc, seq) => { responses.push({ kind: 'place', player, loc, seq }); };
WailsBridge.respondSortCard = (perm) => { responses.push({ kind: 'sort', perm: [...perm] }); };
WailsBridge.respondSortCardCancel = () => { responses.push({ kind: 'sortCancel' }); };
WailsBridge.respondIdleCmd = (idx, cmdType) => { responses.push({ kind: 'idle', idx, cmdType }); };
WailsBridge.respondBattleCmd = (idx, cmdType) => { responses.push({ kind: 'battle', idx, cmdType }); };
// 卡图走确定性桩（真实实现会去 CDN 拉，冒烟不依赖外网）
WailsBridge.getCardImage = async (code) => ({ url: `/textures/cover.jpg#${code}`, full: false });

// React mounts the prompt host + hint bar + log drawer (all store-driven).
const observerLeaves = [];
const leaveSends = [];
WailsBridge.leaveGame = () => leaveSends.push(1);
const root = createRoot(document.getElementById('root'));
root.render(
  <React.Fragment>
    <PromptHost />
    <HintBar />
    <LogDrawer />
    <RightControls onLeaveObserver={() => observerLeaves.push(1)} />
  </React.Fragment>
);

// The manager still owns the command state + protocol plumbing the prompts
// smoke exercises (handlePlayerAction, select_place auto-answer).
const field = new DuelField3D(document.getElementById('stage'), () => {}, () => {});
const manager = new DuelManager(field, { interactive: true });
void manager;

const waitFor = (predicate, timeoutMs = 5000) => new Promise((resolve, reject) => {
  const started = Date.now();
  const tick = () => {
    if (predicate()) return resolve();
    if (Date.now() - started > timeoutMs) return reject(new Error('waitFor timeout'));
    setTimeout(tick, 25);
  };
  tick();
});

const overlay = () => document.getElementById('modal-overlay');
const waitForModal = (selector) => waitFor(() => overlay().querySelector(selector));
// React unmounts modal children on close; waiting for the overlay to go
// inert keeps later waitForModal calls from racing the previous modal.
const waitModalClosed = () => waitFor(() => !overlay().className.includes('active'));
// Waits for exactly one new response to land (baseline marker, not absolute
// counts — those go stale the moment a section is inserted or reordered).
let respMark = 0;
const waitResp = () => waitFor(() => responses.length > respMark).then(() => {
  respMark = responses.length;
  return responses[respMark - 1];
});

window.__promptsSmoke = { checks: {}, ready: false, responses, store: duelStore };

const run = async () => {
  const checks = {};
  // 提前挂引用：fatal 时保留已完成的断言，便于定位卡在哪个 waitFor
  window.__promptsSmoke.checks = checks;
  window.__promptsSmoke.field = field;
  window.__promptsSmoke.manager = manager;
  const assert = (name, cond, detail = '') => {
    checks[name] = cond ? true : `FAIL: ${detail}`;
  };

  // React 18 createRoot commits the initial mount asynchronously — wait for
  // the widgets to exist before emitting events at them.
  await waitFor(() => overlay() && document.getElementById('hint-bar')
    && document.getElementById('duel-log-list'));

  eventBus.emit('duel:start', { playerType: 0, lp0: 8000, lp1: 8000 });

  // ---- MSG_SELECT_COUNTER: uint16 LE per card, summing to count ----
  eventBus.emit('duel:select_counter', {
    player: 0, countertype: 1, count: 3,
    cards: [
      { code: 1, c: 0, l: 0x4, s: 0, cnt: 2 },
      { code: 2, c: 0, l: 0x4, s: 1, cnt: 2 }
    ]
  });
  await waitForModal('.counter-inc');
  const incs = overlay().querySelectorAll('.counter-inc');
  incs[0].click(); incs[0].click(); incs[1].click();
  await waitFor(() => !overlay().querySelector('#counter-confirm').disabled);
  overlay().querySelector('#counter-confirm').click();
  let r = await waitResp();
  await waitModalClosed();
  assert('counterResponse', r.kind === 'counter' && JSON.stringify(r.counts) === '[2,1]', JSON.stringify(r));

  // ---- MSG_SELECT_SUM: [mustCount + pickedCount, ...indices] ----
  eventBus.emit('duel:select_sum', {
    sumMode: 0, player: 0, acc: 7, min: 1, max: 3,
    must: [{ code: 9, c: 0, l: 0x4, s: 0, param: 2 }],
    select: [
      { code: 1, c: 0, l: 0x4, s: 0, param: 5 },
      { code: 2, c: 0, l: 0x4, s: 1, param: 4 },
      { code: 3, c: 0, l: 0x4, s: 2, param: 3 }
    ]
  });
  await waitForModal('.sum-card');
  const sumCards = overlay().querySelectorAll('.sum-card');
  sumCards[0].click(); // param 5; must(2) + 5 = acc 7
  await waitFor(() => !overlay().querySelector('#sum-confirm').disabled);
  overlay().querySelector('#sum-confirm').click();
  r = await waitResp();
  await waitModalClosed();
  assert('sumResponse', r.kind === 'sum' && r.count === 2 && JSON.stringify(r.indices) === '[0]', JSON.stringify(r));

  // ---- MSG_SORT_CARD: permutation bytes (card i -> chosen position) ----
  eventBus.emit('duel:sort_card', {
    player: 0,
    cards: [
      { code: 11, c: 0, l: 0x1, s: 0 },
      { code: 12, c: 0, l: 0x1, s: 1 },
      { code: 13, c: 0, l: 0x1, s: 2 }
    ]
  });
  await waitForModal('.sort-card');
  const sortCards = overlay().querySelectorAll('.sort-card');
  sortCards[2].click(); sortCards[0].click(); sortCards[1].click();
  await waitFor(() => !overlay().querySelector('#sort-confirm').disabled);
  overlay().querySelector('#sort-confirm').click();
  r = await waitResp();
  await waitModalClosed();
  assert('sortResponse', r.kind === 'sort' && JSON.stringify(r.perm) === '[1,2,0]', JSON.stringify(r));

  // ---- MSG_SELECT_UNSELECT_CARD: [1, combinedIndex]; combined list is
  // select-list first, then the unselect list ----
  eventBus.emit('duel:select_unselect', {
    player: 0, finishable: false, cancelable: true, min: 1, max: 1,
    cards: [{ code: 21, c: 0, l: 0x4, s: 0 }],
    unselectList: [{ code: 22, c: 0, l: 0x4, s: 1 }]
  });
  await waitForModal('.select-card-item');
  const unselCards = overlay().querySelectorAll('.select-card-item');
  assert('unselectShowsBothLists', unselCards.length === 2, `got ${unselCards.length}`);
  unselCards[1].click(); // the unselect-list entry → combined index 1
  await waitFor(() => !overlay().querySelector('#modal-select-confirm').disabled);
  overlay().querySelector('#modal-select-confirm').click();
  r = await waitResp();
  await waitModalClosed();
  assert('unselectResponse', r.kind === 'unselect' && r.index === 1, JSON.stringify(r));

  // ---- MSG_ANNOUNCE_RACE: int32 bitmask with exactly count bits ----
  eventBus.emit('duel:announce_race', { player: 0, count: 2, available: 0x2003 });
  await waitForModal('.mask-opt');
  const masks = overlay().querySelectorAll('.mask-opt');
  assert('raceOffersAvailableOnly', masks.length === 3, `got ${masks.length}`);
  masks[0].click(); masks[1].click();
  await waitFor(() => !overlay().querySelector('#mask-confirm').disabled);
  overlay().querySelector('#mask-confirm').click();
  r = await waitResp();
  await waitModalClosed();
  assert('raceResponse', r.kind === 'I' && r.v === 0x3, JSON.stringify(r));

  // ---- MSG_ANNOUNCE_NUMBER: int32 option index ----
  eventBus.emit('duel:announce_number', { player: 0, options: [100, 200, 300] });
  await waitForModal('.num-opt');
  overlay().querySelectorAll('.num-opt')[1].click();
  r = await waitResp();
  await waitModalClosed();
  assert('numberResponse', r.kind === 'I' && r.v === 1, JSON.stringify(r));

  // ---- MSG_ANNOUNCE_CARD: Go-decoded candidates become candidate buttons
  // (the opcode machine now lives in Go — decorateAnnounceCard) ----
  eventBus.emit('duel:announce_card', {
    player: 0, options: [86039056, 0x40000100],
    candidates: [86039056], decodable: true,
  });
  await waitForModal('.cand-card');
  overlay().querySelector('.cand-card').click();
  r = await waitResp();
  await waitModalClosed();
  assert('announceCardResponse', r.kind === 'I' && r.v === 86039056, JSON.stringify(r));

  // ---- MSG_WAITING → stHintMsg 提示条（原版 SysString 1409） ----
  const hintBar = () => document.getElementById('hint-bar');
  eventBus.emit('duel:waiting', {});
  await waitFor(() => hintBar().style.display === 'block' && hintBar().innerText.includes('等待'));
  assert('waitingHint', hintBar().innerText.includes('等待行动中')
    && duelStore.getState().hint === '等待行动中...');

  // ---- MSG_ROCK_PAPER_SCISSORS: f1=石头→1 f2=剪刀→2 f3=布→3（gframe
  // event_handler.cpp:33，出拳值与 STOC 选择手牌同套）；弹窗打开时提示条隐藏 ----
  eventBus.emit('duel:rps', { phase: 0 });
  await waitForModal('#rps-paper');
  await waitFor(() => hintBar().style.display === 'none' && duelStore.getState().hint === null);
  assert('waitingHintCleared', hintBar().style.display === 'none');
  const rockBtn = overlay().querySelector('#rps-rock');
  assert('rpsHandTexture', /\/f1-[^"]*\.jpg/.test(getComputedStyle(rockBtn).backgroundImage),
    getComputedStyle(rockBtn).backgroundImage);
  overlay().querySelector('#rps-paper').click();
  r = await waitResp();
  await waitModalClosed();
  assert('rpsPaperIsThree', r.kind === 'I' && r.v === 3, JSON.stringify(r));

  // ---- MSG_PAY_LPCOST: LP drops by the paid amount (store holds the truth) ----
  const lpBefore = duelStore.getState().lp[0];
  eventBus.emit('duel:pay_lpcost', { player: 0, amount: 1000 });
  await waitFor(() => duelStore.getState().lp[0] === lpBefore - 1000);
  assert('payLpCost', duelStore.getState().lp[0] === 7000, `lp=${duelStore.getState().lp[0]}`);

  // ---- MSG_TOSS_COIN: logged with 正/反 faces ----
  eventBus.emit('duel:toss_coin', { player: 0, results: [1, 0] });
  await waitFor(() => document.getElementById('duel-log-list').innerText.includes('掷硬币'));
  assert('tossCoinLogged', document.getElementById('duel-log-list').innerText.includes('正、反'));

  // ---- MSG_BECOME_TARGET: logged without crashing ----
  eventBus.emit('duel:become_target', {
    targets: [{ c: 0, l: 0x4, s: 0 }]
  });
  await waitFor(() => document.getElementById('duel-log-list').innerText.includes('被选为对象'));
  assert('becomeTargetLogged', true);

  // ---- MSG_CONFIRM_DECKTOP: top card name logged ----
  eventBus.emit('duel:confirm_decktop', {
    player: 0,
    cards: [{ code: 89631139, c: 0, l: 0x1, s: 0 }]
  });
  await waitFor(() => document.getElementById('duel-log-list').innerText.includes('卡组顶端'));
  assert('deckTopLogged', true);

  // ---- MSG_HAND_RES: logged without crashing ----
  eventBus.emit('duel:hand_res', { res: 0x05 });
  assert('handResNoCrash', true);

  // ---- MSG_ANNOUNCE_CARD with a non-code filter falls back to free input
  // (Go reports decodable=false, the handler shows the numeric input) ----
  eventBus.emit('duel:announce_card', {
    player: 0,
    options: [0x51, 0x40000102, 0x40000007], // TYPE_XYZ, ISTYPE, NOT
    candidates: [],
    decodable: false,
  });
  await waitForModal('#announce-card-input');
  const input = overlay().querySelector('#announce-card-input');
  const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
  setter.call(input, '70902743');
  input.dispatchEvent(new Event('input', { bubbles: true }));
  overlay().querySelector('#announce-card-confirm').click();
  r = await waitResp();
  await waitModalClosed();
  assert('announceCardInputFallback', r.kind === 'I' && r.v === 70902743, JSON.stringify(r));

  // ---- MSG_SELECT_POSITION: 原版 wPosSelect —— 137px 卡图方按钮，
  // 0x1|0x8 只出两个按钮（表攻 / 里守），点击回 POS 位掩码 ----
  eventBus.emit('duel:select_position', { player: 0, code: 89631139, positions: 0x1 | 0x8 });
  await waitForModal('.pos-opt');
  const posBtns = overlay().querySelectorAll('.pos-opt');
  assert('posOffersTwo', posBtns.length === 2, `got ${posBtns.length}`);
  assert('posFirstBtn', !!overlay().querySelector('#pos-first-btn'));
  assert('posFacedownCover', /\/cover[-.][^"]*\.jpg/.test(getComputedStyle(posBtns[1].querySelector('.pos-opt-img'))
    .backgroundImage), getComputedStyle(posBtns[1].querySelector('.pos-opt-img')).backgroundImage);
  posBtns[1].click(); // 里侧守备表示 = 0x8
  r = await waitResp();
  await waitModalClosed();
  assert('posResponse', r.kind === 'I' && r.v === 0x8, JSON.stringify(r));

  // ---- MSG_ANNOUNCE_NUMBER 分页：7 个选项一页 5 项，>>> 翻页后选第 7 项 ----
  eventBus.emit('duel:announce_number', { player: 0, options: [1, 2, 3, 4, 5, 6, 7] });
  await waitForModal('.num-opt');
  assert('optPaged', overlay().querySelectorAll('.num-opt').length === 5
    && !!overlay().querySelector('#opt-next'),
    `opts=${overlay().querySelectorAll('.num-opt').length}`);
  overlay().querySelector('#opt-next').click();
  await waitFor(() => overlay().querySelector('.num-opt[data-idx="6"]'));
  overlay().querySelector('.num-opt[data-idx="6"]').click();
  r = await waitResp();
  await waitModalClosed();
  assert('numberPagingResponse', r.kind === 'I' && r.v === 6, JSON.stringify(r));

  // ---- 卡片图块：选择卡牌弹窗的卡牌项异步换上真实卡图（cover 兜底） ----
  eventBus.emit('duel:select_card', {
    player: 0, cancelable: false, min: 1, max: 1,
    cards: [{ code: 89631139, c: 0, l: 0x2, s: 0 }],
  });
  await waitForModal('.card-tile');
  const tile = overlay().querySelector('.card-tile');
  assert('tileHasNameStrip', !!tile.querySelector('.card-tile-name')
    && tile.querySelector('.card-tile-name').innerText.includes('Blue-Eyes'),
    tile.innerText);
  await waitFor(() => tile.classList.contains('card-tile-loaded'));
  assert('tileImageLoaded', tile.style.backgroundImage.includes('#89631139'),
    tile.style.backgroundImage.slice(0, 60));
  // 单卡选择也要点确认（min=1 选中后确认钮可用）
  tile.click();
  await waitFor(() => !overlay().querySelector('#modal-select-confirm').disabled);
  overlay().querySelector('#modal-select-confirm').click();
  r = await waitResp();
  await waitModalClosed();
  assert('singleCardSelectResponse', r.kind === 'card' && JSON.stringify(r.indices) === '[0]',
    JSON.stringify(r));

  // ---- MSG_SELECT_YESNO ----
  eventBus.emit('duel:select_yesno', { player: 0, desc: 0 });
  await waitForModal('#modal-btn-no');
  overlay().querySelector('#modal-btn-no').click();
  r = await waitResp();
  await waitModalClosed();
  assert('yesNoResponse', r.kind === 'I' && r.v === 0, JSON.stringify(r));

  // ---- MSG_SELECT_YESNO + ResolveDesc：desc 可解析时消息用真实文本 ----
  WailsBridge.resolveDesc = async (id) => (id === 0x896311391 ? '把「青眼白龙」解放吗？' : '');
  eventBus.emit('duel:select_yesno', { player: 0, desc: 0x896311391 });
  await waitForModal('#modal-btn-no');
  assert('yesNoResolvedText', overlay().innerText.includes('青眼白龙'), overlay().innerText);
  overlay().querySelector('#modal-btn-no').click();
  r = await waitResp();
  await waitModalClosed();
  assert('yesNoResolvedResponse', r.kind === 'I' && r.v === 0, JSON.stringify(r));

  // ---- MSG_SELECT_OPTION：可解析的用原文，解析不出的按编号兜底 ----
  eventBus.emit('duel:select_option', { player: 0, options: [0x896311391, 0] });
  await waitForModal('.card-tile');
  const optionNames = Array.from(overlay().querySelectorAll('.card-tile-name')).map(n => n.innerText);
  assert('optionResolvedNames', optionNames.length === 2
    && optionNames[0].includes('青眼白龙') && optionNames[1].includes('选项 2'),
    JSON.stringify(optionNames));
  overlay().querySelectorAll('.card-tile')[0].click();
  await waitFor(() => !overlay().querySelector('#modal-select-confirm').disabled);
  overlay().querySelector('#modal-select-confirm').click();
  r = await waitResp();
  await waitModalClosed();
  assert('optionResponse', r.kind === 'I' && r.v === 0, JSON.stringify(r));

  // ---- MSG_SELECT_EFFECTYN：desc 解析优先于卡名兜底 ----
  eventBus.emit('duel:select_effectyn', { player: 0, code: 89631139, loc: 0x4, desc: 0x896311391 });
  await waitForModal('#modal-btn-no');
  assert('effectynResolvedText', overlay().innerText.includes('青眼白龙'), overlay().innerText);
  overlay().querySelector('#modal-btn-yes').click();
  r = await waitResp();
  await waitModalClosed();
  assert('effectynResponse', r.kind === 'I' && r.v === 1, JSON.stringify(r));
  WailsBridge.resolveDesc = async () => '';

  // ---- 满足最少张数即闪黄（原版 select_ready 高亮，drawing.cpp:577：
  //    btnCancelOrFinish && select_ready → 黄色 selection line；
  //    select_ready = 已选 ≥ select_min，duelclient.cpp:1574）----
  eventBus.emit('duel:select_card', {
    player: 0, cancelable: false, min: 1, max: 2,
    cards: [
      { code: 89631139, c: 0, l: 0x2, s: 0 },
      { code: 46986414, c: 0, l: 0x2, s: 1 },
    ],
  });
  await waitForModal('.card-tile');
  const flashConfirm = () => overlay().querySelector('#modal-select-confirm');
  assert('unselectedNotFlashing', !flashConfirm().classList.contains('btn-flash-gold'),
    flashConfirm().className);
  overlay().querySelectorAll('.card-tile')[0].click();
  await waitFor(() => flashConfirm().classList.contains('btn-flash-gold'));
  assert('selectReadyFlashesAtMin', flashConfirm().classList.contains('btn-flash-gold'),
    flashConfirm().className);
  overlay().querySelectorAll('.card-tile')[1].click();
  await waitFor(() => flashConfirm().classList.contains('btn-flash-gold'));
  assert('fullSelectStillFlashes', flashConfirm().classList.contains('btn-flash-gold'));
  flashConfirm().click();
  r = await waitResp();
  await waitModalClosed();
  assert('fullSelectResponse', r.kind === 'card' && JSON.stringify(r.indices) === '[0,1]',
    JSON.stringify(r));

  // ---- MSG_SELECT_IDLECMD: phase confirms go through RespondIdleCmd(0, 6/7) ----
  eventBus.emit('duel:select_idlecmd', {
    player: 0, summon: [], spsummon: [], repos: [], mset: [], sset: [],
    activate: [], toBP: 1, toEP: 1, shuffle: 0
  });
  manager.handlePlayerAction('phase_change', 'BP');
  r = await waitResp();
  assert('idleToBP', r.kind === 'idle' && r.idx === 0 && r.cmdType === 6, JSON.stringify(r));
  manager.handlePlayerAction('phase_change', 'EP');
  r = await waitResp();
  assert('idleToEP', r.kind === 'idle' && r.idx === 0 && r.cmdType === 7, JSON.stringify(r));

  // ---- MSG_SELECT_IDLECMD: summon response uses list idx + cmdType 0 ----
  eventBus.emit('duel:select_idlecmd', {
    player: 0, summon: [{ code: 89631139, c: 0, l: 0x2, s: 0, idx: 1 }],
    spsummon: [], repos: [], mset: [], sset: [], activate: [],
    toBP: 0, toEP: 1, shuffle: 0
  });
  manager.handlePlayerAction('summon', { code: 89631139 });
  r = await waitResp();
  assert('idleSummon', r.kind === 'idle' && r.idx === 1 && r.cmdType === 0, JSON.stringify(r));

  // ---- MSG_SELECT_BATTLECMD: M2/EP confirms + attack by slot (battlecmd
  // type 1, attack branch matches a.s === data.s) ----
  eventBus.emit('duel:select_battlecmd', {
    player: 0, activate: [],
    attack: [{ code: 1, c: 0, l: 0x4, s: 0, diratt: 0, idx: 2 }],
    toM2: 1, toEP: 1
  });
  manager.handlePlayerAction('phase_change', 'M2');
  r = await waitResp();
  assert('battleToM2', r.kind === 'battle' && r.idx === 0 && r.cmdType === 2, JSON.stringify(r));
  manager.handlePlayerAction('attack', { s: 0 });
  r = await waitResp();
  assert('battleAttack', r.kind === 'battle' && r.idx === 2 && r.cmdType === 1, JSON.stringify(r));
  manager.handlePlayerAction('phase_change', 'EP');
  r = await waitResp();
  assert('battleToEP', r.kind === 'battle' && r.idx === 0 && r.cmdType === 3, JSON.stringify(r));

  // ---- MSG_SELECT_PLACE: semantic zones [{loc,seq}] decoded by Go ----
  eventBus.emit('duel:select_place', {
    player: 0, count: 1, flag: 0x8,
    zones: [{ loc: 0x04, seq: 3 }]
  });
  r = await waitResp();
  assert('semanticPlace', r.kind === 'place' && r.player === 0 && r.loc === 0x04 && r.seq === 3, JSON.stringify(r));

  // ---- 波 B：MSG_ADD/REMOVE_COUNTER → 卡上黄色数字角标（field3d 子精灵）
  // + store.counters 键 `c:l:s:type` ----
  const cntMesh = field.placeCard(0, 'mzone', 0, 89631139, null, 0x1);
  eventBus.emit('duel:add_counter', { type: 0x1051, c: 0, l: 0x4, s: 0, count: 2 });
  await waitFor(() => cntMesh.userData.counterSprite);
  const cntKey = `0:4:0:${0x1051}`;
  assert('counterBadgeShown', cntMesh.userData.counters[String(0x1051)] === 2,
    JSON.stringify(cntMesh.userData.counters));
  eventBus.emit('duel:remove_counter', { type: 0x1051, c: 0, l: 0x4, s: 0, count: 2 });
  await waitFor(() => !cntMesh.userData.counterSprite);
  assert('counterBadgeCleared', !cntMesh.userData.counters[String(0x1051)]
    && duelStore.getState().counters[cntKey] === undefined);

  // ---- MSG_EQUIP / MSG_UNEQUIP → 装备线（橙）出现与消失 ----
  field.placeCard(0, 'szone', 0, 5318639, null, 0x1);
  field.placeCard(0, 'mzone', 1, 46986414, null, 0x1);
  eventBus.emit('duel:equip', { card: { c: 0, l: 0x8, s: 0 }, target: { c: 0, l: 0x4, s: 1 } });
  await waitFor(() => field.relationLines.some((l) => l.kind === 'equip'));
  assert('equipLineAdded', field.relationLines.length === 1
    && field.relationLines[0].fromMesh.userData.cardCode === 5318639);
  eventBus.emit('duel:unequip', { card: { c: 0, l: 0x8, s: 0 } });
  await waitFor(() => !field.relationLines.some((l) => l.kind === 'equip'));
  assert('equipLineRemoved', field.relationLines.length === 0);

  // ---- MSG_CARD_TARGET / MSG_CANCEL_TARGET → 目标线（青） ----
  eventBus.emit('duel:card_target', { card: { c: 0, l: 0x4, s: 0 }, target: { c: 0, l: 0x4, s: 1 } });
  await waitFor(() => field.relationLines.some((l) => l.kind === 'target'));
  assert('targetLineAdded', field.relationLines.length === 1);
  eventBus.emit('duel:cancel_target', { card: { c: 0, l: 0x4, s: 0 }, target: { c: 0, l: 0x4, s: 1 } });
  await waitFor(() => field.relationLines.length === 0);
  assert('targetLineRemoved', true);

  // ---- MSG_SHUFFLE_SET_CARD：盖卡 mesh 按卡密码配对换位 ----
  const setA = field.placeCard(0, 'szone', 2, 10000001, null, 0x8);
  const setB = field.placeCard(0, 'szone', 3, 20000002, null, 0x8);
  eventBus.emit('duel:shuffle_set_card', {
    loc: 0x8,
    cards: [
      { code: 20000002, c: 0, l: 0x8, s: 2, p: 0x8 },
      { code: 10000001, c: 0, l: 0x8, s: 3, p: 0x8 },
    ],
  });
  await waitFor(() => field.cardsOnField[0].szone[2] === setB
    && field.cardsOnField[0].szone[3] === setA);
  assert('shuffleSetCardSwapped', setB.userData.cardCode === 20000002
    && setA.userData.cardCode === 10000001);

  // ---- MSG_CONFIRM_CARDS：日志 + 场上卡闪光（无异常即过） ----
  eventBus.emit('duel:confirm_cards', {
    player: 0, skipPanel: true, cards: [{ code: 89631139, c: 0, l: 0x4, s: 0 }],
  });
  await waitFor(() => duelStore.getState().log.some((e) => e.text.includes('翻开确认')));
  assert('confirmCardsLogged', true);

  // ---- MSG_MISSED_EFFECT：原版 SysString 1622「错过时点」 ----
  eventBus.emit('duel:missed_effect', { card: { c: 0, l: 0x4, s: 0 }, code: 89631139 });
  await waitFor(() => duelStore.getState().log.some((e) => e.text.includes('错过') && e.text.includes('时点')));
  assert('missedEffectLogged', true);

  // ---- MSG_MATCH_KILL / MSG_FIELD_DISABLED：状态入 store ----
  eventBus.emit('duel:match_kill', { code: 89631139 });
  await waitFor(() => duelStore.getState().matchKill === 89631139);
  assert('matchKillStored', true);
  eventBus.emit('duel:field_disabled', { zones: 0x0f });
  await waitFor(() => duelStore.getState().fieldDisabled === 0x0f);
  assert('fieldDisabledStored', true);

  // ---- MSG_SHUFFLE_HAND：本方按新序重建手牌；对方的归零牌序不动本方手牌 ----
  eventBus.emit('duel:shuffle_hand', { player: 0, cards: [46986414, 89631139, 123456] });
  await waitFor(() => duelStore.getState().hand.length === 3
    && duelStore.getState().hand[0].code === 46986414);
  assert('shuffleHandMineReordered', true);
  eventBus.emit('duel:shuffle_hand', { player: 1, cards: [0, 0, 0, 0] });
  await waitFor(() => duelStore.getState().hand.length === 3);
  assert('shuffleHandOpponentIgnored', duelStore.getState().hand[0].code === 46986414);

  // ---- MSG_UPDATE_CARD：query blob 合并进 store.board（表示形式翻转） ----
  eventBus.emit('duel:set', { code: 89631139, cc: 0, cl: 0x4, cs: 2, cp: 0x8 });
  await waitFor(() => duelStore.getState().board[0].mzone[2]?.code === 89631139);
  eventBus.emit('duel:update_card', {
    player: 0, location: 0x4, sequence: 2,
    code: 89631139, position: { c: 0, l: 0x4, s: 2, p: 0x1 },
  });
  await waitFor(() => duelStore.getState().board[0].mzone[2]?.pos === 0x1);
  assert('updateCardMerged', duelStore.getState().board[0].mzone[2].pos === 0x1,
    JSON.stringify(duelStore.getState().board[0].mzone[2]));

  // ---- MSG_BATTLE：结算体不打崩 3D 层 ----
  eventBus.emit('duel:battle', {
    attacker: { c: 0, l: 0x4, s: 0 }, attackerATK: 3000, attackerDEF: 2500, attackerDirect: false,
    target: { c: 1, l: 0x4, s: 0 }, targetATK: 1900, targetDEF: 2100, targetDirect: false,
  });
  assert('battleNoCrash', true);

  // ---- MSG_HINT：HINT_EVENT 文本经 ResolveDesc 显示在提示条 ----
  WailsBridge.resolveDesc = async (id) => (id === 777 ? '效果提示文本' : '');
  eventBus.emit('duel:hint', { type: 1, player: 0, data: 777 });
  await waitFor(() => duelStore.getState().hint === '效果提示文本');
  assert('engineHintShown', hintBar().style.display === 'block'
    && hintBar().innerText.includes('效果提示文本'));
  WailsBridge.resolveDesc = async () => '';

  // ---- 波 4：观战者 btnLeaveGame（duelclient.cpp:653-656）+ 队友投降高亮 ----
  assert('duelistShowsSurrender', !!document.getElementById('surrender-btn'));
  eventBus.emit('stoc:teammate_surrender', {});
  await waitFor(() => document.getElementById('surrender-btn').textContent.includes('1/2'));
  assert('teammateSurrenderLabel', true);
  // MSG_START playertype 高 4 位非 0 = 观战者视角（duelclient.cpp:1269-1270）
  eventBus.emit('duel:start', { playerType: 0x10, lp0: 8000, lp1: 8000 });
  await waitFor(() => !!document.getElementById('leave-game-btn'));
  assert('observerShowsLeaveGame', !document.getElementById('surrender-btn')
    && document.getElementById('leave-game-btn').textContent === '离开',
    document.getElementById('leave-game-btn') && document.getElementById('leave-game-btn').textContent);
  // 队友投降时 btnLeaveGame 常亮黄框（drawing.cpp:579-580 DrawSelectionLine）
  assert('observerLeaveGlow', document.getElementById('leave-game-btn').classList.contains('btn-hint-glow'));
  document.getElementById('leave-game-btn').click();
  await waitFor(() => leaveSends.length === 1 && observerLeaves.length === 1);
  assert('observerLeaveSends', leaveSends.length === 1 && observerLeaves.length === 1);

  window.__promptsSmoke.checks = checks;
  window.__promptsSmoke.ready = true;
};

run().catch((err) => {
  // 保留 fatal 前已写入的断言，便于定位卡在哪个 waitFor
  window.__promptsSmoke.checks.fatal = String((err && err.stack) || err);
  window.__promptsSmoke.ready = true;
});
