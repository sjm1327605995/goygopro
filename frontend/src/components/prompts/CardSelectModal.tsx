import { CardTile } from './CardTile.tsx';
import type { SelectCard } from './CardTile.tsx';
import { useCardSelection } from './useCardSelection.ts';

export function CardSelectModal({ title, cards, min, max, cancelable, respond }: {
  title: string;
  cards: SelectCard[];
  min: number;
  max: number;
  cancelable: boolean;
  respond: (result: { indices: number[] | null }) => void;
}) {
  const { selected, toggle, minMet } = useCardSelection(min, max);
  const done = (indices: number[] | null) => respond({ indices });

  return (
    <div className="modal-box" style={{ minWidth: '550px' }}>
      <div className="modal-title">{`${title}（选择 ${min}-${max} 张）`}</div>
      <div className="modal-cards-grid">
        {cards.map((c, idx) => (
          <CardTile
            key={idx}
            code={c.code}
            name={c.name}
            dataAttrs={{ 'data-idx': idx }}
            className={selected.includes(idx) ? 'selected' : ''}
            onPick={() => toggle(idx)}
          />
        ))}
      </div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: '12px' }}>
        <span id="select-count-text" style={{ fontSize: '13px', color: 'var(--text-muted)' }}>
          {`已选 ${selected.length} / ${max} 张`}
        </span>
        <div style={{ display: 'flex', gap: '8px' }}>
          {cancelable && (
            <button id="modal-select-cancel" className="btn btn-secondary" onClick={() => done(null)}>取消</button>
          )}
          <button
            id="modal-select-confirm"
            className={`btn btn-gold${minMet ? ' btn-flash-gold' : ''}`}
            disabled={!minMet}
            onClick={() => done(selected)}
          >确定</button>
        </div>
      </div>
    </div>
  );
}