import { PHASE_DRAW, PHASE_STANDBY, PHASE_MAIN1, PHASE_BATTLE_START, PHASE_MAIN2, PHASE_END } from './constants.ts';
import { victoryString } from './sys_strings.ts';
import { cardName } from './card_names.ts';

// duelclient.cpp WaitFrameSignal and drawing.cpp DrawSpec case 101 (60 FPS).
export const PHASE_BANNER_MS = 40 * 1000 / 60;
export const WIN_BANNER_MS = 120 * 1000 / 60;

export function phaseBannerText(phase: number): string {
  return ({
    [PHASE_DRAW]: 'Draw Phase',
    [PHASE_STANDBY]: 'Standby Phase',
    [PHASE_MAIN1]: 'Main Phase 1',
    [PHASE_BATTLE_START]: 'Battle Phase',
    [PHASE_MAIN2]: 'Main Phase 2',
    [PHASE_END]: 'End Phase',
  } as Record<number, string>)[phase] || '';
}

export function winBannerText(winner: number): string {
  return winner === 2 ? 'Draw Game' : winner === 0 ? 'You Win!' : 'You Lose!';
}

export function winReason(winner: number, type: number, names: [string, string], matchKill: number | null, hideNames: boolean): string {
  if (winner === 2) return '';
  if (matchKill) return `由于「${cardName(matchKill)}」获得比赛胜利`;
  const reason = victoryString(type);
  // The ordinary reason describes the losing player (duelclient.cpp:1224-1238).
  return type < 0x10 ? `[${hideNames ? '********' : names[1 - winner]}] ${reason}` : reason;
}
