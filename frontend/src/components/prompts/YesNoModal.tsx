/**
 * YesNo 弹窗（原版 wQuery 的 btnYes/btnNo）。swap_yes_no_button 设置
 * （环境设置·系统设定，game.cpp SwapYesNoButtons）为开时交换左右位置：
 * 原版默认「是」在左「否」在右，交换后「否」左「是」右。
 */
import { useSyncExternalStore } from 'react';
import { settingsStore } from '../../domain/settings.ts';

export function YesNoModal({ title, message, respond }: { title: string; message: string; respond: (yes: boolean) => void }) {
  const s = useSyncExternalStore(settingsStore.subscribe, settingsStore.getSnapshot);
  const swap = !!s.swap_yes_no_button;
  const yesBtn = (
    <button key="yes" id="modal-btn-yes" className="btn btn-primary" onClick={() => respond(true)}>是</button>
  );
  const noBtn = (
    <button key="no" id="modal-btn-no" className="btn btn-secondary" onClick={() => respond(false)}>否</button>
  );
  return (
    <div className="modal-box">
      <div className="modal-title">{title}</div>
      <p style={{ color: 'var(--text-main)', fontSize: '14px', lineHeight: 1.5 }}>{message}</p>
      <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '12px', marginTop: '10px' }}>
        {swap ? [yesBtn, noBtn] : [noBtn, yesBtn]}
      </div>
    </div>
  );
}