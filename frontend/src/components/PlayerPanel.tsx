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

/**
 * LP 条分层配色（drawing.cpp:590-619）：LP 超过初始值时按 layerCount 换行
 * 取 lp.png 的第 N 行彩条——底层铺 (layerCount-1)%5 行，前景只铺余数比例、
 * 用 layerCount%5 行；不超过初始值时整条用第 0 行。
 */
function lpBarLayers(lp: number, maxLP: number): {
  fgRow: number; bgRow: number; fgPct: number; layered: boolean;
} {
  if (lp > maxLP && maxLP > 0) {
    const layerCount = Math.floor(lp / maxLP);
    const partial = lp % maxLP;
    return {
      fgRow: layerCount % 5,
      bgRow: (layerCount - 1) % 5,
      fgPct: (partial / maxLP) * 100,
      layered: true,
    };
  }
  return { fgRow: 0, bgRow: 0, fgPct: Math.max(0, (lp / maxLP) * 100), layered: false };
}

export default function PlayerPanel({ side }: { side: 'player' | 'opponent' }) {
  const state = useSyncExternalStore(duelStore.subscribe, duelStore.getState);
  const settings = useSyncExternalStore(settingsStore.subscribe, settingsStore.getSnapshot);
  const disp = side === 'player' ? 0 : 1;
  // 隐藏玩家名（原版 chkHidePlayerName，聊天显示 [********]）
  const shownName = settings.hide_player_name ? '[********]' : state.names[disp];
  const lp = state.lp[disp];
  // 归一化基准 = 自己的初始基本分（drawing.cpp:591 maxLP = start_lp）
  const maxLP = state.startLP || Math.max(8000, state.lp[0], state.lp[1]);
  const { fgRow, bgRow, fgPct, layered } = lpBarLayers(lp, maxLP);
  // lp.png 是 16×80 的 5 行彩条：背景尺寸放大 5 倍后按行号挑色
  const rowStyle = (row: number) => ({
    backgroundSize: 'auto 500%',
    backgroundPositionY: `${row * 25}%`,
  });
  const piles = state.piles[disp];
  const timer = state.timer[disp];
  const isTurn = state.started && state.turnPlayer === disp;
  // 时限条（drawing.cpp:637-642）：仅非观战 + 已知时限时画灰条
  const showTimeBar = !state.isObserver && state.timeLimit > 0 && timer != null;
  const timePct = showTimeBar ? Math.max(0, Math.min(100, (timer / state.timeLimit) * 100)) : 0;

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
          {layered && <div className="lp-fill lp-fill-bg" style={{ width: '100%', ...rowStyle(bgRow) }} />}
          <div className="lp-fill" style={{ width: `${fgPct}%`, ...rowStyle(fgRow) }} />
        </div>
        <span id={`${side}-panel-lp`} className="lp-value">{lp}</span>
      </div>
      {showTimeBar && (
        <div className="time-limit-bar" id={`${side}-time-bar`}>
          <div className="time-limit-fill" style={{ width: `${timePct}%` }} />
        </div>
      )}
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
