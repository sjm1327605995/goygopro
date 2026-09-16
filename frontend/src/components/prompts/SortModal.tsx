import { CardTile } from './CardTile.tsx';
import type { SelectCard } from './CardTile.tsx';
import { useCardSelection } from './useCardSelection.ts';

/** Card ordering（MSG_SORT_CARD）：按顺序点击；确认回排序置换字节 */
export function SortModal({ title, cards, respond }: {
  title: string;
  cards: SelectCard[];
  respond: (result: { order: number[] | null }) => void;
}) {
  // min=0（无需最少张数）、max=cards.length（要排满才可确认）；selected 即排序序
  const { selected: order, toggle } = useCardSelection(0, cards.length);

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
              onPick={() => toggle(idx)}
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