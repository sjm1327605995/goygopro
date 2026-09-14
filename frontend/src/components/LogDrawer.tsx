/**
 * 决斗日志抽屉（原版 wInfos 的 Log 页签独立成条）。
 *
 * 条目来自 store.log（reducer 的 withLog 追加，capped 200 条），自动滚到底。
 * 保留旧 hud 的 `.log-entry ${cls}` 类名与 `> ` 前缀，样式/断言可平移。
 */
import React, { useEffect, useRef } from 'react';
import { useSyncExternalStore } from 'react';
import { duelStore } from '../duel/store.ts';

export default function LogDrawer() {
  const log = useSyncExternalStore(duelStore.subscribe, duelStore.getState).log;
  const listRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = listRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [log]);

  return (
    <div className="duel-log-drawer">
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
        <span style={{ fontWeight: 700, color: 'var(--primary)', fontSize: '13px' }}>决斗日志</span>
      </div>
      <div id="duel-log-list" className="log-messages" ref={listRef}>
        {log.map((e) => (
          <div key={e.id} className={`log-entry ${e.cls}`}>{`> ${e.text}`}</div>
        ))}
      </div>
    </div>
  );
}
