import { useState } from 'react';
import { CardTile } from './CardTile.tsx';

/** Card announcement（MSG_ANNOUNCE_CARD）：有候选出卡图按钮，否则自由输入卡号 */
export function AnnounceCardModal({ title, candidates, respond }: {
  title: string;
  candidates: { code: number; name: string }[];
  respond: (code: number) => void;
}) {
  const [input, setInput] = useState('');
  const confirmInput = () => {
    const v = parseInt(input, 10);
    if (!Number.isNaN(v) && v > 0) respond(v);
  };

  return (
    <div className="modal-box modal-announce-card">
      <div className="modal-title">{title}</div>
      {candidates.length > 0 && (
        <div className="modal-cards-grid">
          {candidates.map(({ code, name }, idx) => (
            <CardTile key={idx} code={code} name={name} className="cand-card" dataAttrs={{ 'data-idx': idx }} onPick={() => respond(code)} />
          ))}
        </div>
      )}
      <div className="announce-card-row">
        <input
          id="announce-card-input"
          type="number"
          min={0}
          placeholder="输入卡片密码"
          value={input}
          onChange={(e) => setInput(e.target.value)}
        />
        <button id="announce-card-confirm" className="btn btn-gold" onClick={confirmInput}>确定</button>
      </div>
    </div>
  );
}