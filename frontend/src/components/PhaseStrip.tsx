/**
 * 阶段条（原版 wPhase）：当前阶段 + BP/M2/EP 按钮 + 48pt 阶段横幅。
 *
 * 按钮可用性来自 store.phasePrompt（duel:select_idlecmd / duel:select_battlecmd），
 * 点击走 P1 的语义化响应（responses.go 编码，前端不碰字节）：
 *   - idlecmd：BP → RespondIdleCmd(0, 6)，EP → RespondIdleCmd(0, 7)
 *   - battlecmd：M2 → RespondBattleCmd(0, 2)，EP → RespondBattleCmd(0, 3)
 * 横幅在 duel:new_phase / duel:win 时弹一次 48pt 大字（原版 showcard 动画）。
 */
import React, { useContext, useEffect, useRef, useState, useSyncExternalStore } from 'react';
import { duelStore } from '../duel/store.ts';
import { WailsBridge } from '../wails_bridge.ts';
import { phaseBannerText, winBannerText, winReason, PHASE_BANNER_MS, WIN_BANNER_MS } from '../domain/duel_banners.ts';
import { settingsStore } from '../domain/settings.ts';
import { DesignScaleContext } from '../ui/DesignSpace.tsx';
import { PHASE_ABBR } from '../domain/original_phases.ts';
import { PHASE_BATTLE, PHASE_MAIN2, PHASE_END } from '../domain/constants.ts';

export default function PhaseStrip({ bannerOnly = false, descriptive = false }: { bannerOnly?: boolean; descriptive?: boolean } = {}) {
  const state = useSyncExternalStore(duelStore.subscribe, duelStore.getState);
  const settings = useSyncExternalStore(settingsStore.subscribe, settingsStore.getSnapshot);
  const scale = useContext(DesignScaleContext);
  const [banner, setBanner] = useState<{ key: number; text: string } | null>(null);
  const bannerSeq = useRef(0);

  const phase = state.phase;
  const win = state.win;
  const announcement = state.announcement;
  useEffect(() => {
    const text = win ? winBannerText(win.winner) : announcement ? announcement.text || phaseBannerText(phase) : '';
    const duration = win ? Math.max(0, WIN_BANNER_MS - (Date.now() - win.announcedAt))
      : announcement ? Math.max(0, PHASE_BANNER_MS - (Date.now() - announcement.startedAt)) : 0;
    if (!text || !duration) {
      setBanner(null);
      return undefined;
    }
    bannerSeq.current += 1;
    const entry = { key: bannerSeq.current, text };
    setBanner(entry);
    const t = setTimeout(() => {
      // 只清理自己的横幅，避免快速连跳阶段时把新横幅吃掉
      setBanner((cur) => (cur === entry ? null : cur));
    }, duration);
    return () => clearTimeout(t);
  }, [phase, win, announcement]);

  const prompt = state.phasePrompt;
  const canBP = !!prompt && prompt.mode === 'idle' && prompt.canBP;
  const canM2 = !!prompt && prompt.mode === 'battle' && prompt.canM2;
  const canEP = !!prompt && prompt.canEP;
  const reason = win ? winReason(win.winner, win.type, state.names, state.matchKill, !!settings.hide_player_name) : '';
  const bannerNode = banner ? (
    <React.Fragment key={banner.key}>
      <div className="phase-banner-anchor" style={{ transform: `scale(${1 / scale.sx}, ${1 / scale.sy})`, '--banner-enter': `${360 * scale.sx}px`, '--banner-exit': `${400 * scale.sx}px`, '--banner-font-size': `${Math.trunc(48 * scale.sy)}px` } as React.CSSProperties}>
        <div className={`phase-banner${win ? ' win-banner' : ''}`}>{banner.text}</div>
      </div>
      {win && reason && <div className="victory-reason-anchor" style={{ transform: `scale(${1 / scale.sx}, ${1 / scale.sy})` }}>
        <div id="victory-reason" className="victory-reason">{reason}</div>
      </div>}
    </React.Fragment>
  ) : null;

  // bannerOnly（回放剧场 compact）：原版回放也有中央大字阶段横幅
  // （DrawSpec 的 showcard 文本），但阶段条按钮/回合块不渲染——回放里
  // phase 每步都变，actionable 闪光会一直闪。无 id：theater 冒烟的
  // compactHudHidden 断言 #phase-strip 不得出现在 compact 模式。
  if (bannerOnly) {
    return banner ? (
      <div className="phase-strip" data-banner-only="1">
        {bannerNode}
      </div>
    ) : null;
  }

  const onBP = () => { if (canBP) WailsBridge.respondIdleCmd(0, 6); };
  const onM2 = () => { if (canM2) WailsBridge.respondBattleCmd(0, 2); };
  const onEP = () => {
    if (!canEP) return;
    if (prompt!.mode === 'battle') WailsBridge.respondBattleCmd(0, 3);
    else WailsBridge.respondIdleCmd(0, 7);
  };

  return (
    <div id="phase-strip" className="phase-strip">
      {/* 回合数菱形块（原版顶部居中的 turn 数字，drawing.cpp (632,10)-(688,50)） */}
      <div id="turn-block" className="turn-block">{state.turn || ''}</div>
      {(
        <div className="phase-buttons">
          {/* game.cpp:343-353: one status button; BP and M2 share a position. */}
          {phase !== 0 && <button id="phase-status" className="phase-btn phase-display current" aria-disabled="true">{PHASE_ABBR[phase] || 'ＢＰ'}</button>}
          <button
            id="phase-btn-bp"
            className={`phase-btn${(phase & PHASE_BATTLE) ? ' current-phase' : ''}${canBP ? ' actionable' : ''}`}
            disabled={!canBP}
            style={{display:canBP?'block':'none',left:160}}
            onClick={onBP}
          >{descriptive ? '战斗阶段' : 'ＢＰ'}</button>
          <button
            id="phase-btn-m2"
            className={`phase-btn${(phase & PHASE_MAIN2) ? ' current-phase' : ''}${canM2 ? ' actionable' : ''}`}
            disabled={!canM2}
            style={{display:canM2?'block':'none',left:160}}
            onClick={onM2}
          >{descriptive ? '主要阶段 2' : 'Ｍ２'}</button>
          <button
            id="phase-btn-ep"
            className={`phase-btn${(phase & PHASE_END) ? ' current-phase' : ''}${canEP ? ' actionable' : ''}`}
            disabled={!canEP}
            style={{display:canEP?'block':'none',left:320}}
            onClick={onEP}
          >{descriptive ? '结束回合' : 'ＥＰ'}</button>
        </div>
      )}
      {bannerNode}
    </div>
  );
}
