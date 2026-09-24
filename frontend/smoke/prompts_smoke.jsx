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
import { settingsStore } from '../src/domain/settings.ts';
import PromptHost from '../src/components/PromptHost.tsx';
import HintBar from '../src/components/HintBar.tsx';
import CardPreviewPanel from '../src/components/CardPreviewPanel.tsx';
import RightControls from '../src/components/RightControls.tsx';
import BattleOverlay from '../src/components/BattleOverlay.tsx';
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
WailsBridge.respondSelectPlaces = (places) => { responses.push({ kind: 'places', places: [...places] }); };
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
    <CardPreviewPanel />
    <RightControls onLeaveObserver={() => observerLeaves.push(1)} />
    <BattleOverlay />
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
// 日志收在左侧预览面板的「消息记录」页签（右侧 LogDrawer 已删）：Radix Tabs
// 触发用 mousedown 激活，需先点开页签内容才挂载。
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
  await waitFor(() => overlay() && document.getElementById('hint-bar'));
  await openLogTab();

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
  // select-list first, then the unselect list（非场上来源 → 纯弹窗路径）----
  eventBus.emit('duel:select_unselect', {
    player: 0, finishable: false, cancelable: true, min: 1, max: 1,
    cards: [{ code: 21, c: 0, l: 0x2, s: 0 }],
    unselectList: [{ code: 22, c: 0, l: 0x2, s: 1 }]
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
  assert('tossCoinLogged', document.getElementById('duel-log-list').innerText.includes('正面、反面'));

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

  // ---- announce_card 实时过滤（原版 ebANCard → UpdateDeclarableList，
  // client_field.cpp:1535-1569）：卡名包含匹配 + 卡号片段匹配，候选数据走
  // searchCards 卡库查询通道，有候选清单时与清单求交 ----
  eventBus.emit('duel:announce_card', {
    player: 0, options: [89631139, 46986414],
    candidates: [89631139, 46986414], decodable: true,
  });
  await waitForModal('#announce-card-filter');
  assert('announceFilterShown', overlay().querySelectorAll('.cand-card').length === 2,
    `cands=${overlay().querySelectorAll('.cand-card').length}`);
  const setFilterVal = (v) => {
    const el = overlay().querySelector('#announce-card-filter');
    Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set.call(el, v);
    el.dispatchEvent(new Event('input', { bubbles: true }));
  };
  // 卡名包含匹配：'dark' 命中 Dark Magician（交集后 Blue-Eyes 被滤掉）
  setFilterVal('dark');
  await waitFor(() => overlay().querySelectorAll('.cand-card').length === 1);
  assert('announceFilterByName', overlay().querySelector('.cand-card .card-tile-name')
    .innerText.includes('Dark Magician'), overlay().querySelector('.cand-card').innerText);
  // 卡号片段匹配：'8963' 命中 Blue-Eyes（卡名查询无结果也保留）
  setFilterVal('8963');
  await waitFor(() => overlay().querySelectorAll('.cand-card').length === 1
    && overlay().querySelector('.cand-card .card-tile-name').innerText.includes('Blue-Eyes'));
  assert('announceFilterByCode', true, overlay().querySelector('.cand-card').innerText);
  // 清空过滤词 → 候选全量恢复
  setFilterVal('');
  await waitFor(() => overlay().querySelectorAll('.cand-card').length === 2);
  assert('announceFilterClearRestores', true);
  overlay().querySelector('.cand-card').click();
  r = await waitResp();
  await waitModalClosed();
  assert('announceFilterPickResponse', r.kind === 'I' && r.v === 89631139, JSON.stringify(r));

  // 无候选（decodable=false）也可按卡名检索点选，不再只有裸输卡号一条路
  eventBus.emit('duel:announce_card', {
    player: 0, options: [0x51, 0x40000102, 0x40000007], candidates: [], decodable: false,
  });
  await waitForModal('#announce-card-filter');
  setFilterVal('pot');
  await waitFor(() => overlay().querySelectorAll('.cand-card').length >= 1);
  assert('announceSearchFreeInput', overlay().querySelector('.cand-card .card-tile-name')
    .innerText.includes('Pot of Greed'), overlay().querySelector('.cand-card').innerText);
  overlay().querySelector('.cand-card').click();
  r = await waitResp();
  await waitModalClosed();
  assert('announceSearchPickResponse', r.kind === 'I' && r.v === 55144522, JSON.stringify(r));

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
  // 选满 max=1 立即自动应答（原版 event_handler.cpp:1389：selected>=select_max
  // 即 SendResponse，无需再点确定）
  tile.click();
  r = await waitResp();
  await waitModalClosed();
  assert('singleCardSelectResponse', r.kind === 'card' && JSON.stringify(r.indices) === '[0]',
    JSON.stringify(r));

  // ---- HINT_SELECTMSG（duelclient.cpp:1097/1601）：select_hint 作为
  // select_card 弹窗标题（格式 `提示(min-max)`），用后消费 ----
  eventBus.emit('duel:hint', { type: 3, player: 0, data: 502 }); // 系统串「请选择要破坏的卡」
  assert('selectHintStored', duelStore.getState().selectHint === 502);
  eventBus.emit('duel:select_card', {
    player: 0, cancelable: false, min: 1, max: 2,
    cards: [{ code: 89631139, c: 0, l: 0x2, s: 0 }],
  });
  await waitForModal('.card-tile');
  await waitFor(() => overlay().querySelector('.modal-title').innerText.includes('请选择要破坏的卡'));
  assert('selectCardUsesSelectHint', true, overlay().querySelector('.modal-title').innerText);
  assert('selectHintConsumed', duelStore.getState().selectHint === null);
  assert('selectHintTextArmed', (duelStore.getState().selectHintText || '').includes('请选择要破坏的卡(1-2)'),
    String(duelStore.getState().selectHintText));
  // 只有 1 张可选且已达 min：选满全部可选卡同样立即自动应答
  // （原版 event_handler.cpp:1394-1397：sel>=select_min 且已选=可选总数即应答）
  overlay().querySelectorAll('.card-tile')[0].click();
  r = await waitResp();
  await waitModalClosed();
  assert('selectHintCardResponse', r.kind === 'card' && JSON.stringify(r.indices) === '[0]', JSON.stringify(r));
  assert('selectHintTextDisarmed', duelStore.getState().selectHintText === null);

  // ---- HINT_SELECTMSG → announce_number 标题（原版 SysString 565 让位）----
  eventBus.emit('duel:hint', { type: 3, player: 0, data: 567 }); // 「请宣言一个等级」
  eventBus.emit('duel:announce_number', { player: 0, options: [1, 2, 3] });
  await waitForModal('.num-opt');
  assert('announceNumberUsesSelectHint', overlay().querySelector('.modal-title').innerText.includes('请宣言一个等级'),
    overlay().querySelector('.modal-title').innerText);
  overlay().querySelectorAll('.num-opt')[0].click();
  r = await waitResp();
  await waitModalClosed();
  assert('announceNumberHintResponse', r.kind === 'I' && r.v === 0, JSON.stringify(r));

  // ---- HINT 宣言日志（reducer：SysString 1511/1512）----
  eventBus.emit('duel:hint', { type: 9, player: 1, data: 7 });
  eventBus.emit('duel:hint', { type: 6, player: 1, data: 0x2000 }); // 龙族
  const declLog = duelStore.getState().log.map((e) => e.text);
  assert('hintNumberLogged', declLog.some((t) => t.includes('玩家选择了：7') || (t.includes('7') && t.includes('选择'))),
    JSON.stringify(declLog.slice(-3)));
  assert('hintRaceLogged', declLog.some((t) => t.includes('宣言') && t.includes('龙')),
    JSON.stringify(declLog.slice(-3)));

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

  // ---- swap_yes_no_button：开时是/否按钮交换左右（game.cpp SwapYesNoButtons）----
  eventBus.emit('duel:select_yesno', { player: 0, code: 89631139, loc: 0x4, desc: 0 });
  await waitForModal('#modal-btn-yes');
  const btnRow = () => [...overlay().querySelectorAll('#modal-btn-yes, #modal-btn-no')].map((b) => b.id);
  assert('yesNoDefaultOrder', btnRow()[0] === 'modal-btn-no' && btnRow()[1] === 'modal-btn-yes',
    JSON.stringify(btnRow()));
  settingsStore.set('swap_yes_no_button', 1);
  await waitFor(() => btnRow()[0] === 'modal-btn-yes');
  assert('yesNoSwappedOrder', btnRow()[0] === 'modal-btn-yes' && btnRow()[1] === 'modal-btn-no',
    JSON.stringify(btnRow()));
  settingsStore.set('swap_yes_no_button', 0);
  await waitFor(() => btnRow()[0] === 'modal-btn-no');
  overlay().querySelector('#modal-btn-yes').click();
  r = await waitResp();
  await waitModalClosed();

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
  // 第 2 张选满 max=2 → 立即自动应答（不再点确定）
  overlay().querySelectorAll('.card-tile')[1].click();
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

  // ---- MSG_SELECT_PLACE 自动落点：automonsterpos=1 时怪兽区询问按原版
  // 优先级代答（duelclient.cpp:1856-1896：区内优先序 6,5,2,1,3,0,4）----
  settingsStore.set('automonsterpos', 1);
  eventBus.emit('duel:select_place', {
    player: 0, count: 1, flag: 0x8,
    zones: [
      { player: 0, loc: 0x04, seq: 2 },
      { player: 0, loc: 0x04, seq: 6 },
    ]
  });
  r = await waitResp();
  assert('autoPlacePriority', r.kind === 'place' && r.player === 0 && r.loc === 0x04 && r.seq === 6, JSON.stringify(r));

  // ---- autospellpos 默认 1：纯魔陷区询问自动代答 ----
  eventBus.emit('duel:select_place', {
    player: 0, count: 1, flag: 0x0,
    zones: [{ player: 0, loc: 0x08, seq: 3 }]
  });
  r = await waitResp();
  assert('autoPlaceSpellPos', r.kind === 'place' && r.loc === 0x08 && r.seq === 3, JSON.stringify(r));

  // ---- SELECT_DISFIELD 从不自动（即便 autospellpos=1）→ 进入点选模式 ----
  eventBus.emit('duel:select_place', {
    player: 0, count: 1, flag: 0x0, disfield: true,
    zones: [{ player: 1, loc: 0x08, seq: 2 }]
  });
  await waitFor(() => field.placeSelectMarks.length === 1 && manager.placeSelect);
  assert('disfieldNeverAuto', field.placeSelectMarks.length === 1
    && duelStore.getState().hint.includes('不能使用'), duelStore.getState().hint);
  // 不可取消的询问右键无效应答；显式结束进入下一个用例
  manager.onBoardRightClick(0, 0);
  assert('disfieldRightClickNoCancel', responses.length === respMark
    && !!manager.placeSelect, JSON.stringify(responses[responses.length - 1]));
  manager.endPlaceSelect();

  // ---- 手动点选模式（automonsterpos=0）：高亮布设 → 点格选中（琥珀实线）
  // → 再点同一格取消（回虚线）→ 选满自动应答 + 高亮/提示清除 ----
  settingsStore.set('automonsterpos', 0);
  eventBus.emit('duel:select_place', {
    player: 0, count: 2, flag: 0x8,
    zones: [{ player: 0, loc: 0x04, seq: 3 }, { player: 0, loc: 0x04, seq: 1 }]
  });
  await waitFor(() => field.placeSelectMarks.length === 2 && !!manager.placeSelect);
  assert('placeSelectHighlights', field.placeSelectMarks.length === 2
    && duelStore.getState().hint === '请选择', duelStore.getState().hint);
  manager.onPlaceZoneClick({ player: 0, loc: 0x04, seq: 3 });
  await waitFor(() => field.placeSelectMarks.find((m) => m.zone.seq === 3).mesh.userData.selected === true);
  assert('placeSelectToggleSelected', true);
  manager.onPlaceZoneClick({ player: 0, loc: 0x04, seq: 3 }); // 再点已选格 = 取消
  await waitFor(() => field.placeSelectMarks.every((m) => !m.mesh.userData.selected));
  assert('placeSelectToggleCanceled', responses.length === respMark, 'unexpected response');
  // 重新选中两格 → 选满即应答（seq 升序：1 在前）
  manager.onPlaceZoneClick({ player: 0, loc: 0x04, seq: 3 });
  manager.onPlaceZoneClick({ player: 0, loc: 0x04, seq: 1 });
  r = await waitResp();
  await waitFor(() => field.placeSelectMarks.length === 0 && !manager.placeSelect
    && duelStore.getState().hint === '');
  assert('placeSelectClickResponse', r.kind === 'places' && JSON.stringify(r.places) === '[0,4,1,0,4,3]', JSON.stringify(r));

  // ---- count>1 未满不应答：count=3 每次点击后都未应答；选满后按原版
  // 区域序（己 mzone→己 szone→对 mzone→对 szone，seq 升序）一次性应答 ----
  eventBus.emit('duel:select_place', {
    player: 0, count: 3, flag: 0x0,
    zones: [
      { player: 1, loc: 0x04, seq: 0 },
      { player: 0, loc: 0x08, seq: 1 },
      { player: 0, loc: 0x04, seq: 2 },
    ]
  });
  await waitFor(() => manager.placeSelect && field.placeSelectMarks.length === 3);
  manager.onPlaceZoneClick({ player: 1, loc: 0x04, seq: 0 });
  assert('multiPlaceOneNoResponse', responses.length === respMark, 'answered after 1 pick');
  manager.onPlaceZoneClick({ player: 0, loc: 0x08, seq: 1 });
  assert('multiPlaceTwoNoResponse', responses.length === respMark, 'answered after 2 picks');
  // 右键 = 撤销最近一次已选（szone seq1 退选，mzone seq0(对) 保留），未应答
  manager.onBoardRightClick(0, 0);
  await waitFor(() => manager.placeSelect.selected.length === 1
    && field.placeSelectMarks.every((m) => m.zone.loc !== 0x08 || m.zone.seq !== 1 || !m.mesh.userData.selected));
  assert('placeRightClickPopsLast', responses.length === respMark, 'answered after right-click');
  manager.onPlaceZoneClick({ player: 0, loc: 0x08, seq: 1 }); // 重选退选格
  manager.onPlaceZoneClick({ player: 0, loc: 0x04, seq: 2 });
  r = await waitResp();
  assert('multiPlaceOrderedResponse', r.kind === 'places'
    && JSON.stringify(r.places) === '[0,4,2,0,8,1,1,4,0]', JSON.stringify(r.places));

  // ---- count=0（可取消）询问：点击选位会应答，未选时右键回 [player,0,0]
  // （gframe btnCancelOrFinish → respbuf=[LocalPlayer(0),0,0]）----
  eventBus.emit('duel:select_place', {
    player: 0, count: 0, flag: 0x8,
    zones: [{ player: 0, loc: 0x04, seq: 0 }]
  });
  await waitFor(() => !!manager.placeSelect);
  manager.onBoardRightClick(0, 0);
  r = await waitResp();
  await waitFor(() => !manager.placeSelect);
  assert('placeCancelResponse', r.kind === 'place' && r.player === 0 && r.loc === 0 && r.seq === 0, JSON.stringify(r));

  // ---- HINT_ZONE（duelclient.cpp:1168）：位掩码 → 绿色区域高亮（与
  // select_place 蓝虚线区分）+ 逐格日志；0.8s 自动清除 ----
  eventBus.emit('duel:hint', { type: 11, player: 0, data: 0x5 }); // 本方 mzone seq0/seq2
  await waitFor(() => field.hintZoneMarks.length === 2);
  assert('hintZoneHighlights', field.hintZoneMarks.every((m) => m.userData.hintZone
    && m.userData.hintZone.player === 0 && m.userData.hintZone.loc === 0x04),
    JSON.stringify(field.hintZoneMarks.map((m) => m.userData.hintZone)));
  const zoneLog = duelStore.getState().log.map((e) => e.text);
  assert('hintZoneLogged', zoneLog.some((t) => t.includes('我方怪兽区(1)'))
    && zoneLog.some((t) => t.includes('我方怪兽区(3)')), JSON.stringify(zoneLog.slice(-4)));
  // 对方操作时高低 16 位互换：raw 低位 0x1 → 对方 mzone seq0
  eventBus.emit('duel:hint', { type: 11, player: 1, data: 0x1 });
  await waitFor(() => field.hintZoneMarks.length === 1
    && field.hintZoneMarks[0].userData.hintZone.player === 1);
  assert('hintZoneOpponentSwap', field.hintZoneMarks[0].userData.hintZone.seq === 0,
    JSON.stringify(field.hintZoneMarks[0].userData.hintZone));
  // 阶段切换立即清除（duel_manager new_phase 订阅）
  eventBus.emit('duel:new_phase', { phase: 0x04 });
  assert('hintZoneClearsOnPhase', field.hintZoneMarks.length === 0);
  // 自动清除：再来一个，0.8s 后消失
  eventBus.emit('duel:hint', { type: 11, player: 0, data: 0x100 }); // 本方 szone seq0
  await waitFor(() => field.hintZoneMarks.length === 1);
  await waitFor(() => field.hintZoneMarks.length === 0, 2000);
  assert('hintZoneAutoClears', true);

  // ---- 右键堆区 → ui:show_pile 快速查看（F1-F8 同源）；deck 无列表不发出 ----
  const pileEvents = [];
  const onShowPile = (d) => pileEvents.push(d);
  eventBus.on('ui:show_pile', onShowPile);
  manager.onPileRightClick(0, 'grave');
  manager.onPileRightClick(1, 'extra');
  manager.onPileRightClick(0, 'deck');
  eventBus.off('ui:show_pile', onShowPile);
  assert('pileRightClickShowsList', pileEvents.length === 2
    && pileEvents[0].pile === 'grave' && pileEvents[0].player === 0
    && pileEvents[1].pile === 'extra' && pileEvents[1].player === 1, JSON.stringify(pileEvents));
  settingsStore.set('automonsterpos', 0);

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

  // ---- MSG_MATCH_KILL / MSG_FIELD_DISABLED：状态入 store + 3D 白叉渲染
  // （drawing.cpp:210-241：每禁用格两条对角白线）----
  eventBus.emit('duel:match_kill', { code: 89631139 });
  await waitFor(() => duelStore.getState().matchKill === 89631139);
  assert('matchKillStored', true);
  eventBus.emit('duel:field_disabled', { zones: 0x0f });
  await waitFor(() => duelStore.getState().fieldDisabled === 0x0f
    && field.disabledMarks.length === 8, `marks=${field.disabledMarks.length}`);
  assert('fieldDisabledStored', true);
  assert('fieldDisabledCrosses', field.disabledMarks.length === 8
    && field.disabledMarks.every((l) => l.material.color.getHexString() === 'ffffff'),
    String(field.disabledMarks.length));
  // 白叉落在被禁用的格位上（0x0f = seat0 mzone seq0-3，各两条对角线）
  const lineStart = (i) => {
    const p = field.disabledMarks[i].geometry.attributes.position;
    return [p.getX(0), p.getZ(0)];
  };
  const near = (a, b) => Math.abs(a - b) < 0.01;
  const s0 = lineStart(0); // seq0 第一条对角线起点 (x-0.76, z-1.06)
  const s3 = lineStart(6); // seq3 的 X
  assert('fieldDisabledCrossPositions',
    near(s0[0], -4.5 - 0.76) && near(s0[1], 2.8 - 1.06)
    && near(s3[0], 2.1 - 0.76) && near(s3[1], 2.8 - 1.06),
    JSON.stringify([s0, s3]));
  eventBus.emit('duel:field_disabled', { zones: 0 });
  await waitFor(() => field.disabledMarks.length === 0);
  assert('fieldDisabledCleared', true);

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

  // ---- MSG_BATTLE：攻防对撞浮层（双方 ATK/DEF + 战破旗标结果行），2.4s 淡出 ----
  eventBus.emit('duel:set', { code: 89631139, cc: 0, cl: 0x4, cs: 0, cp: 0x1 });
  eventBus.emit('duel:set', { code: 46986414, cc: 1, cl: 0x4, cs: 0, cp: 0x4 });
  await waitFor(() => duelStore.getState().board[1].mzone[0]?.code === 46986414);
  eventBus.emit('duel:battle', {
    attacker: { c: 0, l: 0x4, s: 0 }, attackerATK: 3000, attackerDEF: 2500, attackerDestroyed: false,
    target: { c: 1, l: 0x4, s: 0 }, targetATK: 2500, targetDEF: 2100, targetDestroyed: true,
  });
  await waitFor(() => !!document.getElementById('battle-overlay'));
  const battleEl = document.getElementById('battle-overlay');
  assert('battleOverlayShown', battleEl.innerText.includes('3000')
    && battleEl.innerText.includes('2100'), battleEl.innerText);
  await waitFor(() => battleEl.innerText.includes('Blue-Eyes'));
  assert('battleOverlayNames', battleEl.innerText.includes('Blue-Eyes')
    && battleEl.innerText.includes('Dark Magician'), battleEl.innerText);
  assert('battleOverlayResult', battleEl.innerText.includes('战斗破坏'), battleEl.innerText);
  await waitFor(() => !document.getElementById('battle-overlay'), 6000);
  assert('battleOverlayFades', !document.getElementById('battle-overlay'));

  // ---- MSG_HINT：HINT_EVENT 文本经 ResolveDesc 显示在提示条 ----
  WailsBridge.resolveDesc = async (id) => (id === 777 ? '效果提示文本' : '');
  eventBus.emit('duel:hint', { type: 1, player: 0, data: 777 });
  await waitFor(() => duelStore.getState().hint === '效果提示文本');
  assert('engineHintShown', hintBar().style.display === 'block'
    && hintBar().innerText.includes('效果提示文本'));
  WailsBridge.resolveDesc = async () => '';

  // ---- select_card 场上点选（原版 drawing.cpp:431-436 黄框 + 直接点卡）：
  // 全部候选在场上（mzone/szone）→ 不弹窗，3D 高亮 + #card-select-bar；
  // 点选/再点取消与弹窗语义一致，选满 max 立即自动应答 ----
  field.placeCard(0, 'mzone', 3, 89631139, null, 0x1);
  field.placeCard(1, 'mzone', 1, 46986414, null, 0x1);
  eventBus.emit('duel:select_card', {
    player: 0, cancelable: true, min: 1, max: 2,
    cards: [
      { code: 89631139, c: 0, l: 0x4, s: 3 },
      { code: 46986414, c: 1, l: 0x4, s: 1 },
    ],
  });
  await waitFor(() => field.cardSelectMarks.length === 2);
  assert('fieldSelectNoModal', !overlay().className.includes('active'), overlay().className);
  assert('fieldSelectBarShown', !!document.getElementById('card-select-bar')
    && document.getElementById('card-select-bar-text').innerText.includes('(1-2)'),
    document.getElementById('card-select-bar') && document.getElementById('card-select-bar').innerText);
  manager.onCardSelectPick(0);
  await waitFor(() => field.cardSelectMarks.find((m) => m.idx === 0).mesh.userData.selected === true);
  assert('fieldSelectToggleOn', document.getElementById('card-select-bar-count').innerText.includes('1 / 2'),
    document.getElementById('card-select-bar-count').innerText);
  manager.onCardSelectPick(0); // 再点同一张 = 取消
  await waitFor(() => field.cardSelectMarks.every((m) => !m.mesh.userData.selected));
  assert('fieldSelectToggleOff', responses.length === respMark, 'unexpected response');
  manager.onCardSelectPick(1);
  manager.onCardSelectPick(0); // 选满 max=2 → 自动应答（点击顺序 [1,0]）
  r = await waitResp();
  await waitFor(() => field.cardSelectMarks.length === 0 && !document.getElementById('card-select-bar'));
  assert('fieldSelectAutoRespond', r.kind === 'card' && JSON.stringify(r.indices) === '[1,0]', JSON.stringify(r));

  // ---- 混合来源（1 场上 + 1 手牌）：弹窗与场上高亮并存、选择集同步 ----
  eventBus.emit('duel:select_card', {
    player: 0, cancelable: false, min: 1, max: 2,
    cards: [
      { code: 89631139, c: 0, l: 0x4, s: 3 },
      { code: 89631139, c: 0, l: 0x2, s: 0 },
    ],
  });
  await waitForModal('.card-tile');
  assert('mixedSelectBothPaths', field.cardSelectMarks.length === 1
    && overlay().querySelectorAll('.card-tile').length === 2,
    `marks=${field.cardSelectMarks.length} tiles=${overlay().querySelectorAll('.card-tile').length}`);
  manager.onCardSelectPick(0); // 场上点选 → 弹窗里同名下标同步选中
  await waitFor(() => overlay().querySelectorAll('.card-tile')[0].classList.contains('selected'));
  assert('mixedFieldPickSyncsModal', document.getElementById('select-count-text').innerText.includes('1 / 2'),
    document.getElementById('select-count-text').innerText);
  overlay().querySelectorAll('.card-tile')[1].click(); // 弹窗再选一张 → 选满自动应答
  r = await waitResp();
  await waitModalClosed();
  assert('mixedSelectAutoRespond', r.kind === 'card' && JSON.stringify(r.indices) === '[0,1]', JSON.stringify(r));
  assert('mixedSelectMarksCleared', field.cardSelectMarks.length === 0);

  // ---- 纯场上可取消询问：#card-select-bar 的取消回 -1 ----
  eventBus.emit('duel:select_card', {
    player: 0, cancelable: true, min: 1, max: 1,
    cards: [{ code: 89631139, c: 0, l: 0x4, s: 3 }],
  });
  await waitFor(() => document.getElementById('card-select-bar-cancel'));
  document.getElementById('card-select-bar-cancel').click();
  r = await waitResp();
  await waitFor(() => !document.getElementById('card-select-bar'));
  assert('fieldSelectCancelResponse', r.kind === 'I' && r.v === -1, JSON.stringify(r));

  // ---- hide_hint_button：场形态选择条藏完成/取消按钮
  // （event_handler.cpp:2425 ShowCancelOrFinishButton）----
  eventBus.emit('duel:select_card', {
    player: 0, cancelable: true, min: 1, max: 2,
    cards: [
      { code: 89631139, c: 0, l: 0x4, s: 3 },
      { code: 46986414, c: 1, l: 0x4, s: 1 },
    ],
  });
  await waitFor(() => document.getElementById('card-select-bar-finish')
    && document.getElementById('card-select-bar-cancel'));
  assert('hintButtonsShownByDefault', true);
  settingsStore.set('hide_hint_button', 1);
  await waitFor(() => !!document.getElementById('card-select-bar')
    && !document.getElementById('card-select-bar-finish')
    && !document.getElementById('card-select-bar-cancel'));
  assert('hintButtonsHiddenWhenSet', true);
  settingsStore.set('hide_hint_button', 0);
  await waitFor(() => document.getElementById('card-select-bar-cancel'));
  document.getElementById('card-select-bar-cancel').click();
  r = await waitResp();
  await waitFor(() => !document.getElementById('card-select-bar'));

  // ---- select_unselect 场上点选：点中可选场卡立即应答其合并下标 ----
  eventBus.emit('duel:select_unselect', {
    player: 0, finishable: true, cancelable: false, min: 1, max: 1,
    cards: [{ code: 89631139, c: 0, l: 0x4, s: 3 }],
    unselectList: [{ code: 46986414, c: 1, l: 0x4, s: 1 }],
  });
  await waitFor(() => field.cardSelectMarks.length === 2);
  assert('unselectFieldNoModal', !overlay().className.includes('active'), overlay().className);
  manager.onCardSelectPick(1); // unselect 列表的场上卡 → 立即应答合并下标 1
  r = await waitResp();
  await waitFor(() => field.cardSelectMarks.length === 0);
  assert('unselectFieldPickResponds', r.kind === 'unselect' && r.index === 1, JSON.stringify(r));

  // ---- tribute（MSG_SELECT_TRIBUTE）保持原弹窗路径：不建选择态、不布高亮、
  // 不自动应答（原版 CheckSelectTribute 求和校验不在本项范围） ----
  eventBus.emit('duel:select_card', {
    player: 0, cancelable: false, min: 1, max: 1, tribute: true,
    cards: [{ code: 89631139, c: 0, l: 0x4, s: 3 }],
  });
  await waitForModal('.card-tile');
  assert('tributeStaysModal', field.cardSelectMarks.length === 0
    && duelStore.getState().cardSelect === null);
  overlay().querySelectorAll('.card-tile')[0].click();
  await waitFor(() => !overlay().querySelector('#modal-select-confirm').disabled);
  overlay().querySelector('#modal-select-confirm').click();
  r = await waitResp();
  await waitModalClosed();
  assert('tributeResponse', r.kind === 'card' && JSON.stringify(r.indices) === '[0]', JSON.stringify(r));

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

  // ---- btnSpectatorSwap（game.cpp:913 → DuelClient::SwapField）：交换视角
  // —— store 显示座翻转（后续事件按翻转后映射落位）+ 相机绕到另一侧 ----
  assert('observerShowsSwap', !!document.getElementById('spectator-swap-btn'));
  eventBus.emit('duel:lp_update', { player: 0, lp: 6000 });
  await waitFor(() => duelStore.getState().lp[0] === 6000);
  document.getElementById('spectator-swap-btn').click();
  await waitFor(() => duelStore.getState().viewSwapped === true);
  assert('swapFlipsDisplaySeats', duelStore.getState().lp[1] === 6000
    && duelStore.getState().lp[0] === 8000, JSON.stringify(duelStore.getState().lp));
  await waitFor(() => field.camera.position.z < 0, 3000);
  assert('swapFlipsCamera', true);
  // 翻转后 seat0 的事件落入显示座 1（换算连续）
  eventBus.emit('duel:lp_update', { player: 0, lp: 5000 });
  await waitFor(() => duelStore.getState().lp[1] === 5000);
  assert('swapKeepsEventMapping', duelStore.getState().lp[0] === 8000,
    JSON.stringify(duelStore.getState().lp));
  document.getElementById('spectator-swap-btn').click();
  await waitFor(() => duelStore.getState().viewSwapped === false);
  assert('swapBackRestores', duelStore.getState().lp[0] === 5000
    && duelStore.getState().lp[1] === 8000, JSON.stringify(duelStore.getState().lp));
  await waitFor(() => field.camera.position.z > 0, 3000);
  assert('swapCameraBack', true);

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
