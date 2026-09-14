/**
 * 胜利/败北覆盖层（原版胜利大字 + 弹窗）。
 *
 * 数据源：store.win（duel:win 事件）。横幅大字由 PhaseStrip 负责，这里
 * 是弹窗与音效。showVictory=false 时（回放剧场）只静默——不弹窗不响喇叭，
 * 回放 seek 也不该残留胜利弹窗（store.reset 自然清掉 win）。
 * 保留旧 hud 的 #modal-duel-exit 按钮与 modal 类名。
 */
import React, { useEffect, useRef } from 'react';
import { useSyncExternalStore } from 'react';
import { duelStore } from '../duel/store.ts';
import { eventBus } from '../wails_bridge.ts';
import { soundManager } from '../audio/sound_manager.ts';

export default function VictoryOverlay({ showVictory = true }: { showVictory?: boolean }) {
  const win = useSyncExternalStore(duelStore.subscribe, duelStore.getState).win;
  const soundedRef = useRef(false);

  const { winner, type } = win || { winner: 0, type: 0 };
  const isWin = winner === 0;

  useEffect(() => {
    if (!win || !showVictory) {
      soundedRef.current = false;
      return;
    }
    if (soundedRef.current) return;
    soundedRef.current = true;
    if (isWin) soundManager.playVictory();
    else soundManager.playDefeat();
  }, [win, showVictory, isWin]);

  if (!win || !showVictory) return null;

  return (
    <div className="modal-overlay active" id="victory-overlay">
      <div className="modal-box" style={{ textAlign: 'center', minWidth: '400px', padding: '36px 24px' }}>
        <div style={{ fontSize: '64px', marginBottom: '12px' }}>{isWin ? '🏆' : '💀'}</div>
        <div style={{ fontSize: '32px', fontWeight: 900, color: isWin ? '#f59e0b' : '#ef4444', marginBottom: '8px' }}>
          {isWin ? '胜利' : '败北'}
        </div>
        <p style={{ color: 'var(--text-muted)', marginBottom: '24px' }}>
          {isWin ? '恭喜！你赢得了这场决斗！' : '虽败犹荣，再接再厉！'}
        </p>
        <button
          id="modal-duel-exit"
          className="btn btn-primary"
          style={{ padding: '12px 32px' }}
          onClick={() => eventBus.emit('nav', 'menu')}
        >返回主菜单</button>
      </div>
    </div>
  );
}
