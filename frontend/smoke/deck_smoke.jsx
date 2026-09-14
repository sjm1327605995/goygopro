// Headless smoke for the P7 deck builder + Wave D (card-group editor completion):
// card-image chips, filter passthrough (race/attr/level/atk/def → Go CardFilter),
// result sorting + count, ×N same-name badge, side-deck 15 cap, dblclick zoom
// overlay, unsaved-changes guard. getCardImage is mocked with a data URL so the
// <img> branch is exercised without network access.
import { createRoot } from 'react-dom/client';
import React from 'react';
import { WailsBridge } from '../src/wails_bridge.ts';
import '../css/style.css';
import DeckBuilder from '../src/components/DeckBuilder.tsx';

const root = createRoot(document.getElementById('root'));
root.render(<DeckBuilder onNavigate={() => {}} />);

window.__deckSmoke = { checks: {}, ready: false };
const checks = window.__deckSmoke.checks;
const record = (name, ok) => { checks[name] = ok; };

// 1x1 cyan GIF as a stand-in card art.
const FAKE_PIC = 'data:image/gif;base64,R0lGODlhAQABAIAAAAUEBAAAACwAAAAAAQABAAACAkQBADs=';
WailsBridge.getCardImage = async (code) => ({ url: FAKE_PIC, full: false });
// 卡信息按 code 给固定值：排序冒烟依赖 type/attack，计数归并依赖无 alias
const CARD_INFO = {
  89631139: { name: 'Blue-Eyes', type: 0x11, attack: 3000, defense: 2500 },
  46986414: { name: 'Dark Magician', type: 0x11, attack: 2500, defense: 2100 },
  38033121: { name: 'Ancient Gear', type: 0x11, attack: 1800, defense: 500 },
  84013237: { name: 'Fusion Monster', type: 0x41, attack: 0, defense: 0 },
  5318639: { name: 'Trap Card', type: 0x4, attack: 0, defense: 0 },
};
WailsBridge.getCard = async (code) => {
  // 与真实现一致：结果写入 _cardCache（排序按钮从缓存读 type/attack）
  const info = CARD_INFO[code]
    ? { code, desc: 'test', ...CARD_INFO[code] }
    : { code, name: 'Card ' + code, type: 0x11, attack: 1000, defense: 500, desc: 'test' };
  WailsBridge._cardCache.set(code, info);
  return info;
};
// 故意乱序：陷阱排最前、高攻怪兽在最后 —— 验证排序（怪兽按攻↓，魔法/陷阱殿后）
const SEARCH_RESULTS = [
  { code: 44095762, name: 'Mirror Force', type: 0x4, attack: 0, defense: 0, desc: '' },
  { code: 46986414, name: 'Dark Magician', type: 0x11, attack: 2500, defense: 2100, desc: '' },
  { code: 89631139, name: 'Blue-Eyes', type: 0x11, attack: 3000, defense: 2500, desc: '' },
];
const searchFilters = [];
WailsBridge.searchCards = async (filter) => { searchFilters.push({ ...filter }); return SEARCH_RESULTS; };
// 相对路径名（'/' = 分类层级），与 Go ListDecks 递归返回一致
WailsBridge.listDecks = async () => ['Alpha', 'Beta', 'Tournament/TestDeck'];
WailsBridge.loadDeck = async (name) => ({
  name: name.split('/').pop(), // Go LoadDeck 的 Name = 文件名（去目录）
  main: [89631139, 46986414, 38033121],
  extra: [84013237],
  side: [5318639],
});
WailsBridge.saveDeck = async (deck) => { saveSends.push(deck); return true; };
const saveSends = [];
const deleteReqs = [];
WailsBridge.deleteDeck = async (name) => { deleteReqs.push(name); return { success: true }; };
const confirmCalls = [];
window.confirm = (msg) => { confirmCalls.push(String(msg)); return true; };
const alertMsgs = [];
window.alert = (msg) => { alertMsgs.push(String(msg)); };

const waitFor = (predicate, timeoutMs = 5000) => new Promise((resolve, reject) => {
  const started = Date.now();
  const tick = () => {
    if (predicate()) return resolve();
    if (Date.now() - started > timeoutMs) return reject(new Error('waitFor timeout'));
    setTimeout(tick, 25);
  };
  tick();
});

// React 受控输入必须走原生 value setter 再派发事件
const setInputValue = (el, value) => {
  const proto = el.tagName === 'SELECT' ? window.HTMLSelectElement.prototype : window.HTMLInputElement.prototype;
  Object.getOwnPropertyDescriptor(proto, 'value').set.call(el, value);
  el.dispatchEvent(new Event(el.tagName === 'SELECT' ? 'change' : 'input', { bubbles: true }));
};

const mainGrid = () => document.querySelectorAll('.deck-grid')[0];
const sideGrid = () => document.querySelectorAll('.deck-grid')[2];

(async () => {
  try {
    await waitFor(() => !!document.getElementById('deck-screen'));
    await waitFor(() => document.querySelectorAll('.deck-grid .deck-card-chip').length > 0);

    // 卡图格子：每个 chip 渲染 <img>（mock 卡图），比例锁定 59x86。
    const chips = [...document.querySelectorAll('.deck-grid .deck-card-chip')];
    record('chips-rendered', chips.length === 5); // 3 main + 1 extra + 1 side
    await waitFor(() => chips.every((c) => c.querySelector('img'))); // 卡图为异步 promise
    record('chips-show-card-images', chips.every((c) => c.querySelector('img') && c.querySelector('img').src === FAKE_PIC));
    record('chip-aspect-ratio', /59\s*\/\s*86/.test(getComputedStyle(chips[0]).aspectRatio || ''));

    // 过滤控件齐备：种族/属性下拉由 constants.ts RACES/ATTRS 驱动
    const raceSel = document.getElementById('deck-filter-race');
    const attrSel = document.getElementById('deck-filter-attr');
    record('filter-controls-exist', !!raceSel && !!attrSel);
    record('race-options-populated', raceSel.options.length > 20);
    record('attr-options-populated', attrSel.options.length >= 7);

    // 搜索 → 过滤字段透传（race 走到 CardFilter）
    setInputValue(document.getElementById('deck-search-input'), 'dragon');
    setInputValue(raceSel, '512'); // 0x200 龙
    const searchBtn = [...document.querySelectorAll('.deck-search-panel .btn')].find((b) => b.textContent.includes('搜索'));
    searchBtn.click();
    await waitFor(() => document.querySelectorAll('.search-results-grid .deck-card-chip').length > 0);
    record('filter-passthrough', searchFilters.length > 0
      && searchFilters[0].keyword === 'dragon'
      && searchFilters[0].race === 512
      && searchFilters[0].type === 0);

    // 排序 + 计数：怪兽按攻↓ 在前（89631139 第一个），结果数显示在标题行
    const results = [...document.querySelectorAll('.search-results-grid .deck-card-chip')];
    record('results-sorted', results.length === 3 && results[0].title.startsWith('Blue-Eyes'));
    record('result-count-shown', document.getElementById('deck-result-count').textContent.includes('3 张'));

    // 悬停进左侧详情：#deck-inspector-pic 现在是 <img>（不再是从未绘制的 canvas）
    // React 的 onMouseEnter 由原生 mouseover 委托实现，直接派发 mouseenter 无效
    results[0].dispatchEvent(new MouseEvent('mouseover', { bubbles: true }));
    await waitFor(() => {
      const pic = document.getElementById('deck-inspector-pic');
      return pic && pic.tagName === 'IMG' && pic.src === FAKE_PIC;
    });
    record('inspector-shows-image', true);

    // 点击加入主卡组 ×2 → 同铭 ×N 角标（mock 无 alias，按卡号归并）。
    // 两次点击间要隔开一拍，否则第二次用的是旧闭包里的 currentDeck。
    const clickN = async (el, n) => {
      for (let i = 0; i < n; i++) {
        el.click();
        await new Promise((r) => setTimeout(r, 25));
      }
    };
    await clickN(results[0], 2);
    await waitFor(() => mainGrid().querySelectorAll('.deck-card-chip').length === 5);
    await waitFor(() => mainGrid().querySelector('.deck-count-badge'));
    // 初始 main 已含 1 张 89631139 → 归并后 3 个 chip 全部 ×3
    record('count-badge-x2', mainGrid().querySelectorAll('.deck-count-badge').length === 3
      && [...mainGrid().querySelectorAll('.deck-count-badge')].every((b) => b.textContent === '×3'));

    // 双击看大图遮罩
    mainGrid().querySelector('.deck-card-chip').dispatchEvent(new MouseEvent('dblclick', { bubbles: true }));
    await waitFor(() => document.getElementById('deck-zoom-overlay'));
    record('zoom-overlay-opens', !!document.querySelector('#deck-zoom-overlay img'));
    document.getElementById('deck-zoom-overlay').click();
    await waitFor(() => !document.getElementById('deck-zoom-overlay'));
    record('zoom-overlay-closes', true);

    // 副卡组 15 上限：切「加入副卡组」后连点（每拍一次），第 16 张触发告警
    setInputValue(document.getElementById('deck-add-target'), 'side');
    await clickN(results[2], 16); // 陷阱卡，不会进额外
    await waitFor(() => alertMsgs.some((m) => m.includes('副卡组已满')));
    record('side-cap-15', sideGrid().querySelectorAll('.deck-card-chip').length === 15
      && alertMsgs.some((m) => m.includes('副卡组已满（最多 15 张）')));

    // 清空条件按钮：重置全部过滤控件
    const clearBtn = document.getElementById('deck-filter-clear');
    clearBtn.click();
    await waitFor(() => document.getElementById('deck-search-input').value === '' && raceSel.value === '0');
    record('clear-filters', true);

    // 删除卡组：走 confirm → DeleteDeck → 列表刷新。
    const deleteBtn = [...document.querySelectorAll('.deck-header .btn')].find((b) => b.textContent.includes('删除卡组'));
    record('delete-btn-exists', !!deleteBtn);
    deleteBtn.click();
    await waitFor(() => deleteReqs.length === 1);
    record('delete-deck-wired', deleteReqs[0] === 'Alpha');

    // 未存保护：此轮有过修改（上面加了卡），新建卡组前应弹确认
    const newBtn = [...document.querySelectorAll('.deck-header .btn')].find((b) => b.textContent.includes('新建卡组'));
    newBtn.click();
    record('unsaved-guard-confirm', confirmCalls.some((m) => m.includes('未保存')));

    // ---- 波 G：卡组分类（gframe cbDBCategory：未分类 + ./deck/ 子目录） ----
    const catSel = document.getElementById('deck-category-select');
    const deckSel = document.getElementById('deck-select');
    record('category-select-exists', !!catSel && !!deckSel
      && [...catSel.options].map((o) => o.value).join(',') === ',Tournament');
    // 默认未分类：卡组下拉只列根目录的 Alpha/Beta，分类卡组被过滤掉
    record('default-category-filters', [...deckSel.options].every((o) => !o.value.includes('/')));

    // 切到 Tournament 分类 → 卡组下拉只显示 TestDeck（值为完整相对名）
    setInputValue(catSel, 'Tournament');
    await waitFor(() => deckSel.options.length === 1 && deckSel.options[0].value === 'Tournament/TestDeck');
    record('category-switch-filters', deckSel.options[0].textContent === 'TestDeck');

    // 加载分类卡组 → deckName 显示文件名（Go LoadDeck Name=basename）
    setInputValue(deckSel, 'Tournament/TestDeck');
    await waitFor(() => [...document.querySelectorAll('.deck-header input')].some((i) => i.value === 'TestDeck'));
    record('category-load-basename', true);

    // 分类下保存 → 完整相对名带分类前缀（GetCategoryPath 语义）
    [...document.querySelectorAll('.deck-header .btn')].find((b) => b.textContent.includes('保存卡组')).click();
    await waitFor(() => saveSends.length === 1);
    record('save-into-category', saveSends[0].name === 'Tournament/TestDeck',
      JSON.stringify(saveSends[0] && saveSends[0].name));

    // ---- 波 G-2：运算符过滤 / 效果类型 / 链接箭头 / 编辑器三键 ----
    const lastFilter = () => searchFilters[searchFilters.length - 1];
    const findSearchBtn = () => [...document.querySelectorAll('.deck-search-panel .btn')]
      .find((b) => b.textContent.includes('搜索'));

    // 运算符字符串输入（gframe parse_filter 语法）透传给 CardFilter
    setInputValue(document.getElementById('deck-filter-atk'), '>=2500');
    setInputValue(document.getElementById('deck-filter-star'), '8');
    findSearchBtn().click();
    await waitFor(() => searchFilters.length >= 2);
    record('operator-passthrough', lastFilter().atkFilter === '>=2500'
      && lastFilter().levelFilter === '8' && lastFilter().multiKeywords === 1);

    // 效果过滤面板：32 复选框（SysString 1100-1131），勾选汇总 effect 位掩码
    document.getElementById('deck-filter-effect-btn').click();
    await waitFor(() => document.getElementById('deck-effect-panel'));
    const effectPanel = document.getElementById('deck-effect-panel');
    record('effect-panel-32-boxes', effectPanel.querySelectorAll('input[type=checkbox]').length === 32);
    const boxes = effectPanel.querySelectorAll('input[type=checkbox]');
    boxes[0].click(); // 魔陷破坏 = bit0
    boxes[1].click(); // 怪兽破坏 = bit1
    await new Promise((r) => setTimeout(r, 50)); // 等 React 受控勾选落地
    findSearchBtn().click();
    await waitFor(() => searchFilters.length >= 3);
    record('effect-mask-passthrough', lastFilter().effect === 3, String(lastFilter().effect));
    // 确定 = BUTTON_CATEGORY_OK：收掩码并关面板
    [...effectPanel.querySelectorAll('button')].find((b) => b.textContent === '确定').click();
    await waitFor(() => !document.getElementById('deck-effect-panel'));
    record('effect-panel-ok-closes', true);

    // 链接箭头面板：↖(0x40) + ↓(0x2) 汇成 0x42
    const arrows = [...document.getElementById('deck-linkmarks-panel').querySelectorAll('button')];
    arrows[0].click();
    arrows.find((b) => b.textContent === '↓').click();
    await new Promise((r) => setTimeout(r, 50));
    findSearchBtn().click();
    await waitFor(() => searchFilters.length >= 4);
    record('linkmarks-passthrough', lastFilter().linkMarks === 0x42, String(lastFilter().linkMarks));

    // 清空条件同时重置运算符/效果/箭头
    clearBtn.click();
    await waitFor(() => document.getElementById('deck-filter-atk').value === '');
    record('clear-resets-new-filters', arrows.every((b) => !b.className.includes('btn-gold')));

    // 编辑器三键存在（deck_con.cpp:172-191）
    const deckClearBtn = document.getElementById('deck-clear-btn');
    const deckSortBtn = document.getElementById('deck-sort-btn');
    const deckShuffleBtn = document.getElementById('deck-shuffle-btn');
    record('editor-buttons-exist', !!deckClearBtn && !!deckSortBtn && !!deckShuffleBtn);

    // 洗牌：随机重排 + 标脏（标题带 *）
    deckShuffleBtn.click();
    await waitFor(() => document.querySelector('#deck-screen h2').textContent.includes('*'));
    record('shuffle-deck-dirties', true);

    // 排序：deck_sort_lv 怪兽按攻↓（3000 → 2500 → 1800）
    deckSortBtn.click();
    const sortedTitles = () => [...mainGrid().querySelectorAll('.deck-card-chip')].map((c) => c.title);
    await waitFor(() => {
      const ts = sortedTitles();
      return ts.length === 3 && ts[0].startsWith('Blue-Eyes')
        && ts[1].startsWith('Dark Magician') && ts[2].startsWith('Ancient Gear');
    });
    record('sort-deck-atk-desc', true, sortedTitles().join('|'));

    // 清空卡组：confirm（SysString 1339）→ 三区全空
    deckClearBtn.click();
    await waitFor(() => confirmCalls.some((m) => m.includes('清空正在编辑的卡组')));
    await waitFor(() => mainGrid().querySelectorAll('.deck-card-chip').length === 0);
    record('clear-deck-empties', sideGrid().querySelectorAll('.deck-card-chip').length === 0);

    record('no-fatal', true);
  } catch (err) {
    checks.fatalMsg = String(err && err.stack || err);
    record('fatal', false);
  } finally {
    window.__deckSmoke.ready = true;
  }
})();
