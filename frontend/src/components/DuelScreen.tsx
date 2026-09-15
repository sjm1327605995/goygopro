import React, { useEffect } from 'react';
import DuelStage from './DuelStage.tsx';
import CardListOverlay from './CardListOverlay.tsx';
import { WailsBridge } from '../wails_bridge.ts';
import { aiSimulator } from '../duel/ai_simulator.ts';
import { installHotkeys } from '../duel/hotkeys.ts';

interface DuelScreenProps {
  mode: string;
  onNavigate: (screen: string) => void;
}

export default function DuelScreen({ mode, onNavigate }: DuelScreenProps) {
  useEffect(() => {
    // In practice mode there is no server; the local AI simulator drives the
    // duel events. Wait one frame so the 3D board has mounted first.
    if (mode === 'practice') {
      const t = setTimeout(() => aiSimulator.start(), 300);
      // Leaving mid-duel must stop the simulator, or its step chain keeps
      // running against a dead screen and the next practice duel never starts.
      return () => {
        clearTimeout(t);
        aiSimulator.stop();
      };
    }
  }, [mode]);

  // 单人模式：离开决斗画面 = 退出谜题（原版 btnLeaveGame → StopPlay）。
  // 引擎 goroutine 经 stopCh 解除等待并发出 single:ended completed=false。
  useEffect(() => {
    if (mode !== 'single') return;
    return () => WailsBridge.stopSingle();
  }, [mode]);

  // 波 F：A/S/D 按住连锁键 + F1-F8 卡片列表快捷键（gframe event_handler）
  useEffect(() => installHotkeys(), []);

  return (
    <div id="duel-screen" className="screen active">
      <DuelStage onExit={onNavigate} />
      {/* F1-F8 墓地/除外/额外/超量素材列表浮窗 */}
      <CardListOverlay />
    </div>
  );
}
