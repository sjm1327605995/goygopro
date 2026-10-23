import React, { useEffect, useState } from 'react';
import { WailsBridge } from '../wails_bridge.ts';
import UiButton from '../ui/UiButton.tsx';
import { UiCheckbox } from '../ui/UiInput.tsx';
import DesignSpace from '../ui/DesignSpace.tsx';
import ScreenHeading from '../ui/ScreenHeading.tsx';

/**
 * 波 J：单人模式窗口 — 布局源自原版 wSinglePlay（game.cpp:848-890）。
 * 现窗口 660×470（css/gframe-window.css .gfw-single）；左侧谜题列表
 * (14,10) 400×420、右侧「主要信息：」标签/卡面自 430 起、回卡组顶端复选框（1238）、
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
    >
      <DesignSpace>
      <ScreenHeading title="单人模式" subtitle="SOLO / 磨练战术，解开残局" />
      <div className="gfw-window gfw-single">
        <div className="gfw-title">单人模式</div>
        {/* 内容区 446px（470-24 标题）；坐标沿用原版窗口相对 y-24
            （原版 tab 控件偏移 +20 已含在下列数值里，game.cpp:850/880-886） */}
        <div className="gfw-body" style={{ padding: 0, position: 'relative', height: '446px' }}>
          {/* 谜题列表：左侧大列表 */}
          <div
            id="single-list"
            className="gfw-list single-list"
            style={{ position: 'absolute', left: 14, top: 10, width: 400, height: 420 }}
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
              <div className="gfw-list-item" style={{ color: '#64748b' }}>（没有找到谜题脚本）</div>
            )}
          </div>

          {/* 主要信息：右侧卡片 */}
          <div className="gfw-label single-info-label">主要信息</div>
          <div id="single-info" className="single-info-card">
            {selectedInfo ? selectedInfo.message : '选择左侧谜题查看说明。'}
          </div>

          {/* chkSinglePlayReturnDeckTop（SysString 1238） */}
          <label className="gfw-label single-return-row">
            <UiCheckbox
              id="single-return-decktop"
              checked={returnDeckTop}
              onChange={(e) => setReturnDeckTop(e.target.checked)}
            />
            不洗切时回卡组改为回顶端
          </label>

          {error && <div id="single-error" className="single-error">{error}</div>}

          {/* 确定 btnLoadSinglePlay / 退出 btnSinglePlayCancel */}
          <UiButton
            id="single-start"
            style={{ position: 'absolute', left: 430, top: 390, width: 105, height: 34 }}
            disabled={!selected}
            onClick={start}
          >确定</UiButton>
          <UiButton
            id="single-exit"
            style={{ position: 'absolute', left: 545, top: 390, width: 100, height: 34 }}
            onClick={() => onNavigate('menu')}
          >退出</UiButton>
        </div>
      </div>
      </DesignSpace>
    </div>
  );
}
