import React, { useEffect, useRef } from 'react';
import { DuelField3D } from '../duel/field3d.ts';
import { DuelManager } from '../duel/duel_manager.ts';
import { WailsBridge } from '../wails_bridge.ts';
import { duelStore } from '../duel/store.ts';
import CardPreviewPanel from './CardPreviewPanel.tsx';
import PlayerPanel from './PlayerPanel.tsx';
import PhaseStrip from './PhaseStrip.tsx';
import RightControls from './RightControls.tsx';
import ChatOverlay from './ChatOverlay.tsx';
import HintBar from './HintBar.tsx';
import LogDrawer from './LogDrawer.tsx';
import HandDock from './HandDock.tsx';
import ActionPopup from './ActionPopup.tsx';
import PromptHost from './PromptHost.tsx';
import VictoryOverlay from './VictoryOverlay.tsx';
import SpecOverlay from './SpecOverlay.tsx';
import CardTooltip from './CardTooltip.tsx';

/**
 * Renders the 3D duel board + HUD and wires the imperative Three.js layer /
 * DuelManager into it once on mount. The duel itself is driven by Go
 * (ocgcore engine + replay driver); the events arrive on the shared event
 * bus: the store (duel/store.js → reducer.ts) is the single source of truth
 * for all 2D HUD state, DuelManager handles only 3D animation + protocol
 * auto-answers.
 *
 * Used by both the live DuelScreen and the ReplayTheater. The theater passes
 * interactive=false: no prompt popups (PromptHost unmounted), no protocol
 * sends, no hand/field click actions, and no victory modal.
 */
export interface StageHandle {
  field3D: any;
  duelManager: any;
}

interface DuelStageProps {
  onExit?: (screen: string) => void;
  showSurrender?: boolean;
  interactive?: boolean;
  /**
   * compact=true 用于回放剧场的小容器：不渲染为全屏设计的 2D HUD
   * （预览列/日志抽屉/手牌坞/聊天/提示条/悬浮特效会把 3D 场地遮死），
   * 也不渲染阶段条——回放里 phase 每步都变，actionable 闪光会一直闪。
   * 只保留有信息量的 LP 面板。
   */
  compact?: boolean;
  onReady?: (stage: StageHandle | null) => void;
}

export default function DuelStage({
  onExit,
  showSurrender = true,
  interactive = true,
  compact = false,
  onReady,
}: DuelStageProps) {
  const fieldRef = useRef<StageHandle | null>(null);
  // Kept in a ref so the mount effect does not depend on the callback identity.
  const onReadyRef = useRef(onReady);
  onReadyRef.current = onReady;

  useEffect(() => {
    const container = document.getElementById('duel-canvas-container')!;

    // The manager is constructed after the field, so the click callback binds
    // to it through a mutable local (all callbacks fire only after mount).
    let manager: DuelManager | null = null;
    const field3D = new DuelField3D(
      container,
      (code: number) => {
        // 左侧 CardPreviewPanel（旧 inspector 的 store 版）
        if (code) duelStore.inspect(code);
      },
      (x: number, y: number, userData: any) => {
        if (manager) manager.onFieldCardClick(x, y, userData);
      }
    );
    manager = new DuelManager(field3D, { interactive });
    // Real card art (pics/<code>.jpg in the app, YGOProDeck CDN in preview).
    field3D.cardImageProvider = (code: number) => WailsBridge.getCardImage(code);

    fieldRef.current = { field3D, duelManager: manager };
    if (onReadyRef.current) onReadyRef.current(fieldRef.current);

    return () => {
      if (onReadyRef.current) onReadyRef.current(null);
      if (fieldRef.current) {
        fieldRef.current.duelManager.dispose();
        fieldRef.current.field3D.dispose();
      }
      fieldRef.current = null;
    };
  }, [interactive]);

  const handleSurrender = (): void => {
    // 右控组确认弹窗（wSurrender）确认后走到这里；离开对局由 onExit 完成。
    WailsBridge.surrender();
    if (onExit) onExit('menu');
  };

  return (
    <div className="duel-stage" style={{ position: 'relative', width: '100%', height: '100%' }}>
      <div id="duel-canvas-container"></div>

      {/* P2 新面板：单一 store 驱动（原版 wCardImg/wInfos + lp.png 血条） */}
      {!compact && <CardPreviewPanel />}
      <PlayerPanel side="opponent" />
      <PlayerPanel side="player" />
      {/* P3：阶段条（BP/M2/EP 走语义化 Respond* 方法）；回放里阶段每步都变，闪光常亮 → compact 不渲染 */}
      {!compact && <PhaseStrip />}

      {/* P5：全部 2D HUD 走 store（hud.js 已删）；compact 下全部藏起，只留 LP 面板 */}
      {!compact && <HintBar />}
      {!compact && <LogDrawer />}
      {/* 手牌坞在剧场也保留：回放里自己的手牌要从这里看（对手手背由 3D 场地渲染） */}
      <HandDock interactive={interactive} />
      {!compact && <ActionPopup />}
      {interactive && <PromptHost />}
      <VictoryOverlay showVictory={interactive} />

      {showSurrender && (
        <RightControls
          onSurrendered={handleSurrender}
          onLeaveObserver={onExit ? () => onExit('menu') : undefined}
        />
      )}
      {!compact && <ChatOverlay />}
      {/* P4 波 6 补完：DrawSpec 特效（翻卡/放大/揭示/无效化 + 猜硬币文本）与 stTip 悬浮提示 */}
      {!compact && <SpecOverlay />}
      {!compact && <CardTooltip />}
    </div>
  );
}
