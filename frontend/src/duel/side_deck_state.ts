/**
 * 最近一次 CTOS_UPDATE_DECK 发送的卡组缓存（主/额外/副三区）。
 *
 * 换副卡组（STOC_CHANGE_SIDE）必须以"上一局实际使用的卡组"为起点逐张
 * 更换——服务端 LoadSide 要求新卡组的主/额外/副数量与旧卡组完全一致
 * （core/duel/deck_manager.go）。这个起点无法从磁盘卡组推断（玩家可能在
 * 建房后改过存档），所以在发送点（大厅准备、换副确认）就地记录。
 */

export interface SideDeckSnapshot {
  main: number[];
  extra: number[];
  side: number[];
}

let lastSent: SideDeckSnapshot | null = null;

export function setLastSentDeck(deck: { main: number[]; extra: number[]; side: number[] }): void {
  lastSent = { main: [...deck.main], extra: [...deck.extra], side: [...deck.side] };
}

export function getLastSentDeck(): SideDeckSnapshot | null {
  return lastSent ? { main: [...lastSent.main], extra: [...lastSent.extra], side: [...lastSent.side] } : null;
}

export function clearLastSentDeck(): void {
  lastSent = null;
}
