/**
 * 手牌坞（原版画面下方的手牌条）。
 *
 * 数据源：store.hand（reducer 的 draw/move 事件同步；store 外不再持手牌真值）。
 * 交互：
 *   - 悬停 → duelStore.inspect(code)（左侧 CardPreviewPanel 消费）
 *   - 点击 → eventBus 'ui:hand_click' {code,x,y}；DuelManager 订阅后按
 *     引擎 idlecmd 提供的动作经 duelStore.openActionPopup 打开 2D 菜单
 *     （非交互模式——回放——点击无效）。
 * 保留旧 hud 的 .hand-card-item / .hand-card-img 类名与 data-code 属性。
 */
import React, { useEffect, useState } from 'react';
import { useSyncExternalStore } from 'react';
import { duelStore } from '../duel/store.ts';
import type { HandCard } from '../domain/duel_state.ts';
import { WailsBridge, eventBus } from '../wails_bridge.ts';

function HandCardItem({ code, selected, interactive, onClick, fanStyle }: {
  code: number;
  selected: boolean;
  interactive: boolean;
  onClick: (code: number, el: HTMLElement) => void;
  fanStyle?: React.CSSProperties;
}) {
  const [name, setName] = useState<string | null>(null);
  const [pic, setPic] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    if (!code) return undefined;
    WailsBridge.getCard(code).then((info) => {
      if (alive && info) setName(String(info.name));
    });
    WailsBridge.getCardImage(code).then((p) => {
      if (alive && p && p.url) setPic(p.url);
    });
    return () => { alive = false; };
  }, [code]);

  return (
    <div
      className={`hand-card-item${selected ? ' selected' : ''}`}
      data-code={code}
      style={fanStyle}
      onMouseEnter={() => duelStore.inspect(code)}
      onClick={(e) => {
        if (!interactive) return;
        e.stopPropagation();
        onClick(code, e.currentTarget as HTMLElement);
      }}
    >
      <div
        className="hand-card-img"
        style={pic ? {
          backgroundImage: `url("${pic}"), linear-gradient(135deg, #1e293b, #0f172a)`,
          backgroundSize: 'cover',
          backgroundPosition: 'center',
        } : undefined}
      >
        <div style={{ padding: '6px', fontSize: '11px', fontWeight: 700, color: '#38bdf8' }}>
          {name || `卡牌 #${code}`}
        </div>
      </div>
    </div>
  );
}

export default function HandDock({ interactive = true }: { interactive?: boolean }) {
  const hand = useSyncExternalStore(duelStore.subscribe, duelStore.getState).hand;
  const [selectedCode, setSelectedCode] = useState<number | null>(null);

  const onClick = (code: number, el: HTMLElement) => {
    setSelectedCode(code);
    const rect = el.getBoundingClientRect();
    eventBus.emit('ui:hand_click', { code, x: rect.left + rect.width / 2, y: rect.top - 10 });
  };

  // 扇形手牌：中间最高、两侧渐低并外撇（原版手牌的持牌观感）。
  // 每张卡通过 CSS 变量下发旋转/升降，:hover 的抬升放大在 style.css 里统一。
  const n = hand.length;
  const mid = (n - 1) / 2;

  return (
    <div className="hand-dock-container">
      <div id="hand-cards-dock" className="hand-cards">
        {hand.map((c, i) => (c ? (
          <HandCardItem
            key={`${c.code}-${i}`}
            code={c.code}
            selected={selectedCode === c.code}
            interactive={interactive}
            onClick={onClick}
            fanStyle={{
              '--fan-rot': `${(i - mid) * 3}deg`,
              '--fan-lift': `${Math.abs(i - mid) * 7}px`,
              '--fan-z': String(10 + i),
            } as React.CSSProperties}
          />
        ) : null))}
      </div>
    </div>
  );
}
