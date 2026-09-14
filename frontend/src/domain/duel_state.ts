/**
 * DuelState store — 单一状态源。
 *
 * 订阅/快照模式（React 用 useSyncExternalStore 消费）：
 *   const state = useSyncExternalStore(duelStore.subscribe, duelStore.getState);
 *
 * 纯领域模块：不 import DOM/Three/事件总线；事件接线在 duel/store.js。
 * 所有状态变更都经 reducer.ts 的 applyEvent（引擎事件）或下面的 UI 动作。
 */
import {
  applyEvent, applyInspect, applyInspectInfo, initialDuelState,
  type DuelState, type CardInspection, type PopupAction,
} from './reducer.ts';

export type { DuelState, CardInspection, PileCounts, HandCard, LogEntry, BoardSide, BoardCard, PopupAction } from './reducer.ts';

export interface DuelStore {
  getState(): DuelState;
  subscribe(fn: () => void): () => void;
  /** 事件总线→store 的唯一入口（duel/store.js 对每个转发事件调一次） */
  dispatch(name: string, data: unknown): void;
  /** UI 动作：左面板查看某张卡（随后用 resolveInspectInfo 回填卡信息） */
  inspect(code: number): void;
  resolveInspectInfo(code: number, info: Record<string, unknown> | null): void;
  /** UI 动作：清空全部对局状态（回放 seek/重开时先归零再重放） */
  reset(): void;
  /** UI 动作：手牌/场地点击的 2D 动作菜单 */
  openActionPopup(x: number, y: number, actions: PopupAction[]): void;
  closeActionPopup(): void;
  /** UI 动作：设置 stHintMsg 提示条文本（duel:hint 的 HINT_EVENT/HINT_MESSAGE） */
  setHint(text: string): void;
}

export function createDuelStore(): DuelStore {
  let state: DuelState = initialDuelState();
  const listeners = new Set<() => void>();

  const notify = () => {
    for (const fn of listeners) fn();
  };
  const commit = (next: DuelState) => {
    if (next === state) return;
    state = next;
    notify();
  };

  return {
    getState: () => state,
    subscribe(fn: () => void) {
      listeners.add(fn);
      return () => listeners.delete(fn);
    },
    dispatch(name: string, data: unknown) {
      commit(applyEvent(state, name, data));
    },
    inspect(code: number) {
      if (state.inspection && state.inspection.code === code) return;
      commit(applyInspect(state, code));
    },
    resolveInspectInfo(code: number, info: Record<string, unknown> | null) {
      commit(applyInspectInfo(state, code, info));
    },
    reset() {
      commit(initialDuelState());
    },
    openActionPopup(x: number, y: number, actions: PopupAction[]) {
      commit({ ...state, actionPopup: { x, y, actions } });
    },
    closeActionPopup() {
      if (!state.actionPopup) return;
      commit({ ...state, actionPopup: null });
    },
    setHint(text: string) {
      if (state.hint === text) return;
      commit({ ...state, hint: text });
    },
  };
}
