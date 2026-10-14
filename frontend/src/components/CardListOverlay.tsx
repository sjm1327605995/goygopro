/**
 * 卡片列表浮窗（gframe wCardDisplay 的翻译，event_handler.cpp:2071-2110）。
 *
 * 数据源：duel:hotkeys.ts 在 F1-F8 松开时发 ui:show_pile {player, pile}。
 * player 0=自己 1=对方（store 的 board 已经做过座位映射，直接下标）；
 * pile grave/banish/extra/overlay。
 *
 * 原版语义：列表按 rbegin→rend 倒序（最新进的排最前）、标题 "%ls(%zu)"、
 * 悬停卡片左侧预览面板同步查看（ClickCard→ShowCardInfo）。浏览器翻译：
 * 再按同一键 / ESC / × 关闭；按其他键切换列表。
 */
import React, { useEffect, useMemo, useState, useSyncExternalStore } from 'react';
import { eventBus, WailsBridge } from '../wails_bridge.ts';
import { duelStore } from '../duel/store.ts';
import { bottomEngineSeat } from '../domain/reducer.ts';
import { LOC_GRAVE, LOC_BANISH, LOC_EXTRA } from '../domain/constants.ts';
import { CardTile } from './prompts/CardTile.tsx';

export type ShowPile = 'grave' | 'banish' | 'extra' | 'overlay';

export interface ShowPileEvent {
  player: number;
  pile: ShowPile;
}

const PILE_LABEL: Record<ShowPile, string> = {
  grave: '墓地',
  banish: '除外',
  extra: '额外',
  overlay: '叠放',
};

// 堆区名 → ocgcore LOCATION 位（cmdActions 条目的 l 域匹配用）
const PILE_LOC: Record<string, number> = {
  grave: LOC_GRAVE,
  banish: LOC_BANISH,
  extra: LOC_EXTRA,
};

/**
 * 堆区卡的可操作条目（原版 ClientCard::cmdFlag 的 ACTIVATE/SPSUMMON 位，
 * event_handler.cpp:1269-1302 点堆区弹「发动/特殊召唤/查看」菜单的列表形态）：
 * 指令询问进行中（store.cmdActions）且条目座标命中该堆区该序号时返回动作。
 */
function pileCardActions(cmdActions: any, engineSeat: number, pile: ShowPile, seq: number): any[] {
  if (!cmdActions) return [];
  const loc = PILE_LOC[pile];
  if (!loc) return [];
  const acts = (cmdActions.activate || [])
    .filter((a: any) => a.c === engineSeat && a.l === loc && a.s === seq)
    .map((a: any) => ({ kind: 'activate', entry: a }));
  const sps = (cmdActions.spsummon || [])
    .filter((a: any) => a.c === engineSeat && a.l === loc && a.s === seq)
    .map((a: any) => ({ kind: 'spsummon', entry: a }));
  return [...acts, ...sps];
}

export default function CardListOverlay() {
  const state = useSyncExternalStore(duelStore.subscribe, duelStore.getState);
  const [open, setOpen] = useState<ShowPileEvent | null>(null);
  const [names, setNames] = useState<Record<number, string>>({});

  useEffect(() => {
    const onShow = (data: ShowPileEvent): void => {
      if (!data || !data.pile) return;
      const st = duelStore.getState();
      if (data.pile === 'grave' && st.cantCheckGrave) return;
      const side = st.board[data.player];
      if (!side || (data.pile === 'overlay' ? side.overlay.flat().length : side[data.pile].length) === 0) return;
      setOpen((cur) => cur || data);
    };
    eventBus.on('ui:show_pile', onShow);
    return () => { eventBus.off('ui:show_pile', onShow); };
  }, []);

  useEffect(() => {
    if (state.cantCheckGrave) setOpen((cur) => cur?.pile === 'grave' ? null : cur);
  }, [state.cantCheckGrave]);

  // 列表内容（倒序同 gframe rbegin→rend）；overlay = 各 mzone 槽素材合集
  const cards = useMemo<number[]>(() => {
    if (!open) return [];
    const side = state.board[open.player] || null;
    if (!side) return [];
    let codes: number[];
    if (open.pile === 'overlay') {
      codes = side.overlay.flat();
    } else {
      codes = side[open.pile].map((c) => c.code);
    }
    return codes.slice().reverse();
  }, [open, state.board]);

  // 卡名异步回填（getCard 结果由 WailsBridge 缓存）
  useEffect(() => {
    let alive = true;
    const missing = cards.filter((c) => !(c in names));
    if (!missing.length) return undefined;
    Promise.all(missing.map(async (c) => {
      try {
        const info = await WailsBridge.getCard(c);
        return [c, (info && info.name) || `#${c}`] as const;
      } catch {
        return [c, `#${c}`] as const;
      }
    })).then((pairs) => {
      if (!alive) return;
      setNames((cur) => {
        const next = { ...cur };
        pairs.forEach(([c, n]) => { next[c] = n; });
        return next;
      });
    });
    return () => { alive = false; };
  }, [cards]); // eslint-disable-line react-hooks/exhaustive-deps

  if (!open) return null;
  const opponent = open.player === 1;
  const title = `${opponent ? '对方' : ''}${PILE_LABEL[open.pile]}(${cards.length})`;
  // 显示座 → 引擎 seat（cmdActions 条目的 c 域是引擎座标）
  const bottom = bottomEngineSeat(state);
  const engineSeat = open.player === 0 ? bottom : 1 - bottom;

  // 可动作卡的点击：构建 ActionPopup 选项（发动含多效果时解析 desc 文本）
  const onCardAction = async (e: React.MouseEvent, actions: any[]) => {
    const opts: any[] = [];
    const acts = actions.filter((a) => a.kind === 'activate');
    for (const a of acts) {
      let label = '发动';
      if (acts.length > 1 && a.entry.desc) {
        const text = await WailsBridge.resolveDesc(a.entry.desc);
        if (text) label = `发动：${text}`;
      }
      opts.push({ label, action: 'activate', code: a.entry.code, idx: a.entry.idx });
    }
    for (const a of actions.filter((x) => x.kind === 'spsummon')) {
      opts.push({ label: '特殊召唤', action: 'spsummon', code: a.entry.code, idx: a.entry.idx });
    }
    if (opts.length) duelStore.openActionPopup(e.clientX, e.clientY, opts);
  };

  // 原版 wCardDisplay（game.cpp:590-600）：窗口 (320,100)-(1000,400) 680×300，
  // 每卡一列 = 位置标签 120×20 + 卡图 120×170（列距 125），>5 张横向滚动，
  // 底部确定 (300,265)-(380,290)。悬停卡片左侧预览面板同步查看（原版同语义）。
  return (
    <div id="card-display-overlay" className="modal-box wcs-window card-display-overlay">
      <div className="modal-title"><span id="card-display-title">{title}</span></div>
      <div id="card-display-list" className="wcs-cardrow">
        {cards.length === 0
          ? <div className="card-display-empty">（空）</div>
          : cards.map((code, i) => {
            // 列表倒序展示（最新进的排最前）；条目匹配的 s 是堆内正序序号
            const actions = pileCardActions(state.cmdActions, engineSeat, open.pile, cards.length - 1 - i);
            return (
            <div className="wcs-cell" key={`${code}-${i}`}>
              <div className="wcs-pos"></div>
              <div
                className={`card-display-item${actions.length ? ' card-display-actionable' : ''}`}
                onMouseEnter={() => duelStore.inspect(code)}
                onClick={actions.length ? (e) => void onCardAction(e, actions) : undefined}
              >
                <CardTile code={code} name={names[code] || `#${code}`} />
              </div>
            </div>
            );
          })}
      </div>
      <div className="wcs-footer">
        <span />
        <button
          id="card-display-close"
          className="btn btn-gold"
          style={{ width: '80px' }}
          onClick={() => setOpen(null)}
        >确定</button>
      </div>
    </div>
  );
}
