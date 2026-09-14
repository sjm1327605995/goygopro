/**
 * 2D 动作菜单（手牌/场上卡牌点击后弹出的动作按钮条）。
 *
 * 数据源：store.actionPopup（DuelManager 在 ui:hand_click / 场上点击回调里
 * openActionPopup）。点击按钮 → eventBus 'ui:player_action' {type, data}，
 * DuelManager 订阅后走 handlePlayerAction 发语义化响应。点击弹窗/手牌
 * 以外区域关闭（与旧 hud.js 的 document 监听同语义）。
 * 保留旧 hud 的 #action-popup / .action-btn 类名。
 */
import React, { useEffect } from 'react';
import { useSyncExternalStore } from 'react';
import { duelStore } from '../duel/store.ts';
import type { PopupAction } from '../domain/duel_state.ts';
import { eventBus } from '../wails_bridge.ts';

export default function ActionPopup() {
  const popup = useSyncExternalStore(duelStore.subscribe, duelStore.getState).actionPopup;

  useEffect(() => {
    const onDocClick = (e: MouseEvent) => {
      const t = e.target as Element | null;
      if (t && (t.closest('.action-popup') || t.closest('.hand-card-item'))) return;
      duelStore.closeActionPopup();
    };
    document.addEventListener('click', onDocClick);
    return () => document.removeEventListener('click', onDocClick);
  }, []);

  const open = (act: PopupAction) => {
    duelStore.closeActionPopup();
    eventBus.emit('ui:player_action', { type: act.action, data: act });
  };

  const style: React.CSSProperties = popup
    ? {
      left: `${Math.max(20, Math.min(window.innerWidth - 180, popup.x - 80))}px`,
      top: `${Math.max(20, popup.y - popup.actions.length * 35)}px`,
      display: 'flex',
    }
    : { display: 'none' };

  return (
    <div id="action-popup" className="action-popup" style={style}>
      {popup ? popup.actions.map((act) => (
        <button
          key={`${act.action}-${act.code ?? act.s ?? ''}`}
          className="action-btn"
          onClick={() => open(act)}
        >{act.label}</button>
      )) : null}
    </div>
  );
}
