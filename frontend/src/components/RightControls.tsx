/**
 * 右侧控制组（原版 gframe game.cpp:916-923 的连锁三键 + wSurrender）。
 *
 * 连锁三键不发任何包：它们是本地偏好，决定 duel:select_chain 到来时是否
 * 自动代答 -1（语义见 duel/chain_prefs.js，已核对 duelclient.cpp:1776）。
 * 投降走两步确认（原版 wSurrender 弹窗），确认后 CTOS_SURRENDER。
 */
import React, { useEffect, useState } from 'react';
import { eventBus, WailsBridge } from '../wails_bridge.ts';
import chainPrefs from '../duel/chain_prefs.ts';
import type { ChainPrefMode } from '../duel/chain_prefs.ts';

const MODES: { key: ChainPrefMode; label: string; title: string }[] = [
  { key: 'ignore', label: '放弃连锁', title: '自动放弃一切连锁机会' },
  { key: 'always', label: '连锁监视', title: '每次连锁窗口都保持询问' },
  { key: 'whenAvail', label: '自动连锁', title: '有任何可发动时询问' },
];

export default function RightControls({ onSurrendered }: { onSurrendered?: () => void } = {}) {
  const [mode, setMode] = useState<string | null>(chainPrefs.get());
  const [confirming, setConfirming] = useState(false);
  // 组队赛队友已请求投降（STOC_TEAMMATE_SURRENDER）→ 文案换 SysString 1355
  const [teammateSurrender, setTeammateSurrender] = useState(false);

  useEffect(() => chainPrefs.subscribe(setMode), []);
  useEffect(() => {
    const onMate = (): void => setTeammateSurrender(true);
    eventBus.on('stoc:teammate_surrender', onMate);
    return () => { eventBus.off('stoc:teammate_surrender', onMate); };
  }, []);

  const pick = (key: ChainPrefMode) => {
    chainPrefs.set(key);
    setMode(chainPrefs.get());
  };

  const doSurrender = () => {
    setConfirming(false);
    WailsBridge.surrender();
    if (onSurrendered) onSurrendered();
  };

  return (
    <div id="right-controls" className="right-controls">
      <div className="chain-group">
        {MODES.map((m) => (
          <button
            key={m.key}
            id={`chain-btn-${m.key}`}
            className={`chain-btn ${mode === m.key ? 'pressed' : ''}`}
            title={m.title}
            onClick={() => pick(m.key)}
          >
            {m.label}
          </button>
        ))}
      </div>
      <button id="surrender-btn" className="surrender-btn" onClick={() => setConfirming(true)}>
        {teammateSurrender ? '投降(1/2)' : '投降'}
      </button>
      {confirming && (
        <div className="surrender-confirm">
          <div className="surrender-confirm-title">是否投降？</div>
          <div className="surrender-confirm-row">
            <button id="surrender-yes" className="btn btn-gold" onClick={doSurrender}>是</button>
            <button id="surrender-no" className="btn btn-secondary" onClick={() => setConfirming(false)}>否</button>
          </div>
        </div>
      )}
    </div>
  );
}
