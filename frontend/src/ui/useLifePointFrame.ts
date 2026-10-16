import { useEffect, useState } from 'react';
import { lifePointFrame, type LifePointChange } from '../domain/life_points.ts';

export function useLifePointFrame(change: LifePointChange | null) {
  const [, tick] = useState(0);
  useEffect(() => {
    if (!change) return;
    let raf = 0;
    const update = () => {
      tick(value => value + 1);
      if (!lifePointFrame(change, Date.now()).done) raf = requestAnimationFrame(update);
    };
    update();
    return () => cancelAnimationFrame(raf);
  }, [change]);
  return change ? lifePointFrame(change, Date.now()) : null;
}
