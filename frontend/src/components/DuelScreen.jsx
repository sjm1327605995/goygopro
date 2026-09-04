import React, { useEffect } from 'react';
import DuelStage from './DuelStage.jsx';
import { aiSimulator } from '../duel/ai_simulator.js';

export default function DuelScreen({ mode, onNavigate }) {
  useEffect(() => {
    // In practice mode there is no server; the local AI simulator drives the
    // duel events. Wait one frame so the 3D board has mounted first.
    if (mode === 'practice') {
      const t = setTimeout(() => aiSimulator.start(), 300);
      return () => clearTimeout(t);
    }
  }, [mode]);

  return (
    <div id="duel-screen" className="screen active">
      <DuelStage onExit={onNavigate} />
    </div>
  );
}
