import React, { useEffect, useState, useSyncExternalStore } from 'react';
import { eventBus, WailsBridge } from '../wails_bridge.ts';
import { settingsStore } from '../domain/settings.ts';

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
    eventBus.on('stoc:replay', onReplay);
    eventBus.on('single:replay', onSingleReplay);
    return () => {
      eventBus.off('stoc:replay', onReplay);
      eventBus.off('single:replay', onSingleReplay);
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
  // App 根不在 DesignSpace：用视口百分比等效非均匀缩放（与 1024×640 同数学）。
  const swap = !!settingsSnap.swap_yes_no_button;
  return (
    <div
      id="replay-save-prompt"
      className="gfw-window"
      style={{
        position: 'fixed', left: '49.8%', top: '31.25%', width: '30.28%', height: '18.75%',
        transform: 'none', zIndex: 1600,
      }}
    >
      <div className="gfw-title">保存录像</div>
      <div style={{ position: 'relative', height: 'calc(100% - 24px)' }}>
        <div className="gfw-label" style={{ position: 'absolute', left: '6.5%', top: '1%' }}>是否保存录像？</div>
        <input
          id="replay-save-name"
          className="form-input"
          style={{ position: 'absolute', left: '6.5%', top: '27%', width: '87%', boxSizing: 'border-box' }}
          value={name}
          onChange={(e) => setName(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter') save(); }}
          placeholder="录像名称"
        />
        <div id="replay-save-status" style={{ position: 'absolute', left: '6.5%', top: '47%', fontSize: '12px', color: 'var(--text-muted)' }}>{status}</div>
        <div style={{ position: 'absolute', left: '22.6%', top: '58%', display: 'flex', gap: '9.7%' }}>
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
