/**
 * 阶段条（原版 wPhase）：折叠钮 + BP/M2/EP 按钮 + 48pt 阶段横幅。
 *
 * 按钮可用性来自 store.phasePrompt（duel:select_idlecmd / duel:select_battlecmd），
 * 点击走 P1 的语义化响应（responses.go 编码，前端不碰字节）：
 *   - idlecmd：BP → RespondIdleCmd(0, 6)，EP → RespondIdleCmd(0, 7)
 *   - battlecmd：M2 → RespondBattleCmd(0, 2)，EP → RespondBattleCmd(0, 3)
 * 横幅在 duel:new_phase / duel:win 时弹一次 48pt 大字（原版 showcard 动画）。
 */
import React, { useEffect, useRef, useState, useSyncExternalStore } from 'react';
import { duelStore } from '../duel/store.ts';
import { WailsBridge } from '../wails_bridge.ts';
import {
  PHASE_DRAW, PHASE_STANDBY, PHASE_MAIN1, PHASE_BATTLE_START,
  PHASE_BATTLE, PHASE_MAIN2, PHASE_END,
} from '../domain/constants.ts';

const PHASE_LABELS: [number, string][] = [
  [PHASE_DRAW, '抽卡阶段'],
  [PHASE_STANDBY, '准备阶段'],
  [PHASE_MAIN1, '主要阶段 1'],
  [PHASE_BATTLE_START, '战斗阶段'],
  [PHASE_MAIN2, '主要阶段 2'],
  [PHASE_END, '结束阶段'],
];

function phaseName(phase: number): string {
  if (phase & PHASE_BATTLE_START) return PHASE_LABELS[3][1];
  for (const [bit, label] of PHASE_LABELS) {
    if (phase & bit) return label;
  }
  return '';
}

/** 当前战斗阶段是否处于战斗步骤内（BP 横幅在 step 期间继续显示） */
function inBattle(phase: number): boolean {
  return (phase & (PHASE_BATTLE_START | PHASE_BATTLE)) !== 0;
}

export default function PhaseStrip() {
  const state = useSyncExternalStore(duelStore.subscribe, duelStore.getState);
  const [folded, setFolded] = useState(false);
  const [banner, setBanner] = useState<{ key: number; text: string } | null>(null);
  const bannerSeq = useRef(0);

  const phase = state.phase;
  const win = state.win;
  useEffect(() => {
    const text = win
      ? (win.winner === 0 ? '胜利！' : '败北…')
      : phaseName(phase);
    if (!text) return undefined;
    bannerSeq.current += 1;
    const entry = { key: bannerSeq.current, text };
    setBanner(entry);
    const t = setTimeout(() => {
      // 只清理自己的横幅，避免快速连跳阶段时把新横幅吃掉
      setBanner((cur) => (cur === entry ? null : cur));
    }, 1400);
    return () => clearTimeout(t);
  }, [phase, win]);

  const prompt = state.phasePrompt;
  const canBP = !!prompt && prompt.mode === 'idle' && prompt.canBP;
  const canM2 = !!prompt && prompt.mode === 'battle' && prompt.canM2;
  const canEP = !!prompt && prompt.canEP;

  const onBP = () => { if (canBP) WailsBridge.respondIdleCmd(0, 6); };
  const onM2 = () => { if (canM2) WailsBridge.respondBattleCmd(0, 2); };
  const onEP = () => {
    if (!canEP) return;
    if (prompt!.mode === 'battle') WailsBridge.respondBattleCmd(0, 3);
    else WailsBridge.respondIdleCmd(0, 7);
  };

  return (
    <div id="phase-strip" className={`phase-strip${folded ? ' folded' : ''}`}>
      {/* 回合数菱形块（原版顶部居中的 turn 数字，drawing.cpp (632,10)-(688,50)） */}
      <div id="turn-block" className="turn-block">{state.turn || ''}</div>
      <button
        className="phase-fold"
        title={folded ? '展开阶段条' : '折叠阶段条'}
        onClick={() => setFolded((f) => !f)}
      >{folded ? '«' : '»'}</button>
      {!folded && (
        <div className="phase-buttons">
          <button
            id="phase-btn-bp"
            className={`phase-btn${canBP ? ' actionable' : ''}`}
            disabled={!canBP}
            onClick={onBP}
          >ＢＰ</button>
          <button
            id="phase-btn-m2"
            className={`phase-btn${canM2 ? ' actionable' : ''}`}
            disabled={!canM2}
            onClick={onM2}
          >Ｍ２</button>
          <button
            id="phase-btn-ep"
            className={`phase-btn${canEP ? ' actionable' : ''}`}
            disabled={!canEP}
            onClick={onEP}
          >ＥＰ</button>
        </div>
      )}
      {banner ? (
        <div key={banner.key} className="phase-banner">{banner.text}</div>
      ) : null}
    </div>
  );
}
