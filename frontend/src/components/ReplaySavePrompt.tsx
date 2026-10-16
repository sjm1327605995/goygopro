import React, { useEffect, useState, useSyncExternalStore } from 'react';
import { eventBus, WailsBridge } from '../wails_bridge.ts';
import { settingsStore } from '../domain/settings.ts';
import { duelStore } from '../duel/store.ts';
import { WIN_BANNER_MS } from '../domain/duel_banners.ts';

/**
 * 录像保存确认（gframe duelclient.cpp:727-774 wReplaySave 的翻译）：
 * 服务器在决斗结束时推送 STOC_REPLAY，auto_save_replay=1 直接按建议名
 * 落盘；否则弹窗让玩家确认/改名，取消则不保存。
 */
export default function ReplaySavePrompt() {
  const [suggested, setSuggested] = useState<string | null>(null);
  const [name, setName] = useState('');
  const [status, setStatus] = useState('');
  const [saving, setSaving] = useState(false);
  const [mode, setMode] = useState<'net' | 'single'>('net');
  const settingsSnap = useSyncExternalStore(settingsStore.subscribe, settingsStore.getSnapshot);
  const autoSaveRef = useRefLatest(!!settingsSnap.auto_save_replay);

  useEffect(() => {
    const pending = new Set<ReturnType<typeof setTimeout>>();
    const afterResult = (callback: () => void): void => {
      const win = duelStore.getState().win;
      const remaining = win ? Math.max(0, win.announcedAt + WIN_BANNER_MS - Date.now()) : 0;
      if (!remaining) { callback(); return; }
      const timer = setTimeout(() => { pending.delete(timer); callback(); }, remaining);
      pending.add(timer);
    };
    const onReplay = (data: any): void => {
      setMode('net');
      const suggestedName = (data && data.name) || '';
      if (autoSaveRef.current) {
        // 原版自动保存路径：不弹窗直接落盘
        WailsBridge.saveLastReplay(suggestedName).then((res) => {
          setStatus(res && res.success ? `已自动保存录像 ${res.name}` : `自动保存失败：${res && res.error}`);
        });
        return;
      }
      setName(suggestedName);
      setStatus('');
      setSuggested(suggestedName);
    };
    const onSingleReplay = (data: any): void => {
      setMode('single');
      const suggestedName = (data && data.name) || '';
      // 单机录像由服务端录制；auto_save 时 Go 已直接落盘，只提示结果、不再二次保存。
      if (data && data.autoSaved) {
        setSuggested(null);
        setStatus(`已自动保存录像 ${suggestedName}`);
        return;
      }
      setName(suggestedName);
      setStatus('');
      setSuggested(suggestedName);
    };
    const onNet = (data: any) => afterResult(() => onReplay(data));
    const onSingle = (data: any) => afterResult(() => onSingleReplay(data));
    const onStart = () => {
      pending.forEach(clearTimeout);
      pending.clear();
      setSuggested(null);
      setStatus('');
    };
    eventBus.on('stoc:replay', onNet);
    eventBus.on('single:replay', onSingle);
    eventBus.on('duel:start', onStart);
    return () => {
      pending.forEach(clearTimeout);
      eventBus.off('stoc:replay', onNet);
      eventBus.off('single:replay', onSingle);
      eventBus.off('duel:start', onStart);
    };
  }, [autoSaveRef]);

  const save = async (): Promise<void> => {
    setSaving(true);
    const res = mode === 'single'
      ? await WailsBridge.saveSingleReplay(name)
      : await WailsBridge.saveLastReplay(name);
    setSaving(false);
    if (res && res.success) {
      setStatus(`已保存录像 ${res.name}`);
      setSuggested(null);
    } else {
      setStatus(`保存失败：${res && res.error}`);
    }
  };

  if (suggested === null) {
    return status ? (
      <div id="replay-save-status" style={{
        position: 'fixed', bottom: '20px', left: '50%', transform: 'translateX(-50%)',
        background: 'var(--bg-card)', border: '1px solid var(--border-cyan)',
        borderRadius: '8px', padding: '8px 16px', color: 'var(--primary)',
        fontSize: '13px', zIndex: 1500,
      }}>{status}</div>
    ) : null;
  }

  // 原版 wReplaySave（game.cpp:888-895）：(510,200)-(820,320) 310×120 无遮罩小窗；
  // 标签 (20,25)-(290,45)、ebRSName (20,50)-(290,70)、是 (70,80)-(140,105)、
  // 否 (170,80)-(240,105)；swap_yes_no_button 时两钮互换（game.cpp:1931）。
  // ResizeWin scales the center (665,260), preserving the 310×120 window size.
  const swap = !!settingsSnap.swap_yes_no_button;
  return (
    <div
      id="replay-save-prompt"
      className="gfw-window"
      style={{
        position: 'fixed', left: 'calc(64.94140625% - 155px)', top: 'calc(40.625% - 60px)', width: 310, height: 120,
        transform: 'none', zIndex: 1600,
      }}
    >
      <div className="gfw-title">是否保存录像？</div>
      <div style={{ position: 'relative', height: 'calc(100% - 24px)' }}>
        <div className="gfw-label" style={{ position: 'absolute', left: 20, top: 1 }}>录像文件：</div>
        <input
          id="replay-save-name"
          className="form-input"
          style={{ position: 'absolute', left: 20, top: 26, width: 270, height: 20, textAlign: 'center', boxSizing: 'border-box' }}
          value={name}
          onChange={(e) => setName(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter') save(); }}
          placeholder="录像名称"
        />
        <div id="replay-save-status" style={{ position: 'absolute', left: '6.5%', top: '47%', fontSize: '12px', color: 'var(--text-muted)' }}>{status}</div>
        <div style={{ position: 'absolute', left: 70, top: 56, display: 'flex', gap: 30 }}>
          <button
            id="replay-save-confirm"
            className="btn btn-primary"
            style={{ width: '70px', height: '25px', padding: 0, order: swap ? 2 : 1 }}
            onClick={save}
            disabled={saving}
          >保存</button>
          <button
            id="replay-save-cancel"
            className="btn btn-secondary"
            style={{ width: '70px', height: '25px', padding: 0, order: swap ? 1 : 2 }}
            onClick={() => setSuggested(null)}
          >取消</button>
        </div>
      </div>
    </div>
  );
}

// 捕获最新值的 ref（事件回调里读设置不需要重新订阅）
function useRefLatest<T>(value: T) {
  const ref = React.useRef(value);
  ref.current = value;
  return ref;
}
