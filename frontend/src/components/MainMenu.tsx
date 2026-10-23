import React, { useSyncExternalStore } from 'react';
import UiButton from '../ui/UiButton.tsx';
import DesignSpace from '../ui/DesignSpace.tsx';
import { settingsStore } from '../domain/settings.ts';
import '../../css/menu-masterduel.css';

// Keep the existing navigation and desktop bridge contracts in this presentation layer.
interface MainMenuProps {
  onDuel: () => void;
  onPractice: () => void;
  onDeck: () => void;
  onReplay: () => void;
  onQuit: () => void;
}

const MENU_VERSION = 'YGOPro Version:1.036.2';

export default function MainMenu({ onDuel, onPractice, onDeck, onReplay, onQuit }: MainMenuProps) {
  const settings = useSyncExternalStore(settingsStore.subscribe, settingsStore.getSnapshot);
  const playerName =
    String(settings.nickname || 'DUELIST').trim().slice(0, 16) || 'DUELIST';
  const playerInitial = Array.from(playerName)[0] || 'D';

  return (
    <div id="main-menu-screen" className="screen active">
      <DesignSpace>
        <div id="main-menu-window" className="gfw-window gfw-mainmenu">
          {/* 版本水印：仍是窗口内唯一 .gfw-title，文本不可改（冒烟契约） */}
          <div className="gfw-title">{MENU_VERSION}</div>

          <div className="gfw-body">
            <div className="mdm-stage" aria-hidden="true">
              <span className="mdm-stage-slab mdm-stage-slab--a" />
              <span className="mdm-stage-slab mdm-stage-slab--b" />
              <span className="mdm-stage-line mdm-stage-line--a" />
              <span className="mdm-stage-line mdm-stage-line--b" />
              <span className="mdm-stage-glow" />
            </div>

            <header className="mdm-topbar">
              <section className="mdm-player" aria-label="玩家信息">
                <span className="mdm-avatar" aria-hidden="true">
                  <span className="mdm-avatar-initial">{playerInitial}</span>
                </span>
                <span className="mdm-player-copy">
                  <span className="mdm-player-name">{playerName}</span>
                  <span className="mdm-player-meta" aria-hidden="true">
                    <span className="mdm-level">决斗者 / DUELIST</span>
                  </span>
                </span>
              </section>

              <div className="mdm-topright">
                <div className="mdm-brand" aria-hidden="true">GOYGOPRO</div>
                <div className="mdm-status" aria-label="客户端信息">
                  <span className="mdm-status-item mdm-status-item--online">
                    <span className="mdm-chip-dot" aria-hidden="true" />
                    本地客户端
                  </span>
                  <span className="mdm-status-item">MR2020</span>
                  <span className="mdm-status-item">LOCAL</span>
                </div>
              </div>
            </header>

            <div className="mdm-intro">
              <p className="mdm-eyebrow">THE NEXT TURN IS YOURS</p>
              <h1>每一张卡，<br />开启无限可能<span>。</span></h1>
              <p className="mdm-intro-sub">构筑你的战术，迎接下一场决斗。</p>
            </div>

            <figure className="mdm-showcase" aria-label="青眼白龙主题展示">
              <div className="mdm-orbit mdm-orbit--outer" />
              <div className="mdm-orbit mdm-orbit--inner" />
              <div className="mdm-monster-art" role="img" aria-label="青眼白龙卡片插画" />
              <span className="mdm-art-index" aria-hidden="true">001 / LEGENDARY MONSTER</span>
              <figcaption>
                <span className="mdm-eyebrow">BLUE-EYES WHITE DRAGON</span>
                <strong>青眼白龙</strong>
                <span className="mdm-monster-stats">LIGHT · DRAGON <i /> ATK <b>3000</b> / DEF <b>2500</b></span>
              </figcaption>
            </figure>

            <nav className="menu-actions mdm-menu" aria-label="主菜单">
              <UiButton
                id="menu-btn-lan"
                className="mdm-item mdm-item--lan"
                aria-label="联机模式"
                size="compact-md"
                onClick={onDuel}
              >
                联机模式
              </UiButton>
              <UiButton
                id="menu-btn-single"
                className="mdm-item mdm-item--single"
                aria-label="单人模式"
                size="compact-md"
                onClick={onPractice}
              >
                单人模式
              </UiButton>
              <UiButton
                id="menu-btn-replay"
                className="mdm-item mdm-item--replay"
                aria-label="观看录像"
                size="compact-md"
                onClick={onReplay}
              >
                观看录像
              </UiButton>
              <UiButton
                id="menu-btn-deck"
                className="mdm-item mdm-item--deck"
                aria-label="编辑卡组"
                size="compact-md"
                onClick={onDeck}
              >
                编辑卡组
              </UiButton>
              <UiButton
                id="menu-btn-exit"
                className="mdm-item mdm-item--exit"
                aria-label="退出"
                size="compact-md"
                onClick={onQuit}
              >
                退出
              </UiButton>
            </nav>

            <aside className="mdm-topics" aria-label="资讯">
              <div className="mdm-topics-head" aria-hidden="true">
                <span>决斗指南</span>
                <span>GET STARTED / 01</span>
              </div>
              <div className="mdm-topics-body">
                <span className="mdm-topics-art" aria-hidden="true" />
                <p className="mdm-topics-title">从你的第一套卡组开始</p>
                <p className="mdm-topics-sub">编辑卡组 → 单人练习 → 联机决斗</p>
              </div>
            </aside>

            <footer className="menu-foot"><span>GOYGOPRO / DUEL TERMINAL</span><span>© goygopro · ocgcore</span></footer>
          </div>
        </div>
      </DesignSpace>
    </div>
  );
}
