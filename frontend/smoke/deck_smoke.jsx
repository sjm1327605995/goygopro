// Headless smoke for the P7 deck builder + Wave D (card-group editor completion):
// card-image chips, filter passthrough (race/attr/level/atk/def → Go CardFilter),
// result sorting + count, ×N same-name badge, side-deck 15 cap, dblclick zoom
// overlay, unsaved-changes guard. getCardImage is mocked with a data URL so the
// <img> branch is exercised without network access.
import UiRoot from '../src/ui/UiRoot.tsx';
import { createRoot } from 'react-dom/client';
import React from 'react';
import { WailsBridge } from '../src/wails_bridge.ts';
import { settingsStore } from '../src/domain/settings.ts';
import '../css/style.css';
import DeckBuilder from '../src/components/DeckBuilder.tsx';

const root = createRoot(document.getElementById('root'));
root.render(<UiRoot><DeckBuilder onNavigate={() => {}} /></UiRoot>);

window.__deckSmoke = { checks: {}, ready: false };
const checks = window.__deckSmoke.checks;
const record = (name, ok) => { checks[name] = ok; };

// 1x1 cyan GIF as a stand-in card art.
const FAKE_PIC = 'data:image/gif;base64,R0lGODlhAQABAIAAAAUEBAAAACwAAAAAAQABAAACAkQBADs=';
WailsBridge.getCardImage = async (code) => ({ url: FAKE_PIC, full: false });
// 卡信息按 code 给固定值：排序冒烟依赖 type/attack，计数归并依赖无 alias
const CARD_INFO = {
  89631139: { name: 'Blue-Eyes', type: 0x11, attack: 3000, defense: 2500, level: 8, race: 0x200, attribute: 0x10 },
  46986414: { name: 'Dark Magician', type: 0x11, attack: 2500, defense: 2100, level: 7, race: 0x200, attribute: 0x20 },
  38033121: { name: 'Ancient Gear', type: 0x11, attack: 1800, defense: 500, level: 5, race: 0x200, attribute: 0x20 },
  84013237: { name: 'Fusion Monster', type: 0x41, attack: 0, defense: 0, level: 8, race: 0x200, attribute: 0x20 },
  5318639: { name: 'Trap Card', type: 0x4, attack: 0, defense: 0 },
};
WailsBridge.getCard = async (code) => {
  // 与真实现一致：结果写入 _cardCache（排序按钮从缓存读 type/attack）
  const info = CARD_INFO[code]
    ? { code, desc: 'test', ...CARD_INFO[code] }
    : { code, name: 'Card ' + code, type: 0x11, attack: 1000, defense: 500, level: 4, race: 0x1, attribute: 0x1, desc: 'test' };
  WailsBridge._cardCache.set(code, info);
  return info;
};
// 故意乱序：陷阱排最前、高攻怪兽在最后 —— 验证排序（怪兽按攻↓，魔法/陷阱殿后）
const SEARCH_RESULTS = [
  { code: 44095762, name: 'Mirror Force', type: 0x4, attack: 0, defense: 0, desc: '' },
  { code: 46986414, name: 'Dark Magician', type: 0x11, attack: 2500, defense: 2100, level: 7, race: 0x200, attribute: 0x20, desc: '' },
  { code: 89631139, name: 'Blue-Eyes', type: 0x11, attack: 3000, defense: 2500, level: 8, race: 0x200, attribute: 0x10, desc: '' },
];
// 18 张互异陷阱卡，供副卡组 15 上限测试用（同名 3 张限制不干扰互异卡）
const ALL_TRAPS = Array.from({ length: 18 }, (_, i) => ({
  code: 90000000 + i, name: 'Trap ' + i, type: 0x4, attack: 0, defense: 0, desc: '',
}));
const searchFilters = [];
// 关键词含 'trap' 返回互异陷阱卡，否则返回固定三卡（保留排序/计数断言不变）
WailsBridge.searchCards = async (filter) => {
  searchFilters.push({ ...filter });
  return (filter.keyword && filter.keyword.includes('trap')) ? ALL_TRAPS : SEARCH_RESULTS;
};
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
  Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set.call(el, value);
  el.dispatchEvent(new Event('input', { bubbles: true }));
};

// ---- Radix Select（GfwSelect）驱动助手：trigger 靠 pointerdown 打开 ----
const openSelect = async (id) => {
  const t = document.getElementById(id);
  t.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, cancelable: true, button: 0, pointerId: 1, pointerType: 'mouse' }));
  t.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true, button: 0 }));
  t.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, cancelable: true, button: 0 }));
  t.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, button: 0 }));
  await waitFor(() => !!document.querySelector('[role="option"]'));
};
const closeSelect = async () => {
  document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }));
  await tryWaitFor(() => !document.querySelector('[role="option"]'));
};
// 打开下拉读出全部 option（GfwSelect 给每个 Item 输出 data-value），再 Escape 关闭
const readOptions = async (id) => {
  await openSelect(id);
  const opts = [...document.querySelectorAll('[role="option"]')]
    .map((o) => ({ value: o.getAttribute('data-value'), label: o.textContent.trim() }));
  await closeSelect();
  return opts;
};
const optionCount = async (id) => (await readOptions(id)).length;
// 打开下拉并按 data-value 点选（Radix Item 在 pointerup 上提交选中）
const selectOption = async (id, value) => {
  await openSelect(id);
  const opt = document.querySelector(`[role="option"][data-value="${CSS.escape(value)}"]`);
  if (!opt) throw new Error(`option not found: ${id}=${value}`);
  opt.dispatchEvent(new PointerEvent('pointerup', { bubbles: true, cancelable: true, button: 0, pointerId: 1, pointerType: 'mouse' }));
  opt.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, cancelable: true, button: 0 }));
  opt.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, button: 0 }));
  await waitFor(() => !document.querySelector('[role="option"]'));
};
// 软等待：到时返回 predicate 当前值而不是抛错（用于判定式断言）
const tryWaitFor = async (predicate, ms = 1500) => {
  const started = Date.now();
  while (!predicate() && Date.now() - started < ms) await new Promise((r) => setTimeout(r, 25));
  return predicate();
};

const mainGrid = () => document.querySelectorAll('.deck-grid')[0];
const sideGrid = () => document.querySelectorAll('.deck-grid')[2];

// The editor follows gframe mouse commands; real event ordering is also
// exercised separately by tools/ui_parity_e2e.cjs.
const idle=()=>new Promise(r=>setTimeout(r,30));
const right=el=>el.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true,button:2}));
const middle=el=>el.dispatchEvent(new MouseEvent('auxclick',{bubbles:true,cancelable:true,button:1}));
const drop=(source,target)=>{const dt=new DataTransfer();source.dispatchEvent(new DragEvent('dragstart',{bubbles:true,dataTransfer:dt}));target.dispatchEvent(new DragEvent('drop',{bubbles:true,cancelable:true,dataTransfer:dt}));};
const extraGrid=()=>document.querySelectorAll('.deck-grid')[1];
const mainCount=()=>mainGrid().querySelectorAll('.deck-card-chip').length;
const sideCount=()=>sideGrid().querySelectorAll('.deck-card-chip').length;
const searchBtn=()=>[...document.querySelectorAll('.deck-search-panel .btn')].find(b=>b.textContent.includes('搜索'));
const clearBtn=()=>document.getElementById('deck-filter-clear');
WailsBridge.listLFLists=async()=>[{hash:1,name:'Test LF'},{hash:0,name:'N/A'}];
WailsBridge.lfListContent=async()=>({46986414:2,5318639:1});
WailsBridge.listDeckCategories=async()=>['Tournament'];
(async()=>{try{
 await waitFor(()=>mainGrid());await waitFor(()=>mainCount()===3);await idle();
 const chips=[...document.querySelectorAll('.deck-grid .deck-card-chip')];
 await waitFor(()=>chips.every(c=>c.querySelector('img').src===FAKE_PIC));
 record('chips-rendered',chips.length===5);record('chips-show-images',true);
 record('chip-original-ratio',/44\s*\/\s*64/.test(getComputedStyle(chips[0]).aspectRatio));
 record('initial-deck-selected',document.getElementById('deck-select').textContent.includes('Alpha'));
 const initial=mainCount();mainGrid().firstElementChild.firstElementChild.click();await idle();record('leftclick-preserves-deck',mainCount()===initial);
 mainGrid().querySelector('.deck-card-chip').dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));
 await waitFor(()=>document.getElementById('deck-big-card-close'));record('big-card-controls',document.querySelectorAll('#deck-zoom-overlay button').length===4);
 document.getElementById('deck-big-card-close').click();await waitFor(()=>!document.getElementById('deck-zoom-overlay'));
 middle(extraGrid().querySelector('.deck-card-chip'));await waitFor(()=>extraGrid().querySelectorAll('.deck-card-chip').length===2);
 right(extraGrid().querySelector('.deck-card-chip'));await waitFor(()=>extraGrid().querySelectorAll('.deck-card-chip').length===1);record('middle-copy-right-delete',true);
 await selectOption('deck-filter-type1','1');await selectOption('deck-filter-race','512');
 setInputValue(document.getElementById('deck-search-input'),'dragon');await idle();searchBtn().click();
 await waitFor(()=>document.querySelectorAll('.deck-result-row').length===3);
 const filter=searchFilters.at(-1);record('filter-passthrough',filter.type===1&&filter.race===512&&filter.keyword==='dragon'&&filter.allResults);
 record('sort-four-modes',(await readOptions('deck-result-sort')).length===4);
 let results=[...document.querySelectorAll('.deck-result-row')];record('level-order',results[0].title==='Blue-Eyes');
 await selectOption('deck-result-sort','name');results=[...document.querySelectorAll('.deck-result-row')];record('name-order',results.map(c=>c.title).join(',')==='Blue-Eyes,Dark Magician,Mirror Force');
 results[0].dispatchEvent(new MouseEvent('mouseover',{bubbles:true}));await waitFor(()=>document.getElementById('deck-inspector-pic'));record('inspector-image',true);
 right(results[0]);await idle();right(results[0]);await waitFor(()=>mainCount()===5);right(results[0]);await idle();record('three-copy-limit',mainCount()===5);
 right(results[1]);await idle();const before=mainCount();right(results[1]);await idle();record('lflist-limit',before===6&&mainCount()===6);
 record('lflist-atlas',mainGrid().querySelector('.deck-limit-2')&&getComputedStyle(mainGrid().querySelector('.deck-limit-2')).backgroundImage.includes('lim'));
 drop(results[2],sideGrid());await waitFor(()=>sideCount()===2);record('search-drag-to-side',true);
 drop(mainGrid().querySelectorAll('.deck-card-chip')[0],mainGrid().querySelectorAll('.deck-card-chip')[3]);await idle();record('drag-position',[...mainGrid().querySelectorAll('.deck-card-chip')][2].title==='Blue-Eyes');
 setInputValue(document.getElementById('deck-search-input'),'trap');await idle();searchBtn().click();await waitFor(()=>document.querySelectorAll('.deck-result-row').length===18);
 results=[...document.querySelectorAll('.deck-result-row')];for(let i=0;i<14;i++){drop(results[i],sideGrid());await idle();}record('side-limit-15',sideCount()===15);
 document.getElementById('deck-clear-btn').click();await waitFor(()=>mainCount()===0&&sideCount()===0);record('clear-all-sections',true);
 clearBtn().click();await waitFor(()=>document.querySelectorAll('.deck-result-row').length===0);record('clear-search-removes-results',true);
 await selectOption('deck-filter-type1','1');setInputValue(document.getElementById('deck-filter-atk'),'>=2500');setInputValue(document.getElementById('deck-filter-star'),'8');await idle();
 document.getElementById('deck-filter-atk').dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true}));await waitFor(()=>searchFilters.at(-1).atkFilter==='>=2500');record('operator-enter-search',searchFilters.at(-1).levelFilter==='8');
 document.getElementById('deck-filter-effect-btn').click();await waitFor(()=>document.getElementById('deck-effect-panel'));record('effect-32-options',document.querySelectorAll('#deck-effect-panel input[type=checkbox]').length===32);
 document.querySelector('#deck-effect-panel input').click();[...document.querySelectorAll('#deck-effect-panel button')].find(b=>b.textContent==='确定').click();await idle();searchBtn().click();await waitFor(()=>searchFilters.at(-1).effect===1);record('effect-passthrough',true);
 document.getElementById('deck-marks-filter-btn').click();await waitFor(()=>document.getElementById('deck-linkmarks-panel'));
 const arrows=[...document.querySelectorAll('#deck-linkmarks-panel button')];arrows[0].click();arrows.find(b=>b.textContent==='↓').click();arrows.find(b=>b.textContent==='确定').click();await idle();searchBtn().click();await waitFor(()=>searchFilters.at(-1).linkMarks===0x42);record('link-mask-passthrough',true);
 clearBtn().click();settingsStore.set('auto_search_limit',3);await idle();setInputValue(document.getElementById('deck-search-input'),'dragon');await waitFor(()=>searchFilters.at(-1).keyword==='dragon');record('instant-search',true);settingsStore.set('auto_search_limit',-1);
 await selectOption('deck-category-select','Tournament');await waitFor(()=>document.getElementById('deck-select').textContent.includes('TestDeck'));record('category-loads-first-deck',true);
 document.querySelector('.deck-header input').value='TestDeck';[...document.querySelectorAll('.deck-header .btn')].find(b=>b.textContent==='保存').click();await waitFor(()=>saveSends.length);record('save-selected-path',saveSends.at(-1).name==='Tournament/TestDeck');
 document.getElementById('deck-manage-btn').click();await waitFor(()=>document.getElementById('deck-manage-modal'));record('manage-has-native-query',!document.getElementById('dm-name-input'));
 const categoryNames=['Tournament'];WailsBridge.listDeckCategories=async()=>categoryNames;WailsBridge.createDeckCategory=async name=>{categoryNames.push(name);};
 document.getElementById('dm-new-category').click();await waitFor(()=>document.getElementById('dm-query'));setInputValue(document.getElementById('dm-name-input'),'Empty');await idle();document.getElementById('dm-query-ok').click();await waitFor(()=>!document.getElementById('dm-query'));
 await waitFor(()=>document.getElementById('dm-category-list').textContent.includes('Empty'));record('empty-category-visible',true);
 let copied=null;Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async t=>{copied=t;}}});document.getElementById('deck-export-code-btn').click();await waitFor(()=>copied);record('export-ydk',copied.includes('#main')&&copied.includes('#extra')&&copied.includes('!side'));
 document.getElementById('dm-close').click();await waitFor(()=>!document.getElementById('deck-manage-modal'));record('manage-closes',true);
 record('no-fatal',true);
}catch(e){checks.fatalMsg=String(e.stack||e);record('fatal',false);}finally{window.__deckSmoke.ready=true;}})();
