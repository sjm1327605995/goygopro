/**
 * 玩家状态面板（原版 lp.png 血条嵌 lpf.png 框 + 名字 + 计时 + 堆区计数）。
 *
 * 数据源：duelStore（显示座 0=本方 1=对方）。LP 血条宽度按初始 LP 归一，
 * 与原版 drawing.cpp 的 292px 满条成比例。
 */
import React, { useSyncExternalStore } from 'react';
import { duelStore } from '../duel/store.ts';
import { settingsStore } from '../domain/settings.ts';
import type { PileCounts } from '../domain/reducer.ts';

const PILE_LABELS: [keyof PileCounts, string][] = [
  ['deck', '卡组'], ['grave', '墓地'], ['banish', '除外'], ['extra', '额外'],
];

function fmtTimer(sec: number | null): string {
  if (sec == null) return '--:--';
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
}

export default function PlayerPanel({ side }: { side: 'player' | 'opponent' }) {
  const state = useSyncExternalStore(duelStore.subscribe, duelStore.getState);
  const settings = useSyncExternalStore(settingsStore.subscribe, settingsStore.getSnapshot);
  const disp = side === 'player' ? 0 : 1;
  // 隐藏玩家名（原版 chkHidePlayerName，聊天显示 [********]）
  const shownName = settings.hide_player_name ? '[********]' : state.names[disp];
  const lp = state.lp[disp];
  // 归一化基准取两者初始较大值，避免先攻 8000/对方 8000 之外的特殊开局失真
  const maxLP = Math.max(8000, state.lp[0], state.lp[1]);
  const pct = Math.max(0, Math.min(100, (lp / maxLP) * 100));
  const piles = state.piles[disp];
  const timer = state.timer[disp];
  const isTurn = state.started && state.turnPlayer === disp;

  return (
    <div
      id={`${side}-panel`}
      className={`player-state-panel${isTurn ? ' is-turn' : ''}`}
    >
      <div className="player-state-head">
        <span id={`${side}-panel-name`} className="player-state-name">
          {shownName}
        </span>
        {isTurn ? <span className="player-turn-chip">回合 {state.turn}</span> : null}
      </div>
      <div className="lp-frame">
        <div className="lp-track">
          <div className="lp-fill" style={{ width: `${pct}%` }} />
        </div>
        <span id={`${side}-panel-lp`} className="lp-value">{lp}</span>
      </div>
      <div className="player-state-row">
        <span className="player-timer">{fmtTimer(timer)}</span>
        {PILE_LABELS.map(([key, label]) => (
          <span key={key} className="pile-counter">
            {label} <b id={`${side}-pile-${key}`}>{piles[key]}</b>
          </span>
        ))}
      </div>
    </div>
  );
}
