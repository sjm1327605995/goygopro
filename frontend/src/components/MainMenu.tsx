import React from 'react';

// 波 H：主菜单 1:1 对齐原版 wMainMenu（game.cpp:199-206）：
// 280×215 小窗、标题 YGOPro Version:%X.0%X.%X（PRO_VERSION=0x1362）、
// 5 个 260×30 纵排按钮（SysString 1200/1201/1202/1204/1210）。
// 单人模式按钮暂直连现有练习决斗（原版 wSinglePlay/bot 列表是后续波）。
interface MainMenuProps {
  onDuel: () => void;
  onPractice: () => void;
  onDeck: () => void;
  onReplay: () => void;
  onQuit: () => void;
}

const MENU_VERSION = 'YGOPro Version:1.03.2';

export default function MainMenu({ onDuel, onPractice, onDeck, onReplay, onQuit }: MainMenuProps) {
  return (
    <div id="main-menu-screen" className="screen active">
      <div id="main-menu-window" className="gfw-window gfw-mainmenu">
        <div className="gfw-title">{MENU_VERSION}</div>
        <div className="gfw-body" style={{ padding: '10px' }}>
          <div className="gfw-col">
            <button id="menu-btn-lan" className="gfw-btn btn" onClick={onDuel}>联机模式</button>
            <button id="menu-btn-single" className="gfw-btn btn" onClick={onPractice}>单人模式</button>
            <button id="menu-btn-replay" className="gfw-btn btn" onClick={onReplay}>观看录像</button>
            <button id="menu-btn-deck" className="gfw-btn btn" onClick={onDeck}>编辑卡组</button>
            <button id="menu-btn-exit" className="gfw-btn btn" onClick={onQuit}>退出</button>
          </div>
        </div>
      </div>
    </div>
  );
}
