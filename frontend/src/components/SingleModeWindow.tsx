import React, { useEffect, useState } from 'react';
import { WailsBridge } from '../wails_bridge.ts';

/**
 * 波 J：单人模式窗口 — 1:1 对齐原版 wSinglePlay（game.cpp:848-890）。
 * 580×420 窗口、左侧谜题列表（10,10→350,350）、右侧「主要信息：」面板
 * （360,10→550,30 + 360,40→560,160）、回卡组顶端复选框（1238）、
 * 确定（1211）/退出（1210）。
 * 原版的人机模式 tab 仅在 enable_bot_mode 时创建；无 bot.conf 的构建里
 * 窗口只有残局列表 —— 这里对应无 bot 模式的形态。
 */
interface SingleModeWindowProps {
  onNavigate: (screen: string) => void;
  /** 确定（BUTTON_LOAD_SINGLEPLAY）：谜题已加载，切决斗画面 */
  onStart: () => void;
}

export default function SingleModeWindow({ onNavigate, onStart }: SingleModeWindowProps) {
  const [singles, setSingles] = useState<{ name: string; message: string }[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [returnDeckTop, setReturnDeckTop] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    WailsBridge.listSingles().then((list) => {
      setSingles(list || []);
      if (list && list.length) setSelected(list[0].name);
    });
  }, []);

  const start = async (): Promise<void> => {
    if (!selected) return;
    const res = await WailsBridge.startSingle(selected, returnDeckTop);
    if (res && res.success) {
      onStart();
    } else {
      setError((res && res.error) || '加载谜题失败');
    }
  };

  const selectedInfo = singles.find((s) => s.name === selected);

  return (
    <div
      id="single-screen"
      className="screen active"
      style={{ background: "url('../textures/bg_menu.jpg') center/cover no-repeat" }}
    >
      <div className="gfw-window gfw-single">
        <div className="gfw-title">单人模式</div>
        <div className="gfw-body" style={{ padding: 0 }}>
          {/* 谜题列表（原版 tab 内 10,10→350,350；窗口坐标 y+20） */}
          <div
            id="single-list"
            className="gfw-list"
            style={{ position: 'absolute', left: 10, top: 30, width: 340, height: 340 }}
          >
            {singles.map((s) => (
              <div
                key={s.name}
                className={`gfw-list-item${s.name === selected ? ' gfw-selected' : ''}`}
                onClick={() => setSelected(s.name)}
                onDoubleClick={start}
              >
                {s.name}
              </div>
            ))}
            {!singles.length && (
              <div className="gfw-list-item" style={{ color: '#8a8a8a' }}>（没有找到谜题脚本）</div>
            )}
          </div>

          {/* 主要信息：stSinglePlayInfo（360,10→550,30 标签 + 360,40→560,160 文本） */}
          <div className="gfw-label" style={{ position: 'absolute', left: 360, top: 30, width: 190 }}>
            主要信息：
          </div>
          <div
            id="single-info"
            style={{
              position: 'absolute', left: 360, top: 60, width: 200, height: 120,
              fontSize: 13, color: '#20202a', whiteSpace: 'pre-wrap', overflowY: 'auto',
            }}
          >
            {selectedInfo ? selectedInfo.message : ''}
          </div>

          {/* chkSinglePlayReturnDeckTop（SysString 1238） */}
          <label
            className="gfw-label"
            style={{ position: 'absolute', left: 360, top: 280, width: 200, display: 'flex', gap: 6, alignItems: 'center' }}
          >
            <input
              id="single-return-decktop"
              type="checkbox"
              className="gfw-check"
              checked={returnDeckTop}
              onChange={(e) => setReturnDeckTop(e.target.checked)}
            />
            不洗切时回卡组改为回顶端
          </label>

          {/* 确定（459,301→569,326）/ 退出（459,331→569,356） */}
          <button
            id="single-start"
            className="gfw-btn btn"
            style={{ position: 'absolute', left: 459, top: 321, width: 110, height: 25 }}
            disabled={!selected}
            onClick={start}
          >确定</button>
          <button
            id="single-exit"
            className="gfw-btn btn"
            style={{ position: 'absolute', left: 459, top: 351, width: 110, height: 25 }}
            onClick={() => onNavigate('menu')}
          >退出</button>

          {error && (
            <div
              id="single-error"
              style={{ position: 'absolute', left: 360, top: 200, width: 200, color: '#b91c1c', fontSize: 12 }}
            >{error}</div>
          )}
        </div>
      </div>
    </div>
  );
}
