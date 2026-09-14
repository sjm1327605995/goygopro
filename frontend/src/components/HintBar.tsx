/**
 * stHintMsg 等价物：屏幕上方居中的提示条（原版 game.cpp:636 白底黑字）。
 *
 * 文本来自 store.hint（duel:waiting → SysString 1409「等待对方行动中...」）；
 * 任何弹窗类提示事件到达时 reducer 的 HINT_CLEARING 会清掉它——与原版
 * "弹窗打开时提示条让位" 的行为一致。
 */
import React from 'react';
import { useSyncExternalStore } from 'react';
import { duelStore } from '../duel/store.ts';

export default function HintBar() {
  const hint = useSyncExternalStore(duelStore.subscribe, duelStore.getState).hint;
  return (
    <div id="hint-bar" className="hint-bar" style={{ display: hint ? 'block' : 'none' }}>
      {hint || ''}
    </div>
  );
}
