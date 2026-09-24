/**
 * 换副卡组界面（原版 wDeckManager 换侧流程的逐张版，deck_con.cpp is_siding 语义）。
 *
 * STOC_CHANGE_SIDE 后（三局两胜局间）：
 * - 起点 = 上一局实际使用的卡组（发送 CTOS_UPDATE_DECK 时缓存于
 *   side_deck_state；缓存缺失时回退到首个预存卡组）；
 * - 主/额外 ↔ 副 逐张点击或拖拽互换。编辑期容量按原版 siding 规则放宽
 *   +5（deck_con.cpp:1777/1792/1805），让"先取出、再补回"的两步操作可行；
 * - 「完成」前数量必须回到上一局的主/额外/副计数（原版 BUTTON_SIDE_OK +
 *   SysString 1410），与服务端 LoadSide 的校验一致；
 * - 确认后发 CTOS_UPDATE_DECK（mainList = 主卡组+额外），服务端用它作为
 *   "再准备"信号（single_duel.go:392-400），齐后回 STOC_DUEL_START。
 */
import React, { useEffect, useRef, useState } from 'react';
import { eventBus, WailsBridge } from '../wails_bridge.ts';
import { EXTRA_TYPES } from '../domain/constants.ts';
import { getLastSentDeck, setLastSentDeck, SideDeckSnapshot } from '../duel/side_deck_state.ts';
import GfwSelect from './ui/GfwSelect.tsx';

// 编辑期容量放宽量（原版 is_siding ? MAX+5）
const SIDE_SLACK = 5;

interface SideDeckState extends SideDeckSnapshot {}

const cloneDeck = (d: SideDeckState): SideDeckState => ({ main: [...d.main], extra: [...d.extra], side: [...d.side] });

export default function SideDecking() {
  const [visible, setVisible] = useState(false);
  const [waiting, setWaiting] = useState(false);
  const [decks, setDecks] = useState<string[]>([]);
  const [picked, setPicked] = useState('');
  const [deck, setDeck] = useState<SideDeckState>({ main: [], extra: [], side: [] });
  const [original, setOriginal] = useState<SideDeckState>({ main: [], extra: [], side: [] });
  // 上一局的各区计数（完成时必须严格一致，否则服务端 LoadSide 拒绝）
  const [pre, setPre] = useState({ main: 0, extra: 0, side: 0 });
  const [error, setError] = useState('');
  const draggedRef = useRef<{ section: keyof SideDeckState; index: number } | null>(null);

  useEffect(() => {
    const onChangeSide = async () => {
      setVisible(true);
      setWaiting(false);
      setError('');
      let base: SideDeckState = { main: [], extra: [], side: [] };
      const cached = getLastSentDeck();
      if (cached) {
        // 正常路径：以本局准备时发送的卡组为起点
        base = cached;
      } else {
        // 缓存缺失（如重连后直进换侧）：回退到首个预存卡组
        try {
          const list = await WailsBridge.listDecks();
          setDecks(list || []);
          setPicked((prev) => prev || (list && list[0]) || '');
          if (list && list[0]) {
            const d = await WailsBridge.loadDeck(list[0]);
            if (d) base = { main: d.main || [], extra: d.extra || [], side: d.side || [] };
          }
        } catch { /* 空卡组起点 */ }
      }
      setDeck(cloneDeck(base));
      setOriginal(cloneDeck(base));
      setPre({ main: base.main.length, extra: base.extra.length, side: base.side.length });
    };
    const onDuelStart = () => {
      // 服务端已开下一局（双方都确认了卡组）
      setVisible(false);
    };
    eventBus.on('stoc:change_side', onChangeSide);
    eventBus.on('stoc:duel_start', onDuelStart);
    return () => {
      eventBus.off('stoc:change_side', onChangeSide);
      eventBus.off('stoc:duel_start', onDuelStart);
    };
  }, []);

  if (!visible) return null;

  // 同铭计数上限沿用了默认 3（siding 换副时 lflist 已由服务端在 LoadSide 后
  // 于下一局 CheckDeck？——不检查；原版 siding 编辑器也不做禁限校验）
  const targetOf = (section: keyof SideDeckState): number => pre[section];

  // 主/额外 → 副；副 → 额外（额外怪兽）或主
  const swap = (section: keyof SideDeckState, index: number): void => {
    const code = deck[section][index];
    const next = cloneDeck(deck);
    next[section].splice(index, 1);
    if (section !== 'side') {
      if (next.side.length >= targetOf('side') + SIDE_SLACK) { setError(`副卡组最多临时放 ${targetOf('side') + SIDE_SLACK} 张`); return; }
      next.side.push(code);
    } else {
      const info = WailsBridge._cardCache.get(code);
      const isExtra = !!(info && (info.type & EXTRA_TYPES));
      const to: keyof SideDeckState = isExtra ? 'extra' : 'main';
      if (next[to].length >= targetOf(to) + SIDE_SLACK) { setError(`${to === 'main' ? '主卡组' : '额外卡组'}最多临时放 ${targetOf(to) + SIDE_SLACK} 张`); return; }
      next[to].push(code);
    }
    setError('');
    setDeck(next);
  };

  const loadPickedDeck = async (): Promise<void> => {
    if (!picked) return;
    try {
      const d = await WailsBridge.loadDeck(picked);
      if (!d) { setError('卡组加载失败'); return; }
      setDeck({ main: [...d.main], extra: [...d.extra], side: [...d.side] });
      setError('');
    } catch {
      setError('卡组加载失败');
    }
  };

  const confirm = (): void => {
    if (deck.main.length !== pre.main || deck.extra.length !== pre.extra || deck.side.length !== pre.side) {
      setError(`主卡组（当前 ${deck.main.length} / 需 ${pre.main}）、额外卡组（${deck.extra.length} / ${pre.extra}）、副卡组（${deck.side.length} / ${pre.side}）数量须与上一局一致`);
      return;
    }
    setWaiting(true);
    setLastSentDeck(deck);
    WailsBridge.updateDeck([...deck.main, ...deck.extra], deck.side);
  };

  // 卡片区（卡图 + 卡名兜底），点击=按 siding 语义换区，支持 HTML5 拖拽
  const renderSection = (codes: number[], section: keyof SideDeckState, title: string): React.ReactNode => (
    <div className="side-deck-section">
      <div className="side-deck-section-title">
        <span>{title}</span>
        <span className={`side-deck-count${codes.length === targetOf(section) ? ' ok' : ' warn'}`}>
          {codes.length} / 需 {targetOf(section)}
        </span>
      </div>
      <div
        className="side-deck-grid"
        onDragOver={(e) => e.preventDefault()}
        onDrop={(e) => {
          e.preventDefault();
          const d = draggedRef.current;
          if (d && d.section !== section) swap(d.section, d.index);
        }}
      >
        {codes.map((code, i) => (
          <SideChip
            key={`${code}-${i}`}
            code={code}
            onSwap={() => swap(section, i)}
            onDragStart={() => { draggedRef.current = { section, index: i }; }}
          />
        ))}
        {codes.length === 0 ? <div className="side-deck-empty">（空）</div> : null}
      </div>
    </div>
  );

  return (
    <div id="side-decking" className="side-decking">
      <div className="side-decking-box side-decking-box-wide">
        <div className="side-decking-title">换副卡组</div>
        <div className="side-decking-hint">点击或拖拽卡片在主/额外卡组与副卡组之间逐张交换；完成后各区数量须与上一局一致</div>
        {renderSection(deck.main, 'main', '主卡组')}
        {renderSection(deck.extra, 'extra', '额外卡组')}
        {renderSection(deck.side, 'side', '副卡组')}
        <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
          <GfwSelect
            id="side-deck-select"
            flex={1}
            value={picked}
            onValueChange={setPicked}
            options={decks.map((d) => ({ value: d, label: d }))}
          />
          <button className="btn btn-secondary" onClick={loadPickedDeck} disabled={!picked}>载入预存卡组</button>
          <button className="btn btn-secondary" onClick={() => { setDeck(cloneDeck(original)); setError(''); }}>还原</button>
        </div>
        {error && <div className="side-decking-error">{error}</div>}
        <button id="side-deck-confirm" className="btn btn-gold" disabled={waiting} onClick={confirm}>
          {waiting ? '等待对方完成换副卡组...' : '副卡组更换完成'}
        </button>
      </div>
    </div>
  );
}

// 单卡缩略格：卡图优先，无图退化为卡名块；卡信息拉一次进 _cardCache
// （副卡组卡判定额外怪兽要用 type）
function SideChip({ code, onSwap, onDragStart }: {
  code: number;
  onSwap: () => void;
  onDragStart: () => void;
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
      className="side-deck-chip"
      style={{ cursor: 'pointer' }}
      draggable
      onDragStart={(e) => { onDragStart(); e.dataTransfer.effectAllowed = 'move'; e.dataTransfer.setData('text/plain', String(code)); }}
      onClick={onSwap}
      title={info ? `${info.name}（点击换区）` : String(code)}
    >
      {pic
        ? <img src={pic} alt={info ? info.name : String(code)} draggable={false}
            style={{ width: '100%', height: '100%', objectFit: 'cover', display: 'block' }} />
        : <div className="side-deck-chip-fallback">{info ? info.name : code}</div>}
    </div>
  );
}
