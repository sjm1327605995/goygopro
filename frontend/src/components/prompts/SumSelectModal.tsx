import { CardTile } from './CardTile.tsx';
import type { SelectCard } from './CardTile.tsx';
import { useCardSelection } from './useCardSelection.ts';

/** sum-limited selection（MSG_SELECT_SUM）。sumMode 0 = 恰好 acc（超量/链接），
 *  1 = 至少 acc（上级召唤/仪式）；must 卡已锁定只展示 */
export function SumSelectModal({ title, cards, must, acc, min, max, sumMode, respond }: {
  title: string;
  cards: SelectCard[];
  must: SelectCard[];
  acc: number;
  min: number;
  max: number;
  sumMode: number;
  respond: (selected: number[]) => void;
}) {
  const { selected, toggle, minMet } = useCardSelection(min, max);
  const cardVal = (c: SelectCard): number => {
    // sum_param 打包两个备选值；一张卡按其中较小者计
    const o1 = (Number(c.param) || 0) & 0xffff;
    const o2 = (Number(c.param) || 0) >>> 16;
    return o2 && o2 < o1 ? o2 : o1;
  };
  const mustSum = must.reduce((s, c) => s + cardVal(c), 0);
  const sum = mustSum + selected.reduce((s, i) => s + cardVal(cards[i]), 0);
  const countOk = minMet && selected.length <= max;
  const sumOk = sumMode === 0 ? sum === acc : sum >= acc;

  return (
    <div className="modal-box" style={{ minWidth: '560px' }}>
      <div className="modal-title">{title}</div>
      <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginBottom: '8px' }}>
        {`需要 ${sumMode === 0 ? '合计恰好' : '合计至少'} ${acc}${mustSum ? `（必选已占 ${mustSum}）` : ''}，选择 ${min}-${max} 张`}
      </div>
      {must.length > 0 && (
        <div style={{ marginBottom: '8px' }}>
          {must.map((c, i) => (
            <div key={i} style={{ background: '#0f172a', padding: '6px', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>{`🔒 ${c.name}`}</div>
              <span style={{ fontSize: '11px', color: '#f59e0b' }}>{cardVal(c)}</span>
            </div>
          ))}
        </div>
      )}
      <div className="modal-cards-grid">
        {cards.map((c, idx) => (
          <CardTile
            key={idx}
            code={c.code}
            name={c.name}
            className={`sum-card${selected.includes(idx) ? ' selected' : ''}`}
            dataAttrs={{ 'data-idx': idx }}
            onPick={() => toggle(idx)}
          >
            <span className="sum-param">{cardVal(c)}</span>
          </CardTile>
        ))}
      </div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: '12px' }}>
        <span id="sum-status" style={{ fontSize: '13px', color: sumOk && countOk ? '#22c55e' : 'var(--text-muted)' }}>
          {`已选 ${selected.length} 张，合计 ${sum}`}
        </span>
        <button
          id="sum-confirm"
          className={`btn btn-gold${sumOk && countOk ? ' btn-flash-gold' : ''}`}
          disabled={!(sumOk && countOk)}
          onClick={() => respond(selected)}
        >确定</button>
      </div>
    </div>
  );
}