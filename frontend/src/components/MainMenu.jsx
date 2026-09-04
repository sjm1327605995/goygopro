import React from 'react';

export default function MainMenu({ onDuel, onPractice, onDeck, onReplay }) {
  return (
    <div id="main-menu-screen" className="screen active">
      <div className="menu-header">
        <h1 className="game-title">YGOPro 3D</h1>
        <p className="game-subtitle">次世代决斗界面与引擎</p>
      </div>

      <div className="menu-grid">
        <div className="menu-card-btn" onClick={onDuel}>
          <div className="menu-card-icon">⚔️</div>
          <div className="menu-card-title">在线决斗</div>
          <div className="menu-card-desc">连接 YGOPro 服务器，与全球决斗者对战。</div>
        </div>
        <div className="menu-card-btn" onClick={onPractice}>
          <div className="menu-card-icon">⚡</div>
          <div className="menu-card-title">练习决斗</div>
          <div className="menu-card-desc">离线对抗智能 AI，尽享全 3D 动画。</div>
        </div>
        <div className="menu-card-btn" onClick={onDeck}>
          <div className="menu-card-icon">🎴</div>
          <div className="menu-card-title">卡组构筑</div>
          <div className="menu-card-desc">构建、搜索、测试起手，保存 YDK 卡组。</div>
        </div>
        <div className="menu-card-btn" onClick={onReplay}>
          <div className="menu-card-icon">🎬</div>
          <div className="menu-card-title">回放剧场</div>
          <div className="menu-card-desc">逐步回放对局，支持倍速播放与分析。</div>
        </div>
      </div>

      <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginBottom: '10px' }}>
        基于 React、Three.js 与 Go 后端桥接
      </div>
    </div>
  );
}
