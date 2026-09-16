import React, { useEffect, useState, useSyncExternalStore } from 'react';
import { WailsBridge } from '../wails_bridge.ts';
import { settingsStore } from '../domain/settings.ts';
import {
  EXTRA_TYPES, TYPE_MONSTER, TYPE_SPELL, TYPE_TRAP,
  TYPE_NORMAL, TYPE_EFFECT, TYPE_FUSION, TYPE_RITUAL, TYPE_SYNCHRO, TYPE_XYZ,
  TYPE_PENDULUM, TYPE_LINK, TYPE_QUICKPLAY, TYPE_CONTINUOUS, TYPE_EQUIP,
  TYPE_FIELD, TYPE_COUNTER, TYPE_TUNER, TYPE_FLIP, TYPE_SPIRIT, TYPE_UNION,
  TYPE_DUAL, TYPE_TOON,
  RACES, ATTRS,
} from '../domain/constants.ts';

// 展示顺序即原版 Info 面板的种类拼接顺序（kind 在前，稀有子类在后）
const SUBTYPE_LABELS: [number, string][] = [
  [TYPE_NORMAL, '通常'], [TYPE_EFFECT, '效果'], [TYPE_FUSION, '融合'], [TYPE_RITUAL, '仪式'],
  [TYPE_SYNCHRO, '同调'], [TYPE_XYZ, '超量'], [TYPE_PENDULUM, '灵摆'], [TYPE_LINK, '连接'],
  [TYPE_QUICKPLAY, '速攻'], [TYPE_CONTINUOUS, '永续'], [TYPE_EQUIP, '装备'], [TYPE_FIELD, '场地'],
  [TYPE_COUNTER, '反击'], [TYPE_TUNER, '调整'], [TYPE_FLIP, '反转'], [TYPE_SPIRIT, '灵魂'],
  [TYPE_UNION, '同盟'], [TYPE_DUAL, '二重'], [TYPE_TOON, '卡通'],
];
const cardKind = (t: number): string =>
  (t & TYPE_MONSTER) ? '怪兽' : (t & TYPE_SPELL) ? '魔法' : (t & TYPE_TRAP) ? '陷阱' : '';
const cardSubtypes = (t: number): string[] =>
  SUBTYPE_LABELS.filter(([mask]) => (t & mask) !== 0).map(([, label]) => label);
// atk/def -2 是引擎的「?」占位
const fmtStat = (v: any): string => (v === -2 ? '?' : String(v ?? 0));

// 效果类型过滤：gframe wCategories 32 复选框，标签 = SysString 1100-1131，
// bit = 1<<i，匹配语义 (category & filter) != 0（deck_con.cpp:1534）
const EFFECT_CATEGORIES: [number, string][] = [
  '魔陷破坏', '怪兽破坏', '卡片除外', '送去墓地', '返回手卡', '返回卡组',
  '手卡破坏', '卡组破坏', '抽卡辅助', '卡组检索', '卡片回收', '表示形式',
  '控制权', '攻守变化', '穿刺伤害', '多次攻击', '攻击限制', '直接攻击',
  '特殊召唤', '衍生物', '种族相关', '属性相关', 'LP伤害', 'LP回复',
  '破坏耐性', '效果耐性', '指示物', '幸运', '融合相关', '同调相关',
  '超量相关', '效果无效',
].map((label, i) => [2 ** i, label]);

// 链接箭头过滤：wLinkMarks 8 箭头按钮的位映射（deck_con.cpp:793-812），
// 匹配语义 (link_marker & marks) == marks（子集）
const LINK_MARKS: [number, string][] = [
  [0x40, '↖'], [0x80, '↑'], [0x100, '↗'],
  [0x8, '←'], [0x20, '→'],
  [0x1, '↙'], [0x2, '↓'], [0x4, '↘'],
];

// 卡图缩略格子（原版 deck editor 是纯图片网格）。拉一次卡图与卡名缓存。
function CardChip({ code, count, onInspect, onRemove, onZoom }: {
  code: number;
  count?: number;
  onInspect?: (info: any) => void;
  onRemove?: () => void;
  onZoom?: (code: number) => void;
}) {
  const [info, setInfo] = useState<any>(null);
  const [pic, setPic] = useState<string | null>(null);
  useEffect(() => {
    let alive = true;
    WailsBridge.getCard(code).then((c) => { if (alive) setInfo(c); });
    WailsBridge.getCardImage(code).then((p) => { if (alive && p) setPic(p.url); });
    return () => { alive = false; };
  }, [code]);

  return (
    <div
      className="select-card-item deck-card-chip"
      style={{ cursor: 'pointer', position: 'relative', overflow: 'hidden' }}
      onMouseEnter={() => info && onInspect && onInspect(info)}
      onClick={() => onRemove && onRemove()}
      onDoubleClick={() => onZoom && onZoom(code)}
      title={info ? info.name : String(code)}
    >
      {pic
        ? <img src={pic} alt={info ? info.name : String(code)} draggable={false}
            style={{ width: '100%', height: '100%', objectFit: 'cover', display: 'block' }} />
        : <div style={{
            width: '100%', height: '100%', background: '#0f172a',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontSize: '9px', color: '#38bdf8', textAlign: 'center', padding: '2px',
          }}>{info ? info.name : code}</div>}
      {count && count > 1
        ? <span className="deck-count-badge">{`×${count}`}</span>
        : null}
    </div>
  );
}

interface DeckList {
  name: string;
  main: number[];
  extra: number[];
  side: number[];
}

// 搜索结果格子：卡图 + 点击加入卡组，悬停进左侧预览，双击看大图。
function SearchResultChip({ card, onInspect, onAdd, onZoom }: {
  card: any;
  onInspect: (info: any) => void;
  onAdd: () => void;
  onZoom: (code: number) => void;
}) {
  const [pic, setPic] = useState<string | null>(null);
  useEffect(() => {
    let alive = true;
    WailsBridge.getCardImage(card.code).then((p) => { if (alive && p) setPic(p.url); });
    return () => { alive = false; };
  }, [card.code]);

  return (
    <div
      className="select-card-item deck-card-chip"
      style={{ cursor: 'pointer', position: 'relative', overflow: 'hidden' }}
      onMouseEnter={() => onInspect(card)}
      onClick={onAdd}
      onDoubleClick={() => onZoom(card.code)}
      title={`${card.name}${card.attack !== undefined ? ` (ATK/${card.attack})` : ''}`}
    >
      {pic
        ? <img src={pic} alt={card.name} draggable={false}
            style={{ width: '100%', height: '100%', objectFit: 'cover', display: 'block' }} />
        : <div style={{
            width: '100%', height: '100%', background: '#1e293b',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontSize: '9px', color: '#38bdf8', textAlign: 'center', padding: '2px',
          }}>{card.name}</div>}
    </div>
  );
}

// 双击大图遮罩（原版双击卡片看全尺寸卡图）
function ZoomOverlay({ code, onClose }: { code: number; onClose: () => void }) {
  const [pic, setPic] = useState<string | null>(null);
  useEffect(() => {
    let alive = true;
    WailsBridge.getCardImage(code).then((p) => { if (alive && p) setPic(p.url); });
    return () => { alive = false; };
  }, [code]);
  return (
    <div
      id="deck-zoom-overlay"
      onClick={onClose}
      style={{
        position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.82)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        zIndex: 2000, cursor: 'zoom-out',
      }}
      title="点击关闭"
    >
      {pic
        ? <img src={pic} alt={String(code)} draggable={false}
            style={{ width: 370, height: 540, objectFit: 'cover', borderRadius: 8, border: '2px solid var(--primary)' }} />
        : <div style={{ color: '#38bdf8', fontSize: 14 }}>加载卡图……</div>}
    </div>
  );
}

type DeckSection = 'main' | 'extra' | 'side';
type SectionCounts = Record<DeckSection, Record<number, number>>;

export default function DeckBuilder({ onNavigate }: { onNavigate: (screen: string) => void }) {
  const [deckList, setDeckList] = useState<string[]>([]);
  const [currentDeck, setCurrentDeck] = useState<DeckList>({ name: '新卡组', main: [], extra: [], side: [] });
  const [deckName, setDeckName] = useState('新卡组');
  // gframe cbDBCategory：未分类 + ./deck/ 子目录（'' = 未分类卡组）；
  // 卡包/人机分类需 pack 目录与 bot 配置，暂不提供入口
  const [category, setCategory] = useState('');
  // 卡组下拉当前选中的完整相对名（含分类路径；新建/改名后与选项不匹配则显示空白）
  const [loadedDeck, setLoadedDeck] = useState('');
  const [isModified, setModified] = useState(false);
  // 每区按「同铭卡组」计数（alias 归并：alias!=0 的卡与本体共用计数）
  const [counts, setCounts] = useState<SectionCounts>({ main: {}, extra: {}, side: {} });
  const [searchKeyword, setSearchKeyword] = useState('');
  const [typeFilter, setTypeFilter] = useState('0');
  const [raceFilter, setRaceFilter] = useState('0');
  const [attrFilter, setAttrFilter] = useState('0');
  // gframe 风格运算符输入（parse_filter）：`=3000`/裸数字=相等、`>=`/`>`/`<=`/`<`、
  // `?` 查「?」占位卡；空串 = 不过滤
  const [starFilter, setStarFilter] = useState('');
  const [atkFilter, setAtkFilter] = useState('');
  const [defFilter, setDefFilter] = useState('');
  const [scaleFilter, setScaleFilter] = useState('');
  // 效果类型过滤（wCategories 32 复选框）与链接箭头过滤（wLinkMarks）
  const [effectBits, setEffectBits] = useState<Set<number>>(new Set());
  const [showEffectPanel, setShowEffectPanel] = useState(false);
  const [linkMarks, setLinkMarks] = useState(0);
  const [addTarget, setAddTarget] = useState<'main' | 'side'>('main');
  const [searchResults, setSearchResults] = useState<any[]>([]);
  const [inspected, setInspected] = useState<any>(null);
  const [inspectedPic, setInspectedPic] = useState<string | null>(null);
  const [zoomCode, setZoomCode] = useState(0);
  const settingsSnap = useSyncExternalStore(settingsStore.subscribe, settingsStore.getSnapshot);

  // 分类清单 = 卡组名的首段（有 '/' 的才属于分类；gframe TraversalDir isdir）
  const categories = Array.from(new Set(deckList.filter((n) => n.includes('/')).map((n) => n.split('/')[0])));
  // 当前分类下的卡组（未分类 = 根目录无 '/' 的名字）
  const decksInCategory = deckList.filter((n) => (category === '' ? !n.includes('/') : n.startsWith(category + '/')));

  useEffect(() => {
    (async () => {
      const list = await WailsBridge.listDecks();
      setDeckList(list || []);
      if (list && list.length > 0) {
        const deck = await WailsBridge.loadDeck(list[0]);
        if (deck) { setCurrentDeck(deck); setDeckName(deck.name || list[0]); buildCounts(deck).then(setCounts); }
      }
    })();
  }, []);

  useEffect(() => {
    if (!inspected) { setInspectedPic(null); return; }
    let alive = true;
    WailsBridge.getCardImage(inspected.code).then((p) => { if (alive) setInspectedPic(p ? p.url : null); });
    return () => { alive = false; };
  }, [inspected && inspected.code]);

  // 同铭归组需要 alias，而 alias 来自 getCard（带 memo 缓存）——先把整副
  // 卡组的卡信息拉齐，再按 alias||code 归并计数。
  const buildCounts = async (deck: DeckList): Promise<SectionCounts> => {
    await Promise.all([...deck.main, ...deck.extra, ...deck.side]
      .map((c) => WailsBridge.getCard(c).catch(() => null)));
    const groupOf = (code: number): number => {
      const info = WailsBridge._cardCache.get(code);
      return (info && info.alias) || code;
    };
    const tally = (codes: number[]): Record<number, number> => {
      const m: Record<number, number> = {};
      for (const c of codes) {
        const g = groupOf(c);
        m[g] = (m[g] || 0) + 1;
      }
      return m;
    };
    return { main: tally(deck.main), extra: tally(deck.extra), side: tally(deck.side) };
  };

  // 所有卡组变更的唯一入口：换引用、标脏、重算同铭计数
  const applyDeck = (deck: DeckList, { modified = true } = {}): void => {
    setCurrentDeck(deck);
    setModified(modified);
    buildCounts(deck).then(setCounts);
  };

  // 未存保护：切卡组/新建/删除/返回主菜单前，脏卡组要确认
  const discardGuard = (): boolean =>
    isModified && !window.confirm('当前卡组有未保存的修改，确定丢弃？');

  const loadDeck = async (name: string): Promise<void> => {
    const deck = await WailsBridge.loadDeck(name);
    if (deck) {
      applyDeck(deck, { modified: false });
      setDeckName(deck.name || name);
      // 分类跟随卡组名（gframe lastcategory 语义；'' = 未分类）
      setCategory(name.includes('/') ? name.split('/')[0] : '');
      setLoadedDeck(name);
    }
  };

  const saveDeck = async (): Promise<void> => {
    const base = deckName.trim() || '未命名卡组';
    // 分类下拉非「未分类」时，存到 ./deck/<分类>/<名>.ydk（gframe GetCategoryPath）
    const name = category ? `${category}/${base}` : base;
    const deck = { ...currentDeck, name };
    applyDeck(deck, { modified: false });
    setDeckName(base);
    setLoadedDeck(name);
    await WailsBridge.saveDeck(deck);
    alert(`卡组「${name}」保存成功！`);
    const list = await WailsBridge.listDecks();
    setDeckList(list);
  };

  const newDeck = (): void => {
    applyDeck({ name: '新卡组', main: [], extra: [], side: [] }, { modified: false });
    setDeckName('新卡组');
    setLoadedDeck('');
  };

  const deleteDeck = async (): Promise<void> => {
    // 删除目标 = 分类 + 当前名（分类未选时退化为本名）
    const base = deckName.trim();
    if (!base) { alert('请先选择要删除的卡组。'); return; }
    const name = category ? `${category}/${base}` : base;
    if (!window.confirm(`确定删除卡组「${name}」？此操作不可撤销。`)) return;
    await WailsBridge.deleteDeck(name);
    const list: string[] = (await WailsBridge.listDecks()) || [];
    setDeckList(list);
    const next = list.filter((n: string) => (category === '' ? !n.includes('/') : n.startsWith(category + '/')));
    if (next.length > 0) {
      await loadDeck(next[0]);
    } else {
      applyDeck({ name: '新卡组', main: [], extra: [], side: [] }, { modified: false });
      setDeckName('新卡组');
      setLoadedDeck('');
    }
  };

  const clearFilters = (): void => {
    setSearchKeyword('');
    setTypeFilter('0');
    setRaceFilter('0');
    setAttrFilter('0');
    setStarFilter(''); setAtkFilter(''); setDefFilter(''); setScaleFilter('');
    setEffectBits(new Set());
    setLinkMarks(0);
  };

  // 多关键词空格分隔（gframe search_multiple_keywords=1 的缺省语义）。
  // 位是互不重叠的 2 的幂，用加法合并避免 1<<31 的 int32 溢出
  const effectMask = [...effectBits].reduce((acc, b) => acc + b, 0);
  const performSearch = async (): Promise<void> => {
    const results = await WailsBridge.searchCards({
      keyword: searchKeyword.trim(),
      multiKeywords: 1,
      type: parseInt(typeFilter, 10) || 0,
      race: parseInt(raceFilter, 10) || 0,
      attribute: parseInt(attrFilter, 10) || 0,
      levelFilter: starFilter.trim(),
      atkFilter: atkFilter.trim(),
      defFilter: defFilter.trim(),
      scaleFilter: scaleFilter.trim(),
      effect: effectMask,
      linkMarks,
      limit: 60,
    });
    setSearchResults(results || []);
  };

  // 结果排序：主卡组怪兽 → 额外区怪兽 → 魔法 → 陷阱，同区按攻↓、卡名
  const typeOrderOf = (t: number): number =>
    (t & TYPE_MONSTER) ? ((t & EXTRA_TYPES) ? 1 : 0) : (t & TYPE_SPELL) ? 2 : 3;
  const sortedResults = [...searchResults].sort((a, b) =>
    typeOrderOf(a.type || 0) - typeOrderOf(b.type || 0)
    || ((b.attack || 0) - (a.attack || 0))
    || (a.name < b.name ? -1 : 1));

  const addCardToDeck = (card: any): void => {
    const isExtra = (card.type & EXTRA_TYPES) !== 0;
    const deck = { ...currentDeck, main: [...currentDeck.main], extra: [...currentDeck.extra], side: [...currentDeck.side] };
    if (isExtra) {
      if (deck.extra.length >= 15) { alert('额外卡组已满（最多 15 张）。'); return; }
      deck.extra.push(card.code);
    } else if (addTarget === 'side') {
      if (deck.side.length >= 15) { alert('副卡组已满（最多 15 张）。'); return; }
      deck.side.push(card.code);
    } else {
      if (deck.main.length >= 60) { alert('主卡组已满（最多 60 张）。'); return; }
      deck.main.push(card.code);
    }
    applyDeck(deck);
  };

  const removeCardFromDeck = (section: DeckSection, index: number): void => {
    const deck = { ...currentDeck, main: [...currentDeck.main], extra: [...currentDeck.extra], side: [...currentDeck.side] };
    deck[section].splice(index, 1);
    applyDeck(deck);
  };

  const simulateSampleHand = async (): Promise<void> => {
    if (currentDeck.main.length < 5) { alert('主卡组至少需要 5 张卡才能测试起手。'); return; }
    const shuffled = [...currentDeck.main].sort(() => 0.5 - Math.random());
    const names = await Promise.all(shuffled.slice(0, 5)
      .map((c) => WailsBridge.getCard(c).then((i) => (i && i.name) ? i.name : String(c))));
    alert('模拟起手 5 张：\n' + names.join('\n'));
  };

  // ---- 编辑器三键（deck_con.cpp:172-191）----

  const clearDeck = (): void => {
    // SysString 1339「是否清空正在编辑的卡组？」
    if (!window.confirm('是否清空正在编辑的卡组？')) return;
    applyDeck({ ...currentDeck, main: [], extra: [], side: [] });
  };

  // deck_sort_lv（data_manager.cpp:535）：type&0x7 升序分堆（1=怪兽/2=魔法/4=陷阱）；
  // 怪兽：带特殊位(0x48020c0=融合|仪式|同调|超量|链接)的按 type&0x48020c1、
  // 其余按 type&0x31 升序 → level↓ → atk↓ → def↓ → code↑；
  // 非怪兽：type&0xfffffff8 升序 → code↑。卡信息从 _cardCache 读
  // （buildCounts 已把整副卡组拉齐），缺信息的退化按卡号排。
  const deckSortKey = (a: number, b: number): number => {
    const ca = WailsBridge._cardCache.get(a);
    const cb = WailsBridge._cardCache.get(b);
    const ta = ca?.type ?? 0;
    const tb = cb?.type ?? 0;
    if ((ta & 0x7) !== (tb & 0x7)) return (ta & 0x7) - (tb & 0x7);
    if (ta & TYPE_MONSTER) {
      const typeOf = (t: number): number =>
        (t & 0x48020c0) ? (t & 0x48020c1) : (t & 0x31);
      const sa = typeOf(ta);
      const sb = typeOf(tb);
      if (sa !== sb) return sa - sb;
      const la = ca?.level ?? 0;
      const lb = cb?.level ?? 0;
      if (la !== lb) return lb - la;
      const aa = ca?.attack ?? 0;
      const ab = cb?.attack ?? 0;
      if (aa !== ab) return ab - aa;
      const da = ca?.defense ?? 0;
      const db = cb?.defense ?? 0;
      if (da !== db) return db - da;
      return a - b;
    }
    const sa = ta & ~0x7;
    const sb = tb & ~0x7;
    if (sa !== sb) return sa - sb;
    return a - b;
  };

  const sortDeck = (): void => {
    applyDeck({
      ...currentDeck,
      main: [...currentDeck.main].sort(deckSortKey),
      extra: [...currentDeck.extra].sort(deckSortKey),
      side: [...currentDeck.side].sort(deckSortKey),
    });
  };

  const shuffleDeck = (): void => {
    const shuffled = (codes: number[]): number[] => {
      const arr = [...codes];
      for (let i = arr.length - 1; i > 0; i--) {
        const j = Math.floor(Math.random() * (i + 1));
        [arr[i], arr[j]] = [arr[j], arr[i]];
      }
      return arr;
    };
    applyDeck({
      ...currentDeck,
      main: shuffled(currentDeck.main),
      extra: shuffled(currentDeck.extra),
      side: shuffled(currentDeck.side),
    });
  };

  // 效果复选框切换（wCategories → BUTTON_CATEGORY_OK 汇位掩码）
  const toggleEffectBit = (bit: number): void => {
    const next = new Set(effectBits);
    if (next.has(bit)) next.delete(bit); else next.add(bit);
    setEffectBits(next);
  };

  // 箭头按钮切换（wLinkMarks → BUTTON_MARKERS_OK 汇位掩码）
  const toggleLinkMark = (bit: number): void => {
    setLinkMarks((m) => m ^ bit);
  };

  const renderSection = (codes: number[], section: DeckSection) => (
    <div className="deck-grid" style={{ minHeight: section === 'main' ? '220px' : '80px' }}>
      {codes.map((code, i) => (
        <CardChip
          key={`${code}-${i}`}
          code={code}
          count={counts[section][(WailsBridge._cardCache.get(code)?.alias) || code]}
          onInspect={setInspected}
          onRemove={() => removeCardFromDeck(section, i)}
          onZoom={setZoomCode}
        />
      ))}
    </div>
  );

  // gframe 数值过滤是单输入框 + 运算符前缀（ebAttack/ebDefense/ebStar/ebScale）
  const statInput = (id: string, placeholder: string, value: string, setter: (v: string) => void) => (
    <input
      id={id}
      className="form-input deck-filter-num"
      type="text"
      placeholder={placeholder}
      title="支持运算符：3000 / =3000 / >=2500 / <1500 / ?（占位卡）"
      value={value}
      onChange={(e) => setter(e.target.value)}
    />
  );

  return (
    <div id="deck-screen" className="screen active">
      <div className="deck-header">
        <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
          <h2 style={{ color: 'var(--primary)' }}>卡组构筑{isModified ? ' *' : ''}</h2>
          {/* 分类下拉（gframe cbDBCategory：未分类卡组 + ./deck/ 子目录）
              与当前分类下的卡组下拉；名字含 '/' 时只显示最后一段 */}
          <select
            id="deck-category-select"
            className="form-select"
            style={{ width: '130px' }}
            value={category}
            onChange={(e) => {
              const cat = e.target.value;
              if (!discardGuard()) { setCategory(cat); }
            }}
          >
            <option value="">未分类卡组</option>
            {categories.map((c) => <option key={c} value={c}>{c}</option>)}
          </select>
          <select
            id="deck-select"
            className="form-select"
            style={{ width: '200px' }}
            value={decksInCategory.includes(loadedDeck) ? loadedDeck : ''}
            onChange={(e) => { if (e.target.value && !discardGuard()) loadDeck(e.target.value); }}
          >
            {decksInCategory.map((n) => <option key={n} value={n}>{n.split('/').pop()}</option>)}
          </select>
          <input className="form-input" style={{ width: '180px' }} value={deckName} onChange={(e) => setDeckName(e.target.value)} />
          <button className="btn btn-primary" onClick={saveDeck}>保存</button>
          <button className="btn btn-secondary" onClick={() => { if (!discardGuard()) newDeck(); }}>新建卡组</button>
          <button className="btn btn-danger" onClick={deleteDeck}>删除</button>
          <button className="btn btn-gold" onClick={simulateSampleHand}>测试起手 5 张</button>
        </div>
        <button className="btn btn-secondary" onClick={() => { if (!discardGuard()) onNavigate('menu'); }}>退出编辑</button>
      </div>

      <div className="deck-main-layout">
        <div className="card-inspector" style={{ position: 'static', width: '100%', height: '100%' }}>
          <div className="inspector-pic-box">
            {inspectedPic
              ? <img id="deck-inspector-pic" src={inspectedPic} alt={inspected.name} draggable={false}
                  style={{ width: '100%', height: '100%', objectFit: 'cover', display: 'block' }} />
              : <div style={{
                  width: '100%', height: '100%', background: '#0f172a',
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                  fontSize: '11px', color: '#38bdf8',
                }}>{inspected ? '卡图加载中……' : '卡牌详情'}</div>}
          </div>
          <div className="inspector-details">
            <div className="inspector-name">{inspected ? inspected.name : '卡牌详情'}</div>
            {inspected ? (
              <>
                <div className="inspector-meta" id="deck-inspector-type">
                  {[cardKind(inspected.type || 0), ...cardSubtypes(inspected.type || 0)].filter(Boolean).join('｜')}
                </div>
                {(inspected.type & TYPE_MONSTER) !== 0 ? (
                  <div className="inspector-meta" id="deck-inspector-stats">
                    {(inspected.type & TYPE_LINK) ? `LINK-${inspected.level}` : `星数 ${inspected.level}`}
                    {' ｜ '}
                    {(RACES.find((r) => r[0] === inspected.race) || [0, ''])[1] as string}
                    {' ｜ '}
                    {(ATTRS.find((a) => a[0] === inspected.attribute) || [0, ''])[1] as string}
                  </div>
                ) : null}
                {(inspected.type & TYPE_MONSTER) !== 0 ? (
                  <div className="inspector-meta">
                    攻击 {fmtStat(inspected.attack)} / 守备 {fmtStat(inspected.defense)}
                  </div>
                ) : null}
                {!settingsSnap.hide_setname && inspected.setNames && inspected.setNames.length > 0 ? (
                  <div className="inspector-meta" id="deck-inspector-setnames">
                    系列：{inspected.setNames.join(' / ')}
                  </div>
                ) : null}
                <div className="inspector-desc">{inspected.desc || ''}</div>
              </>
            ) : (
              <div className="inspector-desc">悬停任意卡牌查看详情，双击查看大图。</div>
            )}
          </div>
        </div>

        <div className="deck-zones-container">
          {/* 编辑器三键（deck_con.cpp:172-191 BUTTON_CLEAR/SORT/SHUFFLE_DECK） */}
          <div style={{ display: 'flex', gap: '8px' }}>
            <button id="deck-clear-btn" className="btn btn-secondary" onClick={clearDeck}>清空</button>
            <button id="deck-sort-btn" className="btn btn-secondary" onClick={sortDeck}>排序</button>
            <button id="deck-shuffle-btn" className="btn btn-secondary" onClick={shuffleDeck}>打乱</button>
          </div>
          <div>
            <div className="deck-section-title">
              <span>主卡组：</span>
              <span className="badge badge-attr">{currentDeck.main.length} / 60</span>
            </div>
            {renderSection(currentDeck.main, 'main')}
          </div>
          <div>
            <div className="deck-section-title">
              <span>额外卡组：</span>
              <span className="badge badge-type">{currentDeck.extra.length} / 15</span>
            </div>
            {renderSection(currentDeck.extra, 'extra')}
          </div>
          <div>
            <div className="deck-section-title">
              <span>副卡组：</span>
              <span className="badge badge-attr">{currentDeck.side.length} / 15</span>
            </div>
            {renderSection(currentDeck.side, 'side')}
          </div>
        </div>

        <div className="deck-search-panel">
          <h3 style={{ color: 'var(--primary)', fontSize: '16px' }}>卡牌搜索</h3>
          <input
            id="deck-search-input"
            className="form-input"
            type="text"
            placeholder="名称/效果/编号；$卡名 @系列名"
            value={searchKeyword}
            onChange={(e) => setSearchKeyword(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') performSearch(); }}
          />
          <div style={{ display: 'flex', gap: '6px' }}>
            <select className="form-select" value={typeFilter} onChange={(e) => setTypeFilter(e.target.value)}>
              <option value="0">（无）</option>
              <option value="1">怪兽</option>
              <option value="2">魔法</option>
              <option value="4">陷阱</option>
              <option value="32">效果怪兽</option>
              <option value="64">融合</option>
              <option value="128">仪式</option>
              <option value="8192">同调</option>
              <option value="8388608">超量</option>
              <option value="16777216">灵摆</option>
              <option value="4194304">连接</option>
            </select>
            <select id="deck-filter-race" className="form-select" value={raceFilter} onChange={(e) => setRaceFilter(e.target.value)}>
              <option value="0">（无）</option>
              {RACES.map(([v, name]) => <option key={v} value={String(v)}>{name}</option>)}
            </select>
            <select id="deck-filter-attr" className="form-select" value={attrFilter} onChange={(e) => setAttrFilter(e.target.value)}>
              <option value="0">（无）</option>
              {ATTRS.map(([v, name]) => <option key={v} value={String(v)}>{name}</option>)}
            </select>
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '4px' }}>
            {statInput('deck-filter-star', '等级（如 8 / >=7）', starFilter, setStarFilter)}
            {statInput('deck-filter-scale', '灵摆刻度（如 4 / <=3）', scaleFilter, setScaleFilter)}
            {statInput('deck-filter-atk', '攻击力（如 1900 / >=2500 / ?）', atkFilter, setAtkFilter)}
            {statInput('deck-filter-def', '守备力（如 2000 / <1000）', defFilter, setDefFilter)}
          </div>
          {/* 效果类型过滤（wCategories 32 复选框）+ 链接箭头（wLinkMarks） */}
          <div style={{ display: 'flex', gap: '8px', marginTop: '8px' }}>
            <button
              id="deck-filter-effect-btn"
              className="btn btn-secondary"
              style={{ flex: 1 }}
              onClick={() => setShowEffectPanel((v) => !v)}
            >
              效果过滤{effectBits.size > 0 ? `（${effectBits.size}）` : ''}
            </button>
          </div>
          {showEffectPanel ? (
            <div id="deck-effect-panel" style={{
              border: '1px solid var(--primary)', borderRadius: 6, padding: '6px',
              marginTop: '4px', maxHeight: '180px', overflowY: 'auto',
              display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '2px', fontSize: '11px',
            }}>
              {EFFECT_CATEGORIES.map(([bit, label]) => (
                <label key={bit} style={{ display: 'flex', alignItems: 'center', gap: '2px', cursor: 'pointer' }}>
                  <input type="checkbox" checked={effectBits.has(bit)} onChange={() => toggleEffectBit(bit)} />
                  {label}
                </label>
              ))}
              <button
                className="btn btn-primary"
                style={{ gridColumn: '1 / -1', marginTop: '4px' }}
                onClick={() => setShowEffectPanel(false)}
              >
                确定
              </button>
            </div>
          ) : null}
          <div id="deck-linkmarks-panel" style={{ marginTop: '8px' }}>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginBottom: '2px' }}>
              连接标记{linkMarks > 0 ? `（掩码 ${linkMarks}）` : ''}
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 32px)', gap: '2px', justifyItems: 'center' }}>
              {LINK_MARKS.map(([bit, arrow], i) => (
                <React.Fragment key={bit}>
                  {/* 3x3 网格中央留空（原版中心是确定按钮，我们即时生效无需它） */}
                  {i === 4 ? <span /> : null}
                  <button
                    className={`btn ${((linkMarks & bit) !== 0) ? 'btn-gold' : 'btn-secondary'}`}
                    style={{ width: 32, height: 26, padding: 0, fontSize: 13 }}
                    title="按住的箭头方向都要命中（子集语义）"
                    onClick={() => toggleLinkMark(bit)}
                  >
                    {arrow}
                  </button>
                </React.Fragment>
              ))}
            </div>
          </div>
          <div style={{ display: 'flex', gap: '8px', marginTop: '8px' }}>
            <button className="btn btn-primary" style={{ flex: 1 }} onClick={performSearch}>搜索</button>
            <button id="deck-filter-clear" className="btn btn-secondary" onClick={clearFilters}>清空</button>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginTop: '8px' }}>
            <label style={{ fontSize: '12px', color: 'var(--text-muted)' }}>点击加入：</label>
            <select id="deck-add-target" className="form-select" value={addTarget} onChange={(e) => setAddTarget(e.target.value as 'main' | 'side')}>
              <option value="main">主卡组</option>
              <option value="side">副卡组</option>
            </select>
          </div>

          <div id="deck-result-count" style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '4px' }}>
            搜索结果（{sortedResults.length} 张，点击添加，双击看大图）：
          </div>
          <div className="search-results-grid">
            {sortedResults.map((card, i) => (
              <SearchResultChip
                key={`${card.code}-${i}`}
                card={card}
                onInspect={setInspected}
                onAdd={() => addCardToDeck(card)}
                onZoom={setZoomCode}
              />
            ))}
          </div>
        </div>
      </div>

      {zoomCode ? <ZoomOverlay code={zoomCode} onClose={() => setZoomCode(0)} /> : null}
    </div>
  );
}
