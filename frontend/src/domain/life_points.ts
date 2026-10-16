export type LifePointKind = 'damage' | 'recover' | 'pay' | 'update';
export interface LifePointChange {
  player: 0 | 1;
  from: number;
  to: number;
  amount: number;
  kind: LifePointKind;
  startedAt: number;
}

// duelclient.cpp MSG_DAMAGE/RECOVER/PAY_LPCOST and drawing.cpp lpframe.
export function lifePointFrame(change: LifePointChange, now: number) {
  const delay = change.kind === 'update' ? 0 : 30;
  const frame = Math.max(0, Math.floor((now - change.startedAt) * 60 / 1000));
  const steps = Math.min(10, Math.max(0, frame - delay + 1));
  const done = frame >= delay + 11;
  return {
    value: done ? change.to : change.from - Math.trunc((change.from - change.to) / 10) * steps,
    floating: change.kind !== 'update' && !done,
    alpha: (255 - steps * 25) / 255,
    color: change.kind === 'recover' ? '#00ff00' : change.kind === 'pay' ? '#0000ff' : '#ff0000',
    done,
  };
}
