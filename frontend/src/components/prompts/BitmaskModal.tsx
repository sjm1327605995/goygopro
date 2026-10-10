import { useState } from 'react';

/** Multi-pick bitmask（MSG_ANNOUNCE_RACE / ATTRIB）：恰好 count 项，回位掩码。
 *  variant 对齐原版窗口：race = wANRace (480,200)-(850,410)、
 *  attrib = wANAttribute (500,200)-(830,285)（game.cpp:622-634）。 */
export function BitmaskModal({ title, options, count, variant, respond }: {
  title: string;
  options: [number, string][];
  count: number;
  variant?: 'race' | 'attrib';
  respond: (mask: number) => void;
}) {
  const [mask, setMask] = useState(0);
  const picked = options.reduce((s, [v]) => s + (mask & v ? 1 : 0), 0);

  return (
    <div className={`modal-box${variant === 'race' ? ' modal-anrace' : variant === 'attrib' ? ' modal-anattr' : ''}`} style={variant ? undefined : { minWidth: '480px', textAlign: 'center' }}>
      <div className="modal-title">{`${title}（选择 ${count} 项）`}</div>
      <div style={{ margin: '12px 0' }}>
        {options.map(([value, label]) => (
          <button
            key={value}
            className={`btn ${mask & value ? 'btn-primary' : 'btn-secondary'} mask-opt`}
            data-value={value}
            style={{ margin: '4px', padding: '6px 14px' }}
            onClick={() => {
              setMask((cur) => {
                if (cur & value) return cur & ~value;
                if (picked < count) return cur | value;
                return cur;
              });
            }}
          >{label}</button>
        ))}
      </div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: '8px' }}>
        <span id="mask-status" style={{ fontSize: '13px', color: 'var(--text-muted)' }}>{`已选 ${picked} / ${count}`}</span>
        <button id="mask-confirm" className="btn btn-gold" disabled={picked !== count} onClick={() => respond(mask)}>确定</button>
      </div>
    </div>
  );
}