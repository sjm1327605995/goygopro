/**
 * 右侧控制组（原版 gframe game.cpp:916-923 的连锁三键 + wSurrender）。
 *
 * 连锁三键不发任何包：它们是本地偏好，决定 duel:select_chain 到来时是否
 * 自动代答 -1（语义见 duel/chain_prefs.js，已核对 duelclient.cpp:1776）。
 * 投降走两步确认（原版 wSurrender 弹窗），确认后 CTOS_SURRENDER。
 *
 * 观战者（playerType 高 4 位非 0）只有 btnLeaveGame「离开」
 * （duelclient.cpp:653-656 SysString 1350），无连锁键/投降键。
 */
import React, { useEffect, useState, useSyncExternalStore } from 'react';
import { eventBus, WailsBridge } from '../wails_bridge.ts';
import chainPrefs from '../duel/chain_prefs.ts';
import type { ChainPrefMode } from '../duel/chain_prefs.ts';
import { duelStore } from '../duel/store.ts';

const MODES: { key: ChainPrefMode; label: string; title: string }[] = [
  { key: 'ignore', label: '忽略时点', title: '自动放弃一切连锁机会' },
  { key: 'always', label: '显示时点', title: '每次连锁窗口都保持询问' },
  { key: 'whenAvail', label: '可用时点', title: '有任何可发动时询问' },
];

export default function RightControls({ onSurrendered, onLeaveObserver }: {
  onSurrendered?: () => void;
  /** 观战者的 btnLeaveGame：CTOS_LEAVE_GAME 后退出决斗画面 */
  onLeaveObserver?: () => void;
} = {}) {
  const state = useSyncExternalStore(duelStore.subscribe, duelStore.getState);
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

  // 观战者：连锁/投降都不适用，只留「离开」
  if (state.isObserver) {
    return (
      <div id="right-controls" className="right-controls">
        <button
          id="leave-game-btn"
          className={`leave-game-btn${teammateSurrender ? ' btn-hint-glow' : ''}`}
          onClick={() => {
            WailsBridge.leaveGame();
            if (onLeaveObserver) onLeaveObserver();
          }}
        >
          离开
        </button>
      </div>
    );
  }

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
          <div className="surrender-confirm-title">是否确定投降？</div>
          <div className="surrender-confirm-row">
            <button id="surrender-yes" className="btn btn-gold" onClick={doSurrender}>是</button>
            <button id="surrender-no" className="btn btn-secondary" onClick={() => setConfirming(false)}>否</button>
          </div>
        </div>
      )}
    </div>
  );
}
