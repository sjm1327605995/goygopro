import { useRef, useState } from 'react';
import { CardTile } from './CardTile.tsx';
import type { SelectCard } from './CardTile.tsx';

/** Card ordering（MSG_SORT_CARD）：按顺序点击；确认回排序置换字节 */
export function SortModal({ title, cards, respond }: {
  title: string;
  cards: SelectCard[];
  respond: (result: { order: number[] | null }) => void;
}) {
  // selected 即排序序（点击顺序）。ref 与 state 同步：连点在同一渲染批次内
  // 完成时闭包里的 state 仍是旧值，自动应答判定必须读 ref。
  const [order, setOrder] = useState<number[]>([]);
  const orderRef = useRef<number[]>([]);

  // 原版 event_handler.cpp:784-791：最后一张指派序号后立即应答并关窗
  const onPick = (idx: number) => {
    const cur = orderRef.current;
    if (cur.includes(idx)) {
      orderRef.current = cur.filter((i) => i !== idx);
    } else {
      orderRef.current = [...cur, idx];
      if (orderRef.current.length === cards.length) {
        respond({ order: orderRef.current });
        return;
      }
    }
    setOrder(orderRef.current);
  };

  return (
    <div className="modal-box" style={{ minWidth: '520px' }}>
      <div className="modal-title">{title}</div>
      <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginBottom: '8px' }}>
        按希望的顺序依次点击卡牌（点击已排序的卡可撤销）
      </div>
      <div className="modal-cards-grid">
        {cards.map((c, idx) => {
          const rank = order.indexOf(idx);
          return (
            <CardTile
              key={idx}
              code={c.code}
              name={c.name}
              className="sort-card"
              dataAttrs={{ 'data-idx': idx }}
              onPick={() => onPick(idx)}
            >
              <span className="sort-badge" data-idx={idx} style={{ display: rank > -1 ? 'block' : 'none' }}>
                {rank > -1 ? rank + 1 : ''}
              </span>
            </CardTile>
          );
        })}
      </div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: '12px' }}>
        <button id="sort-cancel" className="btn btn-secondary" onClick={() => respond({ order: null })}>放弃排序</button>
        <button
          id="sort-confirm"
          className="btn btn-gold"
          disabled={order.length !== cards.length}
          onClick={() => respond({ order })}
        >确定</button>
      </div>
    </div>
  );
}