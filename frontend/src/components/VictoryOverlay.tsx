/**
 * 胜利/败北覆盖层（原版胜利大字 + 弹窗 + 击杀卡图/胜负原因）。
 *
 * 数据源：store.win（duel:win 事件，winner 已是显示座）与 store.matchKill
 * （MSG_MATCH_KILL 的击杀卡）。原版 MSG_WIN 的 showcard=101 横幅由
 * PhaseStrip 负责，这里复刻弹窗部分：
 *   - 胜/负标题 + 胜负原因（victoryString(type)，!victory 子集）
 *   - match kill：胜利画面中央展示击杀卡图 + 卡名（原版 GetVictoryString
 *     0xffff 语义：「用%ls获胜」类展示）
 *   - 音效：胜利号角 / 败北下行音（showVictory=false 的回放剧场静默）
 * 保留旧 hud 的 #modal-duel-exit 按钮与 modal 类名。
 */
import React, { useEffect, useRef, useState, useSyncExternalStore } from 'react';
import { duelStore } from '../duel/store.ts';
import { eventBus, WailsBridge } from '../wails_bridge.ts';
import { soundManager } from '../audio/sound_manager.ts';
import { victoryString } from '../domain/sys_strings.ts';
import { cardName } from '../domain/card_names.ts';

export default function VictoryOverlay({ showVictory = true }: { showVictory?: boolean }) {
  const state = useSyncExternalStore(duelStore.subscribe, duelStore.getState);
  const win = state.win;
  const matchKill = state.matchKill;
  const [killArt, setKillArt] = useState<string | null>(null);
  const soundedRef = useRef(false);

  const { winner, type } = win || { winner: 0, type: 0 };
  const isWin = winner === 0;

  // 击杀卡图（原版胜利画面的 match kill 卡展示）
  useEffect(() => {
    let alive = true;
    if (!win || !matchKill) {
      setKillArt(null);
      return undefined;
    }
    WailsBridge.getCardImage(matchKill).then((p) => {
      if (alive && p && p.url) setKillArt(p.url);
    });
    return () => { alive = false; };
  }, [win, matchKill]);

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

  const reason = victoryString(type);

  return (
    <div className="modal-overlay active" id="victory-overlay">
      <div className="modal-box" style={{ textAlign: 'center', minWidth: '400px', padding: '36px 24px' }}>
        <div style={{ fontSize: '64px', marginBottom: '12px' }}>{isWin ? '🏆' : '💀'}</div>
        <div style={{ fontSize: '32px', fontWeight: 900, color: isWin ? '#f59e0b' : '#ef4444', marginBottom: '8px' }}>
          {isWin ? '胜利' : '败北'}
        </div>
        {/* 胜负原因（strings.conf !victory 子集；特殊胜利 type>=0x10 文案自带卡名） */}
        {reason && (
          <p style={{ color: 'var(--text-muted)', marginBottom: '8px' }}>原因：{reason}</p>
        )}
        {/* match kill 卡图展示（原版胜利画面击杀卡） */}
        {isWin && matchKill ? (
          <div id="match-kill-card" style={{ margin: '12px auto 16px', width: '140px' }}>
            {killArt ? (
              <img
                src={killArt}
                alt={cardName(matchKill)}
                draggable={false}
                style={{ width: '100%', borderRadius: '6px', border: '1px solid #b89a5a', display: 'block' }}
              />
            ) : (
              <div style={{
                width: '100%', height: '196px', borderRadius: '6px',
                border: '1px solid #b89a5a', background: '#0f172a',
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                color: '#38bdf8', fontSize: '12px', padding: '6px',
              }}
              >
                {cardName(matchKill)}
              </div>
            )}
            <div style={{ color: '#b89a5a', fontSize: '12px', marginTop: '6px' }}>
              比赛击杀：{cardName(matchKill)}
            </div>
          </div>
        ) : (
          <p style={{ color: 'var(--text-muted)', marginBottom: '24px' }}>
            {isWin ? '恭喜！你赢得了这场决斗！' : '虽败犹荣，再接再厉！'}
          </p>
        )}
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
