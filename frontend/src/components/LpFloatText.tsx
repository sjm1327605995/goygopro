import React, { useEffect, useState } from 'react';
import { eventBus } from '../wails_bridge.ts';
import { duelStore } from '../duel/store.ts';
import { seatToDisplay } from '../domain/reducer.ts';

/**
 * LP 变动浮字（原版 lpcstring，drawing.cpp:630-636）：
 * LP 增减时场中大字渐隐——显示座 0（己方，屏幕下方）画在 (400,472)-(922,520)，
 * 显示座 1（对方）画在 (400,162)-(922,210)。原版同时刻只存一条（新值覆盖），
 * 这里同样只保留每方最新一条。
 */
interface FloatItem {
  disp: 0 | 1;
  text: string;
  key: number;
}

let seq = 0;

export default function LpFloatText() {
  const [floats, setFloats] = useState<Partial<Record<0 | 1, FloatItem>>>({});

  useEffect(() => {
    const timers: number[] = [];
    const push = (seat: number, delta: number) => {
      if (!delta) return;
      const disp = seatToDisplay(duelStore.getState(), seat);
      const key = ++seq;
      setFloats((f) => ({ ...f, [disp]: { disp, text: delta > 0 ? `+${delta}` : `${delta}`, key } }));
      timers.push(window.setTimeout(() => {
        setFloats((f) => (f[disp]?.key === key ? { ...f, [disp]: undefined } : f));
      }, 1300));
    };
    const onDamage = (d: any) => push(Number(d.player), -Math.abs(Number(d.amount) || 0));
    const onRecover = (d: any) => push(Number(d.player), Math.abs(Number(d.amount) || 0));
    const onPay = (d: any) => push(Number(d.player), -Math.abs(Number(d.amount) || 0));
    eventBus.on('duel:damage', onDamage);
    eventBus.on('duel:recover', onRecover);
    eventBus.on('duel:pay_lpcost', onPay);
    return () => {
      eventBus.off('duel:damage', onDamage);
      eventBus.off('duel:recover', onRecover);
      eventBus.off('duel:pay_lpcost', onPay);
      timers.forEach((t) => window.clearTimeout(t));
    };
  }, []);

  return (
    <>
      {([0, 1] as const).map((disp) => {
        const item = floats[disp];
        if (!item) return null;
        return (
          <div
            key={item.key}
            id={`lp-float-${disp}`}
            className={`lp-float lp-float-${disp}`}
          >
            {item.text}
          </div>
        );
      })}
    </>
  );
}
