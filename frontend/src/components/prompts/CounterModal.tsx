import { useState } from 'react';

/** Counter removal（MSG_SELECT_COUNTER）：每卡一个 uint16，合计恰为 count */
export function CounterModal({ title, cards, count, respond }: {
  title: string;
  cards: { code: number; name: string; cnt: number }[];
  count: number;
  respond: (counts: number[]) => void;
}) {
  const [counts, setCounts] = useState<number[]>(() => cards.map(() => 0));
  const maxTotal = cards.reduce((s, c) => s + (c.cnt || 0), 0);
  const total = counts.reduce((s, v) => s + v, 0);

  return (
    <div className="modal-box" style={{ minWidth: '500px' }}>
      <div className="modal-title">{`${title}（需移除 ${count} 个指示物）`}</div>
      <div>
        {cards.map((c, i) => (
          <div key={i} className="select-card-item counter-row" data-idx={i}
            style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', background: '#1e293b', padding: '8px' }}>
            <div style={{ fontSize: '12px', color: '#38bdf8', fontWeight: 700 }}>{c.name}</div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>{`可移 ${c.cnt || 0}`}</span>
              <button className="btn btn-secondary counter-dec" data-idx={i} style={{ padding: '2px 10px' }}
                onClick={() => setCounts((cur) => cur.map((v, j) => (j === i && v > 0 ? v - 1 : v)))}>-</button>
              <span className="counter-val" data-idx={i} style={{ minWidth: '24px', textAlign: 'center', fontWeight: 900 }}>{counts[i]}</span>
              <button className="btn btn-secondary counter-inc" data-idx={i} style={{ padding: '2px 10px' }}
                onClick={() => setCounts((cur) => cur.map((v, j) => (j === i && v < (c.cnt || 0) ? v + 1 : v)))}>+</button>
            </div>
          </div>
        ))}
      </div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: '12px' }}>
        <span id="counter-total" style={{ fontSize: '13px', color: 'var(--text-muted)' }}>
          {`已选 ${total} / ${count}（总供给 ${maxTotal}）`}
        </span>
        <button id="counter-confirm" className="btn btn-gold" disabled={total !== count} onClick={() => respond(counts)}>确定</button>
      </div>
    </div>
  );
}