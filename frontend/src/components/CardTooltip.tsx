/**
 * stTip 等价物（原版 game.cpp:952）：跟随鼠标的悬浮卡名提示。
 *
 * 数据源：field3d.handleHover 在悬停/离开场上卡牌时向
 * #duel-canvas-container 派发 'ygo:cardhover' / 'ygo:cardhoverend'
 * CustomEvent。原版是白底 80% 不透明小块，文字居中；这里按卡种着色
 * 左边框（怪兽红/魔法绿/陷阱紫，同原版卡名颜色约定）。
 *
 * 选择进行中（store.selectHintText 非空）首行附带选择提示文本——原版
 * stHintMsg 的 `提示(min-max)` 在选择期间常驻（event_handler.cpp 悬停
 * 期间 stTip 与选择提示并存的语义）。
 */
import React, { useEffect, useRef, useState, useSyncExternalStore } from 'react';
import { duelStore } from '../duel/store.ts';

interface HoverDetail {
  code?: number;
  info?: any;
  x?: number;
  y?: number;
}

interface TipState {
  text: string;
  x: number;
  y: number;
  kind: string;
}

function borderFor(info: any): string {
  if (!info || typeof info.type !== 'number') return '#94a3b8';
  if (info.type & 0x1) return '#f87171';       // 怪兽
  if (info.type & 0x2) return '#4ade80';       // 魔法
  if (info.type & 0x4) return '#c084fc';       // 陷阱
  return '#94a3b8';
}

export default function CardTooltip() {
  const [tip, setTip] = useState<TipState | null>(null);
  const tipRef = useRef<HTMLDivElement | null>(null);
  const selectHintText = useSyncExternalStore(
    duelStore.subscribe,
    () => duelStore.getState().selectHintText,
  );

  useEffect(() => {
    const container = document.getElementById('duel-canvas-container');
    if (!container) return;

    const onHover = (ev: Event) => {
      const d = (ev as CustomEvent<HoverDetail>).detail || {};
      const text = d.info && d.info.name ? d.info.name : (d.code ? String(d.code) : '');
      if (!text) return;
      setTip({ text, x: d.x || 0, y: d.y || 0, kind: borderFor(d.info) });
    };
    const onHoverEnd = () => setTip(null);

    container.addEventListener('ygo:cardhover', onHover);
    container.addEventListener('ygo:cardhoverend', onHoverEnd);
    return () => {
      container.removeEventListener('ygo:cardhover', onHover);
      container.removeEventListener('ygo:cardhoverend', onHoverEnd);
    };
  }, []);

  if (!tip) return null;

  // 提示块跟随鼠标，右下偏移 14px；贴近屏幕右/下缘时翻到鼠标另一侧。
  const w = tipRef.current ? tipRef.current.offsetWidth : 150;
  const h = tipRef.current ? tipRef.current.offsetHeight : 28;
  const x = tip.x + 14 + w > window.innerWidth ? tip.x - w - 8 : tip.x + 14;
  const y = tip.y + 14 + h > window.innerHeight ? tip.y - h - 8 : tip.y + 14;

  return (
    <div
      ref={tipRef}
      id="st-tip"
      style={{
        position: 'fixed', left: `${x}px`, top: `${y}px`, zIndex: 1200,
        background: 'rgba(255,255,255,0.9)', color: '#1a1a1a',
        borderLeft: `3px solid ${tip.kind}`,
        padding: '3px 10px', fontSize: '12px', fontWeight: 700,
        borderRadius: '3px', pointerEvents: 'none',
        maxWidth: '220px', whiteSpace: 'nowrap',
        overflow: 'hidden', textOverflow: 'ellipsis',
        boxShadow: '0 2px 8px rgba(0,0,0,0.4)',
      }}
    >
      {selectHintText && (
        <div
          id="st-tip-select-hint"
          style={{ color: '#b45309', borderBottom: '1px dashed #d4d4d4', marginBottom: '2px', paddingBottom: '2px' }}
        >
          {selectHintText}
        </div>
      )}
      {tip.text}
    </div>
  );
}
