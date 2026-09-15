import React from 'react';

// 主菜单按 docs/main_menu_layout.html 原型：320 宽窗口、18vh 顶部留白、
// 标题居中 26px 深蓝渐变（版本号 1.036.2）、5 个 296×32 按钮间距 6。
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
      <div id="main-menu-window" className="gfw-window gfw-mainmenu">
        <div className="gfw-title">{MENU_VERSION}</div>
        <div className="gfw-body" style={{ padding: '8px 12px 10px' }}>
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
