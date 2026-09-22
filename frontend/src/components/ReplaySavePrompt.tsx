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

  return (
    <div id="replay-save-prompt" style={{
      position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.6)',
      display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1600,
    }}>
      <div style={{
        background: 'var(--bg-card)', border: '1px solid var(--border-cyan)',
        borderRadius: '12px', padding: '24px', width: '340px',
        display: 'flex', flexDirection: 'column', gap: '12px',
      }}>
        <h3 style={{ color: 'var(--primary)', margin: 0 }}>是否保存录像？</h3>
        <input
          id="replay-save-name"
          className="form-input"
          value={name}
          onChange={(e) => setName(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter') save(); }}
          placeholder="录像名称"
        />
        <div id="replay-save-status" style={{ fontSize: '12px', color: 'var(--text-muted)', minHeight: '16px' }}>{status}</div>
        <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
          <button id="replay-save-cancel" className="btn btn-secondary" onClick={() => setSuggested(null)}>取消</button>
          <button id="replay-save-confirm" className="btn btn-primary" onClick={save} disabled={saving}>保存</button>
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
