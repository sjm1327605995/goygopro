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

export default function CardListOverlay() {
  const state = useSyncExternalStore(duelStore.subscribe, duelStore.getState);
  const [open, setOpen] = useState<ShowPileEvent | null>(null);
  const [names, setNames] = useState<Record<number, string>>({});

  useEffect(() => {
    const onShow = (data: ShowPileEvent): void => {
      if (!data || !data.pile) return;
      setOpen((cur) =>
        cur && cur.pile === data.pile && cur.player === data.player ? null : data);
    };
    eventBus.on('ui:show_pile', onShow);
    return () => { eventBus.off('ui:show_pile', onShow); };
  }, []);

  // 打开期间 ESC 关闭（原版 ESC 是最小化主窗口，浏览器译为关浮层）
  useEffect(() => {
    if (!open) return undefined;
    const onKey = (e: KeyboardEvent): void => {
      if (e.key === 'Escape') setOpen(null);
    };
    window.addEventListener('keydown', onKey);
    return () => { window.removeEventListener('keydown', onKey); };
  }, [open]);

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

  return (
    <div id="card-display-overlay" className="card-display-overlay">
      <div className="card-display-window">
        <div className="card-display-header">
          <span id="card-display-title">{title}</span>
          <button
            id="card-display-close"
            className="card-display-close"
            onClick={() => setOpen(null)}
            aria-label="关闭"
          >×</button>
        </div>
        <div id="card-display-list" className="card-display-list">
          {cards.length === 0
            ? <div className="card-display-empty">（空）</div>
            : cards.map((code, i) => (
              <div
                key={`${code}-${i}`}
                className="card-display-item"
                onMouseEnter={() => duelStore.inspect(code)}
              >
                <CardThumb code={code} name={names[code] || `#${code}`} />
              </div>
            ))}
        </div>
      </div>
    </div>
  );
}

function CardThumb({ code, name }: { code: number; name: string }) {
  const [pic, setPic] = useState<string | null>(null);
  useEffect(() => {
    let alive = true;
    WailsBridge.getCardImage(code).then((p) => {
      if (alive) setPic(p && p.url ? p.url : null);
    });
    return () => { alive = false; };
  }, [code]);
  return (
    <>
      {pic
        ? <img className="card-display-pic" src={pic} alt={name} />
        : <div className="card-display-pic card-display-pic-empty" />}
      <span className="card-display-name">{name}</span>
    </>
  );
}
