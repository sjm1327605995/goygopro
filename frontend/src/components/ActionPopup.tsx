/**
 * 2D 动作菜单（手牌/场上卡牌点击后弹出的动作按钮条）。
 *
 * 数据源：store.actionPopup（DuelManager 在 ui:hand_click / 场上点击回调里
 * openActionPopup）。点击按钮 → eventBus 'ui:player_action' {type, data}，
 * DuelManager 订阅后走 handlePlayerAction 发语义化响应。点击弹窗/手牌
 * 以外区域关闭（与旧 hud.js 的 document 监听同语义）。
 * 保留旧 hud 的 #action-popup / .action-btn 类名。
 */
import React, { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { useSyncExternalStore } from 'react';
import { duelStore } from '../duel/store.ts';
import type { PopupAction } from '../domain/duel_state.ts';
import { eventBus } from '../wails_bridge.ts';

export default function ActionPopup() {
  const popup = useSyncExternalStore(duelStore.subscribe, duelStore.getState).actionPopup;
  const popupRef = useRef<HTMLDivElement | null>(null);
  const [position, setPosition] = useState({ left: 8, top: 8 });

  useEffect(() => {
    const onDocClick = (e: MouseEvent) => {
      const t = e.target as Element | null;
      if (t && (t.closest('.action-popup') || t.closest('.hand-card-item'))) return;
      duelStore.closeActionPopup();
    };
    document.addEventListener('click', onDocClick);
    return () => document.removeEventListener('click', onDocClick);
  }, []);

  useLayoutEffect(() => {
    if (!popup) return undefined;
    const update = () => {
      const el = popupRef.current;
      if (!el) return;
      const margin = 8;
      const gap = 12;
      const width = el.offsetWidth;
      const height = el.offsetHeight;
      let left = popup.x + gap;
      let top = popup.y + gap;
      if (left + width > window.innerWidth - margin) left = popup.x - width - gap;
      if (top + height > window.innerHeight - margin) top = popup.y - height - gap;
      left = Math.max(margin, Math.min(left, window.innerWidth - width - margin));
      top = Math.max(margin, Math.min(top, window.innerHeight - height - margin));
      setPosition({ left, top });
    };
    update();
    window.addEventListener('resize', update);
    return () => window.removeEventListener('resize', update);
  }, [popup]);

  useEffect(() => {
    if (popup) popupRef.current?.querySelector<HTMLButtonElement>('.action-btn')?.focus();
  }, [popup]);

  const open = (act: PopupAction) => {
    duelStore.closeActionPopup();
    eventBus.emit('ui:player_action', { type: act.action, data: act });
  };

  const onKeyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
    if (!popup) return;
    if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      duelStore.closeActionPopup();
      return;
    }
    const buttons = Array.from(popupRef.current?.querySelectorAll<HTMLButtonElement>('.action-btn') || []);
    if (!buttons.length) return;
    const current = buttons.indexOf(document.activeElement as HTMLButtonElement);
    let next = -1;
    if (e.key === 'ArrowDown') next = (current + 1 + buttons.length) % buttons.length;
    else if (e.key === 'ArrowUp') next = (current - 1 + buttons.length) % buttons.length;
    else if (e.key === 'Home') next = 0;
    else if (e.key === 'End') next = buttons.length - 1;
    if (next < 0) return;
    e.preventDefault();
    buttons[next].focus();
  };

  const style: React.CSSProperties = popup
    ? {
      left: `${position.left}px`,
      top: `${position.top}px`,
      display: 'flex',
      maxWidth: 'calc(100vw - 16px)',
      maxHeight: 'calc(100vh - 16px)',
      overflowY: 'auto',
    }
    : { display: 'none' };

  return (
    <div
      ref={popupRef}
      id="action-popup"
      className="action-popup"
      role="menu"
      aria-label="卡牌操作"
      style={style}
      onKeyDown={onKeyDown}
    >
      {popup ? popup.actions.map((act) => (
        <button
          key={`${act.action}-${act.code ?? act.s ?? ''}`}
          type="button"
          role="menuitem"
          className="action-btn"
          onClick={() => open(act)}
        >{act.label}</button>
      )) : null}
    </div>
  );
}
