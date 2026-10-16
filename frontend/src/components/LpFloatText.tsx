import React, { useContext, useSyncExternalStore } from 'react';
import { duelStore } from '../duel/store.ts';
import { useLifePointFrame } from '../ui/useLifePointFrame.ts';
import { DesignScaleContext } from '../ui/DesignSpace.tsx';

/**
 * LP 变动浮字（原版 lpcstring，drawing.cpp:630-636）：
 * LP 增减时场中大字渐隐——显示座 0（己方，屏幕下方）画在 (400,472)-(922,520)，
 * 显示座 1（对方）画在 (400,162)-(922,210)。原版同时刻只存一条（新值覆盖），
 * 这里同样只保留每方最新一条。
 */
export default function LpFloatText() {
  const { lpChange } = useSyncExternalStore(duelStore.subscribe, duelStore.getState);
  const frame = useLifePointFrame(lpChange);
  const scale = useContext(DesignScaleContext);
  if (!lpChange || !frame?.floating) return null;
  return <div style={{ position: 'fixed', left: 661, top: lpChange.player === 0 ? 472 : 162,
    transformOrigin: '0 0', transform: `scale(${1 / scale.sx}, ${1 / scale.sy})`, pointerEvents: 'none', zIndex: 900 }}>
    <div id={`lp-float-${lpChange.player}`} className={`lp-float lp-float-${lpChange.player}`}
      style={{ position: 'relative', left: 0, top: 0, width: 'max-content', transform: 'translateX(-50%)',
        fontSize: Math.trunc(48 * scale.sy), lineHeight: 1, fontWeight: 400, animation: 'none',
        color: frame.color, opacity: frame.alpha, textShadow: `0 ${2 * scale.sy}px #fff` }}>
      {lpChange.kind === 'recover' ? '+' : '-'}{lpChange.amount}
    </div>
  </div>;
}
