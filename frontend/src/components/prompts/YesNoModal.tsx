/**
 * YesNo 弹窗（原版 wQuery，game.cpp:535-541）：窗口 (490,200)-(840,340) 350×140；
 * 是 (100,105)-(150,130)、否 (200,105)-(250,130)。swap_yes_no_button
 * （game.cpp:1925 SwapYesNoButtons）为开时两钮位置互换（否左是右）。
 */
import { useSyncExternalStore } from 'react';
import { settingsStore } from '../../domain/settings.ts';

export function YesNoModal({ title, message, respond }: { title: string; message: string; respond: (yes: boolean) => void }) {
  const s = useSyncExternalStore(settingsStore.subscribe, settingsStore.getSnapshot);
  const swap = !!s.swap_yes_no_button;
  return (
    <div className="modal-box modal-query">
      <div className="modal-title">{title}</div>
      <div className="query-msg">{message}</div>
      {/* 原版默认是左否右（swap 时互换）。DOM 顺序恒为是/否（冒烟按 id 定位），
          视觉位置由 order 控制 */}
      <div className="query-btns">
        <button id="modal-btn-yes" className="btn btn-primary" style={{ order: swap ? 2 : 1 }} onClick={() => respond(true)}>是</button>
        <button id="modal-btn-no" className="btn btn-secondary" style={{ order: swap ? 1 : 2 }} onClick={() => respond(false)}>否</button>
      </div>
    </div>
  );
}
