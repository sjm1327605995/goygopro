import React from 'react';
import UiButton from '../ui/UiButton.tsx';
import DesignSpace from '../ui/DesignSpace.tsx';

// 主菜单 = 原版 wMainMenu（game.cpp:199-206）：(370,200)-(650,415) 280×215 窗口、
// 标题栏版本号、5 个 260×30 按钮（10,30 起间距 5），整体在 1024×640 设计
// 坐标系里随窗口拉伸（DesignSpace）。
interface MainMenuProps {
  onDuel: () => void;
  onPractice: () => void;
  onDeck: () => void;
  onReplay: () => void;
  onQuit: () => void;
}

const MENU_VERSION = 'YGOPro Version:1.036.2';

export default function MainMenu({ onDuel, onPractice, onDeck, onReplay, onQuit }: MainMenuProps) {
  return (
    <div id="main-menu-screen" className="screen active">
      <DesignSpace>
        <div id="main-menu-window" className="gfw-window gfw-mainmenu">
          <div className="gfw-title">{MENU_VERSION}</div>
          <div className="gfw-body">
            <div className="gfw-col">
              {/* 原版按钮 (10,30) 起 260×30 纵向间距 35（高 30 + 间隙 5） */}
              <UiButton id="menu-btn-lan" size="compact-md" fullWidth onClick={onDuel}>联机模式</UiButton>
              <UiButton id="menu-btn-single" size="compact-md" fullWidth onClick={onPractice}>单人模式</UiButton>
              <UiButton id="menu-btn-replay" size="compact-md" fullWidth onClick={onReplay}>观看录像</UiButton>
              <UiButton id="menu-btn-deck" size="compact-md" fullWidth onClick={onDeck}>编辑卡组</UiButton>
              <UiButton id="menu-btn-exit" size="compact-md" fullWidth onClick={onQuit}>退出</UiButton>
            </div>
          </div>
        </div>
      </DesignSpace>
    </div>
  );
}
