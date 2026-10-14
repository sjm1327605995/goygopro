import { useEffect, useRef, useSyncExternalStore } from 'react';
import { duelStore } from '../duel/store.ts';
import { soundManager } from '../audio/sound_manager.ts';

// DrawSpec's transient result and reason are rendered together in PhaseStrip.
export default function VictoryOverlay({ showVictory = true }: { showVictory?: boolean }) {
  const { win } = useSyncExternalStore(duelStore.subscribe, duelStore.getState);
  const soundedRef = useRef<number | null>(null);
  useEffect(() => {
    if (!win || !showVictory) {
      soundedRef.current = null;
      return;
    }
    if (soundedRef.current === win.announcedAt) return;
    soundedRef.current = win.announcedAt;
    if (win.winner === 0) soundManager.playVictory();
    else if (win.winner === 1) soundManager.playDefeat();
  }, [win, showVictory]);
  return null;
}
