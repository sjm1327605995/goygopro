import React, { useEffect, useRef } from 'react';
import { DuelField3D } from '../duel/field3d.js';
import { DuelHUD } from '../ui/hud.js';
import { DuelManager } from '../duel/duel_manager.js';
import { WailsBridge } from '../wails_bridge.js';

/**
 * Renders the 3D duel board + HUD and wires the imperative Three.js / HUD /
 * DuelManager stack into it once on mount. The duel itself is driven by Go
 * (ocgcore engine + replay driver); the events arrive on the shared event bus
 * and this component only visualizes them.
 *
 * Used by both the live DuelScreen and the ReplayTheater (which replays the
 * recorded event stream against the same 3D board).
 */
export default function DuelStage({ onExit, showSurrender = true, onReady }) {
  const fieldRef = useRef(null);
  // Kept in a ref so the mount effect does not depend on the callback identity.
  const onReadyRef = useRef(onReady);
  onReadyRef.current = onReady;

  useEffect(() => {
    const container = document.getElementById('duel-canvas-container');
    const hudElements = {
      playerLP: document.getElementById('player-lp-val'),
      playerLPFill: document.getElementById('player-lp-fill'),
      opponentLP: document.getElementById('opponent-lp-val'),
      opponentLPFill: document.getElementById('opponent-lp-fill'),
      handContainer: document.getElementById('hand-cards-dock'),
      actionPopup: document.getElementById('action-popup'),
      inspectorPicCanvas: document.getElementById('inspector-pic-canvas'),
      inspectorName: document.getElementById('inspector-card-name'),
      inspectorBadges: document.getElementById('inspector-badges'),
      inspectorStats: document.getElementById('inspector-stats'),
      inspectorDesc: document.getElementById('inspector-card-desc'),
      modalOverlay: document.getElementById('modal-overlay'),
      logList: document.getElementById('duel-log-list'),
    };

    // The manager is constructed after the field, so the click callback binds
    // to it through a mutable local (all callbacks fire only after mount).
    let manager = null;
    const field3D = new DuelField3D(
      container,
      (code, info) => hud.inspectCard(code, info),
      (x, y, userData) => {
        if (manager) manager.onFieldCardClick(x, y, userData);
      }
    );
    const hud = new DuelHUD(hudElements, (type, data) => {
      if (manager) manager.handlePlayerAction(type, data);
    });
    manager = new DuelManager(field3D, hud);
    // Contextual hand popup: only actions the engine offered this turn.
    hud.handActionProvider = (code) => manager.getHandOptions(code);
    // Real card art (pics/<code>.jpg in the app, YGOProDeck CDN in preview).
    field3D.cardImageProvider = (code) => WailsBridge.getCardImage(code);

    fieldRef.current = { field3D, hud, duelManager };
    if (onReadyRef.current) onReadyRef.current(fieldRef.current);

    return () => {
      if (onReadyRef.current) onReadyRef.current(null);
      if (fieldRef.current) {
        fieldRef.current.duelManager.dispose();
        fieldRef.current.field3D.dispose();
      }
      fieldRef.current = null;
    };
  }, []);

  const handleSurrender = () => {
    if (window.confirm('确定要投降吗？')) {
      WailsBridge.surrender();
      if (onExit) onExit('menu');
    }
  };

  return (
    <div className="duel-stage" style={{ position: 'relative', width: '100%', height: '100%' }}>
      <div id="duel-canvas-container"></div>

      <div id="duel-hud-layer">
        <div className="duel-top-bar">
          <div className="player-info-card">
            <div className="player-avatar" style={{ borderColor: '#ef4444' }}>OP</div>
            <div className="player-meta">
              <div className="player-name">
                <span id="opponent-name-val">SetoKaiba</span>
                <span id="opponent-lp-val" className="lp-number">8000</span>
              </div>
              <div className="lp-bar-container">
                <div id="opponent-lp-fill" className="lp-bar-fill"></div>
              </div>
            </div>
          </div>

          <div className="phase-bar">
            <div className="phase-item active" data-phase="DP">DP</div>
            <div className="phase-item" data-phase="SP">SP</div>
            <div className="phase-item" data-phase="M1">M1</div>
            <div className="phase-item" data-phase="BP">BP</div>
            <div className="phase-item" data-phase="M2">M2</div>
            <div className="phase-item" data-phase="EP">EP</div>
          </div>

          <div className="player-info-card">
            <div className="player-avatar">YOU</div>
            <div className="player-meta">
              <div className="player-name">
                <span id="player-name-val">YugiMuto</span>
                <span id="player-lp-val" className="lp-number">8000</span>
              </div>
              <div className="lp-bar-container">
                <div id="player-lp-fill" className="lp-bar-fill"></div>
              </div>
            </div>
          </div>
        </div>

        <div className="card-inspector">
          <div className="inspector-pic-box">
            <canvas id="inspector-pic-canvas" width="220" height="230"></canvas>
          </div>
          <div className="inspector-details">
            <div id="inspector-card-name" className="inspector-name">选择一张卡</div>
            <div id="inspector-badges" className="inspector-badges"></div>
            <div id="inspector-stats" className="inspector-stats" style={{ display: 'none' }}></div>
            <div id="inspector-card-desc" className="inspector-desc">悬停场上或手牌中的卡牌，查看完整详情、卡牌背景与效果。</div>
          </div>
        </div>

        <div className="duel-log-drawer">
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
            <span style={{ fontWeight: 700, color: 'var(--primary)', fontSize: '13px' }}>决斗日志</span>
            {showSurrender && (
              <button className="btn btn-danger" style={{ padding: '4px 10px', fontSize: '11px' }} onClick={handleSurrender}>投降</button>
            )}
          </div>
          <div id="duel-log-list" className="log-messages"></div>
        </div>

        <div className="hand-dock-container">
          <div id="hand-cards-dock" className="hand-cards"></div>
        </div>
      </div>

      <div id="action-popup" className="action-popup" style={{ display: 'none' }}></div>
    </div>
  );
}
