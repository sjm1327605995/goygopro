import React, { useEffect, useRef, useState, useSyncExternalStore } from 'react';
import { WailsBridge } from '../wails_bridge.ts';
import { settingsStore } from '../domain/settings.ts';
import GfwSelect from './ui/GfwSelect.tsx';
import UiButton from '../ui/UiButton.tsx';
import { UiInput, UiCheckbox } from '../ui/UiInput.tsx';
import DesignSpace from '../ui/DesignSpace.tsx';
import DeckManageModal from './DeckManageModal.tsx';
import {
  EXTRA_TYPES, TYPE_MONSTER, TYPE_SPELL, TYPE_TRAP,
  TYPE_NORMAL, TYPE_EFFECT, TYPE_FUSION, TYPE_RITUAL, TYPE_SYNCHRO, TYPE_XYZ,
  TYPE_PENDULUM, TYPE_LINK, TYPE_SPSUMMON, TYPE_QUICKPLAY, TYPE_CONTINUOUS, TYPE_EQUIP,
  TYPE_FIELD, TYPE_COUNTER, TYPE_TUNER, TYPE_FLIP, TYPE_SPIRIT, TYPE_UNION,
  TYPE_DUAL, TYPE_TOON,
  AVAIL_OCG, AVAIL_TCG, AVAIL_SC, AVAIL_CUSTOM, AVAIL_OCGTCG,
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

// 副种类（cbCardType2）选项表，按种类（cbCardType）联动，值 = 精确 type 掩码
// （deck_con.cpp:913-970 COMBOBOX_MAINTYPE 的填表逻辑）。
// 怪兽是子集语义 (type & mask) == mask；魔陷是精确相等 type == mask。
const TYPE2_OPTIONS: Record<string, [number, string][]> = {
  '1': [
    [TYPE_MONSTER | TYPE_NORMAL, '通常'], [TYPE_MONSTER | TYPE_EFFECT, '效果'],
    [TYPE_MONSTER | TYPE_FUSION, '融合'], [TYPE_MONSTER | TYPE_RITUAL, '仪式'],
    [TYPE_MONSTER | TYPE_SYNCHRO, '同调'], [TYPE_MONSTER | TYPE_XYZ, '超量'],
    [TYPE_MONSTER | TYPE_PENDULUM, '灵摆'], [TYPE_MONSTER | TYPE_LINK, '连接'],
    [TYPE_MONSTER | TYPE_SPSUMMON, '特殊召唤'],
    [TYPE_MONSTER | TYPE_NORMAL | TYPE_TUNER, '通常|调整'],
    [TYPE_MONSTER | TYPE_NORMAL | TYPE_PENDULUM, '通常|灵摆'],
    [TYPE_MONSTER | TYPE_SYNCHRO | TYPE_TUNER, '同调|调整'],
    [TYPE_MONSTER | TYPE_TUNER, '调整'], [TYPE_MONSTER | TYPE_DUAL, '二重'],
    [TYPE_MONSTER | TYPE_UNION, '同盟'], [TYPE_MONSTER | TYPE_SPIRIT, '灵魂'],
    [TYPE_MONSTER | TYPE_FLIP, '反转'], [TYPE_MONSTER | TYPE_TOON, '卡通'],
  ],
  '2': [
    [TYPE_SPELL, '通常'], [TYPE_SPELL | TYPE_QUICKPLAY, '速攻'],
    [TYPE_SPELL | TYPE_CONTINUOUS, '永续'], [TYPE_SPELL | TYPE_RITUAL, '仪式'],
    [TYPE_SPELL | TYPE_EQUIP, '装备'], [TYPE_SPELL | TYPE_FIELD, '场地'],
  ],
  '3': [
    [TYPE_TRAP, '通常'], [TYPE_TRAP | TYPE_CONTINUOUS, '永续'],
    [TYPE_TRAP | TYPE_COUNTER, '反击'],
  ],
};

// 禁限下拉（cbLimit，deck_con.cpp:1391/1542-1553）：索引即 filter_lm
const LIMIT_OPTIONS: string[] = [
  '（无）', '禁止', '限制', '准限制', 'OCG', 'TCG', 'SC', '自定义', 'OCG&TCG',
];

// 卡图缩略格子（原版 deck editor 是纯图片网格）。拉一次卡图与卡名缓存。
// lfLimit：禁限卡表档位（0 禁/1 限/2 准限，undefined = 无限制），画左上角
// 角标（原版 DrawThumb 在缩略图左上叠 lim 贴图，drawing.cpp:1176-1198）。
function CardChip({ code, count, lfLimit, style, onInspect, onRemove, onZoom, onContextMenu, onDragStart, onDrop }: {
  code: number;
  count?: number;
  lfLimit?: number;
  style?: React.CSSProperties;
  onInspect?: (info: any) => void;
  onRemove?: () => void;
  onZoom?: (code: number) => void;
  onContextMenu?: (e: React.MouseEvent) => void;
  onDragStart?: (e: React.DragEvent) => void;
  onDrop?: (e: React.DragEvent) => void;
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
      style={{ cursor: 'pointer', position: 'relative', overflow: 'hidden', ...style }}
      draggable={!!onDragStart}
      onDragStart={onDragStart}
      onDragOver={onDrop ? (e) => e.preventDefault() : undefined}
      onDrop={onDrop}
      onContextMenu={onContextMenu}
      onMouseEnter={() => info && onInspect && onInspect(info)}
      onClick={() => onRemove && onRemove()}
      onDoubleClick={() => onZoom && onZoom(code)}
      title={info ? info.name : String(code)}
    >
      {/* 卡图只等比缩放（contain），不裁剪不拉升 */}
      {pic
        ? <img src={pic} alt={info ? info.name : String(code)} draggable={false}
            style={{ width: '100%', height: '100%', objectFit: 'contain', display: 'block' }} />
        : <img src="textures/unknown.jpg" alt={info ? info.name : String(code)} draggable={false}
            style={{ width: '100%', height: '100%', objectFit: 'contain', display: 'block' }} />}
      {count && count > 1
        ? <span className="deck-count-badge">{`×${count}`}</span>
        : null}
      {lfLimit !== undefined && lfLimit < 3
        ? <span className={`deck-limit-badge deck-limit-${lfLimit}`}>{LIMIT_BADGE_LABELS[lfLimit]}</span>
        : null}
    </div>
  );
}

// 禁限角标文案（SysString 1316/1317/1318：禁止/限制/准限制）
const LIMIT_BADGE_LABELS: Record<number, string> = { 0: '禁', 1: '限', 2: '准' };

interface DeckList {
  name: string;
  main: number[];
  extra: number[];
  side: number[];
}

// 卡组码 = 原版剪贴板 ydk 文本格式（deck_con.cpp BUTTON_EXPORT_DECK_CODE →
// DeckManager::SaveDeck(deck, stringstream)）：卡号十进制逐行，#main/#extra/!side 分段。
export const serializeDeckYdk = (deck: DeckList): string => {
  const lines = ['#created by goygopro', '#main'];
  for (const c of deck.main) lines.push(String(c));
  lines.push('#extra');
  for (const c of deck.extra) lines.push(String(c));
  lines.push('!side');
  for (const c of deck.side) lines.push(String(c));
  return lines.join('\n') + '\n';
};

// 与 Go LoadDeck（card_db.go）同语义：#main/#extra/!side 分段，# 其余行与
// 空行忽略，非法卡号行跳过。没有任何有效行（至少一个分段头或卡号）返回 null。
export const parseDeckYdk = (text: string): { main: number[]; extra: number[]; side: number[] } | null => {
  const deck = { main: [] as number[], extra: [] as number[], side: [] as number[] };
  let section: 'main' | 'extra' | 'side' | null = null;
  let sawContent = false;
  for (const raw of text.split(/\r?\n/)) {
    const line = raw.trim();
    if (line.startsWith('#main')) { section = 'main'; sawContent = true; continue; }
    if (line.startsWith('#extra')) { section = 'extra'; sawContent = true; continue; }
    if (line.startsWith('!side')) { section = 'side'; sawContent = true; continue; }
    if (line.startsWith('#') || line === '') continue;
    if (!/^\d+$/.test(line)) continue;
    const code = parseInt(line, 10);
    if (!Number.isSafeInteger(code) || code <= 0 || code > 0xffffffff) continue;
    if (section) { deck[section].push(code); sawContent = true; }
  }
  return sawContent ? deck : null;
};

// 搜索结果行：纵向列表的一行（原版 DrawDeckBd 搜索区 drawing.cpp:1296-1360），
// 左缩略图 + 右侧卡名/种类/属性/种族/星级/攻守文本；点击加入卡组，悬停进左侧
// 预览，双击看大图。
function SearchResultChip({ card, lfLimit, onInspect, onAdd, onAddSide, onZoom }: {
  card: any;
  lfLimit?: number;
  onInspect: (info: any) => void;
  onAdd: () => void;
  /** 右键加入副卡组 */
  onAddSide: () => void;
  onZoom: (code: number) => void;
}) {
  const [pic, setPic] = useState<string | null>(null);
  useEffect(() => {
    let alive = true;
    WailsBridge.getCardImage(card.code).then((p) => { if (alive && p) setPic(p.url); });
    return () => { alive = false; };
  }, [card.code]);

  const t = card.type || 0;
  const isMonster = (t & TYPE_MONSTER) !== 0;
  const raceName = (RACES.find((r) => r[0] === card.race) || [0, ''])[1] as string;
  const attrName = (ATTRS.find((a) => a[0] === card.attribute) || [0, ''])[1] as string;
  const kind = [cardKind(t), ...cardSubtypes(t)].filter(Boolean).join('｜');

  return (
    <div
      className="deck-card-chip deck-result-row"
      style={{ cursor: 'pointer', position: 'relative', overflow: 'hidden' }}
      onMouseEnter={() => onInspect(card)}
      onClick={onAdd}
      onContextMenu={(e) => { e.preventDefault(); onAddSide(); }}
      onDoubleClick={() => onZoom(card.code)}
      title={card.name}
    >
      <div className="deck-result-thumb">
        <img src={pic || 'textures/unknown.jpg'} alt={card.name} draggable={false}
            style={{ width: '100%', height: '100%', objectFit: 'contain', display: 'block' }} />
      </div>
      <div className="deck-result-text">
        <div className="deck-result-name">{card.name}</div>
        <div className="deck-result-sub">
          {isMonster
            ? `${attrName}/${raceName} ${(t & TYPE_LINK) ? `LINK-${card.level}` : `★${card.level}`}`
            : kind}
        </div>
        {isMonster ? (
          <div className="deck-result-stats">
            {fmtStat(card.attack)}/{fmtStat(card.defense)}
          </div>
        ) : null}
        {lfLimit !== undefined && lfLimit < 3
          ? <span className={`deck-limit-badge deck-limit-${lfLimit}`}>{LIMIT_BADGE_LABELS[lfLimit]}</span>
          : null}
      </div>
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
            style={{ width: 370, height: 540, objectFit: 'contain', borderRadius: 8, border: '2px solid var(--primary)' }} />
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
  // 禁限卡表：当前应用的表内容（卡码→0 禁/1 限/2 准限）与表名。
  // 表选择沿用原版配置语义（deck_con.cpp Initialize）：use_lflist=1 时用
  // default_lflist 索引指定的表，否则用 N/A（哈希 0，无限制）。
  const [lfContent, setLfContent] = useState<Record<number, number>>({});
  const [lfName, setLfName] = useState('');
  const [searchKeyword, setSearchKeyword] = useState('');
  // 种类（cbCardType，deck_con.cpp:893）：0=（无） 1=怪兽 2=魔法 3=陷阱；
  // 副种类（cbCardType2）选项按种类联动、值是精确 type 掩码
  const [typeFilter, setTypeFilter] = useState('0');
  const [type2Filter, setType2Filter] = useState('0');
  // 禁限（cbLimit，deck_con.cpp:1391）：0=（无） 1=禁止 2=限制 3=准限制
  // 4=OCG 5=TCG 6=SC 7=自定义 8=OCG&TCG
  const [limitFilter, setLimitFilter] = useState('0');
  const [raceFilter, setRaceFilter] = useState('0');
  const [attrFilter, setAttrFilter] = useState('0');
  // gframe 风格运算符输入（parse_filter）：`=3000`/裸数字=相等、`>=`/`>`/`<=`/`<`、
  // `?` 查「?」占位卡；空串 = 不过滤
  const [starFilter, setStarFilter] = useState('');
  const [atkFilter, setAtkFilter] = useState('');
  const [defFilter, setDefFilter] = useState('');
  const [scaleFilter, setScaleFilter] = useState('');
  // 效果类型过滤（wCategories 32 复选框）与连接标记过滤（wLinkMarks 弹窗：
  // 箭头先改挂起态，确定键提交——原版 BUTTON_MARKERS_OK 语义）。
  // 挂起态同时放 ref：合成事件连续点击时 React 批处理会让「确定」闭包
  // 读到旧 state，原版读的是按钮实时按下态，ref 才是等效实现
  const [effectBits, setEffectBits] = useState<Set<number>>(new Set());
  const [showEffectPanel, setShowEffectPanel] = useState(false);
  const [linkMarks, setLinkMarks] = useState(0);
  const [showLinkMarks, setShowLinkMarks] = useState(false);
  const [linkMarksPending, setLinkMarksPending] = useState(0);
  const linkMarksPendingRef = useRef(0);
  const togglePendingMark = (bit: number): void => {
    linkMarksPendingRef.current ^= bit;
    setLinkMarksPending(linkMarksPendingRef.current);
  };
  const toggleLinkMarksPopup = (): void => {
    if (showLinkMarks) { setShowLinkMarks(false); return; }
    linkMarksPendingRef.current = linkMarks;
    setLinkMarksPending(linkMarks);
    setShowLinkMarks(true);
  };
  const commitLinkMarks = (): void => {
    setLinkMarks(linkMarksPendingRef.current);
    setShowLinkMarks(false);
  };
  const [addTarget, setAddTarget] = useState<'main' | 'side'>('main');
  const [searchResults, setSearchResults] = useState<any[]>([]);
  const [inspected, setInspected] = useState<any>(null);
  const [inspectedPic, setInspectedPic] = useState<string | null>(null);
  const [zoomCode, setZoomCode] = useState(0);
  // 原版 wInfos 的 Card info / Log 页签：Log 记录本次编辑的增删操作
  const [inspectorTab, setInspectorTab] = useState<'info' | 'log'>('info');
  const [editLog, setEditLog] = useState<string[]>([]);
  const logEdit = (msg: string) => setEditLog((prev) => [...prev.slice(-99), msg]);
  // 拖拽源（dragstart 存 ref，dragover/drop 读回，不动 state 免得整页重渲染）
  const draggedRef = React.useRef<{ section: DeckSection; index: number } | null>(null);
  // 右键上下文菜单状态（卡组卡片：移到主/额外/副卡组 + 移出）
  const [ctxMenu, setCtxMenu] = useState<{ x: number; y: number; section: DeckSection; index: number } | null>(null);
  // 卡组码导入导出 / wDeckManage 管理窗口
  const [exportText, setExportText] = useState<string | null>(null);
  const [showImport, setShowImport] = useState(false);
  const [importText, setImportText] = useState('');
  const [showManage, setShowManage] = useState(false);
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

  // check_limit（deck_con.cpp:1834-1853）：同名卡（alias 归并）上限默认 3，
  // 命中禁限卡表时按表取 0/1/2（禁/限/准限）。表内容来自 Go 侧
  // LFListContent（lflist.conf 解析，deck_manager.go LoadLFListSingle）。
  const groupOf = (code: number): number => {
    const info = WailsBridge._cardCache.get(code);
    return (info && info.alias) || code;
  };
  const countInDeck = (deck: DeckList, group: number): number =>
    [...deck.main, ...deck.extra, ...deck.side].reduce((acc, c) => acc + (groupOf(c) === group ? 1 : 0), 0);
  // 表的键是 duel_code（alias 本体），lookup 也要归并
  const limitOf = (group: number): number => lfContent[group] ?? 3;

  // 挂载时按原版配置语义选表并拉内容（use_lflist=0 → N/A 空表）
  useEffect(() => {
    let alive = true;
    (async () => {
      const lists = await WailsBridge.listLFLists();
      await settingsStore.load();
      const snap = settingsStore.getSnapshot();
      let hash = 0;
      if (Number(snap.use_lflist ?? 1) === 1 && lists && lists.length > 0) {
        const idx = Math.min(Math.max(Number(snap.default_lflist ?? 0) || 0, 0), lists.length - 1);
        hash = lists[idx].hash;
        if (alive) setLfName(lists[idx].name);
      } else if (lists && lists.length > 0) {
        const na = lists.find((l) => l.hash === 0);
        if (na && alive) setLfName(na.name);
      }
      const content = await WailsBridge.lfListContent(hash);
      if (alive) setLfContent(content || {});
    })();
    return () => { alive = false; };
  }, []);

  // 所有卡组变更的唯一入口：换引用、标脏、重算同铭计数
  const applyDeck = (deck: DeckList, { modified = true } = {}): void => {
    setCurrentDeck(deck);
    setModified(modified);
    buildCounts(deck).then(setCounts);
  };

  // 未存保护：切卡组/新建/删除/返回主菜单前，脏卡组要确认。
  // ignore_deck_changes（deck_con.cpp:259/296/861/877 的 chkIgnoreDeckChanges）
  // 勾选时跳过确认，仍保留 isModified 的「*」标记。
  const discardGuard = (): boolean =>
    !settingsSnap.ignore_deck_changes
    && isModified
    && !window.confirm('当前卡组有未保存的修改，确定丢弃？');

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

  // BUTTON_SAVE_DECK（deck_con.cpp:192-204）：保存 = 覆盖当前已载入卡组（不读名字输入框）。
  // 若有已载入卡组（loadedDeck），写回原路径；否则（新建卡组）退化为另存为。
  const saveDeckOverwrite = async (): Promise<void> => {
    if (!loadedDeck) { await saveDeck(); return; }
    const deck = { ...currentDeck, name: loadedDeck };
    applyDeck(deck, { modified: false });
    setDeckName(loadedDeck.split('/').pop() || loadedDeck);
    await WailsBridge.saveDeck(deck);
    alert(`卡组「${loadedDeck}」保存成功！`);
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

  // btnClearFilter（deck_con.cpp ClearSearch：种类/副种类/禁限/种族/属性/
  // 运算符输入/效果/箭头全部复位，弹窗一并收起）
  const clearFilters = (): void => {
    setSearchKeyword('');
    setTypeFilter('0');
    setType2Filter('0');
    setLimitFilter('0');
    setRaceFilter('0');
    setAttrFilter('0');
    setStarFilter(''); setAtkFilter(''); setDefFilter(''); setScaleFilter('');
    setEffectBits(new Set());
    setShowEffectPanel(false);
    setLinkMarks(0);
    setLinkMarksPending(0);
    linkMarksPendingRef.current = 0;
    setShowLinkMarks(false);
  };

  // 种类切换（COMBOBOX_MAINTYPE deck_con.cpp:893-984）：副种类归零，
  // 种族/属性/攻守/星级/刻度一并清空；种类=（无）时这些控件禁用
  const onType1Change = (v: string): void => {
    setTypeFilter(v);
    setType2Filter('0');
    setRaceFilter('0');
    setAttrFilter('0');
    setStarFilter(''); setAtkFilter(''); setDefFilter(''); setScaleFilter('');
  };

  // 副种类切换（COMBOBOX_SECONDTYPE deck_con.cpp:994-1003）：
  // 怪兽选「连接」时守备输入禁用并清空（连接怪兽没有守备力）
  const onType2Change = (v: string): void => {
    setType2Filter(v);
    if (parseInt(v, 10) === (TYPE_MONSTER | TYPE_LINK)) setDefFilter('');
  };

  const isMonsterKind = typeFilter === '1';
  const isLinkType2 = isMonsterKind && parseInt(type2Filter, 10) === (TYPE_MONSTER | TYPE_LINK);
  // 种类=（无）时副种类/种族/属性/数值输入全部禁用；魔陷时仅副种类可用
  const statsDisabled = typeFilter !== '1';
  const type2Disabled = typeFilter === '0';

  // 禁限搜索过滤（deck_con.cpp:1542-1553 filter_lm）：1-3 查当前禁限卡表
  // （键 = cdb 行卡码，与原版 content.count(code) 一致），4-8 查 ot 可用范围
  const applyLimitFilter = (list: any[]): any[] => {
    const lm = parseInt(limitFilter, 10);
    if (!lm) return list;
    if (lm <= 3) return list.filter((c) => lfContent[c.code] === lm - 1);
    if (lm === 4) return list.filter((c) => (c.ot & AVAIL_OCG) !== 0);
    if (lm === 5) return list.filter((c) => (c.ot & AVAIL_TCG) !== 0);
    if (lm === 6) return list.filter((c) => (c.ot & AVAIL_SC) !== 0);
    if (lm === 7) return list.filter((c) => (c.ot & AVAIL_CUSTOM) !== 0);
    return list.filter((c) => (c.ot & AVAIL_OCGTCG) === AVAIL_OCGTCG);
  };

  // 多关键词分隔符来自设置 search_multiple_keywords（0=整串 / 1=空格 / 2=加号，
  // deck_con.cpp:1418；Go card_db.go parseKeywordElements 已支持三档）。
  // 位是互不重叠的 2 的幂，用加法合并避免 1<<31 的 int32 溢出
  const effectMask = [...effectBits].reduce((acc, b) => acc + b, 0);
  const performSearch = async (): Promise<void> => {
    // 种类/副种类 → type 参数（deck_con.cpp:1486-1530 的语义）：
    // 怪兽 = 子集掩码（(type & mask) == mask，Go 侧现有语义）；
    // 魔陷选了副种类 = 精确相等（type == mask，前端后置过滤）。
    const t1 = parseInt(typeFilter, 10);
    const t2 = parseInt(type2Filter, 10);
    let type = 0;
    let typeExact = 0;
    if (t1 === 1) type = t2 || TYPE_MONSTER;
    else if (t1 === 2) { if (t2) typeExact = t2; else type = TYPE_SPELL; }
    else if (t1 === 3) { if (t2) typeExact = t2; else type = TYPE_TRAP; }
    // 禁限过滤在前端后置（lfContent 已同步到本机），此时放宽服务端
    // 条数上限，避免先截断后过滤导致漏卡
    const lmActive = limitFilter !== '0';
    let results = await WailsBridge.searchCards({
      keyword: searchKeyword.trim(),
      multiKeywords: Number(settingsStore.get('search_multiple_keywords')) || 0,
      type,
      race: isMonsterKind ? parseInt(raceFilter, 10) || 0 : 0,
      attribute: isMonsterKind ? parseInt(attrFilter, 10) || 0 : 0,
      levelFilter: isMonsterKind ? starFilter.trim() : '',
      atkFilter: isMonsterKind ? atkFilter.trim() : '',
      defFilter: isMonsterKind && !isLinkType2 ? defFilter.trim() : '',
      scaleFilter: isMonsterKind ? scaleFilter.trim() : '',
      effect: effectMask,
      linkMarks,
      limit: lmActive || typeExact ? 500 : 60,
    });
    results = results || [];
    if (typeExact) results = results.filter((c: any) => c.type === typeExact);
    setSearchResults(applyLimitFilter(results));
  };

  // auto_search_limit >= 0：输入满 N 字自动搜索（deck_con.cpp InstantSearch，
  // deck_con.cpp:1594）。300ms 防抖避免逐字打满搜索请求。
  const autoSearchLimit = Number(settingsSnap.auto_search_limit ?? -1);
  const performSearchRef = useRef(performSearch);
  performSearchRef.current = performSearch;
  useEffect(() => {
    if (autoSearchLimit < 0) return undefined;
    if (searchKeyword.trim().length < autoSearchLimit) return undefined;
    const t = setTimeout(() => { performSearchRef.current(); }, 300);
    return () => clearTimeout(t);
  }, [searchKeyword, autoSearchLimit]);

  // 结果排序：主卡组怪兽 → 额外区怪兽 → 魔法 → 陷阱，同区按攻↓、卡名
  const typeOrderOf = (t: number): number =>
    (t & TYPE_MONSTER) ? ((t & EXTRA_TYPES) ? 1 : 0) : (t & TYPE_SPELL) ? 2 : 3;
  const sortedResults = [...searchResults].sort((a, b) =>
    typeOrderOf(a.type || 0) - typeOrderOf(b.type || 0)
    || ((b.attack || 0) - (a.attack || 0))
    || (a.name < b.name ? -1 : 1));

  const addCardToDeck = (card: any, target?: 'main' | 'side'): void => {
    // 右键固定加副卡组；左键按「点击加入」下拉（addTarget）
    const dest = target || addTarget;
    // check_limit：同名卡（alias 归并）达禁限表上限则拒加（原版
    // deck_con.cpp:1834——limit 默认 3，命中 lflist 取 0/1/2）
    const group = card.alias || groupOf(card.code);
    const limit = limitOf(group);
    if (countInDeck(currentDeck, group) >= limit) {
      alert(limit === 0 ? '此卡在当前禁限卡表中被禁止，不能加入卡组。'
        : limit < 3 ? `同名卡在「${lfName || '禁限卡表'}」中最多 ${limit} 张。`
        : '同名卡已达 3 张上限。');
      return;
    }
    const isExtra = (card.type & EXTRA_TYPES) !== 0;
    const deck = { ...currentDeck, main: [...currentDeck.main], extra: [...currentDeck.extra], side: [...currentDeck.side] };
    if (isExtra) {
      if (deck.extra.length >= 15) { alert('额外卡组已满（最多 15 张）。'); return; }
      deck.extra.push(card.code);
    } else if (dest === 'side') {
      if (deck.side.length >= 15) { alert('副卡组已满（最多 15 张）。'); return; }
      deck.side.push(card.code);
    } else {
      if (deck.main.length >= 60) { alert('主卡组已满（最多 60 张）。'); return; }
      deck.main.push(card.code);
    }
    logEdit(`加入 ${card.name || card.code} → ${isExtra ? '额外卡组' : dest === 'side' ? '副卡组' : '主卡组'}`);
    applyDeck(deck);
  };

  const removeCardFromDeck = (section: DeckSection, index: number): void => {
    const deck = { ...currentDeck, main: [...currentDeck.main], extra: [...currentDeck.extra], side: [...currentDeck.side] };
    const code = deck[section][index];
    const info = WailsBridge._cardCache.get(code);
    logEdit(`移出 ${(info && info.name) || code} ← ${{ main: '主卡组', extra: '额外卡组', side: '副卡组' }[section]}`);
    deck[section].splice(index, 1);
    applyDeck(deck);
  };

  // push_* / pop_* 语义（deck_con.cpp:1773-1833）：主卡组拒绝额外怪兽、额外只收
  // 融合/同调/超量/连接、副卡组不限；上限 60/15/15。区内拖动=插到目标位之前。
  const moveCard = (from: DeckSection, fromIndex: number, to: DeckSection, toIndex: number): void => {
    if (fromIndex < 0 || fromIndex >= currentDeck[from].length) return;
    const deck = { ...currentDeck, main: [...currentDeck.main], extra: [...currentDeck.extra], side: [...currentDeck.side] };
    const code = deck[from][fromIndex];
    const info = WailsBridge._cardCache.get(code);
    const isExtraType = !!(info && (info.type & EXTRA_TYPES));
    if (from !== to) {
      if (to === 'main' && isExtraType) { alert('额外卡组的怪兽不能放入主卡组。'); return; }
      if (to === 'extra' && !isExtraType) { alert('只有融合/同调/超量/连接怪兽能进额外卡组。'); return; }
      const max = to === 'main' ? 60 : 15;
      if (deck[to].length >= max) { alert(to === 'main' ? '主卡组已满（最多 60 张）。' : '该卡组已满（最多 15 张）。'); return; }
    }
    deck[from].splice(fromIndex, 1);
    const insertAt = Math.min(Math.max(toIndex, 0), deck[to].length);
    deck[to].splice(insertAt, 0, code);
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

  // ---- 卡组码导入导出（deck_con.cpp BUTTON_IMPORT/EXPORT_DECK_CODE）----

  // 导出：序列化为原版 ydk 文本复制到剪贴板；Wails/受限环境没有剪贴板
  // 权限时弹文本框供手动复制（原版 wACMessage 提示 1480，这里失败才弹框）。
  const exportDeckCode = async (): Promise<void> => {
    const text = serializeDeckYdk(currentDeck);
    try {
      if (!navigator.clipboard || !navigator.clipboard.writeText) throw new Error('no clipboard');
      await navigator.clipboard.writeText(text);
      alert('卡组码已复制到剪贴板。');
    } catch {
      setExportText(text);
    }
  };

  // 导入：粘贴的 ydk 文本解析进编辑器（不自动存盘——标脏，走现有保存流程）。
  // 覆盖当前编辑内容前沿用未存保护确认。
  const importDeckCode = (): void => {
    const parsed = parseDeckYdk(importText);
    if (!parsed) { alert('无法识别卡组码：请粘贴 #main/#extra/!side 格式的 ydk 文本。'); return; }
    if (discardGuard()) return;
    if (parsed.main.length === 0 && parsed.extra.length === 0 && parsed.side.length === 0) {
      alert('卡组码是空的。');
      return;
    }
    const total = parsed.main.length + parsed.extra.length + parsed.side.length;
    applyDeck({ name: '导入卡组', ...parsed }, { modified: true });
    setDeckName('导入卡组');
    setLoadedDeck('');
    logEdit(`导入卡组码：主 ${parsed.main.length} / 额外 ${parsed.extra.length} / 副 ${parsed.side.length}（共 ${total} 张）`);
    setShowImport(false);
    setImportText('');
  };

  // wDeckManage 操作后刷新清单；已载入卡组被改名/移动/删除时脱离关联
  // （编辑内容保留，退化为未保存状态，与原版 prev_deck 失效语义一致）
  const onManageRefresh = (list: string[]): void => {
    setDeckList(list);
    if (loadedDeck && !list.includes(loadedDeck)) setLoadedDeck('');
  };

  // 效果复选框切换（wCategories → BUTTON_CATEGORY_OK 汇位掩码）
  const toggleEffectBit = (bit: number): void => {
    const next = new Set(effectBits);
    if (next.has(bit)) next.delete(bit); else next.add(bit);
    setEffectBits(next);
  };

  // 卡组网格几何 = 原版 DrawDeckBd（drawing.cpp:1228-1295）设计像素：
  // 缩略格 44×64（CARD_THUMB game.h:26-27），行距 dy=68，列距 dx=436/9（≤40 张
  // 主卡组 10 列）；主卡组 >40 张后每多 4 张多 1 列（lx=(n-41)/4+11）、
  // dx=436/(lx-1) 压缩层叠；额外/副单行，>10 张时 dx=436/(n-1) 同法压缩。
  const DECK_DX_SPAN = 436;
  const mainCols = (n: number): number => (n <= 40 ? 10 : Math.floor((n - 41) / 4) + 11);

  const renderSection = (codes: number[], section: DeckSection) => {
    // 每行内：首张无偏移，后续每张 marginLeft = dx - 44（负数即层叠在左卡之上）。
    // idx = 全局索引（增删/拖拽用），col = 行内索引（层叠偏移用）
    const chipAt = (code: number, idx: number, col: number, step: number) => (
      <CardChip
        key={`${code}-${idx}`}
        code={code}
        count={counts[section][(WailsBridge._cardCache.get(code)?.alias) || code]}
        lfLimit={limitOf(groupOf(code))}
        style={{ marginLeft: col > 0 ? step - 44 : 0 }}
        onInspect={setInspected}
        onRemove={() => removeCardFromDeck(section, idx)}
        onZoom={setZoomCode}
        onContextMenu={(e) => { e.preventDefault(); setCtxMenu({ x: e.clientX, y: e.clientY, section, index: idx }); }}
        onDragStart={(e) => {
          draggedRef.current = { section, index: idx };
          e.dataTransfer.effectAllowed = 'move';
          e.dataTransfer.setData('text/plain', String(code));
        }}
        onDrop={(e) => {
          e.preventDefault();
          const d = draggedRef.current;
          if (d) moveCard(d.section, d.index, section, idx);
        }}
      />
    );

    // 主卡组按 lx 列折行；额外/副卡组单行
    const lx = section === 'main' ? mainCols(codes.length) : Math.max(codes.length, 1);
    const step = section === 'main'
      ? (codes.length <= 40 ? DECK_DX_SPAN / 9 : DECK_DX_SPAN / (lx - 1))
      : (codes.length <= 10 ? DECK_DX_SPAN / 9 : DECK_DX_SPAN / (codes.length - 1));
    const rows: number[][] = [];
    for (let i = 0; i < codes.length; i += lx) rows.push(codes.slice(i, i + lx));

    return (
      <div
        className={`deck-grid ds-grid-${section}`}
        onDragOver={(e) => e.preventDefault()}
        onDrop={(e) => {
          e.preventDefault();
          const d = draggedRef.current;
          if (d) moveCard(d.section, d.index, section, codes.length);
        }}
      >
        {rows.map((row, r) => (
          <div className="deck-grid-row" key={r}>
            {row.map((code, i) => chipAt(code, r * lx + i, i, step))}
          </div>
        ))}
      </div>
    );
  };

  // gframe 数值过滤是单输入框 + 运算符前缀（ebAttack/ebDefense/ebStar/ebScale）
  const statInput = (id: string, placeholder: string, value: string, setter: (v: string) => void, disabled = false) => (
    <UiInput
      id={id}
      className="deck-filter-num"
      type="text"
      placeholder={placeholder}
      title="支持运算符：3000 / =3000 / >=2500 / <1500 / ?（占位卡）"
      value={value}
      disabled={disabled}
      onChange={(e) => setter(e.target.value)}
    />
  );

  return (
    <div id="deck-screen" className="screen active">
      {/* 原版 1024×640 设计坐标系（game.cpp Resize 非均匀拉伸语义）：
          控件按 game.cpp / drawing.cpp 的设计像素坐标绝对落位 */}
      <DesignSpace>
      {/* 左列：wCardImg 框 (1,1)-(198,273)（game.cpp:335），卡图在内 (10,9)-(187,263)；
          wInfos 页签信息 (1,275)-(301,639)（game.cpp:356） */}
      <div className="card-inspector">
        <div className="inspector-pic-frame">
        <div className="inspector-pic-box">
          {inspectedPic
            ? <img id="deck-inspector-pic" src={inspectedPic} alt={inspected.name} draggable={false}
                style={{ width: '100%', height: '100%', objectFit: 'contain', display: 'block' }} />
            : <div style={{
                width: '100%', height: '100%', background: '#0f172a',
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                fontSize: '11px', color: '#38bdf8',
              }}>{inspected ? '卡图加载中……' : '卡牌详情'}</div>}
        </div>
        </div>
        <div className="inspector-infos">
        <div className="inspector-tabs" role="tablist">
          <button
            role="tab"
            className={`inspector-tab${inspectorTab === 'info' ? ' active' : ''}`}
            onClick={() => setInspectorTab('info')}
          >卡片信息</button>
          <button
            role="tab"
            className={`inspector-tab${inspectorTab === 'log' ? ' active' : ''}`}
            onClick={() => setInspectorTab('log')}
          >日志</button>
        </div>
        {inspectorTab === 'log' ? (
          <div className="inspector-details inspector-log" id="deck-inspector-log">
            {editLog.length === 0 ? (
              <div className="inspector-desc">本次编辑还没有操作记录。</div>
            ) : editLog.slice().reverse().map((entry, i) => (
              <div key={`${editLog.length - i}`} className="inspector-log-line">{entry}</div>
            ))}
          </div>
        ) : (
        <div className="inspector-details">
          <div className="inspector-name">{inspected ? `${inspected.name}[${inspected.code}]` : '卡牌详情'}</div>
          {inspected ? (
            <>
              <div className="inspector-meta inspector-type-line" id="deck-inspector-type">
                {`[${[cardKind(inspected.type || 0), ...cardSubtypes(inspected.type || 0)].filter(Boolean).join('/')}]`}
                {(inspected.type & TYPE_MONSTER) !== 0
                  ? ` ${(RACES.find((r) => r[0] === inspected.race) || [0, ''])[1] as string}/${(ATTRS.find((a) => a[0] === inspected.attribute) || [0, ''])[1] as string}`
                  : ''}
              </div>
              {(inspected.type & TYPE_MONSTER) !== 0 ? (
                <div className="inspector-meta" id="deck-inspector-stats">
                  {(inspected.type & TYPE_LINK)
                    ? `LINK-${inspected.level}`
                    : '★'.repeat(Math.min(inspected.level || 0, 13))}
                  {' '}{fmtStat(inspected.attack)}/{fmtStat(inspected.defense)}
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
        )}
        </div>
      </div>

      {/* btnLeaveGame（game.cpp:949）：(205,5)-(295,80) 竖长退出按钮 */}
      <UiButton className="ds-leave-edit"
        onClick={() => { if (!discardGuard()) onNavigate('menu'); }}>退出编辑</UiButton>
      {/* 起手测试（扩展功能）：置 btnLeaveGame 下方空区 */}
      <UiButton variant="gold" className="ds-test-hand" onClick={simulateSampleHand}>测试起手 5 张</UiButton>

      {/* wDeckEdit (309,5)-(605,130)（game.cpp:656-718）。内部按原版排布：
          左列 label(70) + 下拉/输入(140)，右列竖排按钮(65)：管理/保存/另存为/删除；
          底行 打乱/排序/清空 50 宽三连。标题行仅作冒烟契约（h2 带 * = 未保存），
          视觉隐藏——原版 wDeckEdit 无标题栏 */}
      <div className="deck-header">
            <div className="deck-header-title">
              <h2>卡组构筑{isModified ? ' *' : ''}</h2>
              <span id="deck-lflist-name">
                禁限卡表：{lfName || 'N/A'}
              </span>
            </div>
            <div className="deck-header-row">
              <label className="gfw-label dh-label">卡组分类</label>
              {/* 分类下拉（gframe cbDBCategory：未分类卡组 + ./deck/ 子目录）。
                  Radix Item 不允许空串 value：未分类 '' 用哨兵 '__root__' 表示 */}
              <GfwSelect
                id="deck-category-select"
                width={140}
                value={category || '__root__'}
                onValueChange={(v) => {
                  const cat = v === '__root__' ? '' : v;
                  if (!discardGuard()) { setCategory(cat); }
                }}
                options={[
                  { value: '__root__', label: '未分类卡组' },
                  ...categories.map((c) => ({ value: c, label: c })),
                ]}
              />
              {/* btnManageDeck (225,5)-(290,30) */}
              <UiButton id="deck-manage-btn" className="dh-right" onClick={() => setShowManage(true)}>卡组管理</UiButton>
            </div>
            <div className="deck-header-row">
              <label className="gfw-label dh-label">卡组</label>
              <GfwSelect
                id="deck-select"
                width={140}
                value={decksInCategory.includes(loadedDeck) ? loadedDeck : ''}
                onValueChange={(v) => { if (v && !discardGuard()) loadDeck(v); }}
                options={decksInCategory.map((n) => ({ value: n, label: n.split('/').pop() || n }))}
              />
              {/* btnSaveDeck (225,35)-(290,60) */}
              <UiButton className="dh-right" onClick={saveDeckOverwrite}>保存</UiButton>
            </div>
            <div className="deck-header-row">
              {/* ebDeckname (80,65)-(220,90)：原版无 label，缩进对齐下拉列 */}
              <span className="dh-spacer" />
              <UiInput style={{ width: '140px' }} placeholder="卡组名" title="卡组名"
                value={deckName} onChange={(e) => setDeckName(e.target.value)} />
              {/* btnSaveDeckAs (225,65)-(290,90) */}
              <UiButton className="dh-right" onClick={saveDeck}>另存为</UiButton>
            </div>
            <div className="deck-header-row">
              {/* 编辑器三键 (5/60/115,99)×50（deck_con.cpp:172-191） */}
              <UiButton id="deck-shuffle-btn" className="dh-trio" onClick={shuffleDeck}>打乱</UiButton>
              <UiButton id="deck-sort-btn" className="dh-trio" onClick={sortDeck}>排序</UiButton>
              <UiButton id="deck-clear-btn" className="dh-trio" onClick={clearDeck}>清空</UiButton>
              {/* btnDeleteDeck (225,95)-(290,120) */}
              <UiButton variant="danger" className="dh-right" onClick={deleteDeck}>删除</UiButton>
            </div>
            <div className="deck-header-row">
              {/* 扩展件（原版在 wDeckManage 内：btnNewDeck/btnExport/btnImport） */}
              <UiButton onClick={() => { if (!discardGuard()) newDeck(); }}>新建卡组</UiButton>
              <UiButton id="deck-export-code-btn" onClick={exportDeckCode}>导出卡组码</UiButton>
              <UiButton id="deck-import-code-btn" onClick={() => { setImportText(''); setShowImport(true); }}>导入卡组码</UiButton>
            </div>
          </div>

          {/* wFilter（game.cpp:740-813）内部 5 行网格：
              R1 种类/副种类/禁限，R2 属性/攻击，R3 种族/守备，R4 星数/刻度/卡名，
              R5 连接标记/清空/开始搜索；效果过滤按钮跨 R2-R3 右列 */}
          <div className="deck-search-panel">
            <span className="gfw-label fw-l-type">种类</span>
            <div className="fw-type1">
              <GfwSelect
                id="deck-filter-type1"
                aria-label="卡片种类过滤"
                value={typeFilter}
                onValueChange={onType1Change}
                options={[
                  { value: '0', label: '（无）' }, { value: '1', label: '怪兽' },
                  { value: '2', label: '魔法' }, { value: '3', label: '陷阱' },
                ]}
              />
            </div>
            <div className="fw-type2">
              <GfwSelect
                id="deck-filter-type2"
                aria-label="副种类过滤"
                disabled={type2Disabled}
                value={type2Filter}
                onValueChange={onType2Change}
                options={[
                  { value: '0', label: '（无）' },
                  ...(TYPE2_OPTIONS[typeFilter] || []).map(([mask, label]) => ({ value: String(mask), label })),
                ]}
              />
            </div>
            <span className="gfw-label fw-l-limit">禁限</span>
            <div className="fw-limit">
              <GfwSelect
                id="deck-filter-limit"
                aria-label="禁限过滤"
                value={limitFilter}
                onValueChange={setLimitFilter}
                options={LIMIT_OPTIONS.map((label, i) => ({ value: String(i), label }))}
              />
            </div>
            <span className="gfw-label fw-l-attr">属性</span>
            <div className="fw-attr">
              <GfwSelect
                id="deck-filter-attr"
                aria-label="属性过滤"
                disabled={statsDisabled}
                value={attrFilter}
                onValueChange={setAttrFilter}
                options={[{ value: '0', label: '（无）' }, ...ATTRS.map(([v, name]) => ({ value: String(v), label: String(name) }))]}
              />
            </div>
            <span className="gfw-label fw-l-atk">攻击</span>
            <div className="fw-atk">{statInput('deck-filter-atk', '', atkFilter, setAtkFilter, statsDisabled)}</div>
            {/* 效果类型过滤（wCategories 32 复选框浮窗，game.cpp:796 (600,55)-(1000,300)） */}
            <UiButton
              id="deck-filter-effect-btn"
              className="fw-effect"
              onClick={() => setShowEffectPanel((v) => !v)}
            >
              效果{effectBits.size > 0 ? `（${effectBits.size}）` : ''}
            </UiButton>
            <span className="gfw-label fw-l-race">种族</span>
            <div className="fw-race">
              <GfwSelect
                id="deck-filter-race"
                aria-label="种族过滤"
                disabled={statsDisabled}
                value={raceFilter}
                onValueChange={setRaceFilter}
                options={[{ value: '0', label: '（无）' }, ...RACES.map(([v, name]) => ({ value: String(v), label: String(name) }))]}
              />
            </div>
            <span className="gfw-label fw-l-def">守备</span>
            <div className="fw-def">{statInput('deck-filter-def', '', defFilter, setDefFilter, statsDisabled || isLinkType2)}</div>
            <span className="gfw-label fw-l-star">星数</span>
            <div className="fw-starscale">
              {statInput('deck-filter-star', '', starFilter, setStarFilter, statsDisabled)}
              <span className="gfw-label">刻度</span>
              {statInput('deck-filter-scale', '', scaleFilter, setScaleFilter, statsDisabled)}
            </div>
            <span className="gfw-label fw-l-keyword">卡名</span>
            <div className="fw-keyword">
              <UiInput
                id="deck-search-input"
                type="text"
                placeholder="名称/效果/编号；$卡名 @系列名"
                value={searchKeyword}
                onChange={(e) => setSearchKeyword(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter') performSearch(); }}
              />
            </div>
            {/* btnMarksFilter（game.cpp:813）：弹 wLinkMarks；有掩码时呈按下态 */}
            <UiButton
              id="deck-marks-filter-btn"
              className="fw-marks"
              variant={linkMarks > 0 ? 'gold' : 'default'}
              onClick={toggleLinkMarksPopup}
            >
              连接标记{linkMarks > 0 ? `（${linkMarks}）` : ''}
            </UiButton>
            {/* separate_clear_button（game.cpp:792-794）：关闭时开始搜索占满整行 */}
            {!!settingsSnap.separate_clear_button && (
              <UiButton id="deck-filter-clear" className="fw-clear" onClick={clearFilters}>清空</UiButton>
            )}
            <UiButton
              className={settingsSnap.separate_clear_button ? 'fw-search' : 'fw-search-wide'}
              onClick={performSearch}
            >搜索</UiButton>
          </div>

          {/* wCategories 效果复选浮窗（(600,55)-(1000,300)，32 复选框） */}
          {showEffectPanel ? (
            <div id="deck-effect-panel" className="ds-effect-popup">
              {EFFECT_CATEGORIES.map(([bit, label]) => (
                <label key={bit} style={{ display: 'flex', alignItems: 'center', gap: '2px', cursor: 'pointer' }}>
                  <UiCheckbox checked={effectBits.has(bit)} onChange={() => toggleEffectBit(bit)} />
                  {label}
                </label>
              ))}
              <UiButton
                style={{ gridColumn: '1 / -1', marginTop: '4px' }}
                onClick={() => setShowEffectPanel(false)}
              >
                确定
              </UiButton>
            </div>
          ) : null}

          {/* wLinkMarks 连接标记浮窗（game.cpp:814-828：(700,30)-(820,150)，
              8 箭头 30×30 按 35 间距排布，中央 btnMarksOK 确定提交） */}
          {showLinkMarks ? (
            <div id="deck-linkmarks-panel" className="ds-linkmarks-popup">
              {LINK_MARKS.slice(0, 3).map(([bit, arrow]) => (
                <UiButton key={bit} variant={(linkMarksPending & bit) !== 0 ? 'gold' : 'default'}
                  className="ds-lm-btn" title="按住的箭头方向都要命中（子集语义）"
                  onClick={() => togglePendingMark(bit)}>{arrow}</UiButton>
              ))}
              <UiButton variant={(linkMarksPending & LINK_MARKS[3][0]) !== 0 ? 'gold' : 'default'}
                className="ds-lm-btn" title="按住的箭头方向都要命中（子集语义）"
                onClick={() => togglePendingMark(LINK_MARKS[3][0])}>{LINK_MARKS[3][1]}</UiButton>
              <UiButton className="ds-lm-btn ds-lm-ok" onClick={commitLinkMarks}>确定</UiButton>
              <UiButton variant={(linkMarksPending & LINK_MARKS[4][0]) !== 0 ? 'gold' : 'default'}
                className="ds-lm-btn" title="按住的箭头方向都要命中（子集语义）"
                onClick={() => togglePendingMark(LINK_MARKS[4][0])}>{LINK_MARKS[4][1]}</UiButton>
              {LINK_MARKS.slice(5).map(([bit, arrow]) => (
                <UiButton key={bit} variant={(linkMarksPending & bit) !== 0 ? 'gold' : 'default'}
                  className="ds-lm-btn" title="按住的箭头方向都要命中（子集语义）"
                  onClick={() => togglePendingMark(bit)}>{arrow}</UiButton>
              ))}
            </div>
          ) : null}

        {/* 主/额外/副卡组区（DrawDeckBd drawing.cpp:1228-1295 设计坐标）：
            标题短条 (309,136/439/536 起 102×22) + 网格区直接铺在背景上 */}
        <div className="deck-section-title ds-title-main">
          <span>主卡组：</span>
          <span className="badge badge-attr">{currentDeck.main.length} / 60</span>
        </div>
        {renderSection(currentDeck.main, 'main')}
        <div className="deck-section-title ds-title-extra">
          <span>额外卡组：</span>
          <span className="badge badge-type">{currentDeck.extra.length} / 15</span>
        </div>
        {renderSection(currentDeck.extra, 'extra')}
        <div className="deck-section-title ds-title-side">
          <span>副卡组：</span>
          <span className="badge badge-attr">{currentDeck.side.length} / 15</span>
        </div>
        {renderSection(currentDeck.side, 'side')}

        {/* 搜索结果区（DrawDeckBd drawing.cpp:1296-1360）：标题条 + 纵向列表 */}
        <div className="deck-results">
          <div id="deck-result-count" title="点击添加，右键加副卡组，双击看大图">
            搜索结果（{sortedResults.length} 张）
          </div>
          <div className="deck-add-target-row">
            <label>点击加入：</label>
            <GfwSelect
              id="deck-add-target"
              value={addTarget}
              onValueChange={(v) => setAddTarget(v as 'main' | 'side')}
              options={[{ value: 'main', label: '主卡组' }, { value: 'side', label: '副卡组' }]}
            />
          </div>
          <div className="search-results-grid">
            {sortedResults.map((card, i) => (
              <SearchResultChip
                key={`${card.code}-${i}`}
                card={card}
                lfLimit={limitOf(card.alias || groupOf(card.code))}
                onInspect={setInspected}
                onAdd={() => addCardToDeck(card)}
                onAddSide={() => addCardToDeck(card, 'side')}
                onZoom={setZoomCode}
              />
            ))}
          </div>
        </div>
      </DesignSpace>

      {zoomCode ? <ZoomOverlay code={zoomCode} onClose={() => setZoomCode(0)} /> : null}

      {/* 导出卡组码的剪贴板回退：无剪贴板权限时给文本框手动复制 */}
      {exportText !== null ? (
        <div
          id="deck-export-overlay"
          style={{
            position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.7)',
            display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 2200,
          }}
          onClick={() => setExportText(null)}
        >
          <div className="modal-box" style={{ width: 420, maxWidth: '90vw' }} onClick={(e) => e.stopPropagation()}>
            <div className="modal-title">导出卡组码</div>
            <div style={{ fontSize: 12, color: 'var(--text-muted)', margin: '6px 0' }}>
              当前环境无法直接写剪贴板，请手动复制下面的文本：
            </div>
            <textarea
              id="deck-export-text"
             className="form-input"
              readOnly
              style={{ width: '100%', height: 200, fontFamily: 'monospace', fontSize: 12 }}
              value={exportText}
              onFocus={(e) => e.target.select()}
            />
            <div style={{ textAlign: 'right', marginTop: 6 }}>
              <UiButton id="deck-export-close" onClick={() => setExportText(null)}>关闭</UiButton>
            </div>
          </div>
        </div>
      ) : null}

      {/* 导入卡组码：粘贴 ydk 文本（原版从剪贴板读，这里显式粘贴框跨环境一致） */}
      {showImport ? (
        <div
          id="deck-import-overlay"
          style={{
            position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.7)',
            display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 2200,
          }}
          onClick={() => setShowImport(false)}
        >
          <div className="modal-box" style={{ width: 420, maxWidth: '90vw' }} onClick={(e) => e.stopPropagation()}>
            <div className="modal-title">导入卡组码</div>
            <div style={{ fontSize: 12, color: 'var(--text-muted)', margin: '6px 0' }}>
              粘贴 ydk 格式卡组码（#main / #extra / !side 分段），导入后替换当前编辑内容：
            </div>
            <textarea
              id="deck-import-text"
              className="form-input"
              style={{ width: '100%', height: 200, fontFamily: 'monospace', fontSize: 12 }}
              placeholder={'#created by ...\n#main\n89631139\n#extra\n!side'}
              value={importText}
              onChange={(e) => setImportText(e.target.value)}
            />
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 6, marginTop: 6 }}>
              <UiButton id="deck-import-cancel" onClick={() => setShowImport(false)}>取消</UiButton>
              <UiButton id="deck-import-confirm" variant="gold" onClick={importDeckCode}>导入</UiButton>
            </div>
          </div>
        </div>
      ) : null}

      {/* wDeckManage：分类/卡组管理窗口 */}
      {showManage ? (
        <DeckManageModal
          deckList={deckList}
          onClose={() => setShowManage(false)}
          onRefresh={onManageRefresh}
        />
      ) : null}

      {/* 右键上下文菜单（deck_con.cpp:1129-1195 的 pop_* / 移副语义） */}
      {ctxMenu ? (
        <>
          <div
            style={{ position: 'fixed', inset: 0, zIndex: 2099 }}
            onClick={() => setCtxMenu(null)}
            onContextMenu={(e) => { e.preventDefault(); setCtxMenu(null); }}
          />
          <div id="deck-context-menu" style={{
            position: 'fixed', left: ctxMenu.x, top: ctxMenu.y, zIndex: 2100,
            background: '#1e293b', border: '1px solid var(--primary)', borderRadius: 6,
            padding: '4px', minWidth: 132, boxShadow: '0 4px 12px rgba(0,0,0,0.6)',
            display: 'flex', flexDirection: 'column', gap: '3px',
          }}>
            {ctxMenu.section !== 'main' && (
              <UiButton style={{ textAlign: 'left' }}
                onClick={() => { moveCard(ctxMenu.section, ctxMenu.index, 'main', currentDeck.main.length); setCtxMenu(null); }}>
                移到主卡组
              </UiButton>
            )}
            {ctxMenu.section !== 'extra' && (
              <UiButton style={{ textAlign: 'left' }}
                onClick={() => { moveCard(ctxMenu.section, ctxMenu.index, 'extra', currentDeck.extra.length); setCtxMenu(null); }}>
                移到额外卡组
              </UiButton>
            )}
            {ctxMenu.section !== 'side' && (
              <UiButton style={{ textAlign: 'left' }}
                onClick={() => { moveCard(ctxMenu.section, ctxMenu.index, 'side', currentDeck.side.length); setCtxMenu(null); }}>
                移到副卡组
              </UiButton>
            )}
            <UiButton variant="danger" style={{ textAlign: 'left' }}
              onClick={() => { removeCardFromDeck(ctxMenu.section, ctxMenu.index); setCtxMenu(null); }}>
              移出卡组
            </UiButton>
          </div>
        </>
      ) : null}
    </div>
  );
}
